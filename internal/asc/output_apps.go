package asc

// AppRenameResult represents a localized app-name change.
type AppRenameResult struct {
	AppID          string `json:"appId"`
	AppInfoID      string `json:"appInfoId"`
	Locale         string `json:"locale"`
	Name           string `json:"name"`
	Action         string `json:"action"`
	LocalizationID string `json:"localizationId"`
}

func appRenameResultRows(result *AppRenameResult) ([]string, [][]string) {
	return []string{"App ID", "App Info ID", "Locale", "Name", "Action", "Localization ID"}, [][]string{{
		result.AppID,
		result.AppInfoID,
		result.Locale,
		compactWhitespace(result.Name),
		result.Action,
		result.LocalizationID,
	}}
}

func appsRows(resp *AppsResponse) ([]string, [][]string) {
	headers := []string{"ID", "Name", "Bundle ID", "SKU"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{
			item.ID,
			compactWhitespace(item.Attributes.Name),
			item.Attributes.BundleID,
			item.Attributes.SKU,
		})
	}
	return headers, rows
}
