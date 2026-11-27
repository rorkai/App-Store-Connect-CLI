package cmdtest

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func TestUsersCommandsWarnDeprecatedAccessToReportsRole(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		request  asctest.Matcher
		response string
		wantID   string
	}{
		{
			name:     "update",
			args:     []string{"users", "update", "--id", "user-1", "--roles", "access_to_reports", "--output", "json"},
			request:  asctest.PATCH("/v1/users/user-1").JSONBody(`{"data":{"type":"users","id":"user-1","attributes":{"roles":["ACCESS_TO_REPORTS"]}}}`),
			response: `{"data":{"type":"users","id":"user-1","attributes":{"username":"user@example.com","roles":["ACCESS_TO_REPORTS"],"allAppsVisible":true,"provisioningAllowed":false}}}`,
			wantID:   "user-1",
		},
		{
			name:     "invite",
			args:     []string{"users", "invite", "--email", "user@example.com", "--first-name", "Jane", "--last-name", "Doe", "--roles", "access_to_reports", "--all-apps", "--output", "json", "--confirm"},
			request:  asctest.POST("/v1/userInvitations").JSONBody(`{"data":{"type":"userInvitations","attributes":{"email":"user@example.com","firstName":"Jane","lastName":"Doe","roles":["ACCESS_TO_REPORTS"],"allAppsVisible":true}}}`),
			response: `{"data":{"type":"userInvitations","id":"invite-1","attributes":{"email":"user@example.com","roles":["ACCESS_TO_REPORTS"],"allAppsVisible":true,"provisioningAllowed":false}}}`,
			wantID:   "invite-1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reg := asctest.NewRegistry(t)
			reg.Register(tt.request, asctest.JSON(http.StatusOK, tt.response))

			stdout, stderr, runErr := runRootWithDeps(t, stubClient(t, reg), tt.args...)
			if runErr != nil {
				t.Fatalf("run error: %v; stderr=%q stdout=%q", runErr, stderr, stdout)
			}
			requireStderrContainsWarning(t, stderr, "Warning: ACCESS_TO_REPORTS is deprecated in App Store Connect API 4.4")

			var got struct {
				Data struct {
					ID string `json:"id"`
				} `json:"data"`
			}
			if err := json.Unmarshal([]byte(stdout), &got); err != nil {
				t.Fatalf("expected valid JSON output, got parse error: %v; stdout=%q", err, stdout)
			}
			if got.Data.ID != tt.wantID {
				t.Fatalf("expected %s, got %#v", tt.wantID, got.Data)
			}
		})
	}
}
