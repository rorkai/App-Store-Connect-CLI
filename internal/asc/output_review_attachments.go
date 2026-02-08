package asc

import (
	"fmt"
	"os"
)

type appStoreReviewAttachmentField struct {
	Name  string
	Value string
}

func printAppStoreReviewAttachmentsTable(resp *AppStoreReviewAttachmentsResponse) error {
	headers := []string{"ID", "File Name", "File Size", "Checksum", "Delivery State"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		attrs := item.Attributes
		rows = append(rows, []string{
			sanitizeTerminal(item.ID),
			sanitizeTerminal(fallbackValue(attrs.FileName)),
			formatAttachmentFileSize(attrs.FileSize),
			sanitizeTerminal(fallbackValue(attrs.SourceFileChecksum)),
			sanitizeTerminal(formatAssetDeliveryState(attrs.AssetDeliveryState)),
		})
	}
	RenderTable(headers, rows)
	return nil
}

func printAppStoreReviewAttachmentsMarkdown(resp *AppStoreReviewAttachmentsResponse) error {
	fmt.Fprintln(os.Stdout, "| ID | File Name | File Size | Checksum | Delivery State |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- | --- | --- |")
	for _, item := range resp.Data {
		attrs := item.Attributes
		fmt.Fprintf(os.Stdout, "| %s | %s | %s | %s | %s |\n",
			escapeMarkdown(item.ID),
			escapeMarkdown(fallbackValue(attrs.FileName)),
			escapeMarkdown(formatAttachmentFileSize(attrs.FileSize)),
			escapeMarkdown(fallbackValue(attrs.SourceFileChecksum)),
			escapeMarkdown(formatAssetDeliveryState(attrs.AssetDeliveryState)),
		)
	}
	return nil
}

func printAppStoreReviewAttachmentTable(resp *AppStoreReviewAttachmentResponse) error {
	fields := appStoreReviewAttachmentFields(resp)
	headers := []string{"Field", "Value"}
	rows := make([][]string, 0, len(fields))
	for _, field := range fields {
		rows = append(rows, []string{field.Name, field.Value})
	}
	RenderTable(headers, rows)
	return nil
}

func printAppStoreReviewAttachmentMarkdown(resp *AppStoreReviewAttachmentResponse) error {
	fields := appStoreReviewAttachmentFields(resp)
	fmt.Fprintln(os.Stdout, "| Field | Value |")
	fmt.Fprintln(os.Stdout, "| --- | --- |")
	for _, field := range fields {
		fmt.Fprintf(os.Stdout, "| %s | %s |\n", escapeMarkdown(field.Name), escapeMarkdown(field.Value))
	}
	return nil
}

func appStoreReviewAttachmentFields(resp *AppStoreReviewAttachmentResponse) []appStoreReviewAttachmentField {
	if resp == nil {
		return nil
	}
	attrs := resp.Data.Attributes
	return []appStoreReviewAttachmentField{
		{Name: "ID", Value: fallbackValue(resp.Data.ID)},
		{Name: "Type", Value: fallbackValue(string(resp.Data.Type))},
		{Name: "File Name", Value: fallbackValue(attrs.FileName)},
		{Name: "File Size", Value: formatAttachmentFileSize(attrs.FileSize)},
		{Name: "Source File Checksum", Value: fallbackValue(attrs.SourceFileChecksum)},
		{Name: "Delivery State", Value: formatAssetDeliveryState(attrs.AssetDeliveryState)},
	}
}

func printAppStoreReviewAttachmentDeleteResultTable(result *AppStoreReviewAttachmentDeleteResult) error {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	RenderTable(headers, rows)
	return nil
}

func printAppStoreReviewAttachmentDeleteResultMarkdown(result *AppStoreReviewAttachmentDeleteResult) error {
	fmt.Fprintln(os.Stdout, "| ID | Deleted |")
	fmt.Fprintln(os.Stdout, "| --- | --- |")
	fmt.Fprintf(os.Stdout, "| %s | %t |\n", escapeMarkdown(result.ID), result.Deleted)
	return nil
}

func formatAssetDeliveryState(state *AppMediaAssetState) string {
	if state == nil || state.State == nil {
		return ""
	}
	return *state.State
}

func formatAttachmentFileSize(size int64) string {
	if size <= 0 {
		return ""
	}
	return fmt.Sprintf("%d", size)
}
