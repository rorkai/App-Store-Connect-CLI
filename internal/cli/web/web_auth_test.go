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

func TestReadPasswordFromTerminalFD(t *testing.T) {
	origReadPassword := termReadPasswordFn
	t.Cleanup(func() {
		termReadPasswordFn = origReadPassword
	})

	t.Run("preserves interactive password bytes and writes prompt", func(t *testing.T) {
		termReadPasswordFn = func(fd int) ([]byte, error) {
			return []byte("  secret-pass  "), nil
		}
		var prompt bytes.Buffer

		password, err := readPasswordFromTerminalFD(context.Background(), &prompt)
		if err != nil {
			t.Fatalf("readPasswordFromTerminalFD returned error: %v", err)
		}
		if password != "  secret-pass  " {
			t.Fatalf("expected password %q, got %q", "  secret-pass  ", password)
		}
		if !strings.Contains(prompt.String(), "Apple Account password:") {
			t.Fatalf("expected password prompt text, got %q", prompt.String())
		}
	})

	t.Run("rejects whitespace-only password", func(t *testing.T) {
		termReadPasswordFn = func(fd int) ([]byte, error) {
			return []byte("   "), nil
		}

		_, err := readPasswordFromTerminalFD(context.Background(), &bytes.Buffer{})
		if err == nil {
			t.Fatal("expected error for whitespace-only password")
		}
		if !strings.Contains(err.Error(), "password is required") {
			t.Fatalf("unexpected error: %v", err)
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
	if !strings.Contains(err.Error(), "--"+deprecatedTwoFactorCodeFlagName) {
		t.Fatalf("expected deprecated compatibility flag hint in error, got %v", err)
	}
}

func TestTwoFactorCodeCommandShellArgs(t *testing.T) {
	args := twoFactorCodeCommandShellArgs("printf '123456\\n'")

	if runtime.GOOS == "windows" {
		want := []string{"/d", "/s", "/c", "printf '123456\\n'"}
		if len(args) != len(want) {
			t.Fatalf("expected %d args, got %d (%v)", len(want), len(args), args)
		}
		for i, part := range want {
			if args[i] != part {
				t.Fatalf("expected arg %d to be %q, got %q", i, part, args[i])
			}
		}
		return
	}

	if len(args) != 2 {
		t.Fatalf("expected 2 args, got %d (%v)", len(args), args)
	}
	if args[0] != "-c" {
		t.Fatalf("expected non-login shell flag %q, got %q", "-c", args[0])
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

	session, err := loginWithOptionalTwoFactor(context.Background(), "user@example.com", "secret", "")
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

	session, err := loginWithOptionalTwoFactor(context.Background(), "user@example.com", "secret", "654321")
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
		return "", errors.New("2fa required: run in a terminal for an interactive prompt, pass --two-factor-code-command, set " + webTwoFactorCodeCommandEnv + ", or re-run with deprecated --" + deprecatedTwoFactorCodeFlagName)
	}
	submitTwoFactorCodeFn = func(ctx context.Context, session *webcore.AuthSession, code string) error {
		t.Fatal("did not expect submit when prompt fails")
		return nil
	}

	_, err := loginWithOptionalTwoFactor(context.Background(), "user@example.com", "secret", "", "")
	if err == nil {
		t.Fatal("expected error when prompt fails")
	}
	if !strings.Contains(err.Error(), "--two-factor-code-command") || !strings.Contains(err.Error(), webTwoFactorCodeCommandEnv) {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(err.Error(), "--"+deprecatedTwoFactorCodeFlagName) {
		t.Fatalf("expected deprecated compatibility flag hint in error, got %v", err)
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

	session, err := loginWithOptionalTwoFactor(context.Background(), "user@example.com", "secret", "", "osascript ./get-2fa.scpt")
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

	requestCtx, cancel := shared.ContextWithTimeoutDuration(context.Background(), 30*time.Millisecond)
	t.Cleanup(cancel)

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
		if deadline, ok := ctx.Deadline(); !ok || time.Until(deadline) <= 0 {
			t.Fatalf("expected verification context to have a future deadline, got ok=%v deadline=%v", ok, deadline)
		}
		return nil
	}

	session, err := loginWithOptionalTwoFactor(requestCtx, "user@example.com", "secret", "", "osascript ./get-2fa.scpt")
	if err != nil {
		t.Fatalf("loginWithOptionalTwoFactor returned error: %v", err)
	}
	if session == nil {
		t.Fatal("expected non-nil session")
	}
	if !errors.Is(requestCtx.Err(), context.DeadlineExceeded) {
		t.Fatalf("expected original request context to time out, got %v", requestCtx.Err())
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

	if _, err := loginWithOptionalTwoFactor(context.Background(), "user@example.com", "secret", ""); err != nil {
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

	if _, err := loginWithOptionalTwoFactor(context.Background(), "user@example.com", "secret", "123456"); err != nil {
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
		return &webcore.TwoFactorChallenge{Method: "trusted-device"}, nil
	}
	ensureTwoFactorCodeRequestedFn = func(ctx context.Context, session *webcore.AuthSession) (*webcore.TwoFactorChallenge, error) {
		t.Fatal("did not expect upfront phone-code request for trusted-device challenge")
		return nil, nil
	}
	promptTwoFactorCodeFn = func() (string, error) {
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

	session, err := loginWithOptionalTwoFactor(context.Background(), "user@example.com", "secret", "")
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
	if output := statusOutput.String(); !strings.Contains(output, "Verification code sent to +1 (•••) •••-••66.") {
		t.Fatalf("expected fallback delivery notice, got %q", output)
	}
	if output := statusOutput.String(); !strings.Contains(output, "Trusted-device verification was rejected. Enter the phone verification code that was just sent.") {
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
		return &webcore.TwoFactorChallenge{Method: "trusted-device"}, nil
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

	session, err := loginWithOptionalTwoFactor(context.Background(), "user@example.com", "secret", "", "osascript ./get-2fa.scpt")
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

	_, err := loginWithOptionalTwoFactor(context.Background(), "user@example.com", "secret", "")
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
