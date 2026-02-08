package asc

import (
	"fmt"
	"os"
)

func printTerritoriesTable(resp *TerritoriesResponse) error {
	headers := []string{"ID", "Currency"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{item.ID, item.Attributes.Currency})
	}
	RenderTable(headers, rows)
	return nil
}

func printTerritoriesMarkdown(resp *TerritoriesResponse) error {
	fmt.Fprintln(os.Stdout, "| ID | Currency |")
	fmt.Fprintln(os.Stdout, "| --- | --- |")
	for _, item := range resp.Data {
		fmt.Fprintf(os.Stdout, "| %s | %s |\n",
			escapeMarkdown(item.ID),
			escapeMarkdown(item.Attributes.Currency),
		)
	}
	return nil
}

func printAppPricePointsTable(resp *AppPricePointsV3Response) error {
	headers := []string{"ID", "Customer Price", "Proceeds"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{
			item.ID,
			item.Attributes.CustomerPrice,
			item.Attributes.Proceeds,
		})
	}
	RenderTable(headers, rows)
	return nil
}

func printAppPricePointsMarkdown(resp *AppPricePointsV3Response) error {
	fmt.Fprintln(os.Stdout, "| ID | Customer Price | Proceeds |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- |")
	for _, item := range resp.Data {
		fmt.Fprintf(os.Stdout, "| %s | %s | %s |\n",
			escapeMarkdown(item.ID),
			escapeMarkdown(item.Attributes.CustomerPrice),
			escapeMarkdown(item.Attributes.Proceeds),
		)
	}
	return nil
}

func printAppPricesTable(resp *AppPricesResponse) error {
	headers := []string{"ID", "Start Date", "End Date", "Manual"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{
			item.ID,
			compactWhitespace(item.Attributes.StartDate),
			compactWhitespace(item.Attributes.EndDate),
			fmt.Sprintf("%t", item.Attributes.Manual),
		})
	}
	RenderTable(headers, rows)
	return nil
}

func printAppPricesMarkdown(resp *AppPricesResponse) error {
	fmt.Fprintln(os.Stdout, "| ID | Start Date | End Date | Manual |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- | --- |")
	for _, item := range resp.Data {
		fmt.Fprintf(os.Stdout, "| %s | %s | %s | %t |\n",
			escapeMarkdown(item.ID),
			escapeMarkdown(item.Attributes.StartDate),
			escapeMarkdown(item.Attributes.EndDate),
			item.Attributes.Manual,
		)
	}
	return nil
}

func printAppPriceScheduleTable(resp *AppPriceScheduleResponse) error {
	headers := []string{"ID"}
	rows := [][]string{{resp.Data.ID}}
	RenderTable(headers, rows)
	return nil
}

func printAppPriceScheduleMarkdown(resp *AppPriceScheduleResponse) error {
	fmt.Fprintln(os.Stdout, "| ID |")
	fmt.Fprintln(os.Stdout, "| --- |")
	fmt.Fprintf(os.Stdout, "| %s |\n", escapeMarkdown(resp.Data.ID))
	return nil
}

func printAppAvailabilityTable(resp *AppAvailabilityV2Response) error {
	headers := []string{"ID", "Available In New Territories"}
	rows := [][]string{{resp.Data.ID, fmt.Sprintf("%t", resp.Data.Attributes.AvailableInNewTerritories)}}
	RenderTable(headers, rows)
	return nil
}

func printAppAvailabilityMarkdown(resp *AppAvailabilityV2Response) error {
	fmt.Fprintln(os.Stdout, "| ID | Available In New Territories |")
	fmt.Fprintln(os.Stdout, "| --- | --- |")
	fmt.Fprintf(os.Stdout, "| %s | %t |\n",
		escapeMarkdown(resp.Data.ID),
		resp.Data.Attributes.AvailableInNewTerritories,
	)
	return nil
}

func printTerritoryAvailabilitiesTable(resp *TerritoryAvailabilitiesResponse) error {
	headers := []string{"ID", "Available", "Release Date", "Preorder Enabled"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{
			item.ID,
			fmt.Sprintf("%t", item.Attributes.Available),
			compactWhitespace(item.Attributes.ReleaseDate),
			fmt.Sprintf("%t", item.Attributes.PreOrderEnabled),
		})
	}
	RenderTable(headers, rows)
	return nil
}

func printTerritoryAvailabilitiesMarkdown(resp *TerritoryAvailabilitiesResponse) error {
	fmt.Fprintln(os.Stdout, "| ID | Available | Release Date | Preorder Enabled |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- | --- |")
	for _, item := range resp.Data {
		fmt.Fprintf(os.Stdout, "| %s | %t | %s | %t |\n",
			escapeMarkdown(item.ID),
			item.Attributes.Available,
			escapeMarkdown(item.Attributes.ReleaseDate),
			item.Attributes.PreOrderEnabled,
		)
	}
	return nil
}
