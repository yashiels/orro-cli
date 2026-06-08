// Package output formats orro command results for the terminal or scripts.
package output

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/fatih/color"
)

var (
	keyColor = color.New(color.FgCyan)
	valColor = color.New(color.Reset)
)

// IsTTY reports whether stdout is a terminal.
func IsTTY() bool {
	fi, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

// Print prints payload in the requested format ("table" or "json").
func Print(payload any, format string) {
	if format == "json" {
		printJSON(payload)
		return
	}
	switch v := payload.(type) {
	case map[string]any:
		printTable(v)
	default:
		fmt.Println(payload)
	}
}

func printJSON(payload any) {
	b, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "orro: json marshal: %v\n", err)
		return
	}
	fmt.Println(string(b))
}

func printTable(m map[string]any) {
	if len(m) == 0 {
		return
	}

	// Sort keys for stable output.
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// Find longest key for alignment.
	maxLen := 0
	for _, k := range keys {
		if len(k) > maxLen {
			maxLen = len(k)
		}
	}

	for _, k := range keys {
		label := k + ":"
		padding := strings.Repeat(" ", maxLen-len(k)+1)
		keyColor.Printf("%-*s", maxLen+1, label)
		valColor.Printf("%s%s\n", padding, formatValue(m[k]))
	}
}

func formatValue(v any) string {
	if v == nil {
		return ""
	}
	switch val := v.(type) {
	case bool:
		if val {
			return "yes"
		}
		return "no"
	case map[string]any:
		parts := make([]string, 0, len(val))
		keys := make([]string, 0, len(val))
		for k := range val {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			parts = append(parts, fmt.Sprintf("%s=%v", k, val[k]))
		}
		return strings.Join(parts, ", ")
	case map[string]int:
		parts := make([]string, 0, len(val))
		keys := make([]string, 0, len(val))
		for k := range val {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			parts = append(parts, fmt.Sprintf("%s=%d", k, val[k]))
		}
		return strings.Join(parts, ", ")
	case map[string]string:
		parts := make([]string, 0, len(val))
		keys := make([]string, 0, len(val))
		for k := range val {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			parts = append(parts, fmt.Sprintf("%s=%s", k, val[k]))
		}
		return strings.Join(parts, ", ")
	case []any:
		strs := make([]string, len(val))
		for i, item := range val {
			strs[i] = fmt.Sprintf("%v", item)
		}
		return strings.Join(strs, ", ")
	default:
		return fmt.Sprintf("%v", val)
	}
}
