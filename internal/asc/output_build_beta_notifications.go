package asc

func buildBetaNotificationRows(resp *BuildBetaNotificationResponse) ([]string, [][]string) {
	headers := []string{"ID"}
	rows := [][]string{{resp.Data.ID}}
	return headers, rows
}
