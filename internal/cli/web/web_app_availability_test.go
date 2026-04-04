package web

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"io"
	"os"
	"strings"
	"testing"

	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

func captureWebCommandOutput(t *testing.T, fn func()) (string, string) {
	t.Helper()

	oldStdout := os.Stdout
	oldStderr := os.Stderr

	rOut, wOut, err := os.Pipe()
	if err != nil {
		t.Fatalf("stdout pipe: %v", err)
	}
	rErr, wErr, err := os.Pipe()
	if err != nil {
		t.Fatalf("stderr pipe: %v", err)
	}

	os.Stdout = wOut
	os.Stderr = wErr

	outC := make(chan string)
	errC := make(chan string)

	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, rOut)
		_ = rOut.Close()
		outC <- buf.String()
	}()

	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, rErr)
		_ = rErr.Close()
		errC <- buf.String()
	}()

	defer func() {
		os.Stdout = oldStdout
		os.Stderr = oldStderr
		_ = wOut.Close()
		_ = wErr.Close()
	}()

	fn()

	_ = wOut.Close()
	_ = wErr.Close()
	os.Stdout = oldStdout
	os.Stderr = oldStderr

	return <-outC, <-errC
}

func TestWebAppsAvailabilityCreateValidationErrors(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{name: "missing app", args: []string{"--territory", "USA", "--available-in-new-territories", "false"}, wantErr: "--app is required"},
		{name: "missing territory", args: []string{"--app", "app-1", "--available-in-new-territories", "false"}, wantErr: "--territory is required"},
		{name: "missing available in new territories", args: []string{"--app", "app-1", "--territory", "USA"}, wantErr: "--available-in-new-territories is required"},
		{name: "invalid territory", args: []string{"--app", "app-1", "--territory", "Atlantis", "--available-in-new-territories", "false"}, wantErr: "territory \"Atlantis\" could not be mapped to an App Store Connect territory ID"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("ASC_APP_ID", "")
			cmd := WebAppsAvailabilityCreateCommand()
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

func TestWebAppsAvailabilityCreateResolvesSessionBeforeTimeoutContext(t *testing.T) {
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

	cmd := WebAppsAvailabilityCreateCommand()
	if err := cmd.FlagSet.Parse([]string{
		"--app", "app-1",
		"--territory", "USA",
		"--available-in-new-territories", "false",
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

func TestWebAppsAvailabilityCreateSkipsCreateWhenAvailabilityExists(t *testing.T) {
	origResolveSession := resolveSessionFn
	origNewWebClient := newWebClientFn
	origGet := getWebAppAvailabilityFn
	origCreate := createWebAppAvailabilityFn
	t.Cleanup(func() {
		resolveSessionFn = origResolveSession
		newWebClientFn = origNewWebClient
		getWebAppAvailabilityFn = origGet
		createWebAppAvailabilityFn = origCreate
	})

	resolveSessionFn = func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{}, "cache", nil
	}
	newWebClientFn = func(session *webcore.AuthSession) *webcore.Client {
		return &webcore.Client{}
	}

	getWebAppAvailabilityFn = func(ctx context.Context, client *webcore.Client, appID string) (*webcore.AppAvailability, error) {
		return &webcore.AppAvailability{ID: "avail-1"}, nil
	}
	createCalled := false
	createWebAppAvailabilityFn = func(ctx context.Context, client *webcore.Client, attrs webcore.AppAvailabilityCreateAttributes) (*webcore.AppAvailability, error) {
		createCalled = true
		return nil, nil
	}

	cmd := WebAppsAvailabilityCreateCommand()
	if err := cmd.FlagSet.Parse([]string{
		"--app", "app-1",
		"--territory", "USA",
		"--available-in-new-territories", "false",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	err := cmd.Exec(context.Background(), nil)
	if err == nil {
		t.Fatal("expected existing-availability error")
	}
	if !strings.Contains(err.Error(), `app availability already exists for app "app-1"`) {
		t.Fatalf("unexpected error: %v", err)
	}
	if createCalled {
		t.Fatal("expected create to be skipped when availability already exists")
	}
}

func TestWebAppsAvailabilityCreateCreatesMissingAvailability(t *testing.T) {
	origResolveSession := resolveSessionFn
	origNewWebClient := newWebClientFn
	origGet := getWebAppAvailabilityFn
	origCreate := createWebAppAvailabilityFn
	t.Cleanup(func() {
		resolveSessionFn = origResolveSession
		newWebClientFn = origNewWebClient
		getWebAppAvailabilityFn = origGet
		createWebAppAvailabilityFn = origCreate
	})

	resolveSessionFn = func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{}, "cache", nil
	}
	newWebClientFn = func(session *webcore.AuthSession) *webcore.Client {
		return &webcore.Client{}
	}

	getWebAppAvailabilityFn = func(ctx context.Context, client *webcore.Client, appID string) (*webcore.AppAvailability, error) {
		return nil, &webcore.APIError{Status: 404}
	}

	var received webcore.AppAvailabilityCreateAttributes
	createWebAppAvailabilityFn = func(ctx context.Context, client *webcore.Client, attrs webcore.AppAvailabilityCreateAttributes) (*webcore.AppAvailability, error) {
		received = attrs
		return &webcore.AppAvailability{
			ID:                        "avail-123",
			AvailableInNewTerritories: false,
			AvailableTerritories:      []string{"GBR", "USA"},
		}, nil
	}

	cmd := WebAppsAvailabilityCreateCommand()
	if err := cmd.FlagSet.Parse([]string{
		"--app", "app-1",
		"--territory", "US,France",
		"--available-in-new-territories", "false",
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
	if !strings.Contains(stdout, `"availabilityId":"avail-123"`) {
		t.Fatalf("expected JSON output to include availability id, got %q", stdout)
	}
	if !strings.Contains(stdout, `"availableTerritories":["GBR","USA"]`) {
		t.Fatalf("expected JSON output to use created territories, got %q", stdout)
	}
	if received.AppID != "app-1" {
		t.Fatalf("expected app id app-1, got %q", received.AppID)
	}
	if received.AvailableInNewTerritories {
		t.Fatal("expected availableInNewTerritories=false")
	}
	if got := strings.Join(received.AvailableTerritories, ","); got != "USA,FRA" {
		t.Fatalf("expected normalized territory ids USA,FRA, got %q", got)
	}
}

func TestWebAppsAvailabilityCreateWrapsAuthErrors(t *testing.T) {
	origResolveSession := resolveSessionFn
	origNewWebClient := newWebClientFn
	origGet := getWebAppAvailabilityFn
	origCreate := createWebAppAvailabilityFn
	t.Cleanup(func() {
		resolveSessionFn = origResolveSession
		newWebClientFn = origNewWebClient
		getWebAppAvailabilityFn = origGet
		createWebAppAvailabilityFn = origCreate
	})

	resolveSessionFn = func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{}, "cache", nil
	}
	newWebClientFn = func(session *webcore.AuthSession) *webcore.Client {
		return &webcore.Client{}
	}

	getWebAppAvailabilityFn = func(ctx context.Context, client *webcore.Client, appID string) (*webcore.AppAvailability, error) {
		return nil, errors.New("unexpected")
	}
	authErr := &webcore.APIError{Status: 401}
	createWebAppAvailabilityFn = func(ctx context.Context, client *webcore.Client, attrs webcore.AppAvailabilityCreateAttributes) (*webcore.AppAvailability, error) {
		return nil, authErr
	}

	cmd := WebAppsAvailabilityCreateCommand()
	if err := cmd.FlagSet.Parse([]string{
		"--app", "app-1",
		"--territory", "USA",
		"--available-in-new-territories", "false",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	// Force create path.
	getWebAppAvailabilityFn = func(ctx context.Context, client *webcore.Client, appID string) (*webcore.AppAvailability, error) {
		return nil, &webcore.APIError{Status: 404}
	}

	err := cmd.Exec(context.Background(), nil)
	if err == nil {
		t.Fatal("expected auth-wrapped error")
	}
	if !strings.Contains(err.Error(), "web apps availability create failed: web session is unauthorized or expired") {
		t.Fatalf("unexpected error: %v", err)
	}
}
