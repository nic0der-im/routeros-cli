package device

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/nic0der-im/routeros-cli/internal/config"
)

// newTestInventory creates an Inventory backed by a temp config file. The
// caller does not need to clean up -- t.TempDir handles that.
func newTestInventory(t *testing.T) *Inventory {
	t.Helper()
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.toml")

	cfg := &config.Config{
		DefaultOutput: "table",
		Devices:       make(map[string]config.DeviceConfig),
	}
	return NewInventory(cfg, cfgPath)
}

func sampleDevice(addr string) config.DeviceConfig {
	return config.DeviceConfig{
		Address:  addr,
		Username: "admin",
		TLS:      false,
	}
}

// ---------------------------------------------------------------------------
// Add + List
// ---------------------------------------------------------------------------

func TestAddAndList(t *testing.T) {
	inv := newTestInventory(t)

	if err := inv.Add("router1", sampleDevice("192.168.1.1:8728")); err != nil {
		t.Fatalf("Add router1: %v", err)
	}
	if err := inv.Add("router2", sampleDevice("192.168.1.2:8728")); err != nil {
		t.Fatalf("Add router2: %v", err)
	}

	devices := inv.List()
	if len(devices) != 2 {
		t.Fatalf("expected 2 devices, got %d", len(devices))
	}
	if devices["router1"].Address != "192.168.1.1:8728" {
		t.Errorf("router1 address = %q, want %q", devices["router1"].Address, "192.168.1.1:8728")
	}
	if devices["router2"].Address != "192.168.1.2:8728" {
		t.Errorf("router2 address = %q, want %q", devices["router2"].Address, "192.168.1.2:8728")
	}
}

// ---------------------------------------------------------------------------
// Add duplicate
// ---------------------------------------------------------------------------

func TestAddDuplicateReturnsError(t *testing.T) {
	inv := newTestInventory(t)

	if err := inv.Add("router1", sampleDevice("192.168.1.1:8728")); err != nil {
		t.Fatalf("first Add: %v", err)
	}
	if err := inv.Add("router1", sampleDevice("10.0.0.1:8728")); err == nil {
		t.Fatal("expected error when adding duplicate device, got nil")
	}
}

// ---------------------------------------------------------------------------
// Remove
// ---------------------------------------------------------------------------

func TestRemove(t *testing.T) {
	inv := newTestInventory(t)

	if err := inv.Add("router1", sampleDevice("192.168.1.1:8728")); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if err := inv.Remove("router1"); err != nil {
		t.Fatalf("Remove: %v", err)
	}

	devices := inv.List()
	if len(devices) != 0 {
		t.Fatalf("expected 0 devices after Remove, got %d", len(devices))
	}
}

// ---------------------------------------------------------------------------
// Remove non-existent
// ---------------------------------------------------------------------------

func TestRemoveNonExistentReturnsError(t *testing.T) {
	inv := newTestInventory(t)

	if err := inv.Remove("ghost"); err == nil {
		t.Fatal("expected error when removing non-existent device, got nil")
	}
}

// ---------------------------------------------------------------------------
// Get existing and non-existent
// ---------------------------------------------------------------------------

func TestGetExisting(t *testing.T) {
	inv := newTestInventory(t)

	if err := inv.Add("router1", sampleDevice("192.168.1.1:8728")); err != nil {
		t.Fatalf("Add: %v", err)
	}

	dev, err := inv.Get("router1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if dev.Address != "192.168.1.1:8728" {
		t.Errorf("Address = %q, want %q", dev.Address, "192.168.1.1:8728")
	}
	if dev.Username != "admin" {
		t.Errorf("Username = %q, want %q", dev.Username, "admin")
	}
}

func TestGetNonExistentReturnsError(t *testing.T) {
	inv := newTestInventory(t)

	if _, err := inv.Get("ghost"); err == nil {
		t.Fatal("expected error when getting non-existent device, got nil")
	}
}

// ---------------------------------------------------------------------------
// SetDefault
// ---------------------------------------------------------------------------

func TestSetDefault(t *testing.T) {
	inv := newTestInventory(t)

	if err := inv.Add("router1", sampleDevice("192.168.1.1:8728")); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if err := inv.SetDefault("router1"); err != nil {
		t.Fatalf("SetDefault: %v", err)
	}
	if got := inv.Default(); got != "router1" {
		t.Errorf("Default() = %q, want %q", got, "router1")
	}
}

func TestSetDefaultNonExistentReturnsError(t *testing.T) {
	inv := newTestInventory(t)

	if err := inv.SetDefault("ghost"); err == nil {
		t.Fatal("expected error when setting default to non-existent device, got nil")
	}
}

// ---------------------------------------------------------------------------
// Resolve
// ---------------------------------------------------------------------------

func TestResolveExplicit(t *testing.T) {
	inv := newTestInventory(t)

	if err := inv.Add("router1", sampleDevice("192.168.1.1:8728")); err != nil {
		t.Fatalf("Add router1: %v", err)
	}
	if err := inv.Add("router2", sampleDevice("192.168.1.2:8728")); err != nil {
		t.Fatalf("Add router2: %v", err)
	}
	// Set default to router1 but resolve explicitly to router2.
	if err := inv.SetDefault("router1"); err != nil {
		t.Fatalf("SetDefault: %v", err)
	}

	name, dev, err := inv.Resolve("router2")
	if err != nil {
		t.Fatalf("Resolve explicit: %v", err)
	}
	if name != "router2" {
		t.Errorf("name = %q, want %q", name, "router2")
	}
	if dev.Address != "192.168.1.2:8728" {
		t.Errorf("Address = %q, want %q", dev.Address, "192.168.1.2:8728")
	}
}

func TestResolveDefault(t *testing.T) {
	inv := newTestInventory(t)

	if err := inv.Add("router1", sampleDevice("192.168.1.1:8728")); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if err := inv.SetDefault("router1"); err != nil {
		t.Fatalf("SetDefault: %v", err)
	}

	name, dev, err := inv.Resolve("")
	if err != nil {
		t.Fatalf("Resolve default: %v", err)
	}
	if name != "router1" {
		t.Errorf("name = %q, want %q", name, "router1")
	}
	if dev.Address != "192.168.1.1:8728" {
		t.Errorf("Address = %q, want %q", dev.Address, "192.168.1.1:8728")
	}
}

func TestResolveNoDeviceReturnsError(t *testing.T) {
	inv := newTestInventory(t)

	_, _, err := inv.Resolve("")
	if err == nil {
		t.Fatal("expected error when resolving with no explicit and no default, got nil")
	}
	if err.Error() != "no device specified and no default set" {
		t.Errorf("error = %q, want %q", err.Error(), "no device specified and no default set")
	}
}

func TestResolveExplicitNonExistentReturnsError(t *testing.T) {
	inv := newTestInventory(t)

	if _, _, err := inv.Resolve("ghost"); err == nil {
		t.Fatal("expected error when resolving non-existent explicit device, got nil")
	}
}

// ---------------------------------------------------------------------------
// Persistence: Add should write to disk
// ---------------------------------------------------------------------------

func TestAddPersistsToDisk(t *testing.T) {
	inv := newTestInventory(t)

	if err := inv.Add("router1", sampleDevice("192.168.1.1:8728")); err != nil {
		t.Fatalf("Add: %v", err)
	}

	// Verify the config file was written.
	data, err := os.ReadFile(inv.cfgPath)
	if err != nil {
		t.Fatalf("reading config file: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("config file is empty after Add")
	}

	// Reload config from disk and verify the device is present.
	reloaded, err := config.Load(inv.cfgPath)
	if err != nil {
		t.Fatalf("reloading config: %v", err)
	}
	if _, ok := reloaded.Devices["router1"]; !ok {
		t.Error("router1 not found in reloaded config")
	}
}

// ---------------------------------------------------------------------------
// List returns empty map when no devices
// ---------------------------------------------------------------------------

func TestListEmpty(t *testing.T) {
	inv := newTestInventory(t)

	devices := inv.List()
	if devices == nil {
		t.Fatal("List() returned nil, want empty map")
	}
	if len(devices) != 0 {
		t.Fatalf("expected 0 devices, got %d", len(devices))
	}
}

// ---------------------------------------------------------------------------
// Default returns empty string when no default set
// ---------------------------------------------------------------------------

func TestDefaultEmpty(t *testing.T) {
	inv := newTestInventory(t)

	if got := inv.Default(); got != "" {
		t.Errorf("Default() = %q, want empty string", got)
	}
}
