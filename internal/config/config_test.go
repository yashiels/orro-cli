package config_test

import (
	"os"
	"os/exec"
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
	// Skip if 1Password CLI is on PATH and signed in — it would supply credentials,
	// making the "missing" assertion unreliable on developer machines.
	if _, err := exec.LookPath("op"); err == nil {
		// Try a quick probe; if it succeeds, 1Password is active.
		out, err2 := exec.Command("op", "whoami").Output()
		if err2 == nil && len(out) > 0 {
			t.Skip("1Password CLI is active; skipping missing-credentials test")
		}
	}

	// Clear any env vars that might satisfy requirements.
	for _, k := range []string{"ORRO_ENDPOINT", "ORRO_ACCESS_ID", "ORRO_ACCESS_SECRET", "ORRO_DEVICE_ID"} {
		t.Setenv(k, "")
	}

	_, err := config.Load(config.LoadOptions{
		ConfigPath:         "/nonexistent/path/config.toml",
		RequireCredentials: true,
	})
	if err == nil {
		t.Fatal("expected error for missing credentials, got nil")
	}
	if !strings.Contains(err.Error(), "missing required configuration") {
		t.Errorf("error should mention 'missing required configuration', got: %v", err)
	}
}

func TestToMapRedact(t *testing.T) {
	t.Setenv("ORRO_ENDPOINT", "https://test.example.com")
	t.Setenv("ORRO_ACCESS_ID", "test_id")
	t.Setenv("ORRO_ACCESS_SECRET", "super_secret")
	t.Setenv("ORRO_DEVICE_ID", "test_device")
	t.Setenv("ORRO_LOCAL_KEY", "my_local_key")

	cfg, err := config.Load(config.LoadOptions{RequireCredentials: true})
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

	cfg, err := config.Load(config.LoadOptions{RequireCredentials: true})
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
