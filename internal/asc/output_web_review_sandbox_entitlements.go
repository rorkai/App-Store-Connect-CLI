package asc

import "fmt"

// WebReviewSandboxEntitlementResult is the receipt for one verified App
// Sandbox Information mutation on an App Store version's App Review details.
type WebReviewSandboxEntitlementResult struct {
	VersionID      string `json:"versionId"`
	ReviewDetailID string `json:"reviewDetailId"`
	Entitlement    string `json:"entitlement"`
	UsageID        string `json:"usageId,omitempty"`
	Justification  string `json:"justification,omitempty"`
	Changed        bool   `json:"changed"`
	Status         string `json:"status"`
}

func webReviewSandboxEntitlementResultRows(result *WebReviewSandboxEntitlementResult) ([]string, [][]string) {
	headers := []string{"Field", "Value"}
	if result == nil {
		return headers, nil
	}
	rows := [][]string{
		{"Version ID", result.VersionID},
		{"Review Detail ID", result.ReviewDetailID},
		{"Entitlement", result.Entitlement},
	}
	if result.UsageID != "" {
		rows = append(rows, []string{"Usage ID", result.UsageID})
	}
	if result.Justification != "" {
		rows = append(rows, []string{"Justification", result.Justification})
	}
	rows = append(rows,
		[]string{"Changed", fmt.Sprintf("%t", result.Changed)},
		[]string{"Status", result.Status},
	)
	return headers, rows
}

// WebReviewSandboxEntitlementsApplyResult is the receipt for App Sandbox
// Information rows applied from a file to one App Store version, each verified
// with a fresh read.
type WebReviewSandboxEntitlementsApplyResult struct {
	VersionID      string                                `json:"versionId"`
	ReviewDetailID string                                `json:"reviewDetailId"`
	Changed        bool                                  `json:"changed"`
	Rows           []WebReviewSandboxEntitlementApplyRow `json:"rows"`
}

// WebReviewSandboxEntitlementApplyRow is one row of an applied file: created,
// updated, or unchanged.
type WebReviewSandboxEntitlementApplyRow struct {
	Entitlement   string `json:"entitlement"`
	UsageID       string `json:"usageId"`
	Justification string `json:"justification"`
	Status        string `json:"status"`
}

func webReviewSandboxEntitlementsApplyResultRows(result *WebReviewSandboxEntitlementsApplyResult) ([]string, [][]string) {
	headers := []string{"Entitlement", "Status", "Usage ID", "Justification"}
	if result == nil {
		return headers, nil
	}
	rows := make([][]string, 0, len(result.Rows))
	for _, row := range result.Rows {
		rows = append(rows, []string{row.Entitlement, row.Status, row.UsageID, row.Justification})
	}
	return headers, rows
}
