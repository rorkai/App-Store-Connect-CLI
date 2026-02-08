package asc

import (
	"fmt"
	"strings"
)

func alternativeDistributionDomainsRows(resp *AlternativeDistributionDomainsResponse) ([]string, [][]string) {
	headers := []string{"ID", "Domain", "Reference Name", "Created Date"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{
			item.ID,
			compactWhitespace(item.Attributes.Domain),
			compactWhitespace(item.Attributes.ReferenceName),
			item.Attributes.CreatedDate,
		})
	}
	return headers, rows
}

func alternativeDistributionKeysRows(resp *AlternativeDistributionKeysResponse) ([]string, [][]string) {
	headers := []string{"ID", "Public Key"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{
			item.ID,
			compactWhitespace(item.Attributes.PublicKey),
		})
	}
	return headers, rows
}

func alternativeDistributionPackageVersionsRows(resp *AlternativeDistributionPackageVersionsResponse) ([]string, [][]string) {
	headers := []string{"ID", "Version", "State", "File Checksum", "URL", "URL Expiration Date"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{
			item.ID,
			compactWhitespace(item.Attributes.Version),
			compactWhitespace(string(item.Attributes.State)),
			compactWhitespace(item.Attributes.FileChecksum),
			compactWhitespace(item.Attributes.URL),
			compactWhitespace(item.Attributes.URLExpirationDate),
		})
	}
	return headers, rows
}

func alternativeDistributionPackageVariantsRows(resp *AlternativeDistributionPackageVariantsResponse) ([]string, [][]string) {
	headers := []string{"ID", "URL", "URL Expiration Date", "Key Blob", "File Checksum"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{
			item.ID,
			compactWhitespace(item.Attributes.URL),
			compactWhitespace(item.Attributes.URLExpirationDate),
			compactWhitespace(item.Attributes.AlternativeDistributionKeyBlob),
			compactWhitespace(item.Attributes.FileChecksum),
		})
	}
	return headers, rows
}

func alternativeDistributionPackageDeltasRows(resp *AlternativeDistributionPackageDeltasResponse) ([]string, [][]string) {
	headers := []string{"ID", "URL", "URL Expiration Date", "Key Blob", "File Checksum"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{
			item.ID,
			compactWhitespace(item.Attributes.URL),
			compactWhitespace(item.Attributes.URLExpirationDate),
			compactWhitespace(item.Attributes.AlternativeDistributionKeyBlob),
			compactWhitespace(item.Attributes.FileChecksum),
		})
	}
	return headers, rows
}

func alternativeDistributionPackageRows(resp *AlternativeDistributionPackageResponse) ([]string, [][]string) {
	headers := []string{"ID", "Source File Checksum"}
	rows := [][]string{{
		resp.Data.ID,
		compactWhitespace(formatAlternativeDistributionChecksums(resp.Data.Attributes.SourceFileChecksum)),
	}}
	return headers, rows
}

func formatAlternativeDistributionChecksums(checksums *Checksums) string {
	if checksums == nil {
		return ""
	}
	parts := []string{}
	if checksums.File != nil {
		parts = append(parts, formatAlternativeDistributionChecksum("file", checksums.File))
	}
	if checksums.Composite != nil {
		parts = append(parts, formatAlternativeDistributionChecksum("composite", checksums.Composite))
	}
	return strings.Join(parts, ", ")
}

func formatAlternativeDistributionChecksum(label string, checksum *Checksum) string {
	if checksum == nil {
		return ""
	}
	if checksum.Algorithm != "" {
		return fmt.Sprintf("%s:%s (%s)", label, checksum.Hash, checksum.Algorithm)
	}
	return fmt.Sprintf("%s:%s", label, checksum.Hash)
}

func alternativeDistributionDeleteResultRows(id string, deleted bool) ([]string, [][]string) {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{id, fmt.Sprintf("%t", deleted)}}
	return headers, rows
}
