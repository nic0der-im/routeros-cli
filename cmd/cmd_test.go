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
	"github.com/spf13/cobra"
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
	for _, p := range []string{"full", "network", "security", "hygiene", ""} {
		if err := validateAuditProfile(p); err != nil {
			t.Fatalf("profile %q: %v", p, err)
		}
	}
	if err := validateAuditProfile("nope"); err == nil {
		t.Fatal("expected error for unknown profile")
	} else if !strings.Contains(err.Error(), "hygiene") {
		t.Fatalf("error should list hygiene: %v", err)
	}
	got := auditProfiles()
	if len(got) != 4 || got[3] != "hygiene" {
		t.Fatalf("auditProfiles=%v", got)
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

func TestSummarizeCloud(t *testing.T) {
	got := summarizeCloud([]map[string]string{{
		"ddns-enabled": "auto", "update-time": "false", "status": "updated",
		"public-address": "1.2.3.4", "dns-name": "x.sn.mynetname.net",
	}})
	joined := strings.Join(got, "\n")
	for _, want := range []string{"DDNS", "auto", "false", "updated", "public-address", "1.2.3.4"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("missing %q in:\n%s", want, joined)
		}
	}
}

func TestSummarizeBackupFiles(t *testing.T) {
	got := summarizeBackupFiles([]map[string]string{
		{"name": "script.rsc", "size": "100"},
		{"name": "ros-backup-20260729.backup", "size": "1048576"},
		{"name": "old.BACKUP", "size": "2048"},
	})
	joined := strings.Join(got, "\n")
	if !strings.Contains(joined, "ros-backup-20260729.backup") || !strings.Contains(joined, "old.BACKUP") {
		t.Fatalf("expected .backup files:\n%s", joined)
	}
	if strings.Contains(joined, "script.rsc") {
		t.Fatalf("should skip non-backup:\n%s", joined)
	}
	if empty := summarizeBackupFiles(nil); len(empty) != 1 || !strings.Contains(empty[0], "no .backup") {
		t.Fatalf("empty: %v", empty)
	}
}

func TestSummarizeIfaceErrors(t *testing.T) {
	got := summarizeIfaceErrors([]map[string]string{
		{"name": "ether1", "running": "true", "rx-drop": "0", "tx-drop": "0", "tx-queue-drop": "12"},
		{"name": "ether2", "running": "true", "rx-drop": "0", "tx-drop": "0", "tx-queue-drop": "0"},
		{"name": "ether3", "running": "false", "tx-queue-drop": "99"},
		{"name": "ether4", "running": "true", "rx-drop": "3", "tx-drop": "1", "tx-queue-drop": "0"},
	})
	joined := strings.Join(got, "\n")
	if !strings.Contains(joined, "ether1") || !strings.Contains(joined, "12") {
		t.Fatalf("expected ether1 tx-queue-drop:\n%s", joined)
	}
	if !strings.Contains(joined, "ether4") {
		t.Fatalf("expected ether4 drops:\n%s", joined)
	}
	if strings.Contains(joined, "ether2") || strings.Contains(joined, "ether3") {
		t.Fatalf("should omit clean/not-running:\n%s", joined)
	}
}

func TestSummarizeIPSettings(t *testing.T) {
	got := summarizeIPSettings([]map[string]string{{
		"allow-fast-path":        "true",
		"ipv4-fast-path-active":  "true",
		"ipv4-fasttrack-active":  "true",
		"icmp-rate-limit":        "10",
	}})
	joined := strings.Join(got, "\n")
	for _, want := range []string{"allow-fast-path", "ipv4-fasttrack-active", "true"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("missing %q in:\n%s", want, joined)
		}
	}
	if strings.Contains(joined, "icmp-rate-limit") {
		t.Fatalf("should only show fast-path flags:\n%s", joined)
	}
}

func TestSummarizeLeaseHygiene(t *testing.T) {
	got := summarizeLeaseHygiene([]map[string]string{
		{"status": "bound", "mac-address": "AA:BB:CC:DD:EE:01"},
		{"status": "waiting", "mac-address": "AA:BB:CC:DD:EE:01"},
		{"status": "waiting", "mac-address": "11:22:33:44:55:66"},
		{"status": "bound", "mac-address": "FF:FF:FF:FF:FF:FF"},
	})
	joined := strings.Join(got, "\n")
	if !strings.Contains(joined, "2 waiting") {
		t.Fatalf("waiting count:\n%s", joined)
	}
	if !strings.Contains(joined, "warn:") || !strings.Contains(joined, "aa:bb:cc:dd:ee:01") {
		t.Fatalf("dup MAC warn:\n%s", joined)
	}
	if strings.Contains(joined, "ff:ff:ff:ff:ff:ff") {
		t.Fatalf("unique MAC should not warn:\n%s", joined)
	}
}

func TestSummarizeServicesHygiene(t *testing.T) {
	got := summarizeServicesHygiene([]map[string]string{
		{"name": "ssh", "port": "22", "disabled": "false", "address": "192.168.88.0/24"},
		{"name": "api-ssl", "port": "8729", "disabled": "true"},
		{"name": "ftp", "port": "21", "disabled": "true"},
		{"name": "bandwidth-test", "port": "2000", "disabled": "true"},
	})
	joined := strings.Join(got, "\n")
	if !strings.Contains(joined, "ssh") {
		t.Fatalf("enabled ssh:\n%s", joined)
	}
	if !strings.Contains(joined, "disabled mgmt:") || !strings.Contains(joined, "api-ssl") || !strings.Contains(joined, "ftp") {
		t.Fatalf("disabled mgmt leftovers:\n%s", joined)
	}
	if strings.Contains(joined, "bandwidth-test") {
		t.Fatalf("non-mgmt disabled should stay off the leftovers line:\n%s", joined)
	}
}

func TestRenderAuditHygiene(t *testing.T) {
	var buf strings.Builder
	order := []string{"system_identity", "system_resource", "ip_cloud", "files", "interfaces", "ip_settings", "services", "dhcp_leases"}
	sections := map[string]interface{}{
		"system_identity": []map[string]string{{"name": "home"}},
		"system_resource": []map[string]string{{
			"board-name": "RB2011", "architecture-name": "mipsbe", "version": "7.18", "uptime": "1d",
			"cpu-load": "5", "cpu-count": "1", "cpu-frequency": "600", "cpu": "MIPS",
			"free-memory": "28651520", "total-memory": "67108864",
			"free-hdd-space": "115187712", "total-hdd-space": "134217728",
			"bad-blocks": "0.7",
		}},
		"ip_cloud": []map[string]string{{"ddns-enabled": "auto", "update-time": "false", "status": "updated"}},
		"files": []map[string]string{
			{"name": "keep.backup", "size": "1024"},
			{"name": "note.txt", "size": "10"},
		},
		"interfaces": []map[string]string{
			{"name": "ether1", "running": "true", "tx-queue-drop": "5", "rx-drop": "0", "tx-drop": "0"},
			{"name": "ether2", "running": "true", "tx-queue-drop": "0", "rx-drop": "0", "tx-drop": "0"},
		},
		"ip_settings": []map[string]string{{"allow-fast-path": "true", "ipv4-fasttrack-active": "true"}},
		"services": []map[string]string{
			{"name": "ssh", "port": "22", "disabled": "false"},
			{"name": "api-ssl", "port": "8729", "disabled": "true"},
		},
		"dhcp_leases": []map[string]string{
			{"status": "waiting", "mac-address": "AA:BB:CC:00:00:01"},
			{"status": "bound", "mac-address": "AA:BB:CC:00:00:01"},
		},
	}
	if err := renderAuditHuman(&buf, "home", "hygiene", order, sections, false); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{
		"profile=hygiene",
		"┌─ IP CLOUD",
		"auto",
		"┌─ BACKUP FILES",
		"keep.backup",
		"┌─ IFACE ERRORS",
		"ether1",
		"┌─ IP SETTINGS",
		"ipv4-fasttrack-active",
		"┌─ SERVICES",
		"disabled mgmt:",
		"api-ssl",
		"┌─ DHCP HYGIENE",
		"1 waiting",
		"warn:",
		"┌─ FINDINGS",
		"bad-blocks",
		"waiting DHCP",
		"same-MAC",
		"TX-QUEUE-DROP",
		"no dangerous cleartext services",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in:\n%s", want, out)
		}
	}
	for _, no := range []string{"note.txt", "TOP CPU", "ether2", "PPP ACTIVE"} {
		if strings.Contains(out, no) {
			t.Fatalf("should omit %q:\n%s", no, out)
		}
	}
}

func TestDeriveFindings(t *testing.T) {
	t.Parallel()
	sections := map[string]interface{}{
		"system_resource": []map[string]string{{"bad-blocks": "1.2"}},
		"files": []map[string]string{
			{"name": "a.backup", "size": "1"},
			{"name": "b.backup", "size": "2"},
			{"name": "note.txt", "size": "3"},
		},
		"interfaces": []map[string]string{
			{"name": "ether1", "running": "true", "tx-queue-drop": "9"},
			{"name": "ether2", "running": "true", "tx-queue-drop": "0"},
		},
		"ip_settings": []map[string]string{{
			"allow-fast-path":       "true",
			"ipv4-fasttrack-active": "false",
		}},
		"dhcp_leases": []map[string]string{
			{"status": "waiting", "mac-address": "aa:bb:cc:00:00:01"},
			{"status": "bound", "mac-address": "aa:bb:cc:00:00:01"},
		},
		"services": []map[string]string{
			{"name": "telnet", "disabled": "false"},
			{"name": "ftp", "disabled": "true"},
			{"name": "www", "disabled": "true"},
			{"name": "ssh", "disabled": "false"},
		},
		"ip_cloud": []map[string]string{{
			"ddns-enabled": "yes",
			"status":       "updated",
			"warning":      "behind NAT",
		}},
	}
	got := deriveFindings(sections)
	joined := strings.Join(got, "\n")
	for _, want := range []string{
		"warn: bad-blocks 1.2%",
		"warn: 2 *.backup",
		"file remove",
		"warn: iface drops",
		"ether1",
		"info: allow-fast-path true but FastTrack inactive",
		"warn: 1 waiting DHCP lease",
		"lease cleanup-waiting",
		"warn: same-MAC multi-lease",
		"warn: dangerous services enabled: telnet",
		"set /ip/service",
		"warn: cloud DDNS behind NAT",
		"set /ip/cloud",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("missing %q in findings:\n%s", want, joined)
		}
	}
	for _, no := range []string{"ok: bad-blocks", "ok: no *.backup", "ok: FastTrack", "ok: DHCP leases clean", "ok: no dangerous cleartext"} {
		if strings.Contains(joined, no) {
			t.Fatalf("unexpected ok line %q in:\n%s", no, joined)
		}
	}
}

func TestDeriveFindingsClean(t *testing.T) {
	t.Parallel()
	sections := map[string]interface{}{
		"system_resource": []map[string]string{{"bad-blocks": "0"}},
		"files":           []map[string]string{{"name": "readme.txt"}},
		"interfaces":      []map[string]string{{"name": "ether1", "running": "true", "tx-queue-drop": "0"}},
		"ip_settings":     []map[string]string{{"allow-fast-path": "true", "ipv4-fasttrack-active": "true"}},
		"dhcp_leases":     []map[string]string{{"status": "bound", "mac-address": "aa:bb:cc:00:00:02"}},
		"services": []map[string]string{
			{"name": "telnet", "disabled": "true"},
			{"name": "ftp", "disabled": "true"},
			{"name": "www", "disabled": "true"},
			{"name": "ssh", "disabled": "false"},
		},
		"ip_cloud": []map[string]string{{"ddns-enabled": "auto", "status": "updated"}},
	}
	got := deriveFindings(sections)
	joined := strings.Join(got, "\n")
	for _, want := range []string{
		"ok: bad-blocks 0",
		"ok: no *.backup clutter",
		"ok: no notable iface drops",
		"ok: FastTrack active",
		"ok: DHCP leases clean",
		"ok: no dangerous cleartext services",
		"ok: cloud DDNS not forced on",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("missing %q in:\n%s", want, joined)
		}
	}
	if strings.Contains(joined, "warn:") || strings.Contains(joined, "info:") {
		t.Fatalf("expected clean findings only:\n%s", joined)
	}
}

func TestDoctorCommand(t *testing.T) {
	cmd := newDoctorCmd()
	if cmd.Use != "doctor" {
		t.Fatalf("Use=%q", cmd.Use)
	}
	if !strings.Contains(cmd.Short, "FINDINGS") || !strings.Contains(cmd.Short, "hygiene") {
		t.Fatalf("Short=%q", cmd.Short)
	}
}

func TestRootHasDoctor(t *testing.T) {
	root := &cobra.Command{Use: "ros"}
	root.AddCommand(newDoctorCmd(), newAuditCmd())
	names := map[string]bool{}
	for _, c := range root.Commands() {
		names[c.Name()] = true
	}
	if !names["doctor"] || !names["audit"] {
		t.Fatalf("names=%v", names)
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

func TestDefaultBinaryBackupNameUTC(t *testing.T) {
	got := defaultBinaryBackupName(time.Date(2026, 7, 29, 17, 8, 15, 0, time.UTC))
	if got != "ros-backup-20260729-170815" {
		t.Fatalf("got %q", got)
	}
	// Non-UTC input still formats in UTC.
	loc := time.FixedZone("ART", -3*3600)
	got = defaultBinaryBackupName(time.Date(2026, 7, 29, 14, 8, 15, 0, loc))
	if got != "ros-backup-20260729-170815" {
		t.Fatalf("UTC convert got %q", got)
	}
}

func TestFileRemoveArg(t *testing.T) {
	if got := fileRemoveArg("stale.backup"); got != "=numbers=stale.backup" {
		t.Fatalf("name: %q", got)
	}
	if got := fileRemoveArg("*A"); got != "=.id=*A" {
		t.Fatalf("id: %q", got)
	}
}

func TestFileCommandTree(t *testing.T) {
	cmd := newFileCmd()
	names := map[string]bool{}
	for _, c := range cmd.Commands() {
		names[c.Name()] = true
	}
	for _, want := range []string{"list", "get", "remove"} {
		if !names[want] {
			t.Errorf("missing file %s", want)
		}
	}
}

func TestSingletonSetJournalingMocked(t *testing.T) {
	a, mock := testApp(t)
	mock.RunFunc = func(_ context.Context, command string, args ...string) (*client.Result, error) {
		if command == "/ip/cloud/print" {
			return &client.Result{Sentences: []map[string]string{{
				"ddns-enabled": "yes",
				"update-time":  "true",
			}}}, nil
		}
		return &client.Result{}, nil
	}
	if _, err := a.Sessions.Begin("lab", true); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	pre, err := fetchPreState(ctx, mock, "/ip/cloud", "")
	if err != nil {
		t.Fatal(err)
	}
	apiArgs := []string{"=ddns-enabled=auto", "=update-time=false"}
	inv := session.BuildSetInverse("/ip/cloud/set", "", pre, apiArgs)
	if len(inv) < 3 {
		t.Fatalf("inverse: %v", inv)
	}
	for _, a := range inv {
		if strings.HasPrefix(a, "=.id=") {
			t.Fatalf("singleton inverse has .id: %v", inv)
		}
	}
	if err := a.recordSafeChange("lab", session.Change{
		Command:  "/ip/cloud/set",
		Args:     apiArgs,
		Inverse:  inv,
		PreState: pre,
		Note:     "set singleton",
	}); err != nil {
		t.Fatal(err)
	}
	active, err := a.Sessions.Active("lab")
	if err != nil || active == nil {
		t.Fatalf("active: %v %v", active, err)
	}
	if len(active.Changes) != 1 {
		t.Fatalf("changes=%d", len(active.Changes))
	}
	if active.Changes[0].Inverse[0] != "/ip/cloud/set" {
		t.Fatalf("inverse cmd: %v", active.Changes[0].Inverse)
	}
}
