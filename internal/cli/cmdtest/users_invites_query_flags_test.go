package cmdtest

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

func TestUsersInvitesQueryFlagsSendTheirSchemaParameter(t *testing.T) {
	list := []string{"users", "invites", "list"}
	view := []string{"users", "invites", "view", "--id", "invite-1"}
	tests := []struct {
		name string
		args []string
		path string
		want url.Values
	}{
		{name: "list without query flags", args: list, path: "/v1/userInvitations", want: url.Values{}},
		{name: "list limit", args: append(list, "--limit", "10"), path: "/v1/userInvitations", want: url.Values{"limit": {"10"}}},
		{name: "filter", args: append(list, "--email", "a@example.com,b@example.com"), path: "/v1/userInvitations", want: url.Values{"filter[email]": {"a@example.com,b@example.com"}}},
		{name: "filter with an enum", args: append(list, "--role", "developer, APP_MANAGER"), path: "/v1/userInvitations", want: url.Values{"filter[roles]": {"DEVELOPER,APP_MANAGER"}}},
		{name: "relationship filter", args: append(list, "--visible-app", "app-1,app-2"), path: "/v1/userInvitations", want: url.Values{"filter[visibleApps]": {"app-1,app-2"}}},
		{name: "sort", args: append(list, "--sort", "-lastName,email"), path: "/v1/userInvitations", want: url.Values{"sort": {"-lastName,email"}}},
		{name: "fields of the listed type", args: append(list, "--fields", "email,roles"), path: "/v1/userInvitations", want: url.Values{"fields[userInvitations]": {"email,roles"}}},
		{name: "fields of an included type", args: append(list, "--include", "visibleApps", "--app-fields", "name,bundleId"), path: "/v1/userInvitations", want: url.Values{"include": {"visibleApps"}, "fields[apps]": {"name,bundleId"}}},
		{name: "relationship limit", args: append(list, "--include", "visibleApps", "--visible-apps-limit", "25", "--limit", "5"), path: "/v1/userInvitations", want: url.Values{"include": {"visibleApps"}, "limit[visibleApps]": {"25"}, "limit": {"5"}}},
		{name: "view without query flags", args: view, path: "/v1/userInvitations/invite-1", want: url.Values{}},
		{name: "view fields", args: append(view, "--fields", "email"), path: "/v1/userInvitations/invite-1", want: url.Values{"fields[userInvitations]": {"email"}}},
		{name: "view include", args: append(view, "--include", "visibleApps", "--visible-apps-limit", "3", "--app-fields", "name"), path: "/v1/userInvitations/invite-1", want: url.Values{"include": {"visibleApps"}, "limit[visibleApps]": {"3"}, "fields[apps]": {"name"}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
			var got *url.URL
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				got = req.URL
				w.Header().Set("Content-Type", "application/json")
				if strings.HasSuffix(req.URL.Path, "/invite-1") {
					_, _ = io.WriteString(w, `{"data":{"type":"userInvitations","id":"invite-1"}}`)
					return
				}
				_, _ = io.WriteString(w, `{"data":[{"type":"userInvitations","id":"invite-1"}],"links":{"next":""}}`)
			}))
			t.Cleanup(server.Close)

			_, stderr, err := runRootWithDeps(t, usersListTestClient(t, server), append(test.args, "--output", "json")...)
			if err != nil {
				t.Fatalf("run error: %v; stderr = %q", err, stderr)
			}
			if got == nil || got.Path != test.path {
				t.Fatalf("request = %v, want path %s", got, test.path)
			}
			if got.Query().Encode() != test.want.Encode() {
				t.Fatalf("query = %q, want %q", got.Query().Encode(), test.want.Encode())
			}
		})
	}
}

func TestUsersInvitesRejectInvalidQueryFlagsBeforeClient(t *testing.T) {
	const nextURL = "https://api.appstoreconnect.apple.com/v1/userInvitations?cursor=2"
	list := []string{"users", "invites", "list"}
	view := []string{"users", "invites", "view", "--id", "invite-1"}
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "enum filter", args: append(list, "--role", "OWNER"), want: "Error: users invites list: --role must be one of: ADMIN, FINANCE, ACCOUNT_HOLDER,"},
		{name: "sort", args: append(list, "--sort", "createdDate"), want: "Error: users invites list: --sort must be one of: email, -email, lastName, -lastName"},
		{name: "fields", args: append(list, "--fields", "username"), want: "Error: users invites list: --fields must be one of: email, firstName,"},
		{name: "included fields", args: append(view, "--app-fields", "title"), want: "Error: users invites view: --app-fields must be one of: accessibilityUrl, name,"},
		{name: "include", args: append(view, "--include", "apps"), want: "Error: users invites view: --include must be one of: visibleApps"},
		{name: "empty list", args: append(list, "--email", " , "), want: "Error: users invites list: --email must not be empty"},
		{name: "relationship limit above the maximum", args: append(list, "--include", "visibleApps", "--visible-apps-limit", "51"), want: "Error: users invites list: --visible-apps-limit must be between 1 and 50"},
		{name: "relationship limit without its include", args: append(view, "--visible-apps-limit", "5"), want: "Error: users invites view: --visible-apps-limit requires --include visibleApps"},
		{name: "query flag with --next", args: append(list, "--next", nextURL, "--role", "ADMIN"), want: "Error: users invites list: --next cannot be combined with --role"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			clientCalled := false
			client := func() (*asc.Client, error) {
				clientCalled = true
				return nil, errors.New("client factory must not run during validation")
			}
			assertUsageExitWithDeps(t, client, test.args, test.want)
			if clientCalled {
				t.Fatal("client factory ran before query flag validation")
			}
		})
	}
}
