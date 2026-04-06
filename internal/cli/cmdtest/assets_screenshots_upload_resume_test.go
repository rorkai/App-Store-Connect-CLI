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
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
)

func TestRunScreenshotsUploadResumeRejectsSelectorFlags(t *testing.T) {
	_, stderr := captureOutput(t, func() {
		code := cmd.Run([]string{
			"screenshots", "upload",
			"--resume", "artifact.json",
			"--path", "./screenshots",
		}, "1.2.3")
		if code != cmd.ExitUsage {
			t.Fatalf("expected exit code %d, got %d", cmd.ExitUsage, code)
		}
	})

	if !strings.Contains(stderr, "--resume cannot be combined with --version-localization, --path, or --device-type") {
		t.Fatalf("expected resume conflict message, got %q", stderr)
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

	first := writeCmdtestScreenshotPNG(t, workDir, "01-home.png", 1242, 2688)
	second := writeCmdtestScreenshotPNG(t, workDir, "02-settings.png", 1242, 2688)
	third := writeCmdtestScreenshotPNG(t, workDir, "03-profile.png", 1242, 2688)

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

func writeCmdtestScreenshotPNG(t *testing.T, dir, name string, width, height int) string {
	t.Helper()

	path := filepath.Join(dir, name)
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("create png: %v", err)
	}
	defer file.Close()

	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
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
