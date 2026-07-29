package filexfer

import (
	"context"
	"fmt"
	"strings"

	"github.com/nic0der-im/routeros-cli/internal/client"
)

// SSHServiceState is a snapshot of /ip/service ssh for restore.
type SSHServiceState struct {
	ID       string
	Name     string
	Port     string
	Address  string
	Disabled string
}

// CaptureSSHService reads the ssh service row.
func CaptureSSHService(ctx context.Context, c client.Client) (*SSHServiceState, error) {
	result, err := c.Run(ctx, "/ip/service/print", "?name=ssh")
	if err != nil {
		return nil, fmt.Errorf("reading ssh service: %w", err)
	}
	if len(result.Sentences) == 0 {
		return nil, fmt.Errorf("ssh service not found")
	}
	s := result.Sentences[0]
	return &SSHServiceState{
		ID:       s[".id"],
		Name:     s["name"],
		Port:     s["port"],
		Address:  s["address"],
		Disabled: s["disabled"],
	}, nil
}

// ApplySSHAccess enables SSH (if needed) and sets the address allowlist.
func ApplySSHAccess(ctx context.Context, c client.Client, state *SSHServiceState, address string) error {
	if state == nil {
		return fmt.Errorf("nil ssh state")
	}
	args := []string{}
	if state.ID != "" {
		args = append(args, "=.id="+state.ID)
	} else {
		args = append(args, "=numbers=ssh")
	}
	args = append(args, "=disabled=no", "=address="+address)
	_, err := c.Run(ctx, "/ip/service/set", args...)
	if err != nil {
		return fmt.Errorf("applying ephemeral ssh allowlist: %w", err)
	}
	return nil
}

// RestoreSSHService restores a previously captured ssh service state.
func RestoreSSHService(ctx context.Context, c client.Client, state *SSHServiceState) error {
	if state == nil {
		return nil
	}
	args := []string{}
	if state.ID != "" {
		args = append(args, "=.id="+state.ID)
	} else {
		args = append(args, "=numbers=ssh")
	}
	disabled := state.Disabled
	if disabled == "" {
		disabled = "false"
	}
	args = append(args, "=disabled="+disabled, "=address="+state.Address)
	_, err := c.Run(ctx, "/ip/service/set", args...)
	if err != nil {
		return fmt.Errorf("restoring ssh service: %w", err)
	}
	return nil
}

// SSHPort returns the TCP port from state or 22.
func (s *SSHServiceState) SSHPort() string {
	if s == nil || strings.TrimSpace(s.Port) == "" {
		return "22"
	}
	return s.Port
}
