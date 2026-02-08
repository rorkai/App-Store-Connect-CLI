package asc

import "fmt"

func appTagsRows(resp *AppTagsResponse) ([]string, [][]string) {
	headers := []string{"ID", "Name", "Visible In App Store"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{
			item.ID,
			compactWhitespace(item.Attributes.Name),
			fmt.Sprintf("%t", item.Attributes.VisibleInAppStore),
		})
	}
	return headers, rows
}
