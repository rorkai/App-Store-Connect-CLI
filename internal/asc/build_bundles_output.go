package asc

import (
	"fmt"
	"os"
)

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func boolValue(value *bool) string {
	if value == nil {
		return ""
	}
	return fmt.Sprintf("%t", *value)
}

func int64Value(value *int64) string {
	if value == nil {
		return ""
	}
	return fmt.Sprintf("%d", *value)
}

func buildBundleTypeValue(value *BuildBundleType) string {
	if value == nil {
		return ""
	}
	return string(*value)
}

func buildBundlesRows(resp *BuildBundlesResponse) ([]string, [][]string) {
	headers := []string{"ID", "Bundle ID", "Type", "File Name", "SDK Build", "Platform Build"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		attrs := item.Attributes
		rows = append(rows, []string{
			item.ID,
			stringValue(attrs.BundleID),
			buildBundleTypeValue(attrs.BundleType),
			stringValue(attrs.FileName),
			stringValue(attrs.SDKBuild),
			stringValue(attrs.PlatformBuild),
		})
	}
	return headers, rows
}

func printBuildBundlesTable(resp *BuildBundlesResponse) error {
	h, r := buildBundlesRows(resp)
	RenderTable(h, r)
	return nil
}

func printBuildBundlesMarkdown(resp *BuildBundlesResponse) error {
	h, r := buildBundlesRows(resp)
	RenderMarkdown(h, r)
	return nil
}

func buildBundleFileSizesRows(resp *BuildBundleFileSizesResponse) ([]string, [][]string) {
	headers := []string{"ID", "Device Model", "OS Version", "Download Bytes", "Install Bytes"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		attrs := item.Attributes
		rows = append(rows, []string{
			item.ID,
			stringValue(attrs.DeviceModel),
			stringValue(attrs.OSVersion),
			int64Value(attrs.DownloadBytes),
			int64Value(attrs.InstallBytes),
		})
	}
	return headers, rows
}

func printBuildBundleFileSizesTable(resp *BuildBundleFileSizesResponse) error {
	h, r := buildBundleFileSizesRows(resp)
	RenderTable(h, r)
	return nil
}

func printBuildBundleFileSizesMarkdown(resp *BuildBundleFileSizesResponse) error {
	h, r := buildBundleFileSizesRows(resp)
	RenderMarkdown(h, r)
	return nil
}

func betaAppClipInvocationsRows(resp *BetaAppClipInvocationsResponse) ([]string, [][]string) {
	headers := []string{"ID", "URL"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{item.ID, stringValue(item.Attributes.URL)})
	}
	return headers, rows
}

func printBetaAppClipInvocationsTable(resp *BetaAppClipInvocationsResponse) error {
	h, r := betaAppClipInvocationsRows(resp)
	RenderTable(h, r)
	return nil
}

func printBetaAppClipInvocationsMarkdown(resp *BetaAppClipInvocationsResponse) error {
	h, r := betaAppClipInvocationsRows(resp)
	RenderMarkdown(h, r)
	return nil
}

func appClipDomainStatusMainRows(result *AppClipDomainStatusResult) ([]string, [][]string) {
	headers := []string{"Build Bundle ID", "Available", "Status ID", "Last Updated"}
	rows := [][]string{{
		result.BuildBundleID,
		fmt.Sprintf("%t", result.Available),
		result.StatusID,
		stringValue(result.LastUpdatedDate),
	}}
	return headers, rows
}

func appClipDomainStatusDomainRows(result *AppClipDomainStatusResult) ([]string, [][]string) {
	headers := []string{"Domain", "Valid", "Last Updated", "Error"}
	rows := make([][]string, 0, len(result.Domains))
	for _, domain := range result.Domains {
		rows = append(rows, []string{
			stringValue(domain.Domain),
			boolValue(domain.IsValid),
			stringValue(domain.LastUpdatedDate),
			stringValue(domain.ErrorCode),
		})
	}
	return headers, rows
}

func printAppClipDomainStatusResultTable(result *AppClipDomainStatusResult) error {
	h, r := appClipDomainStatusMainRows(result)
	RenderTable(h, r)
	if len(result.Domains) == 0 {
		return nil
	}
	fmt.Fprintln(os.Stdout, "\nDomains")
	dh, dr := appClipDomainStatusDomainRows(result)
	RenderTable(dh, dr)
	return nil
}

func printAppClipDomainStatusResultMarkdown(result *AppClipDomainStatusResult) error {
	h, r := appClipDomainStatusMainRows(result)
	RenderMarkdown(h, r)
	if len(result.Domains) == 0 {
		return nil
	}
	fmt.Fprintln(os.Stdout)
	dh, dr := appClipDomainStatusDomainRows(result)
	RenderMarkdown(dh, dr)
	return nil
}
