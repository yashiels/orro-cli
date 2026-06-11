package cmd

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/yashiels/orro-cli/internal/desk"
)

var heightCmd = &cobra.Command{
	Use:   "height <cm>",
	Short: "Drive the desk to an exact height in centimetres, then stop",
	Args:  cobra.ExactArgs(1),
	RunE:  runHeight,
}

func init() {
	rootCmd.AddCommand(heightCmd)
}

func runHeight(_ *cobra.Command, args []string) error {
	cm, err := strconv.ParseFloat(args[0], 64)
	if err != nil {
		die("invalid height %q: %v", args[0], err)
	}
	if cm <= 0 || cm > 300 {
		die("height must be between 0 and 300 cm, got %.1f", cm)
	}

	cfg := loadConfig(true)
	cfg.Info(fmt.Sprintf("driving to %.1f cm…", cm))

	result, err := desk.GoToHeight(cfg, cm, flagCloud)
	if err != nil {
		die("%v", err)
	}
	printAndExit(result)
	return nil
}
