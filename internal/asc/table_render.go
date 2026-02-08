package asc

import (
	"os"

	"github.com/olekukonko/tablewriter"
	"github.com/olekukonko/tablewriter/tw"
)

// RenderTable writes a bordered Unicode table to stdout.
// headers defines the column names; rows contains the data.
func RenderTable(headers []string, rows [][]string) {
	table := tablewriter.NewTable(os.Stdout,
		tablewriter.WithConfig(tablewriter.Config{
			Header: tw.CellConfig{
				Formatting: tw.CellFormatting{
					AutoFormat: tw.Off,
				},
			},
		}),
	)
	table.Header(headers)
	_ = table.Bulk(rows)
	_ = table.Render()
}
