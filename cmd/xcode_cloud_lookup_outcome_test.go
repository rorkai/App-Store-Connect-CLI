package cmd

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/telemetry"
)

func TestRunXcodeCloudLookupFailuresAreClassified(t *testing.T) {
	const (
		oneProduct   = `{"data":[{"type":"ciProducts","id":"prod-1","attributes":{"name":"App"}}],"links":{}}`
		noProducts   = `{"data":[],"links":{}}`
		oneWorkflow  = `{"data":[{"type":"ciWorkflows","id":"wf-1","attributes":{"name":"CI"}}],"links":{}}`
		noWorkflows  = `{"data":[],"links":{}}`
		twoWorkflows = `{"data":[{"type":"ciWorkflows","id":"wf-1","attributes":{"name":"CI"}},{"type":"ciWorkflows","id":"wf-2","attributes":{"name":"ci"}}],"links":{}}`
		mainRef      = `{"data":[{"type":"scmGitReferences","id":"ref-1","attributes":{"name":"main","canonicalName":"refs/heads/main"}}],"links":{}}`
		twoMainRefs  = `{"data":[{"type":"scmGitReferences","id":"ref-1","attributes":{"name":"main","canonicalName":"refs/heads/main"}},{"type":"scmGitReferences","id":"ref-2","attributes":{"name":"main","canonicalName":"refs/tags/main"}}],"links":{}}`
	)
	runArgs := func(workflow, branch string) []string {
		return []string{"xcode-cloud", "run", "--app", "123456789", "--workflow", workflow, "--branch", branch, "--output", "json"}
	}

	tests := []struct {
		name           string
		args           []string
		products       string
		workflows      string
		gitRefs        string
		wantStderr     string
		wantDiagnostic shared.DiagnosticCode
		wantParameter  string
	}{
		{
			name:           "workflows list without an Xcode Cloud product",
			args:           []string{"xcode-cloud", "workflows", "list", "--app", "123456789", "--output", "json"},
			products:       noProducts,
			wantStderr:     "Error: xcode-cloud workflows: no Xcode Cloud product found for app \"123456789\" (ensure Xcode Cloud is enabled)\n",
			wantDiagnostic: shared.DiagnosticStateNotReady,
			wantParameter:  "--app",
		},
		{
			name:           "workflows without an Xcode Cloud product",
			args:           []string{"xcode-cloud", "workflows", "--app", "123456789", "--output", "json"},
			products:       noProducts,
			wantStderr:     "Error: xcode-cloud workflows: no Xcode Cloud product found for app \"123456789\" (ensure Xcode Cloud is enabled)\n",
			wantDiagnostic: shared.DiagnosticStateNotReady,
			wantParameter:  "--app",
		},
		{
			name:           "run without an Xcode Cloud product",
			args:           runArgs("CI", "main"),
			products:       noProducts,
			wantStderr:     "Error: xcode-cloud run: no Xcode Cloud product found for app \"123456789\" (ensure Xcode Cloud is enabled)\n",
			wantDiagnostic: shared.DiagnosticStateNotReady,
			wantParameter:  "--app",
		},
		{
			name:           "run with no matching workflow",
			args:           runArgs("Nightly", "main"),
			products:       oneProduct,
			workflows:      oneWorkflow,
			wantStderr:     "Error: xcode-cloud run: no workflow named \"Nightly\" found; available: CI\n",
			wantDiagnostic: shared.DiagnosticResourceNotFound,
			wantParameter:  "--workflow",
		},
		{
			name:           "run with a product that has no workflows",
			args:           runArgs("CI", "main"),
			products:       oneProduct,
			workflows:      noWorkflows,
			wantStderr:     "Error: xcode-cloud run: no Xcode Cloud workflows found for product \"prod-1\"\n",
			wantDiagnostic: shared.DiagnosticResourceNotFound,
			wantParameter:  "--workflow",
		},
		{
			name:           "run with an ambiguous workflow name",
			args:           runArgs("CI", "main"),
			products:       oneProduct,
			workflows:      twoWorkflows,
			wantStderr:     "Error: xcode-cloud run: multiple workflows named \"CI\" found; use --workflow-id with one of: wf-1, wf-2\n",
			wantDiagnostic: shared.DiagnosticInvalidInput,
			wantParameter:  "--workflow-id",
		},
		{
			name:           "run with no matching branch",
			args:           runArgs("CI", "release"),
			products:       oneProduct,
			workflows:      oneWorkflow,
			gitRefs:        mainRef,
			wantStderr:     "Error: xcode-cloud run: no git reference named \"release\" found; use --git-reference-id to specify directly\n",
			wantDiagnostic: shared.DiagnosticResourceNotFound,
			wantParameter:  "--branch",
		},
		{
			name:           "run with an ambiguous branch name",
			args:           runArgs("CI", "main"),
			products:       oneProduct,
			workflows:      oneWorkflow,
			gitRefs:        twoMainRefs,
			wantStderr:     "Error: xcode-cloud run: multiple git references match \"main\"; use --git-reference-id with one of: ref-1 (refs/heads/main), ref-2 (refs/tags/main)\n",
			wantDiagnostic: shared.DiagnosticInvalidInput,
			wantParameter:  "--git-reference-id",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			resetReportFlags(t)
			t.Setenv("ASC_APP_ID", "")

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				body := ""
				switch {
				case req.Method == http.MethodGet && req.URL.Path == "/v1/ciProducts":
					body = test.products
				case req.Method == http.MethodGet && req.URL.Path == "/v1/ciProducts/prod-1/workflows":
					body = test.workflows
				case req.Method == http.MethodGet && req.URL.Path == "/v1/ciWorkflows/wf-1/repository":
					body = `{"data":{"type":"scmRepositories","id":"repo-1"}}`
				case req.Method == http.MethodGet && req.URL.Path == "/v1/scmRepositories/repo-1/gitReferences":
					body = test.gitRefs
				}
				if body == "" {
					t.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
					w.WriteHeader(http.StatusNotFound)
					return
				}
				fmt.Fprint(w, body)
			}))
			defer server.Close()

			client := newHTTPStatusTestClient(t, server.URL)
			t.Cleanup(shared.SetASCClientFactoryForTesting(func() (*asc.Client, error) { return client, nil }))

			originalEmitTelemetry := emitTelemetry
			t.Cleanup(func() { emitTelemetry = originalEmitTelemetry })
			var gotExitCode int
			var gotContext telemetry.EventContext
			emitTelemetry = func(_ string, _ string, _ time.Duration, exitCode int, eventContext telemetry.EventContext) {
				gotExitCode = exitCode
				gotContext = eventContext
			}

			stdout, stderr := captureCommandOutput(t, func() {
				if code := Run(test.args, "5.15.0"); code != ExitError {
					t.Fatalf("Run() exit code = %d, want %d", code, ExitError)
				}
			})

			if stdout != "" {
				t.Fatalf("stdout = %q, want empty", stdout)
			}
			if !strings.HasSuffix(stderr, test.wantStderr) {
				t.Fatalf("stderr = %q, want suffix %q", stderr, test.wantStderr)
			}
			if gotExitCode != ExitError ||
				gotContext.OutcomeKind != telemetry.OutcomeExpectedNegative ||
				gotContext.DiagnosticCode != string(test.wantDiagnostic) ||
				gotContext.FailureParameter != test.wantParameter {
				t.Fatalf("unexpected telemetry: exit=%d context=%+v", gotExitCode, gotContext)
			}
		})
	}
}
