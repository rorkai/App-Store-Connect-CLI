package asc

import (
	"strings"
	"testing"
)

func TestPrintTable_PassTypeIDs(t *testing.T) {
	resp := &PassTypeIDsResponse{
		Data: []Resource[PassTypeIDAttributes]{
			{
				ID: "pass-1",
				Attributes: PassTypeIDAttributes{
					Name:       "Wallet",
					Identifier: "pass.com.example",
				},
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintTable(resp)
	})

	if !strings.Contains(output, "ID") || !strings.Contains(output, "Name") || !strings.Contains(output, "Identifier") {
		t.Fatalf("expected pass type IDs headers, got: %s", output)
	}
	if !strings.Contains(output, "pass-1") || !strings.Contains(output, "Wallet") || !strings.Contains(output, "pass.com.example") {
		t.Fatalf("expected pass type ID values, got: %s", output)
	}
}
