package asc

import (
	"fmt"
	"os"
	"strings"
)

func printAlternativeDistributionDomainsTable(resp *AlternativeDistributionDomainsResponse) error {
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
	RenderTable(headers, rows)
	return nil
}

func printAlternativeDistributionDomainsMarkdown(resp *AlternativeDistributionDomainsResponse) error {
	fmt.Fprintln(os.Stdout, "| ID | Domain | Reference Name | Created Date |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- | --- |")
	for _, item := range resp.Data {
		fmt.Fprintf(os.Stdout, "| %s | %s | %s | %s |\n",
			escapeMarkdown(item.ID),
			escapeMarkdown(item.Attributes.Domain),
			escapeMarkdown(item.Attributes.ReferenceName),
			escapeMarkdown(item.Attributes.CreatedDate),
		)
	}
	return nil
}

func printAlternativeDistributionKeysTable(resp *AlternativeDistributionKeysResponse) error {
	headers := []string{"ID", "Public Key"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{
			item.ID,
			compactWhitespace(item.Attributes.PublicKey),
		})
	}
	RenderTable(headers, rows)
	return nil
}

func printAlternativeDistributionKeysMarkdown(resp *AlternativeDistributionKeysResponse) error {
	fmt.Fprintln(os.Stdout, "| ID | Public Key |")
	fmt.Fprintln(os.Stdout, "| --- | --- |")
	for _, item := range resp.Data {
		fmt.Fprintf(os.Stdout, "| %s | %s |\n",
			escapeMarkdown(item.ID),
			escapeMarkdown(item.Attributes.PublicKey),
		)
	}
	return nil
}

func printAlternativeDistributionPackageVersionsTable(resp *AlternativeDistributionPackageVersionsResponse) error {
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
	RenderTable(headers, rows)
	return nil
}

func printAlternativeDistributionPackageVersionsMarkdown(resp *AlternativeDistributionPackageVersionsResponse) error {
	fmt.Fprintln(os.Stdout, "| ID | Version | State | File Checksum | URL | URL Expiration Date |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- | --- | --- | --- |")
	for _, item := range resp.Data {
		fmt.Fprintf(os.Stdout, "| %s | %s | %s | %s | %s | %s |\n",
			escapeMarkdown(item.ID),
			escapeMarkdown(item.Attributes.Version),
			escapeMarkdown(string(item.Attributes.State)),
			escapeMarkdown(item.Attributes.FileChecksum),
			escapeMarkdown(item.Attributes.URL),
			escapeMarkdown(item.Attributes.URLExpirationDate),
		)
	}
	return nil
}

func printAlternativeDistributionPackageVariantsTable(resp *AlternativeDistributionPackageVariantsResponse) error {
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
	RenderTable(headers, rows)
	return nil
}

func printAlternativeDistributionPackageVariantsMarkdown(resp *AlternativeDistributionPackageVariantsResponse) error {
	fmt.Fprintln(os.Stdout, "| ID | URL | URL Expiration Date | Key Blob | File Checksum |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- | --- | --- |")
	for _, item := range resp.Data {
		fmt.Fprintf(os.Stdout, "| %s | %s | %s | %s | %s |\n",
			escapeMarkdown(item.ID),
			escapeMarkdown(item.Attributes.URL),
			escapeMarkdown(item.Attributes.URLExpirationDate),
			escapeMarkdown(item.Attributes.AlternativeDistributionKeyBlob),
			escapeMarkdown(item.Attributes.FileChecksum),
		)
	}
	return nil
}

func printAlternativeDistributionPackageDeltasTable(resp *AlternativeDistributionPackageDeltasResponse) error {
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
	RenderTable(headers, rows)
	return nil
}

func printAlternativeDistributionPackageDeltasMarkdown(resp *AlternativeDistributionPackageDeltasResponse) error {
	fmt.Fprintln(os.Stdout, "| ID | URL | URL Expiration Date | Key Blob | File Checksum |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- | --- | --- |")
	for _, item := range resp.Data {
		fmt.Fprintf(os.Stdout, "| %s | %s | %s | %s | %s |\n",
			escapeMarkdown(item.ID),
			escapeMarkdown(item.Attributes.URL),
			escapeMarkdown(item.Attributes.URLExpirationDate),
			escapeMarkdown(item.Attributes.AlternativeDistributionKeyBlob),
			escapeMarkdown(item.Attributes.FileChecksum),
		)
	}
	return nil
}

func printAlternativeDistributionPackageTable(resp *AlternativeDistributionPackageResponse) error {
	headers := []string{"ID", "Source File Checksum"}
	rows := [][]string{{
		resp.Data.ID,
		compactWhitespace(formatAlternativeDistributionChecksums(resp.Data.Attributes.SourceFileChecksum)),
	}}
	RenderTable(headers, rows)
	return nil
}

func printAlternativeDistributionPackageMarkdown(resp *AlternativeDistributionPackageResponse) error {
	fmt.Fprintln(os.Stdout, "| ID | Source File Checksum |")
	fmt.Fprintln(os.Stdout, "| --- | --- |")
	fmt.Fprintf(os.Stdout, "| %s | %s |\n",
		escapeMarkdown(resp.Data.ID),
		escapeMarkdown(formatAlternativeDistributionChecksums(resp.Data.Attributes.SourceFileChecksum)),
	)
	return nil
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

func printAlternativeDistributionDeleteResultTable(id string, deleted bool) error {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{id, fmt.Sprintf("%t", deleted)}}
	RenderTable(headers, rows)
	return nil
}

func printAlternativeDistributionDeleteResultMarkdown(id string, deleted bool) error {
	fmt.Fprintln(os.Stdout, "| ID | Deleted |")
	fmt.Fprintln(os.Stdout, "| --- | --- |")
	fmt.Fprintf(os.Stdout, "| %s | %t |\n", escapeMarkdown(id), deleted)
	return nil
}

func printAlternativeDistributionDomainDeleteResultTable(result *AlternativeDistributionDomainDeleteResult) error {
	return printAlternativeDistributionDeleteResultTable(result.ID, result.Deleted)
}

func printAlternativeDistributionDomainDeleteResultMarkdown(result *AlternativeDistributionDomainDeleteResult) error {
	return printAlternativeDistributionDeleteResultMarkdown(result.ID, result.Deleted)
}

func printAlternativeDistributionKeyDeleteResultTable(result *AlternativeDistributionKeyDeleteResult) error {
	return printAlternativeDistributionDeleteResultTable(result.ID, result.Deleted)
}

func printAlternativeDistributionKeyDeleteResultMarkdown(result *AlternativeDistributionKeyDeleteResult) error {
	return printAlternativeDistributionDeleteResultMarkdown(result.ID, result.Deleted)
}
