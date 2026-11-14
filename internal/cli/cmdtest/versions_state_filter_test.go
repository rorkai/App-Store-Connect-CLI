package cmdtest

import (
	"context"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func TestVersionsListReadyForDistributionUsesAppVersionStateFilter(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/apps/123456789/appStoreVersions", "filter[appVersionState]=READY_FOR_DISTRIBUTION"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"version-live","attributes":{"versionString":"1.0","platform":"IOS","appVersionState":"READY_FOR_DISTRIBUTION"}}]}`))

	stdout, stderr, err := runCommand(t, []string{"versions", "list", "--app", "123456789", "--state", "READY_FOR_DISTRIBUTION"})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"id":"version-live"`) {
		t.Fatalf("expected version output, got %q", stdout)
	}
}

func TestVersionsListMixedReadyStatesUsesSingleAppVersionStateFilter(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/apps/123456789/appStoreVersions", "filter[appVersionState]=READY_FOR_REVIEW,READY_FOR_DISTRIBUTION"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"version-review","attributes":{"versionString":"1.1","platform":"IOS","appVersionState":"READY_FOR_REVIEW"}},{"type":"appStoreVersions","id":"version-live","attributes":{"versionString":"1.0","platform":"IOS","appVersionState":"READY_FOR_DISTRIBUTION"}}]}`))

	stdout, stderr, err := runCommand(t, []string{"versions", "list", "--app", "123456789", "--state", "READY_FOR_REVIEW,READY_FOR_DISTRIBUTION"})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"id":"version-review"`) || !strings.Contains(stdout, `"id":"version-live"`) {
		t.Fatalf("expected both versions in output, got %q", stdout)
	}
}

func TestVersionsListRejectsMixedAppStoreAndAppVersionOnlyStates(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	if err := root.Parse([]string{"versions", "list", "--app", "123456789", "--state", "READY_FOR_SALE,READY_FOR_DISTRIBUTION"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	err := root.Run(context.Background())
	if err == nil {
		t.Fatal("expected mixed state filter error")
	}
	if !strings.Contains(err.Error(), "cannot mix appVersionState-only values with appStoreState-only values") {
		t.Fatalf("expected mixed state filter error, got %q", err.Error())
	}
}
