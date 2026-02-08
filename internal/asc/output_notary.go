package asc

func notarySubmissionStatusRows(resp *NotarySubmissionStatusResponse) ([]string, [][]string) {
	headers := []string{"ID", "Status", "Name", "Created"}
	rows := [][]string{{
		resp.Data.ID,
		string(resp.Data.Attributes.Status),
		compactWhitespace(resp.Data.Attributes.Name),
		resp.Data.Attributes.CreatedDate,
	}}
	return headers, rows
}

func printNotarySubmissionStatusTable(resp *NotarySubmissionStatusResponse) error {
	h, r := notarySubmissionStatusRows(resp)
	RenderTable(h, r)
	return nil
}

func printNotarySubmissionStatusMarkdown(resp *NotarySubmissionStatusResponse) error {
	h, r := notarySubmissionStatusRows(resp)
	RenderMarkdown(h, r)
	return nil
}

func notarySubmissionsListRows(resp *NotarySubmissionsListResponse) ([]string, [][]string) {
	headers := []string{"ID", "Status", "Name", "Created"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{
			item.ID,
			string(item.Attributes.Status),
			compactWhitespace(item.Attributes.Name),
			item.Attributes.CreatedDate,
		})
	}
	return headers, rows
}

func printNotarySubmissionsListTable(resp *NotarySubmissionsListResponse) error {
	h, r := notarySubmissionsListRows(resp)
	RenderTable(h, r)
	return nil
}

func printNotarySubmissionsListMarkdown(resp *NotarySubmissionsListResponse) error {
	h, r := notarySubmissionsListRows(resp)
	RenderMarkdown(h, r)
	return nil
}

func notarySubmissionLogsRows(resp *NotarySubmissionLogsResponse) ([]string, [][]string) {
	headers := []string{"ID", "Developer Log URL"}
	rows := [][]string{{resp.Data.ID, resp.Data.Attributes.DeveloperLogURL}}
	return headers, rows
}

func printNotarySubmissionLogsTable(resp *NotarySubmissionLogsResponse) error {
	h, r := notarySubmissionLogsRows(resp)
	RenderTable(h, r)
	return nil
}

func printNotarySubmissionLogsMarkdown(resp *NotarySubmissionLogsResponse) error {
	h, r := notarySubmissionLogsRows(resp)
	RenderMarkdown(h, r)
	return nil
}
