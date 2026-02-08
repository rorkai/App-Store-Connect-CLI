package asc

import (
	"encoding/json"
	"fmt"
)

func territoryAgeRatingsRows(resp *TerritoryAgeRatingsResponse) ([]string, [][]string, error) {
	headers := []string{"ID", "Territory", "App Store Age Rating"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		territoryID, err := territoryAgeRatingTerritoryID(item.Relationships)
		if err != nil {
			return nil, nil, err
		}
		rows = append(rows, []string{item.ID, territoryID, string(item.Attributes.AppStoreAgeRating)})
	}
	return headers, rows, nil
}

func printTerritoryAgeRatingsTable(resp *TerritoryAgeRatingsResponse) error {
	h, r, err := territoryAgeRatingsRows(resp)
	if err != nil {
		return err
	}
	RenderTable(h, r)
	return nil
}

func printTerritoryAgeRatingsMarkdown(resp *TerritoryAgeRatingsResponse) error {
	h, r, err := territoryAgeRatingsRows(resp)
	if err != nil {
		return err
	}
	RenderMarkdown(h, r)
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
