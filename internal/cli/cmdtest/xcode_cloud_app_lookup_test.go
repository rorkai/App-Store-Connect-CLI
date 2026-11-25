package cmdtest

import (
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func TestXcodeCloudWorkflowsResolvesAppByBundleID(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	reg := asctest.NewRegistry(t)
	reg.Register(asctest.GET("/v1/apps", "filter[bundleId]=com.example.lookup", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"apps","id":"app-lookup","attributes":{"bundleId":"com.example.lookup"}}]}`))
	reg.Register(asctest.GET("/v1/ciProducts", "filter[app]=app-lookup", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"ciProducts","id":"prod-1"}]}`))
	reg.Register(asctest.GET("/v1/ciProducts/prod-1/workflows"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"ciWorkflows","id":"wf-1","attributes":{"name":"CI"}}]}`))

	stdout, stderr, err := runRootWithDeps(t, stubClient(t, reg), "xcode-cloud", "workflows", "--app", "com.example.lookup")
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"id":"wf-1"`) {
		t.Fatalf("expected workflow output, got %q", stdout)
	}
}

func TestXcodeCloudProductsResolvesAppByExactName(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	reg := asctest.NewRegistry(t)
	reg.Register(asctest.GET("/v1/apps", "filter[bundleId]=Focus Rail", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[]}`))
	reg.Register(asctest.GET("/v1/apps", "filter[name]=Focus Rail", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"apps","id":"app-lookup","attributes":{"name":"Focus Rail"}}]}`))
	reg.Register(asctest.GET("/v1/ciProducts", "filter[app]=app-lookup"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"ciProducts","id":"prod-1","attributes":{"name":"Focus Rail"}}]}`))

	stdout, stderr, err := runRootWithDeps(t, stubClient(t, reg), "xcode-cloud", "products", "--app", "Focus Rail")
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"id":"prod-1"`) {
		t.Fatalf("expected product output, got %q", stdout)
	}
}

func TestXcodeCloudRunResolvesAppByExactNameWhenWorkflowNameProvided(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	reg := asctest.NewRegistry(t)
	reg.Register(asctest.GET("/v1/apps", "filter[bundleId]=Focus Rail", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[]}`))
	reg.Register(asctest.GET("/v1/apps", "filter[name]=Focus Rail", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"apps","id":"app-lookup","attributes":{"name":"Focus Rail"}}]}`))
	reg.Register(asctest.GET("/v1/ciProducts", "filter[app]=app-lookup", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"ciProducts","id":"prod-1"}]}`))
	reg.Register(asctest.GET("/v1/ciProducts/prod-1/workflows", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"ciWorkflows","id":"wf-1","attributes":{"name":"CI"}}],"links":{"next":""}}`))
	reg.Register(asctest.GET("/v1/ciWorkflows/wf-1/repository"),
		asctest.JSON(http.StatusOK, `{"data":{"type":"scmRepositories","id":"repo-1"}}`))
	reg.Register(asctest.GET("/v1/scmRepositories/repo-1/gitReferences", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"scmGitReferences","id":"ref-1","attributes":{"name":"main","canonicalName":"refs/heads/main"}}],"links":{"next":""}}`))
	reg.Register(asctest.POST("/v1/ciBuildRuns").JSONBody(`{"data":{"type":"ciBuildRuns","relationships":{"workflow":{"data":{"type":"ciWorkflows","id":"wf-1"}},"sourceBranchOrTag":{"data":{"type":"scmGitReferences","id":"ref-1"}}}}}`),
		asctest.JSON(http.StatusCreated, `{"data":{"type":"ciBuildRuns","id":"run-1","attributes":{"number":1,"executionProgress":"PENDING","startReason":"MANUAL","createdDate":"2026-03-14T00:00:00Z"}}}`))

	stdout, stderr, err := runRootWithDeps(t, stubClient(t, reg), "xcode-cloud", "run", "--app", "Focus Rail", "--workflow", "CI", "--branch", "main")
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"buildRunId":"run-1"`) {
		t.Fatalf("expected build run output, got %q", stdout)
	}
}

func TestXcodeCloudProductsRejectsUniqueFuzzyAppName(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	reg := asctest.NewRegistry(t)
	reg.Register(asctest.GET("/v1/apps", "filter[bundleId]=Relax", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[]}`))
	reg.Register(asctest.GET("/v1/apps", "filter[name]=Relax", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"apps","id":"app-fuzzy","attributes":{"name":"Relax: Sleep + Focus"}}],"links":{"next":""}}`))
	reg.Register(asctest.GET("/v1/apps", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"apps","id":"app-fuzzy","attributes":{"name":"Relax: Sleep + Focus"}}],"links":{"next":""}}`))

	stdout, stderr, runErr := runRootWithDeps(t, stubClient(t, reg), "xcode-cloud", "products", "--app", "Relax")

	if runErr == nil {
		t.Fatal("expected fuzzy app-name lookup to fail")
	}
	if !strings.Contains(runErr.Error(), `app "Relax" not found`) {
		t.Fatalf("expected strict not-found error, got %v", runErr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if callCount := len(reg.Requests()); callCount != 3 {
		t.Fatalf("expected exactly 3 lookup requests, got %d", callCount)
	}
}

func TestXcodeCloudProductsTreatsNumericAppAsIDBeforeExactNameLookup(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	reg := asctest.NewRegistry(t)
	reg.Register(asctest.GET("/v1/ciProducts", "filter[app]=2048"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"ciProducts","id":"prod-2048","attributes":{"name":"2048"}}]}`))

	stdout, stderr, err := runRootWithDeps(t, stubClient(t, reg), "xcode-cloud", "products", "--app", "2048")
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"id":"prod-2048"`) {
		t.Fatalf("expected product output, got %q", stdout)
	}
	if callCount := len(reg.Requests()); callCount != 1 {
		t.Fatalf("expected exactly 1 request, got %d", callCount)
	}
}
