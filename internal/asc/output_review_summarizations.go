package asc

import (
	"fmt"
	"os"
)

func customerReviewSummarizationTerritoryID(resource CustomerReviewSummarizationResource) string {
	if resource.Relationships == nil || resource.Relationships.Territory == nil {
		return ""
	}
	return resource.Relationships.Territory.Data.ID
}

func printCustomerReviewSummarizationsTable(resp *CustomerReviewSummarizationsResponse) error {
	headers := []string{"ID", "Locale", "Platform", "Territory", "Created", "Text"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{
			item.ID,
			compactWhitespace(item.Attributes.Locale),
			compactWhitespace(string(item.Attributes.Platform)),
			compactWhitespace(customerReviewSummarizationTerritoryID(item)),
			compactWhitespace(item.Attributes.CreatedDate),
			compactWhitespace(item.Attributes.Text),
		})
	}
	RenderTable(headers, rows)
	return nil
}

func printCustomerReviewSummarizationsMarkdown(resp *CustomerReviewSummarizationsResponse) error {
	fmt.Fprintln(os.Stdout, "| ID | Locale | Platform | Territory | Created | Text |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- | --- | --- | --- |")
	for _, item := range resp.Data {
		fmt.Fprintf(os.Stdout, "| %s | %s | %s | %s | %s | %s |\n",
			escapeMarkdown(item.ID),
			escapeMarkdown(item.Attributes.Locale),
			escapeMarkdown(string(item.Attributes.Platform)),
			escapeMarkdown(customerReviewSummarizationTerritoryID(item)),
			escapeMarkdown(item.Attributes.CreatedDate),
			escapeMarkdown(item.Attributes.Text),
		)
	}
	return nil
}
