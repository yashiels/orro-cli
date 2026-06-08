package desk

import (
	"fmt"
	"time"

	"github.com/yashiels/orro-cli/internal/config"
)

const (
	toleranceMM   = 5
	pollInterval  = 500 * time.Millisecond
	heightTimeout = 30 * time.Second
)

// GoToHeight drives the desk to a target height in cm, polling until it arrives.
func GoToHeight(cfg *config.Config, cm float64, forceCloud bool) (map[string]any, error) {
	targetMM := int(cm * 10)

	// Get initial status to determine current height and path.
	statusPayload, err := GetStatus(cfg, forceCloud)
	if err != nil {
		return nil, fmt.Errorf("height: initial status: %w", err)
	}

	path := statusPayload["path"].(string)
	statusMap := extractStatus(statusPayload)
	current, ok := CurrentHeightMM(statusMap, cfg, path)
	if !ok {
		return nil, fmt.Errorf(
			"height_display is not available for this device — use presets or up/down/stop instead",
		)
	}

	if abs(current-targetMM) <= toleranceMM {
		return map[string]any{
			"path":      path,
			"target_mm": targetMM,
			"height_mm": current,
			"target_cm": cm,
			"height_cm": float64(current) / 10,
			"result":    "already_at_target",
		}, nil
	}

	direction := "up"
	if targetMM < current {
		direction = "down"
	}

	moveCommands, err := MovementCommands(direction, cfg, "")
	if err != nil {
		return nil, err
	}

	useCloud := forceCloud || path == "cloud"
	if _, err := LanThenCloud(cfg, moveCommands, useCloud); err != nil {
		return nil, fmt.Errorf("height: start movement: %w", err)
	}

	stopCommands, err := MovementCommands("stop", cfg, "")
	if err != nil {
		return nil, err
	}

	last := current
	started := time.Now()

	defer func() {
		// Always send stop, even if we error out.
		_, _ = LanThenCloud(cfg, stopCommands, useCloud)
	}()

	for time.Since(started) < heightTimeout {
		time.Sleep(pollInterval)

		sp, err := GetStatus(cfg, useCloud)
		if err != nil {
			cfg.Debug(fmt.Sprintf("height: poll error: %v", err))
			continue
		}
		sm := extractStatus(sp)
		newPath := sp["path"].(string)
		if h, ok := CurrentHeightMM(sm, cfg, newPath); ok {
			last = h
		}

		if abs(last-targetMM) <= toleranceMM {
			break
		}
		if direction == "up" && last > targetMM {
			break
		}
		if direction == "down" && last < targetMM {
			break
		}
	}

	return map[string]any{
		"path":      path,
		"target_mm": targetMM,
		"height_mm": last,
		"target_cm": cm,
		"height_cm": float64(last) / 10,
		"result":    "stopped",
	}, nil
}

// extractStatus pulls the inner status map from a GetStatus result.
func extractStatus(payload map[string]any) map[string]any {
	if s, ok := payload["status"].(map[string]any); ok {
		return s
	}
	return payload
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
