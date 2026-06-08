// Package cmd implements all orro CLI commands using Cobra.
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/yashiels/orro-cli/internal/config"
	"github.com/yashiels/orro-cli/internal/output"
)

var (
	flagCloud   bool
	flagOutput  string
	flagJSON    bool
	flagVerbose bool
	flagQuiet   bool
	flagConfig  string
)

var rootCmd = &cobra.Command{
	Use:   "orro",
	Short: "Control your Tuya-based standing desk from the terminal",
	Long: `orro — Control your Tuya-based standing desk from the terminal.

LAN-first control via the Tuya protocol with automatic fallback to the
Tuya Cloud API when the desk is unreachable over the local network.`,
	Version:       config.Version,
	SilenceErrors: true,
	SilenceUsage:  true,
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.PersistentFlags().BoolVar(&flagCloud, "cloud", false,
		"force Tuya Cloud API instead of LAN-first")
	rootCmd.PersistentFlags().StringVar(&flagOutput, "output", "table",
		"output format: table (default) or json")
	rootCmd.PersistentFlags().BoolVar(&flagJSON, "json", false,
		"shorthand for --output json")
	rootCmd.PersistentFlags().BoolVarP(&flagVerbose, "verbose", "v", false,
		"print debug information to stderr")
	rootCmd.PersistentFlags().BoolVarP(&flagQuiet, "quiet", "q", false,
		"suppress info banners")
	rootCmd.PersistentFlags().StringVar(&flagConfig, "config", "",
		"path to config file (default: ~/.config/orro/config.toml)")
}

// outFormat returns the resolved output format (json or table).
func outFormat() string {
	if flagJSON {
		return "json"
	}
	return flagOutput
}

// loadConfig loads and validates configuration, exiting on fatal errors.
func loadConfig(requireCreds bool) *config.Config {
	cfg, err := config.Load(config.LoadOptions{
		ConfigPath:         flagConfig,
		Verbose:            flagVerbose,
		Quiet:              flagQuiet,
		RequireCredentials: requireCreds,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "orro: %v\n", err)
		os.Exit(3)
	}
	return cfg
}

// printAndExit prints the payload in the configured format and exits 0.
func printAndExit(payload any) {
	output.Print(payload, outFormat())
	os.Exit(0)
}

// die prints an error to stderr and exits 1.
func die(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "orro: "+format+"\n", args...)
	os.Exit(1)
}
