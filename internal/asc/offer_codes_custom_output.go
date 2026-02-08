package asc

import (
	"encoding/json"
	"fmt"
)

func offerCodeCustomCodesRows(resp *SubscriptionOfferCodeCustomCodesResponse) ([]string, [][]string) {
	headers := []string{"ID", "Custom Code", "Codes", "Expires", "Created", "Active"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		attrs := item.Attributes
		rows = append(rows, []string{
			sanitizeTerminal(item.ID),
			sanitizeTerminal(attrs.CustomCode),
			fmt.Sprintf("%d", attrs.NumberOfCodes),
			sanitizeTerminal(attrs.ExpirationDate),
			sanitizeTerminal(attrs.CreatedDate),
			fmt.Sprintf("%t", attrs.Active),
		})
	}
	return headers, rows
}

func printOfferCodeCustomCodesTable(resp *SubscriptionOfferCodeCustomCodesResponse) error {
	h, r := offerCodeCustomCodesRows(resp)
	RenderTable(h, r)
	return nil
}

func printOfferCodeCustomCodesMarkdown(resp *SubscriptionOfferCodeCustomCodesResponse) error {
	h, r := offerCodeCustomCodesRows(resp)
	RenderMarkdown(h, r)
	return nil
}

func offerCodePricesRows(resp *SubscriptionOfferCodePricesResponse) ([]string, [][]string, error) {
	headers := []string{"ID", "Territory", "Price Point"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		territoryID, pricePointID, err := offerCodePriceRelationshipIDs(item.Relationships)
		if err != nil {
			return nil, nil, err
		}
		rows = append(rows, []string{sanitizeTerminal(item.ID), sanitizeTerminal(territoryID), sanitizeTerminal(pricePointID)})
	}
	return headers, rows, nil
}

func printOfferCodePricesTable(resp *SubscriptionOfferCodePricesResponse) error {
	h, r, err := offerCodePricesRows(resp)
	if err != nil {
		return err
	}
	RenderTable(h, r)
	return nil
}

func printOfferCodePricesMarkdown(resp *SubscriptionOfferCodePricesResponse) error {
	h, r, err := offerCodePricesRows(resp)
	if err != nil {
		return err
	}
	RenderMarkdown(h, r)
	return nil
}

func offerCodePriceRelationshipIDs(raw json.RawMessage) (string, string, error) {
	if len(raw) == 0 {
		return "", "", nil
	}
	var relationships SubscriptionOfferCodePriceRelationships
	if err := json.Unmarshal(raw, &relationships); err != nil {
		return "", "", fmt.Errorf("decode offer code price relationships: %w", err)
	}
	return relationships.Territory.Data.ID, relationships.SubscriptionPricePoint.Data.ID, nil
}

func offerCodeValuesRows(result *OfferCodeValuesResult) ([]string, [][]string) {
	headers := []string{"Code"}
	rows := make([][]string, 0, len(result.Codes))
	for _, code := range result.Codes {
		rows = append(rows, []string{sanitizeTerminal(code)})
	}
	return headers, rows
}

func printOfferCodeValuesTable(result *OfferCodeValuesResult) error {
	h, r := offerCodeValuesRows(result)
	RenderTable(h, r)
	return nil
}

func printOfferCodeValuesMarkdown(result *OfferCodeValuesResult) error {
	h, r := offerCodeValuesRows(result)
	RenderMarkdown(h, r)
	return nil
}
