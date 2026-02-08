package asc

import (
	"fmt"
	"os"
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

func printWebhooksTable(resp *WebhooksResponse) error {
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
	RenderTable(headers, rows)
	return nil
}

func printWebhooksMarkdown(resp *WebhooksResponse) error {
	fmt.Fprintln(os.Stdout, "| ID | Name | Enabled | URL | Events |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- | --- | --- |")
	for _, item := range resp.Data {
		fmt.Fprintf(os.Stdout, "| %s | %s | %s | %s | %s |\n",
			escapeMarkdown(item.ID),
			escapeMarkdown(item.Attributes.Name),
			escapeMarkdown(strconv.FormatBool(item.Attributes.Enabled)),
			escapeMarkdown(item.Attributes.URL),
			escapeMarkdown(webhookEventTypes(item.Attributes.EventTypes)),
		)
	}
	return nil
}

func printWebhookDeliveriesTable(resp *WebhookDeliveriesResponse) error {
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
	RenderTable(headers, rows)
	return nil
}

func printWebhookDeliveriesMarkdown(resp *WebhookDeliveriesResponse) error {
	fmt.Fprintln(os.Stdout, "| ID | State | Created | Sent | Error |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- | --- | --- |")
	for _, item := range resp.Data {
		fmt.Fprintf(os.Stdout, "| %s | %s | %s | %s | %s |\n",
			escapeMarkdown(item.ID),
			escapeMarkdown(item.Attributes.DeliveryState),
			escapeMarkdown(item.Attributes.CreatedDate),
			escapeMarkdown(item.Attributes.SentDate),
			escapeMarkdown(item.Attributes.ErrorMessage),
		)
	}
	return nil
}

func printWebhookDeleteResultTable(result *WebhookDeleteResult) error {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	RenderTable(headers, rows)
	return nil
}

func printWebhookDeleteResultMarkdown(result *WebhookDeleteResult) error {
	fmt.Fprintln(os.Stdout, "| ID | Deleted |")
	fmt.Fprintln(os.Stdout, "| --- | --- |")
	fmt.Fprintf(os.Stdout, "| %s | %t |\n", escapeMarkdown(result.ID), result.Deleted)
	return nil
}

func printWebhookPingTable(resp *WebhookPingResponse) error {
	headers := []string{"ID"}
	rows := [][]string{{resp.Data.ID}}
	RenderTable(headers, rows)
	return nil
}

func printWebhookPingMarkdown(resp *WebhookPingResponse) error {
	fmt.Fprintln(os.Stdout, "| ID |")
	fmt.Fprintln(os.Stdout, "| --- |")
	fmt.Fprintf(os.Stdout, "| %s |\n", escapeMarkdown(resp.Data.ID))
	return nil
}
