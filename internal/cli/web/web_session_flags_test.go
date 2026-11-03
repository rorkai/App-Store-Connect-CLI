package web

import (
	"context"
	"errors"
	"flag"
	"net/http"
	"strings"
	"testing"
	"time"

	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

func TestBindWebSessionFlagsOmitsRemovedTwoFactorCodeAlias(t *testing.T) {
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	flags := bindWebSessionFlags(fs)

	if fs.Lookup("two-factor-code") != nil {
		t.Fatal("removed --two-factor-code alias is still registered")
	}
	if flags.twoFactorCodeCommand == nil {
		t.Fatal("expected two-factor-code-command pointer to be populated")
	}
	if fs.Lookup("two-factor-code-command") == nil {
		t.Fatal("expected --two-factor-code-command to remain registered")
	}
	if fs.Lookup("provider-id") == nil {
		t.Fatal("expected --provider-id to be registered")
	}
	if fs.Lookup("public-provider-id") == nil {
		t.Fatal("expected --public-provider-id to be registered")
	}
}

func TestResolveWebSessionForCommandPassesTwoFactorCodeCommand(t *testing.T) {
	restoreResolve := SetResolveWebSession(func(ctx context.Context, appleID, password, twoFactorCode, twoFactorCodeCommand string) (*webcore.AuthSession, string, error) {
		if appleID != "user@example.com" {
			t.Fatalf("appleID = %q, want %q", appleID, "user@example.com")
		}
		if twoFactorCode != "" {
			t.Fatalf("twoFactorCode = %q, want empty", twoFactorCode)
		}
		if twoFactorCodeCommand != "osascript /tmp/get-apple-2fa-code.scpt" {
			t.Fatalf("twoFactorCodeCommand = %q, want osascript helper", twoFactorCodeCommand)
		}
		return &webcore.AuthSession{}, "test", nil
	})
	t.Cleanup(restoreResolve)

	flags := webSessionFlags{
		appleID:              ptrTo("user@example.com"),
		twoFactorCodeCommand: ptrTo("osascript /tmp/get-apple-2fa-code.scpt"),
	}

	session, _, cancel, err := resolveWebSessionForCommand(context.Background(), flags)
	defer cancel()
	if err != nil {
		t.Fatalf("resolveWebSessionForCommand() error = %v", err)
	}
	if session == nil {
		t.Fatal("expected session")
	}
}

func TestResolveWebSessionForCommandSelectsProvider(t *testing.T) {
	expected := &webcore.AuthSession{UserEmail: "user@example.com"}
	restoreResolve := SetResolveWebSession(func(ctx context.Context, appleID, password, twoFactorCode, twoFactorCodeCommand string) (*webcore.AuthSession, string, error) {
		return expected, "cache", nil
	})
	t.Cleanup(restoreResolve)

	origSelectProvider := selectWebProviderFn
	origPersist := persistWebSessionFn
	t.Cleanup(func() {
		selectWebProviderFn = origSelectProvider
		persistWebSessionFn = origPersist
	})

	selected := false
	selectWebProviderFn = func(ctx context.Context, session *webcore.AuthSession, selection webcore.ProviderSelection) error {
		selected = true
		if session != expected {
			t.Fatal("expected resolved session to be selected")
		}
		if selection.ProviderID != 123456 {
			t.Fatalf("ProviderID = %d, want 123456", selection.ProviderID)
		}
		if selection.PublicProviderID != "TEAM123" {
			t.Fatalf("PublicProviderID = %q, want TEAM123", selection.PublicProviderID)
		}
		session.ProviderID = selection.ProviderID
		session.PublicProviderID = selection.PublicProviderID
		return nil
	}
	persisted := false
	persistWebSessionFn = func(session *webcore.AuthSession) error {
		persisted = true
		if session != expected {
			t.Fatal("expected selected session to be persisted")
		}
		return nil
	}

	providerID := int64(123456)
	flags := webSessionFlags{
		appleID:              ptrTo("user@example.com"),
		twoFactorCodeCommand: ptrTo(""),
		providerID:           &providerID,
		publicProviderID:     ptrTo("TEAM123"),
	}

	session, _, cancel, err := resolveWebSessionForCommand(context.Background(), flags)
	defer cancel()
	if err != nil {
		t.Fatalf("resolveWebSessionForCommand() error = %v", err)
	}
	if session != expected {
		t.Fatal("expected selected session")
	}
	if !selected {
		t.Fatal("expected provider selection")
	}
	if !persisted {
		t.Fatal("expected selected provider session to be persisted")
	}
}

func TestResolveWebSessionForCommandDoesNotPersistBeforeProviderSelection(t *testing.T) {
	origTryResume := tryResumeSessionFn
	origTryResumeLast := tryResumeLastFn
	origLoadCachedSession := loadCachedSessionFn
	origLoadLastCachedSession := loadLastCachedSessionFn
	origWebLogin := webLoginFn
	origWebLoginWithClient := webLoginWithClientFn
	origSelectProvider := selectWebProviderFn
	origPersist := persistWebSessionFn
	t.Cleanup(func() {
		tryResumeSessionFn = origTryResume
		tryResumeLastFn = origTryResumeLast
		loadCachedSessionFn = origLoadCachedSession
		loadLastCachedSessionFn = origLoadLastCachedSession
		webLoginFn = origWebLogin
		webLoginWithClientFn = origWebLoginWithClient
		selectWebProviderFn = origSelectProvider
		persistWebSessionFn = origPersist
	})

	t.Setenv(webPasswordEnv, "secret")

	providerID := int64(123456)
	flags := webSessionFlags{
		appleID:              ptrTo("user@example.com"),
		twoFactorCodeCommand: ptrTo(""),
		providerID:           &providerID,
		publicProviderID:     ptrTo("TEAM123"),
	}

	selectErr := errors.New("provider unavailable")
	selectWebProviderFn = func(ctx context.Context, session *webcore.AuthSession, selection webcore.ProviderSelection) error {
		return selectErr
	}
	persistWebSessionFn = func(session *webcore.AuthSession) error {
		t.Fatal("did not expect session cache persistence before provider selection succeeds")
		return nil
	}

	t.Run("fresh login", func(t *testing.T) {
		tryResumeSessionFn = func(ctx context.Context, username string) (*webcore.AuthSession, bool, error) {
			return nil, false, nil
		}
		tryResumeLastFn = func(ctx context.Context) (*webcore.AuthSession, bool, error) {
			t.Fatal("did not expect last-session cache lookup when apple-id is provided")
			return nil, false, nil
		}
		webLoginFn = func(ctx context.Context, creds webcore.LoginCredentials) (*webcore.AuthSession, error) {
			if creds.Username != "user@example.com" {
				t.Fatalf("Username = %q, want user@example.com", creds.Username)
			}
			return &webcore.AuthSession{UserEmail: creds.Username}, nil
		}

		_, _, cancel, err := resolveWebSessionForCommand(context.Background(), flags)
		defer cancel()
		if !errors.Is(err, selectErr) {
			t.Fatalf("expected provider selection error, got %v", err)
		}
	})

	t.Run("auto reauth", func(t *testing.T) {
		cachedClient := &http.Client{}
		tryResumeSessionFn = func(ctx context.Context, username string) (*webcore.AuthSession, bool, error) {
			return nil, false, webcore.ErrCachedSessionExpired
		}
		tryResumeLastFn = func(ctx context.Context) (*webcore.AuthSession, bool, error) {
			t.Fatal("did not expect last-session cache lookup when apple-id is provided")
			return nil, false, nil
		}
		loadCachedSessionFn = func(username string) (*webcore.AuthSession, bool, error) {
			return &webcore.AuthSession{Client: cachedClient, UserEmail: username}, true, nil
		}
		loadLastCachedSessionFn = func() (*webcore.AuthSession, bool, error) {
			t.Fatal("did not expect last cached-session load when apple-id is provided")
			return nil, false, nil
		}
		webLoginWithClientFn = func(ctx context.Context, client *http.Client, creds webcore.LoginCredentials) (*webcore.AuthSession, error) {
			if client != cachedClient {
				t.Fatal("expected cached client to be reused")
			}
			return &webcore.AuthSession{Client: client, UserEmail: creds.Username}, nil
		}

		_, _, cancel, err := resolveWebSessionForCommand(context.Background(), flags)
		defer cancel()
		if !errors.Is(err, selectErr) {
			t.Fatalf("expected provider selection error, got %v", err)
		}
	})
}

func ptrTo(value string) *string {
	return &value
}

func TestWebCommandOperationSurvivesSlowInteractiveAuthentication(t *testing.T) {
	_ = stubWebProgressLabels(t)
	t.Setenv("ASC_TIMEOUT", "150ms")

	origResolveSession := resolveSessionFn
	origGetOverview := getAnalyticsOverviewFn
	t.Cleanup(func() {
		resolveSessionFn = origResolveSession
		getAnalyticsOverviewFn = origGetOverview
	})

	resolveSessionFn = func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		// Stands in for the password prompt, the 2FA prompt, or a configured
		// --two-factor-code-command, any of which can outlast the request budget.
		time.Sleep(300 * time.Millisecond)
		return &webcore.AuthSession{Client: &http.Client{}}, "fresh", nil
	}

	var operationCtxErr error
	getAnalyticsOverviewFn = func(ctx context.Context, client *webcore.Client, appID, startDate, endDate string) (*webcore.AnalyticsOverview, error) {
		operationCtxErr = ctx.Err()
		return &webcore.AnalyticsOverview{AppID: appID, StartDate: startDate, EndDate: endDate}, nil
	}

	cmd := WebAnalyticsOverviewCommand()
	if err := cmd.FlagSet.Parse([]string{
		"--apple-id", "user@example.com",
		"--app", "app-1",
		"--start", "2025-12-24",
		"--end", "2026-03-23",
		"--output", "json",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	captureOutput(t, func() {
		if err := cmd.Exec(context.Background(), nil); err != nil {
			t.Fatalf("exec error: %v", err)
		}
	})

	if operationCtxErr != nil {
		t.Fatalf("operation context expired during authentication: %v", operationCtxErr)
	}
}

func TestResolveWebSessionForCommandStartsRequestBudgetAfterAuthentication(t *testing.T) {
	t.Setenv("ASC_TIMEOUT", "150ms")

	restoreResolve := SetResolveWebSession(func(ctx context.Context, appleID, password, twoFactorCode, twoFactorCodeCommand string) (*webcore.AuthSession, string, error) {
		if _, ok := ctx.Deadline(); ok {
			t.Error("expected session resolution to run without the request deadline")
		}
		time.Sleep(300 * time.Millisecond)
		return &webcore.AuthSession{}, "fresh", nil
	})
	t.Cleanup(restoreResolve)

	flags := webSessionFlags{
		appleID:              ptrTo("user@example.com"),
		twoFactorCodeCommand: ptrTo(""),
	}

	session, requestCtx, cancel, err := resolveWebSessionForCommand(context.Background(), flags)
	defer cancel()
	if err != nil {
		t.Fatalf("resolveWebSessionForCommand() error = %v", err)
	}
	if session == nil {
		t.Fatal("expected session")
	}
	if err := requestCtx.Err(); err != nil {
		t.Fatalf("request context expired during authentication: %v", err)
	}
	deadline, ok := requestCtx.Deadline()
	if !ok {
		t.Fatal("expected the request context to be bounded")
	}
	if remaining := time.Until(deadline); remaining <= 0 {
		t.Fatalf("expected a positive request budget, got %v", remaining)
	}
}

func TestResolveWebSessionForCommandSelectsProviderOnRefreshedContext(t *testing.T) {
	t.Setenv("ASC_TIMEOUT", "150ms")

	expected := &webcore.AuthSession{UserEmail: "user@example.com"}
	restoreResolve := SetResolveWebSession(func(ctx context.Context, appleID, password, twoFactorCode, twoFactorCodeCommand string) (*webcore.AuthSession, string, error) {
		time.Sleep(300 * time.Millisecond)
		return expected, "fresh", nil
	})
	t.Cleanup(restoreResolve)

	origSelectProvider := selectWebProviderFn
	origPersist := persistWebSessionFn
	t.Cleanup(func() {
		selectWebProviderFn = origSelectProvider
		persistWebSessionFn = origPersist
	})

	var selectionCtxErr error
	selectWebProviderFn = func(ctx context.Context, session *webcore.AuthSession, selection webcore.ProviderSelection) error {
		selectionCtxErr = ctx.Err()
		return nil
	}
	persistWebSessionFn = func(session *webcore.AuthSession) error { return nil }

	providerID := int64(123456)
	flags := webSessionFlags{
		appleID:              ptrTo("user@example.com"),
		twoFactorCodeCommand: ptrTo(""),
		providerID:           &providerID,
	}

	_, _, cancel, err := resolveWebSessionForCommand(context.Background(), flags)
	defer cancel()
	if err != nil {
		t.Fatalf("resolveWebSessionForCommand() error = %v", err)
	}
	if selectionCtxErr != nil {
		t.Fatalf("provider selection ran on an expired context: %v", selectionCtxErr)
	}
}

func TestResolveWebSessionForCommandRequestContextFollowsParentCancellation(t *testing.T) {
	restoreResolve := SetResolveWebSession(func(ctx context.Context, appleID, password, twoFactorCode, twoFactorCodeCommand string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{}, "cache", nil
	})
	t.Cleanup(restoreResolve)

	parent, cancelParent := context.WithCancel(context.Background())
	flags := webSessionFlags{
		appleID:              ptrTo("user@example.com"),
		twoFactorCodeCommand: ptrTo(""),
	}

	_, requestCtx, cancel, err := resolveWebSessionForCommand(parent, flags)
	defer cancel()
	if err != nil {
		t.Fatalf("resolveWebSessionForCommand() error = %v", err)
	}

	cancelParent()
	select {
	case <-requestCtx.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("expected parent cancellation to cancel the request context")
	}
	if !errors.Is(requestCtx.Err(), context.Canceled) {
		t.Fatalf("request context error = %v, want context.Canceled", requestCtx.Err())
	}
}

func TestWithWebAuthHintCoversSessionInfoAuthStatus(t *testing.T) {
	for _, status := range []int{http.StatusUnauthorized, http.StatusForbidden} {
		session := &webcore.AuthSession{Client: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: status, Body: http.NoBody, Header: http.Header{}}, nil
		})}}
		err := webcore.SelectProvider(context.Background(), session, webcore.ProviderSelection{ProviderID: 123456})
		if err == nil {
			t.Fatalf("SelectProvider() error = nil, want session info status %d", status)
		}
		if got := withWebAuthHint(err, "web apps list").Error(); !strings.Contains(got, "run 'asc web auth login'") {
			t.Fatalf("withWebAuthHint() = %q, want the web auth login hint for status %d", got, status)
		}
	}
}

func TestWithWebAuthHintDoesNotRepeatDeveloperPortalSessionGuidance(t *testing.T) {
	for _, status := range []int{http.StatusUnauthorized, http.StatusForbidden} {
		session := &webcore.AuthSession{Client: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: status, Body: http.NoBody, Header: http.Header{}}, nil
		})}}
		_, err := webcore.NewClient(session).AssignDeveloperAppGroup(context.Background(), webcore.DeveloperAppGroupAssignRequest{BundleID: "bundle-1", GroupID: "GROUP1"})
		if err == nil {
			t.Fatalf("AssignDeveloperAppGroup() error = nil, want Developer Portal status %d", status)
		}
		got := withWebAuthHint(err, "web app-groups assign").Error()
		if want := "web app-groups assign failed: " + err.Error(); got != want {
			t.Fatalf("withWebAuthHint() = %q, want %q", got, want)
		}
	}
}
