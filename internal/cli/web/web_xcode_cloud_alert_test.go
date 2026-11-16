package web

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

func TestValidateUsageAlertThresholds(t *testing.T) {
	tests := []struct {
		name       string
		warnAt     int
		criticalAt int
		wantErr    string
	}{
		{
			name:       "valid thresholds",
			warnAt:     80,
			criticalAt: 95,
		},
		{
			name:       "warn too low",
			warnAt:     0,
			criticalAt: 95,
			wantErr:    "--warn-at must be between 1 and 99",
		},
		{
			name:       "critical too high",
			warnAt:     80,
			criticalAt: 101,
			wantErr:    "--critical-at must be between 1 and 100",
		},
		{
			name:       "warn equal critical",
			warnAt:     90,
			criticalAt: 90,
			wantErr:    "--warn-at must be less than --critical-at",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateUsageAlertThresholds(tt.warnAt, tt.criticalAt)
			if tt.wantErr == "" && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error %q", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("expected error to contain %q, got %v", tt.wantErr, err)
				}
			}
		})
	}
}

func TestWebXcodeCloudUsageAlertRejectsInvalidNotifyOn(t *testing.T) {
	d, depsStdout, depsStderr := newWebTestDeps()
	cmd := webXcodeCloudUsageAlertCommand(d)
	if err := cmd.FlagSet.Parse([]string{
		"--apple-id", "user@example.com",
		"--notify-on", "invalid",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	var runErr error
	_, stderr := runWithWebTestDeps(depsStdout, depsStderr, func() {
		runErr = cmd.Exec(context.Background(), nil)
	})
	if !errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp, got %v", runErr)
	}
	if !strings.Contains(stderr, "--notify-on must be one of: none, warning, critical, always") {
		t.Fatalf("expected notify-on usage error, got %q", stderr)
	}
}

func TestWebXcodeCloudUsageAlertRejectsInvalidWebhookHeader(t *testing.T) {
	d, depsStdout, depsStderr := newWebTestDeps()
	cmd := webXcodeCloudUsageAlertCommand(d)
	if err := cmd.FlagSet.Parse([]string{
		"--apple-id", "user@example.com",
		"--webhook", "https://example.com/alert",
		"--webhook-header", "Authorization Bearer token",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	var runErr error
	_, stderr := runWithWebTestDeps(depsStdout, depsStderr, func() {
		runErr = cmd.Exec(context.Background(), nil)
	})
	if !errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp, got %v", runErr)
	}
	if !strings.Contains(stderr, "--webhook-header must be in 'Key: Value' format") {
		t.Fatalf("expected webhook-header usage error, got %q", stderr)
	}
}

func TestWebXcodeCloudUsageAlertReturnsThresholdErrorWithJSONOutput(t *testing.T) {
	origResolveSession := resolveSessionFn
	t.Cleanup(func() {
		resolveSessionFn = origResolveSession
	})

	webTestNow := func() time.Time { return time.Date(2026, time.February, 28, 10, 0, 0, 0, time.UTC) }
	summary := &webcore.CIUsageSummary{
		Plan: webcore.CIUsagePlan{
			Name:      "Starter",
			Used:      920,
			Available: 80,
			Total:     1000,
			ResetDate: "2026-03-01",
		},
		Links: map[string]string{"manage": "https://appstoreconnect.apple.com/xcode-cloud"},
	}
	resolveSessionFn = stubUsageAlertSessionWithResponses(t, summary, nil)

	d, depsStdout, depsStderr := newWebTestDeps()
	d.Now = webTestNow
	cmd := webXcodeCloudUsageAlertCommand(d)
	if err := cmd.FlagSet.Parse([]string{
		"--apple-id", "user@example.com",
		"--trend-months", "0",
		"--fail-on", "warning",
		"--output", "json",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	var runErr error
	stdout, _ := runWithWebTestDeps(depsStdout, depsStderr, func() {
		runErr = cmd.Exec(context.Background(), nil)
	})
	if runErr == nil {
		t.Fatal("expected threshold breach error")
	}
	if !strings.Contains(runErr.Error(), "threshold breach") {
		t.Fatalf("expected threshold breach error, got %v", runErr)
	}

	var result CIUsageAlertResult
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &result); err != nil {
		t.Fatalf("expected valid json output, got error %v (stdout=%q)", err, stdout)
	}
	if result.Severity != usageAlertSeverityWarning {
		t.Fatalf("expected warning severity, got %q", result.Severity)
	}
	if result.Plan.UsedPercent != 92 {
		t.Fatalf("expected used percent 92, got %d", result.Plan.UsedPercent)
	}
}

func TestWebXcodeCloudUsageAlertUsesExactThresholdRatios(t *testing.T) {
	origResolveSession := resolveSessionFn
	t.Cleanup(func() {
		resolveSessionFn = origResolveSession
	})

	webTestNow := func() time.Time { return time.Date(2026, time.February, 28, 10, 0, 0, 0, time.UTC) }
	summary := &webcore.CIUsageSummary{
		Plan: webcore.CIUsagePlan{
			Name:      "Starter",
			Used:      945,
			Available: 55,
			Total:     1000,
			ResetDate: "2026-03-01",
		},
	}
	resolveSessionFn = stubUsageAlertSessionWithResponses(t, summary, nil)

	d, depsStdout, depsStderr := newWebTestDeps()
	d.Now = webTestNow
	cmd := webXcodeCloudUsageAlertCommand(d)
	if err := cmd.FlagSet.Parse([]string{
		"--apple-id", "user@example.com",
		"--warn-at", "95",
		"--critical-at", "96",
		"--trend-months", "0",
		"--fail-on", "warning",
		"--output", "json",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	var runErr error
	stdout, _ := runWithWebTestDeps(depsStdout, depsStderr, func() {
		runErr = cmd.Exec(context.Background(), nil)
	})
	if runErr != nil {
		t.Fatalf("expected no threshold error, got %v", runErr)
	}

	var result CIUsageAlertResult
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &result); err != nil {
		t.Fatalf("expected valid json output, got error %v", err)
	}
	if result.Severity != usageAlertSeverityOK {
		t.Fatalf("expected ok severity with exact ratio checks, got %q", result.Severity)
	}
	if result.Plan.UsedPercent != 95 {
		t.Fatalf("expected rounded display percent 95, got %d", result.Plan.UsedPercent)
	}
}

func TestWebXcodeCloudUsageAlertSendsSlackOnCritical(t *testing.T) {
	origResolveSession := resolveSessionFn
	origSendSlack := sendUsageAlertSlackFn
	t.Cleanup(func() {
		resolveSessionFn = origResolveSession
		sendUsageAlertSlackFn = origSendSlack
	})

	webTestNow := func() time.Time { return time.Date(2026, time.February, 28, 10, 0, 0, 0, time.UTC) }
	summary := &webcore.CIUsageSummary{
		Plan: webcore.CIUsagePlan{
			Name:      "Pro",
			Used:      980,
			Available: 20,
			Total:     1000,
			ResetDate: "2026-03-01",
		},
	}
	resolveSessionFn = stubUsageAlertSessionWithResponses(t, summary, nil)

	slackCalls := 0
	sendUsageAlertSlackFn = func(ctx context.Context, _ *shared.Deps, webhookURL string, result *CIUsageAlertResult) (int, error) {
		slackCalls++
		if webhookURL != "https://hooks.slack.com/services/T/B/KEY" {
			t.Fatalf("unexpected slack webhook url %q", webhookURL)
		}
		if result == nil || result.Severity != usageAlertSeverityCritical {
			t.Fatalf("expected critical result in slack payload, got %+v", result)
		}
		return http.StatusOK, nil
	}

	d, depsStdout, depsStderr := newWebTestDeps()
	d.Now = webTestNow
	cmd := webXcodeCloudUsageAlertCommand(d)
	if err := cmd.FlagSet.Parse([]string{
		"--apple-id", "user@example.com",
		"--trend-months", "0",
		"--fail-on", "none",
		"--notify-on", "critical",
		"--slack-webhook", "https://hooks.slack.com/services/T/B/KEY",
		"--output", "json",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	var runErr error
	stdout, _ := runWithWebTestDeps(depsStdout, depsStderr, func() {
		runErr = cmd.Exec(context.Background(), nil)
	})
	if runErr != nil {
		t.Fatalf("expected no error, got %v", runErr)
	}
	if slackCalls != 1 {
		t.Fatalf("expected one slack call, got %d", slackCalls)
	}

	var result CIUsageAlertResult
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &result); err != nil {
		t.Fatalf("expected valid json output, got error %v", err)
	}
	if len(result.Notifications) != 1 {
		t.Fatalf("expected one notification result, got %d", len(result.Notifications))
	}
	if !result.Notifications[0].Triggered || !result.Notifications[0].Delivered {
		t.Fatalf("expected delivered notification, got %+v", result.Notifications[0])
	}
}

func TestWebXcodeCloudUsageAlertDoesNotNotifyBelowLevel(t *testing.T) {
	origResolveSession := resolveSessionFn
	origSendSlack := sendUsageAlertSlackFn
	t.Cleanup(func() {
		resolveSessionFn = origResolveSession
		sendUsageAlertSlackFn = origSendSlack
	})

	webTestNow := func() time.Time { return time.Date(2026, time.February, 28, 10, 0, 0, 0, time.UTC) }
	summary := &webcore.CIUsageSummary{
		Plan: webcore.CIUsagePlan{
			Name:      "Pro",
			Used:      850,
			Available: 150,
			Total:     1000,
			ResetDate: "2026-03-01",
		},
	}
	resolveSessionFn = stubUsageAlertSessionWithResponses(t, summary, nil)

	slackCalls := 0
	sendUsageAlertSlackFn = func(ctx context.Context, _ *shared.Deps, webhookURL string, result *CIUsageAlertResult) (int, error) {
		slackCalls++
		return http.StatusOK, nil
	}

	d, depsStdout, depsStderr := newWebTestDeps()
	d.Now = webTestNow
	cmd := webXcodeCloudUsageAlertCommand(d)
	if err := cmd.FlagSet.Parse([]string{
		"--apple-id", "user@example.com",
		"--trend-months", "0",
		"--fail-on", "none",
		"--notify-on", "critical",
		"--slack-webhook", "https://hooks.slack.com/services/T/B/KEY",
		"--output", "json",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	var runErr error
	stdout, _ := runWithWebTestDeps(depsStdout, depsStderr, func() {
		runErr = cmd.Exec(context.Background(), nil)
	})
	if runErr != nil {
		t.Fatalf("expected no error, got %v", runErr)
	}
	if slackCalls != 0 {
		t.Fatalf("expected zero slack calls, got %d", slackCalls)
	}

	var result CIUsageAlertResult
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &result); err != nil {
		t.Fatalf("expected valid json output, got error %v", err)
	}
	if len(result.Notifications) != 1 {
		t.Fatalf("expected one notification result, got %d", len(result.Notifications))
	}
	if result.Notifications[0].Triggered {
		t.Fatalf("expected non-triggered notification, got %+v", result.Notifications[0])
	}
}

func TestWebXcodeCloudUsageAlertLoadsMonthlyTrend(t *testing.T) {
	origResolveSession := resolveSessionFn
	t.Cleanup(func() {
		resolveSessionFn = origResolveSession
	})

	webTestNow := func() time.Time { return time.Date(2026, time.February, 28, 10, 0, 0, 0, time.UTC) }
	summary := &webcore.CIUsageSummary{
		Plan: webcore.CIUsagePlan{
			Name:      "Starter",
			Used:      700,
			Available: 300,
			Total:     1000,
			ResetDate: "2026-03-01",
		},
	}
	months := &webcore.CIUsageMonths{
		Usage: []webcore.CIMonthUsage{
			{Year: 2026, Month: 1, Duration: 320, NumberOfBuilds: 22},
			{Year: 2026, Month: 2, Duration: 380, NumberOfBuilds: 24},
		},
	}
	resolveSessionFn = stubUsageAlertSessionWithResponses(t, summary, months)

	d, depsStdout, depsStderr := newWebTestDeps()
	d.Now = webTestNow
	cmd := webXcodeCloudUsageAlertCommand(d)
	if err := cmd.FlagSet.Parse([]string{
		"--apple-id", "user@example.com",
		"--fail-on", "none",
		"--trend-months", "2",
		"--output", "json",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	var runErr error
	stdout, _ := runWithWebTestDeps(depsStdout, depsStderr, func() {
		runErr = cmd.Exec(context.Background(), nil)
	})
	if runErr != nil {
		t.Fatalf("expected no error, got %v", runErr)
	}

	var result CIUsageAlertResult
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &result); err != nil {
		t.Fatalf("expected valid json output, got error %v", err)
	}
	if result.Trend == nil {
		t.Fatal("expected trend payload")
	}
	if !result.Trend.Available {
		t.Fatalf("expected available trend, got %+v", result.Trend)
	}
	if len(result.Trend.Months) != 2 {
		t.Fatalf("expected two trend months, got %d", len(result.Trend.Months))
	}
}

func TestWebXcodeCloudUsageAlertTrendUsesMonthAnchoredWindow(t *testing.T) {
	origResolveSession := resolveSessionFn
	t.Cleanup(func() {
		resolveSessionFn = origResolveSession
	})

	webTestNow := func() time.Time { return time.Date(2026, time.March, 31, 20, 15, 0, 0, time.UTC) }

	summary := &webcore.CIUsageSummary{
		Plan: webcore.CIUsagePlan{
			Name:      "Starter",
			Used:      700,
			Available: 300,
			Total:     1000,
			ResetDate: "2026-04-01",
		},
	}
	months := &webcore.CIUsageMonths{
		Usage: []webcore.CIMonthUsage{
			{Year: 2026, Month: 2, Duration: 310, NumberOfBuilds: 20},
			{Year: 2026, Month: 3, Duration: 350, NumberOfBuilds: 21},
		},
	}

	sawMonthsRequest := false
	resolveSessionFn = func(
		ctx context.Context,
		appleID, password, twoFactorCode string,
	) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{
			PublicProviderID: "TEAM-123",
			Client: &http.Client{
				Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
					switch {
					case strings.Contains(req.URL.Path, "/usage/summary"):
						return usageAlertJSONResponse(t, http.StatusOK, summary), nil
					case strings.Contains(req.URL.Path, "/usage/months"):
						sawMonthsRequest = true
						query := req.URL.Query()
						if query.Get("start_month") != "2" || query.Get("start_year") != "2026" {
							t.Fatalf("expected start window 02/2026, got %s/%s", query.Get("start_month"), query.Get("start_year"))
						}
						if query.Get("end_month") != "3" || query.Get("end_year") != "2026" {
							t.Fatalf("expected end window 03/2026, got %s/%s", query.Get("end_month"), query.Get("end_year"))
						}
						return usageAlertJSONResponse(t, http.StatusOK, months), nil
					default:
						return usageAlertJSONResponse(t, http.StatusNotFound, map[string]any{"error": "not found"}), nil
					}
				}),
			},
		}, "", nil
	}

	d, depsStdout, depsStderr := newWebTestDeps()
	d.Now = webTestNow
	cmd := webXcodeCloudUsageAlertCommand(d)
	if err := cmd.FlagSet.Parse([]string{
		"--apple-id", "user@example.com",
		"--fail-on", "none",
		"--trend-months", "2",
		"--output", "json",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	var runErr error
	_, _ = runWithWebTestDeps(depsStdout, depsStderr, func() {
		runErr = cmd.Exec(context.Background(), nil)
	})
	if runErr != nil {
		t.Fatalf("expected no error, got %v", runErr)
	}
	if !sawMonthsRequest {
		t.Fatal("expected usage months request")
	}
}

func stubUsageAlertSessionWithResponses(
	t *testing.T,
	summary *webcore.CIUsageSummary,
	months *webcore.CIUsageMonths,
) func(context.Context, string, string, string) (*webcore.AuthSession, string, error) {
	t.Helper()

	if summary == nil {
		summary = &webcore.CIUsageSummary{}
	}
	if months == nil {
		months = &webcore.CIUsageMonths{}
	}

	return func(
		ctx context.Context,
		appleID, password, twoFactorCode string,
	) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{
			PublicProviderID: "TEAM-123",
			Client: &http.Client{
				Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
					switch {
					case strings.Contains(req.URL.Path, "/usage/summary"):
						return usageAlertJSONResponse(t, http.StatusOK, summary), nil
					case strings.Contains(req.URL.Path, "/usage/months"):
						return usageAlertJSONResponse(t, http.StatusOK, months), nil
					default:
						return usageAlertJSONResponse(t, http.StatusNotFound, map[string]any{
							"error": "not found",
						}), nil
					}
				}),
			},
		}, "", nil
	}
}

func usageAlertJSONResponse(t *testing.T, status int, payload any) *http.Response {
	t.Helper()
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal response payload: %v", err)
	}
	return &http.Response{
		StatusCode: status,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(string(body))),
	}
}
