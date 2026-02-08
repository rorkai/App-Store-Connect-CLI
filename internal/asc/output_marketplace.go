package asc

import (
	"fmt"
)

func marketplaceSearchDetailsRows(resp *MarketplaceSearchDetailsResponse) ([]string, [][]string) {
	headers := []string{"ID", "Catalog URL"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{
			item.ID,
			compactWhitespace(item.Attributes.CatalogURL),
		})
	}
	return headers, rows
}

func marketplaceWebhooksRows(resp *MarketplaceWebhooksResponse) ([]string, [][]string) {
	headers := []string{"ID", "Endpoint URL"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{
			item.ID,
			compactWhitespace(item.Attributes.EndpointURL),
		})
	}
	return headers, rows
}

func marketplaceSearchDetailDeleteResultRows(result *MarketplaceSearchDetailDeleteResult) ([]string, [][]string) {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	return headers, rows
}

func marketplaceWebhookDeleteResultRows(result *MarketplaceWebhookDeleteResult) ([]string, [][]string) {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	return headers, rows
}
