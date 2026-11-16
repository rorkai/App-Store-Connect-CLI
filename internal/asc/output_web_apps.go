package asc

import "fmt"

// WebAppDeleteResult is the mutation and dry-run receipt for `asc web apps delete`.
type WebAppDeleteResult struct {
	AppID    string `json:"appId"`
	Name     string `json:"name,omitempty"`
	BundleID string `json:"bundleId,omitempty"`
	Removed  bool   `json:"removed"`
	DryRun   bool   `json:"dryRun,omitempty"`
}

func webAppDeleteRows(result *WebAppDeleteResult) ([]string, [][]string) {
	if result == nil {
		result = &WebAppDeleteResult{}
	}
	return []string{"App ID", "Name", "Bundle ID", "Removed", "Dry Run"}, [][]string{{
		result.AppID,
		result.Name,
		result.BundleID,
		fmt.Sprintf("%t", result.Removed),
		fmt.Sprintf("%t", result.DryRun),
	}}
}

// WebAgeRatingEditResult identifies a declaration when Iris returns no body.
type WebAgeRatingEditResult struct {
	DeclarationID string `json:"declarationId"`
}

func webAgeRatingEditRows(result *WebAgeRatingEditResult) ([]string, [][]string) {
	return []string{"declaration_id"}, [][]string{{result.DeclarationID}}
}
