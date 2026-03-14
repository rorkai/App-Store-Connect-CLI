package asc

// AppWallEntry represents one row in apps wall output.
type AppWallEntry struct {
	ID          string `json:"id,omitempty"`
	Name        string `json:"name"`
	AppStoreURL string `json:"appStoreUrl"`
	ReleaseDate string `json:"releaseDate,omitempty"`
}

// AppsWallResult is the response payload for apps wall output.
type AppsWallResult struct {
	Data []AppWallEntry `json:"data"`
}

func appsWallRows(resp *AppsWallResult) ([]string, [][]string) {
	headers := []string{"App", "Link"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{
			compactWhitespace(item.Name),
			item.AppStoreURL,
		})
	}
	return headers, rows
}
