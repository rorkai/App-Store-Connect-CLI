package web

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

// writeTestCachedWebSession writes a file-backed cache entry the way the
// session store lays it out, without a last-session pointer.
func writeTestCachedWebSession(t *testing.T, dir, email string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	sum := sha256.Sum256([]byte(strings.ToLower(strings.TrimSpace(email))))
	raw, err := json.Marshal(map[string]any{
		"version":    1,
		"user_email": email,
		"cookies": map[string]any{
			"https://appstoreconnect.apple.com": []map[string]any{{"name": "myacinfo", "value": "cookie-" + email}},
		},
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "session-"+hex.EncodeToString(sum[:])+".json"), raw, 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
}

func stubDefaultAppleIDResolverInputs(t *testing.T, d *shared.Deps, cacheDir string) *bytes.Buffer {
	t.Helper()
	t.Setenv("ASC_WEB_SESSION_CACHE_BACKEND", "file")
	t.Setenv("ASC_WEB_SESSION_CACHE_DIR", cacheDir)
	t.Setenv(webPasswordEnv, "")

	origTryResume := tryResumeSessionFn
	origTryResumeFromSource := tryResumeSessionFromSourceFn
	origTryResumeLast := tryResumeLastFn
	origLoadCachedFromSource := loadCachedSessionFromSourceFn
	t.Cleanup(func() {
		tryResumeSessionFn = origTryResume
		tryResumeSessionFromSourceFn = origTryResumeFromSource
		tryResumeLastFn = origTryResumeLast
		loadCachedSessionFromSourceFn = origLoadCachedFromSource
	})

	stderr := &bytes.Buffer{}
	d.IO.ErrOut = stderr
	tryResumeLastFn = func(ctx context.Context) (*webcore.AuthSession, bool, error) {
		return nil, false, nil
	}
	tryResumeSessionFn = func(ctx context.Context, username string) (*webcore.AuthSession, bool, error) {
		t.Fatalf("unexpected user-scoped cache lookup for %q", username)
		return nil, false, nil
	}
	tryResumeSessionFromSourceFn = func(ctx context.Context, username string, _ webcore.CachedSessionSource) (*webcore.AuthSession, bool, error) {
		return tryResumeSessionFn(ctx, username)
	}
	loadCachedSessionFromSourceFn = func(username string, _ webcore.CachedSessionSource) (*webcore.AuthSession, bool, error) {
		return loadCachedSessionFn(username)
	}
	return stderr
}

func TestResolveSessionDefaultsToSoleCachedAppleID(t *testing.T) {
	d, _, _ := newWebTestDeps()
	dir := t.TempDir()
	stderr := stubDefaultAppleIDResolverInputs(t, d, dir)
	writeTestCachedWebSession(t, dir, "only@example.com")

	var lookups []string
	tryResumeSessionFn = func(ctx context.Context, username string) (*webcore.AuthSession, bool, error) {
		lookups = append(lookups, username)
		return &webcore.AuthSession{UserEmail: username}, true, nil
	}

	session, source, err := resolveSession(context.Background(), d, "", "", "")
	if err != nil {
		t.Fatalf("resolveSession() error = %v", err)
	}
	if source != "cache" {
		t.Fatalf("source = %q, want cache", source)
	}
	if session == nil || session.UserEmail != "only@example.com" {
		t.Fatalf("session = %+v, want only@example.com", session)
	}
	if want := []string{"only@example.com"}; strings.Join(lookups, ",") != strings.Join(want, ",") {
		t.Fatalf("lookups = %v, want %v", lookups, want)
	}
	if got, want := stderr.String(), "Using cached web session for only@example.com; pass --apple-id to override\n"; got != want {
		t.Fatalf("stderr = %q, want %q", got, want)
	}
}

func TestResolveSessionSanitizesSoleCachedAppleIDNotice(t *testing.T) {
	d, _, _ := newWebTestDeps()
	dir := t.TempDir()
	stderr := stubDefaultAppleIDResolverInputs(t, d, dir)
	writeTestCachedWebSession(t, dir, "attacker@example.com\nINJECTED\x1b[31m\u202e")

	var lookup string
	tryResumeSessionFn = func(ctx context.Context, username string) (*webcore.AuthSession, bool, error) {
		lookup = username
		return &webcore.AuthSession{UserEmail: username}, true, nil
	}

	if _, _, err := resolveSession(context.Background(), d, "", "", ""); err != nil {
		t.Fatalf("resolveSession() error = %v", err)
	}
	if got, want := lookup, "attacker@example.com\nINJECTED\x1b[31m\u202e"; got != want {
		t.Fatalf("session lookup = %q, want unsanitized identity %q", got, want)
	}
	if got, want := stderr.String(), "Using cached web session for attacker@example.com INJECTED[31m; pass --apple-id to override\n"; got != want {
		t.Fatalf("stderr = %q, want sanitized notice %q", got, want)
	}
}

func TestResolveSessionSoleCachedAppleIDCanAutoReauthenticate(t *testing.T) {
	d, _, _ := newWebTestDeps()
	dir := t.TempDir()
	stderr := stubDefaultAppleIDResolverInputs(t, d, dir)
	writeTestCachedWebSession(t, dir, "only@example.com")
	t.Setenv(webPasswordEnv, "env-secret")

	originalLoadCached := loadCachedSessionFn
	originalLoginWithClient := webLoginWithClientFn
	originalPersist := persistWebSessionFn
	t.Cleanup(func() {
		loadCachedSessionFn = originalLoadCached
		webLoginWithClientFn = originalLoginWithClient
		persistWebSessionFn = originalPersist
	})

	cachedClient := &http.Client{}
	expected := &webcore.AuthSession{Client: cachedClient, UserEmail: "only@example.com"}
	tryResumeSessionFn = func(ctx context.Context, username string) (*webcore.AuthSession, bool, error) {
		if username != "only@example.com" {
			t.Fatalf("username = %q, want only@example.com", username)
		}
		return nil, false, webcore.ErrCachedSessionExpired
	}
	loadCachedSessionFn = func(username string) (*webcore.AuthSession, bool, error) {
		if username != "only@example.com" {
			t.Fatalf("loaded username = %q, want only@example.com", username)
		}
		return &webcore.AuthSession{Client: cachedClient, UserEmail: username}, true, nil
	}
	webLoginWithClientFn = func(ctx context.Context, client *http.Client, credentials webcore.LoginCredentials) (*webcore.AuthSession, error) {
		if client != cachedClient {
			t.Fatal("expected cached client to be reused")
		}
		if credentials.Username != "only@example.com" || credentials.Password != "env-secret" {
			t.Fatalf("credentials = %+v, want only@example.com and environment password", credentials)
		}
		return expected, nil
	}
	persisted := false
	persistWebSessionFn = func(session *webcore.AuthSession) error {
		persisted = session == expected
		return nil
	}

	session, source, err := resolveSession(context.Background(), d, "", "", "")
	if err != nil {
		t.Fatalf("resolveSession() error = %v", err)
	}
	if session != expected || source != "auto-reauth" {
		t.Fatalf("resolveSession() = (%+v, %q), want selected account auto-reauth", session, source)
	}
	if !persisted {
		t.Fatal("expected refreshed session to be persisted")
	}
	if got, want := stderr.String(), "Using cached web session for only@example.com; pass --apple-id to override\n"; got != want {
		t.Fatalf("stderr = %q, want %q", got, want)
	}
}

func TestResolveSessionWithoutCachedSessionsReportsMissingWebSession(t *testing.T) {
	d, _, _ := newWebTestDeps()
	dir := t.TempDir()
	stderr := stubDefaultAppleIDResolverInputs(t, d, dir)

	_, _, err := resolveSession(context.Background(), d, "", "", "")
	if errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected the missing-session error without the usage page, got %v", err)
	}
	if !errors.Is(err, shared.ErrMissingWebSession) || !errors.Is(err, errNoCachedWebSession) {
		t.Fatalf("expected ErrMissingWebSession caused by errNoCachedWebSession, got %v", err)
	}
	missing, ok := errors.AsType[*shared.MissingWebSessionError](err)
	if !ok {
		t.Fatalf("expected *shared.MissingWebSessionError, got %T", err)
	}
	if got, want := err.Error(), "no Apple web session is cached"; got != want {
		t.Fatalf("error = %q, want %q", got, want)
	}
	if !strings.Contains(missing.Hint, "'asc web auth login --apple-id EMAIL' in a terminal") ||
		!strings.Contains(missing.Hint, "'asc web auth import --file FILE'") {
		t.Fatalf("hint = %q, want the terminal sign-in and session import next steps", missing.Hint)
	}
	diagnostic, ok := shared.DiagnosticFromError(err)
	if !ok || diagnostic.Code != shared.DiagnosticRequiredInputMissing || diagnostic.Parameter != "--apple-id" {
		t.Fatalf("diagnostic = %+v (found=%v), want the required_input_missing --apple-id diagnostic the usage error reported", diagnostic, ok)
	}
	if got := shared.ClassifyUsageError(err); got != shared.UsageErrorMissingRequired {
		t.Fatalf("usage classification = %q, want %q", got, shared.UsageErrorMissingRequired)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestResolveSessionForUncachedAccountWithoutPasswordReportsMissingWebSession(t *testing.T) {
	d, _, _ := newWebTestDeps()
	dir := t.TempDir()
	_ = stubDefaultAppleIDResolverInputs(t, d, dir)
	t.Setenv("ASC_WEB_DONT_STORE_PASSWORD", "1")
	tryResumeSessionFn = func(context.Context, string) (*webcore.AuthSession, bool, error) {
		return nil, false, nil
	}
	origPromptPassword := promptPasswordFn
	t.Cleanup(func() { promptPasswordFn = origPromptPassword })
	promptPasswordFn = func(context.Context) (string, error) { return "", nil }

	_, _, err := resolveSession(context.Background(), d, "user@example.com", "", "")
	if errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected the missing-session error without the usage page, got %v", err)
	}
	if !errors.Is(err, shared.ErrMissingWebSession) {
		t.Fatalf("expected ErrMissingWebSession, got %v", err)
	}
	if got, want := err.Error(), "no usable Apple web session for user@example.com"; got != want {
		t.Fatalf("error = %q, want %q", got, want)
	}
}

// TestResolveSessionForSignInKeepsUsageErrors pins the one command whose job is
// to create the session: `asc web auth login` still treats a missing account or
// password as a usage error.
func TestResolveSessionForSignInKeepsUsageErrors(t *testing.T) {
	d, _, _ := newWebTestDeps()
	dir := t.TempDir()
	_ = stubDefaultAppleIDResolverInputs(t, d, dir)
	t.Setenv("ASC_WEB_DONT_STORE_PASSWORD", "1")
	tryResumeSessionFn = func(context.Context, string) (*webcore.AuthSession, bool, error) {
		return nil, false, nil
	}
	origPromptPassword := promptPasswordFn
	t.Cleanup(func() { promptPasswordFn = origPromptPassword })
	promptPasswordFn = func(context.Context) (string, error) { return "", nil }

	signInCtx := contextForWebSignIn(context.Background())
	for _, tc := range []struct {
		name    string
		appleID string
		want    string
	}{
		{name: "no account", want: "--apple-id is required when no cached web session is available; run 'asc web auth login --apple-id EMAIL'"},
		{name: "no password", appleID: "user@example.com", want: "password is required: run in a terminal for an interactive prompt or set ASC_WEB_PASSWORD"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := resolveSession(signInCtx, d, tc.appleID, "", "")
			if !errors.Is(err, flag.ErrHelp) || errors.Is(err, shared.ErrMissingWebSession) {
				t.Fatalf("expected the sign-in usage error, got %v", err)
			}
			if got := err.Error(); got != tc.want {
				t.Fatalf("error = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestResolveSessionNamesPublicAPIAlternative(t *testing.T) {
	d, _, _ := newWebTestDeps()
	dir := t.TempDir()
	_ = stubDefaultAppleIDResolverInputs(t, d, dir)

	ctx := contextWithPublicAPIAlternative(context.Background(), "Without a web session, use the public API.")
	_, _, err := resolveSession(ctx, d, "", "", "")
	missing, ok := errors.AsType[*shared.MissingWebSessionError](err)
	if !ok {
		t.Fatalf("expected *shared.MissingWebSessionError, got %v", err)
	}
	if !strings.HasSuffix(missing.Hint, " Without a web session, use the public API.") {
		t.Fatalf("hint = %q, want the public API alternative appended", missing.Hint)
	}
}

func TestResolveSessionWithMultipleCachedSessionsListsAppleIDs(t *testing.T) {
	d, _, _ := newWebTestDeps()
	dir := t.TempDir()
	stderr := stubDefaultAppleIDResolverInputs(t, d, dir)
	writeTestCachedWebSession(t, dir, "zed@example.com")
	writeTestCachedWebSession(t, dir, "amy@example.com")

	_, _, err := resolveSession(context.Background(), d, "", "", "")
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected usage error, got %v", err)
	}
	if got, want := err.Error(), "--apple-id is required: multiple cached web sessions are available (amy@example.com, zed@example.com); pass --apple-id to choose one"; got != want {
		t.Fatalf("error = %q, want %q", got, want)
	}
	diagnostic, ok := shared.DiagnosticFromError(err)
	if !ok || diagnostic.Code != shared.DiagnosticRequiredInputMissing || diagnostic.Parameter != "--apple-id" {
		t.Fatalf("diagnostic = %+v (found=%v), want required_input_missing --apple-id", diagnostic, ok)
	}
	// The usage error is the only line: no cached-session notice.
	if got, want := stderr.String(), "Error: --apple-id is required: multiple cached web sessions are available (amy@example.com, zed@example.com); pass --apple-id to choose one\n"; got != want {
		t.Fatalf("stderr = %q, want %q", got, want)
	}
}

func TestResolveSessionExplicitAppleIDSkipsCachedDefault(t *testing.T) {
	d, _, _ := newWebTestDeps()
	dir := t.TempDir()
	stderr := stubDefaultAppleIDResolverInputs(t, d, dir)
	writeTestCachedWebSession(t, dir, "zed@example.com")
	writeTestCachedWebSession(t, dir, "amy@example.com")

	tryResumeLastFn = func(ctx context.Context) (*webcore.AuthSession, bool, error) {
		t.Fatal("did not expect last-session lookup when --apple-id is set")
		return nil, false, nil
	}
	var lookups []string
	tryResumeSessionFn = func(ctx context.Context, username string) (*webcore.AuthSession, bool, error) {
		lookups = append(lookups, username)
		return &webcore.AuthSession{UserEmail: username}, true, nil
	}

	session, _, err := resolveSession(context.Background(), d, " flag@example.com ", "", "")
	if err != nil {
		t.Fatalf("resolveSession() error = %v", err)
	}
	if session.UserEmail != "flag@example.com" {
		t.Fatalf("session.UserEmail = %q, want flag@example.com", session.UserEmail)
	}
	if strings.Join(lookups, ",") != "flag@example.com" {
		t.Fatalf("lookups = %v, want [flag@example.com]", lookups)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestResolveSessionPrefersLastCachedSessionOverSoleDefault(t *testing.T) {
	d, _, _ := newWebTestDeps()
	dir := t.TempDir()
	stderr := stubDefaultAppleIDResolverInputs(t, d, dir)
	writeTestCachedWebSession(t, dir, "only@example.com")

	tryResumeLastFn = func(ctx context.Context) (*webcore.AuthSession, bool, error) {
		return &webcore.AuthSession{UserEmail: "last@example.com"}, true, nil
	}

	session, _, err := resolveSession(context.Background(), d, "", "", "")
	if err != nil {
		t.Fatalf("resolveSession() error = %v", err)
	}
	if session.UserEmail != "last@example.com" {
		t.Fatalf("session.UserEmail = %q, want last@example.com", session.UserEmail)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestResolveSessionDefaultLookupFailureFallsBackToMissingWebSession(t *testing.T) {
	d, _, _ := newWebTestDeps()
	dir := t.TempDir()
	stderr := stubDefaultAppleIDResolverInputs(t, d, dir)

	origDefault := defaultCachedAppleIDFn
	t.Cleanup(func() { defaultCachedAppleIDFn = origDefault })
	defaultCachedAppleIDFn = func() (string, webcore.CachedSessionSource, error) {
		return "", webcore.CachedSessionSourceUnknown, errors.New("boom")
	}

	_, _, err := resolveSession(context.Background(), d, "", "", "")
	missing, ok := errors.AsType[*shared.MissingWebSessionError](err)
	if !ok {
		t.Fatalf("expected the missing-session error, got %v", err)
	}
	if !strings.Contains(missing.Hint, "'asc web auth login --apple-id EMAIL'") {
		t.Fatalf("hint = %q, want login hint", missing.Hint)
	}
	if !strings.Contains(stderr.String(), "Warning: listing cached web sessions failed: boom") {
		t.Fatalf("stderr = %q, want cache listing warning", stderr.String())
	}
}

func TestResolveWebSessionEmptyCachePromptsWithoutPrintingUsageError(t *testing.T) {
	d, _, _ := newWebTestDeps()
	dir := t.TempDir()
	stubDefaultAppleIDResolverInputs(t, d, dir)

	var prompted bool
	tryResumeSessionFn = func(ctx context.Context, username string) (*webcore.AuthSession, bool, error) {
		if username != "prompted@example.com" {
			t.Fatalf("username = %q, want prompted@example.com", username)
		}
		return &webcore.AuthSession{UserEmail: username}, true, nil
	}

	var (
		session *webcore.AuthSession
		source  string
		err     error
	)
	_, stderr := captureOutput(t, func() {
		session, source, err = resolveWebSessionWith(context.Background(), d, "", "", "", webSessionResolveOptions{
			promptAppleID: func(appleID *string) error {
				prompted = true
				*appleID = "prompted@example.com"
				return nil
			},
			resolvePassword: resolveSessionPassword,
		})
	})
	if err != nil {
		t.Fatalf("resolveWebSession() error = %v", err)
	}
	if !prompted {
		t.Fatal("expected empty cache to prompt for an Apple ID")
	}
	if session == nil || session.UserEmail != "prompted@example.com" || source != "cache" {
		t.Fatalf("resolveWebSession() = (%+v, %q), want prompted cached session", session, source)
	}
	if strings.Contains(stderr, "Error:") {
		t.Fatalf("stderr = %q, want no usage error before successful prompt", stderr)
	}
}

func TestResolveWebSessionEmptyCacheNonInteractiveReportsMissingWebSessionWithoutPrinting(t *testing.T) {
	d, _, _ := newWebTestDeps()
	dir := t.TempDir()
	stubDefaultAppleIDResolverInputs(t, d, dir)
	originalCanPrompt := appCreateCanPromptInteractivelyFn
	appCreateCanPromptInteractivelyFn = func() bool { return false }
	t.Cleanup(func() { appCreateCanPromptInteractivelyFn = originalCanPrompt })

	var err error
	_, stderr := captureOutput(t, func() {
		_, _, err = resolveWebSessionWith(context.Background(), d, "", "", "", webSessionResolveOptions{
			promptAppleID:   promptAppsCreateSessionAppleID,
			resolvePassword: resolveSessionPassword,
		})
	})
	if !errors.Is(err, shared.ErrMissingWebSession) || errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected the missing-session error without the usage page, got %v", err)
	}
	// The root renderer prints the single diagnostic; the resolver must not.
	if strings.Contains(stderr, "Error:") {
		t.Fatalf("stderr = %q, want no diagnostic before the root renderer", stderr)
	}
}

func TestResolveWebSessionForCommandSessionFromEnvSkipsCachedDefault(t *testing.T) {
	d, _, _ := newWebTestDeps()
	dir := t.TempDir()
	stubDefaultAppleIDResolverInputs(t, d, dir)
	writeTestCachedWebSession(t, dir, "only@example.com")
	t.Setenv(webSessionBundleEnvName, "")

	origDefault := defaultCachedAppleIDFn
	t.Cleanup(func() { defaultCachedAppleIDFn = origDefault })
	defaultCachedAppleIDFn = func() (string, webcore.CachedSessionSource, error) {
		t.Fatal("did not expect cached-session default resolution with --session-from-env")
		return "", webcore.CachedSessionSourceUnknown, nil
	}

	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	flags := bindWebSessionFlagsWithSessionFromEnv(fs)
	if err := fs.Parse([]string{"--session-from-env"}); err != nil {
		t.Fatal(err)
	}
	_, _, cancel, err := resolveWebSessionForCommand(context.Background(), flags)
	defer cancel()
	if err == nil || !strings.Contains(err.Error(), webSessionBundleEnvName+" is unset or empty") {
		t.Fatalf("error = %v, want unset ASC_WEB_SESSION usage error", err)
	}
}

// TestWrapWebAuthCapabilitiesSessionErrorPreservesAppleIDUsageErrors pins the
// interaction with the capability-specific session diagnostics: both --apple-id
// usage errors are already printed with their own guidance, so the capabilities
// wrapper must pass them through instead of restating them as a generic
// session failure.
func TestWrapWebAuthCapabilitiesSessionErrorPreservesAppleIDUsageErrors(t *testing.T) {
	for _, tc := range []struct {
		name string
		make func(*shared.Deps) error
	}{
		{name: "no cached session", make: missingAppleIDUsageError},
		{
			name: "ambiguous cache",
			make: func(d *shared.Deps) error {
				return ambiguousAppleIDUsageError(d, []string{"amy@example.com", "zed@example.com"})
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var err, wrapped error
			d, _, _ := newWebTestDeps()
			err = tc.make(d)
			wrapped = wrapWebAuthCapabilitiesSessionError(err)
			if got, want := wrapped.Error(), err.Error(); got != want {
				t.Fatalf("wrapped error = %q, want the original guidance %q", got, want)
			}
			if !errors.Is(wrapped, flag.ErrHelp) {
				t.Fatalf("wrapped error lost its usage classification: %v", wrapped)
			}
			if got := shared.ClassifyUsageError(wrapped); got != shared.UsageErrorMissingRequired {
				t.Fatalf("usage classification = %q, want %q", got, shared.UsageErrorMissingRequired)
			}
		})
	}
}
