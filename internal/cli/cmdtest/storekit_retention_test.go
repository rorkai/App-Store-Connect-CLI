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

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/config"
)

func TestStoreKitRetentionMessagingCommandTree(t *testing.T) {
	root := RootCommand("1.2.3")
	paths := [][]string{
		{"storekit", "auth", "login"},
		{"storekit", "auth", "status"},
		{"storekit", "auth", "switch"},
		{"storekit", "auth", "doctor"},
		{"storekit", "auth", "logout"},
		{"storekit", "retention-messaging", "images", "list"},
		{"storekit", "retention-messaging", "images", "upload"},
		{"storekit", "retention-messaging", "images", "delete"},
		{"storekit", "retention-messaging", "messages", "list"},
		{"storekit", "retention-messaging", "messages", "upload"},
		{"storekit", "retention-messaging", "messages", "delete"},
		{"storekit", "retention-messaging", "defaults", "view"},
		{"storekit", "retention-messaging", "defaults", "set"},
		{"storekit", "retention-messaging", "defaults", "delete"},
		{"storekit", "retention-messaging", "endpoint", "view"},
		{"storekit", "retention-messaging", "endpoint", "set"},
		{"storekit", "retention-messaging", "endpoint", "delete"},
		{"storekit", "retention-messaging", "performance", "start"},
		{"storekit", "retention-messaging", "performance", "view"},
		{"storekit", "retention-messaging", "performance", "wait"},
	}
	for _, path := range paths {
		if findSubcommand(root, path...) == nil {
			t.Errorf("missing command path %s", strings.Join(path, " "))
		}
	}
}

func TestStoreKitRetentionRequiresExplicitEnvironment(t *testing.T) {
	t.Setenv("ASC_STOREKIT_ENVIRONMENT", "")
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"storekit", "retention-messaging", "messages", "list"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("run error = %v, want flag.ErrHelp", err)
		}
	})
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	if !strings.Contains(stderr, "--environment is required (or set ASC_STOREKIT_ENVIRONMENT)") {
		t.Fatalf("stderr = %q", stderr)
	}
}

func TestStoreKitDeleteRequiresConfirm(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"storekit", "retention-messaging", "messages", "delete",
			"--message-id", "33333333-3333-4333-8333-333333333333",
			"--environment", "sandbox",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("run error = %v, want flag.ErrHelp", err)
		}
	})
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	if !strings.Contains(stderr, "--confirm is required") {
		t.Fatalf("stderr = %q", stderr)
	}
}

func TestStoreKitMessagesListSuccess(t *testing.T) {
	setupStoreKitAuth(t)
	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet || req.URL.Host != "api.storekit-sandbox.apple.com" || req.URL.Path != "/inApps/v1/messaging/message/list" {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
		}
		if !strings.HasPrefix(req.Header.Get("Authorization"), "Bearer ") {
			t.Fatalf("missing bearer token: %q", req.Header.Get("Authorization"))
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body: io.NopCloser(strings.NewReader(
				`{"messageIdentifiers":[{"messageIdentifier":"33333333-3333-4333-8333-333333333333","messageState":"APPROVED"}]}`,
			)),
		}, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"storekit", "retention-messaging", "messages", "list", "--environment", "sandbox", "--output", "json"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})
	if stderr != "" {
		t.Fatalf("stderr = %q", stderr)
	}
	var response struct {
		Messages []struct {
			Identifier string `json:"messageIdentifier"`
			State      string `json:"messageState"`
		} `json:"messageIdentifiers"`
	}
	if err := json.Unmarshal([]byte(stdout), &response); err != nil {
		t.Fatalf("invalid JSON output %q: %v", stdout, err)
	}
	if len(response.Messages) != 1 || response.Messages[0].State != "APPROVED" {
		t.Fatalf("response = %#v", response)
	}
}

func TestStoreKitAuthLoginStoresDedicatedProfile(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.json")
	keyPath := filepath.Join(t.TempDir(), "SubscriptionKey.p8")
	writeECDSAPEM(t, keyPath)
	t.Setenv("ASC_CONFIG_PATH", configPath)
	t.Setenv("ASC_STOREKIT_BYPASS_KEYCHAIN", "1")

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"storekit", "auth", "login", "--name", " Retention ",
			"--key-id", " STOREKIT_KEY ", "--issuer-id", " STOREKIT_ISSUER ",
			"--private-key", " " + keyPath + " ", "--bundle-id", " com.example.app ",
			"--bypass-keychain",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})
	if stderr != "" || !strings.Contains(stdout, "Successfully registered StoreKit API key") {
		t.Fatalf("stdout=%q stderr=%q", stdout, stderr)
	}
	cfg, err := config.LoadAt(configPath)
	if err != nil {
		t.Fatalf("LoadAt() error = %v", err)
	}
	if cfg.StoreKit.DefaultKeyName != "Retention" || len(cfg.StoreKit.Keys) != 1 || cfg.StoreKit.Keys[0].BundleID != "com.example.app" {
		t.Fatalf("StoreKit config = %#v", cfg.StoreKit)
	}
	if len(cfg.Keys) != 0 {
		t.Fatalf("App Store Connect credentials changed: %#v", cfg.Keys)
	}
}

func TestStoreKitAuthStatusValidatesEnvironmentCredentials(t *testing.T) {
	setupStoreKitAuth(t)
	requests := 0
	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests++
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"messageIdentifiers":[]}`)),
		}, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"storekit", "auth", "status", "--validate", "--environment", "sandbox", "--output", "json"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})
	if stderr != "" || requests != 1 {
		t.Fatalf("stderr=%q requests=%d", stderr, requests)
	}
	if !strings.Contains(stdout, `"name":"environment"`) || strings.Contains(stdout, "PRIVATE KEY") || strings.Contains(stdout, "SubscriptionKey.p8") {
		t.Fatalf("unsafe or incomplete status output: %q", stdout)
	}
}

func TestStoreKitMessagesUploadSuccess(t *testing.T) {
	setupStoreKitAuth(t)
	messagePath := filepath.Join(t.TempDir(), "message.json")
	messageJSON := `{"header":"Stay with Example","body":"Keep everything you have unlocked."}`
	if err := os.WriteFile(messagePath, []byte(messageJSON), 0o600); err != nil {
		t.Fatal(err)
	}
	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodPut || req.URL.Path != "/inApps/v1/messaging/message/33333333-3333-4333-8333-333333333333" {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
		}
		var payload map[string]any
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if payload["header"] != "Stay with Example" {
			t.Fatalf("payload = %#v", payload)
		}
		return &http.Response{StatusCode: http.StatusOK, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(""))}, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"storekit", "retention-messaging", "messages", "upload",
			"--message-id", "33333333-3333-4333-8333-333333333333",
			"--file", messagePath, "--environment", "sandbox", "--output", "json",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})
	if stderr != "" {
		t.Fatalf("stderr = %q", stderr)
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("invalid JSON output %q: %v", stdout, err)
	}
	if result["success"] != true || result["action"] != "uploaded" {
		t.Fatalf("result = %#v", result)
	}
}

func TestStoreKitPerformanceWaitReturnsErrorForFailedTest(t *testing.T) {
	setupStoreKitAuth(t)
	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet || req.URL.Path != "/inApps/v1/messaging/performanceTest/result/request-1" {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"result":"FAIL","successRate":90,"numPending":0}`)),
		}, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"storekit", "retention-messaging", "performance", "wait",
			"--request-id", "request-1", "--environment", "sandbox", "--output", "json",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})
	if runErr == nil || !strings.Contains(runErr.Error(), `performance test "request-1" failed`) {
		t.Fatalf("run error = %v", runErr)
	}
	if stderr != "" || !strings.Contains(stdout, `"result":"FAIL"`) {
		t.Fatalf("stdout=%q stderr=%q", stdout, stderr)
	}
}

func TestStoreKitInvalidValuesAreUsageErrors(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "environment",
			args:    []string{"storekit", "retention-messaging", "messages", "list", "--environment", "staging"},
			wantErr: "environment must be one of: production, sandbox",
		},
		{
			name:    "image size",
			args:    []string{"storekit", "retention-messaging", "images", "upload", "--image-id", "11111111-1111-4111-8111-111111111111", "--file", "unused.png", "--image-size", "small", "--environment", "sandbox"},
			wantErr: "--image-size must be one of: FULL_SIZE, BULLET_POINT",
		},
		{
			name:    "endpoint URL",
			args:    []string{"storekit", "retention-messaging", "endpoint", "set", "--url", "http://example.com", "--environment", "sandbox"},
			wantErr: "--url must be an absolute HTTPS URL",
		},
		{
			name:    "poll interval",
			args:    []string{"storekit", "retention-messaging", "performance", "wait", "--request-id", "request-1", "--interval", "1s", "--environment", "sandbox"},
			wantErr: "--interval must be at least 10s",
		},
		{
			name:    "performance production",
			args:    []string{"storekit", "retention-messaging", "performance", "view", "--request-id", "request-1", "--environment", "production"},
			wantErr: "performance tests require --environment sandbox",
		},
		{
			name:    "start interval without wait",
			args:    []string{"storekit", "retention-messaging", "performance", "start", "--original-transaction-id", "2000000000000000", "--interval", "20s", "--environment", "sandbox"},
			wantErr: "--interval and --timeout require --wait",
		},
		{
			name: "auth login environment without network",
			args: []string{
				"storekit", "auth", "login", "--name", "Retention", "--key-id", "KEY",
				"--issuer-id", "ISSUER", "--private-key", "unused.p8", "--bundle-id", "com.example.app",
				"--skip-validation", "--environment", "sandbox",
			},
			wantErr: "--environment requires --network",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)
			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(tt.args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				if err := root.Run(context.Background()); !errors.Is(err, flag.ErrHelp) {
					t.Fatalf("run error = %v, want flag.ErrHelp", err)
				}
			})
			if stdout != "" || !strings.Contains(stderr, tt.wantErr) {
				t.Fatalf("stdout=%q stderr=%q, want %q", stdout, stderr, tt.wantErr)
			}
		})
	}
}

func TestStoreKitBuiltBinaryUsageExitCode(t *testing.T) {
	t.Setenv("ASC_STOREKIT_ENVIRONMENT", "")
	t.Setenv("ASC_STOREKIT_KEY_ID", "")
	t.Setenv("ASC_STOREKIT_ISSUER_ID", "")

	assertUsageExit(
		t,
		[]string{"storekit", "retention-messaging", "messages", "list"},
		"--environment is required",
	)
}

func setupStoreKitAuth(t *testing.T) {
	t.Helper()
	keyPath := filepath.Join(t.TempDir(), "SubscriptionKey.p8")
	writeECDSAPEM(t, keyPath)
	t.Setenv("ASC_STOREKIT_KEY_ID", "STOREKIT_KEY")
	t.Setenv("ASC_STOREKIT_ISSUER_ID", "STOREKIT_ISSUER")
	t.Setenv("ASC_STOREKIT_PRIVATE_KEY_PATH", keyPath)
	t.Setenv("ASC_STOREKIT_PRIVATE_KEY", "")
	t.Setenv("ASC_STOREKIT_PRIVATE_KEY_B64", "")
	t.Setenv("ASC_STOREKIT_BUNDLE_ID", "com.example.app")
	t.Setenv("ASC_STOREKIT_PROFILE", "")
	t.Setenv("ASC_STOREKIT_STRICT_AUTH", "")
	t.Setenv("ASC_STOREKIT_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "missing-config.json"))
}
