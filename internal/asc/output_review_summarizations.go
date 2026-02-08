package asc

func customerReviewSummarizationTerritoryID(resource CustomerReviewSummarizationResource) string {
	if resource.Relationships == nil || resource.Relationships.Territory == nil {
		return ""
	}
	return resource.Relationships.Territory.Data.ID
}

func customerReviewSummarizationsRows(resp *CustomerReviewSummarizationsResponse) ([]string, [][]string) {
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
	return headers, rows
}

func printCustomerReviewSummarizationsTable(resp *CustomerReviewSummarizationsResponse) error {
	h, r := customerReviewSummarizationsRows(resp)
	RenderTable(h, r)
	return nil
}

func printCustomerReviewSummarizationsMarkdown(resp *CustomerReviewSummarizationsResponse) error {
	h, r := customerReviewSummarizationsRows(resp)
	RenderMarkdown(h, r)
	return nil
}
