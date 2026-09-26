package asc

import "fmt"

// InAppPurchaseImportProductResult reports the outcome of one product in an
// `asc iap import` run.
type InAppPurchaseImportProductResult struct {
	ProductID            string `json:"productId"`
	ReferenceName        string `json:"referenceName"`
	Type                 string `json:"type"`
	Status               string `json:"status"`
	InAppPurchaseID      string `json:"inAppPurchaseId,omitempty"`
	VersionID            string `json:"versionId,omitempty"`
	LocalizationsCreated int    `json:"localizationsCreated"`
	ReviewScreenshotID   string `json:"reviewScreenshotId,omitempty"`
	Error                string `json:"error,omitempty"`
}

// InAppPurchaseImportResult is the receipt for `asc iap import`. Created and
// CreatedProductIDs count the in-app purchases that exist in App Store Connect
// after the run, including a product whose localizations or review screenshot
// failed afterwards, so an operator can reconcile without re-reading the input
// file. Per-product Status still reports whether that product finished.
type InAppPurchaseImportResult struct {
	AppID                string                             `json:"appId"`
	File                 string                             `json:"file"`
	DryRun               bool                               `json:"dryRun"`
	SkipExisting         bool                               `json:"skipExisting"`
	Total                int                                `json:"total"`
	Created              int                                `json:"created"`
	Skipped              int                                `json:"skipped"`
	LocalizationsCreated int                                `json:"localizationsCreated"`
	ScreenshotsUploaded  int                                `json:"screenshotsUploaded"`
	CreatedProductIDs    []string                           `json:"createdProductIds"`
	Products             []InAppPurchaseImportProductResult `json:"products"`
	FailedProductID      string                             `json:"failedProductId,omitempty"`
	Error                string                             `json:"error,omitempty"`
}

func inAppPurchaseImportSummaryRows(result *InAppPurchaseImportResult) ([]string, [][]string) {
	headers := []string{"App ID", "File", "Dry Run", "Skip Existing", "Total", "Created", "Skipped", "Localizations", "Screenshots", "Failed Product", "Error"}
	rows := [][]string{{
		SanitizeTerminalText(result.AppID),
		SanitizeTerminalText(result.File),
		fmt.Sprintf("%t", result.DryRun),
		fmt.Sprintf("%t", result.SkipExisting),
		fmt.Sprintf("%d", result.Total),
		fmt.Sprintf("%d", result.Created),
		fmt.Sprintf("%d", result.Skipped),
		fmt.Sprintf("%d", result.LocalizationsCreated),
		fmt.Sprintf("%d", result.ScreenshotsUploaded),
		SanitizeTerminalText(result.FailedProductID),
		compactWhitespace(result.Error),
	}}
	return headers, rows
}

func inAppPurchaseImportProductRows(result *InAppPurchaseImportResult) ([]string, [][]string) {
	headers := []string{"Product ID", "Reference Name", "Type", "Status", "IAP ID", "Version ID", "Localizations", "Screenshot ID", "Error"}
	rows := make([][]string, 0, len(result.Products))
	for _, product := range result.Products {
		rows = append(rows, []string{
			SanitizeTerminalText(product.ProductID),
			compactWhitespace(product.ReferenceName),
			SanitizeTerminalText(product.Type),
			SanitizeTerminalText(product.Status),
			SanitizeTerminalText(product.InAppPurchaseID),
			SanitizeTerminalText(product.VersionID),
			fmt.Sprintf("%d", product.LocalizationsCreated),
			SanitizeTerminalText(product.ReviewScreenshotID),
			compactWhitespace(product.Error),
		})
	}
	return headers, rows
}

func inAppPurchaseImportTables(result *InAppPurchaseImportResult, render func([]string, [][]string)) error {
	summaryHeaders, summaryRows := inAppPurchaseImportSummaryRows(result)
	render(summaryHeaders, summaryRows)
	if len(result.Products) > 0 {
		productHeaders, productRows := inAppPurchaseImportProductRows(result)
		render(productHeaders, productRows)
	}
	return nil
}
