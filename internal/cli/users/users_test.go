package users

import (
	"context"
	"flag"
	"testing"

	"github.com/peterbourgon/ff/v3/ffcli"
)

func TestUsersGetCommand_MissingID(t *testing.T) {
	cmd := UsersGetCommand()

	if err := cmd.FlagSet.Parse([]string{}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); err != flag.ErrHelp {
		t.Fatalf("expected flag.ErrHelp when --id is missing, got %v", err)
	}
}

func TestUsersUpdateCommand_MissingID(t *testing.T) {
	cmd := UsersUpdateCommand()

	if err := cmd.FlagSet.Parse([]string{"--roles", "ADMIN"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); err != flag.ErrHelp {
		t.Fatalf("expected flag.ErrHelp when --id is missing, got %v", err)
	}
}

func TestUsersUpdateCommand_MissingRoles(t *testing.T) {
	cmd := UsersUpdateCommand()

	if err := cmd.FlagSet.Parse([]string{"--id", "USER_ID"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); err != flag.ErrHelp {
		t.Fatalf("expected flag.ErrHelp when --roles is missing, got %v", err)
	}
}

func TestUsersDeleteCommand_MissingConfirm(t *testing.T) {
	cmd := UsersDeleteCommand()

	if err := cmd.FlagSet.Parse([]string{"--id", "USER_ID"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); err != flag.ErrHelp {
		t.Fatalf("expected flag.ErrHelp when --confirm is missing, got %v", err)
	}
}

func TestUsersDeleteCommand_MissingID(t *testing.T) {
	cmd := UsersDeleteCommand()

	if err := cmd.FlagSet.Parse([]string{"--confirm"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); err != flag.ErrHelp {
		t.Fatalf("expected flag.ErrHelp when --id is missing, got %v", err)
	}
}

func TestUsersInviteCommand_MissingEmail(t *testing.T) {
	cmd := UsersInviteCommand()

	if err := cmd.FlagSet.Parse([]string{"--first-name", "Jane", "--last-name", "Doe", "--roles", "ADMIN", "--all-apps"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); err != flag.ErrHelp {
		t.Fatalf("expected flag.ErrHelp when --email is missing, got %v", err)
	}
}

func TestUsersInviteCommand_MissingFirstName(t *testing.T) {
	cmd := UsersInviteCommand()

	if err := cmd.FlagSet.Parse([]string{"--email", "user@example.com", "--last-name", "Doe", "--roles", "ADMIN", "--all-apps"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); err != flag.ErrHelp {
		t.Fatalf("expected flag.ErrHelp when --first-name is missing, got %v", err)
	}
}

func TestUsersInviteCommand_MissingLastName(t *testing.T) {
	cmd := UsersInviteCommand()

	if err := cmd.FlagSet.Parse([]string{"--email", "user@example.com", "--first-name", "Jane", "--roles", "ADMIN", "--all-apps"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); err != flag.ErrHelp {
		t.Fatalf("expected flag.ErrHelp when --last-name is missing, got %v", err)
	}
}

func TestUsersInviteCommand_MissingRoles(t *testing.T) {
	cmd := UsersInviteCommand()

	if err := cmd.FlagSet.Parse([]string{"--email", "user@example.com", "--first-name", "Jane", "--last-name", "Doe", "--all-apps"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); err != flag.ErrHelp {
		t.Fatalf("expected flag.ErrHelp when --roles is missing, got %v", err)
	}
}

func TestUsersInviteCommand_MissingAccess(t *testing.T) {
	cmd := UsersInviteCommand()

	if err := cmd.FlagSet.Parse([]string{"--email", "user@example.com", "--first-name", "Jane", "--last-name", "Doe", "--roles", "ADMIN"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); err != flag.ErrHelp {
		t.Fatalf("expected flag.ErrHelp when --all-apps or --visible-app is missing, got %v", err)
	}
}

func TestUsersInviteCommand_ConflictingAccess(t *testing.T) {
	cmd := UsersInviteCommand()

	if err := cmd.FlagSet.Parse([]string{"--email", "user@example.com", "--first-name", "Jane", "--last-name", "Doe", "--roles", "ADMIN", "--all-apps", "--visible-app", "APP_ID"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	err := cmd.Exec(context.Background(), []string{})
	if err == nil {
		t.Fatal("expected error for conflicting access flags")
	}
	if err == flag.ErrHelp {
		// This is acceptable - the command shows help when there's a conflict
		return
	}
}

func TestUsersInvitesGetCommand_MissingID(t *testing.T) {
	cmd := UsersInvitesGetCommand()

	if err := cmd.FlagSet.Parse([]string{}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); err != flag.ErrHelp {
		t.Fatalf("expected flag.ErrHelp when --id is missing, got %v", err)
	}
}

func TestUsersInvitesRevokeCommand_MissingConfirm(t *testing.T) {
	cmd := UsersInvitesRevokeCommand()

	if err := cmd.FlagSet.Parse([]string{"--id", "INVITE_ID"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); err != flag.ErrHelp {
		t.Fatalf("expected flag.ErrHelp when --confirm is missing, got %v", err)
	}
}

func TestUsersInvitesRevokeCommand_MissingID(t *testing.T) {
	cmd := UsersInvitesRevokeCommand()

	if err := cmd.FlagSet.Parse([]string{"--confirm"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); err != flag.ErrHelp {
		t.Fatalf("expected flag.ErrHelp when --id is missing, got %v", err)
	}
}

func TestUsersVisibleAppsListCommand_MissingID(t *testing.T) {
	cmd := UsersVisibleAppsListCommand()

	if err := cmd.FlagSet.Parse([]string{}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); err != flag.ErrHelp {
		t.Fatalf("expected flag.ErrHelp when --id is missing, got %v", err)
	}
}

func TestUsersVisibleAppsGetCommand_MissingID(t *testing.T) {
	cmd := UsersVisibleAppsGetCommand()

	if err := cmd.FlagSet.Parse([]string{}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); err != flag.ErrHelp {
		t.Fatalf("expected flag.ErrHelp when --id is missing, got %v", err)
	}
}

func TestUsersCommands_DefaultOutputJSON(t *testing.T) {
	commands := []*struct {
		name string
		cmd  func() *ffcli.Command
	}{
		{"list", UsersListCommand},
		{"get", UsersGetCommand},
		{"update", UsersUpdateCommand},
		{"delete", UsersDeleteCommand},
		{"invite", UsersInviteCommand},
		{"invites list", UsersInvitesListCommand},
		{"invites get", UsersInvitesGetCommand},
		{"invites revoke", UsersInvitesRevokeCommand},
		{"visible-apps list", UsersVisibleAppsListCommand},
		{"visible-apps get", UsersVisibleAppsGetCommand},
	}

	for _, tc := range commands {
		t.Run(tc.name, func(t *testing.T) {
			cmd := tc.cmd()
			f := cmd.FlagSet.Lookup("output")
			if f == nil {
				t.Fatalf("expected --output flag to be defined")
			}
			if f.DefValue != "json" {
				t.Fatalf("expected --output default to be 'json', got %q", f.DefValue)
			}
		})
	}
}

func TestUsersListCommand_HasPaginationFlags(t *testing.T) {
	cmd := UsersListCommand()

	flags := []string{"limit", "next", "paginate"}
	for _, flagName := range flags {
		f := cmd.FlagSet.Lookup(flagName)
		if f == nil {
			t.Fatalf("expected --%s flag to be defined", flagName)
		}
	}
}

func TestUsersInvitesListCommand_HasPaginationFlags(t *testing.T) {
	cmd := UsersInvitesListCommand()

	flags := []string{"limit", "next", "paginate"}
	for _, flagName := range flags {
		f := cmd.FlagSet.Lookup(flagName)
		if f == nil {
			t.Fatalf("expected --%s flag to be defined", flagName)
		}
	}
}

func TestUsersListCommand_HasFilterFlags(t *testing.T) {
	cmd := UsersListCommand()

	flags := []string{"email", "role"}
	for _, flagName := range flags {
		f := cmd.FlagSet.Lookup(flagName)
		if f == nil {
			t.Fatalf("expected --%s flag to be defined", flagName)
		}
	}
}
