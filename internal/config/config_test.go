package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// tempConfigPath returns a path inside a temporary directory suitable for a
// config file. The directory is cleaned up when the test finishes.
func tempConfigPath(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	return filepath.Join(dir, "routeros-cli", "config.toml")
}

func TestDefaultPath(t *testing.T) {
	p := DefaultPath()
	if p == "" {
		t.Fatal("DefaultPath returned empty string")
	}
	if filepath.Base(p) != "config.toml" {
		t.Errorf("expected config.toml, got %s", filepath.Base(p))
	}
	if !filepath.IsAbs(p) {
		t.Errorf("expected absolute path, got %s", p)
	}
}

func TestLoadCreatesDefault(t *testing.T) {
	path := tempConfigPath(t)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.DefaultOutput != "table" {
		t.Errorf("expected default_output=table, got %q", cfg.DefaultOutput)
	}
	if cfg.Devices == nil {
		t.Fatal("expected non-nil Devices map")
	}
	if len(cfg.Devices) != 0 {
		t.Errorf("expected empty Devices map, got %d entries", len(cfg.Devices))
	}
	if cfg.TLS.InsecureSkipVerify {
		t.Error("expected insecure_skip_verify=false by default")
	}

	// The file should have been written to disk.
	if _, err := os.Stat(path); err != nil {
		t.Errorf("expected config file to exist at %s: %v", path, err)
	}
}

func TestLoadReadsTOML(t *testing.T) {
	path := tempConfigPath(t)
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	content := []byte(`
default_device = "home"
default_output = "json"

[devices.home]
address = "192.168.88.1:8728"
username = "admin"
tls = false

[devices.office]
address = "10.0.0.1:8729"
username = "operator"
tls = true

[tls]
insecure_skip_verify = true
ca_cert = "/etc/ssl/ca.pem"
`)
	if err := os.WriteFile(path, content, 0600); err != nil {
		t.Fatalf("write: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.DefaultDevice != "home" {
		t.Errorf("default_device: want home, got %q", cfg.DefaultDevice)
	}
	if cfg.DefaultOutput != "json" {
		t.Errorf("default_output: want json, got %q", cfg.DefaultOutput)
	}
	if len(cfg.Devices) != 2 {
		t.Fatalf("expected 2 devices, got %d", len(cfg.Devices))
	}

	home := cfg.Devices["home"]
	if home.Address != "192.168.88.1:8728" {
		t.Errorf("home address: got %q", home.Address)
	}
	if home.Username != "admin" {
		t.Errorf("home username: got %q", home.Username)
	}
	if home.TLS {
		t.Error("home tls: expected false")
	}

	office := cfg.Devices["office"]
	if !office.TLS {
		t.Error("office tls: expected true")
	}

	if !cfg.TLS.InsecureSkipVerify {
		t.Error("tls insecure_skip_verify: expected true")
	}
	if cfg.TLS.CACert != "/etc/ssl/ca.pem" {
		t.Errorf("tls ca_cert: got %q", cfg.TLS.CACert)
	}
}

func TestSaveLoadRoundtrip(t *testing.T) {
	path := tempConfigPath(t)

	original := &Config{
		DefaultDevice: "router1",
		DefaultOutput: "json",
		Devices: map[string]DeviceConfig{
			"router1": {
				Address:  "192.168.1.1:8728",
				Username: "admin",
				TLS:      false,
			},
			"router2": {
				Address:  "10.0.0.1:8729",
				Username: "operator",
				TLS:      true,
			},
		},
		TLS: TLSConfig{
			InsecureSkipVerify: true,
			CACert:             "/tmp/ca.pem",
		},
	}

	if err := original.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if loaded.DefaultDevice != original.DefaultDevice {
		t.Errorf("DefaultDevice: want %q, got %q", original.DefaultDevice, loaded.DefaultDevice)
	}
	if loaded.DefaultOutput != original.DefaultOutput {
		t.Errorf("DefaultOutput: want %q, got %q", original.DefaultOutput, loaded.DefaultOutput)
	}
	if len(loaded.Devices) != len(original.Devices) {
		t.Fatalf("Devices count: want %d, got %d", len(original.Devices), len(loaded.Devices))
	}
	for name, orig := range original.Devices {
		got, ok := loaded.Devices[name]
		if !ok {
			t.Errorf("device %q missing after roundtrip", name)
			continue
		}
		if !reflect.DeepEqual(got, orig) {
			t.Errorf("device %q: want %+v, got %+v", name, orig, got)
		}
	}
	if loaded.TLS != original.TLS {
		t.Errorf("TLS: want %+v, got %+v", original.TLS, loaded.TLS)
	}
}

func TestAddDevice(t *testing.T) {
	cfg := defaultConfig()

	dev := DeviceConfig{Address: "192.168.1.1:8728", Username: "admin"}
	if err := cfg.AddDevice("home", dev); err != nil {
		t.Fatalf("AddDevice: %v", err)
	}

	if _, ok := cfg.Devices["home"]; !ok {
		t.Fatal("device 'home' not found after AddDevice")
	}

	// Adding the same name again should fail.
	err := cfg.AddDevice("home", dev)
	if err == nil {
		t.Fatal("expected error when adding duplicate device")
	}
}

func TestRemoveDevice(t *testing.T) {
	cfg := defaultConfig()
	dev := DeviceConfig{Address: "10.0.0.1:8728", Username: "admin"}
	_ = cfg.AddDevice("office", dev)
	cfg.DefaultDevice = "office"

	if err := cfg.RemoveDevice("office"); err != nil {
		t.Fatalf("RemoveDevice: %v", err)
	}
	if _, ok := cfg.Devices["office"]; ok {
		t.Error("device 'office' still present after RemoveDevice")
	}
	if cfg.DefaultDevice != "" {
		t.Errorf("DefaultDevice should be cleared, got %q", cfg.DefaultDevice)
	}

	// Removing a non-existent device should fail.
	if err := cfg.RemoveDevice("ghost"); err == nil {
		t.Error("expected error when removing non-existent device")
	}
}

func TestRemoveDeviceKeepsDefaultIfDifferent(t *testing.T) {
	cfg := defaultConfig()
	_ = cfg.AddDevice("a", DeviceConfig{Address: "1.1.1.1:8728"})
	_ = cfg.AddDevice("b", DeviceConfig{Address: "2.2.2.2:8728"})
	cfg.DefaultDevice = "a"

	if err := cfg.RemoveDevice("b"); err != nil {
		t.Fatalf("RemoveDevice: %v", err)
	}
	if cfg.DefaultDevice != "a" {
		t.Errorf("DefaultDevice should remain 'a', got %q", cfg.DefaultDevice)
	}
}

func TestSetDefaultDevice(t *testing.T) {
	cfg := defaultConfig()
	_ = cfg.AddDevice("router", DeviceConfig{Address: "10.0.0.1:8728", Username: "admin"})

	if err := cfg.SetDefaultDevice("router"); err != nil {
		t.Fatalf("SetDefaultDevice: %v", err)
	}
	if cfg.DefaultDevice != "router" {
		t.Errorf("want DefaultDevice=router, got %q", cfg.DefaultDevice)
	}

	// Setting a non-existent device should fail.
	if err := cfg.SetDefaultDevice("nope"); err == nil {
		t.Error("expected error when setting default to non-existent device")
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		output  string
		wantErr bool
	}{
		{"table", false},
		{"json", false},
		{"yaml", true},
		{"", true},
		{"TABLE", true},
		{"JSON", true},
	}

	for _, tt := range tests {
		cfg := defaultConfig()
		cfg.DefaultOutput = tt.output
		err := cfg.Validate()
		if (err != nil) != tt.wantErr {
			t.Errorf("Validate(%q): err=%v, wantErr=%v", tt.output, err, tt.wantErr)
		}
	}
}

func TestSaveFilePermissions(t *testing.T) {
	path := tempConfigPath(t)

	cfg := defaultConfig()
	if err := cfg.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}

	perm := info.Mode().Perm()
	if perm != 0600 {
		t.Errorf("file permissions: want 0600, got %04o", perm)
	}

	// Check that the parent directory has restricted permissions.
	dirInfo, err := os.Stat(filepath.Dir(path))
	if err != nil {
		t.Fatalf("Stat dir: %v", err)
	}
	dirPerm := dirInfo.Mode().Perm()
	if dirPerm != 0700 {
		t.Errorf("directory permissions: want 0700, got %04o", dirPerm)
	}
}

func TestLoadReturnsErrorOnBadTOML(t *testing.T) {
	path := tempConfigPath(t)
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	if err := os.WriteFile(path, []byte("not valid [[ toml"), 0600); err != nil {
		t.Fatalf("write: %v", err)
	}

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error loading invalid TOML")
	}
}

func TestLoadNilDevicesMapInitialized(t *testing.T) {
	path := tempConfigPath(t)
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	// A minimal TOML file with no [devices] section.
	content := []byte(`default_output = "table"` + "\n")
	if err := os.WriteFile(path, content, 0600); err != nil {
		t.Fatalf("write: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Devices == nil {
		t.Fatal("Devices map should be initialized even when omitted from TOML")
	}
}

func TestAddDeviceNilMap(t *testing.T) {
	cfg := &Config{}
	dev := DeviceConfig{Address: "1.2.3.4:8728", Username: "admin"}
	if err := cfg.AddDevice("test", dev); err != nil {
		t.Fatalf("AddDevice on nil map: %v", err)
	}
	if _, ok := cfg.Devices["test"]; !ok {
		t.Error("device not added when starting from nil map")
	}
}

func TestLoadEnvClassAndBlastRadiusFields(t *testing.T) {
	path := tempConfigPath(t)
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	content := []byte(`
default_output = "table"

[devices.edge]
address = "10.0.0.1:8728"
username = "admin"
env_class = "prod"
max_session_changes = 10
allowed_write_paths = ["/ip/address", "/ip/firewall"]
denied_write_paths = ["/system/script"]
exec_allow = ["/interface/*", "/ip/address/*"]
exec_deny = ["/tool/*"]
require_backup_before_write = true

[devices.lab]
address = "192.168.88.1:8728"
username = "admin"
`)
	if err := os.WriteFile(path, content, 0600); err != nil {
		t.Fatalf("write: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	edge := cfg.Devices["edge"]
	if edge.EnvClass != "prod" {
		t.Errorf("env_class: got %q", edge.EnvClass)
	}
	if edge.MaxSessionChanges != 10 {
		t.Errorf("max_session_changes: got %d", edge.MaxSessionChanges)
	}
	if len(edge.AllowedWritePaths) != 2 || edge.AllowedWritePaths[0] != "/ip/address" {
		t.Errorf("allowed_write_paths: %+v", edge.AllowedWritePaths)
	}
	if len(edge.DeniedWritePaths) != 1 || edge.DeniedWritePaths[0] != "/system/script" {
		t.Errorf("denied_write_paths: %+v", edge.DeniedWritePaths)
	}
	if len(edge.ExecAllow) != 2 || edge.ExecAllow[0] != "/interface/*" {
		t.Errorf("exec_allow: %+v", edge.ExecAllow)
	}
	if len(edge.ExecDeny) != 1 || edge.ExecDeny[0] != "/tool/*" {
		t.Errorf("exec_deny: %+v", edge.ExecDeny)
	}
	if !edge.RequireBackupBeforeWrite {
		t.Error("require_backup_before_write: want true")
	}

	lab := cfg.Devices["lab"]
	if lab.EnvClass != "" {
		t.Errorf("lab env_class should be empty, got %q", lab.EnvClass)
	}
	if lab.RequireBackupBeforeWrite {
		t.Error("lab require_backup_before_write should default false")
	}
}

func TestEffectiveEnvClass(t *testing.T) {
	tests := []struct {
		envClass string
		strict   bool
		want     string
	}{
		{"", false, EnvClassLab},
		{"lab", false, EnvClassLab},
		{"staging", false, EnvClassStaging},
		{"prod", false, EnvClassProd},
		{"production", false, EnvClassProd},
		{"PROD", false, EnvClassProd},
		{"lab", true, EnvClassProd},
		{"", true, EnvClassProd},
		{"unknown", false, EnvClassLab},
	}
	for _, tt := range tests {
		got := EffectiveEnvClass(DeviceConfig{EnvClass: tt.envClass}, tt.strict)
		if got != tt.want {
			t.Errorf("EffectiveEnvClass(%q, strict=%v) = %q, want %q", tt.envClass, tt.strict, got, tt.want)
		}
	}
}

func TestEffectiveMaxSessionChanges(t *testing.T) {
	if got := EffectiveMaxSessionChanges(DeviceConfig{}, EnvClassProd); got != DefaultMaxSessionChangesProd {
		t.Errorf("prod default: got %d", got)
	}
	if got := EffectiveMaxSessionChanges(DeviceConfig{MaxSessionChanges: 5}, EnvClassProd); got != 5 {
		t.Errorf("explicit: got %d", got)
	}
	if got := EffectiveMaxSessionChanges(DeviceConfig{}, EnvClassLab); got != 0 {
		t.Errorf("lab unlimited: got %d", got)
	}
	if got := EffectiveMaxSessionChanges(DeviceConfig{}, EnvClassStaging); got != 0 {
		t.Errorf("staging unlimited default: got %d", got)
	}
}

func TestROSStrict(t *testing.T) {
	t.Setenv("ROS_STRICT", "")
	if ROSStrict() {
		t.Fatal("expected false when unset")
	}
	t.Setenv("ROS_STRICT", "1")
	if !ROSStrict() {
		t.Fatal("expected true for ROS_STRICT=1")
	}
	t.Setenv("ROS_STRICT", "true")
	if !ROSStrict() {
		t.Fatal("expected true for ROS_STRICT=true")
	}
}

func TestSaveLoadRoundtripGuardrailFields(t *testing.T) {
	path := tempConfigPath(t)
	original := &Config{
		DefaultOutput: "table",
		Devices: map[string]DeviceConfig{
			"edge": {
				Address:           "10.0.0.1:8728",
				Username:          "admin",
				EnvClass:          "prod",
				MaxSessionChanges: 12,
				AllowedWritePaths: []string{"/ip/address"},
				DeniedWritePaths:  []string{"/system/script"},
				ExecAllow:         []string{"/interface/*"},
				ExecDeny:          []string{"/tool/*"},
			},
		},
	}
	if err := original.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	got := loaded.Devices["edge"]
	want := original.Devices["edge"]
	if !reflect.DeepEqual(got, want) {
		t.Errorf("want %+v, got %+v", want, got)
	}
}
