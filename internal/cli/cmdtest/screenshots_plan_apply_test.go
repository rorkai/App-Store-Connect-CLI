package cmdtest

import (
	"context"
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

func TestScreenshotsPlanAndApplyValidationErrors(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "screenshots plan missing app",
			args:    []string{"screenshots", "plan", "--version", "1.2.3"},
			wantErr: "--app is required",
		},
		{
			name:    "screenshots plan missing version selector",
			args:    []string{"screenshots", "plan", "--app", "123456789"},
			wantErr: "--version or --version-id is required",
		},
		{
			name:    "screenshots apply missing confirm",
			args:    []string{"screenshots", "apply", "--app", "123456789", "--version", "1.2.3"},
			wantErr: "--confirm is required to apply screenshot uploads",
		},
		{
			name:    "screenshots apply positional args rejected",
			args:    []string{"screenshots", "apply", "--app", "123456789", "--version", "1.2.3", "--confirm", "extra"},
			wantErr: "screenshots apply does not accept positional arguments",
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

func TestScreenshotsPlanBuildsApprovedUploadGroups(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	reviewDir, _ := writeScreenshotReviewArtifacts(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch req.URL.Path {
		case "/v1/apps/123456789/appStoreVersions":
			return statusJSONResponse(`{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.2.3","platform":"IOS"}}]}`), nil
		case "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
			return statusJSONResponse(`{"data":[{"type":"appStoreVersionLocalizations","id":"LOC_123","attributes":{"locale":"en-US"}}]}`), nil
		case "/v1/appStoreVersionLocalizations/LOC_123/appScreenshotSets":
			return statusJSONResponse(`{"data":[],"links":{}}`), nil
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"screenshots", "plan",
			"--app", "123456789",
			"--version", "1.2.3",
			"--review-output-dir", reviewDir,
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout=%s", err, stdout)
	}

	if payload["plannedGroups"] != float64(1) {
		t.Fatalf("expected plannedGroups=1, got %v", payload["plannedGroups"])
	}
	if payload["approvedReadyEntries"] != float64(1) {
		t.Fatalf("expected approvedReadyEntries=1, got %v", payload["approvedReadyEntries"])
	}
	if payload["warningCount"] != float64(1) {
		t.Fatalf("expected warningCount=1 for missing focused coverage, got %v", payload["warningCount"])
	}

	groups, ok := payload["groups"].([]any)
	if !ok || len(groups) != 1 {
		t.Fatalf("expected one planned group, got %T %v", payload["groups"], payload["groups"])
	}
	group := groups[0].(map[string]any)
	if group["displayType"] != "APP_IPHONE_65" {
		t.Fatalf("expected displayType APP_IPHONE_65, got %v", group["displayType"])
	}
	result := group["result"].(map[string]any)
	results := result["results"].([]any)
	if results[0].(map[string]any)["state"] != "would-upload" {
		t.Fatalf("expected would-upload state, got %v", results[0].(map[string]any)["state"])
	}
}

func TestScreenshotsApplyUploadsApprovedArtifacts(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	reviewDir, imagePath := writeScreenshotReviewArtifacts(t)
	fileInfo, err := os.Stat(imagePath)
	if err != nil {
		t.Fatalf("stat review artifact: %v", err)
	}

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/123456789/appStoreVersions":
			return statusJSONResponse(`{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.2.3","platform":"IOS"}}]}`), nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
			return statusJSONResponse(`{"data":[{"type":"appStoreVersionLocalizations","id":"LOC_123","attributes":{"locale":"en-US"}}]}`), nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersionLocalizations/LOC_123/appScreenshotSets":
			return statusJSONResponse(`{"data":[{"type":"appScreenshotSets","id":"set-1","attributes":{"screenshotDisplayType":"APP_IPHONE_65"}}],"links":{}}`), nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appScreenshotSets/set-1/relationships/appScreenshots":
			return statusJSONResponse(`{"data":[],"links":{}}`), nil
		case req.Method == http.MethodPost && req.URL.Path == "/v1/appScreenshots":
			body := fmt.Sprintf(`{"data":{"type":"appScreenshots","id":"new-1","attributes":{"uploadOperations":[{"method":"PUT","url":"https://upload.example/new-1","length":%d,"offset":0}]}}}`, fileInfo.Size())
			return statusJSONResponse(body), nil
		case req.Method == http.MethodPut && req.URL.Host == "upload.example":
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader("")),
				Header:     http.Header{},
			}, nil
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/appScreenshots/new-1":
			return statusJSONResponse(`{"data":{"type":"appScreenshots","id":"new-1","attributes":{"uploaded":true}}}`), nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appScreenshots/new-1":
			return statusJSONResponse(`{"data":{"type":"appScreenshots","id":"new-1","attributes":{"assetDeliveryState":{"state":"COMPLETE"}}}}`), nil
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/appScreenshotSets/set-1/relationships/appScreenshots":
			return statusJSONResponse(`{}`), nil
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"screenshots", "apply",
			"--app", "123456789",
			"--version", "1.2.3",
			"--review-output-dir", reviewDir,
			"--confirm",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout=%s", err, stdout)
	}

	if payload["applied"] != true {
		t.Fatalf("expected applied=true, got %v", payload["applied"])
	}
	groups := payload["groups"].([]any)
	result := groups[0].(map[string]any)["result"].(map[string]any)
	results := result["results"].([]any)
	if results[0].(map[string]any)["assetId"] != "new-1" {
		t.Fatalf("expected uploaded assetId new-1, got %v", results[0].(map[string]any)["assetId"])
	}
}

func TestScreenshotsPlanRejectsVersionIDFromDifferentApp(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	reviewDir, _ := writeScreenshotReviewArtifacts(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch req.URL.Path {
		case "/v1/appStoreVersions/version-1":
			return statusJSONResponse(`{
				"data":{
					"type":"appStoreVersions",
					"id":"version-1",
					"attributes":{"versionString":"1.2.3","platform":"IOS"},
					"relationships":{"app":{"data":{"type":"apps","id":"999999999"}}}
				}
			}`), nil
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"screenshots", "plan",
			"--app", "123456789",
			"--version-id", "version-1",
			"--review-output-dir", reviewDir,
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if runErr == nil {
		t.Fatal("expected version/app mismatch error")
	}
	if errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("expected runtime validation error, got ErrHelp")
	}
	if !strings.Contains(runErr.Error(), `version "version-1" belongs to app "999999999", not "123456789"`) {
		t.Fatalf("expected mismatch error, got %v", runErr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
}

func writeScreenshotReviewArtifacts(t *testing.T) (string, string) {
	t.Helper()

	reviewDir := t.TempDir()
	imagePath := filepath.Join(reviewDir, "home.png")
	if err := os.WriteFile(imagePath, tinyPNG(), 0o600); err != nil {
		t.Fatalf("write screenshot image: %v", err)
	}

	manifest := `{
		"generated_at":"2026-03-15T00:00:00Z",
		"raw_dir":"",
		"framed_dir":"` + reviewDir + `",
		"output_dir":"` + reviewDir + `",
		"approval_path":"` + filepath.Join(reviewDir, "approved.json") + `",
		"summary":{"total":1,"ready":1,"missing_raw":0,"invalid_size":0,"approved":0,"pending_approval":1},
		"entries":[
			{
				"key":"en-US|iphone|home",
				"screenshot_id":"home",
				"locale":"en-US",
				"device":"iphone",
				"framed_path":"` + imagePath + `",
				"framed_relative_path":"home.png",
				"width":1290,
				"height":2796,
				"display_types":["APP_IPHONE_65"],
				"valid_app_store_size":true,
				"status":"ready",
				"approved":false,
				"approval_state":"pending"
			}
		]
	}`
	if err := os.WriteFile(filepath.Join(reviewDir, "manifest.json"), []byte(manifest), 0o600); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	approvals := `{"approved":["en-US|iphone|home"]}`
	if err := os.WriteFile(filepath.Join(reviewDir, "approved.json"), []byte(approvals), 0o600); err != nil {
		t.Fatalf("write approvals: %v", err)
	}

	return reviewDir, imagePath
}

func tinyPNG() []byte {
	return []byte{
		0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a,
		0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52,
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x06, 0x00, 0x00, 0x00, 0x1f, 0x15, 0xc4,
		0x89, 0x00, 0x00, 0x00, 0x0d, 0x49, 0x44, 0x41,
		0x54, 0x78, 0x9c, 0x63, 0xf8, 0xcf, 0xc0, 0x00,
		0x00, 0x03, 0x01, 0x01, 0x00, 0xc9, 0xfe, 0x92,
		0xef, 0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4e,
		0x44, 0xae, 0x42, 0x60, 0x82,
	}
}
