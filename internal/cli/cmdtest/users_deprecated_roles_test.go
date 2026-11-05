package cmdtest

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"
)

func TestUsersCommandsWarnDeprecatedAccessToReportsRole(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		method   string
		path     string
		response string
		wantID   string
	}{
		{
			name:     "update",
			args:     []string{"users", "update", "--id", "user-1", "--roles", "access_to_reports", "--output", "json"},
			method:   http.MethodPatch,
			path:     "/v1/users/user-1",
			response: `{"data":{"type":"users","id":"user-1","attributes":{"username":"user@example.com","roles":["ACCESS_TO_REPORTS"],"allAppsVisible":true,"provisioningAllowed":false}}}`,
			wantID:   "user-1",
		},
		{
			name:     "invite",
			args:     []string{"users", "invite", "--email", "user@example.com", "--first-name", "Jane", "--last-name", "Doe", "--roles", "access_to_reports", "--all-apps", "--output", "json"},
			method:   http.MethodPost,
			path:     "/v1/userInvitations",
			response: `{"data":{"type":"userInvitations","id":"invite-1","attributes":{"email":"user@example.com","roles":["ACCESS_TO_REPORTS"],"allAppsVisible":true,"provisioningAllowed":false}}}`,
			wantID:   "invite-1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setupAuth(t)

			installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.Method != tt.method || req.URL.Path != tt.path {
					t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
					return nil, nil
				}
				return jsonResponse(http.StatusOK, tt.response)
			}))

			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			var runErr error
			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(tt.args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				runErr = root.Run(context.Background())
			})
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
