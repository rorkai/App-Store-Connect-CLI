package cmdtest

import (
	"context"
	"net/http"
	"strings"
	"testing"

	cmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	webcmd "github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/web"
	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

func TestWebReviewSubscriptionsMutationReportsUnknownAttachmentState(t *testing.T) {
	for _, operation := range []string{"remove"} {
		t.Run(operation, func(t *testing.T) {
			setCmdtestHome(t)
			t.Setenv("ASC_WEB_MIN_REQUEST_INTERVAL", "0")
			restore := webcmd.SetResolveWebSession(func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
				return &webcore.AuthSession{
					Client: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
						if req.Method != http.MethodGet || req.URL.Path != "/iris/v1/apps/app-1/subscriptionGroups" {
							t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
						}
						return webAgreementsJSONResponse(http.StatusOK, `{
							"data": [{
								"id": "group-1",
								"type": "subscriptionGroups",
								"relationships": {"subscriptions": {"data": [{"type": "subscriptions", "id": "sub-1"}]}}
							}],
							"included": [{
								"id": "sub-1",
								"type": "subscriptions",
								"attributes": {"productId": "com.example.monthly", "state": "READY_TO_SUBMIT"}
							}]
						}`), nil
					})},
				}, "cache", nil
			})
			t.Cleanup(restore)

			stdout, stderr, code := runWithDeps(t, nil,
				"web", "review", "subscriptions", operation,
				"--app", "app-1",
				"--subscription-id", "sub-1",
				"--confirm",
				"--output", "json",
			)
			if code != cmd.ExitError {
				t.Fatalf("exit code = %d, want %d", code, cmd.ExitError)
			}
			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			for _, want := range []string{
				`did not return the next-version attachment state for subscription "sub-1"`,
				`asc web review subscriptions list --app "app-1"`,
			} {
				if !strings.Contains(stderr, want) {
					t.Fatalf("expected stderr to contain %q, got %q", want, stderr)
				}
			}
		})
	}
}
