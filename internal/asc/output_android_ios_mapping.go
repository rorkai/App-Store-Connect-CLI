package asc

import (
	"strings"
)

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

func formatAndroidToIosFingerprints(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return strings.Join(values, ", ")
}
