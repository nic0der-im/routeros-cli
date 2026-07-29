package cmd

import "testing"

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
