package web

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

func TestWebAppsCreateDefersPasswordResolutionToResolveSession(t *testing.T) {
	origResolveSession := resolveSessionFn
	origPromptPassword := promptPasswordFn
	t.Cleanup(func() {
		resolveSessionFn = origResolveSession
		promptPasswordFn = origPromptPassword
	})

	promptErr := errors.New("prompt should not run before session resolution")
	resolveErr := errors.New("stop before network call")

	promptPasswordFn = func() (string, error) {
		return "", promptErr
	}

	var (
		calledResolve bool
		receivedID    string
		receivedPass  string
	)
	resolveSessionFn = func(ctx context.Context, appleID, password, twoFactorCode string, usePasswordStdin bool) (*webcore.AuthSession, string, error) {
		calledResolve = true
		receivedID = appleID
		receivedPass = password
		return nil, "", resolveErr
	}

	cmd := WebAppsCreateCommand()
	if err := cmd.FlagSet.Parse([]string{
		"--name", "My App",
		"--bundle-id", "com.example.app",
		"--sku", "SKU123",
		"--apple-id", "user@example.com",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	err := cmd.Exec(context.Background(), nil)
	if !errors.Is(err, resolveErr) {
		t.Fatalf("expected resolveSession error %v, got %v", resolveErr, err)
	}
	if !calledResolve {
		t.Fatal("expected resolveSession to be called")
	}
	if receivedID != "user@example.com" {
		t.Fatalf("expected apple ID %q, got %q", "user@example.com", receivedID)
	}
	if receivedPass != "" {
		t.Fatalf("expected empty password argument, got %q", receivedPass)
	}
}

func TestWebAppsCreateResolvesSessionBeforeTimeoutContext(t *testing.T) {
	origResolveSession := resolveSessionFn
	t.Cleanup(func() {
		resolveSessionFn = origResolveSession
	})

	resolveErr := errors.New("stop before network call")
	hadDeadline := false
	resolveSessionFn = func(ctx context.Context, appleID, password, twoFactorCode string, usePasswordStdin bool) (*webcore.AuthSession, string, error) {
		_, hadDeadline = ctx.Deadline()
		return nil, "", resolveErr
	}

	cmd := WebAppsCreateCommand()
	if err := cmd.FlagSet.Parse([]string{
		"--name", "My App",
		"--bundle-id", "com.example.app",
		"--sku", "SKU123",
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

func TestWebAppsCreateEnsuresBundleIDBeforeCreateApp(t *testing.T) {
	origResolveSession := resolveSessionFn
	origNewWebClient := newWebClientFn
	origEnsureBundleID := ensureBundleIDFn
	origCreateWebApp := createWebAppFn
	t.Cleanup(func() {
		resolveSessionFn = origResolveSession
		newWebClientFn = origNewWebClient
		ensureBundleIDFn = origEnsureBundleID
		createWebAppFn = origCreateWebApp
	})

	resolveSessionFn = func(ctx context.Context, appleID, password, twoFactorCode string, usePasswordStdin bool) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{}, "cache", nil
	}
	newWebClientFn = func(session *webcore.AuthSession) *webcore.Client {
		return &webcore.Client{}
	}

	callOrder := make([]string, 0, 2)
	ensureBundleIDFn = func(ctx context.Context, bundleID, appName, platform string) (bool, error) {
		callOrder = append(callOrder, "ensure")
		if bundleID != "com.example.app" {
			t.Fatalf("expected bundle id %q, got %q", "com.example.app", bundleID)
		}
		if appName != "My App" {
			t.Fatalf("expected app name %q, got %q", "My App", appName)
		}
		if platform != "IOS" {
			t.Fatalf("expected platform %q, got %q", "IOS", platform)
		}
		return true, nil
	}
	createWebAppFn = func(ctx context.Context, client *webcore.Client, attrs webcore.AppCreateAttributes) (*webcore.AppResponse, error) {
		callOrder = append(callOrder, "create")
		resp := &webcore.AppResponse{}
		resp.Data.ID = "app-123"
		return resp, nil
	}

	cmd := WebAppsCreateCommand()
	if err := cmd.FlagSet.Parse([]string{
		"--name", "My App",
		"--bundle-id", "com.example.app",
		"--sku", "SKU123",
		"--apple-id", "user@example.com",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	if err := cmd.Exec(context.Background(), nil); err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if len(callOrder) != 2 {
		t.Fatalf("expected 2 calls, got %d", len(callOrder))
	}
	if callOrder[0] != "ensure" || callOrder[1] != "create" {
		t.Fatalf("expected ensure before create, got %v", callOrder)
	}
}

func TestWebAppsCreateFailsWhenBundleIDPreflightFails(t *testing.T) {
	origResolveSession := resolveSessionFn
	origNewWebClient := newWebClientFn
	origEnsureBundleID := ensureBundleIDFn
	origCreateWebApp := createWebAppFn
	t.Cleanup(func() {
		resolveSessionFn = origResolveSession
		newWebClientFn = origNewWebClient
		ensureBundleIDFn = origEnsureBundleID
		createWebAppFn = origCreateWebApp
	})

	resolveSessionFn = func(ctx context.Context, appleID, password, twoFactorCode string, usePasswordStdin bool) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{}, "cache", nil
	}
	newWebClientFn = func(session *webcore.AuthSession) *webcore.Client {
		return &webcore.Client{}
	}

	preflightErr := errors.New("preflight failed")
	ensureBundleIDFn = func(ctx context.Context, bundleID, appName, platform string) (bool, error) {
		return false, preflightErr
	}
	createCalled := false
	createWebAppFn = func(ctx context.Context, client *webcore.Client, attrs webcore.AppCreateAttributes) (*webcore.AppResponse, error) {
		createCalled = true
		return nil, nil
	}

	cmd := WebAppsCreateCommand()
	if err := cmd.FlagSet.Parse([]string{
		"--name", "My App",
		"--bundle-id", "com.example.app",
		"--sku", "SKU123",
		"--apple-id", "user@example.com",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	err := cmd.Exec(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "bundle id preflight failed") {
		t.Fatalf("expected bundle preflight message, got %v", err)
	}
	if !errors.Is(err, preflightErr) {
		t.Fatalf("expected wrapped preflight error, got %v", err)
	}
	if createCalled {
		t.Fatal("expected create app to be skipped on preflight failure")
	}
}

func TestBundleIDPlatformForWebApp(t *testing.T) {
	t.Run("maps UNIVERSAL to IOS for bundle id create", func(t *testing.T) {
		got, err := bundleIDPlatformForWebApp("UNIVERSAL")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != asc.PlatformIOS {
			t.Fatalf("expected %q, got %q", asc.PlatformIOS, got)
		}
	})

	t.Run("keeps explicit mac platform", func(t *testing.T) {
		got, err := bundleIDPlatformForWebApp("MAC_OS")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != asc.PlatformMacOS {
			t.Fatalf("expected %q, got %q", asc.PlatformMacOS, got)
		}
	})

	t.Run("rejects invalid platform with web command contract", func(t *testing.T) {
		_, err := bundleIDPlatformForWebApp("VISION_OS")
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "IOS, MAC_OS, TV_OS, UNIVERSAL") {
			t.Fatalf("expected web platform list in error, got %v", err)
		}
	})
}
