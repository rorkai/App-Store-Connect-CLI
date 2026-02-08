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

func printMarketplaceSearchDetailsTable(resp *MarketplaceSearchDetailsResponse) error {
	h, r := marketplaceSearchDetailsRows(resp)
	RenderTable(h, r)
	return nil
}

func printMarketplaceSearchDetailsMarkdown(resp *MarketplaceSearchDetailsResponse) error {
	h, r := marketplaceSearchDetailsRows(resp)
	RenderMarkdown(h, r)
	return nil
}

func printMarketplaceSearchDetailTable(resp *MarketplaceSearchDetailResponse) error {
	return printMarketplaceSearchDetailsTable(&MarketplaceSearchDetailsResponse{
		Data: []Resource[MarketplaceSearchDetailAttributes]{resp.Data},
	})
}

func printMarketplaceSearchDetailMarkdown(resp *MarketplaceSearchDetailResponse) error {
	return printMarketplaceSearchDetailsMarkdown(&MarketplaceSearchDetailsResponse{
		Data: []Resource[MarketplaceSearchDetailAttributes]{resp.Data},
	})
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

func printMarketplaceWebhooksTable(resp *MarketplaceWebhooksResponse) error {
	h, r := marketplaceWebhooksRows(resp)
	RenderTable(h, r)
	return nil
}

func printMarketplaceWebhooksMarkdown(resp *MarketplaceWebhooksResponse) error {
	h, r := marketplaceWebhooksRows(resp)
	RenderMarkdown(h, r)
	return nil
}

func printMarketplaceWebhookTable(resp *MarketplaceWebhookResponse) error {
	return printMarketplaceWebhooksTable(&MarketplaceWebhooksResponse{
		Data: []Resource[MarketplaceWebhookAttributes]{resp.Data},
	})
}

func printMarketplaceWebhookMarkdown(resp *MarketplaceWebhookResponse) error {
	return printMarketplaceWebhooksMarkdown(&MarketplaceWebhooksResponse{
		Data: []Resource[MarketplaceWebhookAttributes]{resp.Data},
	})
}

func marketplaceSearchDetailDeleteResultRows(result *MarketplaceSearchDetailDeleteResult) ([]string, [][]string) {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	return headers, rows
}

func printMarketplaceSearchDetailDeleteResultTable(result *MarketplaceSearchDetailDeleteResult) error {
	h, r := marketplaceSearchDetailDeleteResultRows(result)
	RenderTable(h, r)
	return nil
}

func printMarketplaceSearchDetailDeleteResultMarkdown(result *MarketplaceSearchDetailDeleteResult) error {
	h, r := marketplaceSearchDetailDeleteResultRows(result)
	RenderMarkdown(h, r)
	return nil
}

func marketplaceWebhookDeleteResultRows(result *MarketplaceWebhookDeleteResult) ([]string, [][]string) {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	return headers, rows
}

func printMarketplaceWebhookDeleteResultTable(result *MarketplaceWebhookDeleteResult) error {
	h, r := marketplaceWebhookDeleteResultRows(result)
	RenderTable(h, r)
	return nil
}

func printMarketplaceWebhookDeleteResultMarkdown(result *MarketplaceWebhookDeleteResult) error {
	h, r := marketplaceWebhookDeleteResultRows(result)
	RenderMarkdown(h, r)
	return nil
}
