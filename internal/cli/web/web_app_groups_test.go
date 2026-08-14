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

func TestWebAppGroupsCommandHierarchy(t *testing.T) {
	command := WebAppGroupsCommand()
	if command.Name != "app-groups" || command.UsageFunc == nil {
		t.Fatalf("unexpected command: %+v", command)
	}
	want := []string{"list", "create", "assign"}
	if len(command.Subcommands) != len(want) {
		t.Fatalf("subcommands = %+v", command.Subcommands)
	}
	for index, name := range want {
		if command.Subcommands[index].Name != name || command.Subcommands[index].UsageFunc == nil {
			t.Fatalf("subcommand %d = %+v", index, command.Subcommands[index])
		}
	}
}

func TestWebAppGroupsValidationErrors(t *testing.T) {
	tests := []struct {
		name    string
		command func() *ffcli.Command
		args    []string
		wantErr string
	}{
		{name: "list positional", command: WebAppGroupsListCommand, args: []string{"extra"}, wantErr: "does not accept positional arguments"},
		{name: "create missing name", command: WebAppGroupsCreateCommand, args: []string{"--identifier", "group.com.example", "--confirm"}, wantErr: "--name is required"},
		{name: "create invalid identifier", command: WebAppGroupsCreateCommand, args: []string{"--name", "Example", "--identifier", "com.example", "--confirm"}, wantErr: "must start with group."},
		{name: "create missing confirm", command: WebAppGroupsCreateCommand, args: []string{"--name", "Example", "--identifier", "group.com.example"}, wantErr: "--confirm is required"},
		{name: "assign missing group", command: WebAppGroupsAssignCommand, args: []string{"--bundle-id", "bundle-1", "--confirm"}, wantErr: "--group is required"},
		{name: "assign missing bundle", command: WebAppGroupsAssignCommand, args: []string{"--group", "group-1", "--confirm"}, wantErr: "--bundle-id is required"},
		{name: "assign missing confirm", command: WebAppGroupsAssignCommand, args: []string{"--group", "group-1", "--bundle-id", "bundle-1"}, wantErr: "--confirm is required"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			command := test.command()
			if err := command.FlagSet.Parse(test.args); err != nil {
				t.Fatalf("parse error: %v", err)
			}
			_, stderr := captureWebCommandOutput(t, func() {
				err := command.Exec(context.Background(), command.FlagSet.Args())
				if !errors.Is(err, flag.ErrHelp) {
					t.Fatalf("expected usage error, got %v", err)
				}
			})
			if !strings.Contains(stderr, test.wantErr) {
				t.Fatalf("stderr %q does not contain %q", stderr, test.wantErr)
			}
		})
	}
}

func TestWebAppGroupsListOutputsJSON(t *testing.T) {
	restore, cleanup := stubWebAppGroupsDependencies(t)
	defer cleanup()
	listDeveloperAppGroupsFn = func(context.Context, *webcore.Client, webcore.DeveloperAppGroupsListOptions) (*webcore.DeveloperAppGroupsListResult, error) {
		return &webcore.DeveloperAppGroupsListResult{Data: []webcore.DeveloperAppGroup{{ID: "GROUP1", Name: "Shared", Identifier: "group.com.example.shared"}}}, nil
	}
	defer restore()

	command := WebAppGroupsListCommand()
	if err := command.FlagSet.Parse([]string{"--output", "json"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	stdout, stderr := captureWebCommandOutput(t, func() {
		if err := command.Exec(context.Background(), nil); err != nil {
			t.Fatalf("Exec() error: %v", err)
		}
	})
	if stderr != "" {
		t.Fatalf("unexpected stderr: %q", stderr)
	}
	var result webcore.DeveloperAppGroupsListResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("decode output: %v; stdout=%q", err, stdout)
	}
	if len(result.Data) != 1 || result.Data[0].ID != "GROUP1" {
		t.Fatalf("unexpected output: %+v", result)
	}

	tableCommand := WebAppGroupsListCommand()
	if err := tableCommand.FlagSet.Parse([]string{"--output", "table"}); err != nil {
		t.Fatalf("parse table: %v", err)
	}
	stdout, _ = captureWebCommandOutput(t, func() {
		if err := tableCommand.Exec(context.Background(), nil); err != nil {
			t.Fatalf("table Exec() error: %v", err)
		}
	})
	for _, expected := range []string{"ID", "Name", "Identifier", "Status", "GROUP1", "Shared", "group.com.example.shared"} {
		if !strings.Contains(stdout, expected) {
			t.Fatalf("table output %q does not contain %q", stdout, expected)
		}
	}
}

func TestWebAppGroupsListPropagatesPagination(t *testing.T) {
	restore, cleanup := stubWebAppGroupsDependencies(t)
	defer cleanup()
	defer restore()

	var received webcore.DeveloperAppGroupsListOptions
	listDeveloperAppGroupsFn = func(_ context.Context, _ *webcore.Client, options webcore.DeveloperAppGroupsListOptions) (*webcore.DeveloperAppGroupsListResult, error) {
		received = options
		return &webcore.DeveloperAppGroupsListResult{Data: []webcore.DeveloperAppGroup{}}, nil
	}
	command := WebAppGroupsListCommand()
	if err := command.FlagSet.Parse([]string{"--paginate", "--output", "json"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	_, _ = captureWebCommandOutput(t, func() {
		if err := command.Exec(context.Background(), nil); err != nil {
			t.Fatalf("Exec() error: %v", err)
		}
	})
	if !received.Paginate {
		t.Fatal("Paginate = false, want true")
	}
}

func TestWebAppGroupsWarnsWhenRefreshedSessionCannotBePersisted(t *testing.T) {
	restore, cleanup := stubWebAppGroupsDependencies(t)
	defer cleanup()
	defer restore()

	listDeveloperAppGroupsFn = func(context.Context, *webcore.Client, webcore.DeveloperAppGroupsListOptions) (*webcore.DeveloperAppGroupsListResult, error) {
		return &webcore.DeveloperAppGroupsListResult{Data: []webcore.DeveloperAppGroup{{ID: "GROUP1", Name: "Shared", Identifier: "group.com.example.shared"}}}, nil
	}
	persistWebSessionFn = func(*webcore.AuthSession) error { return errors.New("disk full") }
	command := WebAppGroupsListCommand()
	if err := command.FlagSet.Parse([]string{"--output", "json"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	stdout, stderr := captureWebCommandOutput(t, func() {
		if err := command.Exec(context.Background(), nil); err != nil {
			t.Fatalf("Exec() error: %v", err)
		}
	})
	if !strings.Contains(stdout, `"id":"GROUP1"`) {
		t.Fatalf("successful output missing from stdout: %q", stdout)
	}
	if !strings.Contains(stderr, "failed to persist refreshed web session") || !strings.Contains(stderr, "disk full") {
		t.Fatalf("persistence warning missing from stderr: %q", stderr)
	}
}

func TestWebAppGroupsCreateAndAssignCallClient(t *testing.T) {
	restore, cleanup := stubWebAppGroupsDependencies(t)
	defer cleanup()
	defer restore()

	var createRequest webcore.DeveloperAppGroupCreateRequest
	createDeveloperAppGroupFn = func(_ context.Context, _ *webcore.Client, request webcore.DeveloperAppGroupCreateRequest) (*webcore.DeveloperAppGroup, error) {
		createRequest = request
		return &webcore.DeveloperAppGroup{ID: "GROUP1", Name: request.Name, Identifier: request.Identifier}, nil
	}
	create := WebAppGroupsCreateCommand()
	if err := create.FlagSet.Parse([]string{"--name", "Example Preview", "--identifier", "group.com.example.preview", "--confirm", "--output", "json"}); err != nil {
		t.Fatalf("parse create: %v", err)
	}
	stdout, _ := captureWebCommandOutput(t, func() {
		if err := create.Exec(context.Background(), nil); err != nil {
			t.Fatalf("create Exec() error: %v", err)
		}
	})
	if createRequest.Name != "Example Preview" || createRequest.Identifier != "group.com.example.preview" || !strings.Contains(stdout, `"id":"GROUP1"`) {
		t.Fatalf("unexpected create request/output: %+v %q", createRequest, stdout)
	}

	var assignRequest webcore.DeveloperAppGroupAssignRequest
	assignDeveloperAppGroupFn = func(_ context.Context, _ *webcore.Client, request webcore.DeveloperAppGroupAssignRequest) (*webcore.DeveloperAppGroupAssignResult, error) {
		assignRequest = request
		return &webcore.DeveloperAppGroupAssignResult{BundleID: request.BundleID, GroupID: request.GroupID, Changed: true, Status: "assigned"}, nil
	}
	assign := WebAppGroupsAssignCommand()
	if err := assign.FlagSet.Parse([]string{"--group", "GROUP1", "--bundle-id", "bundle-1", "--confirm", "--output", "json"}); err != nil {
		t.Fatalf("parse assign: %v", err)
	}
	stdout, _ = captureWebCommandOutput(t, func() {
		if err := assign.Exec(context.Background(), nil); err != nil {
			t.Fatalf("assign Exec() error: %v", err)
		}
	})
	if assignRequest.GroupID != "GROUP1" || assignRequest.BundleID != "bundle-1" || !strings.Contains(stdout, `"status":"assigned"`) {
		t.Fatalf("unexpected assign request/output: %+v %q", assignRequest, stdout)
	}
}

func stubWebAppGroupsDependencies(t *testing.T) (restoreFunctions func(), cleanupSession func()) {
	t.Helper()
	originalNewClient := newWebClientFn
	originalList := listDeveloperAppGroupsFn
	originalCreate := createDeveloperAppGroupFn
	originalAssign := assignDeveloperAppGroupFn
	originalPersist := persistWebSessionFn
	cleanupSession = SetResolveWebSession(func(context.Context, string, string, string, string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{}, "cache", nil
	})
	newWebClientFn = func(*webcore.AuthSession) *webcore.Client { return &webcore.Client{} }
	persistWebSessionFn = func(*webcore.AuthSession) error { return nil }
	return func() {
		newWebClientFn = originalNewClient
		listDeveloperAppGroupsFn = originalList
		createDeveloperAppGroupFn = originalCreate
		assignDeveloperAppGroupFn = originalAssign
		persistWebSessionFn = originalPersist
	}, cleanupSession
}
