package asc

import "strings"

// formatPlatforms converts a slice of Platform to a comma-separated string.
func formatPlatforms(platforms []Platform) string {
	strs := make([]string, len(platforms))
	for i, p := range platforms {
		strs[i] = string(p)
	}
	return strings.Join(strs, ", ")
}

func appCategoriesRows(resp *AppCategoriesResponse) ([]string, [][]string) {
	headers := []string{"ID", "Platforms"}
	rows := make([][]string, 0, len(resp.Data))
	for _, cat := range resp.Data {
		rows = append(rows, []string{cat.ID, formatPlatforms(cat.Attributes.Platforms)})
	}
	return headers, rows
}

func printAppCategoriesTable(resp *AppCategoriesResponse) error {
	h, r := appCategoriesRows(resp)
	RenderTable(h, r)
	return nil
}

func printAppCategoriesMarkdown(resp *AppCategoriesResponse) error {
	h, r := appCategoriesRows(resp)
	RenderMarkdown(h, r)
	return nil
}
