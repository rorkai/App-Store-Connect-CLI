package cmdtest

import (
	"context"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
)

func TestBetaGroupsAddTestersMergesTesterAndEmailWithoutDuplicates(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

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
			if req.URL.Path != "/v1/betaGroups/group-1/app" {
				t.Fatalf("expected path /v1/betaGroups/group-1/app, got %s", req.URL.Path)
			}
			body := `{"data":{"type":"apps","id":"app-1"}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 2:
			if req.Method != http.MethodGet {
				t.Fatalf("expected GET, got %s", req.Method)
			}
			if req.URL.Path != "/v1/betaTesters" {
				t.Fatalf("expected path /v1/betaTesters, got %s", req.URL.Path)
			}
			query := req.URL.Query()
			if query.Get("filter[apps]") != "app-1" {
				t.Fatalf("expected filter[apps]=app-1, got %q", query.Get("filter[apps]"))
			}
			if query.Get("filter[email]") != "tester@example.com" {
				t.Fatalf("expected filter[email]=tester@example.com, got %q", query.Get("filter[email]"))
			}
			body := `{"data":[{"type":"betaTesters","id":"tester-1"}]}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 3:
			if req.Method != http.MethodPost {
				t.Fatalf("expected POST, got %s", req.Method)
			}
			if req.URL.Path != "/v1/betaGroups/group-1/relationships/betaTesters" {
				t.Fatalf("expected path /v1/betaGroups/group-1/relationships/betaTesters, got %s", req.URL.Path)
			}
			payload, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("read body error: %v", err)
			}
			if strings.Count(string(payload), `"id":"tester-1"`) != 1 {
				t.Fatalf("expected deduplicated tester id once, got payload %s", string(payload))
			}
			return &http.Response{
				StatusCode: http.StatusNoContent,
				Body:       io.NopCloser(strings.NewReader("")),
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
		if err := root.Parse([]string{
			"testflight", "beta-groups", "add-testers",
			"--group", "group-1",
			"--tester", "tester-1",
			"--email", "tester@example.com",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "Successfully added 1 tester(s) to group group-1") {
		t.Fatalf("expected deduped success message, got %q", stderr)
	}
}

func TestBetaGroupsAddTestersEmailPartialLookupFailureDoesNotMutate(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

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
			if req.URL.Path != "/v1/betaGroups/group-1/app" {
				t.Fatalf("expected path /v1/betaGroups/group-1/app, got %s", req.URL.Path)
			}
			body := `{"data":{"type":"apps","id":"app-1"}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 2:
			if req.Method != http.MethodGet {
				t.Fatalf("expected GET, got %s", req.Method)
			}
			if req.URL.Path != "/v1/betaTesters" {
				t.Fatalf("expected path /v1/betaTesters, got %s", req.URL.Path)
			}
			query := req.URL.Query()
			if query.Get("filter[apps]") != "app-1" {
				t.Fatalf("expected filter[apps]=app-1, got %q", query.Get("filter[apps]"))
			}
			if query.Get("filter[email]") != "valid@example.com" {
				t.Fatalf("expected first email lookup valid@example.com, got %q", query.Get("filter[email]"))
			}
			body := `{"data":[{"type":"betaTesters","id":"tester-valid"}]}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 3:
			if req.Method != http.MethodGet {
				t.Fatalf("expected GET, got %s", req.Method)
			}
			if req.URL.Path != "/v1/betaTesters" {
				t.Fatalf("expected path /v1/betaTesters, got %s", req.URL.Path)
			}
			query := req.URL.Query()
			if query.Get("filter[apps]") != "app-1" {
				t.Fatalf("expected filter[apps]=app-1, got %q", query.Get("filter[apps]"))
			}
			if query.Get("filter[email]") != "missing@example.com" {
				t.Fatalf("expected second email lookup missing@example.com, got %q", query.Get("filter[email]"))
			}
			body := `{"data":[]}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		default:
			t.Fatalf("unexpected mutation request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, _ := captureOutput(t, func() {
		if err := root.Parse([]string{
			"testflight", "beta-groups", "add-testers",
			"--group", "group-1",
			"--email", "valid@example.com,missing@example.com",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if runErr == nil {
		t.Fatal("expected lookup failure, got nil")
	}
	if !strings.Contains(runErr.Error(), `tester email "missing@example.com" not found for app "app-1"`) {
		t.Fatalf("expected missing email error, got %v", runErr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout on failure, got %q", stdout)
	}
}
