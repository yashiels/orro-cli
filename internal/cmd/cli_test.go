package cmd_test

import (
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// orroBin finds or builds the test binary.
func orroBin(t *testing.T) string {
	t.Helper()
	// Go test caches build results; build into temp dir.
	bin := filepath.Join(t.TempDir(), "orro")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	cmd := exec.Command("go", "build", "-o", bin, "github.com/yashiels/orro-cli/cmd/orro")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build failed: %v\n%s", err, out)
	}
	return bin
}

func TestVersion(t *testing.T) {
	bin := orroBin(t)
	out, err := exec.Command(bin, "--version").Output()
	if err != nil {
		t.Fatalf("--version failed: %v", err)
	}
	if !strings.Contains(string(out), "0.2.0") {
		t.Errorf("--version output %q does not contain '0.2.0'", out)
	}
}

func TestHelp(t *testing.T) {
	bin := orroBin(t)
	cmd := exec.Command(bin, "--help")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("--help failed: %v\n%s", err, out)
	}
	for _, want := range []string{"status", "up", "down", "stop", "sit", "stand", "goto", "height", "presets", "config"} {
		if !strings.Contains(string(out), want) {
			t.Errorf("--help output missing command %q", want)
		}
	}
}

func TestConfigPath(t *testing.T) {
	bin := orroBin(t)
	out, err := exec.Command(bin, "config", "path").Output()
	if err != nil {
		t.Fatalf("config path failed: %v", err)
	}
	s := strings.TrimSpace(string(out))
	if !strings.HasSuffix(s, "config.toml") {
		t.Errorf("config path = %q, want path ending in config.toml", s)
	}
}

func TestConfigPathCustom(t *testing.T) {
	bin := orroBin(t)
	out, err := exec.Command(bin, "--config", "/tmp/my-orro.toml", "config", "path").Output()
	if err != nil {
		t.Fatalf("config path with --config failed: %v", err)
	}
	s := strings.TrimSpace(string(out))
	if s != "/tmp/my-orro.toml" {
		t.Errorf("config path = %q, want /tmp/my-orro.toml", s)
	}
}

func TestConfigInitTemplate(t *testing.T) {
	bin := orroBin(t)
	// When stdin is not a TTY (as in tests), should print template.
	cmd := exec.Command(bin, "config", "init")
	cmd.Stdin = strings.NewReader("") // non-TTY stdin
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("config init failed: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "endpoint") {
		t.Errorf("config init template missing 'endpoint': %s", out)
	}
}

func TestNoCommandPrintsHelp(t *testing.T) {
	bin := orroBin(t)
	cmd := exec.Command(bin)
	out, _ := cmd.CombinedOutput()
	if !strings.Contains(string(out), "Usage:") {
		t.Errorf("running orro with no command should print usage, got: %s", out)
	}
}
