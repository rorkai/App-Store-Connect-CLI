package cmdtest

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func TestAppInfoSetBatchValidationErrors(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name: "locale and locales are mutually exclusive",
			args: []string{
				"apps", "info", "edit",
				"--app", "APP_ID",
				"--locale", "en-US",
				"--locales", "de-DE",
				"--whats-new", "Fixes",
			},
			wantErr: "--locale and --locales are mutually exclusive",
		},
		{
			name: "invalid locale",
			args: []string{
				"apps", "info", "edit",
				"--app", "APP_ID",
				"--locales", "en_US,de-DE",
				"--whats-new", "Fixes",
			},
			wantErr: "invalid locale",
		},
		{
			name: "from-dir cannot be combined with locale",
			args: []string{
				"apps", "info", "edit",
				"--app", "APP_ID",
				"--from-dir", "/tmp/metadata",
				"--locale", "en-US",
			},
			wantErr: "--from-dir cannot be used with --locale or --locales",
		},
		{
			name: "from-dir cannot be combined with inline fields",
			args: []string{
				"apps", "info", "edit",
				"--app", "APP_ID",
				"--from-dir", "/tmp/metadata",
				"--whats-new", "Fixes",
			},
			wantErr: "--from-dir cannot be used with inline update flags",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(test.args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				err := root.Run(context.Background())
				if !errors.Is(err, flag.ErrHelp) {
					t.Fatalf("expected ErrHelp, got %v", err)
				}
			})

			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, test.wantErr) {
				t.Fatalf("expected error %q, got %q", test.wantErr, stderr)
			}
		})
	}
}

func TestRunAppInfoSetRejectsOverLimitKeywordsBeforeAuthResolution(t *testing.T) {
	overLimit := strings.Repeat("語", 101)
	tests := []struct {
		name string
		args func(t *testing.T) []string
	}{
		{
			name: "inline keywords",
			args: func(*testing.T) []string {
				return []string{"apps", "info", "edit", "--app", "APP_ID", "--locale", "ja", "--keywords", overLimit}
			},
		},
		{
			name: "from dir",
			args: func(t *testing.T) []string {
				inputDir := t.TempDir()
				content := `{"description":"Japanese description","keywords":"` + overLimit + `"}`
				if err := os.WriteFile(filepath.Join(inputDir, "ja.json"), []byte(content), 0o644); err != nil {
					t.Fatalf("write ja file: %v", err)
				}
				return []string{"apps", "info", "edit", "--app", "APP_ID", "--from-dir", inputDir}
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
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))
			asctest.NewRegistry(t).Install()
			args := test.args(t)

			_, stderr := captureOutput(t, func() {
				if code := cmd.Run(args, "1.2.3"); code != cmd.ExitUsage {
					t.Fatalf("expected exit code %d, got %d", cmd.ExitUsage, code)
				}
			})

			if !strings.Contains(stderr, "keywords exceed 100 characters") {
				t.Fatalf("expected keyword character-limit error, got %q", stderr)
			}
		})
	}
}

func TestAppInfoSetBatchDryRunInlineLocales(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/apps/app-1/appStoreVersions", "limit=200"), asctest.JSON(http.StatusOK, `{
				"data":[
					{
						"type":"appStoreVersions",
						"id":"ver-1",
						"attributes":{"createdDate":"2026-02-01T00:00:00Z"}
					}
				]
			}`))
	reg.Register(asctest.GET("/v1/appStoreVersions/ver-1/appStoreVersionLocalizations", "limit=200"), asctest.JSON(http.StatusOK, `{
				"data":[
					{
						"type":"appStoreVersionLocalizations",
						"id":"loc-en",
						"attributes":{"locale":"en-US"}
					}
				]
			}`))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"apps", "info", "edit",
			"--app", "app-1",
			"--locales", "en-US,de-DE",
			"--whats-new", "Bug fixes and improvements",
			"--dry-run",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if !strings.Contains(stderr, "creating locale de-DE would make it participate in submission validation") {
		t.Fatalf("expected planned create warning, got %q", stderr)
	}
	if !strings.Contains(stderr, "description, keywords, supportUrl") {
		t.Fatalf("expected missing fields in warning, got %q", stderr)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout=%s", err, stdout)
	}
	if payload["dryRun"] != true {
		t.Fatalf("expected dryRun true, got %v", payload["dryRun"])
	}
	if intValue(payload["total"]) != 2 {
		t.Fatalf("expected total=2, got %v", payload["total"])
	}
	if intValue(payload["planned"]) != 2 {
		t.Fatalf("expected planned=2 for dry-run, got %v", payload["planned"])
	}
	if intValue(payload["succeeded"]) != 0 {
		t.Fatalf("expected succeeded=0 for dry-run, got %v", payload["succeeded"])
	}
	if intValue(payload["failed"]) != 0 {
		t.Fatalf("expected failed=0, got %v", payload["failed"])
	}

	results, ok := payload["results"].([]any)
	if !ok {
		t.Fatalf("expected results array, got %T", payload["results"])
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	byLocale := map[string]map[string]any{}
	for _, item := range results {
		entry, ok := item.(map[string]any)
		if !ok {
			t.Fatalf("expected object result item, got %T", item)
		}
		locale, _ := entry["locale"].(string)
		byLocale[locale] = entry
	}

	if byLocale["en-US"]["action"] != "update" || byLocale["en-US"]["status"] != "planned" {
		t.Fatalf("expected en-US to be planned update, got %+v", byLocale["en-US"])
	}
	if byLocale["de-DE"]["action"] != "create" || byLocale["de-DE"]["status"] != "planned" {
		t.Fatalf("expected de-DE to be planned create, got %+v", byLocale["de-DE"])
	}
}

func TestAppsInfoEditTargetsLatestVersionAcrossPages(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	const nextURL = "https://api.appstoreconnect.apple.com/v1/apps/app-1/appStoreVersions?cursor=page-2"
	requests := make([]string, 0, 4)
	patchBody := ""
	oldVersionRequested := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if !strings.HasPrefix(req.Header.Get("Authorization"), "Bearer ") {
			t.Errorf("request is missing bearer authorization: %s %s", req.Method, req.URL.String())
			http.Error(w, "missing authorization", http.StatusUnauthorized)
			return
		}
		requests = append(requests, req.Method+" "+req.URL.RequestURI())
		w.Header().Set("Content-Type", "application/json")
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/appStoreVersions" && req.URL.Query().Get("cursor") == "":
			_, _ = io.WriteString(w, `{"data":[{"type":"appStoreVersions","id":"old","attributes":{"createdDate":"2026-01-01T00:00:00Z"}}],"links":{"next":"`+nextURL+`"}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/appStoreVersions" && req.URL.Query().Get("cursor") == "page-2":
			_, _ = io.WriteString(w, `{"data":[{"type":"appStoreVersions","id":"new","attributes":{"createdDate":"2026-02-01T00:00:00Z"}}]}`)
		case req.Method == http.MethodGet && (req.URL.Path == "/v1/appStoreVersions/new/appStoreVersionLocalizations" || req.URL.Path == "/v1/appStoreVersions/old/appStoreVersionLocalizations"):
			versionID := strings.TrimSuffix(strings.TrimPrefix(req.URL.Path, "/v1/appStoreVersions/"), "/appStoreVersionLocalizations")
			oldVersionRequested = versionID == "old"
			_, _ = io.WriteString(w, `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-`+versionID+`","attributes":{"locale":"en-US","description":"Description","keywords":"keywords","supportUrl":"https://example.com/support","whatsNew":"Old notes"}}]}`)
		case req.Method == http.MethodPatch && (req.URL.Path == "/v1/appStoreVersionLocalizations/loc-new" || req.URL.Path == "/v1/appStoreVersionLocalizations/loc-old"):
			body, err := io.ReadAll(req.Body)
			if err != nil {
				t.Errorf("read PATCH body: %v", err)
				http.Error(w, "failed to read body", http.StatusInternalServerError)
				return
			}
			patchBody = string(body)
			localizationID := strings.TrimPrefix(req.URL.Path, "/v1/appStoreVersionLocalizations/")
			oldVersionRequested = oldVersionRequested || localizationID == "loc-old"
			_, _ = io.WriteString(w, `{"data":{"type":"appStoreVersionLocalizations","id":"`+localizationID+`","attributes":{"locale":"en-US","whatsNew":"Fixes"}}}`)
		default:
			t.Errorf("unexpected request: %s %s", req.Method, req.URL.String())
			http.Error(w, "unexpected request", http.StatusBadRequest)
		}
	}))
	t.Cleanup(server.Close)

	serverURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("parse test server URL: %v", err)
	}
	transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Scheme != "https" || req.URL.Host != "api.appstoreconnect.apple.com" {
			return nil, errors.New("test transport received a non-App Store Connect URL")
		}
		routed := req.Clone(req.Context())
		routed.URL.Scheme = serverURL.Scheme
		routed.URL.Host = serverURL.Host
		routed.Host = serverURL.Host
		return server.Client().Transport.RoundTrip(routed)
	})
	client, err := asc.NewClientWithHTTPClient(
		os.Getenv("ASC_KEY_ID"),
		os.Getenv("ASC_ISSUER_ID"),
		os.Getenv("ASC_PRIVATE_KEY_PATH"),
		&http.Client{Transport: transport},
	)
	if err != nil {
		t.Fatalf("create test client: %v", err)
	}
	t.Cleanup(shared.SetASCClientFactoryForTesting(func() (*asc.Client, error) {
		return client, nil
	}))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	captureOutput(t, func() {
		if err := root.Parse([]string{
			"apps", "info", "edit",
			"--app", "app-1",
			"--locale", "en-US",
			"--whats-new", "Fixes",
			"--output", "json",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	wantRequests := []string{
		"GET /v1/apps/app-1/appStoreVersions?limit=200",
		"GET /v1/apps/app-1/appStoreVersions?cursor=page-2",
		"GET /v1/appStoreVersions/new/appStoreVersionLocalizations?filter%5Blocale%5D=en-US&limit=200",
		"PATCH /v1/appStoreVersionLocalizations/loc-new",
	}
	if !slices.Equal(requests, wantRequests) {
		t.Fatalf("request sequence = %v, want %v", requests, wantRequests)
	}
	if oldVersionRequested {
		t.Fatal("edit requested the older version's localization")
	}
	if !strings.Contains(patchBody, `"whatsNew":"Fixes"`) {
		t.Fatalf("expected latest localization PATCH body to contain whatsNew, got %q", patchBody)
	}
}

func TestAppInfoSetBatchApplyCreateWarningsStaySorted(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/apps/app-1/appStoreVersions", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"ver-1","attributes":{"createdDate":"2026-02-01T00:00:00Z"}}]}`))
	reg.Register(asctest.GET("/v1/appStoreVersions/ver-1/appStoreVersionLocalizations", "limit=200"), asctest.JSON(http.StatusOK, `{"data":[]}`))
	for _, locale := range []string{"de-DE", "fr-FR"} {
		reg.Register(asctest.POST("/v1/appStoreVersionLocalizations").JSONBody(`{"data":{"type":"appStoreVersionLocalizations","attributes":{"locale":"`+locale+`","whatsNew":"Bug fixes and improvements"},"relationships":{"appStoreVersion":{"data":{"type":"appStoreVersions","id":"ver-1"}}}}}`),
			asctest.JSON(http.StatusCreated, `{"data":{"type":"appStoreVersionLocalizations","id":"loc-created","attributes":{"locale":"created"}}}`))
	}

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"apps", "info", "edit",
			"--app", "app-1",
			"--locales", "fr-FR,de-DE",
			"--whats-new", "Bug fixes and improvements",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	createCount := 0
	for _, req := range reg.Requests() {
		if req.Method == http.MethodPost {
			createCount++
		}
	}
	if got := createCount; got != 2 {
		t.Fatalf("expected 2 create calls, got %d", got)
	}
	firstIdx := strings.Index(stderr, "created locale de-DE now participates in submission validation")
	secondIdx := strings.Index(stderr, "created locale fr-FR now participates in submission validation")
	if firstIdx == -1 || secondIdx == -1 {
		t.Fatalf("expected warnings for both locales, got %q", stderr)
	}
	if firstIdx > secondIdx {
		t.Fatalf("expected warnings sorted by locale, got %q", stderr)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout=%s", err, stdout)
	}
	if intValue(payload["succeeded"]) != 2 {
		t.Fatalf("expected succeeded=2, got %v", payload["succeeded"])
	}
	if intValue(payload["failed"]) != 0 {
		t.Fatalf("expected failed=0, got %v", payload["failed"])
	}
}

func TestAppInfoSetFromDirPartialFailureReturnsReportedError(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	inputDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(inputDir, "en-US.json"), []byte(`{"description":"English description","whatsNew":"Bug fixes"}`), 0o644); err != nil {
		t.Fatalf("write en-US file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(inputDir, "de-DE.json"), []byte(`{"description":"Deutsche Beschreibung","whatsNew":"Fehlerbehebungen"}`), 0o644); err != nil {
		t.Fatalf("write de-DE file: %v", err)
	}

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/apps/app-1/appStoreVersions", "limit=200"), asctest.JSON(http.StatusOK, `{
				"data":[
					{
						"type":"appStoreVersions",
						"id":"ver-1",
						"attributes":{"createdDate":"2026-02-01T00:00:00Z"}
					}
				]
			}`))
	reg.Register(asctest.GET("/v1/appStoreVersions/ver-1/appStoreVersionLocalizations", "limit=200"), asctest.JSON(http.StatusOK, `{
				"data":[
					{
						"type":"appStoreVersionLocalizations",
						"id":"loc-en",
						"attributes":{"locale":"en-US"}
					}
				]
			}`))
	reg.Register(asctest.PATCH("/v1/appStoreVersionLocalizations/loc-en").JSONBody(`{"data":{"type":"appStoreVersionLocalizations","id":"loc-en","attributes":{"description":"English description","whatsNew":"Bug fixes"}}}`),
		asctest.JSON(http.StatusOK, `{
				"data":{
					"type":"appStoreVersionLocalizations",
					"id":"loc-en",
					"attributes":{"locale":"en-US"}
				}
			}`))
	reg.Register(asctest.POST("/v1/appStoreVersionLocalizations").JSONBody(`{"data":{"type":"appStoreVersionLocalizations","attributes":{"locale":"de-DE","description":"Deutsche Beschreibung","whatsNew":"Fehlerbehebungen"},"relationships":{"appStoreVersion":{"data":{"type":"appStoreVersions","id":"ver-1"}}}}}`),
		asctest.JSON(http.StatusUnprocessableEntity, `{
				"errors":[
					{
						"status":"422",
						"code":"ENTITY_ERROR.ATTRIBUTE.INVALID",
						"title":"The provided entity includes an invalid attribute",
						"detail":"The locale de-DE failed validation"
					}
				]
			}`))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, _ := captureOutput(t, func() {
		if err := root.Parse([]string{
			"apps", "info", "edit",
			"--app", "app-1",
			"--from-dir", inputDir,
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if _, ok := errors.AsType[ReportedError](runErr); !ok {
		t.Fatalf("expected ReportedError, got %T: %v", runErr, runErr)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout=%s", err, stdout)
	}
	if intValue(payload["total"]) != 2 {
		t.Fatalf("expected total=2, got %v", payload["total"])
	}
	if intValue(payload["succeeded"]) != 1 {
		t.Fatalf("expected succeeded=1, got %v", payload["succeeded"])
	}
	if intValue(payload["failed"]) != 1 {
		t.Fatalf("expected failed=1, got %v", payload["failed"])
	}

	results, ok := payload["results"].([]any)
	if !ok {
		t.Fatalf("expected results array, got %T", payload["results"])
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	var failedEntry map[string]any
	for _, item := range results {
		entry, ok := item.(map[string]any)
		if !ok {
			t.Fatalf("expected object result item, got %T", item)
		}
		if entry["status"] == "failed" {
			failedEntry = entry
			break
		}
	}
	if failedEntry == nil {
		t.Fatalf("expected one failed locale entry, got %+v", results)
	}
	if strings.TrimSpace(asString(failedEntry["error"])) == "" {
		t.Fatalf("expected failed locale to include error message, got %+v", failedEntry)
	}
}

func TestRunAppInfoSetBatchPartialFailureReturnsExitError(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	inputDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(inputDir, "en-US.json"), []byte(`{"description":"English description"}`), 0o644); err != nil {
		t.Fatalf("write en-US file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(inputDir, "de-DE.json"), []byte(`{"description":"Deutsche Beschreibung"}`), 0o644); err != nil {
		t.Fatalf("write de-DE file: %v", err)
	}

	reg := asctest.NewRegistry(t)
	reg.Install()
	versions := `{"data":[{"type":"appStoreVersions","id":"ver-1","attributes":{"createdDate":"2026-02-01T00:00:00Z","platform":"IOS"}}]}`
	reg.Register(asctest.GET("/v1/apps/app-1/appStoreVersions", "limit=200"), asctest.JSON(http.StatusOK, versions))
	reg.Register(asctest.GET("/v1/apps/app-1/appStoreVersions", "filter[appStoreState]=READY_FOR_SALE,DEVELOPER_REMOVED_FROM_SALE,REMOVED_FROM_SALE", "filter[platform]=IOS", "limit=1"),
		asctest.JSON(http.StatusOK, `{"data":[]}`))
	reg.Register(asctest.GET("/v1/apps/app-1/appStoreVersions", "filter[appVersionState]=READY_FOR_DISTRIBUTION", "filter[platform]=IOS", "limit=1"),
		asctest.JSON(http.StatusOK, versions))
	reg.Register(asctest.GET("/v1/appStoreVersions/ver-1/appStoreVersionLocalizations", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-en","attributes":{"locale":"en-US"}}]}`))
	reg.Register(asctest.PATCH("/v1/appStoreVersionLocalizations/loc-en").JSONBody(`{"data":{"type":"appStoreVersionLocalizations","id":"loc-en","attributes":{"description":"English description"}}}`),
		asctest.JSON(http.StatusOK, `{"data":{"type":"appStoreVersionLocalizations","id":"loc-en","attributes":{"locale":"en-US"}}}`))
	reg.Register(asctest.POST("/v1/appStoreVersionLocalizations").JSONBody(`{"data":{"type":"appStoreVersionLocalizations","attributes":{"locale":"de-DE","description":"Deutsche Beschreibung"},"relationships":{"appStoreVersion":{"data":{"type":"appStoreVersions","id":"ver-1"}}}}}`),
		asctest.JSON(http.StatusUnprocessableEntity, `{"errors":[{"status":"422","code":"ENTITY_ERROR.ATTRIBUTE.INVALID","title":"Invalid","detail":"de-DE failed"}]}`))

	stdout, _ := captureOutput(t, func() {
		code := cmd.Run([]string{
			"apps", "info", "edit",
			"--app", "app-1",
			"--from-dir", inputDir,
		}, "1.2.3")
		if code != cmd.ExitError {
			t.Fatalf("expected exit code %d, got %d", cmd.ExitError, code)
		}
	})

	var payload map[string]any
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout=%s", err, stdout)
	}
	if intValue(payload["failed"]) != 1 {
		t.Fatalf("expected failed=1 in output, got %v", payload["failed"])
	}
}

func intValue(value any) int {
	switch v := value.(type) {
	case float64:
		return int(v)
	case int:
		return v
	default:
		return 0
	}
}

func asString(value any) string {
	if s, ok := value.(string); ok {
		return s
	}
	return ""
}
