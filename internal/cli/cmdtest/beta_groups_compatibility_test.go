package cmdtest

import (
	"context"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
)

func TestBetaGroupsEditCompatibilityFlags(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", req.Method)
		}
		if req.URL.Path != "/v1/betaGroups/bg-1" {
			t.Fatalf("expected path /v1/betaGroups/bg-1, got %s", req.URL.Path)
		}
		payload, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("read body error: %v", err)
		}
		body := string(payload)
		if !strings.Contains(body, `"iosBuildsAvailableForAppleSiliconMac":false`) {
			t.Fatalf("expected Apple silicon Mac compatibility flag in body, got %s", body)
		}
		if !strings.Contains(body, `"iosBuildsAvailableForAppleVision":false`) {
			t.Fatalf("expected Apple Vision compatibility flag in body, got %s", body)
		}

		responseBody := `{"data":{"type":"betaGroups","id":"bg-1","attributes":{"name":"External","iosBuildsAvailableForAppleSiliconMac":false,"iosBuildsAvailableForAppleVision":false}}}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(responseBody)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"testflight", "groups", "edit",
			"--id", "bg-1",
			"--ios-builds-available-for-apple-silicon-mac", "false",
			"--ios-builds-available-for-apple-vision", "false",
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
	if !strings.Contains(stdout, `"iosBuildsAvailableForAppleSiliconMac":false`) {
		t.Fatalf("expected Mac compatibility flag in output, got %q", stdout)
	}
	if !strings.Contains(stdout, `"iosBuildsAvailableForAppleVision":false`) {
		t.Fatalf("expected Vision compatibility flag in output, got %q", stdout)
	}
}
