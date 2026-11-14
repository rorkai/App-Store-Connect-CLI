package asc

func nominationsRows(resp *NominationsResponse) ([]string, [][]string) {
	headers := []string{"ID", "Name", "Type", "State", "Publish Start", "Publish End"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		attrs := item.Attributes
		rows = append(rows, []string{
			SanitizeTerminalText(item.ID),
			compactWhitespace(fallbackValue(attrs.Name)),
			SanitizeTerminalText(fallbackValue(string(attrs.Type))),
			SanitizeTerminalText(fallbackValue(string(attrs.State))),
			SanitizeTerminalText(fallbackValue(attrs.PublishStartDate)),
			SanitizeTerminalText(fallbackValue(attrs.PublishEndDate)),
		})
	}
	return headers, rows
}
