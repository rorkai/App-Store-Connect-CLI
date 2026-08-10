package asc

import (
	"io"
	"os"

	"github.com/olekukonko/tablewriter"
	"github.com/olekukonko/tablewriter/renderer"
	"github.com/olekukonko/tablewriter/tw"
)

type renderErrorWriter struct {
	writer io.Writer
	err    error
}

func (w *renderErrorWriter) Write(data []byte) (int, error) {
	if w.err != nil {
		return 0, w.err
	}

	written, err := w.writer.Write(data)
	if err != nil {
		w.err = err
		return written, err
	}
	if written != len(data) {
		w.err = io.ErrShortWrite
		return written, w.err
	}
	return written, nil
}

// RenderTable writes a bordered Unicode table to stdout.
// Headers preserve their original casing and are center-aligned.
// Data rows are left-aligned for readability.
func RenderTable(headers []string, rows [][]string) {
	_ = renderTable(headers, rows)
}

func renderTable(headers []string, rows [][]string) error {
	safeHeaders, safeRows := sanitizeHumanTableData(headers, rows)
	output := &renderErrorWriter{writer: os.Stdout}
	table := tablewriter.NewTable(
		output,
		tablewriter.WithConfig(tablewriter.Config{
			Header: tw.CellConfig{
				Formatting: tw.CellFormatting{
					AutoFormat: tw.Off,
				},
				Alignment: tw.CellAlignment{Global: tw.AlignCenter},
			},
			Row: tw.CellConfig{
				Alignment: tw.CellAlignment{Global: tw.AlignLeft},
			},
		}),
	)
	table.Header(safeHeaders)
	if err := table.Bulk(safeRows); err != nil {
		return err
	}
	if err := table.Render(); err != nil {
		return err
	}
	return output.err
}

// RenderMarkdown writes a Markdown-formatted table to stdout.
// Headers preserve their original casing. Data rows are left-aligned.
// Pipe characters in cell values are escaped automatically by the renderer.
func RenderMarkdown(headers []string, rows [][]string) {
	_ = renderMarkdown(headers, rows)
}

func renderMarkdown(headers []string, rows [][]string) error {
	safeHeaders, safeRows := sanitizeHumanTableData(headers, rows)
	output := &renderErrorWriter{writer: os.Stdout}
	table := tablewriter.NewTable(
		output,
		tablewriter.WithRenderer(renderer.NewMarkdown()),
		tablewriter.WithConfig(tablewriter.Config{
			Header: tw.CellConfig{
				Formatting: tw.CellFormatting{
					AutoFormat: tw.Off,
				},
				Alignment: tw.CellAlignment{Global: tw.AlignLeft},
			},
			Row: tw.CellConfig{
				Alignment: tw.CellAlignment{Global: tw.AlignLeft},
			},
		}),
	)
	table.Header(safeHeaders)
	if err := table.Bulk(safeRows); err != nil {
		return err
	}
	if err := table.Render(); err != nil {
		return err
	}
	return output.err
}

func sanitizeHumanTableData(headers []string, rows [][]string) ([]string, [][]string) {
	safeHeaders := make([]string, len(headers))
	for i, header := range headers {
		safeHeaders[i] = SanitizeTerminalText(header)
	}

	safeRows := make([][]string, len(rows))
	for i, row := range rows {
		safeRows[i] = make([]string, len(row))
		for j, cell := range row {
			safeRows[i][j] = SanitizeTerminalText(cell)
		}
	}
	return safeHeaders, safeRows
}
