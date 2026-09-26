package assets

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

const screenshotLocaleTestLocalizationsJSON = `{"data":[` +
	`{"type":"appStoreVersionLocalizations","id":"loc-en","attributes":{"locale":"en-US"}},` +
	`{"type":"appStoreVersionLocalizations","id":"loc-fr","attributes":{"locale":"fr-FR"}}` +
	`],"links":{}}`

// isOtherLocaleScreenshotRequest reports whether a request touches the fr-FR
// localization, its screenshot set, or its screenshots.
func isOtherLocaleScreenshotRequest(req *http.Request) bool {
	return strings.Contains(req.URL.Path, "loc-fr") ||
		strings.Contains(req.URL.Path, "set-fr") ||
		strings.Contains(req.URL.Path, "existing-fr")
}

func TestExecuteScreenshotUploadCommandLocaleReplaceTouchesOnlySelectedLocaleSet(t *testing.T) {
	t.Chdir(t.TempDir())
	localeDir := t.TempDir()
	filePath := writeAssetsTestPNGWithSize(t, localeDir, "01-home.png", 1242, 2688)
	fileSizeBytes := fileSize(t, filePath)

	var (
		mu               sync.Mutex
		deleted          []string
		otherLocaleCalls []string
		reservedSets     []string
	)
	client := newAssetsUploadTestServerClient(t, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		if isOtherLocaleScreenshotRequest(req) {
			otherLocaleCalls = append(otherLocaleCalls, req.Method+" "+req.URL.Path)
			http.Error(w, "other locale must not be touched", http.StatusConflict)
			return
		}
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/123456789/appStoreVersions":
			if got := req.URL.Query().Get("filter[versionString]"); got != "1.2.3" {
				t.Errorf("filter[versionString] = %q, want 1.2.3", got)
			}
			writeAssetsTestJSON(w, http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"platform":"IOS","versionString":"1.2.3"}}],"links":{}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
			writeAssetsTestJSON(w, http.StatusOK, screenshotLocaleTestLocalizationsJSON)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersionLocalizations/loc-en/appScreenshotSets":
			writeAssetsTestJSON(w, http.StatusOK, `{"data":[{"type":"appScreenshotSets","id":"set-en","attributes":{"screenshotDisplayType":"APP_IPHONE_65"}}],"links":{}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appScreenshotSets/set-en/appScreenshots":
			writeAssetsTestJSON(w, http.StatusOK, `{"data":[{"type":"appScreenshots","id":"existing-en-1","attributes":{"fileName":"old.png"}}],"links":{}}`)
		case req.Method == http.MethodDelete && req.URL.Path == "/v1/appScreenshots/existing-en-1":
			deleted = append(deleted, "existing-en-1")
			w.WriteHeader(http.StatusNoContent)
		case req.Method == http.MethodPost && req.URL.Path == "/v1/appScreenshots":
			reservedSets = append(reservedSets, "set-en")
			writeAssetsTestJSON(w, http.StatusCreated, fmt.Sprintf(`{"data":{"type":"appScreenshots","id":"new-en-1","attributes":{"uploadOperations":[{"method":"PUT","url":"http://%s/uploads/new-en-1","length":%d,"offset":0}]}}}`, req.Host, fileSizeBytes))
		case req.Method == http.MethodPut && req.URL.Path == "/uploads/new-en-1":
			writeAssetsTestJSON(w, http.StatusOK, `{}`)
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/appScreenshots/new-en-1":
			writeAssetsTestJSON(w, http.StatusOK, `{"data":{"type":"appScreenshots","id":"new-en-1","attributes":{"assetDeliveryState":{"state":"UPLOAD_COMPLETE"}}}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appScreenshots/new-en-1":
			writeAssetsTestJSON(w, http.StatusOK, `{"data":{"type":"appScreenshots","id":"new-en-1","attributes":{"sourceFileChecksum":"settled","assetDeliveryState":{"state":"COMPLETE"}}}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appScreenshotSets/set-en/relationships/appScreenshots":
			writeAssetsTestJSON(w, http.StatusOK, `{"data":[{"type":"appScreenshots","id":"new-en-1"}],"links":{}}`)
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/appScreenshotSets/set-en/relationships/appScreenshots":
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Errorf("unexpected request: %s %s", req.Method, req.URL.String())
			http.Error(w, "unexpected request", http.StatusNotFound)
		}
	}))

	result, err := executeScreenshotUploadCommand(context.Background(), screenshotUploadCommandOptions{
		AppID:      "123456789",
		Version:    "1.2.3",
		Locale:     "en-us",
		Path:       localeDir,
		DeviceType: "IPHONE_65",
		Replace:    true,
		Confirm:    true,
	}, screenshotUploadDependencies{
		GetClient: func() (*asc.Client, error) { return client, nil },
	})
	if err != nil {
		t.Fatalf("executeScreenshotUploadCommand() error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(otherLocaleCalls) > 0 {
		t.Fatalf("--replace with --locale en-US touched fr-FR resources: %v", otherLocaleCalls)
	}
	if strings.Join(deleted, ",") != "existing-en-1" {
		t.Fatalf("deleted = %v, want only existing-en-1", deleted)
	}
	if strings.Join(reservedSets, ",") != "set-en" {
		t.Fatalf("reservations = %v, want one in set-en", reservedSets)
	}

	fanout, ok := result.(*asc.AppScreenshotFanoutUploadResult)
	if !ok {
		t.Fatalf("result type = %T, want *asc.AppScreenshotFanoutUploadResult", result)
	}
	if fanout.AppID != "123456789" || fanout.Version != "1.2.3" || fanout.VersionID != "version-1" || fanout.Platform != "IOS" {
		t.Fatalf("unexpected fan-out header: %#v", fanout)
	}
	if len(fanout.Localizations) != 1 {
		t.Fatalf("expected exactly one localization result, got %#v", fanout.Localizations)
	}
	got := fanout.Localizations[0]
	if got.Locale != "en-US" || got.VersionLocalizationID != "loc-en" || got.SetID != "set-en" || got.Uploaded != 1 {
		t.Fatalf("unexpected localization result: %#v", got)
	}
}

func TestExecuteScreenshotUploadCommandLocaleDryRunAcceptsSingleFilePath(t *testing.T) {
	localeDir := t.TempDir()
	filePath := writeAssetsTestPNGWithSize(t, localeDir, "01-home.png", 1242, 2688)

	var otherLocaleCalls []string
	client := newAssetsUploadTestServerClient(t, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if isOtherLocaleScreenshotRequest(req) {
			otherLocaleCalls = append(otherLocaleCalls, req.Method+" "+req.URL.Path)
			http.Error(w, "other locale must not be touched", http.StatusConflict)
			return
		}
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-1":
			writeAssetsTestJSON(w, http.StatusOK, `{"data":{"type":"appStoreVersions","id":"version-1","attributes":{"platform":"IOS","versionString":"1.2.3"},"relationships":{"app":{"data":{"type":"apps","id":"123456789"}}}}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
			writeAssetsTestJSON(w, http.StatusOK, screenshotLocaleTestLocalizationsJSON)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersionLocalizations/loc-en/appScreenshotSets":
			writeAssetsTestJSON(w, http.StatusOK, `{"data":[],"links":{}}`)
		default:
			t.Errorf("unexpected request: %s %s", req.Method, req.URL.String())
			http.Error(w, "unexpected request", http.StatusNotFound)
		}
	}))

	result, err := executeScreenshotUploadCommand(context.Background(), screenshotUploadCommandOptions{
		AppID:      "123456789",
		VersionID:  "version-1",
		Locale:     "en-US",
		Path:       filePath,
		DeviceType: "IPHONE_65",
		DryRun:     true,
	}, screenshotUploadDependencies{
		GetClient: func() (*asc.Client, error) { return client, nil },
	})
	if err != nil {
		t.Fatalf("executeScreenshotUploadCommand() error: %v", err)
	}
	if len(otherLocaleCalls) > 0 {
		t.Fatalf("--locale en-US dry run touched fr-FR resources: %v", otherLocaleCalls)
	}
	fanout, ok := result.(*asc.AppScreenshotFanoutUploadResult)
	if !ok {
		t.Fatalf("result type = %T, want *asc.AppScreenshotFanoutUploadResult", result)
	}
	if !fanout.DryRun || len(fanout.Localizations) != 1 {
		t.Fatalf("unexpected dry-run result: %#v", fanout)
	}
	item := fanout.Localizations[0]
	if item.Locale != "en-US" || len(item.Results) != 1 || item.Results[0].State != "would-upload" || item.Results[0].FileName != "01-home.png" {
		t.Fatalf("unexpected dry-run localization result: %#v", item)
	}
}

// --locale points --path directly at the locale's files, so a directory that
// contains locale subdirectories is not scanned as a fan-out tree.
func TestExecuteScreenshotUploadCommandLocaleDoesNotScanLocaleSubdirectories(t *testing.T) {
	rootDir := t.TempDir()
	for _, locale := range []string{"en-US", "fr-FR"} {
		dir := filepath.Join(rootDir, locale)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("MkdirAll() error: %v", err)
		}
		writeAssetsTestPNGWithSize(t, dir, "01-home.png", 1242, 2688)
	}

	clientCalled := false
	_, err := executeScreenshotUploadCommand(context.Background(), screenshotUploadCommandOptions{
		AppID:      "123456789",
		Version:    "1.2.3",
		Locale:     "en-US",
		Path:       rootDir,
		DeviceType: "IPHONE_65",
		DryRun:     true,
	}, screenshotUploadDependencies{
		GetClient: func() (*asc.Client, error) {
			clientCalled = true
			return nil, errors.New("client must not be created")
		},
	})
	if err == nil {
		t.Fatal("expected an error for a --locale path without screenshot files")
	}
	if !strings.Contains(err.Error(), "no files found") {
		t.Fatalf("error = %v, want no files found", err)
	}
	if clientCalled {
		t.Fatal("client was created before local validation failed")
	}
}

func TestExecuteScreenshotUploadCommandLocaleMissingRemoteLocalizationListsValidLocales(t *testing.T) {
	localeDir := t.TempDir()
	writeAssetsTestPNGWithSize(t, localeDir, "01-home.png", 1242, 2688)

	client := newAssetsUploadTestServerClient(t, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/123456789/appStoreVersions":
			writeAssetsTestJSON(w, http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"platform":"IOS","versionString":"1.2.3"}}],"links":{}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
			writeAssetsTestJSON(w, http.StatusOK, screenshotLocaleTestLocalizationsJSON)
		default:
			t.Errorf("unexpected request: %s %s", req.Method, req.URL.String())
			http.Error(w, "unexpected request", http.StatusNotFound)
		}
	}))

	result, err := executeScreenshotUploadCommand(context.Background(), screenshotUploadCommandOptions{
		AppID:      "123456789",
		Version:    "1.2.3",
		Locale:     "de-DE",
		Path:       localeDir,
		DeviceType: "IPHONE_65",
		Replace:    true,
		Confirm:    true,
	}, screenshotUploadDependencies{
		GetClient: func() (*asc.Client, error) { return client, nil },
	})
	if err == nil {
		t.Fatal("expected missing localization error")
	}
	want := `no App Store version localization found for locale "de-DE"; available locales: en-US, fr-FR`
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("error = %v, want %q", err, want)
	}
	if fanout, ok := result.(*asc.AppScreenshotFanoutUploadResult); ok && len(fanout.Localizations) > 0 {
		t.Fatalf("expected no localization results, got %#v", fanout.Localizations)
	}
}

func TestExecuteScreenshotUploadCommandLocaleUsageErrors(t *testing.T) {
	localeDir := t.TempDir()
	writeAssetsTestPNGWithSize(t, localeDir, "01-home.png", 1242, 2688)

	tests := []struct {
		name string
		opts screenshotUploadCommandOptions
		want string
	}{
		{
			name: "with version localization",
			opts: screenshotUploadCommandOptions{VersionLocalizationID: "LOC_ID", Locale: "en-US"},
			want: "--locale cannot be combined with --version-localization",
		},
		{
			name: "without app-scoped mode",
			opts: screenshotUploadCommandOptions{Locale: "en-US"},
			want: "--locale requires --app with --version or --version-id",
		},
		{
			name: "multiple locales",
			opts: screenshotUploadCommandOptions{AppID: "123456789", Version: "1.2.3", Locale: "en-US,fr-FR"},
			want: "--locale accepts exactly one locale",
		},
		{
			name: "invalid locale",
			opts: screenshotUploadCommandOptions{AppID: "123456789", Version: "1.2.3", Locale: "not a locale"},
			want: "--locale",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("ASC_APP_ID", "")
			opts := test.opts
			opts.Path = localeDir
			opts.DeviceType = "IPHONE_65"
			clientCalled := false
			_, err := executeScreenshotUploadCommand(context.Background(), opts, screenshotUploadDependencies{
				GetClient: func() (*asc.Client, error) {
					clientCalled = true
					return nil, errors.New("client must not be created")
				},
			})
			if err == nil {
				t.Fatal("expected usage error")
			}
			if !errors.Is(err, flag.ErrHelp) {
				t.Fatalf("error = %v, want usage error", err)
			}
			if !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want %q", err, test.want)
			}
			if clientCalled {
				t.Fatal("client was created before the usage error")
			}
		})
	}
}
