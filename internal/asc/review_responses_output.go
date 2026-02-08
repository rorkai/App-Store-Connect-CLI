package asc

import (
	"fmt"
	"os"
)

func printCustomerReviewResponseTable(resp *CustomerReviewResponseResponse) error {
	headers := []string{"ID", "State", "Last Modified", "Response Body"}
	rows := [][]string{{
		resp.Data.ID,
		sanitizeTerminal(resp.Data.Attributes.State),
		sanitizeTerminal(resp.Data.Attributes.LastModified),
		compactWhitespace(resp.Data.Attributes.ResponseBody),
	}}
	RenderTable(headers, rows)
	return nil
}

func printCustomerReviewResponseMarkdown(resp *CustomerReviewResponseResponse) error {
	fmt.Fprintln(os.Stdout, "| ID | State | Last Modified | Response Body |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- | --- |")
	fmt.Fprintf(os.Stdout, "| %s | %s | %s | %s |\n",
		escapeMarkdown(resp.Data.ID),
		escapeMarkdown(resp.Data.Attributes.State),
		escapeMarkdown(resp.Data.Attributes.LastModified),
		escapeMarkdown(resp.Data.Attributes.ResponseBody),
	)
	return nil
}

func printCustomerReviewResponseDeleteResultTable(result *CustomerReviewResponseDeleteResult) error {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	RenderTable(headers, rows)
	return nil
}

func printCustomerReviewResponseDeleteResultMarkdown(result *CustomerReviewResponseDeleteResult) error {
	fmt.Fprintln(os.Stdout, "| ID | Deleted |")
	fmt.Fprintln(os.Stdout, "| --- | --- |")
	fmt.Fprintf(os.Stdout, "| %s | %t |\n",
		escapeMarkdown(result.ID),
		result.Deleted,
	)
	return nil
}
