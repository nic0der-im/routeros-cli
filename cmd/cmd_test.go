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
			"central hub BA": {
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
	_ = creds.Set("central hub BA", "secret")

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

func TestRenderAuditHuman(t *testing.T) {
	var buf strings.Builder
	order := []string{"system_identity", "system_resource", "cpu_profile", "interfaces", "ip_addresses", "firewall_filter", "services", "ppp_active"}
	sections := map[string]interface{}{
		"system_identity": []map[string]string{{"name": "lab"}},
		"system_resource": []map[string]string{{
			"board-name": "RB2011", "architecture-name": "mipsbe", "version": "7.18", "uptime": "1d",
			"cpu-load": "5", "cpu-count": "1", "cpu-frequency": "600", "cpu": "MIPS",
			"free-memory": "28651520", "total-memory": "67108864",
			"free-hdd-space": "115187712", "total-hdd-space": "134217728",
			"bad-blocks": "0.7", "write-sect-total": "1000", "write-sect-since-reboot": "10",
		}},
		"cpu_profile": []map[string]string{
			{".section": "2", "name": "total", "usage": "5"},
			{".section": "2", "name": "networking", "usage": "2.5"},
			{".section": "2", "name": "management", "usage": "1.0"},
			{".section": "2", "name": "ssh", "usage": "0.5"},
			{".section": "2", "name": "profiling", "usage": "0"},
		},
		"interfaces": []map[string]string{
			{"name": "ether1", "type": "ether", "running": "true", "comment": "WAN", "rx-byte": "2147483648", "tx-byte": "536870912"},
			{"name": "ether9", "type": "ether", "running": "false"},
			{"name": "<pppoe-alice>", "type": "pppoe-in", "running": "true"},
			{"name": "<pppoe-bob>", "type": "pppoe-in", "running": "true"},
		},
		"ip_addresses": []map[string]string{
			{"address": "192.168.88.1/24", "interface": "bridge", "disabled": "false"},
			{"address": "10.1.1.2/32", "interface": "<pppoe-alice>", "disabled": "false", "dynamic": "true"},
		},
		"firewall_filter": []map[string]string{
			{".id": "*E", "chain": "forward", "action": "passthrough", "dynamic": "true"},
			{".id": "*1", "chain": "forward", "action": "accept", "comment": "ok", "dynamic": "false"},
		},
		"services": []map[string]string{
			{"name": "ssh", "port": "22", "address": "192.168.88.0/24", "disabled": "false"},
			{"name": "ftp", "port": "21", "disabled": "true"},
		},
		"ppp_active": []map[string]string{
			{"name": "alice", "address": "10.1.1.2"},
			{"name": "bob", "address": "10.1.1.3"},
		},
	}
	if err := renderAuditHuman(&buf, "lab", "full", order, sections, false); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{
		`AUDIT  lab`,
		"┌─ SYSTEM",
		"memory 27 MB free / 64 MB total",
		"storage 110 MB free / 128 MB total · bad-blocks 0.7%",
		"└───────────────────────────────────────────────────────",
		"┌─ TOP CPU",
		"networking",
		"┌─ INTERFACES",
		"NAME",
		"ether1",
		"2.00 GB",
		"512 MB",
		"WAN",
		"ppp/pppoe omitted",
		"┌─ ADDRESSES",
		"192.168.88.1/24",
		"┌─ FIREWALL FILTER",
		"forward",
		"accept",
		"┌─ SERVICES",
		"ssh",
		"192.168.88.0/24",
		"┌─ PPP ACTIVE",
		"2 sessions",
		"--show-ppp",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in:\n%s", want, out)
		}
	}
	for _, no := range []string{"ether9", "passthrough", "ftp", "<pppoe-alice>"} {
		if strings.Contains(out, no) {
			t.Fatalf("should omit %q in compact audit:\n%s", no, out)
		}
	}

	var buf2 strings.Builder
	if err := renderAuditHuman(&buf2, "lab", "full", order, sections, true); err != nil {
		t.Fatal(err)
	}
	out2 := buf2.String()
	if !strings.Contains(out2, "<pppoe-alice>") || !strings.Contains(out2, "alice") {
		t.Fatalf("expected pppoe details with --show-ppp:\n%s", out2)
	}
}

func TestFormatBytes(t *testing.T) {
	if got := formatBytes("67108864"); got != "64 MB" {
		t.Fatalf("got %q", got)
	}
}

func TestIsPPPLike(t *testing.T) {
	if !isPPPLike(map[string]string{"type": "pppoe-in", "name": "x"}) {
		t.Fatal("pppoe-in")
	}
	if !isPPPLike(map[string]string{"name": "<pppoe-user>"}) {
		t.Fatal("dynamic name")
	}
	if isPPPLike(map[string]string{"type": "ether", "name": "ether1"}) {
		t.Fatal("ether should not match")
	}
}

func TestFilterPresentColumns(t *testing.T) {
	rows := []map[string]string{{"name": "ssh", "port": "22", "disabled": "false"}}
	got := summarizeServices(rows)
	if len(got) < 2 || !strings.Contains(got[0], "NAME") || !strings.Contains(got[1], "ssh") {
		t.Fatalf("%v", got)
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
