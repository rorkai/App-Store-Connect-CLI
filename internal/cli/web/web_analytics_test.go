package web

import (
	"context"
	"errors"
	"flag"
	"net/http"
	"strings"
	"testing"

	"github.com/peterbourgon/ff/v3/ffcli"

	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

func TestWebAnalyticsCommandHierarchy(t *testing.T) {
	cmd := WebAnalyticsCommand()
	if cmd.Name != "analytics" {
		t.Fatalf("expected command name %q, got %q", "analytics", cmd.Name)
	}
	if len(cmd.Subcommands) != 13 {
		t.Fatalf("expected 13 subcommands, got %d", len(cmd.Subcommands))
	}

	names := map[string]bool{}
	for _, sub := range cmd.Subcommands {
		names[sub.Name] = true
	}
	for _, expected := range []string{
		"overview",
		"sources",
		"product-pages",
		"in-app-events",
		"app-clips",
		"campaigns",
		"sales",
		"subscriptions",
		"offers",
		"retention",
		"benchmarks",
		"metrics",
		"cohorts",
	} {
		if !names[expected] {
			t.Fatalf("expected %q subcommand", expected)
		}
	}
}

func TestWebAnalyticsSubcommandsResolveSessionWithinTimeoutContext(t *testing.T) {
	origResolveSession := resolveSessionFn
	t.Cleanup(func() {
		resolveSessionFn = origResolveSession
	})

	resolveErr := errors.New("stop before network call")
	tests := []struct {
		name  string
		build func() *ffcli.Command
		args  []string
	}{
		{
			name:  "sources",
			build: WebAnalyticsSourcesCommand,
			args: []string{
				"--apple-id", "user@example.com",
				"--app", "app-1",
				"--start", "2025-12-24",
				"--end", "2026-03-23",
			},
		},
		{
			name:  "product-pages",
			build: WebAnalyticsProductPagesCommand,
			args: []string{
				"--apple-id", "user@example.com",
				"--app", "app-1",
			},
		},
		{
			name:  "in-app-events",
			build: WebAnalyticsInAppEventsCommand,
			args: []string{
				"--apple-id", "user@example.com",
				"--app", "app-1",
				"--start", "2025-12-24",
				"--end", "2026-03-23",
			},
		},
		{
			name:  "app-clips",
			build: WebAnalyticsAppClipsCommand,
			args: []string{
				"--apple-id", "user@example.com",
				"--app", "app-1",
			},
		},
		{
			name:  "campaigns",
			build: WebAnalyticsCampaignsCommand,
			args: []string{
				"--apple-id", "user@example.com",
				"--app", "app-1",
				"--start", "2025-12-24",
				"--end", "2026-03-23",
			},
		},
		{
			name:  "sales",
			build: WebAnalyticsSalesCommand,
			args: []string{
				"--apple-id", "user@example.com",
				"--app", "app-1",
				"--start", "2025-12-24",
				"--end", "2026-03-23",
			},
		},
		{
			name:  "overview",
			build: WebAnalyticsOverviewCommand,
			args: []string{
				"--apple-id", "user@example.com",
				"--app", "app-1",
				"--start", "2025-12-24",
				"--end", "2026-03-23",
			},
		},
		{
			name:  "subscriptions",
			build: WebAnalyticsSubscriptionsCommand,
			args: []string{
				"--apple-id", "user@example.com",
				"--app", "app-1",
				"--start", "2025-12-24",
				"--end", "2026-03-23",
			},
		},
		{
			name:  "offers",
			build: WebAnalyticsOffersCommand,
			args: []string{
				"--apple-id", "user@example.com",
				"--app", "app-1",
			},
		},
		{
			name:  "metrics",
			build: WebAnalyticsMetricsCommand,
			args: []string{
				"--apple-id", "user@example.com",
				"--app", "app-1",
				"--start", "2025-12-24",
				"--end", "2026-03-23",
				"--measures", "units,redownloads",
			},
		},
		{
			name:  "benchmarks",
			build: WebAnalyticsBenchmarksCommand,
			args: []string{
				"--apple-id", "user@example.com",
				"--app", "app-1",
			},
		},
		{
			name:  "retention",
			build: WebAnalyticsRetentionCommand,
			args: []string{
				"--apple-id", "user@example.com",
				"--app", "app-1",
				"--start", "2026-02-22",
				"--end", "2026-03-23",
			},
		},
		{
			name:  "cohorts",
			build: WebAnalyticsCohortsCommand,
			args: []string{
				"--apple-id", "user@example.com",
				"--app", "app-1",
				"--start", "2025-12-24",
				"--end", "2026-03-23",
				"--measures", "cohort-download-to-paid-rate",
				"--periods", "d1,d7,d35",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hadDeadline := false
			resolveSessionFn = func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
				_, hadDeadline = ctx.Deadline()
				return nil, "", resolveErr
			}

			cmd := tt.build()
			if err := cmd.FlagSet.Parse(tt.args); err != nil {
				t.Fatalf("parse error: %v", err)
			}

			err := cmd.Exec(context.Background(), nil)
			if !errors.Is(err, resolveErr) {
				t.Fatalf("expected resolveErr, got %v", err)
			}
			if !hadDeadline {
				t.Fatal("expected resolveSessionFn to receive a timeout context")
			}
		})
	}
}

func TestWebAnalyticsOverviewCommandOutputsJSON(t *testing.T) {
	_ = stubWebProgressLabels(t)

	origResolveSession := resolveSessionFn
	origGetOverview := getAnalyticsOverviewFn
	t.Cleanup(func() {
		resolveSessionFn = origResolveSession
		getAnalyticsOverviewFn = origGetOverview
	})

	resolveSessionFn = func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{Client: &http.Client{}}, "cache", nil
	}
	getAnalyticsOverviewFn = func(ctx context.Context, client *webcore.Client, appID, startDate, endDate string) (*webcore.AnalyticsOverview, error) {
		if appID != "app-1" || startDate != "2025-12-24" || endDate != "2026-03-23" {
			t.Fatalf("unexpected overview args: %q %q %q", appID, startDate, endDate)
		}
		return &webcore.AnalyticsOverview{
			AppID:     appID,
			StartDate: startDate,
			EndDate:   endDate,
			Acquisition: []webcore.AnalyticsMeasureResult{
				{Measure: "units", Total: floatPtr(94), PercentChange: floatPtr(0.093)},
			},
		}, nil
	}

	cmd := WebAnalyticsOverviewCommand()
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

	if !strings.Contains(stdout, "\"appId\":\"app-1\"") {
		t.Fatalf("expected appId in JSON output, got %q", stdout)
	}
	if !strings.Contains(stdout, "\"measure\":\"units\"") {
		t.Fatalf("expected units measure in JSON output, got %q", stdout)
	}
	if !strings.Contains(stdout, "\"total\":94") {
		t.Fatalf("expected total 94 in JSON output, got %q", stdout)
	}
}

func TestWebAnalyticsSubscriptionsCommandTableIncludesSummaryLabels(t *testing.T) {
	_ = stubWebProgressLabels(t)

	origResolveSession := resolveSessionFn
	origGetSubscriptions := getAnalyticsSubscriptionsSummaryFn
	t.Cleanup(func() {
		resolveSessionFn = origResolveSession
		getAnalyticsSubscriptionsSummaryFn = origGetSubscriptions
	})

	resolveSessionFn = func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{Client: &http.Client{}}, "cache", nil
	}
	getAnalyticsSubscriptionsSummaryFn = func(ctx context.Context, client *webcore.Client, appID, startDate, endDate string) (*webcore.AnalyticsSubscriptionsSummary, error) {
		return &webcore.AnalyticsSubscriptionsSummary{
			AppID:     appID,
			StartDate: startDate,
			EndDate:   endDate,
			Summary: []webcore.AnalyticsMeasureResult{
				{Measure: "subscription-state-plans-active", Total: floatPtr(4), PercentChange: floatPtr(3)},
				{Measure: "revenue-recurring", Total: floatPtr(6), PercentChange: floatPtr(0.2)},
			},
		}, nil
	}

	cmd := WebAnalyticsSubscriptionsCommand()
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

	if !strings.Contains(stdout, "Active Plans") {
		t.Fatalf("expected Active Plans label in table output, got %q", stdout)
	}
	if !strings.Contains(stdout, "Monthly Recurring Revenue") {
		t.Fatalf("expected Monthly Recurring Revenue label in table output, got %q", stdout)
	}
	if !strings.Contains(stdout, "Summary Cards") {
		t.Fatalf("expected Summary Cards section in table output, got %q", stdout)
	}
	if !strings.Contains(stdout, "Subscriptions\n-------------") {
		t.Fatalf("expected sectioned table heading in table output, got %q", stdout)
	}
	if !strings.Contains(stdout, "+300.0%") {
		t.Fatalf("expected ratio-based percent change in table output, got %q", stdout)
	}
}

func TestAnalyticsPercentChangeStringUsesRatioValues(t *testing.T) {
	if got := analyticsPercentChangeString(floatPtr(3)); got != "+300.0%" {
		t.Fatalf("expected ratio value to render as +300.0%%, got %q", got)
	}
	if got := analyticsPercentChangeString(floatPtr(0.093)); got != "+9.3%" {
		t.Fatalf("expected 0.093 ratio to render as +9.3%%, got %q", got)
	}
}

func TestWebAnalyticsOverviewCommandTableUsesSectionedLayout(t *testing.T) {
	_ = stubWebProgressLabels(t)

	origResolveSession := resolveSessionFn
	origGetOverview := getAnalyticsOverviewFn
	t.Cleanup(func() {
		resolveSessionFn = origResolveSession
		getAnalyticsOverviewFn = origGetOverview
	})

	resolveSessionFn = func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{Client: &http.Client{}}, "cache", nil
	}
	getAnalyticsOverviewFn = func(ctx context.Context, client *webcore.Client, appID, startDate, endDate string) (*webcore.AnalyticsOverview, error) {
		return &webcore.AnalyticsOverview{
			AppID:     appID,
			StartDate: startDate,
			EndDate:   endDate,
			Acquisition: []webcore.AnalyticsMeasureResult{
				{Measure: "units", Total: floatPtr(94), PreviousTotal: floatPtr(86), PercentChange: floatPtr(0.093)},
			},
			Sales: []webcore.AnalyticsMeasureResult{
				{Measure: "proceeds", Total: floatPtr(46), PreviousTotal: floatPtr(38), PercentChange: floatPtr(0.211)},
			},
		}, nil
	}

	cmd := WebAnalyticsOverviewCommand()
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

	if !strings.Contains(stdout, "Overview\n--------") {
		t.Fatalf("expected Overview section heading in table output, got %q", stdout)
	}
	if !strings.Contains(stdout, "Acquisition\n-----------") {
		t.Fatalf("expected Acquisition section heading in table output, got %q", stdout)
	}
	if !strings.Contains(stdout, "Sales\n-----") {
		t.Fatalf("expected Sales section heading in table output, got %q", stdout)
	}
}

func TestWebAnalyticsMetricsCommandRequiresMeasures(t *testing.T) {
	cmd := WebAnalyticsMetricsCommand()
	if err := cmd.FlagSet.Parse([]string{
		"--app", "app-1",
		"--start", "2025-12-24",
		"--end", "2026-03-23",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	_, stderr := captureOutput(t, func() {
		err := cmd.Exec(context.Background(), nil)
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected flag.ErrHelp, got %v", err)
		}
	})
	if !strings.Contains(stderr, "--measures is required") {
		t.Fatalf("unexpected stderr: %q", stderr)
	}
}

func TestWebAnalyticsMetricsCommandOutputsJSON(t *testing.T) {
	_ = stubWebProgressLabels(t)

	origResolveSession := resolveSessionFn
	origGetMeasures := getAnalyticsMeasuresFn
	t.Cleanup(func() {
		resolveSessionFn = origResolveSession
		getAnalyticsMeasuresFn = origGetMeasures
	})

	resolveSessionFn = func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{Client: &http.Client{}}, "cache", nil
	}
	getAnalyticsMeasuresFn = func(ctx context.Context, client *webcore.Client, req webcore.AnalyticsMeasuresRequest) (*webcore.AnalyticsMeasuresResponse, error) {
		if req.AppID != "app-1" || len(req.Measures) != 2 {
			t.Fatalf("unexpected metrics request: %#v", req)
		}
		return &webcore.AnalyticsMeasuresResponse{
			Size: 2,
			Results: []webcore.AnalyticsMeasureResult{
				{Measure: "units", Total: floatPtr(94)},
				{Measure: "redownloads", Total: floatPtr(32)},
			},
		}, nil
	}

	cmd := WebAnalyticsMetricsCommand()
	if err := cmd.FlagSet.Parse([]string{
		"--app", "app-1",
		"--start", "2025-12-24",
		"--end", "2026-03-23",
		"--measures", "units,redownloads",
		"--output", "json",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	stdout, _ := captureOutput(t, func() {
		if err := cmd.Exec(context.Background(), nil); err != nil {
			t.Fatalf("exec error: %v", err)
		}
	})

	if !strings.Contains(stdout, "\"measure\":\"units\"") || !strings.Contains(stdout, "\"measure\":\"redownloads\"") {
		t.Fatalf("expected measures in JSON output, got %q", stdout)
	}
}

func TestSortedAnalyticsPeriodsCoversExtendedPeriods(t *testing.T) {
	values := map[string]*float64{
		"d90":  floatPtr(1),
		"d14":  floatPtr(1),
		"d180": floatPtr(1),
		"d35":  floatPtr(1),
		"d7":   floatPtr(1),
		"m2":   floatPtr(1),
		"m12":  floatPtr(1),
		"m9":   floatPtr(1),
		"m1":   floatPtr(1),
	}

	got := strings.Join(sortedAnalyticsPeriods(values), ",")
	want := "d7,d14,d35,d90,d180,m1,m2,m9,m12"
	if got != want {
		t.Fatalf("unexpected sorted periods: got %q want %q", got, want)
	}
}

func floatPtr(v float64) *float64 {
	return &v
}
