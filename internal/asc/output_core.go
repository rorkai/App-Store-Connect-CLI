package asc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
)

func printPrettyRawJSON(data json.RawMessage) error {
	var buf bytes.Buffer
	if err := json.Indent(&buf, data, "", "  "); err != nil {
		return fmt.Errorf("pretty-print json: %w", err)
	}
	buf.WriteByte('\n')
	_, err := os.Stdout.Write(buf.Bytes())
	return err
}

// PrintMarkdown prints data as Markdown table.
func PrintMarkdown(data any) error {
	return printHumanOutput(data, renderMarkdown)
}

// PrintTable prints data as a formatted table.
func PrintTable(data any) error {
	return printHumanOutput(data, renderTable)
}

func printHumanOutput(data any, render func([]string, [][]string) error) error {
	var renderErr error
	dispatchErr := renderByRegistry(data, func(headers []string, rows [][]string) {
		if renderErr != nil {
			return
		}
		renderErr = render(headers, rows)
	})
	if dispatchErr != nil {
		return dispatchErr
	}
	return renderErr
}

// PrintJSON prints data as minified JSON (best for AI agents).
func PrintJSON(data any) error {
	enc := json.NewEncoder(os.Stdout)
	return enc.Encode(data)
}

// PrintPrettyJSON prints data as indented JSON (best for debugging).
func PrintPrettyJSON(data any) error {
	switch v := data.(type) {
	case *PerfPowerMetricsResponse:
		return printPrettyRawJSON(v.Data)
	case *DiagnosticLogsResponse:
		return printPrettyRawJSON(v.Data)
	case *BetaBuildUsagesResponse:
		return printPrettyRawJSON(v.Data)
	case *BetaTesterUsagesResponse:
		return printPrettyRawJSON(v.Data)
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(data)
}
