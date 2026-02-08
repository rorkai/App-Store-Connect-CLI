package asc

import (
	"fmt"
	"os"
)

// AppStoreVersionPromotionCreateResult represents CLI output for promotion creation.
type AppStoreVersionPromotionCreateResult struct {
	PromotionID string `json:"promotionId"`
	VersionID   string `json:"versionId"`
	TreatmentID string `json:"treatmentId,omitempty"`
}

func printAppStoreVersionPromotionCreateTable(result *AppStoreVersionPromotionCreateResult) error {
	headers := []string{"Promotion ID", "Version ID", "Treatment ID"}
	rows := [][]string{{result.PromotionID, result.VersionID, result.TreatmentID}}
	RenderTable(headers, rows)
	return nil
}

func printAppStoreVersionPromotionCreateMarkdown(result *AppStoreVersionPromotionCreateResult) error {
	fmt.Fprintln(os.Stdout, "| Promotion ID | Version ID | Treatment ID |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- |")
	fmt.Fprintf(os.Stdout, "| %s | %s | %s |\n",
		escapeMarkdown(result.PromotionID),
		escapeMarkdown(result.VersionID),
		escapeMarkdown(result.TreatmentID),
	)
	return nil
}
