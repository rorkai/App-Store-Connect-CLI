package asc

import (
	"fmt"
	"os"
)

// PassTypeIDDeleteResult represents CLI output for pass type ID deletions.
type PassTypeIDDeleteResult struct {
	ID      string `json:"id"`
	Deleted bool   `json:"deleted"`
}

func printPassTypeIDsTable(resp *PassTypeIDsResponse) error {
	headers := []string{"ID", "Name", "Identifier"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{
			item.ID,
			compactWhitespace(item.Attributes.Name),
			item.Attributes.Identifier,
		})
	}
	RenderTable(headers, rows)
	return nil
}

func printPassTypeIDsMarkdown(resp *PassTypeIDsResponse) error {
	fmt.Fprintln(os.Stdout, "| ID | Name | Identifier |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- |")
	for _, item := range resp.Data {
		fmt.Fprintf(os.Stdout, "| %s | %s | %s |\n",
			item.ID,
			escapeMarkdown(item.Attributes.Name),
			escapeMarkdown(item.Attributes.Identifier),
		)
	}
	return nil
}

func printPassTypeIDDeleteResultTable(result *PassTypeIDDeleteResult) error {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	RenderTable(headers, rows)
	return nil
}

func printPassTypeIDDeleteResultMarkdown(result *PassTypeIDDeleteResult) error {
	fmt.Fprintln(os.Stdout, "| ID | Deleted |")
	fmt.Fprintln(os.Stdout, "| --- | --- |")
	fmt.Fprintf(os.Stdout, "| %s | %t |\n",
		escapeMarkdown(result.ID),
		result.Deleted,
	)
	return nil
}
