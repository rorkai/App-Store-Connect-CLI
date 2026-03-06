package web

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"strings"
	"testing"

	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

func TestReadPasswordFromInput(t *testing.T) {
	origPromptPassword := promptPasswordFn
	t.Cleanup(func() {
		promptPasswordFn = origPromptPassword
	})

	t.Run("uses environment variable before prompt fallback", func(t *testing.T) {
		t.Setenv(webPasswordEnv, " env-password ")
		promptPasswordFn = func() (string, error) {
			t.Fatal("did not expect prompt fallback when env password is set")
			return "", nil
		}

		password, err := readPasswordFromInput()
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
		promptPasswordFn = func() (string, error) {
			called = true
			return " prompted-password ", nil
		}

		password, err := readPasswordFromInput()
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

		password, err := readPasswordFromTerminalFD(0, &prompt)
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

		_, err := readPasswordFromTerminalFD(0, &bytes.Buffer{})
		if err == nil {
			t.Fatal("expected read failure")
		}
		if !strings.Contains(err.Error(), "failed to read password") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
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
	promptPasswordFn = func() (string, error) {
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
	origPromptPassword := promptPasswordFn
	origWebLogin := webLoginFn
	origExpiredWriter := sessionExpiredWriter
	t.Cleanup(func() {
		tryResumeSessionFn = origTryResume
		tryResumeLastFn = origTryResumeLast
		promptPasswordFn = origPromptPassword
		webLoginFn = origWebLogin
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
	promptPasswordFn = func() (string, error) {
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
