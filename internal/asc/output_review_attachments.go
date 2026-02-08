package asc

import "fmt"

type appStoreReviewAttachmentField struct {
	Name  string
	Value string
}

func appStoreReviewAttachmentsRows(resp *AppStoreReviewAttachmentsResponse) ([]string, [][]string) {
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
	return headers, rows
}

func printAppStoreReviewAttachmentsTable(resp *AppStoreReviewAttachmentsResponse) error {
	h, r := appStoreReviewAttachmentsRows(resp)
	RenderTable(h, r)
	return nil
}

func printAppStoreReviewAttachmentsMarkdown(resp *AppStoreReviewAttachmentsResponse) error {
	h, r := appStoreReviewAttachmentsRows(resp)
	RenderMarkdown(h, r)
	return nil
}

func appStoreReviewAttachmentRows(resp *AppStoreReviewAttachmentResponse) ([]string, [][]string) {
	fields := appStoreReviewAttachmentFields(resp)
	headers := []string{"Field", "Value"}
	rows := make([][]string, 0, len(fields))
	for _, field := range fields {
		rows = append(rows, []string{field.Name, field.Value})
	}
	return headers, rows
}

func printAppStoreReviewAttachmentTable(resp *AppStoreReviewAttachmentResponse) error {
	h, r := appStoreReviewAttachmentRows(resp)
	RenderTable(h, r)
	return nil
}

func printAppStoreReviewAttachmentMarkdown(resp *AppStoreReviewAttachmentResponse) error {
	h, r := appStoreReviewAttachmentRows(resp)
	RenderMarkdown(h, r)
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

func appStoreReviewAttachmentDeleteResultRows(result *AppStoreReviewAttachmentDeleteResult) ([]string, [][]string) {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	return headers, rows
}

func printAppStoreReviewAttachmentDeleteResultTable(result *AppStoreReviewAttachmentDeleteResult) error {
	h, r := appStoreReviewAttachmentDeleteResultRows(result)
	RenderTable(h, r)
	return nil
}

func printAppStoreReviewAttachmentDeleteResultMarkdown(result *AppStoreReviewAttachmentDeleteResult) error {
	h, r := appStoreReviewAttachmentDeleteResultRows(result)
	RenderMarkdown(h, r)
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
