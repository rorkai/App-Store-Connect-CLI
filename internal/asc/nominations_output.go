package asc

import (
	"fmt"
	"os"
)

func printNominationsTable(resp *NominationsResponse) error {
	headers := []string{"ID", "Name", "Type", "State", "Publish Start", "Publish End"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		attrs := item.Attributes
		rows = append(rows, []string{
			sanitizeTerminal(item.ID),
			compactWhitespace(fallbackValue(attrs.Name)),
			sanitizeTerminal(fallbackValue(string(attrs.Type))),
			sanitizeTerminal(fallbackValue(string(attrs.State))),
			sanitizeTerminal(fallbackValue(attrs.PublishStartDate)),
			sanitizeTerminal(fallbackValue(attrs.PublishEndDate)),
		})
	}
	RenderTable(headers, rows)
	return nil
}

func printNominationsMarkdown(resp *NominationsResponse) error {
	fmt.Fprintln(os.Stdout, "| ID | Name | Type | State | Publish Start | Publish End |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- | --- | --- | --- |")
	for _, item := range resp.Data {
		attrs := item.Attributes
		fmt.Fprintf(os.Stdout, "| %s | %s | %s | %s | %s | %s |\n",
			escapeMarkdown(item.ID),
			escapeMarkdown(fallbackValue(attrs.Name)),
			escapeMarkdown(fallbackValue(string(attrs.Type))),
			escapeMarkdown(fallbackValue(string(attrs.State))),
			escapeMarkdown(fallbackValue(attrs.PublishStartDate)),
			escapeMarkdown(fallbackValue(attrs.PublishEndDate)),
		)
	}
	return nil
}

func printNominationDeleteResultTable(result *NominationDeleteResult) error {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	RenderTable(headers, rows)
	return nil
}

func printNominationDeleteResultMarkdown(result *NominationDeleteResult) error {
	fmt.Fprintln(os.Stdout, "| ID | Deleted |")
	fmt.Fprintln(os.Stdout, "| --- | --- |")
	fmt.Fprintf(os.Stdout, "| %s | %t |\n", escapeMarkdown(result.ID), result.Deleted)
	return nil
}
