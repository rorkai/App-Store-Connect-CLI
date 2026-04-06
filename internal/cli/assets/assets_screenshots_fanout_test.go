package assets

import (
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

func TestUploadScreenshotsFanoutUsesLocaleDirectoriesForResolvedVersion(t *testing.T) {
	rootDir := t.TempDir()
	enDir := filepath.Join(rootDir, "en-US", "iphone")
	frDir := filepath.Join(rootDir, "fr-FR", "iphone")
	if err := os.MkdirAll(enDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}
	if err := os.MkdirAll(frDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}
	writeAssetsTestPNGWithSize(t, enDir, "01-home.png", 1242, 2688)
	writeAssetsTestPNGWithSize(t, frDir, "01-home.png", 1242, 2688)

	origTransport := http.DefaultTransport
	http.DefaultTransport = assetsUploadRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
			return assetsJSONResponse(http.StatusOK, `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-en","attributes":{"locale":"en-US"}},{"type":"appStoreVersionLocalizations","id":"loc-fr","attributes":{"locale":"fr-FR"}},{"type":"appStoreVersionLocalizations","id":"loc-es","attributes":{"locale":"es-ES"}}],"links":{}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersionLocalizations/loc-en/appScreenshotSets":
			return assetsJSONResponse(http.StatusOK, `{"data":[],"links":{}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersionLocalizations/loc-fr/appScreenshotSets":
			return assetsJSONResponse(http.StatusOK, `{"data":[],"links":{}}`)
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})
	t.Cleanup(func() {
		http.DefaultTransport = origTransport
	})

	result, err := uploadScreenshotsFanout(context.Background(), screenshotUploadFanoutConfig{
		Client:      newAssetsUploadTestClient(t),
		AppID:       "123456789",
		Version:     "1.2.3",
		VersionID:   "version-1",
		Platform:    "IOS",
		RootPath:    rootDir,
		DisplayType: asc.CanonicalScreenshotDisplayTypeForAPI("APP_IPHONE_65"),
		DryRun:      true,
	})
	if err != nil {
		t.Fatalf("uploadScreenshotsFanout() error: %v", err)
	}

	if result.AppID != "123456789" {
		t.Fatalf("expected app ID 123456789, got %q", result.AppID)
	}
	if result.VersionID != "version-1" {
		t.Fatalf("expected version ID version-1, got %q", result.VersionID)
	}
	if len(result.Localizations) != 2 {
		t.Fatalf("expected 2 localization results, got %d", len(result.Localizations))
	}
	if result.Localizations[0].Locale != "en-US" || result.Localizations[0].VersionLocalizationID != "loc-en" {
		t.Fatalf("unexpected first localization result: %#v", result.Localizations[0])
	}
	if result.Localizations[1].Locale != "fr-FR" || result.Localizations[1].VersionLocalizationID != "loc-fr" {
		t.Fatalf("unexpected second localization result: %#v", result.Localizations[1])
	}
	for _, localization := range result.Localizations {
		if len(localization.Results) != 1 {
			t.Fatalf("expected one dry-run file result for %s, got %d", localization.Locale, len(localization.Results))
		}
		if localization.Results[0].State != "would-upload" {
			t.Fatalf("expected would-upload for %s, got %#v", localization.Locale, localization.Results[0])
		}
		if !strings.Contains(localization.Results[0].FilePath, localization.Locale) {
			t.Fatalf("expected file path to include locale %q, got %q", localization.Locale, localization.Results[0].FilePath)
		}
	}
}

func TestUploadScreenshotsFanoutErrorsWhenLocalLocaleHasNoRemoteMatch(t *testing.T) {
	rootDir := t.TempDir()
	jaDir := filepath.Join(rootDir, "ja", "iphone")
	if err := os.MkdirAll(jaDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}
	writeAssetsTestPNGWithSize(t, jaDir, "01-home.png", 1242, 2688)

	origTransport := http.DefaultTransport
	http.DefaultTransport = assetsUploadRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
			return assetsJSONResponse(http.StatusOK, `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-en","attributes":{"locale":"en-US"}}],"links":{}}`)
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})
	t.Cleanup(func() {
		http.DefaultTransport = origTransport
	})

	_, err := uploadScreenshotsFanout(context.Background(), screenshotUploadFanoutConfig{
		Client:      newAssetsUploadTestClient(t),
		AppID:       "123456789",
		Version:     "1.2.3",
		VersionID:   "version-1",
		Platform:    "IOS",
		RootPath:    rootDir,
		DisplayType: asc.CanonicalScreenshotDisplayTypeForAPI("APP_IPHONE_65"),
		DryRun:      true,
	})
	if err == nil {
		t.Fatal("expected missing remote localization error")
	}
	if !strings.Contains(err.Error(), `no matching App Store version localizations found for locales: ja`) {
		t.Fatalf("expected missing locale error, got %v", err)
	}
}

func TestUploadScreenshotsFanoutFiltersMixedDeviceTreesBySelectedDisplayType(t *testing.T) {
	rootDir := t.TempDir()
	enIPhoneDir := filepath.Join(rootDir, "en-US", "iphone")
	enIPadDir := filepath.Join(rootDir, "en-US", "ipad")
	if err := os.MkdirAll(enIPhoneDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}
	if err := os.MkdirAll(enIPadDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}
	writeAssetsTestPNGWithSize(t, enIPhoneDir, "01-home.png", 1242, 2688)
	writeAssetsTestPNGWithSize(t, enIPadDir, "01-home.png", 2048, 2732)

	origTransport := http.DefaultTransport
	http.DefaultTransport = assetsUploadRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
			return assetsJSONResponse(http.StatusOK, `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-en","attributes":{"locale":"en-US"}}],"links":{}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersionLocalizations/loc-en/appScreenshotSets":
			return assetsJSONResponse(http.StatusOK, `{"data":[],"links":{}}`)
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})
	t.Cleanup(func() {
		http.DefaultTransport = origTransport
	})

	result, err := uploadScreenshotsFanout(context.Background(), screenshotUploadFanoutConfig{
		Client:      newAssetsUploadTestClient(t),
		AppID:       "123456789",
		Version:     "1.2.3",
		VersionID:   "version-1",
		Platform:    "IOS",
		RootPath:    rootDir,
		DisplayType: asc.CanonicalScreenshotDisplayTypeForAPI("APP_IPHONE_65"),
		DryRun:      true,
	})
	if err != nil {
		t.Fatalf("uploadScreenshotsFanout() error: %v", err)
	}

	if len(result.Localizations) != 1 {
		t.Fatalf("expected 1 localization result, got %d", len(result.Localizations))
	}
	if len(result.Localizations[0].Results) != 1 {
		t.Fatalf("expected 1 selected file result, got %d", len(result.Localizations[0].Results))
	}
	if !strings.Contains(result.Localizations[0].Results[0].FilePath, filepath.Join("en-US", "iphone")) {
		t.Fatalf("expected iPhone file to be selected, got %q", result.Localizations[0].Results[0].FilePath)
	}
}

func TestCollectLocaleAssetFilesSkipsIgnoredSubdirectoriesWithoutMatchingScreenshots(t *testing.T) {
	rootDir := t.TempDir()
	enDir := filepath.Join(rootDir, "en-US", "iphone")
	hiddenDir := filepath.Join(rootDir, ".git")
	buildDir := filepath.Join(rootDir, "build")
	if err := os.MkdirAll(enDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}
	if err := os.MkdirAll(hiddenDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}
	if err := os.MkdirAll(buildDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}
	writeAssetsTestPNGWithSize(t, enDir, "01-home.png", 1242, 2688)
	writeAssetsTestPNGWithSize(t, buildDir, "icon.png", 100, 100)
	if err := os.WriteFile(filepath.Join(hiddenDir, "HEAD"), []byte("ref: refs/heads/main\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	files, err := collectLocaleAssetFiles(rootDir, asc.CanonicalScreenshotDisplayTypeForAPI("APP_IPHONE_65"))
	if err != nil {
		t.Fatalf("collectLocaleAssetFiles() error: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 locale result, got %d", len(files))
	}
	if files[0].Locale != "en-US" {
		t.Fatalf("expected en-US locale, got %#v", files[0])
	}
}

func TestCollectLocaleAssetFilesIgnoresMalformedProbeFilesInNonLocaleDirectories(t *testing.T) {
	rootDir := t.TempDir()
	enDir := filepath.Join(rootDir, "en-US", "iphone")
	buildDir := filepath.Join(rootDir, "build")
	if err := os.MkdirAll(enDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}
	if err := os.MkdirAll(buildDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}
	writeAssetsTestPNGWithSize(t, enDir, "01-home.png", 1242, 2688)
	if err := os.WriteFile(filepath.Join(buildDir, "stale.png"), []byte("not a png"), 0o644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	files, err := collectLocaleAssetFiles(rootDir, asc.CanonicalScreenshotDisplayTypeForAPI("APP_IPHONE_65"))
	if err != nil {
		t.Fatalf("collectLocaleAssetFiles() error: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 locale result, got %d", len(files))
	}
	if files[0].Locale != "en-US" {
		t.Fatalf("expected en-US locale, got %#v", files[0])
	}
}

func TestCollectLocaleAssetFilesIgnoresUnknownLocaleLikeDirectoriesWithoutMatchingScreenshots(t *testing.T) {
	rootDir := t.TempDir()
	enDir := filepath.Join(rootDir, "en-US", "iphone")
	imgDir := filepath.Join(rootDir, "img")
	if err := os.MkdirAll(enDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}
	if err := os.MkdirAll(imgDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}
	writeAssetsTestPNGWithSize(t, enDir, "01-home.png", 1242, 2688)
	writeAssetsTestPNGWithSize(t, imgDir, "icon.png", 100, 100)

	files, err := collectLocaleAssetFiles(rootDir, asc.CanonicalScreenshotDisplayTypeForAPI("APP_IPHONE_65"))
	if err != nil {
		t.Fatalf("collectLocaleAssetFiles() error: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 locale result, got %d", len(files))
	}
	if files[0].Locale != "en-US" {
		t.Fatalf("expected en-US locale, got %#v", files[0])
	}
}

func TestCollectLocaleAssetFilesErrorsOnInvalidLocaleDirectoryWithMatchingScreenshots(t *testing.T) {
	rootDir := t.TempDir()
	iphoneDir := filepath.Join(rootDir, "iphone")
	if err := os.MkdirAll(iphoneDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}
	writeAssetsTestPNGWithSize(t, iphoneDir, "01-home.png", 1242, 2688)

	_, err := collectLocaleAssetFiles(rootDir, asc.CanonicalScreenshotDisplayTypeForAPI("APP_IPHONE_65"))
	if err == nil {
		t.Fatal("expected invalid locale directory error")
	}
	if !strings.Contains(err.Error(), `invalid locale directory "iphone"`) {
		t.Fatalf("expected invalid locale error, got %v", err)
	}
}

func TestCollectLocaleAssetFilesRejectsDuplicateCanonicalLocales(t *testing.T) {
	rootDir := t.TempDir()
	enUSDir := filepath.Join(rootDir, "en-US", "iphone")
	enUnderscoreDir := filepath.Join(rootDir, "en_US", "iphone")
	if err := os.MkdirAll(enUSDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}
	if err := os.MkdirAll(enUnderscoreDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}
	writeAssetsTestPNGWithSize(t, enUSDir, "01-home.png", 1242, 2688)
	writeAssetsTestPNGWithSize(t, enUnderscoreDir, "02-home.png", 1242, 2688)

	_, err := collectLocaleAssetFiles(rootDir, asc.CanonicalScreenshotDisplayTypeForAPI("APP_IPHONE_65"))
	if err == nil {
		t.Fatal("expected duplicate locale directory error")
	}
	if !strings.Contains(err.Error(), `duplicate locale "en-US" in fan-out path (dirs: "en-US", "en_US")`) {
		t.Fatalf("expected duplicate locale error, got %v", err)
	}
}

func TestCollectLocaleAssetFilesRejectsSymlinkLocaleDirectories(t *testing.T) {
	rootDir := t.TempDir()
	frDir := filepath.Join(rootDir, "fr-FR", "iphone")
	if err := os.MkdirAll(frDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}
	writeAssetsTestPNGWithSize(t, frDir, "01-home.png", 1242, 2688)

	sharedDir := filepath.Join(t.TempDir(), "shared-en", "iphone")
	if err := os.MkdirAll(sharedDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}
	writeAssetsTestPNGWithSize(t, sharedDir, "01-home.png", 1242, 2688)

	symlinkPath := filepath.Join(rootDir, "en-US")
	if err := os.Symlink(filepath.Dir(sharedDir), symlinkPath); err != nil {
		t.Skipf("symlink not supported: %v", err)
	}

	_, err := collectLocaleAssetFiles(rootDir, asc.CanonicalScreenshotDisplayTypeForAPI("APP_IPHONE_65"))
	if err == nil {
		t.Fatal("expected symlink locale directory error")
	}
	if !strings.Contains(err.Error(), "refusing to read symlink") || !strings.Contains(err.Error(), symlinkPath) {
		t.Fatalf("expected symlink rejection error, got %v", err)
	}
}

func TestCollectLocaleAssetFilesRecursiveSkipsNonImageFiles(t *testing.T) {
	rootDir := t.TempDir()
	if err := os.MkdirAll(rootDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}
	writeAssetsTestPNGWithSize(t, rootDir, "01-home.png", 1242, 2688)
	if err := os.WriteFile(filepath.Join(rootDir, "README.md"), []byte("notes"), 0o644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(rootDir, ".gitkeep"), []byte{}, 0o644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	files, err := collectLocaleAssetFilesRecursive(rootDir, asc.CanonicalScreenshotDisplayTypeForAPI("APP_IPHONE_65"))
	if err != nil {
		t.Fatalf("collectLocaleAssetFilesRecursive() error: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 matching screenshot file, got %d", len(files))
	}
	if filepath.Base(files[0]) != "01-home.png" {
		t.Fatalf("expected 01-home.png, got %q", files[0])
	}
}

func TestExecuteScreenshotUploadCommandValidatesFanoutFilesBeforeClientCreation(t *testing.T) {
	rootDir := t.TempDir()
	enDir := filepath.Join(rootDir, "en-US")
	if err := os.MkdirAll(enDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}
	writeAssetsTestPNGWithSize(t, enDir, "01-home.png", 100, 100)

	clientCalled := false
	_, err := executeScreenshotUploadCommand(context.Background(), screenshotUploadCommandOptions{
		AppID:      "123456789",
		Version:    "1.2.3",
		Path:       rootDir,
		DeviceType: "IPHONE_65",
	}, screenshotUploadDependencies{
		GetClient: func() (*asc.Client, error) {
			clientCalled = true
			return nil, nil
		},
	})
	if err == nil {
		t.Fatal("expected local validation error")
	}
	if clientCalled {
		t.Fatal("expected client creation to be skipped on local validation failure")
	}
	if !strings.Contains(err.Error(), "no screenshot files matching APP_IPHONE_65 found") {
		t.Fatalf("expected local validation error, got %v", err)
	}
}

func TestExecuteScreenshotUploadCommandUsesASCAppIDFallbackForExplicitAppMode(t *testing.T) {
	t.Setenv("ASC_APP_ID", "123456789")

	rootDir := t.TempDir()
	enDir := filepath.Join(rootDir, "en-US", "iphone")
	if err := os.MkdirAll(enDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}
	writeAssetsTestPNGWithSize(t, enDir, "01-home.png", 1242, 2688)

	sentinelErr := errors.New("client requested")
	clientCalled := false

	_, err := executeScreenshotUploadCommand(context.Background(), screenshotUploadCommandOptions{
		Version:    "1.2.3",
		Path:       rootDir,
		DeviceType: "IPHONE_65",
	}, screenshotUploadDependencies{
		GetClient: func() (*asc.Client, error) {
			clientCalled = true
			return nil, sentinelErr
		},
	})
	if !errors.Is(err, sentinelErr) {
		t.Fatalf("expected sentinel client error, got %v", err)
	}
	if !clientCalled {
		t.Fatal("expected client creation when app mode is explicitly requested")
	}
}

func TestUploadScreenshotsFanoutRejectsDuplicateLocaleAssets(t *testing.T) {
	rootDir := t.TempDir()
	writeAssetsTestPNGWithSize(t, rootDir, "01-home.png", 1242, 2688)
	writeAssetsTestPNGWithSize(t, rootDir, "02-home.png", 1242, 2688)

	_, err := uploadScreenshotsFanout(context.Background(), screenshotUploadFanoutConfig{
		Client:      newAssetsUploadTestClient(t),
		VersionID:   "version-1",
		DisplayType: asc.CanonicalScreenshotDisplayTypeForAPI("APP_IPHONE_65"),
		LocaleAssets: []screenshotLocaleAssetFiles{
			{Locale: "en-US", Files: []string{filepath.Join(rootDir, "01-home.png")}},
			{Locale: "en_US", Files: []string{filepath.Join(rootDir, "02-home.png")}},
		},
	})
	if err == nil {
		t.Fatal("expected duplicate locale upload error")
	}
	if !strings.Contains(err.Error(), `duplicate locale "en-US" in fan-out upload (inputs: "en-US", "en_US")`) {
		t.Fatalf("expected duplicate locale upload error, got %v", err)
	}
}
