package cmdtest

import (
	"errors"
	"flag"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSnitchUsageErrors(t *testing.T) {
	runUsageExitCases(t, []usageExitCase{
		{
			name:    "MissingDescription",
			args:    []string{"snitch"},
			wantErr: "description is required",
			deps:    true,
		},
		{
			name:    "InvalidSeverity",
			args:    []string{"snitch", "--severity", "critical", "test issue"},
			wantErr: "--severity must be one of",
			deps:    true,
		},
		{
			name:    "RejectsTrailingSnitchFlagsAfterDescription",
			args:    []string{"snitch", "misordered description", "--confirm"},
			wantErr: "flags must appear before the description; move --confirm before the description",
			deps:    true,
		},
		{
			name:    "FlushSuggestsFileForPositionalPath",
			args:    []string{"snitch", "flush", "/tmp/snitch.log"},
			wantErr: "Error: unexpected argument \"/tmp/snitch.log\"\nDid you mean: asc snitch flush --file /tmp/snitch.log\n",
			deps:    true,
		},
	})
}

func TestSnitchDryRunWithValidLabels(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "test-token")
	t.Setenv("GH_TOKEN", "")

	callCount := 0
	transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		callCount++
		switch callCount {
		case 1:
			if req.Method != http.MethodGet || req.URL.Host != "api.github.com" || req.URL.Path != "/repos/rorkai/App-Store-Connect-CLI/labels" {
				t.Fatalf("unexpected first request: %s %s", req.Method, req.URL.String())
			}
			if req.Header.Get("Authorization") != "Bearer test-token" {
				t.Fatalf("unexpected auth header: %q", req.Header.Get("Authorization"))
			}
			body := `[{"name":"enhancement"},{"name":"p3"},{"name":"easy"},{"name":"asc-snitch"}]`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 2:
			if req.Method != http.MethodGet || req.URL.Host != "api.github.com" || req.URL.Path != "/search/issues" {
				t.Fatalf("unexpected second request: %s %s", req.Method, req.URL.String())
			}
			body := `{"items":[]}`
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

	_, stderr, err := runRootWithHTTPTransport(t, nil, transport, "snitch", "--dry-run", "--label", "enhancement", "--label", "p3", "valid label request")
	if err != nil {
		t.Fatalf("expected no error for valid labels, got %v", err)
	}

	if !strings.Contains(stderr, "Dry run: would create issue") {
		t.Fatalf("expected dry-run output, got %q", stderr)
	}
	if !strings.Contains(stderr, "valid label request") {
		t.Fatalf("expected issue preview, got %q", stderr)
	}
}

func TestSnitchDryRunContinuesWhenLabelValidationLookupFails(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "test-token")
	t.Setenv("GH_TOKEN", "")

	callCount := 0
	transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		callCount++
		switch callCount {
		case 1:
			if req.Method != http.MethodGet || req.URL.Host != "api.github.com" || req.URL.Path != "/repos/rorkai/App-Store-Connect-CLI/labels" {
				t.Fatalf("unexpected first request: %s %s", req.Method, req.URL.String())
			}
			return &http.Response{
				StatusCode: http.StatusServiceUnavailable,
				Body:       io.NopCloser(strings.NewReader(`{"message":"try later"}`)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 2:
			if req.Method != http.MethodGet || req.URL.Host != "api.github.com" || req.URL.Path != "/search/issues" {
				t.Fatalf("unexpected second request: %s %s", req.Method, req.URL.String())
			}
			body := `{"items":[]}`
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

	_, stderr, err := runRootWithHTTPTransport(t, nil, transport, "snitch", "--dry-run", "--label", "enhancement", "label lookup degraded")
	if err != nil {
		t.Fatalf("expected lookup failures to warn and continue, got %v", err)
	}

	if !strings.Contains(stderr, "continuing without preflight label validation") {
		t.Fatalf("expected label validation warning, got %q", stderr)
	}
	if !strings.Contains(stderr, "Dry run: would create issue") {
		t.Fatalf("expected dry-run output, got %q", stderr)
	}
	if !strings.Contains(stderr, "Labels: asc-snitch, bug, enhancement") {
		t.Fatalf("expected preview labels, got %q", stderr)
	}
}

func TestSnitchDryRunContinuesAfterLabelValidationTimeout(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "test-token")
	t.Setenv("GH_TOKEN", "")
	t.Setenv("ASC_TIMEOUT", "1ms")

	callCount := 0
	transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		callCount++
		switch callCount {
		case 1:
			if req.Method != http.MethodGet || req.URL.Host != "api.github.com" || req.URL.Path != "/repos/rorkai/App-Store-Connect-CLI/labels" {
				t.Fatalf("unexpected first request: %s %s", req.Method, req.URL.String())
			}
			<-req.Context().Done()
			return nil, req.Context().Err()
		case 2:
			if req.Method != http.MethodGet || req.URL.Host != "api.github.com" || req.URL.Path != "/search/issues" {
				t.Fatalf("unexpected second request: %s %s", req.Method, req.URL.String())
			}
			body := `{"items":[]}`
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

	_, stderr, err := runRootWithHTTPTransport(t, nil, transport, "snitch", "--dry-run", "--label", "enhancement", "label timeout degraded")
	if err != nil {
		t.Fatalf("expected timeout failures to warn and continue, got %v", err)
	}

	if !strings.Contains(stderr, "continuing without preflight label validation") {
		t.Fatalf("expected label validation warning, got %q", stderr)
	}
	if !strings.Contains(stderr, "Dry run: would create issue") {
		t.Fatalf("expected dry-run output, got %q", stderr)
	}
	if callCount != 2 {
		t.Fatalf("expected validation + duplicate-search requests, got %d", callCount)
	}
}

func TestSnitchInvalidLabelReturnsUsage(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "test-token")
	t.Setenv("GH_TOKEN", "")

	transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet || req.URL.Host != "api.github.com" || req.URL.Path != "/repos/rorkai/App-Store-Connect-CLI/labels" {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
		}
		body := `[{"name":"enhancement"},{"name":"p3"},{"name":"easy"},{"name":"asc-snitch"}]`
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

	_, stderr, err := runRootWithHTTPTransport(t, nil, transport, "snitch", "--dry-run", "--label", "not-a-real-label", "invalid label request")
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected ErrHelp, got %v", err)
	}

	if !strings.Contains(stderr, "--label must reference existing repo labels") {
		t.Fatalf("expected invalid label validation error, got %q", stderr)
	}
	if !strings.Contains(stderr, "not-a-real-label") {
		t.Fatalf("expected invalid label name in error, got %q", stderr)
	}
}

func TestSnitchDryRunNoToken(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "")

	_, stderr, err := runRootWithDeps(t, nil, "snitch", "--dry-run", "test dry run")
	if err != nil {
		t.Fatalf("expected no error for dry-run, got %v", err)
	}

	if !strings.Contains(stderr, "Dry run: would create issue") {
		t.Fatalf("expected dry-run output, got %q", stderr)
	}
	if !strings.Contains(stderr, "skipping duplicate search") {
		t.Fatalf("expected offline duplicate search note, got %q", stderr)
	}
	if !strings.Contains(stderr, "test dry run") {
		t.Fatalf("expected issue title in dry-run output, got %q", stderr)
	}
}

func TestSnitchDryRunConfirmNoToken(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "")

	_, stderr, err := runRootWithDeps(t, nil, "snitch", "--dry-run", "--confirm", "test dry run confirm")
	if err != nil {
		t.Fatalf("expected no error for dry-run confirm, got %v", err)
	}

	if !strings.Contains(stderr, "Dry run: would create issue") {
		t.Fatalf("expected dry-run output, got %q", stderr)
	}
	if !strings.Contains(stderr, "skipping duplicate search") {
		t.Fatalf("expected offline duplicate search note, got %q", stderr)
	}
	if !strings.Contains(stderr, "test dry run confirm") {
		t.Fatalf("expected issue title in dry-run output, got %q", stderr)
	}
	if strings.Contains(stderr, "GITHUB_TOKEN or GH_TOKEN is required") {
		t.Fatalf("did not expect token requirement in dry-run confirm output, got %q", stderr)
	}
}

func TestSnitchPreviewWithoutConfirmNoToken(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "")

	_, stderr, err := runRootWithDeps(t, nil, "snitch", "preview", "without", "confirm")
	if err != nil {
		t.Fatalf("expected preview mode without confirm, got %v", err)
	}

	if !strings.Contains(stderr, "Preview only: rerun with --confirm to create issue") {
		t.Fatalf("expected preview-only message, got %q", stderr)
	}
	if !strings.Contains(stderr, "preview without confirm") {
		t.Fatalf("expected full multi-word description, got %q", stderr)
	}
}

func TestSnitchDryRunLocalDoesNotWriteLog(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "")

	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	defer func() {
		if err := os.Chdir(origDir); err != nil {
			t.Fatalf("os.Chdir restore error: %v", err)
		}
	}()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("os.Chdir temp dir error: %v", err)
	}

	_, stderr, err := runRootWithDeps(t, nil, "snitch", "--dry-run", "--local", "local dry run entry")
	if err != nil {
		t.Fatalf("expected no error for dry-run local, got %v", err)
	}

	if !strings.Contains(stderr, "Dry run: would create issue") {
		t.Fatalf("expected dry-run output, got %q", stderr)
	}
	if strings.Contains(stderr, "Friction logged") {
		t.Fatalf("did not expect local log write output, got %q", stderr)
	}

	logPath := filepath.Join(".asc", "snitch.log")
	if _, err := os.Stat(logPath); !os.IsNotExist(err) {
		t.Fatalf("expected no local log file, got err=%v", err)
	}
}

func TestSnitchConfirmNoTokenReturnsError(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "")

	_, _, err := runRootWithDeps(t, nil, "snitch", "--confirm", "test without token")
	if err == nil {
		t.Fatal("expected error when no token is set")
	}
	if !strings.Contains(err.Error(), "GITHUB_TOKEN or GH_TOKEN is required") {
		t.Fatalf("expected token error, got: %v", err)
	}
}

func TestSnitchFlushNoFile(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	defer func() {
		if err := os.Chdir(origDir); err != nil {
			t.Fatalf("os.Chdir restore error: %v", err)
		}
	}()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("os.Chdir temp dir error: %v", err)
	}

	_, stderr, err := runRootWithDeps(t, nil, "snitch", "flush")
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if !strings.Contains(stderr, "No local snitch entries found") {
		t.Fatalf("expected no entries message, got %q", stderr)
	}
}

func TestSnitchFlushFormatsEntries(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	defer func() {
		if err := os.Chdir(origDir); err != nil {
			t.Fatalf("os.Chdir restore error: %v", err)
		}
	}()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("os.Chdir temp dir error: %v", err)
	}

	_, logStderr, err := runRootWithDeps(t, nil, "snitch", "--local", "--repro", `asc status --app "com.example.app"`, "status command needs bundle ID support")
	if err != nil {
		t.Fatalf("run error: %v", err)
	}
	if !strings.Contains(logStderr, "Friction logged") {
		t.Fatalf("expected friction logged message, got %q", logStderr)
	}

	stdout, stderr, err := runRootWithDeps(t, nil, "snitch", "flush")
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected no stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, "[1] bug: status command needs bundle ID support") {
		t.Fatalf("expected formatted flush output, got %q", stdout)
	}
	if !strings.Contains(stdout, `asc status --app "com.example.app"`) {
		t.Fatalf("expected reproduction details in flush output, got %q", stdout)
	}
}
