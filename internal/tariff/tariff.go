package tariff

import (
	"fmt"
	"math"

	"github.com/DankeyDevDave/pp-zapvend-cli/internal/config"
)

type TierEntry struct {
	Tier   int     `json:"tier"`
	KWh    float64 `json:"kwh"`
	Rate   float64 `json:"rate"`
	Amount float64 `json:"amount"`
}

type Result struct {
	KWh           float64     `json:"kwh"`
	VendingFee    float64     `json:"vending_fee"`
	VATAmount     float64     `json:"vat_amount"`
	ElecAmount    float64     `json:"elec_amount"`
	TierBreakdown []TierEntry `json:"tier_breakdown,omitempty"`
}

// Calculate converts a Rand amount to kWh using the given tariff, exactly
// matching the logic in api/core/electricity.py::calculate_purchase.
func Calculate(rands float64, t config.TariffConfig, vendingFee float64, currentMonthlyKWh float64) (Result, error) {
	elecAmount := rands - vendingFee
	if elecAmount <= 0 {
		return Result{}, fmt.Errorf("amount too low (vending fee is R%.2f)", vendingFee)
	}

	vatMult := 1.0 + t.VATRate

	var kwh float64
	var tiers []TierEntry

	switch {
	case t.FlatRate > 0:
		inclusiveRate := t.FlatRate * vatMult
		kwh = elecAmount / inclusiveRate

	case t.Tier1Rate > 0:
		if t.Tier2Rate <= 0 {
			return Result{}, fmt.Errorf("tier 2 rate not configured")
		}
		tier1IncRate := t.Tier1Rate * vatMult
		tier2IncRate := t.Tier2Rate * vatMult
		remainingTier1 := math.Max(0, t.Tier1Limit-currentMonthlyKWh)

		if remainingTier1 > 0 {
			tier1Cost := remainingTier1 * tier1IncRate
			if elecAmount <= tier1Cost {
				kwh = elecAmount / tier1IncRate
				tiers = []TierEntry{{Tier: 1, KWh: kwh, Rate: tier1IncRate, Amount: elecAmount}}
			} else {
				tier1Kwh := remainingTier1
				remaining := elecAmount - tier1Cost
				tier2Kwh := remaining / tier2IncRate
				kwh = tier1Kwh + tier2Kwh
				tiers = []TierEntry{
					{Tier: 1, KWh: tier1Kwh, Rate: tier1IncRate, Amount: tier1Cost},
					{Tier: 2, KWh: tier2Kwh, Rate: tier2IncRate, Amount: remaining},
				}
			}
		} else {
			kwh = elecAmount / tier2IncRate
			tiers = []TierEntry{{Tier: 2, KWh: kwh, Rate: tier2IncRate, Amount: elecAmount}}
		}

	default:
		return Result{}, fmt.Errorf("no valid rate configured for this street")
	}

	vatAmount := elecAmount - (elecAmount / vatMult)
	return Result{
		KWh:           kwh,
		VendingFee:    vendingFee,
		VATAmount:     vatAmount,
		ElecAmount:    elecAmount,
		TierBreakdown: tiers,
	}, nil
}

// RateDescription returns a human-readable description of the tariff.
func RateDescription(t config.TariffConfig) string {
	if t.FlatRate > 0 {
		return fmt.Sprintf("R%.4f/kWh flat (excl. %.0f%% VAT)", t.FlatRate, t.VATRate*100)
	}
	if t.Tier1Rate > 0 {
		return fmt.Sprintf("tiered: R%.4f/kWh (tier 1, first %.0f kWh), R%.4f/kWh (tier 2) excl. %.0f%% VAT",
			t.Tier1Rate, t.Tier1Limit, t.Tier2Rate, t.VATRate*100)
	}
	return "unknown"
}
