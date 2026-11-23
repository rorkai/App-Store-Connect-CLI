package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

func TestWebReviewDraftsCommandExposesCRUD(t *testing.T) {
	d, _, _ := newWebTestDeps()
	command := newWebReviewDraftsCommand(d)
	if command == nil {
		t.Fatal("newWebReviewDraftsCommand returned nil")
	}
	if command.Name != "drafts" {
		t.Fatalf("command name = %q, want drafts", command.Name)
	}
	if len(command.Subcommands) != 3 {
		t.Fatalf("subcommand count = %d, want 3", len(command.Subcommands))
	}
	want := []string{"create", "update", "delete"}
	for index, subcommand := range command.Subcommands {
		if subcommand.Name != want[index] {
			t.Fatalf("subcommand %d = %q, want %q", index, subcommand.Name, want[index])
		}
	}
}

type webReviewDraftHTTPResponse struct {
	method string
	path   string
	status int
	body   string
	err    error
}

type webReviewDraftHTTPRequest struct {
	method string
	path   string
	body   []byte
}

func stubWebReviewDraftSequence(t *testing.T, responses []webReviewDraftHTTPResponse) *[]webReviewDraftHTTPRequest {
	t.Helper()
	originalResolveSession := resolveSessionFn
	t.Cleanup(func() { resolveSessionFn = originalResolveSession })

	requests := make([]webReviewDraftHTTPRequest, 0, len(responses))
	index := 0
	resolveSessionFn = func(context.Context, string, string, string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{
			Client: &http.Client{
				Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
					var requestBody []byte
					if req.Body != nil {
						var err error
						requestBody, err = io.ReadAll(req.Body)
						if err != nil {
							return nil, err
						}
					}
					requests = append(requests, webReviewDraftHTTPRequest{
						method: req.Method,
						path:   req.URL.Path,
						body:   requestBody,
					})
					if index >= len(responses) {
						return nil, fmt.Errorf("unexpected request %s %s", req.Method, req.URL.Path)
					}
					want := responses[index]
					index++
					if req.Method != want.method || req.URL.Path != want.path {
						return nil, fmt.Errorf("request %d = %s %s, want %s %s", index, req.Method, req.URL.Path, want.method, want.path)
					}
					if want.err != nil {
						return nil, want.err
					}
					return &http.Response{
						StatusCode: want.status,
						Header:     http.Header{"Content-Type": []string{"application/json"}},
						Body:       io.NopCloser(strings.NewReader(want.body)),
						Request:    req,
					}, nil
				}),
			},
		}, "cache", nil
	}
	return &requests
}

const webReviewDraftSingleThreadFixture = `{"data":[{"id":"thread-1","type":"resolutionCenterThreads","attributes":{"canDeveloperAddNote":false}}]}`

func webReviewDraftResponse(id, body string) string {
	payload := map[string]any{
		"data": map[string]any{
			"id":         id,
			"type":       "resolutionCenterDraftMessages",
			"attributes": map[string]any{"messageBody": body},
		},
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		panic(err)
	}
	return string(encoded)
}

func webReviewDraftCommand(t *testing.T, commandName string, args ...string) (*ffcli.Command, *shared.Deps) {
	t.Helper()
	d, _, _ := newWebTestDeps()
	var command *ffcli.Command
	switch commandName {
	case "create":
		command = newWebReviewDraftCreateCommand(d)
	case "update":
		command = newWebReviewDraftUpdateCommand(d)
	case "delete":
		command = newWebReviewDraftDeleteCommand(d)
	default:
		t.Fatalf("unsupported draft command %q", commandName)
	}
	if err := command.FlagSet.Parse(args); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	return command, d
}

func TestWebReviewDraftCreateVerifiesBodyAndReceipt(t *testing.T) {
	body := "\n  <p>Keep this body exactly.</p> \t\n"
	requested := stubWebReviewDraftSequence(t, []webReviewDraftHTTPResponse{
		{method: http.MethodGet, path: "/iris/v1/apps/app-1/resolutionCenterThreads", status: http.StatusOK, body: webReviewDraftSingleThreadFixture},
		{method: http.MethodGet, path: "/iris/v1/resolutionCenterThreads/thread-1/resolutionCenterDraftMessage", status: http.StatusOK, body: `{"data":null}`},
		{method: http.MethodPost, path: "/iris/v1/resolutionCenterDraftMessages", status: http.StatusCreated, body: webReviewDraftResponse("draft-1", body)},
		{method: http.MethodGet, path: "/iris/v1/resolutionCenterThreads/thread-1/resolutionCenterDraftMessage", status: http.StatusOK, body: webReviewDraftResponse("draft-1", body)},
	})

	command, d := webReviewDraftCommand(t, "create", "--app", "app-1", "--thread-id", "thread-1", "--message", body, "--confirm", "--output", "json")
	stdout, _ := runWebReviewCommand(t, d, func() error { return command.Exec(context.Background(), nil) })

	var receipt struct {
		AppID    string `json:"appId"`
		ThreadID string `json:"threadId"`
		DraftID  string `json:"draftId"`
		Action   string `json:"action"`
		Verified bool   `json:"verified"`
	}
	if err := json.Unmarshal([]byte(stdout), &receipt); err != nil {
		t.Fatalf("decode receipt %q: %v", stdout, err)
	}
	if receipt.AppID != "app-1" || receipt.ThreadID != "thread-1" || receipt.DraftID != "draft-1" || receipt.Action != "created" || !receipt.Verified {
		t.Fatalf("unexpected receipt: %#v", receipt)
	}
	if strings.Contains(stdout, body) {
		t.Fatalf("draft body must not be printed in receipt: %q", stdout)
	}
	if len(*requested) != 4 {
		t.Fatalf("request count = %d, want 4 (%#v)", len(*requested), *requested)
	}
	var createPayload struct {
		Data struct {
			Attributes struct {
				MessageBody string `json:"messageBody"`
			} `json:"attributes"`
			Relationships map[string]struct {
				Data struct {
					ID string `json:"id"`
				} `json:"data"`
			} `json:"relationships"`
		} `json:"data"`
	}
	if err := json.Unmarshal((*requested)[2].body, &createPayload); err != nil {
		t.Fatalf("decode create payload: %v", err)
	}
	if createPayload.Data.Attributes.MessageBody != body {
		t.Fatalf("create body = %q, want %q", createPayload.Data.Attributes.MessageBody, body)
	}
	if got := createPayload.Data.Relationships["resolutionCenterThread"].Data.ID; got != "thread-1" {
		t.Fatalf("thread relationship ID = %q, want thread-1", got)
	}
}

// Without --draft-id, update and delete act on the thread's one draft that the
// preflight read found, so the same requests go out either way.
func TestWebReviewDraftUpdateAndDeleteVerifyExactDraft(t *testing.T) {
	for _, selector := range [][]string{{"--draft-id", "draft-1"}, nil} {
		t.Run(fmt.Sprintf("selector %v", selector), func(t *testing.T) {
			testWebReviewDraftUpdateAndDelete(t, selector)
		})
	}
}

func testWebReviewDraftUpdateAndDelete(t *testing.T, selector []string) {
	t.Run("update", func(t *testing.T) {
		body := "updated\nbody"
		requested := stubWebReviewDraftSequence(t, []webReviewDraftHTTPResponse{
			{method: http.MethodGet, path: "/iris/v1/apps/app-1/resolutionCenterThreads", status: http.StatusOK, body: webReviewDraftSingleThreadFixture},
			{method: http.MethodGet, path: "/iris/v1/resolutionCenterThreads/thread-1/resolutionCenterDraftMessage", status: http.StatusOK, body: webReviewDraftResponse("draft-1", "old")},
			{method: http.MethodPatch, path: "/iris/v1/resolutionCenterDraftMessages/draft-1", status: http.StatusOK, body: webReviewDraftResponse("draft-1", body)},
			{method: http.MethodGet, path: "/iris/v1/resolutionCenterThreads/thread-1/resolutionCenterDraftMessage", status: http.StatusOK, body: webReviewDraftResponse("draft-1", body)},
		})
		command, d := webReviewDraftCommand(t, "update", append([]string{"--app", "app-1", "--thread-id", "thread-1", "--message", body, "--confirm", "--output", "json"}, selector...)...)
		stdout, _ := runWebReviewCommand(t, d, func() error { return command.Exec(context.Background(), nil) })
		if !strings.Contains(stdout, `"action":"updated"`) || !strings.Contains(stdout, `"verified":true`) {
			t.Fatalf("unexpected update receipt: %s", stdout)
		}
		if !strings.Contains(stdout, `"draftId":"draft-1"`) {
			t.Fatalf("update receipt %s, want draft-1", stdout)
		}
		if got := (*requested)[2].method; got != http.MethodPatch {
			t.Fatalf("mutation method = %s, want PATCH", got)
		}
	})

	t.Run("delete", func(t *testing.T) {
		requested := stubWebReviewDraftSequence(t, []webReviewDraftHTTPResponse{
			{method: http.MethodGet, path: "/iris/v1/apps/app-1/resolutionCenterThreads", status: http.StatusOK, body: webReviewDraftSingleThreadFixture},
			{method: http.MethodGet, path: "/iris/v1/resolutionCenterThreads/thread-1/resolutionCenterDraftMessage", status: http.StatusOK, body: webReviewDraftResponse("draft-1", "old")},
			{method: http.MethodDelete, path: "/iris/v1/resolutionCenterDraftMessages/draft-1", status: http.StatusNoContent, body: ""},
			{method: http.MethodGet, path: "/iris/v1/resolutionCenterThreads/thread-1/resolutionCenterDraftMessage", status: http.StatusOK, body: `{"data":null}`},
		})
		command, d := webReviewDraftCommand(t, "delete", append([]string{"--app", "app-1", "--thread-id", "thread-1", "--confirm", "--output", "json"}, selector...)...)
		stdout, _ := runWebReviewCommand(t, d, func() error { return command.Exec(context.Background(), nil) })
		if !strings.Contains(stdout, `"action":"deleted"`) || !strings.Contains(stdout, `"draftId":"draft-1"`) || !strings.Contains(stdout, `"verified":true`) {
			t.Fatalf("unexpected delete receipt: %s", stdout)
		}
		if got := (*requested)[2].method; got != http.MethodDelete {
			t.Fatalf("mutation method = %s, want DELETE", got)
		}
	})
}

func TestWebReviewDraftCreateRefusesAnyExistingDraftBeforeWrite(t *testing.T) {
	requested := stubWebReviewDraftSequence(t, []webReviewDraftHTTPResponse{
		{method: http.MethodGet, path: "/iris/v1/apps/app-1/resolutionCenterThreads", status: http.StatusOK, body: webReviewDraftSingleThreadFixture},
		{method: http.MethodGet, path: "/iris/v1/resolutionCenterThreads/thread-1/resolutionCenterDraftMessage", status: http.StatusOK, body: `{"data":{"type":"resolutionCenterDraftMessages","attributes":{"messageBody":"existing but malformed"}}}`},
	})
	command, d := webReviewDraftCommand(t, "create", "--app", "app-1", "--thread-id", "thread-1", "--message", "new", "--confirm", "--output", "json")
	var commandErr error
	_, _ = captureWebTestDeps(d, func() { commandErr = command.Exec(context.Background(), nil) })
	if commandErr == nil || !strings.Contains(commandErr.Error(), "already has") {
		t.Fatalf("expected existing-draft refusal, got %v", commandErr)
	}
	if len(*requested) != 2 {
		t.Fatalf("request count = %d, want 2 and no POST (%#v)", len(*requested), *requested)
	}
}

func TestWebReviewDraftCreateRefusesMissingPreflightData(t *testing.T) {
	requested := stubWebReviewDraftSequence(t, []webReviewDraftHTTPResponse{
		{method: http.MethodGet, path: "/iris/v1/apps/app-1/resolutionCenterThreads", status: http.StatusOK, body: webReviewDraftSingleThreadFixture},
		{method: http.MethodGet, path: "/iris/v1/resolutionCenterThreads/thread-1/resolutionCenterDraftMessage", status: http.StatusOK, body: `{}`},
	})
	command, d := webReviewDraftCommand(t, "create", "--app", "app-1", "--thread-id", "thread-1", "--message", "new", "--confirm", "--output", "json")
	var commandErr error
	stdout, _ := captureWebTestDeps(d, func() { commandErr = command.Exec(context.Background(), nil) })
	if commandErr == nil {
		t.Fatalf("expected missing data error, got %v", commandErr)
	}
	if stdout != "" || len(*requested) != 2 {
		t.Fatalf("expected no receipt or POST, got stdout=%q requests=%#v", stdout, *requested)
	}
}

func TestWebReviewDraftDeleteRejectsMissingVerificationData(t *testing.T) {
	requested := stubWebReviewDraftSequence(t, []webReviewDraftHTTPResponse{
		{method: http.MethodGet, path: "/iris/v1/apps/app-1/resolutionCenterThreads", status: http.StatusOK, body: webReviewDraftSingleThreadFixture},
		{method: http.MethodGet, path: "/iris/v1/resolutionCenterThreads/thread-1/resolutionCenterDraftMessage", status: http.StatusOK, body: webReviewDraftResponse("draft-1", "old")},
		{method: http.MethodDelete, path: "/iris/v1/resolutionCenterDraftMessages/draft-1", status: http.StatusNoContent},
		{method: http.MethodGet, path: "/iris/v1/resolutionCenterThreads/thread-1/resolutionCenterDraftMessage", status: http.StatusOK, body: `{}`},
	})
	command, d := webReviewDraftCommand(t, "delete", "--app", "app-1", "--thread-id", "thread-1", "--draft-id", "draft-1", "--confirm", "--output", "json")
	var commandErr error
	stdout, _ := captureWebTestDeps(d, func() { commandErr = command.Exec(context.Background(), nil) })
	if commandErr == nil || !strings.Contains(commandErr.Error(), "post-read verification failed; do not retry automatically") {
		t.Fatalf("expected unverified deletion error, got %v", commandErr)
	}
	if stdout != "" || len(*requested) != 4 {
		t.Fatalf("expected no receipt or retry, got stdout=%q requests=%#v", stdout, *requested)
	}
}

func TestWebReviewDraftUpdateRefusesWrongDraftBeforeWrite(t *testing.T) {
	tests := []struct {
		name     string
		existing string
		selector []string
		want     string
	}{
		{name: "mismatched --draft-id", existing: webReviewDraftResponse("other-draft", "existing"), selector: []string{"--draft-id", "draft-1"}, want: "not requested draft"},
		{name: "no draft to default to", existing: `{"data":null}`, want: `create one with ` + "`" + `asc web review drafts create --app "app-1" --thread-id "thread-1" --message MESSAGE --confirm` + "`"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			requested := stubWebReviewDraftSequence(t, []webReviewDraftHTTPResponse{
				{method: http.MethodGet, path: "/iris/v1/apps/app-1/resolutionCenterThreads", status: http.StatusOK, body: webReviewDraftSingleThreadFixture},
				{method: http.MethodGet, path: "/iris/v1/resolutionCenterThreads/thread-1/resolutionCenterDraftMessage", status: http.StatusOK, body: test.existing},
			})
			command, d := webReviewDraftCommand(t, "update", append([]string{"--app", "app-1", "--thread-id", "thread-1", "--message", "new", "--confirm", "--output", "json"}, test.selector...)...)
			var commandErr error
			_, _ = captureWebTestDeps(d, func() { commandErr = command.Exec(context.Background(), nil) })
			if commandErr == nil || !strings.Contains(commandErr.Error(), test.want) {
				t.Fatalf("error = %v, want %q", commandErr, test.want)
			}
			if len(*requested) != 2 {
				t.Fatalf("request count = %d, want 2 and no PATCH (%#v)", len(*requested), *requested)
			}
		})
	}
}

func TestWebReviewDraftUpdateDoesNotRetryMismatchedResponse(t *testing.T) {
	requested := stubWebReviewDraftSequence(t, []webReviewDraftHTTPResponse{
		{method: http.MethodGet, path: "/iris/v1/apps/app-1/resolutionCenterThreads", status: http.StatusOK, body: webReviewDraftSingleThreadFixture},
		{method: http.MethodGet, path: "/iris/v1/resolutionCenterThreads/thread-1/resolutionCenterDraftMessage", status: http.StatusOK, body: webReviewDraftResponse("draft-1", "existing")},
		{method: http.MethodPatch, path: "/iris/v1/resolutionCenterDraftMessages/draft-1", status: http.StatusOK, body: webReviewDraftResponse("other-draft", "new")},
	})
	command, d := webReviewDraftCommand(t, "update", "--app", "app-1", "--thread-id", "thread-1", "--draft-id", "draft-1", "--message", "new", "--confirm", "--output", "json")
	var commandErr error
	_, _ = captureWebTestDeps(d, func() { commandErr = command.Exec(context.Background(), nil) })
	if commandErr == nil || !strings.Contains(commandErr.Error(), "do not retry automatically") {
		t.Fatalf("expected no-retry mismatch error, got %v", commandErr)
	}
	if len(*requested) != 3 {
		t.Fatalf("request count = %d, want exactly one PATCH and no retry (%#v)", len(*requested), *requested)
	}
}

func TestWebReviewDraftCreateClassifiesHTTPRejections(t *testing.T) {
	for _, status := range []int{400, 401, 403, 408, 500} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			requested := stubWebReviewDraftSequence(t, []webReviewDraftHTTPResponse{
				{method: http.MethodGet, path: "/iris/v1/apps/app-1/resolutionCenterThreads", status: http.StatusOK, body: webReviewDraftSingleThreadFixture},
				{method: http.MethodGet, path: "/iris/v1/resolutionCenterThreads/thread-1/resolutionCenterDraftMessage", status: http.StatusOK, body: `{"data":null}`},
				{method: http.MethodPost, path: "/iris/v1/resolutionCenterDraftMessages", status: status, body: `{"errors":[{"detail":"request rejected"}]}`},
			})
			command, d := webReviewDraftCommand(t, "create", "--app", "app-1", "--thread-id", "thread-1", "--message", "new", "--confirm", "--output", "json")
			var commandErr error
			stdout, _ := captureWebTestDeps(d, func() { commandErr = command.Exec(context.Background(), nil) })
			if commandErr == nil || stdout != "" || len(*requested) != 3 {
				t.Fatalf("error=%v stdout=%q requests=%d", commandErr, stdout, len(*requested))
			}
			ambiguous := status == 408 || status >= 500
			if strings.Contains(commandErr.Error(), "outcome may be unknown") != ambiguous {
				t.Fatalf("HTTP %d ambiguity=%t error=%v", status, ambiguous, commandErr)
			}
		})
	}
}

func TestWebReviewDraftCreateWarnsWhenMutationResponseIsLost(t *testing.T) {
	transportErr := errors.New("connection reset by peer")
	requested := stubWebReviewDraftSequence(t, []webReviewDraftHTTPResponse{
		{method: http.MethodGet, path: "/iris/v1/apps/app-1/resolutionCenterThreads", status: http.StatusOK, body: webReviewDraftSingleThreadFixture},
		{method: http.MethodGet, path: "/iris/v1/resolutionCenterThreads/thread-1/resolutionCenterDraftMessage", status: http.StatusOK, body: `{"data":null}`},
		{method: http.MethodPost, path: "/iris/v1/resolutionCenterDraftMessages", err: transportErr},
	})
	command, d := webReviewDraftCommand(t, "create", "--app", "app-1", "--thread-id", "thread-1", "--message", "new", "--confirm", "--output", "json")
	var commandErr error
	_, _ = captureWebTestDeps(d, func() { commandErr = command.Exec(context.Background(), nil) })
	if commandErr == nil {
		t.Fatal("expected lost-response error")
	}
	if !errors.Is(commandErr, transportErr) {
		t.Fatalf("error = %v, want transport cause", commandErr)
	}
	for _, want := range []string{"outcome may be unknown", "do not retry automatically"} {
		if !strings.Contains(commandErr.Error(), want) {
			t.Fatalf("error = %v, want %q", commandErr, want)
		}
	}
	if len(*requested) != 3 {
		t.Fatalf("request count = %d, want exactly one POST and no retry (%#v)", len(*requested), *requested)
	}
}

func TestWebReviewDraftUpdateWarnsWhenMutationResponseCannotBeDecoded(t *testing.T) {
	requested := stubWebReviewDraftSequence(t, []webReviewDraftHTTPResponse{
		{method: http.MethodGet, path: "/iris/v1/apps/app-1/resolutionCenterThreads", status: http.StatusOK, body: webReviewDraftSingleThreadFixture},
		{method: http.MethodGet, path: "/iris/v1/resolutionCenterThreads/thread-1/resolutionCenterDraftMessage", status: http.StatusOK, body: webReviewDraftResponse("draft-1", "existing")},
		{method: http.MethodPatch, path: "/iris/v1/resolutionCenterDraftMessages/draft-1", status: http.StatusOK, body: "not-json"},
	})
	command, d := webReviewDraftCommand(t, "update", "--app", "app-1", "--thread-id", "thread-1", "--draft-id", "draft-1", "--message", "new", "--confirm", "--output", "json")
	var commandErr error
	_, _ = captureWebTestDeps(d, func() { commandErr = command.Exec(context.Background(), nil) })
	if commandErr == nil {
		t.Fatal("expected response-decode error")
	}
	for _, want := range []string{"outcome may be unknown", "do not retry automatically", "failed to parse resolution center draft update response"} {
		if !strings.Contains(commandErr.Error(), want) {
			t.Fatalf("error = %v, want %q", commandErr, want)
		}
	}
	if len(*requested) != 3 {
		t.Fatalf("request count = %d, want exactly one PATCH and no retry (%#v)", len(*requested), *requested)
	}
}

func TestWebReviewDraftBodyFilePreservesVerbatimBody(t *testing.T) {
	body := "\n\tbody from file\n\n"
	path := filepath.Join(t.TempDir(), "reply.txt")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write body file: %v", err)
	}
	requested := stubWebReviewDraftSequence(t, []webReviewDraftHTTPResponse{
		{method: http.MethodGet, path: "/iris/v1/apps/app-1/resolutionCenterThreads", status: http.StatusOK, body: webReviewDraftSingleThreadFixture},
		{method: http.MethodGet, path: "/iris/v1/resolutionCenterThreads/thread-1/resolutionCenterDraftMessage", status: http.StatusOK, body: `{"data":null}`},
		{method: http.MethodPost, path: "/iris/v1/resolutionCenterDraftMessages", status: http.StatusCreated, body: webReviewDraftResponse("draft-1", body)},
		{method: http.MethodGet, path: "/iris/v1/resolutionCenterThreads/thread-1/resolutionCenterDraftMessage", status: http.StatusOK, body: webReviewDraftResponse("draft-1", body)},
	})
	command, d := webReviewDraftCommand(t, "create", "--app", "app-1", "--thread-id", "thread-1", "--body-file", path, "--confirm", "--output", "json")
	_, _ = runWebReviewCommand(t, d, func() error { return command.Exec(context.Background(), nil) })
	var payload struct {
		Data struct {
			Attributes struct {
				MessageBody string `json:"messageBody"`
			} `json:"attributes"`
		} `json:"data"`
	}
	if err := json.Unmarshal((*requested)[2].body, &payload); err != nil {
		t.Fatalf("decode create payload: %v", err)
	}
	if payload.Data.Attributes.MessageBody != body {
		t.Fatalf("body = %q, want %q", payload.Data.Attributes.MessageBody, body)
	}
}

func TestWebReviewDraftRejectsInvalidUTF8BeforeSession(t *testing.T) {
	path := filepath.Join(t.TempDir(), "invalid.txt")
	if err := os.WriteFile(path, []byte{'x', 0xff}, 0o600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"--body-file", path}, {"--message", string([]byte{'x', 0xff})}} {
		t.Run(args[0], func(t *testing.T) {
			original := resolveSessionFn
			t.Cleanup(func() { resolveSessionFn = original })
			calls := 0
			resolveSessionFn = func(context.Context, string, string, string) (*webcore.AuthSession, string, error) {
				calls++
				return nil, "", errors.New("unexpected session resolution")
			}
			flags := append([]string{"--app", "app-1", "--thread-id", "thread-1", "--confirm", "--output", "json"}, args...)
			command, _ := webReviewDraftCommand(t, "create", flags...)
			err := command.Exec(context.Background(), nil)
			if err == nil || !strings.Contains(err.Error(), "UTF-8") || calls != 0 {
				t.Fatalf("error=%v session calls=%d, want UTF-8 refusal before authentication", err, calls)
			}
		})
	}
}

func TestWebReviewDraftValidatesBeforeSession(t *testing.T) {
	originalResolveSession := resolveSessionFn
	t.Cleanup(func() { resolveSessionFn = originalResolveSession })
	resolveSessionFn = func(context.Context, string, string, string) (*webcore.AuthSession, string, error) {
		t.Fatal("session resolution must not run before draft validation")
		return nil, "", nil
	}

	t.Run("confirm", func(t *testing.T) {
		command, _ := webReviewDraftCommand(t, "delete", "--app", "app-1", "--thread-id", "thread-1", "--draft-id", "draft-1", "--output", "json")
		if err := command.Exec(context.Background(), nil); err == nil || !strings.Contains(err.Error(), "--confirm is required") {
			t.Fatalf("expected missing confirm error, got %v", err)
		}
	})
	t.Run("body sources", func(t *testing.T) {
		command, _ := webReviewDraftCommand(t, "create", "--app", "app-1", "--thread-id", "thread-1", "--message", "one", "--body-file", "two", "--confirm", "--output", "json")
		if err := command.Exec(context.Background(), nil); err == nil || !strings.Contains(err.Error(), "cannot be combined") {
			t.Fatalf("expected body-source error, got %v", err)
		}
	})
}
