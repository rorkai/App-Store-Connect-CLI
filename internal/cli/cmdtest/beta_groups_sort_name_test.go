package cmdtest

import (
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

// TestBetaGroupsListSortPropagates proves --sort reaches the API.
func TestBetaGroupsListSortPropagates(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/betaGroups", "filter[app]=app-1", "sort=-createdDate"),
		asctest.JSON(http.StatusOK, `{"data":[]}`))

	stdout, stderr := runBetaGroupsList(t, "--app", "app-1", "--sort", "-createdDate")

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"data":[]`) {
		t.Fatalf("expected empty data envelope, got %q", stdout)
	}
}

// TestBetaGroupsListGlobalNameAndSortPropagate proves --name and --sort reach
// the global endpoint together.
func TestBetaGroupsListGlobalNameAndSortPropagate(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/betaGroups", "filter[name]=QA Testers", "sort=name"),
		asctest.JSON(http.StatusOK, `{"data":[]}`))

	_, stderr := runBetaGroupsList(t, "--global", "--name", "QA Testers", "--sort", "name")

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
}

// TestBetaGroupsListNameFilterUsesTopLevelEndpoint proves --name routes the
// app-scoped lookup through the endpoint that supports filter[name], and that
// a name typed with ASCII spaces finds Apple's default group, whose name has a
// non-breaking space, with one extra lookup and no extra request on an exact match.
func TestBetaGroupsListNameFilterUsesTopLevelEndpoint(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	const appGroups = `{"data":[` +
		`{"type":"betaGroups","id":"bg-default","attributes":{"name":"App\u00a0Store Connect Users"}},` +
		`{"type":"betaGroups","id":"bg-qa","attributes":{"name":"QA"}}]}`
	const nbspGroup = `{"data":[{"type":"betaGroups","id":"bg-default","attributes":{"name":"App\u00a0Store Connect Users"}}]}`

	type request struct{ path, name, body string }
	tests := []struct {
		name       string
		nameFlag   string
		requests   []request
		wantStdout string
	}{
		{
			name:     "exact match makes no extra request",
			nameFlag: "Beta Testers",
			requests: []request{
				{"/v1/betaGroups", "Beta Testers", `{"data":[{"type":"betaGroups","id":"bg-1","attributes":{"name":"Beta Testers"}}]}`},
			},
			wantStdout: `"id":"bg-1"`,
		},
		{
			name:     "ASCII spaces resolve the NBSP-named group",
			nameFlag: "app store connect users",
			requests: []request{
				{"/v1/betaGroups", "app store connect users", `{"data":[]}`},
				{"/v1/apps/app-1/betaGroups", "", appGroups},
				{"/v1/betaGroups", "App\u00a0Store Connect Users", nbspGroup},
			},
			wantStdout: `"id":"bg-default"`,
		},
		{
			name:     "no match prints the empty envelope after one lookup",
			nameFlag: "Missing Group",
			requests: []request{
				{"/v1/betaGroups", "Missing Group", `{"data":[]}`},
				{"/v1/apps/app-1/betaGroups", "", appGroups},
			},
			wantStdout: `"data":[]`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			reg := asctest.NewRegistry(t)
			reg.Install()
			for _, want := range test.requests {
				query := []string{"fields[betaGroups]=name", "limit=200"}
				if want.path == "/v1/betaGroups" {
					query = []string{"filter[app]=app-1", "filter[name]=" + want.name}
				}
				reg.Register(asctest.GET(want.path, query...), asctest.JSON(http.StatusOK, want.body))
			}

			stdout, stderr := runBetaGroupsList(t, "--app", "app-1", "--name", test.nameFlag, "--output", "json")

			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}
			if count := len(reg.Requests()); count != len(test.requests) {
				t.Fatalf("made %d requests, want %d", count, len(test.requests))
			}
			if !strings.Contains(stdout, test.wantStdout) {
				t.Fatalf("stdout = %q, want %s", stdout, test.wantStdout)
			}
		})
	}
}

// TestBetaGroupsListInvalidSortIsUsageError proves an unsupported --sort value
// fails with a usage error that lists the valid values.
func TestBetaGroupsListInvalidSortIsUsageError(t *testing.T) {
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	assertUsageExit(
		t,
		[]string{"testflight", "groups", "list", "--app", "app-1", "--sort", "nope"},
		"--sort must be one of: name, -name, createdDate, -createdDate, publicLinkEnabled, -publicLinkEnabled, publicLinkLimit, -publicLinkLimit",
	)
}

// TestBetaGroupsListBlankSortAndNameAreUsageErrors proves an explicitly blank
// --name or --sort is reported instead of quietly widening the listing.
func TestBetaGroupsListBlankSortAndNameAreUsageErrors(t *testing.T) {
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	tests := []struct {
		args    []string
		wantErr string
	}{
		{[]string{"testflight", "groups", "list", "--app", "app-1", "--sort", ""}, "--sort cannot be empty"},
		{[]string{"testflight", "groups", "list", "--app", "app-1", "--sort", "   "}, "--sort cannot be empty"},
		{[]string{"testflight", "groups", "list", "--app", "app-1", "--name", ""}, "--name cannot be empty"},
		{[]string{"testflight", "groups", "list", "--app", "app-1", "--name", "   "}, "--name cannot be empty"},
	}
	for _, test := range tests {
		assertUsageExit(t, test.args, test.wantErr)
	}
}

// TestBetaGroupsListSortAndNameRejectedWithBuildID proves the membership lookup
// does not silently ignore query flags it cannot honor.
func TestBetaGroupsListSortAndNameRejectedWithBuildID(t *testing.T) {
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	tests := [][]string{
		{"testflight", "groups", "list", "--build-id", "build-1", "--sort", "name"},
		{"testflight", "groups", "list", "--build-id", "build-1", "--name", "Beta Testers"},
	}
	for _, args := range tests {
		assertUsageExit(t, args, "Error: --name and --sort cannot be used with --build-id")
	}
}
