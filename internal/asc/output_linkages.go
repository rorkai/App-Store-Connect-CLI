package asc

func linkagesRows(resp *LinkagesResponse) ([]string, [][]string) {
	headers := []string{"Type", "ID"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{string(item.Type), item.ID})
	}
	return headers, rows
}

func printLinkagesTable(resp *LinkagesResponse) error {
	h, r := linkagesRows(resp)
	RenderTable(h, r)
	return nil
}

func printLinkagesMarkdown(resp *LinkagesResponse) error {
	h, r := linkagesRows(resp)
	RenderMarkdown(h, r)
	return nil
}
