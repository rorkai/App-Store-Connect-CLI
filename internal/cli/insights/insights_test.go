package insights

import (
	"bytes"
	"compress/gzip"
	"strings"
	"testing"
	"time"
)

func TestNormalizeWeekStart(t *testing.T) {
	parsed, err := normalizeWeekStart("2026-02-16")
	if err != nil {
		t.Fatalf("normalizeWeekStart error: %v", err)
	}
	if parsed.Format("2006-01-02") != "2026-02-16" {
		t.Fatalf("unexpected week start %q", parsed.Format("2006-01-02"))
	}

	if _, err := normalizeWeekStart("2026-2-16"); err == nil {
		t.Fatal("expected invalid date error")
	}
}

func TestParseSalesReportMetrics(t *testing.T) {
	report := strings.Join([]string{
		"Provider\tSKU\tApple Identifier\tParent Identifier\tSubscription\tUnits\tDeveloper Proceeds\tCustomer Price",
		"foo\tChromism12345\t1500196580\t\t \t10\t0.00\t0.00",
		"foo\tcom.rudrankriyam.chroma_plus\t1619633372\tChromism12345\tRenewal\t3\t0.75\t1.00",
		"foo\tcom.rudrankriyam.chroma_plus\t1619633372\tChromism12345\tNew\t2\t1.25\t2.00",
		"foo\tother\t999999\tOtherSKU\tRenewal\t500\t9.99\t9.99",
		"",
	}, "\n")

	compressed := gzipText(t, report)
	metrics, err := ParseSalesReportMetrics(bytes.NewReader(compressed), salesScope{
		AppID:  "1500196580",
		AppSKU: "Chromism12345",
	})
	if err != nil {
		t.Fatalf("ParseSalesReportMetrics error: %v", err)
	}

	if metrics.RowCount != 3 {
		t.Fatalf("expected RowCount=3, got %d", metrics.RowCount)
	}
	if !metrics.UnitsColumnPresent || metrics.UnitsTotal != 15 {
		t.Fatalf("unexpected units totals: %+v", metrics)
	}
	if metrics.DownloadUnitsTotal != 10 {
		t.Fatalf("unexpected download units totals: %+v", metrics)
	}
	if metrics.MonetizedUnitsTotal != 5 {
		t.Fatalf("unexpected monetized units totals: %+v", metrics)
	}
	if !metrics.DeveloperProceedsColumnPresent || metrics.DeveloperProceedsTotal != 2 {
		t.Fatalf("unexpected developer proceeds totals: %+v", metrics)
	}
	if !metrics.CustomerPriceColumnPresent || metrics.CustomerPriceTotal != 3 {
		t.Fatalf("unexpected customer price totals: %+v", metrics)
	}
	if !metrics.SubscriptionColumnPresent || metrics.SubscriptionRows != 2 || metrics.SubscriptionUnitsTotal != 5 {
		t.Fatalf("unexpected subscription totals: %+v", metrics)
	}
	if metrics.RenewalRows != 1 || metrics.RenewalUnitsTotal != 3 || metrics.RenewalDeveloperProceeds != 0.75 {
		t.Fatalf("unexpected renewal totals: %+v", metrics)
	}
}

func TestContainsDate(t *testing.T) {
	window := weekWindowFromStart(time.Date(2026, 2, 16, 0, 0, 0, 0, time.UTC))

	if !containsDate(window, time.Date(2026, 2, 16, 15, 0, 0, 0, time.UTC)) {
		t.Fatal("expected first day to be in range")
	}
	if !containsDate(window, time.Date(2026, 2, 22, 23, 59, 0, 0, time.UTC)) {
		t.Fatal("expected last day to be in range")
	}
	if containsDate(window, time.Date(2026, 2, 23, 0, 0, 0, 0, time.UTC)) {
		t.Fatal("expected next week date to be out of range")
	}
}

func gzipText(t *testing.T, value string) []byte {
	t.Helper()

	var out bytes.Buffer
	zw := gzip.NewWriter(&out)
	if _, err := zw.Write([]byte(value)); err != nil {
		t.Fatalf("gzip write error: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("gzip close error: %v", err)
	}
	return out.Bytes()
}
