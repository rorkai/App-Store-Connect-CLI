package cmdtest

import (
	"encoding/json"
	"fmt"
	"hash/crc32"
	"image"
	"image/color"
	"image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
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
			name: "confirm",
			args: []string{"--confirm"},
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

			if !strings.Contains(stderr, "--resume cannot be combined with --skip-existing, --replace, --confirm, --dry-run, or --max-screenshots") {
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
	firstRunCreates := 0
	resumeCreates := 0
	relationshipPatchCount := 0

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
			if phase == "first" {
				firstRunCreates++
				if firstRunCreates == 1 {
					return screenshotsUploadJSONResponse(http.StatusCreated, fmt.Sprintf(`{"data":{"type":"appScreenshots","id":"new-1","attributes":{"uploadOperations":[{"method":"PUT","url":"https://upload.example/new-1","length":%d,"offset":0}]}}}`, firstSize))
				}
				return screenshotsUploadJSONResponse(http.StatusInternalServerError, `{"errors":[{"status":"500","code":"INTERNAL_ERROR","detail":"upload create failed"}]}`)
			}

			resumeCreates++
			switch resumeCreates {
			case 1:
				return screenshotsUploadJSONResponse(http.StatusCreated, fmt.Sprintf(`{"data":{"type":"appScreenshots","id":"new-2","attributes":{"uploadOperations":[{"method":"PUT","url":"https://upload.example/new-2","length":%d,"offset":0}]}}}`, secondSize))
			case 2:
				return screenshotsUploadJSONResponse(http.StatusCreated, fmt.Sprintf(`{"data":{"type":"appScreenshots","id":"new-3","attributes":{"uploadOperations":[{"method":"PUT","url":"https://upload.example/new-3","length":%d,"offset":0}]}}}`, thirdSize))
			default:
				t.Fatalf("unexpected extra create during resume: %d", resumeCreates)
				return nil, nil
			}
		case req.Method == http.MethodPut && req.URL.Host == "upload.example":
			return screenshotsUploadJSONResponse(http.StatusOK, `{}`)
		case req.Method == http.MethodPatch && strings.HasPrefix(req.URL.Path, "/v1/appScreenshots/"):
			id := strings.TrimPrefix(req.URL.Path, "/v1/appScreenshots/")
			return screenshotsUploadJSONResponse(http.StatusOK, fmt.Sprintf(`{"data":{"type":"appScreenshots","id":"%s","attributes":{"uploaded":true}}}`, id))
		case req.Method == http.MethodGet && strings.HasPrefix(req.URL.Path, "/v1/appScreenshots/"):
			id := strings.TrimPrefix(req.URL.Path, "/v1/appScreenshots/")
			return screenshotsUploadJSONResponse(http.StatusOK, fmt.Sprintf(`{"data":{"type":"appScreenshots","id":"%s","attributes":{"sourceFileChecksum":"settled","assetDeliveryState":{"state":"COMPLETE"}}}}`, id))
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
			"--concurrency", "1",
			"--output", "json",
		}, "1.2.3")
		if code != cmd.ExitHTTPInternalServer {
			t.Fatalf("expected exit code %d, got %d", cmd.ExitHTTPInternalServer, code)
		}
	})

	if err := json.Unmarshal([]byte(stdout), &firstResult); err != nil {
		t.Fatalf("failed to parse first stdout JSON: %v\nstdout=%s", err, stdout)
	}
	if firstResult.FailureArtifactPath == "" {
		t.Fatalf("expected failureArtifactPath in stdout, got %s", stdout)
	}
	if !strings.Contains(stderr, "Error: screenshots upload: 2 of 3 file(s) not uploaded: ") || !strings.Contains(stderr, "upload create failed") {
		t.Fatalf("expected partial failure and its cause on stderr, got %q", stderr)
	}
	quotedPath, ok := shared.ShellQuote(firstResult.FailureArtifactPath)
	if !ok {
		t.Fatalf("generated artifact path cannot be quoted: %q", firstResult.FailureArtifactPath)
	}
	if !strings.Contains(stderr, fmt.Sprintf("Hint: resume with `asc screenshots upload --resume %s`", quotedPath)) {
		t.Fatalf("expected resume hint on stderr, got %q", stderr)
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

func TestRunScreenshotsUploadWarnsWhenFileNamesAlreadyInSet(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	workDir := t.TempDir()
	writeCmdtestScreenshotPNG(t, workDir, "01-home.png")
	writeCmdtestScreenshotPNG(t, workDir, "02-settings.png")

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersionLocalizations/LOC_123/appScreenshotSets":
			return screenshotsUploadJSONResponse(http.StatusOK, `{"data":[{"type":"appScreenshotSets","id":"set-1","attributes":{"screenshotDisplayType":"APP_IPHONE_65"}}],"links":{}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appScreenshotSets/set-1/appScreenshots":
			return screenshotsUploadJSONResponse(http.StatusOK, `{"data":[{"type":"appScreenshots","id":"old-1","attributes":{"fileName":"01-home.png"}}],"links":{}}`)
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})

	_, stderr := captureOutput(t, func() {
		code := cmd.Run([]string{
			"screenshots", "upload",
			"--version-localization", "LOC_123",
			"--path", workDir,
			"--device-type", "IPHONE_65",
			"--dry-run",
			"--output", "json",
		}, "1.2.3")
		if code != cmd.ExitSuccess {
			t.Fatalf("expected exit code %d, got %d", cmd.ExitSuccess, code)
		}
	})

	want := "Warning: screenshots upload: 1 file(s) already in the target screenshot set (01-home.png); uploading them again creates duplicates. Use --skip-existing to upload only missing files, --replace --confirm to replace the set, or --resume with the failure artifact to finish a failed upload.\n"
	if stderr != want {
		t.Fatalf("stderr = %q, want %q", stderr, want)
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
			var payload struct {
				Data struct {
					Type       string `json:"type"`
					Attributes struct {
						FileName string `json:"fileName"`
						FileSize int64  `json:"fileSize"`
					} `json:"attributes"`
					Relationships struct {
						AppScreenshotSet struct {
							Data struct {
								ID string `json:"id"`
							} `json:"data"`
						} `json:"appScreenshotSet"`
					} `json:"relationships"`
				} `json:"data"`
			}
			if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
				t.Fatalf("decode screenshot create request: %v", err)
			}
			if payload.Data.Type != "appScreenshots" {
				t.Fatalf("expected screenshot create type appScreenshots, got %q", payload.Data.Type)
			}
			switch {
			case payload.Data.Relationships.AppScreenshotSet.Data.ID == "set-en" && payload.Data.Attributes.FileName == "01-home.png" && payload.Data.Attributes.FileSize == int64(enSize):
				return screenshotsUploadJSONResponse(http.StatusCreated, fmt.Sprintf(`{"data":{"type":"appScreenshots","id":"new-en-1","attributes":{"uploadOperations":[{"method":"PUT","url":"https://upload.example/new-en-1","length":%d,"offset":0}]}}}`, enSize))
			case payload.Data.Relationships.AppScreenshotSet.Data.ID == "set-fr" && payload.Data.Attributes.FileName == "01-home.png" && payload.Data.Attributes.FileSize == int64(frFirstSize):
				return screenshotsUploadJSONResponse(http.StatusCreated, fmt.Sprintf(`{"data":{"type":"appScreenshots","id":"new-fr-1","attributes":{"uploadOperations":[{"method":"PUT","url":"https://upload.example/new-fr-1","length":%d,"offset":0}]}}}`, frFirstSize))
			case payload.Data.Relationships.AppScreenshotSet.Data.ID == "set-fr" && payload.Data.Attributes.FileName == "02-settings.png" && payload.Data.Attributes.FileSize > 0:
				return screenshotsUploadJSONResponse(http.StatusInternalServerError, `{"errors":[{"status":"500","code":"INTERNAL_ERROR","detail":"upload create failed"}]}`)
			default:
				t.Fatalf("unexpected screenshot create request: %#v", payload.Data)
				return nil, nil
			}
		case req.Method == http.MethodPut && req.URL.Host == "upload.example":
			return screenshotsUploadJSONResponse(http.StatusOK, `{}`)
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/appScreenshots/new-en-1":
			return screenshotsUploadJSONResponse(http.StatusOK, `{"data":{"type":"appScreenshots","id":"new-en-1","attributes":{"uploaded":true}}}`)
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/appScreenshots/new-fr-1":
			return screenshotsUploadJSONResponse(http.StatusOK, `{"data":{"type":"appScreenshots","id":"new-fr-1","attributes":{"uploaded":true}}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appScreenshots/new-en-1":
			return screenshotsUploadJSONResponse(http.StatusOK, `{"data":{"type":"appScreenshots","id":"new-en-1","attributes":{"sourceFileChecksum":"settled","assetDeliveryState":{"state":"COMPLETE"}}}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appScreenshots/new-fr-1":
			return screenshotsUploadJSONResponse(http.StatusOK, `{"data":{"type":"appScreenshots","id":"new-fr-1","attributes":{"sourceFileChecksum":"settled","assetDeliveryState":{"state":"COMPLETE"}}}}`)
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
			"--concurrency", "1",
			"--output", "json",
		}, "1.2.3")
		if code != cmd.ExitHTTPInternalServer {
			t.Fatalf("expected exit code %d, got %d", cmd.ExitHTTPInternalServer, code)
		}
	})

	if !strings.Contains(stderr, "Error: screenshots upload: 1 of 2 file(s) not uploaded: ") || !strings.Contains(stderr, "Hint: resume with `asc screenshots upload --resume ") {
		t.Fatalf("expected fan-out partial failure and resume hint on stderr, got %q", stderr)
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
	// Distinct names get distinct pixels: uploads reject duplicate screenshots.
	sum := crc32.ChecksumIEEE([]byte(name))
	img.Set(0, 0, color.RGBA{R: byte(sum), G: byte(sum >> 8), B: byte(sum >> 16), A: 255})
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

func TestRunScreenshotsResumeKeepsLiteralFailureArtifactPath(t *testing.T) {
	for _, name := range []string{"retry$NAME$(printf expanded)`printf backtick`'quote.json", "retry\x1b[2J\nforged.json"} {
		t.Run(name, func(t *testing.T) {
			if runtime.GOOS == "windows" && strings.ContainsAny(name, "\x1b\n") {
				t.Skip("Windows does not permit control characters in filenames")
			}
			setupAuth(t)
			t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
			t.Setenv("ASC_APP_ID", "")
			path := filepath.Join(t.TempDir(), name)
			artifact := `{"versionLocalizationId":"LOC_123","setId":"SET_123","cleanupFailures":[{"assetId":"SHOT_BAD"}]}`
			if err := os.WriteFile(path, []byte(artifact), 0o600); err != nil {
				t.Fatal(err)
			}
			originalTransport := http.DefaultTransport
			t.Cleanup(func() { http.DefaultTransport = originalTransport })
			calls := 0
			http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.Method != http.MethodDelete || req.URL.Path != "/v1/appScreenshots/SHOT_BAD" {
					t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
				}
				calls++
				return screenshotsUploadJSONResponse(http.StatusBadRequest, `{"errors":[{"status":"400","code":"BAD_REQUEST","detail":"cleanup failed"}]}`)
			})
			stdout, stderr := captureOutput(t, func() {
				if code := cmd.Run([]string{"screenshots", "upload", "--resume", path, "--output", "json"}, "1.2.3"); code != cmd.ExitHTTPBadRequest {
					t.Fatalf("expected HTTP 400 exit %d, got %d", cmd.ExitHTTPBadRequest, code)
				}
			})
			var result struct {
				FailureArtifactPath string `json:"failureArtifactPath"`
				Resumed             bool   `json:"resumed"`
			}
			if err := json.Unmarshal([]byte(stdout), &result); err != nil {
				t.Fatalf("invalid receipt: %v, %q", err, stdout)
			}
			if result.FailureArtifactPath != path || !result.Resumed || calls != 1 {
				t.Fatalf("resume outcome changed: %+v, calls=%d", result, calls)
			}
			if !strings.Contains(stderr, "cleanup failed") || strings.Count(stderr, "Error:") != 1 {
				t.Fatalf("failure cause not preserved: %q", stderr)
			}
			if quoted, ok := shared.ShellQuote(path); ok {
				if !strings.Contains(stderr, "Hint: resume with `asc screenshots upload --resume "+quoted+"`\n") {
					t.Fatalf("literal artifact path lost: %q", stderr)
				}
			} else if strings.Contains(stderr, "`asc screenshots upload --resume ") || !strings.Contains(stderr, "failure artifact path from the JSON result") {
				t.Fatalf("unsafe filename must not be approximated: %q", stderr)
			}
		})
	}
}
