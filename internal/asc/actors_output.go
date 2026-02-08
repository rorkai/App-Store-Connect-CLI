package asc

func actorsRows(resp *ActorsResponse) ([]string, [][]string) {
	headers := []string{"ID", "Type", "Name", "Email", "API Key ID"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		attr := item.Attributes
		rows = append(rows, []string{
			item.ID,
			compactWhitespace(attr.ActorType),
			compactWhitespace(formatPersonName(attr.UserFirstName, attr.UserLastName)),
			compactWhitespace(attr.UserEmail),
			compactWhitespace(attr.APIKeyID),
		})
	}
	return headers, rows
}

func printActorsTable(resp *ActorsResponse) error {
	h, r := actorsRows(resp)
	RenderTable(h, r)
	return nil
}

func printActorsMarkdown(resp *ActorsResponse) error {
	h, r := actorsRows(resp)
	RenderMarkdown(h, r)
	return nil
}
