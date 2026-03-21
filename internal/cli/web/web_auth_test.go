package web

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/creack/pty"
	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

func TestReadPasswordFromInput(t *testing.T) {
	origPromptPassword := promptPasswordFn
	t.Cleanup(func() {
		promptPasswordFn = origPromptPassword
	})

	t.Run("uses environment variable before prompt fallback", func(t *testing.T) {
		t.Setenv(webPasswordEnv, " env-password ")
		promptPasswordFn = func(ctx context.Context) (string, error) {
			t.Fatal("did not expect prompt fallback when env password is set")
			return "", nil
		}

		password, err := readPasswordFromInput(context.Background())
		if err != nil {
			t.Fatalf("readPasswordFromInput returned error: %v", err)
		}
		if password != "env-password" {
			t.Fatalf("expected env password %q, got %q", "env-password", password)
		}
	})

	t.Run("falls back to interactive prompt when env is not provided", func(t *testing.T) {
		t.Setenv(webPasswordEnv, "")
		called := false
		promptPasswordFn = func(ctx context.Context) (string, error) {
			called = true
			return " prompted-password ", nil
		}

		password, err := readPasswordFromInput(context.Background())
		if err != nil {
			t.Fatalf("readPasswordFromInput returned error: %v", err)
		}
		if !called {
			t.Fatal("expected interactive prompt fallback to be used")
		}
		if password != "prompted-password" {
			t.Fatalf("expected prompted password %q, got %q", "prompted-password", password)
		}
	})
}

func TestReadPasswordFromTerminalFD(t *testing.T) {
	origReadPassword := termReadPasswordFn
	t.Cleanup(func() {
		termReadPasswordFn = origReadPassword
	})

	t.Run("trims interactive password and writes prompt", func(t *testing.T) {
		termReadPasswordFn = func(fd int) ([]byte, error) {
			return []byte("  secret-pass  "), nil
		}
		var prompt bytes.Buffer

		password, err := readPasswordFromTerminalFD(context.Background(), &prompt)
		if err != nil {
			t.Fatalf("readPasswordFromTerminalFD returned error: %v", err)
		}
		if password != "secret-pass" {
			t.Fatalf("expected password %q, got %q", "secret-pass", password)
		}
		if !strings.Contains(prompt.String(), "Apple Account password:") {
			t.Fatalf("expected password prompt text, got %q", prompt.String())
		}
	})

	t.Run("propagates terminal read failure", func(t *testing.T) {
		termReadPasswordFn = func(fd int) ([]byte, error) {
			return nil, errors.New("terminal read failed")
		}

		_, err := readPasswordFromTerminalFD(context.Background(), &bytes.Buffer{})
		if err == nil {
			t.Fatal("expected read failure")
		}
		if !strings.Contains(err.Error(), "failed to read password") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("preserves prompt cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		termReadPasswordFn = func(fd int) ([]byte, error) {
			return nil, errors.New("read aborted")
		}

		_, err := readPasswordFromTerminalFD(ctx, &bytes.Buffer{})
		if err == nil {
			t.Fatal("expected cancellation error")
		}
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected context cancellation, got %v", err)
		}
	})
}

func TestReadPasswordFromTerminalPropagatesCtrlCAsInterrupt(t *testing.T) {
	origSignalProcessInterrupt := signalProcessInterruptFn
	t.Cleanup(func() {
		signalProcessInterruptFn = origSignalProcessInterrupt
	})

	ptmx, tty, err := pty.Open()
	if err != nil {
		t.Fatalf("pty.Open() error: %v", err)
	}
	t.Cleanup(func() {
		_ = ptmx.Close()
		_ = tty.Close()
	})

	interrupts := make(chan struct{}, 1)
	signalProcessInterruptFn = func() error {
		select {
		case interrupts <- struct{}{}:
		default:
		}
		return nil
	}

	promptSeen := make(chan struct{})
	readPromptDone := make(chan error, 1)
	go func() {
		buf := make([]byte, 128)
		for {
			n, err := ptmx.Read(buf)
			if n > 0 && strings.Contains(string(buf[:n]), "Apple Account password:") {
				close(promptSeen)
				readPromptDone <- nil
				return
			}
			if err != nil {
				readPromptDone <- err
				return
			}
		}
	}()

	errCh := make(chan error, 1)
	go func() {
		_, err := readPasswordFromTerminal(context.Background(), tty, tty, false)
		errCh <- err
	}()

	select {
	case <-promptSeen:
	case err := <-readPromptDone:
		t.Fatalf("failed waiting for password prompt: %v", err)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for password prompt")
	}

	time.Sleep(50 * time.Millisecond)

	if _, err := ptmx.Write([]byte{3}); err != nil {
		t.Fatalf("ptmx.Write() error: %v", err)
	}

	select {
	case <-interrupts:
	case <-time.After(2 * time.Second):
		t.Fatal("expected Ctrl+C to be re-emitted as a process interrupt")
	}

	select {
	case err := <-errCh:
		if err == nil {
			t.Fatal("expected cancellation error")
		}
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected context cancellation, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for password prompt to return")
	}
}

func TestReadTwoFactorCodeFrom(t *testing.T) {
	t.Run("trims input", func(t *testing.T) {
		input := strings.NewReader(" 123456 \n")
		var prompt bytes.Buffer

		code, err := readTwoFactorCodeFrom(input, &prompt)
		if err != nil {
			t.Fatalf("readTwoFactorCodeFrom returned error: %v", err)
		}
		if code != "123456" {
			t.Fatalf("expected code %q, got %q", "123456", code)
		}
		if !strings.Contains(prompt.String(), "Enter 2FA code") {
			t.Fatalf("expected prompt text, got %q", prompt.String())
		}
	})

	t.Run("rejects empty", func(t *testing.T) {
		input := strings.NewReader("\n")
		var prompt bytes.Buffer

		_, err := readTwoFactorCodeFrom(input, &prompt)
		if err == nil {
			t.Fatal("expected error for empty input")
		}
		if !strings.Contains(err.Error(), "empty 2fa code") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestReadTwoFactorCodeFromTerminalFD(t *testing.T) {
	origReadPassword := termReadPasswordFn
	t.Cleanup(func() {
		termReadPasswordFn = origReadPassword
	})

	t.Run("trims input", func(t *testing.T) {
		termReadPasswordFn = func(fd int) ([]byte, error) {
			return []byte(" 654321 "), nil
		}
		var prompt bytes.Buffer

		code, err := readTwoFactorCodeFromTerminalFD(0, &prompt)
		if err != nil {
			t.Fatalf("readTwoFactorCodeFromTerminalFD returned error: %v", err)
		}
		if code != "654321" {
			t.Fatalf("expected code %q, got %q", "654321", code)
		}
		if !strings.Contains(prompt.String(), "Enter 2FA code") {
			t.Fatalf("expected prompt text, got %q", prompt.String())
		}
	})

	t.Run("rejects empty", func(t *testing.T) {
		termReadPasswordFn = func(fd int) ([]byte, error) {
			return []byte("   "), nil
		}

		_, err := readTwoFactorCodeFromTerminalFD(0, &bytes.Buffer{})
		if err == nil {
			t.Fatal("expected error for empty input")
		}
		if !strings.Contains(err.Error(), "empty 2fa code") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("read failure", func(t *testing.T) {
		termReadPasswordFn = func(fd int) ([]byte, error) {
			return nil, errors.New("tty read failed")
		}

		_, err := readTwoFactorCodeFromTerminalFD(0, &bytes.Buffer{})
		if err == nil {
			t.Fatal("expected read error")
		}
		if !strings.Contains(err.Error(), "failed to read 2fa code") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestLoginWithOptionalTwoFactorPromptsWhenCodeMissing(t *testing.T) {
	origPrompt := promptTwoFactorCodeFn
	origLogin := webLoginFn
	origSubmit := submitTwoFactorCodeFn
	t.Cleanup(func() {
		promptTwoFactorCodeFn = origPrompt
		webLoginFn = origLogin
		submitTwoFactorCodeFn = origSubmit
	})

	var prompted bool
	var submittedCode string

	webLoginFn = func(ctx context.Context, creds webcore.LoginCredentials) (*webcore.AuthSession, error) {
		return &webcore.AuthSession{}, &webcore.TwoFactorRequiredError{}
	}
	promptTwoFactorCodeFn = func() (string, error) {
		prompted = true
		return "654321", nil
	}
	submitTwoFactorCodeFn = func(ctx context.Context, session *webcore.AuthSession, code string) error {
		submittedCode = code
		return nil
	}

	session, err := loginWithOptionalTwoFactor(context.Background(), "user@example.com", "secret", "")
	if err != nil {
		t.Fatalf("loginWithOptionalTwoFactor returned error: %v", err)
	}
	if session == nil {
		t.Fatal("expected non-nil session")
	}
	if !prompted {
		t.Fatal("expected interactive prompt for missing 2fa code")
	}
	if submittedCode != "654321" {
		t.Fatalf("expected submitted code %q, got %q", "654321", submittedCode)
	}
}

func TestLoginWithOptionalTwoFactorReturnsPromptError(t *testing.T) {
	origPrompt := promptTwoFactorCodeFn
	origLogin := webLoginFn
	origSubmit := submitTwoFactorCodeFn
	t.Cleanup(func() {
		promptTwoFactorCodeFn = origPrompt
		webLoginFn = origLogin
		submitTwoFactorCodeFn = origSubmit
	})

	webLoginFn = func(ctx context.Context, creds webcore.LoginCredentials) (*webcore.AuthSession, error) {
		return &webcore.AuthSession{}, &webcore.TwoFactorRequiredError{}
	}
	promptTwoFactorCodeFn = func() (string, error) {
		return "", errors.New("2fa required: re-run with --two-factor-code")
	}
	submitTwoFactorCodeFn = func(ctx context.Context, session *webcore.AuthSession, code string) error {
		t.Fatal("did not expect submit when prompt fails")
		return nil
	}

	_, err := loginWithOptionalTwoFactor(context.Background(), "user@example.com", "secret", "")
	if err == nil {
		t.Fatal("expected error when prompt fails")
	}
	if !strings.Contains(err.Error(), "re-run with --two-factor-code") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestResolveSessionUsesLastCachedSessionWhenAppleIDMissing(t *testing.T) {
	origTryResume := tryResumeSessionFn
	origTryResumeLast := tryResumeLastFn
	origPromptPassword := promptPasswordFn
	t.Cleanup(func() {
		tryResumeSessionFn = origTryResume
		tryResumeLastFn = origTryResumeLast
		promptPasswordFn = origPromptPassword
	})

	expected := &webcore.AuthSession{UserEmail: "cached@example.com"}
	tryResumeSessionFn = func(ctx context.Context, username string) (*webcore.AuthSession, bool, error) {
		t.Fatal("did not expect user-scoped cache lookup when apple-id is omitted")
		return nil, false, nil
	}
	tryResumeLastFn = func(ctx context.Context) (*webcore.AuthSession, bool, error) {
		return expected, true, nil
	}
	promptPasswordFn = func(ctx context.Context) (string, error) {
		t.Fatal("did not expect password prompt when cache hit")
		return "", nil
	}

	session, source, err := resolveSession(context.Background(), "", "", "")
	if err != nil {
		t.Fatalf("resolveSession returned error: %v", err)
	}
	if source != "cache" {
		t.Fatalf("expected source %q, got %q", "cache", source)
	}
	if session != expected {
		t.Fatalf("expected cached session pointer to be returned")
	}
}

func TestResolveSessionRequiresAppleIDWhenNoCachedSessionExists(t *testing.T) {
	origTryResumeLast := tryResumeLastFn
	t.Cleanup(func() {
		tryResumeLastFn = origTryResumeLast
	})

	tryResumeLastFn = func(ctx context.Context) (*webcore.AuthSession, bool, error) {
		return nil, false, nil
	}

	_, _, err := resolveSession(context.Background(), "", "", "")
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected ErrHelp, got %v", err)
	}
}

func TestResolveSessionPrintsExpiredNoticeBeforePrompt(t *testing.T) {
	origTryResume := tryResumeSessionFn
	origTryResumeLast := tryResumeLastFn
	origLoadCachedSession := loadCachedSessionFn
	origLoadLastCachedSession := loadLastCachedSessionFn
	origPromptPassword := promptPasswordFn
	origWebLogin := webLoginFn
	origPersistWebSession := persistWebSessionFn
	origWebLoginWithClient := webLoginWithClientFn
	origExpiredWriter := sessionExpiredWriter
	t.Cleanup(func() {
		tryResumeSessionFn = origTryResume
		tryResumeLastFn = origTryResumeLast
		loadCachedSessionFn = origLoadCachedSession
		loadLastCachedSessionFn = origLoadLastCachedSession
		promptPasswordFn = origPromptPassword
		webLoginFn = origWebLogin
		persistWebSessionFn = origPersistWebSession
		webLoginWithClientFn = origWebLoginWithClient
		sessionExpiredWriter = origExpiredWriter
	})

	t.Setenv("ASC_WEB_SESSION_CACHE", "0")
	t.Setenv(webPasswordEnv, "")

	expected := &webcore.AuthSession{UserEmail: "user@example.com"}
	var notice bytes.Buffer
	sessionExpiredWriter = &notice

	tryResumeSessionFn = func(ctx context.Context, username string) (*webcore.AuthSession, bool, error) {
		if username != "user@example.com" {
			t.Fatalf("expected username user@example.com, got %q", username)
		}
		return nil, false, webcore.ErrCachedSessionExpired
	}
	tryResumeLastFn = func(ctx context.Context) (*webcore.AuthSession, bool, error) {
		t.Fatal("did not expect last-session cache lookup when apple-id is provided")
		return nil, false, nil
	}
	loadCachedSessionFn = func(username string) (*webcore.AuthSession, bool, error) {
		return nil, false, nil
	}
	loadLastCachedSessionFn = func() (*webcore.AuthSession, bool, error) {
		t.Fatal("did not expect last cached-session load when apple-id is provided")
		return nil, false, nil
	}
	persistWebSessionFn = func(session *webcore.AuthSession) error {
		if session != expected {
			t.Fatal("expected prompted fresh-login session to be persisted")
		}
		return nil
	}
	webLoginWithClientFn = func(ctx context.Context, client *http.Client, creds webcore.LoginCredentials) (*webcore.AuthSession, error) {
		t.Fatal("did not expect cached-client relogin without an env password")
		return nil, nil
	}
	promptPasswordFn = func(ctx context.Context) (string, error) {
		if got := notice.String(); got != "Session expired.\n" {
			t.Fatalf("expected expired notice before password prompt, got %q", got)
		}
		return "secret", nil
	}
	webLoginFn = func(ctx context.Context, creds webcore.LoginCredentials) (*webcore.AuthSession, error) {
		if creds.Username != "user@example.com" {
			t.Fatalf("expected login username user@example.com, got %q", creds.Username)
		}
		if creds.Password != "secret" {
			t.Fatalf("expected prompted password to be used, got %q", creds.Password)
		}
		return expected, nil
	}

	session, source, err := resolveSession(context.Background(), "user@example.com", "", "")
	if err != nil {
		t.Fatalf("resolveSession returned error: %v", err)
	}
	if source != "fresh" {
		t.Fatalf("expected source %q, got %q", "fresh", source)
	}
	if session != expected {
		t.Fatal("expected fresh login session to be returned")
	}
	if got := notice.String(); got != "Session expired.\n" {
		t.Fatalf("expected expired notice output, got %q", got)
	}
}

func TestResolveSessionAutoReauthsExpiredCachedSessionUsingEnvPassword(t *testing.T) {
	origTryResume := tryResumeSessionFn
	origTryResumeLast := tryResumeLastFn
	origLoadCachedSession := loadCachedSessionFn
	origLoadLastCachedSession := loadLastCachedSessionFn
	origPromptPassword := promptPasswordFn
	origWebLogin := webLoginFn
	origPersistWebSession := persistWebSessionFn
	origWebLoginWithClient := webLoginWithClientFn
	origExpiredWriter := sessionExpiredWriter
	t.Cleanup(func() {
		tryResumeSessionFn = origTryResume
		tryResumeLastFn = origTryResumeLast
		loadCachedSessionFn = origLoadCachedSession
		loadLastCachedSessionFn = origLoadLastCachedSession
		promptPasswordFn = origPromptPassword
		webLoginFn = origWebLogin
		persistWebSessionFn = origPersistWebSession
		webLoginWithClientFn = origWebLoginWithClient
		sessionExpiredWriter = origExpiredWriter
	})

	t.Setenv(webPasswordEnv, "env-secret")

	var notice bytes.Buffer
	sessionExpiredWriter = &notice

	cachedClient := &http.Client{}
	expected := &webcore.AuthSession{Client: cachedClient, UserEmail: "user@example.com", ProviderID: 7}

	tryResumeSessionFn = func(ctx context.Context, username string) (*webcore.AuthSession, bool, error) {
		if username != "user@example.com" {
			t.Fatalf("expected username user@example.com, got %q", username)
		}
		return nil, false, webcore.ErrCachedSessionExpired
	}
	tryResumeLastFn = func(ctx context.Context) (*webcore.AuthSession, bool, error) {
		t.Fatal("did not expect last-session cache lookup when apple-id is provided")
		return nil, false, nil
	}
	loadCachedSessionFn = func(username string) (*webcore.AuthSession, bool, error) {
		if username != "user@example.com" {
			t.Fatalf("expected cached-session load for user@example.com, got %q", username)
		}
		return &webcore.AuthSession{Client: cachedClient, UserEmail: "user@example.com"}, true, nil
	}
	loadLastCachedSessionFn = func() (*webcore.AuthSession, bool, error) {
		t.Fatal("did not expect last cached-session load when apple-id is provided")
		return nil, false, nil
	}
	promptPasswordFn = func(ctx context.Context) (string, error) {
		t.Fatal("did not expect password prompt during silent auto-reauth")
		return "", nil
	}
	webLoginFn = func(ctx context.Context, creds webcore.LoginCredentials) (*webcore.AuthSession, error) {
		t.Fatal("did not expect fresh-login path during silent auto-reauth")
		return nil, nil
	}
	persistWebSessionFn = func(session *webcore.AuthSession) error {
		if session != expected {
			t.Fatal("expected auto-reauth session to be persisted")
		}
		return nil
	}
	webLoginWithClientFn = func(ctx context.Context, client *http.Client, creds webcore.LoginCredentials) (*webcore.AuthSession, error) {
		if client != cachedClient {
			t.Fatal("expected cached client to be reused for auto-reauth")
		}
		if creds.Username != "user@example.com" {
			t.Fatalf("expected login username user@example.com, got %q", creds.Username)
		}
		if creds.Password != "env-secret" {
			t.Fatalf("expected env password to be used, got %q", creds.Password)
		}
		return expected, nil
	}

	session, source, err := resolveSession(context.Background(), "user@example.com", "", "")
	if err != nil {
		t.Fatalf("resolveSession returned error: %v", err)
	}
	if source != "auto-reauth" {
		t.Fatalf("expected source %q, got %q", "auto-reauth", source)
	}
	if session != expected {
		t.Fatal("expected auto-reauth session to be returned")
	}
	if got := notice.String(); got != "" {
		t.Fatalf("did not expect expired-session notice on successful auto-reauth, got %q", got)
	}
}

func TestResolveSessionAutoReauthsExpiredLastCachedSessionUsingStoredEmail(t *testing.T) {
	origTryResume := tryResumeSessionFn
	origTryResumeLast := tryResumeLastFn
	origLoadCachedSession := loadCachedSessionFn
	origLoadLastCachedSession := loadLastCachedSessionFn
	origPromptPassword := promptPasswordFn
	origWebLogin := webLoginFn
	origPersistWebSession := persistWebSessionFn
	origWebLoginWithClient := webLoginWithClientFn
	origExpiredWriter := sessionExpiredWriter
	t.Cleanup(func() {
		tryResumeSessionFn = origTryResume
		tryResumeLastFn = origTryResumeLast
		loadCachedSessionFn = origLoadCachedSession
		loadLastCachedSessionFn = origLoadLastCachedSession
		promptPasswordFn = origPromptPassword
		webLoginFn = origWebLogin
		persistWebSessionFn = origPersistWebSession
		webLoginWithClientFn = origWebLoginWithClient
		sessionExpiredWriter = origExpiredWriter
	})

	t.Setenv(webPasswordEnv, "env-secret")

	var notice bytes.Buffer
	sessionExpiredWriter = &notice

	cachedClient := &http.Client{}
	expected := &webcore.AuthSession{Client: cachedClient, UserEmail: "cached@example.com", ProviderID: 42}

	tryResumeSessionFn = func(ctx context.Context, username string) (*webcore.AuthSession, bool, error) {
		t.Fatal("did not expect user-scoped cache lookup when apple-id is omitted")
		return nil, false, nil
	}
	tryResumeLastFn = func(ctx context.Context) (*webcore.AuthSession, bool, error) {
		return nil, false, webcore.ErrCachedSessionExpired
	}
	loadCachedSessionFn = func(username string) (*webcore.AuthSession, bool, error) {
		t.Fatal("did not expect user-scoped cached-session load when apple-id is omitted")
		return nil, false, nil
	}
	loadLastCachedSessionFn = func() (*webcore.AuthSession, bool, error) {
		return &webcore.AuthSession{Client: cachedClient, UserEmail: "cached@example.com"}, true, nil
	}
	promptPasswordFn = func(ctx context.Context) (string, error) {
		t.Fatal("did not expect password prompt during silent auto-reauth")
		return "", nil
	}
	webLoginFn = func(ctx context.Context, creds webcore.LoginCredentials) (*webcore.AuthSession, error) {
		t.Fatal("did not expect fresh-login path during silent auto-reauth")
		return nil, nil
	}
	persistWebSessionFn = func(session *webcore.AuthSession) error {
		if session != expected {
			t.Fatal("expected auto-reauth session to be persisted")
		}
		return nil
	}
	webLoginWithClientFn = func(ctx context.Context, client *http.Client, creds webcore.LoginCredentials) (*webcore.AuthSession, error) {
		if client != cachedClient {
			t.Fatal("expected cached client to be reused for last-session auto-reauth")
		}
		if creds.Username != "cached@example.com" {
			t.Fatalf("expected stored email cached@example.com, got %q", creds.Username)
		}
		if creds.Password != "env-secret" {
			t.Fatalf("expected env password to be used, got %q", creds.Password)
		}
		return expected, nil
	}

	session, source, err := resolveSession(context.Background(), "", "", "")
	if err != nil {
		t.Fatalf("resolveSession returned error: %v", err)
	}
	if source != "auto-reauth" {
		t.Fatalf("expected source %q, got %q", "auto-reauth", source)
	}
	if session != expected {
		t.Fatal("expected auto-reauth session to be returned")
	}
	if got := notice.String(); got != "" {
		t.Fatalf("did not expect expired-session notice on successful auto-reauth, got %q", got)
	}
}

func TestResolveSessionAutoReauthFallsBackToFreshLoginWhenCachedClientFails(t *testing.T) {
	origTryResume := tryResumeSessionFn
	origTryResumeLast := tryResumeLastFn
	origLoadCachedSession := loadCachedSessionFn
	origLoadLastCachedSession := loadLastCachedSessionFn
	origPromptPassword := promptPasswordFn
	origWebLogin := webLoginFn
	origPersistWebSession := persistWebSessionFn
	origWebLoginWithClient := webLoginWithClientFn
	origExpiredWriter := sessionExpiredWriter
	t.Cleanup(func() {
		tryResumeSessionFn = origTryResume
		tryResumeLastFn = origTryResumeLast
		loadCachedSessionFn = origLoadCachedSession
		loadLastCachedSessionFn = origLoadLastCachedSession
		promptPasswordFn = origPromptPassword
		webLoginFn = origWebLogin
		persistWebSessionFn = origPersistWebSession
		webLoginWithClientFn = origWebLoginWithClient
		sessionExpiredWriter = origExpiredWriter
	})

	t.Setenv(webPasswordEnv, "env-secret")

	var notice bytes.Buffer
	sessionExpiredWriter = &notice

	cachedClient := &http.Client{}
	freshSession := &webcore.AuthSession{UserEmail: "user@example.com", ProviderID: 99}
	cachedTried := false
	freshTried := false

	tryResumeSessionFn = func(ctx context.Context, username string) (*webcore.AuthSession, bool, error) {
		return nil, false, webcore.ErrCachedSessionExpired
	}
	tryResumeLastFn = func(ctx context.Context) (*webcore.AuthSession, bool, error) {
		t.Fatal("did not expect last-session cache lookup when apple-id is provided")
		return nil, false, nil
	}
	loadCachedSessionFn = func(username string) (*webcore.AuthSession, bool, error) {
		return &webcore.AuthSession{Client: cachedClient, UserEmail: "user@example.com"}, true, nil
	}
	loadLastCachedSessionFn = func() (*webcore.AuthSession, bool, error) {
		t.Fatal("did not expect last cached-session load when apple-id is provided")
		return nil, false, nil
	}
	promptPasswordFn = func(ctx context.Context) (string, error) {
		t.Fatal("did not expect password prompt when env password is set")
		return "", nil
	}
	webLoginWithClientFn = func(ctx context.Context, client *http.Client, creds webcore.LoginCredentials) (*webcore.AuthSession, error) {
		cachedTried = true
		return nil, errors.New("cached client rejected")
	}
	webLoginFn = func(ctx context.Context, creds webcore.LoginCredentials) (*webcore.AuthSession, error) {
		freshTried = true
		if creds.Password != "env-secret" {
			t.Fatalf("expected env password to be reused for fresh fallback, got %q", creds.Password)
		}
		return freshSession, nil
	}
	persistWebSessionFn = func(session *webcore.AuthSession) error {
		if session != freshSession {
			t.Fatal("expected fresh fallback session to be persisted")
		}
		return nil
	}

	session, source, err := resolveSession(context.Background(), "user@example.com", "", "")
	if err != nil {
		t.Fatalf("resolveSession returned error: %v", err)
	}
	if !cachedTried {
		t.Fatal("expected cached-client auto-reauth attempt")
	}
	if !freshTried {
		t.Fatal("expected fresh-login fallback after cached-client failure")
	}
	if source != "fresh" {
		t.Fatalf("expected source %q, got %q", "fresh", source)
	}
	if session != freshSession {
		t.Fatal("expected fresh fallback session to be returned")
	}
	if got := notice.String(); got != "Session expired.\n" {
		t.Fatalf("expected expired notice before fresh fallback, got %q", got)
	}
}

func TestResolveSessionAutoReauthIgnoresPersistFailure(t *testing.T) {
	origTryResume := tryResumeSessionFn
	origTryResumeLast := tryResumeLastFn
	origLoadCachedSession := loadCachedSessionFn
	origLoadLastCachedSession := loadLastCachedSessionFn
	origPromptPassword := promptPasswordFn
	origWebLogin := webLoginFn
	origPersistWebSession := persistWebSessionFn
	origWebLoginWithClient := webLoginWithClientFn
	origExpiredWriter := sessionExpiredWriter
	t.Cleanup(func() {
		tryResumeSessionFn = origTryResume
		tryResumeLastFn = origTryResumeLast
		loadCachedSessionFn = origLoadCachedSession
		loadLastCachedSessionFn = origLoadLastCachedSession
		promptPasswordFn = origPromptPassword
		webLoginFn = origWebLogin
		persistWebSessionFn = origPersistWebSession
		webLoginWithClientFn = origWebLoginWithClient
		sessionExpiredWriter = origExpiredWriter
	})

	t.Setenv(webPasswordEnv, "env-secret")

	var notice bytes.Buffer
	sessionExpiredWriter = &notice

	cachedClient := &http.Client{}
	expected := &webcore.AuthSession{Client: cachedClient, UserEmail: "user@example.com"}

	tryResumeSessionFn = func(ctx context.Context, username string) (*webcore.AuthSession, bool, error) {
		return nil, false, webcore.ErrCachedSessionExpired
	}
	tryResumeLastFn = func(ctx context.Context) (*webcore.AuthSession, bool, error) {
		t.Fatal("did not expect last-session cache lookup when apple-id is provided")
		return nil, false, nil
	}
	loadCachedSessionFn = func(username string) (*webcore.AuthSession, bool, error) {
		return &webcore.AuthSession{Client: cachedClient, UserEmail: "user@example.com"}, true, nil
	}
	loadLastCachedSessionFn = func() (*webcore.AuthSession, bool, error) {
		t.Fatal("did not expect last cached-session load when apple-id is provided")
		return nil, false, nil
	}
	promptPasswordFn = func(ctx context.Context) (string, error) {
		t.Fatal("did not expect password prompt during silent auto-reauth")
		return "", nil
	}
	webLoginFn = func(ctx context.Context, creds webcore.LoginCredentials) (*webcore.AuthSession, error) {
		t.Fatal("did not expect fresh-login fallback on successful cached-client auto-reauth")
		return nil, nil
	}
	webLoginWithClientFn = func(ctx context.Context, client *http.Client, creds webcore.LoginCredentials) (*webcore.AuthSession, error) {
		return expected, nil
	}
	persistWebSessionFn = func(session *webcore.AuthSession) error {
		return errors.New("keychain offline")
	}

	session, source, err := resolveSession(context.Background(), "user@example.com", "", "")
	if err != nil {
		t.Fatalf("resolveSession returned error: %v", err)
	}
	if source != "auto-reauth" {
		t.Fatalf("expected source %q, got %q", "auto-reauth", source)
	}
	if session != expected {
		t.Fatal("expected successful auto-reauth session to be returned")
	}
	if got := notice.String(); got != "" {
		t.Fatalf("did not expect expired-session notice on successful auto-reauth, got %q", got)
	}
}

func TestResolveSessionRequiresAppleIDToRefreshLegacyLastCachedSession(t *testing.T) {
	origTryResume := tryResumeSessionFn
	origTryResumeLast := tryResumeLastFn
	origLoadCachedSession := loadCachedSessionFn
	origLoadLastCachedSession := loadLastCachedSessionFn
	origPromptPassword := promptPasswordFn
	origWebLogin := webLoginFn
	origPersistWebSession := persistWebSessionFn
	origWebLoginWithClient := webLoginWithClientFn
	origExpiredWriter := sessionExpiredWriter
	t.Cleanup(func() {
		tryResumeSessionFn = origTryResume
		tryResumeLastFn = origTryResumeLast
		loadCachedSessionFn = origLoadCachedSession
		loadLastCachedSessionFn = origLoadLastCachedSession
		promptPasswordFn = origPromptPassword
		webLoginFn = origWebLogin
		persistWebSessionFn = origPersistWebSession
		webLoginWithClientFn = origWebLoginWithClient
		sessionExpiredWriter = origExpiredWriter
	})

	t.Setenv(webPasswordEnv, "env-secret")

	tryResumeSessionFn = func(ctx context.Context, username string) (*webcore.AuthSession, bool, error) {
		t.Fatal("did not expect user-scoped cache lookup when apple-id is omitted")
		return nil, false, nil
	}
	tryResumeLastFn = func(ctx context.Context) (*webcore.AuthSession, bool, error) {
		return nil, false, webcore.ErrCachedSessionExpired
	}
	loadCachedSessionFn = func(username string) (*webcore.AuthSession, bool, error) {
		t.Fatal("did not expect user-scoped cached-session load when apple-id is omitted")
		return nil, false, nil
	}
	loadLastCachedSessionFn = func() (*webcore.AuthSession, bool, error) {
		return &webcore.AuthSession{Client: &http.Client{}}, true, nil
	}
	promptPasswordFn = func(ctx context.Context) (string, error) {
		t.Fatal("did not expect password prompt during legacy cache detection")
		return "", nil
	}
	webLoginFn = func(ctx context.Context, creds webcore.LoginCredentials) (*webcore.AuthSession, error) {
		t.Fatal("did not expect fresh-login path for legacy cache compatibility error")
		return nil, nil
	}
	webLoginWithClientFn = func(ctx context.Context, client *http.Client, creds webcore.LoginCredentials) (*webcore.AuthSession, error) {
		t.Fatal("did not expect cached-client auto-reauth without stored apple id metadata")
		return nil, nil
	}
	persistWebSessionFn = func(session *webcore.AuthSession) error {
		t.Fatal("did not expect session persist during legacy cache compatibility error")
		return nil
	}

	var stderr bytes.Buffer
	origStderr := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe error: %v", err)
	}
	os.Stderr = w
	done := make(chan struct{})
	go func() {
		_, _ = io.Copy(&stderr, r)
		close(done)
	}()

	_, _, runErr := resolveSession(context.Background(), "", "", "")

	_ = w.Close()
	os.Stderr = origStderr
	<-done
	_ = r.Close()

	if !errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("expected ErrHelp, got %v", runErr)
	}
	if !strings.Contains(stderr.String(), "predates stored Apple ID metadata") {
		t.Fatalf("expected legacy-cache guidance, got %q", stderr.String())
	}
}

func TestResolveSessionReturnsPromptCancellationWithoutUsageFallback(t *testing.T) {
	origTryResume := tryResumeSessionFn
	origTryResumeLast := tryResumeLastFn
	origPromptPassword := promptPasswordFn
	origReadPassword := termReadPasswordFn
	t.Cleanup(func() {
		tryResumeSessionFn = origTryResume
		tryResumeLastFn = origTryResumeLast
		promptPasswordFn = origPromptPassword
		termReadPasswordFn = origReadPassword
	})

	t.Setenv(webPasswordEnv, "")

	tryResumeSessionFn = func(ctx context.Context, username string) (*webcore.AuthSession, bool, error) {
		return nil, false, nil
	}
	tryResumeLastFn = func(ctx context.Context) (*webcore.AuthSession, bool, error) {
		t.Fatal("did not expect last-session cache lookup when apple-id is provided")
		return nil, false, nil
	}
	termReadPasswordFn = func(fd int) ([]byte, error) {
		return nil, errors.New("tty closed")
	}
	promptPasswordFn = func(ctx context.Context) (string, error) {
		return readPasswordFromTerminalFD(ctx, &bytes.Buffer{})
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, _, err := resolveSession(ctx, "user@example.com", "", "")
	if err == nil {
		t.Fatal("expected prompt cancellation error")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context cancellation, got %v", err)
	}
	if errors.Is(err, flag.ErrHelp) {
		t.Fatalf("did not expect usage error for prompt cancellation: %v", err)
	}
	if strings.Contains(err.Error(), "password is required") {
		t.Fatalf("did not expect password-required fallback, got %v", err)
	}
}

func TestWebAuthLoginReportsInvalidCredentialMessage(t *testing.T) {
	origTryResume := tryResumeSessionFn
	origTryResumeLast := tryResumeLastFn
	origWebLogin := webLoginFn
	t.Cleanup(func() {
		tryResumeSessionFn = origTryResume
		tryResumeLastFn = origTryResumeLast
		webLoginFn = origWebLogin
	})

	t.Setenv(webPasswordEnv, "secret")

	tryResumeSessionFn = func(ctx context.Context, username string) (*webcore.AuthSession, bool, error) {
		if username != "user@example.com" {
			t.Fatalf("expected username user@example.com, got %q", username)
		}
		return nil, false, nil
	}
	tryResumeLastFn = func(ctx context.Context) (*webcore.AuthSession, bool, error) {
		t.Fatal("did not expect last-session cache lookup when apple-id is provided")
		return nil, false, nil
	}
	webLoginFn = func(ctx context.Context, creds webcore.LoginCredentials) (*webcore.AuthSession, error) {
		if creds.Username != "user@example.com" {
			t.Fatalf("expected login username user@example.com, got %q", creds.Username)
		}
		if creds.Password != "secret" {
			t.Fatalf("expected password from env to be used, got %q", creds.Password)
		}
		return nil, errors.New("srp login failed: signin complete failed: incorrect Apple Account email or password")
	}

	cmd := WebAuthLoginCommand()
	if err := cmd.FlagSet.Parse([]string{"--apple-id", "user@example.com"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	err := cmd.Exec(context.Background(), nil)
	if err == nil {
		t.Fatal("expected login error")
	}
	if got, want := err.Error(), "web auth login failed: srp login failed: signin complete failed: incorrect Apple Account email or password"; got != want {
		t.Fatalf("expected error %q, got %q", want, got)
	}
}
