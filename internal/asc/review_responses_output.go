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

func customerReviewResponseDeleteResultRows(result *CustomerReviewResponseDeleteResult) ([]string, [][]string) {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	return headers, rows
}
