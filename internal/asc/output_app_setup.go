package asc

// AppSetupInfoResult represents CLI output for app-setup info updates.
type AppSetupInfoResult struct {
	AppID               string                       `json:"appId"`
	App                 *AppResponse                 `json:"app,omitempty"`
	AppInfoLocalization *AppInfoLocalizationResponse `json:"appInfoLocalization,omitempty"`
}

func appSetupInfoResultRows(result *AppSetupInfoResult) ([]string, [][]string) {
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
	return headers, rows
}

func printAppSetupInfoResultTable(result *AppSetupInfoResult) error {
	h, r := appSetupInfoResultRows(result)
	RenderTable(h, r)
	return nil
}

func printAppSetupInfoResultMarkdown(result *AppSetupInfoResult) error {
	h, r := appSetupInfoResultRows(result)
	RenderMarkdown(h, r)
	return nil
}
