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

func localizationDownloadResultRows(result *LocalizationDownloadResult) ([]string, [][]string) {
	headers := []string{"Locale", "Path"}
	rows := make([][]string, 0, len(result.Files))
	for _, file := range result.Files {
		rows = append(rows, []string{file.Locale, file.Path})
	}
	return headers, rows
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

func appStoreVersionLocalizationDeleteResultRows(result *AppStoreVersionLocalizationDeleteResult) ([]string, [][]string) {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	return headers, rows
}

func betaAppLocalizationDeleteResultRows(result *BetaAppLocalizationDeleteResult) ([]string, [][]string) {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	return headers, rows
}

func betaBuildLocalizationDeleteResultRows(result *BetaBuildLocalizationDeleteResult) ([]string, [][]string) {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	return headers, rows
}
