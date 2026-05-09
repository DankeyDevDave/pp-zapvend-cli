package cmd

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/DankeyDevDave/pp-zapvend-cli/internal/config"
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

When ZAPVEND_API_URL is set, the CLI routes through the Zapvend API so the
transaction is recorded in the database and notifications are sent. Otherwise
it calls the automator directly (token generated but not recorded in DB).

Examples:
  pp-zapvend-cli vend anita 100
  pp-zapvend-cli vend "Van Passel Main" 200 --demo
  pp-zapvend-cli vend 07138134767 150 --json

Set ZAPVEND_API_URL=http://localhost:8001 to enable DB recording.`,
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

		demo := vendDemo || os.Getenv("CYBERVENDIT_DEMO_MODE") == "true"
		if vendForceReal {
			demo = false
		}

		// Try the Zapvend API first — it handles DB recording and notifications.
		apiURL := config.APIURL()
		if apiURL != "" {
			return vendViaAPI(apiURL, meter, rands, demo)
		}

		// Fallback: call automator directly (no DB recording).
		fmt.Fprintln(os.Stderr, "note: ZAPVEND_API_URL not set — transaction will not be recorded in DB")
		return vendDirect(meter, rands, demo)
	},
}

func vendViaAPI(apiURL string, meter *config.MeterInfo, rands float64, demo bool) error {
	result, err := runner.GenerateTokenViaAPI(apiURL, meter.MeterNumber, rands, demo, !vendSlow)
	if err != nil {
		// If the API is simply not running, fall back gracefully.
		if isConnRefused(err) {
			fmt.Fprintf(os.Stderr, "note: Zapvend API at %s is not running — falling back to direct mode (not recorded in DB)\n", apiURL)
			return vendDirect(meter, rands, demo)
		}
		if jsonOut {
			printJSON(map[string]any{"success": false, "error": err.Error()})
			os.Exit(1)
		}
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}

	// The API returns kWh already calculated; show a local calculation for the
	// breakdown display (rates come from config.yaml).
	monthlyKWh := cfg.CurrentMonthlyKWh(meter.Street, meter.MeterNumber)
	calc, _ := tariff.Calculate(rands, meter.Street.Tariffs, meter.Street.VendingFee, monthlyKWh)

	printVendResult(result, meter, rands, calc)
	return nil
}

func vendDirect(meter *config.MeterInfo, rands float64, demo bool) error {
	monthlyKWh := cfg.CurrentMonthlyKWh(meter.Street, meter.MeterNumber)
	calc, err := tariff.Calculate(rands, meter.Street.Tariffs, meter.Street.VendingFee, monthlyKWh)
	if err != nil {
		return fmt.Errorf("rate calculation: %w", err)
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

	printVendResult(result, meter, rands, calc)
	return nil
}

func printVendResult(result *runner.GenerateTokenResult, meter *config.MeterInfo, rands float64, calc tariff.Result) {
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
		return
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
}

func isConnRefused(err error) bool {
	return strings.Contains(err.Error(), "connection refused") ||
		strings.Contains(err.Error(), "no such host") ||
		strings.Contains(err.Error(), "connect: connection refused")
}

func init() {
	rootCmd.AddCommand(vendCmd)
	vendCmd.Flags().BoolVar(&vendDemo, "demo", false, "Demo mode: stop before confirming the transaction")
	vendCmd.Flags().BoolVar(&vendForceReal, "force-real", false, "Force real transaction even if demo env var is set")
	vendCmd.Flags().BoolVar(&vendSlow, "slow", false, "Sync balance and logout after vending (slower)")
}
