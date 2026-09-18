package cmdtest

import (
	"context"
	"errors"
	"flag"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
)

// "e" plus U+0301 has a precomposed form, "q" plus U+0301 does not, so the
// same input exercises NFC composition and leftover-mark removal alongside the
// "<" character App Store Connect rejects.
const (
	rejectedTestNotes   = "Cafe\u0301 <b q\u0301"
	normalizedTestNotes = "Café b q"
)

func assertNormalizedTestNotesPayload(t *testing.T, payload string) {
	t.Helper()

	if !strings.Contains(payload, `"whatsNew":"`+normalizedTestNotes+`"`) {
		t.Fatalf("request body = %s, want normalized whatsNew %q", payload, normalizedTestNotes)
	}
	for _, unwanted := range []string{"<", `\u003c`, "\u0301", `\u0301`} {
		if strings.Contains(payload, unwanted) {
			t.Fatalf("request body = %s, must not contain %q", payload, unwanted)
		}
	}
}

func assertNormalizedTestNotesNotice(t *testing.T, stderr string) {
	t.Helper()

	if !strings.Contains(stderr, "normalized") {
		t.Fatalf("stderr = %q, want a normalization notice", stderr)
	}
	if strings.Count(strings.TrimSpace(stderr), "\n") != 0 {
		t.Fatalf("stderr = %q, want a single-line notice", stderr)
	}
	if strings.Contains(stderr, "Caf") {
		t.Fatalf("stderr = %q, must not echo the notes", stderr)
	}
}

func TestBuildsTestNotesCreateNormalizesNotesBeforeSending(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	payload := ""
	notesPosts := 0
	stubTransport(t, func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/builds/build-1":
			return jsonResponse(http.StatusOK, `{"data":{"type":"builds","id":"build-1","attributes":{"version":"42","processingState":"VALID"},"relationships":{"app":{"data":{"type":"apps","id":"app-1"}}}}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/builds/build-1/app":
			return jsonResponse(http.StatusOK, `{"data":{"type":"apps","id":"app-1"}}`)
		case req.Method == http.MethodGet && strings.Contains(req.URL.Path, "betaAppLocalizations"):
			return jsonResponse(http.StatusOK, `{"data":[]}`)
		case req.Method == http.MethodPost && req.URL.Path == "/v1/betaAppLocalizations":
			return jsonResponse(http.StatusCreated, `{"data":{"type":"betaAppLocalizations","id":"bal-1","attributes":{"locale":"en-US"}}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/builds/build-1/betaBuildLocalizations":
			return jsonResponse(http.StatusOK, `{"data":[]}`)
		case req.Method == http.MethodPost && req.URL.Path == "/v1/betaBuildLocalizations":
			notesPosts++
			body, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("read request body: %v", err)
			}
			payload = string(body)
			return jsonResponse(http.StatusCreated, `{"data":{"type":"betaBuildLocalizations","id":"loc-1","attributes":{"locale":"en-US","whatsNew":"Caf\u00e9 b q"}}}`)
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"builds", "test-notes", "create",
			"--build-id", "build-1",
			"--locale", "en-US",
			"--whats-new", rejectedTestNotes,
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if notesPosts != 1 {
		t.Fatalf("notes create requests = %d, want 1", notesPosts)
	}
	assertNormalizedTestNotesPayload(t, payload)
	assertNormalizedTestNotesNotice(t, stderr)
	if !strings.Contains(stdout, `"id":"loc-1"`) {
		t.Fatalf("stdout = %q, want the created localization", stdout)
	}
}

func TestBuildsTestNotesUpdateNormalizesNotesBeforeSending(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	payload := ""
	requestCount := 0
	stubTransport(t, func(req *http.Request) (*http.Response, error) {
		requestCount++
		if requestCount != 1 {
			t.Fatalf("unexpected request %d: %s %s", requestCount, req.Method, req.URL.Path)
		}
		if req.Method != http.MethodPatch || req.URL.Path != "/v1/betaBuildLocalizations/loc-1" {
			t.Fatalf("request = %s %s, want PATCH /v1/betaBuildLocalizations/loc-1", req.Method, req.URL.Path)
		}
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("read request body: %v", err)
		}
		payload = string(body)
		return jsonResponse(http.StatusOK, `{"data":{"type":"betaBuildLocalizations","id":"loc-1","attributes":{"locale":"en-US","whatsNew":"Caf\u00e9 b q"}}}`)
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"builds", "test-notes", "update",
			"--localization-id", "loc-1",
			"--whats-new", rejectedTestNotes,
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	assertNormalizedTestNotesPayload(t, payload)
	assertNormalizedTestNotesNotice(t, stderr)
	if !strings.Contains(stdout, `"id":"loc-1"`) {
		t.Fatalf("stdout = %q, want the updated localization", stdout)
	}
}

func TestBuildsTestNotesRejectEmptyNormalizedNotesWithoutRequest(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{
			name: "create",
			args: []string{
				"builds", "test-notes", "create",
				"--build-id", "build-1",
				"--locale", "en-US",
				"--whats-new", "< \u0301",
			},
		},
		{
			name: "update",
			args: []string{
				"builds", "test-notes", "update",
				"--localization-id", "loc-1",
				"--whats-new", "< \u0301",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

			stubTransport(t, func(req *http.Request) (*http.Response, error) {
				t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
				return nil, errors.New("unexpected request")
			})

			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			var runErr error
			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(test.args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				runErr = root.Run(context.Background())
			})

			if !errors.Is(runErr, flag.ErrHelp) {
				t.Fatalf("run error = %v, want usage-class failure", runErr)
			}
			if stdout != "" {
				t.Fatalf("stdout = %q, want empty", stdout)
			}
			if !strings.Contains(stderr, "What to Test notes") {
				t.Fatalf("stderr = %q, want a diagnostic naming the notes", stderr)
			}
		})
	}
}

func TestBuildsTestNotesCreateAcceptedNotesPrintNoNotice(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	stubTransport(t, func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/builds/build-1":
			return jsonResponse(http.StatusOK, `{"data":{"type":"builds","id":"build-1","attributes":{"version":"42","processingState":"VALID"}}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/builds/build-1/app":
			return jsonResponse(http.StatusOK, `{"data":{"type":"apps","id":"app-1"}}`)
		case req.Method == http.MethodGet && (strings.Contains(req.URL.Path, "betaAppLocalizations") || strings.Contains(req.URL.Path, "betaBuildLocalizations")):
			return jsonResponse(http.StatusOK, `{"data":[]}`)
		case req.Method == http.MethodPost && req.URL.Path == "/v1/betaAppLocalizations":
			return jsonResponse(http.StatusCreated, `{"data":{"type":"betaAppLocalizations","id":"bal-1","attributes":{"locale":"en-US"}}}`)
		case req.Method == http.MethodPost && req.URL.Path == "/v1/betaBuildLocalizations":
			body, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("read request body: %v", err)
			}
			if !strings.Contains(string(body), `"whatsNew":"Test the new export flow"`) {
				t.Fatalf("request body = %s, want the notes unchanged", string(body))
			}
			return jsonResponse(http.StatusCreated, `{"data":{"type":"betaBuildLocalizations","id":"loc-1","attributes":{"locale":"en-US","whatsNew":"Test the new export flow"}}}`)
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	_, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"builds", "test-notes", "create",
			"--build-id", "build-1",
			"--locale", "en-US",
			"--whats-new", "Test the new export flow",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("stderr = %q, want empty for accepted notes", stderr)
	}
}
