package asc

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"
)

func printOfferCodeCustomCodesTable(resp *SubscriptionOfferCodeCustomCodesResponse) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tCustom Code\tCodes\tExpires\tCreated\tActive")
	for _, item := range resp.Data {
		attrs := item.Attributes
		fmt.Fprintf(w, "%s\t%s\t%d\t%s\t%s\t%t\n",
			sanitizeTerminal(item.ID),
			sanitizeTerminal(attrs.CustomCode),
			attrs.NumberOfCodes,
			sanitizeTerminal(attrs.ExpirationDate),
			sanitizeTerminal(attrs.CreatedDate),
			attrs.Active,
		)
	}
	return w.Flush()
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
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tTerritory\tPrice Point")
	for _, item := range resp.Data {
		territoryID, pricePointID, err := offerCodePriceRelationshipIDs(item.Relationships)
		if err != nil {
			return err
		}
		fmt.Fprintf(w, "%s\t%s\t%s\n", sanitizeTerminal(item.ID), sanitizeTerminal(territoryID), sanitizeTerminal(pricePointID))
	}
	return w.Flush()
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
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "Code")
	for _, code := range result.Codes {
		fmt.Fprintf(w, "%s\n", sanitizeTerminal(code))
	}
	return w.Flush()
}

func printOfferCodeValuesMarkdown(result *OfferCodeValuesResult) error {
	fmt.Fprintln(os.Stdout, "| Code |")
	fmt.Fprintln(os.Stdout, "| --- |")
	for _, code := range result.Codes {
		fmt.Fprintf(os.Stdout, "| %s |\n", escapeMarkdown(code))
	}
	return nil
}
