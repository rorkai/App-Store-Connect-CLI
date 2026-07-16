package cmdtest

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSubscriptionVersionsListJSON(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "missing.json"))
	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet || req.URL.Path != "/v1/subscriptions/123456789/versions" {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
		}
		if req.URL.Query().Get("filter[state]") != "PREPARE_FOR_SUBMISSION" || req.URL.Query().Get("include") != "localizations" {
			t.Fatalf("unexpected query: %s", req.URL.RawQuery)
		}
		return insightsJSONResponse(`{"data":[{"type":"subscriptionVersions","id":"ver-1","attributes":{"version":1,"state":"PREPARE_FOR_SUBMISSION"}}],"links":{}}`), nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"subscriptions", "versions", "list", "--subscription-id", "123456789", "--state", "PREPARE_FOR_SUBMISSION", "--include", "localizations", "--output", "json"}); err != nil {
			t.Fatal(err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})
	if stderr != "" {
		t.Fatalf("stderr = %q", stderr)
	}
	var payload struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout)
	}
	if len(payload.Data) != 1 || payload.Data[0].ID != "ver-1" {
		t.Fatalf("unexpected output: %s", stdout)
	}
}

func TestSubscriptionVersionsValidationUsesUsageErrors(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		message string
	}{
		{name: "missing version ID", args: []string{"subscriptions", "versions", "view"}, message: "Error: --id is required"},
		{name: "invalid state", args: []string{"subscriptions", "versions", "list", "--subscription-id", "123456789", "--state", "UNKNOWN"}, message: "invalid --state"},
		{name: "invalid relationship limit", args: []string{"subscriptions", "versions", "view", "--id", "ver-1", "--image-limit", "51"}, message: "--image-limit must be between 1 and 50"},
		{name: "next query conflict", args: []string{"subscriptions", "versions", "list", "--next", "https://api.appstoreconnect.apple.com/v1/subscriptions/sub-1/versions?cursor=next", "--state", "APPROVED"}, message: "--next cannot be combined with --state"},
		{name: "delete confirm", args: []string{"subscriptions", "versions", "images", "delete", "--id", "img-1"}, message: "Error: --confirm is required"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)
			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(test.args); err != nil {
					t.Fatal(err)
				}
				err := root.Run(context.Background())
				if !errors.Is(err, flag.ErrHelp) {
					t.Fatalf("expected usage error, got %v", err)
				}
			})
			if stdout != "" {
				t.Fatalf("stdout = %q", stdout)
			}
			if !strings.Contains(stderr, test.message) {
				t.Fatalf("stderr = %q, want %q", stderr, test.message)
			}
		})
	}
}

func TestSubscriptionsListRejectsVersionQueryFlagsForResolvedApp(t *testing.T) {
	t.Setenv("ASC_APP_ID", "6759231657")
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"subscriptions", "list", "--include", "versions"}); err != nil {
			t.Fatal(err)
		}
		err := root.Run(context.Background())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected usage error, got %v", err)
		}
	})
	if stdout != "" {
		t.Fatalf("stdout = %q", stdout)
	}
	if !strings.Contains(stderr, "require --group-id") {
		t.Fatalf("stderr = %q", stderr)
	}
}

func TestSubscriptionVersionImageUploadLifecycle(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "missing.json"))
	imagePath := filepath.Join(t.TempDir(), "image.png")
	if err := os.WriteFile(imagePath, []byte("test-image"), 0o600); err != nil {
		t.Fatal(err)
	}
	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })
	step := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		step++
		switch step {
		case 1:
			if req.Method != http.MethodPost || req.URL.Path != "/v2/subscriptionImages" {
				t.Fatalf("reservation request = %s %s", req.Method, req.URL.Path)
			}
			return insightsJSONResponse(`{"data":{"type":"subscriptionImages","id":"img-1","attributes":{"fileName":"image.png","fileSize":10,"uploadOperations":[{"method":"PUT","url":"https://upload.example.com/part","offset":0,"length":10}]}}}`), nil
		case 2:
			if req.Method != http.MethodPut || req.URL.Host != "upload.example.com" || req.ContentLength != 10 {
				t.Fatalf("upload request = %s %s length=%d", req.Method, req.URL.String(), req.ContentLength)
			}
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("")), Header: http.Header{}}, nil
		case 3:
			if req.Method != http.MethodPatch || req.URL.Path != "/v2/subscriptionImages/img-1" {
				t.Fatalf("commit request = %s %s", req.Method, req.URL.Path)
			}
			body, err := io.ReadAll(req.Body)
			if err != nil || !strings.Contains(string(body), `"uploaded":true`) {
				t.Fatalf("commit body = %s, err=%v", body, err)
			}
			return insightsJSONResponse(`{"data":{"type":"subscriptionImages","id":"img-1","attributes":{"fileName":"image.png","assetDeliveryState":{"state":"AWAITING_UPLOAD"}}}}`), nil
		default:
			t.Fatalf("unexpected request %d: %s %s", step, req.Method, req.URL.String())
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"subscriptions", "versions", "images", "upload", "--version-id", "ver-1", "--file", imagePath, "--output", "json"}); err != nil {
			t.Fatal(err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})
	if step != 3 {
		t.Fatalf("upload lifecycle performed %d steps, want 3", step)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q", stderr)
	}
	if !strings.Contains(stdout, `"id":"img-1"`) {
		t.Fatalf("stdout = %q", stdout)
	}
}
