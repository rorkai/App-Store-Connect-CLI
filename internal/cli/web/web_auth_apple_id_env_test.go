package web

import (
	"context"
	"flag"
	"strings"
	"testing"

	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

// recordSessionLookups makes the user-scoped cache lookup succeed for whatever
// Apple ID the resolver settled on and records every account it asked for, so a
// test can assert which source supplied the Apple ID without a live login.
func recordSessionLookups(lookups *[]string) {
	tryResumeSessionFn = func(ctx context.Context, username string) (*webcore.AuthSession, bool, error) {
		*lookups = append(*lookups, username)
		return &webcore.AuthSession{UserEmail: username}, true, nil
	}
}

func TestResolveSessionUsesEnvAppleIDForFreshLogin(t *testing.T) {
	d, _, _ := newWebTestDeps()
	dir := t.TempDir()
	stderr := stubDefaultAppleIDResolverInputs(t, d, dir)
	t.Setenv(webAppleIDEnv, "  env@example.com  ")
	t.Setenv(webPasswordEnv, "env-secret")

	var lookups []string
	tryResumeSessionFn = func(ctx context.Context, username string) (*webcore.AuthSession, bool, error) {
		lookups = append(lookups, username)
		return nil, false, nil
	}
	originalLogin := webLoginFn
	originalPersist := persistWebSessionFn
	t.Cleanup(func() {
		webLoginFn = originalLogin
		persistWebSessionFn = originalPersist
	})
	webLoginFn = func(ctx context.Context, creds webcore.LoginCredentials) (*webcore.AuthSession, error) {
		if creds.Username != "env@example.com" || creds.Password != "env-secret" {
			t.Fatalf("credentials = %+v, want environment Apple ID and password", creds)
		}
		return &webcore.AuthSession{UserEmail: creds.Username}, nil
	}
	var persisted *webcore.AuthSession
	persistWebSessionFn = func(session *webcore.AuthSession) error {
		persisted = session
		return nil
	}

	session, source, err := resolveSession(context.Background(), d, "", "", "")
	if err != nil {
		t.Fatalf("resolveSession() error = %v", err)
	}
	if source != "fresh" {
		t.Fatalf("source = %q, want fresh", source)
	}
	if session == nil || session.UserEmail != "env@example.com" {
		t.Fatalf("session = %+v, want env@example.com", session)
	}
	if persisted != session {
		t.Fatalf("persisted session = %p, want %p", persisted, session)
	}
	if strings.Join(lookups, ",") != "env@example.com" {
		t.Fatalf("lookups = %v, want [env@example.com]", lookups)
	}
	if got, want := stderr.String(), "Using web session for env@example.com from "+webAppleIDEnv+"; pass --apple-id to override\n"; got != want {
		t.Fatalf("stderr = %q, want %q", got, want)
	}
}

func TestResolveSessionSanitizesEnvAppleIDNotice(t *testing.T) {
	d, _, _ := newWebTestDeps()
	dir := t.TempDir()
	stderr := stubDefaultAppleIDResolverInputs(t, d, dir)
	t.Setenv(webAppleIDEnv, "attacker@example.com\nINJECTED\x1b[31m\u202e")

	var lookups []string
	recordSessionLookups(&lookups)

	if _, _, err := resolveSession(context.Background(), d, "", "", ""); err != nil {
		t.Fatalf("resolveSession() error = %v", err)
	}
	if got, want := strings.Join(lookups, ","), "attacker@example.com\nINJECTED\x1b[31m\u202e"; got != want {
		t.Fatalf("session lookup = %q, want unsanitized identity %q", got, want)
	}
	if got, want := stderr.String(), "Using web session for attacker@example.com INJECTED[31m from "+webAppleIDEnv+"; pass --apple-id to override\n"; got != want {
		t.Fatalf("stderr = %q, want sanitized notice %q", got, want)
	}
}

func TestResolveSessionExplicitAppleIDOverridesEnvAppleID(t *testing.T) {
	d, _, _ := newWebTestDeps()
	dir := t.TempDir()
	stderr := stubDefaultAppleIDResolverInputs(t, d, dir)
	t.Setenv(webAppleIDEnv, "env@example.com")

	origDefault := defaultCachedAppleIDFn
	t.Cleanup(func() { defaultCachedAppleIDFn = origDefault })
	defaultCachedAppleIDFn = func() (string, webcore.CachedSessionSource, error) {
		t.Fatal("did not expect cached-session default resolution when --apple-id is set")
		return "", webcore.CachedSessionSourceUnknown, nil
	}

	var lookups []string
	recordSessionLookups(&lookups)

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

func TestResolveSessionIgnoresBlankEnvAppleID(t *testing.T) {
	d, _, _ := newWebTestDeps()
	dir := t.TempDir()
	stderr := stubDefaultAppleIDResolverInputs(t, d, dir)
	writeTestCachedWebSession(t, dir, "only@example.com")
	// An exported-but-empty variable is the shape a CI runner produces for an
	// unset secret; it must behave like no variable at all.
	t.Setenv(webAppleIDEnv, "   ")

	var lookups []string
	recordSessionLookups(&lookups)

	session, _, err := resolveSession(context.Background(), d, "", "", "")
	if err != nil {
		t.Fatalf("resolveSession() error = %v", err)
	}
	if session.UserEmail != "only@example.com" {
		t.Fatalf("session.UserEmail = %q, want only@example.com", session.UserEmail)
	}
	if strings.Join(lookups, ",") != "only@example.com" {
		t.Fatalf("lookups = %v, want [only@example.com]", lookups)
	}
	if got, want := stderr.String(), "Using cached web session for only@example.com; pass --apple-id to override\n"; got != want {
		t.Fatalf("stderr = %q, want the cached-session notice %q", got, want)
	}
}

func TestResolveSessionEnvAppleIDPreemptsLastCachedSession(t *testing.T) {
	d, _, _ := newWebTestDeps()
	dir := t.TempDir()
	stderr := stubDefaultAppleIDResolverInputs(t, d, dir)
	writeTestCachedWebSession(t, dir, "last@example.com")
	t.Setenv(webAppleIDEnv, "env@example.com")

	tryResumeLastFn = func(ctx context.Context) (*webcore.AuthSession, bool, error) {
		t.Fatal("did not expect a last-session lookup when " + webAppleIDEnv + " names an account")
		return nil, false, nil
	}
	var lookups []string
	recordSessionLookups(&lookups)

	session, _, err := resolveSession(context.Background(), d, "", "", "")
	if err != nil {
		t.Fatalf("resolveSession() error = %v", err)
	}
	if session.UserEmail != "env@example.com" {
		t.Fatalf("session.UserEmail = %q, want env@example.com", session.UserEmail)
	}
	if strings.Join(lookups, ",") != "env@example.com" {
		t.Fatalf("lookups = %v, want [env@example.com]", lookups)
	}
	if !strings.Contains(stderr.String(), webAppleIDEnv) {
		t.Fatalf("stderr = %q, want it to name %s", stderr.String(), webAppleIDEnv)
	}
}

// TestResolveWebSessionEnvAppleIDSkipsAppleIDPromptWithEmptyCache pins the
// interactive path (asc web apps create): with nothing cached the resolver would
// otherwise prompt for an Apple ID, and the environment fallback has to answer
// that question before the prompt is reached.
func TestResolveWebSessionEnvAppleIDSkipsAppleIDPromptWithEmptyCache(t *testing.T) {
	d, _, _ := newWebTestDeps()
	dir := t.TempDir()
	stderr := stubDefaultAppleIDResolverInputs(t, d, dir)
	t.Setenv(webAppleIDEnv, "env@example.com")

	origDefault := defaultCachedAppleIDFn
	t.Cleanup(func() { defaultCachedAppleIDFn = origDefault })
	defaultCachedAppleIDFn = func() (string, webcore.CachedSessionSource, error) {
		t.Fatal("did not expect cached-session default resolution when " + webAppleIDEnv + " names an account")
		return "", webcore.CachedSessionSourceUnknown, nil
	}

	var lookups []string
	recordSessionLookups(&lookups)

	session, _, err := resolveWebSessionWith(context.Background(), d, "", "", "", webSessionResolveOptions{
		resolvePassword: func(ctx context.Context, password string) (string, error) {
			t.Fatal("did not expect password resolution for a cached session")
			return "", nil
		},
		promptAppleID: func(appleID *string) error {
			t.Fatal("did not expect an interactive Apple ID prompt when " + webAppleIDEnv + " is set")
			return nil
		},
	})
	if err != nil {
		t.Fatalf("resolveWebSession() error = %v", err)
	}
	if session.UserEmail != "env@example.com" {
		t.Fatalf("session.UserEmail = %q, want env@example.com", session.UserEmail)
	}
	if strings.Join(lookups, ",") != "env@example.com" {
		t.Fatalf("lookups = %v, want [env@example.com]", lookups)
	}
	if !strings.Contains(stderr.String(), "env@example.com") {
		t.Fatalf("stderr = %q, want the environment notice", stderr.String())
	}
}

// TestResolveWebSessionForCommandSessionFromEnvIgnoresEnvAppleID pins the
// carve-out: --session-from-env authenticates from the supplied bundle alone, so
// the Apple ID environment fallback must neither select an account nor announce
// one on that path.
func TestResolveWebSessionForCommandSessionFromEnvIgnoresEnvAppleID(t *testing.T) {
	d, _, _ := newWebTestDeps()
	dir := t.TempDir()
	stderr := stubDefaultAppleIDResolverInputs(t, d, dir)
	writeTestCachedWebSession(t, dir, "only@example.com")
	t.Setenv(webSessionBundleEnvName, "")
	t.Setenv(webAppleIDEnv, "env@example.com")

	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	flags := bindWebSessionFlagsWithSessionFromEnv(fs)
	if err := fs.Parse([]string{"--session-from-env"}); err != nil {
		t.Fatal(err)
	}
	_, _, cancel, err := resolveWebSessionForCommand(context.Background(), flags)
	defer cancel()
	if err == nil || !strings.Contains(err.Error(), webSessionBundleEnvName+" is unset or empty") {
		t.Fatalf("error = %v, want unset %s usage error", err, webSessionBundleEnvName)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}
