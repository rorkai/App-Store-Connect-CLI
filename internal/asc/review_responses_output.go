package asc

func customerReviewResponseRows(resp *CustomerReviewResponseResponse) ([]string, [][]string) {
	headers := []string{"ID", "State", "Last Modified", "Response Body"}
	rows := [][]string{{
		SanitizeTerminalText(resp.Data.ID),
		SanitizeTerminalText(resp.Data.Attributes.State),
		SanitizeTerminalText(resp.Data.Attributes.LastModified),
		compactWhitespace(resp.Data.Attributes.ResponseBody),
	}}
	return headers, rows
}
