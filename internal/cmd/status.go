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
	printResult(result)
	return nil
}
