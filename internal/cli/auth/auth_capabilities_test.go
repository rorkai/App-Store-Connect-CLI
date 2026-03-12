package auth

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func TestAuthCapabilitiesCommandFlagValidation(t *testing.T) {
	t.Run("unsupported output", func(t *testing.T) {
		cmd := AuthCapabilitiesCommand()
		if err := cmd.FlagSet.Parse([]string{"--output", "yaml"}); err != nil {
			t.Fatalf("Parse() error: %v", err)
		}

		_, stderr := captureAuthOutput(t, func() {
			err := cmd.Exec(context.Background(), []string{})
			if !errors.Is(err, flag.ErrHelp) {
				t.Fatalf("expected flag.ErrHelp, got %v", err)
			}
		})
		if !strings.Contains(stderr, "unsupported format") {
			t.Fatalf("expected unsupported format error, got %q", stderr)
		}
	})

	t.Run("pretty requires json output", func(t *testing.T) {
		cmd := AuthCapabilitiesCommand()
		if err := cmd.FlagSet.Parse([]string{"--output", "table", "--pretty"}); err != nil {
			t.Fatalf("Parse() error: %v", err)
		}

		_, stderr := captureAuthOutput(t, func() {
			err := cmd.Exec(context.Background(), []string{})
			if !errors.Is(err, flag.ErrHelp) {
				t.Fatalf("expected flag.ErrHelp, got %v", err)
			}
		})
		if !strings.Contains(stderr, "--pretty is only valid with JSON output") {
			t.Fatalf("expected pretty/json error, got %q", stderr)
		}
	})
}

func TestDefaultAuthCapabilitiesOutputFormat(t *testing.T) {
	t.Run("json", func(t *testing.T) {
		t.Setenv("ASC_DEFAULT_OUTPUT", "json")
		shared.ResetDefaultOutputFormat()
		t.Cleanup(shared.ResetDefaultOutputFormat)

		if got := defaultAuthCapabilitiesOutputFormat(); got != "json" {
			t.Fatalf("defaultAuthCapabilitiesOutputFormat() = %q, want %q", got, "json")
		}
	})

	t.Run("markdown", func(t *testing.T) {
		t.Setenv("ASC_DEFAULT_OUTPUT", "markdown")
		shared.ResetDefaultOutputFormat()
		t.Cleanup(shared.ResetDefaultOutputFormat)

		if got := defaultAuthCapabilitiesOutputFormat(); got != "markdown" {
			t.Fatalf("defaultAuthCapabilitiesOutputFormat() = %q, want %q", got, "markdown")
		}
	})
}

func TestSummarizeAuthCapabilities(t *testing.T) {
	red := summarizeAuthCapabilities([]authCapabilityCheck{
		{Name: "apps", Status: "available"},
		{Name: "analytics", Status: "inconclusive", Message: "analytics probe failed"},
	})
	if red.Health != "red" || red.InconclusiveCount != 1 || red.NextAction != "analytics probe failed" {
		t.Fatalf("unexpected red summary: %+v", red)
	}

	yellow := summarizeAuthCapabilities([]authCapabilityCheck{
		{Name: "apps", Status: "available"},
		{Name: "sales", Status: "unavailable", Message: "sales unavailable"},
	})
	if yellow.Health != "yellow" || yellow.UnavailableCount != 1 || yellow.NextAction != "sales unavailable" {
		t.Fatalf("unexpected yellow summary: %+v", yellow)
	}

	green := summarizeAuthCapabilities([]authCapabilityCheck{
		{Name: "apps", Status: "available"},
		{Name: "builds", Status: "skipped", Message: "provide --app"},
	})
	if green.Health != "green" || green.SkippedCount != 1 || green.NextAction != "provide --app" {
		t.Fatalf("unexpected green summary: %+v", green)
	}
}

func TestAuthCapabilitiesCommandJSONOutput(t *testing.T) {
	prevCollector := authCapabilitiesCollector
	authCapabilitiesCollector = func(context.Context, string, string) (*authCapabilitiesResponse, error) {
		return &authCapabilitiesResponse{
			Summary: authCapabilitiesSummary{
				Health:         "green",
				NextAction:     "No action needed.",
				AvailableCount: 1,
			},
			Inputs: authCapabilitiesInputs{
				AppID:        "123456789",
				VendorNumber: "98765432",
			},
			Capabilities: []authCapabilityCheck{
				{Name: "apps", Scope: "account", Status: "available", Message: "can list apps"},
			},
			GeneratedAt: "2026-03-12T00:00:00Z",
		}, nil
	}
	t.Cleanup(func() {
		authCapabilitiesCollector = prevCollector
	})

	cmd := AuthCapabilitiesCommand()
	if err := cmd.FlagSet.Parse([]string{"--output", "json"}); err != nil {
		t.Fatalf("Parse() error: %v", err)
	}

	stdout, _ := captureAuthOutput(t, func() {
		if err := cmd.Exec(context.Background(), []string{}); err != nil {
			t.Fatalf("Exec() error: %v", err)
		}
	})

	var payload authCapabilitiesResponse
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("json.Unmarshal() error: %v", err)
	}
	if payload.Summary.Health != "green" || len(payload.Capabilities) != 1 {
		t.Fatalf("unexpected payload: %+v", payload)
	}
}

func TestCollectAuthCapabilities_AllAvailable(t *testing.T) {
	prevClientFn := authCapabilitiesClientFn
	prevNow := authCapabilitiesNow
	authCapabilitiesClientFn = func() (authCapabilitiesClient, error) {
		return &authCapabilitiesClientStub{
			salesDownload:   &asc.ReportDownload{Body: io.NopCloser(strings.NewReader("sales"))},
			financeDownload: &asc.ReportDownload{Body: io.NopCloser(strings.NewReader("finance"))},
		}, nil
	}
	authCapabilitiesNow = func() time.Time { return time.Date(2026, time.March, 12, 0, 0, 0, 0, time.UTC) }
	t.Cleanup(func() {
		authCapabilitiesClientFn = prevClientFn
		authCapabilitiesNow = prevNow
	})

	resp, err := collectAuthCapabilities(context.Background(), "123456789", "98765432")
	if err != nil {
		t.Fatalf("collectAuthCapabilities() error: %v", err)
	}
	if resp.Summary.Health != "green" || resp.Summary.AvailableCount != 7 {
		t.Fatalf("unexpected summary: %+v", resp.Summary)
	}
}

func TestCollectAuthCapabilities_SkipsMissingScopes(t *testing.T) {
	prevClientFn := authCapabilitiesClientFn
	authCapabilitiesClientFn = func() (authCapabilitiesClient, error) {
		return &authCapabilitiesClientStub{}, nil
	}
	t.Cleanup(func() {
		authCapabilitiesClientFn = prevClientFn
	})

	resp, err := collectAuthCapabilities(context.Background(), "", "")
	if err != nil {
		t.Fatalf("collectAuthCapabilities() error: %v", err)
	}
	if resp.Summary.AvailableCount != 1 || resp.Summary.SkippedCount != 6 {
		t.Fatalf("unexpected summary: %+v", resp.Summary)
	}
}

func TestCollectAuthCapabilities_MapsDeniedAndUnauthorized(t *testing.T) {
	prevClientFn := authCapabilitiesClientFn
	authCapabilitiesClientFn = func() (authCapabilitiesClient, error) {
		return &authCapabilitiesClientStub{
			getAppsErr:    asc.ErrForbidden,
			getBuildsErr:  asc.ErrForbidden,
			getReviewsErr: asc.ErrUnauthorized,
		}, nil
	}
	t.Cleanup(func() {
		authCapabilitiesClientFn = prevClientFn
	})

	resp, err := collectAuthCapabilities(context.Background(), "123456789", "")
	if err != nil {
		t.Fatalf("collectAuthCapabilities() error: %v", err)
	}
	if resp.Capabilities[0].Status != "unavailable" {
		t.Fatalf("apps status = %q, want unavailable", resp.Capabilities[0].Status)
	}
	if resp.Capabilities[1].Status != "unavailable" {
		t.Fatalf("builds status = %q, want unavailable", resp.Capabilities[1].Status)
	}
	if resp.Capabilities[2].Status != "inconclusive" || !strings.Contains(resp.Capabilities[2].Message, "unauthorized or expired") {
		t.Fatalf("unexpected reviews check: %+v", resp.Capabilities[2])
	}
}

func TestAuthCapabilityCheckFromErrorNotFound(t *testing.T) {
	check := authCapabilityCheckFromError(
		"analytics",
		"app",
		asc.ErrNotFound,
		"ok",
		"denied",
		"analytics probe failed",
	)
	if check.Status != "inconclusive" || !strings.Contains(check.Message, "not found or is not visible") {
		t.Fatalf("unexpected check: %+v", check)
	}
}

func TestCollectAuthCapabilities_UsesExpectedVendorReportDates(t *testing.T) {
	stub := &authCapabilitiesClientStub{
		salesDownload:   &asc.ReportDownload{Body: io.NopCloser(strings.NewReader("sales"))},
		financeDownload: &asc.ReportDownload{Body: io.NopCloser(strings.NewReader("finance"))},
	}
	prevClientFn := authCapabilitiesClientFn
	prevNow := authCapabilitiesNow
	authCapabilitiesClientFn = func() (authCapabilitiesClient, error) {
		return stub, nil
	}
	authCapabilitiesNow = func() time.Time { return time.Date(2026, time.March, 12, 0, 0, 0, 0, time.UTC) }
	t.Cleanup(func() {
		authCapabilitiesClientFn = prevClientFn
		authCapabilitiesNow = prevNow
	})

	if _, err := collectAuthCapabilities(context.Background(), "", "98765432"); err != nil {
		t.Fatalf("collectAuthCapabilities() error: %v", err)
	}
	if stub.lastSalesParams.ReportDate != "2026-03-11" {
		t.Fatalf("sales report date = %q, want %q", stub.lastSalesParams.ReportDate, "2026-03-11")
	}
	if stub.lastFinanceParams.ReportDate != "2026-02" {
		t.Fatalf("finance report date = %q, want %q", stub.lastFinanceParams.ReportDate, "2026-02")
	}
}

type authCapabilitiesClientStub struct {
	getAppsErr               error
	getBuildsErr             error
	getReviewsErr            error
	getSubscriptionGroupsErr error
	getAnalyticsErr          error
	getSalesErr              error
	getFinanceErr            error
	salesDownload            *asc.ReportDownload
	financeDownload          *asc.ReportDownload
	lastSalesParams          asc.SalesReportParams
	lastFinanceParams        asc.FinanceReportParams
}

func (s *authCapabilitiesClientStub) GetApps(context.Context, ...asc.AppsOption) (*asc.AppsResponse, error) {
	return &asc.AppsResponse{}, s.getAppsErr
}

func (s *authCapabilitiesClientStub) GetBuilds(context.Context, string, ...asc.BuildsOption) (*asc.BuildsResponse, error) {
	return &asc.BuildsResponse{}, s.getBuildsErr
}

func (s *authCapabilitiesClientStub) GetReviews(context.Context, string, ...asc.ReviewOption) (*asc.ReviewsResponse, error) {
	return &asc.ReviewsResponse{}, s.getReviewsErr
}

func (s *authCapabilitiesClientStub) GetSubscriptionGroups(context.Context, string, ...asc.SubscriptionGroupsOption) (*asc.SubscriptionGroupsResponse, error) {
	return &asc.SubscriptionGroupsResponse{}, s.getSubscriptionGroupsErr
}

func (s *authCapabilitiesClientStub) GetAnalyticsReportRequests(context.Context, string, ...asc.AnalyticsReportRequestsOption) (*asc.AnalyticsReportRequestsResponse, error) {
	return &asc.AnalyticsReportRequestsResponse{}, s.getAnalyticsErr
}

func (s *authCapabilitiesClientStub) GetSalesReport(_ context.Context, params asc.SalesReportParams) (*asc.ReportDownload, error) {
	s.lastSalesParams = params
	if s.salesDownload == nil {
		s.salesDownload = &asc.ReportDownload{Body: io.NopCloser(strings.NewReader(""))}
	}
	return s.salesDownload, s.getSalesErr
}

func (s *authCapabilitiesClientStub) DownloadFinanceReport(_ context.Context, params asc.FinanceReportParams) (*asc.ReportDownload, error) {
	s.lastFinanceParams = params
	if s.financeDownload == nil {
		s.financeDownload = &asc.ReportDownload{Body: io.NopCloser(strings.NewReader(""))}
	}
	return s.financeDownload, s.getFinanceErr
}
