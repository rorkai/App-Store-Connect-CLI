package cmdtest

import (
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

const addGroupsExternalQABody = `{"data":[{"type":"betaGroups","id":"group-external","attributes":{"name":"External QA","isInternalGroup":false}}]}`

func registerAddGroupsExternalPreflight(reg *asctest.Registry, externalBuildState string) {
	reg.Register(asctest.GET("/v1/builds/build-1"),
		asctest.JSON(http.StatusOK, `{"data":{"type":"builds","id":"build-1","attributes":{"processingState":"VALID","expired":false,"buildAudienceType":"APP_STORE_ELIGIBLE","usesNonExemptEncryption":false}}}`))
	reg.Register(asctest.GET("/v1/builds/build-1/buildBetaDetail"),
		asctest.JSON(http.StatusOK, `{"data":{"type":"buildBetaDetails","id":"detail-1","attributes":{"externalBuildState":"`+externalBuildState+`"}}}`))
}

func registerAddGroupsPost(reg *asctest.Registry, groupID string) {
	reg.Register(asctest.POST("/v1/builds/build-1/relationships/betaGroups").JSONBody(`{"data":[{"type":"betaGroups","id":"`+groupID+`"}]}`),
		asctest.JSON(http.StatusNoContent, ""))
}

var addGroupsSubmitArgs = []string{
	"builds", "add-groups",
	"--build-id", "build-1",
	"--group", "group-external",
	"--submit",
	"--confirm",
}

func TestBuildsAddGroupsSubmitCreatesBetaReviewSubmissionForExternalGroups(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	reg := asctest.NewRegistry(t)
	reg.Install()
	registerAddGroupsLookups(reg, addGroupsExternalQABody)
	registerAddGroupsExternalPreflight(reg, "READY_FOR_BETA_SUBMISSION")
	registerAddGroupsPost(reg, "group-external")
	reg.Register(asctest.GET("/v1/builds/build-1/betaAppReviewSubmission"),
		asctest.JSON(http.StatusNotFound, `{"errors":[{"status":"404","code":"NOT_FOUND","title":"Not Found"}]}`))
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

func TestBuildsAddGroupsSubmitSkipsBetaReviewSubmissionForInternalGroups(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	reg := asctest.NewRegistry(t)
	reg.Install()
	registerAddGroupsLookups(reg, `{"data":[{"type":"betaGroups","id":"group-internal","attributes":{"name":"Friends & Family","isInternalGroup":true}}]}`)
	reg.Register(asctest.GET("/v1/builds/build-1"),
		asctest.JSON(http.StatusOK, `{"data":{"type":"builds","id":"build-1","attributes":{"processingState":"VALID","expired":false}}}`))
	registerAddGroupsPost(reg, "group-internal")

	stdout, stderr, err := runCommand(t, []string{
		"builds", "add-groups",
		"--build-id", "build-1",
		"--group", "group-internal",
		"--submit",
		"--confirm",
	})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	assertBuildsRequestOrder(t, reg,
		"GET /v1/builds/build-1/app",
		"GET /v1/apps/app-1/betaGroups",
		"GET /v1/builds/build-1",
		"POST /v1/builds/build-1/relationships/betaGroups",
	)
	if !strings.Contains(stdout, `"groupIds":["group-internal"]`) {
		t.Fatalf("expected internal group in output, got %q", stdout)
	}
	if !strings.Contains(stderr, "Successfully added 1 group(s) to build build-1") {
		t.Fatalf("expected add-groups success message, got %q", stderr)
	}
	if !strings.Contains(stderr, "Skipped beta app review submission for build build-1 because no external groups were added") {
		t.Fatalf("expected beta review skip message, got %q", stderr)
	}
}

func TestBuildsAddGroupsSubmitTreatsExistingSubmissionAsAlreadyDone(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	reg := asctest.NewRegistry(t)
	reg.Install()
	registerAddGroupsLookups(reg, addGroupsExternalQABody)
	registerAddGroupsExternalPreflight(reg, "READY_FOR_BETA_TESTING")
	registerAddGroupsPost(reg, "group-external")
	reg.Register(asctest.GET("/v1/builds/build-1/betaAppReviewSubmission"),
		asctest.JSON(http.StatusOK, `{"data":{"type":"betaAppReviewSubmissions","id":"submission-existing"}}`))

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
	)
	if !strings.Contains(stdout, `"groupIds":["group-external"]`) {
		t.Fatalf("expected external group in output, got %q", stdout)
	}
	if !strings.Contains(stderr, "Build build-1 already has beta app review submission submission-existing") {
		t.Fatalf("expected existing submission message, got %q", stderr)
	}
}

func TestBuildsAddGroupsSubmitPreservesPartialSuccessWhenSubmissionFails(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	reg := asctest.NewRegistry(t)
	reg.Install()
	registerAddGroupsLookups(reg, addGroupsExternalQABody)
	registerAddGroupsExternalPreflight(reg, "READY_FOR_BETA_TESTING")
	registerAddGroupsPost(reg, "group-external")
	reg.Register(asctest.GET("/v1/builds/build-1/betaAppReviewSubmission"),
		asctest.JSON(http.StatusNotFound, `{"errors":[{"status":"404","code":"NOT_FOUND","title":"Not Found"}]}`))
	reg.Register(asctest.POST("/v1/betaAppReviewSubmissions").JSONBody(`{"data":{"type":"betaAppReviewSubmissions","relationships":{"build":{"data":{"type":"builds","id":"build-1"}}}}}`),
		asctest.JSON(http.StatusServiceUnavailable, `{"errors":[{"status":"503","code":"SERVICE_UNAVAILABLE","title":"Service unavailable","detail":"beta review temporarily unavailable"}]}`))

	stdout, stderr, runErr := runCommand(t, addGroupsSubmitArgs)

	if runErr == nil {
		t.Fatal("expected error")
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
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(runErr.Error(), `builds add-groups: beta groups were added to build "build-1", but beta app review submission failed`) {
		t.Fatalf("expected partial-success error, got %v", runErr)
	}
	if strings.Contains(runErr.Error(), "failed to add groups") {
		t.Fatalf("did not expect misleading add-groups wrapper, got %v", runErr)
	}
	if !strings.Contains(runErr.Error(), "Service unavailable: beta review temporarily unavailable") {
		t.Fatalf("expected underlying submission error, got %v", runErr)
	}
}
