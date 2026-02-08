package asc

import (
	"encoding/json"
	"fmt"
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

func bundleIDsRows(resp *BundleIDsResponse) ([]string, [][]string) {
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
	return headers, rows
}

func printBundleIDsTable(resp *BundleIDsResponse) error {
	h, r := bundleIDsRows(resp)
	RenderTable(h, r)
	return nil
}

func printBundleIDsMarkdown(resp *BundleIDsResponse) error {
	h, r := bundleIDsRows(resp)
	RenderMarkdown(h, r)
	return nil
}

func bundleIDCapabilitiesRows(resp *BundleIDCapabilitiesResponse) ([]string, [][]string) {
	headers := []string{"ID", "Capability", "Settings"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{
			item.ID,
			item.Attributes.CapabilityType,
			formatCapabilitySettings(item.Attributes.Settings),
		})
	}
	return headers, rows
}

func printBundleIDCapabilitiesTable(resp *BundleIDCapabilitiesResponse) error {
	h, r := bundleIDCapabilitiesRows(resp)
	RenderTable(h, r)
	return nil
}

func printBundleIDCapabilitiesMarkdown(resp *BundleIDCapabilitiesResponse) error {
	h, r := bundleIDCapabilitiesRows(resp)
	RenderMarkdown(h, r)
	return nil
}

func bundleIDDeleteResultRows(result *BundleIDDeleteResult) ([]string, [][]string) {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	return headers, rows
}

func printBundleIDDeleteResultTable(result *BundleIDDeleteResult) error {
	h, r := bundleIDDeleteResultRows(result)
	RenderTable(h, r)
	return nil
}

func printBundleIDDeleteResultMarkdown(result *BundleIDDeleteResult) error {
	h, r := bundleIDDeleteResultRows(result)
	RenderMarkdown(h, r)
	return nil
}

func bundleIDCapabilityDeleteResultRows(result *BundleIDCapabilityDeleteResult) ([]string, [][]string) {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	return headers, rows
}

func printBundleIDCapabilityDeleteResultTable(result *BundleIDCapabilityDeleteResult) error {
	h, r := bundleIDCapabilityDeleteResultRows(result)
	RenderTable(h, r)
	return nil
}

func printBundleIDCapabilityDeleteResultMarkdown(result *BundleIDCapabilityDeleteResult) error {
	h, r := bundleIDCapabilityDeleteResultRows(result)
	RenderMarkdown(h, r)
	return nil
}

func certificatesRows(resp *CertificatesResponse) ([]string, [][]string) {
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
	return headers, rows
}

func printCertificatesTable(resp *CertificatesResponse) error {
	h, r := certificatesRows(resp)
	RenderTable(h, r)
	return nil
}

func printCertificatesMarkdown(resp *CertificatesResponse) error {
	h, r := certificatesRows(resp)
	RenderMarkdown(h, r)
	return nil
}

func certificateRevokeResultRows(result *CertificateRevokeResult) ([]string, [][]string) {
	headers := []string{"ID", "Revoked"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Revoked)}}
	return headers, rows
}

func printCertificateRevokeResultTable(result *CertificateRevokeResult) error {
	h, r := certificateRevokeResultRows(result)
	RenderTable(h, r)
	return nil
}

func printCertificateRevokeResultMarkdown(result *CertificateRevokeResult) error {
	h, r := certificateRevokeResultRows(result)
	RenderMarkdown(h, r)
	return nil
}

func profilesRows(resp *ProfilesResponse) ([]string, [][]string) {
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
	return headers, rows
}

func printProfilesTable(resp *ProfilesResponse) error {
	h, r := profilesRows(resp)
	RenderTable(h, r)
	return nil
}

func printProfilesMarkdown(resp *ProfilesResponse) error {
	h, r := profilesRows(resp)
	RenderMarkdown(h, r)
	return nil
}

func profileDeleteResultRows(result *ProfileDeleteResult) ([]string, [][]string) {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	return headers, rows
}

func printProfileDeleteResultTable(result *ProfileDeleteResult) error {
	h, r := profileDeleteResultRows(result)
	RenderTable(h, r)
	return nil
}

func printProfileDeleteResultMarkdown(result *ProfileDeleteResult) error {
	h, r := profileDeleteResultRows(result)
	RenderMarkdown(h, r)
	return nil
}

func profileDownloadResultRows(result *ProfileDownloadResult) ([]string, [][]string) {
	headers := []string{"ID", "Name", "Output Path"}
	rows := [][]string{{
		result.ID,
		compactWhitespace(result.Name),
		result.OutputPath,
	}}
	return headers, rows
}

func printProfileDownloadResultTable(result *ProfileDownloadResult) error {
	h, r := profileDownloadResultRows(result)
	RenderTable(h, r)
	return nil
}

func printProfileDownloadResultMarkdown(result *ProfileDownloadResult) error {
	h, r := profileDownloadResultRows(result)
	RenderMarkdown(h, r)
	return nil
}

func joinSigningList(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return strings.Join(values, ", ")
}

func signingFetchResultRows(result *SigningFetchResult) ([]string, [][]string) {
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
	return headers, rows
}

func printSigningFetchResultTable(result *SigningFetchResult) error {
	h, r := signingFetchResultRows(result)
	RenderTable(h, r)
	return nil
}

func printSigningFetchResultMarkdown(result *SigningFetchResult) error {
	h, r := signingFetchResultRows(result)
	RenderMarkdown(h, r)
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
