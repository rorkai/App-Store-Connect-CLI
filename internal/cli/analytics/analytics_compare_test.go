package analytics

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"flag"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/insights"
)

func TestAnalyticsCompareValidationErrors(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_VENDOR_NUMBER", "")
	t.Setenv("ASC_ANALYTICS_VENDOR_NUMBER", "")

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "missing source",
			args:    []string{"analytics", "compare", "--app", "123", "--from", "2026-01-01", "--to", "2026-02-01", "--frequency", "DAILY"},
			wantErr: "--source is required",
		},
		{
			name:    "invalid source",
			args:    []string{"analytics", "compare", "--source", "invalid", "--app", "123", "--from", "2026-01-01", "--to", "2026-02-01", "--frequency", "DAILY"},
			wantErr: "--source must be sales",
		},
		{
			name:    "missing app",
			args:    []string{"analytics", "compare", "--source", "sales", "--vendor", "V", "--from", "2026-01-01", "--to", "2026-02-01", "--frequency", "DAILY"},
			wantErr: "--app is required",
		},
		{
			name:    "missing vendor",
			args:    []string{"analytics", "compare", "--source", "sales", "--app", "123", "--from", "2026-01-01", "--to", "2026-02-01", "--frequency", "DAILY"},
			wantErr: "--vendor is required",
		},
		{
			name:    "missing from",
			args:    []string{"analytics", "compare", "--source", "sales", "--app", "123", "--vendor", "V", "--to", "2026-02-01", "--frequency", "DAILY"},
			wantErr: "--from is required",
		},
		{
			name:    "missing to",
			args:    []string{"analytics", "compare", "--source", "sales", "--app", "123", "--vendor", "V", "--from", "2026-01-01", "--frequency", "DAILY"},
			wantErr: "--to is required",
		},
		{
			name:    "missing frequency",
			args:    []string{"analytics", "compare", "--source", "sales", "--app", "123", "--vendor", "V", "--from", "2026-01-01", "--to", "2026-02-01"},
			wantErr: "--frequency is required",
		},
		{
			name:    "invalid frequency",
			args:    []string{"analytics", "compare", "--source", "sales", "--app", "123", "--vendor", "V", "--from", "2026-01-01", "--to", "2026-02-01", "--frequency", "BIWEEKLY"},
			wantErr: "--frequency must be",
		},
		{
			name:    "from-end before from",
			args:    []string{"analytics", "compare", "--source", "sales", "--app", "123", "--vendor", "V", "--from", "2026-02-01", "--from-end", "2026-01-01", "--to", "2026-03-01", "--frequency", "DAILY"},
			wantErr: "--from-end must not be before --from",
		},
		{
			name:    "to-end before to",
			args:    []string{"analytics", "compare", "--source", "sales", "--app", "123", "--vendor", "V", "--from", "2026-01-01", "--to", "2026-03-01", "--to-end", "2026-02-01", "--frequency", "DAILY"},
			wantErr: "--to-end must not be before --to",
		},
		{
			name:    "unexpected args",
			args:    []string{"analytics", "compare", "--source", "sales", "--app", "123", "--vendor", "V", "--from", "2026-01-01", "--to", "2026-02-01", "--frequency", "DAILY", "extra"},
			wantErr: "unexpected argument(s)",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stdout, stderr, err := runAnalyticsCommand(t, test.args)
			if !errors.Is(err, flag.ErrHelp) {
				t.Fatalf("expected ErrHelp, got %v", err)
			}

			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, test.wantErr) {
				t.Fatalf("expected error %q, got %q", test.wantErr, stderr)
			}
		})
	}
}

func TestGenerateReportDates_Daily(t *testing.T) {
	dates, err := generateReportDates("2026-01-01", "2026-01-03", asc.SalesReportFrequencyDaily)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"2026-01-01", "2026-01-02", "2026-01-03"}
	if len(dates) != len(want) {
		t.Fatalf("expected %d dates, got %d: %v", len(want), len(dates), dates)
	}
	for i, d := range dates {
		if d != want[i] {
			t.Fatalf("date[%d] = %q, want %q", i, d, want[i])
		}
	}
}

func TestGenerateReportDates_Monthly(t *testing.T) {
	dates, err := generateReportDates("2026-01", "2026-03", asc.SalesReportFrequencyMonthly)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"2026-01", "2026-02", "2026-03"}
	if len(dates) != len(want) {
		t.Fatalf("expected %d dates, got %d: %v", len(want), len(dates), dates)
	}
	for i, d := range dates {
		if d != want[i] {
			t.Fatalf("date[%d] = %q, want %q", i, d, want[i])
		}
	}
}

func TestGenerateReportDates_Yearly(t *testing.T) {
	dates, err := generateReportDates("2024", "2026", asc.SalesReportFrequencyYearly)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"2024", "2025", "2026"}
	if len(dates) != len(want) {
		t.Fatalf("expected %d dates, got %d: %v", len(want), len(dates), dates)
	}
	for i, d := range dates {
		if d != want[i] {
			t.Fatalf("date[%d] = %q, want %q", i, d, want[i])
		}
	}
}

func TestGenerateReportDates_SingleDate(t *testing.T) {
	dates, err := generateReportDates("2026-03-01", "2026-03-01", asc.SalesReportFrequencyDaily)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(dates) != 1 || dates[0] != "2026-03-01" {
		t.Fatalf("expected single date [2026-03-01], got %v", dates)
	}
}

func TestNormalizeCompareDateRange_EndBeforeStart(t *testing.T) {
	_, _, err := normalizeCompareDateRange("2026-03-01", "2026-01-01", asc.SalesReportFrequencyDaily)
	if err == nil {
		t.Fatal("expected error when end is before start")
	}
	if !strings.Contains(err.Error(), "--from-end must not be before --from") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNormalizeCompareDateRange_MonthlyEndBeforeStart(t *testing.T) {
	_, _, err := normalizeCompareDateRange("2026-03", "2026-01", asc.SalesReportFrequencyMonthly)
	if err == nil {
		t.Fatal("expected error when end is before start")
	}
}

func TestNormalizeCompareDateRange_DefaultsEndToStart(t *testing.T) {
	start, end, err := normalizeCompareDateRange("2026-05-01", "", asc.SalesReportFrequencyDaily)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if start != "2026-05-01" || end != "2026-05-01" {
		t.Fatalf("expected start=end=2026-05-01, got start=%q end=%q", start, end)
	}
}

func TestAggregateSalesMetrics(t *testing.T) {
	a := insights.SalesMetrics{
		RowCount:                       2,
		UnitsColumnPresent:             true,
		DeveloperProceedsColumnPresent: true,
		CustomerPriceColumnPresent:     true,
		SubscriptionColumnPresent:      true,
		UnitsTotal:                     10,
		DownloadUnitsTotal:             5,
		MonetizedUnitsTotal:            3,
		DeveloperProceedsTotal:         8.50,
		CustomerPriceTotal:             12.00,
		SubscriptionRows:               1,
		SubscriptionUnitsTotal:         2,
		SubscriptionDeveloperProceeds:  1.50,
		SubscriptionCustomerPrice:      2.00,
		RenewalRows:                    1,
		RenewalUnitsTotal:              1,
		RenewalDeveloperProceeds:       0.75,
		RenewalCustomerPrice:           1.00,
	}
	b := insights.SalesMetrics{
		RowCount:                       3,
		UnitsColumnPresent:             true,
		DeveloperProceedsColumnPresent: true,
		CustomerPriceColumnPresent:     true,
		SubscriptionColumnPresent:      true,
		UnitsTotal:                     20,
		DownloadUnitsTotal:             10,
		MonetizedUnitsTotal:            7,
		DeveloperProceedsTotal:         15.00,
		CustomerPriceTotal:             25.00,
		SubscriptionRows:               2,
		SubscriptionUnitsTotal:         4,
		SubscriptionDeveloperProceeds:  3.00,
		SubscriptionCustomerPrice:      5.00,
		RenewalRows:                    1,
		RenewalUnitsTotal:              2,
		RenewalDeveloperProceeds:       1.50,
		RenewalCustomerPrice:           2.00,
	}

	result := aggregateSalesMetrics(a, b)
	if result.RowCount != 5 {
		t.Fatalf("expected RowCount=5, got %d", result.RowCount)
	}
	if result.UnitsTotal != 30 {
		t.Fatalf("expected UnitsTotal=30, got %.2f", result.UnitsTotal)
	}
	if result.DownloadUnitsTotal != 15 {
		t.Fatalf("expected DownloadUnitsTotal=15, got %.2f", result.DownloadUnitsTotal)
	}
	if result.DeveloperProceedsTotal != 23.50 {
		t.Fatalf("expected DeveloperProceedsTotal=23.50, got %.2f", result.DeveloperProceedsTotal)
	}
	if result.RenewalRows != 2 {
		t.Fatalf("expected RenewalRows=2, got %d", result.RenewalRows)
	}
	if result.RenewalDeveloperProceeds != 2.25 {
		t.Fatalf("expected RenewalDeveloperProceeds=2.25, got %.2f", result.RenewalDeveloperProceeds)
	}
}

func TestBuildCompareMetrics(t *testing.T) {
	baseline := insights.SalesMetrics{
		RowCount:                       5,
		UnitsColumnPresent:             true,
		DeveloperProceedsColumnPresent: true,
		CustomerPriceColumnPresent:     true,
		SubscriptionColumnPresent:      true,
		UnitsTotal:                     100,
		DownloadUnitsTotal:             50,
		MonetizedUnitsTotal:            30,
		DeveloperProceedsTotal:         200.00,
		CustomerPriceTotal:             300.00,
	}
	comparison := insights.SalesMetrics{
		RowCount:                       8,
		UnitsColumnPresent:             true,
		DeveloperProceedsColumnPresent: true,
		CustomerPriceColumnPresent:     true,
		SubscriptionColumnPresent:      true,
		UnitsTotal:                     150,
		DownloadUnitsTotal:             80,
		MonetizedUnitsTotal:            50,
		DeveloperProceedsTotal:         300.00,
		CustomerPriceTotal:             400.00,
	}

	metrics := buildCompareMetrics(baseline, comparison, "", "")
	found := false
	for _, m := range metrics {
		if m.Name == "download_units" {
			found = true
			if m.Status != "ok" {
				t.Fatalf("expected status ok, got %q", m.Status)
			}
			if m.Baseline == nil || *m.Baseline != 50 {
				t.Fatalf("expected baseline=50, got %v", m.Baseline)
			}
			if m.Comparison == nil || *m.Comparison != 80 {
				t.Fatalf("expected comparison=80, got %v", m.Comparison)
			}
			if m.Delta == nil || *m.Delta != 30 {
				t.Fatalf("expected delta=30, got %v", m.Delta)
			}
			if m.DeltaPercent == nil || *m.DeltaPercent != 60 {
				t.Fatalf("expected deltaPercent=60, got %v", m.DeltaPercent)
			}
		}
	}
	if !found {
		t.Fatal("expected download_units metric in output")
	}
}

func TestCompareResponseJSONStructure(t *testing.T) {
	resp := &compareResponse{
		AppID: "123456789",
		Source: compareSource{
			Name:         "sales",
			VendorNumber: "V",
			ReportType:   "SALES",
		},
		Baseline:   comparePeriod{Start: "2026-01-01", End: "2026-01-01", ReportsFound: 1},
		Comparison: comparePeriod{Start: "2026-02-01", End: "2026-02-01", ReportsFound: 1},
		Metrics: []compareMetric{
			{Name: "units", Unit: "count", Status: "ok"},
		},
		GeneratedAt: "2026-03-23T10:00:00Z",
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("json.Marshal error: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}

	if parsed["appId"] != "123456789" {
		t.Fatalf("unexpected appId: %v", parsed["appId"])
	}
	metrics, ok := parsed["metrics"].([]any)
	if !ok || len(metrics) != 1 {
		t.Fatalf("expected 1 metric, got %v", parsed["metrics"])
	}
}

func gzipCompareText(t *testing.T, value string) []byte {
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
