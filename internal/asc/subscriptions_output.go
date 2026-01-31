package asc

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"
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

func printSubscriptionGroupsTable(resp *SubscriptionGroupsResponse) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tReference Name")
	for _, item := range resp.Data {
		fmt.Fprintf(w, "%s\t%s\n",
			item.ID,
			compactWhitespace(item.Attributes.ReferenceName),
		)
	}
	return w.Flush()
}

func printSubscriptionGroupsMarkdown(resp *SubscriptionGroupsResponse) error {
	fmt.Fprintln(os.Stdout, "| ID | Reference Name |")
	fmt.Fprintln(os.Stdout, "| --- | --- |")
	for _, item := range resp.Data {
		fmt.Fprintf(os.Stdout, "| %s | %s |\n",
			escapeMarkdown(item.ID),
			escapeMarkdown(item.Attributes.ReferenceName),
		)
	}
	return nil
}

func printSubscriptionsTable(resp *SubscriptionsResponse) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tName\tProduct ID\tPeriod\tState")
	for _, item := range resp.Data {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			item.ID,
			compactWhitespace(item.Attributes.Name),
			item.Attributes.ProductID,
			item.Attributes.SubscriptionPeriod,
			item.Attributes.State,
		)
	}
	return w.Flush()
}

func printSubscriptionsMarkdown(resp *SubscriptionsResponse) error {
	fmt.Fprintln(os.Stdout, "| ID | Name | Product ID | Period | State |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- | --- | --- |")
	for _, item := range resp.Data {
		fmt.Fprintf(os.Stdout, "| %s | %s | %s | %s | %s |\n",
			escapeMarkdown(item.ID),
			escapeMarkdown(item.Attributes.Name),
			escapeMarkdown(item.Attributes.ProductID),
			escapeMarkdown(item.Attributes.SubscriptionPeriod),
			escapeMarkdown(item.Attributes.State),
		)
	}
	return nil
}

func printSubscriptionPriceTable(resp *SubscriptionPriceResponse) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tStart Date\tPreserved")
	fmt.Fprintf(w, "%s\t%s\t%t\n",
		resp.Data.ID,
		resp.Data.Attributes.StartDate,
		resp.Data.Attributes.Preserved,
	)
	return w.Flush()
}

func printSubscriptionPriceMarkdown(resp *SubscriptionPriceResponse) error {
	fmt.Fprintln(os.Stdout, "| ID | Start Date | Preserved |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- |")
	fmt.Fprintf(os.Stdout, "| %s | %s | %t |\n",
		escapeMarkdown(resp.Data.ID),
		escapeMarkdown(resp.Data.Attributes.StartDate),
		resp.Data.Attributes.Preserved,
	)
	return nil
}

func printSubscriptionPricesTable(resp *SubscriptionPricesResponse) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tTerritory\tPrice Point\tStart Date\tPreserved")
	for _, item := range resp.Data {
		territoryID, pricePointID, err := subscriptionPriceRelationshipIDs(item.Relationships)
		if err != nil {
			return err
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%t\n",
			item.ID,
			territoryID,
			pricePointID,
			item.Attributes.StartDate,
			item.Attributes.Preserved,
		)
	}
	return w.Flush()
}

func printSubscriptionPricesMarkdown(resp *SubscriptionPricesResponse) error {
	fmt.Fprintln(os.Stdout, "| ID | Territory | Price Point | Start Date | Preserved |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- | --- | --- |")
	for _, item := range resp.Data {
		territoryID, pricePointID, err := subscriptionPriceRelationshipIDs(item.Relationships)
		if err != nil {
			return err
		}
		fmt.Fprintf(os.Stdout, "| %s | %s | %s | %s | %t |\n",
			escapeMarkdown(item.ID),
			escapeMarkdown(territoryID),
			escapeMarkdown(pricePointID),
			escapeMarkdown(item.Attributes.StartDate),
			item.Attributes.Preserved,
		)
	}
	return nil
}

func printSubscriptionAvailabilityTable(resp *SubscriptionAvailabilityResponse) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tAvailable In New Territories")
	fmt.Fprintf(w, "%s\t%t\n",
		resp.Data.ID,
		resp.Data.Attributes.AvailableInNewTerritories,
	)
	return w.Flush()
}

func printSubscriptionAvailabilityMarkdown(resp *SubscriptionAvailabilityResponse) error {
	fmt.Fprintln(os.Stdout, "| ID | Available In New Territories |")
	fmt.Fprintln(os.Stdout, "| --- | --- |")
	fmt.Fprintf(os.Stdout, "| %s | %t |\n",
		escapeMarkdown(resp.Data.ID),
		resp.Data.Attributes.AvailableInNewTerritories,
	)
	return nil
}

func printSubscriptionGracePeriodTable(resp *SubscriptionGracePeriodResponse) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tOpt In\tSandbox Opt In\tDuration\tRenewal Type")
	fmt.Fprintf(w, "%s\t%t\t%t\t%s\t%s\n",
		resp.Data.ID,
		resp.Data.Attributes.OptIn,
		resp.Data.Attributes.SandboxOptIn,
		resp.Data.Attributes.Duration,
		resp.Data.Attributes.RenewalType,
	)
	return w.Flush()
}

func printSubscriptionGracePeriodMarkdown(resp *SubscriptionGracePeriodResponse) error {
	fmt.Fprintln(os.Stdout, "| ID | Opt In | Sandbox Opt In | Duration | Renewal Type |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- | --- | --- |")
	fmt.Fprintf(os.Stdout, "| %s | %t | %t | %s | %s |\n",
		escapeMarkdown(resp.Data.ID),
		resp.Data.Attributes.OptIn,
		resp.Data.Attributes.SandboxOptIn,
		escapeMarkdown(resp.Data.Attributes.Duration),
		escapeMarkdown(resp.Data.Attributes.RenewalType),
	)
	return nil
}

func printSubscriptionGroupDeleteResultTable(result *SubscriptionGroupDeleteResult) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tDeleted")
	fmt.Fprintf(w, "%s\t%t\n", result.ID, result.Deleted)
	return w.Flush()
}

func printSubscriptionGroupDeleteResultMarkdown(result *SubscriptionGroupDeleteResult) error {
	fmt.Fprintln(os.Stdout, "| ID | Deleted |")
	fmt.Fprintln(os.Stdout, "| --- | --- |")
	fmt.Fprintf(os.Stdout, "| %s | %t |\n",
		escapeMarkdown(result.ID),
		result.Deleted,
	)
	return nil
}

func printSubscriptionDeleteResultTable(result *SubscriptionDeleteResult) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tDeleted")
	fmt.Fprintf(w, "%s\t%t\n", result.ID, result.Deleted)
	return w.Flush()
}

func printSubscriptionDeleteResultMarkdown(result *SubscriptionDeleteResult) error {
	fmt.Fprintln(os.Stdout, "| ID | Deleted |")
	fmt.Fprintln(os.Stdout, "| --- | --- |")
	fmt.Fprintf(os.Stdout, "| %s | %t |\n",
		escapeMarkdown(result.ID),
		result.Deleted,
	)
	return nil
}

func printSubscriptionPriceDeleteResultTable(result *SubscriptionPriceDeleteResult) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tDeleted")
	fmt.Fprintf(w, "%s\t%t\n", result.ID, result.Deleted)
	return w.Flush()
}

func printSubscriptionPriceDeleteResultMarkdown(result *SubscriptionPriceDeleteResult) error {
	fmt.Fprintln(os.Stdout, "| ID | Deleted |")
	fmt.Fprintln(os.Stdout, "| --- | --- |")
	fmt.Fprintf(os.Stdout, "| %s | %t |\n",
		escapeMarkdown(result.ID),
		result.Deleted,
	)
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
