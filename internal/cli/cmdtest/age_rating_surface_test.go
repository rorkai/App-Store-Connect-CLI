package cmdtest

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestDeprecatedAgeRatingGetAliasWarnsAndMatchesViewOutput(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_PROFILE", "")

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/appInfos/info-1/ageRatingDeclaration" {
			t.Fatalf("expected age rating path, got %s", req.URL.Path)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body: io.NopCloser(strings.NewReader(`{
				"data":{"type":"ageRatingDeclarations","id":"age-1","attributes":{"gambling":false}}
			}`)),
			Header: http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

	run := func(args []string) (string, string) {
		root := RootCommand("1.2.3")
		root.FlagSet.SetOutput(io.Discard)

		return captureOutput(t, func() {
			if err := root.Parse(args); err != nil {
				t.Fatalf("parse error: %v", err)
			}
			if err := root.Run(context.Background()); err != nil {
				t.Fatalf("run error: %v", err)
			}
		})
	}

	canonicalStdout, canonicalStderr := run([]string{"age-rating", "view", "--app-info-id", "info-1", "--output", "json"})
	aliasStdout, aliasStderr := run([]string{"age-rating", "get", "--app-info-id", "info-1", "--output", "json"})

	if canonicalStderr != "" {
		t.Fatalf("expected canonical command to avoid warnings, got %q", canonicalStderr)
	}
	requireStderrContainsWarning(t, aliasStderr, "Warning: `asc age-rating get` has been renamed to `asc age-rating view`.")

	var canonicalPayload map[string]any
	if err := json.Unmarshal([]byte(canonicalStdout), &canonicalPayload); err != nil {
		t.Fatalf("parse canonical stdout: %v", err)
	}
	var aliasPayload map[string]any
	if err := json.Unmarshal([]byte(aliasStdout), &aliasPayload); err != nil {
		t.Fatalf("parse alias stdout: %v", err)
	}
	if canonicalStdout != aliasStdout {
		t.Fatalf("expected canonical and alias output to match, canonical=%q alias=%q", canonicalStdout, aliasStdout)
	}
}
