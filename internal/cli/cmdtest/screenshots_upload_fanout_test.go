package cmdtest

import (
	"encoding/json"
	"errors"
	"flag"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
)

func TestScreenshotsUploadAppScopedModeRequiresVersionSelector(t *testing.T) {
	stdout, stderr, runErr := runRootCommand(t, []string{
		"screenshots", "upload",
		"--app", "123456789",
		"--path", "./screenshots",
		"--device-type", "IPHONE_65",
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp, got %v", runErr)
	}
	if !strings.Contains(stderr, "Error: --version or --version-id is required with --app") {
		t.Fatalf("expected missing app-scoped version selector error, got %q", stderr)
	}
}

func TestScreenshotsUploadRejectsMixingDirectAndAppScopedSelectors(t *testing.T) {
	stdout, stderr, runErr := runRootCommand(t, []string{
		"screenshots", "upload",
		"--version-localization", "LOC_ID",
		"--app", "123456789",
		"--version", "1.2.3",
		"--path", "./screenshots",
		"--device-type", "IPHONE_65",
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp, got %v", runErr)
	}
	if !strings.Contains(stderr, "Error: --version-localization cannot be combined with --app, --version, --version-id, or --platform") {
		t.Fatalf("expected direct/app-scoped selector conflict error, got %q", stderr)
	}
}

func TestScreenshotsUploadIgnoresASCAppIDUntilAppScopedModeIsRequested(t *testing.T) {
	t.Setenv("ASC_APP_ID", "123456789")

	stdout, stderr, runErr := runRootCommand(t, []string{
		"screenshots", "upload",
		"--path", "./screenshots",
		"--device-type", "IPHONE_65",
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp, got %v", runErr)
	}
	if !strings.Contains(stderr, "Error: choose an upload mode: --version-localization VERSION_LOCALIZATION_ID; (--app APP_ID or ASC_APP_ID) with --version VERSION or --version-id VERSION_ID; or --resume ARTIFACT_PATH") {
		t.Fatalf("expected upload mode selector error, got %q", stderr)
	}
}

func TestScreenshotsUploadAppScopedModeRejectsInvalidPlatformBeforeAuth(t *testing.T) {
	t.Setenv("ASC_KEY_ID", "")
	t.Setenv("ASC_ISSUER_ID", "")
	t.Setenv("ASC_PRIVATE_KEY_PATH", "")
	t.Setenv("ASC_PRIVATE_KEY", "")
	t.Setenv("ASC_PRIVATE_KEY_B64", "")
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_PROFILE", "")
	t.Setenv("ASC_STRICT_AUTH", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	rootDir := t.TempDir()
	localeDir := filepath.Join(rootDir, "en-US", "iphone")
	if err := os.MkdirAll(localeDir, 0o755); err != nil {
		t.Fatalf("mkdir locale dir: %v", err)
	}
	writePNG(t, filepath.Join(localeDir, "01-home.png"), 1284, 2778)

	stdout, stderr, runErr := runRootCommand(t, []string{
		"screenshots", "upload",
		"--app", "123456789",
		"--version", "1.2.3",
		"--platform", "ANDROID",
		"--path", rootDir,
		"--device-type", "IPHONE_65",
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp, got %v", runErr)
	}
	if !strings.Contains(stderr, "Error: --platform must be one of: IOS, MAC_OS, TV_OS, VISION_OS") {
		t.Fatalf("expected invalid platform usage error, got %q", stderr)
	}
	if strings.Contains(stderr, "screenshots upload:") {
		t.Fatalf("expected raw usage error without command prefix, got %q", stderr)
	}
}

func TestRunScreenshotsUploadVersionIDUsesResolvedPlatformWithoutExplicitPlatform(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	workDir := t.TempDir()
	localeDir := filepath.Join(workDir, "en-US")
	if err := os.MkdirAll(localeDir, 0o755); err != nil {
		t.Fatalf("mkdir locale dir: %v", err)
	}
	writePNG(t, filepath.Join(localeDir, "01-home.png"), 2880, 1800)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-mac":
			return screenshotsUploadJSONResponse(http.StatusOK, `{"data":{"type":"appStoreVersions","id":"version-mac","attributes":{"platform":"MAC_OS","versionString":"2.0.0"},"relationships":{"app":{"data":{"type":"apps","id":"123456789"}}}}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-mac/appStoreVersionLocalizations":
			return screenshotsUploadJSONResponse(http.StatusOK, `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-en","attributes":{"locale":"en-US"}}],"links":{}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersionLocalizations/loc-en/appScreenshotSets":
			return screenshotsUploadJSONResponse(http.StatusOK, `{"data":[],"links":{}}`)
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})

	var payload struct {
		VersionID   string `json:"versionId"`
		Version     string `json:"version"`
		Platform    string `json:"platform"`
		DisplayType string `json:"displayType"`
		DryRun      bool   `json:"dryRun"`
	}

	stdout, stderr := captureOutput(t, func() {
		code := cmd.Run([]string{
			"screenshots", "upload",
			"--app", "123456789",
			"--version-id", "version-mac",
			"--path", workDir,
			"--device-type", "DESKTOP",
			"--dry-run",
			"--output", "json",
		}, "1.2.3")
		if code != cmd.ExitSuccess {
			t.Fatalf("expected exit code %d, got %d", cmd.ExitSuccess, code)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("failed to parse stdout JSON: %v\nstdout=%s", err, stdout)
	}
	if payload.VersionID != "version-mac" {
		t.Fatalf("expected versionId version-mac, got %q", payload.VersionID)
	}
	if payload.Version != "2.0.0" {
		t.Fatalf("expected version 2.0.0, got %q", payload.Version)
	}
	if payload.Platform != "MAC_OS" {
		t.Fatalf("expected platform MAC_OS, got %q", payload.Platform)
	}
	if payload.DisplayType != "APP_DESKTOP" {
		t.Fatalf("expected display type APP_DESKTOP, got %q", payload.DisplayType)
	}
	if !payload.DryRun {
		t.Fatalf("expected dryRun=true, got %#v", payload)
	}
}
