package web

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

func TestWebAppsAgeRatingsEdit(t *testing.T) {
	t.Setenv("ASC_WEB_MIN_REQUEST_INTERVAL", "0")
	originalResolve := resolveSessionFn
	t.Cleanup(func() { resolveSessionFn = originalResolve })

	const input = `{"advertising":false,"contests":"NONE","kidsAgeBand":null,"developerAgeRatingInfoUrl":"https://example.com/age-suitability"}`
	const clearURL = `{"developerAgeRatingInfoUrl":null}`
	const response = `{"data":{"type":"ageRatingDeclarations","id":"declaration-1","attributes":{"advertising":false}}}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		const pathPrefix = "/iris/v1/ageRatingDeclarations/"
		id, ok := strings.CutPrefix(req.URL.Path, pathPrefix)
		if req.Method != http.MethodPatch || !ok || req.URL.RawQuery != "" {
			t.Errorf("request = %s %s", req.Method, req.URL)
		}
		if got := req.Header.Get("Content-Type"); got != "application/json" {
			t.Errorf("Content-Type = %q", got)
		}
		var got, want any
		if err := json.NewDecoder(req.Body).Decode(&got); err != nil {
			t.Error(err)
		}
		attrs := input
		if id == "no-content" {
			attrs = clearURL
		}
		if err := json.Unmarshal([]byte(`{"data":{"type":"ageRatingDeclarations","id":"`+id+`","attributes":`+attrs+`}}`), &want); err != nil {
			t.Error(err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("request body = %v, want %v", got, want)
		}
		switch id {
		case "declaration-1":
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, response)
		case "no-content":
			w.WriteHeader(http.StatusNoContent)
		case "invalid-response":
			_, _ = io.WriteString(w, "<html>")
		case "redirect":
			http.Redirect(w, req, pathPrefix+"redirect-target", http.StatusTemporaryRedirect)
		default:
			t.Errorf("unexpected PATCH target %q", id)
			http.NotFound(w, req)
		}
	}))
	t.Cleanup(server.Close)
	target, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	transport := customAppUsersRewriteTransport{target: target, base: server.Client().Transport}
	var sessionResolutions int
	resolveSessionFn = func(context.Context, string, string, string) (*webcore.AuthSession, string, error) {
		sessionResolutions++
		return &webcore.AuthSession{Client: &http.Client{Transport: transport}}, "fixture", nil
	}

	for _, tc := range []struct {
		name, id, input, response string
		diagnostic, requestError  string
		extra                     []string
	}{
		{name: "sparse PATCH", input: input, response: response, extra: []string{"--confirm"}},
		{name: "clear URL with empty success", id: "no-content", input: clearURL, response: `{"declarationId":"no-content"}`, extra: []string{"--confirm"}},
		{name: "invalid successful response", id: "invalid-response", input: input, requestError: "check the declaration before retrying", extra: []string{"--confirm"}},
		{name: "unexpected arguments", input: input, diagnostic: "unexpected positional arguments", extra: []string{"--confirm", "stray"}},
		{name: "redirect stops request", id: "redirect", input: input, requestError: "unexpected redirect during age rating update", extra: []string{"--confirm"}},
		{name: "confirmation required", input: input, diagnostic: "--confirm is required"},
		{name: "invalid output before auth", input: input, extra: []string{"--confirm", "--output", "csv"}, diagnostic: "--output"},
		{name: "unknown field", input: `{"koreaAgeRatingOverride":"NONE"}`, extra: []string{"--confirm"}, diagnostic: `unsupported --input field "koreaAgeRatingOverride"`},
		{name: "null boolean", input: `{"advertising":null}`, extra: []string{"--confirm"}, diagnostic: `--input field "advertising" must be a boolean`},
		{name: "invalid enum", input: `{"contests":"INVALID"}`, extra: []string{"--confirm"}, diagnostic: `--input field "contests" must be one of`},
		{name: "duplicate field", input: `{"advertising":true,"advertising":false}`, extra: []string{"--confirm"}, diagnostic: `duplicate --input field "advertising"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "ratings.json")
			if err := os.WriteFile(path, []byte(tc.input), 0o600); err != nil {
				t.Fatal(err)
			}
			before := sessionResolutions
			d, stdout, stderr := newWebTestDeps()
			id := tc.id
			if id == "" {
				id = "declaration-1"
			}
			args := []string{"apps", "age-ratings", "edit", "--id", id, "--input", path, "--output", "json"}
			err := NewWebCommand(d).ParseAndRun(context.Background(), append(args, tc.extra...))
			if tc.diagnostic != "" {
				if shared.ClassifyUsageError(err) == "" || !strings.Contains(stderr.String(), tc.diagnostic) {
					t.Fatalf("error = %v, stderr = %q, want usage diagnostic %q", err, stderr, tc.diagnostic)
				}
				if sessionResolutions != before || stdout.Len() != 0 {
					t.Fatalf("invalid request resolved a session or printed stdout: %d calls, stdout = %q", sessionResolutions-before, stdout)
				}
				return
			}
			if tc.requestError != "" {
				if err == nil || !strings.Contains(err.Error(), tc.requestError) || stdout.Len() != 0 {
					t.Fatalf("error = %v, stdout = %q, want request error %q", err, stdout, tc.requestError)
				}
				return
			}
			if err != nil || stderr.Len() != 0 || strings.TrimSpace(stdout.String()) != tc.response || sessionResolutions != before+1 {
				t.Fatalf("error = %v, stderr = %q, stdout = %q, session resolutions = %d", err, stderr, stdout, sessionResolutions-before)
			}
		})
	}
}
