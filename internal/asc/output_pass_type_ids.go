package asc

import "fmt"

// PassTypeIDDeleteResult represents CLI output for pass type ID deletions.
type PassTypeIDDeleteResult struct {
	ID      string `json:"id"`
	Deleted bool   `json:"deleted"`
}

func passTypeIDsRows(resp *PassTypeIDsResponse) ([]string, [][]string) {
	headers := []string{"ID", "Name", "Identifier"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{
			item.ID,
			compactWhitespace(item.Attributes.Name),
			item.Attributes.Identifier,
		})
	}
	return headers, rows
}

func printPassTypeIDsTable(resp *PassTypeIDsResponse) error {
	h, r := passTypeIDsRows(resp)
	RenderTable(h, r)
	return nil
}

func printPassTypeIDsMarkdown(resp *PassTypeIDsResponse) error {
	h, r := passTypeIDsRows(resp)
	RenderMarkdown(h, r)
	return nil
}

func passTypeIDDeleteResultRows(result *PassTypeIDDeleteResult) ([]string, [][]string) {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	return headers, rows
}

func printPassTypeIDDeleteResultTable(result *PassTypeIDDeleteResult) error {
	h, r := passTypeIDDeleteResultRows(result)
	RenderTable(h, r)
	return nil
}

func printPassTypeIDDeleteResultMarkdown(result *PassTypeIDDeleteResult) error {
	h, r := passTypeIDDeleteResultRows(result)
	RenderMarkdown(h, r)
	return nil
}
