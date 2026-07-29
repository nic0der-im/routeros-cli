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
)

// Via selects the transfer mechanism.
type Via string

const (
	ViaAuto Via = "auto"
	ViaAPI  Via = "api"
	ViaFTP  Via = "ftp"
)

// Download pulls a named file from the router to localPath.
func Download(ctx context.Context, c client.Client, name, localPath string, via Via, ftpHost, user, pass string) (int64, error) {
	switch via {
	case ViaAPI:
		return downloadAPI(ctx, c, name, localPath)
	case ViaFTP:
		return downloadFTP(ctx, ftpHost, user, pass, name, localPath)
	default:
		n, err := downloadAPI(ctx, c, name, localPath)
		if err == nil && n > 0 {
			return n, nil
		}
		if ftpHost == "" {
			if err != nil {
				return 0, fmt.Errorf("API download failed (%v); enable FTP or pass --via ftp", err)
			}
			return 0, fmt.Errorf("API returned empty contents for %q; enable FTP or pass --via ftp", name)
		}
		return downloadFTP(ctx, ftpHost, user, pass, name, localPath)
	}
}

func downloadAPI(ctx context.Context, c client.Client, name, localPath string) (int64, error) {
	result, err := c.Run(ctx, "/file/print", "?name="+name, "=.proplist=name,size,contents,type")
	if err != nil {
		// older firmware may not like .proplist
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
		return 0, fmt.Errorf("file %q has no API contents (binary files usually require FTP)", name)
	}
	if err := os.WriteFile(localPath, []byte(contents), 0o644); err != nil {
		return 0, err
	}
	return int64(len(contents)), nil
}

func downloadFTP(ctx context.Context, host, user, pass, remoteName, localPath string) (int64, error) {
	host = strings.TrimSpace(host)
	if host == "" {
		return 0, fmt.Errorf("FTP host required")
	}
	// strip API port if present
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
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

	f, err := os.Create(localPath)
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
