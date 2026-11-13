package cmdtest

import (
	"context"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
)

func TestAppsRemoveBetaTestersValidationErrors(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "apps remove-beta-testers missing app",
			args:    []string{"apps", "remove-beta-testers", "--tester", "TESTER_ID", "--confirm"},
			wantErr: "--app is required",
		},
		{
			name:    "apps remove-beta-testers missing tester",
			args:    []string{"apps", "remove-beta-testers", "--app", "APP_ID", "--confirm"},
			wantErr: "--tester is required",
		},
		{
			name:    "apps remove-beta-testers missing confirm",
			args:    []string{"apps", "remove-beta-testers", "--app", "APP_ID", "--tester", "TESTER_ID"},
			wantErr: "--confirm is required",
		},
	}

	runValidationTests(t, tests)
}

func TestAppsRemoveBetaTestersOutput(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodDelete {
			t.Fatalf("expected DELETE, got %s", req.Method)
		}
		if req.URL.Path != "/v1/apps/app-1/relationships/betaTesters" {
			t.Fatalf("expected path /v1/apps/app-1/relationships/betaTesters, got %s", req.URL.Path)
		}
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("read body error: %v", err)
		}
		if !strings.Contains(string(body), `"id":"tester-1"`) {
			t.Fatalf("expected tester-1 in body, got %s", string(body))
		}
		return &http.Response{
			StatusCode: http.StatusNoContent,
			Body:       io.NopCloser(strings.NewReader("")),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"apps", "remove-beta-testers", "--app", "app-1", "--tester", "tester-1,tester-2", "--confirm"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"appId":"app-1"`) {
		t.Fatalf("expected app id in output, got %q", stdout)
	}
	if !strings.Contains(stdout, `"testerIds"`) {
		t.Fatalf("expected tester ids in output, got %q", stdout)
	}
	if !strings.Contains(stdout, `"action":"removed"`) {
		t.Fatalf("expected action removed in output, got %q", stdout)
	}
}
