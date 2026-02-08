package asc

// AppStoreVersionPromotionCreateResult represents CLI output for promotion creation.
type AppStoreVersionPromotionCreateResult struct {
	PromotionID string `json:"promotionId"`
	VersionID   string `json:"versionId"`
	TreatmentID string `json:"treatmentId,omitempty"`
}

func appStoreVersionPromotionCreateRows(result *AppStoreVersionPromotionCreateResult) ([]string, [][]string) {
	headers := []string{"Promotion ID", "Version ID", "Treatment ID"}
	rows := [][]string{{result.PromotionID, result.VersionID, result.TreatmentID}}
	return headers, rows
}
