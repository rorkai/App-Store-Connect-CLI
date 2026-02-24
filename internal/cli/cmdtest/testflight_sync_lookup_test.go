package cmdtest

import (
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTestFlightSyncPullResolvesAppByBundleID(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	outputPath := filepath.Join(t.TempDir(), "sync.yaml")

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	callCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		callCount++
		switch callCount {
		case 1:
			if req.Method != http.MethodGet {
				t.Fatalf("expected GET, got %s", req.Method)
			}
			if req.URL.Path != "/v1/apps" {
				t.Fatalf("expected path /v1/apps, got %s", req.URL.Path)
			}
			query := req.URL.Query()
			if query.Get("filter[bundleId]") != "com.example.sync" {
				t.Fatalf("expected bundle filter com.example.sync, got %q", query.Get("filter[bundleId]"))
			}
			if query.Get("limit") != "2" {
				t.Fatalf("expected limit=2, got %q", query.Get("limit"))
			}
			body := `{"data":[{"type":"apps","id":"app-sync"}]}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 2:
			if req.Method != http.MethodGet {
				t.Fatalf("expected GET, got %s", req.Method)
			}
			if req.URL.Path != "/v1/apps/app-sync" {
				t.Fatalf("expected path /v1/apps/app-sync, got %s", req.URL.Path)
			}
			body := `{"data":{"type":"apps","id":"app-sync","attributes":{"name":"Sync Demo","bundleId":"com.example.sync"}}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 3:
			if req.Method != http.MethodGet {
				t.Fatalf("expected GET, got %s", req.Method)
			}
			if req.URL.Path != "/v1/apps/app-sync/betaGroups" {
				t.Fatalf("expected path /v1/apps/app-sync/betaGroups, got %s", req.URL.Path)
			}
			if req.URL.Query().Get("limit") != "200" {
				t.Fatalf("expected limit=200, got %q", req.URL.Query().Get("limit"))
			}
			body := `{"data":[],"links":{"next":""}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		default:
			t.Fatalf("unexpected request count %d", callCount)
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"testflight", "sync", "pull", "--app", "com.example.sync", "--output", outputPath}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"app":"Sync Demo"`) {
		t.Fatalf("expected summary to contain app name, got %q", stdout)
	}
	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("expected sync output file, got error: %v", err)
	}
	if !strings.Contains(string(data), "id: app-sync") {
		t.Fatalf("expected sync yaml to contain app id, got %q", string(data))
	}
}

func TestTestFlightSyncPullLookupNotFoundByName(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	outputPath := filepath.Join(t.TempDir(), "sync.yaml")

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	callCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		callCount++
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/apps" {
			t.Fatalf("expected path /v1/apps, got %s", req.URL.Path)
		}

		query := req.URL.Query()
		switch callCount {
		case 1:
			if query.Get("filter[bundleId]") != "Missing App" {
				t.Fatalf("expected bundle lookup for Missing App, got %q", query.Get("filter[bundleId]"))
			}
		case 2:
			if query.Get("filter[name]") != "Missing App" {
				t.Fatalf("expected name lookup for Missing App, got %q", query.Get("filter[name]"))
			}
		case 3:
			// Full-scan fallback request (no name filter) for exact-name matching.
			if query.Get("filter[name]") != "" {
				t.Fatalf("expected fallback full scan without name filter, got %q", query.Get("filter[name]"))
			}
		case 4:
			// Legacy fuzzy fallback request for backward compatibility.
			if query.Get("filter[name]") != "Missing App" {
				t.Fatalf("expected legacy fuzzy lookup for Missing App, got %q", query.Get("filter[name]"))
			}
		default:
			t.Fatalf("unexpected request count %d", callCount)
		}

		body := `{"data":[]}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, _ := captureOutput(t, func() {
		if err := root.Parse([]string{"testflight", "sync", "pull", "--app", "Missing App", "--output", outputPath}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if runErr == nil {
		t.Fatal("expected lookup not found error, got nil")
	}
	if !strings.Contains(runErr.Error(), `app "Missing App" not found`) {
		t.Fatalf("expected not found lookup error, got %v", runErr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout on lookup failure, got %q", stdout)
	}
	if _, err := os.Stat(outputPath); err == nil {
		t.Fatalf("expected no output file on lookup failure, but file exists: %s", outputPath)
	}
}

func TestTestFlightSyncPullLookupAmbiguousName(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	outputPath := filepath.Join(t.TempDir(), "sync.yaml")

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	callCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		callCount++
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/apps" {
			t.Fatalf("expected path /v1/apps, got %s", req.URL.Path)
		}

		switch callCount {
		case 1:
			body := `{"data":[]}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 2:
			body := `{"data":[{"type":"apps","id":"app-1","attributes":{"name":"Ambiguous App"}},{"type":"apps","id":"app-2","attributes":{"name":"Ambiguous App"}}]}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		default:
			t.Fatalf("unexpected request count %d", callCount)
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, _ := captureOutput(t, func() {
		if err := root.Parse([]string{"testflight", "sync", "pull", "--app", "Ambiguous App", "--output", outputPath}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if runErr == nil {
		t.Fatal("expected ambiguous lookup error, got nil")
	}
	if !strings.Contains(runErr.Error(), `multiple apps found for name "Ambiguous App"`) {
		t.Fatalf("expected ambiguous name error, got %v", runErr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout on lookup failure, got %q", stdout)
	}
	if _, err := os.Stat(outputPath); err == nil {
		t.Fatalf("expected no output file on lookup failure, but file exists: %s", outputPath)
	}
}
