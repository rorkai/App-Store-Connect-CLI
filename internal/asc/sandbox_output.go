package asc

import (
	"fmt"
	"strings"
)

// SandboxTesterClearHistoryResult represents CLI output for clear history requests.
type SandboxTesterClearHistoryResult struct {
	RequestID string `json:"requestId"`
	TesterID  string `json:"testerId"`
	Cleared   bool   `json:"cleared"`
}

func formatSandboxTesterName(attr SandboxTesterAttributes) string {
	return compactWhitespace(strings.TrimSpace(attr.FirstName + " " + attr.LastName))
}

func sandboxTestersRows(resp *SandboxTestersResponse) ([]string, [][]string) {
	headers := []string{"ID", "Email", "Name", "Territory"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{
			item.ID,
			sandboxTesterEmail(item.Attributes),
			formatSandboxTesterName(item.Attributes),
			sandboxTesterTerritory(item.Attributes),
		})
	}
	return headers, rows
}

func printSandboxTestersTable(resp *SandboxTestersResponse) error {
	h, r := sandboxTestersRows(resp)
	RenderTable(h, r)
	return nil
}

func printSandboxTestersMarkdown(resp *SandboxTestersResponse) error {
	h, r := sandboxTestersRows(resp)
	RenderMarkdown(h, r)
	return nil
}

func sandboxTesterClearHistoryResultRows(result *SandboxTesterClearHistoryResult) ([]string, [][]string) {
	headers := []string{"Request ID", "Tester ID", "Cleared"}
	rows := [][]string{{
		result.RequestID,
		result.TesterID,
		fmt.Sprintf("%t", result.Cleared),
	}}
	return headers, rows
}

func printSandboxTesterClearHistoryResultTable(result *SandboxTesterClearHistoryResult) error {
	h, r := sandboxTesterClearHistoryResultRows(result)
	RenderTable(h, r)
	return nil
}

func printSandboxTesterClearHistoryResultMarkdown(result *SandboxTesterClearHistoryResult) error {
	h, r := sandboxTesterClearHistoryResultRows(result)
	RenderMarkdown(h, r)
	return nil
}
