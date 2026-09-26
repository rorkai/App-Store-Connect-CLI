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
		rows = append(rows, bundleIDRow(item))
	}
	return headers, rows
}

func bundleIDResponseRows(resp *BundleIDResponse) ([]string, [][]string) {
	item := resp.Data
	return []string{"ID", "Name", "Identifier", "Platform", "Seed ID"}, [][]string{bundleIDRow(Resource[BundleIDAttributes]{
		ID:         item.ID,
		Attributes: item.Attributes,
	})}
}

func bundleIDRow(item Resource[BundleIDAttributes]) []string {
	return []string{
		item.ID,
		compactWhitespace(item.Attributes.Name),
		item.Attributes.Identifier,
		string(item.Attributes.Platform),
		item.Attributes.SeedID,
	}
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

func bundleIDDeleteResultRows(result *BundleIDDeleteResult) ([]string, [][]string) {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	return headers, rows
}

func bundleIDCapabilityDeleteResultRows(result *BundleIDCapabilityDeleteResult) ([]string, [][]string) {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	return headers, rows
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

func certificateRevokeResultRows(result *CertificateRevokeResult) ([]string, [][]string) {
	headers := []string{"ID", "Revoked"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Revoked)}}
	return headers, rows
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

func profileDeleteResultRows(result *ProfileDeleteResult) ([]string, [][]string) {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	return headers, rows
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
	if stale := result.StaleProfiles; stale != nil {
		headers = append(headers, "Stale Dry Run", "Stale Planned", "Stale Deleted", "Stale Failed")
		failed := make([]string, 0, len(stale.Failed))
		for _, item := range stale.Failed {
			failed = append(failed, item.ID+": "+item.Error)
		}
		rows[0] = append(
			rows[0],
			fmt.Sprintf("%t", stale.DryRun),
			joinSigningList(signingStaleProfileIDs(stale.Planned)),
			joinSigningList(signingStaleProfileIDs(stale.Deleted)),
			joinSigningList(failed),
		)
	}
	return headers, rows
}

func signingStaleProfileIDs(profiles []SigningStaleProfile) []string {
	ids := make([]string, 0, len(profiles))
	for _, profile := range profiles {
		ids = append(ids, profile.ID)
	}
	return ids
}

func formatCapabilitySettings(settings []CapabilitySetting) string {
	if len(settings) == 0 {
		return ""
	}
	payload, err := json.Marshal(settings)
	if err != nil {
		return ""
	}
	return SanitizeTerminalText(string(payload))
}

func certificateDisplayName(attrs CertificateAttributes) string {
	if strings.TrimSpace(attrs.DisplayName) != "" {
		return attrs.DisplayName
	}
	return attrs.Name
}
