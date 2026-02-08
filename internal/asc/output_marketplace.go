package asc

import (
	"fmt"
	"os"
)

func printMarketplaceSearchDetailsTable(resp *MarketplaceSearchDetailsResponse) error {
	headers := []string{"ID", "Catalog URL"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{
			item.ID,
			compactWhitespace(item.Attributes.CatalogURL),
		})
	}
	RenderTable(headers, rows)
	return nil
}

func printMarketplaceSearchDetailsMarkdown(resp *MarketplaceSearchDetailsResponse) error {
	fmt.Fprintln(os.Stdout, "| ID | Catalog URL |")
	fmt.Fprintln(os.Stdout, "| --- | --- |")
	for _, item := range resp.Data {
		fmt.Fprintf(os.Stdout, "| %s | %s |\n",
			escapeMarkdown(item.ID),
			escapeMarkdown(item.Attributes.CatalogURL),
		)
	}
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

func printMarketplaceWebhooksTable(resp *MarketplaceWebhooksResponse) error {
	headers := []string{"ID", "Endpoint URL"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{
			item.ID,
			compactWhitespace(item.Attributes.EndpointURL),
		})
	}
	RenderTable(headers, rows)
	return nil
}

func printMarketplaceWebhooksMarkdown(resp *MarketplaceWebhooksResponse) error {
	fmt.Fprintln(os.Stdout, "| ID | Endpoint URL |")
	fmt.Fprintln(os.Stdout, "| --- | --- |")
	for _, item := range resp.Data {
		fmt.Fprintf(os.Stdout, "| %s | %s |\n",
			escapeMarkdown(item.ID),
			escapeMarkdown(item.Attributes.EndpointURL),
		)
	}
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

func printMarketplaceSearchDetailDeleteResultTable(result *MarketplaceSearchDetailDeleteResult) error {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	RenderTable(headers, rows)
	return nil
}

func printMarketplaceSearchDetailDeleteResultMarkdown(result *MarketplaceSearchDetailDeleteResult) error {
	fmt.Fprintln(os.Stdout, "| ID | Deleted |")
	fmt.Fprintln(os.Stdout, "| --- | --- |")
	fmt.Fprintf(os.Stdout, "| %s | %t |\n",
		escapeMarkdown(result.ID),
		result.Deleted,
	)
	return nil
}

func printMarketplaceWebhookDeleteResultTable(result *MarketplaceWebhookDeleteResult) error {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	RenderTable(headers, rows)
	return nil
}

func printMarketplaceWebhookDeleteResultMarkdown(result *MarketplaceWebhookDeleteResult) error {
	fmt.Fprintln(os.Stdout, "| ID | Deleted |")
	fmt.Fprintln(os.Stdout, "| --- | --- |")
	fmt.Fprintf(os.Stdout, "| %s | %t |\n",
		escapeMarkdown(result.ID),
		result.Deleted,
	)
	return nil
}
