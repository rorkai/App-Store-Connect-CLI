package users

import (
	"bytes"
	"testing"

	"github.com/peterbourgon/ff/v3/ffcli"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func TestExtractUserIDFromNextURL(t *testing.T) {
	next := "https://api.appstoreconnect.apple.com/v1/users/user-123/visibleApps?cursor=abc"
	got, err := extractUserIDFromNextURL(next)
	if err != nil {
		t.Fatalf("extractUserIDFromNextURL() error: %v", err)
	}
	if got != "user-123" {
		t.Fatalf("expected user-123, got %q", got)
	}
}

func TestExtractUserIDFromNextURLRelationships(t *testing.T) {
	next := "https://api.appstoreconnect.apple.com/v1/users/user-123/relationships/visibleApps?cursor=abc"
	got, err := extractUserIDFromNextURL(next)
	if err != nil {
		t.Fatalf("extractUserIDFromNextURL() error: %v", err)
	}
	if got != "user-123" {
		t.Fatalf("expected user-123, got %q", got)
	}
}

func TestExtractUserIDFromNextURL_Invalid(t *testing.T) {
	_, err := extractUserIDFromNextURL("https://api.appstoreconnect.apple.com/v1/users")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestExtractUserIDFromNextURL_RejectsMalformedHost(t *testing.T) {
	tests := []string{
		"http://localhost:80:80/v1/users/user-123/visibleApps?cursor=abc",
		"http://::1/v1/users/user-123/visibleApps?cursor=abc",
	}

	for _, next := range tests {
		t.Run(next, func(t *testing.T) {
			if _, err := extractUserIDFromNextURL(next); err == nil {
				t.Fatalf("expected error for malformed URL %q", next)
			}
		})
	}
}

func TestExtractUserInvitationIDFromNextURL(t *testing.T) {
	next := "https://api.appstoreconnect.apple.com/v1/userInvitations/invite-123/visibleApps?cursor=abc"
	got, err := extractUserInvitationIDFromNextURL(next)
	if err != nil {
		t.Fatalf("extractUserInvitationIDFromNextURL() error: %v", err)
	}
	if got != "invite-123" {
		t.Fatalf("expected invite-123, got %q", got)
	}
}

func TestExtractUserInvitationIDFromNextURLRelationships(t *testing.T) {
	next := "https://api.appstoreconnect.apple.com/v1/userInvitations/invite-123/relationships/visibleApps?cursor=abc"
	got, err := extractUserInvitationIDFromNextURL(next)
	if err != nil {
		t.Fatalf("extractUserInvitationIDFromNextURL() error: %v", err)
	}
	if got != "invite-123" {
		t.Fatalf("expected invite-123, got %q", got)
	}
}

func TestExtractUserInvitationIDFromNextURL_Invalid(t *testing.T) {
	_, err := extractUserInvitationIDFromNextURL("https://api.appstoreconnect.apple.com/v1/userInvitations")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestExtractUserInvitationIDFromNextURL_RejectsMalformedHost(t *testing.T) {
	tests := []string{
		"http://localhost:80:80/v1/userInvitations/invite-123/visibleApps?cursor=abc",
		"http://::1/v1/userInvitations/invite-123/visibleApps?cursor=abc",
	}

	for _, next := range tests {
		t.Run(next, func(t *testing.T) {
			if _, err := extractUserInvitationIDFromNextURL(next); err == nil {
				t.Fatalf("expected error for malformed URL %q", next)
			}
		})
	}
}

func TestUsersCommands_DefaultOutputJSON(t *testing.T) {
	commands := []*struct {
		name string
		cmd  func(*shared.Deps) *ffcli.Command
	}{
		{"list", newUsersListCommand},
		{"get", newUsersGetCommand},
		{"update", newUsersUpdateCommand},
		{"delete", newUsersDeleteCommand},
		{"invite", newUsersInviteCommand},
		{"invites list", newUsersInvitesListCommand},
		{"invites get", newUsersInvitesGetCommand},
		{"invites revoke", newUsersInvitesRevokeCommand},
		{"invites visible-apps list", newUsersInvitesVisibleAppsListCommand},
		{"visible-apps list", newUsersVisibleAppsListCommand},
		{"visible-apps get", newUsersVisibleAppsGetCommand},
	}

	for _, tc := range commands {
		t.Run(tc.name, func(t *testing.T) {
			cmd := tc.cmd(&shared.Deps{IO: &shared.IO{Out: &bytes.Buffer{}, ErrOut: &bytes.Buffer{}}})
			f := cmd.FlagSet.Lookup("output")
			if f == nil {
				t.Fatalf("expected --output flag to be defined")
				return
			}
			if f.DefValue != "json" {
				t.Fatalf("expected --output default to be 'json', got %q", f.DefValue)
			}
		})
	}
}
