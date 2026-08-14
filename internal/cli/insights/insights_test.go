package insights

import (
	"bytes"
	"compress/gzip"
	"strings"
	"testing"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
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

func TestAnalyticsReportRequestIsActiveUsesCurrentAttribute(t *testing.T) {
	active := false
	stopped := true
	if !analyticsReportRequestIsActive(asc.AnalyticsReportRequestAttributes{StoppedDueToInactivity: &active}) {
		t.Fatal("expected stoppedDueToInactivity=false to be active")
	}
	if analyticsReportRequestIsActive(asc.AnalyticsReportRequestAttributes{StoppedDueToInactivity: &stopped}) {
		t.Fatal("expected stoppedDueToInactivity=true to be inactive")
	}
	if !analyticsReportRequestIsActive(asc.AnalyticsReportRequestAttributes{}) {
		t.Fatal("expected absent stoppedDueToInactivity to remain compatible and active")
	}
}

func TestParseSalesReportMetrics(t *testing.T) {
	report := strings.Join([]string{
		"Provider\tSKU\tApple Identifier\tParent Identifier\tProduct Type Identifier\tSubscription\tUnits\tDeveloper Proceeds\tCustomer Price",
		"foo\tChromism12345\t1500196580\t\t1F\t \t10\t0.00\t0.00",
		"foo\tChromism12345\t1500196580\t\t3F\t \t4\t0.00\t0.00",
		"foo\tChromism12345\t1500196580\t\t7F\t \t6\t0.00\t0.00",
		"foo\tChromism12345\t1500196580\t\tF1\t \t2\t0.00\t0.00",
		"foo\tcom.rudrankriyam.chroma_plus\t1619633372\tChromism12345\tIAY\tRenewal\t3\t0.75\t1.00",
		"foo\tcom.rudrankriyam.chroma_plus\t1619633372\tChromism12345\tIAY\tNew\t2\t1.25\t2.00",
		"foo\tother\t999999\tOtherSKU\t1F\tRenewal\t500\t9.99\t9.99",
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

	if metrics.RowCount != 6 {
		t.Fatalf("expected RowCount=6, got %d", metrics.RowCount)
	}
	if !metrics.UnitsColumnPresent || metrics.UnitsTotal != 27 {
		t.Fatalf("unexpected units totals: %+v", metrics)
	}
	if !metrics.DownloadUnitsAvailable {
		t.Fatal("expected download units to be available")
	}
	if metrics.DownloadUnitsTotal != 12 {
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

func TestIsInitialAppDownloadProductType(t *testing.T) {
	tests := []struct {
		name        string
		productType string
		want        bool
	}{
		{name: "app", productType: "1", want: true},
		{name: "app bundle", productType: "1-B", want: true},
		{name: "custom iOS app", productType: "1E", want: true},
		{name: "custom iPadOS app", productType: "1EP", want: true},
		{name: "custom universal app", productType: "1EU", want: true},
		{name: "universal app", productType: "1F", want: true},
		{name: "iPad app", productType: "1T", want: true},
		{name: "Mac app", productType: "F1", want: true},
		{name: "Mac app bundle", productType: "F1-B", want: true},
		{name: "redownload", productType: "3", want: false},
		{name: "universal redownload", productType: "3F", want: false},
		{name: "update", productType: "7", want: false},
		{name: "universal update", productType: "7F", want: false},
		{name: "iPad update", productType: "7T", want: false},
		{name: "Mac update", productType: "F7", want: false},
		{name: "in-app purchase", productType: "IA1", want: false},
		{name: "unknown", productType: "future", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isInitialAppDownloadProductType(tt.productType); got != tt.want {
				t.Fatalf("isInitialAppDownloadProductType(%q) = %v, want %v", tt.productType, got, tt.want)
			}
		})
	}
}

func TestParseSalesReportMetricsMarksDownloadUnitsUnavailableWithoutProductType(t *testing.T) {
	report := strings.Join([]string{
		"Provider\tSKU\tApple Identifier\tParent Identifier\tUnits",
		"foo\tChromism12345\t1500196580\t\t10",
	}, "\n")

	metrics, err := ParseSalesReportMetrics(bytes.NewReader(gzipText(t, report)), salesScope{
		AppID:  "1500196580",
		AppSKU: "Chromism12345",
	})
	if err != nil {
		t.Fatalf("ParseSalesReportMetrics error: %v", err)
	}
	if metrics.DownloadUnitsAvailable {
		t.Fatal("expected download units to be unavailable without Product Type Identifier")
	}
	if metrics.DownloadUnitsTotal != 0 {
		t.Fatalf("expected no inferred download units, got %.2f", metrics.DownloadUnitsTotal)
	}
	if metrics.UnitsTotal != 10 {
		t.Fatalf("expected all units to remain available, got %.2f", metrics.UnitsTotal)
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
