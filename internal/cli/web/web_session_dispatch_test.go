package web

import (
	"context"
	"errors"
	"strings"
	"testing"

	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

func TestCallResolveAppCreateSessionFnPassesTwoFactorCommand(t *testing.T) {
	origResolveAppCreateSession := resolveAppCreateSessionFn
	t.Cleanup(func() { resolveAppCreateSessionFn = origResolveAppCreateSession })

	var gotCommand string
	resolveAppCreateSessionFn = func(ctx context.Context, appleID, password, twoFactorCode, twoFactorCodeCommand string) (*webcore.AuthSession, string, error) {
		gotCommand = twoFactorCodeCommand
		return &webcore.AuthSession{}, "fresh", nil
	}

	_, _, err := callResolveAppCreateSessionFn(context.Background(), "user@example.com", "secret", "", "osascript ./get-2fa.scpt")
	if err != nil {
		t.Fatalf("callResolveAppCreateSessionFn returned error: %v", err)
	}
	if gotCommand != "osascript ./get-2fa.scpt" {
		t.Fatalf("expected command to be forwarded, got %q", gotCommand)
	}
}

func TestCallResolveSessionFnRejectsDroppedTwoFactorCommand(t *testing.T) {
	origResolveSession := resolveSessionFn
	t.Cleanup(func() { resolveSessionFn = origResolveSession })

	resolveSessionFn = func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		t.Fatal("expected three-argument test hook not to be called when a command is supplied")
		return nil, "", nil
	}

	_, _, err := callResolveSessionFn(context.Background(), "user@example.com", "secret", "", "osascript ./get-2fa.scpt")
	if err == nil {
		t.Fatal("expected callResolveSessionFn to reject silently dropping the two-factor command")
	}
	if !strings.Contains(err.Error(), "cannot accept --two-factor-code-command") {
		t.Fatalf("expected dropped-command error, got %v", err)
	}
}

func TestPersistFreshAppCreateSessionIgnoresCacheFailures(t *testing.T) {
	origPersistWebSession := persistWebSessionFn
	t.Cleanup(func() { persistWebSessionFn = origPersistWebSession })

	called := false
	persistWebSessionFn = func(session *webcore.AuthSession) error {
		called = true
		return errors.New("cache unavailable")
	}

	if err := persistFreshAppCreateSession(&webcore.AuthSession{}); err != nil {
		t.Fatalf("persistFreshAppCreateSession returned error: %v", err)
	}
	if !called {
		t.Fatal("expected persistFreshAppCreateSession to attempt cache persistence")
	}
}
