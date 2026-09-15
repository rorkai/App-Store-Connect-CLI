package cmdtest

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	cmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

type locUpdateRoundTripFunc func(*http.Request) (*http.Response, error)

func (fn locUpdateRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func setupLocUpdateAuth(t *testing.T) {
	t.Helper()
	tempDir := t.TempDir()
	keyPath := filepath.Join(tempDir, "AuthKey.p8")
	writeECDSAPEM(t, keyPath)
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_KEY_ID", "TEST_KEY")
	t.Setenv("ASC_ISSUER_ID", "TEST_ISSUER")
	t.Setenv("ASC_PRIVATE_KEY_PATH", keyPath)
}

func locUpdateJSONResponse(body string) (*http.Response, error) {
	return &http.Response{
		Status:     fmt.Sprintf("%d %s", http.StatusOK, http.StatusText(http.StatusOK)),
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}, nil
}

func TestLocalizationsUpdateRequiresLocale(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	_, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"localizations", "update", "--version", "ver-1", "--description", "test"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	if !strings.Contains(stderr, "--locale is required") {
		t.Fatalf("expected locale required error, got: %q", stderr)
	}
}

func TestLocalizationsUpdateRequiresAtLeastOneField(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	_, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"localizations", "update", "--version", "ver-1", "--locale", "en-US"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	if !strings.Contains(stderr, "at least one version field") {
		t.Fatalf("expected field required error, got: %q", stderr)
	}
}

func TestLocalizationsUpdateAppInfoRequiresApp(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	_, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"localizations", "update", "--type", "app-info", "--locale", "en-US", "--subtitle", "test"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	if !strings.Contains(stderr, "--app is required") {
		t.Fatalf("expected app required error, got: %q", stderr)
	}
}

func TestLocalizationsUpdateVersionRequiresVersion(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	_, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"localizations", "update", "--locale", "en-US", "--description", "test"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	if !strings.Contains(stderr, "--version is required") {
		t.Fatalf("expected version required error, got: %q", stderr)
	}
}

func TestLocalizationsUpdateVersionByResourceIDSkipsDiscovery(t *testing.T) {
	setupLocUpdateAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	requestCount := 0
	http.DefaultTransport = locUpdateRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		if req.Method != http.MethodPatch || req.URL.Path != "/v1/appStoreVersionLocalizations/loc-1" {
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
		}
		body, _ := io.ReadAll(req.Body)
		if !strings.Contains(string(body), `"description":"Updated"`) {
			t.Fatalf("unexpected patch body: %s", body)
		}
		return locUpdateJSONResponse(`{"data":{"type":"appStoreVersionLocalizations","id":"loc-1","attributes":{"locale":"en-US","description":"Updated"}}}`)
	})

	stdout, stderr := captureOutput(t, func() {
		code := cmd.Run([]string{
			"localizations", "update",
			"--id", "loc-1",
			"--description", "Updated",
		}, "1.2.3")
		if code != cmd.ExitSuccess {
			t.Fatalf("expected exit code %d, got %d", cmd.ExitSuccess, code)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if requestCount != 1 {
		t.Fatalf("request count = %d, want one direct PATCH", requestCount)
	}
	if !strings.Contains(stdout, `"id":"loc-1"`) {
		t.Fatalf("expected updated localization output, got %q", stdout)
	}
}

func TestLocalizationsUpdateAppInfoByResourceIDSkipsDiscovery(t *testing.T) {
	setupLocUpdateAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	requestCount := 0
	http.DefaultTransport = locUpdateRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		if req.Method != http.MethodPatch || req.URL.Path != "/v1/appInfoLocalizations/loc-1" {
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
		}
		return locUpdateJSONResponse(`{"data":{"type":"appInfoLocalizations","id":"loc-1","attributes":{"locale":"en-US","name":"Updated"}}}`)
	})

	stdout, stderr := captureOutput(t, func() {
		code := cmd.Run([]string{
			"localizations", "update",
			"--type", "app-info",
			"--id", "loc-1",
			"--name", "Updated",
		}, "1.2.3")
		if code != cmd.ExitSuccess {
			t.Fatalf("expected exit code %d, got %d", cmd.ExitSuccess, code)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if requestCount != 1 {
		t.Fatalf("request count = %d, want one direct PATCH", requestCount)
	}
	if !strings.Contains(stdout, `"id":"loc-1"`) {
		t.Fatalf("expected updated localization output, got %q", stdout)
	}
}

func TestLocalizationsUpdateResourceIDRejectsDiscoverySelectorsBeforeAuth(t *testing.T) {
	clientCalls := 0
	t.Cleanup(shared.SetASCClientFactoryForTesting(func() (*asc.Client, error) {
		clientCalls++
		return nil, errors.New("client should not be created")
	}))

	tests := [][]string{
		{"--id", "loc-1", "--version", "ver-1", "--description", "Updated"},
		{"--id", "loc-1", "--locale", "en-US", "--description", "Updated"},
		{"--type", "app-info", "--id", "loc-1", "--app", "123456789", "--name", "Updated"},
		{"--type", "app-info", "--id", "loc-1", "--app-info", "info-1", "--name", "Updated"},
	}

	for _, args := range tests {
		stdout, stderr := captureOutput(t, func() {
			code := cmd.Run(append([]string{"localizations", "update"}, args...), "1.2.3")
			if code != cmd.ExitUsage {
				t.Fatalf("args %v: expected exit code %d, got %d", args, cmd.ExitUsage, code)
			}
		})
		if stdout != "" {
			t.Fatalf("args %v: expected empty stdout, got %q", args, stdout)
		}
		if !strings.Contains(stderr, "--id cannot be combined") {
			t.Fatalf("args %v: expected selector conflict, got %q", args, stderr)
		}
	}

	if clientCalls != 0 {
		t.Fatalf("expected no client creation, got %d", clientCalls)
	}
}

func TestLocalizationsUpdate_RejectsUnsupportedLocaleWithSuggestion(t *testing.T) {
	setupLocUpdateAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	requestCount := 0
	http.DefaultTransport = locUpdateRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
		return nil, nil
	})

	stdout, stderr := captureOutput(t, func() {
		code := cmd.Run([]string{
			"localizations", "update",
			"--version", "ver-1",
			"--locale", "nl",
			"--description", "Updated description",
		}, "1.2.3")
		if code != cmd.ExitUsage {
			t.Fatalf("expected exit code %d, got %d", cmd.ExitUsage, code)
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	for _, want := range []string{`unsupported locale "nl"`, "nl-NL"} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("expected stderr to contain %q, got %q", want, stderr)
		}
	}
	if requestCount != 0 {
		t.Fatalf("expected no HTTP requests, got %d", requestCount)
	}
}

func TestLocalizationsUpdate_RejectsOverLimitKeywordBytesBeforeRequest(t *testing.T) {
	setupLocUpdateAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	requestCount := 0
	http.DefaultTransport = locUpdateRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
		return nil, nil
	})

	stdout, stderr := captureOutput(t, func() {
		code := cmd.Run([]string{
			"localizations", "update",
			"--version", "ver-1",
			"--locale", "ja",
			"--keywords", strings.Repeat("語", 101),
		}, "1.2.3")
		if code != cmd.ExitUsage {
			t.Fatalf("expected exit code %d, got %d", cmd.ExitUsage, code)
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "keywords exceed 100 characters") {
		t.Fatalf("expected keyword character-limit error, got %q", stderr)
	}
	if requestCount != 0 {
		t.Fatalf("expected no HTTP requests, got %d", requestCount)
	}
}

func TestLocalizationsUpdate_RejectsRawKeywordCharactersIncludingTrailingSpaceBeforeRequest(t *testing.T) {
	setupLocUpdateAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	requestCount := 0
	http.DefaultTransport = locUpdateRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
		return nil, nil
	})

	stdout, stderr := captureOutput(t, func() {
		code := cmd.Run([]string{
			"localizations", "update",
			"--version", "ver-1",
			"--locale", "ja",
			"--keywords", strings.Repeat("a", 100) + " ",
		}, "1.2.3")
		if code != cmd.ExitUsage {
			t.Fatalf("expected exit code %d, got %d", cmd.ExitUsage, code)
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "keywords exceed 100 characters") {
		t.Fatalf("expected keyword character-limit error, got %q", stderr)
	}
	if requestCount != 0 {
		t.Fatalf("expected no HTTP requests, got %d", requestCount)
	}
}

func TestLocalizationsUpdate_RejectsInvalidVersionFieldsBeforeClientCreation(t *testing.T) {
	tests := []struct {
		name    string
		flag    string
		value   string
		wantErr string
	}{
		{name: "description", flag: "--description", value: strings.Repeat("d", 4001), wantErr: "description exceeds 4000 characters"},
		{name: "keywords", flag: "--keywords", value: strings.Repeat("k", 101), wantErr: "keywords exceed 100 characters"},
		{name: "whats new", flag: "--whats-new", value: strings.Repeat("w", 4001), wantErr: "whatsNew exceeds 4000 characters"},
		{name: "promotional text", flag: "--promotional-text", value: strings.Repeat("p", 171), wantErr: "promotionalText exceeds 170 characters"},
		{name: "support URL", flag: "--support-url", value: "relative/support", wantErr: "supportUrl must be a valid URI"},
		{name: "marketing URL", flag: "--marketing-url", value: "relative/marketing", wantErr: "marketingUrl must be a valid URI"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			clientCalls := 0
			t.Cleanup(shared.SetASCClientFactoryForTesting(func() (*asc.Client, error) {
				clientCalls++
				return nil, errors.New("client should not be created")
			}))

			stdout, stderr := captureOutput(t, func() {
				code := cmd.Run([]string{
					"localizations", "update",
					"--version", "ver-1",
					"--locale", "en-US",
					test.flag, test.value,
				}, "1.2.3")
				if code != cmd.ExitUsage {
					t.Fatalf("expected exit code %d, got %d", cmd.ExitUsage, code)
				}
			})

			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, test.wantErr) {
				t.Fatalf("expected stderr to contain %q, got %q", test.wantErr, stderr)
			}
			if clientCalls != 0 {
				t.Fatalf("expected no client creation, got %d", clientCalls)
			}
		})
	}
}

func TestLocalizationsUpdate_AllowsForwardCompatibleLocaleCodes(t *testing.T) {
	setupLocUpdateAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	var patchBody string
	http.DefaultTransport = locUpdateRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/ver-1/appStoreVersionLocalizations":
			return locUpdateJSONResponse(`{"data":[{"type":"appStoreVersionLocalizations","id":"loc-forward","attributes":{"locale":"zh-Hant-HK","description":"Old"}}],"links":{}}`)
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/appStoreVersionLocalizations/loc-forward":
			body, _ := io.ReadAll(req.Body)
			patchBody = string(body)
			return locUpdateJSONResponse(`{"data":{"type":"appStoreVersionLocalizations","id":"loc-forward","attributes":{"locale":"zh-Hant-HK","description":"Updated description"}}}`)
		default:
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"localizations", "update",
			"--version", "ver-1",
			"--locale", "zh-Hant-HK",
			"--description", "Updated description",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(patchBody, "Updated description") {
		t.Fatalf("expected patch body to contain updated description, got %s", patchBody)
	}
	if !strings.Contains(stdout, `"locale":"zh-Hant-HK"`) {
		t.Fatalf("expected forward-compatible locale in output, got %q", stdout)
	}
}

func TestLocalizationsUpdateAppInfoSubtitle(t *testing.T) {
	setupLocUpdateAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	var patchBody string
	http.DefaultTransport = locUpdateRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		// Resolve app info ID
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/appInfos":
			return locUpdateJSONResponse(`{"data":[{"type":"appInfos","id":"appinfo-1","attributes":{}}]}`)

		// List existing localizations
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appInfos/appinfo-1/appInfoLocalizations":
			return locUpdateJSONResponse(`{"data":[{"type":"appInfoLocalizations","id":"loc-en","attributes":{"locale":"en-US","name":"MyApp","subtitle":"Old"}}],"links":{}}`)

		// Update localization
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/appInfoLocalizations/loc-en":
			body, _ := io.ReadAll(req.Body)
			patchBody = string(body)
			return locUpdateJSONResponse(`{"data":{"type":"appInfoLocalizations","id":"loc-en","attributes":{"locale":"en-US","name":"MyApp","subtitle":"New Subtitle"}}}`)

		default:
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, _ := captureOutput(t, func() {
		if err := root.Parse([]string{
			"localizations", "update",
			"--type", "app-info",
			"--app", "app-1",
			"--locale", "en-US",
			"--subtitle", "New Subtitle",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	// Verify the PATCH body contains subtitle
	if !strings.Contains(patchBody, "New Subtitle") {
		t.Fatalf("expected subtitle in PATCH body, got: %s", patchBody)
	}

	// Verify JSON output
	var result map[string]any
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v (stdout=%q)", err, stdout)
	}
}

func TestLocalizationsUpdateAppInfoFailsWhenAppInfoIsAmbiguous(t *testing.T) {
	setupLocUpdateAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	http.DefaultTransport = locUpdateRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/appInfos":
			return locUpdateJSONResponse(`{"data":[
				{"type":"appInfos","id":"appinfo-live","attributes":{"state":"READY_FOR_SALE"}},
				{"type":"appInfos","id":"appinfo-rejected","attributes":{"state":"REJECTED"}}
			]}`)
		default:
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"localizations", "update",
			"--type", "app-info",
			"--app", "app-1",
			"--locale", "en-US",
			"--subtitle", "New Subtitle",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if runErr == nil {
		t.Fatal("expected run error, got nil")
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	for _, want := range []string{
		`2 app infos match app "app-1"; pass --app-info with one of:`,
		`asc apps info list --app "app-1"`,
		"READY_FOR_SALE",
		"REJECTED",
	} {
		if !strings.Contains(runErr.Error(), want) {
			t.Fatalf("expected error to contain %q, got %v", want, runErr)
		}
	}
}

func TestLocalizationsUpdateVersionErrorIncludesAttemptedFields(t *testing.T) {
	setupLocUpdateAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	http.DefaultTransport = locUpdateRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/ver-1/appStoreVersionLocalizations":
			return locUpdateJSONResponse(`{"data":[{"type":"appStoreVersionLocalizations","id":"loc-fr","attributes":{"locale":"fr-FR","description":"Old","supportUrl":"https://example.com/support-old"}}],"links":{}}`)
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/appStoreVersionLocalizations/loc-fr":
			body := `{"errors":[{"status":"409","code":"ENTITY_ERROR.INVALID","title":"One or more parameters passed to the function were not valid.","detail":"(-50)"}]}`
			return &http.Response{
				StatusCode: http.StatusConflict,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		default:
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"localizations", "update",
			"--version", "ver-1",
			"--locale", "fr-FR",
			"--description", "Updated description",
			"--support-url", "https://example.com/support-new",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if runErr == nil {
		t.Fatal("expected run error, got nil")
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	for _, want := range []string{
		`localizations update: update version localization "fr-FR"`,
		"PATCH /v1/appStoreVersionLocalizations/loc-fr",
		"description",
		"supportUrl",
		"(-50)",
	} {
		if !strings.Contains(runErr.Error(), want) {
			t.Fatalf("expected error to contain %q, got %v", want, runErr)
		}
	}
}

func TestLocalizationsUpdateAppInfoErrorIncludesResolvedEndpoint(t *testing.T) {
	setupLocUpdateAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	http.DefaultTransport = locUpdateRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/appInfos":
			return locUpdateJSONResponse(`{"data":[{"type":"appInfos","id":"appinfo-1","attributes":{}}]}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appInfos/appinfo-1/appInfoLocalizations":
			return locUpdateJSONResponse(`{"data":[{"type":"appInfoLocalizations","id":"loc-en","attributes":{"locale":"en-US","name":"Old Name"}}],"links":{}}`)
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/appInfoLocalizations/loc-en":
			body := `{"errors":[{"status":"409","code":"ENTITY_ERROR.INVALID","title":"One or more parameters passed to the function were not valid.","detail":"(-50)"}]}`
			return &http.Response{
				StatusCode: http.StatusConflict,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		default:
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"localizations", "update",
			"--type", "app-info",
			"--app", "app-1",
			"--locale", "en-US",
			"--name", "New Name",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if runErr == nil {
		t.Fatal("expected run error, got nil")
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	for _, want := range []string{
		`localizations update: update app-info localization "en-US"`,
		"PATCH /v1/appInfoLocalizations/loc-en",
		"name",
		"(-50)",
	} {
		if !strings.Contains(runErr.Error(), want) {
			t.Fatalf("expected error to contain %q, got %v", want, runErr)
		}
	}
}
