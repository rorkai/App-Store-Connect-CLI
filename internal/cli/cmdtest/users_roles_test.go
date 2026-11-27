package cmdtest

import (
	"net/http"
	"strings"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func TestUsersListNormalizesRoles(t *testing.T) {
	reg := asctest.NewRegistry(t)
	reg.Register(asctest.GET("/v1/users", "filter[roles]=DEVELOPER,APP_MANAGER"), asctest.JSON(http.StatusOK, `{"data":[]}`))

	if _, _, err := runRootWithDeps(t, stubClient(t, reg), "users", "list", "--role", " developer, app_manager ", "--output", "json"); err != nil {
		t.Fatalf("run error: %v", err)
	}
}

func TestUsersUpdateNormalizesRoles(t *testing.T) {
	reg := asctest.NewRegistry(t)
	reg.Register(asctest.GET("/v1/users/user-1"),
		asctest.JSON(http.StatusOK, `{"data":{"type":"users","id":"user-1","attributes":{"roles":["DEVELOPER"],"provisioningAllowed":true}}}`))
	reg.Register(asctest.PATCH("/v1/users/user-1").JSONBody(`{"data":{"type":"users","id":"user-1","attributes":{"roles":["DEVELOPER","APP_MANAGER"],"provisioningAllowed":true}}}`),
		asctest.JSON(http.StatusOK, `{"data":{"type":"users","id":"user-1","attributes":{"roles":["DEVELOPER","APP_MANAGER"]}}}`))

	if _, _, err := runRootWithDeps(t, stubClient(t, reg), "users", "update", "--id", "user-1", "--roles", "developer,app_manager", "--output", "json"); err != nil {
		t.Fatalf("run error: %v", err)
	}
}

func TestUsersInviteNormalizesRoles(t *testing.T) {
	reg := asctest.NewRegistry(t)
	reg.Register(asctest.POST("/v1/userInvitations").JSONBody(`{"data":{"type":"userInvitations","attributes":{"email":"user@example.com","firstName":"Jane","lastName":"Doe","roles":["DEVELOPER"],"allAppsVisible":true}}}`),
		asctest.JSON(http.StatusCreated, `{"data":{"type":"userInvitations","id":"invite-1","attributes":{"roles":["DEVELOPER"]}}}`))

	if _, _, err := runRootWithDeps(t, stubClient(t, reg), "users", "invite", "--email", "user@example.com", "--first-name", "Jane", "--last-name", "Doe", "--roles", "developer", "--all-apps", "--output", "json", "--confirm"); err != nil {
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
			stdout, stderr, code := runWithDeps(t, stubClient(t, asctest.NewRegistry(t)), test.args...)
			if code != rootcmd.ExitUsage {
				t.Fatalf("exit code = %d, want %d", code, rootcmd.ExitUsage)
			}
			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, test.flag+" must be one of: ADMIN, FINANCE, ACCOUNT_HOLDER") {
				t.Fatalf("unexpected stderr: %q", stderr)
			}
		})
	}
}
