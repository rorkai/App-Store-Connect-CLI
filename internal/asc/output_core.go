package asc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
)

func printPrettyRawJSON(w io.Writer, data json.RawMessage) error {
	var buf bytes.Buffer
	if err := json.Indent(&buf, data, "", "  "); err != nil {
		return fmt.Errorf("pretty-print json: %w", err)
	}
	buf.WriteByte('\n')
	_, err := w.Write(buf.Bytes())
	return err
}

// PrintMarkdown prints data as Markdown table.
func PrintMarkdown(data any) error {
	return PrintMarkdownTo(os.Stdout, data)
}

// PrintMarkdownTo writes data as a Markdown table to w.
func PrintMarkdownTo(w io.Writer, data any) error {
	return printHumanOutput(w, data, renderMarkdown)
}

// PrintTable prints data as a formatted table.
func PrintTable(data any) error {
	return PrintTableTo(os.Stdout, data)
}

// PrintTableTo writes data as a formatted table to w.
func PrintTableTo(w io.Writer, data any) error {
	return printHumanOutput(w, data, renderTable)
}

func printHumanOutput(w io.Writer, data any, render func(io.Writer, []string, [][]string) error) error {
	var renderErr error
	dispatchErr := renderByRegistryTo(w, data, func(headers []string, rows [][]string) {
		if renderErr != nil {
			return
		}
		renderErr = render(w, headers, rows)
	})
	if dispatchErr != nil {
		return dispatchErr
	}
	return renderErr
}

// PrintJSON prints data as minified JSON (best for AI agents).
func PrintJSON(data any) error {
	return PrintJSONTo(os.Stdout, data)
}

// PrintJSONTo writes data as minified JSON to w.
func PrintJSONTo(w io.Writer, data any) error {
	enc := json.NewEncoder(w)
	return enc.Encode(data)
}

// PrintPrettyJSON prints data as indented JSON (best for debugging).
func PrintPrettyJSON(data any) error {
	return PrintPrettyJSONTo(os.Stdout, data)
}

// PrintPrettyJSONTo writes data as indented JSON to w.
func PrintPrettyJSONTo(w io.Writer, data any) error {
	switch v := data.(type) {
	case json.RawMessage:
		return printPrettyRawJSON(w, v)
	case *PerformanceOverviewResponse:
		return printPrettyRawJSON(w, v.Data)
	case *PerfPowerMetricsResponse:
		return printPrettyRawJSON(w, v.Data)
	case *DiagnosticLogsResponse:
		return printPrettyRawJSON(w, v.Data)
	case *BetaBuildUsagesResponse:
		return printPrettyRawJSON(w, v.Data)
	case *BetaTesterUsagesResponse:
		return printPrettyRawJSON(w, v.Data)
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(data)
}
