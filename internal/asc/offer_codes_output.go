package asc

import (
	"fmt"
	"strings"
)

func offerCodesRows(resp *SubscriptionOfferCodeOneTimeUseCodesResponse) ([]string, [][]string) {
	headers := []string{"ID", "Codes", "Expires", "Created", "Active"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		attrs := item.Attributes
		rows = append(rows, []string{
			sanitizeTerminal(item.ID),
			fmt.Sprintf("%d", attrs.NumberOfCodes),
			sanitizeTerminal(attrs.ExpirationDate),
			sanitizeTerminal(attrs.CreatedDate),
			fmt.Sprintf("%t", attrs.Active),
		})
	}
	return headers, rows
}

func printOfferCodesTable(resp *SubscriptionOfferCodeOneTimeUseCodesResponse) error {
	h, r := offerCodesRows(resp)
	RenderTable(h, r)
	return nil
}

func printOfferCodesMarkdown(resp *SubscriptionOfferCodeOneTimeUseCodesResponse) error {
	h, r := offerCodesRows(resp)
	RenderMarkdown(h, r)
	return nil
}

func subscriptionOfferCodeRows(resp *SubscriptionOfferCodeResponse) ([]string, [][]string) {
	headers := []string{"ID", "Name", "Customer Eligibilities", "Offer Eligibility", "Duration", "Mode", "Periods", "Total Codes", "Production Codes", "Sandbox Codes", "Active", "Auto Renew"}
	attrs := resp.Data.Attributes
	rows := [][]string{{
		sanitizeTerminal(resp.Data.ID),
		compactWhitespace(attrs.Name),
		sanitizeTerminal(formatOfferCodeCustomerEligibilities(attrs.CustomerEligibilities)),
		sanitizeTerminal(string(attrs.OfferEligibility)),
		sanitizeTerminal(string(attrs.Duration)),
		sanitizeTerminal(string(attrs.OfferMode)),
		fmt.Sprintf("%d", attrs.NumberOfPeriods),
		fmt.Sprintf("%d", attrs.TotalNumberOfCodes),
		fmt.Sprintf("%d", attrs.ProductionCodeCount),
		fmt.Sprintf("%d", attrs.SandboxCodeCount),
		fmt.Sprintf("%t", attrs.Active),
		formatOptionalBool(attrs.AutoRenewEnabled),
	}}
	return headers, rows
}

func printSubscriptionOfferCodeTable(resp *SubscriptionOfferCodeResponse) error {
	h, r := subscriptionOfferCodeRows(resp)
	RenderTable(h, r)
	return nil
}

func printSubscriptionOfferCodeMarkdown(resp *SubscriptionOfferCodeResponse) error {
	h, r := subscriptionOfferCodeRows(resp)
	RenderMarkdown(h, r)
	return nil
}

func formatOfferCodeCustomerEligibilities(values []SubscriptionCustomerEligibility) string {
	if len(values) == 0 {
		return ""
	}
	labels := make([]string, 0, len(values))
	for _, value := range values {
		labels = append(labels, string(value))
	}
	return strings.Join(labels, ", ")
}
