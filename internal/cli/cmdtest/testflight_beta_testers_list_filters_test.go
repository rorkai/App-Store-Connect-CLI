package cmdtest

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func TestTestFlightBetaTestersListSendsSortIncludeAndInviteType(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/betaTesters", "filter[apps]=app-1", "filter[inviteType]=EMAIL,PUBLIC_LINK", "include=betaGroups", "sort=-lastName"),
		asctest.JSON(http.StatusOK, `{"data":[],"links":{}}`))

	_, stderr, err := runCommand(t, []string{
		"testflight", "testers", "list",
		"--app", "app-1",
		"--sort", "-lastName",
		"--include", "betaGroups",
		"--invite-type", "EMAIL,PUBLIC_LINK",
		"--output", "json",
	})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}
	if !strings.Contains(stderr, "included relationships can be partial") {
		t.Fatalf("expected included-relationship completeness warning, got %q", stderr)
	}
	requests := reg.Requests()
	if len(requests) == 0 {
		t.Fatal("expected an outbound request")
	}
	captured := requests[len(requests)-1]

	query := captured.URL.Query()
	if got := query.Get("sort"); got != "-lastName" {
		t.Fatalf("expected sort=-lastName, got %q", got)
	}
	if got := query.Get("include"); got != "betaGroups" {
		t.Fatalf("expected include=betaGroups, got %q", got)
	}
	if got := query.Get("filter[inviteType]"); got != "EMAIL,PUBLIC_LINK" {
		t.Fatalf("expected filter[inviteType]=EMAIL,PUBLIC_LINK, got %q", got)
	}
	if got := query.Get("filter[apps]"); got != "app-1" {
		t.Fatalf("expected filter[apps]=app-1, got %q", got)
	}
}

func TestTestFlightBetaTestersListLowercaseInviteTypeIsNormalized(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/betaTesters", "filter[apps]=app-1", "filter[inviteType]=PUBLIC_LINK"),
		asctest.JSON(http.StatusOK, `{"data":[],"links":{}}`))

	_, stderr, err := runCommand(t, []string{
		"testflight", "testers", "list",
		"--app", "app-1",
		"--invite-type", "public_link",
		"--output", "json",
	})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	requests := reg.Requests()
	if len(requests) == 0 {
		t.Fatal("expected an outbound request")
	}
	captured := requests[len(requests)-1]
	if got := captured.URL.Query().Get("filter[inviteType]"); got != "PUBLIC_LINK" {
		t.Fatalf("expected filter[inviteType]=PUBLIC_LINK, got %q", got)
	}
}

func TestTestFlightBetaTestersListNotesIncludeIsJSONOnly(t *testing.T) {
	tests := []struct {
		name     string
		format   string
		wantNote bool
	}{
		{name: "table drops included", format: "table", wantNote: true},
		{name: "markdown drops included", format: "markdown", wantNote: true},
		{name: "json renders included", format: "json", wantNote: false},
		{name: "uppercase json renders included", format: "JSON", wantNote: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

			reg := asctest.NewRegistry(t)
			reg.Install()
			reg.Register(asctest.GET("/v1/betaTesters", "filter[apps]=app-1", "include=betaGroups"),
				asctest.JSON(http.StatusOK, `{"data":[{"type":"betaTesters","id":"tester-1","attributes":{"email":"tester@example.com"}}],`+
					`"included":[{"type":"betaGroups","id":"group-a","attributes":{"name":"Alpha"}}],"links":{}}`))

			_, stderr, err := runCommand(t, []string{
				"testflight", "testers", "list",
				"--app", "app-1",
				"--include", "betaGroups",
				"--output", test.format,
			})
			if err != nil {
				t.Fatalf("run error: %v", err)
			}

			gotNote := strings.Contains(stderr, "included resources are only rendered in JSON output")
			if gotNote != test.wantNote {
				t.Fatalf("note presence = %v, want %v (stderr=%q)", gotNote, test.wantNote, stderr)
			}
			if !strings.Contains(stderr, "included relationships can be partial") {
				t.Fatalf("expected included-relationship completeness warning, got %q", stderr)
			}
		})
	}
}

func TestTestFlightBetaTestersListNotesNextURLIncludedResourcesAreJSONOnly(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	const nextURL = "https://api.appstoreconnect.apple.com/v1/betaTesters?cursor=AQ&include=betaGroups"
	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/betaTesters", "cursor=AQ", "include=betaGroups"),
		asctest.JSON(http.StatusOK, `{"data":[],"included":[{"type":"betaGroups","id":"group-a"}],"links":{}}`))

	_, stderr, err := runCommand(t, []string{
		"testflight", "testers", "list",
		"--next", nextURL,
		"--output", "table",
	})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}
	if !strings.Contains(stderr, "included resources are only rendered in JSON output") {
		t.Fatalf("expected JSON-only included-resource note, got %q", stderr)
	}
	if !strings.Contains(stderr, "included relationships can be partial") {
		t.Fatalf("expected included-relationship completeness warning, got %q", stderr)
	}
}

func TestTestFlightBetaTestersListRejectsInvalidFilterValues(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		wantHint string
	}{
		{
			name:     "invalid sort",
			args:     []string{"testflight", "testers", "list", "--app", "app-1", "--sort", "createdDate"},
			wantHint: "--sort must be one of: firstName, -firstName, lastName, -lastName, email, -email, inviteType, -inviteType, state, -state",
		},
		{
			name:     "invalid include",
			args:     []string{"testflight", "testers", "list", "--app", "app-1", "--include", "appDevices"},
			wantHint: "--include must be a comma-separated list of: apps, betaGroups, builds",
		},
		{
			name:     "invalid fields",
			args:     []string{"testflight", "testers", "list", "--app", "app-1", "--fields", "name"},
			wantHint: "--fields must be one of: firstName, lastName, email",
		},
		{
			name:     "invalid invite type",
			args:     []string{"testflight", "testers", "list", "--app", "app-1", "--invite-type", "SMS"},
			wantHint: "--invite-type must be a comma-separated list of: EMAIL, PUBLIC_LINK",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
			asctest.NewRegistry(t).Install()

			stdout, stderr := captureOutput(t, func() {
				if code := cmd.Run(test.args, "1.2.3"); code != cmd.ExitUsage {
					t.Fatalf("expected exit code %d, got %d", cmd.ExitUsage, code)
				}
			})
			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, test.wantHint) {
				t.Fatalf("expected stderr to list valid values %q, got %q", test.wantHint, stderr)
			}
		})
	}
}

func TestTestFlightBetaTestersListRejectsFiltersWithNextURL(t *testing.T) {
	const nextURL = "https://api.appstoreconnect.apple.com/v1/betaTesters?cursor=AQ&limit=200"

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "sort",
			args:    []string{"testflight", "testers", "list", "--next", nextURL, "--sort", "email"},
			wantErr: "--next cannot be combined with --sort",
		},
		{
			name:    "include",
			args:    []string{"testflight", "testers", "list", "--next", nextURL, "--include", "betaGroups"},
			wantErr: "--next cannot be combined with --include",
		},
		{
			name:    "fields",
			args:    []string{"testflight", "testers", "list", "--next", nextURL, "--fields", "email"},
			wantErr: "--next cannot be combined with --fields",
		},
		{
			name:    "invite type",
			args:    []string{"testflight", "testers", "list", "--next", nextURL, "--invite-type", "EMAIL"},
			wantErr: "--next cannot be combined with --invite-type",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
			asctest.NewRegistry(t).Install()

			stdout, stderr := captureOutput(t, func() {
				if code := cmd.Run(test.args, "1.2.3"); code != cmd.ExitUsage {
					t.Fatalf("expected exit code %d, got %d", cmd.ExitUsage, code)
				}
			})
			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, test.wantErr) {
				t.Fatalf("expected stderr to contain %q, got %q", test.wantErr, stderr)
			}
		})
	}
}

func TestTestFlightBetaTestersListPaginateMergesIncludedBetaGroups(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	const secondURL = "https://api.appstoreconnect.apple.com/v1/betaTesters?cursor=BQ&filter%5Bapps%5D=app-1&include=betaGroups&limit=200"

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/betaTesters", "filter[apps]=app-1", "include=betaGroups", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"betaTesters","id":"tester-1","relationships":{"betaGroups":{"data":[{"type":"betaGroups","id":"group-a"}]}}}],`+
			`"included":[{"type":"betaGroups","id":"group-a","attributes":{"name":"Alpha"}}],`+
			`"links":{"next":"`+secondURL+`"}}`))
	reg.Register(asctest.GET("/v1/betaTesters", "cursor=BQ", "filter[apps]=app-1", "include=betaGroups", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"betaTesters","id":"tester-2","relationships":{"betaGroups":{"data":[{"type":"betaGroups","id":"group-b"}]}}}],`+
			`"included":[{"type":"betaGroups","id":"group-a","attributes":{"name":"Alpha updated"}},{"type":"betaGroups","id":"group-b","attributes":{"name":"Bravo"}}],`+
			`"links":{}}`))

	stdout, _, err := runCommand(t, []string{
		"testflight", "testers", "list",
		"--app", "app-1",
		"--include", "betaGroups",
		"--paginate",
		"--output", "json",
	})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if requestCount := len(reg.Requests()); requestCount != 2 {
		t.Fatalf("expected 2 requests, got %d", requestCount)
	}

	var response struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
		Included []struct {
			Type       string `json:"type"`
			ID         string `json:"id"`
			Attributes struct {
				Name string `json:"name"`
			} `json:"attributes"`
		} `json:"included"`
	}
	if err := json.Unmarshal([]byte(stdout), &response); err != nil {
		t.Fatalf("decode stdout: %v (stdout=%q)", err, stdout)
	}
	if len(response.Data) != 2 {
		t.Fatalf("expected 2 aggregated testers, got %d (stdout=%q)", len(response.Data), stdout)
	}

	if len(response.Included) != 2 {
		t.Fatalf("expected one included resource per type and ID, got %+v (stdout=%q)", response.Included, stdout)
	}
	if response.Included[0].ID != "group-a" || response.Included[0].Attributes.Name != "Alpha" ||
		response.Included[1].ID != "group-b" {
		t.Fatalf("expected first group-a representation followed by group-b, got %+v (stdout=%q)", response.Included, stdout)
	}
}
