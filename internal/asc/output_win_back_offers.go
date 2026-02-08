package asc

import (
	"encoding/json"
	"fmt"
	"strconv"
)

// WinBackOfferDeleteResult represents CLI output for win-back offer deletions.
type WinBackOfferDeleteResult struct {
	ID      string `json:"id"`
	Deleted bool   `json:"deleted"`
}

func winBackOffersRows(resp *WinBackOffersResponse) ([]string, [][]string) {
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
	return headers, rows
}

func winBackOfferPricesRows(resp *WinBackOfferPricesResponse) ([]string, [][]string, error) {
	headers := []string{"ID", "Territory", "Price Point"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		territoryID, pricePointID, err := winBackOfferPriceRelationshipIDs(item.Relationships)
		if err != nil {
			return nil, nil, err
		}
		rows = append(rows, []string{item.ID, territoryID, pricePointID})
	}
	return headers, rows, nil
}

func winBackOfferDeleteResultRows(result *WinBackOfferDeleteResult) ([]string, [][]string) {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	return headers, rows
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
