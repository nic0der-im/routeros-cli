package filexfer

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

func downloadSFTP(ctx context.Context, host, port, user, pass, remoteName, localPath string) (int64, error) {
	if host == "" {
		return 0, fmt.Errorf("sftp host required")
	}
	if port == "" {
		port = "22"
	}
	addr := net.JoinHostPort(host, port)

	config := &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{
			ssh.Password(pass),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), //nolint:gosec // first-connect MSP automation; TOFU later
		Timeout:         20 * time.Second,
	}

	dialer := net.Dialer{Timeout: 20 * time.Second}
	raw, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return 0, fmt.Errorf("sftp dial %s: %w", addr, err)
	}

	type result struct {
		client *ssh.Client
		err    error
	}
	ch := make(chan result, 1)
	go func() {
		conn, chans, reqs, err := ssh.NewClientConn(raw, addr, config)
		if err != nil {
			_ = raw.Close()
			ch <- result{err: err}
			return
		}
		ch <- result{client: ssh.NewClient(conn, chans, reqs)}
	}()

	var sshClient *ssh.Client
	select {
	case <-ctx.Done():
		_ = raw.Close()
		return 0, ctx.Err()
	case r := <-ch:
		if r.err != nil {
			return 0, fmt.Errorf("sftp ssh handshake: %w", r.err)
		}
		sshClient = r.client
	}
	defer func() { _ = sshClient.Close() }()

	sftpClient, err := sftp.NewClient(sshClient)
	if err != nil {
		return 0, fmt.Errorf("sftp client: %w", err)
	}
	defer func() { _ = sftpClient.Close() }()

	remote, err := sftpClient.Open(remoteName)
	if err != nil {
		return 0, fmt.Errorf("sftp open %q: %w", remoteName, err)
	}
	defer func() { _ = remote.Close() }()

	local, err := os.OpenFile(localPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return 0, err
	}
	defer func() { _ = local.Close() }()

	n, err := io.Copy(local, remote)
	if err != nil {
		return n, fmt.Errorf("sftp copy: %w", err)
	}
	return n, nil
}
