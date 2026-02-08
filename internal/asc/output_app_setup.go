package asc

import (
	"fmt"
	"os"
)

// AppSetupInfoResult represents CLI output for app-setup info updates.
type AppSetupInfoResult struct {
	AppID               string                       `json:"appId"`
	App                 *AppResponse                 `json:"app,omitempty"`
	AppInfoLocalization *AppInfoLocalizationResponse `json:"appInfoLocalization,omitempty"`
}

func printAppSetupInfoResultTable(result *AppSetupInfoResult) error {
	headers := []string{"Resource", "ID", "Locale", "Name", "Subtitle", "Bundle ID", "Primary Locale", "Privacy Policy URL"}
	var rows [][]string
	if result.App != nil {
		attrs := result.App.Data.Attributes
		rows = append(rows, []string{"app", result.App.Data.ID, "", "", "", attrs.BundleID, attrs.PrimaryLocale, ""})
	}
	if result.AppInfoLocalization != nil {
		attrs := result.AppInfoLocalization.Data.Attributes
		rows = append(rows, []string{
			"appInfoLocalization",
			result.AppInfoLocalization.Data.ID,
			attrs.Locale,
			compactWhitespace(attrs.Name),
			compactWhitespace(attrs.Subtitle),
			"",
			"",
			attrs.PrivacyPolicyURL,
		})
	}
	RenderTable(headers, rows)
	return nil
}

func printAppSetupInfoResultMarkdown(result *AppSetupInfoResult) error {
	fmt.Fprintln(os.Stdout, "| Resource | ID | Locale | Name | Subtitle | Bundle ID | Primary Locale | Privacy Policy URL |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- | --- | --- | --- | --- | --- |")
	if result.App != nil {
		attrs := result.App.Data.Attributes
		fmt.Fprintf(
			os.Stdout,
			"| app | %s |  |  |  | %s | %s |  |\n",
			escapeMarkdown(result.App.Data.ID),
			escapeMarkdown(attrs.BundleID),
			escapeMarkdown(attrs.PrimaryLocale),
		)
	}
	if result.AppInfoLocalization != nil {
		attrs := result.AppInfoLocalization.Data.Attributes
		fmt.Fprintf(
			os.Stdout,
			"| appInfoLocalization | %s | %s | %s | %s |  |  | %s |\n",
			escapeMarkdown(result.AppInfoLocalization.Data.ID),
			escapeMarkdown(attrs.Locale),
			escapeMarkdown(attrs.Name),
			escapeMarkdown(attrs.Subtitle),
			escapeMarkdown(attrs.PrivacyPolicyURL),
		)
	}
	return nil
}
