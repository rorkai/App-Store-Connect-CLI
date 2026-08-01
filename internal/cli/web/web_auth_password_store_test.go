package web

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"net/http"
	"strings"
	"testing"

	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

func preserveWebPasswordHooks(t *testing.T) {
	t.Helper()

	originalLoad := loadStoredWebPasswordFn
	originalStore := storeStoredWebPasswordFn
	originalExists := storedWebPasswordExistsFn
	originalDelete := deleteStoredWebPasswordFn
	originalDeleteAll := deleteAllStoredWebPasswordsFn
	originalDeleteSession := deleteWebSessionFn
	originalDeleteAllSessions := deleteAllWebSessionsFn
	originalWarningWriter := passwordStoreWarningWriter
	t.Cleanup(func() {
		loadStoredWebPasswordFn = originalLoad
		storeStoredWebPasswordFn = originalStore
		storedWebPasswordExistsFn = originalExists
		deleteStoredWebPasswordFn = originalDelete
		deleteAllStoredWebPasswordsFn = originalDeleteAll
		deleteWebSessionFn = originalDeleteSession
		deleteAllWebSessionsFn = originalDeleteAllSessions
		passwordStoreWarningWriter = originalWarningWriter
	})
}

func preserveWebLoginHooks(t *testing.T) {
	t.Helper()

	originalTryResume := tryResumeSessionFn
	originalTryResumeLast := tryResumeLastFn
	originalLoadCached := loadCachedSessionFn
	originalLoadLastCached := loadLastCachedSessionFn
	originalPromptPassword := promptPasswordFn
	originalPromptTwoFactor := promptTwoFactorCodeFn
	originalWebLogin := webLoginFn
	originalWebLoginWithClient := webLoginWithClientFn
	originalPrepare := prepareTwoFactorChallengeFn
	originalEnsure := ensureTwoFactorCodeRequestedFn
	originalSubmit := submitTwoFactorCodeFn
	originalPersist := persistWebSessionFn
	originalExpiredWriter := sessionExpiredWriter
	t.Cleanup(func() {
		tryResumeSessionFn = originalTryResume
		tryResumeLastFn = originalTryResumeLast
		loadCachedSessionFn = originalLoadCached
		loadLastCachedSessionFn = originalLoadLastCached
		promptPasswordFn = originalPromptPassword
		promptTwoFactorCodeFn = originalPromptTwoFactor
		webLoginFn = originalWebLogin
		webLoginWithClientFn = originalWebLoginWithClient
		prepareTwoFactorChallengeFn = originalPrepare
		ensureTwoFactorCodeRequestedFn = originalEnsure
		submitTwoFactorCodeFn = originalSubmit
		persistWebSessionFn = originalPersist
		sessionExpiredWriter = originalExpiredWriter
	})
}

func configureFreshPasswordLoginTest(t *testing.T) *webcore.AuthSession {
	t.Helper()

	preserveWebPasswordHooks(t)
	preserveWebLoginHooks(t)
	t.Setenv(webPasswordEnv, "")
	t.Setenv(webDontStorePasswordEnv, "")
	t.Setenv("ASC_BYPASS_KEYCHAIN", "")

	tryResumeSessionFn = func(context.Context, string) (*webcore.AuthSession, bool, error) {
		return nil, false, nil
	}
	loadStoredWebPasswordFn = func(string) (string, bool, error) {
		return "", false, nil
	}
	persistWebSessionFn = func(*webcore.AuthSession) error { return nil }
	expected := &webcore.AuthSession{UserEmail: "user@example.com"}
	webLoginFn = func(_ context.Context, credentials webcore.LoginCredentials) (*webcore.AuthSession, error) {
		if credentials.Username != "user@example.com" || credentials.Password != "prompted-secret" {
			t.Fatalf("Login credentials = %+v, want prompted credentials", credentials)
		}
		return expected, nil
	}
	return expected
}

func TestResolveSessionStoresPromptedPasswordAfterSuccessfulLogin(t *testing.T) {
	expected := configureFreshPasswordLoginTest(t)

	loginSucceeded := false
	originalLogin := webLoginFn
	webLoginFn = func(ctx context.Context, credentials webcore.LoginCredentials) (*webcore.AuthSession, error) {
		session, err := originalLogin(ctx, credentials)
		loginSucceeded = err == nil
		return session, err
	}
	promptPasswordFn = func(context.Context) (string, error) { return "prompted-secret", nil }
	var storedAppleID, storedPassword string
	storeStoredWebPasswordFn = func(appleID, password string) error {
		if !loginSucceeded {
			t.Fatal("password was stored before login succeeded")
		}
		storedAppleID, storedPassword = appleID, password
		return nil
	}

	session, source, err := resolveSession(context.Background(), "user@example.com", "", "")
	if err != nil {
		t.Fatalf("resolveSession() error = %v", err)
	}
	if session != expected || source != "fresh" {
		t.Fatalf("resolveSession() = (%+v, %q), want (%+v, fresh)", session, source, expected)
	}
	if storedAppleID != "user@example.com" || storedPassword != "prompted-secret" {
		t.Fatalf("stored credentials = (%q, %q), want prompted credentials", storedAppleID, storedPassword)
	}
}

func TestResolveSessionDoesNotStoreEnvironmentPassword(t *testing.T) {
	preserveWebPasswordHooks(t)
	preserveWebLoginHooks(t)
	t.Setenv(webPasswordEnv, "env-secret")
	t.Setenv(webDontStorePasswordEnv, "")
	t.Setenv("ASC_BYPASS_KEYCHAIN", "")

	tryResumeSessionFn = func(context.Context, string) (*webcore.AuthSession, bool, error) { return nil, false, nil }
	loadStoredWebPasswordFn = func(string) (string, bool, error) {
		t.Fatal("stored password should not be loaded when the environment password is set")
		return "", false, nil
	}
	promptPasswordFn = func(context.Context) (string, error) {
		t.Fatal("password should not be prompted when the environment password is set")
		return "", nil
	}
	storeStoredWebPasswordFn = func(string, string) error {
		t.Fatal("environment password should not be stored")
		return nil
	}
	webLoginFn = func(_ context.Context, credentials webcore.LoginCredentials) (*webcore.AuthSession, error) {
		if credentials.Password != "env-secret" {
			t.Fatalf("password = %q, want env-secret", credentials.Password)
		}
		return &webcore.AuthSession{UserEmail: credentials.Username}, nil
	}
	persistWebSessionFn = func(*webcore.AuthSession) error { return nil }

	if _, _, err := resolveSession(context.Background(), "user@example.com", "", ""); err != nil {
		t.Fatalf("resolveSession() error = %v", err)
	}
}

func TestResolveSessionOptOutSkipsStoredPasswordReadAndWrite(t *testing.T) {
	for _, optOutEnv := range []string{webDontStorePasswordEnv, "ASC_BYPASS_KEYCHAIN"} {
		t.Run(optOutEnv, func(t *testing.T) {
			configureFreshPasswordLoginTest(t)
			t.Setenv(optOutEnv, "1")

			loadStoredWebPasswordFn = func(string) (string, bool, error) {
				t.Error("stored password should not be loaded when storage is disabled")
				return "", false, nil
			}
			storeStoredWebPasswordFn = func(string, string) error {
				t.Error("prompted password should not be stored when storage is disabled")
				return nil
			}
			promptPasswordFn = func(context.Context) (string, error) { return "prompted-secret", nil }

			if _, _, err := resolveSession(context.Background(), "user@example.com", "", ""); err != nil {
				t.Fatalf("resolveSession() error = %v", err)
			}
		})
	}
}

func TestResolveSessionInvalidOptOutFailsClosedWithWarning(t *testing.T) {
	configureFreshPasswordLoginTest(t)
	t.Setenv(webDontStorePasswordEnv, "treu")

	loadStoredWebPasswordFn = func(string) (string, bool, error) {
		t.Error("stored password should not be loaded for an invalid opt-out value")
		return "", false, nil
	}
	storeStoredWebPasswordFn = func(string, string) error {
		t.Error("prompted password should not be stored for an invalid opt-out value")
		return nil
	}
	promptPasswordFn = func(context.Context) (string, error) { return "prompted-secret", nil }
	var warning bytes.Buffer
	passwordStoreWarningWriter = &warning
	resetInvalidWebPasswordOptOutWarnings()
	t.Cleanup(resetInvalidWebPasswordOptOutWarnings)

	if _, _, err := resolveSession(context.Background(), "user@example.com", "", ""); err != nil {
		t.Fatalf("resolveSession() error = %v", err)
	}
	if got := warning.String(); !strings.Contains(got, `invalid ASC_WEB_DONT_STORE_PASSWORD value "treu"`) || !strings.Contains(got, "password storage disabled") {
		t.Fatalf("warning = %q, want invalid opt-out warning with fail-closed behavior", got)
	}
}

func TestResolveSessionUsesStoredPasswordAndCachedCookiesForTwoFactorReauth(t *testing.T) {
	preserveWebPasswordHooks(t)
	preserveWebLoginHooks(t)
	t.Setenv(webPasswordEnv, "")
	t.Setenv(webDontStorePasswordEnv, "")
	t.Setenv("ASC_BYPASS_KEYCHAIN", "")

	cachedClient := &http.Client{}
	expected := &webcore.AuthSession{Client: cachedClient, UserEmail: "user@example.com"}
	tryResumeSessionFn = func(context.Context, string) (*webcore.AuthSession, bool, error) {
		return nil, false, webcore.ErrCachedSessionExpired
	}
	loadCachedSessionFn = func(string) (*webcore.AuthSession, bool, error) {
		return &webcore.AuthSession{Client: cachedClient, UserEmail: "user@example.com"}, true, nil
	}
	loadStoredWebPasswordFn = func(appleID string) (string, bool, error) {
		if appleID != "user@example.com" {
			t.Fatalf("stored password lookup Apple ID = %q", appleID)
		}
		return "stored-secret", true, nil
	}
	promptPasswordFn = func(context.Context) (string, error) {
		t.Fatal("stored password should avoid the password prompt")
		return "", nil
	}
	webLoginFn = func(context.Context, webcore.LoginCredentials) (*webcore.AuthSession, error) {
		t.Fatal("reauthentication should reuse the cached client")
		return nil, nil
	}
	webLoginWithClientFn = func(_ context.Context, client *http.Client, credentials webcore.LoginCredentials) (*webcore.AuthSession, error) {
		if client != cachedClient || credentials.Password != "stored-secret" {
			t.Fatalf("cached login = (%p, %+v), want cached client and stored password", client, credentials)
		}
		return expected, &webcore.TwoFactorRequiredError{}
	}
	prepareTwoFactorChallengeFn = func(context.Context, *webcore.AuthSession) (*webcore.TwoFactorChallenge, error) {
		return &webcore.TwoFactorChallenge{Method: "trusted-device"}, nil
	}
	ensureTwoFactorCodeRequestedFn = func(context.Context, *webcore.AuthSession) (*webcore.TwoFactorChallenge, error) {
		t.Fatal("trusted-device challenge should not request phone delivery")
		return nil, nil
	}
	promptTwoFactorCodeFn = func() (string, error) { return "123456", nil }
	submitTwoFactorCodeFn = func(_ context.Context, session *webcore.AuthSession, code string) error {
		if session != expected || code != "123456" {
			t.Fatalf("2FA submission = (%+v, %q), want expected session and code", session, code)
		}
		return nil
	}
	persistWebSessionFn = func(session *webcore.AuthSession) error {
		if session != expected {
			t.Fatalf("persisted session = %+v, want expected", session)
		}
		return nil
	}
	storeStoredWebPasswordFn = func(string, string) error {
		t.Fatal("an existing stored password should not be written again")
		return nil
	}

	session, source, err := resolveSession(context.Background(), "user@example.com", "", "")
	if err != nil {
		t.Fatalf("resolveSession() error = %v", err)
	}
	if session != expected || source != "auto-reauth" {
		t.Fatalf("resolveSession() = (%+v, %q), want (%+v, auto-reauth)", session, source, expected)
	}
}

func TestResolveAppCreateSessionPersistsStoredPasswordAutoReauth(t *testing.T) {
	preserveWebPasswordHooks(t)
	preserveWebLoginHooks(t)
	t.Setenv(webPasswordEnv, "")
	t.Setenv(webDontStorePasswordEnv, "")
	t.Setenv("ASC_BYPASS_KEYCHAIN", "")

	cachedClient := &http.Client{}
	expected := &webcore.AuthSession{Client: cachedClient, UserEmail: "user@example.com"}
	tryResumeSessionFn = func(context.Context, string) (*webcore.AuthSession, bool, error) {
		return nil, false, webcore.ErrCachedSessionExpired
	}
	loadCachedSessionFn = func(string) (*webcore.AuthSession, bool, error) {
		return &webcore.AuthSession{Client: cachedClient, UserEmail: "user@example.com"}, true, nil
	}
	loadStoredWebPasswordFn = func(appleID string) (string, bool, error) {
		if appleID != "user@example.com" {
			t.Fatalf("stored password lookup Apple ID = %q", appleID)
		}
		return "stored-secret", true, nil
	}
	webLoginWithClientFn = func(_ context.Context, client *http.Client, credentials webcore.LoginCredentials) (*webcore.AuthSession, error) {
		if client != cachedClient || credentials.Password != "stored-secret" {
			t.Fatalf("cached login = (%p, %+v), want cached client and stored password", client, credentials)
		}
		return expected, nil
	}
	promptPasswordFn = func(context.Context) (string, error) {
		t.Fatal("stored password should avoid the password prompt")
		return "", nil
	}
	webLoginFn = func(context.Context, webcore.LoginCredentials) (*webcore.AuthSession, error) {
		t.Fatal("auto-reauthentication should reuse the cached client")
		return nil, nil
	}
	var persisted *webcore.AuthSession
	persistWebSessionFn = func(session *webcore.AuthSession) error {
		persisted = session
		return nil
	}

	session, source, err := resolveAppCreateSession(context.Background(), "user@example.com", "", "")
	if err != nil {
		t.Fatalf("resolveAppCreateSession() error = %v", err)
	}
	if session != expected || source != "auto-reauth" {
		t.Fatalf("resolveAppCreateSession() = (%+v, %q), want (%+v, auto-reauth)", session, source, expected)
	}
	if persisted != expected {
		t.Fatalf("persisted session = %+v, want refreshed app-create session %+v", persisted, expected)
	}
}

func TestResolveSessionPromptedPasswordFallsBackFromExpiredCookiesToFreshClient(t *testing.T) {
	preserveWebPasswordHooks(t)
	preserveWebLoginHooks(t)
	t.Setenv(webPasswordEnv, "")
	t.Setenv(webDontStorePasswordEnv, "")
	t.Setenv("ASC_BYPASS_KEYCHAIN", "")

	cachedClient := &http.Client{}
	expected := &webcore.AuthSession{UserEmail: "user@example.com"}
	tryResumeSessionFn = func(context.Context, string) (*webcore.AuthSession, bool, error) {
		return nil, false, webcore.ErrCachedSessionExpired
	}
	loadCachedSessionFn = func(string) (*webcore.AuthSession, bool, error) {
		return &webcore.AuthSession{Client: cachedClient, UserEmail: "user@example.com"}, true, nil
	}
	loadStoredWebPasswordFn = func(string) (string, bool, error) { return "", false, nil }
	promptPasswordFn = func(context.Context) (string, error) { return "prompted-secret", nil }
	webLoginWithClientFn = func(_ context.Context, client *http.Client, credentials webcore.LoginCredentials) (*webcore.AuthSession, error) {
		if client != cachedClient || credentials.Password != "prompted-secret" {
			t.Fatalf("cached login = (%p, %+v), want cached client and prompted password", client, credentials)
		}
		return nil, errors.New("cached cookie jar rejected")
	}
	freshAttempts := 0
	webLoginFn = func(_ context.Context, credentials webcore.LoginCredentials) (*webcore.AuthSession, error) {
		freshAttempts++
		if credentials.Password != "prompted-secret" {
			t.Fatalf("fresh password = %q, want prompted-secret", credentials.Password)
		}
		return expected, nil
	}
	persistWebSessionFn = func(*webcore.AuthSession) error { return nil }
	storeStoredWebPasswordFn = func(string, string) error { return nil }

	session, source, err := resolveSession(context.Background(), "user@example.com", "", "")
	if err != nil {
		t.Fatalf("resolveSession() error = %v", err)
	}
	if session != expected || source != "fresh" || freshAttempts != 1 {
		t.Fatalf("result = (%+v, %q, fresh attempts %d), want fresh fallback", session, source, freshAttempts)
	}
}

func TestResolveSessionPromptedPasswordDoesNotRetryFreshAfterTwoFactorStarts(t *testing.T) {
	preserveWebPasswordHooks(t)
	preserveWebLoginHooks(t)
	t.Setenv(webPasswordEnv, "")
	t.Setenv(webDontStorePasswordEnv, "")
	t.Setenv("ASC_BYPASS_KEYCHAIN", "")

	cachedClient := &http.Client{}
	twoFactorSession := &webcore.AuthSession{Client: cachedClient, UserEmail: "user@example.com"}
	tryResumeSessionFn = func(context.Context, string) (*webcore.AuthSession, bool, error) {
		return nil, false, webcore.ErrCachedSessionExpired
	}
	loadCachedSessionFn = func(string) (*webcore.AuthSession, bool, error) {
		return &webcore.AuthSession{Client: cachedClient, UserEmail: "user@example.com"}, true, nil
	}
	loadStoredWebPasswordFn = func(string) (string, bool, error) { return "", false, nil }
	promptPasswordFn = func(context.Context) (string, error) { return "prompted-secret", nil }
	webLoginWithClientFn = func(context.Context, *http.Client, webcore.LoginCredentials) (*webcore.AuthSession, error) {
		return twoFactorSession, &webcore.TwoFactorRequiredError{}
	}
	prepareTwoFactorChallengeFn = func(context.Context, *webcore.AuthSession) (*webcore.TwoFactorChallenge, error) {
		return nil, errors.New("challenge setup failed")
	}
	webLoginFn = func(context.Context, webcore.LoginCredentials) (*webcore.AuthSession, error) {
		t.Fatal("2FA flow errors must not trigger a fresh login retry")
		return nil, nil
	}

	_, _, err := resolveSession(context.Background(), "user@example.com", "", "")
	if err == nil || !strings.Contains(err.Error(), "2fa challenge setup failed") {
		t.Fatalf("resolveSession() error = %v, want original 2FA setup failure", err)
	}
}

func TestResolveSessionAutoReauthDoesNotRetryFreshAfterTwoFactorStarts(t *testing.T) {
	preserveWebLoginHooks(t)
	t.Setenv(webPasswordEnv, "env-secret")

	cachedClient := &http.Client{}
	twoFactorSession := &webcore.AuthSession{Client: cachedClient, UserEmail: "user@example.com"}
	tryResumeSessionFn = func(context.Context, string) (*webcore.AuthSession, bool, error) {
		return nil, false, webcore.ErrCachedSessionExpired
	}
	loadCachedSessionFn = func(string) (*webcore.AuthSession, bool, error) {
		return &webcore.AuthSession{Client: cachedClient, UserEmail: "user@example.com"}, true, nil
	}
	webLoginWithClientFn = func(_ context.Context, client *http.Client, credentials webcore.LoginCredentials) (*webcore.AuthSession, error) {
		if client != cachedClient || credentials.Password != "env-secret" {
			t.Fatalf("cached login = (%p, %+v), want cached client and environment password", client, credentials)
		}
		return twoFactorSession, &webcore.TwoFactorRequiredError{}
	}
	prepareTwoFactorChallengeFn = func(context.Context, *webcore.AuthSession) (*webcore.TwoFactorChallenge, error) {
		return nil, errors.New("challenge setup failed")
	}
	webLoginFn = func(context.Context, webcore.LoginCredentials) (*webcore.AuthSession, error) {
		t.Fatal("automatic 2FA flow errors must not trigger a fresh login retry")
		return nil, nil
	}

	_, _, err := resolveSession(context.Background(), "user@example.com", "", "")
	if err == nil || !strings.Contains(err.Error(), "2fa challenge setup failed") {
		t.Fatalf("resolveSession() error = %v, want original automatic 2FA setup failure", err)
	}
}

func TestResolveSessionReplacesRejectedStoredPasswordOnlyAfterSuccessfulLogin(t *testing.T) {
	preserveWebPasswordHooks(t)
	preserveWebLoginHooks(t)
	t.Setenv(webPasswordEnv, "")
	t.Setenv(webDontStorePasswordEnv, "")
	t.Setenv("ASC_BYPASS_KEYCHAIN", "")

	cachedClient := &http.Client{}
	expected := &webcore.AuthSession{Client: cachedClient, UserEmail: "user@example.com"}
	tryResumeSessionFn = func(context.Context, string) (*webcore.AuthSession, bool, error) {
		return nil, false, webcore.ErrCachedSessionExpired
	}
	loadCachedSessionFn = func(string) (*webcore.AuthSession, bool, error) {
		return &webcore.AuthSession{Client: cachedClient, UserEmail: "user@example.com"}, true, nil
	}
	loadStoredWebPasswordFn = func(string) (string, bool, error) { return "old-secret", true, nil }
	promptPasswordFn = func(context.Context) (string, error) { return "new-secret", nil }

	loginAttempts := 0
	webLoginWithClientFn = func(_ context.Context, client *http.Client, credentials webcore.LoginCredentials) (*webcore.AuthSession, error) {
		loginAttempts++
		if client != cachedClient {
			t.Fatal("expected cached client to be retained across password replacement")
		}
		switch loginAttempts {
		case 1:
			if credentials.Password != "old-secret" {
				t.Fatalf("first password = %q, want old-secret", credentials.Password)
			}
			return nil, webcore.ErrInvalidAppleAccountCredentials
		case 2:
			if credentials.Password != "new-secret" {
				t.Fatalf("second password = %q, want new-secret", credentials.Password)
			}
			return expected, nil
		default:
			t.Fatalf("unexpected login attempt %d", loginAttempts)
			return nil, nil
		}
	}
	webLoginFn = func(context.Context, webcore.LoginCredentials) (*webcore.AuthSession, error) {
		t.Fatal("password replacement should retain cached cookies")
		return nil, nil
	}
	persistWebSessionFn = func(*webcore.AuthSession) error { return nil }
	var warning bytes.Buffer
	passwordStoreWarningWriter = &warning
	stored := ""
	storeStoredWebPasswordFn = func(_ string, password string) error {
		if loginAttempts != 2 {
			t.Fatal("replacement password was stored before its login succeeded")
		}
		stored = password
		return nil
	}

	session, source, err := resolveSession(context.Background(), "user@example.com", "", "")
	if err != nil {
		t.Fatalf("resolveSession() error = %v", err)
	}
	if session != expected || source != "fresh" || stored != "new-secret" {
		t.Fatalf("result = (%+v, %q, stored %q), want expected fresh session with replacement", session, source, stored)
	}
	if got := warning.String(); !strings.Contains(got, "Saved Apple Account password was rejected; enter the current password to replace it.") {
		t.Fatalf("warning = %q, want stored-password rejection notice", got)
	}
}

func TestResolveSessionReplacesRejectedStoredPasswordWithoutCachedSession(t *testing.T) {
	preserveWebPasswordHooks(t)
	preserveWebLoginHooks(t)
	t.Setenv(webPasswordEnv, "")
	t.Setenv(webDontStorePasswordEnv, "")
	t.Setenv("ASC_BYPASS_KEYCHAIN", "")

	expected := &webcore.AuthSession{UserEmail: "user@example.com"}
	tryResumeSessionFn = func(context.Context, string) (*webcore.AuthSession, bool, error) {
		return nil, false, nil
	}
	loadStoredWebPasswordFn = func(string) (string, bool, error) { return "old-secret", true, nil }
	promptPasswordFn = func(context.Context) (string, error) { return "new-secret", nil }
	persistWebSessionFn = func(*webcore.AuthSession) error { return nil }

	loginAttempts := 0
	webLoginFn = func(_ context.Context, credentials webcore.LoginCredentials) (*webcore.AuthSession, error) {
		loginAttempts++
		switch loginAttempts {
		case 1:
			if credentials.Password != "old-secret" {
				t.Fatalf("first password = %q, want old-secret", credentials.Password)
			}
			return nil, webcore.ErrInvalidAppleAccountCredentials
		case 2:
			if credentials.Password != "new-secret" {
				t.Fatalf("second password = %q, want new-secret", credentials.Password)
			}
			return expected, nil
		default:
			t.Fatalf("unexpected login attempt %d", loginAttempts)
			return nil, nil
		}
	}
	var warning bytes.Buffer
	passwordStoreWarningWriter = &warning
	stored := ""
	storeStoredWebPasswordFn = func(_ string, password string) error {
		if loginAttempts != 2 {
			t.Fatal("replacement password was stored before its login succeeded")
		}
		stored = password
		return nil
	}

	session, source, err := resolveSession(context.Background(), "user@example.com", "", "")
	if err != nil {
		t.Fatalf("resolveSession() error = %v", err)
	}
	if session != expected || source != "fresh" || stored != "new-secret" {
		t.Fatalf("result = (%+v, %q, stored %q), want expected fresh session with replacement", session, source, stored)
	}
	if got := warning.String(); !strings.Contains(got, "Saved Apple Account password was rejected; enter the current password to replace it.") {
		t.Fatalf("warning = %q, want stored-password rejection notice", got)
	}
}

func TestResolveSessionWarnsButSucceedsWhenPromptedPasswordCannotBeStored(t *testing.T) {
	configureFreshPasswordLoginTest(t)
	promptPasswordFn = func(context.Context) (string, error) { return "prompted-secret", nil }
	storeStoredWebPasswordFn = func(string, string) error { return errors.New("keychain locked") }
	var warning bytes.Buffer
	passwordStoreWarningWriter = &warning

	if _, _, err := resolveSession(context.Background(), "user@example.com", "", ""); err != nil {
		t.Fatalf("resolveSession() error = %v", err)
	}
	if got := warning.String(); !strings.Contains(got, "could not save") || !strings.Contains(got, "keychain locked") {
		t.Fatalf("warning = %q, want non-fatal password-store warning", got)
	}
}

func TestWebAuthStatusReportsStoredPasswordForExpiredSession(t *testing.T) {
	preserveWebPasswordHooks(t)
	preserveWebLoginHooks(t)
	t.Setenv(webDontStorePasswordEnv, "")
	t.Setenv("ASC_BYPASS_KEYCHAIN", "")
	tryResumeSessionFn = func(context.Context, string) (*webcore.AuthSession, bool, error) {
		return nil, false, webcore.ErrCachedSessionExpired
	}
	storedWebPasswordExistsFn = func(appleID string) (bool, error) {
		return appleID == "user@example.com", nil
	}

	cmd := WebAuthStatusCommand()
	if err := cmd.FlagSet.Parse([]string{"--apple-id", "user@example.com", "--output", "json"}); err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	var runErr error
	stdout, _ := captureWebCommandOutput(t, func() {
		runErr = cmd.Exec(context.Background(), nil)
	})
	if runErr != nil {
		t.Fatalf("status Exec() error = %v", runErr)
	}
	if !strings.Contains(stdout, `"authenticated":false`) || !strings.Contains(stdout, `"passwordStored":true`) {
		t.Fatalf("status output = %q, want unauthenticated with stored password", stdout)
	}
}

func TestWebAuthLogoutForgetPasswordRequiresConfirmation(t *testing.T) {
	preserveWebPasswordHooks(t)
	deleteWebSessionFn = func(string) error {
		t.Fatal("session should not be deleted without --confirm")
		return nil
	}
	deleteStoredWebPasswordFn = func(string) error {
		t.Fatal("password should not be deleted without --confirm")
		return nil
	}

	cmd := WebAuthLogoutCommand()
	if err := cmd.FlagSet.Parse([]string{"--apple-id", "user@example.com", "--forget-password"}); err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	var runErr error
	_, stderr := captureWebCommandOutput(t, func() {
		runErr = cmd.Exec(context.Background(), nil)
	})
	if !errors.Is(runErr, flag.ErrHelp) || !strings.Contains(stderr, "--confirm") {
		t.Fatalf("Exec() = %v, stderr %q; want --confirm usage error", runErr, stderr)
	}
}

func TestWebAuthLogoutCanForgetPasswordWithSession(t *testing.T) {
	preserveWebPasswordHooks(t)
	var deletedSession, deletedPassword string
	deleteWebSessionFn = func(appleID string) error {
		deletedSession = appleID
		return nil
	}
	deleteStoredWebPasswordFn = func(appleID string) error {
		deletedPassword = appleID
		return nil
	}

	cmd := WebAuthLogoutCommand()
	if err := cmd.FlagSet.Parse([]string{"--apple-id", "user@example.com", "--forget-password", "--confirm"}); err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	var runErr error
	stdout, _ := captureWebCommandOutput(t, func() {
		runErr = cmd.Exec(context.Background(), nil)
	})
	if runErr != nil {
		t.Fatalf("logout Exec() error = %v", runErr)
	}
	if deletedSession != "user@example.com" || deletedPassword != "user@example.com" {
		t.Fatalf("deleted = (%q, %q), want both account entries", deletedSession, deletedPassword)
	}
	if !strings.Contains(stdout, "stored password") {
		t.Fatalf("logout output = %q, want stored-password confirmation", stdout)
	}
}

func TestWebAuthLogoutForgetPasswordFailsBeforeDeletingSession(t *testing.T) {
	preserveWebPasswordHooks(t)
	deleteStoredWebPasswordFn = func(string) error { return webcore.ErrPasswordStoreUnavailable }
	deleteWebSessionFn = func(string) error {
		t.Fatal("session should not be deleted when the saved password cannot be removed")
		return nil
	}

	cmd := WebAuthLogoutCommand()
	if err := cmd.FlagSet.Parse([]string{"--apple-id", "user@example.com", "--forget-password", "--confirm"}); err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	err := cmd.Exec(context.Background(), nil)
	if !errors.Is(err, webcore.ErrPasswordStoreUnavailable) || !strings.Contains(err.Error(), "failed to forget saved password") {
		t.Fatalf("Exec() error = %v, want password-store failure before session deletion", err)
	}
}

func TestWebAuthLogoutAllForgetPasswordRequiresConfirmationWithoutDeletingAnything(t *testing.T) {
	preserveWebPasswordHooks(t)
	deleteAllStoredWebPasswordsFn = func() error {
		t.Fatal("passwords should not be deleted without --confirm")
		return nil
	}
	deleteAllWebSessionsFn = func() error {
		t.Fatal("sessions should not be deleted without --confirm")
		return nil
	}

	cmd := WebAuthLogoutCommand()
	if err := cmd.FlagSet.Parse([]string{"--all", "--forget-password"}); err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	var runErr error
	_, stderr := captureWebCommandOutput(t, func() {
		runErr = cmd.Exec(context.Background(), nil)
	})
	if !errors.Is(runErr, flag.ErrHelp) || !strings.Contains(stderr, "--confirm") {
		t.Fatalf("Exec() = %v, stderr %q; want --confirm usage error", runErr, stderr)
	}
}

func TestWebAuthLogoutAllCanForgetPasswordsWithSessions(t *testing.T) {
	preserveWebPasswordHooks(t)
	var calls []string
	deleteAllStoredWebPasswordsFn = func() error {
		calls = append(calls, "passwords")
		return nil
	}
	deleteAllWebSessionsFn = func() error {
		calls = append(calls, "sessions")
		return nil
	}

	cmd := WebAuthLogoutCommand()
	if err := cmd.FlagSet.Parse([]string{"--all", "--forget-password", "--confirm"}); err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	var runErr error
	stdout, _ := captureWebCommandOutput(t, func() {
		runErr = cmd.Exec(context.Background(), nil)
	})
	if runErr != nil {
		t.Fatalf("logout Exec() error = %v", runErr)
	}
	if got := strings.Join(calls, ","); got != "passwords,sessions" {
		t.Fatalf("deletion order = %q, want passwords,sessions", got)
	}
	if !strings.Contains(stdout, "stored passwords") {
		t.Fatalf("logout output = %q, want stored-password confirmation", stdout)
	}
}

func TestWebAuthLogoutPasswordFlagsAreExperimental(t *testing.T) {
	cmd := WebAuthLogoutCommand()
	for _, name := range []string{"forget-password", "confirm"} {
		flag := cmd.FlagSet.Lookup(name)
		if flag == nil {
			t.Fatalf("--%s flag not found", name)
		}
		if !strings.HasPrefix(flag.Usage, "[experimental]") {
			t.Fatalf("--%s usage = %q, want [experimental] prefix", name, flag.Usage)
		}
	}
}
