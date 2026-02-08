package asc

import (
	"fmt"
	"os"
)

func printLinkagesTable(resp *LinkagesResponse) error {
	headers := []string{"Type", "ID"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{string(item.Type), item.ID})
	}
	RenderTable(headers, rows)
	return nil
}

func printLinkagesMarkdown(resp *LinkagesResponse) error {
	fmt.Fprintln(os.Stdout, "| Type | ID |")
	fmt.Fprintln(os.Stdout, "| --- | --- |")
	for _, item := range resp.Data {
		fmt.Fprintf(os.Stdout, "| %s | %s |\n",
			escapeMarkdown(string(item.Type)),
			escapeMarkdown(item.ID),
		)
	}
	return nil
}
