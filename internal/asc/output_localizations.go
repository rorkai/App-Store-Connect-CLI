package asc

import (
	"fmt"
	"os"
)

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

func printAppStoreVersionLocalizationsTable(resp *AppStoreVersionLocalizationsResponse) error {
	headers := []string{"Locale", "Whats New", "Keywords"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{
			item.Attributes.Locale,
			compactWhitespace(item.Attributes.WhatsNew),
			compactWhitespace(item.Attributes.Keywords),
		})
	}
	RenderTable(headers, rows)
	return nil
}

func printBetaAppLocalizationsTable(resp *BetaAppLocalizationsResponse) error {
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
	RenderTable(headers, rows)
	return nil
}

func printBetaBuildLocalizationsTable(resp *BetaBuildLocalizationsResponse) error {
	headers := []string{"Locale", "What to Test"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{
			item.Attributes.Locale,
			compactWhitespace(item.Attributes.WhatsNew),
		})
	}
	RenderTable(headers, rows)
	return nil
}

func printAppInfoLocalizationsTable(resp *AppInfoLocalizationsResponse) error {
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
	RenderTable(headers, rows)
	return nil
}

func printAppStoreVersionLocalizationsMarkdown(resp *AppStoreVersionLocalizationsResponse) error {
	fmt.Fprintln(os.Stdout, "| Locale | Whats New | Keywords |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- |")
	for _, item := range resp.Data {
		fmt.Fprintf(os.Stdout, "| %s | %s | %s |\n",
			escapeMarkdown(item.Attributes.Locale),
			escapeMarkdown(item.Attributes.WhatsNew),
			escapeMarkdown(item.Attributes.Keywords),
		)
	}
	return nil
}

func printBetaAppLocalizationsMarkdown(resp *BetaAppLocalizationsResponse) error {
	fmt.Fprintln(os.Stdout, "| Locale | Description | Feedback Email | Marketing URL | Privacy Policy URL | TVOS Privacy Policy |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- | --- | --- | --- |")
	for _, item := range resp.Data {
		fmt.Fprintf(os.Stdout, "| %s | %s | %s | %s | %s | %s |\n",
			escapeMarkdown(item.Attributes.Locale),
			escapeMarkdown(item.Attributes.Description),
			escapeMarkdown(item.Attributes.FeedbackEmail),
			escapeMarkdown(item.Attributes.MarketingURL),
			escapeMarkdown(item.Attributes.PrivacyPolicyURL),
			escapeMarkdown(item.Attributes.TvOsPrivacyPolicy),
		)
	}
	return nil
}

func printBetaBuildLocalizationsMarkdown(resp *BetaBuildLocalizationsResponse) error {
	fmt.Fprintln(os.Stdout, "| Locale | What to Test |")
	fmt.Fprintln(os.Stdout, "| --- | --- |")
	for _, item := range resp.Data {
		fmt.Fprintf(os.Stdout, "| %s | %s |\n",
			escapeMarkdown(item.Attributes.Locale),
			escapeMarkdown(item.Attributes.WhatsNew),
		)
	}
	return nil
}

func printAppInfoLocalizationsMarkdown(resp *AppInfoLocalizationsResponse) error {
	fmt.Fprintln(os.Stdout, "| Locale | Name | Subtitle | Privacy Policy URL |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- | --- |")
	for _, item := range resp.Data {
		fmt.Fprintf(os.Stdout, "| %s | %s | %s | %s |\n",
			escapeMarkdown(item.Attributes.Locale),
			escapeMarkdown(item.Attributes.Name),
			escapeMarkdown(item.Attributes.Subtitle),
			escapeMarkdown(item.Attributes.PrivacyPolicyURL),
		)
	}
	return nil
}

func printLocalizationDownloadResultTable(result *LocalizationDownloadResult) error {
	headers := []string{"Locale", "Path"}
	rows := make([][]string, 0, len(result.Files))
	for _, file := range result.Files {
		rows = append(rows, []string{file.Locale, file.Path})
	}
	RenderTable(headers, rows)
	return nil
}

func printLocalizationDownloadResultMarkdown(result *LocalizationDownloadResult) error {
	fmt.Fprintln(os.Stdout, "| Locale | Path |")
	fmt.Fprintln(os.Stdout, "| --- | --- |")
	for _, file := range result.Files {
		fmt.Fprintf(os.Stdout, "| %s | %s |\n",
			escapeMarkdown(file.Locale),
			escapeMarkdown(file.Path),
		)
	}
	return nil
}

func printLocalizationUploadResultTable(result *LocalizationUploadResult) error {
	headers := []string{"Locale", "Action", "Localization ID"}
	rows := make([][]string, 0, len(result.Results))
	for _, item := range result.Results {
		rows = append(rows, []string{
			item.Locale,
			item.Action,
			item.LocalizationID,
		})
	}
	RenderTable(headers, rows)
	return nil
}

func printLocalizationUploadResultMarkdown(result *LocalizationUploadResult) error {
	fmt.Fprintln(os.Stdout, "| Locale | Action | Localization ID |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- |")
	for _, item := range result.Results {
		fmt.Fprintf(os.Stdout, "| %s | %s | %s |\n",
			escapeMarkdown(item.Locale),
			escapeMarkdown(item.Action),
			escapeMarkdown(item.LocalizationID),
		)
	}
	return nil
}

func printAppStoreVersionLocalizationDeleteResultTable(result *AppStoreVersionLocalizationDeleteResult) error {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	RenderTable(headers, rows)
	return nil
}

func printAppStoreVersionLocalizationDeleteResultMarkdown(result *AppStoreVersionLocalizationDeleteResult) error {
	fmt.Fprintln(os.Stdout, "| ID | Deleted |")
	fmt.Fprintln(os.Stdout, "| --- | --- |")
	fmt.Fprintf(os.Stdout, "| %s | %t |\n",
		escapeMarkdown(result.ID),
		result.Deleted,
	)
	return nil
}

func printBetaAppLocalizationDeleteResultTable(result *BetaAppLocalizationDeleteResult) error {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	RenderTable(headers, rows)
	return nil
}

func printBetaAppLocalizationDeleteResultMarkdown(result *BetaAppLocalizationDeleteResult) error {
	fmt.Fprintln(os.Stdout, "| ID | Deleted |")
	fmt.Fprintln(os.Stdout, "| --- | --- |")
	fmt.Fprintf(os.Stdout, "| %s | %t |\n",
		escapeMarkdown(result.ID),
		result.Deleted,
	)
	return nil
}

func printBetaBuildLocalizationDeleteResultTable(result *BetaBuildLocalizationDeleteResult) error {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	RenderTable(headers, rows)
	return nil
}

func printBetaBuildLocalizationDeleteResultMarkdown(result *BetaBuildLocalizationDeleteResult) error {
	fmt.Fprintln(os.Stdout, "| ID | Deleted |")
	fmt.Fprintln(os.Stdout, "| --- | --- |")
	fmt.Fprintf(os.Stdout, "| %s | %t |\n",
		escapeMarkdown(result.ID),
		result.Deleted,
	)
	return nil
}
