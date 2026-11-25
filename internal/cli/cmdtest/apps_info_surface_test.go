package cmdtest

import (
	"net/http"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func TestAppsInfoViewIncludeFailsWhenAppInfoIsAmbiguous(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_PROFILE", "")

	reg := asctest.NewRegistry(t)
	reg.Register(asctest.GET("/v1/apps/100000001/appInfos"), asctest.JSON(http.StatusOK, `{
				"data":[
					{"type":"appInfos","id":"info-live","attributes":{"state":"READY_FOR_DISTRIBUTION"}},
					{"type":"appInfos","id":"info-rejected","attributes":{"state":"REJECTED"}}
				]
			}`))

	stdout, stderr, runErr := runRootWithDeps(t, stubClient(t, reg), "apps", "info", "view", "--app", "100000001", "--include", "primaryCategory", "--output", "json")

	if runErr == nil {
		t.Fatal("expected run error, got nil")
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	for _, want := range []string{
		`2 app infos match app "100000001"; pass --info-id with one of:`,
		`asc apps info list --app "100000001"`,
		"READY_FOR_DISTRIBUTION",
		"REJECTED",
	} {
		if !strings.Contains(runErr.Error(), want) {
			t.Fatalf("expected error to contain %q, got %v", want, runErr)
		}
	}
}

func TestAppsInfoRelationshipsViewIncludesEveryCategory(t *testing.T) {
	reg := asctest.NewRegistry(t)
	reg.Register(asctest.GET("/v1/apps/100000001/appInfos"), asctest.JSON(http.StatusOK, `{"data":[{"type":"appInfos","id":"info-1","attributes":{"state":"READY_FOR_DISTRIBUTION"}}]}`))
	reg.Register(asctest.GET("/v1/appInfos/info-1", "include=primaryCategory,primarySubcategoryOne,primarySubcategoryTwo,secondaryCategory,secondarySubcategoryOne,secondarySubcategoryTwo"),
		asctest.JSON(http.StatusOK, `{"data":{"type":"appInfos","id":"info-1"},"included":[{"type":"appCategories","id":"PRODUCTIVITY"}]}`))

	stdout, _, err := runRootWithDeps(t, stubClient(t, reg), "apps", "info", "relationships", "view", "--app", "100000001", "--output", "json")
	if err != nil {
		t.Fatalf("run error: %v", err)
	}
	if !strings.Contains(stdout, `"included":[{"type":"appCategories","id":"PRODUCTIVITY"}]`) {
		t.Fatalf("expected the included categories in output, got %q", stdout)
	}
}

// Every `apps info relationships` subcommand selects the app info the same
// way, so `--id` works on view as it does on its six siblings.
func TestAppsInfoRelationshipsViewAcceptsIDAlias(t *testing.T) {
	reg := asctest.NewRegistry(t)
	reg.Register(asctest.GET("/v1/appInfos/info-1", "include=primaryCategory,primarySubcategoryOne,primarySubcategoryTwo,secondaryCategory,secondarySubcategoryOne,secondarySubcategoryTwo"),
		asctest.JSON(http.StatusOK, `{"data":{"type":"appInfos","id":"info-1"},"included":[{"type":"appCategories","id":"PRODUCTIVITY"}]}`))

	stdout, _, err := runRootWithDeps(t, stubClient(t, reg), "apps", "info", "relationships", "view", "--id", "info-1", "--output", "json")
	if err != nil {
		t.Fatalf("run error: %v", err)
	}
	if !strings.Contains(stdout, `"id":"info-1"`) {
		t.Fatalf("expected app info info-1 in output, got %q", stdout)
	}
}
