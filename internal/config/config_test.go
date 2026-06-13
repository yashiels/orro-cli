package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yashiels/orro-cli/internal/config"
)

func TestDefaultConfigPath(t *testing.T) {
	home, _ := os.UserHomeDir()
	want := filepath.Join(home, ".config", "orro", "config.toml")
	got := config.DefaultConfigPath()
	if got != want {
		t.Errorf("DefaultConfigPath() = %q, want %q", got, want)
	}
}

func TestDefaultConfigPathXDG(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/tmp/xdgtest")
	got := config.DefaultConfigPath()
	want := "/tmp/xdgtest/orro/config.toml"
	if got != want {
		t.Errorf("DefaultConfigPath() with XDG = %q, want %q", got, want)
	}
}

func TestDefaultLANDPs(t *testing.T) {
	dps := config.DefaultLANDPs
	required := []string{"child_lock", "move_up", "move_down", "height_display", "memory_location"}
	for _, code := range required {
		if _, ok := dps[code]; !ok {
			t.Errorf("DefaultLANDPs missing %q", code)
		}
	}
	if dps["move_up"] != 150 {
		t.Errorf("move_up DP = %d, want 150", dps["move_up"])
	}
}

func TestDefaultPresets(t *testing.T) {
	p := config.DefaultPresets
	if p["sit"] != "mem1" {
		t.Errorf("sit preset = %q, want mem1", p["sit"])
	}
	if p["stand"] != "mem3" {
		t.Errorf("stand preset = %q, want mem3", p["stand"])
	}
}

func TestLoadEnvVars(t *testing.T) {
	t.Setenv("ORRO_ENDPOINT", "https://test.example.com")
	t.Setenv("ORRO_ACCESS_ID", "test_id")
	t.Setenv("ORRO_ACCESS_SECRET", "test_secret")
	t.Setenv("ORRO_DEVICE_ID", "test_device")

	cfg, err := config.Load(config.LoadOptions{
		SkipOP:             true,
		RequireCredentials: true,
	})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Endpoint != "https://test.example.com" {
		t.Errorf("Endpoint = %q", cfg.Endpoint)
	}
	if cfg.AccessID != "test_id" {
		t.Errorf("AccessID = %q", cfg.AccessID)
	}
	if cfg.DeviceID != "test_device" {
		t.Errorf("DeviceID = %q", cfg.DeviceID)
	}
}

func TestLoadMissingCredentials(t *testing.T) {
	// Clear any env vars that might satisfy requirements.
	for _, k := range []string{"ORRO_ENDPOINT", "ORRO_ACCESS_ID", "ORRO_ACCESS_SECRET", "ORRO_DEVICE_ID"} {
		t.Setenv(k, "")
	}

	_, err := config.Load(config.LoadOptions{
		ConfigPath:         "/nonexistent/path/config.toml",
		SkipOP:             true,
		RequireCredentials: true,
	})
	if err == nil {
		t.Fatal("expected error for missing credentials, got nil")
	}
	if !strings.Contains(err.Error(), "missing required configuration") {
		t.Errorf("error should mention 'missing required configuration', got: %v", err)
	}
}

func TestLoadEnvVarPresets(t *testing.T) {
	t.Setenv("ORRO_ENDPOINT", "https://test.example.com")
	t.Setenv("ORRO_ACCESS_ID", "test_id")
	t.Setenv("ORRO_ACCESS_SECRET", "test_secret")
	t.Setenv("ORRO_DEVICE_ID", "test_device")
	t.Setenv("ORRO_PRESETS", `{"sit":"mem2","stand":"mem4"}`)

	cfg, err := config.Load(config.LoadOptions{
		SkipOP:             true,
		RequireCredentials: true,
	})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Presets["sit"] != "mem2" {
		t.Errorf("sit preset = %q, want mem2", cfg.Presets["sit"])
	}
	if cfg.Presets["stand"] != "mem4" {
		t.Errorf("stand preset = %q, want mem4", cfg.Presets["stand"])
	}
}

func TestLoadEnvVarLANDPMap(t *testing.T) {
	t.Setenv("ORRO_ENDPOINT", "https://test.example.com")
	t.Setenv("ORRO_ACCESS_ID", "test_id")
	t.Setenv("ORRO_ACCESS_SECRET", "test_secret")
	t.Setenv("ORRO_DEVICE_ID", "test_device")
	t.Setenv("ORRO_LAN_DP_MAP", `{"custom_dp":99}`)

	cfg, err := config.Load(config.LoadOptions{
		SkipOP:             true,
		RequireCredentials: true,
	})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.LANDPs["custom_dp"] != 99 {
		t.Errorf("custom_dp = %d, want 99", cfg.LANDPs["custom_dp"])
	}
	// Default DPs should still be present.
	if cfg.LANDPs["move_up"] != 150 {
		t.Errorf("move_up should still be 150, got %d", cfg.LANDPs["move_up"])
	}
}

func TestToMapRedact(t *testing.T) {
	t.Setenv("ORRO_ENDPOINT", "https://test.example.com")
	t.Setenv("ORRO_ACCESS_ID", "test_id")
	t.Setenv("ORRO_ACCESS_SECRET", "super_secret")
	t.Setenv("ORRO_DEVICE_ID", "test_device")
	t.Setenv("ORRO_LOCAL_KEY", "my_local_key")

	cfg, err := config.Load(config.LoadOptions{SkipOP: true, RequireCredentials: true})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	m := cfg.ToMap(true)
	if m["access_secret"] != "***" {
		t.Errorf("access_secret should be redacted, got %q", m["access_secret"])
	}
	if m["local_key"] != "***" {
		t.Errorf("local_key should be redacted, got %q", m["local_key"])
	}
	if m["access_id"] != "test_id" {
		t.Errorf("access_id should not be redacted, got %q", m["access_id"])
	}
}

func TestToMapNoRedact(t *testing.T) {
	t.Setenv("ORRO_ENDPOINT", "https://test.example.com")
	t.Setenv("ORRO_ACCESS_ID", "test_id")
	t.Setenv("ORRO_ACCESS_SECRET", "super_secret")
	t.Setenv("ORRO_DEVICE_ID", "test_device")

	cfg, err := config.Load(config.LoadOptions{SkipOP: true, RequireCredentials: true})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	m := cfg.ToMap(false)
	if m["access_secret"] != "super_secret" {
		t.Errorf("access_secret should not be redacted, got %q", m["access_secret"])
	}
}

func TestWriteAndReadConfigFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	values := map[string]any{
		"endpoint":  "https://openapi.example.com",
		"device_id": "abc123",
	}
	written, err := config.WriteConfigFile(values, path)
	if err != nil {
		t.Fatalf("WriteConfigFile() error = %v", err)
	}
	if written != path {
		t.Errorf("WriteConfigFile() path = %q, want %q", written, path)
	}

	// Verify file permissions.
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat error: %v", err)
	}
	if fi.Mode().Perm() != 0o600 {
		t.Errorf("file permissions = %v, want 0600", fi.Mode().Perm())
	}

	// Load back.
	t.Setenv("ORRO_ACCESS_SECRET", "s")
	t.Setenv("ORRO_ACCESS_ID", "i")
	cfg, err := config.Load(config.LoadOptions{
		ConfigPath:         path,
		SkipOP:             true,
		RequireCredentials: false,
	})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Endpoint != "https://openapi.example.com" {
		t.Errorf("Endpoint = %q", cfg.Endpoint)
	}
	if cfg.DeviceID != "abc123" {
		t.Errorf("DeviceID = %q", cfg.DeviceID)
	}
}

func TestWriteConfigFileMerge(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	// Write initial values.
	_, err := config.WriteConfigFile(map[string]any{
		"endpoint":  "https://openapi.example.com",
		"device_id": "dev1",
	}, path)
	if err != nil {
		t.Fatalf("initial WriteConfigFile error: %v", err)
	}

	// Merge new values — endpoint should be updated, device_id preserved.
	_, err = config.WriteConfigFile(map[string]any{
		"endpoint": "https://openapi.tuyaus.com",
	}, path)
	if err != nil {
		t.Fatalf("merge WriteConfigFile error: %v", err)
	}

	// Load and verify merge.
	t.Setenv("ORRO_ACCESS_SECRET", "s")
	t.Setenv("ORRO_ACCESS_ID", "i")
	cfg, err := config.Load(config.LoadOptions{
		ConfigPath:         path,
		SkipOP:             true,
		RequireCredentials: false,
	})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Endpoint != "https://openapi.tuyaus.com" {
		t.Errorf("Endpoint after merge = %q, want tuyaus", cfg.Endpoint)
	}
	if cfg.DeviceID != "dev1" {
		t.Errorf("DeviceID should be preserved after merge, got %q", cfg.DeviceID)
	}
}

func TestLoadOverrides(t *testing.T) {
	t.Setenv("ORRO_ENDPOINT", "https://env.example.com")
	t.Setenv("ORRO_ACCESS_ID", "env_id")
	t.Setenv("ORRO_ACCESS_SECRET", "env_secret")
	t.Setenv("ORRO_DEVICE_ID", "env_device")

	cfg, err := config.Load(config.LoadOptions{
		SkipOP:             true,
		RequireCredentials: true,
		Overrides: map[string]string{
			"endpoint": "https://override.example.com",
		},
	})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	// Override should win over env.
	if cfg.Endpoint != "https://override.example.com" {
		t.Errorf("Endpoint = %q, want override", cfg.Endpoint)
	}
	// Non-overridden fields should come from env.
	if cfg.AccessID != "env_id" {
		t.Errorf("AccessID = %q, want env_id", cfg.AccessID)
	}
}
