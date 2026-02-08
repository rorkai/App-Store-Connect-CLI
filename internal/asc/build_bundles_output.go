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

func printBuildBundlesTable(resp *BuildBundlesResponse) error {
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
	RenderTable(headers, rows)
	return nil
}

func printBuildBundlesMarkdown(resp *BuildBundlesResponse) error {
	fmt.Fprintln(os.Stdout, "| ID | Bundle ID | Type | File Name | SDK Build | Platform Build |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- | --- | --- | --- |")
	for _, item := range resp.Data {
		attrs := item.Attributes
		fmt.Fprintf(os.Stdout, "| %s | %s | %s | %s | %s | %s |\n",
			escapeMarkdown(item.ID),
			escapeMarkdown(stringValue(attrs.BundleID)),
			escapeMarkdown(buildBundleTypeValue(attrs.BundleType)),
			escapeMarkdown(stringValue(attrs.FileName)),
			escapeMarkdown(stringValue(attrs.SDKBuild)),
			escapeMarkdown(stringValue(attrs.PlatformBuild)),
		)
	}
	return nil
}

func printBuildBundleFileSizesTable(resp *BuildBundleFileSizesResponse) error {
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
	RenderTable(headers, rows)
	return nil
}

func printBuildBundleFileSizesMarkdown(resp *BuildBundleFileSizesResponse) error {
	fmt.Fprintln(os.Stdout, "| ID | Device Model | OS Version | Download Bytes | Install Bytes |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- | --- | --- |")
	for _, item := range resp.Data {
		attrs := item.Attributes
		fmt.Fprintf(os.Stdout, "| %s | %s | %s | %s | %s |\n",
			escapeMarkdown(item.ID),
			escapeMarkdown(stringValue(attrs.DeviceModel)),
			escapeMarkdown(stringValue(attrs.OSVersion)),
			escapeMarkdown(int64Value(attrs.DownloadBytes)),
			escapeMarkdown(int64Value(attrs.InstallBytes)),
		)
	}
	return nil
}

func printBetaAppClipInvocationsTable(resp *BetaAppClipInvocationsResponse) error {
	headers := []string{"ID", "URL"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{item.ID, stringValue(item.Attributes.URL)})
	}
	RenderTable(headers, rows)
	return nil
}

func printBetaAppClipInvocationsMarkdown(resp *BetaAppClipInvocationsResponse) error {
	fmt.Fprintln(os.Stdout, "| ID | URL |")
	fmt.Fprintln(os.Stdout, "| --- | --- |")
	for _, item := range resp.Data {
		fmt.Fprintf(os.Stdout, "| %s | %s |\n",
			escapeMarkdown(item.ID),
			escapeMarkdown(stringValue(item.Attributes.URL)),
		)
	}
	return nil
}

func printAppClipDomainStatusResultTable(result *AppClipDomainStatusResult) error {
	headers := []string{"Build Bundle ID", "Available", "Status ID", "Last Updated"}
	rows := [][]string{{
		result.BuildBundleID,
		fmt.Sprintf("%t", result.Available),
		result.StatusID,
		stringValue(result.LastUpdatedDate),
	}}
	RenderTable(headers, rows)
	if len(result.Domains) == 0 {
		return nil
	}
	fmt.Fprintln(os.Stdout, "\nDomains")
	domainHeaders := []string{"Domain", "Valid", "Last Updated", "Error"}
	domainRows := make([][]string, 0, len(result.Domains))
	for _, domain := range result.Domains {
		domainRows = append(domainRows, []string{
			stringValue(domain.Domain),
			boolValue(domain.IsValid),
			stringValue(domain.LastUpdatedDate),
			stringValue(domain.ErrorCode),
		})
	}
	RenderTable(domainHeaders, domainRows)
	return nil
}

func printAppClipDomainStatusResultMarkdown(result *AppClipDomainStatusResult) error {
	fmt.Fprintln(os.Stdout, "| Build Bundle ID | Available | Status ID | Last Updated |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- | --- |")
	fmt.Fprintf(os.Stdout, "| %s | %t | %s | %s |\n",
		escapeMarkdown(result.BuildBundleID),
		result.Available,
		escapeMarkdown(result.StatusID),
		escapeMarkdown(stringValue(result.LastUpdatedDate)),
	)
	if len(result.Domains) == 0 {
		return nil
	}
	fmt.Fprintln(os.Stdout, "\n| Domain | Valid | Last Updated | Error |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- | --- |")
	for _, domain := range result.Domains {
		fmt.Fprintf(os.Stdout, "| %s | %s | %s | %s |\n",
			escapeMarkdown(stringValue(domain.Domain)),
			boolValue(domain.IsValid),
			escapeMarkdown(stringValue(domain.LastUpdatedDate)),
			escapeMarkdown(stringValue(domain.ErrorCode)),
		)
	}
	return nil
}
