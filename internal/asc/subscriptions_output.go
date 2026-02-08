package asc

import (
	"encoding/json"
	"fmt"
)

// SubscriptionGroupDeleteResult represents CLI output for group deletions.
type SubscriptionGroupDeleteResult struct {
	ID      string `json:"id"`
	Deleted bool   `json:"deleted"`
}

// SubscriptionDeleteResult represents CLI output for subscription deletions.
type SubscriptionDeleteResult struct {
	ID      string `json:"id"`
	Deleted bool   `json:"deleted"`
}

// SubscriptionPriceDeleteResult represents CLI output for subscription price deletions.
type SubscriptionPriceDeleteResult struct {
	ID      string `json:"id"`
	Deleted bool   `json:"deleted"`
}

func subscriptionGroupsRows(resp *SubscriptionGroupsResponse) ([]string, [][]string) {
	headers := []string{"ID", "Reference Name"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{
			item.ID,
			compactWhitespace(item.Attributes.ReferenceName),
		})
	}
	return headers, rows
}

func printSubscriptionGroupsTable(resp *SubscriptionGroupsResponse) error {
	h, r := subscriptionGroupsRows(resp)
	RenderTable(h, r)
	return nil
}

func printSubscriptionGroupsMarkdown(resp *SubscriptionGroupsResponse) error {
	h, r := subscriptionGroupsRows(resp)
	RenderMarkdown(h, r)
	return nil
}

func subscriptionsRows(resp *SubscriptionsResponse) ([]string, [][]string) {
	headers := []string{"ID", "Name", "Product ID", "Period", "State"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{
			item.ID,
			compactWhitespace(item.Attributes.Name),
			item.Attributes.ProductID,
			item.Attributes.SubscriptionPeriod,
			item.Attributes.State,
		})
	}
	return headers, rows
}

func printSubscriptionsTable(resp *SubscriptionsResponse) error {
	h, r := subscriptionsRows(resp)
	RenderTable(h, r)
	return nil
}

func printSubscriptionsMarkdown(resp *SubscriptionsResponse) error {
	h, r := subscriptionsRows(resp)
	RenderMarkdown(h, r)
	return nil
}

func subscriptionPriceRows(resp *SubscriptionPriceResponse) ([]string, [][]string) {
	headers := []string{"ID", "Start Date", "Preserved"}
	rows := [][]string{{
		resp.Data.ID,
		resp.Data.Attributes.StartDate,
		fmt.Sprintf("%t", resp.Data.Attributes.Preserved),
	}}
	return headers, rows
}

func printSubscriptionPriceTable(resp *SubscriptionPriceResponse) error {
	h, r := subscriptionPriceRows(resp)
	RenderTable(h, r)
	return nil
}

func printSubscriptionPriceMarkdown(resp *SubscriptionPriceResponse) error {
	h, r := subscriptionPriceRows(resp)
	RenderMarkdown(h, r)
	return nil
}

func subscriptionPricesRows(resp *SubscriptionPricesResponse) ([]string, [][]string, error) {
	headers := []string{"ID", "Territory", "Price Point", "Start Date", "Preserved"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		territoryID, pricePointID, err := subscriptionPriceRelationshipIDs(item.Relationships)
		if err != nil {
			return nil, nil, err
		}
		rows = append(rows, []string{
			item.ID,
			territoryID,
			pricePointID,
			item.Attributes.StartDate,
			fmt.Sprintf("%t", item.Attributes.Preserved),
		})
	}
	return headers, rows, nil
}

func printSubscriptionPricesTable(resp *SubscriptionPricesResponse) error {
	h, r, err := subscriptionPricesRows(resp)
	if err != nil {
		return err
	}
	RenderTable(h, r)
	return nil
}

func printSubscriptionPricesMarkdown(resp *SubscriptionPricesResponse) error {
	h, r, err := subscriptionPricesRows(resp)
	if err != nil {
		return err
	}
	RenderMarkdown(h, r)
	return nil
}

func subscriptionAvailabilityRows(resp *SubscriptionAvailabilityResponse) ([]string, [][]string) {
	headers := []string{"ID", "Available In New Territories"}
	rows := [][]string{{
		resp.Data.ID,
		fmt.Sprintf("%t", resp.Data.Attributes.AvailableInNewTerritories),
	}}
	return headers, rows
}

func printSubscriptionAvailabilityTable(resp *SubscriptionAvailabilityResponse) error {
	h, r := subscriptionAvailabilityRows(resp)
	RenderTable(h, r)
	return nil
}

func printSubscriptionAvailabilityMarkdown(resp *SubscriptionAvailabilityResponse) error {
	h, r := subscriptionAvailabilityRows(resp)
	RenderMarkdown(h, r)
	return nil
}

func subscriptionGracePeriodRows(resp *SubscriptionGracePeriodResponse) ([]string, [][]string) {
	headers := []string{"ID", "Opt In", "Sandbox Opt In", "Duration", "Renewal Type"}
	rows := [][]string{{
		resp.Data.ID,
		fmt.Sprintf("%t", resp.Data.Attributes.OptIn),
		fmt.Sprintf("%t", resp.Data.Attributes.SandboxOptIn),
		resp.Data.Attributes.Duration,
		resp.Data.Attributes.RenewalType,
	}}
	return headers, rows
}

func printSubscriptionGracePeriodTable(resp *SubscriptionGracePeriodResponse) error {
	h, r := subscriptionGracePeriodRows(resp)
	RenderTable(h, r)
	return nil
}

func printSubscriptionGracePeriodMarkdown(resp *SubscriptionGracePeriodResponse) error {
	h, r := subscriptionGracePeriodRows(resp)
	RenderMarkdown(h, r)
	return nil
}

func subscriptionGroupDeleteResultRows(result *SubscriptionGroupDeleteResult) ([]string, [][]string) {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	return headers, rows
}

func printSubscriptionGroupDeleteResultTable(result *SubscriptionGroupDeleteResult) error {
	h, r := subscriptionGroupDeleteResultRows(result)
	RenderTable(h, r)
	return nil
}

func printSubscriptionGroupDeleteResultMarkdown(result *SubscriptionGroupDeleteResult) error {
	h, r := subscriptionGroupDeleteResultRows(result)
	RenderMarkdown(h, r)
	return nil
}

func subscriptionDeleteResultRows(result *SubscriptionDeleteResult) ([]string, [][]string) {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	return headers, rows
}

func printSubscriptionDeleteResultTable(result *SubscriptionDeleteResult) error {
	h, r := subscriptionDeleteResultRows(result)
	RenderTable(h, r)
	return nil
}

func printSubscriptionDeleteResultMarkdown(result *SubscriptionDeleteResult) error {
	h, r := subscriptionDeleteResultRows(result)
	RenderMarkdown(h, r)
	return nil
}

func subscriptionPriceDeleteResultRows(result *SubscriptionPriceDeleteResult) ([]string, [][]string) {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	return headers, rows
}

func printSubscriptionPriceDeleteResultTable(result *SubscriptionPriceDeleteResult) error {
	h, r := subscriptionPriceDeleteResultRows(result)
	RenderTable(h, r)
	return nil
}

func printSubscriptionPriceDeleteResultMarkdown(result *SubscriptionPriceDeleteResult) error {
	h, r := subscriptionPriceDeleteResultRows(result)
	RenderMarkdown(h, r)
	return nil
}

func subscriptionPriceRelationshipIDs(raw json.RawMessage) (string, string, error) {
	if len(raw) == 0 {
		return "", "", nil
	}
	var relationships SubscriptionPriceRelationships
	if err := json.Unmarshal(raw, &relationships); err != nil {
		return "", "", fmt.Errorf("decode subscription price relationships: %w", err)
	}
	territoryID := ""
	pricePointID := ""
	if relationships.Territory != nil {
		territoryID = relationships.Territory.Data.ID
	}
	if relationships.SubscriptionPricePoint != nil {
		pricePointID = relationships.SubscriptionPricePoint.Data.ID
	}
	return territoryID, pricePointID, nil
}
