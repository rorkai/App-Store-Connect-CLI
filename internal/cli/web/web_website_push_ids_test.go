package web

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

func TestWebWebsitePushIDsCommandHierarchy(t *testing.T) {
	command := WebWebsitePushIDsCommand()
	if command.Name != "website-push-ids" || command.UsageFunc == nil {
		t.Fatalf("unexpected command: %+v", command)
	}
	want := []string{"list", "view", "create", "delete"}
	if len(command.Subcommands) != len(want) {
		t.Fatalf("subcommands = %+v, want %v", command.Subcommands, want)
	}
	for index, name := range want {
		if command.Subcommands[index].Name != name {
			t.Fatalf("subcommand %d = %q, want %q", index, command.Subcommands[index].Name, name)
		}
	}
}

func TestWebWebsitePushIDsListPrintsCapturedJSON(t *testing.T) {
	restore := stubWebWebsitePushIDsDependencies(t)
	defer restore()

	listDeveloperWebsitePushIDsFn = func(context.Context, *webcore.Client) (*webcore.DeveloperWebsitePushIDsListResult, error) {
		return &webcore.DeveloperWebsitePushIDsListResult{
			WebsitePushIDList: []webcore.DeveloperWebsitePushID{{
				"websitePushId": "web.example.com",
				"name":          "Example Website",
			}},
			Raw: json.RawMessage(`{"resultCode":0,"pageNumber":1,"pageSize":1000,"websitePushIdList":[{"websitePushId":"web.example.com","name":"Example Website"}],"providerExtension":{"keep":true}}`),
		}, nil
	}

	command := WebWebsitePushIDsListCommand()
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
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal([]byte(stdout), &envelope); err != nil {
		t.Fatalf("decode JSON output %q: %v", stdout, err)
	}
	if _, ok := envelope["websitePushIdList"]; !ok {
		t.Fatalf("JSON output omitted top-level websitePushIdList: %s", stdout)
	}
	if got := string(envelope["providerExtension"]); got != `{"keep":true}` {
		t.Fatalf("providerExtension = %s, want {\"keep\":true}", got)
	}
}

func TestWebWebsitePushIDsListPrintsTableProjection(t *testing.T) {
	restore := stubWebWebsitePushIDsDependencies(t)
	defer restore()

	listDeveloperWebsitePushIDsFn = func(context.Context, *webcore.Client) (*webcore.DeveloperWebsitePushIDsListResult, error) {
		return &webcore.DeveloperWebsitePushIDsListResult{
			WebsitePushIDList: []webcore.DeveloperWebsitePushID{{
				"websitePushId": "web.example.com",
				"name":          "Example Website",
				"identifier":    "web.example.com",
			}},
		}, nil
	}

	command := WebWebsitePushIDsListCommand()
	if err := command.FlagSet.Parse([]string{"--output", "table"}); err != nil {
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
	for _, want := range []string{"Website Push ID", "Name", "Identifier", "web.example.com", "Example Website"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("table output %q does not contain %q", stdout, want)
		}
	}
}

func stubWebWebsitePushIDsDependencies(t *testing.T) func() {
	t.Helper()
	origResolveSession := resolveSessionFn
	origNewWebClient := newWebClientFn
	origPersist := persistWebSessionFn
	origList := listDeveloperWebsitePushIDsFn

	resolveSessionFn = func(context.Context, string, string, string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{}, "cache", nil
	}
	newWebClientFn = func(*webcore.AuthSession) *webcore.Client { return &webcore.Client{} }
	persistWebSessionFn = func(*webcore.AuthSession) error { return nil }

	return func() {
		resolveSessionFn = origResolveSession
		newWebClientFn = origNewWebClient
		persistWebSessionFn = origPersist
		listDeveloperWebsitePushIDsFn = origList
	}
}
