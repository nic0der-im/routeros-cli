// Package policy enforces access modes such as read-only for agent workflows.
package policy

import (
	"context"
	"fmt"
	"strings"

	"github.com/nic0der-im/routeros-cli/internal/client"
)

// ErrReadOnly is returned when a write command is attempted in read-only mode.
type ErrReadOnly struct {
	Command string
}

func (e *ErrReadOnly) Error() string {
	return fmt.Sprintf("read-only mode: refusing write command %q", e.Command)
}

// IsWrite reports whether a RouterOS API command mutates device state.
func IsWrite(command string) bool {
	cmd := strings.TrimSpace(command)
	if cmd == "" {
		return false
	}

	lower := strings.ToLower(cmd)

	// Explicit allowlist of safe read paths.
	if strings.HasSuffix(lower, "/print") ||
		strings.Contains(lower, "/monitor") ||
		lower == "/export" ||
		strings.HasPrefix(lower, "/tool/ping") ||
		strings.HasPrefix(lower, "/tool/torch") {
		return false
	}

	writeSuffixes := []string{
		"/add", "/set", "/remove", "/enable", "/disable",
		"/move", "/reset", "/reboot", "/shutdown",
		"/save", "/load", "/import", "/restore",
		"/run", "/cancel", "/comment",
	}
	for _, s := range writeSuffixes {
		if strings.HasSuffix(lower, s) {
			return true
		}
	}

	// Conservative: unknown non-print commands are treated as writes.
	return true
}

// ReadOnlyClient wraps a Client and rejects mutating commands.
type ReadOnlyClient struct {
	Inner client.Client
}

// Compile-time check: ReadOnlyClient must satisfy client.Client.
var _ client.Client = (*ReadOnlyClient)(nil)

// WrapReadOnly returns a Client that rejects write commands.
func WrapReadOnly(inner client.Client) client.Client {
	return &ReadOnlyClient{Inner: inner}
}

// Run rejects write commands and forwards read commands to the inner client.
func (c *ReadOnlyClient) Run(ctx context.Context, command string, args ...string) (*client.Result, error) {
	if IsWrite(command) {
		return nil, &ErrReadOnly{Command: command}
	}
	return c.Inner.Run(ctx, command, args...)
}

// Close closes the underlying client.
func (c *ReadOnlyClient) Close() error {
	return c.Inner.Close()
}
