package web

import (
	"context"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

func stubWebProgressLabels(t *testing.T) *[]string {
	t.Helper()

	origSpinner := webWithSpinnerDelayedFn
	labels := []string{}
	webWithSpinnerDelayedFn = func(label string, delay time.Duration, fn func() error) error {
		labels = append(labels, label)
		return fn()
	}
	t.Cleanup(func() {
		webWithSpinnerDelayedFn = origSpinner
	})

	return &labels
}

func TestResolveSessionUsesProgressLabels(t *testing.T) {
	labels := stubWebProgressLabels(t)

	origTryResumeSession := tryResumeSessionFn
	origTryResumeLast := tryResumeLastFn
	origPromptPassword := promptPasswordFn
	origLogin := webLoginFn
	t.Cleanup(func() {
		tryResumeSessionFn = origTryResumeSession
		tryResumeLastFn = origTryResumeLast
		promptPasswordFn = origPromptPassword
		webLoginFn = origLogin
	})

	t.Setenv("ASC_WEB_SESSION_CACHE", "0")

	tryResumeSessionFn = func(ctx context.Context, username string) (*webcore.AuthSession, bool, error) {
		if username != "user@example.com" {
			t.Fatalf("expected username user@example.com, got %q", username)
		}
		return nil, false, nil
	}
	tryResumeLastFn = func(context.Context) (*webcore.AuthSession, bool, error) {
		t.Fatal("did not expect last-session cache lookup")
		return nil, false, nil
	}
	promptPasswordFn = func(ctx context.Context) (string, error) {
		return "secret", nil
	}
	webLoginFn = func(ctx context.Context, creds webcore.LoginCredentials) (*webcore.AuthSession, error) {
		return &webcore.AuthSession{UserEmail: creds.Username}, nil
	}

	session, source, err := resolveSession(context.Background(), "user@example.com", "", "")
	if err != nil {
		t.Fatalf("resolveSession() error = %v", err)
	}
	if session == nil {
		t.Fatal("expected non-nil session")
	}
	if source != "fresh" {
		t.Fatalf("expected source fresh, got %q", source)
	}

	want := []string{
		"Checking cached web session",
		"Signing in to Apple web session",
	}
	if !reflect.DeepEqual(*labels, want) {
		t.Fatalf("expected labels %v, got %v", want, *labels)
	}
}

func TestLoginWithOptionalTwoFactorUsesProgressLabels(t *testing.T) {
	labels := stubWebProgressLabels(t)

	origLogin := webLoginFn
	origPrepare := prepareTwoFactorChallengeFn
	origSubmit := submitTwoFactorCodeFn
	t.Cleanup(func() {
		webLoginFn = origLogin
		prepareTwoFactorChallengeFn = origPrepare
		submitTwoFactorCodeFn = origSubmit
	})

	webLoginFn = func(ctx context.Context, creds webcore.LoginCredentials) (*webcore.AuthSession, error) {
		return &webcore.AuthSession{}, &webcore.TwoFactorRequiredError{}
	}
	prepareTwoFactorChallengeFn = func(ctx context.Context, session *webcore.AuthSession) (*webcore.TwoFactorChallenge, error) {
		return &webcore.TwoFactorChallenge{Method: "trusted-device"}, nil
	}
	submitTwoFactorCodeFn = func(ctx context.Context, session *webcore.AuthSession, code string) error {
		if code != "654321" {
			t.Fatalf("expected code 654321, got %q", code)
		}
		return nil
	}

	if _, err := loginWithOptionalTwoFactor(context.Background(), "user@example.com", "secret", "654321"); err != nil {
		t.Fatalf("loginWithOptionalTwoFactor() error = %v", err)
	}

	want := []string{
		"Signing in to Apple web session",
		"Verifying two-factor code",
	}
	if !reflect.DeepEqual(*labels, want) {
		t.Fatalf("expected labels %v, got %v", want, *labels)
	}
}

func TestWebAppsCreateUsesProgressLabels(t *testing.T) {
	labels := stubWebProgressLabels(t)

	origResolveAppCreateSession := resolveAppCreateSessionFn
	origNewWebClient := newWebClientFn
	origEnsureBundleID := ensureBundleIDFn
	origCreateWebApp := createWebAppFn
	t.Cleanup(func() {
		resolveAppCreateSessionFn = origResolveAppCreateSession
		newWebClientFn = origNewWebClient
		ensureBundleIDFn = origEnsureBundleID
		createWebAppFn = origCreateWebApp
	})

	resolveAppCreateSessionFn = func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{}, "cache", nil
	}
	newWebClientFn = func(session *webcore.AuthSession) *webcore.Client {
		return &webcore.Client{}
	}
	ensureBundleIDFn = func(ctx context.Context, bundleID, appName, platform string) (bool, error) {
		return false, nil
	}
	createWebAppFn = func(ctx context.Context, client *webcore.Client, attrs webcore.AppCreateAttributes) (*webcore.AppResponse, error) {
		resp := &webcore.AppResponse{}
		resp.Data.ID = "app-123"
		return resp, nil
	}

	cmd := WebAppsCreateCommand()
	if err := cmd.FlagSet.Parse([]string{
		"--name", "My App",
		"--bundle-id", "com.example.app",
		"--sku", "SKU123",
		"--apple-id", "user@example.com",
		"--output", "json",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	_, _ = captureOutput(t, func() {
		if err := cmd.Exec(context.Background(), nil); err != nil {
			t.Fatalf("exec error: %v", err)
		}
	})

	want := []string{
		"Checking or creating Bundle ID",
		"Creating app via Apple web API",
	}
	if !reflect.DeepEqual(*labels, want) {
		t.Fatalf("expected labels %v, got %v", want, *labels)
	}
}

func TestWebReviewListUsesProgressLabel(t *testing.T) {
	labels := stubWebProgressLabels(t)

	origResolveSession := resolveSessionFn
	t.Cleanup(func() {
		resolveSessionFn = origResolveSession
	})

	resolveSessionFn = func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{
			Client: &http.Client{
				Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
					if req.URL.Path != "/iris/v1/apps/app-1/reviewSubmissions" {
						t.Fatalf("unexpected path: %s", req.URL.Path)
					}
					body := `{
						"data": [{
							"id": "sub-1",
							"type": "reviewSubmissions",
							"attributes": {
								"state": "UNRESOLVED_ISSUES",
								"submittedDate": "2026-02-25T00:00:00Z",
								"platform": "IOS"
							}
						}]
					}`
					return &http.Response{
						StatusCode: http.StatusOK,
						Header:     http.Header{"Content-Type": []string{"application/json"}},
						Body:       io.NopCloser(strings.NewReader(body)),
						Request:    req,
					}, nil
				}),
			},
		}, "cache", nil
	}

	cmd := WebReviewListCommand()
	if err := cmd.FlagSet.Parse([]string{"--app", "app-1", "--output", "json"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	_, _ = captureOutput(t, func() {
		if err := cmd.Exec(context.Background(), nil); err != nil {
			t.Fatalf("exec error: %v", err)
		}
	})

	want := []string{"Loading review submissions"}
	if !reflect.DeepEqual(*labels, want) {
		t.Fatalf("expected labels %v, got %v", want, *labels)
	}
}

func TestWebXcodeCloudUsageSummaryUsesProgressLabel(t *testing.T) {
	labels := stubWebProgressLabels(t)

	origResolveSession := resolveSessionFn
	t.Cleanup(func() {
		resolveSessionFn = origResolveSession
	})

	resolveSessionFn = func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{
			PublicProviderID: "team-1",
			Client: &http.Client{
				Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
					if req.URL.Path != "/ci/api/teams/team-1/usage/summary" {
						t.Fatalf("unexpected path: %s", req.URL.Path)
					}
					body := `{"plan":{"name":"Plan","used":150,"available":1350,"total":1500}}`
					return &http.Response{
						StatusCode: http.StatusOK,
						Header:     http.Header{"Content-Type": []string{"application/json"}},
						Body:       io.NopCloser(strings.NewReader(body)),
						Request:    req,
					}, nil
				}),
			},
		}, "cache", nil
	}

	cmd := webXcodeCloudUsageSummaryCommand()
	if err := cmd.FlagSet.Parse([]string{"--apple-id", "user@example.com", "--output", "json"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	_, _ = captureOutput(t, func() {
		if err := cmd.Exec(context.Background(), nil); err != nil {
			t.Fatalf("exec error: %v", err)
		}
	})

	want := []string{"Loading Xcode Cloud usage summary"}
	if !reflect.DeepEqual(*labels, want) {
		t.Fatalf("expected labels %v, got %v", want, *labels)
	}
}
