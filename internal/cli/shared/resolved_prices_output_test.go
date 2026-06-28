package shared

import (
	"strings"
	"testing"
)

func TestPrintResolvedPrices_TableAndMarkdown(t *testing.T) {
	result := &ResolvedPricesResult{
		Prices: []ResolvedPriceRow{
			{
				Territory:     "USA",
				PriceID:       "price-1",
				PricePointID:  "pp-1",
				CustomerPrice: "9.99",
				Currency:      "USD",
				Proceeds:      "8.49",
				ProceedsYear2: "9.10",
				StartDate:     "2025-01-01",
				Manual:        boolPtrResolvedTest(true),
			},
		},
	}

	tableOut, _ := captureOutput(t, func() {
		if err := PrintResolvedPrices(result, "table", false); err != nil {
			t.Fatalf("PrintResolvedPrices(table) error = %v", err)
		}
	})
	if !strings.Contains(tableOut, "Customer Price") || !strings.Contains(tableOut, "Proceeds Y2") {
		t.Fatalf("expected resolved table headers, got %q", tableOut)
	}
	if !strings.Contains(tableOut, "USA") || !strings.Contains(tableOut, "9.99") || !strings.Contains(tableOut, "true") {
		t.Fatalf("expected resolved table row, got %q", tableOut)
	}

	markdownOut, _ := captureOutput(t, func() {
		if err := PrintResolvedPrices(result, "markdown", false); err != nil {
			t.Fatalf("PrintResolvedPrices(markdown) error = %v", err)
		}
	})
	if !strings.Contains(markdownOut, "| Territory | Plan Type | Price ID |") {
		t.Fatalf("expected markdown header row, got %q", markdownOut)
	}
	if !strings.Contains(markdownOut, "| USA       |           | price-1  | pp-1") || !strings.Contains(markdownOut, "true") {
		t.Fatalf("expected markdown value row, got %q", markdownOut)
	}
}

func boolPtrResolvedTest(value bool) *bool {
	v := value
	return &v
}
