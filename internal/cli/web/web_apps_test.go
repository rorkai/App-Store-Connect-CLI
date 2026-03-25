package web

import (
	"context"
	"errors"
	"flag"
	"net/http"
	"net/http/cookiejar"
	"os"
	"strings"
	"testing"

	"github.com/AlecAivazis/survey/v2"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

func TestWebAppsCreatePassesPasswordCompatibilityFlagToSessionResolver(t *testing.T) {
	origResolveAppCreateSession := resolveAppCreateSessionFn
	origNewWebClient := newWebClientFn
	origEnsureBundleID := ensureBundleIDFn
	origCreateWebApp := createWebAppFn
	t.Cleanup(func() {
		resolveAppCreateSessionFn = origResolveAppCreateSession
		newWebClientFn = origNewWebClient
		ensureBundleIDFn = origEnsureBundleID
		createWebAppFn = origCreateWebApp
	})
	var (
		receivedID   string
		receivedPass string
	)
	resolveAppCreateSessionFn = func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		receivedID = appleID
		receivedPass = password
		return &webcore.AuthSession{}, "cache", nil
	}
	newWebClientFn = func(session *webcore.AuthSession) *webcore.Client {
		return &webcore.Client{}
	}
	ensureBundleIDFn = func(ctx context.Context, bundleID, appName, platform string) (bool, error) {
		return false, nil
	}
	createWebAppFn = func(ctx context.Context, client *webcore.Client, attrs webcore.AppCreateAttributes) (*webcore.AppResponse, error) {
		resp := &webcore.AppResponse{}
		resp.Data.ID = "app-123"
		return resp, nil
	}
	passwordFlag := "--" + "password"

	cmd := WebAppsCreateCommand()
	if err := cmd.FlagSet.Parse([]string{
		"--name", "My App",
		"--bundle-id", "com.example.app",
		"--sku", "SKU123",
		"--apple-id", "user@example.com",
		passwordFlag, "  fixture-password  ",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	if err := cmd.Exec(context.Background(), nil); err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if receivedID != "user@example.com" {
		t.Fatalf("expected apple ID %q, got %q", "user@example.com", receivedID)
	}
	if receivedPass != "  fixture-password  " {
		t.Fatalf("expected password %q, got %q", "  fixture-password  ", receivedPass)
	}
}

func TestResolveAppCreateSessionUsesCacheEvenWhenPasswordProvided(t *testing.T) {
	origTryResume := tryResumeSessionFn
	origTryResumeLast := tryResumeLastFn
	origWebLogin := webLoginFn
	t.Cleanup(func() {
		tryResumeSessionFn = origTryResume
		tryResumeLastFn = origTryResumeLast
		webLoginFn = origWebLogin
	})

	expected := &webcore.AuthSession{UserEmail: "cached@example.com"}
	tryResumeSessionFn = func(ctx context.Context, username string) (*webcore.AuthSession, bool, error) {
		if username != "user@example.com" {
			t.Fatalf("expected cache lookup for %q, got %q", "user@example.com", username)
		}
		return expected, true, nil
	}
	tryResumeLastFn = func(ctx context.Context) (*webcore.AuthSession, bool, error) {
		t.Fatal("did not expect last-session cache lookup when apple-id is provided")
		return nil, false, nil
	}
	webLoginFn = func(ctx context.Context, creds webcore.LoginCredentials) (*webcore.AuthSession, error) {
		t.Fatal("did not expect fresh login when cache hit succeeds")
		return nil, nil
	}

	session, source, err := resolveAppCreateSession(context.Background(), "user@example.com", "fixture-password", "")
	if err != nil {
		t.Fatalf("resolveAppCreateSession returned error: %v", err)
	}
	if source != "cache" {
		t.Fatalf("expected source %q, got %q", "cache", source)
	}
	if session != expected {
		t.Fatal("expected cached session pointer to be returned")
	}
}

func TestResolveAppCreateSessionFallsBackToFreshLoginWhenCacheLookupFails(t *testing.T) {
	origTryResume := tryResumeSessionFn
	origTryResumeLast := tryResumeLastFn
	origWebLogin := webLoginFn
	t.Cleanup(func() {
		tryResumeSessionFn = origTryResume
		tryResumeLastFn = origTryResumeLast
		webLoginFn = origWebLogin
	})

	cacheErr := errors.New("cache metadata unreadable")
	tryResumeSessionFn = func(ctx context.Context, username string) (*webcore.AuthSession, bool, error) {
		if username != "user@example.com" {
			t.Fatalf("expected cache lookup for %q, got %q", "user@example.com", username)
		}
		return nil, false, cacheErr
	}
	tryResumeLastFn = func(ctx context.Context) (*webcore.AuthSession, bool, error) {
		t.Fatal("did not expect last-session cache lookup when apple-id is provided")
		return nil, false, nil
	}
	webLoginFn = func(ctx context.Context, creds webcore.LoginCredentials) (*webcore.AuthSession, error) {
		if creds.Username != "user@example.com" {
			t.Fatalf("expected fresh login username %q, got %q", "user@example.com", creds.Username)
		}
		if creds.Password != "fixture-password" {
			t.Fatalf("expected fresh login password %q, got %q", "fixture-password", creds.Password)
		}
		return &webcore.AuthSession{UserEmail: creds.Username}, nil
	}

	session, source, err := resolveAppCreateSession(context.Background(), "user@example.com", "fixture-password", "")
	if err != nil {
		t.Fatalf("resolveAppCreateSession returned error: %v", err)
	}
	if source != "fresh" {
		t.Fatalf("expected source %q, got %q", "fresh", source)
	}
	if session == nil || session.UserEmail != "user@example.com" {
		t.Fatalf("expected fresh login session for %q, got %+v", "user@example.com", session)
	}
}

func TestResolveAppCreateSessionUsesPasswordEnvWithoutTrimming(t *testing.T) {
	origTryResume := tryResumeSessionFn
	origTryResumeLast := tryResumeLastFn
	origWebLogin := webLoginFn
	t.Cleanup(func() {
		tryResumeSessionFn = origTryResume
		tryResumeLastFn = origTryResumeLast
		webLoginFn = origWebLogin
	})

	tryResumeSessionFn = func(ctx context.Context, username string) (*webcore.AuthSession, bool, error) {
		return nil, false, nil
	}
	tryResumeLastFn = func(ctx context.Context) (*webcore.AuthSession, bool, error) {
		return nil, false, nil
	}

	var received webcore.LoginCredentials
	webLoginFn = func(ctx context.Context, creds webcore.LoginCredentials) (*webcore.AuthSession, error) {
		received = creds
		return &webcore.AuthSession{UserEmail: creds.Username}, nil
	}

	t.Setenv(webPasswordEnv, "  env-password  ")

	session, source, err := resolveAppCreateSession(context.Background(), "user@example.com", "", "")
	if err != nil {
		t.Fatalf("resolveAppCreateSession returned error: %v", err)
	}
	if source != "fresh" {
		t.Fatalf("expected fresh source, got %q", source)
	}
	if session == nil {
		t.Fatal("expected session")
	}
	if received.Password != "  env-password  " {
		t.Fatalf("expected env password %q, got %q", "  env-password  ", received.Password)
	}
}

func TestResolveAppCreateSessionPromptedAppleIDReusesCachedSession(t *testing.T) {
	origTryResume := tryResumeSessionFn
	origTryResumeLast := tryResumeLastFn
	origWebLogin := webLoginFn
	origAskOne := appCreateAskOneFn
	origCanPrompt := appCreateCanPromptInteractivelyFn
	t.Cleanup(func() {
		tryResumeSessionFn = origTryResume
		tryResumeLastFn = origTryResumeLast
		webLoginFn = origWebLogin
		appCreateAskOneFn = origAskOne
		appCreateCanPromptInteractivelyFn = origCanPrompt
	})

	expected := &webcore.AuthSession{UserEmail: "prompted@example.com"}
	tryResumeLastFn = func(ctx context.Context) (*webcore.AuthSession, bool, error) {
		return nil, false, nil
	}
	tryResumeSessionFn = func(ctx context.Context, username string) (*webcore.AuthSession, bool, error) {
		if username != "prompted@example.com" {
			t.Fatalf("expected prompted cache lookup for %q, got %q", "prompted@example.com", username)
		}
		return expected, true, nil
	}
	appCreateCanPromptInteractivelyFn = func() bool { return true }
	appCreateAskOneFn = func(p survey.Prompt, response interface{}, _ ...survey.AskOpt) error {
		prompt, ok := p.(*survey.Input)
		if !ok {
			t.Fatalf("expected apple-id input prompt, got %T", p)
		}
		if prompt.Message != "Apple ID (email):" {
			t.Fatalf("unexpected prompt message %q", prompt.Message)
		}
		target, ok := response.(*string)
		if !ok {
			t.Fatalf("expected *string response, got %T", response)
		}
		*target = "prompted@example.com"
		return nil
	}
	webLoginFn = func(ctx context.Context, creds webcore.LoginCredentials) (*webcore.AuthSession, error) {
		t.Fatal("did not expect fresh login when prompted apple-id cache hit succeeds")
		return nil, nil
	}

	session, source, err := resolveAppCreateSession(context.Background(), "", "", "")
	if err != nil {
		t.Fatalf("resolveAppCreateSession returned error: %v", err)
	}
	if source != "cache" {
		t.Fatalf("expected source %q, got %q", "cache", source)
	}
	if session != expected {
		t.Fatal("expected prompted cached session pointer to be returned")
	}
}

func TestRunAppsCreatePromptsAppleIDBeforeResolvingWhenPasswordProvided(t *testing.T) {
	origAskOne := appCreateAskOneFn
	origCanPrompt := appCreateCanPromptInteractivelyFn
	origResolveAppCreateSession := resolveAppCreateSessionFn
	origNewWebClient := newWebClientFn
	origEnsureBundleID := ensureBundleIDFn
	origCreateWebApp := createWebAppFn
	t.Cleanup(func() {
		appCreateAskOneFn = origAskOne
		appCreateCanPromptInteractivelyFn = origCanPrompt
		resolveAppCreateSessionFn = origResolveAppCreateSession
		newWebClientFn = origNewWebClient
		ensureBundleIDFn = origEnsureBundleID
		createWebAppFn = origCreateWebApp
	})

	appCreateCanPromptInteractivelyFn = func() bool { return true }
	appCreateAskOneFn = func(p survey.Prompt, response interface{}, _ ...survey.AskOpt) error {
		prompt, ok := p.(*survey.Input)
		if !ok {
			t.Fatalf("expected apple-id input prompt, got %T", p)
		}
		if prompt.Message != "Apple ID (email):" {
			t.Fatalf("unexpected prompt message %q", prompt.Message)
		}
		target, ok := response.(*string)
		if !ok {
			t.Fatalf("expected *string response, got %T", response)
		}
		*target = "prompted@example.com"
		return nil
	}

	var resolvedAppleID string
	resolveAppCreateSessionFn = func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		resolvedAppleID = appleID
		return &webcore.AuthSession{}, "cache", nil
	}
	newWebClientFn = func(session *webcore.AuthSession) *webcore.Client {
		return &webcore.Client{}
	}
	ensureBundleIDFn = func(ctx context.Context, bundleID, appName, platform string) (bool, error) {
		return false, nil
	}
	createWebAppFn = func(ctx context.Context, client *webcore.Client, attrs webcore.AppCreateAttributes) (*webcore.AppResponse, error) {
		resp := &webcore.AppResponse{}
		resp.Data.ID = "app-123"
		return resp, nil
	}

	err := RunAppsCreate(context.Background(), AppsCreateRunOptions{
		Name:                         "My App",
		BundleID:                     "com.example.app",
		SKU:                          "SKU123",
		Password:                     "fixture-password",
		PromptForAppleIDWithPassword: true,
	})
	if err != nil {
		t.Fatalf("RunAppsCreate returned error: %v", err)
	}
	if resolvedAppleID != "prompted@example.com" {
		t.Fatalf("expected prompted apple ID %q, got %q", "prompted@example.com", resolvedAppleID)
	}
}

func TestRunAppsCreatePromptsAppleIDBeforeResolvingWhenWhitespacePasswordProvided(t *testing.T) {
	origAskOne := appCreateAskOneFn
	origCanPrompt := appCreateCanPromptInteractivelyFn
	origResolveAppCreateSession := resolveAppCreateSessionFn
	origNewWebClient := newWebClientFn
	origEnsureBundleID := ensureBundleIDFn
	origCreateWebApp := createWebAppFn
	t.Cleanup(func() {
		appCreateAskOneFn = origAskOne
		appCreateCanPromptInteractivelyFn = origCanPrompt
		resolveAppCreateSessionFn = origResolveAppCreateSession
		newWebClientFn = origNewWebClient
		ensureBundleIDFn = origEnsureBundleID
		createWebAppFn = origCreateWebApp
	})

	appCreateCanPromptInteractivelyFn = func() bool { return true }
	appCreateAskOneFn = func(p survey.Prompt, response interface{}, _ ...survey.AskOpt) error {
		prompt, ok := p.(*survey.Input)
		if !ok {
			t.Fatalf("expected apple-id input prompt, got %T", p)
		}
		if prompt.Message != "Apple ID (email):" {
			t.Fatalf("unexpected prompt message %q", prompt.Message)
		}
		target, ok := response.(*string)
		if !ok {
			t.Fatalf("expected *string response, got %T", response)
		}
		*target = "prompted@example.com"
		return nil
	}

	var (
		resolvedAppleID string
		resolvedPass    string
	)
	resolveAppCreateSessionFn = func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		resolvedAppleID = appleID
		resolvedPass = password
		return &webcore.AuthSession{}, "cache", nil
	}
	newWebClientFn = func(session *webcore.AuthSession) *webcore.Client {
		return &webcore.Client{}
	}
	ensureBundleIDFn = func(ctx context.Context, bundleID, appName, platform string) (bool, error) {
		return false, nil
	}
	createWebAppFn = func(ctx context.Context, client *webcore.Client, attrs webcore.AppCreateAttributes) (*webcore.AppResponse, error) {
		resp := &webcore.AppResponse{}
		resp.Data.ID = "app-123"
		return resp, nil
	}

	err := RunAppsCreate(context.Background(), AppsCreateRunOptions{
		Name:                         "My App",
		BundleID:                     "com.example.app",
		SKU:                          "SKU123",
		Password:                     "   ",
		PromptForAppleIDWithPassword: true,
	})
	if err != nil {
		t.Fatalf("RunAppsCreate returned error: %v", err)
	}
	if resolvedAppleID != "prompted@example.com" {
		t.Fatalf("expected prompted apple ID %q, got %q", "prompted@example.com", resolvedAppleID)
	}
	if resolvedPass != "   " {
		t.Fatalf("expected whitespace password to be preserved, got %q", resolvedPass)
	}
}

func TestResolveAppCreateSessionWhitespaceOnlyPasswordFallsBackToEnv(t *testing.T) {
	origTryResume := tryResumeSessionFn
	origTryResumeLast := tryResumeLastFn
	origWebLogin := webLoginFn
	t.Cleanup(func() {
		tryResumeSessionFn = origTryResume
		tryResumeLastFn = origTryResumeLast
		webLoginFn = origWebLogin
	})

	tryResumeSessionFn = func(ctx context.Context, username string) (*webcore.AuthSession, bool, error) {
		return nil, false, nil
	}
	tryResumeLastFn = func(ctx context.Context) (*webcore.AuthSession, bool, error) {
		return nil, false, nil
	}

	var received webcore.LoginCredentials
	webLoginFn = func(ctx context.Context, creds webcore.LoginCredentials) (*webcore.AuthSession, error) {
		received = creds
		return &webcore.AuthSession{UserEmail: creds.Username}, nil
	}

	t.Setenv(webPasswordEnv, "env-password")

	if _, _, err := resolveAppCreateSession(context.Background(), "user@example.com", "   ", ""); err != nil {
		t.Fatalf("resolveAppCreateSession returned error: %v", err)
	}
	if received.Password != "env-password" {
		t.Fatalf("expected env password fallback %q, got %q", "env-password", received.Password)
	}
}

func TestPromptAppsCreatePasswordPreservesWhitespace(t *testing.T) {
	origAskOne := appCreateAskOneFn
	t.Cleanup(func() {
		appCreateAskOneFn = origAskOne
	})

	appCreateAskOneFn = func(p survey.Prompt, response interface{}, _ ...survey.AskOpt) error {
		prompt, ok := p.(*survey.Password)
		if !ok {
			t.Fatalf("expected password prompt, got %T", p)
		}
		if prompt.Message != "Apple ID password:" {
			t.Fatalf("unexpected prompt message %q", prompt.Message)
		}
		target, ok := response.(*string)
		if !ok {
			t.Fatalf("expected *string response, got %T", response)
		}
		*target = "  prompted-password  "
		return nil
	}

	password := ""
	if err := promptAppsCreatePassword(&password); err != nil {
		t.Fatalf("promptAppsCreatePassword returned error: %v", err)
	}
	if password != "  prompted-password  " {
		t.Fatalf("expected prompted password %q, got %q", "  prompted-password  ", password)
	}
}

func TestResolveAppCreateSessionPromptedWhitespacePasswordReturnsUsageError(t *testing.T) {
	origTryResume := tryResumeSessionFn
	origTryResumeLast := tryResumeLastFn
	origWebLogin := webLoginFn
	origAskOne := appCreateAskOneFn
	origCanPrompt := appCreateCanPromptInteractivelyFn
	t.Cleanup(func() {
		tryResumeSessionFn = origTryResume
		tryResumeLastFn = origTryResumeLast
		webLoginFn = origWebLogin
		appCreateAskOneFn = origAskOne
		appCreateCanPromptInteractivelyFn = origCanPrompt
	})

	tryResumeSessionFn = func(ctx context.Context, username string) (*webcore.AuthSession, bool, error) {
		return nil, false, nil
	}
	tryResumeLastFn = func(ctx context.Context) (*webcore.AuthSession, bool, error) {
		return nil, false, nil
	}
	appCreateCanPromptInteractivelyFn = func() bool { return true }
	appCreateAskOneFn = func(p survey.Prompt, response interface{}, _ ...survey.AskOpt) error {
		prompt, ok := p.(*survey.Password)
		if !ok {
			t.Fatalf("expected password prompt, got %T", p)
		}
		if prompt.Message != "Apple ID password:" {
			t.Fatalf("unexpected prompt message %q", prompt.Message)
		}
		target, ok := response.(*string)
		if !ok {
			t.Fatalf("expected *string response, got %T", response)
		}
		*target = "   "
		return nil
	}
	webLoginFn = func(ctx context.Context, creds webcore.LoginCredentials) (*webcore.AuthSession, error) {
		t.Fatal("did not expect login attempt with whitespace-only prompted password")
		return nil, nil
	}

	_, _, err := resolveAppCreateSession(context.Background(), "user@example.com", "", "")
	if err == nil {
		t.Fatal("expected usage error for whitespace-only prompted password")
	}
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected ErrHelp usage error, got %v", err)
	}
}

func TestResolveAppCreateSessionKeepsFreshSessionWhenCachePersistFails(t *testing.T) {
	origTryResume := tryResumeSessionFn
	origTryResumeLast := tryResumeLastFn
	origWebLogin := webLoginFn
	t.Cleanup(func() {
		tryResumeSessionFn = origTryResume
		tryResumeLastFn = origTryResumeLast
		webLoginFn = origWebLogin
	})

	tryResumeSessionFn = func(ctx context.Context, username string) (*webcore.AuthSession, bool, error) {
		return nil, false, nil
	}
	tryResumeLastFn = func(ctx context.Context) (*webcore.AuthSession, bool, error) {
		return nil, false, nil
	}

	cacheDir := t.TempDir()
	t.Setenv("ASC_WEB_SESSION_CACHE", "1")
	t.Setenv("ASC_WEB_SESSION_CACHE_BACKEND", "file")
	t.Setenv("ASC_WEB_SESSION_CACHE_DIR", cacheDir)
	if err := os.Chmod(cacheDir, 0o500); err != nil {
		t.Fatalf("chmod cache dir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chmod(cacheDir, 0o700)
	})

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookiejar.New error: %v", err)
	}
	expected := &webcore.AuthSession{
		Client:    &http.Client{Jar: jar},
		UserEmail: "user@example.com",
	}
	webLoginFn = func(ctx context.Context, creds webcore.LoginCredentials) (*webcore.AuthSession, error) {
		return expected, nil
	}

	session, source, err := resolveAppCreateSession(context.Background(), "user@example.com", "secret", "")
	if err != nil {
		t.Fatalf("resolveAppCreateSession returned error: %v", err)
	}
	if source != "fresh" {
		t.Fatalf("expected source %q, got %q", "fresh", source)
	}
	if session != expected {
		t.Fatal("expected fresh login session to be returned")
	}
	if err := os.Chmod(cacheDir, 0o700); err != nil {
		t.Fatalf("restore cache dir perms: %v", err)
	}
}

func TestWebAppsCreateResolvesSessionBeforeTimeoutContext(t *testing.T) {
	origResolveAppCreateSession := resolveAppCreateSessionFn
	t.Cleanup(func() {
		resolveAppCreateSessionFn = origResolveAppCreateSession
	})

	resolveErr := errors.New("stop before network call")
	hadDeadline := false
	resolveAppCreateSessionFn = func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
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

func TestWebAppsCreateInteractiveWizardPromptsForMissingFields(t *testing.T) {
	origAskOne := appCreateAskOneFn
	origResolveAppCreateSession := resolveAppCreateSessionFn
	origNewWebClient := newWebClientFn
	origEnsureBundleID := ensureBundleIDFn
	origCreateWebApp := createWebAppFn
	origCanPrompt := appCreateCanPromptInteractivelyFn
	t.Cleanup(func() {
		appCreateAskOneFn = origAskOne
		resolveAppCreateSessionFn = origResolveAppCreateSession
		newWebClientFn = origNewWebClient
		ensureBundleIDFn = origEnsureBundleID
		createWebAppFn = origCreateWebApp
		appCreateCanPromptInteractivelyFn = origCanPrompt
	})

	promptOrder := []string{}
	appCreateCanPromptInteractivelyFn = func() bool { return true }
	appCreateAskOneFn = func(p survey.Prompt, response interface{}, _ ...survey.AskOpt) error {
		switch prompt := p.(type) {
		case *survey.Input:
			promptOrder = append(promptOrder, prompt.Message)
			target, ok := response.(*string)
			if !ok {
				t.Fatalf("expected *string response for input prompt %q", prompt.Message)
			}
			switch prompt.Message {
			case "App name:":
				*target = "My App"
			case "Bundle ID:":
				*target = "com.example.app"
			case "SKU:":
				*target = "SKU123"
			case "Primary locale:":
				*target = "en-US"
			default:
				t.Fatalf("unexpected input prompt %q", prompt.Message)
			}
		case *survey.Select:
			promptOrder = append(promptOrder, prompt.Message)
			target, ok := response.(*string)
			if !ok {
				t.Fatalf("expected *string response for select prompt %q", prompt.Message)
			}
			if prompt.Message != "Platform:" {
				t.Fatalf("unexpected select prompt %q", prompt.Message)
			}
			*target = "IOS"
		default:
			t.Fatalf("unexpected prompt type %T", p)
		}
		return nil
	}
	resolveAppCreateSessionFn = func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{}, "cache", nil
	}
	newWebClientFn = func(session *webcore.AuthSession) *webcore.Client {
		return &webcore.Client{}
	}
	ensureBundleIDFn = func(ctx context.Context, bundleID, appName, platform string) (bool, error) {
		if bundleID != "com.example.app" {
			t.Fatalf("expected prompted bundle id, got %q", bundleID)
		}
		if appName != "My App" {
			t.Fatalf("expected prompted app name, got %q", appName)
		}
		if platform != "IOS" {
			t.Fatalf("expected prompted platform, got %q", platform)
		}
		return false, nil
	}
	createWebAppFn = func(ctx context.Context, client *webcore.Client, attrs webcore.AppCreateAttributes) (*webcore.AppResponse, error) {
		if attrs.Name != "My App" {
			t.Fatalf("expected prompted name, got %q", attrs.Name)
		}
		if attrs.BundleID != "com.example.app" {
			t.Fatalf("expected prompted bundle id, got %q", attrs.BundleID)
		}
		if attrs.SKU != "SKU123" {
			t.Fatalf("expected prompted sku, got %q", attrs.SKU)
		}
		if attrs.PrimaryLocale != "en-US" {
			t.Fatalf("expected prompted locale, got %q", attrs.PrimaryLocale)
		}
		if attrs.Platform != "IOS" {
			t.Fatalf("expected prompted platform, got %q", attrs.Platform)
		}
		resp := &webcore.AppResponse{}
		resp.Data.ID = "app-123"
		return resp, nil
	}

	cmd := WebAppsCreateCommand()
	if err := cmd.FlagSet.Parse([]string{"--output", "json"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	if err := cmd.Exec(context.Background(), nil); err != nil {
		t.Fatalf("expected success, got %v", err)
	}

	wantOrder := []string{"App name:", "Bundle ID:", "SKU:", "Primary locale:", "Platform:"}
	if len(promptOrder) != len(wantOrder) {
		t.Fatalf("expected prompt order %v, got %v", wantOrder, promptOrder)
	}
	for i := range wantOrder {
		if promptOrder[i] != wantOrder[i] {
			t.Fatalf("expected prompt order %v, got %v", wantOrder, promptOrder)
		}
	}
}

func TestWebAppsCreateInteractiveWizardPreservesProvidedLocaleDefault(t *testing.T) {
	origAskOne := appCreateAskOneFn
	origResolveAppCreateSession := resolveAppCreateSessionFn
	origNewWebClient := newWebClientFn
	origEnsureBundleID := ensureBundleIDFn
	origCreateWebApp := createWebAppFn
	origCanPrompt := appCreateCanPromptInteractivelyFn
	t.Cleanup(func() {
		appCreateAskOneFn = origAskOne
		resolveAppCreateSessionFn = origResolveAppCreateSession
		newWebClientFn = origNewWebClient
		ensureBundleIDFn = origEnsureBundleID
		createWebAppFn = origCreateWebApp
		appCreateCanPromptInteractivelyFn = origCanPrompt
	})

	appCreateCanPromptInteractivelyFn = func() bool { return true }
	appCreateAskOneFn = func(p survey.Prompt, response interface{}, _ ...survey.AskOpt) error {
		switch prompt := p.(type) {
		case *survey.Input:
			target, ok := response.(*string)
			if !ok {
				t.Fatalf("expected *string response for input prompt %q", prompt.Message)
			}
			switch prompt.Message {
			case "App name:":
				*target = "My App"
			case "Bundle ID:":
				*target = "com.example.app"
			case "SKU:":
				*target = "SKU123"
			case "Primary locale:":
				if prompt.Default != "de-DE" {
					t.Fatalf("expected locale prompt default %q, got %q", "de-DE", prompt.Default)
				}
				*target = prompt.Default
			default:
				t.Fatalf("unexpected input prompt %q", prompt.Message)
			}
		case *survey.Select:
			target, ok := response.(*string)
			if !ok {
				t.Fatalf("expected *string response for select prompt %q", prompt.Message)
			}
			if prompt.Message != "Platform:" {
				t.Fatalf("unexpected select prompt %q", prompt.Message)
			}
			*target = "IOS"
		default:
			t.Fatalf("unexpected prompt type %T", p)
		}
		return nil
	}
	resolveAppCreateSessionFn = func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{}, "cache", nil
	}
	newWebClientFn = func(session *webcore.AuthSession) *webcore.Client {
		return &webcore.Client{}
	}
	ensureBundleIDFn = func(ctx context.Context, bundleID, appName, platform string) (bool, error) {
		return false, nil
	}
	createWebAppFn = func(ctx context.Context, client *webcore.Client, attrs webcore.AppCreateAttributes) (*webcore.AppResponse, error) {
		if attrs.PrimaryLocale != "de-DE" {
			t.Fatalf("expected preserved locale %q, got %q", "de-DE", attrs.PrimaryLocale)
		}
		resp := &webcore.AppResponse{}
		resp.Data.ID = "app-123"
		return resp, nil
	}

	cmd := WebAppsCreateCommand()
	if err := cmd.FlagSet.Parse([]string{"--primary-locale", "de-DE", "--output", "json"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	if err := cmd.Exec(context.Background(), nil); err != nil {
		t.Fatalf("expected success, got %v", err)
	}
}

func TestWebAppsCreateSkipsBundleIDPreflightWhenOfficialAuthMissing(t *testing.T) {
	origResolveAppCreateSession := resolveAppCreateSessionFn
	origNewWebClient := newWebClientFn
	origEnsureBundleID := ensureBundleIDFn
	origCreateWebApp := createWebAppFn
	t.Cleanup(func() {
		resolveAppCreateSessionFn = origResolveAppCreateSession
		newWebClientFn = origNewWebClient
		ensureBundleIDFn = origEnsureBundleID
		createWebAppFn = origCreateWebApp
	})

	resolveAppCreateSessionFn = func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{}, "cache", nil
	}
	newWebClientFn = func(session *webcore.AuthSession) *webcore.Client {
		return &webcore.Client{}
	}

	createCalled := false
	ensureBundleIDFn = func(ctx context.Context, bundleID, appName, platform string) (bool, error) {
		return false, shared.ErrMissingAuth
	}
	createWebAppFn = func(ctx context.Context, client *webcore.Client, attrs webcore.AppCreateAttributes) (*webcore.AppResponse, error) {
		createCalled = true
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
	if !createCalled {
		t.Fatal("expected app creation to continue when official bundle-id auth is unavailable")
	}
}

func TestWebAppsCreateSkipsBundleIDPreflightWhenOfficialAuthBroken(t *testing.T) {
	origResolveAppCreateSession := resolveAppCreateSessionFn
	origNewWebClient := newWebClientFn
	origEnsureBundleID := ensureBundleIDFn
	origCreateWebApp := createWebAppFn
	t.Cleanup(func() {
		resolveAppCreateSessionFn = origResolveAppCreateSession
		newWebClientFn = origNewWebClient
		ensureBundleIDFn = origEnsureBundleID
		createWebAppFn = origCreateWebApp
	})

	resolveAppCreateSessionFn = func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{}, "cache", nil
	}
	newWebClientFn = func(session *webcore.AuthSession) *webcore.Client {
		return &webcore.Client{}
	}

	createCalled := false
	ensureBundleIDFn = func(ctx context.Context, bundleID, appName, platform string) (bool, error) {
		return false, errors.Join(errBundleIDPreflightAuthUnavailable, errors.New("broken api auth"))
	}
	createWebAppFn = func(ctx context.Context, client *webcore.Client, attrs webcore.AppCreateAttributes) (*webcore.AppResponse, error) {
		createCalled = true
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
	if !createCalled {
		t.Fatal("expected app creation to continue when official bundle-id auth is broken")
	}
}

func TestWebAppsCreateEnsuresBundleIDBeforeCreateApp(t *testing.T) {
	origResolveAppCreateSession := resolveAppCreateSessionFn
	origNewWebClient := newWebClientFn
	origEnsureBundleID := ensureBundleIDFn
	origCreateWebApp := createWebAppFn
	t.Cleanup(func() {
		resolveAppCreateSessionFn = origResolveAppCreateSession
		newWebClientFn = origNewWebClient
		ensureBundleIDFn = origEnsureBundleID
		createWebAppFn = origCreateWebApp
	})

	resolveAppCreateSessionFn = func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
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

func TestRunAppsCreateCanDisableBundleIDPreflight(t *testing.T) {
	origResolveAppCreateSession := resolveAppCreateSessionFn
	origNewWebClient := newWebClientFn
	origEnsureBundleID := ensureBundleIDFn
	origCreateWebApp := createWebAppFn
	t.Cleanup(func() {
		resolveAppCreateSessionFn = origResolveAppCreateSession
		newWebClientFn = origNewWebClient
		ensureBundleIDFn = origEnsureBundleID
		createWebAppFn = origCreateWebApp
	})

	resolveAppCreateSessionFn = func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{}, "cache", nil
	}
	newWebClientFn = func(session *webcore.AuthSession) *webcore.Client {
		return &webcore.Client{}
	}
	ensureBundleIDFn = func(ctx context.Context, bundleID, appName, platform string) (bool, error) {
		t.Fatal("did not expect bundle-id preflight when compatibility mode disables it")
		return false, nil
	}
	createCalled := false
	createWebAppFn = func(ctx context.Context, client *webcore.Client, attrs webcore.AppCreateAttributes) (*webcore.AppResponse, error) {
		createCalled = true
		resp := &webcore.AppResponse{}
		resp.Data.ID = "app-123"
		return resp, nil
	}

	err := RunAppsCreate(context.Background(), AppsCreateRunOptions{
		Name:                     "My App",
		BundleID:                 "com.example.app",
		SKU:                      "SKU123",
		AppleID:                  "user@example.com",
		Output:                   "json",
		DisableBundleIDPreflight: true,
	})
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if !createCalled {
		t.Fatal("expected app creation to continue when bundle-id preflight is disabled")
	}
}

func TestWebAppsCreateFailsWhenBundleIDPreflightFails(t *testing.T) {
	origResolveAppCreateSession := resolveAppCreateSessionFn
	origNewWebClient := newWebClientFn
	origEnsureBundleID := ensureBundleIDFn
	origCreateWebApp := createWebAppFn
	t.Cleanup(func() {
		resolveAppCreateSessionFn = origResolveAppCreateSession
		newWebClientFn = origNewWebClient
		ensureBundleIDFn = origEnsureBundleID
		createWebAppFn = origCreateWebApp
	})

	resolveAppCreateSessionFn = func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
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

func TestWebAppsCreateRollsBackCreatedBundleIDWhenCreateFails(t *testing.T) {
	origResolveAppCreateSession := resolveAppCreateSessionFn
	origNewWebClient := newWebClientFn
	origEnsureBundleID := ensureBundleIDFn
	origCreateWebApp := createWebAppFn
	origDeleteBundleID := deleteBundleIDFn
	t.Cleanup(func() {
		resolveAppCreateSessionFn = origResolveAppCreateSession
		newWebClientFn = origNewWebClient
		ensureBundleIDFn = origEnsureBundleID
		createWebAppFn = origCreateWebApp
		deleteBundleIDFn = origDeleteBundleID
	})

	resolveAppCreateSessionFn = func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{}, "cache", nil
	}
	newWebClientFn = func(session *webcore.AuthSession) *webcore.Client {
		return &webcore.Client{}
	}

	createErr := errors.New("create failed")
	deletedBundleID := ""
	ensureBundleIDFn = func(ctx context.Context, bundleID, appName, platform string) (bool, error) {
		return true, nil
	}
	createWebAppFn = func(ctx context.Context, client *webcore.Client, attrs webcore.AppCreateAttributes) (*webcore.AppResponse, error) {
		return nil, createErr
	}
	deleteBundleIDFn = func(ctx context.Context, bundleID string) error {
		deletedBundleID = bundleID
		return nil
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
	if !errors.Is(err, createErr) {
		t.Fatalf("expected wrapped create error, got %v", err)
	}
	if deletedBundleID != "com.example.app" {
		t.Fatalf("expected rollback for bundle id %q, got %q", "com.example.app", deletedBundleID)
	}
}

func TestWebAppsCreateSurfacesBundleIDRollbackFailure(t *testing.T) {
	origResolveAppCreateSession := resolveAppCreateSessionFn
	origNewWebClient := newWebClientFn
	origEnsureBundleID := ensureBundleIDFn
	origCreateWebApp := createWebAppFn
	origDeleteBundleID := deleteBundleIDFn
	t.Cleanup(func() {
		resolveAppCreateSessionFn = origResolveAppCreateSession
		newWebClientFn = origNewWebClient
		ensureBundleIDFn = origEnsureBundleID
		createWebAppFn = origCreateWebApp
		deleteBundleIDFn = origDeleteBundleID
	})

	resolveAppCreateSessionFn = func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{}, "cache", nil
	}
	newWebClientFn = func(session *webcore.AuthSession) *webcore.Client {
		return &webcore.Client{}
	}

	createErr := errors.New("create failed")
	rollbackErr := errors.New("rollback failed")
	ensureBundleIDFn = func(ctx context.Context, bundleID, appName, platform string) (bool, error) {
		return true, nil
	}
	createWebAppFn = func(ctx context.Context, client *webcore.Client, attrs webcore.AppCreateAttributes) (*webcore.AppResponse, error) {
		return nil, createErr
	}
	deleteBundleIDFn = func(ctx context.Context, bundleID string) error {
		return rollbackErr
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
	if !errors.Is(err, createErr) {
		t.Fatalf("expected wrapped create error, got %v", err)
	}
	if !errors.Is(err, rollbackErr) {
		t.Fatalf("expected rollback error to be surfaced, got %v", err)
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
