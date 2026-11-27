package cmdtest

import (
	"net/http"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

// users invite puts a person on the team, so it requires --confirm. Without
// it the command refuses before any request, with usage exit code 2.
func TestUsersInviteRequiresConfirm(t *testing.T) {
	const (
		inviteBody  = `{"data":{"type":"userInvitations","attributes":{"email":"user@example.com","firstName":"Jane","lastName":"Doe","roles":["DEVELOPER"],"allAppsVisible":true}}}`
		inviteReply = `{"data":{"type":"userInvitations","id":"invite-1","attributes":{"email":"user@example.com","roles":["DEVELOPER"],"allAppsVisible":true}}}`
	)
	base := []string{"users", "invite", "--email", "user@example.com", "--first-name", "Jane", "--last-name", "Doe", "--roles", "DEVELOPER", "--output", "json"}
	tests := []struct {
		name     string
		args     []string
		sends    bool
		wantCode int
		wantErr  string
	}{
		{
			name:     "without --confirm refuses before any request",
			args:     append(append([]string{}, base...), "--all-apps"),
			wantCode: 2,
			wantErr:  "Error: --confirm is required",
		},
		{
			name:     "with --confirm sends exactly one invitation",
			args:     append(append([]string{}, base...), "--all-apps", "--confirm"),
			sends:    true,
			wantCode: 0,
		},
		{
			// The access flags are required flags, so they are reported first.
			name:     "missing access is reported before --confirm",
			args:     base,
			wantCode: 2,
			wantErr:  "Error: --all-apps or --visible-app is required",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			reg := asctest.NewRegistry(t)
			if test.sends {
				reg.Register(asctest.POST("/v1/userInvitations").JSONBody(inviteBody), asctest.JSON(http.StatusCreated, inviteReply))
			}

			stdout, stderr, code := runWithDeps(t, stubClient(t, reg), test.args...)

			if code != test.wantCode {
				t.Fatalf("exit code = %d, want %d (stderr %q)", code, test.wantCode, stderr)
			}
			requests := reg.Requests()
			if !test.sends {
				if len(requests) != 0 {
					t.Fatalf("sent %d requests, want none", len(requests))
				}
				assertUsageExitOutput(t, stdout, stderr, test.wantErr)
				return
			}
			if len(requests) != 1 {
				t.Fatalf("sent %d requests, want exactly one POST", len(requests))
			}
			if stdout == "" || stderr != "" {
				t.Fatalf("stdout = %q, stderr = %q, want the invitation on stdout and nothing on stderr", stdout, stderr)
			}
		})
	}
}
