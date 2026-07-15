package cmdtest

import (
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
)

func TestRunScreenshotsUploadResumeRejectsSelectorFlags(t *testing.T) {
	_, stderr := captureOutput(t, func() {
		code := cmd.Run([]string{
			"screenshots", "upload",
			"--resume", "artifact.json",
			"--app", "123456789",
			"--version", "1.2.3",
			"--path", "./screenshots",
		}, "1.2.3")
		if code != cmd.ExitUsage {
			t.Fatalf("expected exit code %d, got %d", cmd.ExitUsage, code)
		}
	})

	if !strings.Contains(stderr, "--resume cannot be combined with --version-localization, --app, --version, --version-id, --platform, --path, or --device-type") {
		t.Fatalf("expected resume conflict message, got %q", stderr)
	}
}

func TestRunScreenshotsUploadResumeRejectsExecutionModeFlags(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{
			name: "skip-existing",
			args: []string{"--skip-existing"},
		},
		{
			name: "replace",
			args: []string{"--replace"},
		},
		{
			name: "dry-run",
			args: []string{"--dry-run"},
		},
		{
			name: "max-screenshots",
			args: []string{"--max-screenshots", "5"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, stderr := captureOutput(t, func() {
				runArgs := append([]string{
					"screenshots", "upload",
					"--resume", "artifact.json",
				}, tt.args...)
				code := cmd.Run(runArgs, "1.2.3")
				if code != cmd.ExitUsage {
					t.Fatalf("expected exit code %d, got %d", cmd.ExitUsage, code)
				}
			})

			if !strings.Contains(stderr, "--resume cannot be combined with --skip-existing, --replace, --dry-run, or --max-screenshots") {
				t.Fatalf("expected resume execution-mode conflict message, got %q", stderr)
			}
		})
	}
}

func TestRunScreenshotsUploadWritesFailureArtifactAndResumeCompletes(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	workDir := t.TempDir()
	previousDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error: %v", err)
	}
	if err := os.Chdir(workDir); err != nil {
		t.Fatalf("Chdir() error: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(previousDir)
	})

	first := writeCmdtestScreenshotPNG(t, workDir, "01-home.png")
	second := writeCmdtestScreenshotPNG(t, workDir, "02-settings.png")
	third := writeCmdtestScreenshotPNG(t, workDir, "03-profile.png")

	firstSize := cmdtestFileSize(t, first)
	secondSize := cmdtestFileSize(t, second)
	thirdSize := cmdtestFileSize(t, third)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	phase := "first"
	relationshipPatchCount := 0

	// Uploads run concurrently: the failing creates block until 01-home.png
	// fully completes so the partial-failure outcome stays deterministic.
	firstUploadComplete := make(chan struct{})
	var firstUploadCompleteOnce sync.Once

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersionLocalizations/LOC_123/appScreenshotSets":
			return screenshotsUploadJSONResponse(http.StatusOK, `{"data":[{"type":"appScreenshotSets","id":"set-1","attributes":{"screenshotDisplayType":"APP_IPHONE_65"}}],"links":{}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appScreenshotSets/set-1/appScreenshots":
			if phase == "resume" {
				return screenshotsUploadJSONResponse(http.StatusOK, `{"data":[{"type":"appScreenshots","id":"new-1"}],"links":{}}`)
			}
			return screenshotsUploadJSONResponse(http.StatusOK, `{"data":[],"links":{}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appScreenshotSets/set-1/relationships/appScreenshots":
			if phase == "resume" {
				return screenshotsUploadJSONResponse(http.StatusOK, `{"data":[{"type":"appScreenshots","id":"new-1"}],"links":{}}`)
			}
			return screenshotsUploadJSONResponse(http.StatusOK, `{"data":[],"links":{}}`)
		case req.Method == http.MethodPost && req.URL.Path == "/v1/appScreenshots":
			body, readErr := io.ReadAll(req.Body)
			if readErr != nil {
				return nil, readErr
			}
			if phase == "first" {
				if strings.Contains(string(body), "01-home.png") {
					return screenshotsUploadJSONResponse(http.StatusCreated, fmt.Sprintf(`{"data":{"type":"appScreenshots","id":"new-1","attributes":{"uploadOperations":[{"method":"PUT","url":"https://upload.example/new-1","length":%d,"offset":0}]}}}`, firstSize))
				}
				select {
				case <-firstUploadComplete:
				case <-req.Context().Done():
					return nil, req.Context().Err()
				}
				return screenshotsUploadJSONResponse(http.StatusInternalServerError, `{"errors":[{"status":"500","code":"INTERNAL_ERROR","detail":"upload create failed"}]}`)
			}

			switch {
			case strings.Contains(string(body), "02-settings.png"):
				return screenshotsUploadJSONResponse(http.StatusCreated, fmt.Sprintf(`{"data":{"type":"appScreenshots","id":"new-2","attributes":{"uploadOperations":[{"method":"PUT","url":"https://upload.example/new-2","length":%d,"offset":0}]}}}`, secondSize))
			case strings.Contains(string(body), "03-profile.png"):
				return screenshotsUploadJSONResponse(http.StatusCreated, fmt.Sprintf(`{"data":{"type":"appScreenshots","id":"new-3","attributes":{"uploadOperations":[{"method":"PUT","url":"https://upload.example/new-3","length":%d,"offset":0}]}}}`, thirdSize))
			default:
				return screenshotsUploadJSONResponse(http.StatusBadRequest, fmt.Sprintf(`{"errors":[{"status":"400","code":"UNEXPECTED","detail":"unexpected create during resume: %s"}]}`, string(body)))
			}
		case req.Method == http.MethodPut && req.URL.Host == "upload.example":
			return screenshotsUploadJSONResponse(http.StatusOK, `{}`)
		case req.Method == http.MethodPatch && strings.HasPrefix(req.URL.Path, "/v1/appScreenshots/"):
			id := strings.TrimPrefix(req.URL.Path, "/v1/appScreenshots/")
			return screenshotsUploadJSONResponse(http.StatusOK, fmt.Sprintf(`{"data":{"type":"appScreenshots","id":"%s","attributes":{"uploaded":true}}}`, id))
		case req.Method == http.MethodGet && strings.HasPrefix(req.URL.Path, "/v1/appScreenshots/"):
			id := strings.TrimPrefix(req.URL.Path, "/v1/appScreenshots/")
			if id == "new-1" {
				firstUploadCompleteOnce.Do(func() { close(firstUploadComplete) })
			}
			return screenshotsUploadJSONResponse(http.StatusOK, fmt.Sprintf(`{"data":{"type":"appScreenshots","id":"%s","attributes":{"assetDeliveryState":{"state":"COMPLETE"}}}}`, id))
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/appScreenshotSets/set-1/relationships/appScreenshots":
			relationshipPatchCount++
			body, readErr := io.ReadAll(req.Body)
			if readErr != nil {
				t.Fatalf("ReadAll() error: %v", readErr)
			}
			if !strings.Contains(string(body), `"id":"new-1"`) || !strings.Contains(string(body), `"id":"new-2"`) || !strings.Contains(string(body), `"id":"new-3"`) {
				t.Fatalf("expected relationship patch to include resumed ordering, got %s", string(body))
			}
			return screenshotsUploadJSONResponse(http.StatusNoContent, "")
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})

	var firstResult struct {
		FailureArtifactPath string `json:"failureArtifactPath"`
		Pending             int    `json:"pending"`
		Failed              int    `json:"failed"`
	}

	stdout, stderr := captureOutput(t, func() {
		code := cmd.Run([]string{
			"screenshots", "upload",
			"--version-localization", "LOC_123",
			"--path", workDir,
			"--device-type", "IPHONE_65",
			"--output", "json",
		}, "1.2.3")
		if code != cmd.ExitError {
			t.Fatalf("expected exit code %d, got %d", cmd.ExitError, code)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr for reported upload failure, got %q", stderr)
	}
	if err := json.Unmarshal([]byte(stdout), &firstResult); err != nil {
		t.Fatalf("failed to parse first stdout JSON: %v\nstdout=%s", err, stdout)
	}
	if firstResult.FailureArtifactPath == "" {
		t.Fatalf("expected failureArtifactPath in stdout, got %s", stdout)
	}
	if firstResult.Pending != 2 {
		t.Fatalf("expected pending=2 after partial failure, got %d", firstResult.Pending)
	}
	if firstResult.Failed != 1 {
		t.Fatalf("expected failed=1 after partial failure, got %d", firstResult.Failed)
	}

	artifactPath := firstResult.FailureArtifactPath
	if !filepath.IsAbs(artifactPath) {
		artifactPath = filepath.Join(workDir, artifactPath)
	}

	artifactData, err := os.ReadFile(artifactPath)
	if err != nil {
		t.Fatalf("ReadFile(%q) error: %v", artifactPath, err)
	}

	var artifact struct {
		SetID        string   `json:"setId"`
		OrderedIDs   []string `json:"orderedIds"`
		PendingFiles []string `json:"pendingFiles"`
	}
	if err := json.Unmarshal(artifactData, &artifact); err != nil {
		t.Fatalf("failed to parse artifact JSON: %v\nartifact=%s", err, string(artifactData))
	}
	if artifact.SetID != "set-1" {
		t.Fatalf("expected setId=set-1, got %q", artifact.SetID)
	}
	if len(artifact.OrderedIDs) != 1 || artifact.OrderedIDs[0] != "new-1" {
		t.Fatalf("expected orderedIds to preserve uploaded screenshot, got %#v", artifact.OrderedIDs)
	}
	if len(artifact.PendingFiles) != 2 {
		t.Fatalf("expected 2 pending files in artifact, got %#v", artifact.PendingFiles)
	}

	phase = "resume"

	var resumedResult struct {
		Resumed bool `json:"resumed"`
		Pending int  `json:"pending"`
		Failed  int  `json:"failed"`
		Results []struct {
			FileName string `json:"fileName"`
			AssetID  string `json:"assetId"`
		} `json:"results"`
	}

	stdout, stderr = captureOutput(t, func() {
		code := cmd.Run([]string{
			"screenshots", "upload",
			"--resume", artifactPath,
			"--output", "json",
		}, "1.2.3")
		if code != cmd.ExitSuccess {
			t.Fatalf("expected exit code %d, got %d", cmd.ExitSuccess, code)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr on resume success, got %q", stderr)
	}
	if err := json.Unmarshal([]byte(stdout), &resumedResult); err != nil {
		t.Fatalf("failed to parse resumed stdout JSON: %v\nstdout=%s", err, stdout)
	}
	if !resumedResult.Resumed {
		t.Fatalf("expected resumed=true, got %s", stdout)
	}
	if resumedResult.Pending != 0 {
		t.Fatalf("expected pending=0 after successful resume, got %d", resumedResult.Pending)
	}
	if resumedResult.Failed != 0 {
		t.Fatalf("expected failed=0 after successful resume, got %d", resumedResult.Failed)
	}
	if len(resumedResult.Results) != 3 {
		t.Fatalf("expected 3 total results after resume, got %#v", resumedResult.Results)
	}
	if relationshipPatchCount != 1 {
		t.Fatalf("expected exactly one relationship reorder patch on resume, got %d", relationshipPatchCount)
	}
}

func TestRunScreenshotsUploadFanoutPrintsPartialResultsOnLocaleFailure(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	workDir := t.TempDir()
	previousDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error: %v", err)
	}
	if err := os.Chdir(workDir); err != nil {
		t.Fatalf("Chdir() error: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(previousDir)
	})

	enDir := filepath.Join(workDir, "en-US")
	frDir := filepath.Join(workDir, "fr-FR")
	if err := os.MkdirAll(enDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error: %v", enDir, err)
	}
	if err := os.MkdirAll(frDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error: %v", frDir, err)
	}

	enFile := writeCmdtestScreenshotPNG(t, enDir, "01-home.png")
	frFirst := writeCmdtestScreenshotPNG(t, frDir, "01-home.png")
	frSecond := writeCmdtestScreenshotPNG(t, frDir, "02-settings.png")

	enSize := cmdtestFileSize(t, enFile)
	frFirstSize := cmdtestFileSize(t, frFirst)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	// fr-FR uploads run concurrently: the failing create blocks until the
	// fr-FR success fully completes so the outcome stays deterministic.
	frSuccessComplete := make(chan struct{})
	var frSuccessCompleteOnce sync.Once
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-1":
			return screenshotsUploadJSONResponse(http.StatusOK, `{"data":{"type":"appStoreVersions","id":"version-1","attributes":{"platform":"IOS","versionString":"1.2.3"},"relationships":{"app":{"data":{"type":"apps","id":"123456789"}}}}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
			return screenshotsUploadJSONResponse(http.StatusOK, `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-en","attributes":{"locale":"en-US"}},{"type":"appStoreVersionLocalizations","id":"loc-fr","attributes":{"locale":"fr-FR"}}],"links":{}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersionLocalizations/loc-en/appScreenshotSets":
			return screenshotsUploadJSONResponse(http.StatusOK, `{"data":[{"type":"appScreenshotSets","id":"set-en","attributes":{"screenshotDisplayType":"APP_IPHONE_65"}}],"links":{}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersionLocalizations/loc-fr/appScreenshotSets":
			return screenshotsUploadJSONResponse(http.StatusOK, `{"data":[{"type":"appScreenshotSets","id":"set-fr","attributes":{"screenshotDisplayType":"APP_IPHONE_65"}}],"links":{}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appScreenshotSets/set-en/relationships/appScreenshots":
			return screenshotsUploadJSONResponse(http.StatusOK, `{"data":[],"links":{}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appScreenshotSets/set-en/appScreenshots":
			return screenshotsUploadJSONResponse(http.StatusOK, `{"data":[],"links":{}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appScreenshotSets/set-fr/relationships/appScreenshots":
			return screenshotsUploadJSONResponse(http.StatusOK, `{"data":[],"links":{}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appScreenshotSets/set-fr/appScreenshots":
			return screenshotsUploadJSONResponse(http.StatusOK, `{"data":[],"links":{}}`)
		case req.Method == http.MethodPost && req.URL.Path == "/v1/appScreenshots":
			body, readErr := io.ReadAll(req.Body)
			if readErr != nil {
				return nil, readErr
			}
			switch {
			case strings.Contains(string(body), `"set-en"`):
				return screenshotsUploadJSONResponse(http.StatusCreated, fmt.Sprintf(`{"data":{"type":"appScreenshots","id":"new-en-1","attributes":{"uploadOperations":[{"method":"PUT","url":"https://upload.example/new-en-1","length":%d,"offset":0}]}}}`, enSize))
			case strings.Contains(string(body), "01-home.png"):
				return screenshotsUploadJSONResponse(http.StatusCreated, fmt.Sprintf(`{"data":{"type":"appScreenshots","id":"new-fr-1","attributes":{"uploadOperations":[{"method":"PUT","url":"https://upload.example/new-fr-1","length":%d,"offset":0}]}}}`, frFirstSize))
			default:
				select {
				case <-frSuccessComplete:
				case <-req.Context().Done():
					return nil, req.Context().Err()
				}
				return screenshotsUploadJSONResponse(http.StatusInternalServerError, `{"errors":[{"status":"500","code":"INTERNAL_ERROR","detail":"upload create failed"}]}`)
			}
		case req.Method == http.MethodPut && req.URL.Host == "upload.example":
			return screenshotsUploadJSONResponse(http.StatusOK, `{}`)
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/appScreenshots/new-en-1":
			return screenshotsUploadJSONResponse(http.StatusOK, `{"data":{"type":"appScreenshots","id":"new-en-1","attributes":{"uploaded":true}}}`)
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/appScreenshots/new-fr-1":
			return screenshotsUploadJSONResponse(http.StatusOK, `{"data":{"type":"appScreenshots","id":"new-fr-1","attributes":{"uploaded":true}}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appScreenshots/new-en-1":
			return screenshotsUploadJSONResponse(http.StatusOK, `{"data":{"type":"appScreenshots","id":"new-en-1","attributes":{"assetDeliveryState":{"state":"COMPLETE"}}}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appScreenshots/new-fr-1":
			frSuccessCompleteOnce.Do(func() { close(frSuccessComplete) })
			return screenshotsUploadJSONResponse(http.StatusOK, `{"data":{"type":"appScreenshots","id":"new-fr-1","attributes":{"assetDeliveryState":{"state":"COMPLETE"}}}}`)
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/appScreenshotSets/set-en/relationships/appScreenshots":
			body, readErr := io.ReadAll(req.Body)
			if readErr != nil {
				t.Fatalf("ReadAll() error: %v", readErr)
			}
			if !strings.Contains(string(body), `"id":"new-en-1"`) {
				t.Fatalf("expected set-en relationship patch to include new-en-1, got %s", string(body))
			}
			return screenshotsUploadJSONResponse(http.StatusNoContent, "")
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/appScreenshotSets/set-fr/relationships/appScreenshots":
			t.Fatalf("unexpected relationship patch for failing fr-FR upload")
			return nil, nil
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})

	var payload struct {
		Localizations []struct {
			Locale              string `json:"locale"`
			Pending             int    `json:"pending"`
			Failed              int    `json:"failed"`
			FailureArtifactPath string `json:"failureArtifactPath"`
			Results             []struct {
				FileName string `json:"fileName"`
				AssetID  string `json:"assetId"`
			} `json:"results"`
			Failures []struct {
				FilePath string `json:"filePath"`
			} `json:"failures"`
		} `json:"localizations"`
	}

	stdout, stderr := captureOutput(t, func() {
		code := cmd.Run([]string{
			"screenshots", "upload",
			"--app", "123456789",
			"--version-id", "version-1",
			"--path", workDir,
			"--device-type", "IPHONE_65",
			"--output", "json",
		}, "1.2.3")
		if code != cmd.ExitError {
			t.Fatalf("expected exit code %d, got %d", cmd.ExitError, code)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr for reported fan-out upload failure, got %q", stderr)
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("failed to parse fan-out stdout JSON: %v\nstdout=%s", err, stdout)
	}
	if len(payload.Localizations) != 2 {
		t.Fatalf("expected 2 localization results in stdout, got %#v", payload.Localizations)
	}
	if payload.Localizations[0].Locale != "en-US" || len(payload.Localizations[0].Results) != 1 {
		t.Fatalf("expected preserved en-US success result, got %#v", payload.Localizations[0])
	}
	if payload.Localizations[1].Locale != "fr-FR" {
		t.Fatalf("expected fr-FR failing localization, got %#v", payload.Localizations[1])
	}
	if payload.Localizations[1].Pending != 1 || payload.Localizations[1].Failed != 1 {
		t.Fatalf("expected pending=1 failed=1 for fr-FR failure, got %#v", payload.Localizations[1])
	}
	if payload.Localizations[1].FailureArtifactPath == "" {
		t.Fatalf("expected failure artifact path in fan-out stdout, got %s", stdout)
	}
	if len(payload.Localizations[1].Failures) != 1 || payload.Localizations[1].Failures[0].FilePath != frSecond {
		t.Fatalf("expected fr-FR failure details in stdout, got %#v", payload.Localizations[1].Failures)
	}

	artifactPath := payload.Localizations[1].FailureArtifactPath
	if !filepath.IsAbs(artifactPath) {
		artifactPath = filepath.Join(workDir, artifactPath)
	}
	artifactData, err := os.ReadFile(artifactPath)
	if err != nil {
		t.Fatalf("ReadFile(%q) error: %v", artifactPath, err)
	}
	if !strings.Contains(string(artifactData), frSecond) {
		t.Fatalf("expected fan-out artifact to mention pending file %q, got %s", frSecond, string(artifactData))
	}
}

func writeCmdtestScreenshotPNG(t *testing.T, dir, name string) string {
	t.Helper()

	path := filepath.Join(dir, name)
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("create png: %v", err)
	}
	defer file.Close()

	img := image.NewRGBA(image.Rect(0, 0, 1242, 2688))
	for y := 0; y < 2688; y++ {
		for x := 0; x < 1242; x++ {
			img.Set(x, y, color.RGBA{R: 10, G: 20, B: 30, A: 255})
		}
	}
	if err := png.Encode(file, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return path
}

func cmdtestFileSize(t *testing.T, path string) int64 {
	t.Helper()

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat(%q) error: %v", path, err)
	}
	return info.Size()
}

func screenshotsUploadJSONResponse(status int, body string) (*http.Response, error) {
	return &http.Response{
		Status:     fmt.Sprintf("%d %s", status, http.StatusText(status)),
		StatusCode: status,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}, nil
}
