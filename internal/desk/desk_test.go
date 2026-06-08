package desk_test

import (
	"testing"

	"github.com/yashiels/orro-cli/internal/config"
	"github.com/yashiels/orro-cli/internal/desk"
)

func TestMovementCommands(t *testing.T) {
	cfg := &config.Config{
		Presets: map[string]string{
			"sit":   "mem1",
			"stand": "mem3",
		},
	}

	tests := []struct {
		action  string
		target  string
		wantLen int
		wantErr bool
	}{
		{"up", "", 3, false},
		{"down", "", 3, false},
		{"stop", "", 2, false},
		{"sit", "", 2, false},
		{"stand", "", 2, false},
		{"goto", "mem2", 2, false},
		{"goto", "", 0, true},
		{"unknown", "", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.action+"_"+tt.target, func(t *testing.T) {
			cmds, err := desk.MovementCommands(tt.action, cfg, tt.target)
			if (err != nil) != tt.wantErr {
				t.Errorf("MovementCommands(%q) error = %v, wantErr = %v", tt.action, err, tt.wantErr)
			}
			if !tt.wantErr && len(cmds) != tt.wantLen {
				t.Errorf("MovementCommands(%q) len = %d, want %d", tt.action, len(cmds), tt.wantLen)
			}
		})
	}
}

func TestMovementCommandsUpStructure(t *testing.T) {
	cfg := &config.Config{Presets: config.DefaultPresets}
	cmds, err := desk.MovementCommands("up", cfg, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cmds[0].Code != "child_lock" {
		t.Errorf("first command should be child_lock wake, got %q", cmds[0].Code)
	}
	if cmds[0].Value != false {
		t.Errorf("child_lock wake should set to false, got %v", cmds[0].Value)
	}
	if cmds[2].Code != "move_up" || cmds[2].Value != true {
		t.Errorf("last command should be move_up=true, got %+v", cmds[2])
	}
}

func TestMovementCommandsStopNoWake(t *testing.T) {
	cfg := &config.Config{Presets: config.DefaultPresets}
	cmds, err := desk.MovementCommands("stop", cfg, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Stop should NOT send child_lock wake — just halt both directions.
	for _, c := range cmds {
		if c.Code == "child_lock" {
			t.Error("stop should not include child_lock wake command")
		}
	}
}

func TestCurrentHeightMMFromCloud(t *testing.T) {
	cfg := &config.Config{
		LANDPs: config.DefaultLANDPs,
	}
	status := map[string]any{
		"height_display": float64(1100),
	}
	h, ok := desk.CurrentHeightMM(status, cfg, "cloud")
	if !ok {
		t.Fatal("expected height to be found")
	}
	if h != 1100 {
		t.Errorf("height = %d, want 1100", h)
	}
}

func TestCurrentHeightMMFromLAN(t *testing.T) {
	cfg := &config.Config{
		LANDPs: map[string]int{
			"height_display": 152,
		},
	}
	// LAN status DPs are keyed by string DP number, nested under "dps".
	status := map[string]any{
		"dps": map[string]any{
			"152": float64(950),
		},
	}
	h, ok := desk.CurrentHeightMM(status, cfg, "lan")
	if !ok {
		t.Fatal("expected height to be found in LAN status")
	}
	if h != 950 {
		t.Errorf("height = %d, want 950", h)
	}
}

func TestCurrentHeightMMNotPresent(t *testing.T) {
	cfg := &config.Config{LANDPs: config.DefaultLANDPs}
	status := map[string]any{
		"child_lock": false,
	}
	_, ok := desk.CurrentHeightMM(status, cfg, "cloud")
	if ok {
		t.Error("expected height not found, but got ok=true")
	}
}
