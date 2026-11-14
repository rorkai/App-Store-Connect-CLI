package cmdtest

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func TestTestFlightConfigExportResolvesAppByBundleID(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	outputPath := filepath.Join(t.TempDir(), "sync.yaml")

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/apps", "filter[bundleId]=com.example.sync", "limit=2"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"apps","id":"app-sync"}]}`))
	reg.Register(asctest.GET("/v1/apps/app-sync/betaGroups", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[],"links":{"next":""}}`))
	reg.Register(asctest.GET("/v1/apps/app-sync"),
		asctest.JSON(http.StatusOK, `{"data":{"type":"apps","id":"app-sync","attributes":{"name":"Sync Demo","bundleId":"com.example.sync"}}}`))

	stdout, stderr, err := runCommand(t, []string{"testflight", "config", "export", "--app", "com.example.sync", "--output", outputPath})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"app":"Sync Demo"`) {
		t.Fatalf("expected summary to contain app name, got %q", stdout)
	}
	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("expected sync output file, got error: %v", err)
	}
	if !strings.Contains(string(data), "id: app-sync") {
		t.Fatalf("expected sync yaml to contain app id, got %q", string(data))
	}
}

func TestTestFlightConfigExportLookupNotFoundByName(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	outputPath := filepath.Join(t.TempDir(), "sync.yaml")

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/apps", "filter[bundleId]=Missing App", "limit=2"),
		asctest.JSON(http.StatusOK, `{"data":[]}`))
	reg.Register(asctest.GET("/v1/apps", "filter[name]=Missing App", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[]}`))
	reg.Register(asctest.GET("/v1/apps", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[]}`))
	reg.Register(asctest.GET("/v1/apps", "filter[name]=Missing App", "limit=2"),
		asctest.JSON(http.StatusOK, `{"data":[]}`))

	stdout, _, runErr := runCommand(t, []string{"testflight", "config", "export", "--app", "Missing App", "--output", outputPath})

	if runErr == nil {
		t.Fatal("expected lookup not found error, got nil")
	}
	if !strings.Contains(runErr.Error(), `app "Missing App" not found`) {
		t.Fatalf("expected not found lookup error, got %v", runErr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout on lookup failure, got %q", stdout)
	}
	if _, err := os.Stat(outputPath); err == nil {
		t.Fatalf("expected no output file on lookup failure, but file exists: %s", outputPath)
	}
}

func TestTestFlightConfigExportLookupAmbiguousName(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	outputPath := filepath.Join(t.TempDir(), "sync.yaml")

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/apps", "filter[bundleId]=Ambiguous App", "limit=2"),
		asctest.JSON(http.StatusOK, `{"data":[]}`))
	reg.Register(asctest.GET("/v1/apps", "filter[name]=Ambiguous App", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"apps","id":"app-1","attributes":{"name":"Ambiguous App"}},{"type":"apps","id":"app-2","attributes":{"name":"Ambiguous App"}}]}`))

	stdout, _, runErr := runCommand(t, []string{"testflight", "config", "export", "--app", "Ambiguous App", "--output", outputPath})

	if runErr == nil {
		t.Fatal("expected ambiguous lookup error, got nil")
	}
	if !strings.Contains(runErr.Error(), `2 apps match "Ambiguous App"; pass --app with one of:`) {
		t.Fatalf("expected ambiguous name error, got %v", runErr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout on lookup failure, got %q", stdout)
	}
	if _, err := os.Stat(outputPath); err == nil {
		t.Fatalf("expected no output file on lookup failure, but file exists: %s", outputPath)
	}
}
