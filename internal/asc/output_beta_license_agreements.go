package asc

import (
	"fmt"
	"os"
)

func betaLicenseAgreementAppID(resource BetaLicenseAgreementResource) string {
	if resource.Relationships == nil || resource.Relationships.App == nil {
		return ""
	}
	return resource.Relationships.App.Data.ID
}

func printBetaLicenseAgreementsTable(resp *BetaLicenseAgreementsResponse) error {
	headers := []string{"ID", "App ID", "Agreement Text"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{
			item.ID,
			betaLicenseAgreementAppID(item),
			compactWhitespace(item.Attributes.AgreementText),
		})
	}
	RenderTable(headers, rows)
	return nil
}

func printBetaLicenseAgreementsMarkdown(resp *BetaLicenseAgreementsResponse) error {
	fmt.Fprintln(os.Stdout, "| ID | App ID | Agreement Text |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- |")
	for _, item := range resp.Data {
		fmt.Fprintf(os.Stdout, "| %s | %s | %s |\n",
			escapeMarkdown(item.ID),
			escapeMarkdown(betaLicenseAgreementAppID(item)),
			escapeMarkdown(item.Attributes.AgreementText),
		)
	}
	return nil
}
