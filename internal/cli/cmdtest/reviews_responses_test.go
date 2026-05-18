package cmdtest

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	cmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
)

func TestReviewsRespondValidationErrors(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
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
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(test.args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				err := root.Run(context.Background())
				if !errors.Is(err, flag.ErrHelp) {
					t.Fatalf("expected ErrHelp, got %v", err)
				}
			})

			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, test.wantErr) {
				t.Fatalf("expected error %q, got %q", test.wantErr, stderr)
			}
		})
	}
}

func writeReviewBatchFile(t *testing.T, body string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "replies.json")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write batch file: %v", err)
	}
	return path
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

	var postBodies []string
	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/customerReviews":
			if got := req.URL.Query().Get("include"); got != "response" {
				t.Fatalf("expected include=response, got %q", got)
			}
			if got := req.URL.Query().Get("limit"); got != "200" {
				t.Fatalf("expected limit=200, got %q", got)
			}
			return jsonResponse(http.StatusOK, `{
				"data": [
					{"type":"customerReviews","id":"review-1","attributes":{"rating":5}},
					{"type":"customerReviews","id":"review-2","attributes":{"rating":4}}
				],
				"links": {"self": "https://api.appstoreconnect.apple.com/v1/apps/app-1/customerReviews"}
			}`)
		case req.Method == http.MethodPost && req.URL.Path == "/v1/customerReviewResponses":
			body, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("read request body: %v", err)
			}
			postBodies = append(postBodies, string(body))
			id := "response-1"
			if len(postBodies) == 2 {
				id = "response-2"
			}
			return jsonResponse(http.StatusCreated, `{"data":{"type":"customerReviewResponses","id":"`+id+`","attributes":{"responseBody":"Thanks for the feedback.","state":"PENDING_PUBLISH"}}}`)
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	}))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"reviews", "respond-batch", "--app", "app-1", "--file", inputPath, "--output", "json"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
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
	requests := 0
	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests++
		if req.Method != http.MethodGet || req.URL.Path != "/v1/apps/app-1/customerReviews" {
			t.Fatalf("dry-run should only fetch reviews, got %s %s", req.Method, req.URL.Path)
		}
		return jsonResponse(http.StatusOK, `{"data":[{"type":"customerReviews","id":"review-1"},{"type":"customerReviews","id":"review-2"}]}`)
	}))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"reviews", "respond-batch", "--app", "app-1", "--file", inputPath, "--dry-run", "--output", "json"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if requests != 1 {
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
	posts := 0
	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/customerReviews":
			return jsonResponse(http.StatusOK, `{"data":[
				{"type":"customerReviews","id":"review-1","relationships":{"response":{"data":{"type":"customerReviewResponses","id":"existing-response"}}}},
				{"type":"customerReviews","id":"review-2","relationships":{"response":{"data":null}}}
			]}`)
		case req.Method == http.MethodPost && req.URL.Path == "/v1/customerReviewResponses":
			posts++
			body, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("read request body: %v", err)
			}
			if !strings.Contains(string(body), `"id":"review-2"`) {
				t.Fatalf("expected only review-2 to be posted, got %s", body)
			}
			return jsonResponse(http.StatusCreated, `{"data":{"type":"customerReviewResponses","id":"response-2"}}`)
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	}))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"reviews", "respond-batch", "--app", "app-1", "--file", inputPath, "--skip-existing", "--output", "json"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if posts != 1 {
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
	posts := 0
	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/customerReviews":
			return jsonResponse(http.StatusOK, `{"data":[
				{"type":"customerReviews","id":"review-1","relationships":{"response":{"data":{"type":"customerReviewResponses","id":"existing-response"}}}},
				{"type":"customerReviews","id":"review-2","relationships":{"response":{"data":null}}}
			]}`)
		case req.Method == http.MethodPost && req.URL.Path == "/v1/customerReviewResponses":
			posts++
			return jsonResponse(http.StatusCreated, `{"data":{"type":"customerReviewResponses","id":"response-2"}}`)
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	}))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"reviews", "respond-batch", "--app", "app-1", "--file", inputPath, "--response-state", "unresponded", "--output", "json"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if posts != 1 {
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

func TestReviewsRespondBatchValidationErrors(t *testing.T) {
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
			args:    []string{"reviews", "respond-batch", "--app", "app-1", "--file", "FILE"},
			body:    `{"replies":[`,
			wantErr: "failed to parse",
		},
		{
			name:    "trailing json",
			args:    []string{"reviews", "respond-batch", "--app", "app-1", "--file", "FILE"},
			body:    `{"replies":[{"response":"Thanks","reviewIds":["review-1"]}]} {"extra":true}`,
			wantErr: "multiple JSON values are not allowed",
		},
		{
			name:    "duplicate review id",
			args:    []string{"reviews", "respond-batch", "--app", "app-1", "--file", "FILE"},
			body:    `{"replies":[{"response":"Thanks","reviewIds":["review-1","review-1"]}]}`,
			wantErr: "duplicate review id",
		},
		{
			name:    "empty response",
			args:    []string{"reviews", "respond-batch", "--app", "app-1", "--file", "FILE"},
			body:    `{"replies":[{"response":" ","reviewIds":["review-1"]}]}`,
			wantErr: "response is required",
		},
		{
			name:    "empty review id",
			args:    []string{"reviews", "respond-batch", "--app", "app-1", "--file", "FILE"},
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

			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				err := root.Run(context.Background())
				if !errors.Is(err, flag.ErrHelp) {
					t.Fatalf("expected ErrHelp, got %v", err)
				}
			})

			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, test.wantErr) {
				t.Fatalf("expected stderr to contain %q, got %q", test.wantErr, stderr)
			}
		})
	}
}

func TestRunReviewsRespondBatchInvalidResponseStateReturnsUsageExit(t *testing.T) {
	inputPath := writeReviewBatchFile(t, `{"replies":[{"response":"Thanks","reviewIds":["review-1"]}]}`)

	stdout, stderr := captureOutput(t, func() {
		code := cmd.Run([]string{"reviews", "respond-batch", "--app", "app-1", "--file", inputPath, "--response-state", "maybe"}, "1.2.3")
		if code != cmd.ExitUsage {
			t.Fatalf("expected exit code %d, got %d", cmd.ExitUsage, code)
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "--response-state must be one of: any, unresponded, responded") {
		t.Fatalf("expected response-state validation, got %q", stderr)
	}
}

func TestReviewsListResponseStateAndIncludeResponse(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet || req.URL.Path != "/v1/apps/app-1/customerReviews" {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
		}
		if got := req.URL.Query().Get("exists[publishedResponse]"); got != "false" {
			t.Fatalf("expected exists[publishedResponse]=false, got %q", got)
		}
		if got := req.URL.Query().Get("include"); got != "response" {
			t.Fatalf("expected include=response, got %q", got)
		}
		return jsonResponse(http.StatusOK, `{"data":[{"type":"customerReviews","id":"review-1"}]}`)
	}))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"reviews", "list", "--app", "app-1", "--response-state", "unresponded", "--include-response", "--output", "json"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

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

func TestReviewsListRejectsInvalidResponseState(t *testing.T) {
	stdout, stderr := captureOutput(t, func() {
		code := cmd.Run([]string{"reviews", "list", "--app", "app-1", "--response-state", "maybe"}, "1.2.3")
		if code != cmd.ExitUsage {
			t.Fatalf("expected exit code %d, got %d", cmd.ExitUsage, code)
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "--response-state must be one of: any, unresponded, responded") {
		t.Fatalf("expected response-state validation, got %q", stderr)
	}
}

func TestRunReviewsRespondBatchPartialFailureReturnsExitError(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	inputPath := writeReviewBatchFile(t, `{"replies":[{"response":"Thanks","reviewIds":["review-1","review-2"]}]}`)
	posts := 0
	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/customerReviews":
			return jsonResponse(http.StatusOK, `{"data":[{"type":"customerReviews","id":"review-1"},{"type":"customerReviews","id":"review-2"}]}`)
		case req.Method == http.MethodPost && req.URL.Path == "/v1/customerReviewResponses":
			posts++
			if posts == 1 {
				return jsonResponse(http.StatusCreated, `{"data":{"type":"customerReviewResponses","id":"response-1"}}`)
			}
			return jsonResponse(http.StatusUnprocessableEntity, `{"errors":[{"status":"422","code":"ENTITY_ERROR","title":"Invalid","detail":"review response failed"}]}`)
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	}))

	stdout, stderr := captureOutput(t, func() {
		code := cmd.Run([]string{"reviews", "respond-batch", "--app", "app-1", "--file", inputPath, "--output", "json"}, "1.2.3")
		if code != cmd.ExitError {
			t.Fatalf("expected exit code %d, got %d", cmd.ExitError, code)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
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

func TestReviewsResponseViewValidationErrors(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "reviews response view missing id",
			args:    []string{"reviews", "response", "view"},
			wantErr: "--id is required",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(test.args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				err := root.Run(context.Background())
				if !errors.Is(err, flag.ErrHelp) {
					t.Fatalf("expected ErrHelp, got %v", err)
				}
			})

			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, test.wantErr) {
				t.Fatalf("expected error %q, got %q", test.wantErr, stderr)
			}
		})
	}
}

func TestReviewsResponseDeleteValidationErrors(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
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
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(test.args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				err := root.Run(context.Background())
				if !errors.Is(err, flag.ErrHelp) {
					t.Fatalf("expected ErrHelp, got %v", err)
				}
			})

			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, test.wantErr) {
				t.Fatalf("expected error %q, got %q", test.wantErr, stderr)
			}
		})
	}
}

func TestReviewsResponseForReviewValidationErrors(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "reviews response for-review missing review-id",
			args:    []string{"reviews", "response", "for-review"},
			wantErr: "--review-id is required",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(test.args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				err := root.Run(context.Background())
				if !errors.Is(err, flag.ErrHelp) {
					t.Fatalf("expected ErrHelp, got %v", err)
				}
			})

			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, test.wantErr) {
				t.Fatalf("expected error %q, got %q", test.wantErr, stderr)
			}
		})
	}
}

func TestReviewsListBackwardsCompatibility(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")

	// Test that reviews without subcommand still requires --app
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"reviews"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "--app is required") {
		t.Fatalf("expected missing app error, got %q", stderr)
	}
}

func TestReviewsListSubcommandValidationErrors(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "reviews list missing app",
			args:    []string{"reviews", "list"},
			wantErr: "--app is required",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(test.args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				err := root.Run(context.Background())
				if !errors.Is(err, flag.ErrHelp) {
					t.Fatalf("expected ErrHelp, got %v", err)
				}
			})

			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, test.wantErr) {
				t.Fatalf("expected error %q, got %q", test.wantErr, stderr)
			}
		})
	}
}

func TestReviewsResponseParentShowsHelp(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	_, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"reviews", "response"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	// Should show help with subcommands listed
	if !strings.Contains(stderr, "view") || !strings.Contains(stderr, "delete") || !strings.Contains(stderr, "for-review") {
		t.Fatalf("expected help output with subcommands, got %q", stderr)
	}
}

func TestReviewsResponseForReviewReturnsNotConfiguredStateWhenUnset(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		switch req.URL.Path {
		case "/v1/customerReviews/review-1/response":
			return jsonResponse(http.StatusNotFound, `{"errors":[{"status":"404","code":"NOT_FOUND","title":"Not Found"}]}`)
		case "/v1/customerReviews/review-1":
			return jsonResponse(http.StatusOK, `{"data":{"type":"customerReviews","id":"review-1","attributes":{"rating":5,"title":"Great","body":"Loved it"}}}`)
		default:
			t.Fatalf("unexpected path %s", req.URL.Path)
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"reviews", "response", "for-review", "--review-id", "review-1", "--output", "json"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

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

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		switch req.URL.Path {
		case "/v1/customerReviews/missing-review/response":
			return jsonResponse(http.StatusNotFound, `{"errors":[{"status":"404","code":"NOT_FOUND","title":"Not Found"}]}`)
		case "/v1/customerReviews/missing-review":
			return jsonResponse(http.StatusNotFound, `{"errors":[{"status":"404","code":"NOT_FOUND","title":"Not Found"}]}`)
		default:
			t.Fatalf("unexpected path %s", req.URL.Path)
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"reviews", "response", "for-review", "--review-id", "missing-review", "--output", "json"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

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
