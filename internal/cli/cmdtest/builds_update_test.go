package cmdtest

import (
	"context"
	"errors"
	"flag"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildsUpdateSetsUsesNonExemptEncryption(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	requestCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		if requestCount != 1 {
			t.Fatalf("unexpected request count %d", requestCount)
		}
		if req.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", req.Method)
		}
		if req.URL.Path != "/v1/builds/build-99" {
			t.Fatalf("expected path /v1/builds/build-99, got %s", req.URL.Path)
		}
		payload, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("failed to read request body: %v", err)
		}
		bodyText := string(payload)
		if !strings.Contains(bodyText, `"usesNonExemptEncryption":false`) {
			t.Fatalf("expected usesNonExemptEncryption=false payload, got %s", bodyText)
		}
		body := `{"data":{"type":"builds","id":"build-99","attributes":{"version":"2.0","uploadedDate":"2026-03-18T00:00:00Z","usesNonExemptEncryption":false}}}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"builds", "update",
			"--build", "build-99",
			"--uses-non-exempt-encryption=false",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if requestCount != 1 {
		t.Fatalf("expected exactly one request, got %d", requestCount)
	}
	if !strings.Contains(stdout, `"id":"build-99"`) {
		t.Fatalf("expected updated build in stdout, got %q", stdout)
	}
	if !strings.Contains(stdout, `"usesNonExemptEncryption":false`) {
		t.Fatalf("expected updated encryption attribute in stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "Updated build build-99") {
		t.Fatalf("expected success stderr, got %q", stderr)
	}
}

func TestBuildsUpdateRejectsInvalidUsesNonExemptEncryptionValue(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"builds", "update",
			"--build", "build-99",
			"--uses-non-exempt-encryption=maybe",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if !errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp for invalid encryption value, got %v", runErr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "--uses-non-exempt-encryption must be 'true' or 'false'") {
		t.Fatalf("expected invalid-value stderr, got %q", stderr)
	}
}

func TestBuildsUpdateTreatsAlreadySetValueAsNoOp(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	requestCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		switch requestCount {
		case 1:
			if req.Method != http.MethodPatch {
				t.Fatalf("expected PATCH, got %s", req.Method)
			}
			if req.URL.Path != "/v1/builds/build-99" {
				t.Fatalf("expected path /v1/builds/build-99, got %s", req.URL.Path)
			}
			body := `{"errors":[{"status":"409","code":"ENTITY_ERROR.ATTRIBUTE.INVALID","title":"Build update conflict","detail":"The request could not be completed."}]}`
			return &http.Response{
				StatusCode: http.StatusConflict,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 2:
			if req.Method != http.MethodGet {
				t.Fatalf("expected GET, got %s", req.Method)
			}
			if req.URL.Path != "/v1/builds/build-99" {
				t.Fatalf("expected path /v1/builds/build-99, got %s", req.URL.Path)
			}
			body := `{"data":{"type":"builds","id":"build-99","attributes":{"version":"2.0","uploadedDate":"2026-03-18T00:00:00Z","usesNonExemptEncryption":false}}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		default:
			t.Fatalf("unexpected request count %d", requestCount)
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"builds", "update",
			"--build", "build-99",
			"--uses-non-exempt-encryption=false",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if requestCount != 2 {
		t.Fatalf("expected PATCH then GET, got %d requests", requestCount)
	}
	if !strings.Contains(stdout, `"id":"build-99"`) {
		t.Fatalf("expected current build in stdout, got %q", stdout)
	}
	if !strings.Contains(stdout, `"usesNonExemptEncryption":false`) {
		t.Fatalf("expected false encryption state in stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "Updated build build-99") {
		t.Fatalf("expected success stderr, got %q", stderr)
	}
}
