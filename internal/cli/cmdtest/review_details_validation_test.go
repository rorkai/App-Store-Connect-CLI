package cmdtest

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	cmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
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

func TestReviewDetailsCreateRejectsOverlongDemoAccountPassword(t *testing.T) {
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
			"--demo-account-password", strings.Repeat("p", 101),
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
	if !strings.Contains(stderr, "--demo-account-password must be 100 characters or fewer") {
		t.Fatalf("expected local demo account password length error, got %q", stderr)
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

func TestReviewDetailsUpdateRejectsOverlongDemoAccountPassword(t *testing.T) {
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
			"review", "details-update",
			"--id", "detail-1",
			"--demo-account-password", strings.Repeat("p", 101),
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
	if !strings.Contains(stderr, "--demo-account-password must be 100 characters or fewer") {
		t.Fatalf("expected local demo account password length error, got %q", stderr)
	}
}

func TestReviewDetailsUpdateAcceptsMaxLengthDemoAccountPassword(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodPatch || req.URL.Path != "/v1/appStoreReviewDetails/detail-1" {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
		}
		payload, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("read body error: %v", err)
		}
		body := string(payload)
		if !strings.Contains(body, `"demoAccountPassword":"`+strings.Repeat("p", 100)+`"`) {
			t.Fatalf("expected max-length demo account password in body, got %s", body)
		}
		return jsonResponse(http.StatusOK, `{"data":{"type":"appStoreReviewDetails","id":"detail-1","attributes":{"demoAccountPassword":"`+strings.Repeat("p", 100)+`"}}}`)
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"review", "details-update",
			"--id", "detail-1",
			"--demo-account-password", strings.Repeat("p", 100),
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

func TestRunReviewDetailsRejectsOverlongDemoAccountPasswordWithUsageExit(t *testing.T) {
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_KEY_ID", "")
	t.Setenv("ASC_ISSUER_ID", "")
	t.Setenv("ASC_PRIVATE_KEY_PATH", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	tests := []struct {
		name string
		args []string
	}{
		{
			name: "details-create",
			args: []string{
				"review", "details-create",
				"--version-id", "version-1",
				"--demo-account-password", strings.Repeat("p", 101),
			},
		},
		{
			name: "details-update",
			args: []string{
				"review", "details-update",
				"--id", "detail-1",
				"--demo-account-password", strings.Repeat("p", 101),
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stdout, stderr := captureOutput(t, func() {
				code := cmd.Run(test.args, "1.2.3")
				if code != cmd.ExitUsage {
					t.Fatalf("expected exit code %d, got %d", cmd.ExitUsage, code)
				}
			})

			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, "--demo-account-password must be 100 characters or fewer") {
				t.Fatalf("expected local demo account password length error, got %q", stderr)
			}
		})
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

func TestReviewDetailsForVersionReturnsNotConfiguredStateWhenUnset(t *testing.T) {
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
		switch req.URL.Path {
		case "/v1/appStoreVersions/version-1/appStoreReviewDetail":
			return jsonResponse(http.StatusNotFound, `{"errors":[{"status":"404","code":"NOT_FOUND","title":"Not Found"}]}`)
		case "/v1/appStoreVersions/version-1":
			return jsonResponse(http.StatusOK, `{"data":{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.0"}}}`)
		default:
			t.Fatalf("unexpected path %s", req.URL.Path)
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"review", "details-for-version", "--version-id", "version-1", "--output", "json"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if !strings.Contains(stderr, "Warning: App Store review detail is not configured for version \"version-1\".") {
		t.Fatalf("expected not-configured warning, got %q", stderr)
	}

	var payload struct {
		VersionID  string `json:"versionId"`
		Configured bool   `json:"configured"`
		Message    string `json:"message"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout=%s", err, stdout)
	}
	if payload.VersionID != "version-1" {
		t.Fatalf("expected versionId version-1, got %q", payload.VersionID)
	}
	if payload.Configured {
		t.Fatal("expected configured=false")
	}
	if payload.Message == "" {
		t.Fatal("expected message")
	}
}

func TestReviewDetailsForVersionPreservesErrorForUnknownVersionID(t *testing.T) {
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
		switch req.URL.Path {
		case "/v1/appStoreVersions/missing-version/appStoreReviewDetail":
			return jsonResponse(http.StatusNotFound, `{"errors":[{"status":"404","code":"NOT_FOUND","title":"Not Found"}]}`)
		case "/v1/appStoreVersions/missing-version":
			return jsonResponse(http.StatusNotFound, `{"errors":[{"status":"404","code":"NOT_FOUND","title":"Not Found"}]}`)
		default:
			t.Fatalf("unexpected path %s", req.URL.Path)
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"review", "details-for-version", "--version-id", "missing-version", "--output", "json"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if runErr == nil {
		t.Fatal("expected error, got nil")
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(runErr.Error(), "review details-for-version: failed to fetch:") {
		t.Fatalf("expected wrapped fetch error, got %v", runErr)
	}
	if strings.Contains(runErr.Error(), "not configured") {
		t.Fatalf("expected unknown version to remain a hard error, got %v", runErr)
	}
}
