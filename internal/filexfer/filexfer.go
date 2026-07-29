// Package filexfer downloads RouterOS /file contents to the local workstation.
package filexfer

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"time"

	"github.com/jlaffaye/ftp"
	"github.com/nic0der-im/routeros-cli/internal/client"
	"github.com/nic0der-im/routeros-cli/internal/publicip"
)

// Via selects the transfer mechanism.
type Via string

const (
	ViaAuto Via = "auto"
	ViaAPI  Via = "api"
	ViaFTP  Via = "ftp"
	ViaSFTP Via = "sftp"
)

// Options controls how a file is downloaded from the router.
type Options struct {
	Via Via

	// Host is the router address (host or host:apiPort).
	Host string
	User string
	Pass string

	// SourceIP overrides public-IP detection for the ephemeral SSH allowlist.
	SourceIP string
	// PublicIPURL is the HTTPS endpoint used to discover the caller public IP.
	PublicIPURL string
	// EphemeralSSH temporarily narrows/enables SSH allowlist for the transfer.
	// Default true for ViaSFTP/ViaAuto when Client is set.
	EphemeralSSH *bool

	// Client is required for ephemeral SSH mutations and preferred for auto.
	Client client.Client

	// OnStatus is an optional progress callback (human messages).
	OnStatus func(string)
}

func (o Options) ephemeralEnabled() bool {
	if o.EphemeralSSH == nil {
		return true
	}
	return *o.EphemeralSSH
}

func status(o Options, msg string) {
	if o.OnStatus != nil {
		o.OnStatus(msg)
	}
}

// Download pulls a named file from the router to localPath.
func Download(ctx context.Context, name, localPath string, opts Options) (int64, error) {
	via := opts.Via
	if via == "" {
		via = ViaAuto
	}

	switch via {
	case ViaAPI:
		if opts.Client == nil {
			return 0, fmt.Errorf("api download requires a RouterOS client")
		}
		return downloadAPI(ctx, opts.Client, name, localPath)
	case ViaFTP:
		return downloadFTP(ctx, opts.Host, opts.User, opts.Pass, name, localPath)
	case ViaSFTP:
		return downloadSFTPEphemeral(ctx, name, localPath, opts)
	default: // auto
		if opts.Client != nil {
			if n, err := downloadAPI(ctx, opts.Client, name, localPath); err == nil && n > 0 {
				return n, nil
			}
		}
		return downloadSFTPEphemeral(ctx, name, localPath, opts)
	}
}

func downloadSFTPEphemeral(ctx context.Context, name, localPath string, opts Options) (n int64, err error) {
	host := hostOnly(opts.Host)
	if host == "" {
		return 0, fmt.Errorf("sftp host required")
	}
	if opts.User == "" || opts.Pass == "" {
		return 0, fmt.Errorf("sftp requires username/password")
	}

	port := "22"
	var sshState *SSHServiceState
	restored := true

	if opts.Client != nil && opts.ephemeralEnabled() {
		sshState, err = CaptureSSHService(ctx, opts.Client)
		if err != nil {
			return 0, err
		}
		port = sshState.SSHPort()

		allowIPs, err := collectAllowIPs(ctx, host, opts)
		if err != nil {
			return 0, err
		}
		merged := MergeAddressList(sshState.Address, allowIPs...)
		status(opts, fmt.Sprintf("ephemeral SSH allowlist → %s (was %q)", merged, sshState.Address))
		if err := ApplySSHAccess(ctx, opts.Client, sshState, merged); err != nil {
			return 0, err
		}
		restored = false
		defer func() {
			if restored {
				return
			}
			status(opts, "restoring SSH service allowlist")
			if rerr := RestoreSSHService(context.WithoutCancel(ctx), opts.Client, sshState); rerr != nil {
				if err == nil {
					err = rerr
				} else {
					err = fmt.Errorf("%v; additionally restore failed: %w", err, rerr)
				}
			} else {
				restored = true
			}
		}()
		// Brief settle so service address applies before dial.
		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		case <-time.After(400 * time.Millisecond):
		}
	} else if opts.Client != nil {
		if st, e := CaptureSSHService(ctx, opts.Client); e == nil {
			port = st.SSHPort()
		}
	}

	status(opts, fmt.Sprintf("sftp %s@%s:%s get %s", opts.User, host, port, name))
	n, err = downloadSFTP(ctx, host, port, opts.User, opts.Pass, name, localPath)
	if err != nil {
		return n, err
	}
	return n, nil
}

func collectAllowIPs(ctx context.Context, host string, opts Options) ([]string, error) {
	var out []string
	if opts.SourceIP != "" {
		cidr, err := CIDROrIP(opts.SourceIP)
		if err != nil {
			return nil, fmt.Errorf("source-ip: %w", err)
		}
		out = append(out, cidr)
		return out, nil
	}
	if lip, err := publicip.LocalToward(host); err == nil && lip != "" {
		if cidr, err := CIDROrIP(lip); err == nil {
			out = append(out, cidr)
			status(opts, "local egress IP "+lip)
		}
	}
	if pip, err := publicip.Detect(ctx, opts.PublicIPURL); err == nil && pip != "" {
		if cidr, err := CIDROrIP(pip); err == nil {
			out = append(out, cidr)
			status(opts, "public IP "+pip)
		}
	} else if len(out) == 0 {
		return nil, fmt.Errorf("could not detect public IP (%v); pass --source-ip", err)
	}
	return out, nil
}

func downloadAPI(ctx context.Context, c client.Client, name, localPath string) (int64, error) {
	result, err := c.Run(ctx, "/file/print", "?name="+name, "=.proplist=name,size,contents,type")
	if err != nil {
		result, err = c.Run(ctx, "/file/print", "?name="+name)
		if err != nil {
			return 0, err
		}
	}
	if len(result.Sentences) == 0 {
		return 0, fmt.Errorf("file %q not found on router", name)
	}
	contents := result.Sentences[0]["contents"]
	if contents == "" {
		return 0, fmt.Errorf("file %q has no API contents (binary files usually require SFTP)", name)
	}
	if err := os.WriteFile(localPath, []byte(contents), 0o600); err != nil {
		return 0, err
	}
	return int64(len(contents)), nil
}

func downloadFTP(ctx context.Context, host, user, pass, remoteName, localPath string) (int64, error) {
	host = hostOnly(host)
	if host == "" {
		return 0, fmt.Errorf("FTP host required")
	}
	addr := net.JoinHostPort(host, "21")

	dialer := net.Dialer{Timeout: 15 * time.Second}
	conn, err := ftp.Dial(addr, ftp.DialWithContext(ctx), ftp.DialWithDialFunc(func(network, address string) (net.Conn, error) {
		return dialer.DialContext(ctx, network, address)
	}))
	if err != nil {
		return 0, fmt.Errorf("ftp dial %s: %w", addr, err)
	}
	defer func() { _ = conn.Quit() }()

	if err := conn.Login(user, pass); err != nil {
		return 0, fmt.Errorf("ftp login: %w", err)
	}

	resp, err := conn.Retr(remoteName)
	if err != nil {
		return 0, fmt.Errorf("ftp retr %q: %w", remoteName, err)
	}
	defer func() { _ = resp.Close() }()

	f, err := os.OpenFile(localPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return 0, err
	}
	defer func() { _ = f.Close() }()

	n, err := io.Copy(f, resp)
	if err != nil {
		return n, err
	}
	return n, nil
}

func hostOnly(address string) string {
	host := strings.TrimSpace(address)
	if host == "" {
		return ""
	}
	if h, _, err := net.SplitHostPort(host); err == nil {
		return h
	}
	if i := strings.LastIndex(host, ":"); i > 0 {
		// avoid mangling IPv6 without brackets
		if strings.Count(host, ":") == 1 {
			return host[:i]
		}
	}
	return host
}
