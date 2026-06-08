package cmd

import (
	"github.com/spf13/cobra"
	"github.com/yashiels/orro-cli/internal/desk"
)

var presetsCmd = &cobra.Command{
	Use:   "presets",
	Short: "Show preset mapping and device memory slot heights",
	Args:  cobra.NoArgs,
	RunE:  runPresets,
}

func init() {
	rootCmd.AddCommand(presetsCmd)
}

func runPresets(_ *cobra.Command, _ []string) error {
	cfg := loadConfig(true)
	result, err := desk.GetPresets(cfg)
	if err != nil {
		die("%v", err)
	}
	printAndExit(result)
	return nil
}
