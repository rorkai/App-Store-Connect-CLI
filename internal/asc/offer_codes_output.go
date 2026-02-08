package asc

import (
	"fmt"
	"os"
	"strings"
)

func printOfferCodesTable(resp *SubscriptionOfferCodeOneTimeUseCodesResponse) error {
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
	RenderTable(headers, rows)
	return nil
}

func printOfferCodesMarkdown(resp *SubscriptionOfferCodeOneTimeUseCodesResponse) error {
	fmt.Fprintln(os.Stdout, "| ID | Codes | Expires | Created | Active |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- | --- | --- |")
	for _, item := range resp.Data {
		attrs := item.Attributes
		fmt.Fprintf(os.Stdout, "| %s | %d | %s | %s | %t |\n",
			escapeMarkdown(item.ID),
			attrs.NumberOfCodes,
			escapeMarkdown(attrs.ExpirationDate),
			escapeMarkdown(attrs.CreatedDate),
			attrs.Active,
		)
	}
	return nil
}

func printSubscriptionOfferCodeTable(resp *SubscriptionOfferCodeResponse) error {
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
	RenderTable(headers, rows)
	return nil
}

func printSubscriptionOfferCodeMarkdown(resp *SubscriptionOfferCodeResponse) error {
	fmt.Fprintln(os.Stdout, "| ID | Name | Customer Eligibilities | Offer Eligibility | Duration | Mode | Periods | Total Codes | Production Codes | Sandbox Codes | Active | Auto Renew |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |")
	attrs := resp.Data.Attributes
	fmt.Fprintf(os.Stdout, "| %s | %s | %s | %s | %s | %s | %d | %d | %d | %d | %t | %s |\n",
		escapeMarkdown(resp.Data.ID),
		escapeMarkdown(attrs.Name),
		escapeMarkdown(formatOfferCodeCustomerEligibilities(attrs.CustomerEligibilities)),
		escapeMarkdown(string(attrs.OfferEligibility)),
		escapeMarkdown(string(attrs.Duration)),
		escapeMarkdown(string(attrs.OfferMode)),
		attrs.NumberOfPeriods,
		attrs.TotalNumberOfCodes,
		attrs.ProductionCodeCount,
		attrs.SandboxCodeCount,
		attrs.Active,
		formatOptionalBool(attrs.AutoRenewEnabled),
	)
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
