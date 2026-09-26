package asc

func gameCenterScoreModerationsRows(resp *GameCenterScoreModerationsResponse) ([]string, [][]string) {
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		a := item.Attributes
		rows = append(rows, []string{item.ID, formatOptionalString(a.Rank), formatOptionalString(a.Score), formatOptionalBool(a.Blocked), formatOptionalString(a.SubmittedDate)})
	}
	return []string{"ID", "Rank", "Score", "Blocked", "Submitted Date"}, rows
}
