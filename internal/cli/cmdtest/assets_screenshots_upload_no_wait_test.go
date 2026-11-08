package cmdtest

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
)

// --no-wait must check each committed screenshot exactly once instead of
// polling until processing finishes. Unconfirmed commits and failed deliveries
// must be reported as errors with a resumable artifact.
func TestRunScreenshotsUploadNoWaitChecksEachFileOnce(t *testing.T) {
	tests := []struct {
		name             string
		state            string
		wantCode         int
		wantState        string
		wantStderr       string
		wantPendingState string
	}{
		{name: "committed", state: "UPLOAD_COMPLETE", wantCode: cmd.ExitSuccess, wantState: "UPLOAD_COMPLETE"},
		{name: "processed", state: "COMPLETE", wantCode: cmd.ExitSuccess, wantState: "COMPLETE"},
		{name: "awaiting", state: "AWAITING_UPLOAD", wantCode: cmd.ExitError, wantStderr: `upload commit not confirmed: delivery state "AWAITING_UPLOAD"`, wantPendingState: "UPLOAD_COMPLETE"},
		{name: "missing state", wantCode: cmd.ExitError, wantStderr: "upload commit not confirmed: missing delivery state", wantPendingState: "UPLOAD_COMPLETE"},
		{name: "failed", state: "FAILED", wantCode: cmd.ExitError, wantStderr: "delivery failed: IMAGE_INCORRECT_DIMENSIONS", wantPendingState: "FAILED"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
			t.Setenv("ASC_APP_ID", "")

			workDir := t.TempDir()
			file := writeCmdtestScreenshotPNG(t, workDir, "01-home.png")
			size := cmdtestFileSize(t, file)

			originalTransport := http.DefaultTransport
			t.Cleanup(func() { http.DefaultTransport = originalTransport })

			var mu sync.Mutex
			stateChecks := 0
			http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
				switch {
				case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersionLocalizations/LOC_123/appScreenshotSets":
					return screenshotsUploadJSONResponse(http.StatusOK, `{"data":[{"type":"appScreenshotSets","id":"set-1","attributes":{"screenshotDisplayType":"APP_IPHONE_65"}}],"links":{}}`)
				case req.Method == http.MethodGet && (req.URL.Path == "/v1/appScreenshotSets/set-1/appScreenshots" ||
					req.URL.Path == "/v1/appScreenshotSets/set-1/relationships/appScreenshots"):
					return screenshotsUploadJSONResponse(http.StatusOK, `{"data":[],"links":{}}`)
				case req.Method == http.MethodPost && req.URL.Path == "/v1/appScreenshots":
					return screenshotsUploadJSONResponse(http.StatusCreated, fmt.Sprintf(`{"data":{"type":"appScreenshots","id":"new-1","attributes":{"uploadOperations":[{"method":"PUT","url":"https://upload.example/new-1","length":%d,"offset":0}]}}}`, size))
				case req.Method == http.MethodPut && req.URL.Host == "upload.example":
					return screenshotsUploadJSONResponse(http.StatusOK, `{}`)
				case req.Method == http.MethodPatch && req.URL.Path == "/v1/appScreenshots/new-1":
					return screenshotsUploadJSONResponse(http.StatusOK, `{"data":{"type":"appScreenshots","id":"new-1","attributes":{"uploaded":true,"assetDeliveryState":{"state":"UPLOAD_COMPLETE"}}}}`)
				case req.Method == http.MethodGet && req.URL.Path == "/v1/appScreenshots/new-1":
					mu.Lock()
					stateChecks++
					check := stateChecks
					mu.Unlock()
					if check > 1 {
						return screenshotsUploadJSONResponse(http.StatusBadRequest, `{"errors":[{"detail":"unexpected extra state check"}]}`)
					}
					if test.state == "" {
						return screenshotsUploadJSONResponse(http.StatusOK, `{"data":{"type":"appScreenshots","id":"new-1","attributes":{}}}`)
					}
					return screenshotsUploadJSONResponse(http.StatusOK, fmt.Sprintf(`{"data":{"type":"appScreenshots","id":"new-1","attributes":{"assetDeliveryState":{"state":%q,"errors":[{"code":"IMAGE_INCORRECT_DIMENSIONS","description":"bad size"}]}}}}`, test.state))
				case req.Method == http.MethodPatch && req.URL.Path == "/v1/appScreenshotSets/set-1/relationships/appScreenshots":
					return screenshotsUploadJSONResponse(http.StatusNoContent, "")
				default:
					t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
					return nil, nil
				}
			})

			previousDir, err := os.Getwd()
			if err != nil {
				t.Fatalf("Getwd() error: %v", err)
			}
			if err := os.Chdir(workDir); err != nil {
				t.Fatalf("Chdir() error: %v", err)
			}
			t.Cleanup(func() { _ = os.Chdir(previousDir) })

			var code int
			stdout, stderr := captureOutput(t, func() {
				code = cmd.Run([]string{
					"screenshots", "upload",
					"--version-localization", "LOC_123",
					"--path", file,
					"--device-type", "IPHONE_65",
					"--no-wait",
					"--output", "json",
				}, "1.2.3")
			})
			if code != test.wantCode {
				t.Fatalf("exit code = %d, want %d; stderr = %q", code, test.wantCode, stderr)
			}

			if stateChecks != 1 {
				t.Fatalf("state checks = %d, want exactly 1", stateChecks)
			}
			var result struct {
				FailureArtifactPath string `json:"failureArtifactPath"`
				Results             []struct {
					AssetID string `json:"assetId"`
					State   string `json:"state"`
				} `json:"results"`
			}
			if err := json.Unmarshal([]byte(stdout), &result); err != nil {
				t.Fatalf("parse stdout: %v\nstdout=%s", err, stdout)
			}
			if test.wantStderr != "" {
				if !strings.Contains(stderr, "screenshot new-1 "+test.wantStderr) {
					t.Fatalf("stderr = %q, want %q", stderr, test.wantStderr)
				}
				if len(result.Results) != 0 || result.FailureArtifactPath == "" {
					t.Fatalf("expected only a failure receipt, got %s", stdout)
				}
				data, err := os.ReadFile(result.FailureArtifactPath)
				if err != nil {
					t.Fatal(err)
				}
				var artifact struct {
					PendingAssets []struct {
						AssetID string `json:"assetId"`
						State   string `json:"state"`
					} `json:"pendingAssets"`
				}
				if err := json.Unmarshal(data, &artifact); err != nil {
					t.Fatal(err)
				}
				if len(artifact.PendingAssets) != 1 || artifact.PendingAssets[0].AssetID != "new-1" || artifact.PendingAssets[0].State != test.wantPendingState {
					t.Fatalf("pending assets = %+v, want new-1 in %s", artifact.PendingAssets, test.wantPendingState)
				}
				return
			}
			if len(result.Results) != 1 || result.Results[0].AssetID != "new-1" || result.Results[0].State != test.wantState {
				t.Fatalf("results = %+v, want new-1 in %s", result.Results, test.wantState)
			}
		})
	}
}
