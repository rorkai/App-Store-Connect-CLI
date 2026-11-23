package cmdtest

import (
	"net/http"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

// users invite gives a person a role on the team, so it is moving to
// --confirm. Until a later release requires it, an invite without --confirm
// still goes through and warns on stderr.
func TestUsersInviteConfirmTransition(t *testing.T) {
	const (
		inviteWarning = "Warning: users invite sends an invitation to join your App Store Connect team and will require --confirm in a future release; pass --confirm to acknowledge.\n"
		inviteBody    = `{"data":{"type":"userInvitations","attributes":{"email":"user@example.com","firstName":"Jane","lastName":"Doe","roles":["DEVELOPER"],"allAppsVisible":true}}}`
		inviteReply   = `{"data":{"type":"userInvitations","id":"invite-1","attributes":{"email":"user@example.com","roles":["DEVELOPER"],"allAppsVisible":true}}}`
	)
	base := []string{"users", "invite", "--email", "user@example.com", "--first-name", "Jane", "--last-name", "Doe", "--roles", "DEVELOPER", "--output", "json"}
	tests := []struct {
		name       string
		args       []string
		sends      bool
		wantCode   int
		wantStderr string
	}{
		{
			name:       "without --confirm sends the invite and warns",
			args:       append(append([]string{}, base...), "--all-apps"),
			sends:      true,
			wantCode:   0,
			wantStderr: inviteWarning,
		},
		{
			name:       "with --confirm sends the invite quietly",
			args:       append(append([]string{}, base...), "--all-apps", "--confirm"),
			sends:      true,
			wantCode:   0,
			wantStderr: "",
		},
		{
			// A usage error comes first; the warning is only for a call
			// that is about to send.
			name:       "usage error does not warn",
			args:       base,
			wantCode:   2,
			wantStderr: "Error: --all-apps or --visible-app is required\n",
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
			if test.sends {
				if !strings.Contains(stdout, `"id":"invite-1"`) {
					t.Fatalf("stdout = %q, want the invitation", stdout)
				}
				if stderr != test.wantStderr {
					t.Fatalf("stderr = %q, want %q", stderr, test.wantStderr)
				}
				return
			}
			if stdout != "" {
				t.Fatalf("stdout = %q, want empty", stdout)
			}
			if !strings.HasPrefix(stderr, test.wantStderr) {
				t.Fatalf("stderr = %q, want it to start with %q", stderr, test.wantStderr)
			}
			if strings.Contains(stderr, "Warning:") {
				t.Fatalf("stderr = %q, want no warning before a usage error", stderr)
			}
		})
	}
}
