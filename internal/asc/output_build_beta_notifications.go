package asc

func buildBetaNotificationRows(resp *BuildBetaNotificationResponse) ([]string, [][]string) {
	if resp.NotificationAction != BuildBetaGroupsNotificationActionNone {
		return []string{"Notification Action"}, [][]string{{string(resp.NotificationAction)}}
	}
	headers := []string{"ID"}
	rows := [][]string{{resp.Data.ID}}
	return headers, rows
}
