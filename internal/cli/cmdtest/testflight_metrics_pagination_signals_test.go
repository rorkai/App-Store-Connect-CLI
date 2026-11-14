package cmdtest

import (
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func TestTestFlightMetricsAppTestersSinglePageWarnsOnMorePages(t *testing.T) {
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	keyPath := filepath.Join(t.TempDir(), "AuthKey.p8")
	writeECDSAPEM(t, keyPath)
	t.Setenv("ASC_KEY_ID", "TEST_KEY")
	t.Setenv("ASC_ISSUER_ID", "TEST_ISSUER")
	t.Setenv("ASC_PRIVATE_KEY_PATH", keyPath)

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/apps/app-123/metrics/betaTesterUsages", "groupBy=betaTesters"),
		asctest.JSON(http.StatusOK, `{"data":[{"id":"usage-1"}],"links":{"next":"https://api.appstoreconnect.apple.com/v1/apps/app-123/metrics/betaTesterUsages?page=2"}}`))

	stdout, stderr, err := runCommand(t, []string{"testflight", "metrics", "app-testers", "--app", "app-123"})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if !strings.Contains(stdout, `"usage-1"`) {
		t.Fatalf("expected metric data in stdout, got %q", stdout)
	}
	wantWarning := "Warning: showing 1 results; more pages exist (use --paginate or --next where supported)\n"
	if stderr != wantWarning {
		t.Fatalf("stderr = %q, want %q", stderr, wantWarning)
	}
}

func TestTestFlightMetricsGroupTestersSinglePageWarnsWithPagingTotal(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/betaGroups/group-1/metrics/betaTesterUsages", "groupBy=betaTesters"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"appsBetaTesterUsages","dataPoints":[{"start":"2026-08-01T00:00:00Z","end":"2026-08-02T00:00:00Z","values":{"sessionCount":12}}],"dimensions":{"betaTesters":{"data":{"type":"betaTesters","id":"tester-1"}}}}],"links":{"next":"https://api.example.test/v1/betaGroups/group-1/metrics/betaTesterUsages?cursor=next"},"meta":{"paging":{"total":7,"limit":1}}}`))

	stdout, stderr, err := runCommand(t, []string{"testflight", "metrics", "group-testers", "--group", "group-1"})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if !strings.Contains(stdout, `"tester-1"`) {
		t.Fatalf("expected metric data in stdout, got %q", stdout)
	}
	wantWarning := "Warning: showing 1 of 7 results; more pages exist (use --paginate or --next where supported)\n"
	if stderr != wantWarning {
		t.Fatalf("stderr = %q, want %q", stderr, wantWarning)
	}
}

func TestTestFlightMetricsResolveTestersMergesIncludedAcrossPages(t *testing.T) {
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	keyPath := filepath.Join(t.TempDir(), "AuthKey.p8")
	writeECDSAPEM(t, keyPath)
	t.Setenv("ASC_KEY_ID", "TEST_KEY")
	t.Setenv("ASC_ISSUER_ID", "TEST_ISSUER")
	t.Setenv("ASC_PRIVATE_KEY_PATH", keyPath)

	secondURL := "https://api.appstoreconnect.apple.com/v1/apps/app-123/metrics/betaTesterUsages?page=2"

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/apps/app-123/metrics/betaTesterUsages", "groupBy=betaTesters", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"dataPoints":[{"start":"2026-05-20","end":"2026-08-18","values":{"sessionCount":3}}],"dimensions":{"betaTesters":{"data":"tester-1"}}}],"included":[{"type":"betaTesters","id":"tester-1","attributes":{"firstName":"Ada","lastName":"Lovelace","email":"ada@example.com","state":"INSTALLED"}}],"links":{"next":"`+secondURL+`"}}`))
	reg.Register(asctest.GET("/v1/apps/app-123/metrics/betaTesterUsages", "page=2"),
		asctest.JSON(http.StatusOK, `{"data":[{"dataPoints":[{"start":"2026-05-20","end":"2026-08-18","values":{"sessionCount":9}}],"dimensions":{"betaTesters":{"data":"tester-2"}}}],"included":[{"type":"betaTesters","id":"tester-2","attributes":{"firstName":"Grace","lastName":"Hopper","email":"grace@example.com","state":"INVITED"}}],"links":{"next":""}}`))

	stdout, _, err := runCommand(t, []string{"testflight", "metrics", "app-testers", "--app", "app-123", "--paginate", "--resolve-testers"})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if callCount := len(reg.Requests()); callCount != 2 {
		t.Fatalf("expected exactly 2 metric page fetches, got %d", callCount)
	}
	for _, want := range []string{`"ada@example.com"`, `"grace@example.com"`, `"tester-1"`, `"tester-2"`} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("expected %s in resolved output, got %q", want, stdout)
		}
	}
}
