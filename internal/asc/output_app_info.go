package asc

import (
	"fmt"
	"os"
)

func printAppInfosTable(resp *AppInfosResponse) error {
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
	RenderTable(headers, rows)
	return nil
}

func printAppInfosMarkdown(resp *AppInfosResponse) error {
	fmt.Fprintln(os.Stdout, "| ID | App Store State | State | Age Rating | Kids Age Band |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- | --- | --- |")
	for _, info := range resp.Data {
		attrs := info.Attributes
		fmt.Fprintf(
			os.Stdout,
			"| %s | %s | %s | %s | %s |\n",
			escapeMarkdown(info.ID),
			escapeMarkdown(appInfoAttrString(attrs, "appStoreState")),
			escapeMarkdown(appInfoAttrString(attrs, "state")),
			escapeMarkdown(appInfoAttrString(attrs, "appStoreAgeRating")),
			escapeMarkdown(appInfoAttrString(attrs, "kidsAgeBand")),
		)
	}
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
