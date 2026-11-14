package web

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/creack/pty"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/appleauth"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
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
		if password != " env-password " {
			t.Fatalf("expected env password %q, got %q", " env-password ", password)
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
		if password != " prompted-password " {
			t.Fatalf("expected prompted password %q, got %q", " prompted-password ", password)
		}
	})

	t.Run("treats whitespace-only env password as missing", func(t *testing.T) {
		t.Setenv(webPasswordEnv, "   ")
		called := false
		promptPasswordFn = func(ctx context.Context) (string, error) {
			called = true
			return "prompted-password", nil
		}

		password, err := readPasswordFromInput(context.Background())
		if err != nil {
			t.Fatalf("readPasswordFromInput returned error: %v", err)
		}
		if !called {
			t.Fatal("expected prompt fallback when env password is whitespace-only")
		}
		if password != "prompted-password" {
			t.Fatalf("expected prompted password %q, got %q", "prompted-password", password)
		}
	})
}

func TestPromptPasswordInteractiveUsesControllingTTYWhenStdinIsNotTerminal(t *testing.T) {
	origOpenTTY := openTTYFn
	origIsTerminal := termIsTerminalFn
	t.Cleanup(func() {
		openTTYFn = origOpenTTY
		termIsTerminalFn = origIsTerminal
	})

	ptmx, tty, err := pty.Open()
	if err != nil {
		t.Fatalf("pty.Open() error: %v", err)
	}
	t.Cleanup(func() {
		_ = ptmx.Close()
		_ = tty.Close()
	})

	openTTYFn = func() (*os.File, error) {
		return tty, nil
	}
	termIsTerminalFn = func(fd int) bool {
		return false
	}

	promptSeen := make(chan error, 1)
	go func() {
		buf := make([]byte, 128)
		for {
			n, err := ptmx.Read(buf)
			if n > 0 && strings.Contains(string(buf[:n]), "Apple Account password:") {
				promptSeen <- nil
				return
			}
			if err != nil {
				promptSeen <- err
				return
			}
		}
	}()

	type promptResult struct {
		password string
		err      error
	}
	resultCh := make(chan promptResult, 1)
	go func() {
		password, err := promptPasswordInteractive(context.Background())
		resultCh <- promptResult{password: password, err: err}
	}()

	select {
	case err := <-promptSeen:
		if err != nil {
			t.Fatalf("failed waiting for password prompt: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for password prompt on controlling TTY")
	}

	if _, err := ptmx.Write([]byte("tty-secret\r")); err != nil {
		t.Fatalf("ptmx.Write() error: %v", err)
	}

	select {
	case result := <-resultCh:
		if result.err != nil {
			t.Fatalf("promptPasswordInteractive() error: %v", result.err)
		}
		if result.password != "tty-secret" {
			t.Fatalf("promptPasswordInteractive() = %q, want %q", result.password, "tty-secret")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for password prompt result")
	}
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

	promptResult := make(chan error, 1)
	go func() {
		buf := make([]byte, 128)
		for {
			n, err := ptmx.Read(buf)
			if n > 0 && strings.Contains(string(buf[:n]), "Apple Account password:") {
				promptResult <- nil
				return
			}
			if err != nil {
				promptResult <- err
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
	case err := <-promptResult:
		if err != nil {
			t.Fatalf("failed waiting for password prompt: %v", err)
		}
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

func TestReadPasswordFromTerminalReturnsPromptInterruptAfterContextCancel(t *testing.T) {
	ptmx, tty, err := pty.Open()
	if err != nil {
		t.Fatalf("pty.Open() error: %v", err)
	}
	t.Cleanup(func() {
		_ = ptmx.Close()
		_ = tty.Close()
	})

	ctx, cancel := context.WithCancel(context.Background())
	promptResult := make(chan error, 1)
	go func() {
		buf := make([]byte, 128)
		for {
			n, err := ptmx.Read(buf)
			if n > 0 && strings.Contains(string(buf[:n]), "Apple Account password:") {
				promptResult <- nil
				return
			}
			if err != nil {
				promptResult <- err
				return
			}
		}
	}()

	errCh := make(chan error, 1)
	go func() {
		_, err := readPasswordFromTerminal(ctx, tty, tty, false)
		errCh <- err
	}()

	select {
	case err := <-promptResult:
		if err != nil {
			t.Fatalf("failed waiting for password prompt: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for password prompt")
	}

	time.Sleep(50 * time.Millisecond)

	cancel()

	select {
	case err := <-errCh:
		if err == nil {
			t.Fatal("expected cancellation error")
		}
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected context cancellation, got %v", err)
		}
		if !strings.Contains(err.Error(), "password prompt interrupted") {
			t.Fatalf("expected interrupt-specific error, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for password prompt to return after context cancellation")
	}
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

func TestPromptTwoFactorCodeInteractiveWithoutTTYReturnsSupportedAutomationHint(t *testing.T) {
	origOpenTTY := openTTYFn
	origIsTerminal := termIsTerminalFn
	t.Cleanup(func() {
		openTTYFn = origOpenTTY
		termIsTerminalFn = origIsTerminal
	})

	openTTYFn = func() (*os.File, error) {
		return nil, errors.New("no tty")
	}
	termIsTerminalFn = func(fd int) bool {
		return false
	}

	_, err := promptTwoFactorCodeInteractive()
	if err == nil {
		t.Fatal("expected error when no interactive terminal is available")
	}
	if !strings.Contains(err.Error(), "--two-factor-code-command") {
		t.Fatalf("expected command hint in error, got %v", err)
	}
	if !strings.Contains(err.Error(), webTwoFactorCodeCommandEnv) {
		t.Fatalf("expected env hint in error, got %v", err)
	}
	want := "2fa required: run in a terminal for an interactive prompt, pass --two-factor-code-command, or set " + webTwoFactorCodeCommandEnv
	if err.Error() != want {
		t.Fatalf("error = %q, want %q", err.Error(), want)
	}
	if strings.Contains(err.Error(), "deprecated") || strings.Contains(err.Error(), "--two-factor-code ") {
		t.Fatalf("error still teaches the removed --two-factor-code alias: %v", err)
	}
}

func TestPromptTwoFactorCodeInteractiveUsesControllingTTYWhenStdinIsNotTerminal(t *testing.T) {
	origOpenTTY := openTTYFn
	origIsTerminal := termIsTerminalFn
	origReadPassword := termReadPasswordFn
	t.Cleanup(func() {
		openTTYFn = origOpenTTY
		termIsTerminalFn = origIsTerminal
		termReadPasswordFn = origReadPassword
	})

	ptmx, tty, err := pty.Open()
	if err != nil {
		t.Fatalf("pty.Open() error: %v", err)
	}
	t.Cleanup(func() {
		_ = ptmx.Close()
		_ = tty.Close()
	})

	openTTYFn = func() (*os.File, error) {
		return tty, nil
	}
	termIsTerminalFn = func(fd int) bool {
		return false
	}
	termReadPasswordFn = func(fd int) ([]byte, error) {
		if fd != int(tty.Fd()) {
			t.Fatalf("term.ReadPassword fd = %d, want controlling TTY fd %d", fd, tty.Fd())
		}
		return []byte(" 123456 "), nil
	}

	code, err := promptTwoFactorCodeInteractive()
	if err != nil {
		t.Fatalf("promptTwoFactorCodeInteractive() error: %v", err)
	}
	if code != "123456" {
		t.Fatalf("promptTwoFactorCodeInteractive() = %q, want %q", code, "123456")
	}
}

func TestReadTwoFactorCodeFromCommand(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell command coverage uses POSIX shell commands")
	}

	t.Run("trims stdout", func(t *testing.T) {
		t.Setenv("SHELL", "/bin/sh")

		code, err := readTwoFactorCodeFromCommand(context.Background(), "printf ' 123456 \\n'")
		if err != nil {
			t.Fatalf("readTwoFactorCodeFromCommand returned error: %v", err)
		}
		if code != "123456" {
			t.Fatalf("expected code %q, got %q", "123456", code)
		}
	})

	t.Run("rejects empty output", func(t *testing.T) {
		t.Setenv("SHELL", "/bin/sh")

		_, err := readTwoFactorCodeFromCommand(context.Background(), "printf '   \\n'")
		if err == nil {
			t.Fatal("expected error for empty output")
		}
		if !strings.Contains(err.Error(), "returned empty output") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("surfaces command stderr", func(t *testing.T) {
		t.Setenv("SHELL", "/bin/sh")

		_, err := readTwoFactorCodeFromCommand(context.Background(), "printf 'boom\\n' >&2; exit 9")
		if err == nil {
			t.Fatal("expected command failure")
		}
		if !strings.Contains(err.Error(), "boom") {
			t.Fatalf("expected stderr in error, got %v", err)
		}
	})

	t.Run("honors asc timeout override while waiting for helper output", func(t *testing.T) {
		t.Setenv("ASC_TIMEOUT", "50ms")

		_, err := readTwoFactorCodeFromCommand(context.Background(), "sleep 0.1; printf '123456\\n'")
		if err == nil {
			t.Fatal("expected timeout error")
		}
		if !strings.Contains(err.Error(), "interrupted") {
			t.Fatalf("expected interrupted error, got %v", err)
		}
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("expected deadline exceeded, got %v", err)
		}
	})

	t.Run("ignores shared request timeout while waiting for helper output", func(t *testing.T) {
		t.Setenv("SHELL", "/bin/sh")

		requestCtx, cancel := shared.ContextWithTimeoutDuration(context.Background(), 30*time.Millisecond)
		t.Cleanup(cancel)

		code, err := readTwoFactorCodeFromCommand(requestCtx, "sleep 0.1; printf '123456\\n'")
		if err != nil {
			t.Fatalf("readTwoFactorCodeFromCommand returned error: %v", err)
		}
		if code != "123456" {
			t.Fatalf("expected code %q, got %q", "123456", code)
		}
		if !errors.Is(requestCtx.Err(), context.DeadlineExceeded) {
			t.Fatalf("expected original request context to time out, got %v", requestCtx.Err())
		}
	})
}

func TestLoginWithOptionalTwoFactorPromptsWhenCodeMissing(t *testing.T) {
	origPrompt := promptTwoFactorCodeFn
	origReadCommand := readTwoFactorCodeFromCommandFn
	origLogin := webLoginFn
	origPrepare := prepareTwoFactorChallengeFn
	origEnsure := ensureTwoFactorCodeRequestedFn
	origSubmit := submitTwoFactorCodeFn
	t.Cleanup(func() {
		promptTwoFactorCodeFn = origPrompt
		readTwoFactorCodeFromCommandFn = origReadCommand
		webLoginFn = origLogin
		prepareTwoFactorChallengeFn = origPrepare
		ensureTwoFactorCodeRequestedFn = origEnsure
		submitTwoFactorCodeFn = origSubmit
	})

	var prompted bool
	var prepared bool
	var submittedCode string

	webLoginFn = func(ctx context.Context, creds webcore.LoginCredentials) (*webcore.AuthSession, error) {
		return &webcore.AuthSession{}, &webcore.TwoFactorRequiredError{}
	}
	prepareTwoFactorChallengeFn = func(ctx context.Context, session *webcore.AuthSession) (*webcore.TwoFactorChallenge, error) {
		prepared = true
		return &webcore.TwoFactorChallenge{Method: "trusted-device"}, nil
	}
	ensureTwoFactorCodeRequestedFn = func(ctx context.Context, session *webcore.AuthSession) (*webcore.TwoFactorChallenge, error) {
		t.Fatal("did not expect phone-code request for trusted-device challenge")
		return nil, nil
	}
	promptTwoFactorCodeFn = func() (string, error) {
		if !prepared {
			t.Fatal("expected 2fa challenge to be prepared before prompting")
		}
		prompted = true
		return "654321", nil
	}
	readTwoFactorCodeFromCommandFn = func(ctx context.Context, command string) (string, error) {
		t.Fatal("did not expect 2FA command when no command is configured")
		return "", nil
	}
	submitTwoFactorCodeFn = func(ctx context.Context, session *webcore.AuthSession, code string) error {
		submittedCode = code
		return nil
	}

	session, err := loginWithOptionalTwoFactor(context.Background(), "user@example.com", "secret", "", nil, nil)
	if err != nil {
		t.Fatalf("loginWithOptionalTwoFactor returned error: %v", err)
	}
	if session == nil {
		t.Fatal("expected non-nil session")
	}
	if !prepared {
		t.Fatal("expected 2fa challenge to be prepared")
	}
	if !prompted {
		t.Fatal("expected interactive prompt for missing 2fa code")
	}
	if submittedCode != "654321" {
		t.Fatalf("expected submitted code %q, got %q", "654321", submittedCode)
	}
}

func TestLoginWithOptionalTwoFactorUsesProvidedCodeWhenPresent(t *testing.T) {
	origLogin := webLoginFn
	origPrepare := prepareTwoFactorChallengeFn
	origSubmit := submitTwoFactorCodeFn
	t.Cleanup(func() {
		webLoginFn = origLogin
		prepareTwoFactorChallengeFn = origPrepare
		submitTwoFactorCodeFn = origSubmit
	})

	var submittedCode string

	webLoginFn = func(ctx context.Context, creds webcore.LoginCredentials) (*webcore.AuthSession, error) {
		return &webcore.AuthSession{}, &webcore.TwoFactorRequiredError{}
	}
	prepareTwoFactorChallengeFn = func(ctx context.Context, session *webcore.AuthSession) (*webcore.TwoFactorChallenge, error) {
		return &webcore.TwoFactorChallenge{Method: "trusted-device"}, nil
	}
	submitTwoFactorCodeFn = func(ctx context.Context, session *webcore.AuthSession, code string) error {
		submittedCode = code
		return nil
	}

	session, err := loginWithOptionalTwoFactor(context.Background(), "user@example.com", "secret", "654321", nil, nil)
	if err != nil {
		t.Fatalf("loginWithOptionalTwoFactor returned error: %v", err)
	}
	if session == nil {
		t.Fatal("expected non-nil session")
	}
	if submittedCode != "654321" {
		t.Fatalf("expected submitted code %q, got %q", "654321", submittedCode)
	}
}

func TestLoginWithOptionalTwoFactorReturnsPromptError(t *testing.T) {
	origPrompt := promptTwoFactorCodeFn
	origLogin := webLoginFn
	origPrepare := prepareTwoFactorChallengeFn
	origEnsure := ensureTwoFactorCodeRequestedFn
	origReadCommand := readTwoFactorCodeFromCommandFn
	origSubmit := submitTwoFactorCodeFn
	t.Cleanup(func() {
		promptTwoFactorCodeFn = origPrompt
		webLoginFn = origLogin
		prepareTwoFactorChallengeFn = origPrepare
		ensureTwoFactorCodeRequestedFn = origEnsure
		readTwoFactorCodeFromCommandFn = origReadCommand
		submitTwoFactorCodeFn = origSubmit
	})

	webLoginFn = func(ctx context.Context, creds webcore.LoginCredentials) (*webcore.AuthSession, error) {
		return &webcore.AuthSession{}, &webcore.TwoFactorRequiredError{}
	}
	prepareTwoFactorChallengeFn = func(ctx context.Context, session *webcore.AuthSession) (*webcore.TwoFactorChallenge, error) {
		return &webcore.TwoFactorChallenge{Method: "trusted-device"}, nil
	}
	ensureTwoFactorCodeRequestedFn = func(ctx context.Context, session *webcore.AuthSession) (*webcore.TwoFactorChallenge, error) {
		t.Fatal("did not expect phone-code request for trusted-device challenge")
		return nil, nil
	}
	readTwoFactorCodeFromCommandFn = func(ctx context.Context, command string) (string, error) {
		t.Fatal("did not expect 2FA command without configuration")
		return "", nil
	}
	promptTwoFactorCodeFn = func() (string, error) {
		return "", errors.New("2fa required: run in a terminal for an interactive prompt, pass --two-factor-code-command, or set " + webTwoFactorCodeCommandEnv)
	}
	submitTwoFactorCodeFn = func(ctx context.Context, session *webcore.AuthSession, code string) error {
		t.Fatal("did not expect submit when prompt fails")
		return nil
	}

	_, err := loginWithOptionalTwoFactor(context.Background(), "user@example.com", "secret", "", nil, nil, "")
	if err == nil {
		t.Fatal("expected error when prompt fails")
	}
	if !strings.Contains(err.Error(), "--two-factor-code-command") || !strings.Contains(err.Error(), webTwoFactorCodeCommandEnv) {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(err.Error(), "deprecated") || strings.Contains(err.Error(), "--two-factor-code ") {
		t.Fatalf("error still teaches the removed --two-factor-code alias: %v", err)
	}
}

func TestLoginWithOptionalTwoFactorUsesCommandWhenConfigured(t *testing.T) {
	origLogin := webLoginFn
	origPrepare := prepareTwoFactorChallengeFn
	origEnsure := ensureTwoFactorCodeRequestedFn
	origReadCommand := readTwoFactorCodeFromCommandFn
	origSubmit := submitTwoFactorCodeFn
	t.Cleanup(func() {
		webLoginFn = origLogin
		prepareTwoFactorChallengeFn = origPrepare
		ensureTwoFactorCodeRequestedFn = origEnsure
		readTwoFactorCodeFromCommandFn = origReadCommand
		submitTwoFactorCodeFn = origSubmit
	})

	var commandValue string
	var submittedCode string

	webLoginFn = func(ctx context.Context, creds webcore.LoginCredentials) (*webcore.AuthSession, error) {
		return &webcore.AuthSession{}, &webcore.TwoFactorRequiredError{}
	}
	prepareTwoFactorChallengeFn = func(ctx context.Context, session *webcore.AuthSession) (*webcore.TwoFactorChallenge, error) {
		return &webcore.TwoFactorChallenge{Method: "trusted-device"}, nil
	}
	ensureTwoFactorCodeRequestedFn = func(ctx context.Context, session *webcore.AuthSession) (*webcore.TwoFactorChallenge, error) {
		t.Fatal("did not expect phone-code request for trusted-device challenge")
		return nil, nil
	}
	readTwoFactorCodeFromCommandFn = func(ctx context.Context, command string) (string, error) {
		commandValue = command
		return "246810", nil
	}
	submitTwoFactorCodeFn = func(ctx context.Context, session *webcore.AuthSession, code string) error {
		submittedCode = code
		return nil
	}

	session, err := loginWithOptionalTwoFactor(context.Background(), "user@example.com", "secret", "", nil, nil, "osascript ./get-2fa.scpt")
	if err != nil {
		t.Fatalf("loginWithOptionalTwoFactor returned error: %v", err)
	}
	if session == nil {
		t.Fatal("expected non-nil session")
	}
	if commandValue != "osascript ./get-2fa.scpt" {
		t.Fatalf("expected command %q, got %q", "osascript ./get-2fa.scpt", commandValue)
	}
	if submittedCode != "246810" {
		t.Fatalf("expected submitted code %q, got %q", "246810", submittedCode)
	}
}

func TestLoginWithOptionalTwoFactorReappliesTimeoutAfterDelayedCommand(t *testing.T) {
	t.Setenv("ASC_TIMEOUT", "30ms")
	origLogin := webLoginFn
	origPrepare := prepareTwoFactorChallengeFn
	origEnsure := ensureTwoFactorCodeRequestedFn
	origReadCommand := readTwoFactorCodeFromCommandFn
	origSubmit := submitTwoFactorCodeFn
	t.Cleanup(func() {
		webLoginFn = origLogin
		prepareTwoFactorChallengeFn = origPrepare
		ensureTwoFactorCodeRequestedFn = origEnsure
		readTwoFactorCodeFromCommandFn = origReadCommand
		submitTwoFactorCodeFn = origSubmit
	})

	webLoginFn = func(ctx context.Context, creds webcore.LoginCredentials) (*webcore.AuthSession, error) {
		return &webcore.AuthSession{}, &webcore.TwoFactorRequiredError{}
	}
	prepareTwoFactorChallengeFn = func(ctx context.Context, session *webcore.AuthSession) (*webcore.TwoFactorChallenge, error) {
		return &webcore.TwoFactorChallenge{Method: "trusted-device"}, nil
	}
	ensureTwoFactorCodeRequestedFn = func(ctx context.Context, session *webcore.AuthSession) (*webcore.TwoFactorChallenge, error) {
		t.Fatal("did not expect phone-code request for trusted-device challenge")
		return nil, nil
	}
	readTwoFactorCodeFromCommandFn = func(ctx context.Context, command string) (string, error) {
		if command != "osascript ./get-2fa.scpt" {
			t.Fatalf("expected command %q, got %q", "osascript ./get-2fa.scpt", command)
		}
		time.Sleep(100 * time.Millisecond)
		if !errors.Is(ctx.Err(), context.DeadlineExceeded) {
			t.Fatalf("expected original request context to expire while waiting for 2FA code, got %v", ctx.Err())
		}
		return "246810", nil
	}
	submitTwoFactorCodeFn = func(ctx context.Context, session *webcore.AuthSession, code string) error {
		if code != "246810" {
			t.Fatalf("expected submitted code %q, got %q", "246810", code)
		}
		if ctx.Err() != nil {
			t.Fatalf("expected fresh verification context, got %v", ctx.Err())
		}
		if deadline, ok := ctx.Deadline(); !ok || time.Until(deadline) != 30*time.Millisecond {
			t.Fatalf("expected verification context to have a fresh 30ms deadline, got ok=%v deadline=%v", ok, deadline)
		}
		return nil
	}

	var session *webcore.AuthSession
	var err, requestErr error
	synctest.Test(t, func(*testing.T) {
		requestCtx, cancel := shared.ContextWithTimeoutDuration(context.Background(), 30*time.Millisecond)
		defer cancel()
		session, err = loginWithOptionalTwoFactor(requestCtx, "user@example.com", "secret", "", nil, nil, "osascript ./get-2fa.scpt")
		requestErr = requestCtx.Err()
	})
	if err != nil {
		t.Fatalf("loginWithOptionalTwoFactor returned error: %v", err)
	}
	if session == nil {
		t.Fatal("expected non-nil session")
	}
	if !errors.Is(requestErr, context.DeadlineExceeded) {
		t.Fatalf("expected original request context to time out, got %v", requestErr)
	}
}

func TestLoginWithOptionalTwoFactorRequestsPhoneCodeBeforePrompt(t *testing.T) {
	origPrompt := promptTwoFactorCodeFn
	origLogin := webLoginFn
	origPrepare := prepareTwoFactorChallengeFn
	origEnsure := ensureTwoFactorCodeRequestedFn
	origSubmit := submitTwoFactorCodeFn
	origStatusWriter := twoFactorStatusWriter
	t.Cleanup(func() {
		promptTwoFactorCodeFn = origPrompt
		webLoginFn = origLogin
		prepareTwoFactorChallengeFn = origPrepare
		ensureTwoFactorCodeRequestedFn = origEnsure
		submitTwoFactorCodeFn = origSubmit
		twoFactorStatusWriter = origStatusWriter
	})

	var (
		order        []string
		statusOutput bytes.Buffer
	)

	webLoginFn = func(ctx context.Context, creds webcore.LoginCredentials) (*webcore.AuthSession, error) {
		return &webcore.AuthSession{}, &webcore.TwoFactorRequiredError{}
	}
	prepareTwoFactorChallengeFn = func(ctx context.Context, session *webcore.AuthSession) (*webcore.TwoFactorChallenge, error) {
		order = append(order, "prepare")
		return &webcore.TwoFactorChallenge{Method: "phone", Destination: "+1 (•••) •••-••66"}, nil
	}
	ensureTwoFactorCodeRequestedFn = func(ctx context.Context, session *webcore.AuthSession) (*webcore.TwoFactorChallenge, error) {
		order = append(order, "ensure")
		return &webcore.TwoFactorChallenge{
			Method:      "phone",
			Destination: "+1 (•••) •••-••66",
			Requested:   true,
		}, nil
	}
	promptTwoFactorCodeFn = func() (string, error) {
		if got, want := order, []string{"prepare", "ensure"}; !reflect.DeepEqual(got, want) {
			t.Fatalf("expected prepare then ensure before prompting, got %v", got)
		}
		order = append(order, "prompt")
		return "654321", nil
	}
	submitTwoFactorCodeFn = func(ctx context.Context, session *webcore.AuthSession, code string) error {
		order = append(order, "submit")
		if code != "654321" {
			t.Fatalf("expected code 654321, got %q", code)
		}
		return nil
	}
	twoFactorStatusWriter = &statusOutput

	if _, err := loginWithOptionalTwoFactor(context.Background(), "user@example.com", "secret", "", nil, nil); err != nil {
		t.Fatalf("loginWithOptionalTwoFactor returned error: %v", err)
	}

	if got, want := order, []string{"prepare", "ensure", "prompt", "submit"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("expected order %v, got %v", want, got)
	}
	if output := statusOutput.String(); !strings.Contains(output, "Verification code sent to +1 (•••) •••-••66.") {
		t.Fatalf("expected delivery notice, got %q", output)
	}
}

func TestLoginWithOptionalTwoFactorSkipsPhoneRequestWhenCodeProvided(t *testing.T) {
	origPrompt := promptTwoFactorCodeFn
	origLogin := webLoginFn
	origPrepare := prepareTwoFactorChallengeFn
	origEnsure := ensureTwoFactorCodeRequestedFn
	origSubmit := submitTwoFactorCodeFn
	origStatusWriter := twoFactorStatusWriter
	t.Cleanup(func() {
		promptTwoFactorCodeFn = origPrompt
		webLoginFn = origLogin
		prepareTwoFactorChallengeFn = origPrepare
		ensureTwoFactorCodeRequestedFn = origEnsure
		submitTwoFactorCodeFn = origSubmit
		twoFactorStatusWriter = origStatusWriter
	})

	var statusOutput bytes.Buffer

	webLoginFn = func(ctx context.Context, creds webcore.LoginCredentials) (*webcore.AuthSession, error) {
		return &webcore.AuthSession{}, &webcore.TwoFactorRequiredError{}
	}
	prepareTwoFactorChallengeFn = func(ctx context.Context, session *webcore.AuthSession) (*webcore.TwoFactorChallenge, error) {
		return &webcore.TwoFactorChallenge{Method: "phone", Destination: "+1 (•••) •••-••66"}, nil
	}
	ensureTwoFactorCodeRequestedFn = func(ctx context.Context, session *webcore.AuthSession) (*webcore.TwoFactorChallenge, error) {
		t.Fatal("did not expect phone-code request when 2fa code is already provided")
		return nil, nil
	}
	promptTwoFactorCodeFn = func() (string, error) {
		t.Fatal("did not expect interactive prompt when 2fa code is already provided")
		return "", nil
	}
	submitTwoFactorCodeFn = func(ctx context.Context, session *webcore.AuthSession, code string) error {
		if code != "123456" {
			t.Fatalf("expected code 123456, got %q", code)
		}
		return nil
	}
	twoFactorStatusWriter = &statusOutput

	if _, err := loginWithOptionalTwoFactor(context.Background(), "user@example.com", "secret", "123456", nil, nil); err != nil {
		t.Fatalf("loginWithOptionalTwoFactor returned error: %v", err)
	}

	if output := statusOutput.String(); output != "" {
		t.Fatalf("expected no delivery notice when no request was made, got %q", output)
	}
}

func TestLoginWithOptionalTwoFactorRepromptsAfterFallbackPhoneRequest(t *testing.T) {
	origPrompt := promptTwoFactorCodeFn
	origLogin := webLoginFn
	origPrepare := prepareTwoFactorChallengeFn
	origEnsure := ensureTwoFactorCodeRequestedFn
	origSubmit := submitTwoFactorCodeFn
	origStatusWriter := twoFactorStatusWriter
	t.Cleanup(func() {
		promptTwoFactorCodeFn = origPrompt
		webLoginFn = origLogin
		prepareTwoFactorChallengeFn = origPrepare
		ensureTwoFactorCodeRequestedFn = origEnsure
		submitTwoFactorCodeFn = origSubmit
		twoFactorStatusWriter = origStatusWriter
	})

	var (
		promptCalls  int
		submitted    []string
		statusOutput bytes.Buffer
	)

	webLoginFn = func(ctx context.Context, creds webcore.LoginCredentials) (*webcore.AuthSession, error) {
		return &webcore.AuthSession{}, &webcore.TwoFactorRequiredError{}
	}
	prepareTwoFactorChallengeFn = func(ctx context.Context, session *webcore.AuthSession) (*webcore.TwoFactorChallenge, error) {
		return &webcore.TwoFactorChallenge{
			Method:                 "trusted-device",
			Destination:            "+1 (•••) •••-••66",
			PhoneFallbackAvailable: true,
		}, nil
	}
	ensureTwoFactorCodeRequestedFn = func(ctx context.Context, session *webcore.AuthSession) (*webcore.TwoFactorChallenge, error) {
		t.Fatal("did not expect upfront phone-code request for trusted-device challenge")
		return nil, nil
	}
	promptTwoFactorCodeFn = func() (string, error) {
		if promptCalls == 0 && !strings.Contains(statusOutput.String(), "Need a phone verification code?") {
			t.Fatalf("expected phone fallback guidance before the first prompt, got %q", statusOutput.String())
		}
		promptCalls++
		switch promptCalls {
		case 1:
			return "111111", nil
		case 2:
			return "222222", nil
		default:
			t.Fatalf("unexpected prompt count %d", promptCalls)
			return "", nil
		}
	}
	submitTwoFactorCodeFn = func(ctx context.Context, session *webcore.AuthSession, code string) error {
		submitted = append(submitted, code)
		if len(submitted) == 1 {
			return &appleauth.PhoneCodeRequestedError{Destination: "+1 (•••) •••-••66"}
		}
		if code != "222222" {
			t.Fatalf("expected second submitted code %q, got %q", "222222", code)
		}
		return nil
	}
	twoFactorStatusWriter = &statusOutput

	session, err := loginWithOptionalTwoFactor(context.Background(), "user@example.com", "secret", "", nil, nil)
	if err != nil {
		t.Fatalf("loginWithOptionalTwoFactor returned error: %v", err)
	}
	if session == nil {
		t.Fatal("expected non-nil session")
	}
	if promptCalls != 2 {
		t.Fatalf("expected two prompts, got %d", promptCalls)
	}
	if got, want := submitted, []string{"111111", "222222"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("expected submitted codes %v, got %v", want, got)
	}
	output := statusOutput.String()
	guidanceIndex := strings.Index(output, "Need a phone verification code? Enter an incorrect trusted-device code once; Apple will then deliver a verification code to +1 (•••) •••-••66.")
	if guidanceIndex < 0 {
		t.Fatalf("expected initial phone fallback guidance, got %q", output)
	}
	if strings.Contains(output, "SMS") {
		t.Fatalf("expected delivery-neutral guidance for voice-capable phone fallbacks, got %q", output)
	}
	deliveryIndex := strings.Index(output, "Verification code sent to +1 (•••) •••-••66.")
	if deliveryIndex < 0 {
		t.Fatalf("expected fallback delivery notice, got %q", output)
	}
	if guidanceIndex > deliveryIndex {
		t.Fatalf("expected phone fallback guidance before delivery notice, got %q", output)
	}
	if !strings.Contains(output, "Trusted-device verification was rejected. Enter the phone verification code that was just sent.") {
		t.Fatalf("expected fallback phone prompt guidance, got %q", output)
	}
}

func TestLoginWithOptionalTwoFactorRerunsCommandAfterFallbackPhoneRequest(t *testing.T) {
	origLogin := webLoginFn
	origPrepare := prepareTwoFactorChallengeFn
	origEnsure := ensureTwoFactorCodeRequestedFn
	origReadCommand := readTwoFactorCodeFromCommandFn
	origSubmit := submitTwoFactorCodeFn
	origStatusWriter := twoFactorStatusWriter
	t.Cleanup(func() {
		webLoginFn = origLogin
		prepareTwoFactorChallengeFn = origPrepare
		ensureTwoFactorCodeRequestedFn = origEnsure
		readTwoFactorCodeFromCommandFn = origReadCommand
		submitTwoFactorCodeFn = origSubmit
		twoFactorStatusWriter = origStatusWriter
	})

	var (
		commandCalls int
		submitted    []string
		statusOutput bytes.Buffer
	)

	webLoginFn = func(ctx context.Context, creds webcore.LoginCredentials) (*webcore.AuthSession, error) {
		return &webcore.AuthSession{}, &webcore.TwoFactorRequiredError{}
	}
	prepareTwoFactorChallengeFn = func(ctx context.Context, session *webcore.AuthSession) (*webcore.TwoFactorChallenge, error) {
		return &webcore.TwoFactorChallenge{
			Method:                 "trusted-device",
			Destination:            "+1 (•••) •••-••66",
			PhoneFallbackAvailable: true,
		}, nil
	}
	ensureTwoFactorCodeRequestedFn = func(ctx context.Context, session *webcore.AuthSession) (*webcore.TwoFactorChallenge, error) {
		t.Fatal("did not expect upfront phone-code request for trusted-device challenge")
		return nil, nil
	}
	readTwoFactorCodeFromCommandFn = func(ctx context.Context, command string) (string, error) {
		if command != "osascript ./get-2fa.scpt" {
			t.Fatalf("expected command %q, got %q", "osascript ./get-2fa.scpt", command)
		}
		commandCalls++
		switch commandCalls {
		case 1:
			return "111111", nil
		case 2:
			return "222222", nil
		default:
			t.Fatalf("unexpected command invocation %d", commandCalls)
			return "", nil
		}
	}
	submitTwoFactorCodeFn = func(ctx context.Context, session *webcore.AuthSession, code string) error {
		submitted = append(submitted, code)
		if len(submitted) == 1 {
			return &appleauth.PhoneCodeRequestedError{Destination: "+1 (•••) •••-••66"}
		}
		if code != "222222" {
			t.Fatalf("expected second submitted code %q, got %q", "222222", code)
		}
		return nil
	}
	twoFactorStatusWriter = &statusOutput

	session, err := loginWithOptionalTwoFactor(context.Background(), "user@example.com", "secret", "", nil, nil, "osascript ./get-2fa.scpt")
	if err != nil {
		t.Fatalf("loginWithOptionalTwoFactor returned error: %v", err)
	}
	if session == nil {
		t.Fatal("expected non-nil session")
	}
	if commandCalls != 2 {
		t.Fatalf("expected command to run twice, got %d", commandCalls)
	}
	if got, want := submitted, []string{"111111", "222222"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("expected submitted codes %v, got %v", want, got)
	}
	if output := statusOutput.String(); !strings.Contains(output, "Verification code sent to +1 (•••) •••-••66.") {
		t.Fatalf("expected fallback delivery notice, got %q", output)
	}
	if output := statusOutput.String(); !strings.Contains(output, "Trusted-device verification was rejected. Re-running the configured 2FA code command for the phone verification code.") {
		t.Fatalf("expected fallback command guidance, got %q", output)
	}
	if output := statusOutput.String(); strings.Contains(output, "Need a phone verification code?") {
		t.Fatalf("did not expect interactive phone fallback guidance for configured command, got %q", output)
	}
}

func TestLoginWithOptionalTwoFactorWrapsFallbackPhoneVerificationError(t *testing.T) {
	origPrompt := promptTwoFactorCodeFn
	origLogin := webLoginFn
	origPrepare := prepareTwoFactorChallengeFn
	origEnsure := ensureTwoFactorCodeRequestedFn
	origSubmit := submitTwoFactorCodeFn
	t.Cleanup(func() {
		promptTwoFactorCodeFn = origPrompt
		webLoginFn = origLogin
		prepareTwoFactorChallengeFn = origPrepare
		ensureTwoFactorCodeRequestedFn = origEnsure
		submitTwoFactorCodeFn = origSubmit
	})

	var submitted []string

	webLoginFn = func(ctx context.Context, creds webcore.LoginCredentials) (*webcore.AuthSession, error) {
		return &webcore.AuthSession{}, &webcore.TwoFactorRequiredError{}
	}
	prepareTwoFactorChallengeFn = func(ctx context.Context, session *webcore.AuthSession) (*webcore.TwoFactorChallenge, error) {
		return &webcore.TwoFactorChallenge{Method: "trusted-device"}, nil
	}
	ensureTwoFactorCodeRequestedFn = func(ctx context.Context, session *webcore.AuthSession) (*webcore.TwoFactorChallenge, error) {
		t.Fatal("did not expect upfront phone-code request for trusted-device challenge")
		return nil, nil
	}
	promptTwoFactorCodeFn = func() (string, error) {
		switch len(submitted) {
		case 0:
			return "111111", nil
		case 1:
			return "222222", nil
		default:
			t.Fatalf("unexpected prompt after %d submissions", len(submitted))
			return "", nil
		}
	}
	submitTwoFactorCodeFn = func(ctx context.Context, session *webcore.AuthSession, code string) error {
		submitted = append(submitted, code)
		if len(submitted) == 1 {
			return &appleauth.PhoneCodeRequestedError{Destination: "+1 (•••) •••-••66"}
		}
		return errors.New("apple rejected code")
	}

	_, err := loginWithOptionalTwoFactor(context.Background(), "user@example.com", "secret", "", nil, nil)
	if err == nil {
		t.Fatal("expected fallback verification error")
	}
	if !strings.Contains(err.Error(), "after switching to phone delivery") {
		t.Fatalf("expected fallback-specific verification error, got %v", err)
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

func TestResolveSessionFallsBackToFreshLoginWhenCacheLookupFailsBeforePrompt(t *testing.T) {
	origTryResume := tryResumeSessionFn
	origTryResumeLast := tryResumeLastFn
	origPromptPassword := promptPasswordFn
	origWebLogin := webLoginFn
	origCacheWarningWriter := sessionCacheWarningWriter
	t.Cleanup(func() {
		tryResumeSessionFn = origTryResume
		tryResumeLastFn = origTryResumeLast
		promptPasswordFn = origPromptPassword
		webLoginFn = origWebLogin
		sessionCacheWarningWriter = origCacheWarningWriter
	})

	var warning bytes.Buffer
	sessionCacheWarningWriter = &warning

	cacheErr := errors.New("cache permission denied")
	tryResumeSessionFn = func(ctx context.Context, username string) (*webcore.AuthSession, bool, error) {
		if username != "user@example.com" {
			t.Fatalf("expected username user@example.com, got %q", username)
		}
		return nil, false, cacheErr
	}
	tryResumeLastFn = func(ctx context.Context) (*webcore.AuthSession, bool, error) {
		t.Fatal("did not expect last-session cache lookup when apple-id is provided")
		return nil, false, nil
	}
	promptPasswordFn = func(ctx context.Context) (string, error) {
		t.Fatal("did not expect password prompt when a password is provided")
		return "", nil
	}
	webLoginFn = func(ctx context.Context, creds webcore.LoginCredentials) (*webcore.AuthSession, error) {
		if creds.Username != "user@example.com" {
			t.Fatalf("expected fresh login username %q, got %q", "user@example.com", creds.Username)
		}
		if creds.Password != "secret" {
			t.Fatalf("expected fresh login password %q, got %q", "secret", creds.Password)
		}
		return &webcore.AuthSession{UserEmail: creds.Username}, nil
	}

	session, source, err := resolveSession(context.Background(), "user@example.com", "secret", "")
	if err != nil {
		t.Fatalf("expected fresh login fallback, got %v", err)
	}
	if source != "fresh" {
		t.Fatalf("expected source %q, got %q", "fresh", source)
	}
	if session == nil || session.UserEmail != "user@example.com" {
		t.Fatalf("expected fresh login session for %q, got %+v", "user@example.com", session)
	}
	if got := warning.String(); !strings.Contains(got, cacheErr.Error()) || !strings.Contains(got, "continuing with fresh login") {
		t.Fatalf("expected cache warning to mention %q and fresh login fallback, got %q", cacheErr.Error(), got)
	}
}

func TestResolveWebSessionFallsBackToFreshLoginAfterPromptedAppleIDCacheError(t *testing.T) {
	origTryResume := tryResumeSessionFn
	origTryResumeLast := tryResumeLastFn
	origWebLogin := webLoginFn
	origCacheWarningWriter := sessionCacheWarningWriter
	t.Cleanup(func() {
		tryResumeSessionFn = origTryResume
		tryResumeLastFn = origTryResumeLast
		webLoginFn = origWebLogin
		sessionCacheWarningWriter = origCacheWarningWriter
	})

	var warning bytes.Buffer
	sessionCacheWarningWriter = &warning

	cacheErr := errors.New("cache metadata unreadable")
	tryResumeLastFn = func(ctx context.Context) (*webcore.AuthSession, bool, error) {
		return nil, false, nil
	}
	tryResumeSessionFn = func(ctx context.Context, username string) (*webcore.AuthSession, bool, error) {
		if username != "user@example.com" {
			t.Fatalf("expected prompted username user@example.com, got %q", username)
		}
		return nil, false, cacheErr
	}

	passwordResolved := false
	loggedIn := false
	webLoginFn = func(ctx context.Context, creds webcore.LoginCredentials) (*webcore.AuthSession, error) {
		loggedIn = true
		if creds.Username != "user@example.com" {
			t.Fatalf("expected prompted login username %q, got %q", "user@example.com", creds.Username)
		}
		if creds.Password != "secret" {
			t.Fatalf("expected prompted login password %q, got %q", "secret", creds.Password)
		}
		return &webcore.AuthSession{UserEmail: creds.Username}, nil
	}

	session, source, err := resolveWebSession(context.Background(), "", "", "", webSessionResolveOptions{
		promptAppleID: func(appleID *string) error {
			*appleID = "user@example.com"
			return nil
		},
		resolvePassword: func(ctx context.Context, password string) (string, error) {
			passwordResolved = true
			return "secret", nil
		},
	})
	if err != nil {
		t.Fatalf("expected prompted fresh login fallback, got %v", err)
	}
	if source != "fresh" {
		t.Fatalf("expected source %q, got %q", "fresh", source)
	}
	if passwordResolved {
		if !loggedIn {
			t.Fatal("expected fresh login after resolving password")
		}
	} else {
		t.Fatal("expected password resolution after prompted cache lookup failure")
	}
	if session == nil || session.UserEmail != "user@example.com" {
		t.Fatalf("expected prompted fresh session for %q, got %+v", "user@example.com", session)
	}
	if got := warning.String(); !strings.Contains(got, cacheErr.Error()) || !strings.Contains(got, "continuing with fresh login") {
		t.Fatalf("expected prompted cache warning to mention %q and fresh login fallback, got %q", cacheErr.Error(), got)
	}
}

func TestResolveWebSessionPrintsExpiredNoticeOnlyOnceAcrossPromptedLookup(t *testing.T) {
	origTryResume := tryResumeSessionFn
	origTryResumeLast := tryResumeLastFn
	origLoadCachedSession := loadCachedSessionFn
	origLoadLastCachedSession := loadLastCachedSessionFn
	origExpiredWriter := sessionExpiredWriter
	origWebLogin := webLoginFn
	t.Cleanup(func() {
		tryResumeSessionFn = origTryResume
		tryResumeLastFn = origTryResumeLast
		loadCachedSessionFn = origLoadCachedSession
		loadLastCachedSessionFn = origLoadLastCachedSession
		sessionExpiredWriter = origExpiredWriter
		webLoginFn = origWebLogin
	})

	var notice bytes.Buffer
	sessionExpiredWriter = &notice

	tryResumeLastFn = func(ctx context.Context) (*webcore.AuthSession, bool, error) {
		return nil, false, webcore.ErrCachedSessionExpired
	}
	tryResumeSessionFn = func(ctx context.Context, username string) (*webcore.AuthSession, bool, error) {
		if username != "user@example.com" {
			t.Fatalf("expected prompted username user@example.com, got %q", username)
		}
		return nil, false, webcore.ErrCachedSessionExpired
	}
	loadCachedSessionFn = func(username string) (*webcore.AuthSession, bool, error) {
		return nil, false, nil
	}
	loadLastCachedSessionFn = func() (*webcore.AuthSession, bool, error) {
		return nil, false, nil
	}
	webLoginFn = func(ctx context.Context, creds webcore.LoginCredentials) (*webcore.AuthSession, error) {
		if got := notice.String(); got != "Session expired.\n" {
			t.Fatalf("expected a single expired-session notice before login, got %q", got)
		}
		return &webcore.AuthSession{UserEmail: creds.Username}, nil
	}

	session, source, err := resolveWebSession(context.Background(), "", "secret", "", webSessionResolveOptions{
		promptAppleID: func(appleID *string) error {
			*appleID = "user@example.com"
			return nil
		},
		resolvePassword: func(ctx context.Context, password string) (string, error) {
			return "secret", nil
		},
	})
	if err != nil {
		t.Fatalf("resolveWebSession returned error: %v", err)
	}
	if source != "fresh" {
		t.Fatalf("expected source %q, got %q", "fresh", source)
	}
	if session == nil {
		t.Fatal("expected fresh login session")
	}
	if got := notice.String(); got != "Session expired.\n" {
		t.Fatalf("expected a single expired-session notice, got %q", got)
	}
}

func TestResolveSessionWhitespaceOnlyPasswordFallsBackToEnv(t *testing.T) {
	origTryResume := tryResumeSessionFn
	origTryResumeLast := tryResumeLastFn
	origWebLogin := webLoginFn
	t.Cleanup(func() {
		tryResumeSessionFn = origTryResume
		tryResumeLastFn = origTryResumeLast
		webLoginFn = origWebLogin
	})

	t.Setenv(webPasswordEnv, "env-secret")

	tryResumeSessionFn = func(ctx context.Context, username string) (*webcore.AuthSession, bool, error) {
		return nil, false, nil
	}
	tryResumeLastFn = func(ctx context.Context) (*webcore.AuthSession, bool, error) {
		t.Fatal("did not expect last-session cache lookup when apple-id is provided")
		return nil, false, nil
	}

	var received webcore.LoginCredentials
	webLoginFn = func(ctx context.Context, creds webcore.LoginCredentials) (*webcore.AuthSession, error) {
		received = creds
		return &webcore.AuthSession{UserEmail: creds.Username}, nil
	}

	session, source, err := resolveSession(context.Background(), "user@example.com", "   ", "")
	if err != nil {
		t.Fatalf("resolveSession returned error: %v", err)
	}
	if source != "fresh" {
		t.Fatalf("expected source %q, got %q", "fresh", source)
	}
	if session == nil {
		t.Fatal("expected session")
	}
	if received.Password != "env-secret" {
		t.Fatalf("expected env password fallback %q, got %q", "env-secret", received.Password)
	}
}

func TestResolveSessionUsesTwoFactorCodeCommandEnvWhen2FARequired(t *testing.T) {
	origTryResume := tryResumeSessionFn
	origTryResumeLast := tryResumeLastFn
	origPromptPassword := promptPasswordFn
	origPrepare := prepareTwoFactorChallengeFn
	origEnsure := ensureTwoFactorCodeRequestedFn
	origReadCommand := readTwoFactorCodeFromCommandFn
	origWebLogin := webLoginFn
	origSubmit := submitTwoFactorCodeFn
	t.Cleanup(func() {
		tryResumeSessionFn = origTryResume
		tryResumeLastFn = origTryResumeLast
		promptPasswordFn = origPromptPassword
		prepareTwoFactorChallengeFn = origPrepare
		ensureTwoFactorCodeRequestedFn = origEnsure
		readTwoFactorCodeFromCommandFn = origReadCommand
		webLoginFn = origWebLogin
		submitTwoFactorCodeFn = origSubmit
	})

	t.Setenv(webPasswordEnv, "")
	t.Setenv(webTwoFactorCodeCommandEnv, "osascript ./get-2fa.scpt")

	var commandValue string
	var submittedCode string

	tryResumeSessionFn = func(ctx context.Context, username string) (*webcore.AuthSession, bool, error) {
		return nil, false, nil
	}
	tryResumeLastFn = func(ctx context.Context) (*webcore.AuthSession, bool, error) {
		t.Fatal("did not expect last-session cache lookup when apple-id is provided")
		return nil, false, nil
	}
	promptPasswordFn = func(ctx context.Context) (string, error) {
		return "secret", nil
	}
	prepareTwoFactorChallengeFn = func(ctx context.Context, session *webcore.AuthSession) (*webcore.TwoFactorChallenge, error) {
		return &webcore.TwoFactorChallenge{Method: "trusted-device"}, nil
	}
	ensureTwoFactorCodeRequestedFn = func(ctx context.Context, session *webcore.AuthSession) (*webcore.TwoFactorChallenge, error) {
		t.Fatal("did not expect phone-code request for trusted-device challenge")
		return nil, nil
	}
	readTwoFactorCodeFromCommandFn = func(ctx context.Context, command string) (string, error) {
		commandValue = command
		return "135790", nil
	}
	webLoginFn = func(ctx context.Context, creds webcore.LoginCredentials) (*webcore.AuthSession, error) {
		return &webcore.AuthSession{UserEmail: "user@example.com"}, &webcore.TwoFactorRequiredError{}
	}
	submitTwoFactorCodeFn = func(ctx context.Context, session *webcore.AuthSession, code string) error {
		submittedCode = code
		return nil
	}

	session, source, err := resolveSession(context.Background(), "user@example.com", "", "", "")
	if err != nil {
		t.Fatalf("resolveSession returned error: %v", err)
	}
	if source != "fresh" {
		t.Fatalf("expected source %q, got %q", "fresh", source)
	}
	if session == nil {
		t.Fatal("expected non-nil session")
	}
	if commandValue != "osascript ./get-2fa.scpt" {
		t.Fatalf("expected command %q, got %q", "osascript ./get-2fa.scpt", commandValue)
	}
	if submittedCode != "135790" {
		t.Fatalf("expected submitted code %q, got %q", "135790", submittedCode)
	}
}

func TestResolveSessionPromptsForTwoFactorCodeWhen2FARequiredWithoutCommand(t *testing.T) {
	origTryResume := tryResumeSessionFn
	origTryResumeLast := tryResumeLastFn
	origPromptPassword := promptPasswordFn
	origPromptTwoFactor := promptTwoFactorCodeFn
	origPrepare := prepareTwoFactorChallengeFn
	origEnsure := ensureTwoFactorCodeRequestedFn
	origReadCommand := readTwoFactorCodeFromCommandFn
	origWebLogin := webLoginFn
	origSubmit := submitTwoFactorCodeFn
	t.Cleanup(func() {
		tryResumeSessionFn = origTryResume
		tryResumeLastFn = origTryResumeLast
		promptPasswordFn = origPromptPassword
		promptTwoFactorCodeFn = origPromptTwoFactor
		prepareTwoFactorChallengeFn = origPrepare
		ensureTwoFactorCodeRequestedFn = origEnsure
		readTwoFactorCodeFromCommandFn = origReadCommand
		webLoginFn = origWebLogin
		submitTwoFactorCodeFn = origSubmit
	})

	t.Setenv(webPasswordEnv, "")
	t.Setenv(webTwoFactorCodeCommandEnv, "")

	var prompted bool
	var submittedCode string

	tryResumeSessionFn = func(ctx context.Context, username string) (*webcore.AuthSession, bool, error) {
		return nil, false, nil
	}
	tryResumeLastFn = func(ctx context.Context) (*webcore.AuthSession, bool, error) {
		t.Fatal("did not expect last-session cache lookup when apple-id is provided")
		return nil, false, nil
	}
	promptPasswordFn = func(ctx context.Context) (string, error) {
		return "secret", nil
	}
	promptTwoFactorCodeFn = func() (string, error) {
		prompted = true
		return "135790", nil
	}
	prepareTwoFactorChallengeFn = func(ctx context.Context, session *webcore.AuthSession) (*webcore.TwoFactorChallenge, error) {
		return &webcore.TwoFactorChallenge{Method: "trusted-device"}, nil
	}
	ensureTwoFactorCodeRequestedFn = func(ctx context.Context, session *webcore.AuthSession) (*webcore.TwoFactorChallenge, error) {
		t.Fatal("did not expect phone-code request for trusted-device challenge")
		return nil, nil
	}
	readTwoFactorCodeFromCommandFn = func(ctx context.Context, command string) (string, error) {
		t.Fatal("did not expect 2FA command when no command is configured")
		return "", nil
	}
	webLoginFn = func(ctx context.Context, creds webcore.LoginCredentials) (*webcore.AuthSession, error) {
		return &webcore.AuthSession{UserEmail: "user@example.com"}, &webcore.TwoFactorRequiredError{}
	}
	submitTwoFactorCodeFn = func(ctx context.Context, session *webcore.AuthSession, code string) error {
		submittedCode = code
		return nil
	}

	session, source, err := resolveSession(context.Background(), "user@example.com", "", "", "")
	if err != nil {
		t.Fatalf("resolveSession returned error: %v", err)
	}
	if source != "fresh" {
		t.Fatalf("expected source %q, got %q", "fresh", source)
	}
	if session == nil {
		t.Fatal("expected non-nil session")
	}
	if !prompted {
		t.Fatal("expected interactive 2FA prompt when no command is configured")
	}
	if submittedCode != "135790" {
		t.Fatalf("expected submitted code %q, got %q", "135790", submittedCode)
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

// A cached cookie jar can carry Apple's trusted-device cookie, so a sign-in
// without any 2FA code source must still try it, and stop only once Apple asks
// for a code, before the challenge is requested.
func TestResolveSessionWithoutTwoFactorSourceStillTriesCachedJar(t *testing.T) {
	for _, needsTwoFactor := range []bool{false, true} {
		t.Run(fmt.Sprintf("two-factor challenge %t", needsTwoFactor), func(t *testing.T) {
			origTryResume := tryResumeSessionFn
			origLoadCachedSession := loadCachedSessionFn
			origWebLogin := webLoginFn
			origWebLoginWithClient := webLoginWithClientFn
			origPersistWebSession := persistWebSessionFn
			origPrepare := prepareTwoFactorChallengeFn
			origPromptAvailable := twoFactorPromptAvailableFn
			origExpiredWriter := sessionExpiredWriter
			t.Cleanup(func() {
				tryResumeSessionFn = origTryResume
				loadCachedSessionFn = origLoadCachedSession
				webLoginFn = origWebLogin
				webLoginWithClientFn = origWebLoginWithClient
				persistWebSessionFn = origPersistWebSession
				prepareTwoFactorChallengeFn = origPrepare
				twoFactorPromptAvailableFn = origPromptAvailable
				sessionExpiredWriter = origExpiredWriter
			})

			t.Setenv(webPasswordEnv, "env-secret")
			t.Setenv(webTwoFactorCodeCommandEnv, "")
			sessionExpiredWriter = io.Discard
			twoFactorPromptAvailableFn = func() bool { return false }

			cachedClient := &http.Client{}
			trusted := &webcore.AuthSession{Client: cachedClient, UserEmail: "user@example.com"}
			tryResumeSessionFn = func(ctx context.Context, username string) (*webcore.AuthSession, bool, error) {
				return nil, false, webcore.ErrCachedSessionExpired
			}
			loadCachedSessionFn = func(username string) (*webcore.AuthSession, bool, error) {
				return &webcore.AuthSession{Client: cachedClient, UserEmail: username}, true, nil
			}
			webLoginWithClientFn = func(ctx context.Context, client *http.Client, creds webcore.LoginCredentials) (*webcore.AuthSession, error) {
				if needsTwoFactor {
					return &webcore.AuthSession{Client: client}, &webcore.TwoFactorRequiredError{}
				}
				return trusted, nil
			}
			webLoginFn = func(ctx context.Context, creds webcore.LoginCredentials) (*webcore.AuthSession, error) {
				t.Fatal("did not expect a fresh sign-in")
				return nil, nil
			}
			persistWebSessionFn = func(*webcore.AuthSession) error { return nil }
			prepareTwoFactorChallengeFn = func(ctx context.Context, session *webcore.AuthSession) (*webcore.TwoFactorChallenge, error) {
				t.Fatal("did not expect a 2FA challenge request without a code source")
				return nil, nil
			}

			session, _, err := resolveSession(context.Background(), "user@example.com", "", "")
			if !needsTwoFactor {
				if err != nil || session != trusted {
					t.Fatalf("resolveSession() = %v, %v; want the trusted cached-jar session", session, err)
				}
				return
			}
			if !errors.Is(err, errNoTwoFactorCodeSource) || !shared.IsValidationError(err) {
				t.Fatalf("resolveSession() error = %v, want validation error for the missing 2FA source", err)
			}
			if diagnostic, ok := shared.DiagnosticFromError(err); !ok || diagnostic.Code != shared.DiagnosticRequiredInputMissing {
				t.Fatalf("diagnostic = %+v, want %q", diagnostic, shared.DiagnosticRequiredInputMissing)
			}
		})
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

func TestResolveSessionAutoReauthDoesNotRetryFreshLoginOnInvalidCredentials(t *testing.T) {
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

	t.Setenv(webPasswordEnv, "wrong-secret")

	var notice bytes.Buffer
	sessionExpiredWriter = &notice

	cachedClient := &http.Client{}
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
		if client != cachedClient {
			t.Fatal("expected cached client to be reused for auto-reauth")
		}
		return nil, fmt.Errorf("srp login failed: %w", webcore.ErrInvalidAppleAccountCredentials)
	}
	webLoginFn = func(ctx context.Context, creds webcore.LoginCredentials) (*webcore.AuthSession, error) {
		freshTried = true
		return nil, nil
	}
	persistWebSessionFn = func(session *webcore.AuthSession) error {
		t.Fatal("did not expect session persist on invalid auto-reauth credentials")
		return nil
	}

	_, _, err := resolveSession(context.Background(), "user@example.com", "", "")
	if err == nil {
		t.Fatal("expected auto-reauth credential error")
	}
	if !errors.Is(err, webcore.ErrInvalidAppleAccountCredentials) {
		t.Fatalf("expected invalid credentials error, got %v", err)
	}
	if freshTried {
		t.Fatal("did not expect fresh-login retry after invalid auto-reauth credentials")
	}
	if got := notice.String(); got != "" {
		t.Fatalf("did not expect expired-session notice when auto-reauth returns invalid credentials, got %q", got)
	}
}

func TestResolveSessionAutoReauthWarnsOnPersistFailure(t *testing.T) {
	origTryResume := tryResumeSessionFn
	origTryResumeLast := tryResumeLastFn
	origLoadCachedSession := loadCachedSessionFn
	origLoadLastCachedSession := loadLastCachedSessionFn
	origPromptPassword := promptPasswordFn
	origWebLogin := webLoginFn
	origPersistWebSession := persistWebSessionFn
	origWebLoginWithClient := webLoginWithClientFn
	origExpiredWriter := sessionExpiredWriter
	origCacheWarningWriter := sessionCacheWarningWriter
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
		sessionCacheWarningWriter = origCacheWarningWriter
	})

	t.Setenv(webPasswordEnv, "env-secret")

	var notice bytes.Buffer
	sessionExpiredWriter = &notice
	var cacheWarning bytes.Buffer
	sessionCacheWarningWriter = &cacheWarning

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
	if got := cacheWarning.String(); !strings.Contains(got, "keychain offline") || !strings.Contains(got, "may need to sign in again") {
		t.Fatalf("expected cache write warning, got %q", got)
	}
}

func TestResolveSessionAutoReauthRestoresCachedProvider(t *testing.T) {
	origTryResume := tryResumeSessionFn
	origLoadCachedSession := loadCachedSessionFn
	origWebLogin := webLoginFn
	origPersistWebSession := persistWebSessionFn
	origWebLoginWithClient := webLoginWithClientFn
	origSelectProvider := selectWebProviderFn
	origExpiredWriter := sessionExpiredWriter
	t.Cleanup(func() {
		tryResumeSessionFn = origTryResume
		loadCachedSessionFn = origLoadCachedSession
		webLoginFn = origWebLogin
		persistWebSessionFn = origPersistWebSession
		webLoginWithClientFn = origWebLoginWithClient
		selectWebProviderFn = origSelectProvider
		sessionExpiredWriter = origExpiredWriter
	})
	t.Setenv(webPasswordEnv, "env-secret")
	sessionExpiredWriter = io.Discard

	cachedClient := &http.Client{}
	tryResumeSessionFn = func(ctx context.Context, username string) (*webcore.AuthSession, bool, error) {
		return nil, false, webcore.ErrCachedSessionExpired
	}
	loadCachedSessionFn = func(username string) (*webcore.AuthSession, bool, error) {
		return &webcore.AuthSession{Client: cachedClient, UserEmail: "user@example.com", ProviderID: 42, DeveloperTeamID: "TEAM42"}, true, nil
	}
	webLoginFn = func(ctx context.Context, creds webcore.LoginCredentials) (*webcore.AuthSession, error) {
		t.Fatal("did not expect fresh login")
		return nil, nil
	}
	webLoginWithClientFn = func(ctx context.Context, client *http.Client, creds webcore.LoginCredentials) (*webcore.AuthSession, error) {
		return &webcore.AuthSession{Client: cachedClient, UserEmail: "user@example.com", ProviderID: 7}, nil
	}
	var persisted []webcore.AuthSession
	persistWebSessionFn = func(session *webcore.AuthSession) error {
		persisted = append(persisted, webcore.AuthSession{ProviderID: session.ProviderID, DeveloperTeamID: session.DeveloperTeamID})
		return nil
	}

	t.Run("restores provider and team", func(t *testing.T) {
		persisted = nil
		selectWebProviderFn = func(ctx context.Context, session *webcore.AuthSession, selection webcore.ProviderSelection) error {
			if selection.ProviderID != 42 {
				t.Fatalf("expected cached provider 42 to be selected, got %+v", selection)
			}
			session.ProviderID = selection.ProviderID
			session.DeveloperTeamID = ""
			return nil
		}
		session, _, err := resolveSession(context.Background(), "user@example.com", "", "")
		if err != nil {
			t.Fatalf("resolveSession returned error: %v", err)
		}
		if session.ProviderID != 42 || session.DeveloperTeamID != "TEAM42" {
			t.Fatalf("expected provider 42 and team TEAM42, got %d and %q", session.ProviderID, session.DeveloperTeamID)
		}
		if len(persisted) != 1 || persisted[0].ProviderID != 42 || persisted[0].DeveloperTeamID != "TEAM42" {
			t.Fatalf("expected the restored session to be persisted, got %+v", persisted)
		}
	})

	t.Run("fails instead of continuing on another provider", func(t *testing.T) {
		persisted = nil
		selectWebProviderFn = func(ctx context.Context, session *webcore.AuthSession, selection webcore.ProviderSelection) error {
			return errors.New("provider not available")
		}
		_, _, err := resolveSession(context.Background(), "user@example.com", "", "")
		if err == nil || !strings.Contains(err.Error(), "provider 42") || !strings.Contains(err.Error(), "--provider-id") {
			t.Fatalf("expected provider restore error naming provider 42 and --provider-id, got %v", err)
		}
		if len(persisted) != 0 {
			t.Fatalf("did not expect the default-provider session to be persisted, got %+v", persisted)
		}
	})

	t.Run("restores provider on the fresh-login fallback", func(t *testing.T) {
		origLoginWithClient := webLoginWithClientFn
		origWebLogin := webLoginFn
		t.Cleanup(func() {
			webLoginWithClientFn = origLoginWithClient
			webLoginFn = origWebLogin
		})
		persisted = nil
		webLoginWithClientFn = func(ctx context.Context, client *http.Client, creds webcore.LoginCredentials) (*webcore.AuthSession, error) {
			return nil, errors.New("cached client rejected")
		}
		webLoginFn = func(ctx context.Context, creds webcore.LoginCredentials) (*webcore.AuthSession, error) {
			return &webcore.AuthSession{Client: &http.Client{}, UserEmail: "user@example.com", ProviderID: 7}, nil
		}
		selectWebProviderFn = func(ctx context.Context, session *webcore.AuthSession, selection webcore.ProviderSelection) error {
			session.ProviderID = selection.ProviderID
			session.DeveloperTeamID = ""
			return nil
		}
		session, source, err := resolveSession(context.Background(), "user@example.com", "", "")
		if err != nil {
			t.Fatalf("resolveSession returned error: %v", err)
		}
		if source != "fresh" || session.ProviderID != 42 || session.DeveloperTeamID != "TEAM42" {
			t.Fatalf("expected fresh session on provider 42 with team TEAM42, got %q, %d, %q", source, session.ProviderID, session.DeveloperTeamID)
		}
		if len(persisted) != 1 || persisted[0].ProviderID != 42 || persisted[0].DeveloperTeamID != "TEAM42" {
			t.Fatalf("expected the restored fresh session to be persisted, got %+v", persisted)
		}
	})

	t.Run("explicit provider selection skips restore", func(t *testing.T) {
		selectWebProviderFn = func(ctx context.Context, session *webcore.AuthSession, selection webcore.ProviderSelection) error {
			t.Fatal("did not expect the cached provider to be restored")
			return nil
		}
		if _, _, err := resolveSessionWithoutPersist(context.Background(), "user@example.com", "", ""); err != nil {
			t.Fatalf("resolveSessionWithoutPersist returned error: %v", err)
		}
	})
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
	t.Cleanup(func() {
		tryResumeSessionFn = origTryResume
		tryResumeLastFn = origTryResumeLast
		promptPasswordFn = origPromptPassword
	})

	t.Setenv(webPasswordEnv, "")

	tryResumeSessionFn = func(ctx context.Context, username string) (*webcore.AuthSession, bool, error) {
		return nil, false, nil
	}
	tryResumeLastFn = func(ctx context.Context) (*webcore.AuthSession, bool, error) {
		t.Fatal("did not expect last-session cache lookup when apple-id is provided")
		return nil, false, nil
	}
	promptPasswordFn = func(ctx context.Context) (string, error) {
		return "", fmt.Errorf("password prompt interrupted: %w", ctx.Err())
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

// Root-caused 2026-08-29: after 2FA succeeded, auto-reauth reused the expired
// cached cookie jar, the App Store Connect session bootstrap returned 401, and
// the CLI aborted with a misleading "2fa verification failed" message instead of
// discarding the stale jar and retrying a fresh login once.
func TestResolveSessionRetriesFreshLoginAfterStaleSessionBootstrap401(t *testing.T) {
	origTryResume := tryResumeSessionFn
	origTryResumeLast := tryResumeLastFn
	origLoadCachedSession := loadCachedSessionFn
	origLoadLastCachedSession := loadLastCachedSessionFn
	origPromptPassword := promptPasswordFn
	origPromptTwoFactor := promptTwoFactorCodeFn
	origReadCommand := readTwoFactorCodeFromCommandFn
	origPrepare := prepareTwoFactorChallengeFn
	origSubmit := submitTwoFactorCodeFn
	origWebLogin := webLoginFn
	origWebLoginWithClient := webLoginWithClientFn
	origPersistWebSession := persistWebSessionFn
	origDeleteWebSession := deleteWebSessionFn
	origDeleteStaleWebSession := deleteStaleWebSessionFn
	origExpiredWriter := sessionExpiredWriter
	origStatusWriter := twoFactorStatusWriter
	t.Cleanup(func() {
		tryResumeSessionFn = origTryResume
		tryResumeLastFn = origTryResumeLast
		loadCachedSessionFn = origLoadCachedSession
		loadLastCachedSessionFn = origLoadLastCachedSession
		promptPasswordFn = origPromptPassword
		promptTwoFactorCodeFn = origPromptTwoFactor
		readTwoFactorCodeFromCommandFn = origReadCommand
		prepareTwoFactorChallengeFn = origPrepare
		submitTwoFactorCodeFn = origSubmit
		webLoginFn = origWebLogin
		webLoginWithClientFn = origWebLoginWithClient
		persistWebSessionFn = origPersistWebSession
		deleteWebSessionFn = origDeleteWebSession
		deleteStaleWebSessionFn = origDeleteStaleWebSession
		sessionExpiredWriter = origExpiredWriter
		twoFactorStatusWriter = origStatusWriter
	})

	t.Setenv(webPasswordEnv, "env-secret")
	t.Setenv(webTwoFactorCodeCommandEnv, "")

	var notice bytes.Buffer
	sessionExpiredWriter = &notice
	twoFactorStatusWriter = io.Discard

	cachedClient := &http.Client{}
	staleSession := &webcore.AuthSession{}
	freshSession := &webcore.AuthSession{UserEmail: "user@example.com", ProviderID: 99}
	cachedAttempts := 0
	freshAttempts := 0
	persisted := 0
	var deletedAppleIDs []string

	deleteStaleWebSessionFn = func(appleID string, _ *webcore.AuthSession) (bool, error) {
		deletedAppleIDs = append(deletedAppleIDs, appleID)
		return true, nil
	}

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
	prepareTwoFactorChallengeFn = func(ctx context.Context, session *webcore.AuthSession) (*webcore.TwoFactorChallenge, error) {
		return &webcore.TwoFactorChallenge{Method: "trusted-device"}, nil
	}
	promptTwoFactorCodeFn = func() (string, error) {
		return "654321", nil
	}
	readTwoFactorCodeFromCommandFn = func(ctx context.Context, command string) (string, error) {
		t.Fatal("did not expect a 2fa code command when none is configured")
		return "", nil
	}
	submitTwoFactorCodeFn = func(ctx context.Context, session *webcore.AuthSession, code string) error {
		if session == staleSession {
			return &webcore.TwoFactorFinalizationError{Status: http.StatusUnauthorized}
		}
		return nil
	}
	webLoginWithClientFn = func(ctx context.Context, client *http.Client, creds webcore.LoginCredentials) (*webcore.AuthSession, error) {
		cachedAttempts++
		if client != cachedClient {
			t.Fatal("expected the cached cookie jar to be reused for auto-reauth")
		}
		return staleSession, &webcore.TwoFactorRequiredError{}
	}
	webLoginFn = func(ctx context.Context, creds webcore.LoginCredentials) (*webcore.AuthSession, error) {
		freshAttempts++
		if creds.Password != "env-secret" {
			t.Fatalf("expected the resolved password to be reused for the fresh retry, got %q", creds.Password)
		}
		return freshSession, nil
	}
	persistWebSessionFn = func(session *webcore.AuthSession) error {
		persisted++
		if session != freshSession {
			t.Fatal("expected only the fresh session to be persisted")
		}
		return nil
	}

	session, source, err := resolveSession(context.Background(), "user@example.com", "", "")
	if err != nil {
		t.Fatalf("resolveSession returned error: %v", err)
	}
	if cachedAttempts != 1 {
		t.Fatalf("expected exactly one cached-jar auto-reauth attempt, got %d", cachedAttempts)
	}
	if freshAttempts != 1 {
		t.Fatalf("expected exactly one fresh retry after the stale session bootstrap 401, got %d", freshAttempts)
	}
	if persisted != 1 {
		t.Fatalf("expected the fresh session to be persisted once, got %d", persisted)
	}
	if len(deletedAppleIDs) != 1 || deletedAppleIDs[0] != "user@example.com" {
		t.Fatalf("expected the proven-stale cached session to be discarded once before the retry, got %v", deletedAppleIDs)
	}
	if source != "fresh" {
		t.Fatalf("expected source %q, got %q", "fresh", source)
	}
	if session != freshSession {
		t.Fatal("expected the fresh session to be returned")
	}
	if got := notice.String(); got != "Session expired.\n" {
		t.Fatalf("expected expired notice before the fresh retry, got %q", got)
	}
}

func TestResolveSessionDoesNotRetryFreshLoginOnRejectedTwoFactorCode(t *testing.T) {
	origTryResume := tryResumeSessionFn
	origTryResumeLast := tryResumeLastFn
	origLoadCachedSession := loadCachedSessionFn
	origLoadLastCachedSession := loadLastCachedSessionFn
	origPromptPassword := promptPasswordFn
	origPromptTwoFactor := promptTwoFactorCodeFn
	origReadCommand := readTwoFactorCodeFromCommandFn
	origPrepare := prepareTwoFactorChallengeFn
	origSubmit := submitTwoFactorCodeFn
	origWebLogin := webLoginFn
	origWebLoginWithClient := webLoginWithClientFn
	origExpiredWriter := sessionExpiredWriter
	origStatusWriter := twoFactorStatusWriter
	t.Cleanup(func() {
		tryResumeSessionFn = origTryResume
		tryResumeLastFn = origTryResumeLast
		loadCachedSessionFn = origLoadCachedSession
		loadLastCachedSessionFn = origLoadLastCachedSession
		promptPasswordFn = origPromptPassword
		promptTwoFactorCodeFn = origPromptTwoFactor
		readTwoFactorCodeFromCommandFn = origReadCommand
		prepareTwoFactorChallengeFn = origPrepare
		submitTwoFactorCodeFn = origSubmit
		webLoginFn = origWebLogin
		webLoginWithClientFn = origWebLoginWithClient
		sessionExpiredWriter = origExpiredWriter
		twoFactorStatusWriter = origStatusWriter
	})

	t.Setenv(webPasswordEnv, "env-secret")
	t.Setenv(webTwoFactorCodeCommandEnv, "")
	sessionExpiredWriter = io.Discard
	twoFactorStatusWriter = io.Discard

	cachedClient := &http.Client{}

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
	prepareTwoFactorChallengeFn = func(ctx context.Context, session *webcore.AuthSession) (*webcore.TwoFactorChallenge, error) {
		return &webcore.TwoFactorChallenge{Method: "trusted-device"}, nil
	}
	promptTwoFactorCodeFn = func() (string, error) {
		return "654321", nil
	}
	readTwoFactorCodeFromCommandFn = func(ctx context.Context, command string) (string, error) {
		t.Fatal("did not expect a 2fa code command when none is configured")
		return "", nil
	}
	submitTwoFactorCodeFn = func(ctx context.Context, session *webcore.AuthSession, code string) error {
		return errors.New("trusted-device 2fa failed (status 400, codes=[-21669])")
	}
	webLoginWithClientFn = func(ctx context.Context, client *http.Client, creds webcore.LoginCredentials) (*webcore.AuthSession, error) {
		return &webcore.AuthSession{}, &webcore.TwoFactorRequiredError{}
	}
	webLoginFn = func(ctx context.Context, creds webcore.LoginCredentials) (*webcore.AuthSession, error) {
		t.Fatal("did not expect a fresh retry after a rejected 2fa code")
		return nil, nil
	}

	_, _, err := resolveSession(context.Background(), "user@example.com", "", "")
	if err == nil {
		t.Fatal("expected auto-reauth to fail on a rejected 2fa code")
	}
	if got := err.Error(); !strings.Contains(got, "2fa verification failed") {
		t.Fatalf("expected rejected-code wording, got %q", got)
	}
}

// A 403 after Apple accepted the code (for example, an account without App
// Store Connect access) fails the same way on a fresh jar, so retrying would
// only burn a second code.
func TestResolveSessionDoesNotRetryFreshLoginAfterTwoFactorFinalizationFailure(t *testing.T) {
	for _, prompted := range []bool{false, true} {
		t.Run(fmt.Sprintf("prompted=%t", prompted), func(t *testing.T) {
			restoreStaleSessionRetryHooks(t)
			if prompted {
				t.Setenv(webPasswordEnv, "")
				promptPasswordFn = func(context.Context) (string, error) { return "prompted-secret", nil }
			}
			loadCachedSessionFn = func(string) (*webcore.AuthSession, bool, error) {
				return &webcore.AuthSession{Client: &http.Client{}, UserEmail: "user@example.com"}, true, nil
			}
			codePrompts := 0
			promptTwoFactorCodeFn = func() (string, error) { codePrompts++; return "654321", nil }
			submitTwoFactorCodeFn = func(context.Context, *webcore.AuthSession, string) error {
				return &webcore.TwoFactorFinalizationError{Status: http.StatusForbidden}
			}
			webLoginWithClientFn = func(context.Context, *http.Client, webcore.LoginCredentials) (*webcore.AuthSession, error) {
				return &webcore.AuthSession{}, &webcore.TwoFactorRequiredError{}
			}
			webLoginFn = func(context.Context, webcore.LoginCredentials) (*webcore.AuthSession, error) {
				t.Fatal("did not expect a fresh sign-in after Apple accepted the code")
				return nil, nil
			}

			_, _, err := resolveSession(context.Background(), "user@example.com", "", "")
			if err == nil || !strings.Contains(err.Error(), "2fa finalization failed") {
				t.Fatalf("expected the finalization failure, got %v", err)
			}
			if codePrompts != 1 {
				t.Fatalf("expected one verification code prompt, got %d", codePrompts)
			}
		})
	}
}

func TestTwoFactorSubmitFailureDistinguishesFinalizationFromVerification(t *testing.T) {
	tests := []struct {
		name               string
		err                error
		afterPhoneDelivery bool
		want               string
	}{
		{
			name: "stale session bootstrap",
			err:  &webcore.TwoFactorFinalizationError{Status: http.StatusUnauthorized},
			want: "2fa finalization failed: session bootstrap returned status 401",
		},
		{
			name:               "stale session bootstrap after phone delivery",
			err:                &webcore.TwoFactorFinalizationError{Status: http.StatusForbidden},
			afterPhoneDelivery: true,
			want:               "2fa finalization failed after switching to phone delivery: session bootstrap returned status 403",
		},
		{
			name: "rejected code",
			err:  errors.New("trusted-device 2fa failed (status 400)"),
			want: "2fa verification failed: trusted-device 2fa failed (status 400)",
		},
		{
			name: "apple rejected trusted-device code",
			err:  fmt.Errorf("trusted-device 2fa failed (status 400): %w", webcore.ErrTwoFactorCodeRejected),
			want: "2fa verification failed: Apple rejected the verification code: trusted-device 2fa failed (status 400): two-factor code rejected",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := twoFactorSubmitFailure(tc.err, tc.afterPhoneDelivery, false)
			if got.Error() != tc.want {
				t.Fatalf("expected %q, got %q", tc.want, got.Error())
			}
			if !errors.Is(got, tc.err) {
				t.Fatal("expected the underlying error to remain unwrappable")
			}
		})
	}
}

// restoreStaleSessionRetryHooks saves and restores every package hook and writer
// touched by the stale-session fresh-retry tests so auth state cannot leak
// between tests.
func restoreStaleSessionRetryHooks(t *testing.T) {
	t.Helper()

	origTryResume := tryResumeSessionFn
	origTryResumeLast := tryResumeLastFn
	origLoadCachedSession := loadCachedSessionFn
	origLoadLastCachedSession := loadLastCachedSessionFn
	origPromptPassword := promptPasswordFn
	origPromptTwoFactor := promptTwoFactorCodeFn
	origReadCommand := readTwoFactorCodeFromCommandFn
	origPrepare := prepareTwoFactorChallengeFn
	origSubmit := submitTwoFactorCodeFn
	origWebLogin := webLoginFn
	origWebLoginWithClient := webLoginWithClientFn
	origPersistWebSession := persistWebSessionFn
	origDeleteWebSession := deleteWebSessionFn
	origDeleteStaleWebSession := deleteStaleWebSessionFn
	origOpenTTY := openTTYFn
	origIsTerminal := termIsTerminalFn
	origExpiredWriter := sessionExpiredWriter
	origStatusWriter := twoFactorStatusWriter
	origCacheWarningWriter := sessionCacheWarningWriter
	t.Cleanup(func() {
		tryResumeSessionFn = origTryResume
		tryResumeLastFn = origTryResumeLast
		loadCachedSessionFn = origLoadCachedSession
		loadLastCachedSessionFn = origLoadLastCachedSession
		promptPasswordFn = origPromptPassword
		promptTwoFactorCodeFn = origPromptTwoFactor
		readTwoFactorCodeFromCommandFn = origReadCommand
		prepareTwoFactorChallengeFn = origPrepare
		submitTwoFactorCodeFn = origSubmit
		webLoginFn = origWebLogin
		webLoginWithClientFn = origWebLoginWithClient
		persistWebSessionFn = origPersistWebSession
		deleteWebSessionFn = origDeleteWebSession
		deleteStaleWebSessionFn = origDeleteStaleWebSession
		openTTYFn = origOpenTTY
		termIsTerminalFn = origIsTerminal
		sessionExpiredWriter = origExpiredWriter
		twoFactorStatusWriter = origStatusWriter
		sessionCacheWarningWriter = origCacheWarningWriter
	})

	t.Setenv(webPasswordEnv, "env-secret")
	t.Setenv(webTwoFactorCodeCommandEnv, "")
	t.Setenv(webDontStorePasswordEnv, "1")

	sessionExpiredWriter = io.Discard
	twoFactorStatusWriter = io.Discard
	sessionCacheWarningWriter = io.Discard
	promptPasswordFn = func(ctx context.Context) (string, error) {
		t.Fatal("did not expect a password prompt when the env password is set")
		return "", nil
	}
	tryResumeLastFn = func(ctx context.Context) (*webcore.AuthSession, bool, error) {
		t.Fatal("did not expect a last-session cache lookup when apple-id is provided")
		return nil, false, nil
	}
	loadLastCachedSessionFn = func() (*webcore.AuthSession, bool, error) {
		t.Fatal("did not expect a last cached-session load when apple-id is provided")
		return nil, false, nil
	}
	tryResumeSessionFn = func(ctx context.Context, username string) (*webcore.AuthSession, bool, error) {
		return nil, false, webcore.ErrCachedSessionExpired
	}
	prepareTwoFactorChallengeFn = func(ctx context.Context, session *webcore.AuthSession) (*webcore.TwoFactorChallenge, error) {
		return &webcore.TwoFactorChallenge{Method: "trusted-device"}, nil
	}
	persistWebSessionFn = func(session *webcore.AuthSession) error { return nil }
	deleteStaleWebSessionFn = func(appleID string, _ *webcore.AuthSession) (bool, error) {
		t.Fatalf("did not expect the cached session to be deleted for %q", appleID)
		return true, nil
	}
	deleteStaleWebSessionFn = func(appleID string, _ *webcore.AuthSession) (bool, error) {
		t.Fatalf("did not expect the cached session to be deleted for %q", appleID)
		return false, nil
	}
}

// Apple consumes the submitted 2FA code before the stale cached jar fails the
// session bootstrap, so the fresh retry must obtain a new code from the
// configured command instead of resubmitting the literal --two-factor-code.
func TestResolveSessionObtainsNewTwoFactorCodeForFreshRetryAfterStaleBootstrap(t *testing.T) {
	restoreStaleSessionRetryHooks(t)
	t.Setenv(webTwoFactorCodeCommandEnv, "print-code")

	cachedClient := &http.Client{}
	staleSession := &webcore.AuthSession{}
	freshSession := &webcore.AuthSession{UserEmail: "user@example.com"}
	var submittedCodes []string
	var deletedAppleIDs []string
	commandCalls := 0

	deleteStaleWebSessionFn = func(appleID string, _ *webcore.AuthSession) (bool, error) {
		deletedAppleIDs = append(deletedAppleIDs, appleID)
		return true, nil
	}

	loadCachedSessionFn = func(username string) (*webcore.AuthSession, bool, error) {
		return &webcore.AuthSession{Client: cachedClient, UserEmail: "user@example.com"}, true, nil
	}
	promptTwoFactorCodeFn = func() (string, error) {
		t.Fatal("did not expect an interactive 2fa prompt when a code command is configured")
		return "", nil
	}
	readTwoFactorCodeFromCommandFn = func(ctx context.Context, command string) (string, error) {
		commandCalls++
		return "222222", nil
	}
	submitTwoFactorCodeFn = func(ctx context.Context, session *webcore.AuthSession, code string) error {
		submittedCodes = append(submittedCodes, code)
		if session == staleSession {
			return &webcore.TwoFactorFinalizationError{Status: http.StatusUnauthorized}
		}
		return nil
	}
	webLoginWithClientFn = func(ctx context.Context, client *http.Client, creds webcore.LoginCredentials) (*webcore.AuthSession, error) {
		return staleSession, &webcore.TwoFactorRequiredError{}
	}
	webLoginFn = func(ctx context.Context, creds webcore.LoginCredentials) (*webcore.AuthSession, error) {
		return freshSession, &webcore.TwoFactorRequiredError{}
	}

	session, source, err := resolveSession(context.Background(), "user@example.com", "", "111111")
	if err != nil {
		t.Fatalf("resolveSession returned error: %v", err)
	}
	if session != freshSession {
		t.Fatal("expected the fresh session to be returned")
	}
	if source != "fresh" {
		t.Fatalf("expected source %q, got %q", "fresh", source)
	}
	if commandCalls != 1 {
		t.Fatalf("expected the 2fa code command to run once for the fresh retry, got %d", commandCalls)
	}
	if len(deletedAppleIDs) != 1 || deletedAppleIDs[0] != "user@example.com" {
		t.Fatalf("expected the proven-stale cached session to be discarded once before the retry, got %v", deletedAppleIDs)
	}
	if len(submittedCodes) != 2 {
		t.Fatalf("expected two 2fa submissions, got %v", submittedCodes)
	}
	if submittedCodes[0] != "111111" {
		t.Fatalf("expected the supplied code on the cached-jar attempt, got %q", submittedCodes[0])
	}
	if submittedCodes[1] != "222222" {
		t.Fatalf("expected a newly obtained code on the fresh retry, got %q", submittedCodes[1])
	}
}

// shortenTwoFactorCodeRotationWait keeps the rotation wait bounded but instant
// so the tests exercise the polling loop without sleeping for a real TOTP
// window.
func shortenTwoFactorCodeRotationWait(t *testing.T) {
	t.Helper()

	origTimeout := webTwoFactorCodeRotationTimeout
	origInterval := webTwoFactorCodeRotationPollInterval
	t.Cleanup(func() {
		webTwoFactorCodeRotationTimeout = origTimeout
		webTwoFactorCodeRotationPollInterval = origInterval
	})
	webTwoFactorCodeRotationTimeout = 50 * time.Millisecond
	webTwoFactorCodeRotationPollInterval = time.Millisecond
}

// A TOTP-style code command keeps printing the same digits until its time
// window rolls over, so the stale-session retry cannot simply re-run it: Apple
// already consumed that value on the cached-jar attempt. The retry must poll
// the command until it returns a different code.
func TestResolveSessionWaitsForRotatedCommandCodeAfterStaleBootstrap(t *testing.T) {
	restoreStaleSessionRetryHooks(t)
	shortenTwoFactorCodeRotationWait(t)
	t.Setenv(webTwoFactorCodeCommandEnv, "print-code")

	cachedClient := &http.Client{}
	staleSession := &webcore.AuthSession{}
	freshSession := &webcore.AuthSession{UserEmail: "user@example.com"}
	commandCodes := []string{"111111", "111111", "222222"}
	commandCalls := 0
	var submittedCodes []string

	deleteStaleWebSessionFn = func(appleID string, _ *webcore.AuthSession) (bool, error) { return true, nil }
	loadCachedSessionFn = func(username string) (*webcore.AuthSession, bool, error) {
		return &webcore.AuthSession{Client: cachedClient, UserEmail: "user@example.com"}, true, nil
	}
	promptTwoFactorCodeFn = func() (string, error) {
		t.Fatal("did not expect an interactive 2fa prompt when a code command is configured")
		return "", nil
	}
	readTwoFactorCodeFromCommandFn = func(ctx context.Context, command string) (string, error) {
		if commandCalls >= len(commandCodes) {
			t.Fatalf("2fa code command ran %d times, more than the %d scripted results", commandCalls+1, len(commandCodes))
		}
		code := commandCodes[commandCalls]
		commandCalls++
		return code, nil
	}
	submitTwoFactorCodeFn = func(ctx context.Context, session *webcore.AuthSession, code string) error {
		submittedCodes = append(submittedCodes, code)
		if session == staleSession {
			return &webcore.TwoFactorFinalizationError{Status: http.StatusUnauthorized}
		}
		if code == commandCodes[0] {
			return errors.New("verification code has already been used")
		}
		return nil
	}
	webLoginWithClientFn = func(ctx context.Context, client *http.Client, creds webcore.LoginCredentials) (*webcore.AuthSession, error) {
		return staleSession, &webcore.TwoFactorRequiredError{}
	}
	webLoginFn = func(ctx context.Context, creds webcore.LoginCredentials) (*webcore.AuthSession, error) {
		return freshSession, &webcore.TwoFactorRequiredError{}
	}

	session, source, err := resolveSession(context.Background(), "user@example.com", "", "")
	if err != nil {
		t.Fatalf("resolveSession returned error: %v", err)
	}
	if session != freshSession {
		t.Fatal("expected the fresh session to be returned")
	}
	if source != "fresh" {
		t.Fatalf("expected source %q, got %q", "fresh", source)
	}
	if commandCalls != 3 {
		t.Fatalf("expected the 2fa code command to be polled until it rotated, got %d calls", commandCalls)
	}
	if len(submittedCodes) != 2 {
		t.Fatalf("expected two 2fa submissions, got %v", submittedCodes)
	}
	if submittedCodes[0] != "111111" {
		t.Fatalf("expected the command code on the cached-jar attempt, got %q", submittedCodes[0])
	}
	if submittedCodes[1] != "222222" {
		t.Fatalf("expected the rotated code on the fresh retry, got %q", submittedCodes[1])
	}
}

// When the configured command never rotates within the bounded wait there is no
// usable code left, so the retry must fail with the consumed-code guidance
// rather than resubmit the value Apple already burned.
func TestResolveSessionFailsWhenCommandCodeNeverRotatesAfterStaleBootstrap(t *testing.T) {
	restoreStaleSessionRetryHooks(t)
	shortenTwoFactorCodeRotationWait(t)
	t.Setenv(webTwoFactorCodeCommandEnv, "print-code")

	cachedClient := &http.Client{}
	staleSession := &webcore.AuthSession{}
	freshSession := &webcore.AuthSession{UserEmail: "user@example.com"}
	commandCalls := 0
	var submittedCodes []string

	deleteStaleWebSessionFn = func(appleID string, _ *webcore.AuthSession) (bool, error) { return true, nil }
	loadCachedSessionFn = func(username string) (*webcore.AuthSession, bool, error) {
		return &webcore.AuthSession{Client: cachedClient, UserEmail: "user@example.com"}, true, nil
	}
	promptTwoFactorCodeFn = func() (string, error) {
		t.Fatal("did not expect an interactive 2fa prompt when a code command is configured")
		return "", nil
	}
	readTwoFactorCodeFromCommandFn = func(ctx context.Context, command string) (string, error) {
		commandCalls++
		return "111111", nil
	}
	submitTwoFactorCodeFn = func(ctx context.Context, session *webcore.AuthSession, code string) error {
		submittedCodes = append(submittedCodes, code)
		if session == staleSession {
			return &webcore.TwoFactorFinalizationError{Status: http.StatusUnauthorized}
		}
		return nil
	}
	webLoginWithClientFn = func(ctx context.Context, client *http.Client, creds webcore.LoginCredentials) (*webcore.AuthSession, error) {
		return staleSession, &webcore.TwoFactorRequiredError{}
	}
	webLoginFn = func(ctx context.Context, creds webcore.LoginCredentials) (*webcore.AuthSession, error) {
		return freshSession, &webcore.TwoFactorRequiredError{}
	}
	persistWebSessionFn = func(session *webcore.AuthSession) error {
		t.Fatal("did not expect a session to be persisted when no new 2fa code is available")
		return nil
	}

	_, _, err := resolveSession(context.Background(), "user@example.com", "", "")
	if err == nil {
		t.Fatal("expected resolveSession to fail when the 2fa code command never rotates")
	}
	got := err.Error()
	if !strings.Contains(got, "already consumed") {
		t.Fatalf("expected the message to explain that the code was consumed, got %q", got)
	}
	if !strings.Contains(got, "two-factor code command") {
		t.Fatalf("expected the message to point at the 2fa code command, got %q", got)
	}
	if len(submittedCodes) != 1 || submittedCodes[0] != "111111" {
		t.Fatalf("expected only the cached-jar submission, got %v", submittedCodes)
	}
	if commandCalls < 2 {
		t.Fatalf("expected the command to be polled more than once before giving up, got %d", commandCalls)
	}
}

// Phone fallback asks the command for a second code after the trusted-device
// one is rejected. That rejected code was consumed too, so the baseline has to
// advance with every code the command hands over; comparing only against the
// code the cached attempt burned lets the just-rejected value through and
// resubmits it.
func TestResolveSessionWaitsForANewCodeOnPhoneFallbackAfterStaleBootstrap(t *testing.T) {
	restoreStaleSessionRetryHooks(t)
	shortenTwoFactorCodeRotationWait(t)
	t.Setenv(webTwoFactorCodeCommandEnv, "print-code")

	cachedClient := &http.Client{}
	staleSession := &webcore.AuthSession{}
	freshSession := &webcore.AuthSession{UserEmail: "user@example.com"}
	// The generator repeats within each window: the cached code, then the
	// rotated one that phone fallback rejects, then a genuinely new one.
	commandCodes := []string{"111111", "111111", "222222", "222222", "333333"}
	commandCalls := 0
	var submittedCodes []string

	deleteStaleWebSessionFn = func(appleID string, _ *webcore.AuthSession) (bool, error) { return true, nil }
	loadCachedSessionFn = func(username string) (*webcore.AuthSession, bool, error) {
		return &webcore.AuthSession{Client: cachedClient, UserEmail: "user@example.com"}, true, nil
	}
	promptTwoFactorCodeFn = func() (string, error) {
		t.Fatal("did not expect an interactive 2fa prompt when a code command is configured")
		return "", nil
	}
	readTwoFactorCodeFromCommandFn = func(ctx context.Context, command string) (string, error) {
		if commandCalls >= len(commandCodes) {
			t.Fatalf("2fa code command ran %d times, more than the %d scripted results", commandCalls+1, len(commandCodes))
		}
		code := commandCodes[commandCalls]
		commandCalls++
		return code, nil
	}
	submitTwoFactorCodeFn = func(ctx context.Context, session *webcore.AuthSession, code string) error {
		for _, seen := range submittedCodes {
			if seen == code {
				return fmt.Errorf("verification code %s has already been used", code)
			}
		}
		submittedCodes = append(submittedCodes, code)
		if session == staleSession {
			return &webcore.TwoFactorFinalizationError{Status: http.StatusUnauthorized}
		}
		if len(submittedCodes) == 2 {
			return &appleauth.PhoneCodeRequestedError{Destination: "+1 (•••) •••-••66"}
		}
		return nil
	}
	webLoginWithClientFn = func(ctx context.Context, client *http.Client, creds webcore.LoginCredentials) (*webcore.AuthSession, error) {
		return staleSession, &webcore.TwoFactorRequiredError{}
	}
	webLoginFn = func(ctx context.Context, creds webcore.LoginCredentials) (*webcore.AuthSession, error) {
		return freshSession, &webcore.TwoFactorRequiredError{}
	}

	session, source, err := resolveSession(context.Background(), "user@example.com", "", "")
	if err != nil {
		t.Fatalf("resolveSession returned error: %v", err)
	}
	if session != freshSession || source != "fresh" {
		t.Fatalf("expected the fresh session, got source %q", source)
	}
	if len(submittedCodes) != 3 {
		t.Fatalf("expected three distinct 2fa submissions, got %v", submittedCodes)
	}
	if submittedCodes[2] != "333333" {
		t.Fatalf("expected a code newer than the one phone fallback rejected, got %q", submittedCodes[2])
	}
	if commandCalls != len(commandCodes) {
		t.Fatalf("expected the command to be polled past each repeated code, got %d calls", commandCalls)
	}
}

// The discard has to be scoped to the jar this resolution actually loaded, so a
// valid session another process persisted during 2FA survives. A preserved
// entry is not a failure either, so it must not surface a warning.
func TestResolveSessionScopesTheStaleDiscardToTheLoadedSession(t *testing.T) {
	restoreStaleSessionRetryHooks(t)

	var warnings bytes.Buffer
	sessionCacheWarningWriter = &warnings

	cachedClient := &http.Client{}
	cachedSession := &webcore.AuthSession{Client: cachedClient, UserEmail: "user@example.com"}
	staleSession := &webcore.AuthSession{}
	freshSession := &webcore.AuthSession{UserEmail: "user@example.com"}
	var discardedFor []*webcore.AuthSession
	discardCalls := 0

	deleteStaleWebSessionFn = func(appleID string, loaded *webcore.AuthSession) (bool, error) {
		discardCalls++
		discardedFor = append(discardedFor, loaded)
		// Stand in for a concurrent process having replaced the entry.
		return false, nil
	}
	loadCachedSessionFn = func(username string) (*webcore.AuthSession, bool, error) {
		return cachedSession, true, nil
	}
	promptTwoFactorCodeFn = func() (string, error) { return "654321", nil }
	submitTwoFactorCodeFn = func(ctx context.Context, session *webcore.AuthSession, code string) error {
		if session == staleSession {
			return &webcore.TwoFactorFinalizationError{Status: http.StatusUnauthorized}
		}
		return nil
	}
	webLoginWithClientFn = func(ctx context.Context, client *http.Client, creds webcore.LoginCredentials) (*webcore.AuthSession, error) {
		return staleSession, &webcore.TwoFactorRequiredError{}
	}
	webLoginFn = func(ctx context.Context, creds webcore.LoginCredentials) (*webcore.AuthSession, error) {
		return freshSession, &webcore.TwoFactorRequiredError{}
	}

	if _, _, err := resolveSession(context.Background(), "user@example.com", "", ""); err != nil {
		t.Fatalf("resolveSession returned error: %v", err)
	}
	if discardCalls != 1 {
		t.Fatalf("expected exactly one scoped discard, got %d", discardCalls)
	}
	if discardedFor[0] != cachedSession {
		t.Fatal("expected the discard to be scoped to the cached session this resolution loaded")
	}
	if got := warnings.String(); got != "" {
		t.Fatalf("did not expect a warning when a newer cached session was preserved, got %q", got)
	}
}

// The rotation budget deliberately outlives the login timeout, so by the time
// the window closes the login context has normally expired. Classifying that as
// caller cancellation would bury the actionable consumed-code guidance under a
// bare "context deadline exceeded". Cancellation has to be read from the untimed
// parent the budget is derived from, not the superseded login context.
func TestReadRotatedTwoFactorCodeReportsExhaustionAfterTheLoginTimeout(t *testing.T) {
	for _, tt := range []struct {
		name    string
		command func(ctx context.Context, command string) (string, error)
	}{
		{
			name: "command keeps returning the consumed code",
			command: func(ctx context.Context, command string) (string, error) {
				return "111111", nil
			},
		},
		{
			name: "command blocks until its context is done",
			command: func(ctx context.Context, command string) (string, error) {
				<-ctx.Done()
				return "", fmt.Errorf("2fa required: two-factor code command interrupted: %w", ctx.Err())
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			origRead := readTwoFactorCodeFromCommandFn
			t.Cleanup(func() { readTwoFactorCodeFromCommandFn = origRead })
			shortenTwoFactorCodeRotationWait(t)
			// The budget must outlive the login deadline, as it does in production.
			webTwoFactorCodeRotationTimeout = 80 * time.Millisecond
			readTwoFactorCodeFromCommandFn = tt.command

			loginCtx, cancel := shared.ContextWithTimeoutDuration(context.Background(), 10*time.Millisecond)
			defer cancel()

			_, err := readRotatedTwoFactorCodeFromCommand(loginCtx, "print-code", "111111")
			if err == nil {
				t.Fatal("expected the exhausted rotation budget to fail")
			}
			got := err.Error()
			if !strings.Contains(got, "already consumed") {
				t.Fatalf("expected the consumed-code guidance, got %q", got)
			}
			if strings.Contains(got, "interrupted") {
				t.Fatalf("expected rotation exhaustion not to be reported as cancellation, got %q", got)
			}
		})
	}
}

// A genuinely cancelled caller still has to be reported as an interruption
// rather than dressed up as an exhausted rotation budget.
func TestReadRotatedTwoFactorCodeReportsGenuineCallerCancellation(t *testing.T) {
	origRead := readTwoFactorCodeFromCommandFn
	t.Cleanup(func() { readTwoFactorCodeFromCommandFn = origRead })
	shortenTwoFactorCodeRotationWait(t)
	webTwoFactorCodeRotationTimeout = 10 * time.Second
	readTwoFactorCodeFromCommandFn = func(ctx context.Context, command string) (string, error) {
		return "111111", nil
	}

	var err error
	synctest.Test(t, func(*testing.T) {
		parent, cancelParent := context.WithCancel(context.Background())
		loginCtx, cancel := shared.ContextWithTimeoutDuration(parent, 5*time.Second)
		defer cancel()
		go func() {
			time.Sleep(20 * time.Millisecond)
			cancelParent()
		}()

		_, err = readRotatedTwoFactorCodeFromCommand(loginCtx, "print-code", "111111")
	})
	if err == nil {
		t.Fatal("expected the cancelled caller to fail")
	}
	if got := err.Error(); !strings.Contains(got, "interrupted") {
		t.Fatalf("expected a cancellation to be reported as an interruption, got %q", got)
	}
}

// The fresh retry raises a brand-new Apple challenge, so the code the failed
// attempt consumed is dead and a replacement has just been delivered. Recovery
// through the existing prompt is the point of the retry for an interactive
// operator, but they have to be told which code to type: the consumed digits
// are still on screen from the first prompt, and retyping them only earns an
// opaque rejection.
func TestResolveSessionExplainsConsumedCodeBeforeThePromptedRetry(t *testing.T) {
	restoreStaleSessionRetryHooks(t)

	var status bytes.Buffer
	twoFactorStatusWriter = &status

	cachedClient := &http.Client{}
	staleSession := &webcore.AuthSession{}
	freshSession := &webcore.AuthSession{UserEmail: "user@example.com"}
	promptCalls := 0
	var submittedCodes []string

	deleteStaleWebSessionFn = func(appleID string, _ *webcore.AuthSession) (bool, error) { return true, nil }
	loadCachedSessionFn = func(username string) (*webcore.AuthSession, bool, error) {
		return &webcore.AuthSession{Client: cachedClient, UserEmail: "user@example.com"}, true, nil
	}
	readTwoFactorCodeFromCommandFn = func(ctx context.Context, command string) (string, error) {
		t.Fatal("did not expect a 2fa code command when none is configured")
		return "", nil
	}
	promptTwoFactorCodeFn = func() (string, error) {
		promptCalls++
		if promptCalls == 1 {
			return "111111", nil
		}
		return "222222", nil
	}
	submitTwoFactorCodeFn = func(ctx context.Context, session *webcore.AuthSession, code string) error {
		submittedCodes = append(submittedCodes, code)
		if session == staleSession {
			return &webcore.TwoFactorFinalizationError{Status: http.StatusUnauthorized}
		}
		return nil
	}
	webLoginWithClientFn = func(ctx context.Context, client *http.Client, creds webcore.LoginCredentials) (*webcore.AuthSession, error) {
		return staleSession, &webcore.TwoFactorRequiredError{}
	}
	webLoginFn = func(ctx context.Context, creds webcore.LoginCredentials) (*webcore.AuthSession, error) {
		return freshSession, &webcore.TwoFactorRequiredError{}
	}

	session, source, err := resolveSession(context.Background(), "user@example.com", "", "")
	if err != nil {
		t.Fatalf("resolveSession returned error: %v", err)
	}
	if session != freshSession || source != "fresh" {
		t.Fatalf("expected the interactive retry to recover, got source %q", source)
	}
	if promptCalls != 2 {
		t.Fatalf("expected the retry to collect the replacement code Apple just sent, got %d prompts", promptCalls)
	}
	if len(submittedCodes) != 2 || submittedCodes[1] != "222222" {
		t.Fatalf("expected the replacement code on the fresh retry, got %v", submittedCodes)
	}
	notice := status.String()
	if !strings.Contains(notice, "consumed") {
		t.Fatalf("expected the operator to be told the previous code was consumed, got %q", notice)
	}
	if !strings.Contains(notice, "new code") {
		t.Fatalf("expected the operator to be pointed at the new code, got %q", notice)
	}
}

// Apple has sent nothing new until the fresh sign-in raises its own challenge.
func TestResolveSessionWithholdsConsumedCodeNoticeUntilTheFreshChallengeExists(t *testing.T) {
	restoreStaleSessionRetryHooks(t)

	var status bytes.Buffer
	twoFactorStatusWriter = &status
	staleSession := &webcore.AuthSession{}
	freshSession := &webcore.AuthSession{UserEmail: "user@example.com"}

	deleteStaleWebSessionFn = func(string, *webcore.AuthSession) (bool, error) { return true, nil }
	loadCachedSessionFn = func(string) (*webcore.AuthSession, bool, error) {
		return &webcore.AuthSession{Client: &http.Client{}, UserEmail: "user@example.com"}, true, nil
	}
	promptTwoFactorCodeFn = func() (string, error) { return "111111", nil }
	submitTwoFactorCodeFn = func(context.Context, *webcore.AuthSession, string) error {
		return &webcore.TwoFactorFinalizationError{Status: http.StatusUnauthorized}
	}
	webLoginWithClientFn = func(context.Context, *http.Client, webcore.LoginCredentials) (*webcore.AuthSession, error) {
		return staleSession, &webcore.TwoFactorRequiredError{}
	}
	webLoginFn = func(context.Context, webcore.LoginCredentials) (*webcore.AuthSession, error) {
		return freshSession, &webcore.TwoFactorRequiredError{}
	}
	prepareTwoFactorChallengeFn = func(_ context.Context, session *webcore.AuthSession) (*webcore.TwoFactorChallenge, error) {
		if session == freshSession {
			return nil, errors.New("auth options unavailable")
		}
		return &webcore.TwoFactorChallenge{Method: "trusted-device"}, nil
	}

	if _, _, err := resolveSession(context.Background(), "user@example.com", "", ""); err == nil {
		t.Fatal("expected the fresh challenge setup failure")
	}
	if notice := status.String(); strings.Contains(notice, "new code") {
		t.Fatalf("notice written before the fresh challenge existed: %q", notice)
	}
}

// The notice is for the operator at the prompt. A configured code command
// fetches the replacement itself, so it must not be told to type anything.
func TestResolveSessionOmitsConsumedCodeNoticeWhenCommandSuppliesTheReplacement(t *testing.T) {
	restoreStaleSessionRetryHooks(t)
	t.Setenv(webTwoFactorCodeCommandEnv, "print-code")

	var status bytes.Buffer
	twoFactorStatusWriter = &status

	cachedClient := &http.Client{}
	staleSession := &webcore.AuthSession{}
	freshSession := &webcore.AuthSession{UserEmail: "user@example.com"}
	commandCalls := 0

	deleteStaleWebSessionFn = func(appleID string, _ *webcore.AuthSession) (bool, error) { return true, nil }
	loadCachedSessionFn = func(username string) (*webcore.AuthSession, bool, error) {
		return &webcore.AuthSession{Client: cachedClient, UserEmail: "user@example.com"}, true, nil
	}
	promptTwoFactorCodeFn = func() (string, error) {
		t.Fatal("did not expect an interactive 2fa prompt when a code command is configured")
		return "", nil
	}
	readTwoFactorCodeFromCommandFn = func(ctx context.Context, command string) (string, error) {
		commandCalls++
		if commandCalls == 1 {
			return "111111", nil
		}
		return "222222", nil
	}
	submitTwoFactorCodeFn = func(ctx context.Context, session *webcore.AuthSession, code string) error {
		if session == staleSession {
			return &webcore.TwoFactorFinalizationError{Status: http.StatusUnauthorized}
		}
		return nil
	}
	webLoginWithClientFn = func(ctx context.Context, client *http.Client, creds webcore.LoginCredentials) (*webcore.AuthSession, error) {
		return staleSession, &webcore.TwoFactorRequiredError{}
	}
	webLoginFn = func(ctx context.Context, creds webcore.LoginCredentials) (*webcore.AuthSession, error) {
		return freshSession, &webcore.TwoFactorRequiredError{}
	}

	if _, _, err := resolveSession(context.Background(), "user@example.com", "", ""); err != nil {
		t.Fatalf("resolveSession returned error: %v", err)
	}
	if notice := status.String(); strings.Contains(notice, "consumed") {
		t.Fatalf("did not expect a type-the-new-code notice for a command-sourced retry, got %q", notice)
	}
}

// The rotation budget has to reach the command itself. A 2FA code command runs
// with its own generous timeout (60s by default, more under an ASC_TIMEOUT
// override), so a slow or blocking one would otherwise stretch the retry far
// past the advertised wait before the deadline is ever consulted.
func TestResolveSessionBoundsBlockingCommandByRotationDeadlineAfterStaleBootstrap(t *testing.T) {
	restoreStaleSessionRetryHooks(t)
	shortenTwoFactorCodeRotationWait(t)
	t.Setenv(webTwoFactorCodeCommandEnv, "print-code")

	cachedClient := &http.Client{}
	staleSession := &webcore.AuthSession{}
	freshSession := &webcore.AuthSession{UserEmail: "user@example.com"}
	commandCalls := 0
	var commandDeadlines []bool

	deleteStaleWebSessionFn = func(appleID string, _ *webcore.AuthSession) (bool, error) { return true, nil }
	loadCachedSessionFn = func(username string) (*webcore.AuthSession, bool, error) {
		return &webcore.AuthSession{Client: cachedClient, UserEmail: "user@example.com"}, true, nil
	}
	promptTwoFactorCodeFn = func() (string, error) {
		t.Fatal("did not expect an interactive 2fa prompt when a code command is configured")
		return "", nil
	}
	readTwoFactorCodeFromCommandFn = func(ctx context.Context, command string) (string, error) {
		commandCalls++
		if commandCalls == 1 {
			return "111111", nil
		}
		// Stand in for a command that blocks: it only returns once its own
		// context is done, which happens solely if the rotation deadline is
		// carried into it.
		_, hasDeadline := ctx.Deadline()
		commandDeadlines = append(commandDeadlines, hasDeadline)
		<-ctx.Done()
		return "", fmt.Errorf("2fa required: two-factor code command interrupted: %w", ctx.Err())
	}
	submitTwoFactorCodeFn = func(ctx context.Context, session *webcore.AuthSession, code string) error {
		if session == staleSession {
			return &webcore.TwoFactorFinalizationError{Status: http.StatusUnauthorized}
		}
		t.Fatalf("did not expect a 2fa submission on the fresh retry, got %q", code)
		return nil
	}
	webLoginWithClientFn = func(ctx context.Context, client *http.Client, creds webcore.LoginCredentials) (*webcore.AuthSession, error) {
		return staleSession, &webcore.TwoFactorRequiredError{}
	}
	webLoginFn = func(ctx context.Context, creds webcore.LoginCredentials) (*webcore.AuthSession, error) {
		return freshSession, &webcore.TwoFactorRequiredError{}
	}
	persistWebSessionFn = func(session *webcore.AuthSession) error {
		t.Fatal("did not expect a session to be persisted when no new 2fa code is available")
		return nil
	}

	started := time.Now()
	_, _, err := resolveSession(context.Background(), "user@example.com", "", "")
	elapsed := time.Since(started)
	if err == nil {
		t.Fatal("expected resolveSession to fail when the 2fa code command blocks past the rotation deadline")
	}
	if len(commandDeadlines) == 0 || !commandDeadlines[0] {
		t.Fatal("expected the rotation deadline to be carried into the 2fa code command context")
	}
	if elapsed > 5*time.Second {
		t.Fatalf("expected the blocked command to be bounded by the rotation deadline, took %s", elapsed)
	}
	got := err.Error()
	if !strings.Contains(got, "already consumed") {
		t.Fatalf("expected the message to explain that the code was consumed, got %q", got)
	}
	if !strings.Contains(got, "two-factor code command") {
		t.Fatalf("expected the message to point at the 2fa code command, got %q", got)
	}
	if strings.Contains(got, "interrupted") {
		t.Fatalf("expected the rotation budget to be explained rather than surfaced as an interruption, got %q", got)
	}
}

// A jar that fails the post-2FA session bootstrap is proven unusable even when
// the fresh retry itself fails, so the cached entry must be discarded at
// detection time. Relying on a successful fresh login to overwrite it would let
// the next invocation reload the same jar and consume another 2FA code against
// it before reaching the identical bootstrap 401.
func TestResolveSessionDiscardsStaleSessionWhenFreshRetryFails(t *testing.T) {
	restoreStaleSessionRetryHooks(t)

	cachedClient := &http.Client{}
	staleSession := &webcore.AuthSession{}
	freshAttempts := 0
	var deletedAppleIDs []string

	deleteStaleWebSessionFn = func(appleID string, _ *webcore.AuthSession) (bool, error) {
		deletedAppleIDs = append(deletedAppleIDs, appleID)
		return true, nil
	}

	loadCachedSessionFn = func(username string) (*webcore.AuthSession, bool, error) {
		return &webcore.AuthSession{Client: cachedClient, UserEmail: "user@example.com"}, true, nil
	}
	promptTwoFactorCodeFn = func() (string, error) { return "654321", nil }
	readTwoFactorCodeFromCommandFn = func(ctx context.Context, command string) (string, error) {
		t.Fatal("did not expect a 2fa code command when none is configured")
		return "", nil
	}
	submitTwoFactorCodeFn = func(ctx context.Context, session *webcore.AuthSession, code string) error {
		if session == staleSession {
			return &webcore.TwoFactorFinalizationError{Status: http.StatusUnauthorized}
		}
		return nil
	}
	webLoginWithClientFn = func(ctx context.Context, client *http.Client, creds webcore.LoginCredentials) (*webcore.AuthSession, error) {
		return staleSession, &webcore.TwoFactorRequiredError{}
	}
	webLoginFn = func(ctx context.Context, creds webcore.LoginCredentials) (*webcore.AuthSession, error) {
		freshAttempts++
		return nil, errors.New("dial tcp: connection refused")
	}
	persistWebSessionFn = func(session *webcore.AuthSession) error {
		t.Fatal("did not expect a failed fresh login to be persisted")
		return nil
	}

	if _, _, err := resolveSession(context.Background(), "user@example.com", "", ""); err == nil {
		t.Fatal("expected the failed fresh retry to return an error")
	}
	if freshAttempts != 1 {
		t.Fatalf("expected exactly one fresh retry, got %d", freshAttempts)
	}
	if len(deletedAppleIDs) != 1 || deletedAppleIDs[0] != "user@example.com" {
		t.Fatalf("expected the proven-stale cached session to be discarded once, got %v", deletedAppleIDs)
	}
}

// Without a code command or a terminal there is no way to replace the consumed
// literal code, so the retry must not resubmit it; it must explain that a new
// code is required.
func TestResolveSessionFailsWhenConsumedTwoFactorCodeCannotBeReplaced(t *testing.T) {
	for _, tt := range []struct {
		name         string
		ttyAvailable bool
	}{
		{name: "without a terminal", ttyAvailable: false},
		{name: "with a terminal available", ttyAvailable: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			restoreStaleSessionRetryHooks(t)

			cachedClient := &http.Client{}
			staleSession := &webcore.AuthSession{}
			var deletedAppleIDs []string

			deleteStaleWebSessionFn = func(appleID string, _ *webcore.AuthSession) (bool, error) {
				deletedAppleIDs = append(deletedAppleIDs, appleID)
				return true, nil
			}

			if tt.ttyAvailable {
				tty, err := os.CreateTemp(t.TempDir(), "tty")
				if err != nil {
					t.Fatalf("creating fake tty failed: %v", err)
				}
				t.Cleanup(func() { _ = tty.Close() })
				openTTYFn = func() (*os.File, error) { return tty, nil }
				termIsTerminalFn = func(fd int) bool { return true }
			} else {
				openTTYFn = func() (*os.File, error) { return nil, errors.New("no tty") }
				termIsTerminalFn = func(fd int) bool { return false }
			}
			loadCachedSessionFn = func(username string) (*webcore.AuthSession, bool, error) {
				return &webcore.AuthSession{Client: cachedClient, UserEmail: "user@example.com"}, true, nil
			}
			promptTwoFactorCodeFn = func() (string, error) {
				t.Fatal("did not expect an interactive 2fa prompt for a scripted --two-factor-code invocation")
				return "", nil
			}
			readTwoFactorCodeFromCommandFn = func(ctx context.Context, command string) (string, error) {
				t.Fatal("did not expect a 2fa code command when none is configured")
				return "", nil
			}
			submitTwoFactorCodeFn = func(ctx context.Context, session *webcore.AuthSession, code string) error {
				return &webcore.TwoFactorFinalizationError{Status: http.StatusUnauthorized}
			}
			webLoginWithClientFn = func(ctx context.Context, client *http.Client, creds webcore.LoginCredentials) (*webcore.AuthSession, error) {
				return staleSession, &webcore.TwoFactorRequiredError{}
			}
			webLoginFn = func(ctx context.Context, creds webcore.LoginCredentials) (*webcore.AuthSession, error) {
				t.Fatal("did not expect a fresh retry that would resubmit the consumed 2fa code")
				return nil, nil
			}

			_, _, err := resolveSession(context.Background(), "user@example.com", "", "111111")
			if err == nil {
				t.Fatal("expected resolveSession to fail when the consumed code cannot be replaced")
			}
			got := err.Error()
			if !strings.Contains(got, "2fa finalization failed: session bootstrap returned status 401") {
				t.Fatalf("expected the finalization cause to be preserved, got %q", got)
			}
			if !strings.Contains(got, "already consumed") {
				t.Fatalf("expected the message to explain that the code was consumed, got %q", got)
			}
			if !strings.Contains(got, webTwoFactorCodeCommandEnv) {
				t.Fatalf("expected the message to point at the 2fa code command, got %q", got)
			}
			if !strings.Contains(got, "discarded") {
				t.Fatalf("expected the message to say the stale cached session was discarded, got %q", got)
			}
			if len(deletedAppleIDs) != 1 || deletedAppleIDs[0] != "user@example.com" {
				t.Fatalf("expected the proven-stale cached session to be discarded once, got %v", deletedAppleIDs)
			}
		})
	}
}

// Interactive password and 2FA entry can outlast the caller's request deadline,
// so the fresh retry must run under a new bounded context instead of the
// already-expired one.
func TestResolveSessionRetriesFreshLoginAfterRequestDeadlineExpired(t *testing.T) {
	restoreStaleSessionRetryHooks(t)

	cachedClient := &http.Client{}
	staleSession := &webcore.AuthSession{}
	freshSession := &webcore.AuthSession{UserEmail: "user@example.com"}
	freshAttempts := 0
	deletedAppleIDs := 0

	deleteStaleWebSessionFn = func(appleID string, _ *webcore.AuthSession) (bool, error) {
		deletedAppleIDs++
		return true, nil
	}

	loadCachedSessionFn = func(username string) (*webcore.AuthSession, bool, error) {
		return &webcore.AuthSession{Client: cachedClient, UserEmail: "user@example.com"}, true, nil
	}
	promptTwoFactorCodeFn = func() (string, error) { return "654321", nil }
	readTwoFactorCodeFromCommandFn = func(ctx context.Context, command string) (string, error) {
		t.Fatal("did not expect a 2fa code command when none is configured")
		return "", nil
	}
	submitTwoFactorCodeFn = func(ctx context.Context, session *webcore.AuthSession, code string) error {
		if session == staleSession {
			return &webcore.TwoFactorFinalizationError{Status: http.StatusUnauthorized}
		}
		return nil
	}
	webLoginWithClientFn = func(ctx context.Context, client *http.Client, creds webcore.LoginCredentials) (*webcore.AuthSession, error) {
		return staleSession, &webcore.TwoFactorRequiredError{}
	}
	webLoginFn = func(ctx context.Context, creds webcore.LoginCredentials) (*webcore.AuthSession, error) {
		freshAttempts++
		if err := ctx.Err(); err != nil {
			t.Fatalf("expected the fresh retry to use a live context, got %v", err)
		}
		if _, ok := ctx.Deadline(); !ok {
			t.Fatal("expected the fresh retry context to stay bounded by a timeout")
		}
		return freshSession, nil
	}

	expiredCtx, cancel := shared.ContextWithTimeoutDuration(context.Background(), time.Nanosecond)
	defer cancel()
	<-expiredCtx.Done()

	session, source, err := resolveSession(expiredCtx, "user@example.com", "", "")
	if err != nil {
		t.Fatalf("resolveSession returned error: %v", err)
	}
	if freshAttempts != 1 {
		t.Fatalf("expected exactly one fresh retry, got %d", freshAttempts)
	}
	if deletedAppleIDs != 1 {
		t.Fatalf("expected the proven-stale cached session to be discarded once, got %d", deletedAppleIDs)
	}
	if session != freshSession || source != "fresh" {
		t.Fatalf("expected the fresh session, got session=%p source=%q", session, source)
	}
}

func TestProviderSelectionRenewsExpiredRequestBudget(t *testing.T) {
	originalSelect, originalPersist := selectWebProviderFn, persistWebSessionFn
	t.Cleanup(func() { selectWebProviderFn, persistWebSessionFn = originalSelect, originalPersist })
	t.Setenv("ASC_TIMEOUT", "1s")
	ctx, cancel := shared.ContextWithResolvedTimeout(context.Background(), time.Nanosecond)
	defer cancel()
	<-ctx.Done()
	persisted := false
	selectWebProviderFn = func(ctx context.Context, session *webcore.AuthSession, selection webcore.ProviderSelection) error {
		return ctx.Err()
	}
	persistWebSessionFn = func(session *webcore.AuthSession) error { persisted = true; return nil }
	if err := selectResolvedWebSessionProvider(ctx, &webcore.AuthSession{}, webcore.ProviderSelection{ProviderID: 7}); err != nil {
		t.Fatal(err)
	}
	if !persisted {
		t.Fatal("authenticated selected session was not persisted")
	}
}

func TestProviderSelectionPreservesCallerCancellation(t *testing.T) {
	originalSelect, originalPersist := selectWebProviderFn, persistWebSessionFn
	t.Cleanup(func() { selectWebProviderFn, persistWebSessionFn = originalSelect, originalPersist })
	parent, stop := context.WithCancel(context.Background())
	ctx, cancel := shared.ContextWithTimeout(parent)
	defer cancel()
	stop()
	selectWebProviderFn = func(ctx context.Context, session *webcore.AuthSession, selection webcore.ProviderSelection) error {
		return ctx.Err()
	}
	persistWebSessionFn = func(session *webcore.AuthSession) error {
		t.Fatal("must not persist after canceled selection")
		return nil
	}
	if err := selectResolvedWebSessionProvider(ctx, &webcore.AuthSession{}, webcore.ProviderSelection{ProviderID: 7}); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected cancellation, got %v", err)
	}
}

func TestLoginRenewsBudgetAfterCredentialPrompt(t *testing.T) {
	t.Setenv("ASC_TIMEOUT", "1s")
	ctx, cancel := shared.ContextWithResolvedTimeout(context.Background(), time.Nanosecond)
	defer cancel()
	<-ctx.Done()
	expected := &webcore.AuthSession{}
	result, err := loginWithOptionalTwoFactorUsing(ctx, "test", "user@example.com", "test-only", "", func(ctx context.Context, creds webcore.LoginCredentials) (*webcore.AuthSession, error) {
		return expected, ctx.Err()
	}, nil, nil)
	if err != nil || result != expected {
		t.Fatalf("credential wait consumed login budget: %v", err)
	}
}
