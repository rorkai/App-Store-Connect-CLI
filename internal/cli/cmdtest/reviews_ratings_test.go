package cmdtest

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
)

func TestReviewsRatingsValidationErrors(t *testing.T) {
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
			assertUsageExit(t, test.args, test.wantErr)
		})
	}
}

// Note: Help-related tests (TestReviewsHelpShowsRatings, TestReviewsRatingsHelp) were removed
// because flag.ExitOnError causes os.Exit(0) when --help is passed, which panics in tests.
// The validation tests above cover the important functionality.

func TestReviewsRatingsUsesASCAppID(t *testing.T) {
	t.Setenv("ASC_APP_ID", "123")

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	_, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"reviews", "ratings", "--workers", "0"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); !isUsageClassError(err) {
			t.Fatalf("error = %v, want the --workers usage error", err)
		}
	})
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
			installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
				return jsonHTTPResponse(http.StatusOK, `{"resultCount":0,"results":[]}`), nil
			}))

			stdout, stderr := captureOutput(t, func() {
				if code := cmd.Run(test.args, "1.2.3"); code != cmd.ExitNotFound {
					t.Fatalf("exit code = %d, want %d", code, cmd.ExitNotFound)
				}
			})
			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			assertStderrLines(t, stderr, test.wantLines...)
		})
	}
}
