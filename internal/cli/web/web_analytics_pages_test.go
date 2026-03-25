package web

import (
	"context"
	"net/http"
	"strings"
	"testing"

	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

func TestWebAnalyticsSourcesCommandOutputsJSON(t *testing.T) {
	_ = stubWebProgressLabels(t)

	origResolveSession := resolveSessionFn
	origGetSources := getAnalyticsSourcesPageFn
	t.Cleanup(func() {
		resolveSessionFn = origResolveSession
		getAnalyticsSourcesPageFn = origGetSources
	})

	resolveSessionFn = func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{Client: &http.Client{}}, "cache", nil
	}
	getAnalyticsSourcesPageFn = func(ctx context.Context, client *webcore.Client, appID, startDate, endDate string) (*webcore.AnalyticsSourcesPage, error) {
		return &webcore.AnalyticsSourcesPage{
			AppID:          appID,
			StartDate:      startDate,
			EndDate:        endDate,
			Measure:        "pageViewUnique",
			GroupDimension: "source",
			Result: &webcore.AnalyticsTimeseriesResponse{
				Size: 1,
				Results: []webcore.AnalyticsTimeseriesResult{
					{
						Group: map[string]any{"key": "Other", "title": "App Store Browse"},
						Data: []map[string]any{
							{"date": "2025-12-24T00:00:00Z", "pageViewUnique": 2.0},
							{"date": "2025-12-25T00:00:00Z", "pageViewUnique": 1.0},
						},
					},
				},
			},
		}, nil
	}

	cmd := WebAnalyticsSourcesCommand()
	if err := cmd.FlagSet.Parse([]string{
		"--app", "app-1",
		"--start", "2025-12-24",
		"--end", "2026-03-23",
		"--output", "json",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	stdout, _ := captureOutput(t, func() {
		if err := cmd.Exec(context.Background(), nil); err != nil {
			t.Fatalf("exec error: %v", err)
		}
	})

	if !strings.Contains(stdout, "\"groupDimension\":\"source\"") {
		t.Fatalf("expected groupDimension in JSON output, got %q", stdout)
	}
	if !strings.Contains(stdout, "\"pageViewUnique\"") {
		t.Fatalf("expected measure key in JSON output, got %q", stdout)
	}
}

func TestWebAnalyticsSalesCommandTableIncludesRevenueLabels(t *testing.T) {
	_ = stubWebProgressLabels(t)

	origResolveSession := resolveSessionFn
	origGetSales := getAnalyticsSalesSummaryFn
	t.Cleanup(func() {
		resolveSessionFn = origResolveSession
		getAnalyticsSalesSummaryFn = origGetSales
	})

	resolveSessionFn = func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{Client: &http.Client{}}, "cache", nil
	}
	getAnalyticsSalesSummaryFn = func(ctx context.Context, client *webcore.Client, appID, startDate, endDate string) (*webcore.AnalyticsSalesSummary, error) {
		return &webcore.AnalyticsSalesSummary{
			AppID:     appID,
			StartDate: startDate,
			EndDate:   endDate,
			Summary: []webcore.AnalyticsMeasureResult{
				{Measure: "proceeds", Total: floatPtr(46)},
			},
			DownloadToPaid: &webcore.AnalyticsCohortsResponse{
				Results: map[string][]any{
					"period":                       {"d1"},
					"cohort-download-to-paid-rate": {2.4},
				},
			},
			RevenueByTerritory: &webcore.AnalyticsBreakdown{
				Name:  "Proceeds by Territory",
				Items: []webcore.AnalyticsBreakdownItem{{Label: "Poland", Value: 25}},
			},
		}, nil
	}

	cmd := WebAnalyticsSalesCommand()
	if err := cmd.FlagSet.Parse([]string{
		"--app", "app-1",
		"--start", "2025-12-24",
		"--end", "2026-03-23",
		"--output", "table",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	stdout, _ := captureOutput(t, func() {
		if err := cmd.Exec(context.Background(), nil); err != nil {
			t.Fatalf("exec error: %v", err)
		}
	})

	if !strings.Contains(stdout, "Proceeds by Territory") {
		t.Fatalf("expected territory breakdown label in table output, got %q", stdout)
	}
	if !strings.Contains(stdout, "Download to Paid") {
		t.Fatalf("expected cohort label in table output, got %q", stdout)
	}
}

func TestWebAnalyticsBenchmarksCommandTableIncludesPercentiles(t *testing.T) {
	_ = stubWebProgressLabels(t)

	origResolveSession := resolveSessionFn
	origGetBenchmarks := getAnalyticsBenchmarksFn
	t.Cleanup(func() {
		resolveSessionFn = origResolveSession
		getAnalyticsBenchmarksFn = origGetBenchmarks
	})

	resolveSessionFn = func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{Client: &http.Client{}}, "cache", nil
	}
	getAnalyticsBenchmarksFn = func(ctx context.Context, client *webcore.Client, appID string) (*webcore.AnalyticsBenchmarksSummary, error) {
		return &webcore.AnalyticsBenchmarksSummary{
			AppID:        appID,
			Category:     "GENRE_6027",
			WeekStart:    "2026-02-23",
			WeekEnd:      "2026-03-01",
			PeerGroupIDs: []string{"202", "74"},
			SelectedGroups: []webcore.AnalyticsBenchmarkPeerGroup{
				{ID: "202", Title: "IPG-202", Monetization: "SUBS", Size: "ALL", MemberOf: true, Primary: true},
			},
			Metrics: []webcore.AnalyticsBenchmarkMetric{
				{Key: "conversionRate", Label: "Conversion Rate", AppValue: floatPtr(0.62), P25: floatPtr(1.42), P50: floatPtr(3.06), P75: floatPtr(6.99)},
			},
		}, nil
	}

	cmd := WebAnalyticsBenchmarksCommand()
	if err := cmd.FlagSet.Parse([]string{
		"--app", "app-1",
		"--output", "table",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	stdout, _ := captureOutput(t, func() {
		if err := cmd.Exec(context.Background(), nil); err != nil {
			t.Fatalf("exec error: %v", err)
		}
	})

	if !strings.Contains(stdout, "Conversion Rate") {
		t.Fatalf("expected metric label in table output, got %q", stdout)
	}
	if !strings.Contains(stdout, "6.99%") {
		t.Fatalf("expected percentile output in table output, got %q", stdout)
	}
	if !strings.Contains(stdout, "Peer Groups") {
		t.Fatalf("expected Peer Groups section in table output, got %q", stdout)
	}
	if !strings.Contains(stdout, "Benchmark Metrics") {
		t.Fatalf("expected Benchmark Metrics section in table output, got %q", stdout)
	}
}

func TestWebAnalyticsProductPagesCommandOutputsCapabilityJSON(t *testing.T) {
	_ = stubWebProgressLabels(t)

	origResolveSession := resolveSessionFn
	origGetAppInfo := getAnalyticsAppInfoFn
	t.Cleanup(func() {
		resolveSessionFn = origResolveSession
		getAnalyticsAppInfoFn = origGetAppInfo
	})

	resolveSessionFn = func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{Client: &http.Client{}}, "cache", nil
	}
	getAnalyticsAppInfoFn = func(ctx context.Context, client *webcore.Client, appID string) (*webcore.AnalyticsAppInfoResult, error) {
		return &webcore.AnalyticsAppInfoResult{
			AdamID:   appID,
			BundleID: "com.example.app",
			Features: []webcore.AnalyticsAppFeature{{ID: "customProductPage", Count: 0}},
		}, nil
	}

	cmd := WebAnalyticsProductPagesCommand()
	if err := cmd.FlagSet.Parse([]string{
		"--app", "app-1",
		"--output", "json",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	stdout, _ := captureOutput(t, func() {
		if err := cmd.Exec(context.Background(), nil); err != nil {
			t.Fatalf("exec error: %v", err)
		}
	})

	if !strings.Contains(stdout, "\"page\":\"product-pages\"") {
		t.Fatalf("expected page in JSON output, got %q", stdout)
	}
	if !strings.Contains(stdout, "\"status\":\"unavailable\"") {
		t.Fatalf("expected unavailable status in JSON output, got %q", stdout)
	}
}
