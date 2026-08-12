package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nic0der-im/routeros-cli/internal/config"
	"github.com/nic0der-im/routeros-cli/internal/guardrails"
	"github.com/nic0der-im/routeros-cli/internal/session"
)

// removeFixture wires an isolated config, doctor state dir, session dir, and
// backups dir, then seeds per-device state for "edge" and "keep".
func removeFixture(t *testing.T) (cfgPath string, sessionDir string, backupsDir string) {
	t.Helper()

	home := t.TempDir()
	t.Setenv("HOME", home)

	cfgPath = filepath.Join(home, "config.toml")
	cfg := &config.Config{
		DefaultDevice: "edge",
		DefaultOutput: "table",
		Devices: map[string]config.DeviceConfig{
			"edge": {ID: "edge01", Address: "10.0.0.1:8728", Username: "admin"},
			"keep": {ID: "keep01", Address: "10.0.0.2:8728", Username: "admin"},
		},
	}
	if err := cfg.Save(cfgPath); err != nil {
		t.Fatal(err)
	}
	oldCfg := flagConfig
	t.Cleanup(func() { flagConfig = oldCfg })
	flagConfig = cfgPath

	guardrails.SetDoctorStateDirForTest(filepath.Join(home, "state"))
	t.Cleanup(func() { guardrails.SetDoctorStateDirForTest("") })
	if err := guardrails.RecordDoctorAt("edge", time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if err := guardrails.RecordDoctorAt("keep", time.Now().UTC()); err != nil {
		t.Fatal(err)
	}

	sessionDir = session.DefaultDir()
	store, err := session.NewStore(sessionDir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Begin("edge", true); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Begin("keep", true); err != nil {
		t.Fatal(err)
	}

	backupsDir = filepath.Join(home, "backups")
	oldBackups := backupsBaseDirForTest
	t.Cleanup(func() { backupsBaseDirForTest = oldBackups })
	backupsBaseDirForTest = backupsDir
	if err := os.MkdirAll(filepath.Join(backupsDir, "edge", "20260101-000000"), 0o700); err != nil {
		t.Fatal(err)
	}

	return cfgPath, sessionDir, backupsDir
}

func runDeviceRemove(t *testing.T, args ...string) (string, error) {
	t.Helper()
	var out bytes.Buffer
	c := newDeviceRemoveCmd()
	c.SetArgs(args)
	c.SetOut(&out)
	c.SetErr(&out)
	err := c.Execute()
	return out.String(), err
}

func TestDeviceRemovePurgesPerDeviceState(t *testing.T) {
	cfgPath, sessionDir, backupsDir := removeFixture(t)

	out, err := runDeviceRemove(t, "edge", "--force", "--confirm", "edge", "--purge-backups")
	if err != nil {
		t.Fatalf("remove: %v (%s)", err, out)
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := cfg.Devices["edge"]; ok {
		t.Error("device still in inventory")
	}
	if cfg.DefaultDevice != "" {
		t.Errorf("default_device=%q, want cleared", cfg.DefaultDevice)
	}

	if _, err := os.Stat(guardrails.DoctorStatePath("edge")); !os.IsNotExist(err) {
		t.Errorf("doctor state kept: %v", err)
	}
	if _, err := os.Stat(filepath.Join(sessionDir, "active-edge.lock")); !os.IsNotExist(err) {
		t.Errorf("active session lock kept: %v", err)
	}
	if _, err := os.Stat(filepath.Join(backupsDir, "edge")); !os.IsNotExist(err) {
		t.Errorf("backups kept despite --purge-backups: %v", err)
	}

	// The other device is untouched.
	if _, ok := cfg.Devices["keep"]; !ok {
		t.Error("unrelated device removed from inventory")
	}
	if _, err := os.Stat(guardrails.DoctorStatePath("keep")); err != nil {
		t.Errorf("unrelated doctor state removed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(sessionDir, "active-keep.lock")); err != nil {
		t.Errorf("unrelated session lock removed: %v", err)
	}
}

func TestDeviceRemoveKeepsBackupsByDefault(t *testing.T) {
	_, _, backupsDir := removeFixture(t)

	out, err := runDeviceRemove(t, "edge", "--force", "--confirm", "edge")
	if err != nil {
		t.Fatalf("remove: %v (%s)", err, out)
	}
	if _, err := os.Stat(filepath.Join(backupsDir, "edge")); err != nil {
		t.Errorf("backups deleted without --purge-backups: %v", err)
	}
	if !strings.Contains(out, "--purge-backups") {
		t.Errorf("output does not point at retained backups: %q", out)
	}
}

func TestDeviceRemoveRefusesActiveSessionWithoutForce(t *testing.T) {
	cfgPath, _, _ := removeFixture(t)

	// No --force: the seeded active session for "edge" must block removal.
	out, err := runDeviceRemove(t, "edge", "--confirm", "edge")
	if err == nil {
		t.Fatalf("expected refusal, got output %q", out)
	}
	if !strings.Contains(err.Error(), "active safe session") {
		t.Fatalf("unexpected error: %v", err)
	}

	cfg, loadErr := config.Load(cfgPath)
	if loadErr != nil {
		t.Fatal(loadErr)
	}
	if _, ok := cfg.Devices["edge"]; !ok {
		t.Error("device removed despite refusal")
	}
}

func TestDeviceRemoveAliases(t *testing.T) {
	c := newDeviceRemoveCmd()
	for _, alias := range []string{"delete", "rm"} {
		if !c.HasAlias(alias) {
			t.Errorf("missing alias %q", alias)
		}
	}
}
