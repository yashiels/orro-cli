// Package config handles configuration loading for orro.
//
// Priority (highest first):
//  1. Explicit CLI flag overrides
//  2. Environment variables (ORRO_*)
//  3. TOML config file (~/.config/orro/config.toml)
//  4. 1Password vault (OpenClaw → Tuya IoT Platform)
//  5. Built-in defaults
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

const (
	DefaultVault = "OpenClaw"
	DefaultItem  = "Tuya IoT Platform"
)

// Version is the current orro version, overridden at build time via ldflags.
var Version = "0.2.0"

// DefaultLANDPs are the verified DP numbers for Orro standing desks.
var DefaultLANDPs = map[string]int{
	"child_lock":      6,
	"move_up":         150,
	"move_down":       151,
	"height_display":  152,
	"memory_location": 155,
}

// DefaultPresets maps friendly names to device memory slots.
var DefaultPresets = map[string]string{
	"sit":   "mem1",
	"stand": "mem3",
}

// SecretFields are redacted in config show output.
var SecretFields = map[string]bool{
	"access_secret": true,
	"local_key":     true,
}

// fileConfig mirrors the TOML file structure for unmarshalling.
type fileConfig struct {
	Endpoint     string            `toml:"endpoint"`
	AccessID     string            `toml:"access_id"`
	AccessSecret string            `toml:"access_secret"`
	DeviceID     string            `toml:"device_id"`
	LocalKey     string            `toml:"local_key"`
	LANIP        string            `toml:"lan_ip"`
	LANVersion   string            `toml:"lan_version"`
	LANDPs       map[string]int    `toml:"lan_dps"`
	Presets      map[string]string `toml:"presets"`
	OP           opSection         `toml:"op"`
}

type opSection struct {
	Item  string `toml:"item"`
	Vault string `toml:"vault"`
}

// Config holds the fully resolved runtime configuration.
type Config struct {
	Endpoint     string
	AccessID     string
	AccessSecret string
	DeviceID     string
	LocalKey     string
	LANIP        string
	LANVersion   string
	LANDPs       map[string]int
	Presets      map[string]string

	// Runtime flags — not persisted.
	Verbose bool
	Quiet   bool
}

// Debug prints to stderr when verbose mode is enabled.
func (c *Config) Debug(msg string) {
	if c.Verbose {
		fmt.Fprintf(os.Stderr, "[debug] %s\n", msg)
	}
}

// Info prints to stderr unless quiet mode is enabled.
func (c *Config) Info(msg string) {
	if !c.Quiet {
		fmt.Fprintln(os.Stderr, msg)
	}
}

// ToMap returns a plain map of config values, optionally redacting secrets.
func (c *Config) ToMap(redact bool) map[string]any {
	m := map[string]any{
		"endpoint":    c.Endpoint,
		"access_id":   c.AccessID,
		"device_id":   c.DeviceID,
		"local_key":   orNA(c.LocalKey),
		"lan_ip":      orNA(c.LANIP),
		"lan_version": orNA(c.LANVersion),
		"lan_dps":     c.LANDPs,
		"presets":     c.Presets,
	}
	if redact {
		if c.AccessSecret != "" {
			m["access_secret"] = "***"
		} else {
			m["access_secret"] = ""
		}
		if c.LocalKey != "" {
			m["local_key"] = "***"
		}
	} else {
		m["access_secret"] = c.AccessSecret
		m["local_key"] = c.LocalKey
	}
	return m
}

func orNA(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// DefaultConfigPath returns ~/.config/orro/config.toml (XDG-aware).
func DefaultConfigPath() string {
	xdg := os.Getenv("XDG_CONFIG_HOME")
	var base string
	if xdg != "" {
		base = xdg
	} else {
		home, _ := os.UserHomeDir()
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "orro", "config.toml")
}

// LoadOptions controls Config.Load behaviour.
type LoadOptions struct {
	ConfigPath         string
	Overrides          map[string]string
	Verbose            bool
	Quiet              bool
	RequireCredentials bool
	SkipOP             bool // Skip 1Password lookup (for testing).
}

// Load builds a Config by layering all sources.
func Load(opts LoadOptions) (*Config, error) {
	cfg := &Config{
		Verbose: opts.Verbose,
		Quiet:   opts.Quiet,
		LANDPs:  copyIntMap(DefaultLANDPs),
		Presets: copyStringMap(DefaultPresets),
	}

	// Layer 4: 1Password.
	opVault, opItem := DefaultVault, DefaultItem
	if !opts.SkipOP {
		if err := applyFromOP(cfg, opVault, opItem); err != nil {
			cfg.Debug(fmt.Sprintf("1Password unavailable: %v", err))
		}
	}

	// Layer 3: config file.
	configPath := opts.ConfigPath
	if configPath == "" {
		configPath = os.Getenv("ORRO_CONFIG")
	}
	if configPath == "" {
		configPath = DefaultConfigPath()
	}
	if err := applyFromFile(cfg, configPath); err != nil {
		cfg.Debug(fmt.Sprintf("config file: %v", err))
	}

	// Layer 2: environment variables.
	applyFromEnv(cfg)

	// Layer 1: explicit overrides.
	if opts.Overrides != nil {
		applyOverrides(cfg, opts.Overrides)
	}

	if opts.RequireCredentials {
		var missing []string
		for _, pair := range [][2]string{
			{"endpoint", cfg.Endpoint},
			{"access_id", cfg.AccessID},
			{"access_secret", cfg.AccessSecret},
			{"device_id", cfg.DeviceID},
		} {
			if pair[1] == "" {
				missing = append(missing, pair[0])
			}
		}
		if len(missing) > 0 {
			return nil, fmt.Errorf(
				"missing required configuration: %s.\nSet environment variables, create a config file, or install the 1Password CLI (op)",
				strings.Join(missing, ", "),
			)
		}
	}

	return cfg, nil
}

// applyFromOP reads credentials from the 1Password vault.
func applyFromOP(cfg *Config, vault, item string) error {
	if !opAvailable() {
		return fmt.Errorf("op CLI not found on PATH")
	}

	fields := map[string]*string{
		"API Endpoint":  &cfg.Endpoint,
		"Access ID":     &cfg.AccessID,
		"Access Secret": &cfg.AccessSecret,
		"Device ID":     &cfg.DeviceID,
		"Local Key":     &cfg.LocalKey,
		"LAN IP":        &cfg.LANIP,
		"LAN Version":   &cfg.LANVersion,
	}

	for opField, ptr := range fields {
		val, err := opRead(vault, item, opField)
		if err != nil {
			continue // field might not exist
		}
		if val != "" {
			*ptr = val
		}
	}

	// JSON blobs.
	if raw, err := opRead(vault, item, "LAN DP Map"); err == nil && raw != "" {
		var m map[string]int
		if err2 := json.Unmarshal([]byte(raw), &m); err2 == nil {
			for k, v := range m {
				cfg.LANDPs[k] = v
			}
		}
	}
	if raw, err := opRead(vault, item, "Presets"); err == nil && raw != "" {
		var m map[string]string
		if err2 := json.Unmarshal([]byte(raw), &m); err2 == nil {
			for k, v := range m {
				cfg.Presets[k] = v
			}
		}
	}

	return nil
}

func opAvailable() bool {
	_, err := exec.LookPath("op")
	return err == nil
}

func opRead(vault, item, field string) (string, error) {
	ref := fmt.Sprintf("op://%s/%s/%s", vault, item, field)
	out, err := exec.Command("op", "read", ref).Output()
	if err != nil {
		return "", err
	}
	return strings.TrimRight(string(out), "\n"), nil
}

// applyFromFile reads and applies the TOML config file.
func applyFromFile(cfg *Config, path string) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil
	}
	var fc fileConfig
	if _, err := toml.DecodeFile(path, &fc); err != nil {
		return fmt.Errorf("parse %s: %w", path, err)
	}

	if fc.Endpoint != "" {
		cfg.Endpoint = fc.Endpoint
	}
	if fc.AccessID != "" {
		cfg.AccessID = fc.AccessID
	}
	if fc.AccessSecret != "" {
		cfg.AccessSecret = fc.AccessSecret
	}
	if fc.DeviceID != "" {
		cfg.DeviceID = fc.DeviceID
	}
	if fc.LocalKey != "" {
		cfg.LocalKey = fc.LocalKey
	}
	if fc.LANIP != "" {
		cfg.LANIP = fc.LANIP
	}
	if fc.LANVersion != "" {
		cfg.LANVersion = fc.LANVersion
	}
	for k, v := range fc.LANDPs {
		cfg.LANDPs[k] = v
	}
	for k, v := range fc.Presets {
		cfg.Presets[k] = v
	}

	// Honour op section overrides.
	if fc.OP.Vault != "" || fc.OP.Item != "" {
		vault := DefaultVault
		if fc.OP.Vault != "" {
			vault = fc.OP.Vault
		}
		item := DefaultItem
		if fc.OP.Item != "" {
			item = fc.OP.Item
		}
		// Re-read with custom vault/item (errors are soft).
		_ = applyFromOP(cfg, vault, item)
	}

	return nil
}

// applyFromEnv reads ORRO_* environment variables.
func applyFromEnv(cfg *Config) {
	pairs := map[string]*string{
		"ORRO_ENDPOINT":      &cfg.Endpoint,
		"ORRO_ACCESS_ID":     &cfg.AccessID,
		"ORRO_ACCESS_SECRET": &cfg.AccessSecret,
		"ORRO_DEVICE_ID":     &cfg.DeviceID,
		"ORRO_LOCAL_KEY":     &cfg.LocalKey,
		"ORRO_LAN_IP":        &cfg.LANIP,
		"ORRO_LAN_VERSION":   &cfg.LANVersion,
	}
	for env, ptr := range pairs {
		if v := os.Getenv(env); v != "" {
			*ptr = v
		}
	}
	if raw := os.Getenv("ORRO_LAN_DP_MAP"); raw != "" {
		var m map[string]int
		if err := json.Unmarshal([]byte(raw), &m); err == nil {
			for k, v := range m {
				cfg.LANDPs[k] = v
			}
		}
	}
	if raw := os.Getenv("ORRO_PRESETS"); raw != "" {
		var m map[string]string
		if err := json.Unmarshal([]byte(raw), &m); err == nil {
			for k, v := range m {
				cfg.Presets[k] = v
			}
		}
	}
}

// applyOverrides applies explicit CLI flag overrides.
func applyOverrides(cfg *Config, overrides map[string]string) {
	pairs := map[string]*string{
		"endpoint":      &cfg.Endpoint,
		"access_id":     &cfg.AccessID,
		"access_secret": &cfg.AccessSecret,
		"device_id":     &cfg.DeviceID,
		"local_key":     &cfg.LocalKey,
		"lan_ip":        &cfg.LANIP,
		"lan_version":   &cfg.LANVersion,
	}
	for k, ptr := range pairs {
		if v, ok := overrides[k]; ok && v != "" {
			*ptr = v
		}
	}
}

// WriteConfigFile writes values to the TOML config file, merging with existing content.
func WriteConfigFile(values map[string]any, path string) (string, error) {
	if path == "" {
		path = DefaultConfigPath()
	}

	// Read existing.
	existing := map[string]any{}
	if _, err := os.Stat(path); err == nil {
		var fc fileConfig
		if _, err2 := toml.DecodeFile(path, &fc); err2 == nil {
			// Flatten to map for merging.
			existing = flattenFileConfig(fc)
		}
	}

	// Merge.
	for k, v := range values {
		existing[k] = v
	}

	// Write.
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return "", err
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return "", err
	}
	defer f.Close()

	enc := toml.NewEncoder(f)
	if err := enc.Encode(existing); err != nil {
		return "", err
	}
	return path, nil
}

func flattenFileConfig(fc fileConfig) map[string]any {
	m := map[string]any{}
	if fc.Endpoint != "" {
		m["endpoint"] = fc.Endpoint
	}
	if fc.AccessID != "" {
		m["access_id"] = fc.AccessID
	}
	if fc.AccessSecret != "" {
		m["access_secret"] = fc.AccessSecret
	}
	if fc.DeviceID != "" {
		m["device_id"] = fc.DeviceID
	}
	if fc.LocalKey != "" {
		m["local_key"] = fc.LocalKey
	}
	if fc.LANIP != "" {
		m["lan_ip"] = fc.LANIP
	}
	if fc.LANVersion != "" {
		m["lan_version"] = fc.LANVersion
	}
	if len(fc.LANDPs) > 0 {
		m["lan_dps"] = fc.LANDPs
	}
	if len(fc.Presets) > 0 {
		m["presets"] = fc.Presets
	}
	if fc.OP.Item != "" || fc.OP.Vault != "" {
		m["op"] = fc.OP
	}
	return m
}

func copyIntMap(m map[string]int) map[string]int {
	out := make(map[string]int, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

func copyStringMap(m map[string]string) map[string]string {
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}
