package asc

import (
	"fmt"
	"strings"
)

func offerCodesRows(resp *SubscriptionOfferCodeOneTimeUseCodesResponse) ([]string, [][]string) {
	headers := []string{"ID", "Codes", "Expires", "Created", "Active", "Environment"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		attrs := item.Attributes
		rows = append(rows, []string{
			SanitizeTerminalText(item.ID),
			fmt.Sprintf("%d", attrs.NumberOfCodes),
			SanitizeTerminalText(attrs.ExpirationDate),
			SanitizeTerminalText(attrs.CreatedDate),
			fmt.Sprintf("%t", attrs.Active),
			SanitizeTerminalText(attrs.Environment),
		})
	}
	return headers, rows
}

func subscriptionOfferCodesRows(resp *SubscriptionOfferCodesResponse) ([]string, [][]string) {
	headers := []string{"ID", "Name", "Customer Eligibilities", "Offer Eligibility", "Duration", "Mode", "Periods", "Total Codes", "Production Codes", "Sandbox Codes", "Active", "Auto Renew", "Target Plan Type"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		attrs := item.Attributes
		rows = append(rows, []string{
			SanitizeTerminalText(item.ID),
			SanitizeTerminalText(compactWhitespace(attrs.Name)),
			SanitizeTerminalText(formatOfferCodeCustomerEligibilities(attrs.CustomerEligibilities)),
			SanitizeTerminalText(string(attrs.OfferEligibility)),
			SanitizeTerminalText(string(attrs.Duration)),
			SanitizeTerminalText(string(attrs.OfferMode)),
			fmt.Sprintf("%d", attrs.NumberOfPeriods),
			fmt.Sprintf("%d", attrs.TotalNumberOfCodes),
			fmt.Sprintf("%d", attrs.ProductionCodeCount),
			fmt.Sprintf("%d", attrs.SandboxCodeCount),
			fmt.Sprintf("%t", attrs.Active),
			formatOptionalBool(attrs.AutoRenewEnabled),
			SanitizeTerminalText(string(attrs.TargetSubscriptionPlanType)),
		})
	}
	return headers, rows
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
