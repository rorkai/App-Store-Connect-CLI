package publish

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

// TestPublishTestFlightNormalizesTestNotesAndRecoveryPayload proves the request
// body carries the normalized text and that the retry payload stores what was
// submitted, so a retry cannot resubmit the characters Apple rejected.
func TestPublishTestFlightNormalizesTestNotesAndRecoveryPayload(t *testing.T) {
	restore := overridePublishCommandTestHooks(t)
	defer restore()

	const (
		rejectedNotes   = "Cafe\u0301 <b q\u0301"
		normalizedNotes = "Café b q"
	)

	getPublishASCClientFn = func(time.Duration) (*asc.Client, error) { return newPublishCommandTestClient(t), nil }
	resolvePublishAppIDWithLookupFn = func(_ context.Context, _ *asc.Client, _ string) (string, error) {
		return "app-123", nil
	}
	validatePublishIPAPathFn = func(string) (os.FileInfo, error) {
		return newPublishTestFileInfo(t)
	}
	uploadBuildAndWaitForIDFn = func(_ context.Context, _ *asc.Client, _ string, _ string, _ os.FileInfo, version, buildNumber string, _ asc.Platform, _ time.Duration, _ time.Duration, _ bool) (*publishUploadResult, error) {
		return &publishUploadResult{
			Build: &asc.BuildResponse{Data: asc.Resource[asc.BuildAttributes]{
				ID: "build-123",
				Attributes: asc.BuildAttributes{
					Version:         buildNumber,
					ProcessingState: asc.BuildProcessingStateProcessing,
				},
			}},
			Version:     version,
			BuildNumber: buildNumber,
		}, nil
	}
	waitForPublishBuildProcessingFn = func(context.Context, *asc.Client, string, time.Duration) (*asc.BuildResponse, error) {
		return &asc.BuildResponse{Data: asc.Resource[asc.BuildAttributes]{
			ID: "build-123",
			Attributes: asc.BuildAttributes{
				Version:         "42",
				ProcessingState: asc.BuildProcessingStateValid,
			},
		}}, nil
	}

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })
	payload := ""
	requestCount := 0
	http.DefaultTransport = publishCommandRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		switch requestCount {
		case 1:
			return publishCommandJSONResponse(http.StatusOK, `{"data":[{"type":"betaGroups","id":"group-1","attributes":{"name":"External","isInternalGroup":false}}]}`)
		case 2:
			return publishCommandJSONResponse(http.StatusOK, `{"data":[]}`)
		case 3:
			if req.Method != http.MethodPost || req.URL.Path != "/v1/betaBuildLocalizations" {
				t.Fatalf("request %d = %s %s, want POST /v1/betaBuildLocalizations", requestCount, req.Method, req.URL.Path)
			}
			body, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("read request body: %v", err)
			}
			payload = string(body)
			return publishCommandJSONResponse(http.StatusUnprocessableEntity, `{"errors":[{"status":"422","code":"ENTITY_ERROR.ATTRIBUTE.INVALID","title":"The provided entity has an invalid attribute","detail":"What to Test was rejected by the server"}]}`)
		default:
			t.Fatalf("unexpected request %d: %s %s", requestCount, req.Method, req.URL.String())
			return nil, nil
		}
	})

	cmd := PublishTestFlightCommand()
	cmd.FlagSet.SetOutput(io.Discard)
	if err := cmd.FlagSet.Parse([]string{
		"--app", "app-123",
		"--ipa", "Demo.ipa",
		"--version", "1.2.3",
		"--build-number", "42",
		"--group", "External",
		"--test-notes", rejectedNotes,
		"--locale", "en-US",
		"--output", "json",
	}); err != nil {
		t.Fatalf("parse flags: %v", err)
	}

	var runErr error
	stdout, stderr := capturePublishCommandOutput(t, func() error {
		runErr = cmd.Exec(context.Background(), nil)
		return runErr
	})
	if runErr == nil {
		t.Fatal("expected the stubbed test notes rejection")
	}

	if !strings.Contains(payload, `"whatsNew":"`+normalizedNotes+`"`) {
		t.Fatalf("request body = %s, want normalized whatsNew %q", payload, normalizedNotes)
	}
	for _, unwanted := range []string{"<", `\u003c`, "\u0301", `\u0301`} {
		if strings.Contains(payload, unwanted) {
			t.Fatalf("request body = %s, must not contain %q", payload, unwanted)
		}
	}
	if !strings.Contains(stderr, "normalized") {
		t.Fatalf("stderr = %q, want a normalization notice", stderr)
	}

	var result asc.TestFlightPublishResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("decode partial publish result: %v\nstdout=%s", err, stdout)
	}
	if result.Recovery == nil {
		t.Fatalf("expected typed test-notes recovery, got %#v", result)
	}
	if result.Recovery.SubmittedNotes != normalizedNotes {
		t.Fatalf("recovery notes = %q, want the submitted text %q", result.Recovery.SubmittedNotes, normalizedNotes)
	}
	for i, argument := range result.Recovery.Arguments {
		if argument == rejectedNotes {
			t.Fatalf("retry argument %d resubmits the rejected notes: %#v", i, result.Recovery.Arguments)
		}
	}
}
