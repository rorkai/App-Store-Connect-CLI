package cmdtest

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
)

// Resource ID flags accept the API self-link that JSON output and webhook
// deliveries carry, so links.self values pipe straight back into asc. These
// tests pin the contract through the real command tree: the request path must
// use the extracted ID, and a self-link of the wrong type is a usage failure
// before authentication or any request.

const selfLinkTestBase = "https://api.appstoreconnect.apple.com"

func recordSelfLinkRequests(t *testing.T, respond func(path string) (string, bool)) func() []string {
	t.Helper()
	var mu sync.Mutex
	var paths []string
	stubTransport(t, func(req *http.Request) (*http.Response, error) {
		mu.Lock()
		paths = append(paths, req.Method+" "+req.URL.Path)
		mu.Unlock()
		body, ok := respond(req.URL.Path)
		if !ok {
			return jsonResponse(http.StatusNotFound, `{"errors":[{"status":"404","code":"NOT_FOUND","title":"not found"}]}`)
		}
		return jsonResponse(http.StatusOK, body)
	})
	return func() []string {
		mu.Lock()
		defer mu.Unlock()
		return append([]string(nil), paths...)
	}
}

func runSelfLinkCommand(t *testing.T, args ...string) (string, string) {
	t.Helper()
	root := RootCommand("test")
	root.FlagSet.SetOutput(io.Discard)
	return captureOutput(t, func() {
		if err := root.Parse(args); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})
}

func assertSelfLinkRequest(t *testing.T, requests []string, want string) {
	t.Helper()
	found := false
	for _, request := range requests {
		if strings.Contains(request, "https:") || strings.Contains(request, "http:") {
			t.Fatalf("request path leaked the self-link: %q", request)
		}
		if request == want {
			found = true
		}
	}
	if !found {
		t.Fatalf("requests = %q, want one equal to %q", requests, want)
	}
}

func TestSelfLinkBuildsInfoUsesExtractedID(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	requests := recordSelfLinkRequests(t, func(path string) (string, bool) {
		switch path {
		case "/v1/builds/build-1":
			return `{"data":{"type":"builds","id":"build-1","attributes":{"version":"9","processingState":"VALID","expired":false}}}`, true
		case "/v1/builds/build-1/preReleaseVersion":
			return `{"data":{"type":"preReleaseVersions","id":"prv-1","attributes":{"version":"1.2.3","platform":"IOS"}}}`, true
		}
		return "", false
	})

	stdout, stderr := runSelfLinkCommand(
		t,
		"builds", "info",
		"--build-id", selfLinkTestBase+"/v1/builds/build-1",
		"--output", "json",
	)
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"id":"build-1"`) {
		t.Fatalf("stdout = %q, want build-1", stdout)
	}
	assertSelfLinkRequest(t, requests(), "GET /v1/builds/build-1")
}

func TestSelfLinkVersionsViewUsesExtractedID(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	requests := recordSelfLinkRequests(t, func(path string) (string, bool) {
		if path == "/v1/appStoreVersions/version-1" {
			return `{"data":{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.0","platform":"IOS"}}}`, true
		}
		return "", false
	})

	stdout, stderr := runSelfLinkCommand(
		t,
		"versions", "view",
		"--version-id", selfLinkTestBase+"/v1/appStoreVersions/version-1?include=app",
		"--output", "json",
	)
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"id":"version-1"`) {
		t.Fatalf("stdout = %q, want version-1", stdout)
	}
	assertSelfLinkRequest(t, requests(), "GET /v1/appStoreVersions/version-1")
}

func TestSelfLinkTestflightCrashesViewUsesExtractedID(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	requests := recordSelfLinkRequests(t, func(path string) (string, bool) {
		if path == "/v1/betaFeedbackCrashSubmissions/crash-1" {
			return `{"data":{"type":"betaFeedbackCrashSubmissions","id":"crash-1","attributes":{"createdDate":"2026-09-01T00:00:00Z"}}}`, true
		}
		return "", false
	})

	stdout, stderr := runSelfLinkCommand(
		t,
		"testflight", "crashes", "view",
		"--submission-id", selfLinkTestBase+"/v1/betaFeedbackCrashSubmissions/crash-1",
		"--output", "json",
	)
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"id":"crash-1"`) {
		t.Fatalf("stdout = %q, want crash-1", stdout)
	}
	assertSelfLinkRequest(t, requests(), "GET /v1/betaFeedbackCrashSubmissions/crash-1")
}

func TestSelfLinkWrongTypeIsUsageErrorBeforeAuthOrRequest(t *testing.T) {
	resetCmdtestState()
	setCmdtestHome(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")
	stubTransport(t, func(req *http.Request) (*http.Response, error) {
		t.Fatalf("unexpected request %s %s", req.Method, req.URL.Path)
		return nil, nil
	})

	tests := []struct {
		name    string
		args    []string
		flag    string
		value   string
		wantErr string
	}{
		{
			name:    "builds info rejects apps link",
			args:    []string{"builds", "info"},
			flag:    "build-id",
			value:   selfLinkTestBase + "/v1/apps/123",
			wantErr: "expected a self-link of type builds, got apps",
		},
		{
			name:    "versions view rejects relationship path",
			args:    []string{"versions", "view"},
			flag:    "version-id",
			value:   selfLinkTestBase + "/v1/appStoreVersions/version-1/relationships/build",
			wantErr: "/v1/<type>/<id>",
		},
		{
			name:    "iap view rejects subscriptions link",
			args:    []string{"iap", "view"},
			flag:    "id",
			value:   selfLinkTestBase + "/v1/subscriptions/sub-1",
			wantErr: "expected a self-link of type inAppPurchases, got subscriptions",
		},
		{
			name:    "iap promoted-purchases view rejects subscriptions link",
			args:    []string{"iap", "promoted-purchases", "view"},
			flag:    "iap-id",
			value:   selfLinkTestBase + "/v1/subscriptions/sub-1",
			wantErr: "expected a self-link of type inAppPurchases, got subscriptions",
		},
		{
			name:    "iap versions links versions rejects version link",
			args:    []string{"iap", "versions", "links", "versions"},
			flag:    "iap-id",
			value:   selfLinkTestBase + "/v1/inAppPurchaseVersions/ver-1",
			wantErr: "expected a self-link of type inAppPurchases, got inAppPurchaseVersions",
		},
		{
			name:    "testflight groups view rejects other host",
			args:    []string{"testflight", "groups", "view"},
			flag:    "id",
			value:   "https://appstoreconnect.apple.com/v1/betaGroups/group-1",
			wantErr: "api.appstoreconnect.apple.com",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			args := append(append([]string(nil), test.args...), "--"+test.flag, test.value)
			stdout, stderr := captureOutput(t, func() {
				code := rootcmd.Run(args, "1.2.3")
				if code != rootcmd.ExitUsage {
					t.Fatalf("exit code = %d, want %d", code, rootcmd.ExitUsage)
				}
			})
			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			firstLine, _, _ := strings.Cut(stderr, "\n")
			wantPrefix := fmt.Sprintf("Error: invalid value %q for flag -%s: ", test.value, test.flag)
			if !strings.HasPrefix(firstLine, wantPrefix) || !strings.Contains(firstLine, test.wantErr) {
				t.Fatalf("first stderr line = %q, want prefix %q containing %q", firstLine, wantPrefix, test.wantErr)
			}
		})
	}
}
