package asc

import "fmt"

func customerReviewResponseRows(resp *CustomerReviewResponseResponse) ([]string, [][]string) {
	headers := []string{"ID", "State", "Last Modified", "Response Body"}
	rows := [][]string{{
		resp.Data.ID,
		sanitizeTerminal(resp.Data.Attributes.State),
		sanitizeTerminal(resp.Data.Attributes.LastModified),
		compactWhitespace(resp.Data.Attributes.ResponseBody),
	}}
	return headers, rows
}

func printCustomerReviewResponseTable(resp *CustomerReviewResponseResponse) error {
	h, r := customerReviewResponseRows(resp)
	RenderTable(h, r)
	return nil
}

func printCustomerReviewResponseMarkdown(resp *CustomerReviewResponseResponse) error {
	h, r := customerReviewResponseRows(resp)
	RenderMarkdown(h, r)
	return nil
}

func customerReviewResponseDeleteResultRows(result *CustomerReviewResponseDeleteResult) ([]string, [][]string) {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	return headers, rows
}

func printCustomerReviewResponseDeleteResultTable(result *CustomerReviewResponseDeleteResult) error {
	h, r := customerReviewResponseDeleteResultRows(result)
	RenderTable(h, r)
	return nil
}

func printCustomerReviewResponseDeleteResultMarkdown(result *CustomerReviewResponseDeleteResult) error {
	h, r := customerReviewResponseDeleteResultRows(result)
	RenderMarkdown(h, r)
	return nil
}
