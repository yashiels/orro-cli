// Package desk provides high-level operations for a Tuya-based standing desk.
package desk

import (
	"encoding/base64"
	"encoding/binary"
	"fmt"

	"github.com/yashiels/orro-cli/internal/config"
	"github.com/yashiels/orro-cli/internal/tuya"
)

// movementCommands builds the DP command list for a given action.
// target is only used for "goto"; for "sit"/"stand" the preset map is consulted.
func movementCommands(action string, cfg *config.Config, target string) ([]tuya.Command, error) {
	wake := tuya.Command{Code: "child_lock", Value: false}

	switch action {
	case "up":
		return []tuya.Command{
			wake,
			{Code: "move_down", Value: false},
			{Code: "move_up", Value: true},
		}, nil
	case "down":
		return []tuya.Command{
			wake,
			{Code: "move_up", Value: false},
			{Code: "move_down", Value: true},
		}, nil
	case "stop":
		return []tuya.Command{
			{Code: "move_up", Value: false},
			{Code: "move_down", Value: false},
		}, nil
	case "sit", "stand":
		preset, ok := cfg.Presets[action]
		if !ok {
			preset = "mem1"
		}
		return []tuya.Command{wake, {Code: "memory_location", Value: preset}}, nil
	case "goto":
		if target == "" {
			return nil, fmt.Errorf("goto: no target specified")
		}
		return []tuya.Command{wake, {Code: "memory_location", Value: target}}, nil
	default:
		return nil, fmt.Errorf("unknown movement action: %q", action)
	}
}

// MovementCommands is exported for command handlers.
func MovementCommands(action string, cfg *config.Config, target string) ([]tuya.Command, error) {
	return movementCommands(action, cfg, target)
}

// ──────────────────────────────────────────────
// LAN-then-cloud dispatch
// ──────────────────────────────────────────────

// LanThenCloud tries LAN first, falls back to Cloud. Returns {"path","result",...}.
func LanThenCloud(cfg *config.Config, commands []tuya.Command, forceCloud bool) (map[string]any, error) {
	if !forceCloud {
		lan, err := tuya.NewLAN(cfg)
		if err == nil {
			defer lan.Close()
			if sendErr := lan.SendCodes(commands); sendErr == nil {
				return map[string]any{"path": "lan", "result": "ok"}, nil
			} else {
				cfg.Debug(fmt.Sprintf("lan send failed (%v), falling back to cloud", sendErr))
			}
		} else {
			cfg.Debug(fmt.Sprintf("lan unavailable (%v), using cloud", err))
		}
	}

	cloud := tuya.NewCloud(cfg)
	result, err := cloud.Send(commands)
	if err != nil {
		return nil, err
	}
	return map[string]any{"path": "cloud", "result": result}, nil
}

// GetStatus fetches desk status, preferring LAN when available.
func GetStatus(cfg *config.Config, forceCloud bool) (map[string]any, error) {
	if !forceCloud {
		lan, err := tuya.NewLAN(cfg)
		if err == nil {
			defer lan.Close()
			status, statusErr := lan.Status()
			if statusErr == nil {
				return map[string]any{
					"path":    "lan",
					"ip":      lan.IP(),
					"version": lan.Version(),
					"status":  status,
				}, nil
			}
			cfg.Debug(fmt.Sprintf("lan status failed (%v), falling back to cloud", statusErr))
		} else {
			cfg.Debug(fmt.Sprintf("lan unavailable (%v), using cloud", err))
		}
	}

	cloud := tuya.NewCloud(cfg)
	status, err := cloud.Status()
	if err != nil {
		return nil, err
	}
	return map[string]any{"path": "cloud", "status": status}, nil
}

// GetPresets fetches preset/memory data from the Cloud shadow properties.
func GetPresets(cfg *config.Config) (map[string]any, error) {
	cloud := tuya.NewCloud(cfg)
	props, err := cloud.Properties()
	if err != nil {
		return nil, err
	}

	memHeightProp := props["memory_height"]
	var rawMemHeight string
	if memHeightProp != nil {
		rawMemHeight, _ = memHeightProp["value"].(string)
	}
	memHeights := decodeMemoryHeights(rawMemHeight)

	var currentHeight any
	if p := props["height_display"]; p != nil {
		currentHeight = p["value"]
	}

	var currentHeightCM any
	if h, ok := currentHeight.(float64); ok {
		currentHeightCM = h / 10
	}

	var memLoc any
	if p := props["memory_location"]; p != nil {
		memLoc = p["value"]
	}

	deviceCM := make(map[string]float64, len(memHeights))
	for k, v := range memHeights {
		deviceCM[k] = float64(v) / 10
	}

	raw := map[string]any{}
	if memHeightProp != nil {
		raw["memory_height"] = memHeightProp["value"]
		raw["memory_height_dp"] = memHeightProp["dp_id"]
	}
	if p := props["memory_location"]; p != nil {
		raw["memory_location_dp"] = p["dp_id"]
	}

	return map[string]any{
		"configured":        cfg.Presets,
		"device":            memHeights,
		"device_cm":         deviceCM,
		"current_height_mm": currentHeight,
		"current_height_cm": currentHeightCM,
		"memory_location":   memLoc,
		"raw":               raw,
	}, nil
}

// CurrentHeightMM extracts the current height in mm from a status map.
// path is "lan" or "cloud".
func CurrentHeightMM(status map[string]any, cfg *config.Config, path string) (int, bool) {
	var raw any
	if path == "lan" {
		dpNum := cfg.LANDPs["height_display"]
		// LAN status dps are keyed by string DP number or nested under "dps".
		dps, hasDPS := status["dps"].(map[string]any)
		if hasDPS {
			raw = dps[fmt.Sprintf("%d", dpNum)]
		} else {
			raw = status[fmt.Sprintf("%d", dpNum)]
			if raw == nil {
				raw = status["height_display"]
			}
		}
	} else {
		raw = status["height_display"]
	}

	if raw == nil {
		return 0, false
	}

	switch v := raw.(type) {
	case float64:
		return int(v), true
	case int:
		return v, true
	case int64:
		return int(v), true
	default:
		return 0, false
	}
}

// decodeMemoryHeights decodes the base64-encoded memory slot heights.
// Format: 8 bytes, each pair of bytes is one memory slot height in mm (big-endian uint16).
func decodeMemoryHeights(raw string) map[string]int {
	if raw == "" {
		return map[string]int{}
	}
	b, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		return map[string]int{}
	}
	heights := make(map[string]int, 4)
	for i := 0; i+1 < len(b) && i < 8; i += 2 {
		slot := i/2 + 1
		heights[fmt.Sprintf("mem%d", slot)] = int(binary.BigEndian.Uint16(b[i : i+2]))
	}
	return heights
}
