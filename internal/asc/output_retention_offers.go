package asc

import "strings"

// SubscriptionRetentionMessageOffersResult represents CLI output for adding or
// removing the retention offers presented with a subscription's retention message.
type SubscriptionRetentionMessageOffersResult struct {
	SubscriptionID    string   `json:"subscriptionId"`
	RetentionOfferIDs []string `json:"retentionOfferIds"`
	Action            string   `json:"action"`
}

func retentionOffersRows(resp *RetentionOffersResponse) ([]string, [][]string) {
	headers := []string{"ID", "Reference Name", "Offer ID", "Duration", "Mode", "Periods", "Paid Months", "Wait Months", "Target Plan Type"}
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
			strings.TrimPrefix(string(attrs.CustomerEligibilityWaitBetweenOffersInMonths), "null"),
			string(attrs.TargetSubscriptionPlanType),
		})
	}
	return headers, rows
}

func retentionOfferPricesRows(resp *RetentionOfferPricesResponse) ([]string, [][]string, error) {
	headers := []string{"ID", "Territory", "Price Point"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		territoryID, pricePointID, err := offerPriceRelationshipIDs(item.Relationships)
		if err != nil {
			return nil, nil, err
		}
		rows = append(rows, []string{item.ID, territoryID, pricePointID})
	}
	return headers, rows, nil
}

func subscriptionRetentionMessageOffersResultRows(result *SubscriptionRetentionMessageOffersResult) ([]string, [][]string) {
	headers := []string{"Subscription ID", "Retention Offer IDs", "Action"}
	return headers, [][]string{{result.SubscriptionID, strings.Join(result.RetentionOfferIDs, ", "), result.Action}}
}
