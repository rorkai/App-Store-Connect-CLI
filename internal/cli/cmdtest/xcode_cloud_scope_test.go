package cmdtest

import (
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func TestXcodeCloudNamedRunBindsSelectedWorkflowContinuationBeforeBuild(t *testing.T) {
	tests := []struct {
		name    string
		nextURL string
		wantErr bool
	}{
		{
			name:    "same selected product",
			nextURL: asc.BaseURL + "/v1/ciProducts/product-1/workflows?cursor=next",
		},
		{
			name:    "drifted product",
			nextURL: "/v1/ciProducts/other/workflows?cursor=next",
			wantErr: true,
		},
		{
			name:    "redirected origin",
			nextURL: "https://evil.example/v1/ciProducts/product-1/workflows?cursor=next",
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_APP_ID", "")
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

			reg := asctest.NewRegistry(t)
			reg.Register(asctest.GET("/v1/ciProducts", "filter[app]=123456789", "limit=200"),
				asctest.JSON(http.StatusOK, `{"data":[{"type":"ciProducts","id":"product-1"}]}`))
			reg.Register(asctest.GET("/v1/ciProducts/product-1/workflows", "limit=200"),
				asctest.JSON(http.StatusOK, `{"data":[{"type":"ciWorkflows","id":"workflow-1","attributes":{"name":"Deploy"}}],"links":{"next":"`+test.nextURL+`"}}`))
			if !test.wantErr {
				reg.Register(asctest.GET("/v1/ciProducts/product-1/workflows", "cursor=next"),
					asctest.JSON(http.StatusOK, `{"data":[{"type":"ciWorkflows","id":"workflow-1","attributes":{"name":"CI"}}],"links":{}}`))
				reg.Register(asctest.GET("/v1/ciWorkflows/workflow-1/repository"),
					asctest.JSON(http.StatusOK, `{"data":{"type":"scmRepositories","id":"repo-1"}}`))
				reg.Register(asctest.GET("/v1/scmRepositories/repo-1/gitReferences", "limit=200"),
					asctest.JSON(http.StatusOK, `{"data":[{"type":"scmGitReferences","id":"ref-1","attributes":{"name":"deploy","canonicalName":"refs/heads/deploy"}}],"links":{"next":"`+asc.BaseURL+`/v1/scmRepositories/repo-1/gitReferences?cursor=next"}}`))
				reg.Register(asctest.GET("/v1/scmRepositories/repo-1/gitReferences", "cursor=next"),
					asctest.JSON(http.StatusOK, `{"data":[{"type":"scmGitReferences","id":"ref-1","attributes":{"name":"main","canonicalName":"refs/heads/main"}}],"links":{}}`))
				reg.Register(asctest.POST("/v1/ciBuildRuns").JSONBody(`{"data":{"type":"ciBuildRuns","relationships":{"workflow":{"data":{"type":"ciWorkflows","id":"workflow-1"}},"sourceBranchOrTag":{"data":{"type":"scmGitReferences","id":"ref-1"}}}}}`),
					asctest.JSON(http.StatusCreated, `{"data":{"type":"ciBuildRuns","id":"run-1","attributes":{"number":1,"executionProgress":"PENDING","startReason":"MANUAL","createdDate":"2026-03-14T00:00:00Z"}}}`))
			}

			stdout, _, err := runRootWithDeps(t, stubClient(t, reg), "xcode-cloud", "run", "--app", "123456789", "--workflow", "CI", "--branch", "main")
			requests := reg.Requests()
			if test.wantErr {
				if err == nil || !strings.Contains(err.Error(), "rejected scoped pagination URL") {
					t.Fatalf("expected scoped continuation error, got %v", err)
				}
				if stdout != "" {
					t.Fatalf("expected empty stdout, got %q", stdout)
				}
				for _, request := range requests {
					if request.Method == http.MethodPost {
						t.Fatal("build POST sent after scoped continuation refusal")
					}
				}
				return
			}
			if err != nil {
				t.Fatalf("run error: %v", err)
			}
			if !strings.Contains(stdout, `"buildRunId":"run-1"`) {
				t.Fatalf("expected build run output, got %q", stdout)
			}
			if len(requests) != 7 {
				t.Fatalf("request count = %d, want 7", len(requests))
			}
		})
	}
}

func TestXcodeCloudSelectedWorkflowListBindsContinuation(t *testing.T) {
	const collection = "/v1/ciProducts/product-1/workflows"
	for _, test := range []struct {
		name string
		next string
		ok   bool
	}{
		{name: "same product", next: asc.BaseURL + collection + "?cursor=next", ok: true},
		{name: "different product", next: "/v1/ciProducts/other/workflows?cursor=next"},
		{name: "different origin", next: "https://fixture.invalid" + collection + "?cursor=next"},
	} {
		t.Run(test.name, func(t *testing.T) {
			reg := asctest.NewRegistry(t)
			reg.Register(asctest.GET("/v1/ciProducts", "filter[app]=123456789", "limit=200"),
				asctest.JSON(http.StatusOK, `{"data":[{"type":"ciProducts","id":"product-1"}]}`))
			reg.Register(asctest.GET(collection, "limit=200"),
				asctest.JSON(http.StatusOK, `{"data":[{"type":"ciWorkflows","id":"workflow-1"}],"links":{"next":"`+test.next+`"}}`))
			if test.ok {
				reg.Register(asctest.GET(collection, "cursor=next"),
					asctest.JSON(http.StatusOK, `{"data":[{"type":"ciWorkflows","id":"workflow-2"}]}`))
			}
			stdout, _, err := runRootWithDeps(t, stubClient(t, reg), "xcode-cloud", "workflows", "list", "--app", "123456789", "--paginate")
			count := len(reg.Requests())
			if test.ok {
				if err != nil || count != 3 || !strings.Contains(stdout, `"id":"workflow-2"`) {
					t.Fatalf("stdout=%q err=%v requests=%d, want paginated workflows", stdout, err, count)
				}
			} else if err == nil || !strings.Contains(err.Error(), "rejected scoped pagination URL") || stdout != "" || count != 2 {
				t.Fatalf("stdout=%q err=%v requests=%d, want refusal before follow-up", stdout, err, count)
			}
		})
	}
}

func TestXcodeCloudWorkflowIDBindsNamedGitReference(t *testing.T) {
	const collection = "/v1/scmRepositories/repo-1/gitReferences"
	for _, test := range []struct {
		name string
		next string
		ok   bool
	}{
		{name: "same repository", next: asc.BaseURL + collection + "?cursor=next", ok: true},
		{name: "different repository", next: "/v1/scmRepositories/other/gitReferences?cursor=next"},
	} {
		t.Run(test.name, func(t *testing.T) {
			reg := asctest.NewRegistry(t)
			reg.Register(asctest.GET("/v1/ciWorkflows/workflow-1/repository"),
				asctest.JSON(http.StatusOK, `{"data":{"type":"scmRepositories","id":"repo-1"}}`))
			reg.Register(asctest.GET(collection, "limit=200"),
				asctest.JSON(http.StatusOK, `{"data":[],"links":{"next":"`+test.next+`"}}`))
			if test.ok {
				reg.Register(asctest.GET(collection, "cursor=next"),
					asctest.JSON(http.StatusOK, `{"data":[{"type":"scmGitReferences","id":"ref-1","attributes":{"name":"main","canonicalName":"refs/heads/main"}}]}`))
				reg.Register(asctest.POST("/v1/ciBuildRuns"),
					asctest.JSON(http.StatusCreated, `{"data":{"type":"ciBuildRuns","id":"run-1"}}`))
			}
			stdout, _, err := runRootWithDeps(t, stubClient(t, reg), "xcode-cloud", "run", "--workflow-id", "workflow-1", "--branch", "main")
			count := len(reg.Requests())
			if test.ok {
				if err != nil || count != 4 || !strings.Contains(stdout, `"buildRunId":"run-1"`) {
					t.Fatalf("stdout=%q err=%v requests=%d, want build after matching repository", stdout, err, count)
				}
			} else if err == nil || !strings.Contains(err.Error(), "rejected scoped pagination URL") || stdout != "" || count != 2 {
				t.Fatalf("stdout=%q err=%v requests=%d, want refusal before build", stdout, err, count)
			}
		})
	}
}

func TestXcodeCloudWorkflowListCursorRejectsExplicitApp(t *testing.T) {
	reg := asctest.NewRegistry(t)
	stdout, stderr, err := runRootWithDeps(t, stubClient(t, reg), "xcode-cloud", "workflows", "list",
		"--app", "123456789", "--next", asc.BaseURL+"/v1/ciProducts/product-1/workflows?cursor=next")
	if err == nil || stdout != "" || !strings.Contains(err.Error()+stderr, "--app") || !strings.Contains(err.Error()+stderr, "--next") {
		t.Fatalf("stdout=%q stderr=%q err=%v, want conflicting selectors", stdout, stderr, err)
	}
	if count := len(reg.Requests()); count != 0 {
		t.Fatalf("requests=%d, want zero", count)
	}
}
