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

func TestReviewDetailsCreateRejectsDemoAccountRequiredWithoutCredentials(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		t.Fatalf("unexpected HTTP request: %s %s", req.Method, req.URL.Path)
		return nil, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"review", "details-create",
			"--version-id", "version-1",
			"--demo-account-required=true",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if !errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("expected ErrHelp, got %v", runErr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "--demo-account-required=true requires both --demo-account-name and --demo-account-password") {
		t.Fatalf("expected local demo credential validation error, got %q", stderr)
	}
}

func TestReviewDetailsCreateAllowsDemoAccountRequiredWithBothCredentials(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/appStoreReviewDetails" {
			t.Fatalf("expected path /v1/appStoreReviewDetails, got %s", req.URL.Path)
		}
		payload, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("read body error: %v", err)
		}
		body := string(payload)
		if !strings.Contains(body, `"demoAccountRequired":true`) {
			t.Fatalf("expected demoAccountRequired=true in body, got %s", body)
		}
		if !strings.Contains(body, `"demoAccountName":"reviewer@example.com"`) {
			t.Fatalf("expected demoAccountName in body, got %s", body)
		}
		if !strings.Contains(body, `"demoAccountPassword":"app-specific-password"`) {
			t.Fatalf("expected demoAccountPassword in body, got %s", body)
		}
		return jsonResponse(http.StatusCreated, `{"data":{"type":"appStoreReviewDetails","id":"detail-1","attributes":{"demoAccountRequired":true,"demoAccountName":"reviewer@example.com","demoAccountPassword":"app-specific-password"}}}`)
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"review", "details-create",
			"--version-id", "version-1",
			"--demo-account-required=true",
			"--demo-account-name", "reviewer@example.com",
			"--demo-account-password", "app-specific-password",
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
	if !strings.Contains(stdout, `"id":"detail-1"`) {
		t.Fatalf("expected detail id in output, got %q", stdout)
	}
}

func TestReviewDetailsUpdateRejectsDemoAccountRequiredWhenExistingCredentialsAreIncomplete(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreReviewDetails/detail-1" {
			return jsonResponse(http.StatusOK, `{"data":{"type":"appStoreReviewDetails","id":"detail-1","attributes":{"contactFirstName":"Dev","contactLastName":"Support","contactEmail":"dev@example.com","contactPhone":"123","demoAccountRequired":false}}}`)
		}
		if req.Method == http.MethodPatch {
			t.Fatalf("unexpected PATCH request: %s", req.URL.Path)
		}
		t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
		return nil, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"review", "details-update",
			"--id", "detail-1",
			"--demo-account-required=true",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if !errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("expected ErrHelp, got %v", runErr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "--demo-account-required=true requires both --demo-account-name and --demo-account-password") {
		t.Fatalf("expected local demo credential validation error, got %q", stderr)
	}
}

func TestReviewDetailsUpdateAllowsDemoAccountRequiredWhenExistingCredentialsArePresent(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreReviewDetails/detail-1":
			return jsonResponse(http.StatusOK, `{"data":{"type":"appStoreReviewDetails","id":"detail-1","attributes":{"contactFirstName":"Dev","contactLastName":"Support","contactEmail":"dev@example.com","contactPhone":"123","demoAccountRequired":false,"demoAccountName":"reviewer@example.com","demoAccountPassword":"app-specific-password"}}}`)
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/appStoreReviewDetails/detail-1":
			payload, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("read body error: %v", err)
			}
			body := string(payload)
			if !strings.Contains(body, `"demoAccountRequired":true`) {
				t.Fatalf("expected demoAccountRequired=true in body, got %s", body)
			}
			if strings.Contains(body, "demoAccountName") || strings.Contains(body, "demoAccountPassword") {
				t.Fatalf("expected update to rely on existing demo credentials, got %s", body)
			}
			return jsonResponse(http.StatusOK, `{"data":{"type":"appStoreReviewDetails","id":"detail-1","attributes":{"demoAccountRequired":true,"demoAccountName":"reviewer@example.com","demoAccountPassword":"app-specific-password"}}}`)
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"review", "details-update",
			"--id", "detail-1",
			"--demo-account-required=true",
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
	if !strings.Contains(stdout, `"id":"detail-1"`) {
		t.Fatalf("expected detail id in output, got %q", stdout)
	}
}
