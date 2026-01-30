package asc

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
)

func printOfferCodesTable(resp *SubscriptionOfferCodeOneTimeUseCodesResponse) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tCodes\tExpires\tCreated\tActive")
	for _, item := range resp.Data {
		attrs := item.Attributes
		fmt.Fprintf(w, "%s\t%d\t%s\t%s\t%t\n",
			sanitizeTerminal(item.ID),
			attrs.NumberOfCodes,
			sanitizeTerminal(attrs.ExpirationDate),
			sanitizeTerminal(attrs.CreatedDate),
			attrs.Active,
		)
	}
	return w.Flush()
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
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tName\tCustomer Eligibilities\tOffer Eligibility\tDuration\tMode\tPeriods\tTotal Codes\tProduction Codes\tSandbox Codes\tActive\tAuto Renew")
	attrs := resp.Data.Attributes
	fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%d\t%d\t%d\t%d\t%t\t%t\n",
		sanitizeTerminal(resp.Data.ID),
		compactWhitespace(attrs.Name),
		sanitizeTerminal(formatOfferCodeCustomerEligibilities(attrs.CustomerEligibilities)),
		sanitizeTerminal(string(attrs.OfferEligibility)),
		sanitizeTerminal(string(attrs.Duration)),
		sanitizeTerminal(string(attrs.OfferMode)),
		attrs.NumberOfPeriods,
		attrs.TotalNumberOfCodes,
		attrs.ProductionCodeCount,
		attrs.SandboxCodeCount,
		attrs.Active,
		attrs.AutoRenewEnabled,
	)
	return w.Flush()
}

func printSubscriptionOfferCodeMarkdown(resp *SubscriptionOfferCodeResponse) error {
	fmt.Fprintln(os.Stdout, "| ID | Name | Customer Eligibilities | Offer Eligibility | Duration | Mode | Periods | Total Codes | Production Codes | Sandbox Codes | Active | Auto Renew |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |")
	attrs := resp.Data.Attributes
	fmt.Fprintf(os.Stdout, "| %s | %s | %s | %s | %s | %s | %d | %d | %d | %d | %t | %t |\n",
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
		attrs.AutoRenewEnabled,
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
