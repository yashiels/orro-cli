package cmd

import (
	"github.com/spf13/cobra"

	"github.com/yashiels/orro-cli/internal/desk"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show desk height, connection path, and protocol in use",
	Args:  cobra.NoArgs,
	RunE:  runStatus,
}

func init() {
	rootCmd.AddCommand(statusCmd)
}

func runStatus(_ *cobra.Command, _ []string) error {
	cfg, err := loadConfig(true)
	if err != nil {
		return err
	}
	result, err := desk.GetStatus(cfg, flagCloud)
	if err != nil {
		return err
	}

	// Decode and label the desk height consistently for both LAN and cloud
	// paths so the output is the same regardless of transport.
	desk.AnnotateHeight(result, cfg)

	printResult(result)
	return nil
}
