package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/nic0der-im/routeros-cli/internal/client"
	"github.com/nic0der-im/routeros-cli/internal/filexfer"
	"github.com/nic0der-im/routeros-cli/internal/publicip"
)

// exportTextOptions configures a local text configuration export.
type exportTextOptions struct {
	DestPath     string // local file path, or directory to place the .rsc into
	Via          string
	SourceIP     string
	PublicIPURL  string
	EphemeralSSH bool
	KeepRemote   bool
	Host         string
	User         string
	Pass         string
	Status       func(string)
}

// exportTextToLocal exports router text config to a local file.
// Prefer the API stream when non-empty; otherwise /export file= + download.
// Does not enforce ensureWritable — callers that need guardrails must check first.
func exportTextToLocal(ctx context.Context, c client.Client, deviceName string, opts exportTextOptions) (localPath string, nbytes int64, err error) {
	if opts.Via == "" {
		opts.Via = "sftp"
	}
	if opts.PublicIPURL == "" {
		opts.PublicIPURL = publicip.DefaultURL
	}

	dest := opts.DestPath
	if dest == "" {
		return "", 0, fmt.Errorf("export destination path is required")
	}

	// Fast path: some devices still stream export text over the API.
	if result, err := c.Run(ctx, "/export"); err == nil {
		if export := extractExportText(result); export != "" {
			out, err := resolveExportOutPath(dest, defaultExportName+".rsc")
			if err != nil {
				return "", 0, err
			}
			if err := os.WriteFile(out, []byte(export), 0o600); err != nil {
				return "", 0, fmt.Errorf("writing export to %q: %w", out, err)
			}
			return out, int64(len(export)), nil
		}
	}

	remoteBase := fmt.Sprintf("%s-%d", defaultExportName, time.Now().Unix())
	remoteName := remoteBase + ".rsc"
	if _, err := c.Run(ctx, "/export", "=file="+remoteBase); err != nil {
		return "", 0, fmt.Errorf("exporting configuration on %q: %w", deviceName, err)
	}

	out, err := resolveExportOutPath(dest, remoteName)
	if err != nil {
		return "", 0, err
	}

	ephem := opts.EphemeralSSH
	status := opts.Status
	if status == nil {
		status = func(string) {}
	}
	n, err := filexfer.Download(ctx, remoteName, out, filexfer.Options{
		Via:          filexfer.Via(opts.Via),
		Host:         opts.Host,
		User:         opts.User,
		Pass:         opts.Pass,
		SourceIP:     opts.SourceIP,
		PublicIPURL:  opts.PublicIPURL,
		EphemeralSSH: &ephem,
		Client:       c,
		OnStatus:     status,
	})
	if !opts.KeepRemote {
		if _, rerr := c.Run(context.WithoutCancel(ctx), "/file/remove", "=numbers="+remoteName); rerr != nil {
			status(fmt.Sprintf("warning: could not remove remote %q: %v", remoteName, rerr))
		}
	}
	if err != nil {
		return "", 0, fmt.Errorf("downloading export: %w", err)
	}
	if n == 0 {
		return "", 0, fmt.Errorf("downloaded empty export from %q", deviceName)
	}
	return out, n, nil
}

func resolveExportOutPath(dest, defaultName string) (string, error) {
	st, err := os.Stat(dest)
	if err == nil && st.IsDir() {
		return filepath.Join(dest, defaultName), nil
	}
	if err != nil && !os.IsNotExist(err) {
		return "", err
	}
	// Treat as file path (may not exist yet). Ensure parent dir exists.
	if err := os.MkdirAll(filepath.Dir(dest), 0o700); err != nil {
		return "", fmt.Errorf("creating export directory: %w", err)
	}
	return dest, nil
}

func statusWriter(w io.Writer) func(string) {
	return func(msg string) {
		if w == nil {
			return
		}
		fmt.Fprintf(w, "  · %s\n", msg)
	}
}
