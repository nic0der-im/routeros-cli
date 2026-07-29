package cmd

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nic0der-im/routeros-cli/internal/client"
	"github.com/nic0der-im/routeros-cli/internal/config"
	"github.com/nic0der-im/routeros-cli/internal/credential"
	"github.com/nic0der-im/routeros-cli/internal/device"
	"github.com/nic0der-im/routeros-cli/internal/output"
	"github.com/nic0der-im/routeros-cli/internal/policy"
	"github.com/nic0der-im/routeros-cli/internal/session"
)

func testApp(t *testing.T) (*App, *client.MockClient) {
	t.Helper()
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.toml")
	cfg := &config.Config{
		DefaultOutput: "json",
		DefaultDevice: "lab",
		Devices: map[string]config.DeviceConfig{
			"lab": {
				Address:  "192.168.88.1:8728",
				Username: "admin",
				TLS:      false,
				ID:       "lab-router",
			},
			"EOC FRONTERA": {
				Address:  "10.0.0.1:8728",
				Username: "admin",
				TLS:      false,
				ID:       "eoc-frontera",
			},
		},
	}
	if err := cfg.Save(cfgPath); err != nil {
		t.Fatal(err)
	}

	sessDir := filepath.Join(dir, "sessions")
	store, err := session.NewStore(sessDir)
	if err != nil {
		t.Fatal(err)
	}

	creds := credential.NewMemoryStore()
	_ = creds.Set("lab", "secret")
	_ = creds.Set("EOC FRONTERA", "secret")

	mock := client.NewMockClient()
	app := &App{
		Config:    cfg,
		CfgPath:   cfgPath,
		Inventory: device.NewInventory(cfg, cfgPath),
		Creds:     creds,
		Sessions:  store,
		OutFormat: output.FormatJSON,
		Timeout:   5 * time.Second,
	}
	return app, mock
}

func TestEnsureWritable_ReadOnly(t *testing.T) {
	a, _ := testApp(t)
	a.ReadOnly = true
	err := a.ensureWritable("/ip/address/add")
	if err == nil {
		t.Fatal("expected error")
	}
	if _, ok := err.(*policy.ErrReadOnly); !ok {
		t.Fatalf("got %T", err)
	}
}

func TestDeviceListRender(t *testing.T) {
	a, _ := testApp(t)
	devices := a.Inventory.List()
	dl := make(deviceList, 0, len(devices))
	for name, dev := range devices {
		dl = append(dl, deviceEntry{
			Name:     name,
			Address:  dev.Address,
			Username: dev.Username,
			TLS:      "false",
			Default:  "",
			ID:       dev.ID,
		})
	}
	var buf bytes.Buffer
	meta := output.Meta{Command: "device list", Count: len(dl)}
	if err := output.Render(&buf, output.FormatJSON, dl, meta); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), `"ok": true`) {
		t.Fatalf("unexpected: %s", buf.String())
	}
}

func TestSessionBeginCommitFlow(t *testing.T) {
	a, _ := testApp(t)
	sess, err := a.Sessions.Begin("lab", true)
	if err != nil {
		t.Fatal(err)
	}
	if err := a.recordSafeChange("lab", session.Change{
		Command: "/ip/address/add",
		Args:    []string{"=address=10.0.0.1/24"},
		Inverse: []string{"/ip/address/remove", "=.id=*1"},
	}); err != nil {
		t.Fatal(err)
	}
	reloaded, err := a.Sessions.Get(sess.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(reloaded.Changes) != 1 {
		t.Fatalf("changes=%d", len(reloaded.Changes))
	}
	if err := a.Sessions.Commit(reloaded); err != nil {
		t.Fatal(err)
	}
}

func TestVersionCommand(t *testing.T) {
	SetVersionInfo("0.2.0-test", "abc", "2026-01-01")
	cmd := newVersionCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "ros 0.2.0-test") {
		t.Fatalf("got %q", buf.String())
	}
}

func TestPolicyBlocksWriteViaMock(t *testing.T) {
	mock := client.NewMockClient()
	ro := policy.WrapReadOnly(mock)
	_, err := ro.Run(context.Background(), "/system/reboot")
	if err == nil {
		t.Fatal("expected block")
	}
}

func TestRootCommandHasRosUse(t *testing.T) {
	if rootCmd.Use != "ros" {
		t.Fatalf("Use=%q", rootCmd.Use)
	}
	found := false
	for _, a := range rootCmd.Aliases {
		if a == "routeros-cli" {
			found = true
		}
	}
	if !found {
		t.Fatal("missing routeros-cli alias")
	}
}

func TestAuditProfileValidation(t *testing.T) {
	cmd := newAuditCmd()
	if cmd.Use != "audit" {
		t.Fatalf("Use=%q", cmd.Use)
	}
}

func TestGetCommandTree(t *testing.T) {
	cmd := newGetCmd()
	subs := map[string]bool{}
	for _, c := range cmd.Commands() {
		subs[c.Name()] = true
	}
	for _, want := range []string{"system", "interface", "ip", "firewall", "dhcp"} {
		if !subs[want] {
			t.Errorf("missing get %s", want)
		}
	}
}

func TestCreateCommandTree(t *testing.T) {
	cmd := newCreateCmd()
	if len(cmd.Commands()) == 0 {
		t.Fatal("expected create subcommands")
	}
}

func TestSessionCommandTree(t *testing.T) {
	cmd := newSessionCmd()
	names := map[string]bool{}
	for _, c := range cmd.Commands() {
		names[c.Name()] = true
	}
	for _, want := range []string{"begin", "commit", "rollback", "status"} {
		if !names[want] {
			t.Errorf("missing session %s", want)
		}
	}
}

func TestLoadAppReadOnlyEnv(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.toml")
	cfg := &config.Config{DefaultOutput: "table", Devices: map[string]config.DeviceConfig{}}
	_ = cfg.Save(cfgPath)

	oldCfg := flagConfig
	oldRO := flagReadOnly
	oldOut := flagOutput
	defer func() {
		flagConfig = oldCfg
		flagReadOnly = oldRO
		flagOutput = oldOut
		_ = os.Unsetenv("ROS_READ_ONLY")
	}()

	flagConfig = cfgPath
	flagReadOnly = false
	flagOutput = ""
	t.Setenv("ROS_READ_ONLY", "1")

	a, err := loadApp()
	if err != nil {
		t.Fatal(err)
	}
	if !a.ReadOnly {
		t.Fatal("expected read-only from env")
	}
}
