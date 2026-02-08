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

func TestPrintMarkdown_PassTypeIDs(t *testing.T) {
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
		return PrintMarkdown(resp)
	})

	if !strings.Contains(output, "ID") || !strings.Contains(output, "Identifier") {
		t.Fatalf("expected pass type IDs header, got: %s", output)
	}
	if !strings.Contains(output, "pass-1") || !strings.Contains(output, "Wallet") || !strings.Contains(output, "pass.com.example") {
		t.Fatalf("expected pass type ID values, got: %s", output)
	}
}

func TestPrintTable_PassTypeIDDeleteResult(t *testing.T) {
	result := &PassTypeIDDeleteResult{ID: "pass-1", Deleted: true}

	output := captureStdout(t, func() error {
		return PrintTable(result)
	})

	if !strings.Contains(output, "ID") || !strings.Contains(output, "Deleted") {
		t.Fatalf("expected delete headers, got: %s", output)
	}
	if !strings.Contains(output, "pass-1") || !strings.Contains(output, "true") {
		t.Fatalf("expected delete values, got: %s", output)
	}
}

func TestPrintMarkdown_PassTypeIDDeleteResult(t *testing.T) {
	result := &PassTypeIDDeleteResult{ID: "pass-1", Deleted: true}

	output := captureStdout(t, func() error {
		return PrintMarkdown(result)
	})

	if !strings.Contains(output, "ID") || !strings.Contains(output, "Deleted") {
		t.Fatalf("expected delete header, got: %s", output)
	}
	if !strings.Contains(output, "pass-1") || !strings.Contains(output, "true") {
		t.Fatalf("expected delete values, got: %s", output)
	}
}
