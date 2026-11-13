package web

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

const reviewThreadsAppFixture = `{
	"data": [
		{
			"id": "thread-sub",
			"type": "resolutionCenterThreads",
			"attributes": {
				"threadType": "REJECTION_BINARY",
				"state": "OPEN",
				"createdDate": "2026-02-25T00:00:00Z",
				"lastMessageResponseDate": "2026-02-26T00:00:00Z"
			},
			"relationships": {"reviewSubmission": {"data": {"type": "reviewSubmissions", "id": "sub-1"}}}
		},
		{
			"id": "thread-app",
			"type": "resolutionCenterThreads",
			"attributes": {
				"threadType": "APP_MESSAGE_INFORMATIONAL",
				"state": "OPEN",
				"createdDate": "2026-01-05T00:00:00Z"
			}
		}
	]
}`

const reviewThreadsDraftFixture = `{
	"data": {
		"id": "draft-1",
		"type": "resolutionCenterDraftMessages",
		"attributes": {
			"createdDate": "2026-03-01T09:00:00Z",
			"messageBody": "<p>Draft reply &amp; notes</p>"
		},
		"relationships": {
			"resolutionCenterMessageAttachments": {"data": [{"type": "resolutionCenterMessageAttachments", "id": "att-1"}]}
		}
	},
	"included": [
		{
			"id": "att-1",
			"type": "resolutionCenterMessageAttachments",
			"attributes": {
				"fileName": "notes.txt",
				"fileSize": 12,
				"downloadUrl": "https://iosapps.itunes.apple.com/signed?token=secret"
			}
		}
	]
}`

// stubWebReviewSession installs a web session whose transport answers iris
// paths from bodies, recording every requested path.
func stubWebReviewSession(t *testing.T, bodies map[string]string) *[]string {
	t.Helper()

	originalResolveSession := resolveSessionFn
	t.Cleanup(func() { resolveSessionFn = originalResolveSession })

	requested := make([]string, 0)
	resolveSessionFn = func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{
			Client: &http.Client{
				Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
					if err := req.Context().Err(); err != nil {
						return nil, err
					}
					requested = append(requested, req.URL.Path)
					body, ok := bodies[req.URL.Path]
					if !ok {
						t.Errorf("unexpected path: %s", req.URL.Path)
						body = `{"errors":[{"code":"TEST_UNEXPECTED_PATH"}]}`
						return &http.Response{
							StatusCode: http.StatusInternalServerError,
							Header:     http.Header{"Content-Type": []string{"application/json"}},
							Body:       io.NopCloser(strings.NewReader(body)),
							Request:    req,
						}, nil
					}
					status := http.StatusOK
					if strings.HasPrefix(body, "!") {
						status = http.StatusBadRequest
						body = strings.TrimPrefix(body, "!")
					}
					return &http.Response{
						StatusCode: status,
						Header:     http.Header{"Content-Type": []string{"application/json"}},
						Body:       io.NopCloser(strings.NewReader(body)),
						Request:    req,
					}, nil
				}),
			},
		}, "cache", nil
	}
	return &requested
}

func runWebReviewCommand(t *testing.T, exec func() error) (string, string) {
	t.Helper()
	var err error
	stdout, stderr := captureOutput(t, func() { err = exec() })
	if err != nil {
		t.Fatalf("command error = %v (stderr=%s)", err, stderr)
	}
	return stdout, stderr
}

func TestWebReviewThreadsListsAppScopedThreads(t *testing.T) {
	requested := stubWebReviewSession(t, map[string]string{
		"/iris/v1/apps/app-1/resolutionCenterThreads": reviewThreadsAppFixture,
	})

	cmd := WebReviewThreadsCommand()
	if err := cmd.FlagSet.Parse([]string{"--app", "app-1", "--output", "json"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	stdout, _ := runWebReviewCommand(t, func() error { return cmd.Exec(context.Background(), nil) })

	var payload []struct {
		Thread struct {
			ID                 string `json:"id"`
			ThreadType         string `json:"threadType"`
			ReviewSubmissionID string `json:"reviewSubmissionId"`
		} `json:"thread"`
		DraftMessage *struct {
			ID string `json:"id"`
		} `json:"draftMessage"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("failed to parse output %q: %v", stdout, err)
	}
	if len(payload) != 2 {
		t.Fatalf("expected 2 threads, got %d (%s)", len(payload), stdout)
	}
	if payload[0].Thread.ID != "thread-sub" || payload[1].Thread.ID != "thread-app" {
		t.Fatalf("unexpected threads: %s", stdout)
	}
	if payload[1].Thread.ReviewSubmissionID != "" {
		t.Fatalf("expected the app-only thread to carry no submission: %s", stdout)
	}
	if payload[0].DraftMessage != nil {
		t.Fatalf("expected no draft without --drafts: %s", stdout)
	}
	for _, path := range *requested {
		if strings.Contains(path, "resolutionCenterDraftMessage") {
			t.Fatalf("expected no draft request without --drafts, got %v", *requested)
		}
	}
}

func TestWebReviewThreadsReadsDraftMessages(t *testing.T) {
	requested := stubWebReviewSession(t, map[string]string{
		"/iris/v1/apps/app-1/resolutionCenterThreads":                              reviewThreadsAppFixture,
		"/iris/v1/resolutionCenterThreads/thread-sub/resolutionCenterDraftMessage": reviewThreadsDraftFixture,
		"/iris/v1/resolutionCenterThreads/thread-app/resolutionCenterDraftMessage": `{"data": null}`,
	})

	cmd := WebReviewThreadsCommand()
	if err := cmd.FlagSet.Parse([]string{"--app", "app-1", "--drafts", "--plain-text", "--output", "json"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	stdout, _ := runWebReviewCommand(t, func() error { return cmd.Exec(context.Background(), nil) })

	if strings.Contains(stdout, "token=secret") {
		t.Fatalf("expected signed attachment urls to be redacted: %s", stdout)
	}

	var payload []struct {
		Thread struct {
			ID string `json:"id"`
		} `json:"thread"`
		DraftMessage *struct {
			ID               string `json:"id"`
			ThreadID         string `json:"threadId"`
			MessageBody      string `json:"messageBody"`
			MessageBodyPlain string `json:"messageBodyPlain"`
			Attachments      []struct {
				AttachmentID string `json:"attachmentId"`
				FileName     string `json:"fileName"`
				DownloadURL  string `json:"downloadUrl"`
			} `json:"attachments"`
		} `json:"draftMessage"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("failed to parse output %q: %v", stdout, err)
	}
	if len(payload) != 2 {
		t.Fatalf("expected 2 threads, got %s", stdout)
	}
	draft := payload[0].DraftMessage
	if draft == nil {
		t.Fatalf("expected a draft on thread-sub: %s", stdout)
	}
	if draft.ID != "draft-1" || draft.ThreadID != "thread-sub" {
		t.Fatalf("unexpected draft identity: %s", stdout)
	}
	if draft.MessageBody != "<p>Draft reply &amp; notes</p>" {
		t.Fatalf("expected raw HTML preserved verbatim: %s", stdout)
	}
	if draft.MessageBodyPlain != "Draft reply & notes" {
		t.Fatalf("unexpected plain text projection: %s", stdout)
	}
	if len(draft.Attachments) != 1 || draft.Attachments[0].FileName != "notes.txt" || draft.Attachments[0].DownloadURL != "" {
		t.Fatalf("unexpected draft attachments: %s", stdout)
	}
	if payload[1].DraftMessage != nil {
		t.Fatalf("expected no draft on thread-app: %s", stdout)
	}

	draftRequests := 0
	for _, path := range *requested {
		if strings.Contains(path, "resolutionCenterDraftMessage") {
			draftRequests++
		}
	}
	if draftRequests != 2 {
		t.Fatalf("expected one draft request per thread, got %v", *requested)
	}
}

func TestWebReviewThreadsRendersTable(t *testing.T) {
	stubWebReviewSession(t, map[string]string{
		"/iris/v1/apps/app-1/resolutionCenterThreads":                              reviewThreadsAppFixture,
		"/iris/v1/resolutionCenterThreads/thread-sub/resolutionCenterDraftMessage": reviewThreadsDraftFixture,
		"/iris/v1/resolutionCenterThreads/thread-app/resolutionCenterDraftMessage": `{"data": null}`,
	})

	cmd := WebReviewThreadsCommand()
	if err := cmd.FlagSet.Parse([]string{"--app", "app-1", "--drafts", "--output", "table"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	stdout, _ := runWebReviewCommand(t, func() error { return cmd.Exec(context.Background(), nil) })

	for _, want := range []string{"Thread ID", "Draft", "thread-sub", "thread-app", "REJECTION_BINARY", "draft-1", "none", "Draft reply & notes"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("table output missing %q: %s", want, stdout)
		}
	}
}

func TestWebReviewThreadsRendersPlainTextDraftBodyInMarkdown(t *testing.T) {
	stubWebReviewSession(t, map[string]string{
		"/iris/v1/apps/app-1/resolutionCenterThreads":                              reviewThreadsAppFixture,
		"/iris/v1/resolutionCenterThreads/thread-sub/resolutionCenterDraftMessage": reviewThreadsDraftFixture,
		"/iris/v1/resolutionCenterThreads/thread-app/resolutionCenterDraftMessage": `{"data": null}`,
	})

	cmd := WebReviewThreadsCommand()
	if err := cmd.FlagSet.Parse([]string{"--app", "app-1", "--drafts", "--plain-text", "--output", "markdown"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	stdout, _ := runWebReviewCommand(t, func() error { return cmd.Exec(context.Background(), nil) })

	for _, want := range []string{"Draft", "draft-1", "none", "Draft reply & notes"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("markdown output missing %q: %s", want, stdout)
		}
	}
	if strings.Contains(stdout, "<p>") || strings.Contains(stdout, "&amp;") {
		t.Fatalf("markdown draft body should be plain text, got %s", stdout)
	}
}

func TestWebReviewThreadsRequiresApp(t *testing.T) {
	cmd := WebReviewThreadsCommand()
	if err := cmd.FlagSet.Parse([]string{"--output", "json"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	var err error
	_, _ = captureOutput(t, func() { err = cmd.Exec(context.Background(), nil) })
	if err == nil {
		t.Fatal("expected --app to be required")
	}
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected a usage error, got %v", err)
	}
	if kind := shared.ClassifyUsageError(err); kind != shared.UsageErrorMissingRequired {
		t.Fatalf("usage kind = %q, want %q", kind, shared.UsageErrorMissingRequired)
	}
}

func TestWebReviewThreadsRejectsPlainTextWithoutDrafts(t *testing.T) {
	cmd := WebReviewThreadsCommand()
	if err := cmd.FlagSet.Parse([]string{"--app", "app-1", "--plain-text", "--output", "json"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	var err error
	_, _ = captureOutput(t, func() { err = cmd.Exec(context.Background(), nil) })
	if err == nil {
		t.Fatal("expected --plain-text without --drafts to fail instead of being ignored")
	}
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected a usage error, got %v", err)
	}
	if !strings.Contains(err.Error(), "--drafts") {
		t.Fatalf("expected the error to name --drafts, got %v", err)
	}
}

func TestWebReviewShowReportsAppThreadsMissedBySubmissionFilter(t *testing.T) {
	stubWebReviewSession(t, map[string]string{
		"/iris/v1/apps/app-1/reviewSubmissions":                                `{"data": [{"id": "sub-1", "type": "reviewSubmissions", "attributes": {"state": "COMPLETE", "submittedDate": "2026-02-25T00:00:00Z"}}]}`,
		"/iris/v1/reviewSubmissions/sub-1/items":                               `{"data": []}`,
		"/iris/v1/resolutionCenterThreads":                                     `{"data": [{"id": "thread-sub", "type": "resolutionCenterThreads", "attributes": {"state": "OPEN"}, "relationships": {"reviewSubmission": {"data": {"type": "reviewSubmissions", "id": "sub-1"}}}}]}`,
		"/iris/v1/resolutionCenterThreads/thread-sub/resolutionCenterMessages": `{"data": []}`,
		"/iris/v1/reviewRejections":                                            `{"data": []}`,
		"/iris/v1/apps/app-1/resolutionCenterThreads":                          reviewThreadsAppFixture,
	})

	cmd := WebReviewShowCommand()
	if err := cmd.FlagSet.Parse([]string{"--app", "app-1", "--output", "json"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	stdout, _ := runWebReviewCommand(t, func() error { return cmd.Exec(context.Background(), nil) })

	var payload struct {
		Threads []struct {
			Thread struct {
				ID string `json:"id"`
			} `json:"thread"`
		} `json:"threads"`
		AppThreads []struct {
			ID         string `json:"id"`
			ThreadType string `json:"threadType"`
		} `json:"appThreads"`
		AppThreadsWarning string `json:"appThreadsWarning"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("failed to parse output %q: %v", stdout, err)
	}
	if len(payload.Threads) != 1 || payload.Threads[0].Thread.ID != "thread-sub" {
		t.Fatalf("unexpected submission threads: %s", stdout)
	}
	if len(payload.AppThreads) != 1 {
		t.Fatalf("expected exactly the app thread the submission filter missed: %s", stdout)
	}
	if payload.AppThreads[0].ID != "thread-app" || payload.AppThreads[0].ThreadType != "APP_MESSAGE_INFORMATIONAL" {
		t.Fatalf("unexpected app thread: %s", stdout)
	}
	if payload.AppThreadsWarning != "" {
		t.Fatalf("expected no warning: %s", stdout)
	}
}

func TestWebReviewShowReportsAppThreadsWhenNoSubmissionExists(t *testing.T) {
	stubWebReviewSession(t, map[string]string{
		"/iris/v1/apps/app-1/reviewSubmissions":       `{"data": []}`,
		"/iris/v1/apps/app-1/resolutionCenterThreads": reviewThreadsAppFixture,
	})

	cmd := WebReviewShowCommand()
	if err := cmd.FlagSet.Parse([]string{"--app", "app-1", "--output", "json"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	stdout, _ := runWebReviewCommand(t, func() error { return cmd.Exec(context.Background(), nil) })

	var payload struct {
		Selection  string `json:"selection"`
		AppThreads []struct {
			ID string `json:"id"`
		} `json:"appThreads"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("failed to parse output %q: %v", stdout, err)
	}
	if payload.Selection != "none" {
		t.Fatalf("unexpected selection: %s", stdout)
	}
	if len(payload.AppThreads) != 2 {
		t.Fatalf("expected app threads without a submission: %s", stdout)
	}
}

func TestWebReviewShowSurvivesAppThreadFailure(t *testing.T) {
	stubWebReviewSession(t, map[string]string{
		"/iris/v1/apps/app-1/reviewSubmissions":                                `{"data": [{"id": "sub-1", "type": "reviewSubmissions", "attributes": {"state": "COMPLETE", "submittedDate": "2026-02-25T00:00:00Z"}}]}`,
		"/iris/v1/reviewSubmissions/sub-1/items":                               `{"data": []}`,
		"/iris/v1/resolutionCenterThreads":                                     `{"data": [{"id": "thread-sub", "type": "resolutionCenterThreads", "attributes": {"state": "OPEN"}, "relationships": {"reviewSubmission": {"data": {"type": "reviewSubmissions", "id": "sub-1"}}}}]}`,
		"/iris/v1/resolutionCenterThreads/thread-sub/resolutionCenterMessages": `{"data": []}`,
		"/iris/v1/reviewRejections":                                            `{"data": []}`,
		"/iris/v1/apps/app-1/resolutionCenterThreads":                          `!{"errors": [{"code": "PARAMETER_ERROR.INVALID"}]}`,
	})

	cmd := WebReviewShowCommand()
	if err := cmd.FlagSet.Parse([]string{"--app", "app-1", "--output", "json"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	stdout, stderr := runWebReviewCommand(t, func() error { return cmd.Exec(context.Background(), nil) })

	var payload struct {
		Threads []struct {
			Thread struct {
				ID string `json:"id"`
			} `json:"thread"`
		} `json:"threads"`
		AppThreadsWarning string `json:"appThreadsWarning"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("failed to parse output %q: %v", stdout, err)
	}
	if len(payload.Threads) != 1 {
		t.Fatalf("expected submission threads to survive an app-scope failure: %s", stdout)
	}
	if !strings.Contains(payload.AppThreadsWarning, "400") {
		t.Fatalf("expected the app-scope failure to be reported, got %q", payload.AppThreadsWarning)
	}
	if !strings.Contains(stderr, "Warning:") {
		t.Fatalf("expected a stderr warning, got %q", stderr)
	}
}

func testWebReviewClient(t *testing.T) *webcore.Client {
	t.Helper()
	resolve, ok := resolveSessionFn.(func(context.Context, string, string, string) (*webcore.AuthSession, string, error))
	if !ok {
		t.Fatalf("resolveSessionFn type %T", resolveSessionFn)
	}
	session, _, err := resolve(context.Background(), "", "", "")
	if err != nil {
		t.Fatalf("resolve session: %v", err)
	}
	return webcore.NewClient(session)
}

func TestLoadAppThreadsPropagatesCancellation(t *testing.T) {
	stubWebReviewSession(t, map[string]string{
		"/iris/v1/apps/app-1/resolutionCenterThreads": reviewThreadsAppFixture,
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, warning, err := loadAppThreads(ctx, testWebReviewClient(t), "app-1")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	if warning != "" {
		t.Fatalf("caller cancellation must not degrade to a warning, got %q", warning)
	}
}

func TestLoadAppThreadsWarnsOnIndependentTimeout(t *testing.T) {
	stubWebReviewSession(t, map[string]string{
		"/iris/v1/apps/app-1/resolutionCenterThreads": reviewThreadsAppFixture,
	})

	ctx, cancel := context.WithTimeout(context.Background(), time.Nanosecond)
	defer cancel()
	time.Sleep(time.Millisecond)

	_, warning, err := loadAppThreads(ctx, testWebReviewClient(t), "app-1")
	if err != nil {
		t.Fatalf("independent timeout should stay warning-only, got %v", err)
	}
	if warning == "" {
		t.Fatal("expected a timeout warning")
	}
}

func TestWebReviewThreadsRendersMarkdownAndEmptyResults(t *testing.T) {
	stubWebReviewSession(t, map[string]string{
		"/iris/v1/apps/app-1/resolutionCenterThreads": reviewThreadsAppFixture,
	})

	cmd := WebReviewThreadsCommand()
	if err := cmd.FlagSet.Parse([]string{"--app", "app-1", "--output", "markdown"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	stdout, _ := runWebReviewCommand(t, func() error { return cmd.Exec(context.Background(), nil) })

	if !strings.Contains(stdout, "Thread ID") || !strings.Contains(stdout, "| thread-app") {
		t.Fatalf("unexpected markdown output: %s", stdout)
	}
	if strings.Contains(stdout, "Draft") {
		t.Fatalf("expected no draft column without --drafts: %s", stdout)
	}

	stubWebReviewSession(t, map[string]string{
		"/iris/v1/apps/app-2/resolutionCenterThreads": `{"data": []}`,
	})

	empty := WebReviewThreadsCommand()
	if err := empty.FlagSet.Parse([]string{"--app", "app-2", "--output", "json"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	emptyStdout, _ := runWebReviewCommand(t, func() error { return empty.Exec(context.Background(), nil) })

	var entries []json.RawMessage
	if err := json.Unmarshal([]byte(emptyStdout), &entries); err != nil {
		t.Fatalf("failed to parse output %q: %v", emptyStdout, err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected an empty list, got %s", emptyStdout)
	}
}

func TestWebReviewShowLoadsAppThreadsAfterEssentialRequests(t *testing.T) {
	requested := stubWebReviewSession(t, map[string]string{
		"/iris/v1/apps/app-1/reviewSubmissions":                                `{"data": [{"id": "sub-1", "type": "reviewSubmissions", "attributes": {"state": "COMPLETE", "submittedDate": "2026-02-25T00:00:00Z"}}]}`,
		"/iris/v1/reviewSubmissions/sub-1/items":                               `{"data": []}`,
		"/iris/v1/resolutionCenterThreads":                                     `{"data": [{"id": "thread-sub", "type": "resolutionCenterThreads", "attributes": {"state": "OPEN"}, "relationships": {"reviewSubmission": {"data": {"type": "reviewSubmissions", "id": "sub-1"}}}}]}`,
		"/iris/v1/resolutionCenterThreads/thread-sub/resolutionCenterMessages": `{"data": []}`,
		"/iris/v1/reviewRejections":                                            `{"data": []}`,
		"/iris/v1/apps/app-1/resolutionCenterThreads":                          reviewThreadsAppFixture,
	})

	cmd := WebReviewShowCommand()
	if err := cmd.FlagSet.Parse([]string{"--app", "app-1", "--output", "json"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	runWebReviewCommand(t, func() error { return cmd.Exec(context.Background(), nil) })

	appThreadsIndex, essentialIndex := -1, -1
	for index, path := range *requested {
		switch path {
		case "/iris/v1/apps/app-1/resolutionCenterThreads":
			if appThreadsIndex < 0 {
				appThreadsIndex = index
			}
		case "/iris/v1/resolutionCenterThreads/thread-sub/resolutionCenterMessages":
			essentialIndex = index
		}
	}
	if appThreadsIndex < 0 || essentialIndex < 0 {
		t.Fatalf("expected both the essential and the app-scoped requests: %v", *requested)
	}
	if appThreadsIndex < essentialIndex {
		t.Fatalf("the best-effort app-scoped lookup must run after the essential submission requests so a hang there cannot expire their budget: %v", *requested)
	}
}

func TestReviewDraftMessagesContextGetsOperationSizedBudget(t *testing.T) {
	t.Setenv("ASC_TIMEOUT", "")

	requestCtx, cancelRequest := shared.ContextWithTimeout(context.Background())
	draftCtx, cancelDrafts := reviewDraftMessagesContext(requestCtx)
	defer cancelDrafts()
	cancelRequest()

	if err := draftCtx.Err(); err != nil {
		t.Fatalf("draft reads must outlive the single-request command budget: %v", err)
	}
	deadline, ok := draftCtx.Deadline()
	if !ok {
		t.Fatal("draft reads must stay bounded")
	}
	if remaining := time.Until(deadline); remaining <= asc.DefaultTimeout {
		t.Fatalf("draft reads are paced one per request interval and need more than the %s single-request budget, got %s", asc.DefaultTimeout, remaining)
	}
}

func TestReviewAppThreadsContextIsIndependentOfRequestBudget(t *testing.T) {
	t.Setenv("ASC_TIMEOUT", "")

	requestCtx, cancelRequest := shared.ContextWithTimeout(context.Background())
	appThreadsCtx, cancelAppThreads := reviewAppThreadsContext(requestCtx)
	defer cancelAppThreads()
	cancelRequest()

	if err := appThreadsCtx.Err(); err != nil {
		t.Fatalf("the best-effort lookup must not inherit an already spent budget: %v", err)
	}
	deadline, ok := appThreadsCtx.Deadline()
	if !ok {
		t.Fatal("the best-effort lookup must stay bounded")
	}
	if remaining := time.Until(deadline); remaining <= 0 {
		t.Fatalf("expected a fresh budget, got %s", remaining)
	}
}
