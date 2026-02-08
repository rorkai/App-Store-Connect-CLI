package asc

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// BundleIDDeleteResult represents CLI output for bundle ID deletions.
type BundleIDDeleteResult struct {
	ID      string `json:"id"`
	Deleted bool   `json:"deleted"`
}

// BundleIDCapabilityDeleteResult represents CLI output for capability deletions.
type BundleIDCapabilityDeleteResult struct {
	ID      string `json:"id"`
	Deleted bool   `json:"deleted"`
}

// CertificateRevokeResult represents CLI output for certificate revocations.
type CertificateRevokeResult struct {
	ID      string `json:"id"`
	Revoked bool   `json:"revoked"`
}

// ProfileDeleteResult represents CLI output for profile deletions.
type ProfileDeleteResult struct {
	ID      string `json:"id"`
	Deleted bool   `json:"deleted"`
}

// ProfileDownloadResult represents CLI output for profile downloads.
type ProfileDownloadResult struct {
	ID         string `json:"id"`
	Name       string `json:"name,omitempty"`
	OutputPath string `json:"outputPath"`
}

func printBundleIDsTable(resp *BundleIDsResponse) error {
	headers := []string{"ID", "Name", "Identifier", "Platform", "Seed ID"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{
			item.ID,
			compactWhitespace(item.Attributes.Name),
			item.Attributes.Identifier,
			string(item.Attributes.Platform),
			item.Attributes.SeedID,
		})
	}
	RenderTable(headers, rows)
	return nil
}

func printBundleIDsMarkdown(resp *BundleIDsResponse) error {
	fmt.Fprintln(os.Stdout, "| ID | Name | Identifier | Platform | Seed ID |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- | --- | --- |")
	for _, item := range resp.Data {
		fmt.Fprintf(os.Stdout, "| %s | %s | %s | %s | %s |\n",
			item.ID,
			escapeMarkdown(item.Attributes.Name),
			escapeMarkdown(item.Attributes.Identifier),
			escapeMarkdown(string(item.Attributes.Platform)),
			escapeMarkdown(item.Attributes.SeedID),
		)
	}
	return nil
}

func printBundleIDCapabilitiesTable(resp *BundleIDCapabilitiesResponse) error {
	headers := []string{"ID", "Capability", "Settings"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{
			item.ID,
			item.Attributes.CapabilityType,
			formatCapabilitySettings(item.Attributes.Settings),
		})
	}
	RenderTable(headers, rows)
	return nil
}

func printBundleIDCapabilitiesMarkdown(resp *BundleIDCapabilitiesResponse) error {
	fmt.Fprintln(os.Stdout, "| ID | Capability | Settings |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- |")
	for _, item := range resp.Data {
		fmt.Fprintf(os.Stdout, "| %s | %s | %s |\n",
			item.ID,
			escapeMarkdown(item.Attributes.CapabilityType),
			escapeMarkdown(formatCapabilitySettings(item.Attributes.Settings)),
		)
	}
	return nil
}

func printBundleIDDeleteResultTable(result *BundleIDDeleteResult) error {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	RenderTable(headers, rows)
	return nil
}

func printBundleIDDeleteResultMarkdown(result *BundleIDDeleteResult) error {
	fmt.Fprintln(os.Stdout, "| ID | Deleted |")
	fmt.Fprintln(os.Stdout, "| --- | --- |")
	fmt.Fprintf(os.Stdout, "| %s | %t |\n",
		escapeMarkdown(result.ID),
		result.Deleted,
	)
	return nil
}

func printBundleIDCapabilityDeleteResultTable(result *BundleIDCapabilityDeleteResult) error {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	RenderTable(headers, rows)
	return nil
}

func printBundleIDCapabilityDeleteResultMarkdown(result *BundleIDCapabilityDeleteResult) error {
	fmt.Fprintln(os.Stdout, "| ID | Deleted |")
	fmt.Fprintln(os.Stdout, "| --- | --- |")
	fmt.Fprintf(os.Stdout, "| %s | %t |\n",
		escapeMarkdown(result.ID),
		result.Deleted,
	)
	return nil
}

func printCertificatesTable(resp *CertificatesResponse) error {
	headers := []string{"ID", "Name", "Type", "Expiration", "Serial"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{
			item.ID,
			compactWhitespace(certificateDisplayName(item.Attributes)),
			item.Attributes.CertificateType,
			item.Attributes.ExpirationDate,
			item.Attributes.SerialNumber,
		})
	}
	RenderTable(headers, rows)
	return nil
}

func printCertificatesMarkdown(resp *CertificatesResponse) error {
	fmt.Fprintln(os.Stdout, "| ID | Name | Type | Expiration | Serial |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- | --- | --- |")
	for _, item := range resp.Data {
		fmt.Fprintf(os.Stdout, "| %s | %s | %s | %s | %s |\n",
			item.ID,
			escapeMarkdown(certificateDisplayName(item.Attributes)),
			escapeMarkdown(item.Attributes.CertificateType),
			escapeMarkdown(item.Attributes.ExpirationDate),
			escapeMarkdown(item.Attributes.SerialNumber),
		)
	}
	return nil
}

func printCertificateRevokeResultTable(result *CertificateRevokeResult) error {
	headers := []string{"ID", "Revoked"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Revoked)}}
	RenderTable(headers, rows)
	return nil
}

func printCertificateRevokeResultMarkdown(result *CertificateRevokeResult) error {
	fmt.Fprintln(os.Stdout, "| ID | Revoked |")
	fmt.Fprintln(os.Stdout, "| --- | --- |")
	fmt.Fprintf(os.Stdout, "| %s | %t |\n",
		escapeMarkdown(result.ID),
		result.Revoked,
	)
	return nil
}

func printProfilesTable(resp *ProfilesResponse) error {
	headers := []string{"ID", "Name", "Type", "State", "Expiration"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{
			item.ID,
			compactWhitespace(item.Attributes.Name),
			item.Attributes.ProfileType,
			string(item.Attributes.ProfileState),
			item.Attributes.ExpirationDate,
		})
	}
	RenderTable(headers, rows)
	return nil
}

func printProfilesMarkdown(resp *ProfilesResponse) error {
	fmt.Fprintln(os.Stdout, "| ID | Name | Type | State | Expiration |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- | --- | --- |")
	for _, item := range resp.Data {
		fmt.Fprintf(os.Stdout, "| %s | %s | %s | %s | %s |\n",
			item.ID,
			escapeMarkdown(item.Attributes.Name),
			escapeMarkdown(item.Attributes.ProfileType),
			escapeMarkdown(string(item.Attributes.ProfileState)),
			escapeMarkdown(item.Attributes.ExpirationDate),
		)
	}
	return nil
}

func printProfileDeleteResultTable(result *ProfileDeleteResult) error {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	RenderTable(headers, rows)
	return nil
}

func printProfileDeleteResultMarkdown(result *ProfileDeleteResult) error {
	fmt.Fprintln(os.Stdout, "| ID | Deleted |")
	fmt.Fprintln(os.Stdout, "| --- | --- |")
	fmt.Fprintf(os.Stdout, "| %s | %t |\n",
		escapeMarkdown(result.ID),
		result.Deleted,
	)
	return nil
}

func printProfileDownloadResultTable(result *ProfileDownloadResult) error {
	headers := []string{"ID", "Name", "Output Path"}
	rows := [][]string{{
		result.ID,
		compactWhitespace(result.Name),
		result.OutputPath,
	}}
	RenderTable(headers, rows)
	return nil
}

func printProfileDownloadResultMarkdown(result *ProfileDownloadResult) error {
	fmt.Fprintln(os.Stdout, "| ID | Name | Output Path |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- |")
	fmt.Fprintf(os.Stdout, "| %s | %s | %s |\n",
		escapeMarkdown(result.ID),
		escapeMarkdown(result.Name),
		escapeMarkdown(result.OutputPath),
	)
	return nil
}

func joinSigningList(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return strings.Join(values, ", ")
}

func printSigningFetchResultTable(result *SigningFetchResult) error {
	headers := []string{"Bundle ID", "Bundle ID Resource", "Profile Type", "Profile ID", "Profile File", "Certificate IDs", "Certificate Files", "Created"}
	rows := [][]string{{
		result.BundleID,
		result.BundleIDResource,
		result.ProfileType,
		result.ProfileID,
		result.ProfileFile,
		joinSigningList(result.CertificateIDs),
		joinSigningList(result.CertificateFiles),
		fmt.Sprintf("%t", result.Created),
	}}
	RenderTable(headers, rows)
	return nil
}

func printSigningFetchResultMarkdown(result *SigningFetchResult) error {
	fmt.Fprintln(os.Stdout, "| Bundle ID | Bundle ID Resource | Profile Type | Profile ID | Profile File | Certificate IDs | Certificate Files | Created |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- | --- | --- | --- | --- | --- |")
	fmt.Fprintf(os.Stdout, "| %s | %s | %s | %s | %s | %s | %s | %t |\n",
		escapeMarkdown(result.BundleID),
		escapeMarkdown(result.BundleIDResource),
		escapeMarkdown(result.ProfileType),
		escapeMarkdown(result.ProfileID),
		escapeMarkdown(result.ProfileFile),
		escapeMarkdown(joinSigningList(result.CertificateIDs)),
		escapeMarkdown(joinSigningList(result.CertificateFiles)),
		result.Created,
	)
	return nil
}

func formatCapabilitySettings(settings []CapabilitySetting) string {
	if len(settings) == 0 {
		return ""
	}
	payload, err := json.Marshal(settings)
	if err != nil {
		return ""
	}
	return sanitizeTerminal(string(payload))
}

func certificateDisplayName(attrs CertificateAttributes) string {
	if strings.TrimSpace(attrs.DisplayName) != "" {
		return attrs.DisplayName
	}
	return attrs.Name
}
