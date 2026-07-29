// Package device manages the device inventory, building on top of the config
// package to provide higher-level operations for adding, removing, listing,
// and resolving RouterOS devices.
package device

import (
	"fmt"
	"net"
	"strings"

	"github.com/nic0der-im/routeros-cli/internal/config"
)

// Inventory manages the device registry backed by a Config that is persisted
// to disk on every mutating operation.
type Inventory struct {
	cfg     *config.Config
	cfgPath string
}

// NewInventory creates a new Inventory from a loaded config.
func NewInventory(cfg *config.Config, cfgPath string) *Inventory {
	return &Inventory{
		cfg:     cfg,
		cfgPath: cfgPath,
	}
}

// Add adds a new device to the inventory and persists the config to disk.
func (inv *Inventory) Add(name string, dev config.DeviceConfig) error {
	if err := inv.cfg.AddDevice(name, dev); err != nil {
		return err
	}
	if err := inv.cfg.Save(inv.cfgPath); err != nil {
		return fmt.Errorf("saving config after add: %w", err)
	}
	return nil
}

// Update updates an existing device and persists.
func (inv *Inventory) Update(name string, dev config.DeviceConfig) error {
	if err := inv.cfg.UpdateDevice(name, dev); err != nil {
		return err
	}
	if err := inv.cfg.Save(inv.cfgPath); err != nil {
		return fmt.Errorf("saving config after update: %w", err)
	}
	return nil
}

// Remove removes a device from the inventory and persists the config to disk.
func (inv *Inventory) Remove(name string) error {
	if err := inv.cfg.RemoveDevice(name); err != nil {
		return err
	}
	if err := inv.cfg.Save(inv.cfgPath); err != nil {
		return fmt.Errorf("saving config after remove: %w", err)
	}
	return nil
}

// List returns all device names and their configurations.
func (inv *Inventory) List() map[string]config.DeviceConfig {
	if inv.cfg.Devices == nil {
		return make(map[string]config.DeviceConfig)
	}
	return inv.cfg.Devices
}

// Get returns the configuration of a single device by exact name.
func (inv *Inventory) Get(name string) (config.DeviceConfig, error) {
	if inv.cfg.Devices == nil {
		return config.DeviceConfig{}, fmt.Errorf("device %q not found", name)
	}
	dev, ok := inv.cfg.Devices[name]
	if !ok {
		return config.DeviceConfig{}, fmt.Errorf("device %q not found", name)
	}
	return dev, nil
}

// Lookup finds a device by name, id slug, or host/IP (unique match).
func (inv *Inventory) Lookup(query string) (string, config.DeviceConfig, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return "", config.DeviceConfig{}, fmt.Errorf("empty device query")
	}

	// Exact name.
	if dev, err := inv.Get(query); err == nil {
		return query, dev, nil
	}

	var byID []string
	var byHost []string
	hostQuery := hostOnly(query)

	for name, dev := range inv.List() {
		if dev.ID != "" && strings.EqualFold(dev.ID, query) {
			byID = append(byID, name)
		}
		if hostOnly(dev.Address) == hostQuery {
			byHost = append(byHost, name)
		}
	}

	if len(byID) == 1 {
		name := byID[0]
		dev, _ := inv.Get(name)
		return name, dev, nil
	}
	if len(byID) > 1 {
		return "", config.DeviceConfig{}, fmt.Errorf("ambiguous device id %q matches %v", query, byID)
	}

	if len(byHost) == 1 {
		name := byHost[0]
		dev, _ := inv.Get(name)
		return name, dev, nil
	}
	if len(byHost) > 1 {
		return "", config.DeviceConfig{}, fmt.Errorf("ambiguous address %q matches %v", query, byHost)
	}

	return "", config.DeviceConfig{}, fmt.Errorf("device %q not found", query)
}

func hostOnly(addr string) string {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		// Maybe bare IP/hostname without port.
		return addr
	}
	return host
}

// SetDefault sets the default device and persists the config to disk.
func (inv *Inventory) SetDefault(name string) error {
	resolved, _, err := inv.Lookup(name)
	if err != nil {
		return err
	}
	if err := inv.cfg.SetDefaultDevice(resolved); err != nil {
		return err
	}
	if err := inv.cfg.Save(inv.cfgPath); err != nil {
		return fmt.Errorf("saving config after set-default: %w", err)
	}
	return nil
}

// Default returns the name of the default device.
func (inv *Inventory) Default() string {
	return inv.cfg.DefaultDevice
}

// Resolve determines which device to use. An explicit query takes highest
// priority (name, id, or IP), followed by the configured default device.
func (inv *Inventory) Resolve(explicit string) (string, config.DeviceConfig, error) {
	if explicit != "" {
		return inv.Lookup(explicit)
	}

	if def := inv.Default(); def != "" {
		dev, err := inv.Get(def)
		if err != nil {
			return "", config.DeviceConfig{}, err
		}
		return def, dev, nil
	}

	return "", config.DeviceConfig{}, fmt.Errorf("no device specified and no default set")
}

// InferTLS returns whether TLS should be used based on the address port
// when the caller did not explicitly set the TLS flag.
func InferTLS(address string, tlsFlagChanged bool, tlsFlag bool) bool {
	if tlsFlagChanged {
		return tlsFlag
	}
	_, port, err := net.SplitHostPort(address)
	if err != nil {
		return tlsFlag
	}
	switch port {
	case "8728":
		return false
	case "8729":
		return true
	default:
		return tlsFlag
	}
}
