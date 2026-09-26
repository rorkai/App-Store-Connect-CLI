package web

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"strings"
	"testing"

	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

func TestWebICloudContainersCommandHierarchy(t *testing.T) {
	command := WebICloudContainersCommand()
	if command.Name != "icloud-containers" || command.UsageFunc == nil {
		t.Fatalf("unexpected command: %+v", command)
	}
	if len(command.Subcommands) != 2 || command.Subcommands[0].Name != "list" || command.Subcommands[1].Name != "create" {
		t.Fatalf("subcommands = %+v, want list and create", command.Subcommands)
	}
	if command.Subcommands[0].UsageFunc == nil || command.Subcommands[1].UsageFunc == nil {
		t.Fatal("iCloud container subcommands must set UsageFunc")
	}
	if command.Subcommands[0].FlagSet.Lookup("paginate") != nil {
		t.Fatal("iCloud container list must not advertise --paginate")
	}

	root := WebCommand()
	var found bool
	for _, subcommand := range root.Subcommands {
		if subcommand.Name == "icloud-containers" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("web command did not register icloud-containers")
	}
}

func TestWebICloudContainersCreateRefusesBeforeAnyRequest(t *testing.T) {
	command := WebICloudContainersCreateCommand()
	if err := command.FlagSet.Parse([]string{
		"--identifier", "iCloud.com.example.app",
		"--name", "Example",
		"--confirm",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	stdout, stderr := captureWebCommandOutput(t, func() {
		err := command.Exec(context.Background(), command.FlagSet.Args())
		if err == nil || errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected a non-usage refusal, got %v", err)
		}
		if !strings.Contains(err.Error(), "no accepted write request has been captured") {
			t.Fatalf("error = %v", err)
		}
	})
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	if !strings.Contains(stderr, "no accepted write request has been captured") {
		t.Fatalf("stderr = %q", stderr)
	}
}

func TestWebICloudContainersCreateRequiresConfirm(t *testing.T) {
	command := WebICloudContainersCreateCommand()
	if err := command.FlagSet.Parse([]string{
		"--identifier", "iCloud.com.example.app",
		"--name", "Example",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	stdout, stderr := captureWebCommandOutput(t, func() {
		err := command.Exec(context.Background(), command.FlagSet.Args())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected usage error, got %v", err)
		}
	})
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	if !strings.Contains(stderr, "--confirm is required") {
		t.Fatalf("stderr = %q", stderr)
	}
}

func TestWebICloudContainersListValidationErrors(t *testing.T) {
	command := WebICloudContainersListCommand()
	if err := command.FlagSet.Parse([]string{"extra"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	stdout, stderr := captureWebCommandOutput(t, func() {
		err := command.Exec(context.Background(), command.FlagSet.Args())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected usage error, got %v", err)
		}
	})
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	if !strings.Contains(stderr, "web icloud-containers list does not accept positional arguments") {
		t.Fatalf("stderr = %q, want positional argument error", stderr)
	}
}

func TestWebICloudContainersListPrintsJSONAndPassesHidden(t *testing.T) {
	restore := stubWebICloudContainerReadDependencies(t)
	defer restore()

	var gotHidden bool
	listDeveloperICloudContainersFn = func(_ context.Context, _ *webcore.Client, hidden bool) (*webcore.DeveloperICloudContainersListResult, error) {
		gotHidden = hidden
		return &webcore.DeveloperICloudContainersListResult{
			Data: []webcore.DeveloperICloudContainer{{
				ID:   "cloud-1",
				Type: "cloudContainers",
				Attributes: webcore.DeveloperICloudContainerAttributes{
					Identifier: "iCloud.com.example.app",
					Name:       "Example Container",
					Prefix:     "TEAM123456",
					CanEdit:    true,
					CanDelete:  false,
				},
			}},
			Raw: json.RawMessage(`{"data":[{"type":"cloudContainers","id":"cloud-1","attributes":{"identifier":"iCloud.com.example.app","name":"Example Container"}}],"links":{},"meta":{},"unknownTopLevel":{"keep":true}}`),
		}, nil
	}

	command := WebICloudContainersListCommand()
	if err := command.FlagSet.Parse([]string{"--hidden", "--output", "json"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	stdout, stderr := captureWebCommandOutput(t, func() {
		if err := command.Exec(context.Background(), nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}
	if !gotHidden {
		t.Fatal("--hidden was not passed to list operation")
	}
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal([]byte(stdout), &envelope); err != nil {
		t.Fatalf("decode JSON output %q: %v", stdout, err)
	}
	if got := string(envelope["unknownTopLevel"]); got != `{"keep":true}` {
		t.Fatalf("unknown top-level member = %s, want {\"keep\":true}", got)
	}
}

func TestWebICloudContainersListPrintsMeaningfulTable(t *testing.T) {
	restore := stubWebICloudContainerReadDependencies(t)
	defer restore()

	listDeveloperICloudContainersFn = func(_ context.Context, _ *webcore.Client, hidden bool) (*webcore.DeveloperICloudContainersListResult, error) {
		if hidden {
			t.Fatal("default hidden flag = true, want false")
		}
		return &webcore.DeveloperICloudContainersListResult{Data: []webcore.DeveloperICloudContainer{{
			ID:   "cloud-1",
			Type: "cloudContainers",
			Attributes: webcore.DeveloperICloudContainerAttributes{
				Identifier: "iCloud.com.example.app",
				Name:       "Example Container",
				Prefix:     "TEAM123456",
				Hidden:     false,
				CanEdit:    true,
				CanDelete:  false,
				ResponseID: "response-1",
			},
		}}}, nil
	}

	command := WebICloudContainersListCommand()
	if err := command.FlagSet.Parse([]string{"--output", "table"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	stdout, stderr := captureWebCommandOutput(t, func() {
		if err := command.Exec(context.Background(), nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}
	for _, want := range []string{"ID", "Name", "Identifier", "Prefix", "Hidden", "Can Edit", "Can Delete", "Response ID", "cloud-1", "Example Container", "iCloud.com.example.app", "TEAM123456", "response-1"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("table output %q does not contain %q", stdout, want)
		}
	}
}

func stubWebICloudContainerReadDependencies(t *testing.T) func() {
	t.Helper()
	origResolveSession := resolveSessionFn
	origNewWebClient := newWebClientFn
	origList := listDeveloperICloudContainersFn
	origPersist := persistWebSessionFn

	resolveSessionFn = func(context.Context, string, string, string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{}, "cache", nil
	}
	newWebClientFn = func(*webcore.AuthSession) *webcore.Client { return &webcore.Client{} }
	persistWebSessionFn = func(*webcore.AuthSession) error { return nil }

	return func() {
		resolveSessionFn = origResolveSession
		newWebClientFn = origNewWebClient
		listDeveloperICloudContainersFn = origList
		persistWebSessionFn = origPersist
	}
}

func TestWebICloudContainersListWarnsAboutIncompletePages(t *testing.T) {
	for _, format := range []string{"table", "markdown", "json"} {
		for _, next := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/next=%t", format, next), func(t *testing.T) {
				restore := stubWebICloudContainerReadDependencies(t)
				defer restore()
				listDeveloperICloudContainersFn = func(context.Context, *webcore.Client, bool) (*webcore.DeveloperICloudContainersListResult, error) {
					links := map[string]any{}
					if next {
						links["next"] = map[string]any{"href": "https://developer.apple.com/next"}
					}
					return &webcore.DeveloperICloudContainersListResult{
						Data:  []webcore.DeveloperICloudContainer{{ID: "cloud-1", Type: "cloudContainers"}},
						Links: links, Meta: map[string]any{"paging": map[string]any{"total": 1001, "limit": 1000}},
					}, nil
				}
				command := WebICloudContainersListCommand()
				if err := command.FlagSet.Parse([]string{"--output", format}); err != nil {
					t.Fatal(err)
				}
				_, stderr := captureWebCommandOutput(t, func() {
					if err := command.Exec(context.Background(), nil); err != nil {
						t.Fatal(err)
					}
				})
				if strings.Count(stderr, "Warning:") != 1 || !strings.Contains(stderr, "showing 1 of 1001 results") {
					t.Fatalf("stderr = %q, want one incomplete-page warning", stderr)
				}
			})
		}
	}
}
