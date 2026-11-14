package cmdtest

import (
	"context"
	"errors"
	"flag"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func TestAppsContentRightsEditRequiresExplicitValue(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"apps", "content-rights", "edit", "--app", "app-1"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if !errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("expected ErrHelp, got %v", runErr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "--uses-third-party-content is required") {
		t.Fatalf("expected missing value guidance, got %q", stderr)
	}
}

func TestAppsContentRightsViewTableShowsDeclaration(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_PROFILE", "")

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/apps/app-1"), asctest.JSON(http.StatusOK, `{
				"data":{
					"type":"apps",
					"id":"app-1",
					"attributes":{
						"name":"Test App",
						"bundleId":"com.example.test",
						"sku":"TESTSKU",
						"contentRightsDeclaration":"DOES_NOT_USE_THIRD_PARTY_CONTENT"
					}
				}
			}`))

	stdout, stderr, runErr := runCommand(t, []string{"apps", "content-rights", "view", "--app", "app-1", "--output", "table"})

	if runErr != nil {
		t.Fatalf("run error: %v", runErr)
	}
	if !strings.Contains(stdout, "content_rights_declaration") {
		t.Fatalf("expected declaration row header, got %q", stdout)
	}
	if !strings.Contains(stdout, "DOES_NOT_USE_THIRD_PARTY_CONTENT") {
		t.Fatalf("expected declaration value in table output, got %q", stdout)
	}
	if stderr != "" {
		t.Fatalf("expected no stderr, got %q", stderr)
	}
}
