// Package device manages the device inventory, building on top of the config
// package to provide higher-level operations for adding, removing, listing,
// and resolving RouterOS devices.
package device

import (
	"fmt"

	"github.com/nic0der-im/routeros-cli/internal/config"
)

// Inventory manages the device registry backed by a Config that is persisted
// to disk on every mutating operation.
type Inventory struct {
	cfg     *config.Config
	cfgPath string
}

// NewInventory creates a new Inventory from a loaded config. The cfgPath is
// used to persist changes back to disk when devices are added or removed.
func NewInventory(cfg *config.Config, cfgPath string) *Inventory {
	return &Inventory{
		cfg:     cfg,
		cfgPath: cfgPath,
	}
}

// Add adds a new device to the inventory and persists the config to disk.
// It returns an error if a device with the given name already exists or if
// the config cannot be saved.
func (inv *Inventory) Add(name string, dev config.DeviceConfig) error {
	if err := inv.cfg.AddDevice(name, dev); err != nil {
		return err
	}
	if err := inv.cfg.Save(inv.cfgPath); err != nil {
		return fmt.Errorf("saving config after add: %w", err)
	}
	return nil
}

// Remove removes a device from the inventory and persists the config to disk.
// It returns an error if the named device does not exist or if the config
// cannot be saved.
func (inv *Inventory) Remove(name string) error {
	if err := inv.cfg.RemoveDevice(name); err != nil {
		return err
	}
	if err := inv.cfg.Save(inv.cfgPath); err != nil {
		return fmt.Errorf("saving config after remove: %w", err)
	}
	return nil
}

// List returns all device names and their configurations. If no devices are
// configured, an empty map is returned.
func (inv *Inventory) List() map[string]config.DeviceConfig {
	if inv.cfg.Devices == nil {
		return make(map[string]config.DeviceConfig)
	}
	return inv.cfg.Devices
}

// Get returns the configuration of a single device by name. It returns an
// error if the device is not found in the inventory.
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

// SetDefault sets the default device and persists the config to disk. It
// returns an error if the named device does not exist in the inventory or if
// the config cannot be saved.
func (inv *Inventory) SetDefault(name string) error {
	if err := inv.cfg.SetDefaultDevice(name); err != nil {
		return err
	}
	if err := inv.cfg.Save(inv.cfgPath); err != nil {
		return fmt.Errorf("saving config after set-default: %w", err)
	}
	return nil
}

// Default returns the name of the default device. An empty string indicates
// that no default has been set.
func (inv *Inventory) Default() string {
	return inv.cfg.DefaultDevice
}

// Resolve determines which device to use. An explicit name takes highest
// priority, followed by the configured default device. If neither is
// available, an error is returned.
func (inv *Inventory) Resolve(explicit string) (string, config.DeviceConfig, error) {
	if explicit != "" {
		dev, err := inv.Get(explicit)
		if err != nil {
			return "", config.DeviceConfig{}, err
		}
		return explicit, dev, nil
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
