package cmdtest

import (
	"errors"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func TestBuildsBetaAppReviewSubmissionViewPrintsAppleEnvelopeWhenNoSubmissionExists(t *testing.T) {
	const body = `{"data":null,"links":{"self":"https://api.appstoreconnect.apple.com/v1/builds/build-1/betaAppReviewSubmission"}}`
	const wantStderr = "No beta app review submission exists for build build-1. Submit it with: asc testflight review submit --build-id \"build-1\" --confirm\n"

	tests := []struct {
		name       string
		output     string
		wantStdout []string
	}{
		{name: "json prints Apple's envelope unchanged", output: "json", wantStdout: []string{body + "\n"}},
		{name: "table shows none submitted", output: "table", wantStdout: []string{"build-1", "none submitted"}},
		{name: "markdown shows none submitted", output: "markdown", wantStdout: []string{"| build-1", "none submitted"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

			reg := asctest.NewRegistry(t)
			reg.Install()
			reg.Register(asctest.GET("/v1/builds/build-1/betaAppReviewSubmission"), asctest.JSON(http.StatusOK, body))

			stdout, stderr, runErr := runCommand(t, []string{"builds", "beta-app-review-submission", "view", "--build-id", "build-1", "--output", tt.output})
			if runErr != nil {
				t.Fatalf("expected success (exit 0), got %v (exit %d)", runErr, cmd.ExitCodeFromError(runErr))
			}
			if tt.output == "json" {
				if stdout != tt.wantStdout[0] {
					t.Fatalf("expected Apple's envelope %q, got %q", tt.wantStdout[0], stdout)
				}
			} else {
				for _, want := range tt.wantStdout {
					if !strings.Contains(stdout, want) {
						t.Fatalf("expected stdout to contain %q, got %q", want, stdout)
					}
				}
			}
			if stderr != wantStderr {
				t.Fatalf("expected stderr %q, got %q", wantStderr, stderr)
			}
		})
	}
}

func TestBuildsAddGroupsSubmitCreatesBetaReviewSubmissionWhenLookupDataIsNull(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	reg := asctest.NewRegistry(t)
	reg.Install()
	registerAddGroupsLookups(reg, addGroupsExternalQABody)
	registerAddGroupsExternalPreflight(reg, "READY_FOR_BETA_TESTING")
	registerAddGroupsPost(reg, "group-external")
	reg.Register(asctest.GET("/v1/builds/build-1/betaAppReviewSubmission"),
		asctest.JSON(http.StatusOK, `{"data":null,"links":{"self":"https://api.appstoreconnect.apple.com/v1/builds/build-1/betaAppReviewSubmission"}}`))
	reg.Register(asctest.POST("/v1/betaAppReviewSubmissions").JSONBody(`{"data":{"type":"betaAppReviewSubmissions","relationships":{"build":{"data":{"type":"builds","id":"build-1"}}}}}`),
		asctest.JSON(http.StatusCreated, `{"data":{"type":"betaAppReviewSubmissions","id":"submission-1"}}`))

	stdout, stderr, err := runCommand(t, addGroupsSubmitArgs)
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	assertBuildsRequestOrder(t, reg,
		"GET /v1/builds/build-1/app",
		"GET /v1/apps/app-1/betaGroups",
		"GET /v1/builds/build-1",
		"GET /v1/builds/build-1/buildBetaDetail",
		"POST /v1/builds/build-1/relationships/betaGroups",
		"GET /v1/builds/build-1/betaAppReviewSubmission",
		"POST /v1/betaAppReviewSubmissions",
	)
	if !strings.Contains(stdout, `"groupIds":["group-external"]`) {
		t.Fatalf("expected external group in output, got %q", stdout)
	}
	if !strings.Contains(stderr, "Successfully added 1 group(s) to build build-1") {
		t.Fatalf("expected add-groups success message, got %q", stderr)
	}
	if !strings.Contains(stderr, "Submitted build build-1 for beta app review (submission-1)") {
		t.Fatalf("expected beta review submission message, got %q", stderr)
	}
}

func TestBuildsBetaAppReviewSubmissionViewPreservesAPI404Context(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/builds/build-404/betaAppReviewSubmission"),
		asctest.JSON(http.StatusNotFound, `{"errors":[{"status":"404","code":"NOT_FOUND","title":"Not Found","detail":"build not found"}]}`))

	stdout, stderr, runErr := runCommand(t, []string{"builds", "beta-app-review-submission", "view", "--build-id", "build-404", "--output", "json"})

	if runErr == nil {
		t.Fatal("expected not-found error")
	}
	if !errors.Is(runErr, asc.ErrNotFound) {
		t.Fatalf("expected asc.ErrNotFound, got %v", runErr)
	}
	if got := cmd.ExitCodeFromError(runErr); got != cmd.ExitNotFound {
		t.Fatalf("expected exit code %d, got %d", cmd.ExitNotFound, got)
	}
	if strings.Contains(runErr.Error(), "no beta app review submission found") {
		t.Fatalf("expected upstream 404 context, got %v", runErr)
	}
	if !strings.Contains(runErr.Error(), "builds beta-app-review-submission view: failed to fetch: Not Found: build not found") {
		t.Fatalf("expected API 404 context, got %v", runErr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
}
