package web

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

func TestWebReviewReplyCreatesSendsAndVerifiesMessage(t *testing.T) {
	requested := stubWebReviewSession(t, map[string]string{
		"/iris/v1/resolutionCenterDraftMessages":                             `{"data":{"id":"draft-1","type":"resolutionCenterDraftMessages","attributes":{"messageBody":"Reply body"}}}`,
		"/iris/v1/resolutionCenterMessages":                                  `{"data":{"id":"message-1","type":"resolutionCenterMessages","attributes":{"createdDate":"2026-09-05T00:00:00Z","messageBody":"Reply body"}}}`,
		"/iris/v1/resolutionCenterThreads/thread-1/resolutionCenterMessages": `{"data":[{"id":"message-1","type":"resolutionCenterMessages","attributes":{"createdDate":"2026-09-05T00:00:00Z","messageBody":"Reply body"}}]}`,
	})

	d, _, _ := newWebTestDeps()
	cmd := newWebReviewReplyCommand(d)
	if err := cmd.FlagSet.Parse([]string{
		"--thread-id", "thread-1",
		"--message", "Reply body",
		"--confirm",
		"--output", "json",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	stdout, _ := runWebReviewCommand(t, d, func() error { return cmd.Exec(context.Background(), nil) })

	var receipt struct {
		ThreadID  string `json:"threadId"`
		DraftID   string `json:"draftId"`
		MessageID string `json:"messageId"`
		Verified  bool   `json:"verified"`
	}
	if err := json.Unmarshal([]byte(stdout), &receipt); err != nil {
		t.Fatalf("decode receipt %q: %v", stdout, err)
	}
	if receipt.ThreadID != "thread-1" || receipt.DraftID != "draft-1" || receipt.MessageID != "message-1" || !receipt.Verified {
		t.Fatalf("unexpected receipt: %#v", receipt)
	}
	if strings.Contains(stdout, "Reply body") {
		t.Fatalf("reply body must not be printed in receipt: %q", stdout)
	}
	wantPaths := []string{
		"/iris/v1/resolutionCenterDraftMessages",
		"/iris/v1/resolutionCenterMessages",
		"/iris/v1/resolutionCenterThreads/thread-1/resolutionCenterMessages",
	}
	if strings.Join(*requested, "\n") != strings.Join(wantPaths, "\n") {
		t.Fatalf("request order = %v, want %v", *requested, wantPaths)
	}
}

func TestWebReviewReplyReportsConfirmedMessageWhenReceiptOutputFails(t *testing.T) {
	requested := stubWebReviewSession(t, map[string]string{
		"/iris/v1/resolutionCenterDraftMessages":                             `{"data":{"id":"draft-1","type":"resolutionCenterDraftMessages","attributes":{"messageBody":"Reply body"}}}`,
		"/iris/v1/resolutionCenterMessages":                                  `{"data":{"id":"message-1","type":"resolutionCenterMessages","attributes":{"createdDate":"2026-09-05T00:00:00Z","messageBody":"Reply body"}}}`,
		"/iris/v1/resolutionCenterThreads/thread-1/resolutionCenterMessages": `{"data":[{"id":"message-1","type":"resolutionCenterMessages","attributes":{"createdDate":"2026-09-05T00:00:00Z","messageBody":"Reply body"}}]}`,
	})

	d, depsStdout, depsStderr := newWebTestDeps()
	cmd := newWebReviewReplyCommand(d)
	if err := cmd.FlagSet.Parse([]string{
		"--thread-id", "thread-1",
		"--message", "Reply body",
		"--confirm",
		"--output", "json",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	var commandErr error
	_, _ = runWithWebTestDeps(depsStdout, depsStderr, func() {
		readEnd, writeEnd, err := os.Pipe()
		if err != nil {
			t.Fatalf("create failing stdout pipe: %v", err)
		}
		if err := readEnd.Close(); err != nil {
			t.Fatalf("close failing stdout reader: %v", err)
		}
		d.IO.Out = writeEnd
		defer func() { _ = writeEnd.Close() }()

		commandErr = cmd.Exec(context.Background(), nil)
	})

	if commandErr == nil {
		t.Fatal("expected receipt output error")
	}
	if !strings.Contains(commandErr.Error(), "message message-1 was sent and verified") {
		t.Fatalf("output error = %v, want confirmed message ID", commandErr)
	}
	if !strings.Contains(commandErr.Error(), "do not retry automatically") {
		t.Fatalf("output error = %v, want no-retry guidance", commandErr)
	}
	wantPaths := []string{
		"/iris/v1/resolutionCenterDraftMessages",
		"/iris/v1/resolutionCenterMessages",
		"/iris/v1/resolutionCenterThreads/thread-1/resolutionCenterMessages",
	}
	if strings.Join(*requested, "\n") != strings.Join(wantPaths, "\n") {
		t.Fatalf("request order = %v, want %v", *requested, wantPaths)
	}
}

func TestWebReviewReplyPreservesMessageBody(t *testing.T) {
	originalResolveSession := resolveSessionFn
	t.Cleanup(func() { resolveSessionFn = originalResolveSession })

	type requestCapture struct {
		path string
		body []byte
	}
	requests := make([]requestCapture, 0, 3)
	responses := map[string]string{
		"/iris/v1/resolutionCenterDraftMessages":                             `{"data":{"id":"draft-1","type":"resolutionCenterDraftMessages","attributes":{"messageBody":"Reply body"}}}`,
		"/iris/v1/resolutionCenterMessages":                                  `{"data":{"id":"message-1","type":"resolutionCenterMessages","attributes":{"createdDate":"2026-09-05T00:00:00Z","messageBody":"Reply body"}}}`,
		"/iris/v1/resolutionCenterThreads/thread-1/resolutionCenterMessages": `{"data":[{"id":"message-1","type":"resolutionCenterMessages","attributes":{"createdDate":"2026-09-05T00:00:00Z","messageBody":"Reply body"}}]}`,
	}
	resolveSessionFn = func(context.Context, string, string, string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{
			Client: &http.Client{
				Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
					var body []byte
					if req.Body != nil {
						var err error
						body, err = io.ReadAll(req.Body)
						if err != nil {
							return nil, err
						}
					}
					requests = append(requests, requestCapture{path: req.URL.Path, body: body})
					responseBody, ok := responses[req.URL.Path]
					if !ok {
						t.Errorf("unexpected path: %s", req.URL.Path)
						responseBody = `{"errors":[{"code":"TEST_UNEXPECTED_PATH"}]}`
					}
					status := http.StatusOK
					if req.URL.Path == "/iris/v1/resolutionCenterDraftMessages" || req.URL.Path == "/iris/v1/resolutionCenterMessages" {
						status = http.StatusCreated
					}
					return &http.Response{
						StatusCode: status,
						Header:     http.Header{"Content-Type": []string{"application/json"}},
						Body:       io.NopCloser(strings.NewReader(responseBody)),
						Request:    req,
					}, nil
				}),
			},
		}, "cache", nil
	}

	messageBody := "\n  Reply body \t\n"
	d, _, _ := newWebTestDeps()
	cmd := newWebReviewReplyCommand(d)
	if err := cmd.FlagSet.Parse([]string{
		"--thread-id", "thread-1",
		"--message", messageBody,
		"--confirm",
		"--output", "json",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	stdout, _ := runWebReviewCommand(t, d, func() error { return cmd.Exec(context.Background(), nil) })
	if strings.Contains(stdout, "Reply body") {
		t.Fatalf("reply body must not be printed in receipt: %q", stdout)
	}

	var createBody []byte
	for _, request := range requests {
		if request.path == "/iris/v1/resolutionCenterDraftMessages" {
			createBody = request.body
			break
		}
	}
	if len(createBody) == 0 {
		t.Fatalf("draft create request was not captured: %#v", requests)
	}
	var payload struct {
		Data struct {
			Attributes struct {
				MessageBody string `json:"messageBody"`
			} `json:"attributes"`
		} `json:"data"`
	}
	if err := json.Unmarshal(createBody, &payload); err != nil {
		t.Fatalf("decode draft create request %q: %v", createBody, err)
	}
	if payload.Data.Attributes.MessageBody != messageBody {
		t.Fatalf("message body = %q, want %q", payload.Data.Attributes.MessageBody, messageBody)
	}
}

func TestWebReviewReplyRequiresConfirmBeforeResolvingSession(t *testing.T) {
	originalResolveSession := resolveSessionFn
	t.Cleanup(func() { resolveSessionFn = originalResolveSession })
	resolveSessionFn = func(context.Context, string, string, string) (*webcore.AuthSession, string, error) {
		t.Fatal("session resolution must not run before --confirm validation")
		return nil, "", nil
	}

	d, depsStdout, depsStderr := newWebTestDeps()
	cmd := newWebReviewReplyCommand(d)
	if err := cmd.FlagSet.Parse([]string{"--thread-id", "thread-1", "--message", "Reply body"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	var err error
	_, stderr := runWithWebTestDeps(depsStdout, depsStderr, func() { err = cmd.Exec(context.Background(), nil) })
	if err == nil || !strings.Contains(err.Error(), "--confirm is required") {
		t.Fatalf("expected missing confirm error, got %v (stderr=%q)", err, stderr)
	}
}

func TestWebReviewReplyRejectsEmptyMessageBeforeResolvingSession(t *testing.T) {
	originalResolveSession := resolveSessionFn
	t.Cleanup(func() { resolveSessionFn = originalResolveSession })
	resolveSessionFn = func(context.Context, string, string, string) (*webcore.AuthSession, string, error) {
		t.Fatal("session resolution must not run before message validation")
		return nil, "", nil
	}

	d, depsStdout, depsStderr := newWebTestDeps()
	cmd := newWebReviewReplyCommand(d)
	if err := cmd.FlagSet.Parse([]string{"--thread-id", "thread-1", "--message", "   ", "--confirm"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	var err error
	_, stderr := runWithWebTestDeps(depsStdout, depsStderr, func() { err = cmd.Exec(context.Background(), nil) })
	if err == nil || !strings.Contains(err.Error(), "--message must not be empty") {
		t.Fatalf("expected empty message error, got %v (stderr=%q)", err, stderr)
	}
}

func TestWebReviewReplyRejectsInvalidOutputBeforeSessionOrWrites(t *testing.T) {
	requested := stubWebReviewSession(t, map[string]string{
		"/iris/v1/resolutionCenterDraftMessages":                             `{"data":{"id":"draft-1","type":"resolutionCenterDraftMessages","attributes":{"messageBody":"Reply body"}}}`,
		"/iris/v1/resolutionCenterMessages":                                  `{"data":{"id":"message-1","type":"resolutionCenterMessages","attributes":{"createdDate":"2026-09-05T00:00:00Z","messageBody":"Reply body"}}}`,
		"/iris/v1/resolutionCenterThreads/thread-1/resolutionCenterMessages": `{"data":[{"id":"message-1","type":"resolutionCenterMessages","attributes":{"createdDate":"2026-09-05T00:00:00Z","messageBody":"Reply body"}}]}`,
	})

	originalResolveSession := resolveSessionFn
	resolve, ok := originalResolveSession.(func(context.Context, string, string, string) (*webcore.AuthSession, string, error))
	if !ok {
		t.Fatalf("resolveSessionFn type %T", originalResolveSession)
	}
	resolveCalls := 0
	resolveSessionFn = func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		resolveCalls++
		return resolve(ctx, appleID, password, twoFactorCode)
	}

	originalPersistSession := persistWebSessionFn
	persistCalls := 0
	persistWebSessionFn = func(session *webcore.AuthSession) error {
		persistCalls++
		return originalPersistSession(session)
	}
	t.Cleanup(func() {
		resolveSessionFn = originalResolveSession
		persistWebSessionFn = originalPersistSession
	})

	d, depsStdout, depsStderr := newWebTestDeps()
	cmd := newWebReviewReplyCommand(d)
	if err := cmd.FlagSet.Parse([]string{
		"--thread-id", "thread-1",
		"--message", "Reply body",
		"--confirm",
		"--output", "table",
		"--pretty",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	var err error
	_, stderr := runWithWebTestDeps(depsStdout, depsStderr, func() { err = cmd.Exec(context.Background(), nil) })
	if err == nil || !strings.Contains(err.Error(), "--pretty is only valid with JSON output") {
		t.Fatalf("expected invalid output error, got %v (stderr=%q)", err, stderr)
	}
	if resolveCalls != 0 {
		t.Fatalf("session resolution calls = %d, want 0", resolveCalls)
	}
	createCalls := 0
	sendCalls := 0
	for _, path := range *requested {
		switch path {
		case "/iris/v1/resolutionCenterDraftMessages":
			createCalls++
		case "/iris/v1/resolutionCenterMessages":
			sendCalls++
		}
	}
	if createCalls != 0 || sendCalls != 0 {
		t.Fatalf("write calls = create %d, send %d, want 0 (paths=%v)", createCalls, sendCalls, *requested)
	}
	if persistCalls != 0 {
		t.Fatalf("session persistence calls = %d, want 0", persistCalls)
	}
}

func TestWebReviewReplyWarnsWhenDraftResponseIsLost(t *testing.T) {
	transportErr := errors.New("connection reset by peer")
	requested := stubWebReviewDraftSequence(t, []webReviewDraftHTTPResponse{
		{method: http.MethodPost, path: "/iris/v1/resolutionCenterDraftMessages", err: transportErr},
	})
	d, depsStdout, depsStderr := newWebTestDeps()
	command := newWebReviewReplyCommand(d)
	if err := command.FlagSet.Parse([]string{"--thread-id", "thread-1", "--message", "new", "--confirm", "--output", "json"}); err != nil {
		t.Fatal(err)
	}
	var commandErr error
	_, _ = runWithWebTestDeps(depsStdout, depsStderr, func() { commandErr = command.Exec(context.Background(), nil) })
	if !errors.Is(commandErr, transportErr) {
		t.Fatalf("error = %v, want transport cause", commandErr)
	}
	for _, want := range []string{"outcome may be unknown", "do not retry automatically"} {
		if !strings.Contains(commandErr.Error(), want) {
			t.Fatalf("error = %v, want %q", commandErr, want)
		}
	}
	if len(*requested) != 1 {
		t.Fatalf("requests = %v, want exactly one draft POST and no send", *requested)
	}
}

func TestWebReviewReplyTakesAppScopeAndBodyFile(t *testing.T) {
	body := "Line one.\n\nLine two keeps its spacing.  \n"
	bodyFile := filepath.Join(t.TempDir(), "reply.txt")
	if err := os.WriteFile(bodyFile, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	const threadsPath = "/iris/v1/apps/app-1/resolutionCenterThreads"

	t.Run("thread under the app is sent from --body-file", func(t *testing.T) {
		requested := stubWebReviewDraftSequence(t, []webReviewDraftHTTPResponse{
			{method: http.MethodGet, path: threadsPath, status: http.StatusOK, body: webReviewDraftSingleThreadFixture},
			{method: http.MethodPost, path: "/iris/v1/resolutionCenterDraftMessages", status: http.StatusCreated, body: webReviewDraftResponse("draft-1", body)},
			{method: http.MethodPost, path: "/iris/v1/resolutionCenterMessages", status: http.StatusCreated, body: `{"data":{"id":"message-1","type":"resolutionCenterMessages"}}`},
			{method: http.MethodGet, path: "/iris/v1/resolutionCenterThreads/thread-1/resolutionCenterMessages", status: http.StatusOK, body: `{"data":[{"id":"message-1","type":"resolutionCenterMessages"}]}`},
		})
		d, depsStdout, depsStderr := newWebTestDeps()
		command := newWebReviewReplyCommand(d)
		if err := command.FlagSet.Parse([]string{"--app", "app-1", "--thread-id", "thread-1", "--body-file", bodyFile, "--confirm", "--output", "json"}); err != nil {
			t.Fatal(err)
		}
		var commandErr error
		_, _ = runWithWebTestDeps(depsStdout, depsStderr, func() { commandErr = command.Exec(context.Background(), nil) })
		if commandErr != nil || len(*requested) != 4 {
			t.Fatalf("error=%v requests=%d, want the scoped reply sent", commandErr, len(*requested))
		}
		var payload struct {
			Data struct {
				Attributes struct {
					MessageBody string `json:"messageBody"`
				} `json:"attributes"`
			} `json:"data"`
		}
		if err := json.Unmarshal((*requested)[1].body, &payload); err != nil {
			t.Fatalf("decode draft payload: %v", err)
		}
		if payload.Data.Attributes.MessageBody != body {
			t.Fatalf("draft body = %q, want the file verbatim %q", payload.Data.Attributes.MessageBody, body)
		}
	})

	t.Run("thread outside the app is refused before any write", func(t *testing.T) {
		requested := stubWebReviewDraftSequence(t, []webReviewDraftHTTPResponse{
			{method: http.MethodGet, path: threadsPath, status: http.StatusOK, body: `{"data":[{"id":"thread-9","type":"resolutionCenterThreads"}]}`},
		})
		d, depsStdout, depsStderr := newWebTestDeps()
		command := newWebReviewReplyCommand(d)
		if err := command.FlagSet.Parse([]string{"--app", "app-1", "--thread-id", "thread-1", "--message", "Reply body", "--confirm", "--output", "json"}); err != nil {
			t.Fatal(err)
		}
		var commandErr error
		_, _ = runWithWebTestDeps(depsStdout, depsStderr, func() { commandErr = command.Exec(context.Background(), nil) })
		if commandErr == nil || !strings.Contains(commandErr.Error(), `thread "thread-1" was not found under app "app-1"`) || len(*requested) != 1 {
			t.Fatalf("error=%v requests=%d, want refusal after the thread read only", commandErr, len(*requested))
		}
	})
}
