package cmdtest

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
)

func TestReviewsRatingsJSONStorefrontMissHasRecoveryHint(t *testing.T) {
	const appID = "987654321"
	requests := 0
	transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests++
		if req.Method != http.MethodGet || req.URL.Path != "/lookup" || req.URL.Query().Get("id") != appID || req.URL.Query().Get("country") != "de" {
			t.Fatalf("unexpected lookup: %s %s", req.Method, req.URL)
		}
		return jsonHTTPResponse(http.StatusOK, `{"resultCount":0,"results":[]}`), nil
	})
	stdout, stderr, code := runWithHTTPTransport(t, nil, transport,
		"--error-format", "json", "reviews", "ratings", "--app", appID, "--country", "de")
	if code != cmd.ExitNotFound || stdout != "" || requests != 1 {
		t.Fatalf("exit=%d requests=%d stdout=%q stderr=%q", code, requests, stdout, stderr)
	}
	var envelope struct {
		Error struct {
			Type string `json:"type"`
			Hint string `json:"hint"`
		} `json:"error"`
	}
	if err := json.Unmarshal([]byte(stderr), &envelope); err != nil {
		t.Fatalf("decode stderr %q: %v", stderr, err)
	}
	if envelope.Error.Type != "resource_not_found" || !strings.Contains(envelope.Error.Hint, "--country") ||
		!strings.Contains(envelope.Error.Hint, "--all") || strings.Contains(stderr, appID) {
		t.Fatalf("unexpected storefront error: %s", stderr)
	}
}

func TestReviewsRatingsValidationErrors(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "reviews ratings missing app",
			args:    []string{"reviews", "ratings"},
			wantErr: "--app is required",
		},
		{
			name:    "reviews ratings invalid workers",
			args:    []string{"reviews", "ratings", "--app", "123", "--workers", "0"},
			wantErr: "--workers must be at least 1",
		},
		{
			name:    "reviews ratings rejects positional args",
			args:    []string{"reviews", "ratings", "--app", "123", "extra"},
			wantErr: `unexpected argument "extra"`,
		},
		{
			name:    "reviews ratings unsupported country",
			args:    []string{"reviews", "ratings", "--app", "123", "--country", "zz"},
			wantErr: "unsupported country code",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assertUsageExitWithDeps(t, nil, test.args, test.wantErr)
		})
	}
}

// Note: Help-related tests (TestReviewsHelpShowsRatings, TestReviewsRatingsHelp) were removed
// because flag.ExitOnError causes os.Exit(0) when --help is passed, which panics in tests.
// The validation tests above cover the important functionality.

func TestReviewsRatingsUsesASCAppID(t *testing.T) {
	t.Setenv("ASC_APP_ID", "123")

	_, stderr, err := runRootWithDeps(t, nil, "reviews", "ratings", "--workers", "0")
	if !isUsageClassError(err) {
		t.Fatalf("error = %v, want the --workers usage error", err)
	}
	if strings.Contains(stderr, "--app is required") || !strings.Contains(stderr, "--workers must be at least 1") {
		t.Fatalf("stderr = %q, want ASC_APP_ID to satisfy --app", stderr)
	}
}

func TestReviewsRatingsAppNotInStorefrontExitsNotFound(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		wantLines []string
	}{
		{
			name: "numeric id single country",
			args: []string{"reviews", "ratings", "--app", "123"},
			wantLines: []string{
				"Error: reviews ratings: app not found: 123",
				"Hint: the app must be released in the US storefront; pass --country with another storefront or use --all to check every storefront.",
			},
		},
		{
			name:      "unresolvable bundle id",
			args:      []string{"reviews", "ratings", "--app", "com.example.missing"},
			wantLines: []string{"Error: reviews ratings: could not resolve --app; pass a numeric App Store ID, exact bundle ID, or exact app name"},
		},
		{
			name:      "all countries",
			args:      []string{"reviews", "ratings", "--app", "123", "--all"},
			wantLines: []string{"Error: reviews ratings: app not found in any country: 123"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
				return jsonHTTPResponse(http.StatusOK, `{"resultCount":0,"results":[]}`), nil
			})

			stdout, stderr, code := runWithHTTPTransport(t, nil, transport, test.args...)
			if code != cmd.ExitNotFound {
				t.Fatalf("exit code = %d, want %d", code, cmd.ExitNotFound)
			}
			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			assertStderrLines(t, stderr, test.wantLines...)
		})
	}
}
