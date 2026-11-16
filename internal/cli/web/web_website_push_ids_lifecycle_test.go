package web

import (
	"context"
	"errors"
	"flag"
	"strings"
	"testing"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

func TestWebWebsitePushIDsUnverifiedWritesPersistSelectedTeam(t *testing.T) {
	for _, operation := range []string{"create", "delete"} {
		t.Run(operation, func(t *testing.T) {
			restore := stubWebWebsitePushIDsDependencies(t)
			defer restore()
			originalCreate, originalDelete := createDeveloperWebsitePushIDFn, deleteDeveloperWebsitePushIDFn
			t.Cleanup(func() {
				createDeveloperWebsitePushIDFn, deleteDeveloperWebsitePushIDFn = originalCreate, originalDelete
			})
			session := &webcore.AuthSession{DeveloperTeamID: "OLDTEAM123"}
			resolveSessionFn = func(context.Context, string, string, string) (*webcore.AuthSession, string, error) {
				return session, "cache", nil
			}
			cause := errors.New("verification read timed out")
			unverified := &webcore.DeveloperWebsitePushIDUnverifiedError{Err: cause}
			var mutations, persisted int
			var savedTeam string
			persistWebSessionFn = func(current *webcore.AuthSession) error {
				persisted++
				savedTeam = current.DeveloperTeamID
				return nil
			}
			mutate := func() {
				mutations++
				// Portal bootstrap updates this shared session after resolving
				// the explicit team, before the mutation's verification fails.
				session.DeveloperTeamID = "NEWTEAM456"
			}
			createDeveloperWebsitePushIDFn = func(context.Context, *webcore.Client, webcore.DeveloperWebsitePushIDCreateRequest) (*asc.WebWebsitePushIDMutationResult, error) {
				mutate()
				return nil, unverified
			}
			deleteDeveloperWebsitePushIDFn = func(context.Context, *webcore.Client, webcore.DeveloperWebsitePushIDDeleteRequest) (*asc.WebWebsitePushIDMutationResult, error) {
				mutate()
				return nil, unverified
			}
			d, depsStdout, depsStderr := newWebTestDeps()
			command := newWebWebsitePushIDsCreateCommand(d)
			args := []string{"--name", "Example", "--identifier", "web.example.com"}
			if operation == "delete" {
				command = newWebWebsitePushIDsDeleteCommand(d)
				args = []string{"--website-push-id", "RESOURCE1"}
			}
			args = append(args, "--developer-team", "NEWTEAM456", "--confirm", "--output", "json")
			if err := command.FlagSet.Parse(args); err != nil {
				t.Fatal(err)
			}
			stdout, _ := runWithWebTestDeps(depsStdout, depsStderr, func() {
				if err := command.Exec(context.Background(), nil); !errors.Is(err, cause) {
					t.Errorf("Exec() error = %v, want preserved uncertainty cause", err)
				}
			})
			if mutations != 1 || persisted != 1 || savedTeam != "NEWTEAM456" || stdout != "" {
				t.Fatalf("mutations=%d persisted=%d savedTeam=%q stdout=%q; want one failed write, selected-team persistence, and no success receipt", mutations, persisted, savedTeam, stdout)
			}
		})
	}
}

func TestWebWebsitePushIDsCommandHierarchyIncludesCapturedLifecycle(t *testing.T) {
	d, _, _ := newWebTestDeps()
	command := newWebWebsitePushIDsCommand(d)
	want := []string{"list", "view", "create", "delete"}
	if len(command.Subcommands) != len(want) {
		t.Fatalf("subcommands = %+v, want %v", command.Subcommands, want)
	}
	for index, name := range want {
		if command.Subcommands[index].Name != name || command.Subcommands[index].UsageFunc == nil {
			t.Fatalf("subcommand %d = %+v", index, command.Subcommands[index])
		}
	}
}

func TestWebWebsitePushIDsLifecycleValidation(t *testing.T) {
	tests := []struct {
		name    string
		command func(*shared.Deps) *ffcli.Command
		args    []string
		wantErr string
	}{
		{name: "view missing id", command: newWebWebsitePushIDsViewCommand, wantErr: "--website-push-id is required"},
		{name: "create missing name", command: newWebWebsitePushIDsCreateCommand, args: []string{"--identifier", "web.example.com", "--confirm"}, wantErr: "--name is required"},
		{name: "create missing identifier", command: newWebWebsitePushIDsCreateCommand, args: []string{"--name", "Example", "--confirm"}, wantErr: "--identifier is required"},
		{name: "create invalid identifier", command: newWebWebsitePushIDsCreateCommand, args: []string{"--name", "Example", "--identifier", "web/example", "--confirm"}, wantErr: "--identifier"},
		{name: "create missing confirm", command: newWebWebsitePushIDsCreateCommand, args: []string{"--name", "Example", "--identifier", "web.example.com"}, wantErr: "--confirm is required"},
		{name: "delete missing id", command: newWebWebsitePushIDsDeleteCommand, args: []string{"--confirm"}, wantErr: "--website-push-id is required"},
		{name: "delete missing confirm", command: newWebWebsitePushIDsDeleteCommand, args: []string{"--website-push-id", "resource-1"}, wantErr: "--confirm is required"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			d, depsStdout, depsStderr := newWebTestDeps()
			command := test.command(d)
			if err := command.FlagSet.Parse(test.args); err != nil {
				t.Fatalf("parse error: %v", err)
			}
			_, stderr := runWithWebTestDeps(depsStdout, depsStderr, func() {
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
