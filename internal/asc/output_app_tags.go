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

func printAppTagsTable(resp *AppTagsResponse) error {
	h, r := appTagsRows(resp)
	RenderTable(h, r)
	return nil
}

func printAppTagsMarkdown(resp *AppTagsResponse) error {
	h, r := appTagsRows(resp)
	RenderMarkdown(h, r)
	return nil
}
