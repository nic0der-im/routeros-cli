// Package config handles loading, saving, and validating the ros
// TOML configuration file stored at ~/.config/ros/config.toml.
package config

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

const (
	appName     = "ros"
	legacyApp   = "routeros-cli"
)

// Config is the top-level configuration for ros.
type Config struct {
	DefaultDevice string                  `toml:"default_device"`
	DefaultOutput string                  `toml:"default_output"`
	Devices       map[string]DeviceConfig `toml:"devices"`
	TLS           TLSConfig               `toml:"tls"`
}

// DeviceConfig holds connection details for a single RouterOS device.
type DeviceConfig struct {
	Address  string   `toml:"address"`
	Username string   `toml:"username"`
	TLS      bool     `toml:"tls"`
	ID       string   `toml:"id,omitempty"`
	Tags     []string `toml:"tags,omitempty"`
	Notes    string   `toml:"notes,omitempty"`
}

// TLSConfig holds TLS-related settings applied globally unless overridden.
type TLSConfig struct {
	InsecureSkipVerify bool   `toml:"insecure_skip_verify"`
	CACert             string `toml:"ca_cert"`
}

// validOutputFormats enumerates the accepted values for DefaultOutput.
var validOutputFormats = map[string]bool{
	"table": true,
	"json":  true,
}

// DefaultPath returns the default configuration file path:
// ~/.config/ros/config.toml (with automatic migration from routeros-cli).
func DefaultPath() string {
	home := homeDir()
	return filepath.Join(home, ".config", appName, "config.toml")
}

// LegacyPath returns the previous config path for migration.
func LegacyPath() string {
	return filepath.Join(homeDir(), ".config", legacyApp, "config.toml")
}

func homeDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = os.Getenv("HOME")
		if home == "" {
			home = "."
		}
	}
	return home
}

// defaultConfig returns a Config populated with safe defaults.
func defaultConfig() *Config {
	return &Config{
		DefaultOutput: "table",
		Devices:       make(map[string]DeviceConfig),
	}
}

// Load reads a TOML configuration file from path. If the file does not exist,
// it attempts migration from the legacy routeros-cli path, otherwise a default
// configuration is created, written to disk, and returned.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("reading config: %w", err)
		}

		// Try migrating from legacy path when loading the default location.
		if path == DefaultPath() {
			if migrated, mErr := migrateLegacy(path); mErr == nil && migrated != nil {
				return migrated, nil
			}
		}

		cfg := defaultConfig()
		if err := cfg.Save(path); err != nil {
			return nil, fmt.Errorf("creating default config: %w", err)
		}
		return cfg, nil
	}

	cfg := defaultConfig()
	if err := toml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	if cfg.Devices == nil {
		cfg.Devices = make(map[string]DeviceConfig)
	}

	return cfg, nil
}

func migrateLegacy(newPath string) (*Config, error) {
	legacy := LegacyPath()
	data, err := os.ReadFile(legacy)
	if err != nil {
		return nil, err
	}
	cfg := defaultConfig()
	if err := toml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}
	if cfg.Devices == nil {
		cfg.Devices = make(map[string]DeviceConfig)
	}
	if err := cfg.Save(newPath); err != nil {
		return nil, err
	}
	return cfg, nil
}

// Save writes the configuration to path as TOML. Parent directories are
// created with mode 0700 and the file itself is written with mode 0600.
func (c *Config) Save(path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}

	var buf bytes.Buffer
	enc := toml.NewEncoder(&buf)
	if err := enc.Encode(c); err != nil {
		return fmt.Errorf("encoding config: %w", err)
	}

	if err := os.WriteFile(path, buf.Bytes(), 0600); err != nil {
		return fmt.Errorf("writing config: %w", err)
	}
	return nil
}

// Validate checks that the configuration values are acceptable.
func (c *Config) Validate() error {
	if !validOutputFormats[c.DefaultOutput] {
		return fmt.Errorf("invalid default_output %q: must be \"table\" or \"json\"", c.DefaultOutput)
	}
	return nil
}

// SetDefaultDevice sets the default device to name.
func (c *Config) SetDefaultDevice(name string) error {
	if _, ok := c.Devices[name]; !ok {
		return fmt.Errorf("device %q not found in configuration", name)
	}
	c.DefaultDevice = name
	return nil
}

// AddDevice adds a new device to the configuration under the given name.
func (c *Config) AddDevice(name string, dev DeviceConfig) error {
	if c.Devices == nil {
		c.Devices = make(map[string]DeviceConfig)
	}
	if _, ok := c.Devices[name]; ok {
		return fmt.Errorf("device %q already exists", name)
	}
	c.Devices[name] = dev
	return nil
}

// UpdateDevice replaces an existing device configuration.
func (c *Config) UpdateDevice(name string, dev DeviceConfig) error {
	if _, ok := c.Devices[name]; !ok {
		return fmt.Errorf("device %q not found", name)
	}
	c.Devices[name] = dev
	return nil
}

// RemoveDevice removes a device from the configuration.
func (c *Config) RemoveDevice(name string) error {
	if _, ok := c.Devices[name]; !ok {
		return fmt.Errorf("device %q not found", name)
	}
	delete(c.Devices, name)
	if c.DefaultDevice == name {
		c.DefaultDevice = ""
	}
	return nil
}
