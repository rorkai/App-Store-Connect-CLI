package web

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"strings"
	"testing"

	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

func TestWebSandboxCreateValidationErrors(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "missing first name",
			args:    []string{"--last-name", "Tester", "--email", "jane@example.com", "--password", "Passwordtest1", "--territory", "USA"},
			wantErr: "--first-name is required",
		},
		{
			name:    "invalid email",
			args:    []string{"--first-name", "Jane", "--last-name", "Tester", "--email", "bad-email", "--password", "Passwordtest1", "--territory", "USA"},
			wantErr: "--email must be a valid email address",
		},
		{
			name:    "display name email is rejected",
			args:    []string{"--first-name", "Jane", "--last-name", "Tester", "--email", "Jane Tester <jane@example.com>", "--password", "Passwordtest1", "--territory", "USA"},
			wantErr: "--email must be a valid email address",
		},
		{
			name:    "invalid password",
			args:    []string{"--first-name", "Jane", "--last-name", "Tester", "--email", "jane@example.com", "--password", "password", "--territory", "USA"},
			wantErr: "--password must be at least 8 characters and include uppercase, lowercase, and numeric characters",
		},
		{
			name:    "invalid territory",
			args:    []string{"--first-name", "Jane", "--last-name", "Tester", "--email", "jane@example.com", "--password", "Passwordtest1", "--territory", "ZZZ"},
			wantErr: "--territory must be a valid App Store territory code",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cmd := WebSandboxCreateCommand()
			if err := cmd.FlagSet.Parse(tc.args); err != nil {
				t.Fatalf("parse error: %v", err)
			}
			stdout, stderr := captureWebCommandOutput(t, func() {
				err := cmd.Exec(context.Background(), nil)
				if !errors.Is(err, flag.ErrHelp) {
					t.Fatalf("expected flag.ErrHelp, got %v", err)
				}
			})
			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, tc.wantErr) {
				t.Fatalf("expected stderr to contain %q, got %q", tc.wantErr, stderr)
			}
		})
	}
}

func TestNormalizeWebSandboxPasswordCountsCharactersNotBytes(t *testing.T) {
	_, err := normalizeWebSandboxPassword("Aéééé1b")
	if err == nil {
		t.Fatal("expected too-short password error")
	}
	if !strings.Contains(err.Error(), "at least 8 characters") {
		t.Fatalf("expected length error, got %v", err)
	}
}

func TestWebSandboxCreateResolvesSessionBeforeTimeoutContext(t *testing.T) {
	origResolveSession := resolveSessionFn
	t.Cleanup(func() {
		resolveSessionFn = origResolveSession
	})

	resolveErr := errors.New("stop before network call")
	hadDeadline := false
	resolveSessionFn = func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		_, hadDeadline = ctx.Deadline()
		return nil, "", resolveErr
	}

	cmd := WebSandboxCreateCommand()
	if err := cmd.FlagSet.Parse([]string{
		"--first-name", "Jane",
		"--last-name", "Tester",
		"--email", "jane@example.com",
		"--password", "Passwordtest1",
		"--territory", "USA",
		"--apple-id", "user@example.com",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	err := cmd.Exec(context.Background(), nil)
	if !errors.Is(err, resolveErr) {
		t.Fatalf("expected resolveSession error %v, got %v", resolveErr, err)
	}
	if hadDeadline {
		t.Fatal("expected resolveSession to run before timeout context creation")
	}
}

func TestWebSandboxCreateSubmitsNormalizedRequest(t *testing.T) {
	origResolveSession := resolveSessionFn
	origNewWebClient := newWebClientFn
	origCreate := createWebSandboxTesterFn
	t.Cleanup(func() {
		resolveSessionFn = origResolveSession
		newWebClientFn = origNewWebClient
		createWebSandboxTesterFn = origCreate
	})

	resolveSessionFn = func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{}, "cache", nil
	}
	newWebClientFn = func(session *webcore.AuthSession) *webcore.Client {
		return &webcore.Client{}
	}

	var received webcore.SandboxAccountCreateAttributes
	createWebSandboxTesterFn = func(ctx context.Context, client *webcore.Client, attrs webcore.SandboxAccountCreateAttributes) error {
		received = attrs
		return nil
	}

	cmd := WebSandboxCreateCommand()
	if err := cmd.FlagSet.Parse([]string{
		"--first-name", " Jane ",
		"--last-name", " Tester ",
		"--email", "jane+sandbox@example.com",
		"--password", "Passwordtest1",
		"--territory", "usa",
		"--output", "json",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	stdout, stderr := captureWebCommandOutput(t, func() {
		if err := cmd.Exec(context.Background(), nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var result webSandboxCreateResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout=%q", err, stdout)
	}
	if !result.Submitted {
		t.Fatal("expected submitted=true")
	}
	if result.Territory != "USA" {
		t.Fatalf("expected territory USA, got %q", result.Territory)
	}
	if result.Email != "jane+sandbox@example.com" {
		t.Fatalf("expected email preserved, got %q", result.Email)
	}

	if received.FirstName != "Jane" {
		t.Fatalf("expected first name Jane, got %q", received.FirstName)
	}
	if received.LastName != "Tester" {
		t.Fatalf("expected last name Tester, got %q", received.LastName)
	}
	if received.AccountName != "jane+sandbox@example.com" {
		t.Fatalf("expected account name email, got %q", received.AccountName)
	}
	if received.AccountPassword != "Passwordtest1" {
		t.Fatalf("expected password preserved, got %q", received.AccountPassword)
	}
	if received.StoreFront != "USA" {
		t.Fatalf("expected storefront USA, got %q", received.StoreFront)
	}
}

func TestWebSandboxCreateWrapsAuthErrors(t *testing.T) {
	origResolveSession := resolveSessionFn
	origNewWebClient := newWebClientFn
	origCreate := createWebSandboxTesterFn
	t.Cleanup(func() {
		resolveSessionFn = origResolveSession
		newWebClientFn = origNewWebClient
		createWebSandboxTesterFn = origCreate
	})

	resolveSessionFn = func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{}, "cache", nil
	}
	newWebClientFn = func(session *webcore.AuthSession) *webcore.Client {
		return &webcore.Client{}
	}
	createWebSandboxTesterFn = func(ctx context.Context, client *webcore.Client, attrs webcore.SandboxAccountCreateAttributes) error {
		return &webcore.APIError{Status: 401}
	}

	cmd := WebSandboxCreateCommand()
	if err := cmd.FlagSet.Parse([]string{
		"--first-name", "Jane",
		"--last-name", "Tester",
		"--email", "jane@example.com",
		"--password", "Passwordtest1",
		"--territory", "USA",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	err := cmd.Exec(context.Background(), nil)
	if err == nil {
		t.Fatal("expected auth-wrapped error")
	}
	if !strings.Contains(err.Error(), "web sandbox create failed: web session is unauthorized or expired") {
		t.Fatalf("unexpected error: %v", err)
	}
}
