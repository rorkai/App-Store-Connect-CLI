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
)

func TestSubscriptionGroupVersionsValidationErrors(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{"create requires group", []string{"subscriptions", "groups", "versions", "create"}, "--group-id is required"},
		{"list requires group", []string{"subscriptions", "groups", "versions", "list"}, "--group-id is required"},
		{"list validates state", []string{"subscriptions", "groups", "versions", "list", "--group-id", "group-1", "--state", "NOPE"}, "--state must be one of"},
		{"list validates include", []string{"subscriptions", "groups", "versions", "list", "--group-id", "group-1", "--include", "subscriptions"}, "--include must be one of"},
		{"view requires version", []string{"subscriptions", "groups", "versions", "view"}, "--version-id is required"},
		{"localizations list requires version", []string{"subscriptions", "groups", "versions", "localizations", "list"}, "--version-id is required"},
		{"localization create requires version", []string{"subscriptions", "groups", "versions", "localizations", "create", "--name", "Premium", "--locale", "en-US"}, "--version-id is required"},
		{"localization view requires id", []string{"subscriptions", "groups", "versions", "localizations", "view"}, "--id is required"},
		{"localization update requires a change", []string{"subscriptions", "groups", "versions", "localizations", "update", "--id", "loc-1"}, "at least one update flag is required"},
		{"localization update rejects set and clear", []string{"subscriptions", "groups", "versions", "localizations", "update", "--id", "loc-1", "--name", "Premium", "--clear-name"}, "--name cannot be used with --clear-name"},
		{"localization delete requires confirm", []string{"subscriptions", "groups", "versions", "localizations", "delete", "--id", "loc-1"}, "--confirm is required"},
		{"links versions requires group", []string{"subscriptions", "groups", "versions", "links", "versions"}, "--group-id is required"},
		{"links localizations requires version", []string{"subscriptions", "groups", "versions", "links", "localizations"}, "--version-id is required"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)
			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(test.args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				err := root.Run(context.Background())
				if !errors.Is(err, flag.ErrHelp) {
					t.Fatalf("expected ErrHelp, got %v", err)
				}
			})
			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, test.wantErr) {
				t.Fatalf("expected stderr to contain %q, got %q", test.wantErr, stderr)
			}
		})
	}
}

func TestSubscriptionGroupVersionsCreateUsesRequiredRelationship(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodPost || req.URL.Path != "/v1/subscriptionGroupVersions" {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL)
		}
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(body), `"subscriptionGroup":{"data":{"type":"subscriptionGroups","id":"group-1"}}`) {
			t.Fatalf("missing group relationship: %s", body)
		}
		return &http.Response{StatusCode: http.StatusCreated, Body: io.NopCloser(strings.NewReader(`{"data":{"type":"subscriptionGroupVersions","id":"version-1","attributes":{"version":1,"state":"PREPARE_FOR_SUBMISSION"}}}`)), Header: http.Header{"Content-Type": []string{"application/json"}}}, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"subscriptions", "groups", "versions", "create", "--group-id", "group-1", "--output", "json"}); err != nil {
			t.Fatal(err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatal(err)
		}
	})
	if stderr != "" {
		t.Fatalf("unexpected stderr: %s", stderr)
	}
	var response map[string]any
	if err := json.Unmarshal([]byte(stdout), &response); err != nil {
		t.Fatalf("invalid JSON output %q: %v", stdout, err)
	}
	data, _ := response["data"].(map[string]any)
	if data["id"] != "version-1" {
		t.Fatalf("unexpected output: %s", stdout)
	}
}

func TestSubscriptionGroupVersionLocalizationUpdateSendsExplicitNull(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodPatch || req.URL.Path != "/v2/subscriptionGroupLocalizations/loc-1" {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL)
		}
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(body), `"customAppName":null`) {
			t.Fatalf("missing explicit null: %s", body)
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"data":{"type":"subscriptionGroupLocalizations","id":"loc-1","attributes":{"name":"Premium","locale":"en-US"}}}`)), Header: http.Header{"Content-Type": []string{"application/json"}}}, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	_, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"subscriptions", "groups", "versions", "localizations", "update", "--id", "loc-1", "--clear-custom-app-name", "--output", "json"}); err != nil {
			t.Fatal(err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatal(err)
		}
	})
	if stderr != "" {
		t.Fatalf("unexpected stderr: %s", stderr)
	}
}

func TestSubscriptionGroupLegacyLocalizationCreateRemainsV1(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodPost || req.URL.Path != "/v1/subscriptionGroupLocalizations" {
			t.Fatalf("legacy command changed endpoint: %s %s", req.Method, req.URL)
		}
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(body), `"subscriptionGroup":{"data":{"type":"subscriptionGroups","id":"group-1"}}`) || strings.Contains(string(body), `"version"`) {
			t.Fatalf("legacy command changed relationship: %s", body)
		}
		return &http.Response{StatusCode: http.StatusCreated, Body: io.NopCloser(strings.NewReader(`{"data":{"type":"subscriptionGroupLocalizations","id":"loc-1","attributes":{"name":"Premium","locale":"en-US"}}}`)), Header: http.Header{"Content-Type": []string{"application/json"}}}, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	_, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"subscriptions", "groups", "localizations", "create", "--group-id", "group-1", "--name", "Premium", "--locale", "en-US", "--output", "json"}); err != nil {
			t.Fatal(err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatal(err)
		}
	})
	if stderr != "" {
		t.Fatalf("unexpected stderr: %s", stderr)
	}
}

func TestSubscriptionGroupLegacyLocalizationCommandsRemainGroupScoped(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	if err := root.Parse([]string{"subscriptions", "groups", "localizations", "create"}); err != nil {
		t.Fatal(err)
	}
	_, stderr := captureOutput(t, func() {
		err := root.Run(context.Background())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})
	if !strings.Contains(stderr, "--group-id is required") || strings.Contains(stderr, "--version-id") {
		t.Fatalf("legacy command changed ownership semantics: %q", stderr)
	}
}
