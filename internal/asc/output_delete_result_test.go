package asc

import (
	"strings"
	"testing"
)

func TestPrintTableAndMarkdown_DeleteResult(t *testing.T) {
	result := &DeleteResult{ID: "resource-1", Deleted: true}

	for name, print := range map[string]func(any) error{"table": PrintTable, "markdown": PrintMarkdown} {
		t.Run(name, func(t *testing.T) {
			output := captureStdout(t, func() error { return print(result) })

			for _, want := range []string{"ID", "Deleted", "resource-1", "true"} {
				if !strings.Contains(output, want) {
					t.Fatalf("expected %q in output, got: %s", want, output)
				}
			}
		})
	}
}
