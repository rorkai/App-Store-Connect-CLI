package asc

import (
	"fmt"
	"os"
	"text/tabwriter"
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
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "Locale\tWhats New\tKeywords")
	for _, item := range resp.Data {
		fmt.Fprintf(w, "%s\t%s\t%s\n",
			item.Attributes.Locale,
			compactWhitespace(item.Attributes.WhatsNew),
			compactWhitespace(item.Attributes.Keywords),
		)
	}
	return w.Flush()
}

func printBetaAppLocalizationsTable(resp *BetaAppLocalizationsResponse) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "Locale\tDescription\tFeedback Email\tMarketing URL\tPrivacy Policy URL\tTVOS Privacy Policy")
	for _, item := range resp.Data {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
			item.Attributes.Locale,
			compactWhitespace(item.Attributes.Description),
			item.Attributes.FeedbackEmail,
			item.Attributes.MarketingURL,
			item.Attributes.PrivacyPolicyURL,
			item.Attributes.TvOsPrivacyPolicy,
		)
	}
	return w.Flush()
}

func printBetaBuildLocalizationsTable(resp *BetaBuildLocalizationsResponse) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "Locale\tWhat to Test")
	for _, item := range resp.Data {
		fmt.Fprintf(w, "%s\t%s\n",
			item.Attributes.Locale,
			compactWhitespace(item.Attributes.WhatsNew),
		)
	}
	return w.Flush()
}

func printAppInfoLocalizationsTable(resp *AppInfoLocalizationsResponse) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "Locale\tName\tSubtitle\tPrivacy Policy URL")
	for _, item := range resp.Data {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
			item.Attributes.Locale,
			compactWhitespace(item.Attributes.Name),
			compactWhitespace(item.Attributes.Subtitle),
			item.Attributes.PrivacyPolicyURL,
		)
	}
	return w.Flush()
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
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "Locale\tPath")
	for _, file := range result.Files {
		fmt.Fprintf(w, "%s\t%s\n", file.Locale, file.Path)
	}
	return w.Flush()
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
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "Locale\tAction\tLocalization ID")
	for _, item := range result.Results {
		fmt.Fprintf(w, "%s\t%s\t%s\n",
			item.Locale,
			item.Action,
			item.LocalizationID,
		)
	}
	return w.Flush()
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
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tDeleted")
	fmt.Fprintf(w, "%s\t%t\n", result.ID, result.Deleted)
	return w.Flush()
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
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tDeleted")
	fmt.Fprintf(w, "%s\t%t\n", result.ID, result.Deleted)
	return w.Flush()
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
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tDeleted")
	fmt.Fprintf(w, "%s\t%t\n", result.ID, result.Deleted)
	return w.Flush()
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
