package cmd

import (
	"fmt"
	"sort"

	"github.com/DankeyDevDave/pp-zapvend-cli/internal/tariff"
	"github.com/spf13/cobra"
)

var metersCmd = &cobra.Command{
	Use:   "meters",
	Short: "List all meters from config.yaml",
	Long:  `List all meters, their names, street, meter number, and tariff.`,
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		meters := cfg.AllMeters()
		sort.Slice(meters, func(i, j int) bool {
			if meters[i].StreetKey != meters[j].StreetKey {
				return meters[i].StreetKey < meters[j].StreetKey
			}
			return meters[i].User.Name < meters[j].User.Name
		})

		if jsonOut {
			type row struct {
				Name        string `json:"name"`
				Meter       string `json:"meter"`
				StreetKey   string `json:"street_key"`
				StreetName  string `json:"street_name"`
				MeterType   string `json:"meter_type,omitempty"`
				RateDesc    string `json:"rate"`
				VendingFee  float64 `json:"vending_fee"`
			}
			var rows []row
			for _, m := range meters {
				rows = append(rows, row{
					Name:       m.User.Name,
					Meter:      m.MeterNumber,
					StreetKey:  m.StreetKey,
					StreetName: m.Street.Name,
					MeterType:  m.User.MeterType,
					RateDesc:   tariff.RateDescription(m.Street.Tariffs),
					VendingFee: m.Street.VendingFee,
				})
			}
			printJSON(rows)
			return nil
		}

		currentStreet := ""
		for _, m := range meters {
			if m.StreetKey != currentStreet {
				currentStreet = m.StreetKey
				fmt.Printf("\n%s (%s)\n", m.Street.Name, m.StreetKey)
				fmt.Printf("  Rate: %s\n", tariff.RateDescription(m.Street.Tariffs))
				if m.Street.VendingFee > 0 {
					fmt.Printf("  Vending fee: R%.2f\n", m.Street.VendingFee)
				}
			}
			mtype := m.User.MeterType
			if mtype == "" {
				mtype = "ELECTRICITY"
			}
			fmt.Printf("  %-30s  %s  [%s]\n", m.User.Name, m.MeterNumber, mtype)
		}
		fmt.Println()
		return nil
	},
}

func init() {
	rootCmd.AddCommand(metersCmd)
}
