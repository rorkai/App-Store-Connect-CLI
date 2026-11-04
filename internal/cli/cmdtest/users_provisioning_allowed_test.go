package cmdtest

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"reflect"
	"testing"
)

func TestUsersProvisioningAllowedRequestBody(t *testing.T) {
	commands := []struct {
		name      string
		args      []string
		method    string
		path      string
		status    int
		response  string
		wantAttrs map[string]any
		// current is the provisioning access read back when the flag is
		// omitted; empty means the command must not read the user.
		current string
		// omitted is the provisioningAllowed sent when the flag is omitted.
		omitted any
	}{
		{
			name:     "update",
			args:     []string{"users", "update", "--id", "user-1", "--roles", "developer", "--output", "json"},
			method:   http.MethodPatch,
			path:     "/v1/users/user-1",
			status:   http.StatusOK,
			response: `{"data":{"type":"users","id":"user-1","attributes":{"roles":["DEVELOPER"]}}}`,
			wantAttrs: map[string]any{
				"roles": []any{"DEVELOPER"},
			},
			current: "true",
			omitted: true,
		},
		{
			name:     "update without current access",
			args:     []string{"users", "update", "--id", "user-1", "--roles", "developer", "--output", "json"},
			method:   http.MethodPatch,
			path:     "/v1/users/user-1",
			status:   http.StatusOK,
			response: `{"data":{"type":"users","id":"user-1","attributes":{"roles":["DEVELOPER"]}}}`,
			wantAttrs: map[string]any{
				"roles": []any{"DEVELOPER"},
			},
			current: "false",
		},
		{
			name:     "update app manager",
			args:     []string{"users", "update", "--id", "user-1", "--roles", "app_manager", "--output", "json"},
			method:   http.MethodPatch,
			path:     "/v1/users/user-1",
			status:   http.StatusOK,
			response: `{"data":{"type":"users","id":"user-1","attributes":{"roles":["APP_MANAGER"]}}}`,
			wantAttrs: map[string]any{
				"roles": []any{"APP_MANAGER"},
			},
			current: "true",
			omitted: true,
		},
		{
			name:     "update with unknown current access",
			args:     []string{"users", "update", "--id", "user-1", "--roles", "developer", "--output", "json"},
			method:   http.MethodPatch,
			path:     "/v1/users/user-1",
			status:   http.StatusOK,
			response: `{"data":{"type":"users","id":"user-1","attributes":{"roles":["DEVELOPER"]}}}`,
			wantAttrs: map[string]any{
				"roles": []any{"DEVELOPER"},
			},
			current: "null",
		},
		{
			name:     "update to role without provisioning",
			args:     []string{"users", "update", "--id", "user-1", "--roles", "marketing", "--output", "json"},
			method:   http.MethodPatch,
			path:     "/v1/users/user-1",
			status:   http.StatusOK,
			response: `{"data":{"type":"users","id":"user-1","attributes":{"roles":["MARKETING"]}}}`,
			wantAttrs: map[string]any{
				"roles": []any{"MARKETING"},
			},
		},
		{
			name:     "invite",
			args:     []string{"users", "invite", "--email", "user@example.com", "--first-name", "Jane", "--last-name", "Doe", "--roles", "developer", "--all-apps", "--output", "json"},
			method:   http.MethodPost,
			path:     "/v1/userInvitations",
			status:   http.StatusCreated,
			response: `{"data":{"type":"userInvitations","id":"invite-1","attributes":{"roles":["DEVELOPER"]}}}`,
			wantAttrs: map[string]any{
				"email":          "user@example.com",
				"firstName":      "Jane",
				"lastName":       "Doe",
				"roles":          []any{"DEVELOPER"},
				"allAppsVisible": true,
			},
		},
	}
	values := []struct {
		name string
		args []string
		want any
	}{
		{name: "true", args: []string{"--provisioning-allowed", "true"}, want: true},
		{name: "explicit false", args: []string{"--provisioning-allowed", "false"}, want: false},
		{name: "explicit false with equals", args: []string{"--provisioning-allowed=false"}, want: false},
		{name: "omitted"},
	}

	for _, command := range commands {
		for _, value := range values {
			t.Run(command.name+" "+value.name, func(t *testing.T) {
				setupAuth(t)

				wantAttrs := map[string]any{}
				for key, attr := range command.wantAttrs {
					wantAttrs[key] = attr
				}
				wantGet := value.want == nil && command.current != ""
				if value.want != nil {
					wantAttrs["provisioningAllowed"] = value.want
				} else if command.omitted != nil {
					wantAttrs["provisioningAllowed"] = command.omitted
				}

				var gotAttrs map[string]any
				gotGet := false
				installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
					if wantGet && req.Method == http.MethodGet && req.URL.Path == command.path {
						gotGet = true
						return jsonResponse(http.StatusOK, `{"data":{"type":"users","id":"user-1","attributes":{"roles":["APP_MANAGER"],"provisioningAllowed":`+command.current+`}}}`)
					}
					if req.Method != command.method || req.URL.Path != command.path {
						t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
					}
					var payload struct {
						Data struct {
							Attributes map[string]any `json:"attributes"`
						} `json:"data"`
					}
					if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
						t.Fatalf("decode request: %v", err)
					}
					gotAttrs = payload.Data.Attributes
					return jsonResponse(command.status, command.response)
				}))

				root := RootCommand("1.2.3")
				root.FlagSet.SetOutput(io.Discard)
				if err := root.Parse(append(append([]string{}, command.args...), value.args...)); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				captureOutput(t, func() {
					if err := root.Run(context.Background()); err != nil {
						t.Fatalf("run error: %v", err)
					}
				})
				if gotGet != wantGet {
					t.Fatalf("read current user = %v, want %v", gotGet, wantGet)
				}
				if !reflect.DeepEqual(gotAttrs, wantAttrs) {
					t.Fatalf("attributes = %#v, want %#v", gotAttrs, wantAttrs)
				}
			})
		}
	}
}
