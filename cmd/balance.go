package cmd

import (
	"fmt"
	"os"

	"github.com/DankeyDevDave/pp-zapvend-cli/internal/runner"
	"github.com/spf13/cobra"
)

var balanceCmd = &cobra.Command{
	Use:   "balance [<who>]",
	Short: "Check remaining CyberVendIT token balance",
	Long: `Check the remaining token balance for a CyberVendIT account.

<who> can be a tenant name, meter number, or street key (e.g. van_passel).
If omitted, checks the primary account.

Examples:
  pp-zapvend-cli balance
  pp-zapvend-cli balance anita
  pp-zapvend-cli balance van_passel
  pp-zapvend-cli balance --json`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		streetKey := "primary"

		if len(args) == 1 {
			who := args[0]
			// Check if it's a known street key directly
			if _, ok := cfg.Streets[who]; ok {
				streetKey = who
			} else {
				// Try meter/name lookup and derive street
				meter, err := cfg.FindMeter(who)
				if err != nil {
					return err
				}
				streetKey = meter.StreetKey
			}
		}

		opts := runner.Options{
			ScriptPath: cfg.ScriptPath,
			Visible:    visible,
			LogLevel:   logLevel,
		}

		result, err := runner.CheckBalance(streetKey, opts)
		if err != nil {
			if jsonOut {
				printJSON(map[string]any{"success": false, "error": err.Error()})
			} else {
				fmt.Fprintln(os.Stderr, "error:", err)
			}
			os.Exit(1)
		}

		if jsonOut {
			printJSON(result)
			return nil
		}

		if result.Success {
			fmt.Printf("Remaining tokens: %d\n", result.RemainingTokens)
		} else {
			fmt.Fprintln(os.Stderr, "balance check failed:", result.Error)
			os.Exit(1)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(balanceCmd)
}
