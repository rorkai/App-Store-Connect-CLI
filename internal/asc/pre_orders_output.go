package asc

import (
	"fmt"
	"os"
)

func printEndAppAvailabilityPreOrderTable(resp *EndAppAvailabilityPreOrderResponse) error {
	headers := []string{"ID"}
	rows := [][]string{{resp.Data.ID}}
	RenderTable(headers, rows)
	return nil
}

func printEndAppAvailabilityPreOrderMarkdown(resp *EndAppAvailabilityPreOrderResponse) error {
	fmt.Fprintln(os.Stdout, "| ID |")
	fmt.Fprintln(os.Stdout, "| --- |")
	fmt.Fprintf(os.Stdout, "| %s |\n", escapeMarkdown(resp.Data.ID))
	return nil
}
