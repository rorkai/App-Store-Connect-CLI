package web

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

// signinUnavailableFixture isolates resolveSession for the #2806 cases: Apple's
// IdMSA answers a sign-in stage with 503 while an expired cache is present.
type signinUnavailableFixture struct {
	cached         bool
	cachedAttempts int
	freshAttempts  int
	discarded      []string
	persisted      []*webcore.AuthSession
	warnings       bytes.Buffer
}

func newSigninUnavailableFixture(t *testing.T, d *shared.Deps, cached bool) *signinUnavailableFixture {
	t.Helper()
	origTryResume := tryResumeSessionFn
	origTryResumeLast := tryResumeLastFn
	origLoadCachedSession := loadCachedSessionFn
	origLoadLastCachedSession := loadLastCachedSessionFn
	origPromptPassword := promptPasswordFn
	origWebLogin := webLoginFn
	origWebLoginWithClient := webLoginWithClientFn
	origPersistWebSession := persistWebSessionFn
	origDeleteStaleWebSession := deleteStaleWebSessionFn
	t.Cleanup(func() {
		tryResumeSessionFn = origTryResume
		tryResumeLastFn = origTryResumeLast
		loadCachedSessionFn = origLoadCachedSession
		loadLastCachedSessionFn = origLoadLastCachedSession
		promptPasswordFn = origPromptPassword
		webLoginFn = origWebLogin
		webLoginWithClientFn = origWebLoginWithClient
		persistWebSessionFn = origPersistWebSession
		deleteStaleWebSessionFn = origDeleteStaleWebSession
	})
	t.Setenv(webPasswordEnv, "env-secret")

	fixture := &signinUnavailableFixture{cached: cached}
	d.IO.ErrOut = &fixture.warnings
	cachedSession := &webcore.AuthSession{Client: &http.Client{}, UserEmail: "user@example.com"}

	tryResumeSessionFn = func(context.Context, string) (*webcore.AuthSession, bool, error) {
		if cached {
			return nil, false, webcore.ErrCachedSessionExpired
		}
		return nil, false, nil
	}
	tryResumeLastFn = func(context.Context) (*webcore.AuthSession, bool, error) {
		t.Fatal("did not expect a last-session lookup when an Apple ID is provided")
		return nil, false, nil
	}
	loadCachedSessionFn = func(string) (*webcore.AuthSession, bool, error) {
		if !cached {
			t.Fatal("did not expect a cached-session load without an expired cache")
		}
		return cachedSession, true, nil
	}
	loadLastCachedSessionFn = func() (*webcore.AuthSession, bool, error) {
		t.Fatal("did not expect a last cached-session load when an Apple ID is provided")
		return nil, false, nil
	}
	promptPasswordFn = func(context.Context) (string, error) {
		t.Fatal("did not expect a password prompt when the password is in the environment")
		return "", nil
	}
	webLoginWithClientFn = func(_ context.Context, client *http.Client, creds webcore.LoginCredentials) (*webcore.AuthSession, error) {
		fixture.cachedAttempts++
		if client != cachedSession.Client || creds.Password != "env-secret" {
			t.Fatal("expected the cached client and saved password on the first attempt")
		}
		return nil, signinUnavailable("signin_init")
	}
	deleteStaleWebSessionFn = func(appleID string, loaded *webcore.AuthSession) (bool, error) {
		if loaded != cachedSession {
			t.Fatal("expected the discard to be scoped to the loaded cache entry")
		}
		fixture.discarded = append(fixture.discarded, appleID)
		return true, nil
	}
	persistWebSessionFn = func(session *webcore.AuthSession) error {
		fixture.persisted = append(fixture.persisted, session)
		return nil
	}
	return fixture
}

func signinUnavailable(stage string) error {
	return &webcore.SigninServiceError{
		Stage:         stage,
		Status:        http.StatusServiceUnavailable,
		ServiceErrors: []string{`-20209: "Your request could not be completed. Try again later."`},
		RetryAfter:    "120",
	}
}

func TestResolveSessionRetriesOnceWithCleanJarAfterCachedSignin503(t *testing.T) {
	d, _, _ := newWebTestDeps()
	fixture := newSigninUnavailableFixture(t, d, true)
	freshSession := &webcore.AuthSession{UserEmail: "user@example.com", ProviderID: 99}
	webLoginFn = func(_ context.Context, creds webcore.LoginCredentials) (*webcore.AuthSession, error) {
		fixture.freshAttempts++
		if creds.Username != "user@example.com" || creds.Password != "env-secret" {
			t.Fatal("expected the clean-jar retry to keep the Apple ID and password")
		}
		return freshSession, nil
	}

	session, source, err := resolveSession(context.Background(), d, "user@example.com", "", "")
	if err != nil {
		t.Fatalf("resolveSession: %v", err)
	}
	if session != freshSession || source != "fresh" {
		t.Fatalf("expected the fresh session, got %v from %q", session, source)
	}
	if fixture.cachedAttempts != 1 || fixture.freshAttempts != 1 {
		t.Fatalf("cached attempts = %d, clean-jar attempts = %d; want exactly one each", fixture.cachedAttempts, fixture.freshAttempts)
	}
	if len(fixture.discarded) != 1 || fixture.discarded[0] != "user@example.com" {
		t.Fatalf("expected the entry the clean jar proved stale to be discarded once, got %v", fixture.discarded)
	}
	if len(fixture.persisted) != 1 || fixture.persisted[0] != freshSession {
		t.Fatal("expected the fresh session to replace the stale cache entry")
	}
}

func TestResolveSessionExplainsRepeatedSignin503(t *testing.T) {
	d, _, _ := newWebTestDeps()
	fixture := newSigninUnavailableFixture(t, d, true)
	webLoginFn = func(context.Context, webcore.LoginCredentials) (*webcore.AuthSession, error) {
		fixture.freshAttempts++
		return nil, signinUnavailable("signin_init")
	}

	_, _, err := resolveSession(context.Background(), d, "user@example.com", "", "")
	if err == nil {
		t.Fatal("expected sign-in to fail")
	}
	if fixture.cachedAttempts != 1 || fixture.freshAttempts != 1 {
		t.Fatalf("cached attempts = %d, clean-jar attempts = %d; want exactly one each", fixture.cachedAttempts, fixture.freshAttempts)
	}
	if len(fixture.discarded) != 0 {
		t.Fatalf("a 5xx on both jars points at Apple, not the cache; expected the entry to be kept, got %v", fixture.discarded)
	}
	if len(fixture.persisted) != 0 {
		t.Fatal("did not expect a failed login to persist a session")
	}
	message := err.Error()
	for _, want := range []string{
		"signin_init",
		"503",
		"Your request could not be completed. Try again later.",
		"Retry-After: 120",
		"Wait several minutes",
		"asc web auth logout --apple-id \"user@example.com\"",
	} {
		if !strings.Contains(message, want) {
			t.Fatalf("expected %q in error %q", want, message)
		}
	}
}

func TestResolveSessionDoesNotRetrySignin503WithoutCachedCookies(t *testing.T) {
	d, _, _ := newWebTestDeps()
	fixture := newSigninUnavailableFixture(t, d, false)
	webLoginFn = func(context.Context, webcore.LoginCredentials) (*webcore.AuthSession, error) {
		fixture.freshAttempts++
		return nil, signinUnavailable("signin_complete")
	}

	_, _, err := resolveSession(context.Background(), d, "user@example.com", "", "")
	if err == nil {
		t.Fatal("expected sign-in to fail")
	}
	if fixture.cachedAttempts != 0 || fixture.freshAttempts != 1 {
		t.Fatalf("cached attempts = %d, fresh attempts = %d; want 0 and 1", fixture.cachedAttempts, fixture.freshAttempts)
	}
	if len(fixture.discarded) != 0 {
		t.Fatalf("did not expect a cache discard without a cached session, got %v", fixture.discarded)
	}
	if !strings.Contains(err.Error(), "signin_complete") || !strings.Contains(err.Error(), "asc web auth logout --apple-id") {
		t.Fatalf("expected an actionable signin_complete error, got %q", err.Error())
	}
}

func TestResolveSessionPromptedPasswordRetriesCleanJarAfterSignin503(t *testing.T) {
	d, _, _ := newWebTestDeps()
	fixture := newSigninUnavailableFixture(t, d, true)
	preserveWebPasswordHooks(t)
	t.Setenv(webPasswordEnv, "")
	t.Setenv(webDontStorePasswordEnv, "")
	t.Setenv("ASC_BYPASS_KEYCHAIN", "")
	loadStoredWebPasswordFn = func(string) (string, bool, error) { return "", false, nil }
	storeStoredWebPasswordFn = func(string, string) error { return nil }
	promptPasswordFn = func(context.Context) (string, error) { return "prompted-secret", nil }
	webLoginWithClientFn = func(context.Context, *http.Client, webcore.LoginCredentials) (*webcore.AuthSession, error) {
		fixture.cachedAttempts++
		return nil, signinUnavailable("signin_complete")
	}
	freshSession := &webcore.AuthSession{UserEmail: "user@example.com"}
	webLoginFn = func(_ context.Context, creds webcore.LoginCredentials) (*webcore.AuthSession, error) {
		fixture.freshAttempts++
		if creds.Password != "prompted-secret" {
			t.Fatal("expected the clean-jar retry to keep the prompted password")
		}
		return freshSession, nil
	}

	session, _, err := resolveSession(context.Background(), d, "user@example.com", "", "")
	if err != nil || session != freshSession {
		t.Fatalf("resolveSession = (%v, %v), want the fresh session", session, err)
	}
	if fixture.cachedAttempts != 1 || fixture.freshAttempts != 1 || len(fixture.discarded) != 1 {
		t.Fatalf("cached = %d, fresh = %d, discarded = %v; want one each", fixture.cachedAttempts, fixture.freshAttempts, fixture.discarded)
	}
}

func TestResolveSessionKeepsCacheWhenCleanRetryFailsLocally(t *testing.T) {
	d, _, _ := newWebTestDeps()
	fixture := newSigninUnavailableFixture(t, d, true)
	webLoginFn = func(context.Context, webcore.LoginCredentials) (*webcore.AuthSession, error) {
		fixture.freshAttempts++
		return nil, errors.New("dial tcp: lookup idmsa.apple.com: no such host")
	}

	_, _, err := resolveSession(context.Background(), d, "user@example.com", "", "")
	if err == nil {
		t.Fatal("expected sign-in to fail")
	}
	if fixture.cachedAttempts != 1 || fixture.freshAttempts != 1 {
		t.Fatalf("cached attempts = %d, clean-jar attempts = %d; want exactly one each", fixture.cachedAttempts, fixture.freshAttempts)
	}
	if len(fixture.discarded) != 0 {
		t.Fatalf("a clean retry that never reached Apple proves nothing about the cache, got discards %v", fixture.discarded)
	}
	if strings.Contains(err.Error(), "asc web auth logout") {
		t.Fatalf("did not expect the 5xx hint for a non-5xx failure: %q", err.Error())
	}
}
