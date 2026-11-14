package cmdtest

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	cmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func TestReviewsUsageErrors(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")

	runUsageExitCases(t, []usageExitCase{
		{
			name:    "reviews respond missing review-id",
			args:    []string{"reviews", "respond", "--response", "Thanks!"},
			wantErr: "--review-id is required",
		},
		{
			name:    "reviews respond missing response",
			args:    []string{"reviews", "respond", "--review-id", "REVIEW_123"},
			wantErr: "--response is required",
		},
		{
			name:    "reviews without subcommand missing app",
			args:    []string{"reviews"},
			wantErr: "--app is required",
		},
		{
			name:    "reviews list missing app",
			args:    []string{"reviews", "list"},
			wantErr: "--app is required",
		},
		{
			name:    "reviews list invalid response state",
			args:    []string{"reviews", "list", "--app", "app-1", "--response-state", "maybe"},
			wantErr: "--response-state must be one of: any, unresponded, unreplied, responded, replied",
		},
		{
			name:    "reviews list invalid response fields",
			args:    []string{"reviews", "list", "--app", "app-1", "--response-fields", "bogus"},
			wantErr: "--response-fields must be a comma-separated list of: responseBody,lastModifiedDate,state,review",
		},
		{
			name:    "reviews response view missing id",
			args:    []string{"reviews", "response", "view"},
			wantErr: "--id is required",
		},
		{
			name:    "reviews response delete missing id",
			args:    []string{"reviews", "response", "delete", "--confirm"},
			wantErr: "--id is required",
		},
		{
			name:    "reviews response delete missing confirm",
			args:    []string{"reviews", "response", "delete", "--id", "RESPONSE_123"},
			wantErr: "--confirm is required",
		},
		{
			name:    "reviews response for-review missing review-id",
			args:    []string{"reviews", "response", "for-review"},
			wantErr: "--review-id is required",
		},
	})
}

func writeReviewBatchFile(t *testing.T, body string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "replies.json")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write batch file: %v", err)
	}
	return path
}

// Documented respond-batch ceilings, restated here so the CLI-level tests fail
// if the enforced limits ever drift from the published contract.
const (
	reviewBatchFileByteLimit     = 64 << 20
	reviewBatchTargetLimit       = 500
	reviewBatchResponseByteLimit = 24 << 10
)

// reviewBatchJSONOfSize returns a valid single-target batch document whose byte
// length is exactly totalSize.
func reviewBatchJSONOfSize(t *testing.T, totalSize int) string {
	t.Helper()

	envelope := func(padding int) string {
		return `{"replies":[{"response":"` + strings.Repeat("a", padding) + `","reviewIds":["review-1"]}]}`
	}
	overhead := len(envelope(0))
	if overhead > totalSize {
		t.Fatalf("cannot build a batch document as small as %d bytes", totalSize)
	}
	body := envelope(totalSize - overhead)
	if len(body) != totalSize {
		t.Fatalf("expected a %d byte batch document, got %d", totalSize, len(body))
	}
	return body
}

func reviewBatchJSONWithReviewIDs(t *testing.T, count int) string {
	t.Helper()

	reviewIDs := make([]string, 0, count)
	for i := range count {
		reviewIDs = append(reviewIDs, fmt.Sprintf(`"review-%d"`, i))
	}
	return `{"replies":[{"response":"Thanks","reviewIds":[` + strings.Join(reviewIDs, ",") + `]}]}`
}

type reviewBatchTestOutput struct {
	Summary reviewBatchTestSummary  `json:"summary"`
	Results []reviewBatchTestResult `json:"results"`
}

type reviewBatchTestSummary struct {
	Total   int `json:"total"`
	Created int `json:"created"`
	Skipped int `json:"skipped"`
	Failed  int `json:"failed"`
	Planned int `json:"planned"`
}

type reviewBatchTestResult struct {
	ReviewID           string `json:"reviewId"`
	Status             string `json:"status"`
	ResponseID         string `json:"responseId"`
	ExistingResponseID string `json:"existingResponseId"`
	Reason             string `json:"reason"`
	Error              string `json:"error"`
}

func decodeReviewBatchTestOutput(t *testing.T, stdout string) reviewBatchTestOutput {
	t.Helper()

	var payload reviewBatchTestOutput
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout=%s", err, stdout)
	}
	return payload
}

func findReviewBatchTestResult(t *testing.T, payload reviewBatchTestOutput, reviewID string) reviewBatchTestResult {
	t.Helper()

	for _, result := range payload.Results {
		if result.ReviewID == reviewID {
			return result
		}
	}
	t.Fatalf("expected result for review %q, got %+v", reviewID, payload.Results)
	return reviewBatchTestResult{}
}

func TestReviewsRespondBatchCreatesGroupedReplies(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	inputPath := writeReviewBatchFile(t, `{
		"replies": [
			{"response": "Thanks for the feedback.", "reviewIds": ["review-1", "review-2"]}
		]
	}`)

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/apps/app-1/customerReviews", "include=response", "limit=200"), asctest.JSON(http.StatusOK, `{
		"data": [
			{"type":"customerReviews","id":"review-1","attributes":{"rating":5}},
			{"type":"customerReviews","id":"review-2","attributes":{"rating":4}}
		],
		"links": {"self": "https://api.appstoreconnect.apple.com/v1/apps/app-1/customerReviews"}
	}`))
	reg.Register(asctest.POST("/v1/customerReviewResponses").JSONBody(`{"data":{"type":"customerReviewResponses","attributes":{"responseBody":"Thanks for the feedback."},"relationships":{"review":{"data":{"type":"customerReviews","id":"review-1"}}}}}`),
		asctest.JSON(http.StatusCreated, `{"data":{"type":"customerReviewResponses","id":"response-1","attributes":{"responseBody":"Thanks for the feedback.","state":"PENDING_PUBLISH"}}}`))
	reg.Register(asctest.POST("/v1/customerReviewResponses").JSONBody(`{"data":{"type":"customerReviewResponses","attributes":{"responseBody":"Thanks for the feedback."},"relationships":{"review":{"data":{"type":"customerReviews","id":"review-2"}}}}}`),
		asctest.JSON(http.StatusCreated, `{"data":{"type":"customerReviewResponses","id":"response-2","attributes":{"responseBody":"Thanks for the feedback.","state":"PENDING_PUBLISH"}}}`))

	stdout, stderr, err := runCommand(t, []string{"reviews", "respond-batch", "--app", "app-1", "--file", inputPath, "--output", "json", "--confirm"})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	postBodies := reviewResponsePostBodies(reg)
	if len(postBodies) != 2 {
		t.Fatalf("expected two create requests, got %d", len(postBodies))
	}
	var payload struct {
		AppID   string `json:"appId"`
		DryRun  bool   `json:"dryRun"`
		Summary struct {
			Total   int `json:"total"`
			Created int `json:"created"`
			Skipped int `json:"skipped"`
			Failed  int `json:"failed"`
			Planned int `json:"planned"`
		} `json:"summary"`
		Results []struct {
			ReviewID   string `json:"reviewId"`
			Status     string `json:"status"`
			ResponseID string `json:"responseId"`
		} `json:"results"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout=%s", err, stdout)
	}
	if payload.AppID != "app-1" || payload.DryRun {
		t.Fatalf("unexpected payload header: %+v", payload)
	}
	if payload.Summary.Total != 2 || payload.Summary.Created != 2 || payload.Summary.Skipped != 0 || payload.Summary.Failed != 0 || payload.Summary.Planned != 0 {
		t.Fatalf("unexpected summary: %+v", payload.Summary)
	}
	if len(payload.Results) != 2 || payload.Results[0].Status != "created" || payload.Results[0].ResponseID != "response-1" {
		t.Fatalf("unexpected results: %+v", payload.Results)
	}
	if !strings.Contains(postBodies[0], `"id":"review-1"`) || !strings.Contains(postBodies[1], `"id":"review-2"`) {
		t.Fatalf("expected create requests for both reviews, got %#v", postBodies)
	}
}

func TestReviewsRespondBatchDryRunDoesNotPost(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	inputPath := writeReviewBatchFile(t, `{"replies":[{"response":"Thanks","reviewIds":["review-1","review-2"]}]}`)
	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/apps/app-1/customerReviews", "include=response", "limit=200"), asctest.JSON(http.StatusOK, `{"data":[{"type":"customerReviews","id":"review-1"},{"type":"customerReviews","id":"review-2"}]}`))

	stdout, stderr, err := runCommand(t, []string{"reviews", "respond-batch", "--app", "app-1", "--file", inputPath, "--dry-run", "--output", "json"})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if requests := len(reg.Requests()); requests != 1 {
		t.Fatalf("expected one preflight request, got %d", requests)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout=%s", err, stdout)
	}
	summary := payload["summary"].(map[string]any)
	if int(summary["planned"].(float64)) != 2 || int(summary["created"].(float64)) != 0 {
		t.Fatalf("unexpected dry-run summary: %+v", summary)
	}
}

func TestReviewsRespondBatchSkipExisting(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	inputPath := writeReviewBatchFile(t, `{"replies":[{"response":"Thanks","reviewIds":["review-1","review-2"]}]}`)
	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/apps/app-1/customerReviews", "include=response", "limit=200"), asctest.JSON(http.StatusOK, `{"data":[
		{"type":"customerReviews","id":"review-1","relationships":{"response":{"data":{"type":"customerReviewResponses","id":"existing-response"}}}},
		{"type":"customerReviews","id":"review-2","relationships":{"response":{"data":null}}}
	]}`))
	reg.Register(asctest.POST("/v1/customerReviewResponses").JSONBody(`{"data":{"type":"customerReviewResponses","attributes":{"responseBody":"Thanks"},"relationships":{"review":{"data":{"type":"customerReviews","id":"review-2"}}}}}`),
		asctest.JSON(http.StatusCreated, `{"data":{"type":"customerReviewResponses","id":"response-2"}}`))

	stdout, stderr, err := runCommand(t, []string{"reviews", "respond-batch", "--app", "app-1", "--file", inputPath, "--skip-existing", "--output", "json", "--confirm"})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if posts := len(reviewResponsePostBodies(reg)); posts != 1 {
		t.Fatalf("expected one create request, got %d", posts)
	}
	payload := decodeReviewBatchTestOutput(t, stdout)
	if payload.Summary.Skipped != 1 || payload.Summary.Created != 1 {
		t.Fatalf("unexpected skip-existing summary: %+v", payload.Summary)
	}
	skipped := findReviewBatchTestResult(t, payload, "review-1")
	if skipped.Status != "skipped" || skipped.ExistingResponseID != "existing-response" || skipped.Reason != "existing-response" {
		t.Fatalf("unexpected skipped result: %+v", skipped)
	}
}

func TestReviewsRespondBatchResponseStateUnrespondedSkipsRespondedReviews(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	inputPath := writeReviewBatchFile(t, `{"replies":[{"response":"Thanks","reviewIds":["review-1","review-2"]}]}`)
	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/apps/app-1/customerReviews", "include=response", "limit=200"), asctest.JSON(http.StatusOK, `{"data":[
		{"type":"customerReviews","id":"review-1","relationships":{"response":{"data":{"type":"customerReviewResponses","id":"existing-response"}}}},
		{"type":"customerReviews","id":"review-2","relationships":{"response":{"data":null}}}
	]}`))
	reg.Register(asctest.POST("/v1/customerReviewResponses").JSONBody(`{"data":{"type":"customerReviewResponses","attributes":{"responseBody":"Thanks"},"relationships":{"review":{"data":{"type":"customerReviews","id":"review-2"}}}}}`),
		asctest.JSON(http.StatusCreated, `{"data":{"type":"customerReviewResponses","id":"response-2"}}`))

	stdout, stderr, err := runCommand(t, []string{"reviews", "respond-batch", "--app", "app-1", "--file", inputPath, "--response-state", "unresponded", "--output", "json", "--confirm"})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if posts := len(reviewResponsePostBodies(reg)); posts != 1 {
		t.Fatalf("expected one create request, got %d", posts)
	}
	payload := decodeReviewBatchTestOutput(t, stdout)
	if payload.Summary.Created != 1 || payload.Summary.Skipped != 1 {
		t.Fatalf("unexpected response-state summary: %+v", payload.Summary)
	}
	skipped := findReviewBatchTestResult(t, payload, "review-1")
	if skipped.Status != "skipped" || skipped.Reason != "response-state-mismatch" {
		t.Fatalf("unexpected response-state skipped result: %+v", skipped)
	}
}

func TestReviewsRespondBatchRejectsOversizedFileWithoutRequests(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	asctest.NewRegistry(t).Install()

	inputPath := writeReviewBatchFile(t, reviewBatchJSONOfSize(t, reviewBatchFileByteLimit+1))

	stdout, stderr := captureOutput(t, func() {
		code := cmd.Run([]string{"reviews", "respond-batch", "--app", "app-1", "--file", inputPath, "--confirm"}, "1.2.3")
		if code != cmd.ExitUsage {
			t.Fatalf("expected exit code %d, got %d", cmd.ExitUsage, code)
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "--file must not exceed 67108864 bytes") {
		t.Fatalf("expected batch file size rejection, got %q", stderr)
	}
}

func TestReviewsRespondBatchRejectsOversizedResponseWithoutRequests(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	asctest.NewRegistry(t).Install()

	oversized := `{"replies":[{"response":"` + strings.Repeat("a", reviewBatchResponseByteLimit+1) + `","reviewIds":["review-1"]}]}`
	inputPath := writeReviewBatchFile(t, oversized)

	stdout, stderr := captureOutput(t, func() {
		code := cmd.Run([]string{"reviews", "respond-batch", "--app", "app-1", "--file", inputPath, "--dry-run"}, "1.2.3")
		if code != cmd.ExitUsage {
			t.Fatalf("expected exit code %d, got %d", cmd.ExitUsage, code)
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "replies[0].response must not exceed 24576 bytes") {
		t.Fatalf("expected response size rejection, got %q", stderr)
	}
}

func TestReviewsRespondBatchRejectsTooManyReviewIDsWithoutRequests(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	asctest.NewRegistry(t).Install()

	inputPath := writeReviewBatchFile(t, reviewBatchJSONWithReviewIDs(t, reviewBatchTargetLimit+1))

	stdout, stderr := captureOutput(t, func() {
		code := cmd.Run([]string{"reviews", "respond-batch", "--app", "app-1", "--file", inputPath, "--dry-run"}, "1.2.3")
		if code != cmd.ExitUsage {
			t.Fatalf("expected exit code %d, got %d", cmd.ExitUsage, code)
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "--file must not contain more than 500 review ids") {
		t.Fatalf("expected batch target count rejection, got %q", stderr)
	}
}

func TestReviewsRespondBatchDryRunAtReviewIDLimit(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	inputPath := writeReviewBatchFile(t, reviewBatchJSONWithReviewIDs(t, reviewBatchTargetLimit))
	reviews := make([]string, 0, reviewBatchTargetLimit)
	for i := range reviewBatchTargetLimit {
		reviews = append(reviews, fmt.Sprintf(`{"type":"customerReviews","id":"review-%d"}`, i))
	}
	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/apps/app-1/customerReviews", "include=response", "limit=200"), asctest.JSON(http.StatusOK, `{"data":[`+strings.Join(reviews, ",")+`]}`))

	stdout, stderr, err := runCommand(t, []string{"reviews", "respond-batch", "--app", "app-1", "--file", inputPath, "--dry-run", "--output", "json"})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if requests := len(reg.Requests()); requests != 1 {
		t.Fatalf("expected one preflight request, got %d", requests)
	}
	payload := decodeReviewBatchTestOutput(t, stdout)
	if payload.Summary.Total != reviewBatchTargetLimit || payload.Summary.Planned != reviewBatchTargetLimit {
		t.Fatalf("unexpected dry-run summary at target limit: %+v", payload.Summary)
	}
	if payload.Summary.Created != 0 || payload.Summary.Failed != 0 || payload.Summary.Skipped != 0 {
		t.Fatalf("expected only planned results at target limit: %+v", payload.Summary)
	}
}

func TestReviewsRespondBatchValidationErrors(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")

	tests := []struct {
		name    string
		args    []string
		body    string
		wantErr string
	}{
		{
			name:    "missing app",
			args:    []string{"reviews", "respond-batch", "--file", "FILE"},
			wantErr: "--app is required",
		},
		{
			name:    "missing file",
			args:    []string{"reviews", "respond-batch", "--app", "app-1"},
			wantErr: "--file is required",
		},
		{
			name:    "bad json",
			args:    []string{"reviews", "respond-batch", "--app", "app-1", "--file", "FILE", "--confirm"},
			body:    `{"replies":[`,
			wantErr: "failed to parse",
		},
		{
			name:    "trailing json",
			args:    []string{"reviews", "respond-batch", "--app", "app-1", "--file", "FILE", "--confirm"},
			body:    `{"replies":[{"response":"Thanks","reviewIds":["review-1"]}]} {"extra":true}`,
			wantErr: "multiple JSON values are not allowed",
		},
		{
			name:    "duplicate review id",
			args:    []string{"reviews", "respond-batch", "--app", "app-1", "--file", "FILE", "--confirm"},
			body:    `{"replies":[{"response":"Thanks","reviewIds":["review-1","review-1"]}]}`,
			wantErr: "duplicate review id",
		},
		{
			name:    "empty response",
			args:    []string{"reviews", "respond-batch", "--app", "app-1", "--file", "FILE", "--confirm"},
			body:    `{"replies":[{"response":" ","reviewIds":["review-1"]}]}`,
			wantErr: "response is required",
		},
		{
			name:    "invalid response state",
			args:    []string{"reviews", "respond-batch", "--app", "app-1", "--file", "FILE", "--response-state", "maybe"},
			body:    `{"replies":[{"response":"Thanks","reviewIds":["review-1"]}]}`,
			wantErr: "--response-state must be one of: any, unresponded, unreplied, responded, replied",
		},
		{
			name:    "empty review id",
			args:    []string{"reviews", "respond-batch", "--app", "app-1", "--file", "FILE", "--confirm"},
			body:    `{"replies":[{"response":"Thanks","reviewIds":[" "]}]}`,
			wantErr: "reviewIds[0] is required",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			args := append([]string(nil), test.args...)
			if strings.Contains(strings.Join(args, " "), "FILE") {
				path := writeReviewBatchFile(t, test.body)
				for i := range args {
					if args[i] == "FILE" {
						args[i] = path
					}
				}
			}

			assertUsageExit(t, args, test.wantErr)
		})
	}
}

func TestReviewsListResponseStateIncludeResponseAndResponseFields(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/apps/app-1/customerReviews", "exists[publishedResponse]=false", "include=response", "fields[customerReviewResponses]=responseBody,state"), asctest.JSON(http.StatusOK, `{"data":[{"type":"customerReviews","id":"review-1"}]}`))

	stdout, stderr, err := runCommand(t, []string{"reviews", "list", "--app", "app-1", "--response-state", "unreplied", "--include-response", "--response-fields", "responseBody,state", "--output", "json"})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	var payload struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout=%s", err, stdout)
	}
	if len(payload.Data) != 1 || payload.Data[0].ID != "review-1" {
		t.Fatalf("unexpected review output: %+v", payload.Data)
	}
}

func TestReviewsListOnlyUnrespondedMapsToPublishedResponseFilter(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/apps/app-1/customerReviews", "exists[publishedResponse]=false"), asctest.JSON(http.StatusOK, `{"data":[]}`))

	stdout, stderr, err := runCommand(t, []string{"reviews", "list", "--app", "app-1", "--only-unresponded", "--output", "json"})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"data":[]`) {
		t.Fatalf("expected empty reviews output, got %q", stdout)
	}
}

func TestRunReviewsRespondBatchPartialFailureReturnsExitError(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	inputPath := writeReviewBatchFile(t, `{"replies":[{"response":"Thanks","reviewIds":["review-1","review-2"]}]}`)
	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/apps/app-1/customerReviews", "include=response", "limit=200"), asctest.JSON(http.StatusOK, `{"data":[{"type":"customerReviews","id":"review-1"},{"type":"customerReviews","id":"review-2"}]}`))
	reg.Register(asctest.POST("/v1/customerReviewResponses").JSONBody(`{"data":{"type":"customerReviewResponses","attributes":{"responseBody":"Thanks"},"relationships":{"review":{"data":{"type":"customerReviews","id":"review-1"}}}}}`),
		asctest.JSON(http.StatusCreated, `{"data":{"type":"customerReviewResponses","id":"response-1"}}`))
	reg.Register(asctest.POST("/v1/customerReviewResponses").JSONBody(`{"data":{"type":"customerReviewResponses","attributes":{"responseBody":"Thanks"},"relationships":{"review":{"data":{"type":"customerReviews","id":"review-2"}}}}}`),
		asctest.JSON(http.StatusUnprocessableEntity, `{"errors":[{"status":"422","code":"ENTITY_ERROR","title":"Invalid","detail":"review response failed"}]}`))

	stdout, stderr := captureOutput(t, func() {
		code := cmd.Run([]string{"reviews", "respond-batch", "--app", "app-1", "--file", inputPath, "--output", "json", "--confirm"}, "1.2.3")
		if code != cmd.ExitError {
			t.Fatalf("expected exit code %d, got %d", cmd.ExitError, code)
		}
	})

	if want := "Error: reviews respond-batch: 1 review(s) failed\n"; stderr != want {
		t.Fatalf("stderr = %q, want %q", stderr, want)
	}
	payload := decodeReviewBatchTestOutput(t, stdout)
	if payload.Summary.Created != 1 || payload.Summary.Failed != 1 {
		t.Fatalf("unexpected partial failure summary: %+v", payload.Summary)
	}
	failed := findReviewBatchTestResult(t, payload, "review-2")
	if failed.Status != "failed" || failed.Error == "" {
		t.Fatalf("unexpected failed result: %+v", failed)
	}
}

func TestReviewsResponseForReviewReturnsNotConfiguredStateWhenUnset(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/customerReviews/review-1/response"), asctest.JSON(http.StatusNotFound, `{"errors":[{"status":"404","code":"NOT_FOUND","title":"Not Found"}]}`))
	reg.Register(asctest.GET("/v1/customerReviews/review-1"), asctest.JSON(http.StatusOK, `{"data":{"type":"customerReviews","id":"review-1","attributes":{"rating":5,"title":"Great","body":"Loved it"}}}`))

	stdout, stderr, err := runCommand(t, []string{"reviews", "response", "for-review", "--review-id", "review-1", "--output", "json"})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if !strings.Contains(stderr, "Warning: Customer review response is not configured for review \"review-1\".") {
		t.Fatalf("expected not-configured warning, got %q", stderr)
	}

	var payload struct {
		ReviewID   string `json:"reviewId"`
		Configured bool   `json:"configured"`
		Message    string `json:"message"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout=%s", err, stdout)
	}
	if payload.ReviewID != "review-1" {
		t.Fatalf("expected reviewId review-1, got %q", payload.ReviewID)
	}
	if payload.Configured {
		t.Fatal("expected configured=false")
	}
	if payload.Message == "" {
		t.Fatal("expected message")
	}
}

func TestReviewsResponseForReviewPreservesErrorForUnknownReviewID(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/customerReviews/missing-review/response"), asctest.JSON(http.StatusNotFound, `{"errors":[{"status":"404","code":"NOT_FOUND","title":"Not Found"}]}`))
	reg.Register(asctest.GET("/v1/customerReviews/missing-review"), asctest.JSON(http.StatusNotFound, `{"errors":[{"status":"404","code":"NOT_FOUND","title":"Not Found"}]}`))

	stdout, stderr, runErr := runCommand(t, []string{"reviews", "response", "for-review", "--review-id", "missing-review", "--output", "json"})

	if runErr == nil {
		t.Fatal("expected error, got nil")
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(runErr.Error(), "reviews response for-review: failed to fetch:") {
		t.Fatalf("expected wrapped fetch error, got %v", runErr)
	}
	if strings.Contains(runErr.Error(), "not configured") {
		t.Fatalf("expected unknown review to remain a hard error, got %v", runErr)
	}
}

func reviewResponsePostBodies(reg *asctest.Registry) []string {
	var bodies []string
	for _, req := range reg.Requests() {
		if req.Method == http.MethodPost && req.URL.Path == "/v1/customerReviewResponses" {
			bodies = append(bodies, string(req.Body))
		}
	}
	return bodies
}
