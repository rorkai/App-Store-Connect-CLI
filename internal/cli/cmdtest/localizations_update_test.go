package cmdtest

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	cmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

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

func TestLocalizationsUpdateValidationErrors(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "missing locale",
			args:    []string{"localizations", "update", "--version", "ver-1", "--description", "test"},
			wantErr: "--locale is required",
		},
		{
			name:    "missing version field",
			args:    []string{"localizations", "update", "--version", "ver-1", "--locale", "en-US"},
			wantErr: "at least one update flag is required (--description, --keywords, --whats-new, --promotional-text, --clear-promotional-text, --support-url, --marketing-url)",
		},
		{
			name:    "app-info field requires app-info type",
			args:    []string{"localizations", "update", "--id", "loc-1", "--name", "Updated"},
			wantErr: "--type app-info is required for app-info update flags",
		},
		{
			name:    "app-info field mixed with version field is refused",
			args:    []string{"localizations", "update", "--id", "loc-1", "--name", "Updated", "--description", "Desc"},
			wantErr: "--type app-info is required for app-info update flags",
		},
		{
			name:    "app-info missing app",
			args:    []string{"localizations", "update", "--type", "app-info", "--locale", "en-US", "--subtitle", "test"},
			wantErr: "--app is required",
		},
		{
			name:    "version missing version",
			args:    []string{"localizations", "update", "--locale", "en-US", "--description", "test"},
			wantErr: "--version is required",
		},
	}

	runValidationTests(t, tests)
}

func TestLocalizationsUpdateByResourceIDSkipsDiscovery(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		path     string
		wantBody string
		response string
	}{
		{
			name:     "version",
			args:     []string{"--description", "Updated"},
			path:     "/v1/appStoreVersionLocalizations/loc-1",
			wantBody: `"description":"Updated"`,
			response: `{"data":{"type":"appStoreVersionLocalizations","id":"loc-1","attributes":{"locale":"en-US","description":"Updated"}}}`,
		},
		{
			name:     "app-info",
			args:     []string{"--type", "app-info", "--name", "Updated"},
			path:     "/v1/appInfoLocalizations/loc-1",
			wantBody: `"name":"Updated"`,
			response: `{"data":{"type":"appInfoLocalizations","id":"loc-1","attributes":{"locale":"en-US","name":"Updated"}}}`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupLocUpdateAuth(t)

			reg := asctest.NewRegistry(t)
			reg.Install()
			reg.Register(asctest.PATCH(test.path), asctest.JSON(http.StatusOK, test.response))

			stdout, stderr := captureOutput(t, func() {
				args := append([]string{"localizations", "update", "--id", "loc-1"}, test.args...)
				if code := cmd.Run(args, "1.2.3"); code != cmd.ExitSuccess {
					t.Fatalf("expected exit code %d, got %d", cmd.ExitSuccess, code)
				}
			})

			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}
			requests := reg.Requests()
			if requestCount := len(requests); requestCount != 1 {
				t.Fatalf("request count = %d, want one direct PATCH", requestCount)
			}
			if !strings.Contains(string(requests[0].Body), test.wantBody) {
				t.Fatalf("unexpected patch body: %s", requests[0].Body)
			}
			if !strings.Contains(stdout, `"id":"loc-1"`) {
				t.Fatalf("expected updated localization output, got %q", stdout)
			}
		})
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
		{"--id", "loc-1", "--platform", "", "--description", "Updated"},
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

	reg := asctest.NewRegistry(t)
	reg.Install()

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
	if requestCount := len(reg.Requests()); requestCount != 0 {
		t.Fatalf("expected no HTTP requests, got %d", requestCount)
	}
}

func TestKeywordWritersRejectOverLimitKeywordsBeforeAuthResolution(t *testing.T) {
	overLimit := strings.Repeat("語", 101)
	trailingSpace := strings.Repeat("a", 100) + " "
	writeStrings := func(t *testing.T, keywords string) string {
		t.Helper()
		dir := t.TempDir()
		content := "\"description\" = \"日本語説明\";\n\"keywords\" = \"" + keywords + "\";\n"
		if err := os.WriteFile(filepath.Join(dir, "ja.strings"), []byte(content), 0o644); err != nil {
			t.Fatalf("write strings file: %v", err)
		}
		return dir
	}

	tests := []struct {
		name string
		args func(t *testing.T) []string
	}{
		{
			name: "localizations create",
			args: func(*testing.T) []string {
				return []string{"localizations", "create", "--version", "version-1", "--locale", "ja", "--keywords", overLimit}
			},
		},
		{
			name: "localizations update",
			args: func(*testing.T) []string {
				return []string{"localizations", "update", "--version", "ver-1", "--locale", "ja", "--keywords", overLimit}
			},
		},
		{
			name: "localizations update counts trailing space",
			args: func(*testing.T) []string {
				return []string{"localizations", "update", "--version", "ver-1", "--locale", "ja", "--keywords", trailingSpace}
			},
		},
		{
			name: "localizations upload",
			args: func(t *testing.T) []string {
				return []string{"localizations", "upload", "--version", "version-1", "--path", writeStrings(t, overLimit)}
			},
		},
		{
			name: "localizations upload counts trailing space",
			args: func(t *testing.T) []string {
				return []string{"localizations", "upload", "--version", "version-1", "--path", writeStrings(t, trailingSpace)}
			},
		},
		{
			name: "metadata push",
			args: func(t *testing.T) []string {
				dir := t.TempDir()
				versionDir := filepath.Join(dir, "version", "1.2.3")
				if err := os.MkdirAll(versionDir, 0o755); err != nil {
					t.Fatalf("mkdir version dir: %v", err)
				}
				if err := os.WriteFile(filepath.Join(versionDir, "ja.json"), []byte(`{"keywords":"`+overLimit+`"}`), 0o644); err != nil {
					t.Fatalf("write version file: %v", err)
				}
				return []string{"metadata", "push", "--app", "app-1", "--version", "1.2.3", "--dir", dir}
			},
		},
		{
			name: "metadata keywords push",
			args: func(t *testing.T) []string {
				inputPath := filepath.Join(t.TempDir(), "keywords.json")
				if err := os.WriteFile(inputPath, []byte(`{"ja":"`+overLimit+`"}`), 0o600); err != nil {
					t.Fatalf("WriteFile() error: %v", err)
				}
				return []string{"metadata", "keywords", "push", "--version-id", "ver-1", "--input", inputPath}
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
			t.Setenv("ASC_KEY_ID", "")
			t.Setenv("ASC_ISSUER_ID", "")
			t.Setenv("ASC_PRIVATE_KEY_PATH", "")
			t.Setenv("ASC_PRIVATE_KEY", "")
			t.Setenv("ASC_PRIVATE_KEY_B64", "")
			t.Setenv("ASC_APP_ID", "")
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
			asctest.NewRegistry(t).Install()
			args := test.args(t)

			stdout, stderr := captureOutput(t, func() {
				if code := cmd.Run(args, "1.2.3"); code != cmd.ExitUsage {
					t.Fatalf("expected exit code %d, got %d", cmd.ExitUsage, code)
				}
			})

			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, "keywords exceed 100 characters") {
				t.Fatalf("expected keyword character-limit error, got %q", stderr)
			}
		})
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

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/appStoreVersions/ver-1/appStoreVersionLocalizations", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-forward","attributes":{"locale":"zh-Hant-HK","description":"Old"}}],"links":{}}`))
	reg.Register(asctest.PATCH("/v1/appStoreVersionLocalizations/loc-forward").JSONBody(`{"data":{"type":"appStoreVersionLocalizations","id":"loc-forward","attributes":{"description":"Updated description"}}}`),
		asctest.JSON(http.StatusOK, `{"data":{"type":"appStoreVersionLocalizations","id":"loc-forward","attributes":{"locale":"zh-Hant-HK","description":"Updated description"}}}`))

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
	var patchBody string
	for _, req := range reg.Requests() {
		if req.Method == http.MethodPatch {
			patchBody = string(req.Body)
		}
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

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/apps/app-1/appInfos"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"appInfos","id":"appinfo-1","attributes":{}}]}`))
	reg.Register(asctest.GET("/v1/appInfos/appinfo-1/appInfoLocalizations", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"appInfoLocalizations","id":"loc-en","attributes":{"locale":"en-US","name":"MyApp","subtitle":"Old"}}],"links":{}}`))
	reg.Register(asctest.PATCH("/v1/appInfoLocalizations/loc-en").JSONBody(`{"data":{"type":"appInfoLocalizations","id":"loc-en","attributes":{"subtitle":"New Subtitle"}}}`),
		asctest.JSON(http.StatusOK, `{"data":{"type":"appInfoLocalizations","id":"loc-en","attributes":{"locale":"en-US","name":"MyApp","subtitle":"New Subtitle"}}}`))

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
	var patchBody string
	for _, req := range reg.Requests() {
		if req.Method == http.MethodPatch {
			patchBody = string(req.Body)
		}
	}
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

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/apps/app-1/appInfos"),
		asctest.JSON(http.StatusOK, `{"data":[
				{"type":"appInfos","id":"appinfo-live","attributes":{"state":"READY_FOR_SALE"}},
				{"type":"appInfos","id":"appinfo-rejected","attributes":{"state":"REJECTED"}}
			]}`))

	stdout, stderr, runErr := runCommand(t, []string{
		"localizations", "update",
		"--type", "app-info",
		"--app", "app-1",
		"--locale", "en-US",
		"--subtitle", "New Subtitle",
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

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/appStoreVersions/ver-1/appStoreVersionLocalizations", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-fr","attributes":{"locale":"fr-FR","description":"Old","supportUrl":"https://example.com/support-old"}}],"links":{}}`))
	reg.Register(asctest.PATCH("/v1/appStoreVersionLocalizations/loc-fr").JSONBody(`{"data":{"type":"appStoreVersionLocalizations","id":"loc-fr","attributes":{"description":"Updated description","supportUrl":"https://example.com/support-new"}}}`),
		asctest.JSON(http.StatusConflict, `{"errors":[{"status":"409","code":"ENTITY_ERROR.INVALID","title":"One or more parameters passed to the function were not valid.","detail":"(-50)"}]}`))

	stdout, stderr, runErr := runCommand(t, []string{
		"localizations", "update",
		"--version", "ver-1",
		"--locale", "fr-FR",
		"--description", "Updated description",
		"--support-url", "https://example.com/support-new",
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

func TestLocalizationsUpdateMissingAppInfoLocaleSuggestsCreation(t *testing.T) {
	setupLocUpdateAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))
	stubTransport(t, func(req *http.Request) (*http.Response, error) {
		switch req.Method + " " + req.URL.Path {
		case "GET /v1/appInfos/selected-info":
			return locUpdateJSONResponse(`{"data":{"type":"appInfos","id":"selected-info","relationships":{"app":{"data":{"type":"apps","id":"app-1"}}}}}`)
		case "GET /v1/appInfos/selected-info/appInfoLocalizations":
			return locUpdateJSONResponse(`{"data":[],"links":{}}`)
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
			return nil, errors.New("unexpected request")
		}
	})
	stdout, stderr := captureOutput(t, func() {
		if code := cmd.Run([]string{"localizations", "update", "--type", "app-info", "--app", "app-1", "--app-info", "selected-info", "--locale", "fr-FR", "--subtitle", "Subtitle"}, "1.2.3"); code != cmd.ExitError {
			t.Fatalf("exit code = %d, want %d", code, cmd.ExitError)
		}
	})
	if stdout != "" || !strings.Contains(stderr, `no existing localization found for locale "fr-FR"`) || !strings.Contains(stderr, `asc apps rename --app "app-1" --app-info "selected-info" --locale "fr-FR" --name "APP_NAME"`) {
		t.Fatalf("stdout=%q stderr=%q, want missing locale and creation command", stdout, stderr)
	}
}

func TestLocalizationsUpdateAppInfoErrorIncludesResolvedEndpoint(t *testing.T) {
	setupLocUpdateAuth(t)

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/apps/app-1/appInfos"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"appInfos","id":"appinfo-1","attributes":{}}]}`))
	reg.Register(asctest.GET("/v1/appInfos/appinfo-1/appInfoLocalizations", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"appInfoLocalizations","id":"loc-en","attributes":{"locale":"en-US","name":"Old Name"}}],"links":{}}`))
	reg.Register(asctest.PATCH("/v1/appInfoLocalizations/loc-en").JSONBody(`{"data":{"type":"appInfoLocalizations","id":"loc-en","attributes":{"name":"New Name"}}}`),
		asctest.JSON(http.StatusConflict, `{"errors":[{"status":"409","code":"ENTITY_ERROR.INVALID","title":"One or more parameters passed to the function were not valid.","detail":"(-50)"}]}`))

	stdout, stderr, runErr := runCommand(t, []string{
		"localizations", "update",
		"--type", "app-info",
		"--app", "app-1",
		"--locale", "en-US",
		"--name", "New Name",
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

func TestLocalizationsMutationSelectorValidationBeforeAuth(t *testing.T) {
	clientCalls := 0
	t.Cleanup(shared.SetASCClientFactoryForTesting(func() (*asc.Client, error) {
		clientCalls++
		return nil, errors.New("unexpected client creation")
	}))
	t.Setenv("ASC_APP_ID", "")
	for _, command := range []string{"create", "update"} {
		for _, test := range []struct {
			name   string
			args   []string
			want   string
			appEnv string
		}{
			{"string needs app", []string{"--version", "1.17.0"}, "--app is required (or set ASC_APP_ID) when --version is a version string", ""},
			{"blank app", []string{"--version", "1.17.0", "--app", " \t"}, "Error: --app must not be empty", ""},
			{"blank app overrides environment", []string{"--version", "1.17.0", "--app", " \t"}, "Error: --app must not be empty", "app-1"},
			{"ID rejects platform", []string{"--version", "ver-1", "--platform", "IOS"}, "--platform requires a version string with --app", ""},
			{"empty platform", []string{"--version", "ver-1", "--platform", ""}, "--platform must not be empty", ""},
			{"invalid platform", []string{"--app", "app-1", "--version", "1.17.0", "--platform", "INVALID"}, "--platform must be one of: IOS, MAC_OS, TV_OS, VISION_OS", ""},
		} {
			t.Run(command+"/"+test.name, func(t *testing.T) {
				t.Setenv("ASC_APP_ID", test.appEnv)
				args := append([]string{"localizations", command, "--locale", "en-US", "--description", "Description"}, test.args...)
				stdout, stderr := captureOutput(t, func() {
					if code := cmd.Run(args, "test"); code != cmd.ExitUsage {
						t.Fatalf("exit=%d, want usage; args=%v", code, args)
					}
				})
				if stdout != "" || !strings.Contains(stderr, test.want) {
					t.Fatalf("stdout=%q stderr=%q, want %q", stdout, stderr, test.want)
				}
			})
		}
	}
	if clientCalls != 0 {
		t.Fatalf("client calls=%d, want 0", clientCalls)
	}
}

// Apple's 409 on PATCH /v1/appStoreVersionLocalizations/{id}, captured live on
// 2026-11-22 for whatsNew on the first version of a never-released app. Apple
// sends the same shape, naming the attribute, for every field of a version
// that is not editable.
func locUpdateNotEditableBody(attribute string) string {
	return `{"errors":[{"id":"9f915c91-9e7e-4542-a15d-096545024364","status":"409","code":"STATE_ERROR","title":"The request cannot be fulfilled because of the state of another resource.","detail":"Attribute '` + attribute + `' cannot be edited at this time"}]}`
}

func TestLocalizationsUpdateConflictNamesVersionStateAndEditableVersion(t *testing.T) {
	version := func(id, versionString, state string) string {
		return `{"type":"appStoreVersions","id":"` + id + `","attributes":{"platform":"IOS","versionString":"` + versionString + `","appStoreState":"` + state + `","appVersionState":"` + state + `"}}`
	}
	tests := []struct {
		name       string
		args       []string
		attribute  string
		versionID  string
		byID       bool
		current    string
		appRows    string
		wantStderr string
	}{
		{
			name:       "whatsNew on the first version of an unreleased app",
			args:       []string{"--id", "loc-1", "--whats-new", "Bug fixes"},
			attribute:  "whatsNew",
			byID:       true,
			current:    version("ver-1", "1.0", "PREPARE_FOR_SUBMISSION"),
			appRows:    version("ver-1", "1.0", "PREPARE_FOR_SUBMISSION"),
			wantStderr: "\nHint: Version 1.0 (IOS) is PREPARE_FOR_SUBMISSION, but the app has no released version on IOS yet, and App Store Connect takes whatsNew only for an update to a released app. Retry without --whats-new.\n",
		},
		{
			name:       "released version with a newer editable one",
			args:       []string{"--version", "ver-1", "--locale", "en-US", "--keywords", "focus,timer"},
			attribute:  "keywords",
			current:    version("ver-1", "1.0", "READY_FOR_DISTRIBUTION"),
			appRows:    version("ver-2", "1.1", "PREPARE_FOR_SUBMISSION") + `,` + version("ver-1", "1.0", "READY_FOR_DISTRIBUTION"),
			wantStderr: "\nHint: Version 1.0 (IOS, id ver-1) is READY_FOR_DISTRIBUTION. App Store Connect edits description, keywords, whatsNew, supportUrl and marketingUrl only while a version is PREPARE_FOR_SUBMISSION, DEVELOPER_REJECTED, REJECTED, METADATA_REJECTED, INVALID_BINARY; promotionalText can change in any state.\nVersion 1.1 (PREPARE_FOR_SUBMISSION) is editable: retry with --version \"ver-2\" --locale \"en-US\".\n",
		},
		{
			name:       "version in review and no editable one",
			args:       []string{"--id", "loc-1", "--description", "New"},
			attribute:  "description",
			byID:       true,
			current:    version("ver-1", "1.0", "WAITING_FOR_REVIEW"),
			appRows:    version("ver-1", "1.0", "WAITING_FOR_REVIEW"),
			wantStderr: "\nNo IOS version is editable: create the next one with `asc versions create --app \"app-1\" --version NEXT_VERSION --platform IOS`, then retry on it.\n",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("ASC_APP_ID", "")
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
			reg := asctest.NewRegistry(t)
			if !test.byID {
				reg.Register(asctest.GET("/v1/appStoreVersions/ver-1/appStoreVersionLocalizations", "limit=200"),
					asctest.JSON(http.StatusOK, `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-1","attributes":{"locale":"en-US"}}]}`))
			}
			reg.Register(asctest.PATCH("/v1/appStoreVersionLocalizations/loc-1"), asctest.JSON(http.StatusConflict, locUpdateNotEditableBody(test.attribute)))
			if test.byID {
				reg.Register(asctest.GET("/v1/appStoreVersionLocalizations/loc-1", "include=appStoreVersion", "fields[appStoreVersionLocalizations]=appStoreVersion"),
					asctest.JSON(http.StatusOK, `{"data":{"type":"appStoreVersionLocalizations","id":"loc-1","relationships":{"appStoreVersion":{"data":{"type":"appStoreVersions","id":"ver-1"}}}}}`))
			}
			current := strings.TrimSuffix(test.current, `}}`) + `},"relationships":{"app":{"data":{"type":"apps","id":"app-1"}}}}`
			reg.Register(asctest.GET("/v1/appStoreVersions/ver-1", "include=app"), asctest.JSON(http.StatusOK, `{"data":`+current+`}`))
			reg.Register(asctest.GET("/v1/apps/app-1/appStoreVersions", "filter[platform]=IOS", "limit=200"), asctest.JSON(http.StatusOK, `{"data":[`+test.appRows+`]}`))

			args := append([]string{"localizations", "update"}, test.args...)
			stdout, stderr, code := runWithDeps(t, stubClient(t, reg), args...)

			if code != cmd.ExitConflict {
				t.Fatalf("exit code = %d, want %d (stderr %q)", code, cmd.ExitConflict, stderr)
			}
			if stdout != "" {
				t.Fatalf("stdout = %q, want empty", stdout)
			}
			if !strings.Contains(stderr, "Attribute '"+test.attribute+"' cannot be edited at this time") {
				t.Fatalf("stderr %q dropped Apple's detail", stderr)
			}
			if !strings.Contains(stderr, test.wantStderr) {
				t.Fatalf("stderr = %q, want it to contain %q", stderr, test.wantStderr)
			}
			if got := strings.Count(stderr, "Hint:"); got != 1 {
				t.Fatalf("stderr has %d hints, want 1:\n%s", got, stderr)
			}
		})
	}
}
