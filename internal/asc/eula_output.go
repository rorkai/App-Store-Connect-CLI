package asc

import (
	"fmt"
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

func endUserLicenseAgreementRows(resp *EndUserLicenseAgreementResponse) ([]string, [][]string) {
	headers := []string{"ID", "App ID", "Territories", "Agreement Text"}
	rows := [][]string{{
		resp.Data.ID,
		compactWhitespace(endUserLicenseAgreementAppID(resp.Data)),
		compactWhitespace(formatEndUserLicenseAgreementTerritories(resp.Data)),
		compactWhitespace(resp.Data.Attributes.AgreementText),
	}}
	return headers, rows
}

func printEndUserLicenseAgreementTable(resp *EndUserLicenseAgreementResponse) error {
	h, r := endUserLicenseAgreementRows(resp)
	RenderTable(h, r)
	return nil
}

func printEndUserLicenseAgreementMarkdown(resp *EndUserLicenseAgreementResponse) error {
	h, r := endUserLicenseAgreementRows(resp)
	RenderMarkdown(h, r)
	return nil
}

func endUserLicenseAgreementDeleteResultRows(result *EndUserLicenseAgreementDeleteResult) ([]string, [][]string) {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	return headers, rows
}

func printEndUserLicenseAgreementDeleteResultTable(result *EndUserLicenseAgreementDeleteResult) error {
	h, r := endUserLicenseAgreementDeleteResultRows(result)
	RenderTable(h, r)
	return nil
}

func printEndUserLicenseAgreementDeleteResultMarkdown(result *EndUserLicenseAgreementDeleteResult) error {
	h, r := endUserLicenseAgreementDeleteResultRows(result)
	RenderMarkdown(h, r)
	return nil
}
