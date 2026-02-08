package asc

import "fmt"

func appInfosRows(resp *AppInfosResponse) ([]string, [][]string) {
	headers := []string{"ID", "App Store State", "State", "Age Rating", "Kids Age Band"}
	rows := make([][]string, 0, len(resp.Data))
	for _, info := range resp.Data {
		attrs := info.Attributes
		rows = append(rows, []string{
			info.ID,
			appInfoAttrString(attrs, "appStoreState"),
			appInfoAttrString(attrs, "state"),
			appInfoAttrString(attrs, "appStoreAgeRating"),
			appInfoAttrString(attrs, "kidsAgeBand"),
		})
	}
	return headers, rows
}

func printAppInfosTable(resp *AppInfosResponse) error {
	h, r := appInfosRows(resp)
	RenderTable(h, r)
	return nil
}

func printAppInfosMarkdown(resp *AppInfosResponse) error {
	h, r := appInfosRows(resp)
	RenderMarkdown(h, r)
	return nil
}

func appInfoAttrString(attrs AppInfoAttributes, key string) string {
	if attrs == nil {
		return ""
	}
	value, ok := attrs[key]
	if !ok || value == nil {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return typed
	default:
		return fmt.Sprintf("%v", typed)
	}
}
