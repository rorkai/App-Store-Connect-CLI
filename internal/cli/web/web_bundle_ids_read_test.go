package web

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"strings"
	"testing"

	"github.com/peterbourgon/ff/v3/ffcli"

	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

func TestWebBundleIDsCommandHierarchyIncludesReadLeavesFirst(t *testing.T) {
	command := WebBundleIDsCommand()
	want := []string{"list", "view", "capabilities"}
	if len(command.Subcommands) != len(want) {
		t.Fatalf("subcommands = %d, want %d", len(command.Subcommands), len(want))
	}
	for index, name := range want {
		if command.Subcommands[index].Name != name || command.Subcommands[index].UsageFunc == nil {
			t.Fatalf("subcommand %d = %+v, want %q with usage", index, command.Subcommands[index], name)
		}
	}
}

func TestWebBundleIDsReadValidationErrors(t *testing.T) {
	tests := []struct {
		name    string
		command func() *ffcli.Command
		args    []string
		wantErr string
	}{
		{name: "view missing bundle id", command: WebBundleIDsViewCommand, args: nil, wantErr: "--bundle-id is required"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			command := tc.command()
			if err := command.FlagSet.Parse(tc.args); err != nil {
				t.Fatalf("parse error: %v", err)
			}
			stdout, stderr := captureWebCommandOutput(t, func() {
				err := command.Exec(context.Background(), command.FlagSet.Args())
				if !errors.Is(err, flag.ErrHelp) {
					t.Fatalf("expected usage error, got %v", err)
				}
			})
			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, tc.wantErr) {
				t.Fatalf("stderr %q does not contain %q", stderr, tc.wantErr)
			}
		})
	}
}

func TestWebBundleIDsListPrintsJSON(t *testing.T) {
	restore := stubWebBundleIDReadDependencies(t)
	defer restore()

	listDeveloperBundleIDsFn = func(context.Context, *webcore.Client) (*webcore.DeveloperBundleIDsListResult, error) {
		return &webcore.DeveloperBundleIDsListResult{
			Data: []webcore.DeveloperBundleID{{
				ID:   "bundle-1",
				Type: "bundleIds",
				Attributes: map[string]any{
					"name":       "Example App",
					"identifier": "com.example.app",
					"platform":   "IOS",
					"wildcard":   false,
				},
			}},
			Raw: json.RawMessage(`{"data":[{"type":"bundleIds","id":"bundle-1","attributes":{"name":"Example App","identifier":"com.example.app","platform":"IOS","wildcard":false}}],"included":[],"links":{},"meta":{},"unknownTopLevel":{"keep":true}}`),
		}, nil
	}

	command := WebBundleIDsListCommand()
	if err := command.FlagSet.Parse([]string{"--output", "json"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	stdout, stderr := captureWebCommandOutput(t, func() {
		if err := command.Exec(context.Background(), nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	var result webcore.DeveloperBundleIDsListResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("decode JSON output %q: %v", stdout, err)
	}
	if len(result.Data) != 1 || result.Data[0].ID != "bundle-1" || result.Data[0].Attributes["identifier"] != "com.example.app" {
		t.Fatalf("unexpected list result: %+v", result)
	}
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal([]byte(stdout), &envelope); err != nil {
		t.Fatalf("decode raw JSON envelope %q: %v", stdout, err)
	}
	for _, member := range []string{"included", "links", "meta", "unknownTopLevel"} {
		if _, ok := envelope[member]; !ok {
			t.Fatalf("JSON output omitted envelope member %q: %s", member, stdout)
		}
	}
	if got := string(envelope["unknownTopLevel"]); got != `{"keep":true}` {
		t.Fatalf("unknown top-level member = %s, want {\"keep\":true}", got)
	}
}

func TestWebBundleIDsListWarnsWhenNextLinkExists(t *testing.T) {
	for _, output := range []string{"table", "markdown"} {
		t.Run(output, func(t *testing.T) {
			restore := stubWebBundleIDReadDependencies(t)
			defer restore()

			listDeveloperBundleIDsFn = func(context.Context, *webcore.Client) (*webcore.DeveloperBundleIDsListResult, error) {
				return &webcore.DeveloperBundleIDsListResult{
					Data: []webcore.DeveloperBundleID{{
						ID:   "bundle-1",
						Type: "bundleIds",
						Attributes: map[string]any{
							"name":       "Example App",
							"identifier": "com.example.app",
						},
					}},
					Links: map[string]any{"next": "/bundleIds?cursor=page-2"},
					Meta:  map[string]any{"paging": map[string]any{"total": float64(2)}},
				}, nil
			}

			command := WebBundleIDsListCommand()
			if err := command.FlagSet.Parse([]string{"--output", output}); err != nil {
				t.Fatalf("parse error: %v", err)
			}
			stdout, stderr := captureWebCommandOutput(t, func() {
				if err := command.Exec(context.Background(), nil); err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			})
			if stdout == "" {
				t.Fatalf("expected %s output", output)
			}
			want := "Warning: showing 1 of 2 results; more pages exist (use --paginate or --next where supported)\n"
			if stderr != want {
				t.Fatalf("stderr = %q, want %q", stderr, want)
			}
		})
	}
}

func TestWebBundleIDsViewPrintsTable(t *testing.T) {
	restore := stubWebBundleIDReadDependencies(t)
	defer restore()

	getDeveloperBundleIDFn = func(_ context.Context, _ *webcore.Client, bundleID string) (*webcore.DeveloperBundleIDGetResult, error) {
		return &webcore.DeveloperBundleIDGetResult{Data: webcore.DeveloperBundleID{
			ID:   bundleID,
			Type: "bundleIds",
			Attributes: map[string]any{
				"name":                "Example App",
				"identifier":          "com.example.app",
				"platform":            "IOS",
				"bundleType":          "STANDARD",
				"wildcard":            false,
				"seedId":              "TEAM123456",
				"~permissions.delete": true,
				"~permissions.edit":   false,
			},
		}}, nil
	}

	command := WebBundleIDsViewCommand()
	if err := command.FlagSet.Parse([]string{"--bundle-id", "bundle-1", "--output", "table"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	stdout, stderr := captureWebCommandOutput(t, func() {
		if err := command.Exec(context.Background(), nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	for _, want := range []string{"ID", "Name", "Identifier", "Platform", "Seed ID", "Wildcard", "Delete", "Edit", "Example App", "com.example.app", "IOS", "TEAM123456", "true", "false"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("table output %q does not contain %q", stdout, want)
		}
	}
	if strings.Contains(stdout, "Bundle Type") {
		t.Fatalf("detail table exposes list-only sparse fields: %q", stdout)
	}
}

func TestWebBundleIDsViewWarnsWhenFormattedOutputOmitsIncludedResources(t *testing.T) {
	for _, output := range []string{"table", "markdown"} {
		t.Run(output, func(t *testing.T) {
			restore := stubWebBundleIDReadDependencies(t)
			defer restore()

			getDeveloperBundleIDFn = func(_ context.Context, _ *webcore.Client, bundleID string) (*webcore.DeveloperBundleIDGetResult, error) {
				return &webcore.DeveloperBundleIDGetResult{
					Data: webcore.DeveloperBundleID{ID: bundleID, Type: "bundleIds"},
					Included: []webcore.DeveloperBundleID{{
						ID:   "capability-1",
						Type: "bundleIdCapabilities",
					}},
				}, nil
			}

			command := WebBundleIDsViewCommand()
			if err := command.FlagSet.Parse([]string{"--bundle-id", "bundle-1", "--output", output}); err != nil {
				t.Fatalf("parse error: %v", err)
			}
			_, stderr := captureWebCommandOutput(t, func() {
				if err := command.Exec(context.Background(), nil); err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			})
			want := developerBundleIDIncludedOutputWarning + "\n"
			if stderr != want {
				t.Fatalf("stderr = %q, want %q", stderr, want)
			}
		})
	}
}

func TestWebBundleIDsViewPrintsRawJSONEnvelope(t *testing.T) {
	restore := stubWebBundleIDReadDependencies(t)
	defer restore()

	getDeveloperBundleIDFn = func(_ context.Context, _ *webcore.Client, bundleID string) (*webcore.DeveloperBundleIDGetResult, error) {
		return &webcore.DeveloperBundleIDGetResult{
			Data: webcore.DeveloperBundleID{
				ID:   bundleID,
				Type: "bundleIds",
				Attributes: map[string]any{
					"name":       "Example App",
					"identifier": "com.example.app",
				},
			},
			Included: []webcore.DeveloperBundleID{{ID: "capability-1", Type: "bundleIdCapabilities"}},
			Raw:      json.RawMessage(`{"data":{"type":"bundleIds","id":"bundle-1","attributes":{"name":"Example App","identifier":"com.example.app"}},"included":[{"type":"bundleIdCapabilities","id":"capability-1"}],"links":{},"meta":{},"unknownTopLevel":[]}`),
		}, nil
	}

	command := WebBundleIDsViewCommand()
	if err := command.FlagSet.Parse([]string{"--bundle-id", "bundle-1", "--output", "json"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	stdout, stderr := captureWebCommandOutput(t, func() {
		if err := command.Exec(context.Background(), nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal([]byte(stdout), &envelope); err != nil {
		t.Fatalf("decode raw JSON envelope %q: %v", stdout, err)
	}
	for _, member := range []string{"included", "links", "meta", "unknownTopLevel"} {
		if _, ok := envelope[member]; !ok {
			t.Fatalf("JSON output omitted envelope member %q: %s", member, stdout)
		}
	}
	if got := string(envelope["unknownTopLevel"]); got != `[]` {
		t.Fatalf("unknown top-level member = %s, want []", got)
	}
	if !strings.Contains(string(envelope["included"]), "capability-1") {
		t.Fatalf("JSON output omitted included capability: %s", envelope["included"])
	}
}

func stubWebBundleIDReadDependencies(t *testing.T) func() {
	t.Helper()
	origResolveSession := resolveSessionFn
	origNewWebClient := newWebClientFn
	origList := listDeveloperBundleIDsFn
	origGet := getDeveloperBundleIDFn
	origPersist := persistWebSessionFn

	resolveSessionFn = func(context.Context, string, string, string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{}, "cache", nil
	}
	newWebClientFn = func(*webcore.AuthSession) *webcore.Client { return &webcore.Client{} }
	persistWebSessionFn = func(*webcore.AuthSession) error { return nil }

	restore := func() {
		resolveSessionFn = origResolveSession
		newWebClientFn = origNewWebClient
		listDeveloperBundleIDsFn = origList
		getDeveloperBundleIDFn = origGet
		persistWebSessionFn = origPersist
	}
	return restore
}
