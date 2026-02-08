package asc

import (
	"fmt"
	"os"
	"strings"
)

func endUserLicenseAgreementAppID(resource EndUserLicenseAgreementResource) string {
	if resource.Relationships == nil || resource.Relationships.App == nil {
		return ""
	}
	return resource.Relationships.App.Data.ID
}

func endUserLicenseAgreementTerritoryIDs(resource EndUserLicenseAgreementResource) []string {
	if resource.Relationships == nil || resource.Relationships.Territories == nil {
		return nil
	}
	ids := make([]string, 0, len(resource.Relationships.Territories.Data))
	for _, item := range resource.Relationships.Territories.Data {
		if strings.TrimSpace(item.ID) != "" {
			ids = append(ids, item.ID)
		}
	}
	return ids
}

func formatEndUserLicenseAgreementTerritories(resource EndUserLicenseAgreementResource) string {
	ids := endUserLicenseAgreementTerritoryIDs(resource)
	if len(ids) == 0 {
		return ""
	}
	return strings.Join(ids, ",")
}

func printEndUserLicenseAgreementTable(resp *EndUserLicenseAgreementResponse) error {
	headers := []string{"ID", "App ID", "Territories", "Agreement Text"}
	rows := [][]string{{
		resp.Data.ID,
		compactWhitespace(endUserLicenseAgreementAppID(resp.Data)),
		compactWhitespace(formatEndUserLicenseAgreementTerritories(resp.Data)),
		compactWhitespace(resp.Data.Attributes.AgreementText),
	}}
	RenderTable(headers, rows)
	return nil
}

func printEndUserLicenseAgreementMarkdown(resp *EndUserLicenseAgreementResponse) error {
	fmt.Fprintln(os.Stdout, "| ID | App ID | Territories | Agreement Text |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- | --- |")
	fmt.Fprintf(os.Stdout, "| %s | %s | %s | %s |\n",
		escapeMarkdown(resp.Data.ID),
		escapeMarkdown(endUserLicenseAgreementAppID(resp.Data)),
		escapeMarkdown(formatEndUserLicenseAgreementTerritories(resp.Data)),
		escapeMarkdown(resp.Data.Attributes.AgreementText),
	)
	return nil
}

func printEndUserLicenseAgreementDeleteResultTable(result *EndUserLicenseAgreementDeleteResult) error {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	RenderTable(headers, rows)
	return nil
}

func printEndUserLicenseAgreementDeleteResultMarkdown(result *EndUserLicenseAgreementDeleteResult) error {
	fmt.Fprintln(os.Stdout, "| ID | Deleted |")
	fmt.Fprintln(os.Stdout, "| --- | --- |")
	fmt.Fprintf(os.Stdout, "| %s | %t |\n",
		escapeMarkdown(result.ID),
		result.Deleted,
	)
	return nil
}
