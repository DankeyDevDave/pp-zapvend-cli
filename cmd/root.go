package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/DankeyDevDave/pp-zapvend-cli/internal/config"
	"github.com/spf13/cobra"
)

var (
	jsonOut  bool
	visible  bool
	logLevel string
	cfg      *config.Config
)

var rootCmd = &cobra.Command{
	Use:   "pp-zapvend-cli",
	Short: "Zapvend-native electricity token CLI",
	Long: `pp-zapvend-cli — generate electricity tokens and check balances via Zapvend.

Reads rates, meters, and account credentials from Zapvend's config.yaml so you
never need to calculate kWh manually. Pass a Rand amount and the CLI resolves
the correct kWh, account, and meter automatically.`,
	SilenceUsage:  true,
	SilenceErrors: true,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if cmd.Name() == "help" {
			return nil
		}
		var err error
		cfg, err = loadConfig()
		return err
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.CompletionOptions.HiddenDefaultCmd = true
	rootCmd.PersistentFlags().BoolVar(&jsonOut, "json", false, "Output as JSON")
	rootCmd.PersistentFlags().BoolVar(&visible, "visible", false, "Show browser window")
	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "", "Log level: DEBUG, INFO, WARNING, ERROR")
}

func loadConfig() (*config.Config, error) {
	path, err := config.Detect()
	if err != nil {
		return nil, err
	}
	return config.Load(path)
}

func printJSON(v any) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}

func exitErr(err error) {
	if jsonOut {
		printJSON(map[string]any{"success": false, "error": err.Error()})
	} else {
		fmt.Fprintln(os.Stderr, "error:", err)
	}
	os.Exit(1)
}
