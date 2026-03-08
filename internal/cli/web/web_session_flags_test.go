package web

import (
	"context"
	"errors"
	"flag"
	"strings"
	"testing"

	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

func TestBindWebSessionFlagsParsesValues(t *testing.T) {
	fs := flag.NewFlagSet("web-session-flags", flag.ContinueOnError)
	flags := bindWebSessionFlags(fs)

	if err := fs.Parse([]string{"--apple-id", "user@example.com", "--two-factor-code", "123456"}); err != nil {
		t.Fatalf("parse flags: %v", err)
	}

	if got := *flags.appleID; got != "user@example.com" {
		t.Fatalf("expected apple-id to be parsed, got %q", got)
	}
	if got := *flags.twoFactorCode; got != "123456" {
		t.Fatalf("expected two-factor-code to be parsed, got %q", got)
	}
}

func TestResolveWebSessionForCommandPassesFlagsToResolver(t *testing.T) {
	origResolveSession := resolveSessionFn
	t.Cleanup(func() {
		resolveSessionFn = origResolveSession
	})

	var gotAppleID, gotPassword, gotTwoFactorCode string
	expectedSession := &webcore.AuthSession{UserEmail: "cached@example.com"}
	resolveSessionFn = func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		gotAppleID = appleID
		gotPassword = password
		gotTwoFactorCode = twoFactorCode
		return expectedSession, "cache", nil
	}

	appleID := "user@example.com"
	twoFactorCode := "654321"
	session, err := resolveWebSessionForCommand(context.Background(), webSessionFlags{
		appleID:       &appleID,
		twoFactorCode: &twoFactorCode,
	})
	if err != nil {
		t.Fatalf("resolveWebSessionForCommand() error: %v", err)
	}
	if session != expectedSession {
		t.Fatalf("expected resolver session pointer to be returned")
	}
	if gotAppleID != appleID {
		t.Fatalf("expected apple-id %q, got %q", appleID, gotAppleID)
	}
	if gotPassword != "" {
		t.Fatalf("expected password to remain empty, got %q", gotPassword)
	}
	if gotTwoFactorCode != twoFactorCode {
		t.Fatalf("expected two-factor-code %q, got %q", twoFactorCode, gotTwoFactorCode)
	}
}

func TestResolveWebSessionForCommandReturnsResolverError(t *testing.T) {
	origResolveSession := resolveSessionFn
	t.Cleanup(func() {
		resolveSessionFn = origResolveSession
	})

	expectedErr := errors.New("session lookup failed")
	resolveSessionFn = func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		return nil, "", expectedErr
	}

	appleID := "user@example.com"
	twoFactorCode := "000000"
	_, err := resolveWebSessionForCommand(context.Background(), webSessionFlags{
		appleID:       &appleID,
		twoFactorCode: &twoFactorCode,
	})
	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected resolver error to be returned, got %v", err)
	}
}

func TestWithWebAuthHint(t *testing.T) {
	t.Run("returns nil when err is nil", func(t *testing.T) {
		if got := withWebAuthHint(nil, "web review list"); got != nil {
			t.Fatalf("expected nil, got %v", got)
		}
	})

	t.Run("does not double-prefix already prefixed errors", func(t *testing.T) {
		input := errors.New("web review list failed: request timed out")
		got := withWebAuthHint(input, "web review list")
		if got != input {
			t.Fatalf("expected original error to be returned unchanged")
		}
	})

	t.Run("adds unauthorized hint for 401 and wraps original error", func(t *testing.T) {
		input := &webcore.APIError{Status: 401}
		got := withWebAuthHint(input, "web review list")
		if got == nil {
			t.Fatal("expected wrapped error")
		}
		if !strings.Contains(got.Error(), "web session is unauthorized or expired (run 'asc web auth login')") {
			t.Fatalf("expected auth hint in error, got %q", got.Error())
		}
		if !strings.Contains(got.Error(), "web review list failed:") {
			t.Fatalf("expected operation prefix in error, got %q", got.Error())
		}
		var apiErr *webcore.APIError
		if !errors.As(got, &apiErr) {
			t.Fatalf("expected wrapped APIError, got %T", got)
		}
	})

	t.Run("adds generic operation prefix for non-auth API errors", func(t *testing.T) {
		input := &webcore.APIError{Status: 500}
		got := withWebAuthHint(input, "xcode-cloud products")
		if got == nil {
			t.Fatal("expected wrapped error")
		}
		if !strings.Contains(got.Error(), "xcode-cloud products failed:") {
			t.Fatalf("expected operation prefix in error, got %q", got.Error())
		}
		if strings.Contains(got.Error(), "web session is unauthorized or expired") {
			t.Fatalf("did not expect unauthorized hint for status 500, got %q", got.Error())
		}
	})
}
