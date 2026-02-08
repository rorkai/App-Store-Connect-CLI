package asc

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
)

// WinBackOfferDeleteResult represents CLI output for win-back offer deletions.
type WinBackOfferDeleteResult struct {
	ID      string `json:"id"`
	Deleted bool   `json:"deleted"`
}

func printWinBackOffersTable(resp *WinBackOffersResponse) error {
	headers := []string{"ID", "Reference Name", "Offer ID", "Duration", "Mode", "Periods", "Paid Months", "Last Subscribed", "Wait Months", "Start Date", "End Date", "Priority", "Promotion Intent"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		attrs := item.Attributes
		rows = append(rows, []string{
			item.ID,
			compactWhitespace(attrs.ReferenceName),
			attrs.OfferID,
			string(attrs.Duration),
			string(attrs.OfferMode),
			formatInt(attrs.PeriodCount),
			formatInt(attrs.CustomerEligibilityPaidSubscriptionDurationInMonths),
			formatIntegerRange(attrs.CustomerEligibilityTimeSinceLastSubscribedInMonths),
			formatOptionalInt(attrs.CustomerEligibilityWaitBetweenOffersInMonths),
			attrs.StartDate,
			formatOptionalString(attrs.EndDate),
			string(attrs.Priority),
			formatPromotionIntent(attrs.PromotionIntent),
		})
	}
	RenderTable(headers, rows)
	return nil
}

func printWinBackOffersMarkdown(resp *WinBackOffersResponse) error {
	fmt.Fprintln(os.Stdout, "| ID | Reference Name | Offer ID | Duration | Mode | Periods | Paid Months | Last Subscribed | Wait Months | Start Date | End Date | Priority | Promotion Intent |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |")
	for _, item := range resp.Data {
		attrs := item.Attributes
		fmt.Fprintf(os.Stdout, "| %s | %s | %s | %s | %s | %s | %s | %s | %s | %s | %s | %s | %s |\n",
			escapeMarkdown(item.ID),
			escapeMarkdown(attrs.ReferenceName),
			escapeMarkdown(attrs.OfferID),
			escapeMarkdown(string(attrs.Duration)),
			escapeMarkdown(string(attrs.OfferMode)),
			escapeMarkdown(formatInt(attrs.PeriodCount)),
			escapeMarkdown(formatInt(attrs.CustomerEligibilityPaidSubscriptionDurationInMonths)),
			escapeMarkdown(formatIntegerRange(attrs.CustomerEligibilityTimeSinceLastSubscribedInMonths)),
			escapeMarkdown(formatOptionalInt(attrs.CustomerEligibilityWaitBetweenOffersInMonths)),
			escapeMarkdown(attrs.StartDate),
			escapeMarkdown(formatOptionalString(attrs.EndDate)),
			escapeMarkdown(string(attrs.Priority)),
			escapeMarkdown(formatPromotionIntent(attrs.PromotionIntent)),
		)
	}
	return nil
}

func printWinBackOfferPricesTable(resp *WinBackOfferPricesResponse) error {
	headers := []string{"ID", "Territory", "Price Point"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		territoryID, pricePointID, err := winBackOfferPriceRelationshipIDs(item.Relationships)
		if err != nil {
			return err
		}
		rows = append(rows, []string{item.ID, territoryID, pricePointID})
	}
	RenderTable(headers, rows)
	return nil
}

func printWinBackOfferPricesMarkdown(resp *WinBackOfferPricesResponse) error {
	fmt.Fprintln(os.Stdout, "| ID | Territory | Price Point |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- |")
	for _, item := range resp.Data {
		territoryID, pricePointID, err := winBackOfferPriceRelationshipIDs(item.Relationships)
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

func printWinBackOfferDeleteResultTable(result *WinBackOfferDeleteResult) error {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	RenderTable(headers, rows)
	return nil
}

func printWinBackOfferDeleteResultMarkdown(result *WinBackOfferDeleteResult) error {
	fmt.Fprintln(os.Stdout, "| ID | Deleted |")
	fmt.Fprintln(os.Stdout, "| --- | --- |")
	fmt.Fprintf(os.Stdout, "| %s | %t |\n",
		escapeMarkdown(result.ID),
		result.Deleted,
	)
	return nil
}

func winBackOfferPriceRelationshipIDs(raw json.RawMessage) (string, string, error) {
	if len(raw) == 0 {
		return "", "", nil
	}
	var relationships WinBackOfferPriceRelationships
	if err := json.Unmarshal(raw, &relationships); err != nil {
		return "", "", fmt.Errorf("decode win-back offer price relationships: %w", err)
	}
	return relationships.Territory.Data.ID, relationships.SubscriptionPricePoint.Data.ID, nil
}

func formatIntegerRange(rangeValue *IntegerRange) string {
	if rangeValue == nil {
		return ""
	}
	minimum := formatOptionalInt(rangeValue.Minimum)
	maximum := formatOptionalInt(rangeValue.Maximum)
	switch {
	case minimum != "" && maximum != "":
		return minimum + "-" + maximum
	case minimum != "":
		return minimum
	case maximum != "":
		return maximum
	default:
		return ""
	}
}

func formatOptionalInt(value *int) string {
	if value == nil {
		return ""
	}
	return strconv.Itoa(*value)
}

func formatInt(value int) string {
	return strconv.Itoa(value)
}

func formatPromotionIntent(value *WinBackOfferPromotionIntent) string {
	if value == nil {
		return ""
	}
	return string(*value)
}
