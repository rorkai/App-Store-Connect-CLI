package asc

func buildBetaNotificationRows(resp *BuildBetaNotificationResponse) ([]string, [][]string) {
	headers := []string{"ID"}
	rows := [][]string{{resp.Data.ID}}
	return headers, rows
}

func printBuildBetaNotificationTable(resp *BuildBetaNotificationResponse) error {
	h, r := buildBetaNotificationRows(resp)
	RenderTable(h, r)
	return nil
}

func printBuildBetaNotificationMarkdown(resp *BuildBetaNotificationResponse) error {
	h, r := buildBetaNotificationRows(resp)
	RenderMarkdown(h, r)
	return nil
}
