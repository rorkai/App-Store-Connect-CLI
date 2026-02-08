package asc

func appsRows(resp *AppsResponse) ([]string, [][]string) {
	headers := []string{"ID", "Name", "Bundle ID", "SKU"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{
			item.ID,
			compactWhitespace(item.Attributes.Name),
			item.Attributes.BundleID,
			item.Attributes.SKU,
		})
	}
	return headers, rows
}
