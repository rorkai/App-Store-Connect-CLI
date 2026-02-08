package asc

import (
	"fmt"
	"strings"
)

// formatPlatforms converts a slice of Platform to a comma-separated string.
func formatPlatforms(platforms []Platform) string {
	strs := make([]string, len(platforms))
	for i, p := range platforms {
		strs[i] = string(p)
	}
	return strings.Join(strs, ", ")
}

func printAppCategoriesMarkdown(resp *AppCategoriesResponse) error {
	fmt.Println("| ID | Platforms |")
	fmt.Println("|---|---|")
	for _, cat := range resp.Data {
		fmt.Printf("| %s | %s |\n", cat.ID, formatPlatforms(cat.Attributes.Platforms))
	}
	return nil
}

func printAppCategoriesTable(resp *AppCategoriesResponse) error {
	headers := []string{"ID", "PLATFORMS"}
	rows := make([][]string, 0, len(resp.Data))
	for _, cat := range resp.Data {
		rows = append(rows, []string{cat.ID, formatPlatforms(cat.Attributes.Platforms)})
	}
	RenderTable(headers, rows)
	return nil
}
