package cmdtest

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

type publishTestFlightSubmitOutput struct {
	BuildID                string   `json:"buildId"`
	GroupIDs               []string `json:"groupIds"`
	BetaReviewSubmitted    *bool    `json:"betaReviewSubmitted,omitempty"`
	BetaReviewSubmissionID string   `json:"betaReviewSubmissionId,omitempty"`
}

func TestPublishTestflightSubmitCreatesBetaReviewSubmissionForExternalGroups(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/apps/123/betaGroups", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"betaGroups","id":"group-external","attributes":{"name":"External QA","isInternalGroup":false}}]}`))
	reg.Register(asctest.GET("/v1/builds/build-1"),
		asctest.JSON(http.StatusOK, `{"data":{"type":"builds","id":"build-1","attributes":{"version":"42","processingState":"VALID"}}}`))
	reg.Register(asctest.POST("/v1/builds/build-1/relationships/betaGroups").JSONBody(`{"data":[{"type":"betaGroups","id":"group-external"}]}`),
		asctest.JSON(http.StatusNoContent, ""))
	reg.Register(asctest.GET("/v1/builds/build-1/betaAppReviewSubmission"),
		asctest.JSON(http.StatusNotFound, `{"errors":[{"status":"404","code":"NOT_FOUND","title":"Not Found"}]}`))
	reg.Register(asctest.POST("/v1/betaAppReviewSubmissions").JSONBody(`{"data":{"type":"betaAppReviewSubmissions","relationships":{"build":{"data":{"type":"builds","id":"build-1"}}}}}`),
		asctest.JSON(http.StatusCreated, `{"data":{"type":"betaAppReviewSubmissions","id":"submission-1"}}`))

	stdout, stderr, err := runCommand(t, []string{
		"publish", "testflight",
		"--app", "123",
		"--build-id", "build-1",
		"--group", "group-external",
		"--submit",
		"--confirm",
	})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if requestCount := len(reg.Requests()); requestCount != 5 {
		t.Fatalf("expected group lookup, build lookup, group add, submission lookup, and submission create; got %d requests", requestCount)
	}
	result := decodePublishTestFlightSubmitOutput(t, stdout)
	if result.BuildID != "build-1" {
		t.Fatalf("expected build-1 output, got %q", result.BuildID)
	}
	if !slices.Contains(result.GroupIDs, "group-external") {
		t.Fatalf("expected group-external in output, got %#v", result.GroupIDs)
	}
	if result.BetaReviewSubmitted == nil || !*result.BetaReviewSubmitted {
		t.Fatalf("expected betaReviewSubmitted=true, got %#v", result.BetaReviewSubmitted)
	}
	if result.BetaReviewSubmissionID != "submission-1" {
		t.Fatalf("expected submission-1 output, got %q", result.BetaReviewSubmissionID)
	}
	if !strings.Contains(stderr, "Submitted build build-1 for beta app review (submission-1)") {
		t.Fatalf("expected beta review submission message, got %q", stderr)
	}
}

func TestPublishTestflightSubmitWaitsForProcessingBeforeAddingGroups(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/apps/123/betaGroups", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"betaGroups","id":"group-external","attributes":{"name":"External QA","isInternalGroup":false}}]}`))
	reg.Register(asctest.GET("/v1/builds/build-1"),
		asctest.JSON(http.StatusOK, `{"data":{"type":"builds","id":"build-1","attributes":{"version":"42","processingState":"PROCESSING"}}}`))
	reg.Register(asctest.GET("/v1/builds/build-1"),
		asctest.JSON(http.StatusOK, `{"data":{"type":"builds","id":"build-1","attributes":{"version":"42","processingState":"VALID"}}}`))
	reg.Register(asctest.POST("/v1/builds/build-1/relationships/betaGroups").JSONBody(`{"data":[{"type":"betaGroups","id":"group-external"}]}`),
		asctest.JSON(http.StatusNoContent, ""))
	reg.Register(asctest.GET("/v1/builds/build-1/betaAppReviewSubmission"),
		asctest.JSON(http.StatusNotFound, `{"errors":[{"status":"404","code":"NOT_FOUND","title":"Not Found"}]}`))
	reg.Register(asctest.POST("/v1/betaAppReviewSubmissions").JSONBody(`{"data":{"type":"betaAppReviewSubmissions","relationships":{"build":{"data":{"type":"builds","id":"build-1"}}}}}`),
		asctest.JSON(http.StatusCreated, `{"data":{"type":"betaAppReviewSubmissions","id":"submission-1"}}`))

	stdout, stderr, err := runCommand(t, []string{
		"publish", "testflight",
		"--app", "123",
		"--build-id", "build-1",
		"--group", "group-external",
		"--poll-interval", "1ms",
		"--submit",
		"--confirm",
	})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if requestCount := len(reg.Requests()); requestCount != 6 {
		t.Fatalf("expected group lookup, build lookup, processing wait, group add, submission lookup, and submission create; got %d requests", requestCount)
	}
	result := decodePublishTestFlightSubmitOutput(t, stdout)
	if result.BetaReviewSubmitted == nil || !*result.BetaReviewSubmitted {
		t.Fatalf("expected betaReviewSubmitted=true, got %#v", result.BetaReviewSubmitted)
	}
	if !strings.Contains(stdout, `"processingState":"VALID"`) {
		t.Fatalf("expected processed build state in output, got %q", stdout)
	}
	if !strings.Contains(stderr, "Submitted build build-1 for beta app review (submission-1)") {
		t.Fatalf("expected beta review submission message, got %q", stderr)
	}
}

func TestPublishTestflightSubmitSkipsBetaReviewSubmissionForInternalGroups(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/apps/123/betaGroups", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"betaGroups","id":"group-internal","attributes":{"name":"Friends and Family","isInternalGroup":true}}]}`))
	reg.Register(asctest.GET("/v1/builds/build-1"),
		asctest.JSON(http.StatusOK, `{"data":{"type":"builds","id":"build-1","attributes":{"version":"42","processingState":"VALID"}}}`))
	reg.Register(asctest.POST("/v1/builds/build-1/relationships/betaGroups").JSONBody(`{"data":[{"type":"betaGroups","id":"group-internal"}]}`),
		asctest.JSON(http.StatusNoContent, ""))

	stdout, stderr, err := runCommand(t, []string{
		"publish", "testflight",
		"--app", "123",
		"--build-id", "build-1",
		"--group", "group-internal",
		"--submit",
		"--confirm",
	})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if requestCount := len(reg.Requests()); requestCount != 3 {
		t.Fatalf("expected group lookup, build lookup, and group add; got %d requests", requestCount)
	}
	result := decodePublishTestFlightSubmitOutput(t, stdout)
	if result.BetaReviewSubmitted == nil || *result.BetaReviewSubmitted {
		t.Fatalf("expected betaReviewSubmitted=false, got %#v", result.BetaReviewSubmitted)
	}
	if result.BetaReviewSubmissionID != "" {
		t.Fatalf("expected empty beta review submission ID, got %q", result.BetaReviewSubmissionID)
	}
	if !strings.Contains(stderr, "Skipped beta app review submission for build build-1 because no external groups were added") {
		t.Fatalf("expected beta review skip message, got %q", stderr)
	}
}

func TestPublishTestflightSubmitTreatsExistingSubmissionAsAlreadyDone(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/apps/123/betaGroups", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"betaGroups","id":"group-external","attributes":{"name":"External QA","isInternalGroup":false}}]}`))
	reg.Register(asctest.GET("/v1/builds/build-1"),
		asctest.JSON(http.StatusOK, `{"data":{"type":"builds","id":"build-1","attributes":{"version":"42","processingState":"VALID"}}}`))
	reg.Register(asctest.POST("/v1/builds/build-1/relationships/betaGroups").JSONBody(`{"data":[{"type":"betaGroups","id":"group-external"}]}`),
		asctest.JSON(http.StatusNoContent, ""))
	reg.Register(asctest.GET("/v1/builds/build-1/betaAppReviewSubmission"),
		asctest.JSON(http.StatusOK, `{"data":{"type":"betaAppReviewSubmissions","id":"submission-existing"}}`))

	stdout, stderr, err := runCommand(t, []string{
		"publish", "testflight",
		"--app", "123",
		"--build-id", "build-1",
		"--group", "group-external",
		"--submit",
		"--confirm",
	})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if requestCount := len(reg.Requests()); requestCount != 4 {
		t.Fatalf("expected group lookup, build lookup, group add, and submission lookup; got %d requests", requestCount)
	}
	result := decodePublishTestFlightSubmitOutput(t, stdout)
	if result.BetaReviewSubmitted == nil || !*result.BetaReviewSubmitted {
		t.Fatalf("expected betaReviewSubmitted=true, got %#v", result.BetaReviewSubmitted)
	}
	if result.BetaReviewSubmissionID != "submission-existing" {
		t.Fatalf("expected existing submission ID, got %q", result.BetaReviewSubmissionID)
	}
	if !strings.Contains(stderr, "Build build-1 already has beta app review submission submission-existing") {
		t.Fatalf("expected existing submission message, got %q", stderr)
	}
}

func TestPublishTestflightSubmitPreservesPartialSuccessWhenSubmissionFails(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/apps/123/betaGroups", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"betaGroups","id":"group-external","attributes":{"name":"External QA","isInternalGroup":false}}]}`))
	reg.Register(asctest.GET("/v1/builds/build-1"),
		asctest.JSON(http.StatusOK, `{"data":{"type":"builds","id":"build-1","attributes":{"version":"42","processingState":"VALID"}}}`))
	reg.Register(asctest.POST("/v1/builds/build-1/relationships/betaGroups").JSONBody(`{"data":[{"type":"betaGroups","id":"group-external"}]}`),
		asctest.JSON(http.StatusNoContent, ""))
	reg.Register(asctest.GET("/v1/builds/build-1/betaAppReviewSubmission"),
		asctest.JSON(http.StatusNotFound, `{"errors":[{"status":"404","code":"NOT_FOUND","title":"Not Found"}]}`))
	reg.Register(asctest.POST("/v1/betaAppReviewSubmissions").JSONBody(`{"data":{"type":"betaAppReviewSubmissions","relationships":{"build":{"data":{"type":"builds","id":"build-1"}}}}}`),
		asctest.JSON(http.StatusServiceUnavailable, `{"errors":[{"status":"503","code":"SERVICE_UNAVAILABLE","title":"Service unavailable","detail":"beta review temporarily unavailable"}]}`))

	stdout, stderr, runErr := runCommand(t, []string{
		"publish", "testflight",
		"--app", "123",
		"--build-id", "build-1",
		"--group", "group-external",
		"--submit",
		"--confirm",
	})

	if runErr == nil {
		t.Fatal("expected error")
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(runErr.Error(), `publish testflight: beta groups were added to build "build-1", but beta app review submission failed`) {
		t.Fatalf("expected partial-success submission error, got %v", runErr)
	}
	if strings.Contains(runErr.Error(), "failed to add groups") {
		t.Fatalf("did not expect misleading add-groups wrapper, got %v", runErr)
	}
	if !strings.Contains(runErr.Error(), "Service unavailable: beta review temporarily unavailable") {
		t.Fatalf("expected underlying submission error, got %v", runErr)
	}
}

func decodePublishTestFlightSubmitOutput(t *testing.T, stdout string) publishTestFlightSubmitOutput {
	t.Helper()

	var result publishTestFlightSubmitOutput
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("failed to decode publish testflight output %q: %v", stdout, err)
	}
	return result
}
