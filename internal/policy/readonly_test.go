package policy

import (
	"context"
	"errors"
	"testing"

	"github.com/nic0der-im/routeros-cli/internal/client"
)

func TestIsWrite(t *testing.T) {
	tests := []struct {
		cmd  string
		want bool
	}{
		{"/system/resource/print", false},
		{"/interface/print", false},
		{"/export", false},
		{"/interface/monitor-traffic", false},
		{"/tool/ping", false},
		{"/ip/address/add", true},
		{"/ip/address/remove", true},
		{"/ip/firewall/filter/set", true},
		{"/system/reboot", true},
		{"/system/backup/save", true},
		{"/ip/firewall/filter/enable", true},
		{"/system/script/run", true},
		{"/unknown/thing", true},
	}

	for _, tt := range tests {
		t.Run(tt.cmd, func(t *testing.T) {
			if got := IsWrite(tt.cmd); got != tt.want {
				t.Errorf("IsWrite(%q) = %v, want %v", tt.cmd, got, tt.want)
			}
		})
	}
}

func TestReadOnlyClient_AllowsPrint(t *testing.T) {
	mock := client.NewMockClient()
	mock.RunFunc = func(_ context.Context, command string, _ ...string) (*client.Result, error) {
		return &client.Result{Sentences: []map[string]string{{"name": "ok"}}}, nil
	}

	ro := WrapReadOnly(mock)
	result, err := ro.Run(context.Background(), "/system/identity/print")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Sentences) != 1 {
		t.Fatalf("expected 1 sentence, got %d", len(result.Sentences))
	}
}

func TestReadOnlyClient_BlocksWrite(t *testing.T) {
	mock := client.NewMockClient()
	ro := WrapReadOnly(mock)

	_, err := ro.Run(context.Background(), "/ip/address/add", "=address=10.0.0.1/24")
	if err == nil {
		t.Fatal("expected read-only error")
	}
	var roErr *ErrReadOnly
	if !errors.As(err, &roErr) {
		t.Fatalf("expected ErrReadOnly, got %T: %v", err, err)
	}
	if roErr.Command != "/ip/address/add" {
		t.Errorf("command = %q", roErr.Command)
	}
}

func TestReadOnlyClient_Close(t *testing.T) {
	mock := client.NewMockClient()
	ro := WrapReadOnly(mock)
	if err := ro.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if !mock.Closed {
		t.Error("expected inner client closed")
	}
}
