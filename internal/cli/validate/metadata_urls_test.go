package validate

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"strings"
	"sync"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/metadataurl"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/validation"
)

type fakeValidateURLChecker struct {
	mu      sync.Mutex
	results map[string]metadataurl.Result
	errors  map[string]error
	calls   map[string]int
}

func (f *fakeValidateURLChecker) Check(_ context.Context, rawURL string) (metadataurl.Result, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.calls == nil {
		f.calls = make(map[string]int)
	}
	f.calls[rawURL]++
	if err := f.errors[rawURL]; err != nil {
		return metadataurl.Result{}, err
	}
	return f.results[rawURL], nil
}

func TestValidateURLChecksDeduplicateAndRedactFindings(t *testing.T) {
	const duplicatedURL = "https://public.example/help"
	const failedURL = "https://private.example/policy?token=secret"
	checker := &fakeValidateURLChecker{
		results: map[string]metadataurl.Result{
			duplicatedURL: {
				FinalURL:   mustParseValidateURL(t, "https://public.example/"),
				StatusCode: 200,
			},
		},
		errors: map[string]error{
			failedURL: errors.New("request failed for https://private.example/policy?token=secret"),
		},
	}
	targets := []validateURLTarget{
		{RawURL: duplicatedURL, Locale: "en-US", Field: "supportUrl", ResourceType: "appStoreVersionLocalization", ResourceID: "version-loc-1"},
		{RawURL: duplicatedURL, Locale: "fr-FR", Field: "supportUrl", ResourceType: "appStoreVersionLocalization", ResourceID: "version-loc-2"},
		{RawURL: failedURL, Locale: "en-US", Field: "privacyPolicyUrl", ResourceType: "appInfoLocalization", ResourceID: "app-info-loc-1"},
	}

	checks, err := checkValidateURLs(context.Background(), checker, targets)
	if err != nil {
		t.Fatalf("checkValidateURLs() error: %v", err)
	}
	if checker.calls[duplicatedURL] != 1 {
		t.Fatalf("duplicate URL calls = %d, want 1", checker.calls[duplicatedURL])
	}
	if len(checks) != 3 {
		t.Fatalf("checks = %+v, want root findings for both fields and one request failure", checks)
	}
	for _, check := range checks {
		if check.Severity != validation.SeverityWarning {
			t.Fatalf("check = %+v, want warning", check)
		}
		if strings.Contains(check.Message, "public.example") || strings.Contains(check.Message, "private.example") || strings.Contains(check.Message, "secret") {
			t.Fatalf("check leaked URL details: %+v", check)
		}
	}
}

func TestValidateURLChecksReportsRedirectAndStatusByStableIDs(t *testing.T) {
	const rawURL = "https://support.example/help"
	checker := &fakeValidateURLChecker{results: map[string]metadataurl.Result{
		rawURL: {
			FinalURL:   mustParseValidateURL(t, "https://landing.example/missing"),
			StatusCode: 404,
		},
	}}
	checks, err := checkValidateURLs(context.Background(), checker, []validateURLTarget{{
		RawURL:       rawURL,
		Locale:       "en-US",
		Field:        "supportUrl",
		ResourceType: "appStoreVersionLocalization",
		ResourceID:   "version-loc-1",
	}})
	if err != nil {
		t.Fatalf("checkValidateURLs() error: %v", err)
	}
	wantIDs := []string{"legal.url.http_status", "legal.url.redirected_host"}
	gotIDs := make([]string, 0, len(checks))
	for _, check := range checks {
		gotIDs = append(gotIDs, check.ID)
	}
	if !equalValidateStrings(gotIDs, wantIDs) {
		t.Fatalf("check IDs = %v, want %v", gotIDs, wantIDs)
	}
	for _, check := range checks {
		if strings.Contains(check.Message, "example") || strings.Contains(check.Remediation, "example") {
			t.Fatalf("check leaked destination details: %+v", check)
		}
	}
}

func TestValidateURLChecksRetainsCrossHostRedirectWarningWhenFinalHostReturns(t *testing.T) {
	const rawURL = "https://support.example/help"
	checker := &fakeValidateURLChecker{results: map[string]metadataurl.Result{
		rawURL: {
			FinalURL:       mustParseValidateURL(t, "https://support.example/final"),
			StatusCode:     200,
			RedirectedHost: true,
		},
	}}
	checks, err := checkValidateURLs(context.Background(), checker, []validateURLTarget{{
		RawURL:       rawURL,
		Locale:       "en-US",
		Field:        "supportUrl",
		ResourceType: "appStoreVersionLocalization",
		ResourceID:   "version-loc-1",
	}})
	if err != nil {
		t.Fatalf("checkValidateURLs() error: %v", err)
	}
	if len(checks) != 1 || checks[0].ID != "legal.url.redirected_host" {
		t.Fatalf("checks = %+v, want one redirected_host warning", checks)
	}
}

func TestValidateURLChecksDoesNotWarnForSameHostResult(t *testing.T) {
	const rawURL = "https://support.example/help"
	checker := &fakeValidateURLChecker{results: map[string]metadataurl.Result{
		rawURL: {
			FinalURL:   mustParseValidateURL(t, "https://support.example/final"),
			StatusCode: 200,
		},
	}}
	checks, err := checkValidateURLs(context.Background(), checker, []validateURLTarget{{
		RawURL:       rawURL,
		Locale:       "en-US",
		Field:        "supportUrl",
		ResourceType: "appStoreVersionLocalization",
		ResourceID:   "version-loc-1",
	}})
	if err != nil {
		t.Fatalf("checkValidateURLs() error: %v", err)
	}
	if len(checks) != 0 {
		t.Fatalf("checks = %+v, want no warnings", checks)
	}
}

func TestValidateURLChecksMapsUnsafeAndTimeoutErrorsWithoutDetails(t *testing.T) {
	checker := &fakeValidateURLChecker{
		errors: map[string]error{
			"https://private.example/policy": metadataurl.ErrUnsafeTarget,
			"https://slow.example/policy":    errors.Join(context.DeadlineExceeded, errors.New("response body contained secret")),
		},
	}
	targets := []validateURLTarget{
		{RawURL: "https://private.example/policy", Locale: "en-US", Field: "privacyPolicyUrl", ResourceType: "appInfoLocalization", ResourceID: "app-info-loc-1"},
		{RawURL: "https://slow.example/policy", Locale: "en-US", Field: "privacyChoicesUrl", ResourceType: "appInfoLocalization", ResourceID: "app-info-loc-1"},
	}

	checks, err := checkValidateURLs(context.Background(), checker, targets)
	if err != nil {
		t.Fatalf("checkValidateURLs() error: %v", err)
	}
	gotIDs := make(map[string]bool, len(checks))
	for _, check := range checks {
		gotIDs[check.ID] = true
	}
	if len(checks) != 2 || !gotIDs["legal.url.unsafe_target"] || !gotIDs["legal.url.request_failed"] {
		t.Fatalf("checks = %+v, want unsafe and request_failed findings", checks)
	}
	for _, check := range checks {
		if strings.Contains(check.Message, "private.example") || strings.Contains(check.Message, "slow.example") || strings.Contains(check.Message, "secret") || strings.Contains(check.Remediation, "example") {
			t.Fatalf("check leaked URL or transport details: %+v", check)
		}
	}
}

func TestValidateURLTargetsIncludesAllSupportedFields(t *testing.T) {
	targets := validateURLTargets(
		[]validation.VersionLocalization{{
			ID:           "version-loc-1",
			Locale:       "en-US",
			SupportURL:   "https://example.com/support",
			MarketingURL: "https://example.com/marketing",
		}},
		[]validation.AppInfoLocalization{{
			ID:                "app-info-loc-1",
			Locale:            "en-US",
			PrivacyPolicyURL:  "https://example.com/privacy",
			PrivacyChoicesURL: "https://example.com/choices",
		}},
	)
	if len(targets) != 4 {
		t.Fatalf("targets = %+v, want all four URL fields", targets)
	}
	for _, target := range targets {
		if target.RawURL == "" || target.ResourceID == "" || target.Locale == "" {
			t.Fatalf("target = %+v, want complete target context", target)
		}
	}

	targets = validateURLTargets(
		[]validation.VersionLocalization{{
			Locale:       "en-US",
			SupportURL:   "not a URL",
			MarketingURL: "ftp://example.com/marketing",
		}},
		nil,
	)
	if len(targets) != 0 {
		t.Fatalf("targets = %+v, want syntax-invalid URLs skipped", targets)
	}
}

func TestRunValidatePassesCheckURLsToReadinessAndPrintsReport(t *testing.T) {
	previous := buildReadinessReportFn
	var received ReadinessOptions
	buildReadinessReportFn = func(_ context.Context, _ *shared.Deps, opts ReadinessOptions) (validation.Report, error) {
		received = opts
		return validation.Report{AppID: opts.AppID, VersionID: opts.VersionID, Checks: []validation.CheckResult{}}, nil
	}
	t.Cleanup(func() { buildReadinessReportFn = previous })

	d, stdoutBuf, _ := validateTestDeps()
	runErr := runValidate(context.Background(), d, validateOptions{
		AppID:     "app-1",
		VersionID: "version-1",
		CheckURLs: true,
		Output:    "json",
	})
	stdout := stdoutBuf.String()
	if runErr != nil {
		t.Fatalf("runValidate() error: %v", runErr)
	}
	if !received.CheckURLs {
		t.Fatalf("readiness options = %+v, want CheckURLs=true", received)
	}
	var report validation.Report
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("json.Unmarshal() error: %v; stdout=%q", err, stdout)
	}
}

func TestRunValidatePrintsReportBeforeValidationError(t *testing.T) {
	previous := buildReadinessReportFn
	buildReadinessReportFn = func(_ context.Context, _ *shared.Deps, _ ReadinessOptions) (validation.Report, error) {
		return validation.Report{
			AppID:     "app-1",
			VersionID: "version-1",
			Summary:   validation.Summary{Warnings: 1, Blocking: 1},
			Checks: []validation.CheckResult{{
				ID:          "metadata.minimum.name",
				Severity:    validation.SeverityWarning,
				Message:     "app name is shorter than 2 characters",
				Remediation: "Use an app name with at least 2 characters",
			}},
		}, nil
	}
	t.Cleanup(func() { buildReadinessReportFn = previous })

	d, stdoutBuf, _ := validateTestDeps()
	err := runValidate(context.Background(), d, validateOptions{
		AppID:     "app-1",
		VersionID: "version-1",
		Output:    "json",
	})
	if err == nil || !shared.IsValidationError(err) {
		t.Fatalf("runValidate() error = %v, want a reported validation error", err)
	}
	stdout := stdoutBuf.String()
	if !strings.Contains(stdout, "metadata.minimum.name") {
		t.Fatalf("report was not printed before validation error: %q", stdout)
	}
}

func TestRunValidateDefaultsToOfflineURLChecks(t *testing.T) {
	previous := buildReadinessReportFn
	var received ReadinessOptions
	buildReadinessReportFn = func(_ context.Context, _ *shared.Deps, opts ReadinessOptions) (validation.Report, error) {
		received = opts
		return validation.Report{}, nil
	}
	t.Cleanup(func() { buildReadinessReportFn = previous })

	d, stdoutBuf, _ := validateTestDeps()
	if err := runValidate(context.Background(), d, validateOptions{AppID: "app-1", VersionID: "version-1", Output: "json"}); err != nil {
		t.Fatalf("runValidate() error: %v", err)
	}
	stdout := stdoutBuf.String()
	if received.CheckURLs {
		t.Fatal("default validation unexpectedly enabled URL checks")
	}
	if strings.TrimSpace(stdout) == "" {
		t.Fatal("expected JSON report output")
	}
}

func mustParseValidateURL(t *testing.T, rawURL string) *url.URL {
	t.Helper()
	parsed, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("url.Parse(%q): %v", rawURL, err)
	}
	return parsed
}

func equalValidateStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}
