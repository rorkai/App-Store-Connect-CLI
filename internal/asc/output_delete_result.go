package asc

import "fmt"

// DeleteResult is the output of a delete command that reports the deleted
// resource's ID.
type DeleteResult struct {
	ID      string `json:"id"`
	Deleted bool   `json:"deleted"`
}

func deleteResultRows(result *DeleteResult) ([]string, [][]string) {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	return headers, rows
}
