package asc

import (
	"fmt"
	"os"
)

func printBuildBetaNotificationTable(resp *BuildBetaNotificationResponse) error {
	headers := []string{"ID"}
	rows := [][]string{{resp.Data.ID}}
	RenderTable(headers, rows)
	return nil
}

func printBuildBetaNotificationMarkdown(resp *BuildBetaNotificationResponse) error {
	fmt.Fprintln(os.Stdout, "| ID |")
	fmt.Fprintln(os.Stdout, "| --- |")
	fmt.Fprintf(os.Stdout, "| %s |\n", escapeMarkdown(resp.Data.ID))
	return nil
}
