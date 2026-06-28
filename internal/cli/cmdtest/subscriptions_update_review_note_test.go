package cmdtest

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
)

func TestSubscriptionsUpdateSendsReviewNote(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	const note = "Same paywall structure, design may differ."

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodPatch || req.URL.Path != "/v1/subscriptions/sub-1" {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
		}

		var payload struct {
			Data struct {
				Attributes struct {
					ReviewNote *string `json:"reviewNote"`
				} `json:"attributes"`
			} `json:"data"`
		}
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}
		if payload.Data.Attributes.ReviewNote == nil || *payload.Data.Attributes.ReviewNote != note {
			t.Fatalf("expected reviewNote %q, got %#v", note, payload.Data.Attributes.ReviewNote)
		}

		body := `{"data":{"type":"subscriptions","id":"sub-1","attributes":{"reviewNote":"` + note + `"}}}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"subscriptions", "update", "--id", "sub-1", "--review-note", note}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if runErr != nil {
		t.Fatalf("run error: %v", runErr)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"reviewNote":"`+note+`"`) {
		t.Fatalf("expected output to contain review note, got %q", stdout)
	}
}

func TestSubscriptionsUpdateRejectsEmptyReviewNoteExitCode(t *testing.T) {
	assertUsageExit(
		t,
		[]string{"subscriptions", "update", "--id", "sub-1", "--review-note", ""},
		"--review-note cannot be empty",
	)
}
