package cmdtest

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	subscriptionscli "github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/subscriptions"
)

func useSubscriptionGroupVersionTestServer(t *testing.T, handler http.HandlerFunc) {
	t.Helper()
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	serverURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("parse test server URL: %v", err)
	}
	transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		cloned := req.Clone(req.Context())
		cloned.URL.Scheme = serverURL.Scheme
		cloned.URL.Host = serverURL.Host
		return server.Client().Transport.RoundTrip(cloned)
	})
	client, err := asc.NewClientWithHTTPClient(
		"TEST_KEY",
		"TEST_ISSUER",
		os.Getenv("ASC_PRIVATE_KEY_PATH"),
		&http.Client{Transport: transport},
	)
	if err != nil {
		t.Fatalf("new test client: %v", err)
	}
	restore := subscriptionscli.SetGroupVersionClientFactory(func() (*asc.Client, error) {
		return client, nil
	})
	t.Cleanup(restore)
}

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
		{"list rejects group owner with next", []string{"subscriptions", "groups", "versions", "list", "--group-id", "group-2", "--next", "https://api.appstoreconnect.apple.com/v1/subscriptionGroups/group-1/versions?cursor=next"}, "--next cannot be combined with --group-id"},
		{"list rejects query flags with next", []string{"subscriptions", "groups", "versions", "list", "--next", "https://api.appstoreconnect.apple.com/v1/subscriptionGroups/group-1/versions?cursor=next", "--state", "READY_FOR_REVIEW"}, "--next cannot be combined with query flags"},
		{"view requires version", []string{"subscriptions", "groups", "versions", "view"}, "--version-id is required"},
		{"localizations list requires version", []string{"subscriptions", "groups", "versions", "localizations", "list"}, "--version-id is required"},
		{"localizations list rejects version owner with next", []string{"subscriptions", "groups", "versions", "localizations", "list", "--version-id", "version-2", "--next", "https://api.appstoreconnect.apple.com/v1/subscriptionGroupVersions/version-1/localizations?cursor=next"}, "--next cannot be combined with --version-id"},
		{"localizations list rejects query flags with next", []string{"subscriptions", "groups", "versions", "localizations", "list", "--next", "https://api.appstoreconnect.apple.com/v1/subscriptionGroupVersions/version-1/localizations?cursor=next", "--include", "version"}, "--next cannot be combined with query flags"},
		{"localization create requires version", []string{"subscriptions", "groups", "versions", "localizations", "create", "--name", "Premium", "--locale", "en-US"}, "--version-id is required"},
		{"localization view requires id", []string{"subscriptions", "groups", "versions", "localizations", "view"}, "--id is required"},
		{"localization update requires a change", []string{"subscriptions", "groups", "versions", "localizations", "update", "--id", "loc-1"}, "at least one update flag is required"},
		{"localization update rejects set and clear", []string{"subscriptions", "groups", "versions", "localizations", "update", "--id", "loc-1", "--name", "Premium", "--clear-name"}, "--name cannot be used with --clear-name"},
		{"localization delete requires confirm", []string{"subscriptions", "groups", "versions", "localizations", "delete", "--id", "loc-1"}, "--confirm is required"},
		{"links versions requires group", []string{"subscriptions", "groups", "versions", "links", "versions"}, "--group-id is required"},
		{"links localizations requires version", []string{"subscriptions", "groups", "versions", "links", "localizations"}, "--version-id is required"},
		{"links versions reject group owner with next", []string{"subscriptions", "groups", "versions", "links", "versions", "--group-id", "group-2", "--next", "https://api.appstoreconnect.apple.com/v1/subscriptionGroups/group-1/relationships/versions?cursor=next"}, "--next cannot be combined with --group-id"},
		{"links localizations reject version owner with next", []string{"subscriptions", "groups", "versions", "links", "localizations", "--version-id", "version-2", "--next", "https://api.appstoreconnect.apple.com/v1/subscriptionGroupVersions/version-1/relationships/localizations?cursor=next"}, "--next cannot be combined with --version-id"},
		{"links reject limit with next", []string{"subscriptions", "groups", "versions", "links", "versions", "--next", "https://api.appstoreconnect.apple.com/v1/subscriptionGroups/group-1/relationships/versions?cursor=next", "--limit", "5"}, "--next cannot be combined with --limit"},
		{"create rejects positional args", []string{"subscriptions", "groups", "versions", "create", "--group-id", "group-1", "unexpected"}, "unexpected argument"},
		{"list rejects positional args", []string{"subscriptions", "groups", "versions", "list", "--group-id", "group-1", "unexpected"}, "unexpected argument"},
		{"view rejects positional args", []string{"subscriptions", "groups", "versions", "view", "--version-id", "version-1", "unexpected"}, "unexpected argument"},
		{"localization list rejects positional args", []string{"subscriptions", "groups", "versions", "localizations", "list", "--version-id", "version-1", "unexpected"}, "unexpected argument"},
		{"localization create rejects positional args", []string{"subscriptions", "groups", "versions", "localizations", "create", "--version-id", "version-1", "--name", "Premium", "--locale", "en-US", "unexpected"}, "unexpected argument"},
		{"localization view rejects positional args", []string{"subscriptions", "groups", "versions", "localizations", "view", "--id", "loc-1", "unexpected"}, "unexpected argument"},
		{"localization update rejects positional args", []string{"subscriptions", "groups", "versions", "localizations", "update", "--id", "loc-1", "--name", "Premium", "unexpected"}, "unexpected argument"},
		{"localization delete rejects positional args", []string{"subscriptions", "groups", "versions", "localizations", "delete", "--id", "loc-1", "--confirm", "unexpected"}, "unexpected argument"},
		{"versions links reject positional args", []string{"subscriptions", "groups", "versions", "links", "versions", "--group-id", "group-1", "unexpected"}, "unexpected argument"},
		{"localizations links reject positional args", []string{"subscriptions", "groups", "versions", "links", "localizations", "--version-id", "version-1", "unexpected"}, "unexpected argument"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			clientFactoryCalled := false
			restore := subscriptionscli.SetGroupVersionClientFactory(func() (*asc.Client, error) {
				clientFactoryCalled = true
				return nil, errors.New("client factory must not run during validation")
			})
			defer restore()

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
			if clientFactoryCalled {
				t.Fatal("expected validation to fail before creating an authenticated client")
			}
		})
	}
}

func TestSubscriptionGroupsListRejectsVersionQueryFlagsWithNext(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	_, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"subscriptions", "groups", "list", "--next", "https://api.appstoreconnect.apple.com/v1/apps/app-1/subscriptionGroups?cursor=next", "--include", "versions"}); err != nil {
			t.Fatal(err)
		}
		err := root.Run(context.Background())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})
	if !strings.Contains(stderr, "--next cannot be combined with query flags") {
		t.Fatalf("unexpected stderr: %q", stderr)
	}
}

func TestSubscriptionGroupsListRejectsExplicitAppWithOpaqueNextBeforeClient(t *testing.T) {
	const next = "https://api.appstoreconnect.apple.com/v1/apps/app-1/subscriptionGroups?cursor=next"
	tests := []struct {
		name string
		args []string
	}{
		{name: "nonempty app", args: []string{"--app", "app-2"}},
		{name: "explicit empty app", args: []string{"--app", ""}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			clientFactoryCalled := false
			restore := subscriptionscli.SetGroupVersionClientFactory(func() (*asc.Client, error) {
				clientFactoryCalled = true
				return nil, errors.New("client factory must not run during validation")
			})
			defer restore()

			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)
			args := append([]string{"subscriptions", "groups", "list", "--next", next}, test.args...)
			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(args); err != nil {
					t.Fatal(err)
				}
				err := root.Run(context.Background())
				if !errors.Is(err, flag.ErrHelp) {
					t.Fatalf("expected usage error, got %v", err)
				}
			})
			if stdout != "" {
				t.Fatalf("stdout = %q", stdout)
			}
			if !strings.Contains(stderr, "--next cannot be combined with --app") {
				t.Fatalf("stderr = %q", stderr)
			}
			if clientFactoryCalled {
				t.Fatal("client factory ran before opaque next validation")
			}
		})
	}
}

func TestSubscriptionGroupVersionLinksExposeOnlyApplicableOwnerFlag(t *testing.T) {
	tests := []struct {
		name           string
		requiredFlag   string
		irrelevantFlag string
	}{
		{
			name:           "versions",
			requiredFlag:   "group-id",
			irrelevantFlag: "version-id",
		},
		{
			name:           "localizations",
			requiredFlag:   "version-id",
			irrelevantFlag: "group-id",
		},
	}

	links := subscriptionscli.SubscriptionsGroupsVersionLinksCommand()
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var commandFound bool
			for _, command := range links.Subcommands {
				if command.Name != test.name {
					continue
				}
				commandFound = true
				if command.FlagSet.Lookup(test.requiredFlag) == nil {
					t.Fatalf("missing required owner flag --%s", test.requiredFlag)
				}
				if command.FlagSet.Lookup(test.irrelevantFlag) != nil {
					t.Fatalf("irrelevant owner flag --%s is exposed", test.irrelevantFlag)
				}
			}
			if !commandFound {
				t.Fatalf("missing links subcommand %q", test.name)
			}
		})
	}
}

func TestSubscriptionGroupVersionsCreateUsesRequiredRelationship(t *testing.T) {
	useSubscriptionGroupVersionTestServer(t, func(w http.ResponseWriter, req *http.Request) {
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
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = io.WriteString(w, `{"data":{"type":"subscriptionGroupVersions","id":"version-1","attributes":{"version":1,"state":"PREPARE_FOR_SUBMISSION"}}}`)
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
	useSubscriptionGroupVersionTestServer(t, func(w http.ResponseWriter, req *http.Request) {
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
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"data":{"type":"subscriptionGroupLocalizations","id":"loc-1","attributes":{"name":"Premium","locale":"en-US"}}}`)
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
	useSubscriptionGroupVersionTestServer(t, func(w http.ResponseWriter, req *http.Request) {
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
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = io.WriteString(w, `{"data":{"type":"subscriptionGroupLocalizations","id":"loc-1","attributes":{"name":"Premium","locale":"en-US"}}}`)
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
