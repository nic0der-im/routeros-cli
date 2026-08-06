package cmd

import (
	"os"
	"strings"
	"testing"
)

func TestJoinHostPort(t *testing.T) {
	tests := []struct {
		host, port, want string
	}{
		{"192.168.88.1", "8728", "192.168.88.1:8728"},
		{"192.168.88.1:8729", "", "192.168.88.1:8729"},
		{"router.local", "8728", "router.local:8728"},
		{"router.local", "", "router.local:8728"},
	}
	for _, tt := range tests {
		got, err := joinHostPort(tt.host, tt.port)
		if err != nil {
			t.Fatalf("joinHostPort(%q,%q): %v", tt.host, tt.port, err)
		}
		if got != tt.want {
			t.Errorf("joinHostPort(%q,%q)=%q want %q", tt.host, tt.port, got, tt.want)
		}
	}
}

func TestJoinHostPortEmptyHost(t *testing.T) {
	if _, err := joinHostPort("", "8728"); err == nil {
		t.Fatal("expected error")
	}
}

func TestReadPasswordFromStrictLineContract(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
		err   string
	}{
		{name: "EOF without terminator", input: "secret", want: "secret"},
		{name: "preserve surrounding spaces", input: " secret 	\n", want: " secret 	"},
		{name: "CRLF", input: "secret\r\n", want: "secret"},
		{name: "empty", input: "\n", err: "empty password"},
		{name: "extra content", input: "secret\nsecond\n", err: "extra or malformed input"},
		{name: "NUL", input: "secret\x00\n", err: "NUL byte"},
		{name: "lone CR", input: "secret\r", err: "extra or malformed input"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := readPasswordFrom(strings.NewReader(tt.input))
			if tt.err != "" {
				if err == nil || !strings.Contains(err.Error(), tt.err) {
					t.Fatalf("error=%v, want %q", err, tt.err)
				}
				return
			}
			if err != nil || got != tt.want {
				t.Fatalf("password accepted=%v err=%v", got != "", err)
			}
		})
	}
}

func TestDeviceAuthPasswordStdinCompatibility(t *testing.T) {
	if newDeviceAuthSetCmd().Flags().Lookup(passwordStdinFlag) == nil {
		t.Fatal("device auth lost --password-stdin")
	}
	oldStdin := os.Stdin
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdin = r
	t.Cleanup(func() {
		os.Stdin = oldStdin
		_ = r.Close()
	})
	if _, err := w.WriteString("device-auth-secret\n"); err != nil {
		t.Fatal(err)
	}
	_ = w.Close()
	got, err := readPassword(true)
	if err != nil || got != "device-auth-secret" {
		t.Fatalf("device auth stdin compatibility failed (err=%v)", err)
	}
}
