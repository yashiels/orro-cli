package output_test

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/fatih/color"
	"github.com/yashiels/orro-cli/internal/output"
)

// captureStdout runs fn and captures what it writes to stdout,
// including output from fatih/color.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	oldColor := color.Output
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w
	color.Output = w

	fn()

	w.Close()
	os.Stdout = old
	color.Output = oldColor
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	return buf.String()
}

func TestPrintJSON(t *testing.T) {
	payload := map[string]any{
		"path":   "lan",
		"result": "ok",
	}

	got := captureStdout(t, func() {
		output.Print(payload, "json")
	})

	var parsed map[string]any
	if err := json.Unmarshal([]byte(got), &parsed); err != nil {
		t.Fatalf("output is not valid JSON: %v\nraw: %s", err, got)
	}
	if parsed["path"] != "lan" {
		t.Errorf("path = %v, want lan", parsed["path"])
	}
	if parsed["result"] != "ok" {
		t.Errorf("result = %v, want ok", parsed["result"])
	}
}

func TestPrintTableMap(t *testing.T) {
	payload := map[string]any{
		"path":   "cloud",
		"height": 1100,
	}

	got := captureStdout(t, func() {
		output.Print(payload, "table")
	})

	if !strings.Contains(got, "path:") {
		t.Errorf("table output missing 'path:': %q", got)
	}
	if !strings.Contains(got, "cloud") {
		t.Errorf("table output missing value 'cloud': %q", got)
	}
}

func TestPrintTableBooleans(t *testing.T) {
	payload := map[string]any{
		"locked": true,
		"moving": false,
	}

	got := captureStdout(t, func() {
		output.Print(payload, "table")
	})

	if !strings.Contains(got, "yes") {
		t.Errorf("true should be formatted as 'yes': %q", got)
	}
	if !strings.Contains(got, "no") {
		t.Errorf("false should be formatted as 'no': %q", got)
	}
}

func TestPrintTableNestedMap(t *testing.T) {
	payload := map[string]any{
		"presets": map[string]string{
			"sit":   "mem1",
			"stand": "mem3",
		},
	}

	got := captureStdout(t, func() {
		output.Print(payload, "table")
	})

	if !strings.Contains(got, "sit=mem1") {
		t.Errorf("nested map should show key=value pairs: %q", got)
	}
}

func TestPrintTableNilValue(t *testing.T) {
	payload := map[string]any{
		"lan_ip": nil,
	}

	got := captureStdout(t, func() {
		output.Print(payload, "table")
	})

	if !strings.Contains(got, "lan_ip:") {
		t.Errorf("nil value should still show key: %q", got)
	}
}

func TestPrintNonMap(t *testing.T) {
	got := captureStdout(t, func() {
		output.Print("hello world", "table")
	})

	if !strings.Contains(got, "hello world") {
		t.Errorf("non-map payload should be printed directly: %q", got)
	}
}

func TestPrintEmptyMap(t *testing.T) {
	got := captureStdout(t, func() {
		output.Print(map[string]any{}, "table")
	})

	if got != "" {
		t.Errorf("empty map should produce no output, got: %q", got)
	}
}

func TestPrintTableIntMap(t *testing.T) {
	payload := map[string]any{
		"dps": map[string]int{
			"move_up":   150,
			"move_down": 151,
		},
	}

	got := captureStdout(t, func() {
		output.Print(payload, "table")
	})

	if !strings.Contains(got, "move_up=150") {
		t.Errorf("int map should show key=value: %q", got)
	}
}

func TestPrintTableSlice(t *testing.T) {
	payload := map[string]any{
		"items": []any{"a", "b", "c"},
	}

	got := captureStdout(t, func() {
		output.Print(payload, "table")
	})

	if !strings.Contains(got, "a, b, c") {
		t.Errorf("slice should show comma-separated: %q", got)
	}
}
