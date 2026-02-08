package asc

import (
	"os"

	"github.com/olekukonko/tablewriter"
	"github.com/olekukonko/tablewriter/renderer"
	"github.com/olekukonko/tablewriter/tw"
)

// RenderTable writes a bordered Unicode table to stdout.
// Headers preserve their original casing and are center-aligned.
// Data rows are left-aligned for readability.
func RenderTable(headers []string, rows [][]string) {
	table := tablewriter.NewTable(os.Stdout,
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
	table.Header(headers)
	_ = table.Bulk(rows)
	_ = table.Render()
}

// RenderMarkdown writes a Markdown-formatted table to stdout.
// Headers preserve their original casing. Data rows are left-aligned.
// Pipe characters in cell values are escaped automatically by the renderer.
func RenderMarkdown(headers []string, rows [][]string) {
	table := tablewriter.NewTable(os.Stdout,
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
	table.Header(headers)
	_ = table.Bulk(rows)
	_ = table.Render()
}
