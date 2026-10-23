package asc

import "fmt"

// SubscriptionPricingPPPIndex identifies the embedded purchasing-power index.
type SubscriptionPricingPPPIndex struct {
	Source  string `json:"source"`
	License string `json:"license"`
	AsOf    string `json:"asOf"`
}

// SubscriptionPricingPPPResult is the stable computed output contract for
// subscriptions pricing ppp. Each row's sourcePrice is Apple's equalized price
// for the base price, and requestedMultiple is the territory's price level
// relative to the base territory.
type SubscriptionPricingPPPResult struct {
	SubscriptionID    string                                `json:"subscriptionId"`
	BaseTerritory     string                                `json:"baseTerritory"`
	BasePrice         string                                `json:"basePrice"`
	BasePricePointID  string                                `json:"basePricePointId"`
	Index             SubscriptionPricingPPPIndex           `json:"index"`
	Rounding          string                                `json:"rounding"`
	StartDate         string                                `json:"startDate,omitempty"`
	AutoScheduled     bool                                  `json:"autoScheduled,omitempty"`
	Preserved         bool                                  `json:"preserved"`
	SubscriptionState string                                `json:"subscriptionState,omitempty"`
	DryRun            bool                                  `json:"dryRun"`
	Summary           SubscriptionPricingDeriveSummary      `json:"summary"`
	Rows              []SubscriptionPricingDeriveRow        `json:"rows"`
	Verification      SubscriptionPricingDeriveVerification `json:"verification"`
}

func subscriptionPricingPPPSummaryRows(result *SubscriptionPricingPPPResult) ([]string, [][]string) {
	return []string{
		"Subscription", "Base Territory", "Base Price", "Index As Of", "Rounding",
		"Dry Run", "Start Date", "Total", "Planned", "Noop", "Skipped", "Applied",
		"Verified", "Unresolved", "Failed",
	}, [][]string{{
		result.SubscriptionID,
		result.BaseTerritory,
		result.BasePrice,
		result.Index.AsOf,
		result.Rounding,
		fmt.Sprintf("%t", result.DryRun),
		result.StartDate,
		fmt.Sprintf("%d", result.Summary.Total),
		fmt.Sprintf("%d", result.Summary.Planned),
		fmt.Sprintf("%d", result.Summary.Noop),
		fmt.Sprintf("%d", result.Summary.Skipped),
		fmt.Sprintf("%d", result.Summary.Applied),
		fmt.Sprintf("%d", result.Summary.Verified),
		fmt.Sprintf("%d", result.Summary.Unresolved),
		fmt.Sprintf("%d", result.Summary.Failed),
	}}
}

func subscriptionPricingPPPRowRows(result *SubscriptionPricingPPPResult) ([]string, [][]string) {
	headers := []string{
		"Territory", "Currency", "Equalized", "PPP Multiple", "Desired",
		"Current", "Selected", "Achieved Multiple", "Action", "Status", "Error",
	}
	rows := make([][]string, 0, len(result.Rows))
	for _, row := range result.Rows {
		rows = append(rows, []string{
			row.Territory,
			row.Currency,
			row.SourcePrice,
			row.RequestedMultiple,
			row.DesiredPrice,
			row.CurrentTargetPrice,
			row.TargetPrice,
			row.AchievedMultiple,
			row.Action,
			row.Status,
			row.Error,
		})
	}
	return headers, rows
}
