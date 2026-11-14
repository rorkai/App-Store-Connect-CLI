package cmdtest

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/migrate"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/validation"
)

func TestMigrateImportDryRunPlan(t *testing.T) {
	root := t.TempDir()
	metadataDir := filepath.Join(root, "metadata", "en-US")
	if err := os.MkdirAll(metadataDir, 0o755); err != nil {
		t.Fatalf("mkdir metadata: %v", err)
	}
	writeFile(t, filepath.Join(metadataDir, "description.txt"), "English description")
	writeFile(t, filepath.Join(metadataDir, "name.txt"), "App Name")
	writeFile(t, filepath.Join(metadataDir, "privacy_url.txt"), "https://example.com/privacy")

	reviewDir := filepath.Join(root, "metadata", "review_information")
	if err := os.MkdirAll(reviewDir, 0o755); err != nil {
		t.Fatalf("mkdir review_information: %v", err)
	}
	writeFile(t, filepath.Join(reviewDir, "first_name.txt"), "Rita")
	writeFile(t, filepath.Join(reviewDir, "email_address.txt"), "rita@example.com")
	writeFile(t, filepath.Join(reviewDir, "demo_required.txt"), "false")

	screenshotsDir := filepath.Join(root, "screenshots", "en-US")
	if err := os.MkdirAll(screenshotsDir, 0o755); err != nil {
		t.Fatalf("mkdir screenshots: %v", err)
	}
	writePNGForMigrate(t, filepath.Join(screenshotsDir, "iphone_65_screen.png"), 1242, 2688)

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(cwd)
	})
	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	rootCmd := RootCommand("1.2.3")
	rootCmd.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := rootCmd.Parse([]string{
			"migrate", "import",
			"--app", "APP_ID",
			"--version-id", "VERSION_ID",
			"--dry-run",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := rootCmd.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var result migrate.MigrateImportResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if !result.DryRun {
		t.Fatalf("expected dry run true")
	}
	if result.VersionID != "VERSION_ID" {
		t.Fatalf("expected version id VERSION_ID, got %q", result.VersionID)
	}
	if len(result.ScreenshotPlan) != 1 {
		t.Fatalf("expected 1 screenshot plan, got %d", len(result.ScreenshotPlan))
	}
	if result.ReviewInformation == nil || result.ReviewInformation.ContactFirstName == nil {
		t.Fatalf("expected review info to be included")
	}
	if len(result.MetadataFiles) == 0 {
		t.Fatalf("expected metadata files plan")
	}
}

func TestMigrateImportDryRunClassifiesIPad13ScreenshotAsModernSlot(t *testing.T) {
	root := t.TempDir()
	metadataDir := filepath.Join(root, "metadata", "en-US")
	if err := os.MkdirAll(metadataDir, 0o755); err != nil {
		t.Fatalf("mkdir metadata: %v", err)
	}
	writeFile(t, filepath.Join(metadataDir, "description.txt"), "English description")

	screenshotsDir := filepath.Join(root, "screenshots", "en-US")
	if err := os.MkdirAll(screenshotsDir, 0o755); err != nil {
		t.Fatalf("mkdir screenshots: %v", err)
	}
	writePNGForMigrate(t, filepath.Join(screenshotsDir, "iPad Pro 13-inch (M5)-1-main-screen.png"), 2064, 2752)

	factoryCalls := 0
	restore := shared.SetASCClientFactoryForTesting(func() (*asc.Client, error) {
		factoryCalls++
		return nil, errors.New("client factory must not run during explicit-ID dry-run")
	})
	t.Cleanup(restore)

	rootCmd := RootCommand("1.2.3")
	rootCmd.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := rootCmd.Parse([]string{
			"migrate", "import",
			"--app", "APP_ID",
			"--version-id", "VERSION_ID",
			"--fastlane-dir", root,
			"--dry-run",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := rootCmd.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if factoryCalls != 0 {
		t.Fatalf("client factory calls = %d, want zero", factoryCalls)
	}

	var result migrate.MigrateImportResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if len(result.ScreenshotPlan) != 1 {
		t.Fatalf("expected 1 screenshot plan, got %d", len(result.ScreenshotPlan))
	}
	if got := result.ScreenshotPlan[0].DisplayType; got != "APP_IPAD_PRO_3GEN_129" {
		t.Fatalf("display type = %q, want APP_IPAD_PRO_3GEN_129", got)
	}
}

func TestMigrateImportDryRunReportsSkippedNonLocaleMetadataDirs(t *testing.T) {
	root := t.TempDir()
	metadataDir := filepath.Join(root, "metadata", "en-US")
	if err := os.MkdirAll(metadataDir, 0o755); err != nil {
		t.Fatalf("mkdir metadata: %v", err)
	}
	writeFile(t, filepath.Join(metadataDir, "description.txt"), "English description")

	// Mimic a common fastlane deliver subdirectory that is not a locale.
	nonLocaleDir := filepath.Join(root, "metadata", "trade_representative_contact_information")
	if err := os.MkdirAll(nonLocaleDir, 0o755); err != nil {
		t.Fatalf("mkdir non-locale dir: %v", err)
	}
	writeFile(t, filepath.Join(nonLocaleDir, "first_name.txt"), "Rita")
	nonLocaleDirResolved, err := filepath.EvalSymlinks(nonLocaleDir)
	if err != nil {
		t.Fatalf("eval symlinks non-locale dir: %v", err)
	}

	// Ensure default screenshots directory exists so it isn't reported as skipped.
	if err := os.MkdirAll(filepath.Join(root, "screenshots"), 0o755); err != nil {
		t.Fatalf("mkdir screenshots: %v", err)
	}

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(cwd)
	})
	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	rootCmd := RootCommand("1.2.3")
	rootCmd.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := rootCmd.Parse([]string{
			"migrate", "import",
			"--app", "APP_ID",
			"--version-id", "VERSION_ID",
			"--dry-run",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := rootCmd.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var result migrate.MigrateImportResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}

	found := false
	for _, item := range result.Skipped {
		itemPathResolved, err := filepath.EvalSymlinks(item.Path)
		if err != nil {
			itemPathResolved = item.Path
		}
		if itemPathResolved == nonLocaleDirResolved && strings.Contains(item.Reason, "skipped non-locale directory") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected skipped to include %q (got %+v)", nonLocaleDir, result.Skipped)
	}
}

func TestMigrateImportRejectsLocalValidationBeforeClientCreation(t *testing.T) {
	tests := []struct {
		name      string
		file      string
		value     string
		wantError string
	}{
		{
			name:      "description length",
			file:      "description.txt",
			value:     strings.Repeat("d", validation.LimitDescription+1),
			wantError: `migrate import: locale "ja": description exceeds 4000 characters`,
		},
		{
			name:      "keyword length",
			file:      "keywords.txt",
			value:     strings.Repeat("語", validation.LimitKeywords+1),
			wantError: `migrate import: locale "ja": keywords exceed 100 characters`,
		},
		{
			name:      "whats new length",
			file:      "release_notes.txt",
			value:     strings.Repeat("n", validation.LimitWhatsNew+1),
			wantError: `migrate import: locale "ja": whatsNew exceeds 4000 characters`,
		},
		{
			name:      "promotional text length",
			file:      "promotional_text.txt",
			value:     strings.Repeat("p", validation.LimitPromotionalText+1),
			wantError: `migrate import: locale "ja": promotionalText exceeds 170 characters`,
		},
		{
			name:      "marketing URI",
			file:      "marketing_url.txt",
			value:     "://invalid",
			wantError: `migrate import: locale "ja": marketingUrl must be a valid URI`,
		},
		{
			name:      "support URI",
			file:      "support_url.txt",
			value:     "://invalid",
			wantError: `migrate import: locale "ja": supportUrl must be a valid URI`,
		},
		{
			name:      "app name length",
			file:      "name.txt",
			value:     strings.Repeat("n", validation.LimitName+1),
			wantError: `migrate import: locale "ja": name exceeds 30 characters`,
		},
		{
			name:      "app subtitle length",
			file:      "subtitle.txt",
			value:     strings.Repeat("s", validation.LimitSubtitle+1),
			wantError: `migrate import: locale "ja": subtitle exceeds 30 characters`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := writeMigrateImportMetadata(t, map[string]map[string]string{
				"en-US": {"description.txt": "English description"},
				"ja":    {test.file: test.value},
			})

			factoryCalls := 0
			restore := shared.SetASCClientFactoryForTesting(func() (*asc.Client, error) {
				factoryCalls++
				return nil, errors.New("client factory must not run before local validation")
			})
			t.Cleanup(restore)

			stdout, stderr, runErr := runMigrateImport(t, root)
			assertMigrateImportError(t, stdout, stderr, runErr, test.wantError)
			if factoryCalls != 0 {
				t.Fatalf("client factory calls = %d, want zero", factoryCalls)
			}
		})
	}
}

func TestMigrateImportPreflightsAppInfoCreatesBeforeMutations(t *testing.T) {
	root := writeMigrateImportMetadata(t, validMigrateAppInfoMetadata())
	api := newMigrateImportAPI(t, migrateImportPlanningExpectations(`{"data":[]}`)...)

	stdout, stderr, runErr := runMigrateImport(t, root)
	const wantError = `migrate import: locale "ja": name is required when creating app info localization`
	assertMigrateImportError(t, stdout, stderr, runErr, wantError)
	api.assertComplete(t, 0)
}

func TestMigrateImportPreflightsWouldCreateVersionLocalesBeforeMutations(t *testing.T) {
	root := writeMigrateImportMetadata(t, map[string]map[string]string{
		"en-US": {"description.txt": "English description"},
		"nl": {
			"description.txt": "Dutch description",
			"name.txt":        "Dutch name",
		},
	})
	planning := migrateImportPlanningExpectations(`{"data":[]}`)
	api := newMigrateImportAPI(t, planning[:2]...)

	stdout, stderr, runErr := runMigrateImport(t, root)
	const wantError = `migrate import: locale "nl": unsupported locale "nl"; did you mean: nl-NL`
	assertMigrateImportError(t, stdout, stderr, runErr, wantError)
	api.assertComplete(t, 0)
}

func TestMigrateImportPreflightsWouldCreateAppInfoLocalesBeforeMutations(t *testing.T) {
	root := writeMigrateImportMetadata(t, map[string]map[string]string{
		"en-US": {"description.txt": "English description"},
		"nl": {
			"description.txt": "Dutch description",
			"name.txt":        "Dutch name",
		},
	})
	planning := migrateImportPlanningExpectations(`{"data":[]}`)
	planning[1].response = `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-en","attributes":{"locale":"en-US"}},{"type":"appStoreVersionLocalizations","id":"loc-nl","attributes":{"locale":"nl"}}]}`
	api := newMigrateImportAPI(t, planning...)

	stdout, stderr, runErr := runMigrateImport(t, root)
	const wantError = `migrate import: locale "nl": unsupported locale "nl"; did you mean: nl-NL`
	assertMigrateImportError(t, stdout, stderr, runErr, wantError)
	api.assertComplete(t, 0)
}

func TestMigrateImportPreflightsWouldCreateScreenshotLocalesBeforeMutations(t *testing.T) {
	root := writeMigrateImportMetadata(t, map[string]map[string]string{
		"en-US": {"description.txt": "English description"},
	})
	screenshotsDir := filepath.Join(root, "screenshots", "nl")
	if err := os.MkdirAll(screenshotsDir, 0o755); err != nil {
		t.Fatalf("mkdir screenshots: %v", err)
	}
	writePNGForMigrate(t, filepath.Join(screenshotsDir, "iphone_65_screen.png"), 1242, 2688)

	planning := migrateImportPlanningExpectations(`{"data":[]}`)
	api := newMigrateImportAPI(t, planning[:2]...)

	stdout, stderr, runErr := runMigrateImportWithOptions(t, root)
	const wantError = `migrate import: locale "nl": unsupported locale "nl"; did you mean: nl-NL`
	assertMigrateImportError(t, stdout, stderr, runErr, wantError)
	api.assertComplete(t, 0)
}

func TestMigrateImportAllowsVersionLocalizationUpdatesFromLaterPages(t *testing.T) {
	root := writeMigrateImportMetadata(t, map[string]map[string]string{
		"en-US": {"description.txt": "English description"},
		"ja": {
			"description.txt":   "Japanese description",
			"release_notes.txt": "Japanese release notes",
		},
	})
	expectations := []migrateImportExpectation{
		{
			method:   http.MethodGet,
			path:     "/v1/appStoreVersions/VERSION_ID",
			response: `{"data":{"type":"appStoreVersions","id":"VERSION_ID","attributes":{"versionString":"1.0","platform":"IOS"},"relationships":{"app":{"data":{"type":"apps","id":"APP_ID"}}}}}`,
		},
		{
			method:   http.MethodGet,
			path:     "/v1/appStoreVersions/VERSION_ID/appStoreVersionLocalizations",
			response: `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-en","attributes":{"locale":"en-US"}}],"links":{"next":"https://api.appstoreconnect.apple.com/v1/appStoreVersions/VERSION_ID/appStoreVersionLocalizations?cursor=page-2&limit=200"}}`,
		},
		{
			method:               http.MethodGet,
			path:                 "/v1/appStoreVersions/VERSION_ID/appStoreVersionLocalizations",
			rawQuery:             "cursor=page-2&limit=200",
			requireAuthorization: true,
			response:             `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-ja","attributes":{"locale":"ja"}}],"links":{"next":""}}`,
		},
		{
			method:   http.MethodPatch,
			path:     "/v1/appStoreVersionLocalizations/loc-en",
			body:     `{"data":{"type":"appStoreVersionLocalizations","id":"loc-en","attributes":{"description":"English description"}}}`,
			response: `{"data":{"type":"appStoreVersionLocalizations","id":"loc-en","attributes":{"locale":"en-US"}}}`,
		},
		{
			method: http.MethodPatch,
			path:   "/v1/appStoreVersionLocalizations/loc-ja",
			body: `{"data":{"type":"appStoreVersionLocalizations","id":"loc-ja","attributes":{` +
				`"description":"Japanese description","whatsNew":"Japanese release notes"}}}`,
			response: `{"data":{"type":"appStoreVersionLocalizations","id":"loc-ja","attributes":{"locale":"ja"}}}`,
		},
	}
	api := newMigrateImportAPI(t, expectations...)

	stdout, stderr, runErr := runMigrateImport(t, root)
	if runErr != nil {
		t.Fatalf("run error: %v", runErr)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	var result migrate.MigrateImportResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if len(result.Uploaded) != 2 || result.Uploaded[1].Action != "update" || result.Uploaded[1].LocalizationID != "loc-ja" {
		t.Fatalf("unexpected version localization uploads: %#v", result.Uploaded)
	}
	if got := api.queries[1]; got != "limit=200" {
		t.Fatalf("first version localization query = %q, want %q", got, "limit=200")
	}
	api.assertComplete(t, 2)
}

func TestMigrateImportAllowsSubtitleOnlyAppInfoUpdatesFromLaterPages(t *testing.T) {
	root := writeMigrateImportMetadata(t, validMigrateAppInfoMetadata())
	expectations := append(migrateImportPlanningExpectations(
		`{"data":[],"links":{"next":"https://api.appstoreconnect.apple.com/v1/appInfos/appinfo-1/appInfoLocalizations?cursor=page-2&limit=200"}}`,
	), []migrateImportExpectation{
		{
			method:               http.MethodGet,
			path:                 "/v1/appInfos/appinfo-1/appInfoLocalizations",
			rawQuery:             "cursor=page-2&limit=200",
			requireAuthorization: true,
			response:             `{"data":[{"type":"appInfoLocalizations","id":"appinfo-loc-ja","attributes":{"locale":"ja"}}],"links":{"next":""}}`,
		},
		{
			method:   http.MethodPatch,
			path:     "/v1/appStoreVersionLocalizations/loc-en",
			body:     `{"data":{"type":"appStoreVersionLocalizations","id":"loc-en","attributes":{"description":"English description"}}}`,
			response: `{"data":{"type":"appStoreVersionLocalizations","id":"loc-en","attributes":{"locale":"en-US"}}}`,
		},
		{
			method:   http.MethodPatch,
			path:     "/v1/appStoreVersionLocalizations/loc-ja",
			body:     `{"data":{"type":"appStoreVersionLocalizations","id":"loc-ja","attributes":{"description":"Japanese description"}}}`,
			response: `{"data":{"type":"appStoreVersionLocalizations","id":"loc-ja","attributes":{"locale":"ja"}}}`,
		},
		{
			method:   http.MethodPatch,
			path:     "/v1/appInfoLocalizations/appinfo-loc-ja",
			body:     `{"data":{"type":"appInfoLocalizations","id":"appinfo-loc-ja","attributes":{"subtitle":"Japanese subtitle"}}}`,
			response: `{"data":{"type":"appInfoLocalizations","id":"appinfo-loc-ja","attributes":{"locale":"ja","subtitle":"Japanese subtitle"}}}`,
		},
	}...)
	api := newMigrateImportAPI(t, expectations...)

	stdout, stderr, runErr := runMigrateImport(t, root)
	if runErr != nil {
		t.Fatalf("run error: %v", runErr)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var result migrate.MigrateImportResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if len(result.AppInfoUploaded) != 1 || result.AppInfoUploaded[0].Action != "update" || result.AppInfoUploaded[0].LocalizationID != "appinfo-loc-ja" {
		t.Fatalf("unexpected app info upload result: %#v", result.AppInfoUploaded)
	}
	if got := api.queries[3]; got != "limit=200" {
		t.Fatalf("first app info localization query = %q, want %q", got, "limit=200")
	}
	api.assertComplete(t, 3)
}

func TestMigrateImportStopsOnAppInfoPaginationErrorBeforeMutations(t *testing.T) {
	root := writeMigrateImportMetadata(t, validMigrateAppInfoMetadata())
	expectations := append(migrateImportPlanningExpectations(
		`{"data":[],"links":{"next":"https://api.appstoreconnect.apple.com/v1/appInfos/appinfo-1/appInfoLocalizations?cursor=page-2&limit=200"}}`,
	), migrateImportExpectation{
		method:               http.MethodGet,
		path:                 "/v1/appInfos/appinfo-1/appInfoLocalizations",
		rawQuery:             "cursor=page-2&limit=200",
		requireAuthorization: true,
		status:               http.StatusInternalServerError,
		response:             `{"errors":[{"status":"500","code":"UNEXPECTED_ERROR","title":"Request failed","detail":"pagination unavailable"}]}`,
	})
	api := newMigrateImportAPI(t, expectations...)

	stdout, stderr, runErr := runMigrateImport(t, root)
	if runErr == nil {
		t.Fatal("expected pagination error")
	}
	for _, want := range []string{"migrate import: failed to fetch app info localizations: page 2:", "pagination unavailable"} {
		if !strings.Contains(runErr.Error(), want) {
			t.Fatalf("run error = %q, want it to contain %q", runErr, want)
		}
	}
	if stdout != "" || stderr != "" {
		t.Fatalf("expected empty output, got stdout %q stderr %q", stdout, stderr)
	}
	if got := api.queries[3]; got != "limit=200" {
		t.Fatalf("first app info localization query = %q, want %q", got, "limit=200")
	}
	api.assertComplete(t, 0)
}

type migrateImportExpectation struct {
	method               string
	path                 string
	rawQuery             string
	requireAuthorization bool
	status               int
	body                 string
	response             string
}

type migrateImportAPI struct {
	expectations []migrateImportExpectation
	requests     []string
	queries      []string
	mutations    int
	factoryCalls int
}

func newMigrateImportAPI(t *testing.T, expectations ...migrateImportExpectation) *migrateImportAPI {
	t.Helper()
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	api := &migrateImportAPI{expectations: expectations}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		requestLabel := req.Method + " " + req.URL.Path
		api.requests = append(api.requests, requestLabel)
		api.queries = append(api.queries, req.URL.RawQuery)
		if req.Method == http.MethodPost || req.Method == http.MethodPatch || req.Method == http.MethodDelete {
			api.mutations++
		}

		index := len(api.requests) - 1
		if index >= len(api.expectations) {
			t.Errorf("unexpected request %d: %s", index+1, requestLabel)
			http.Error(w, "unexpected request", http.StatusInternalServerError)
			return
		}
		expected := api.expectations[index]
		if req.Method != expected.method || req.URL.Path != expected.path {
			t.Errorf("request %d = %s, want %s %s", index+1, requestLabel, expected.method, expected.path)
			http.Error(w, "unexpected request", http.StatusInternalServerError)
			return
		}
		if expected.rawQuery != "" && req.URL.RawQuery != expected.rawQuery {
			t.Errorf("request %d query = %q, want %q", index+1, req.URL.RawQuery, expected.rawQuery)
			http.Error(w, "unexpected query", http.StatusInternalServerError)
			return
		}
		if expected.requireAuthorization && !strings.HasPrefix(req.Header.Get("Authorization"), "Bearer ") {
			t.Errorf("request %d Authorization header = %q, want bearer token", index+1, req.Header.Get("Authorization"))
			http.Error(w, "missing authorization", http.StatusInternalServerError)
			return
		}
		if expected.body != "" {
			assertJSONDocument(t, req.Body, expected.body)
		}

		w.Header().Set("Content-Type", "application/json")
		if expected.status != 0 {
			w.WriteHeader(expected.status)
		}
		_, _ = io.WriteString(w, expected.response)
	}))
	t.Cleanup(server.Close)

	serverURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("parse migrate import test server URL: %v", err)
	}
	transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		cloned := req.Clone(req.Context())
		cloned.URL.Scheme = serverURL.Scheme
		cloned.URL.Host = serverURL.Host
		return server.Client().Transport.RoundTrip(cloned)
	})
	client, err := asc.NewClientWithHTTPClient(
		os.Getenv("ASC_KEY_ID"),
		os.Getenv("ASC_ISSUER_ID"),
		os.Getenv("ASC_PRIVATE_KEY_PATH"),
		&http.Client{Transport: transport},
	)
	if err != nil {
		t.Fatalf("create migrate import test client: %v", err)
	}
	restore := shared.SetASCClientFactoryForTesting(func() (*asc.Client, error) {
		api.factoryCalls++
		return client, nil
	})
	t.Cleanup(restore)
	return api
}

func (api *migrateImportAPI) assertComplete(t *testing.T, wantMutations int) {
	t.Helper()
	wantRequests := make([]string, 0, len(api.expectations))
	for _, expected := range api.expectations {
		wantRequests = append(wantRequests, expected.method+" "+expected.path)
	}
	if !slices.Equal(api.requests, wantRequests) {
		t.Fatalf("requests = %v, want %v", api.requests, wantRequests)
	}
	if api.mutations != wantMutations {
		t.Fatalf("mutations = %d, want %d", api.mutations, wantMutations)
	}
	if api.factoryCalls != 1 {
		t.Fatalf("client factory calls = %d, want 1", api.factoryCalls)
	}
}

func migrateImportPlanningExpectations(appInfoLocalizations string) []migrateImportExpectation {
	return []migrateImportExpectation{
		{
			method:   http.MethodGet,
			path:     "/v1/appStoreVersions/VERSION_ID",
			response: `{"data":{"type":"appStoreVersions","id":"VERSION_ID","attributes":{"versionString":"1.0","platform":"IOS"},"relationships":{"app":{"data":{"type":"apps","id":"APP_ID"}}}}}`,
		},
		{
			method:   http.MethodGet,
			path:     "/v1/appStoreVersions/VERSION_ID/appStoreVersionLocalizations",
			response: `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-en","attributes":{"locale":"en-US"}},{"type":"appStoreVersionLocalizations","id":"loc-ja","attributes":{"locale":"ja"}}]}`,
		},
		{
			method:   http.MethodGet,
			path:     "/v1/apps/APP_ID/appInfos",
			response: `{"data":[{"type":"appInfos","id":"appinfo-1","attributes":{"state":"PREPARE_FOR_SUBMISSION"}}]}`,
		},
		{
			method:   http.MethodGet,
			path:     "/v1/appInfos/appinfo-1/appInfoLocalizations",
			response: appInfoLocalizations,
		},
	}
}

func validMigrateAppInfoMetadata() map[string]map[string]string {
	return map[string]map[string]string{
		"en-US": {"description.txt": "English description"},
		"ja": {
			"description.txt": "Japanese description",
			"subtitle.txt":    "Japanese subtitle",
		},
	}
}

func writeMigrateImportMetadata(t *testing.T, localeFiles map[string]map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for locale, files := range localeFiles {
		localeDir := filepath.Join(root, "metadata", locale)
		if err := os.MkdirAll(localeDir, 0o755); err != nil {
			t.Fatalf("mkdir metadata locale %s: %v", locale, err)
		}
		for file, value := range files {
			writeFile(t, filepath.Join(localeDir, file), value)
		}
	}
	return root
}

func runMigrateImport(t *testing.T, root string) (string, string, error) {
	t.Helper()
	return runMigrateImportWithOptions(t, root, "--skip-screenshots")
}

func runMigrateImportWithOptions(t *testing.T, root string, options ...string) (string, string, error) {
	t.Helper()
	rootCmd := RootCommand("1.2.3")
	rootCmd.FlagSet.SetOutput(io.Discard)

	args := []string{
		"migrate", "import",
		"--app", "APP_ID",
		"--version-id", "VERSION_ID",
		"--fastlane-dir", root,
		"--confirm",
	}
	args = append(args, options...)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := rootCmd.Parse(args); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = rootCmd.Run(context.Background())
	})
	return stdout, stderr, runErr
}

func assertMigrateImportError(t *testing.T, stdout, stderr string, runErr error, wantError string) {
	t.Helper()
	if runErr == nil {
		t.Fatal("expected migrate import error")
	}
	if runErr.Error() != wantError {
		t.Fatalf("run error = %q, want %q", runErr, wantError)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
}

func TestMigrateImportUploadsAndSkipsExistingScreenshots(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	root := t.TempDir()
	fastlaneDir := filepath.Join(root, "fastlane")
	metadataDir := filepath.Join(fastlaneDir, "metadata", "en-US")
	if err := os.MkdirAll(metadataDir, 0o755); err != nil {
		t.Fatalf("mkdir metadata: %v", err)
	}
	writeFile(t, filepath.Join(metadataDir, "description.txt"), "English description")
	if err := os.MkdirAll(filepath.Join(fastlaneDir, "screenshots"), 0o755); err != nil {
		t.Fatalf("mkdir screenshots root: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(fastlaneDir, "screenshots"), 0o755); err != nil {
		t.Fatalf("mkdir screenshots root: %v", err)
	}
	writeFile(t, filepath.Join(metadataDir, "name.txt"), "App Name")

	reviewDir := filepath.Join(fastlaneDir, "metadata", "review_information")
	if err := os.MkdirAll(reviewDir, 0o755); err != nil {
		t.Fatalf("mkdir review_information: %v", err)
	}
	writeFile(t, filepath.Join(reviewDir, "first_name.txt"), "Rita")
	writeFile(t, filepath.Join(reviewDir, "email_address.txt"), "rita@example.com")

	screenshotsDir := filepath.Join(fastlaneDir, "screenshots", "en-US")
	if err := os.MkdirAll(screenshotsDir, 0o755); err != nil {
		t.Fatalf("mkdir screenshots: %v", err)
	}
	writePNGForMigrate(t, filepath.Join(screenshotsDir, "iphone_65_existing.png"), 1242, 2688)
	writePNGForMigrate(t, filepath.Join(screenshotsDir, "iphone_65_new.png"), 1242, 2688)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	requestedUploads := 0
	relationshipPatchCalled := false
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Host == "upload.example.com" {
			requestedUploads++
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader("")),
				Header:     http.Header{"Content-Type": []string{"text/plain"}},
			}, nil
		}

		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/VERSION_ID":
			return migrateJSONResponse(http.StatusOK, `{"data":{"type":"appStoreVersions","id":"VERSION_ID","attributes":{"versionString":"1.0","platform":"IOS"},"relationships":{"app":{"data":{"type":"apps","id":"APP_ID"}}}}}`), nil
		case req.Method == http.MethodGet && strings.HasPrefix(req.URL.Path, "/v1/appStoreVersions/") && strings.HasSuffix(req.URL.Path, "/appStoreVersionLocalizations"):
			body := `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-1","attributes":{"locale":"en-US"}}]}`
			return migrateJSONResponse(http.StatusOK, body), nil
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/appStoreVersionLocalizations/loc-1":
			return migrateJSONResponse(http.StatusOK, `{"data":{"type":"appStoreVersionLocalizations","id":"loc-1","attributes":{"locale":"en-US"}}}`), nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/APP_ID/appInfos":
			body := `{"data":[{"type":"appInfos","id":"appinfo-1","attributes":{"state":"PREPARE_FOR_SUBMISSION"}}]}`
			return migrateJSONResponse(http.StatusOK, body), nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appInfos/appinfo-1/appInfoLocalizations":
			return migrateJSONResponse(http.StatusOK, `{"data":[]}`), nil
		case req.Method == http.MethodPost && req.URL.Path == "/v1/appInfoLocalizations":
			return migrateJSONResponse(http.StatusCreated, `{"data":{"type":"appInfoLocalizations","id":"appinfo-loc-1","attributes":{"locale":"en-US"}}}`), nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/VERSION_ID/appStoreReviewDetail":
			return migrateJSONResponse(http.StatusNotFound, `{"errors":[{"status":"404","title":"not found"}]}`), nil
		case req.Method == http.MethodPost && req.URL.Path == "/v1/appStoreReviewDetails":
			return migrateJSONResponse(http.StatusCreated, `{"data":{"type":"appStoreReviewDetails","id":"review-1"}}`), nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersionLocalizations/loc-1/appScreenshotSets":
			return migrateJSONResponse(http.StatusOK, `{"data":[{"type":"appScreenshotSets","id":"set-1","attributes":{"screenshotDisplayType":"APP_IPHONE_65"}}]}`), nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appScreenshotSets/set-1/appScreenshots":
			body := `{"data":[{"type":"appScreenshots","id":"shot-existing","attributes":{"fileName":"iphone_65_existing.png"}}]}`
			return migrateJSONResponse(http.StatusOK, body), nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appScreenshotSets/set-1/relationships/appScreenshots":
			body := `{"data":[{"type":"appScreenshots","id":"shot-existing"}],"links":{}}`
			return migrateJSONResponse(http.StatusOK, body), nil
		case req.Method == http.MethodPost && req.URL.Path == "/v1/appScreenshots":
			resp := `{"data":{"type":"appScreenshots","id":"shot-new","attributes":{"fileName":"iphone_65_new.png","fileSize":1234,"uploadOperations":[{"method":"PUT","url":"https://upload.example.com/upload/shot-new","length":1234,"offset":0}]}}}`
			return migrateJSONResponse(http.StatusCreated, resp), nil
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/appScreenshots/shot-new":
			return migrateJSONResponse(http.StatusOK, `{"data":{"type":"appScreenshots","id":"shot-new","attributes":{"fileName":"iphone_65_new.png"}}}`), nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appScreenshots/shot-new":
			body := `{"data":{"type":"appScreenshots","id":"shot-new","attributes":{"fileName":"iphone_65_new.png","sourceFileChecksum":"settled","assetDeliveryState":{"state":"COMPLETE"}}}}`
			return migrateJSONResponse(http.StatusOK, body), nil
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/appScreenshotSets/set-1/relationships/appScreenshots":
			body, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("read relationship patch body: %v", err)
			}
			var payload asc.RelationshipRequest
			if err := json.Unmarshal(body, &payload); err != nil {
				t.Fatalf("unmarshal relationship patch body: %v", err)
			}
			if len(payload.Data) != 2 || payload.Data[0].ID != "shot-existing" || payload.Data[1].ID != "shot-new" {
				t.Fatalf("unexpected relationship patch payload: %#v", payload.Data)
			}
			relationshipPatchCalled = true
			return migrateJSONResponse(http.StatusNoContent, ""), nil
		default:
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.String())
		}
	})

	rootCmd := RootCommand("1.2.3")
	rootCmd.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := rootCmd.Parse([]string{
			"migrate", "import",
			"--app", "APP_ID",
			"--version-id", "VERSION_ID",
			"--fastlane-dir", fastlaneDir,
			"--confirm",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := rootCmd.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if requestedUploads != 1 {
		t.Fatalf("expected 1 upload request, got %d", requestedUploads)
	}
	if !relationshipPatchCalled {
		t.Fatal("expected screenshot relationship reorder patch")
	}

	var result migrate.MigrateImportResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if len(result.ScreenshotResults) != 1 {
		t.Fatalf("expected 1 screenshot result, got %d", len(result.ScreenshotResults))
	}
	if len(result.ScreenshotResults[0].Uploaded) != 1 {
		t.Fatalf("expected 1 uploaded screenshot, got %d", len(result.ScreenshotResults[0].Uploaded))
	}
	if len(result.ScreenshotResults[0].Skipped) != 1 {
		t.Fatalf("expected 1 skipped screenshot, got %d", len(result.ScreenshotResults[0].Skipped))
	}
}

func TestMigrateImportReportsPartiallyAppliedLocalesOnFailure(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	root := t.TempDir()
	fastlaneDir := filepath.Join(root, "fastlane")
	for locale, description := range map[string]string{
		"en-US": "English description",
		"ja":    "Japanese description",
	} {
		localeDir := filepath.Join(fastlaneDir, "metadata", locale)
		if err := os.MkdirAll(localeDir, 0o755); err != nil {
			t.Fatalf("mkdir metadata %s: %v", locale, err)
		}
		writeFile(t, filepath.Join(localeDir, "description.txt"), description)
	}

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/appStoreVersions/VERSION_ID", "include=app"),
		asctest.JSON(http.StatusOK, `{"data":{"type":"appStoreVersions","id":"VERSION_ID","attributes":{"versionString":"1.0","platform":"IOS"},"relationships":{"app":{"data":{"type":"apps","id":"APP_ID"}}}}}`))
	reg.Register(asctest.GET("/v1/appStoreVersions/VERSION_ID/appStoreVersionLocalizations", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-en","attributes":{"locale":"en-US"}},{"type":"appStoreVersionLocalizations","id":"loc-ja","attributes":{"locale":"ja"}}]}`))
	reg.Register(asctest.PATCH("/v1/appStoreVersionLocalizations/loc-en").JSONBody(`{"data":{"type":"appStoreVersionLocalizations","id":"loc-en","attributes":{"description":"English description"}}}`),
		asctest.JSON(http.StatusOK, `{"data":{"type":"appStoreVersionLocalizations","id":"loc-en","attributes":{"locale":"en-US"}}}`))
	reg.Register(asctest.PATCH("/v1/appStoreVersionLocalizations/loc-ja").JSONBody(`{"data":{"type":"appStoreVersionLocalizations","id":"loc-ja","attributes":{"description":"Japanese description"}}}`),
		asctest.JSON(http.StatusConflict, `{"errors":[{"status":"409","code":"STATE_ERROR","title":"Request failed","detail":"locale rejected"}]}`))

	rootCmd := RootCommand("1.2.3")
	rootCmd.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, _ := captureOutput(t, func() {
		if err := rootCmd.Parse([]string{
			"migrate", "import",
			"--app", "APP_ID",
			"--version-id", "VERSION_ID",
			"--fastlane-dir", fastlaneDir,
			"--skip-screenshots",
			"--confirm",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = rootCmd.Run(context.Background())
	})

	if runErr == nil {
		t.Fatal("expected migrate import to fail on the second locale")
	}
	if !strings.Contains(runErr.Error(), "failed to update ja") {
		t.Fatalf("run error = %q, want the failing locale", runErr)
	}

	var result migrate.MigrateImportResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("unmarshal partial result from %q: %v", stdout, err)
	}
	if result.Status != "partial" {
		t.Fatalf("status = %q, want partial", result.Status)
	}
	if result.FailureStage != "version_localizations" {
		t.Fatalf("failureStage = %q, want version_localizations", result.FailureStage)
	}
	if !strings.Contains(result.Failure, "failed to update ja") {
		t.Fatalf("failure = %q, want the failing locale", result.Failure)
	}
	if len(result.Uploaded) != 1 || result.Uploaded[0].Locale != "en-US" {
		t.Fatalf("uploaded = %+v, want the already applied en-US localization", result.Uploaded)
	}
}

func TestMigrateImportReportsScreenshotStageFailureAfterSetCreation(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	root := t.TempDir()
	fastlaneDir := filepath.Join(root, "fastlane")
	screenshotsDir := filepath.Join(fastlaneDir, "screenshots", "en-US")
	if err := os.MkdirAll(screenshotsDir, 0o755); err != nil {
		t.Fatalf("mkdir screenshots: %v", err)
	}
	writePNGForMigrate(t, filepath.Join(screenshotsDir, "iphone_65_new.png"), 1242, 2688)
	if err := os.MkdirAll(filepath.Join(fastlaneDir, "metadata"), 0o755); err != nil {
		t.Fatalf("mkdir metadata: %v", err)
	}

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/appStoreVersions/VERSION_ID", "include=app"),
		asctest.JSON(http.StatusOK, `{"data":{"type":"appStoreVersions","id":"VERSION_ID","attributes":{"versionString":"1.0","platform":"IOS"},"relationships":{"app":{"data":{"type":"apps","id":"APP_ID"}}}}}`))
	reg.Register(asctest.GET("/v1/appStoreVersions/VERSION_ID/appStoreVersionLocalizations", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-en","attributes":{"locale":"en-US"}}]}`))
	reg.Register(asctest.GET("/v1/appStoreVersionLocalizations/loc-en/appScreenshotSets"),
		asctest.JSON(http.StatusOK, `{"data":[]}`))
	reg.Register(asctest.POST("/v1/appScreenshotSets").JSONBody(`{"data":{"type":"appScreenshotSets","attributes":{"screenshotDisplayType":"APP_IPHONE_65"},"relationships":{"appStoreVersionLocalization":{"data":{"type":"appStoreVersionLocalizations","id":"loc-en"}}}}}`),
		asctest.JSON(http.StatusCreated, `{"data":{"type":"appScreenshotSets","id":"set-1","attributes":{"screenshotDisplayType":"APP_IPHONE_65"}}}`))
	reg.Register(asctest.POST("/v1/appScreenshots").JSONBody(`{"data":{"type":"appScreenshots","attributes":{"fileSize":62592,"fileName":"iphone_65_new.png"},"relationships":{"appScreenshotSet":{"data":{"type":"appScreenshotSets","id":"set-1"}}}}}`),
		asctest.JSON(http.StatusConflict, `{"errors":[{"status":"409","code":"STATE_ERROR","title":"Request failed","detail":"screenshot rejected"}]}`))

	rootCmd := RootCommand("1.2.3")
	rootCmd.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, _ := captureOutput(t, func() {
		if err := rootCmd.Parse([]string{
			"migrate", "import",
			"--app", "APP_ID",
			"--version-id", "VERSION_ID",
			"--fastlane-dir", fastlaneDir,
			"--confirm",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = rootCmd.Run(context.Background())
	})

	if runErr == nil {
		t.Fatal("expected migrate import to fail on the screenshot upload")
	}

	var result migrate.MigrateImportResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("unmarshal partial result from %q: %v", stdout, err)
	}
	if result.Status != "partial" || result.FailureStage != "screenshots" {
		t.Fatalf("status = %q failureStage = %q, want partial screenshots", result.Status, result.FailureStage)
	}
	if len(result.ScreenshotResults) != 1 || result.ScreenshotResults[0].DisplayType != "APP_IPHONE_65" {
		t.Fatalf("screenshotResults = %+v, want the screenshot set the run created", result.ScreenshotResults)
	}
}

// TestMigrateImportSkippedStagesDoNotReportPartial pins the partial document to
// real changes. Review information that already matched and screenshots that
// already existed left App Store Connect untouched, so a later failure is a
// plain error with nothing on stdout.
func TestMigrateImportSkippedStagesDoNotReportPartial(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	root := t.TempDir()
	fastlaneDir := filepath.Join(root, "fastlane")
	reviewDir := filepath.Join(fastlaneDir, "metadata", "review_information")
	if err := os.MkdirAll(reviewDir, 0o755); err != nil {
		t.Fatalf("mkdir review_information: %v", err)
	}
	writeFile(t, filepath.Join(reviewDir, "first_name.txt"), "Rita")
	writeFile(t, filepath.Join(reviewDir, "email_address.txt"), "rita@example.com")

	screenshotsDir := filepath.Join(fastlaneDir, "screenshots", "en-US")
	if err := os.MkdirAll(screenshotsDir, 0o755); err != nil {
		t.Fatalf("mkdir screenshots: %v", err)
	}
	writePNGForMigrate(t, filepath.Join(screenshotsDir, "iphone_65_existing.png"), 1242, 2688)

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/appStoreVersions/VERSION_ID", "include=app"),
		asctest.JSON(http.StatusOK, `{"data":{"type":"appStoreVersions","id":"VERSION_ID","attributes":{"versionString":"1.0","platform":"IOS"},"relationships":{"app":{"data":{"type":"apps","id":"APP_ID"}}}}}`))
	reg.Register(asctest.GET("/v1/appStoreVersions/VERSION_ID/appStoreVersionLocalizations", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-1","attributes":{"locale":"en-US"}}]}`))
	reg.Register(asctest.GET("/v1/appStoreVersions/VERSION_ID/appStoreReviewDetail"),
		asctest.JSON(http.StatusOK, `{"data":{"type":"appStoreReviewDetails","id":"review-1","attributes":{"contactFirstName":"Rita","contactEmail":"rita@example.com"}}}`))
	reg.Register(asctest.GET("/v1/appStoreVersionLocalizations/loc-1/appScreenshotSets"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"appScreenshotSets","id":"set-1","attributes":{"screenshotDisplayType":"APP_IPHONE_65"}}]}`))
	reg.Register(asctest.GET("/v1/appScreenshotSets/set-1/relationships/appScreenshots", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"appScreenshots","id":"shot-existing"}],"links":{}}`))
	reg.Register(asctest.GET("/v1/appScreenshotSets/set-1/appScreenshots"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"appScreenshots","id":"shot-existing","attributes":{"fileName":"iphone_65_existing.png"}}]}`))
	reg.Register(asctest.PATCH("/v1/appScreenshotSets/set-1/relationships/appScreenshots").JSONBody(`{"data":[{"type":"appScreenshots","id":"shot-existing"}]}`),
		asctest.JSON(http.StatusConflict, `{"errors":[{"status":"409","code":"STATE_ERROR","title":"Request failed","detail":"reorder rejected"}]}`))

	rootCmd := RootCommand("1.2.3")
	rootCmd.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, _ := captureOutput(t, func() {
		if err := rootCmd.Parse([]string{
			"migrate", "import",
			"--app", "APP_ID",
			"--version-id", "VERSION_ID",
			"--fastlane-dir", fastlaneDir,
			"--confirm",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = rootCmd.Run(context.Background())
	})

	if runErr == nil {
		t.Fatal("expected migrate import to fail on the screenshot reorder")
	}
	if !strings.Contains(runErr.Error(), "failed to reorder screenshots") {
		t.Fatalf("run error = %q, want the reorder failure", runErr)
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want nothing because no change reached App Store Connect", stdout)
	}
}

// TestMigrateImportWarnsAboutLocalizationCreatedBeforeScreenshotFailure covers
// the screenshot stage failing before it recorded any result: the localization
// it created for a screenshot-only locale is reported nowhere else.
func TestMigrateImportWarnsAboutLocalizationCreatedBeforeScreenshotFailure(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	root := t.TempDir()
	fastlaneDir := filepath.Join(root, "fastlane")
	if err := os.MkdirAll(filepath.Join(fastlaneDir, "metadata"), 0o755); err != nil {
		t.Fatalf("mkdir metadata: %v", err)
	}
	screenshotsDir := filepath.Join(fastlaneDir, "screenshots", "fr-FR")
	if err := os.MkdirAll(screenshotsDir, 0o755); err != nil {
		t.Fatalf("mkdir screenshots: %v", err)
	}
	writePNGForMigrate(t, filepath.Join(screenshotsDir, "iphone_65_new.png"), 1242, 2688)

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/appStoreVersions/VERSION_ID", "include=app"),
		asctest.JSON(http.StatusOK, `{"data":{"type":"appStoreVersions","id":"VERSION_ID","attributes":{"versionString":"1.0","platform":"IOS"},"relationships":{"app":{"data":{"type":"apps","id":"APP_ID"}}}}}`))
	reg.Register(asctest.GET("/v1/appStoreVersions/VERSION_ID/appStoreVersionLocalizations", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[],"links":{"next":""}}`))
	reg.Register(asctest.POST("/v1/appStoreVersionLocalizations").JSONBody(`{"data":{"type":"appStoreVersionLocalizations","attributes":{"locale":"fr-FR"},"relationships":{"appStoreVersion":{"data":{"type":"appStoreVersions","id":"VERSION_ID"}}}}}`),
		asctest.JSON(http.StatusCreated, `{"data":{"type":"appStoreVersionLocalizations","id":"loc-fr","attributes":{"locale":"fr-FR"}}}`))
	reg.Register(asctest.GET("/v1/appStoreVersionLocalizations/loc-fr/appScreenshotSets"),
		asctest.JSON(http.StatusConflict, `{"errors":[{"status":"409","code":"STATE_ERROR","title":"Request failed","detail":"sets unavailable"}]}`))

	rootCmd := RootCommand("1.2.3")
	rootCmd.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := rootCmd.Parse([]string{
			"migrate", "import",
			"--app", "APP_ID",
			"--version-id", "VERSION_ID",
			"--fastlane-dir", fastlaneDir,
			"--confirm",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = rootCmd.Run(context.Background())
	})

	if runErr == nil {
		t.Fatal("expected migrate import to fail while fetching screenshot sets")
	}
	createdLocalizations := metadataRequestCount(reg, http.MethodPost, "/v1/appStoreVersionLocalizations")
	if createdLocalizations != 1 {
		t.Fatalf("created localizations = %d, want the screenshot-only locale created once", createdLocalizations)
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want no result document when no screenshot result exists", stdout)
	}
	wantWarning := `Warning: created localization "fr-FR" before the failure; re-run import or remove it manually`
	if !strings.Contains(stderr, wantWarning) {
		t.Fatalf("stderr = %q, want %q", stderr, wantWarning)
	}
}

func TestMigrateImportWarnsAboutEveryUnreportedScreenshotLocalization(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	root := t.TempDir()
	fastlaneDir := filepath.Join(root, "fastlane")
	if err := os.MkdirAll(filepath.Join(fastlaneDir, "metadata"), 0o755); err != nil {
		t.Fatalf("mkdir metadata: %v", err)
	}
	for _, locale := range []string{"en-US", "fr-FR"} {
		screenshotsDir := filepath.Join(fastlaneDir, "screenshots", locale)
		if err := os.MkdirAll(screenshotsDir, 0o755); err != nil {
			t.Fatalf("mkdir screenshots %s: %v", locale, err)
		}
		writePNGForMigrate(t, filepath.Join(screenshotsDir, "iphone_65_existing.png"), 1242, 2688)
	}

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/appStoreVersions/VERSION_ID", "include=app"),
		asctest.JSON(http.StatusOK, `{"data":{"type":"appStoreVersions","id":"VERSION_ID","attributes":{"versionString":"1.0","platform":"IOS"},"relationships":{"app":{"data":{"type":"apps","id":"APP_ID"}}}}}`))
	reg.Register(asctest.GET("/v1/appStoreVersions/VERSION_ID/appStoreVersionLocalizations", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[],"links":{"next":""}}`))
	for index, locale := range []string{"en-US", "fr-FR"} {
		reg.Register(asctest.POST("/v1/appStoreVersionLocalizations").JSONBody(fmt.Sprintf(`{"data":{"type":"appStoreVersionLocalizations","attributes":{"locale":%q},"relationships":{"appStoreVersion":{"data":{"type":"appStoreVersions","id":"VERSION_ID"}}}}}`, locale)),
			asctest.JSON(http.StatusCreated, fmt.Sprintf(`{"data":{"type":"appStoreVersionLocalizations","id":"loc-%d","attributes":{"locale":%q}}}`, index+1, locale)))
	}
	reg.Register(asctest.GET("/v1/appStoreVersionLocalizations/loc-1/appScreenshotSets"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"appScreenshotSets","id":"set-1","attributes":{"screenshotDisplayType":"APP_IPHONE_65"}}]}`))
	reg.Register(asctest.GET("/v1/appScreenshotSets/set-1/relationships/appScreenshots", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"appScreenshots","id":"shot-existing"}],"links":{}}`))
	reg.Register(asctest.GET("/v1/appScreenshotSets/set-1/appScreenshots"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"appScreenshots","id":"shot-existing","attributes":{"fileName":"iphone_65_existing.png"}}]}`))
	reg.Register(asctest.PATCH("/v1/appScreenshotSets/set-1/relationships/appScreenshots").JSONBody(`{"data":[{"type":"appScreenshots","id":"shot-existing"}]}`),
		asctest.JSON(http.StatusNoContent, ""))
	reg.Register(asctest.GET("/v1/appStoreVersionLocalizations/loc-2/appScreenshotSets"),
		asctest.JSON(http.StatusConflict, `{"errors":[{"status":"409","code":"STATE_ERROR","title":"Request failed","detail":"sets unavailable"}]}`))

	rootCmd := RootCommand("1.2.3")
	rootCmd.FlagSet.SetOutput(io.Discard)

	var runErr error
	_, stderr := captureOutput(t, func() {
		if err := rootCmd.Parse([]string{
			"migrate", "import",
			"--app", "APP_ID",
			"--version-id", "VERSION_ID",
			"--fastlane-dir", fastlaneDir,
			"--confirm",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = rootCmd.Run(context.Background())
	})

	if runErr == nil {
		t.Fatal("expected migrate import to fail after one screenshot locale completed")
	}
	var createdLocales []string
	for _, req := range reg.Requests() {
		if req.Method != http.MethodPost {
			continue
		}
		var payload struct {
			Data struct {
				Attributes struct {
					Locale string `json:"locale"`
				} `json:"attributes"`
			} `json:"data"`
		}
		if err := json.Unmarshal(req.Body, &payload); err != nil {
			t.Fatalf("decode localization request: %v", err)
		}
		createdLocales = append(createdLocales, payload.Data.Attributes.Locale)
	}
	if len(createdLocales) != 2 {
		t.Fatalf("created locales = %v, want two screenshot-only locales", createdLocales)
	}
	wantWarning := fmt.Sprintf(`Warning: created localization %q before the failure; re-run import or remove it manually`, createdLocales[1])
	if !strings.Contains(stderr, wantWarning) {
		t.Fatalf("stderr = %q, want %q", stderr, wantWarning)
	}
	if strings.Contains(stderr, fmt.Sprintf(`created localization %q before the failure`, createdLocales[0])) {
		t.Fatalf("stderr = %q, completed locale %q must not be warned as unreported", stderr, createdLocales[0])
	}
}

// TestMigrateImportPartialOmitsSkippedReviewInformationStage keeps
// completedStages restricted to stages that changed App Store Connect.
func TestMigrateImportPartialOmitsSkippedReviewInformationStage(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	root := t.TempDir()
	fastlaneDir := filepath.Join(root, "fastlane")
	metadataDir := filepath.Join(fastlaneDir, "metadata", "en-US")
	if err := os.MkdirAll(metadataDir, 0o755); err != nil {
		t.Fatalf("mkdir metadata: %v", err)
	}
	writeFile(t, filepath.Join(metadataDir, "description.txt"), "English description")

	reviewDir := filepath.Join(fastlaneDir, "metadata", "review_information")
	if err := os.MkdirAll(reviewDir, 0o755); err != nil {
		t.Fatalf("mkdir review_information: %v", err)
	}
	writeFile(t, filepath.Join(reviewDir, "first_name.txt"), "Rita")

	screenshotsDir := filepath.Join(fastlaneDir, "screenshots", "en-US")
	if err := os.MkdirAll(screenshotsDir, 0o755); err != nil {
		t.Fatalf("mkdir screenshots: %v", err)
	}
	writePNGForMigrate(t, filepath.Join(screenshotsDir, "iphone_65_new.png"), 1242, 2688)

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/appStoreVersions/VERSION_ID", "include=app"),
		asctest.JSON(http.StatusOK, `{"data":{"type":"appStoreVersions","id":"VERSION_ID","attributes":{"versionString":"1.0","platform":"IOS"},"relationships":{"app":{"data":{"type":"apps","id":"APP_ID"}}}}}`))
	reg.Register(asctest.GET("/v1/appStoreVersions/VERSION_ID/appStoreVersionLocalizations", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-1","attributes":{"locale":"en-US"}}]}`))
	reg.Register(asctest.PATCH("/v1/appStoreVersionLocalizations/loc-1").JSONBody(`{"data":{"type":"appStoreVersionLocalizations","id":"loc-1","attributes":{"description":"English description"}}}`),
		asctest.JSON(http.StatusOK, `{"data":{"type":"appStoreVersionLocalizations","id":"loc-1","attributes":{"locale":"en-US"}}}`))
	reg.Register(asctest.GET("/v1/appStoreVersions/VERSION_ID/appStoreReviewDetail"),
		asctest.JSON(http.StatusOK, `{"data":{"type":"appStoreReviewDetails","id":"review-1","attributes":{"contactFirstName":"Rita"}}}`))
	reg.Register(asctest.GET("/v1/appStoreVersionLocalizations/loc-1/appScreenshotSets"),
		asctest.JSON(http.StatusOK, `{"data":[]}`))
	reg.Register(asctest.POST("/v1/appScreenshotSets").JSONBody(`{"data":{"type":"appScreenshotSets","attributes":{"screenshotDisplayType":"APP_IPHONE_65"},"relationships":{"appStoreVersionLocalization":{"data":{"type":"appStoreVersionLocalizations","id":"loc-1"}}}}}`),
		asctest.JSON(http.StatusCreated, `{"data":{"type":"appScreenshotSets","id":"set-1","attributes":{"screenshotDisplayType":"APP_IPHONE_65"}}}`))
	reg.Register(asctest.POST("/v1/appScreenshots").JSONBody(`{"data":{"type":"appScreenshots","attributes":{"fileSize":62592,"fileName":"iphone_65_new.png"},"relationships":{"appScreenshotSet":{"data":{"type":"appScreenshotSets","id":"set-1"}}}}}`),
		asctest.JSON(http.StatusConflict, `{"errors":[{"status":"409","code":"STATE_ERROR","title":"Request failed","detail":"screenshot rejected"}]}`))

	rootCmd := RootCommand("1.2.3")
	rootCmd.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, _ := captureOutput(t, func() {
		if err := rootCmd.Parse([]string{
			"migrate", "import",
			"--app", "APP_ID",
			"--version-id", "VERSION_ID",
			"--fastlane-dir", fastlaneDir,
			"--confirm",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = rootCmd.Run(context.Background())
	})

	if runErr == nil {
		t.Fatal("expected migrate import to fail on the screenshot upload")
	}

	var result migrate.MigrateImportResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("unmarshal partial result from %q: %v", stdout, err)
	}
	if result.Status != "partial" || result.FailureStage != "screenshots" {
		t.Fatalf("status = %q failureStage = %q, want partial screenshots", result.Status, result.FailureStage)
	}
	if result.ReviewInfoResult == nil || result.ReviewInfoResult.Action != "skip" {
		t.Fatalf("reviewInfoResult = %+v, want the skipped review information", result.ReviewInfoResult)
	}
	if slices.Contains(result.CompletedStages, "review_information") {
		t.Fatalf("completedStages = %v, want no review_information because the remote already matched", result.CompletedStages)
	}
	if !slices.Contains(result.CompletedStages, "version_localizations") {
		t.Fatalf("completedStages = %v, want the version localizations that were applied", result.CompletedStages)
	}
}

func TestMigrateImportPartialFailureStillWarnsForCreatedLocales(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	root := t.TempDir()
	fastlaneDir := filepath.Join(root, "fastlane")
	for locale, description := range map[string]string{
		"en-US": "English description",
		"ja":    "Japanese description",
	} {
		localeDir := filepath.Join(fastlaneDir, "metadata", locale)
		if err := os.MkdirAll(localeDir, 0o755); err != nil {
			t.Fatalf("mkdir metadata %s: %v", locale, err)
		}
		writeFile(t, filepath.Join(localeDir, "description.txt"), description)
	}

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/appStoreVersions/VERSION_ID", "include=app"),
		asctest.JSON(http.StatusOK, `{"data":{"type":"appStoreVersions","id":"VERSION_ID","attributes":{"versionString":"1.0","platform":"IOS"},"relationships":{"app":{"data":{"type":"apps","id":"APP_ID"}}}}}`))
	reg.Register(asctest.GET("/v1/appStoreVersions/VERSION_ID/appStoreVersionLocalizations", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[],"links":{"next":""}}`))
	reg.Register(asctest.GET("/v1/appStoreVersions/VERSION_ID", "include=app"),
		asctest.JSON(http.StatusOK, `{"data":{"type":"appStoreVersions","id":"VERSION_ID","attributes":{"versionString":"1.0","platform":"IOS"},"relationships":{"app":{"data":{"type":"apps","id":"APP_ID"}}}}}`))
	reg.Register(asctest.GET("/v1/apps/APP_ID/appStoreVersions", "filter[appStoreState]=READY_FOR_SALE,DEVELOPER_REMOVED_FROM_SALE,REMOVED_FROM_SALE", "filter[platform]=IOS", "limit=1"),
		func(req *http.Request) (*http.Response, error) {
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.String())
		})
	reg.Register(asctest.POST("/v1/appStoreVersionLocalizations").JSONBody(`{"data":{"type":"appStoreVersionLocalizations","attributes":{"locale":"en-US","description":"English description"},"relationships":{"appStoreVersion":{"data":{"type":"appStoreVersions","id":"VERSION_ID"}}}}}`),
		asctest.JSON(http.StatusCreated, `{"data":{"type":"appStoreVersionLocalizations","id":"loc-en","attributes":{"locale":"en-US","description":"English description"}}}`))
	reg.Register(asctest.POST("/v1/appStoreVersionLocalizations").JSONBody(`{"data":{"type":"appStoreVersionLocalizations","attributes":{"locale":"ja","description":"Japanese description"},"relationships":{"appStoreVersion":{"data":{"type":"appStoreVersions","id":"VERSION_ID"}}}}}`),
		asctest.JSON(http.StatusConflict, `{"errors":[{"status":"409","code":"STATE_ERROR","title":"Request failed","detail":"locale rejected"}]}`))

	rootCmd := RootCommand("1.2.3")
	rootCmd.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := rootCmd.Parse([]string{
			"migrate", "import",
			"--app", "APP_ID",
			"--version-id", "VERSION_ID",
			"--fastlane-dir", fastlaneDir,
			"--skip-screenshots",
			"--confirm",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = rootCmd.Run(context.Background())
	})

	if runErr == nil {
		t.Fatal("expected migrate import to fail on the second locale")
	}

	var result migrate.MigrateImportResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("unmarshal partial result from %q: %v", stdout, err)
	}
	if result.Status != "partial" {
		t.Fatalf("status = %q, want partial", result.Status)
	}
	if len(result.Uploaded) != 1 || result.Uploaded[0].Action != "create" {
		t.Fatalf("uploaded = %+v, want the created en-US localization", result.Uploaded)
	}
	if !strings.Contains(stderr, "created locale en-US now participates in submission validation") {
		t.Fatalf("stderr = %q, want the create warning for the locale that landed", stderr)
	}
}

func TestMigrateImportUploadPhaseKeepsFullUploadBudget(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	root := t.TempDir()
	fastlaneDir := filepath.Join(root, "fastlane")
	metadataDir := filepath.Join(fastlaneDir, "metadata", "en-US")
	if err := os.MkdirAll(metadataDir, 0o755); err != nil {
		t.Fatalf("mkdir metadata: %v", err)
	}
	writeFile(t, filepath.Join(metadataDir, "description.txt"), "English description")

	screenshotsDir := filepath.Join(fastlaneDir, "screenshots", "en-US")
	if err := os.MkdirAll(screenshotsDir, 0o755); err != nil {
		t.Fatalf("mkdir screenshots: %v", err)
	}
	writePNGForMigrate(t, filepath.Join(screenshotsDir, "iphone_65_new.png"), 1242, 2688)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	budgets := make(map[string]time.Duration)
	var budgetsMu sync.Mutex
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		label := req.Method + " " + req.URL.Path
		budgetsMu.Lock()
		if deadline, ok := req.Context().Deadline(); ok {
			if remaining := time.Until(deadline); remaining > budgets[label] {
				budgets[label] = remaining
			}
		} else {
			budgets[label] = -1
		}
		budgetsMu.Unlock()

		if req.URL.Host == "upload.example.com" {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader("")),
				Header:     http.Header{"Content-Type": []string{"text/plain"}},
			}, nil
		}

		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/VERSION_ID":
			return migrateJSONResponse(http.StatusOK, `{"data":{"type":"appStoreVersions","id":"VERSION_ID","attributes":{"versionString":"1.0","platform":"IOS"},"relationships":{"app":{"data":{"type":"apps","id":"APP_ID"}}}}}`), nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/VERSION_ID/appStoreVersionLocalizations":
			return migrateJSONResponse(http.StatusOK, `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-1","attributes":{"locale":"en-US"}}]}`), nil
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/appStoreVersionLocalizations/loc-1":
			return migrateJSONResponse(http.StatusOK, `{"data":{"type":"appStoreVersionLocalizations","id":"loc-1","attributes":{"locale":"en-US"}}}`), nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersionLocalizations/loc-1/appScreenshotSets":
			return migrateJSONResponse(http.StatusOK, `{"data":[{"type":"appScreenshotSets","id":"set-1","attributes":{"screenshotDisplayType":"APP_IPHONE_65"}}]}`), nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appScreenshotSets/set-1/appScreenshots":
			return migrateJSONResponse(http.StatusOK, `{"data":[]}`), nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appScreenshotSets/set-1/relationships/appScreenshots":
			return migrateJSONResponse(http.StatusOK, `{"data":[],"links":{}}`), nil
		case req.Method == http.MethodPost && req.URL.Path == "/v1/appScreenshots":
			resp := `{"data":{"type":"appScreenshots","id":"shot-new","attributes":{"fileName":"iphone_65_new.png","fileSize":1234,"uploadOperations":[{"method":"PUT","url":"https://upload.example.com/upload/shot-new","length":1234,"offset":0}]}}}`
			return migrateJSONResponse(http.StatusCreated, resp), nil
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/appScreenshots/shot-new":
			return migrateJSONResponse(http.StatusOK, `{"data":{"type":"appScreenshots","id":"shot-new","attributes":{"fileName":"iphone_65_new.png"}}}`), nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appScreenshots/shot-new":
			body := `{"data":{"type":"appScreenshots","id":"shot-new","attributes":{"fileName":"iphone_65_new.png","sourceFileChecksum":"settled","assetDeliveryState":{"state":"COMPLETE"}}}}`
			return migrateJSONResponse(http.StatusOK, body), nil
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/appScreenshotSets/set-1/relationships/appScreenshots":
			return migrateJSONResponse(http.StatusNoContent, ""), nil
		default:
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.String())
		}
	})

	rootCmd := RootCommand("1.2.3")
	rootCmd.FlagSet.SetOutput(io.Discard)

	_, stderr := captureOutput(t, func() {
		if err := rootCmd.Parse([]string{
			"migrate", "import",
			"--app", "APP_ID",
			"--version-id", "VERSION_ID",
			"--fastlane-dir", fastlaneDir,
			"--confirm",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := rootCmd.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	// The asset transfer runs on the upload client, so its budget shows whether
	// the mutation phase inherited the planning request deadline.
	putBudget, ok := budgets["PUT /upload/shot-new"]
	if !ok {
		t.Fatalf("no screenshot transfer recorded, got %v", budgets)
	}
	if putBudget < 4*time.Minute {
		t.Fatalf("screenshot transfer budget = %s, want the asset upload budget rather than one request timeout", putBudget)
	}

	// API calls stay bounded by the ordinary request timeout.
	for _, label := range []string{
		"GET /v1/appStoreVersions/VERSION_ID",
		"PATCH /v1/appStoreVersionLocalizations/loc-1",
		"POST /v1/appScreenshots",
	} {
		budget, ok := budgets[label]
		if !ok {
			t.Fatalf("no %s request recorded, got %v", label, budgets)
		}
		if budget <= 0 || budget > time.Minute {
			t.Fatalf("%s budget = %s, want a bounded per-request timeout", label, budget)
		}
	}
}

func TestMigrateImportWarnsForMetadataCreates(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	root := t.TempDir()
	fastlaneDir := filepath.Join(root, "fastlane")
	metadataDir := filepath.Join(fastlaneDir, "metadata", "en-US")
	if err := os.MkdirAll(metadataDir, 0o755); err != nil {
		t.Fatalf("mkdir metadata: %v", err)
	}
	writeFile(t, filepath.Join(metadataDir, "description.txt"), "English description")
	if err := os.MkdirAll(filepath.Join(fastlaneDir, "screenshots"), 0o755); err != nil {
		t.Fatalf("mkdir screenshots root: %v", err)
	}

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/appStoreVersions/VERSION_ID", "include=app"),
		asctest.JSON(http.StatusOK, `{"data":{"type":"appStoreVersions","id":"VERSION_ID","attributes":{"versionString":"1.0","platform":"IOS"},"relationships":{"app":{"data":{"type":"apps","id":"APP_ID"}}}}}`))
	reg.Register(asctest.GET("/v1/appStoreVersions/VERSION_ID/appStoreVersionLocalizations", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[],"links":{"next":""}}`))
	reg.Register(asctest.GET("/v1/appStoreVersions/VERSION_ID", "include=app"),
		asctest.JSON(http.StatusOK, `{"data":{"type":"appStoreVersions","id":"VERSION_ID","attributes":{"versionString":"1.0","platform":"IOS"},"relationships":{"app":{"data":{"type":"apps","id":"APP_ID"}}}}}`))
	reg.Register(asctest.GET("/v1/apps/APP_ID/appStoreVersions", "filter[appStoreState]=READY_FOR_SALE,DEVELOPER_REMOVED_FROM_SALE,REMOVED_FROM_SALE", "filter[platform]=IOS", "limit=1"),
		func(req *http.Request) (*http.Response, error) {
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.String())
		})
	reg.Register(asctest.POST("/v1/appStoreVersionLocalizations").JSONBody(`{"data":{"type":"appStoreVersionLocalizations","attributes":{"locale":"en-US","description":"English description"},"relationships":{"appStoreVersion":{"data":{"type":"appStoreVersions","id":"VERSION_ID"}}}}}`),
		asctest.JSON(http.StatusCreated, `{"data":{"type":"appStoreVersionLocalizations","id":"loc-1","attributes":{"locale":"en-US","description":"English description"}}}`))

	rootCmd := RootCommand("1.2.3")
	rootCmd.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := rootCmd.Parse([]string{
			"migrate", "import",
			"--app", "APP_ID",
			"--version-id", "VERSION_ID",
			"--fastlane-dir", fastlaneDir,
			"--confirm",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := rootCmd.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	createCount := metadataRequestCount(reg, http.MethodPost, "/v1/appStoreVersionLocalizations")
	if createCount != 1 {
		t.Fatalf("expected one metadata create request, got %d", createCount)
	}
	if !strings.Contains(stderr, "created locale en-US now participates in submission validation") {
		t.Fatalf("expected create warning on stderr, got %q", stderr)
	}
	if !strings.Contains(stderr, "keywords, supportUrl") {
		t.Fatalf("expected missing fields in warning, got %q", stderr)
	}

	var result migrate.MigrateImportResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if len(result.Uploaded) != 1 || result.Uploaded[0].Action != "create" || result.Uploaded[0].Locale != "en-US" {
		t.Fatalf("expected single created upload result, got %+v", result.Uploaded)
	}
}

func TestMigrateImportDoesNotWarnForScreenshotBootstrapCreates(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	root := t.TempDir()
	fastlaneDir := filepath.Join(root, "fastlane")
	if err := os.MkdirAll(filepath.Join(fastlaneDir, "metadata"), 0o755); err != nil {
		t.Fatalf("mkdir metadata root: %v", err)
	}
	screenshotsDir := filepath.Join(fastlaneDir, "screenshots", "en-US")
	if err := os.MkdirAll(screenshotsDir, 0o755); err != nil {
		t.Fatalf("mkdir screenshots: %v", err)
	}
	writePNGForMigrate(t, filepath.Join(screenshotsDir, "iphone_65_screen.png"), 1242, 2688)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Host == "upload.example.com" {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader("")),
				Header:     http.Header{"Content-Type": []string{"text/plain"}},
			}, nil
		}

		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/VERSION_ID":
			return migrateJSONResponse(http.StatusOK, `{"data":{"type":"appStoreVersions","id":"VERSION_ID","attributes":{"versionString":"1.0","platform":"IOS"},"relationships":{"app":{"data":{"type":"apps","id":"APP_ID"}}}}}`), nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/VERSION_ID/appStoreVersionLocalizations":
			return migrateJSONResponse(http.StatusOK, `{"data":[],"links":{"next":""}}`), nil
		case req.Method == http.MethodPost && req.URL.Path == "/v1/appStoreVersionLocalizations":
			return migrateJSONResponse(http.StatusCreated, `{"data":{"type":"appStoreVersionLocalizations","id":"loc-shot","attributes":{"locale":"en-US"}}}`), nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersionLocalizations/loc-shot/appScreenshotSets":
			return migrateJSONResponse(http.StatusOK, `{"data":[]}`), nil
		case req.Method == http.MethodPost && req.URL.Path == "/v1/appScreenshotSets":
			return migrateJSONResponse(http.StatusCreated, `{"data":{"type":"appScreenshotSets","id":"set-1","attributes":{"screenshotDisplayType":"APP_IPHONE_65"}}}`), nil
		case req.Method == http.MethodPost && req.URL.Path == "/v1/appScreenshots":
			return migrateJSONResponse(http.StatusCreated, `{"data":{"type":"appScreenshots","id":"shot-1","attributes":{"fileName":"iphone_65_screen.png","fileSize":1234,"uploadOperations":[{"method":"PUT","url":"https://upload.example.com/upload/shot-1","length":1234,"offset":0}]}}}`), nil
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/appScreenshots/shot-1":
			return migrateJSONResponse(http.StatusOK, `{"data":{"type":"appScreenshots","id":"shot-1","attributes":{"fileName":"iphone_65_screen.png"}}}`), nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appScreenshots/shot-1":
			return migrateJSONResponse(http.StatusOK, `{"data":{"type":"appScreenshots","id":"shot-1","attributes":{"fileName":"iphone_65_screen.png","sourceFileChecksum":"settled","assetDeliveryState":{"state":"COMPLETE"}}}}`), nil
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/appScreenshotSets/set-1/relationships/appScreenshots":
			return migrateJSONResponse(http.StatusNoContent, ""), nil
		default:
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.String())
		}
	})

	rootCmd := RootCommand("1.2.3")
	rootCmd.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := rootCmd.Parse([]string{
			"migrate", "import",
			"--app", "APP_ID",
			"--version-id", "VERSION_ID",
			"--fastlane-dir", fastlaneDir,
			"--confirm",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := rootCmd.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var result migrate.MigrateImportResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if len(result.Uploaded) != 0 {
		t.Fatalf("expected no metadata uploads, got %+v", result.Uploaded)
	}
	if len(result.ScreenshotResults) != 1 {
		t.Fatalf("expected one screenshot result, got %+v", result.ScreenshotResults)
	}
}

func TestMigrateImportRejectsInvalidScreenshot(t *testing.T) {
	root := t.TempDir()
	metadataDir := filepath.Join(root, "metadata", "en-US")
	if err := os.MkdirAll(metadataDir, 0o755); err != nil {
		t.Fatalf("mkdir metadata: %v", err)
	}
	writeFile(t, filepath.Join(metadataDir, "description.txt"), "English description")

	screenshotsDir := filepath.Join(root, "screenshots", "en-US")
	if err := os.MkdirAll(screenshotsDir, 0o755); err != nil {
		t.Fatalf("mkdir screenshots: %v", err)
	}
	badPath := filepath.Join(screenshotsDir, "bad.png")
	writeFile(t, badPath, "not an image")

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(cwd)
	})
	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	rootCmd := RootCommand("1.2.3")
	rootCmd.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := rootCmd.Parse([]string{
			"migrate", "import",
			"--app", "APP_ID",
			"--version-id", "VERSION_ID",
			"--dry-run",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = rootCmd.Run(context.Background())
	})

	if runErr == nil {
		t.Fatal("expected error, got nil")
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(runErr.Error(), badPath) {
		t.Fatalf("expected error to mention %q, got %v", badPath, runErr)
	}
}

func TestMigrateImportDryRunSkipScreenshotsFlag(t *testing.T) {
	root := t.TempDir()
	metadataDir := filepath.Join(root, "metadata", "en-US")
	if err := os.MkdirAll(metadataDir, 0o755); err != nil {
		t.Fatalf("mkdir metadata: %v", err)
	}
	writeFile(t, filepath.Join(metadataDir, "description.txt"), "English description")

	screenshotsDir := filepath.Join(root, "screenshots", "en-US")
	if err := os.MkdirAll(screenshotsDir, 0o755); err != nil {
		t.Fatalf("mkdir screenshots: %v", err)
	}
	writePNGForMigrate(t, filepath.Join(screenshotsDir, "iphone_65_screen.png"), 1242, 2688)

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(cwd)
	})
	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	rootCmd := RootCommand("1.2.3")
	rootCmd.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := rootCmd.Parse([]string{
			"migrate", "import",
			"--app", "APP_ID",
			"--version-id", "VERSION_ID",
			"--dry-run",
			"--skip-screenshots",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := rootCmd.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var result migrate.MigrateImportResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if len(result.ScreenshotPlan) != 0 {
		t.Fatalf("expected no screenshot plan, got %+v", result.ScreenshotPlan)
	}
	if len(result.Skipped) == 0 {
		t.Fatalf("expected screenshots dir to be reported as skipped")
	}
}

func TestMigrateImportSkipScreenshotsRejectsInvalidBooleanExitCode(t *testing.T) {
	assertUsageExit(
		t,
		[]string{"migrate", "import", "--app", "APP_ID", "--version-id", "VERSION_ID", "--skip-screenshots=maybe"},
		"invalid boolean value",
	)
}

func TestMigrateImportDryRunSkipScreenshotsAllowsMissingFastlaneScreenshotsDir(t *testing.T) {
	root := t.TempDir()
	fastlaneDir := filepath.Join(root, "fastlane")
	metadataDir := filepath.Join(fastlaneDir, "metadata", "en-US")
	if err := os.MkdirAll(metadataDir, 0o755); err != nil {
		t.Fatalf("mkdir metadata: %v", err)
	}
	writeFile(t, filepath.Join(metadataDir, "description.txt"), "English description")

	rootCmd := RootCommand("1.2.3")
	rootCmd.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := rootCmd.Parse([]string{
			"migrate", "import",
			"--app", "APP_ID",
			"--version-id", "VERSION_ID",
			"--fastlane-dir", fastlaneDir,
			"--dry-run",
			"--skip-screenshots",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := rootCmd.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var result migrate.MigrateImportResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if len(result.MetadataFiles) != 1 {
		t.Fatalf("expected metadata plan to survive missing screenshots dir, got %+v", result.MetadataFiles)
	}
	if len(result.ScreenshotPlan) != 0 {
		t.Fatalf("expected no screenshot plan, got %+v", result.ScreenshotPlan)
	}
}

func TestMigrateImportDryRunDeliverfileSkipScreenshotsAllowsMissingFastlaneScreenshotsDir(t *testing.T) {
	root := t.TempDir()
	fastlaneDir := filepath.Join(root, "fastlane")
	metadataDir := filepath.Join(fastlaneDir, "metadata", "en-US")
	if err := os.MkdirAll(metadataDir, 0o755); err != nil {
		t.Fatalf("mkdir metadata: %v", err)
	}
	writeFile(t, filepath.Join(metadataDir, "description.txt"), "English description")
	writeFile(t, filepath.Join(fastlaneDir, "Deliverfile"), "skip_screenshots true\n")

	rootCmd := RootCommand("1.2.3")
	rootCmd.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := rootCmd.Parse([]string{
			"migrate", "import",
			"--app", "APP_ID",
			"--version-id", "VERSION_ID",
			"--fastlane-dir", fastlaneDir,
			"--dry-run",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := rootCmd.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var result migrate.MigrateImportResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if len(result.MetadataFiles) != 1 {
		t.Fatalf("expected metadata plan to survive missing screenshots dir, got %+v", result.MetadataFiles)
	}
	if len(result.ScreenshotPlan) != 0 {
		t.Fatalf("expected no screenshot plan, got %+v", result.ScreenshotPlan)
	}
	foundDeliverfileSkip := false
	for _, skipped := range result.Skipped {
		if skipped.Reason == "skip_screenshots in Deliverfile" {
			foundDeliverfileSkip = true
			break
		}
	}
	if !foundDeliverfileSkip {
		t.Fatalf("expected skipped list to include Deliverfile skip_screenshots reason, got %+v", result.Skipped)
	}
}

func TestMigrateImportDryRunRejectsUnsupportedCreateLocale(t *testing.T) {
	root := t.TempDir()
	localeDir := filepath.Join(root, "metadata", "nl")
	if err := os.MkdirAll(localeDir, 0o755); err != nil {
		t.Fatalf("mkdir metadata: %v", err)
	}
	writeFile(t, filepath.Join(localeDir, "description.txt"), "Dutch description")

	factoryCalls := 0
	restore := shared.SetASCClientFactoryForTesting(func() (*asc.Client, error) {
		factoryCalls++
		return nil, errors.New("client factory must not run during a local dry-run")
	})
	t.Cleanup(restore)

	rootCmd := RootCommand("1.2.3")
	rootCmd.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := rootCmd.Parse([]string{
			"migrate", "import",
			"--app", "APP_ID",
			"--version-id", "VERSION_ID",
			"--fastlane-dir", root,
			"--skip-screenshots",
			"--dry-run",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = rootCmd.Run(context.Background())
	})

	const wantError = `migrate import: locale "nl": unsupported locale "nl"; did you mean: nl-NL`
	assertMigrateImportError(t, stdout, stderr, runErr, wantError)
	if factoryCalls != 0 {
		t.Fatalf("client factory calls = %d, want zero", factoryCalls)
	}
}

func TestMigrateImportFastlaneDirHonorsDeliverfileMetadataPath(t *testing.T) {
	root := t.TempDir()
	fastlaneDir := filepath.Join(root, "fastlane")
	staleDir := filepath.Join(fastlaneDir, "metadata", "en-US")
	if err := os.MkdirAll(staleDir, 0o755); err != nil {
		t.Fatalf("mkdir metadata: %v", err)
	}
	writeFile(t, filepath.Join(staleDir, "description.txt"), "STALE WRONG DESCRIPTION")

	prodDir := filepath.Join(fastlaneDir, "metadata_prod", "en-US")
	if err := os.MkdirAll(prodDir, 0o755); err != nil {
		t.Fatalf("mkdir metadata_prod: %v", err)
	}
	writeFile(t, filepath.Join(prodDir, "description.txt"), "Production description")
	writeFile(t, filepath.Join(fastlaneDir, "Deliverfile"), "metadata_path \"./metadata_prod\"\nskip_screenshots true\n")

	rootCmd := RootCommand("1.2.3")
	rootCmd.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := rootCmd.Parse([]string{
			"migrate", "import",
			"--app", "APP_ID",
			"--version-id", "VERSION_ID",
			"--fastlane-dir", fastlaneDir,
			"--dry-run",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := rootCmd.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var result migrate.MigrateImportResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if result.MetadataDir != filepath.Join(fastlaneDir, "metadata_prod") {
		t.Fatalf("metadataDir = %q, want Deliverfile metadata_path", result.MetadataDir)
	}
	if len(result.Localizations) != 1 || result.Localizations[0].Description != "Production description" {
		t.Fatalf("localizations = %+v, want the Deliverfile metadata_path description", result.Localizations)
	}
	wantReason := `unused because Deliverfile metadata_path "./metadata_prod" selects another directory`
	found := false
	for _, item := range result.Skipped {
		if item.Path == filepath.Join(fastlaneDir, "metadata") && item.Reason == wantReason {
			found = true
		}
	}
	if !found {
		t.Fatalf("skipped = %+v, want the overridden conventional metadata directory reported", result.Skipped)
	}
}

func migrateJSONResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write file %s: %v", path, err)
	}
}

func writePNGForMigrate(t *testing.T, path string, width, height int) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, color.RGBA{R: 10, G: 20, B: 30, A: 255})
		}
	}
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("create png: %v", err)
	}
	defer file.Close()
	if err := png.Encode(file, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
}
