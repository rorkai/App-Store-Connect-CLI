package cmdtest

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"image"
	"image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
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
			name:    "screenshots plan positional args rejected",
			args:    []string{"screenshots", "plan", "--app", "123456789", "--version", "1.2.3", "extra"},
			wantErr: "screenshots plan does not accept positional arguments",
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

func TestScreenshotsPlanRejectsActualImageDimensionsThatDoNotMatchPlannedDisplayType(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	reviewDir, _ := writeScreenshotReviewArtifactsWithPlannedDisplayType(t, 1, 1, 1284, 2778, []string{"APP_IPHONE_65"})

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
			"--version", "1.2.3",
			"--review-output-dir", reviewDir,
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if runErr == nil {
		t.Fatal("expected dimension validation error")
	}
	if stdout == "" {
		t.Fatal("expected structured output describing the blocking issue")
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout=%s", err, stdout)
	}
	if payload["errorCount"] != float64(1) {
		t.Fatalf("expected errorCount=1, got %v", payload["errorCount"])
	}
	issues, ok := payload["issues"].([]any)
	if !ok || len(issues) == 0 {
		t.Fatalf("expected issues slice, got %T %v", payload["issues"], payload["issues"])
	}
	found := false
	for _, rawIssue := range issues {
		issue := rawIssue.(map[string]any)
		if issue["severity"] == "error" && strings.Contains(fmt.Sprint(issue["message"]), "unsupported size") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected unsupported size error issue, got %v", issues)
	}
}

func writeScreenshotReviewArtifacts(t *testing.T) (string, string) {
	return writeScreenshotReviewArtifactsWithSize(t, 1284, 2778)
}

func writeScreenshotReviewArtifactsWithSize(t *testing.T, width, height int) (string, string) {
	displayTypes := []string(nil)
	validAppStoreSize := screenshotMatchesDisplayType(width, height, "APP_IPHONE_65")
	if validAppStoreSize {
		displayTypes = []string{"APP_IPHONE_65"}
	}
	status := "ready"
	readyCount := 1
	invalidCount := 0
	if !validAppStoreSize {
		status = "invalid_size"
		readyCount = 0
		invalidCount = 1
	}

	return writeScreenshotReviewArtifactsFixture(
		t,
		width,
		height,
		width,
		height,
		displayTypes,
		validAppStoreSize,
		status,
		readyCount,
		invalidCount,
	)
}

func writeScreenshotReviewArtifactsWithPlannedDisplayType(t *testing.T, actualWidth, actualHeight, plannedWidth, plannedHeight int, displayTypes []string) (string, string) {
	t.Helper()

	return writeScreenshotReviewArtifactsFixture(
		t,
		actualWidth,
		actualHeight,
		plannedWidth,
		plannedHeight,
		append([]string(nil), displayTypes...),
		len(displayTypes) > 0,
		"ready",
		1,
		0,
	)
}

func writeScreenshotReviewArtifactsFixture(t *testing.T, actualWidth, actualHeight, manifestWidth, manifestHeight int, displayTypes []string, validAppStoreSize bool, status string, readyCount, invalidCount int) (string, string) {
	t.Helper()

	reviewDir := t.TempDir()
	imagePath := filepath.Join(reviewDir, "home.png")
	if err := os.WriteFile(imagePath, pngBytes(t, actualWidth, actualHeight), 0o600); err != nil {
		t.Fatalf("write screenshot image: %v", err)
	}

	manifestBytes, err := json.Marshal(map[string]any{
		"generated_at":  "2026-03-15T00:00:00Z",
		"raw_dir":       "",
		"framed_dir":    reviewDir,
		"output_dir":    reviewDir,
		"approval_path": filepath.Join(reviewDir, "approved.json"),
		"summary": map[string]any{
			"total":            1,
			"ready":            readyCount,
			"missing_raw":      0,
			"invalid_size":     invalidCount,
			"approved":         0,
			"pending_approval": 1,
		},
		"entries": []map[string]any{
			{
				"key":                  "en-US|iphone|home",
				"screenshot_id":        "home",
				"locale":               "en-US",
				"device":               "iphone",
				"framed_path":          imagePath,
				"framed_relative_path": "home.png",
				"width":                manifestWidth,
				"height":               manifestHeight,
				"display_types":        displayTypes,
				"valid_app_store_size": validAppStoreSize,
				"status":               status,
				"approved":             false,
				"approval_state":       "pending",
			},
		},
	})
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	if err := os.WriteFile(filepath.Join(reviewDir, "manifest.json"), manifestBytes, 0o600); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	approvals := `{"approved":["en-US|iphone|home"]}`
	if err := os.WriteFile(filepath.Join(reviewDir, "approved.json"), []byte(approvals), 0o600); err != nil {
		t.Fatalf("write approvals: %v", err)
	}

	return reviewDir, imagePath
}

func screenshotMatchesDisplayType(width, height int, displayType string) bool {
	dimensions, ok := asc.ScreenshotDimensions(displayType)
	if !ok {
		return false
	}
	for _, dimension := range dimensions {
		if dimension.Width == width && dimension.Height == height {
			return true
		}
	}
	return false
}

func pngBytes(t *testing.T, width, height int) []byte {
	t.Helper()

	img := image.NewRGBA(image.Rect(0, 0, width, height))
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return buf.Bytes()
}
