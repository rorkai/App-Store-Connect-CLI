package asc

import (
	"fmt"
	"os"
)

func printActorsTable(resp *ActorsResponse) error {
	headers := []string{"ID", "Type", "Name", "Email", "API Key ID"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		attr := item.Attributes
		rows = append(rows, []string{
			item.ID,
			compactWhitespace(attr.ActorType),
			compactWhitespace(formatPersonName(attr.UserFirstName, attr.UserLastName)),
			compactWhitespace(attr.UserEmail),
			compactWhitespace(attr.APIKeyID),
		})
	}
	RenderTable(headers, rows)
	return nil
}

func printActorsMarkdown(resp *ActorsResponse) error {
	fmt.Fprintln(os.Stdout, "| ID | Type | Name | Email | API Key ID |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- | --- | --- |")
	for _, item := range resp.Data {
		attr := item.Attributes
		fmt.Fprintf(os.Stdout, "| %s | %s | %s | %s | %s |\n",
			escapeMarkdown(item.ID),
			escapeMarkdown(attr.ActorType),
			escapeMarkdown(formatPersonName(attr.UserFirstName, attr.UserLastName)),
			escapeMarkdown(attr.UserEmail),
			escapeMarkdown(attr.APIKeyID),
		)
	}
	return nil
}
