package asc

import (
	"encoding/json"
	"fmt"
	"os"
)

func printOfferCodeCustomCodesTable(resp *SubscriptionOfferCodeCustomCodesResponse) error {
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
	RenderTable(headers, rows)
	return nil
}

func printOfferCodeCustomCodesMarkdown(resp *SubscriptionOfferCodeCustomCodesResponse) error {
	fmt.Fprintln(os.Stdout, "| ID | Custom Code | Codes | Expires | Created | Active |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- | --- | --- | --- |")
	for _, item := range resp.Data {
		attrs := item.Attributes
		fmt.Fprintf(os.Stdout, "| %s | %s | %d | %s | %s | %t |\n",
			escapeMarkdown(item.ID),
			escapeMarkdown(attrs.CustomCode),
			attrs.NumberOfCodes,
			escapeMarkdown(attrs.ExpirationDate),
			escapeMarkdown(attrs.CreatedDate),
			attrs.Active,
		)
	}
	return nil
}

func printOfferCodePricesTable(resp *SubscriptionOfferCodePricesResponse) error {
	headers := []string{"ID", "Territory", "Price Point"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		territoryID, pricePointID, err := offerCodePriceRelationshipIDs(item.Relationships)
		if err != nil {
			return err
		}
		rows = append(rows, []string{sanitizeTerminal(item.ID), sanitizeTerminal(territoryID), sanitizeTerminal(pricePointID)})
	}
	RenderTable(headers, rows)
	return nil
}

func printOfferCodePricesMarkdown(resp *SubscriptionOfferCodePricesResponse) error {
	fmt.Fprintln(os.Stdout, "| ID | Territory | Price Point |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- |")
	for _, item := range resp.Data {
		territoryID, pricePointID, err := offerCodePriceRelationshipIDs(item.Relationships)
		if err != nil {
			return err
		}
		fmt.Fprintf(os.Stdout, "| %s | %s | %s |\n",
			escapeMarkdown(item.ID),
			escapeMarkdown(territoryID),
			escapeMarkdown(pricePointID),
		)
	}
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

func printOfferCodeValuesTable(result *OfferCodeValuesResult) error {
	headers := []string{"Code"}
	rows := make([][]string, 0, len(result.Codes))
	for _, code := range result.Codes {
		rows = append(rows, []string{sanitizeTerminal(code)})
	}
	RenderTable(headers, rows)
	return nil
}

func printOfferCodeValuesMarkdown(result *OfferCodeValuesResult) error {
	fmt.Fprintln(os.Stdout, "| Code |")
	fmt.Fprintln(os.Stdout, "| --- |")
	for _, code := range result.Codes {
		fmt.Fprintf(os.Stdout, "| %s |\n", escapeMarkdown(code))
	}
	return nil
}
