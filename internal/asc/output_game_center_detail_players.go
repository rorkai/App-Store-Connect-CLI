package asc

func gameCenterDetailPlayersRows(resp *GameCenterDetailPlayersResponse) ([]string, [][]string) {
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		a := item.Attributes
		rows = append(rows, []string{item.ID, formatOptionalString(a.Nickname), formatOptionalBool(a.Blocked), formatOptionalString(a.BundleID)})
	}
	return []string{"ID", "Nickname", "Blocked", "Bundle ID"}, rows
}
