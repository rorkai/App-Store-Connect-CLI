package asc

import "fmt"

func nominationsRows(resp *NominationsResponse) ([]string, [][]string) {
	headers := []string{"ID", "Name", "Type", "State", "Publish Start", "Publish End"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		attrs := item.Attributes
		rows = append(rows, []string{
			sanitizeTerminal(item.ID),
			compactWhitespace(fallbackValue(attrs.Name)),
			sanitizeTerminal(fallbackValue(string(attrs.Type))),
			sanitizeTerminal(fallbackValue(string(attrs.State))),
			sanitizeTerminal(fallbackValue(attrs.PublishStartDate)),
			sanitizeTerminal(fallbackValue(attrs.PublishEndDate)),
		})
	}
	return headers, rows
}

func printNominationsTable(resp *NominationsResponse) error {
	h, r := nominationsRows(resp)
	RenderTable(h, r)
	return nil
}

func printNominationsMarkdown(resp *NominationsResponse) error {
	h, r := nominationsRows(resp)
	RenderMarkdown(h, r)
	return nil
}

func nominationDeleteResultRows(result *NominationDeleteResult) ([]string, [][]string) {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	return headers, rows
}

func printNominationDeleteResultTable(result *NominationDeleteResult) error {
	h, r := nominationDeleteResultRows(result)
	RenderTable(h, r)
	return nil
}

func printNominationDeleteResultMarkdown(result *NominationDeleteResult) error {
	h, r := nominationDeleteResultRows(result)
	RenderMarkdown(h, r)
	return nil
}
