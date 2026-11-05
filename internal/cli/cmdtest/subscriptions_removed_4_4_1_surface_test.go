package cmdtest

import (
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
)

// The App Store Connect API 4.4.1 product-scoped subscription command families
// were removed in 5.0.0. They land on the generic unknown-command usage error
// with the parent help pointer and never resolve an API client.
func TestSubscriptionsRemoved441CommandsAreUnknown(t *testing.T) {
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_APP_ID", "")

	stubTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		t.Fatalf("removed command must not send a request: %s %s", req.Method, req.URL.String())
		return nil, errors.New("unexpected request")
	}))

	tests := []struct {
		name     string
		args     []string
		wantErr  string
		wantHelp string
		wantTry  string
	}{
		{
			name:     "localizations list",
			args:     []string{"subscriptions", "localizations", "list", "--subscription-id", "SUB_ID"},
			wantErr:  "Error: unknown command `asc subscriptions localizations`",
			wantHelp: "asc subscriptions --help",
		},
		{
			name:     "localizations sync",
			args:     []string{"subscriptions", "localizations", "sync", "--subscription-id", "SUB_ID", "--file", "./localizations.json"},
			wantErr:  "Error: unknown command `asc subscriptions localizations`",
			wantHelp: "asc subscriptions --help",
		},
		{
			name:     "images list",
			args:     []string{"subscriptions", "images", "list", "--subscription-id", "SUB_ID"},
			wantErr:  "Error: unknown command `asc subscriptions images`",
			wantHelp: "asc subscriptions --help",
		},
		{
			name:     "groups localizations list",
			args:     []string{"subscriptions", "groups", "localizations", "list", "--group-id", "GROUP_ID"},
			wantErr:  "Error: unknown command `asc subscriptions groups localizations`",
			wantHelp: "asc subscriptions groups --help",
		},
		{
			name:     "groups localizations sync",
			args:     []string{"subscriptions", "groups", "localizations", "sync", "--group-id", "GROUP_ID", "--file", "./localizations.json"},
			wantErr:  "Error: unknown command `asc subscriptions groups localizations`",
			wantHelp: "asc subscriptions groups --help",
		},
		{
			name:     "review submit",
			args:     []string{"subscriptions", "review", "submit", "--subscription-id", "SUB_ID", "--confirm"},
			wantErr:  "Error: unknown command `asc subscriptions review submit`",
			wantHelp: "asc subscriptions review --help",
			wantTry:  "Try:\n  asc review submissions-create --app APP_ID --platform IOS\n  asc review items add --submission SUBMISSION_ID --item-type subscriptionVersions --item-id SUBSCRIPTION_VERSION_ID\n",
		},
		{
			name:     "review submit-group",
			args:     []string{"subscriptions", "review", "submit-group", "--group-id", "GROUP_ID", "--confirm"},
			wantErr:  "Error: unknown command `asc subscriptions review submit-group`",
			wantHelp: "asc subscriptions review --help",
			wantTry:  "Try:\n  asc review submissions-create --app APP_ID --platform IOS\n  asc review items add --submission SUBMISSION_ID --item-type subscriptionGroupVersions --item-id GROUP_VERSION_ID\n",
		},
		{
			name:     "iap submit",
			args:     []string{"iap", "submit", "--iap-id", "IAP_ID", "--confirm"},
			wantErr:  "Error: unknown command `asc iap submit`",
			wantHelp: "asc iap --help",
			wantTry:  "Try:\n  asc review submissions-create --app APP_ID --platform IOS\n  asc iap versions submit --version-id IAP_VERSION_ID --submission SUBMISSION_ID --confirm\n",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stdout, stderr := captureOutput(t, func() {
				if code := cmd.Run(test.args, "5.0.0"); code != cmd.ExitUsage {
					t.Fatalf("exit code = %d, want %d", code, cmd.ExitUsage)
				}
			})
			if stdout != "" {
				t.Fatalf("stdout = %q, want empty", stdout)
			}
			if !strings.Contains(stderr, test.wantErr) {
				t.Fatalf("stderr = %q, want unknown-command error %q", stderr, test.wantErr)
			}
			if !strings.Contains(stderr, test.wantHelp) {
				t.Fatalf("stderr = %q, want help pointer %q", stderr, test.wantHelp)
			}
			if !strings.Contains(stderr, test.wantTry) {
				t.Fatalf("stderr = %q, want replacement hint %q", stderr, test.wantTry)
			}
			if strings.Contains(stderr, "deprecated") {
				t.Fatalf("stderr = %q, want no deprecation guidance for a removed command", stderr)
			}
		})
	}
}
