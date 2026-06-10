package cmdtest

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
)

func runBackgroundAssetsCommandWithTransport(
	t *testing.T,
	args []string,
	wantPath string,
	wantQuery map[string]string,
	body string,
) (string, string, error) {
	t.Helper()

	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != wantPath {
			t.Fatalf("expected path %s, got %s", wantPath, req.URL.Path)
		}
		values := req.URL.Query()
		for key, want := range wantQuery {
			if got := values.Get(key); got != want {
				t.Fatalf("expected %s=%q, got %q", key, want, got)
			}
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse(args); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})
	return stdout, stderr, runErr
}

func TestBackgroundAssetsListFiltersByVersionsLocale(t *testing.T) {
	stdout, stderr, err := runBackgroundAssetsCommandWithTransport(
		t,
		[]string{"background-assets", "list", "--app", "app-1", "--versions-locale", "en-US, ja"},
		"/v1/apps/app-1/backgroundAssets",
		map[string]string{"filter[versions.locale]": "en-US,ja"},
		`{"data":[{"type":"backgroundAssets","id":"asset-1"}]}`,
	)
	if err != nil {
		t.Fatalf("run error: %v", err)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var payload struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if jsonErr := json.Unmarshal([]byte(stdout), &payload); jsonErr != nil {
		t.Fatalf("failed to parse stdout JSON: %v (stdout %q)", jsonErr, stdout)
	}
	if len(payload.Data) != 1 || payload.Data[0].ID != "asset-1" {
		t.Fatalf("expected asset-1 in output, got %+v", payload.Data)
	}
}

func TestBackgroundAssetsVersionsListFiltersByLocale(t *testing.T) {
	stdout, stderr, err := runBackgroundAssetsCommandWithTransport(
		t,
		[]string{"background-assets", "versions", "list", "--background-asset-id", "asset-1", "--locale", "en-US"},
		"/v1/backgroundAssets/asset-1/versions",
		map[string]string{"filter[locale]": "en-US"},
		`{"data":[{"type":"backgroundAssetVersions","id":"version-1"}]}`,
	)
	if err != nil {
		t.Fatalf("run error: %v", err)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var payload struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if jsonErr := json.Unmarshal([]byte(stdout), &payload); jsonErr != nil {
		t.Fatalf("failed to parse stdout JSON: %v (stdout %q)", jsonErr, stdout)
	}
	if len(payload.Data) != 1 || payload.Data[0].ID != "version-1" {
		t.Fatalf("expected version-1 in output, got %+v", payload.Data)
	}
}
