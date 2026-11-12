package asc

// WebDeveloperTeam is one Developer Portal team on the web session's Apple Account.
type WebDeveloperTeam struct {
	TeamID string `json:"teamId"`
	Name   string `json:"name"`
	Status string `json:"status"`
}

// WebDeveloperTeamsResult lists every Developer Portal team on the web session.
type WebDeveloperTeamsResult struct {
	Teams []WebDeveloperTeam `json:"teams"`
}

func webDeveloperTeamsRows(r *WebDeveloperTeamsResult) ([]string, [][]string) {
	headers := []string{"Team ID", "Name", "Status"}
	if r == nil {
		return headers, nil
	}
	rows := make([][]string, 0, len(r.Teams))
	for _, team := range r.Teams {
		rows = append(rows, []string{team.TeamID, team.Name, team.Status})
	}
	return headers, rows
}
