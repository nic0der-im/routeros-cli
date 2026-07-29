package cmd

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nic0der-im/routeros-cli/internal/config"
	"github.com/nic0der-im/routeros-cli/internal/guardrails"
	"github.com/spf13/cobra"
)

func TestRequireConfirmDevice(t *testing.T) {
	if err := requireConfirmDevice("", "edge"); err == nil {
		t.Fatal("empty confirm should fail")
	}
	if err := requireConfirmDevice("   ", "edge"); err == nil {
		t.Fatal("whitespace-only confirm should fail")
	}
	if !strings.Contains(requireConfirmDevice("", "edge").Error(), "require --confirm") {
		t.Fatal("empty message should mention --confirm")
	}
	if err := requireConfirmDevice("other", "edge"); err == nil {
		t.Fatal("mismatch should fail")
	}
	if !strings.Contains(requireConfirmDevice("other", "edge").Error(), "does not match") {
		t.Fatal("mismatch message")
	}
	if err := requireConfirmDevice("edge", "edge"); err != nil {
		t.Fatal(err)
	}
}

func TestRegisterConfirmFlag(t *testing.T) {
	cmd := &cobra.Command{Use: "x"}
	var confirm string
	registerConfirmFlag(cmd, &confirm)
	f := cmd.Flags().Lookup("confirm")
	if f == nil {
		t.Fatal("missing --confirm")
	}
	if f.Usage != confirmFlagUsage {
		t.Fatalf("usage: %q", f.Usage)
	}
}

func assertDestructiveConfirmFlags(t *testing.T, cmd *cobra.Command, wantForce bool) {
	t.Helper()
	if cmd.Flags().Lookup("confirm") == nil {
		t.Fatal("missing --confirm flag")
	}
	if wantForce && cmd.Flags().Lookup("force") == nil {
		t.Fatal("missing --force flag")
	}
	if !strings.Contains(cmd.Long, "--confirm") {
		t.Fatal("Long help should mention --confirm")
	}
	if !strings.Contains(cmd.Long, "does not substitute") && !strings.Contains(cmd.Long, "does not replace") {
		t.Fatal("Long help should clarify --force does not substitute for --confirm")
	}
}

func TestSystemRebootRequiresConfirmFlag(t *testing.T) {
	assertDestructiveConfirmFlags(t, newSystemRebootCmd(), true)
}

func TestFileRemoveRequiresConfirmFlag(t *testing.T) {
	assertDestructiveConfirmFlags(t, newFileRemoveCmd(), false)
}

func TestDeviceRemoveRequiresConfirmFlag(t *testing.T) {
	assertDestructiveConfirmFlags(t, newDeviceRemoveCmd(), true)
}

func TestLeaseCleanupWaitingRequiresConfirmFlag(t *testing.T) {
	cmd := newLeaseCleanupWaitingCmd()
	assertDestructiveConfirmFlags(t, cmd, false)
	if cmd.Flags().Lookup("dry-run") == nil {
		t.Fatal("missing --dry-run flag")
	}
}

func TestDeviceRemoveConfirmWiring(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.toml")
	cfg := &config.Config{
		DefaultOutput: "table",
		Devices: map[string]config.DeviceConfig{
			"edge": {Address: "10.0.0.1:8728", Username: "admin"},
		},
	}
	if err := cfg.Save(cfgPath); err != nil {
		t.Fatal(err)
	}

	oldCfg := flagConfig
	defer func() { flagConfig = oldCfg }()
	flagConfig = cfgPath

	run := func(args ...string) error {
		cmd := newDeviceRemoveCmd()
		cmd.SetArgs(args)
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
		return cmd.Execute()
	}

	if err := run("edge", "--force"); err == nil {
		t.Fatal("empty --confirm should refuse")
	} else if !strings.Contains(err.Error(), "require --confirm") {
		t.Fatalf("empty confirm: %v", err)
	}

	if err := run("edge", "--force", "--confirm", "other"); err == nil {
		t.Fatal("mismatch --confirm should refuse")
	} else if !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("mismatch: %v", err)
	}

	if err := run("edge", "--force", "--confirm", "edge"); err != nil {
		t.Fatalf("matching confirm: %v", err)
	}
	a, err := loadApp()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.Inventory.Get("edge"); err == nil {
		t.Fatal("device should have been removed")
	}
}

func TestEnsureWritable_AgentProfileRequiresSafeSession(t *testing.T) {
	a, _ := testApp(t)
	a.Profile = config.ProfileAgent
	err := a.ensureWritable("lab", "/ip/address/add")
	if err == nil {
		t.Fatal("agent profile should require safe session on lab")
	}
	var req *guardrails.ErrSafeSessionRequired
	if !errors.As(err, &req) {
		t.Fatalf("got %T: %v", err, err)
	}
	if _, err := a.Sessions.Begin("lab", true); err != nil {
		t.Fatal(err)
	}
	if err := a.ensureWritable("lab", "/ip/address/add"); err != nil {
		t.Fatalf("agent + safe session: %v", err)
	}
}

func TestEnsureWritable_AgentStrictTreatsLabAsProd(t *testing.T) {
	a, _ := testApp(t)
	a.Profile = config.ProfileAgentStrict
	t.Setenv("ROS_STRICT", "")
	t.Setenv("ROS_SKIP_DOCTOR_GATE", "1")

	err := a.ensureWritable("lab", "/ip/address/add")
	if err == nil {
		t.Fatal("agent-strict should require safe session")
	}
	var req *guardrails.ErrSafeSessionRequired
	if !errors.As(err, &req) {
		t.Fatalf("got %T: %v", err, err)
	}

	if _, err := a.Sessions.Begin("lab", true); err != nil {
		t.Fatal(err)
	}
	// With skip env, agent-strict lab (effective prod) may write inside safe session.
	if err := a.ensureWritable("lab", "/ip/address/add"); err != nil {
		t.Fatalf("agent-strict + session + skip doctor: %v", err)
	}
}

func TestEnsureWritable_ProdDoctorGate(t *testing.T) {
	a, _ := testApp(t)
	t.Setenv("ROS_SKIP_DOCTOR_GATE", "")
	// Clear seeded freshness so the gate must refuse.
	_ = os.Remove(guardrails.DoctorStatePath("lab"))

	dev := a.Config.Devices["lab"]
	dev.EnvClass = "prod"
	a.Config.Devices["lab"] = dev
	if _, err := a.Sessions.Begin("lab", true); err != nil {
		t.Fatal(err)
	}

	err := a.ensureWritable("lab", "/ip/address/add")
	if err == nil {
		t.Fatal("expected doctor gate refuse")
	}
	var stale *guardrails.ErrDoctorStale
	if !errors.As(err, &stale) {
		t.Fatalf("got %T: %v", err, err)
	}

	if err := guardrails.RecordDoctorAt("lab", time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := a.ensureWritable("lab", "/ip/address/add"); err != nil {
		t.Fatalf("fresh doctor: %v", err)
	}

	if err := guardrails.RecordDoctorAt("lab", time.Now().Add(-2*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := a.ensureWritableForce("lab", "/ip/address/add", true); err != nil {
		t.Fatalf("force bypass: %v", err)
	}
}

func TestLoadAppProfileEnv(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.toml")
	cfg := &config.Config{DefaultOutput: "table", Devices: map[string]config.DeviceConfig{}}
	if err := cfg.Save(cfgPath); err != nil {
		t.Fatal(err)
	}

	oldCfg, oldRO, oldOut, oldForce := flagConfig, flagReadOnly, flagOutput, flagForce
	defer func() {
		flagConfig, flagReadOnly, flagOutput, flagForce = oldCfg, oldRO, oldOut, oldForce
		_ = os.Unsetenv("ROS_PROFILE")
	}()
	flagConfig = cfgPath
	flagReadOnly = false
	flagOutput = ""
	flagForce = false

	t.Setenv("ROS_PROFILE", "agent-strict")
	t.Setenv("ROS_READ_ONLY", "")
	a, err := loadApp()
	if err != nil {
		t.Fatal(err)
	}
	if a.Profile != config.ProfileAgentStrict {
		t.Fatalf("profile: got %q", a.Profile)
	}
	if !a.effectiveStrict() {
		t.Fatal("agent-strict should enable effectiveStrict")
	}
}
