package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/yashiels/orro-cli/internal/config"
	"github.com/yashiels/orro-cli/internal/output"
)

const configTemplate = `# orro config — ~/.config/orro/config.toml
# Uncomment and fill in your values.

endpoint = "https://openapi.tuyaeu.com"
access_id = "your_access_id"
access_secret = "your_access_secret"
device_id = "your_device_id"
# local_key = "your_local_key"
# lan_ip = "192.168.10.95"
# lan_version = "3.4"

[presets]
sit = "mem1"
stand = "mem3"
`

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage orro configuration",
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Print current resolved config (secrets redacted)",
	Args:  cobra.NoArgs,
	RunE:  runConfigShow,
}

var configPathCmd = &cobra.Command{
	Use:   "path",
	Short: "Print the config file path in use",
	Args:  cobra.NoArgs,
	RunE:  runConfigPath,
}

var configSetCmd = &cobra.Command{
	Use:   "set",
	Short: "Write individual config values",
	Args:  cobra.NoArgs,
	RunE:  runConfigSet,
}

var configInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Interactive first-time setup (prints template if not a TTY)",
	Args:  cobra.NoArgs,
	RunE:  runConfigInit,
}

// config set flags
var (
	setEndpoint     string
	setAccessID     string
	setAccessSecret string
	setDeviceID     string
	setLocalKey     string
	setLANIP        string
	setLANVersion   string
)

func init() {
	rootCmd.AddCommand(configCmd)
	configCmd.AddCommand(configShowCmd, configPathCmd, configSetCmd, configInitCmd)

	configSetCmd.Flags().StringVar(&setEndpoint, "endpoint", "", "Tuya API endpoint URL")
	configSetCmd.Flags().StringVar(&setAccessID, "access-id", "", "Tuya Access ID")
	configSetCmd.Flags().StringVar(&setAccessSecret, "access-secret", "", "Tuya Access Secret")
	configSetCmd.Flags().StringVar(&setDeviceID, "device-id", "", "Tuya Device ID")
	configSetCmd.Flags().StringVar(&setLocalKey, "local-key", "", "device local encryption key")
	configSetCmd.Flags().StringVar(&setLANIP, "lan-ip", "", "device LAN IP address")
	configSetCmd.Flags().StringVar(&setLANVersion, "lan-version", "", "Tuya LAN protocol version")
}

func runConfigShow(_ *cobra.Command, _ []string) error {
	cfg, err := config.Load(config.LoadOptions{
		ConfigPath:         flagConfig,
		Verbose:            flagVerbose,
		Quiet:              flagQuiet,
		RequireCredentials: false,
	})
	if err != nil {
		return err
	}
	output.Print(cfg.ToMap(true), outFormat())
	return nil
}

func runConfigPath(_ *cobra.Command, _ []string) error {
	path := flagConfig
	if path == "" {
		path = config.DefaultConfigPath()
	}
	fmt.Println(path)
	return nil
}

func runConfigSet(_ *cobra.Command, _ []string) error {
	values := map[string]any{}
	if setEndpoint != "" {
		values["endpoint"] = setEndpoint
	}
	if setAccessID != "" {
		values["access_id"] = setAccessID
	}
	if setAccessSecret != "" {
		values["access_secret"] = setAccessSecret
	}
	if setDeviceID != "" {
		values["device_id"] = setDeviceID
	}
	if setLocalKey != "" {
		values["local_key"] = setLocalKey
	}
	if setLANIP != "" {
		values["lan_ip"] = setLANIP
	}
	if setLANVersion != "" {
		values["lan_version"] = setLANVersion
	}
	if len(values) == 0 {
		return cmdError("no values specified — use flags like --endpoint, --device-id, etc.")
	}

	path, err := config.WriteConfigFile(values, flagConfig)
	if err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	fmt.Fprintf(os.Stderr, "wrote %s\n", path)
	return nil
}

func runConfigInit(_ *cobra.Command, _ []string) error {
	if !isTTY() {
		fmt.Print(configTemplate)
		return nil
	}
	return interactiveConfigInit()
}

func interactiveConfigInit() error {
	fmt.Println("orro config init — interactive setup")
	fmt.Println()

	type prompt struct {
		key          string
		label        string
		defaultValue string
	}
	prompts := []prompt{
		{"endpoint", "Tuya API endpoint", "https://openapi.tuyaeu.com"},
		{"access_id", "Access ID", ""},
		{"access_secret", "Access Secret", ""},
		{"device_id", "Device ID", ""},
		{"local_key", "Local Key (optional, Enter to skip)", ""},
		{"lan_ip", "LAN IP (optional, Enter to skip)", ""},
		{"lan_version", "LAN protocol version (optional)", "3.4"},
	}

	reader := bufio.NewReader(os.Stdin)
	values := map[string]any{}

	for _, p := range prompts {
		suffix := ""
		if p.defaultValue != "" {
			suffix = fmt.Sprintf(" [%s]", p.defaultValue)
		}
		fmt.Printf("  %s%s: ", p.label, suffix)
		answer, _ := reader.ReadString('\n')
		answer = strings.TrimSpace(answer)
		if answer != "" {
			values[p.key] = answer
		} else if p.defaultValue != "" {
			values[p.key] = p.defaultValue
		}
	}

	path, err := config.WriteConfigFile(values, flagConfig)
	if err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	fmt.Printf("config written to %s\n", path)
	return nil
}

func isTTY() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}
