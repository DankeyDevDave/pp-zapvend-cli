package cmd

import (
	"fmt"
	"os"
	"strconv"

	"github.com/DankeyDevDave/pp-zapvend-cli/internal/runner"
	"github.com/DankeyDevDave/pp-zapvend-cli/internal/tariff"
	"github.com/spf13/cobra"
)

var (
	vendDemo      bool
	vendForceReal bool
	vendSlow      bool
)

var vendCmd = &cobra.Command{
	Use:   "vend <who> <rands>",
	Short: "Generate an electricity token",
	Long: `Generate an STS electricity token for a meter.

<who> can be a tenant name (partial match) or meter number.
<rands> is the amount in South African Rand.

The CLI looks up the meter in config.yaml, reads the street tariff, calculates
the kWh equivalent (matching Zapvend's billing engine), and passes the result
to the CyberVendIT automator.

Examples:
  pp-zapvend-cli vend anita 100
  pp-zapvend-cli vend "Van Passel Main" 200 --demo
  pp-zapvend-cli vend 07123456790 150 --json`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		who := args[0]
		rands, err := strconv.ParseFloat(args[1], 64)
		if err != nil {
			return fmt.Errorf("invalid amount %q: must be a number", args[1])
		}
		if rands <= 0 {
			return fmt.Errorf("amount must be greater than 0")
		}

		meter, err := cfg.FindMeter(who)
		if err != nil {
			return err
		}

		monthlyKWh := cfg.CurrentMonthlyKWh(meter.Street, meter.MeterNumber)

		calc, err := tariff.Calculate(rands, meter.Street.Tariffs, meter.Street.VendingFee, monthlyKWh)
		if err != nil {
			return fmt.Errorf("rate calculation: %w", err)
		}

		demo := vendDemo || os.Getenv("CYBERVENDIT_DEMO_MODE") == "true"
		if vendForceReal {
			demo = false
		}

		opts := runner.Options{
			ScriptPath: cfg.ScriptPath,
			Street:     meter.StreetKey,
			DemoMode:   demo,
			ForceReal:  vendForceReal,
			Fast:       !vendSlow,
			Visible:    visible,
			LogLevel:   logLevel,
		}

		result, err := runner.GenerateToken(meter.MeterNumber, calc.KWh, opts)
		if err != nil {
			if jsonOut {
				printJSON(map[string]any{"success": false, "error": err.Error()})
			} else {
				fmt.Fprintln(os.Stderr, "error:", err)
			}
			os.Exit(1)
		}

		if jsonOut {
			printJSON(map[string]any{
				"success":        result.Success,
				"token":          result.Token,
				"meter":          meter.MeterNumber,
				"name":           meter.User.Name,
				"street":         meter.StreetKey,
				"rands":          rands,
				"kwh":            calc.KWh,
				"vending_fee":    calc.VendingFee,
				"vat_amount":     calc.VATAmount,
				"tier_breakdown": calc.TierBreakdown,
				"demo":           result.Demo,
				"error":          result.Error,
			})
			return nil
		}

		if !result.Success {
			fmt.Fprintln(os.Stderr, "generation failed:", result.Error)
			os.Exit(1)
		}

		if result.Demo {
			fmt.Printf("Demo token:  %s\n", result.Token)
		} else {
			fmt.Printf("Token:  %s\n", result.Token)
		}
		fmt.Printf("Meter:  %s (%s)\n", meter.MeterNumber, meter.User.Name)
		fmt.Printf("Street: %s\n", meter.Street.Name)
		fmt.Printf("Amount: R%.2f → %.4f kWh\n", rands, calc.KWh)
		if calc.VendingFee > 0 {
			fmt.Printf("        (R%.2f vending fee deducted)\n", calc.VendingFee)
		}
		for _, tier := range calc.TierBreakdown {
			fmt.Printf("        Tier %d: %.4f kWh @ R%.4f/kWh = R%.2f\n",
				tier.Tier, tier.KWh, tier.Rate, tier.Amount)
		}
		if result.Demo {
			fmt.Println("(demo mode — transaction NOT completed)")
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(vendCmd)
	vendCmd.Flags().BoolVar(&vendDemo, "demo", false, "Demo mode: stop before confirming the transaction")
	vendCmd.Flags().BoolVar(&vendForceReal, "force-real", false, "Force real transaction even if demo env var is set")
	vendCmd.Flags().BoolVar(&vendSlow, "slow", false, "Sync balance and logout after vending (slower)")
}
