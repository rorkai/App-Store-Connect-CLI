package asc

import "fmt"

// MerchantIDDeleteResult represents CLI output for merchant ID deletions.
type MerchantIDDeleteResult struct {
	ID      string `json:"id"`
	Deleted bool   `json:"deleted"`
}

func merchantIDsRows(resp *MerchantIDsResponse) ([]string, [][]string) {
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

func merchantIDDeleteResultRows(result *MerchantIDDeleteResult) ([]string, [][]string) {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	return headers, rows
}
