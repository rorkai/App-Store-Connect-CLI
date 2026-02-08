package asc

import (
	"fmt"
	"os"
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

func printSandboxTestersTable(resp *SandboxTestersResponse) error {
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
	RenderTable(headers, rows)
	return nil
}

func printSandboxTestersMarkdown(resp *SandboxTestersResponse) error {
	fmt.Fprintln(os.Stdout, "| ID | Email | Name | Territory |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- | --- |")
	for _, item := range resp.Data {
		fmt.Fprintf(os.Stdout, "| %s | %s | %s | %s |\n",
			escapeMarkdown(item.ID),
			escapeMarkdown(sandboxTesterEmail(item.Attributes)),
			escapeMarkdown(formatSandboxTesterName(item.Attributes)),
			escapeMarkdown(sandboxTesterTerritory(item.Attributes)),
		)
	}
	return nil
}

func printSandboxTesterClearHistoryResultTable(result *SandboxTesterClearHistoryResult) error {
	headers := []string{"Request ID", "Tester ID", "Cleared"}
	rows := [][]string{{
		result.RequestID,
		result.TesterID,
		fmt.Sprintf("%t", result.Cleared),
	}}
	RenderTable(headers, rows)
	return nil
}

func printSandboxTesterClearHistoryResultMarkdown(result *SandboxTesterClearHistoryResult) error {
	fmt.Fprintln(os.Stdout, "| Request ID | Tester ID | Cleared |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- |")
	fmt.Fprintf(os.Stdout, "| %s | %s | %t |\n",
		escapeMarkdown(result.RequestID),
		escapeMarkdown(result.TesterID),
		result.Cleared,
	)
	return nil
}
