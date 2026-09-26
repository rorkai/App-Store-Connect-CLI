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
	Locale         string            `json:"locale"`
	Action         string            `json:"action"`
	Status         string            `json:"status"`
	LocalizationID string            `json:"localizationId,omitempty"`
	Error          string            `json:"error,omitempty"`
	DesiredValues  map[string]string `json:"desiredValues,omitempty"`
}

// LocalizationUploadResult represents CLI output for localization uploads.
type LocalizationUploadResult struct {
	Type                 string                           `json:"type"`
	VersionID            string                           `json:"versionId,omitempty"`
	AppID                string                           `json:"appId,omitempty"`
	AppInfoID            string                           `json:"appInfoId,omitempty"`
	DryRun               bool                             `json:"dryRun"`
	InputPath            string                           `json:"inputPath,omitempty"`
	Total                int                              `json:"total"`
	Succeeded            int                              `json:"succeeded"`
	Failed               int                              `json:"failed"`
	FailureArtifactPath  string                           `json:"failureArtifactPath,omitempty"`
	FailureArtifactError string                           `json:"failureArtifactError,omitempty"`
	Results              []LocalizationUploadLocaleResult `json:"results"`
}

// AppInfoSetLocaleResult represents a per-locale app-info set operation result.
type AppInfoSetLocaleResult struct {
	Locale         string `json:"locale"`
	Action         string `json:"action"`
	Status         string `json:"status"`
	LocalizationID string `json:"localizationId,omitempty"`
	Error          string `json:"error,omitempty"`
}

// AppInfoSetBatchResult represents CLI output for batched app-info set operations.
type AppInfoSetBatchResult struct {
	AppID     string                   `json:"appId,omitempty"`
	VersionID string                   `json:"versionId,omitempty"`
	DryRun    bool                     `json:"dryRun"`
	Total     int                      `json:"total"`
	Succeeded int                      `json:"succeeded"`
	Failed    int                      `json:"failed"`
	Planned   int                      `json:"planned,omitempty"`
	Results   []AppInfoSetLocaleResult `json:"results"`
}

func appStoreVersionLocalizationsRows(resp *AppStoreVersionLocalizationsResponse) ([]string, [][]string) {
	headers := []string{"ID", "Locale", "Whats New", "Keywords"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{
			item.ID,
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
	headers := []string{"ID", "Locale", "Name", "Subtitle", "Privacy Policy URL"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{
			item.ID,
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
	headers := []string{"Locale", "Action", "Status", "Localization ID", "Error"}
	rows := make([][]string, 0, len(result.Results))
	for _, item := range result.Results {
		rows = append(rows, []string{
			item.Locale,
			item.Action,
			item.Status,
			item.LocalizationID,
			compactWhitespace(item.Error),
		})
	}
	return headers, rows
}

func appInfoSetBatchResultRows(result *AppInfoSetBatchResult) ([]string, [][]string) {
	headers := []string{"Locale", "Action", "Status", "Localization ID", "Error"}
	rows := make([][]string, 0, len(result.Results))
	for _, item := range result.Results {
		rows = append(rows, []string{
			item.Locale,
			item.Action,
			item.Status,
			item.LocalizationID,
			compactWhitespace(item.Error),
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
