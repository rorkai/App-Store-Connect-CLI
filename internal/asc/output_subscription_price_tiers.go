package asc

import (
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"slices"
)

func subscriptionPriceTiersRows(resp *SubscriptionPriceTiersResponse) ([]string, [][]string, error) {
	included, err := subscriptionPriceIncludedAttributes(resp.Included)
	if err != nil {
		return nil, nil, err
	}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		quantity := ""
		if item.Attributes.MinQuantity != nil {
			quantity = fmt.Sprint(*item.Attributes.MinQuantity)
		}
		_, point, err := subscriptionPriceRelationshipIDs(item.Relationships)
		if err != nil {
			return nil, nil, err
		}
		rows = append(rows, []string{item.ID, quantity, point})
	}
	headers := appendTierPricePointColumns([]string{"ID", "Min Quantity", "Price Point"}, rows, 2, included)
	return headers, rows, nil
}

func subscriptionPriceTierEqualizationTables(w io.Writer, resp *SubscriptionPriceTierEqualizationResponse, render func([]string, [][]string)) error {
	included, err := subscriptionPriceIncludedAttributes(resp.Included)
	if err != nil {
		return err
	}
	var relationships struct {
		PriceTierPricePoints RelationshipList `json:"priceTierPricePoints"`
	}
	if len(resp.Data.Relationships) > 0 {
		if err := json.Unmarshal(resp.Data.Relationships, &relationships); err != nil {
			return err
		}
	}
	linked := make(map[string]bool)
	var points [][]string
	for _, ref := range relationships.PriceTierPricePoints.Data {
		points = append(points, []string{ref.ID})
		linked[ref.ID] = true
	}
	for _, id := range slices.Sorted(maps.Keys(included[ResourceTypeSubscriptionPricePoints])) {
		if !linked[id] {
			points = append(points, []string{id})
		}
	}
	headers := []string{"Territory", "Base Price Point", "Min Quantity", "Tier Price Point"}
	rows := make([][]string, 0)
	for _, item := range resp.Data.Attributes.EqualizedPrices {
		if len(item.AdditionalPriceTiers) == 0 {
			rows = append(rows, []string{item.TerritoryID, item.BasePricePointID, "", ""})
			continue
		}
		for _, tier := range item.AdditionalPriceTiers {
			rows = append(rows, []string{item.TerritoryID, item.BasePricePointID, fmt.Sprint(tier.MinQuantity), tier.SubscriptionPricePointID})
		}
	}
	if len(rows) > 0 || len(points) == 0 {
		render(headers, rows)
	}
	if len(points) > 0 {
		if len(rows) > 0 {
			if _, err := fmt.Fprintln(w); err != nil {
				return err
			}
		}
		pointHeaders := appendTierPricePointColumns([]string{"Price Point"}, points, 0, included)
		render(pointHeaders, points)
	}
	return nil
}

func appendTierPricePointColumns(headers []string, rows [][]string, pointIDColumn int, included subscriptionPriceIncludedAttributesMap) []string {
	if len(included[ResourceTypeSubscriptionPricePoints]) == 0 {
		return headers
	}
	for _, field := range []struct{ key, label string }{
		{"customerPrice", "Customer Price"},
		{"proceeds", "Proceeds"},
		{"proceedsYear2", "Proceeds Y2"},
		{"territory", "Price Point Territory ID"},
		{"equalizations", "Equalizations URL"},
		{"adjustedEqualizations", "Adjusted Equalizations URL"},
	} {
		values := make([]string, len(rows))
		show := false
		for i, row := range rows {
			id := row[pointIDColumn]
			if _, ok := included[ResourceTypeSubscriptionPricePoints][id].Attributes[field.key]; ok {
				values[i] = includedAttributeValue(included, ResourceTypeSubscriptionPricePoints, id, field.key)
				show = true
			} else if value, ok := includedPricePointRelationshipValue(included, id, field.key); ok {
				values[i] = value
				show = true
			}
		}
		if show {
			headers = append(headers, field.label)
			for i, value := range values {
				rows[i] = append(rows[i], value)
			}
		}
	}
	return headers
}

func subscriptionPriceChangePreviewRows(resp *SubscriptionPriceChangePreviewResponse) ([]string, [][]string) {
	headers := []string{"Territory", "Change Type"}
	rows := make([][]string, 0)
	for _, item := range resp.Data.Attributes.ChangeResults {
		rows = append(rows, []string{item.TerritoryID, item.ChangeType})
	}
	return headers, rows
}
