package cmdtest

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUploadReplaceRequiresConfirm(t *testing.T) {
	for _, test := range []struct {
		command string
		wantErr string
	}{
		{command: "screenshots", wantErr: "--confirm is required to delete existing screenshots with --replace"},
		{command: "video-previews", wantErr: "--confirm is required to delete existing previews with --replace"},
	} {
		t.Run(test.command, func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

			installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
				t.Fatalf("expected no requests before --confirm is validated, got %s %s", req.Method, req.URL.String())
				return nil, nil
			}))

			stdout, stderr, runErr := runRootCommand(t, []string{
				test.command, "upload",
				"--version-localization", "LOC_123",
				"--path", filepath.Join(t.TempDir(), "media"),
				"--device-type", "IPHONE_65",
				"--replace",
			})

			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !errors.Is(runErr, flag.ErrHelp) {
				t.Fatalf("expected flag.ErrHelp, got %v", runErr)
			}
			if !strings.Contains(stderr, test.wantErr) {
				t.Fatalf("expected replace confirmation error, got %q", stderr)
			}
		})
	}
}

func TestScreenshotsUploadReplaceDryRunDoesNotRequireConfirm(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	pathDir := t.TempDir()
	writePNG(t, filepath.Join(pathDir, "01-home.png"), 1284, 2778)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersionLocalizations/LOC_123/appScreenshotSets":
			return statusJSONResponse(`{"data":[{"type":"appScreenshotSets","id":"set-1","attributes":{"screenshotDisplayType":"APP_IPHONE_65"}}],"links":{}}`), nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appScreenshotSets/set-1/appScreenshots":
			return statusJSONResponse(`{"data":[{"type":"appScreenshots","id":"old-1","attributes":{"fileName":"old.png"}}],"links":{}}`), nil
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})

	stdout, stderr, runErr := runRootCommand(t, []string{
		"screenshots", "upload",
		"--version-localization", "LOC_123",
		"--path", pathDir,
		"--device-type", "IPHONE_65",
		"--replace",
		"--dry-run",
		"--output", "json",
	})

	if runErr != nil {
		t.Fatalf("expected --dry-run to stay exempt from --confirm, got %v (stderr=%q)", runErr, stderr)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var payload struct {
		DryRun  bool `json:"dryRun"`
		Results []struct {
			State string `json:"state"`
		} `json:"results"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout=%s", err, stdout)
	}
	if !payload.DryRun {
		t.Fatalf("expected dryRun=true, got %s", stdout)
	}
	foundWouldDelete := false
	for _, item := range payload.Results {
		if item.State == "would-delete" {
			foundWouldDelete = true
		}
	}
	if !foundWouldDelete {
		t.Fatalf("expected a would-delete preview of the replace, got %s", stdout)
	}
}

func TestScreenshotsUploadReplaceWithConfirmDeletesExistingScreenshots(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	pathDir := t.TempDir()
	imagePath := filepath.Join(pathDir, "01-home.png")
	writePNG(t, imagePath, 1284, 2778)
	fileInfo, err := os.Stat(imagePath)
	if err != nil {
		t.Fatalf("stat screenshot: %v", err)
	}

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	deleted := make([]string, 0, 1)
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersionLocalizations/LOC_123/appScreenshotSets":
			return statusJSONResponse(`{"data":[{"type":"appScreenshotSets","id":"set-1","attributes":{"screenshotDisplayType":"APP_IPHONE_65"}}],"links":{}}`), nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appScreenshotSets/set-1/appScreenshots":
			return statusJSONResponse(`{"data":[{"type":"appScreenshots","id":"old-1","attributes":{"fileName":"old.png"}}],"links":{}}`), nil
		case req.Method == http.MethodDelete && req.URL.Path == "/v1/appScreenshots/old-1":
			deleted = append(deleted, "old-1")
			return &http.Response{
				StatusCode: http.StatusNoContent,
				Body:       io.NopCloser(strings.NewReader("")),
				Header:     http.Header{},
			}, nil
		case req.Method == http.MethodPost && req.URL.Path == "/v1/appScreenshots":
			return statusJSONResponse(fmt.Sprintf(`{"data":{"type":"appScreenshots","id":"new-1","attributes":{"uploadOperations":[{"method":"PUT","url":"https://upload.example/new-1","length":%d,"offset":0}]}}}`, fileInfo.Size())), nil
		case req.Method == http.MethodPut && req.URL.Host == "upload.example":
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader("")),
				Header:     http.Header{},
			}, nil
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/appScreenshots/new-1":
			return statusJSONResponse(`{"data":{"type":"appScreenshots","id":"new-1","attributes":{"uploaded":true}}}`), nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appScreenshots/new-1":
			return statusJSONResponse(`{"data":{"type":"appScreenshots","id":"new-1","attributes":{"sourceFileChecksum":"settled","assetDeliveryState":{"state":"COMPLETE"}}}}`), nil
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/appScreenshotSets/set-1/relationships/appScreenshots":
			return statusJSONResponse(`{}`), nil
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})

	stdout, stderr, runErr := runRootCommand(t, []string{
		"screenshots", "upload",
		"--version-localization", "LOC_123",
		"--path", pathDir,
		"--device-type", "IPHONE_65",
		"--replace",
		"--confirm",
		"--output", "json",
	})

	if runErr != nil {
		t.Fatalf("run error: %v (stderr=%q)", runErr, stderr)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if len(deleted) != 1 || deleted[0] != "old-1" {
		t.Fatalf("expected the existing screenshot to be deleted, got %v", deleted)
	}

	var payload struct {
		Uploaded int `json:"uploaded"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout=%s", err, stdout)
	}
	if payload.Uploaded != 1 {
		t.Fatalf("expected one uploaded screenshot, got %s", stdout)
	}
}
