package asc

import (
	"encoding/json"
	"fmt"
	"os"
)

func printTerritoryAgeRatingsTable(resp *TerritoryAgeRatingsResponse) error {
	headers := []string{"ID", "Territory", "App Store Age Rating"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		territoryID, err := territoryAgeRatingTerritoryID(item.Relationships)
		if err != nil {
			return err
		}
		rows = append(rows, []string{item.ID, territoryID, string(item.Attributes.AppStoreAgeRating)})
	}
	RenderTable(headers, rows)
	return nil
}

func printTerritoryAgeRatingsMarkdown(resp *TerritoryAgeRatingsResponse) error {
	fmt.Fprintln(os.Stdout, "| ID | Territory | App Store Age Rating |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- |")
	for _, item := range resp.Data {
		territoryID, err := territoryAgeRatingTerritoryID(item.Relationships)
		if err != nil {
			return err
		}
		fmt.Fprintf(os.Stdout, "| %s | %s | %s |\n",
			escapeMarkdown(item.ID),
			escapeMarkdown(territoryID),
			escapeMarkdown(string(item.Attributes.AppStoreAgeRating)),
		)
	}
	return nil
}

func territoryAgeRatingTerritoryID(raw json.RawMessage) (string, error) {
	if len(raw) == 0 {
		return "", nil
	}

	var relationships TerritoryAgeRatingRelationships
	if err := json.Unmarshal(raw, &relationships); err != nil {
		return "", fmt.Errorf("decode territory age rating relationships: %w", err)
	}
	return relationships.Territory.Data.ID, nil
}
