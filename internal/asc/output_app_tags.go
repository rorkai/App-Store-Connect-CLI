package asc

import (
	"fmt"
	"os"
)

func printAppTagsTable(resp *AppTagsResponse) error {
	headers := []string{"ID", "Name", "Visible In App Store"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{
			item.ID,
			compactWhitespace(item.Attributes.Name),
			fmt.Sprintf("%t", item.Attributes.VisibleInAppStore),
		})
	}
	RenderTable(headers, rows)
	return nil
}

func printAppTagsMarkdown(resp *AppTagsResponse) error {
	fmt.Fprintln(os.Stdout, "| ID | Name | Visible In App Store |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- |")
	for _, item := range resp.Data {
		fmt.Fprintf(os.Stdout, "| %s | %s | %t |\n",
			escapeMarkdown(item.ID),
			escapeMarkdown(item.Attributes.Name),
			item.Attributes.VisibleInAppStore,
		)
	}
	return nil
}
