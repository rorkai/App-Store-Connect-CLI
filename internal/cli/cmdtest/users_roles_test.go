package cmdtest

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

func TestUsersListNormalizesRoles(t *testing.T) {
	setupAuth(t)
	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet || req.URL.Path != "/v1/users" {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
		}
		if got := req.URL.Query().Get("filter[roles]"); got != "DEVELOPER,APP_MANAGER" {
			t.Fatalf("filter[roles] = %q, want DEVELOPER,APP_MANAGER", got)
		}
		return jsonResponse(http.StatusOK, `{"data":[]}`)
	}))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	if err := root.Parse([]string{"users", "list", "--role", " developer, app_manager ", "--output", "json"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if err := root.Run(context.Background()); err != nil {
		t.Fatalf("run error: %v", err)
	}
}

func TestUsersUpdateNormalizesRoles(t *testing.T) {
	setupAuth(t)
	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method == http.MethodGet && req.URL.Path == "/v1/users/user-1" {
			return jsonResponse(http.StatusOK, `{"data":{"type":"users","id":"user-1","attributes":{"roles":["DEVELOPER"],"provisioningAllowed":true}}}`)
		}
		if req.Method != http.MethodPatch || req.URL.Path != "/v1/users/user-1" {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
		}
		var payload asc.UserUpdateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if got := payload.Data.Attributes.Roles; len(got) != 2 || got[0] != "DEVELOPER" || got[1] != "APP_MANAGER" {
			t.Fatalf("roles = %#v, want [DEVELOPER APP_MANAGER]", got)
		}
		if payload.Data.Attributes.AllAppsVisible != nil {
			t.Fatalf("allAppsVisible = %v, want omitted", payload.Data.Attributes.AllAppsVisible)
		}
		if payload.Data.Relationships != nil {
			t.Fatalf("relationships = %+v, want omitted", payload.Data.Relationships)
		}
		return jsonResponse(http.StatusOK, `{"data":{"type":"users","id":"user-1","attributes":{"roles":["DEVELOPER","APP_MANAGER"]}}}`)
	}))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	if err := root.Parse([]string{"users", "update", "--id", "user-1", "--roles", "developer,app_manager", "--output", "json"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if err := root.Run(context.Background()); err != nil {
		t.Fatalf("run error: %v", err)
	}
}

func TestUsersInviteNormalizesRoles(t *testing.T) {
	setupAuth(t)
	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodPost || req.URL.Path != "/v1/userInvitations" {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
		}
		var payload asc.UserInvitationCreateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if got := payload.Data.Attributes.Roles; len(got) != 1 || got[0] != "DEVELOPER" {
			t.Fatalf("roles = %#v, want [DEVELOPER]", got)
		}
		return jsonResponse(http.StatusCreated, `{"data":{"type":"userInvitations","id":"invite-1","attributes":{"roles":["DEVELOPER"]}}}`)
	}))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	if err := root.Parse([]string{"users", "invite", "--email", "user@example.com", "--first-name", "Jane", "--last-name", "Doe", "--roles", "developer", "--all-apps", "--output", "json"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if err := root.Run(context.Background()); err != nil {
		t.Fatalf("run error: %v", err)
	}
}

func TestUsersCommandsRejectInvalidRolesWithUsageExit(t *testing.T) {
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_CONFIG_PATH", t.TempDir()+"/config.json")

	tests := []struct {
		name string
		args []string
		flag string
	}{
		{name: "list", args: []string{"users", "list", "--role", "not_a_role"}, flag: "--role"},
		{name: "update", args: []string{"users", "update", "--id", "user-1", "--roles", "not_a_role"}, flag: "--roles"},
		{name: "invite", args: []string{"users", "invite", "--email", "user@example.com", "--first-name", "Jane", "--last-name", "Doe", "--roles", "not_a_role", "--all-apps"}, flag: "--roles"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stdout, stderr := captureOutput(t, func() {
				if code := rootcmd.Run(test.args, "1.2.3"); code != rootcmd.ExitUsage {
					t.Fatalf("exit code = %d, want %d", code, rootcmd.ExitUsage)
				}
			})
			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, test.flag+" must be one of: ADMIN, FINANCE, ACCOUNT_HOLDER") {
				t.Fatalf("unexpected stderr: %q", stderr)
			}
		})
	}
}
