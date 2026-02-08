package asc

import (
	"fmt"
	"strconv"
	"strings"
)

// WebhookDeleteResult represents CLI output for webhook deletions.
type WebhookDeleteResult struct {
	ID      string `json:"id"`
	Deleted bool   `json:"deleted"`
}

func webhookEventTypes(values []WebhookEventType) string {
	if len(values) == 0 {
		return ""
	}
	items := make([]string, 0, len(values))
	for _, value := range values {
		items = append(items, string(value))
	}
	return strings.Join(items, ", ")
}

func webhooksRows(resp *WebhooksResponse) ([]string, [][]string) {
	headers := []string{"ID", "Name", "Enabled", "URL", "Events"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{
			item.ID,
			compactWhitespace(item.Attributes.Name),
			strconv.FormatBool(item.Attributes.Enabled),
			compactWhitespace(item.Attributes.URL),
			compactWhitespace(webhookEventTypes(item.Attributes.EventTypes)),
		})
	}
	return headers, rows
}

func printWebhooksTable(resp *WebhooksResponse) error {
	h, r := webhooksRows(resp)
	RenderTable(h, r)
	return nil
}

func printWebhooksMarkdown(resp *WebhooksResponse) error {
	h, r := webhooksRows(resp)
	RenderMarkdown(h, r)
	return nil
}

func webhookDeliveriesRows(resp *WebhookDeliveriesResponse) ([]string, [][]string) {
	headers := []string{"ID", "State", "Created", "Sent", "Error"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{
			item.ID,
			compactWhitespace(item.Attributes.DeliveryState),
			compactWhitespace(item.Attributes.CreatedDate),
			compactWhitespace(item.Attributes.SentDate),
			compactWhitespace(item.Attributes.ErrorMessage),
		})
	}
	return headers, rows
}

func printWebhookDeliveriesTable(resp *WebhookDeliveriesResponse) error {
	h, r := webhookDeliveriesRows(resp)
	RenderTable(h, r)
	return nil
}

func printWebhookDeliveriesMarkdown(resp *WebhookDeliveriesResponse) error {
	h, r := webhookDeliveriesRows(resp)
	RenderMarkdown(h, r)
	return nil
}

func webhookDeleteResultRows(result *WebhookDeleteResult) ([]string, [][]string) {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	return headers, rows
}

func printWebhookDeleteResultTable(result *WebhookDeleteResult) error {
	h, r := webhookDeleteResultRows(result)
	RenderTable(h, r)
	return nil
}

func printWebhookDeleteResultMarkdown(result *WebhookDeleteResult) error {
	h, r := webhookDeleteResultRows(result)
	RenderMarkdown(h, r)
	return nil
}

func webhookPingRows(resp *WebhookPingResponse) ([]string, [][]string) {
	headers := []string{"ID"}
	rows := [][]string{{resp.Data.ID}}
	return headers, rows
}

func printWebhookPingTable(resp *WebhookPingResponse) error {
	h, r := webhookPingRows(resp)
	RenderTable(h, r)
	return nil
}

func printWebhookPingMarkdown(resp *WebhookPingResponse) error {
	h, r := webhookPingRows(resp)
	RenderMarkdown(h, r)
	return nil
}
