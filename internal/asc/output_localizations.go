package asc

import "fmt"

// AppStoreVersionLocalizationDeleteResult represents CLI output for localization deletions.
type AppStoreVersionLocalizationDeleteResult struct {
	ID      string `json:"id"`
	Deleted bool   `json:"deleted"`
}

// BetaBuildLocalizationDeleteResult represents CLI output for beta build localization deletions.
type BetaBuildLocalizationDeleteResult struct {
	ID      string `json:"id"`
	Deleted bool   `json:"deleted"`
}

// BetaAppLocalizationDeleteResult represents CLI output for beta app localization deletions.
type BetaAppLocalizationDeleteResult struct {
	ID      string `json:"id"`
	Deleted bool   `json:"deleted"`
}

// LocalizationFileResult represents a localization file written or read.
type LocalizationFileResult struct {
	Locale string `json:"locale"`
	Path   string `json:"path"`
}

// LocalizationDownloadResult represents CLI output for localization downloads.
type LocalizationDownloadResult struct {
	Type       string                   `json:"type"`
	VersionID  string                   `json:"versionId,omitempty"`
	AppID      string                   `json:"appId,omitempty"`
	AppInfoID  string                   `json:"appInfoId,omitempty"`
	OutputPath string                   `json:"outputPath"`
	Files      []LocalizationFileResult `json:"files"`
}

// LocalizationUploadLocaleResult represents a per-locale upload result.
type LocalizationUploadLocaleResult struct {
	Locale         string `json:"locale"`
	Action         string `json:"action"`
	LocalizationID string `json:"localizationId,omitempty"`
}

// LocalizationUploadResult represents CLI output for localization uploads.
type LocalizationUploadResult struct {
	Type      string                           `json:"type"`
	VersionID string                           `json:"versionId,omitempty"`
	AppID     string                           `json:"appId,omitempty"`
	AppInfoID string                           `json:"appInfoId,omitempty"`
	DryRun    bool                             `json:"dryRun"`
	Results   []LocalizationUploadLocaleResult `json:"results"`
}

func appStoreVersionLocalizationsRows(resp *AppStoreVersionLocalizationsResponse) ([]string, [][]string) {
	headers := []string{"Locale", "Whats New", "Keywords"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{
			item.Attributes.Locale,
			compactWhitespace(item.Attributes.WhatsNew),
			compactWhitespace(item.Attributes.Keywords),
		})
	}
	return headers, rows
}

func printAppStoreVersionLocalizationsTable(resp *AppStoreVersionLocalizationsResponse) error {
	h, r := appStoreVersionLocalizationsRows(resp)
	RenderTable(h, r)
	return nil
}

func printAppStoreVersionLocalizationsMarkdown(resp *AppStoreVersionLocalizationsResponse) error {
	h, r := appStoreVersionLocalizationsRows(resp)
	RenderMarkdown(h, r)
	return nil
}

func betaAppLocalizationsRows(resp *BetaAppLocalizationsResponse) ([]string, [][]string) {
	headers := []string{"Locale", "Description", "Feedback Email", "Marketing URL", "Privacy Policy URL", "TVOS Privacy Policy"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{
			item.Attributes.Locale,
			compactWhitespace(item.Attributes.Description),
			item.Attributes.FeedbackEmail,
			item.Attributes.MarketingURL,
			item.Attributes.PrivacyPolicyURL,
			item.Attributes.TvOsPrivacyPolicy,
		})
	}
	return headers, rows
}

func printBetaAppLocalizationsTable(resp *BetaAppLocalizationsResponse) error {
	h, r := betaAppLocalizationsRows(resp)
	RenderTable(h, r)
	return nil
}

func printBetaAppLocalizationsMarkdown(resp *BetaAppLocalizationsResponse) error {
	h, r := betaAppLocalizationsRows(resp)
	RenderMarkdown(h, r)
	return nil
}

func betaBuildLocalizationsRows(resp *BetaBuildLocalizationsResponse) ([]string, [][]string) {
	headers := []string{"Locale", "What to Test"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{
			item.Attributes.Locale,
			compactWhitespace(item.Attributes.WhatsNew),
		})
	}
	return headers, rows
}

func printBetaBuildLocalizationsTable(resp *BetaBuildLocalizationsResponse) error {
	h, r := betaBuildLocalizationsRows(resp)
	RenderTable(h, r)
	return nil
}

func printBetaBuildLocalizationsMarkdown(resp *BetaBuildLocalizationsResponse) error {
	h, r := betaBuildLocalizationsRows(resp)
	RenderMarkdown(h, r)
	return nil
}

func appInfoLocalizationsRows(resp *AppInfoLocalizationsResponse) ([]string, [][]string) {
	headers := []string{"Locale", "Name", "Subtitle", "Privacy Policy URL"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{
			item.Attributes.Locale,
			compactWhitespace(item.Attributes.Name),
			compactWhitespace(item.Attributes.Subtitle),
			item.Attributes.PrivacyPolicyURL,
		})
	}
	return headers, rows
}

func printAppInfoLocalizationsTable(resp *AppInfoLocalizationsResponse) error {
	h, r := appInfoLocalizationsRows(resp)
	RenderTable(h, r)
	return nil
}

func printAppInfoLocalizationsMarkdown(resp *AppInfoLocalizationsResponse) error {
	h, r := appInfoLocalizationsRows(resp)
	RenderMarkdown(h, r)
	return nil
}

func localizationDownloadResultRows(result *LocalizationDownloadResult) ([]string, [][]string) {
	headers := []string{"Locale", "Path"}
	rows := make([][]string, 0, len(result.Files))
	for _, file := range result.Files {
		rows = append(rows, []string{file.Locale, file.Path})
	}
	return headers, rows
}

func printLocalizationDownloadResultTable(result *LocalizationDownloadResult) error {
	h, r := localizationDownloadResultRows(result)
	RenderTable(h, r)
	return nil
}

func printLocalizationDownloadResultMarkdown(result *LocalizationDownloadResult) error {
	h, r := localizationDownloadResultRows(result)
	RenderMarkdown(h, r)
	return nil
}

func localizationUploadResultRows(result *LocalizationUploadResult) ([]string, [][]string) {
	headers := []string{"Locale", "Action", "Localization ID"}
	rows := make([][]string, 0, len(result.Results))
	for _, item := range result.Results {
		rows = append(rows, []string{
			item.Locale,
			item.Action,
			item.LocalizationID,
		})
	}
	return headers, rows
}

func printLocalizationUploadResultTable(result *LocalizationUploadResult) error {
	h, r := localizationUploadResultRows(result)
	RenderTable(h, r)
	return nil
}

func printLocalizationUploadResultMarkdown(result *LocalizationUploadResult) error {
	h, r := localizationUploadResultRows(result)
	RenderMarkdown(h, r)
	return nil
}

func appStoreVersionLocalizationDeleteResultRows(result *AppStoreVersionLocalizationDeleteResult) ([]string, [][]string) {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	return headers, rows
}

func printAppStoreVersionLocalizationDeleteResultTable(result *AppStoreVersionLocalizationDeleteResult) error {
	h, r := appStoreVersionLocalizationDeleteResultRows(result)
	RenderTable(h, r)
	return nil
}

func printAppStoreVersionLocalizationDeleteResultMarkdown(result *AppStoreVersionLocalizationDeleteResult) error {
	h, r := appStoreVersionLocalizationDeleteResultRows(result)
	RenderMarkdown(h, r)
	return nil
}

func betaAppLocalizationDeleteResultRows(result *BetaAppLocalizationDeleteResult) ([]string, [][]string) {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	return headers, rows
}

func printBetaAppLocalizationDeleteResultTable(result *BetaAppLocalizationDeleteResult) error {
	h, r := betaAppLocalizationDeleteResultRows(result)
	RenderTable(h, r)
	return nil
}

func printBetaAppLocalizationDeleteResultMarkdown(result *BetaAppLocalizationDeleteResult) error {
	h, r := betaAppLocalizationDeleteResultRows(result)
	RenderMarkdown(h, r)
	return nil
}

func betaBuildLocalizationDeleteResultRows(result *BetaBuildLocalizationDeleteResult) ([]string, [][]string) {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	return headers, rows
}

func printBetaBuildLocalizationDeleteResultTable(result *BetaBuildLocalizationDeleteResult) error {
	h, r := betaBuildLocalizationDeleteResultRows(result)
	RenderTable(h, r)
	return nil
}

func printBetaBuildLocalizationDeleteResultMarkdown(result *BetaBuildLocalizationDeleteResult) error {
	h, r := betaBuildLocalizationDeleteResultRows(result)
	RenderMarkdown(h, r)
	return nil
}
