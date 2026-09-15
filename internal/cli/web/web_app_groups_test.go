package web

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

func TestWebAppGroupsCommandHierarchy(t *testing.T) {
	command := WebAppGroupsCommand()
	if command.Name != "app-groups" || command.UsageFunc == nil {
		t.Fatalf("unexpected command: %+v", command)
	}
	want := []string{"list", "create", "assign", "unassign", "set", "delete"}
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
		{name: "create invalid identifier", command: WebAppGroupsCreateCommand, args: []string{"--name", "Example", "--identifier", "com.example", "--confirm"}, wantErr: "must start with \"group.\""},
		{name: "create missing confirm", command: WebAppGroupsCreateCommand, args: []string{"--name", "Example", "--identifier", "group.com.example"}, wantErr: "--confirm is required"},
		{name: "assign missing group", command: WebAppGroupsAssignCommand, args: []string{"--bundle-id", "bundle-1", "--confirm"}, wantErr: "--group is required"},
		{name: "assign missing bundle", command: WebAppGroupsAssignCommand, args: []string{"--group", "group-1", "--confirm"}, wantErr: "--bundle-id is required"},
		{name: "assign missing confirm", command: WebAppGroupsAssignCommand, args: []string{"--group", "group-1", "--bundle-id", "bundle-1"}, wantErr: "--confirm is required"},
		{name: "unassign missing group", command: WebAppGroupsUnassignCommand, args: []string{"--bundle-id", "bundle-1", "--confirm"}, wantErr: "--group-id is required"},
		{name: "unassign missing bundle", command: WebAppGroupsUnassignCommand, args: []string{"--group-id", "group-1", "--confirm"}, wantErr: "--bundle-id is required"},
		{name: "unassign missing confirm", command: WebAppGroupsUnassignCommand, args: []string{"--group-id", "group-1", "--bundle-id", "bundle-1"}, wantErr: "--confirm is required"},
		{name: "unassign positional", command: WebAppGroupsUnassignCommand, args: []string{"--group-id", "group-1", "--bundle-id", "bundle-1", "--confirm", "extra"}, wantErr: "does not accept positional arguments"},
		{name: "set missing group", command: WebAppGroupsSetCommand, args: []string{"--bundle-id", "bundle-1", "--confirm"}, wantErr: "at least one --group is required"},
		{name: "set missing bundle", command: WebAppGroupsSetCommand, args: []string{"--group", "group-1", "--confirm"}, wantErr: "--bundle-id is required"},
		{name: "set missing confirm", command: WebAppGroupsSetCommand, args: []string{"--group", "group-1", "--bundle-id", "bundle-1"}, wantErr: "--confirm is required"},
		{name: "delete missing group", command: WebAppGroupsDeleteCommand, args: []string{"--confirm"}, wantErr: "--group-id is required"},
		{name: "delete missing confirm", command: WebAppGroupsDeleteCommand, args: []string{"--group-id", "group-1"}, wantErr: "--confirm is required"},
		{name: "delete positional", command: WebAppGroupsDeleteCommand, args: []string{"--group-id", "group-1", "--confirm", "extra"}, wantErr: "does not accept positional arguments"},
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

func TestWebAppGroupsCreateRejectsInvalidIdentifiersBeforeSessionResolution(t *testing.T) {
	restore, cleanup := stubWebAppGroupsDependencies(t)
	defer cleanup()
	defer restore()

	resolveCalls := 0
	restoreResolver := SetResolveWebSession(func(context.Context, string, string, string, string) (*webcore.AuthSession, string, error) {
		resolveCalls++
		return &webcore.AuthSession{}, "cache", nil
	})
	defer restoreResolver()

	tests := []struct {
		name       string
		identifier string
		wantErr    string
	}{
		{name: "empty suffix", identifier: "group.", wantErr: "--identifier must include a name after \"group.\""},
		{name: "invalid character", identifier: "group.com/example", wantErr: "--identifier may contain only letters, numbers, hyphens, and periods"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			command := WebAppGroupsCreateCommand()
			if err := command.FlagSet.Parse([]string{"--name", "Example", "--identifier", test.identifier, "--confirm"}); err != nil {
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
	if resolveCalls != 0 {
		t.Fatalf("web session resolver calls = %d, want 0", resolveCalls)
	}
}

func TestWebAppGroupsMutationsWarnAboutProvisioningProfileInvalidation(t *testing.T) {
	for _, build := range []func() *ffcli.Command{WebAppGroupsAssignCommand, WebAppGroupsUnassignCommand, WebAppGroupsSetCommand, WebAppGroupsDeleteCommand} {
		command := build()
		t.Run(command.Name, func(t *testing.T) {
			for _, expected := range []string{"invalidates existing provisioning profiles", "Regenerate affected profiles"} {
				if !strings.Contains(command.LongHelp, expected) {
					t.Fatalf("LongHelp does not contain %q: %q", expected, command.LongHelp)
				}
			}
			confirm := command.FlagSet.Lookup("confirm")
			if confirm == nil || !strings.Contains(confirm.Usage, "invalidates existing provisioning profiles") {
				t.Fatalf("--confirm usage does not warn about provisioning profile invalidation: %+v", confirm)
			}
		})
	}
}

func TestWebAppGroupsMutationsRequireConfirmBeforeSessionResolution(t *testing.T) {
	restore, cleanup := stubWebAppGroupsDependencies(t)
	defer cleanup()
	defer restore()

	resolveCalls := 0
	restoreResolver := SetResolveWebSession(func(context.Context, string, string, string, string) (*webcore.AuthSession, string, error) {
		resolveCalls++
		return &webcore.AuthSession{}, "cache", nil
	})
	defer restoreResolver()
	deleteDeveloperAppGroupFn = func(context.Context, *webcore.Client, webcore.DeveloperAppGroupDeleteRequest) (*asc.WebAppGroupDeleteResult, error) {
		t.Fatal("delete client must not be called without --confirm")
		return nil, nil
	}
	unassignDeveloperAppGroupFn = func(context.Context, *webcore.Client, webcore.DeveloperAppGroupUnassignRequest) (*asc.WebAppGroupUnassignResult, error) {
		t.Fatal("unassign client must not be called without --confirm")
		return nil, nil
	}
	setDeveloperAppGroupsFn = func(context.Context, *webcore.Client, webcore.DeveloperAppGroupSetRequest) (*asc.WebAppGroupSetResult, error) {
		t.Fatal("set client must not be called without --confirm")
		return nil, nil
	}

	tests := []struct {
		name    string
		command func() *ffcli.Command
		args    []string
	}{
		{name: "delete", command: WebAppGroupsDeleteCommand, args: []string{"--group-id", "GROUP1"}},
		{name: "unassign", command: WebAppGroupsUnassignCommand, args: []string{"--group-id", "GROUP1", "--bundle-id", "bundle-1"}},
		{name: "set", command: WebAppGroupsSetCommand, args: []string{"--group", "GROUP1", "--bundle-id", "bundle-1"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			command := test.command()
			if err := command.FlagSet.Parse(test.args); err != nil {
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
			if !strings.Contains(stderr, "--confirm is required") {
				t.Fatalf("stderr %q does not mention --confirm", stderr)
			}
		})
	}
	if resolveCalls != 0 {
		t.Fatalf("web session resolver calls = %d, want 0", resolveCalls)
	}
}

func TestWebAppGroupsDeleteFailsClosedWhenGroupIsAssigned(t *testing.T) {
	restore, cleanup := stubWebAppGroupsDependencies(t)
	defer cleanup()
	defer restore()

	persistCalls := 0
	persistWebSessionFn = func(*webcore.AuthSession) error {
		persistCalls++
		return nil
	}
	deleteDeveloperAppGroupFn = func(_ context.Context, _ *webcore.Client, request webcore.DeveloperAppGroupDeleteRequest) (*asc.WebAppGroupDeleteResult, error) {
		return nil, &webcore.DeveloperAppGroupInUseError{
			GroupID:    request.GroupID,
			Identifier: "group.com.example.shared",
			Assignments: []webcore.DeveloperAppGroupAssignment{
				{BundleID: "bundle-1", Identifier: "com.example.app", Name: "Example"},
				{BundleID: "bundle-2", Identifier: "com.example.widget", Name: "Widget"},
			},
		}
	}

	command := WebAppGroupsDeleteCommand()
	if err := command.FlagSet.Parse([]string{"--group-id", "GROUP1", "--confirm", "--output", "json"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	var runErr error
	stdout, stderr := captureWebCommandOutput(t, func() {
		runErr = command.Exec(context.Background(), nil)
	})
	if runErr == nil || errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("expected non-usage failure, got %v", runErr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	for _, expected := range []string{"GROUP1", "group.com.example.shared", "com.example.app (bundle-1)", "com.example.widget (bundle-2)", "unassign"} {
		if !strings.Contains(runErr.Error(), expected) {
			t.Fatalf("error %q does not mention %q", runErr.Error(), expected)
		}
	}
	if persistCalls != 0 {
		t.Fatalf("session persisted after failed delete: %d", persistCalls)
	}
	if strings.Contains(stderr, "invalidates existing provisioning profiles") {
		t.Fatalf("refused delete must not warn about a change: %q", stderr)
	}
}

func TestWebAppGroupsDeleteOutputsReceiptAndWarns(t *testing.T) {
	restore, cleanup := stubWebAppGroupsDependencies(t)
	defer cleanup()
	defer restore()

	persistCalls := 0
	persistWebSessionFn = func(*webcore.AuthSession) error {
		persistCalls++
		return nil
	}
	var deleteRequest webcore.DeveloperAppGroupDeleteRequest
	deleteDeveloperAppGroupFn = func(_ context.Context, _ *webcore.Client, request webcore.DeveloperAppGroupDeleteRequest) (*asc.WebAppGroupDeleteResult, error) {
		deleteRequest = request
		return &asc.WebAppGroupDeleteResult{GroupID: request.GroupID, Identifier: "group.com.example.shared", Name: "Shared", Deleted: true, Status: "deleted"}, nil
	}

	command := WebAppGroupsDeleteCommand()
	if err := command.FlagSet.Parse([]string{"--group-id", "GROUP1", "--confirm", "--output", "json"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	stdout, stderr := captureWebCommandOutput(t, func() {
		if err := command.Exec(context.Background(), nil); err != nil {
			t.Fatalf("Exec() error: %v", err)
		}
	})
	if deleteRequest.GroupID != "GROUP1" {
		t.Fatalf("unexpected delete request: %+v", deleteRequest)
	}
	var result asc.WebAppGroupDeleteResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("decode output: %v; stdout=%q", err, stdout)
	}
	if !result.Deleted || result.Status != "deleted" || result.Identifier != "group.com.example.shared" {
		t.Fatalf("unexpected receipt: %+v", result)
	}
	if !strings.Contains(stderr, "invalidates existing provisioning profiles") {
		t.Fatalf("stderr %q is missing the provisioning profile warning", stderr)
	}
	if persistCalls != 1 {
		t.Fatalf("session persist calls = %d, want 1", persistCalls)
	}

	tableCommand := WebAppGroupsDeleteCommand()
	if err := tableCommand.FlagSet.Parse([]string{"--group-id", "GROUP1", "--confirm", "--output", "table"}); err != nil {
		t.Fatalf("parse table: %v", err)
	}
	stdout, _ = captureWebCommandOutput(t, func() {
		if err := tableCommand.Exec(context.Background(), nil); err != nil {
			t.Fatalf("table Exec() error: %v", err)
		}
	})
	for _, expected := range []string{"Group ID", "Identifier", "Deleted", "Status", "GROUP1", "group.com.example.shared", "deleted"} {
		if !strings.Contains(stdout, expected) {
			t.Fatalf("table output %q does not contain %q", stdout, expected)
		}
	}
}

func TestWebAppGroupsSetReportsDiffAndNoOp(t *testing.T) {
	restore, cleanup := stubWebAppGroupsDependencies(t)
	defer cleanup()
	defer restore()

	var setRequest webcore.DeveloperAppGroupSetRequest
	setDeveloperAppGroupsFn = func(_ context.Context, _ *webcore.Client, request webcore.DeveloperAppGroupSetRequest) (*asc.WebAppGroupSetResult, error) {
		setRequest = request
		return &asc.WebAppGroupSetResult{BundleID: request.BundleID, GroupIDs: request.GroupIDs, Added: []string{"GROUP3"}, Removed: []string{"GROUP1"}, Changed: true, Status: "updated"}, nil
	}

	command := WebAppGroupsSetCommand()
	if err := command.FlagSet.Parse([]string{"--bundle-id", "bundle-1", "--group", "GROUP2", "--group", "GROUP3", "--confirm", "--output", "json"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	stdout, stderr := captureWebCommandOutput(t, func() {
		if err := command.Exec(context.Background(), nil); err != nil {
			t.Fatalf("Exec() error: %v", err)
		}
	})
	if setRequest.BundleID != "bundle-1" || strings.Join(setRequest.GroupIDs, ",") != "GROUP2,GROUP3" {
		t.Fatalf("unexpected set request: %+v", setRequest)
	}
	var result asc.WebAppGroupSetResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("decode output: %v; stdout=%q", err, stdout)
	}
	if !result.Changed || result.Status != "updated" || strings.Join(result.Added, ",") != "GROUP3" || strings.Join(result.Removed, ",") != "GROUP1" {
		t.Fatalf("unexpected receipt: %+v", result)
	}
	if !strings.Contains(stderr, "invalidates existing provisioning profiles") {
		t.Fatalf("stderr %q is missing the provisioning profile warning", stderr)
	}

	setDeveloperAppGroupsFn = func(_ context.Context, _ *webcore.Client, request webcore.DeveloperAppGroupSetRequest) (*asc.WebAppGroupSetResult, error) {
		return &asc.WebAppGroupSetResult{BundleID: request.BundleID, GroupIDs: request.GroupIDs, Added: []string{}, Removed: []string{}, Changed: false, Status: "unchanged"}, nil
	}
	noop := WebAppGroupsSetCommand()
	if err := noop.FlagSet.Parse([]string{"--bundle-id", "bundle-1", "--group", "GROUP2", "--confirm", "--output", "json"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	stdout, stderr = captureWebCommandOutput(t, func() {
		if err := noop.Exec(context.Background(), nil); err != nil {
			t.Fatalf("Exec() error: %v", err)
		}
	})
	if !strings.Contains(stdout, `"changed":false`) || !strings.Contains(stdout, `"status":"unchanged"`) {
		t.Fatalf("no-op receipt missing from stdout: %q", stdout)
	}
	if strings.Contains(stderr, "invalidates existing provisioning profiles") {
		t.Fatalf("no-op set must not warn about a change: %q", stderr)
	}

	tableCommand := WebAppGroupsSetCommand()
	if err := tableCommand.FlagSet.Parse([]string{"--bundle-id", "bundle-1", "--group", "GROUP2", "--confirm", "--output", "table"}); err != nil {
		t.Fatalf("parse table: %v", err)
	}
	stdout, _ = captureWebCommandOutput(t, func() {
		if err := tableCommand.Exec(context.Background(), nil); err != nil {
			t.Fatalf("table Exec() error: %v", err)
		}
	})
	for _, expected := range []string{"Bundle ID", "Group IDs", "Added", "Removed", "Changed", "Status", "bundle-1", "GROUP2", "unchanged"} {
		if !strings.Contains(stdout, expected) {
			t.Fatalf("table output %q does not contain %q", stdout, expected)
		}
	}
}

func TestWebAppGroupsUnassignCallsClient(t *testing.T) {
	restore, cleanup := stubWebAppGroupsDependencies(t)
	defer cleanup()
	defer restore()

	var unassignRequest webcore.DeveloperAppGroupUnassignRequest
	unassignDeveloperAppGroupFn = func(_ context.Context, _ *webcore.Client, request webcore.DeveloperAppGroupUnassignRequest) (*asc.WebAppGroupUnassignResult, error) {
		unassignRequest = request
		return &asc.WebAppGroupUnassignResult{BundleID: request.BundleID, GroupID: request.GroupID, RemainingGroupIDs: []string{"GROUP2"}, Changed: true, Status: "unassigned"}, nil
	}
	command := WebAppGroupsUnassignCommand()
	if err := command.FlagSet.Parse([]string{"--group-id", "GROUP1", "--bundle-id", "bundle-1", "--confirm", "--output", "json"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	stdout, stderr := captureWebCommandOutput(t, func() {
		if err := command.Exec(context.Background(), nil); err != nil {
			t.Fatalf("Exec() error: %v", err)
		}
	})
	if unassignRequest.GroupID != "GROUP1" || unassignRequest.BundleID != "bundle-1" {
		t.Fatalf("unexpected unassign request: %+v", unassignRequest)
	}
	var result asc.WebAppGroupUnassignResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("decode output: %v; stdout=%q", err, stdout)
	}
	if !result.Changed || result.Status != "unassigned" || strings.Join(result.RemainingGroupIDs, ",") != "GROUP2" {
		t.Fatalf("unexpected receipt: %+v", result)
	}
	if !strings.Contains(stderr, "invalidates existing provisioning profiles") {
		t.Fatalf("stderr %q is missing the provisioning profile warning", stderr)
	}
}

func TestWebAppGroupsAssignWarnsOnlyWhenChanged(t *testing.T) {
	for _, test := range []struct {
		name       string
		changed    bool
		status     string
		wantWarned bool
	}{
		{name: "assigned", changed: true, status: "assigned", wantWarned: true},
		{name: "already assigned", changed: false, status: "already-assigned", wantWarned: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			restore, cleanup := stubWebAppGroupsDependencies(t)
			defer cleanup()
			defer restore()
			assignDeveloperAppGroupFn = func(_ context.Context, _ *webcore.Client, request webcore.DeveloperAppGroupAssignRequest) (*webcore.DeveloperAppGroupAssignResult, error) {
				return &webcore.DeveloperAppGroupAssignResult{BundleID: request.BundleID, GroupID: request.GroupID, Changed: test.changed, Status: test.status}, nil
			}
			command := WebAppGroupsAssignCommand()
			if err := command.FlagSet.Parse([]string{"--group", "GROUP1", "--bundle-id", "bundle-1", "--confirm", "--output", "json"}); err != nil {
				t.Fatalf("parse error: %v", err)
			}
			_, stderr := captureWebCommandOutput(t, func() {
				if err := command.Exec(context.Background(), nil); err != nil {
					t.Fatalf("Exec() error: %v", err)
				}
			})
			if warned := strings.Contains(stderr, "invalidates existing provisioning profiles"); warned != test.wantWarned {
				t.Fatalf("stderr %q: warned=%t, want %t", stderr, warned, test.wantWarned)
			}
		})
	}
}

func TestWebAppGroupsMutationsWarnWhenAcceptedWriteCannotBeVerified(t *testing.T) {
	unverified := &webcore.DeveloperAppGroupUnverifiedError{Err: errors.New("verification read timed out")}
	tests := []struct {
		name  string
		build func() *ffcli.Command
		args  []string
		stub  func()
	}{
		{
			name: "assign", build: WebAppGroupsAssignCommand,
			args: []string{"--group", "GROUP1", "--bundle-id", "bundle-1", "--confirm"},
			stub: func() {
				assignDeveloperAppGroupFn = func(context.Context, *webcore.Client, webcore.DeveloperAppGroupAssignRequest) (*webcore.DeveloperAppGroupAssignResult, error) {
					return nil, unverified
				}
			},
		},
		{
			name: "unassign", build: WebAppGroupsUnassignCommand,
			args: []string{"--group-id", "GROUP1", "--bundle-id", "bundle-1", "--confirm"},
			stub: func() {
				unassignDeveloperAppGroupFn = func(context.Context, *webcore.Client, webcore.DeveloperAppGroupUnassignRequest) (*asc.WebAppGroupUnassignResult, error) {
					return nil, unverified
				}
			},
		},
		{
			name: "set", build: WebAppGroupsSetCommand,
			args: []string{"--group", "GROUP1", "--bundle-id", "bundle-1", "--confirm"},
			stub: func() {
				setDeveloperAppGroupsFn = func(context.Context, *webcore.Client, webcore.DeveloperAppGroupSetRequest) (*asc.WebAppGroupSetResult, error) {
					return nil, unverified
				}
			},
		},
		{
			name: "delete", build: WebAppGroupsDeleteCommand,
			args: []string{"--group-id", "GROUP1", "--confirm"},
			stub: func() {
				deleteDeveloperAppGroupFn = func(context.Context, *webcore.Client, webcore.DeveloperAppGroupDeleteRequest) (*asc.WebAppGroupDeleteResult, error) {
					return nil, unverified
				}
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			restore, cleanup := stubWebAppGroupsDependencies(t)
			defer cleanup()
			defer restore()
			test.stub()
			persistCalls := 0
			persistWebSessionFn = func(*webcore.AuthSession) error {
				persistCalls++
				return nil
			}
			command := test.build()
			if err := command.FlagSet.Parse(test.args); err != nil {
				t.Fatalf("parse error: %v", err)
			}
			var runErr error
			stdout, stderr := captureWebCommandOutput(t, func() {
				runErr = command.Exec(context.Background(), nil)
			})
			if runErr == nil || !strings.Contains(runErr.Error(), "verification read timed out") {
				t.Fatalf("expected the verification error to propagate, got %v", runErr)
			}
			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if persistCalls != 1 {
				t.Fatalf("persist calls = %d, want 1 so a later command without --developer-team still targets the team that accepted the write", persistCalls)
			}
			if !strings.Contains(stderr, "invalidates existing provisioning profiles") {
				t.Fatalf("stderr %q is missing the provisioning profile warning after an accepted but unverified write", stderr)
			}
		})
	}
}

func TestWebAppGroupsMutationsDoNotPersistOnRejectedWrites(t *testing.T) {
	restore, cleanup := stubWebAppGroupsDependencies(t)
	defer cleanup()
	defer restore()
	assignDeveloperAppGroupFn = func(context.Context, *webcore.Client, webcore.DeveloperAppGroupAssignRequest) (*webcore.DeveloperAppGroupAssignResult, error) {
		return nil, errors.New("portal rejected assign")
	}
	persistCalls := 0
	persistWebSessionFn = func(*webcore.AuthSession) error {
		persistCalls++
		return nil
	}
	command := WebAppGroupsAssignCommand()
	if err := command.FlagSet.Parse([]string{"--group", "GROUP1", "--bundle-id", "bundle-1", "--confirm"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	var runErr error
	_, _ = captureWebCommandOutput(t, func() {
		runErr = command.Exec(context.Background(), nil)
	})
	if runErr == nil || !strings.Contains(runErr.Error(), "portal rejected assign") {
		t.Fatalf("expected the rejected write to propagate, got %v", runErr)
	}
	if persistCalls != 0 {
		t.Fatalf("persist calls = %d, want 0 when the portal did not accept the write", persistCalls)
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

func TestWebAppGroupsCreatePersistsTeamWhenPortalResponseIsAmbiguous(t *testing.T) {
	restore, cleanup := stubWebAppGroupsDependencies(t)
	defer cleanup()
	defer restore()
	createDeveloperAppGroupFn = func(context.Context, *webcore.Client, webcore.DeveloperAppGroupCreateRequest) (*webcore.DeveloperAppGroup, error) {
		return nil, errors.New("failed to parse Developer Portal App Group create response")
	}
	persistCalls := 0
	persistWebSessionFn = func(*webcore.AuthSession) error {
		persistCalls++
		return nil
	}
	command := WebAppGroupsCreateCommand()
	if err := command.FlagSet.Parse([]string{"--name", "Example Preview", "--identifier", "group.com.example.preview", "--confirm"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	var runErr error
	_, _ = captureWebCommandOutput(t, func() {
		runErr = command.Exec(context.Background(), nil)
	})
	if runErr == nil || !strings.Contains(runErr.Error(), "parse Developer Portal App Group create response") {
		t.Fatalf("expected the ambiguous create error to propagate, got %v", runErr)
	}
	if persistCalls != 1 {
		t.Fatalf("persist calls = %d, want 1 so a later list without --developer-team still targets the team that may have registered the group", persistCalls)
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
	originalUnassign := unassignDeveloperAppGroupFn
	originalSet := setDeveloperAppGroupsFn
	originalDelete := deleteDeveloperAppGroupFn
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
		unassignDeveloperAppGroupFn = originalUnassign
		setDeveloperAppGroupsFn = originalSet
		deleteDeveloperAppGroupFn = originalDelete
		persistWebSessionFn = originalPersist
	}, cleanupSession
}

// TestWebAppGroupsAssignConvertsAppGroupIdentifierToUsageError proves the
// identifier-for-resource-ID mistake exits with the usage contract and names
// the flag, instead of an unclassified runtime failure.
func TestWebAppGroupsAssignConvertsAppGroupIdentifierToUsageError(t *testing.T) {
	restore, cleanup := stubWebAppGroupsDependencies(t)
	defer cleanup()
	defer restore()
	assignDeveloperAppGroupFn = func(context.Context, *webcore.Client, webcore.DeveloperAppGroupAssignRequest) (*webcore.DeveloperAppGroupAssignResult, error) {
		return nil, &webcore.DeveloperAppGroupIdentifierError{Given: "group.com.example.shared", GroupID: "GROUP12345", Identifier: "group.com.example.shared"}
	}
	command := WebAppGroupsAssignCommand()
	if err := command.FlagSet.Parse([]string{"--group", "group.com.example.shared", "--bundle-id", "bundle-1", "--confirm"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	var runErr error
	stdout, stderr := captureWebCommandOutput(t, func() {
		runErr = command.Exec(context.Background(), nil)
	})
	if runErr == nil {
		t.Fatal("expected an error")
	}
	if !errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("error %v does not follow the usage-error contract", runErr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "GROUP12345") || !strings.Contains(stderr, "asc web app-groups list") {
		t.Fatalf("stderr %q does not tell the operator which resource ID to use", stderr)
	}
	diagnostic, ok := shared.DiagnosticFromError(runErr)
	if !ok || diagnostic.Code != shared.DiagnosticInvalidInput || diagnostic.Parameter != "--group" {
		t.Fatalf("diagnostic = %+v (found=%t), want invalid_input for --group", diagnostic, ok)
	}
}

// TestWebAppGroupsAssignClassifiesEveryFailurePath keeps every assign failure
// out of the unclassified internal_error bucket in telemetry.
func TestWebAppGroupsAssignClassifiesEveryFailurePath(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		result   *webcore.DeveloperAppGroupAssignResult
		wantCode shared.DiagnosticCode
	}{
		{name: "unreadable portal response", err: &webcore.DeveloperAppGroupUnreadableResponseError{Err: errors.New("cannot safely update Bundle ID \"bundle-1\"")}, wantCode: shared.DiagnosticDependencyFailed},
		{name: "accepted but unverified", err: &webcore.DeveloperAppGroupUnverifiedError{Err: errors.New("verification failed")}, wantCode: shared.DiagnosticStateNotReady},
		{name: "unauthorized", err: &webcore.APIError{Status: 401}, wantCode: shared.DiagnosticAuthenticationRejected},
		{name: "missing resource", err: &webcore.APIError{Status: 404}, wantCode: shared.DiagnosticResourceNotFound},
		{name: "conflict", err: &webcore.APIError{Status: 409}, wantCode: shared.DiagnosticResourceConflict},
		{name: "portal server error", err: &webcore.APIError{Status: 503}, wantCode: shared.DiagnosticRequestFailed},
		{name: "portal refusal envelope", err: &webcore.DeveloperPortalResultError{ResultCode: 1100, Message: "Access denied"}, wantCode: shared.DiagnosticRequestFailed},
		{name: "deadline exceeded", err: fmt.Errorf("write failed: %w", context.DeadlineExceeded), wantCode: shared.DiagnosticRequestFailed},
		{name: "dropped connection", err: fmt.Errorf("write failed: %w", io.EOF), wantCode: shared.DiagnosticRequestFailed},
		{name: "unclassified portal failure", err: errors.New("portal rejected assign"), wantCode: shared.DiagnosticInternalError},
		{name: "missing receipt", result: nil, wantCode: shared.DiagnosticInternalError},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			restore, cleanup := stubWebAppGroupsDependencies(t)
			defer cleanup()
			defer restore()
			assignDeveloperAppGroupFn = func(context.Context, *webcore.Client, webcore.DeveloperAppGroupAssignRequest) (*webcore.DeveloperAppGroupAssignResult, error) {
				return test.result, test.err
			}
			command := WebAppGroupsAssignCommand()
			if err := command.FlagSet.Parse([]string{"--group", "GROUP1", "--bundle-id", "bundle-1", "--confirm"}); err != nil {
				t.Fatalf("parse error: %v", err)
			}
			var runErr error
			_, _ = captureWebCommandOutput(t, func() {
				runErr = command.Exec(context.Background(), nil)
			})
			if runErr == nil {
				t.Fatal("expected an error")
			}
			diagnostic, ok := shared.DiagnosticFromError(runErr)
			if !ok {
				t.Fatalf("error %v carries no diagnostic code", runErr)
			}
			if diagnostic.Code != test.wantCode {
				t.Fatalf("diagnostic code = %q, want %q", diagnostic.Code, test.wantCode)
			}
		})
	}
}

// TestWebAppGroupsClassifiesOperatorFailures keeps routine operator failures
// out of the internal_error bucket and preserves a diagnostic the validator
// already attached.
func TestWebAppGroupsClassifiesOperatorFailures(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		wantCode  shared.DiagnosticCode
		wantParam string
	}{
		{
			name:      "validator diagnostic is preserved",
			err:       shared.WithDiagnostic(errors.New("--apple-id is unsupported"), shared.DiagnosticConflictingInput, "--apple-id"),
			wantCode:  shared.DiagnosticConflictingInput,
			wantParam: "--apple-id",
		},
		{
			name:     "missing team selection",
			err:      fmt.Errorf("%w; run 'asc web auth login'", webcore.ErrDeveloperPortalTeamNotSelected),
			wantCode: shared.DiagnosticAuthenticationRejected,
		},
		{
			name:     "missing App Group",
			err:      &webcore.DeveloperAppGroupNotFoundError{GroupID: "GROUP1"},
			wantCode: shared.DiagnosticResourceNotFound,
		},
		{
			name:     "group still assigned",
			err:      &webcore.DeveloperAppGroupInUseError{GroupID: "GROUP1", Assignments: []webcore.DeveloperAppGroupAssignment{{BundleID: "bundle-1"}}},
			wantCode: shared.DiagnosticResourceConflict,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			restore, cleanup := stubWebAppGroupsDependencies(t)
			defer cleanup()
			defer restore()
			deleteDeveloperAppGroupFn = func(context.Context, *webcore.Client, webcore.DeveloperAppGroupDeleteRequest) (*asc.WebAppGroupDeleteResult, error) {
				return nil, test.err
			}
			command := WebAppGroupsDeleteCommand()
			if err := command.FlagSet.Parse([]string{"--group-id", "GROUP1", "--confirm"}); err != nil {
				t.Fatalf("parse error: %v", err)
			}
			var runErr error
			_, _ = captureWebCommandOutput(t, func() {
				runErr = command.Exec(context.Background(), nil)
			})
			if runErr == nil {
				t.Fatal("expected an error")
			}
			diagnostic, ok := shared.DiagnosticFromError(runErr)
			if !ok {
				t.Fatalf("error %v carries no diagnostic code", runErr)
			}
			if diagnostic.Code != test.wantCode || diagnostic.Parameter != test.wantParam {
				t.Fatalf("diagnostic = %+v, want code %q parameter %q", diagnostic, test.wantCode, test.wantParam)
			}
		})
	}
}

// TestWebAppGroupsPreservesUsageErrorClassification keeps a usage failure from
// session resolution on exit code 2 with an input diagnostic.
func TestWebAppGroupsPreservesUsageErrorClassification(t *testing.T) {
	restore, cleanup := stubWebAppGroupsDependencies(t)
	defer cleanup()
	defer restore()
	assignDeveloperAppGroupFn = func(context.Context, *webcore.Client, webcore.DeveloperAppGroupAssignRequest) (*webcore.DeveloperAppGroupAssignResult, error) {
		return nil, shared.NewReportedUsageError(shared.UsageErrorInvalidValue, "ASC_WEB_SESSION is unset or empty")
	}
	command := WebAppGroupsAssignCommand()
	if err := command.FlagSet.Parse([]string{"--group", "GROUP1", "--bundle-id", "bundle-1", "--confirm"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	var runErr error
	_, _ = captureWebCommandOutput(t, func() {
		runErr = command.Exec(context.Background(), nil)
	})
	if runErr == nil {
		t.Fatal("expected an error")
	}
	if kind := shared.ClassifyUsageError(runErr); kind != shared.UsageErrorInvalidValue {
		t.Fatalf("usage kind = %q, want %q", kind, shared.UsageErrorInvalidValue)
	}
	diagnostic, ok := shared.DiagnosticFromError(runErr)
	if !ok || diagnostic.Code != shared.DiagnosticInvalidInput {
		t.Fatalf("diagnostic = %+v (found=%t), want invalid_input", diagnostic, ok)
	}
}
