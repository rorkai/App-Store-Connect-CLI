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
	reg.Install()
	reg.Register(asctest.GET("/v1/apps/app-1/appInfos"), asctest.JSON(http.StatusOK, `{
				"data":[
					{"type":"appInfos","id":"info-live","attributes":{"state":"READY_FOR_DISTRIBUTION"}},
					{"type":"appInfos","id":"info-rejected","attributes":{"state":"REJECTED"}}
				]
			}`))

	stdout, stderr, runErr := runCommand(t, []string{"apps", "info", "view", "--app", "app-1", "--include", "primaryCategory", "--output", "json"})

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
		`2 app infos match app "app-1"; pass --info-id with one of:`,
		`asc apps info list --app "app-1"`,
		"READY_FOR_DISTRIBUTION",
		"REJECTED",
	} {
		if !strings.Contains(runErr.Error(), want) {
			t.Fatalf("expected error to contain %q, got %v", want, runErr)
		}
	}
}
