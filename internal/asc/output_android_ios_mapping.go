package asc

import (
	"fmt"
	"strings"
)

// AndroidToIosAppMappingDeleteResult represents CLI output for deletions.
type AndroidToIosAppMappingDeleteResult struct {
	ID      string `json:"id"`
	Deleted bool   `json:"deleted"`
}

func androidToIosAppMappingDetailsRows(resp *AndroidToIosAppMappingDetailsResponse) ([]string, [][]string) {
	headers := []string{"ID", "Package Name", "Fingerprints"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{
			item.ID,
			item.Attributes.PackageName,
			formatAndroidToIosFingerprints(item.Attributes.AppSigningKeyPublicCertificateSha256Fingerprints),
		})
	}
	return headers, rows
}

func printAndroidToIosAppMappingDetailsTable(resp *AndroidToIosAppMappingDetailsResponse) error {
	h, r := androidToIosAppMappingDetailsRows(resp)
	RenderTable(h, r)
	return nil
}

func printAndroidToIosAppMappingDetailsMarkdown(resp *AndroidToIosAppMappingDetailsResponse) error {
	h, r := androidToIosAppMappingDetailsRows(resp)
	RenderMarkdown(h, r)
	return nil
}

func androidToIosAppMappingDeleteResultRows(result *AndroidToIosAppMappingDeleteResult) ([]string, [][]string) {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	return headers, rows
}

func printAndroidToIosAppMappingDeleteResultTable(result *AndroidToIosAppMappingDeleteResult) error {
	h, r := androidToIosAppMappingDeleteResultRows(result)
	RenderTable(h, r)
	return nil
}

func printAndroidToIosAppMappingDeleteResultMarkdown(result *AndroidToIosAppMappingDeleteResult) error {
	h, r := androidToIosAppMappingDeleteResultRows(result)
	RenderMarkdown(h, r)
	return nil
}

func formatAndroidToIosFingerprints(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return strings.Join(values, ", ")
}
