package cmdtest

import (
	"net/http"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func assertBuildsRequestOrder(t *testing.T, reg *asctest.Registry, want ...string) {
	t.Helper()
	var got []string
	for _, req := range reg.Requests() {
		got = append(got, req.Method+" "+req.URL.Path)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("requests = %q, want %q", got, want)
	}
}

func registerAddGroupsLookups(reg *asctest.Registry, groupsBody string) {
	reg.Register(asctest.GET("/v1/builds/build-1/app"), asctest.JSON(http.StatusOK, `{"data":{"type":"apps","id":"app-1"}}`))
	reg.Register(asctest.GET("/v1/apps/app-1/betaGroups", "limit=200"), asctest.JSON(http.StatusOK, groupsBody))
}

func TestBuildsAddGroupsInternalGroupAddsGroup(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	reg := asctest.NewRegistry(t)
	reg.Install()
	registerAddGroupsLookups(reg, `{"data":[{"type":"betaGroups","id":"group-internal","attributes":{"name":"Friends & Family","isInternalGroup":true}}]}`)
	reg.Register(asctest.GET("/v1/builds/build-1"),
		asctest.JSON(http.StatusOK, `{"data":{"type":"builds","id":"build-1","attributes":{"processingState":"VALID","expired":false}}}`))
	registerAddGroupsPost(reg, "group-internal")

	stdout, stderr, err := runCommand(t, []string{"builds", "add-groups", "--build-id", "build-1", "--group", "group-internal"})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	assertBuildsRequestOrder(t, reg,
		"GET /v1/builds/build-1/app",
		"GET /v1/apps/app-1/betaGroups",
		"GET /v1/builds/build-1",
		"POST /v1/builds/build-1/relationships/betaGroups",
	)
	if !strings.Contains(stdout, `"groupIds":["group-internal"]`) {
		t.Fatalf("expected internal group in output, got %q", stdout)
	}
	if !strings.Contains(stderr, "Successfully added 1 group(s) to build build-1") {
		t.Fatalf("expected success message, got %q", stderr)
	}
}

func TestBuildsAddGroupsAddsMixedInternalAndExternalGroups(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	reg := asctest.NewRegistry(t)
	reg.Install()
	registerAddGroupsLookups(reg, `{"data":[{"type":"betaGroups","id":"group-internal","attributes":{"name":"Friends & Family","isInternalGroup":true}},{"type":"betaGroups","id":"group-external","attributes":{"name":"External QA","isInternalGroup":false}}]}`)
	registerAddGroupsExternalPreflight(reg, "READY_FOR_BETA_TESTING")
	reg.Register(asctest.POST("/v1/builds/build-1/relationships/betaGroups").JSONBody(`{"data":[{"type":"betaGroups","id":"group-internal"},{"type":"betaGroups","id":"group-external"}]}`),
		asctest.JSON(http.StatusNoContent, ""))

	stdout, stderr, err := runCommand(t, []string{
		"builds", "add-groups",
		"--build-id", "build-1",
		"--group", "group-internal,group-external",
	})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	assertBuildsRequestOrder(t, reg,
		"GET /v1/builds/build-1/app",
		"GET /v1/apps/app-1/betaGroups",
		"GET /v1/builds/build-1",
		"GET /v1/builds/build-1/buildBetaDetail",
		"POST /v1/builds/build-1/relationships/betaGroups",
	)
	if !strings.Contains(stdout, `"groupIds":["group-internal","group-external"]`) {
		t.Fatalf("expected both groups in output, got %q", stdout)
	}
	if !strings.Contains(stderr, "Successfully added 2 group(s) to build build-1") {
		t.Fatalf("expected success message, got %q", stderr)
	}
}

func TestBuildsAddGroupsSkipInternalAddsOnlyExternalGroups(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	reg := asctest.NewRegistry(t)
	reg.Install()
	registerAddGroupsLookups(reg, `{"data":[{"type":"betaGroups","id":"group-internal","attributes":{"name":"Friends & Family","isInternalGroup":true}},{"type":"betaGroups","id":"group-external","attributes":{"name":"External QA","isInternalGroup":false}}]}`)
	registerAddGroupsExternalPreflight(reg, "READY_FOR_BETA_TESTING")
	registerAddGroupsPost(reg, "group-external")

	stdout, stderr, err := runCommand(t, []string{
		"builds", "add-groups",
		"--build-id", "build-1",
		"--group", "group-internal,group-external",
		"--skip-internal",
	})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	assertBuildsRequestOrder(t, reg,
		"GET /v1/builds/build-1/app",
		"GET /v1/apps/app-1/betaGroups",
		"GET /v1/builds/build-1",
		"GET /v1/builds/build-1/buildBetaDetail",
		"POST /v1/builds/build-1/relationships/betaGroups",
	)
	if !strings.Contains(stdout, `"groupIds":["group-external"]`) {
		t.Fatalf("expected only external group in output, got %q", stdout)
	}
	if !strings.Contains(stderr, `Skipped internal group "Friends & Family"`) {
		t.Fatalf("expected skipped internal group message, got %q", stderr)
	}
}

func TestBuildsAddGroupsSkipInternalWithOnlyInternalGroupsIsNoOp(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	reg := asctest.NewRegistry(t)
	reg.Install()
	registerAddGroupsLookups(reg, `{"data":[{"type":"betaGroups","id":"group-internal","attributes":{"name":"Friends & Family","isInternalGroup":true}}]}`)

	stdout, stderr, err := runCommand(t, []string{
		"builds", "add-groups",
		"--build-id", "build-1",
		"--group", "group-internal",
		"--skip-internal",
	})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	assertBuildsRequestOrder(t, reg,
		"GET /v1/builds/build-1/app",
		"GET /v1/apps/app-1/betaGroups",
	)
	if !strings.Contains(stdout, `"groupIds":[]`) {
		t.Fatalf("expected empty group list output, got %q", stdout)
	}
	if !strings.Contains(stderr, "No groups to add for build build-1 after applying filters") {
		t.Fatalf("expected no-op message, got %q", stderr)
	}
}
