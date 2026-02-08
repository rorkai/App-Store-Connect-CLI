package asc

import (
	"fmt"
	"os"
)

func printAppsTable(resp *AppsResponse) error {
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
	RenderTable(headers, rows)
	return nil
}

func printAppsMarkdown(resp *AppsResponse) error {
	fmt.Fprintln(os.Stdout, "| ID | Name | Bundle ID | SKU |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- | --- |")
	for _, item := range resp.Data {
		fmt.Fprintf(os.Stdout, "| %s | %s | %s | %s |\n",
			item.ID,
			escapeMarkdown(item.Attributes.Name),
			escapeMarkdown(item.Attributes.BundleID),
			escapeMarkdown(item.Attributes.SKU),
		)
	}
	return nil
}
