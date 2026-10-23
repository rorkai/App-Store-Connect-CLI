package web

import (
	"context"
	"errors"
	"flag"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/AlecAivazis/survey/v2"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared/errfmt"
	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

func TestFormatAppNameWithSuffixCountsCharacters(t *testing.T) {
	tests := []struct {
		name     string
		baseName string
		suffix   string
		want     string
	}{
		{
			name:     "unicode at truncation boundary",
			baseName: "12345678901234567890123🚀Launch",
			suffix:   "app",
			want:     "12345678901234567890123🚀 - app",
		},
		{
			name:     "ascii behavior",
			baseName: "123456789012345678901234567890123",
			suffix:   "app",
			want:     "123456789012345678901234 - app",
		},
		{
			name:     "unicode suffix-only fallback",
			baseName: "A",
			suffix:   strings.Repeat("🚀", maxAppNameLen+1),
			want:     strings.Repeat("🚀", maxAppNameLen),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatAppNameWithSuffix(tt.baseName, tt.suffix)
			if !utf8.ValidString(got) {
				t.Errorf("formatAppNameWithSuffix() returned invalid UTF-8: %q", got)
			}
			if gotRunes, wantRunes := utf8.RuneCountInString(got), utf8.RuneCountInString(tt.want); gotRunes != wantRunes {
				t.Errorf("formatAppNameWithSuffix() rune count = %d, want %d", gotRunes, wantRunes)
			}
			if gotRunes := utf8.RuneCountInString(got); gotRunes > maxAppNameLen {
				t.Errorf("formatAppNameWithSuffix() rune count = %d, limit %d", gotRunes, maxAppNameLen)
			}
			if got != tt.want {
				t.Errorf("formatAppNameWithSuffix() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRunAppsCreateAutoRenamePreservesUnicode(t *testing.T) {
	origResolveAppCreateSession := resolveAppCreateSessionFn
	t.Cleanup(func() {
		resolveAppCreateSessionFn = origResolveAppCreateSession
	})

	const originalName = "12345678901234567890123🚀Launch"
	const retryName = "12345678901234567890123🚀 - app"
	var requestBodies []string
	transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("read request body: %v", err)
		}
		requestBodies = append(requestBodies, string(body))

		status := http.StatusOK
		responseBody := `{"data":{"id":"app-123","type":"apps","attributes":{}}}`
		if len(requestBodies) == 1 {
			status = http.StatusUnprocessableEntity
			responseBody = `{"errors":[{"code":"ENTITY_ERROR.ATTRIBUTE.INVALID.DUPLICATE.DIFFERENT_ACCOUNT","detail":"The app name you entered is already being used."}]}`
		}
		return &http.Response{
			StatusCode: status,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(responseBody)),
			Request:    req,
		}, nil
	})

	resolveAppCreateSessionFn = func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{Client: &http.Client{Transport: transport}}, "cache", nil
	}

	t.Setenv("ASC_WEB_MIN_REQUEST_INTERVAL", "0")
	err := RunAppsCreate(context.Background(), AppsCreateRunOptions{
		Name:       originalName,
		BundleID:   "com.example.app",
		SKU:        "SKU123",
		AppleID:    "user@example.com",
		Output:     "json",
		AutoRename: true,
	})
	if err != nil {
		t.Fatalf("RunAppsCreate returned error: %v", err)
	}
	if len(requestBodies) != 2 {
		t.Fatalf("expected 2 create requests, got %d", len(requestBodies))
	}
	if !strings.Contains(requestBodies[0], `"name":"`+originalName+`"`) {
		t.Errorf("initial serialized request did not preserve name: %s", requestBodies[0])
	}
	if !strings.Contains(requestBodies[1], `"name":"`+retryName+`"`) {
		t.Errorf("retry serialized request did not preserve name: %s", requestBodies[1])
	}
}

func TestAppCreateCanPromptInteractivelyUsesControllingTTYWhenStdinIsNotTerminal(t *testing.T) {
	origOpenTTY := openTTYFn
	origIsTerminal := termIsTerminalFn
	t.Cleanup(func() {
		openTTYFn = origOpenTTY
		termIsTerminalFn = origIsTerminal
	})

	tty, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatalf("open test TTY: %v", err)
	}
	t.Cleanup(func() { _ = tty.Close() })

	openTTYFn = func() (*os.File, error) {
		return tty, nil
	}
	termIsTerminalFn = func(fd int) bool {
		return false
	}

	if !appCreateCanPromptInteractively() {
		t.Fatal("expected controlling TTY to allow app-create prompts when stdin is not a terminal")
	}
	if _, err := tty.Stat(); err == nil {
		t.Fatal("expected controlling TTY availability probe to close its file")
	}
}

func TestAppCreateCanPromptForFieldsRequiresTerminalStdinAndStdout(t *testing.T) {
	origOpenTTY := openTTYFn
	origIsTerminal := termIsTerminalFn
	t.Cleanup(func() {
		openTTYFn = origOpenTTY
		termIsTerminalFn = origIsTerminal
	})
	openTTYFn = func() (*os.File, error) {
		return os.Open(os.DevNull)
	}

	stdinFD := int(os.Stdin.Fd())
	stdoutFD := int(os.Stdout.Fd())
	tests := []struct {
		name      string
		terminals map[int]bool
		want      bool
	}{
		{name: "stdin only", terminals: map[int]bool{stdinFD: true}, want: false},
		{name: "stdout only", terminals: map[int]bool{stdoutFD: true}, want: false},
		{name: "both", terminals: map[int]bool{stdinFD: true, stdoutFD: true}, want: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			termIsTerminalFn = func(fd int) bool { return test.terminals[fd] }
			if got := appCreateCanPromptForFields(); got != test.want {
				t.Fatalf("appCreateCanPromptForFields() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestWebAppsCreateRemovedPasswordFlagFailsBeforeSession(t *testing.T) {
	origResolveAppCreateSession := resolveAppCreateSessionFn
	origCreateWebApp := createWebAppFn
	t.Cleanup(func() {
		resolveAppCreateSessionFn = origResolveAppCreateSession
		createWebAppFn = origCreateWebApp
	})
	resolveAppCreateSessionFn = func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		t.Fatal("did not expect web session lookup")
		return nil, "", nil
	}
	createWebAppFn = func(ctx context.Context, client *webcore.Client, attrs webcore.AppCreateAttributes) (*webcore.AppResponse, error) {
		t.Fatal("did not expect app creation")
		return nil, nil
	}

	cmd := WebAppsCreateCommand()
	if err := cmd.FlagSet.Parse([]string{
		"--name", "My App",
		"--bundle-id", "com.example.app",
		"--sku", "SKU123",
		"--apple-id", "user@example.com",
		"--" + "password", "fixture-password",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	err := cmd.Exec(context.Background(), nil)
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected usage error, got %v", err)
	}
	want := "`--" + "password` was removed in 6.0.0; run `asc web auth login --apple-id EMAIL` once and rerun without it to reuse the cached web session, or set `" + webPasswordEnv + "` for unattended sign-in"
	if err.Error() != want {
		t.Fatalf("error = %q, want %q", err.Error(), want)
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
	origCanPrompt := appCreateCanPromptForFieldsFn
	t.Cleanup(func() {
		appCreateAskOneFn = origAskOne
		resolveAppCreateSessionFn = origResolveAppCreateSession
		newWebClientFn = origNewWebClient
		ensureBundleIDFn = origEnsureBundleID
		createWebAppFn = origCreateWebApp
		appCreateCanPromptForFieldsFn = origCanPrompt
	})

	promptOrder := []string{}
	appCreateCanPromptForFieldsFn = func() bool { return true }
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
	origCanPrompt := appCreateCanPromptForFieldsFn
	t.Cleanup(func() {
		appCreateAskOneFn = origAskOne
		resolveAppCreateSessionFn = origResolveAppCreateSession
		newWebClientFn = origNewWebClient
		ensureBundleIDFn = origEnsureBundleID
		createWebAppFn = origCreateWebApp
		appCreateCanPromptForFieldsFn = origCanPrompt
	})

	appCreateCanPromptForFieldsFn = func() bool { return true }
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

func TestWebAppsCreateMissingCompanyNameProvidesActionableHint(t *testing.T) {
	origResolveAppCreateSession := resolveAppCreateSessionFn
	origNewWebClient := newWebClientFn
	origEnsureBundleID := ensureBundleIDFn
	origDeleteBundleID := deleteBundleIDFn
	t.Cleanup(func() {
		resolveAppCreateSessionFn = origResolveAppCreateSession
		newWebClientFn = origNewWebClient
		ensureBundleIDFn = origEnsureBundleID
		deleteBundleIDFn = origDeleteBundleID
	})

	const secret = "account-secret-token"
	transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusUnprocessableEntity,
			Header:     make(http.Header),
			Body: io.NopCloser(strings.NewReader(`{"errors":[{
				"title":"The provided entity is missing a required attribute",
				"detail":"You must provide a value for the attribute 'companyName' with this request",
				"code":"ENTITY_ERROR.ATTRIBUTE.REQUIRED",
				"secret":"` + secret + `"
			}]}`)),
			Request: req,
		}, nil
	})
	resolveAppCreateSessionFn = func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{Client: &http.Client{Transport: transport}}, "cache", nil
	}
	newWebClientFn = webcore.NewClient
	ensureBundleIDFn = func(ctx context.Context, bundleID, appName, platform string) (bool, error) {
		return true, nil
	}
	deletedBundleID := ""
	deleteBundleIDFn = func(ctx context.Context, bundleID string) error {
		deletedBundleID = bundleID
		return nil
	}
	t.Setenv("ASC_WEB_MIN_REQUEST_INTERVAL", "0")

	err := RunAppsCreate(context.Background(), AppsCreateRunOptions{
		Name:     "My App",
		BundleID: "com.example.app",
		SKU:      "SKU123",
		AppleID:  "user@example.com",
		Output:   "json",
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "Apple requires a company name for this account") {
		t.Fatalf("error = %q, want actionable company-name guidance", err)
	}
	if !strings.Contains(err.Error(), "--company-name") {
		t.Fatalf("error = %q, want --company-name hint", err)
	}
	if strings.Contains(err.Error(), secret) {
		t.Fatalf("error leaked raw response body secret: %q", err)
	}
	if got := errfmt.FormatStderr(err); !strings.Contains(got, "Error: web apps create failed:") {
		t.Fatalf("formatted stderr = %q, want command error prefix", got)
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
	t.Run("keeps universal bundle id platform", func(t *testing.T) {
		got, err := bundleIDPlatformForWebApp("UNIVERSAL")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != asc.BundleIDPlatformUniversal {
			t.Fatalf("expected %q, got %q", asc.BundleIDPlatformUniversal, got)
		}
	})

	t.Run("maps tvOS app to iOS bundle id platform", func(t *testing.T) {
		got, err := bundleIDPlatformForWebApp("TV_OS")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != asc.BundleIDPlatformIOS {
			t.Fatalf("expected %q, got %q", asc.BundleIDPlatformIOS, got)
		}
	})

	t.Run("keeps explicit mac platform", func(t *testing.T) {
		got, err := bundleIDPlatformForWebApp("MAC_OS")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != asc.BundleIDPlatformMacOS {
			t.Fatalf("expected %q, got %q", asc.BundleIDPlatformMacOS, got)
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

func TestWebAppsDeleteByBundleID(t *testing.T) {
	restoreSession := SetResolveWebSession(func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{}, "cache", nil
	})
	origNewWebClient := newWebClientFn
	origFindWebApp := findWebAppFn
	origGetRemovalState := getWebAppRemovalStateFn
	origGetAvailability := getWebAppAvailabilityFn
	origDeleteWebApp := deleteWebAppFn
	t.Cleanup(func() {
		restoreSession()
		newWebClientFn = origNewWebClient
		findWebAppFn = origFindWebApp
		getWebAppRemovalStateFn = origGetRemovalState
		getWebAppAvailabilityFn = origGetAvailability
		deleteWebAppFn = origDeleteWebApp
	})

	var deletedID string
	var removalReads int
	newWebClientFn = func(session *webcore.AuthSession) *webcore.Client {
		return &webcore.Client{}
	}
	findWebAppFn = func(ctx context.Context, client *webcore.Client, bundleID string) (*webcore.AppResponse, error) {
		if bundleID != "com.example.throwaway" {
			t.Fatalf("expected bundle ID lookup, got %q", bundleID)
		}
		resp := &webcore.AppResponse{}
		resp.Data.ID = "1234567890"
		resp.Data.Type = "apps"
		resp.Data.Attributes = map[string]any{
			"name":     "Throwaway",
			"bundleId": "com.example.throwaway",
		}
		return resp, nil
	}
	getWebAppAvailabilityFn = func(ctx context.Context, client *webcore.Client, appID string) (*webcore.AppAvailability, error) {
		return &webcore.AppAvailability{ID: "avail-1", AvailableTerritories: []string{}, AvailableTerritoriesLoaded: true, AvailableInNewTerritoriesKnown: true}, nil
	}
	getWebAppRemovalStateFn = func(ctx context.Context, client *webcore.Client, appID string) (*webcore.AppRemovalState, error) {
		removalReads++
		return &webcore.AppRemovalState{
			ID:                        appID,
			Name:                      "Throwaway",
			BundleID:                  "com.example.throwaway",
			Removed:                   deletedID != "",
			RemovedKnown:              true,
			AppStoreLegacyStatus:      "PREPARE_FOR_SUBMISSION",
			Marketplace:               "APP_STORE",
			DisplayableVersionsLoaded: true,
		}, nil
	}
	deleteWebAppFn = func(ctx context.Context, client *webcore.Client, appID string) (*webcore.AppResponse, error) {
		deletedID = appID
		resp := &webcore.AppResponse{}
		resp.Data.ID = appID
		resp.Data.Type = "apps"
		return resp, nil
	}

	cmd := WebAppsDeleteCommand()
	if err := cmd.FlagSet.Parse([]string{
		"--app", "com.example.throwaway",
		"--expected-bundle-id", "com.example.throwaway",
		"--confirm",
		"--output", "json",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	stdout, stderr := captureOutput(t, func() {
		if err := cmd.Exec(context.Background(), nil); err != nil {
			t.Fatalf("expected success, got %v", err)
		}
	})
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if deletedID != "1234567890" {
		t.Fatalf("expected deleted app ID, got %q", deletedID)
	}
	if removalReads < 2 {
		t.Fatalf("expected preflight and post-PATCH re-read, got %d", removalReads)
	}
	for _, want := range []string{`"appId":"1234567890"`, `"bundleId":"com.example.throwaway"`, `"removed":true`} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("expected stdout to contain %s, got %q", want, stdout)
		}
	}
}

func TestWebAppDeleteResultFromStateUsesServerRemovedFlag(t *testing.T) {
	state := &webcore.AppRemovalState{
		ID:           "1234567890",
		Name:         "Throwaway",
		BundleID:     "com.example.throwaway",
		Removed:      false,
		RemovedKnown: true,
	}

	result := webAppDeleteResultFromState(state, false)
	if result.AppID != "1234567890" {
		t.Fatalf("expected app ID from server state, got %q", result.AppID)
	}
	if result.Name != "Throwaway" {
		t.Fatalf("expected name from server state, got %q", result.Name)
	}
	if result.BundleID != "com.example.throwaway" {
		t.Fatalf("expected bundle ID from server state, got %q", result.BundleID)
	}
	if result.Removed {
		t.Fatal("expected removed to follow server state, not the request")
	}
	if result.DryRun {
		t.Fatal("did not expect dry-run on a mutation receipt")
	}
}

func TestWebAppsDeleteBundleIDLookupMismatchStopsBeforeDelete(t *testing.T) {
	restoreSession := SetResolveWebSession(func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{}, "cache", nil
	})
	origNewWebClient := newWebClientFn
	origFindWebApp := findWebAppFn
	origDeleteWebApp := deleteWebAppFn
	t.Cleanup(func() {
		restoreSession()
		newWebClientFn = origNewWebClient
		findWebAppFn = origFindWebApp
		deleteWebAppFn = origDeleteWebApp
	})

	newWebClientFn = func(session *webcore.AuthSession) *webcore.Client {
		return &webcore.Client{}
	}
	findWebAppFn = func(ctx context.Context, client *webcore.Client, bundleID string) (*webcore.AppResponse, error) {
		resp := &webcore.AppResponse{}
		resp.Data.ID = "1234567890"
		resp.Data.Type = "apps"
		resp.Data.Attributes = map[string]any{
			"name":     "Wrong App",
			"bundleId": "com.example.actual",
		}
		return resp, nil
	}
	deleteWebAppFn = func(ctx context.Context, client *webcore.Client, appID string) (*webcore.AppResponse, error) {
		t.Fatal("did not expect delete after bundle lookup mismatch")
		return nil, nil
	}

	cmd := WebAppsDeleteCommand()
	if err := cmd.FlagSet.Parse([]string{
		"--app", "com.example.expected",
		"--confirm",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	var err error
	stdout, stderr := captureOutput(t, func() {
		err = cmd.Exec(context.Background(), nil)
	})
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if err == nil {
		t.Fatal("expected bundle lookup mismatch error")
	}
	if !strings.Contains(err.Error(), `exact bundle ID lookup for "com.example.expected" returned "com.example.actual"`) {
		t.Fatalf("expected lookup mismatch error, got %v", err)
	}
}

func TestWebAppsDeleteExpectedBundleIDMismatchStopsBeforeDelete(t *testing.T) {
	restoreSession := SetResolveWebSession(func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{}, "cache", nil
	})
	origNewWebClient := newWebClientFn
	origGetRemovalState := getWebAppRemovalStateFn
	origDeleteWebApp := deleteWebAppFn
	t.Cleanup(func() {
		restoreSession()
		newWebClientFn = origNewWebClient
		getWebAppRemovalStateFn = origGetRemovalState
		deleteWebAppFn = origDeleteWebApp
	})

	newWebClientFn = func(session *webcore.AuthSession) *webcore.Client {
		return &webcore.Client{}
	}
	getWebAppRemovalStateFn = func(ctx context.Context, client *webcore.Client, appID string) (*webcore.AppRemovalState, error) {
		return &webcore.AppRemovalState{
			ID:       appID,
			Name:     "Throwaway",
			BundleID: "com.example.actual",
		}, nil
	}
	deleteWebAppFn = func(ctx context.Context, client *webcore.Client, appID string) (*webcore.AppResponse, error) {
		t.Fatal("did not expect delete after identity guard mismatch")
		return nil, nil
	}

	cmd := WebAppsDeleteCommand()
	if err := cmd.FlagSet.Parse([]string{
		"--app", "1234567890",
		"--expected-bundle-id", "com.example.expected",
		"--confirm",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	var err error
	stdout, stderr := captureOutput(t, func() {
		err = cmd.Exec(context.Background(), nil)
	})
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if err == nil {
		t.Fatal("expected guard mismatch error")
	}
	if !strings.Contains(err.Error(), `expected bundle ID "com.example.expected"`) {
		t.Fatalf("expected bundle mismatch error, got %v", err)
	}
}

func TestWebAppsDeleteRequiresConfirmBeforeResolvingSession(t *testing.T) {
	resolveCalled := false
	restoreSession := SetResolveWebSession(func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		resolveCalled = true
		return &webcore.AuthSession{}, "cache", nil
	})
	t.Cleanup(restoreSession)

	cmd := WebAppsDeleteCommand()
	if err := cmd.FlagSet.Parse([]string{"--app", "1234567890"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	var err error
	stdout, stderr := captureOutput(t, func() {
		err = cmd.Exec(context.Background(), nil)
	})
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "--confirm is required unless --dry-run is set") {
		t.Fatalf("expected confirm stderr, got %q", stderr)
	}
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected usage error, got %v", err)
	}
	if !strings.Contains(err.Error(), "--confirm is required unless --dry-run is set") {
		t.Fatalf("expected confirm error, got %v", err)
	}
	if resolveCalled {
		t.Fatal("did not expect session resolution before confirm validation")
	}
}

func serveBundleIDPrefixMatches(t *testing.T, pages map[string]string) *[]string {
	t.Helper()
	var writes []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/bundleIds":
			_, _ = io.WriteString(w, pages[req.URL.Query().Get("cursor")])
		case req.Method == http.MethodPost && req.URL.Path == "/v1/bundleIds":
			body, _ := io.ReadAll(req.Body)
			writes = append(writes, "POST "+string(body))
			w.WriteHeader(http.StatusCreated)
			_, _ = io.WriteString(w, `{"data":{"type":"bundleIds","id":"id-new","attributes":{"identifier":"com.acme.app"}}}`)
		case req.Method == http.MethodDelete:
			writes = append(writes, "DELETE "+req.URL.Path)
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Errorf("unexpected request: %s %s", req.Method, req.URL.String())
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	t.Cleanup(server.Close)
	setAppCreateASCClient(t, server)
	return &writes
}

func TestEnsureBundleIDExistsCreatesWhenOnlyPrefixSiblingsMatch(t *testing.T) {
	writes := serveBundleIDPrefixMatches(t, map[string]string{
		"": `{"data":[{"type":"bundleIds","id":"id-widget","attributes":{"identifier":"com.acme.app.widget"}}],"links":{}}`,
	})

	created, err := ensureBundleIDExists(context.Background(), "com.acme.app", "Acme", "IOS")
	if err != nil {
		t.Fatalf("ensureBundleIDExists: %v", err)
	}
	if !created || len(*writes) != 1 || !strings.Contains((*writes)[0], `"identifier":"com.acme.app"`) {
		t.Fatalf("created=%v writes=%v, want com.acme.app created", created, *writes)
	}
}

func TestDeleteBundleIDByIdentifierNeverTargetsPrefixSibling(t *testing.T) {
	tests := []struct {
		name  string
		pages map[string]string
		want  []string
	}{
		{
			name: "exact match on a later page",
			pages: map[string]string{
				"":  `{"data":[{"type":"bundleIds","id":"id-widget","attributes":{"identifier":"com.acme.app.widget"}}],"links":{"next":"https://api.appstoreconnect.apple.com/v1/bundleIds?cursor=2"}}`,
				"2": `{"data":[{"type":"bundleIds","id":"id-app","attributes":{"identifier":"com.acme.app"}}],"links":{}}`,
			},
			want: []string{"DELETE /v1/bundleIds/id-app"},
		},
		{
			name: "only prefix siblings",
			pages: map[string]string{
				"": `{"data":[{"type":"bundleIds","id":"id-widget","attributes":{"identifier":"com.acme.app.widget"}}],"links":{}}`,
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			writes := serveBundleIDPrefixMatches(t, test.pages)
			if err := deleteBundleIDByIdentifier(context.Background(), "com.acme.app"); err != nil {
				t.Fatalf("deleteBundleIDByIdentifier: %v", err)
			}
			if strings.Join(*writes, ",") != strings.Join(test.want, ",") {
				t.Fatalf("writes = %v, want %v", *writes, test.want)
			}
		})
	}
}
