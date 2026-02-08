package asc

import (
	"fmt"
	"os"
)

type routingAppCoverageField struct {
	Name  string
	Value string
}

func printRoutingAppCoverageTable(resp *RoutingAppCoverageResponse) error {
	fields := routingAppCoverageFields(resp)
	headers := []string{"Field", "Value"}
	rows := make([][]string, 0, len(fields))
	for _, field := range fields {
		rows = append(rows, []string{field.Name, field.Value})
	}
	RenderTable(headers, rows)
	return nil
}

func printRoutingAppCoverageMarkdown(resp *RoutingAppCoverageResponse) error {
	fields := routingAppCoverageFields(resp)
	fmt.Fprintln(os.Stdout, "| Field | Value |")
	fmt.Fprintln(os.Stdout, "| --- | --- |")
	for _, field := range fields {
		fmt.Fprintf(os.Stdout, "| %s | %s |\n", escapeMarkdown(field.Name), escapeMarkdown(field.Value))
	}
	return nil
}

func routingAppCoverageFields(resp *RoutingAppCoverageResponse) []routingAppCoverageField {
	if resp == nil {
		return nil
	}
	attrs := resp.Data.Attributes
	return []routingAppCoverageField{
		{Name: "ID", Value: fallbackValue(resp.Data.ID)},
		{Name: "Type", Value: fallbackValue(string(resp.Data.Type))},
		{Name: "File Name", Value: fallbackValue(attrs.FileName)},
		{Name: "File Size", Value: formatAttachmentFileSize(attrs.FileSize)},
		{Name: "Source File Checksum", Value: fallbackValue(attrs.SourceFileChecksum)},
		{Name: "Delivery State", Value: formatAssetDeliveryState(attrs.AssetDeliveryState)},
	}
}

func printRoutingAppCoverageDeleteResultTable(result *RoutingAppCoverageDeleteResult) error {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	RenderTable(headers, rows)
	return nil
}

func printRoutingAppCoverageDeleteResultMarkdown(result *RoutingAppCoverageDeleteResult) error {
	fmt.Fprintln(os.Stdout, "| ID | Deleted |")
	fmt.Fprintln(os.Stdout, "| --- | --- |")
	fmt.Fprintf(os.Stdout, "| %s | %t |\n", escapeMarkdown(result.ID), result.Deleted)
	return nil
}
