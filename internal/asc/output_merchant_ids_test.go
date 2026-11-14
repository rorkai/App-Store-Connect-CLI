package asc

import (
	"strings"
	"testing"
)

func TestPrintTable_MerchantIDs(t *testing.T) {
	resp := &MerchantIDsResponse{
		Data: []Resource[MerchantIDAttributes]{
			{
				ID: "m1",
				Attributes: MerchantIDAttributes{
					Name:       "Example",
					Identifier: "merchant.com.example",
				},
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintTable(resp)
	})

	if !strings.Contains(output, "Identifier") || !strings.Contains(output, "Name") {
		t.Fatalf("expected header in output, got: %s", output)
	}
	if !strings.Contains(output, "merchant.com.example") {
		t.Fatalf("expected identifier in output, got: %s", output)
	}
}

func TestPrintMarkdown_MerchantIDs(t *testing.T) {
	resp := &MerchantIDsResponse{
		Data: []Resource[MerchantIDAttributes]{
			{
				ID: "m1",
				Attributes: MerchantIDAttributes{
					Name:       "Example",
					Identifier: "merchant.com.example",
				},
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintMarkdown(resp)
	})

	if !strings.Contains(output, "ID") || !strings.Contains(output, "Identifier") {
		t.Fatalf("expected markdown header, got: %s", output)
	}
	if !strings.Contains(output, "merchant.com.example") {
		t.Fatalf("expected identifier in output, got: %s", output)
	}
}
