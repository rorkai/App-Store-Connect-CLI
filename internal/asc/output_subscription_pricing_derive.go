package asc

import "fmt"

// SubscriptionPricingDeriveSummary summarizes a source-to-target pricing run.
type SubscriptionPricingDeriveSummary struct {
	Total      int `json:"total"`
	Planned    int `json:"planned"`
	Noop       int `json:"noop"`
	Applied    int `json:"applied,omitempty"`
	Verified   int `json:"verified,omitempty"`
	Unresolved int `json:"unresolved"`
	Failed     int `json:"failed"`
	Skipped    int `json:"skipped,omitempty"`
}

// SubscriptionPricingDeriveRow represents one territory's derived target price.
type SubscriptionPricingDeriveRow struct {
	Territory            string `json:"territory"`
	Currency             string `json:"currency,omitempty"`
	SourcePrice          string `json:"sourcePrice"`
	SourcePricePointID   string `json:"sourcePricePointId"`
	DesiredPrice         string `json:"desiredPrice"`
	CurrentTargetPrice   string `json:"currentTargetPrice,omitempty"`
	CurrentTargetPointID string `json:"currentTargetPricePointId,omitempty"`
	TargetPrice          string `json:"targetPrice,omitempty"`
	TargetPricePointID   string `json:"targetPricePointId,omitempty"`
	RequestedMultiple    string `json:"requestedMultiple"`
	AchievedMultiple     string `json:"achievedMultiple,omitempty"`
	MultipleDelta        string `json:"multipleDelta,omitempty"`
	Action               string `json:"action"`
	Status               string `json:"status"`
	Error                string `json:"error,omitempty"`
}

// SubscriptionPricingDeriveVerification summarizes post-apply readback.
type SubscriptionPricingDeriveVerification struct {
	Status   string `json:"status"`
	Verified int    `json:"verified,omitempty"`
	Failed   int    `json:"failed,omitempty"`
}

// SubscriptionPricingDeriveResult is the stable computed output contract for
// subscriptions pricing derive.
type SubscriptionPricingDeriveResult struct {
	SourceSubscriptionID string                                `json:"sourceSubscriptionId"`
	TargetSubscriptionID string                                `json:"targetSubscriptionId"`
	Multiplier           string                                `json:"multiplier"`
	Rounding             string                                `json:"rounding"`
	StartDate            string                                `json:"startDate,omitempty"`
	AutoScheduled        bool                                  `json:"autoScheduled,omitempty"`
	Preserved            bool                                  `json:"preserved"`
	TargetState          string                                `json:"targetSubscriptionState,omitempty"`
	DryRun               bool                                  `json:"dryRun"`
	Summary              SubscriptionPricingDeriveSummary      `json:"summary"`
	Rows                 []SubscriptionPricingDeriveRow        `json:"rows"`
	Verification         SubscriptionPricingDeriveVerification `json:"verification"`
}

func subscriptionPricingDeriveSummaryRows(result *SubscriptionPricingDeriveResult) ([]string, [][]string) {
	return []string{
		"Source Subscription", "Target Subscription", "Multiplier", "Rounding",
		"Dry Run", "Start Date", "Total", "Planned", "Noop", "Applied",
		"Verified", "Unresolved", "Failed",
	}, [][]string{{
		result.SourceSubscriptionID,
		result.TargetSubscriptionID,
		result.Multiplier,
		result.Rounding,
		fmt.Sprintf("%t", result.DryRun),
		result.StartDate,
		fmt.Sprintf("%d", result.Summary.Total),
		fmt.Sprintf("%d", result.Summary.Planned),
		fmt.Sprintf("%d", result.Summary.Noop),
		fmt.Sprintf("%d", result.Summary.Applied),
		fmt.Sprintf("%d", result.Summary.Verified),
		fmt.Sprintf("%d", result.Summary.Unresolved),
		fmt.Sprintf("%d", result.Summary.Failed),
	}}
}

func subscriptionPricingDeriveRowRows(result *SubscriptionPricingDeriveResult) ([]string, [][]string) {
	headers := []string{
		"Territory", "Currency", "Source", "Desired", "Current Target",
		"Selected Target", "Requested Multiple", "Achieved Multiple", "Action",
		"Status", "Error",
	}
	rows := make([][]string, 0, len(result.Rows))
	for _, row := range result.Rows {
		rows = append(rows, []string{
			row.Territory,
			row.Currency,
			row.SourcePrice,
			row.DesiredPrice,
			row.CurrentTargetPrice,
			row.TargetPrice,
			row.RequestedMultiple,
			row.AchievedMultiple,
			row.Action,
			row.Status,
			row.Error,
		})
	}
	return headers, rows
}
