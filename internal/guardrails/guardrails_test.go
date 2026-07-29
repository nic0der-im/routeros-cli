package guardrails

import (
	"errors"
	"strings"
	"testing"
)

func TestRequireSafeSession(t *testing.T) {
	tests := []struct {
		env  string
		want bool
	}{
		{"prod", true},
		{"production", true},
		{"PROD", true},
		{"staging", true},
		{"lab", false},
		{"", false},
		{"dev", false},
	}
	for _, tt := range tests {
		if got := RequireSafeSession(tt.env); got != tt.want {
			t.Errorf("RequireSafeSession(%q) = %v, want %v", tt.env, got, tt.want)
		}
	}
}

func TestRequireBackupBeforeSafeSession(t *testing.T) {
	tests := []struct {
		env     string
		require bool
		want    bool
	}{
		{"prod", false, true},
		{"production", false, true},
		{"PROD", false, true},
		{"staging", false, false},
		{"lab", false, false},
		{"", false, false},
		{"staging", true, true},
		{"lab", true, true},
		{"prod", true, true},
	}
	for _, tt := range tests {
		if got := RequireBackupBeforeSafeSession(tt.env, tt.require); got != tt.want {
			t.Errorf("RequireBackupBeforeSafeSession(%q, %v) = %v, want %v", tt.env, tt.require, got, tt.want)
		}
	}
}

func TestCheckSafeSession(t *testing.T) {
	if err := CheckSafeSession("lab", "r1", false); err != nil {
		t.Fatalf("lab without session: %v", err)
	}
	if err := CheckSafeSession("prod", "edge", true); err != nil {
		t.Fatalf("prod with session: %v", err)
	}
	err := CheckSafeSession("staging", "edge", false)
	if err == nil {
		t.Fatal("expected error for staging without session")
	}
	var req *ErrSafeSessionRequired
	if !errors.As(err, &req) {
		t.Fatalf("got %T: %v", err, err)
	}
	if !strings.Contains(err.Error(), "session begin --safe") {
		t.Errorf("error should mention session begin --safe: %v", err)
	}
	if !strings.Contains(err.Error(), "edge") {
		t.Errorf("error should mention device: %v", err)
	}
}

func TestCheckMaxChanges(t *testing.T) {
	if err := CheckMaxChanges(100, 0); err != nil {
		t.Fatalf("unlimited: %v", err)
	}
	if err := CheckMaxChanges(24, 25); err != nil {
		t.Fatalf("under limit: %v", err)
	}
	err := CheckMaxChanges(25, 25)
	if err == nil {
		t.Fatal("expected limit error")
	}
	var maxErr *ErrMaxChanges
	if !errors.As(err, &maxErr) {
		t.Fatalf("got %T", err)
	}
	if maxErr.Current != 25 || maxErr.Max != 25 {
		t.Errorf("got %+v", maxErr)
	}
}

func TestNormalizePath(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"/IP/Address/Add", "/ip/address/add"},
		{"ip/address/", "/ip/address"},
		{"//ip//firewall//", "/ip/firewall"},
		{"", "/"},
		{"\\system\\reboot", "/system/reboot"},
	}
	for _, tt := range tests {
		if got := NormalizePath(tt.in); got != tt.want {
			t.Errorf("NormalizePath(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestCheckWritePath_BuiltinDenied(t *testing.T) {
	for _, p := range []string{
		"/system/reset-configuration",
		"/system/reset-configuration/run",
		"/system/routerboard",
		"/system/routerboard/settings/set",
		"/System/Routerboard/Upgrade",
	} {
		err := CheckWritePath(p, nil, nil)
		if err == nil {
			t.Errorf("expected deny for %q", p)
			continue
		}
		var denied *ErrPathDenied
		if !errors.As(err, &denied) {
			t.Errorf("%q: got %T", p, err)
		}
	}

	if err := CheckWritePath("/ip/address/add", nil, nil); err != nil {
		t.Fatalf("normal path should allow: %v", err)
	}
}

func TestCheckWritePath_UserDenyAndAllow(t *testing.T) {
	denied := []string{"/ip/firewall"}
	if err := CheckWritePath("/ip/firewall/filter/add", nil, denied); err == nil {
		t.Fatal("expected user deny")
	}
	if err := CheckWritePath("/ip/address/add", nil, denied); err != nil {
		t.Fatalf("unrelated path: %v", err)
	}

	allowed := []string{"/ip/address", "/interface"}
	if err := CheckWritePath("/ip/address/set", allowed, nil); err != nil {
		t.Fatalf("allowed path: %v", err)
	}
	err := CheckWritePath("/ip/route/add", allowed, nil)
	if err == nil {
		t.Fatal("expected not in allowlist")
	}
	if !strings.Contains(err.Error(), "allowed_write_paths") {
		t.Errorf("unexpected: %v", err)
	}

	// Deny wins even if allowed.
	if err := CheckWritePath("/ip/address/add", allowed, []string{"/ip/address"}); err == nil {
		t.Fatal("deny should override allow")
	}
}

func TestCheckExec_BuiltinDeny(t *testing.T) {
	for _, cmd := range []string{
		"/system/reboot",
		"/system/reboot/run",
		"/System/Reboot",
		"/system/reset-configuration",
		"/system/reset-configuration/run",
		"/system/routerboard",
		"/system/routerboard/settings/set",
		"/disk/format-drive",
		"/disk/format-drive/run",
		"/file/format-something",
		"/quit",
		"/password",
		"/password/set",
		"/system/ssh/private-key",
		"/system/ssh/private-key/print",
		"/export/show-sensitive",
	} {
		err := CheckExec(cmd, nil, nil)
		if err == nil {
			t.Errorf("expected builtin deny for %q", cmd)
			continue
		}
		var denied *ErrExecDenied
		if !errors.As(err, &denied) {
			t.Errorf("%q: got %T (%v)", cmd, err, err)
		}
	}

	if err := CheckExec("/interface/print", nil, nil); err != nil {
		t.Fatalf("normal exec should allow: %v", err)
	}
	if err := CheckExec("/ip/address/print", nil, nil); err != nil {
		t.Fatalf("normal exec should allow: %v", err)
	}
}

func TestCheckExec_AllowDenyMatrix(t *testing.T) {
	tests := []struct {
		name    string
		command string
		allow   []string
		deny    []string
		wantErr bool
		reason  string // substring when wantErr
	}{
		{
			name:    "no allow no deny allows read",
			command: "/interface/print",
		},
		{
			name:    "user deny blocks",
			command: "/system/script/run",
			deny:    []string{"/system/script*"},
			wantErr: true,
			reason:  "exec_deny",
		},
		{
			name:    "user deny glob one level",
			command: "/tool/fetch",
			deny:    []string{"/tool/*"},
			wantErr: true,
			reason:  "exec_deny",
		},
		{
			name:    "allowlist permits match",
			command: "/interface/print",
			allow:   []string{"/interface/*", "/ip/address/*"},
		},
		{
			name:    "allowlist prefix without meta",
			command: "/ip/address/print",
			allow:   []string{"/ip/address"},
		},
		{
			name:    "allowlist rejects non-match",
			command: "/system/resource/print",
			allow:   []string{"/interface/*", "/ip/address/*"},
			wantErr: true,
			reason:  "not in exec_allow",
		},
		{
			name:    "builtin deny wins over allow",
			command: "/system/reboot",
			allow:   []string{"/system/*", "/system/reboot"},
			wantErr: true,
			reason:  "builtin deny",
		},
		{
			name:    "user deny wins over allow",
			command: "/interface/set",
			allow:   []string{"/interface/*"},
			deny:    []string{"/interface/set"},
			wantErr: true,
			reason:  "exec_deny",
		},
		{
			name:    "empty allow entries ignored still requires real match",
			command: "/ip/route/print",
			allow:   []string{"", "  ", "/interface/*"},
			wantErr: true,
			reason:  "not in exec_allow",
		},
		{
			name:    "case and slash normalize",
			command: "IP/Address/Print",
			allow:   []string{"/ip/address/*"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := CheckExec(tt.command, tt.allow, tt.deny)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				var denied *ErrExecDenied
				if !errors.As(err, &denied) {
					t.Fatalf("got %T: %v", err, err)
				}
				if tt.reason != "" && !strings.Contains(err.Error(), tt.reason) {
					t.Errorf("error %q missing %q", err.Error(), tt.reason)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected: %v", err)
			}
		})
	}
}
