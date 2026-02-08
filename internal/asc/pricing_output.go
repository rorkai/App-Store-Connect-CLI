package asc

import "fmt"

func territoriesRows(resp *TerritoriesResponse) ([]string, [][]string) {
	headers := []string{"ID", "Currency"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{item.ID, item.Attributes.Currency})
	}
	return headers, rows
}

func printTerritoriesTable(resp *TerritoriesResponse) error {
	h, r := territoriesRows(resp)
	RenderTable(h, r)
	return nil
}

func printTerritoriesMarkdown(resp *TerritoriesResponse) error {
	h, r := territoriesRows(resp)
	RenderMarkdown(h, r)
	return nil
}

func appPricePointsRows(resp *AppPricePointsV3Response) ([]string, [][]string) {
	headers := []string{"ID", "Customer Price", "Proceeds"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{
			item.ID,
			item.Attributes.CustomerPrice,
			item.Attributes.Proceeds,
		})
	}
	return headers, rows
}

func printAppPricePointsTable(resp *AppPricePointsV3Response) error {
	h, r := appPricePointsRows(resp)
	RenderTable(h, r)
	return nil
}

func printAppPricePointsMarkdown(resp *AppPricePointsV3Response) error {
	h, r := appPricePointsRows(resp)
	RenderMarkdown(h, r)
	return nil
}

func appPricesRows(resp *AppPricesResponse) ([]string, [][]string) {
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
	return headers, rows
}

func printAppPricesTable(resp *AppPricesResponse) error {
	h, r := appPricesRows(resp)
	RenderTable(h, r)
	return nil
}

func printAppPricesMarkdown(resp *AppPricesResponse) error {
	h, r := appPricesRows(resp)
	RenderMarkdown(h, r)
	return nil
}

func appPriceScheduleRows(resp *AppPriceScheduleResponse) ([]string, [][]string) {
	headers := []string{"ID"}
	rows := [][]string{{resp.Data.ID}}
	return headers, rows
}

func printAppPriceScheduleTable(resp *AppPriceScheduleResponse) error {
	h, r := appPriceScheduleRows(resp)
	RenderTable(h, r)
	return nil
}

func printAppPriceScheduleMarkdown(resp *AppPriceScheduleResponse) error {
	h, r := appPriceScheduleRows(resp)
	RenderMarkdown(h, r)
	return nil
}

func appAvailabilityRows(resp *AppAvailabilityV2Response) ([]string, [][]string) {
	headers := []string{"ID", "Available In New Territories"}
	rows := [][]string{{resp.Data.ID, fmt.Sprintf("%t", resp.Data.Attributes.AvailableInNewTerritories)}}
	return headers, rows
}

func printAppAvailabilityTable(resp *AppAvailabilityV2Response) error {
	h, r := appAvailabilityRows(resp)
	RenderTable(h, r)
	return nil
}

func printAppAvailabilityMarkdown(resp *AppAvailabilityV2Response) error {
	h, r := appAvailabilityRows(resp)
	RenderMarkdown(h, r)
	return nil
}

func territoryAvailabilitiesRows(resp *TerritoryAvailabilitiesResponse) ([]string, [][]string) {
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
	return headers, rows
}

func printTerritoryAvailabilitiesTable(resp *TerritoryAvailabilitiesResponse) error {
	h, r := territoryAvailabilitiesRows(resp)
	RenderTable(h, r)
	return nil
}

func printTerritoryAvailabilitiesMarkdown(resp *TerritoryAvailabilitiesResponse) error {
	h, r := territoryAvailabilitiesRows(resp)
	RenderMarkdown(h, r)
	return nil
}
