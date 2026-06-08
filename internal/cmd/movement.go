package cmd

import (
	"github.com/spf13/cobra"
	"github.com/yashiels/orro-cli/internal/desk"
)

var upCmd = &cobra.Command{
	Use:   "up",
	Short: "Move desk up continuously (send 'orro stop' to halt)",
	Args:  cobra.NoArgs,
	RunE:  makeMovement("up"),
}

var downCmd = &cobra.Command{
	Use:   "down",
	Short: "Move desk down continuously (send 'orro stop' to halt)",
	Args:  cobra.NoArgs,
	RunE:  makeMovement("down"),
}

var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop any in-progress desk movement",
	Args:  cobra.NoArgs,
	RunE:  makeMovement("stop"),
}

var sitCmd = &cobra.Command{
	Use:   "sit",
	Short: "Recall the configured sit preset (default: mem1)",
	Args:  cobra.NoArgs,
	RunE:  makeMovement("sit"),
}

var standCmd = &cobra.Command{
	Use:   "stand",
	Short: "Recall the configured stand preset (default: mem3)",
	Args:  cobra.NoArgs,
	RunE:  makeMovement("stand"),
}

var gotoCmd = &cobra.Command{
	Use:       "goto <slot>",
	Short:     "Recall a specific memory slot (mem1–mem4)",
	Args:      cobra.ExactArgs(1),
	ValidArgs: []string{"mem1", "mem2", "mem3", "mem4"},
	RunE:      runGoto,
}

func init() {
	rootCmd.AddCommand(upCmd, downCmd, stopCmd, sitCmd, standCmd, gotoCmd)
}

func makeMovement(action string) func(*cobra.Command, []string) error {
	return func(_ *cobra.Command, _ []string) error {
		cfg := loadConfig(true)
		commands, err := desk.MovementCommands(action, cfg, "")
		if err != nil {
			die("%v", err)
		}
		result, err := desk.LanThenCloud(cfg, commands, flagCloud)
		if err != nil {
			die("%v", err)
		}
		printAndExit(result)
		return nil
	}
}

func runGoto(_ *cobra.Command, args []string) error {
	slot := args[0]
	cfg := loadConfig(true)
	commands, err := desk.MovementCommands("goto", cfg, slot)
	if err != nil {
		die("%v", err)
	}
	result, err := desk.LanThenCloud(cfg, commands, flagCloud)
	if err != nil {
		die("%v", err)
	}
	printAndExit(result)
	return nil
}
