package asc

import (
	"encoding/json"
	"fmt"
)

func subscriptionPriceTiersRows(resp *SubscriptionPriceTiersResponse) ([]string, [][]string) {
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		quantity := ""
		if item.Attributes.MinQuantity != nil {
			quantity = fmt.Sprint(*item.Attributes.MinQuantity)
		}
		var rels struct {
			SubscriptionPricePoint *Relationship `json:"subscriptionPricePoint"`
		}
		_ = json.Unmarshal(item.Relationships, &rels)
		point := ""
		if rels.SubscriptionPricePoint != nil {
			point = rels.SubscriptionPricePoint.Data.ID
		}
		rows = append(rows, []string{item.ID, quantity, point})
	}
	return []string{"ID", "Min Quantity", "Price Point"}, rows
}

func subscriptionPriceTierEqualizationRows(resp *SubscriptionPriceTierEqualizationResponse) ([]string, [][]string) {
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
	return headers, rows
}

func subscriptionPriceChangePreviewRows(resp *SubscriptionPriceChangePreviewResponse) ([]string, [][]string) {
	headers := []string{"Territory", "Change Type"}
	rows := make([][]string, 0)
	for _, item := range resp.Data.Attributes.ChangeResults {
		rows = append(rows, []string{item.TerritoryID, item.ChangeType})
	}
	return headers, rows
}
