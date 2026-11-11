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
	"time"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/handlertest"
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

func TestScreenshotsPlanReportsStdoutWriteFailures(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	reviewDir, _ := writeScreenshotReviewArtifacts(t)

	for _, format := range []string{"table", "markdown", "json"} {
		t.Run(format, func(t *testing.T) {
			requests := 0
			installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.Method != http.MethodGet {
					t.Fatalf("unexpected mutation: %s %s", req.Method, req.URL)
				}
				requests++
				switch req.URL.Path {
				case "/v1/apps/123456789/appStoreVersions":
					return statusJSONResponse(`{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.2.3","platform":"IOS"}}]}`), nil
				case "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
					return statusJSONResponse(`{"data":[{"type":"appStoreVersionLocalizations","id":"LOC_123","attributes":{"locale":"en-US"}}]}`), nil
				case "/v1/appStoreVersionLocalizations/LOC_123/appScreenshotSets":
					return statusJSONResponse(`{"data":[],"links":{}}`), nil
				default:
					t.Fatalf("unexpected request: %s %s", req.Method, req.URL)
					return nil, nil
				}
			}))

			readOnly, err := os.Open(filepath.Join(reviewDir, "manifest.json"))
			if err != nil {
				t.Fatalf("open read-only stdout: %v", err)
			}
			t.Cleanup(func() { _ = readOnly.Close() })

			var code int
			_, stderr := captureOutput(t, func() {
				originalStdout := os.Stdout
				os.Stdout = readOnly
				defer func() { os.Stdout = originalStdout }()
				code = rootcmd.Run([]string{
					"screenshots", "plan",
					"--app", "123456789",
					"--version", "1.2.3",
					"--review-output-dir", reviewDir,
					"--output", format,
				}, "1.2.3")
			})

			if requests != 3 {
				t.Fatalf("requests = %d, want completed plan preflight (3 GETs); stderr=%q", requests, stderr)
			}
			if code != rootcmd.ExitError {
				t.Fatalf("exit code = %d, want %d for stdout write failure; stderr=%q", code, rootcmd.ExitError, stderr)
			}
			if want := "write " + readOnly.Name() + ":"; !strings.Contains(stderr, want) {
				t.Fatalf("stderr = %q, want stdout write error containing %q", stderr, want)
			}
		})
	}
}

func TestScreenshotsPlanVersionIDUsesResolvedPlatformWithoutExplicitPlatform(t *testing.T) {
	for _, selector := range []string{"--version-id", "--version"} {
		t.Run(selector, func(t *testing.T) { testScreenshotsPlanVersionIDPlatform(t, selector) })
	}
}

func testScreenshotsPlanVersionIDPlatform(t *testing.T, selector string) {
	t.Helper()
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	reviewDir, _ := writeScreenshotReviewArtifactsWithPlannedDisplayType(t, 2880, 1800, 2880, 1800, []string{"APP_DESKTOP"})

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch req.URL.Path {
		case "/v1/appStoreVersions/version-mac":
			if req.Method != http.MethodGet || req.URL.Query().Get("include") != "app" {
				t.Fatalf("expected owned version lookup, got %s %s", req.Method, req.URL)
			}
			return statusJSONResponse(`{"data":{"type":"appStoreVersions","id":"version-mac","attributes":{"versionString":"2.0.0","platform":"MAC_OS"},"relationships":{"app":{"data":{"type":"apps","id":"123456789"}}}}}`), nil
		case "/v1/appStoreVersions/version-mac/appStoreVersionLocalizations":
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
			selector, "version-mac",
			"--review-output-dir", reviewDir,
			"--output", "json",
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
	if payload["versionId"] != "version-mac" {
		t.Fatalf("expected versionId version-mac, got %v", payload["versionId"])
	}
	if payload["version"] != "2.0.0" {
		t.Fatalf("expected version 2.0.0, got %v", payload["version"])
	}
	if payload["platform"] != "MAC_OS" {
		t.Fatalf("expected platform MAC_OS, got %v", payload["platform"])
	}

	groups, ok := payload["groups"].([]any)
	if !ok || len(groups) != 1 {
		t.Fatalf("expected one planned group, got %T %v", payload["groups"], payload["groups"])
	}
	group := groups[0].(map[string]any)
	if group["displayType"] != "APP_DESKTOP" {
		t.Fatalf("expected displayType APP_DESKTOP, got %v", group["displayType"])
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
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appScreenshotSets/set-1/appScreenshots":
			return statusJSONResponse(`{"data":[],"links":{}}`), nil
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
			return statusJSONResponse(`{"data":{"type":"appScreenshots","id":"new-1","attributes":{"sourceFileChecksum":"settled","assetDeliveryState":{"state":"COMPLETE"}}}}`), nil
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

func TestScreenshotsApplyGivesAssetUploadsTheUploadTimeoutBudget(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_TIMEOUT", "30s")
	t.Setenv("ASC_TIMEOUT_SECONDS", "")
	t.Setenv("ASC_UPLOAD_TIMEOUT", "20m")
	t.Setenv("ASC_UPLOAD_TIMEOUT_SECONDS", "")

	reviewDir, imagePath := writeScreenshotReviewArtifacts(t)
	fileInfo, err := os.Stat(imagePath)
	if err != nil {
		t.Fatalf("stat review artifact: %v", err)
	}

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	var uploadBudget time.Duration
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/123456789/appStoreVersions":
			return statusJSONResponse(`{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.2.3","platform":"IOS"}}]}`), nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
			return statusJSONResponse(`{"data":[{"type":"appStoreVersionLocalizations","id":"LOC_123","attributes":{"locale":"en-US"}}]}`), nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersionLocalizations/LOC_123/appScreenshotSets":
			return statusJSONResponse(`{"data":[{"type":"appScreenshotSets","id":"set-1","attributes":{"screenshotDisplayType":"APP_IPHONE_65"}}],"links":{}}`), nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appScreenshotSets/set-1/appScreenshots":
			return statusJSONResponse(`{"data":[],"links":{}}`), nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appScreenshotSets/set-1/relationships/appScreenshots":
			return statusJSONResponse(`{"data":[],"links":{}}`), nil
		case req.Method == http.MethodPost && req.URL.Path == "/v1/appScreenshots":
			body := fmt.Sprintf(`{"data":{"type":"appScreenshots","id":"new-1","attributes":{"uploadOperations":[{"method":"PUT","url":"https://upload.example/new-1","length":%d,"offset":0}]}}}`, fileInfo.Size())
			return statusJSONResponse(body), nil
		case req.Method == http.MethodPut && req.URL.Host == "upload.example":
			if deadline, ok := req.Context().Deadline(); ok {
				uploadBudget = time.Until(deadline)
			}
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

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	_, stderr := captureOutput(t, func() {
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
	if uploadBudget <= 30*time.Second {
		t.Fatalf("expected screenshot upload budget to use the 20m upload timeout, got %s", uploadBudget)
	}
}

func TestScreenshotsApplyReportsCompletedUploadsWhenALaterUploadFails(t *testing.T) {
	tests := []struct {
		name              string
		sameGroup         bool
		failedDisplayType string
	}{
		{name: "later group", failedDisplayType: "APP_IPHONE_65"},
		{name: "same group", sameGroup: true, failedDisplayType: "APP_IPHONE_61"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
			t.Setenv("ASC_APP_ID", "")

			reviewDir, imagePaths := writeTwoSlotScreenshotReviewArtifacts(t, test.sameGroup)
			firstInfo, err := os.Stat(imagePaths[0])
			if err != nil {
				t.Fatalf("stat review artifact: %v", err)
			}

			firstCompleted := make(chan struct{}, 1)
			fixture := handlertest.New(t)
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
					return statusJSONResponse(`{"data":[{"type":"appScreenshotSets","id":"set-61","attributes":{"screenshotDisplayType":"APP_IPHONE_61"}},{"type":"appScreenshotSets","id":"set-65","attributes":{"screenshotDisplayType":"APP_IPHONE_65"}}],"links":{}}`), nil
				case req.Method == http.MethodGet && strings.HasPrefix(req.URL.Path, "/v1/appScreenshotSets/"):
					return statusJSONResponse(`{"data":[],"links":{}}`), nil
				case req.Method == http.MethodPost && req.URL.Path == "/v1/appScreenshots":
					body, readErr := io.ReadAll(req.Body)
					if readErr != nil {
						return nil, fixture.Errorf("read screenshot create request: %v", readErr)
					}
					if strings.Contains(string(body), filepath.Base(imagePaths[1])) {
						return jsonHTTPResponse(http.StatusInternalServerError, `{"errors":[{"status":"500","detail":"screenshot reservation failed"}]}`), nil
					}
					return statusJSONResponse(fmt.Sprintf(`{"data":{"type":"appScreenshots","id":"new-61","attributes":{"uploadOperations":[{"method":"PUT","url":"https://upload.example/new-61","length":%d,"offset":0}]}}}`, firstInfo.Size())), nil
				case req.Method == http.MethodPut && req.URL.Host == "upload.example":
					return &http.Response{
						StatusCode: http.StatusOK,
						Body:       io.NopCloser(strings.NewReader("")),
						Header:     http.Header{},
					}, nil
				case req.Method == http.MethodPatch && req.URL.Path == "/v1/appScreenshots/new-61":
					return statusJSONResponse(`{"data":{"type":"appScreenshots","id":"new-61","attributes":{"uploaded":true}}}`), nil
				case req.Method == http.MethodGet && req.URL.Path == "/v1/appScreenshots/new-61":
					select {
					case firstCompleted <- struct{}{}:
					default:
					}
					return statusJSONResponse(`{"data":{"type":"appScreenshots","id":"new-61","attributes":{"sourceFileChecksum":"settled","assetDeliveryState":{"state":"COMPLETE"}}}}`), nil
				case req.Method == http.MethodPatch && strings.HasSuffix(req.URL.Path, "/relationships/appScreenshots"):
					return statusJSONResponse(`{}`), nil
				default:
					return nil, fixture.Errorf("unexpected request: %s %s", req.Method, req.URL.String())
				}
			})

			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			var runErr error
			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse([]string{
					"screenshots", "apply",
					"--app", "123456789",
					"--version", "1.2.3",
					"--review-output-dir", reviewDir,
					"--confirm",
					"--output", "json",
				}); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				runErr = root.Run(context.Background())
			})

			select {
			case <-firstCompleted:
			default:
				t.Fatal("expected the first screenshot to complete upload before the command returned")
			}
			if code := rootcmd.ExitCodeFromError(runErr); code != rootcmd.ExitHTTPInternalServer {
				t.Fatalf("exit code = %d, want %d for the failed screenshot: %v", code, rootcmd.ExitHTTPInternalServer, runErr)
			}
			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}
			if stdout == "" {
				t.Fatal("expected completed upload groups to be reported before the error")
			}

			var payload map[string]any
			if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
				t.Fatalf("unmarshal output: %v\nstdout=%s", err, stdout)
			}
			if payload["errorCount"] != float64(1) {
				t.Errorf("errorCount=%v, want 1", payload["errorCount"])
			}
			issues, ok := payload["issues"].([]any)
			foundFailure := false
			for _, raw := range issues {
				issue := raw.(map[string]any)
				if issue["severity"] == "error" && issue["displayType"] == test.failedDisplayType && strings.Contains(fmt.Sprint(issue["message"]), "screenshot reservation failed") {
					foundFailure = true
				}
			}
			if !ok || !foundFailure {
				t.Errorf("missing failed group issue: %v", payload["issues"])
			}
			groups, ok := payload["groups"].([]any)
			if !ok || len(groups) != 1 {
				t.Fatalf("expected the group containing the completed screenshot to be reported, got %T %v", payload["groups"], payload["groups"])
			}
			group := groups[0].(map[string]any)
			if group["displayType"] != "APP_IPHONE_61" {
				t.Fatalf("expected completed group APP_IPHONE_61, got %v", group["displayType"])
			}
			results, ok := group["result"].(map[string]any)["results"].([]any)
			if !ok || len(results) != 1 {
				t.Fatalf("expected only the successful screenshot receipt, got %v", group["result"])
			}
			item := results[0].(map[string]any)
			if item["assetId"] != "new-61" || item["filePath"] != imagePaths[0] || item["state"] != "COMPLETE" {
				t.Fatalf("expected completed screenshot receipt with assetId new-61 and path %q, got %v", imagePaths[0], item)
			}
		})
	}
}

func TestScreenshotsPlanReportsCapacityFailure(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")
	reviewDir, _ := writeScreenshotReviewArtifacts(t)
	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet {
			t.Fatalf("plan mutated assets: %s %s", req.Method, req.URL)
		}
		switch req.URL.Path {
		case "/v1/apps/123456789/appStoreVersions":
			return statusJSONResponse(`{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.2.3","platform":"IOS"}}]}`), nil
		case "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
			return statusJSONResponse(`{"data":[{"type":"appStoreVersionLocalizations","id":"LOC_123","attributes":{"locale":"en-US"}}]}`), nil
		case "/v1/appStoreVersionLocalizations/LOC_123/appScreenshotSets":
			return statusJSONResponse(`{"data":[{"type":"appScreenshotSets","id":"set-1","attributes":{"screenshotDisplayType":"APP_IPHONE_65"}}],"links":{}}`), nil
		case "/v1/appScreenshotSets/set-1/appScreenshots":
			screenshots := make([]map[string]string, 10)
			for i := range screenshots {
				screenshots[i] = map[string]string{"type": "appScreenshots", "id": fmt.Sprintf("existing-%d", i)}
			}
			body, err := json.Marshal(map[string]any{"data": screenshots, "links": map[string]any{}})
			if err != nil {
				t.Fatal(err)
			}
			return statusJSONResponse(string(body)), nil
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL)
			return nil, nil
		}
	})
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	var runErr error
	stdout, _ := captureOutput(t, func() {
		if err := root.Parse([]string{"screenshots", "plan", "--app", "123456789", "--version", "1.2.3", "--review-output-dir", reviewDir, "--output", "json"}); err != nil {
			t.Fatal(err)
		}
		runErr = root.Run(context.Background())
	})
	if runErr == nil {
		t.Fatal("expected capacity failure")
	}
	var payload struct {
		ErrorCount int                                                       `json:"errorCount"`
		Issues     []struct{ Severity, Locale, DisplayType, Message string } `json:"issues"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if payload.ErrorCount != 1 {
		t.Errorf("errorCount=%d, want 1", payload.ErrorCount)
	}
	found := false
	for _, issue := range payload.Issues {
		if issue.Severity == "error" && issue.Locale == "en-US" && issue.DisplayType == "APP_IPHONE_65" && strings.Contains(issue.Message, "no upload slots remain") {
			found = true
		}
	}
	if !found {
		t.Errorf("missing structured capacity failure: %+v", payload.Issues)
	}
}

func TestScreenshotsApplyCanonicalizesLegacyReviewDisplayAliases(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	reviewDir, imagePath := writeScreenshotReviewArtifactsWithPlannedDisplayType(
		t,
		1260,
		2736,
		1260,
		2736,
		[]string{"APP_IPHONE_67", "APP_IPHONE_69"},
	)
	fileInfo, err := os.Stat(imagePath)
	if err != nil {
		t.Fatalf("stat review artifact: %v", err)
	}

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	setListCalls := 0
	screenshotCreates := 0
	uploads := 0
	invalidSetCreates := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/123456789/appStoreVersions":
			return statusJSONResponse(`{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.2.3","platform":"IOS"}}]}`), nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
			return statusJSONResponse(`{"data":[{"type":"appStoreVersionLocalizations","id":"LOC_123","attributes":{"locale":"en-US"}}]}`), nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersionLocalizations/LOC_123/appScreenshotSets":
			setListCalls++
			return statusJSONResponse(`{"data":[{"type":"appScreenshotSets","id":"set-1","attributes":{"screenshotDisplayType":"APP_IPHONE_67"}}],"links":{}}`), nil
		case req.Method == http.MethodPost && req.URL.Path == "/v1/appScreenshotSets":
			invalidSetCreates++
			body, readErr := io.ReadAll(req.Body)
			if readErr != nil {
				t.Fatalf("read screenshot set request: %v", readErr)
			}
			return jsonHTTPResponse(http.StatusUnprocessableEntity, fmt.Sprintf(`{"errors":[{"status":"422","detail":"unexpected screenshot set request: %s"}]}`, body)), nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appScreenshotSets/set-1/appScreenshots":
			return statusJSONResponse(`{"data":[],"links":{}}`), nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appScreenshotSets/set-1/relationships/appScreenshots":
			return statusJSONResponse(`{"data":[],"links":{}}`), nil
		case req.Method == http.MethodPost && req.URL.Path == "/v1/appScreenshots":
			screenshotCreates++
			body := fmt.Sprintf(`{"data":{"type":"appScreenshots","id":"new-1","attributes":{"uploadOperations":[{"method":"PUT","url":"https://upload.example/new-1","length":%d,"offset":0}]}}}`, fileInfo.Size())
			return statusJSONResponse(body), nil
		case req.Method == http.MethodPut && req.URL.Host == "upload.example":
			uploads++
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
	if payload["plannedGroups"] != float64(1) {
		t.Fatalf("expected plannedGroups=1, got %v", payload["plannedGroups"])
	}
	groups, ok := payload["groups"].([]any)
	if !ok || len(groups) != 1 {
		t.Fatalf("expected one canonical group, got %T %v", payload["groups"], payload["groups"])
	}
	if displayType := groups[0].(map[string]any)["displayType"]; displayType != "APP_IPHONE_67" {
		t.Fatalf("expected displayType APP_IPHONE_67, got %v", displayType)
	}
	if setListCalls != 1 || screenshotCreates != 1 || uploads != 1 || invalidSetCreates != 0 {
		t.Fatalf(
			"expected one canonical mutation sequence, got set lists=%d screenshot creates=%d uploads=%d invalid set creates=%d",
			setListCalls,
			screenshotCreates,
			uploads,
			invalidSetCreates,
		)
	}
}

func TestScreenshotsApplyUploadsIPad13ScreenshotToCurrentSlotOnly(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	// Manifests written before the iPad slot correction list both the legacy
	// 12.9" 2nd-generation slot and the current 13" slot for 2048x2732.
	reviewDir, imagePath := writeScreenshotReviewArtifactsWithPlannedDisplayType(
		t,
		2048,
		2732,
		2048,
		2732,
		[]string{"APP_IPAD_PRO_129", "APP_IPAD_PRO_3GEN_129"},
	)
	fileInfo, err := os.Stat(imagePath)
	if err != nil {
		t.Fatalf("stat review artifact: %v", err)
	}

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	setCreates := make([]string, 0, 2)
	screenshotCreates := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/123456789/appStoreVersions":
			return statusJSONResponse(`{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.2.3","platform":"IOS"}}]}`), nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
			return statusJSONResponse(`{"data":[{"type":"appStoreVersionLocalizations","id":"LOC_123","attributes":{"locale":"en-US"}}]}`), nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersionLocalizations/LOC_123/appScreenshotSets":
			return statusJSONResponse(`{"data":[],"links":{}}`), nil
		case req.Method == http.MethodPost && req.URL.Path == "/v1/appScreenshotSets":
			body, readErr := io.ReadAll(req.Body)
			if readErr != nil {
				t.Fatalf("read screenshot set request: %v", readErr)
			}
			var payload struct {
				Data struct {
					Attributes struct {
						ScreenshotDisplayType string `json:"screenshotDisplayType"`
					} `json:"attributes"`
				} `json:"data"`
			}
			if err := json.Unmarshal(body, &payload); err != nil {
				t.Fatalf("unmarshal screenshot set request: %v", err)
			}
			setCreates = append(setCreates, payload.Data.Attributes.ScreenshotDisplayType)
			return statusJSONResponse(fmt.Sprintf(`{"data":{"type":"appScreenshotSets","id":"set-1","attributes":{"screenshotDisplayType":%q}}}`, payload.Data.Attributes.ScreenshotDisplayType)), nil
		case req.Method == http.MethodGet && strings.HasPrefix(req.URL.Path, "/v1/appScreenshotSets/"):
			return statusJSONResponse(`{"data":[],"links":{}}`), nil
		case req.Method == http.MethodPost && req.URL.Path == "/v1/appScreenshots":
			screenshotCreates++
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
		case req.Method == http.MethodPatch && strings.HasSuffix(req.URL.Path, "/relationships/appScreenshots"):
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
	if payload["plannedGroups"] != float64(1) {
		t.Fatalf("expected plannedGroups=1, got %v", payload["plannedGroups"])
	}
	groups, ok := payload["groups"].([]any)
	if !ok || len(groups) != 1 {
		t.Fatalf("expected one upload group, got %T %v", payload["groups"], payload["groups"])
	}
	if displayType := groups[0].(map[string]any)["displayType"]; displayType != "APP_IPAD_PRO_3GEN_129" {
		t.Fatalf("expected displayType APP_IPAD_PRO_3GEN_129, got %v", displayType)
	}
	if len(setCreates) != 1 || setCreates[0] != "APP_IPAD_PRO_3GEN_129" {
		t.Fatalf("expected one APP_IPAD_PRO_3GEN_129 screenshot set create, got %v", setCreates)
	}
	if screenshotCreates != 1 {
		t.Fatalf("expected one screenshot upload, got %d", screenshotCreates)
	}
}

func TestScreenshotsPlanRejectsVersionIDFromDifferentApp(t *testing.T) {
	testScreenshotReviewVersionMismatch(t, "plan", "--version-id", "999999999", "", `belongs to app "999999999", not "123456789"`)
}

func TestScreenshotsApplyRejectsCanonicalVersionMismatch(t *testing.T) {
	t.Run("app", func(t *testing.T) {
		testScreenshotReviewVersionMismatch(t, "apply", "--version", "999999999", "", `belongs to app "999999999", not "123456789"`)
	})
	t.Run("platform", func(t *testing.T) {
		testScreenshotReviewVersionMismatch(t, "apply", "--version", "123456789", "MAC_OS", `is on platform "IOS", not "MAC_OS"`)
	})
}

func testScreenshotReviewVersionMismatch(t *testing.T, command, selector, appID, platform, wantErr string) {
	t.Helper()
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	reviewDir, _ := writeScreenshotReviewArtifacts(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-1":
			return statusJSONResponse(`{
				"data":{
					"type":"appStoreVersions",
					"id":"version-1",
					"attributes":{"versionString":"1.2.3","platform":"IOS"},
					"relationships":{"app":{"data":{"type":"apps","id":"` + appID + `"}}}
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
		args := []string{
			"screenshots", command,
			"--app", "123456789",
			selector, "version-1",
			"--platform", platform,
			"--review-output-dir", reviewDir,
		}
		if command == "apply" {
			args = append(args, "--confirm")
		}
		if err := root.Parse(args); err != nil {
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
	if !strings.Contains(runErr.Error(), wantErr) {
		t.Fatalf("expected mismatch error, got %v", runErr)
	}
	diagnostic, ok := shared.DiagnosticFromError(runErr)
	if !ok || diagnostic.Code != shared.DiagnosticInvalidInput || diagnostic.Parameter != selector {
		t.Fatalf("diagnostic = %+v, %t; want invalid_input for %s", diagnostic, ok, selector)
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

func TestScreenshotsPlanRejectsEmptyReviewDisplayType(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	reviewDir, _ := writeScreenshotReviewArtifactsWithPlannedDisplayType(t, 1260, 2736, 1260, 2736, []string{"APP_IPHONE_67", "   "})

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
		t.Fatal("expected empty display type validation error")
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
	if payload["approvedReadyEntries"] != float64(0) {
		t.Fatalf("expected approvedReadyEntries=0, got %v", payload["approvedReadyEntries"])
	}
	if payload["plannedGroups"] != float64(0) {
		t.Fatalf("expected plannedGroups=0, got %v", payload["plannedGroups"])
	}
	if payload["errorCount"].(float64) < 1 {
		t.Fatalf("expected at least one blocking error, got %v", payload["errorCount"])
	}
	issues, ok := payload["issues"].([]any)
	if !ok || len(issues) == 0 {
		t.Fatalf("expected issues slice, got %T %v", payload["issues"], payload["issues"])
	}
	found := false
	for _, rawIssue := range issues {
		message := fmt.Sprint(rawIssue.(map[string]any)["message"])
		if strings.Contains(message, "empty screenshot display type") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected empty display type issue, got %v", issues)
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

// writeTwoSlotScreenshotReviewArtifacts writes two approved en-US screenshots
// in either one display type or two upload groups, with stable file order.
func writeTwoSlotScreenshotReviewArtifacts(t *testing.T, sameGroup bool) (string, []string) {
	t.Helper()

	reviewDir := t.TempDir()
	slots := []struct {
		id          string
		displayType string
		width       int
		height      int
	}{
		{id: "01-home", displayType: "APP_IPHONE_61", width: 1206, height: 2622},
		{id: "02-details", displayType: "APP_IPHONE_65", width: 1284, height: 2778},
	}

	if sameGroup {
		slots[1].displayType = slots[0].displayType
		slots[1].width = slots[0].width
		slots[1].height = slots[0].height
	}

	imagePaths := make([]string, 0, len(slots))
	entries := make([]map[string]any, 0, len(slots))
	approved := make([]string, 0, len(slots))
	for index, slot := range slots {
		imagePath := filepath.Join(reviewDir, slot.id+".png")
		if err := os.WriteFile(imagePath, markedPNGBytes(t, slot.width, slot.height, byte(index)), 0o600); err != nil {
			t.Fatalf("write screenshot image: %v", err)
		}
		imagePaths = append(imagePaths, imagePath)
		key := "en-US|iphone|" + slot.id
		approved = append(approved, key)
		entries = append(entries, map[string]any{
			"key":                  key,
			"screenshot_id":        slot.id,
			"locale":               "en-US",
			"device":               "iphone",
			"framed_path":          imagePath,
			"framed_relative_path": slot.id + ".png",
			"width":                slot.width,
			"height":               slot.height,
			"display_types":        []string{slot.displayType},
			"valid_app_store_size": true,
			"status":               "ready",
			"approved":             true,
			"approval_state":       "approved",
		})
	}

	manifestBytes, err := json.Marshal(map[string]any{
		"generated_at":  "2026-03-15T00:00:00Z",
		"raw_dir":       "",
		"framed_dir":    reviewDir,
		"output_dir":    reviewDir,
		"approval_path": filepath.Join(reviewDir, "approved.json"),
		"summary": map[string]any{
			"total":            len(entries),
			"ready":            len(entries),
			"missing_raw":      0,
			"invalid_size":     0,
			"approved":         len(entries),
			"pending_approval": 0,
		},
		"entries": entries,
	})
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	if err := os.WriteFile(filepath.Join(reviewDir, "manifest.json"), manifestBytes, 0o600); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	approvalBytes, err := json.Marshal(map[string]any{"approved": approved})
	if err != nil {
		t.Fatalf("marshal approvals: %v", err)
	}
	if err := os.WriteFile(filepath.Join(reviewDir, "approved.json"), approvalBytes, 0o600); err != nil {
		t.Fatalf("write approvals: %v", err)
	}

	return reviewDir, imagePaths
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
	return markedPNGBytes(t, width, height, 0)
}

// markedPNGBytes encodes an opaque image whose first pixel is marker, so
// screenshots in one upload stay distinct.
func markedPNGBytes(t *testing.T, width, height int, marker byte) []byte {
	t.Helper()

	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for i := 3; i < len(img.Pix); i += 4 {
		img.Pix[i] = 255
	}
	img.Pix[0] = marker
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return buf.Bytes()
}
