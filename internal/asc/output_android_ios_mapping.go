package asc

import (
	"fmt"
	"os"
	"strings"
)

// AndroidToIosAppMappingDeleteResult represents CLI output for deletions.
type AndroidToIosAppMappingDeleteResult struct {
	ID      string `json:"id"`
	Deleted bool   `json:"deleted"`
}

func printAndroidToIosAppMappingDetailsTable(resp *AndroidToIosAppMappingDetailsResponse) error {
	headers := []string{"ID", "Package Name", "Fingerprints"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{
			item.ID,
			item.Attributes.PackageName,
			formatAndroidToIosFingerprints(item.Attributes.AppSigningKeyPublicCertificateSha256Fingerprints),
		})
	}
	RenderTable(headers, rows)
	return nil
}

func printAndroidToIosAppMappingDetailsMarkdown(resp *AndroidToIosAppMappingDetailsResponse) error {
	fmt.Fprintln(os.Stdout, "| ID | Package Name | Fingerprints |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- |")
	for _, item := range resp.Data {
		fmt.Fprintf(os.Stdout, "| %s | %s | %s |\n",
			escapeMarkdown(item.ID),
			escapeMarkdown(item.Attributes.PackageName),
			escapeMarkdown(formatAndroidToIosFingerprints(item.Attributes.AppSigningKeyPublicCertificateSha256Fingerprints)),
		)
	}
	return nil
}

func printAndroidToIosAppMappingDeleteResultTable(result *AndroidToIosAppMappingDeleteResult) error {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	RenderTable(headers, rows)
	return nil
}

func printAndroidToIosAppMappingDeleteResultMarkdown(result *AndroidToIosAppMappingDeleteResult) error {
	fmt.Fprintln(os.Stdout, "| ID | Deleted |")
	fmt.Fprintln(os.Stdout, "| --- | --- |")
	fmt.Fprintf(os.Stdout, "| %s | %t |\n",
		escapeMarkdown(result.ID),
		result.Deleted,
	)
	return nil
}

func formatAndroidToIosFingerprints(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return strings.Join(values, ", ")
}
