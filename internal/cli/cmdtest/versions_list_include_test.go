package cmdtest

import (
	"context"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	cmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
)

func TestVersionsListIncludeEmitsIncludeQuery(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	requests := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests++
		if req.URL.Path != "/v1/apps/123456789/appStoreVersions" {
			t.Fatalf("expected app versions path, got %s", req.URL.Path)
		}
		if got := req.URL.Query().Get("include"); got != "build,appStoreVersionSubmission" {
			t.Fatalf("expected include=build,appStoreVersionSubmission, got %q", got)
		}
		body := `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.0","platform":"IOS"}}],"included":[{"type":"builds","id":"build-1","attributes":{"version":"42"}}]}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"versions", "list", "--app", "123456789", "--include", "build,appStoreVersionSubmission"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if requests != 1 {
		t.Fatalf("expected 1 request, got %d", requests)
	}
	if !strings.Contains(stdout, `"id":"build-1"`) {
		t.Fatalf("expected included build in envelope output, got %q", stdout)
	}
}

func TestVersionsListRedactsIncludedReviewCredentialsByDefault(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if got := req.URL.Query().Get("include"); got != "appStoreReviewDetail" {
			t.Fatalf("expected include=appStoreReviewDetail, got %q", got)
		}
		body := `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.0","platform":"IOS"}}],"included":[` +
			includedAppStoreReviewDetailJSON() + `]}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"versions", "list", "--app", "123456789",
			"--include", "appStoreReviewDetail", "--output", "json",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	assertNoSentinel(t, "stdout", appStoreDemoPasswordSentinel, stdout)
	assertNoSentinel(t, "stderr", appStoreDemoPasswordSentinel, stderr)
	if !strings.Contains(stdout, redactedDemoPasswordText) {
		t.Fatalf("expected redacted review credential, got %q", stdout)
	}
}

func TestVersionsListIncludesReviewCredentialsWithExplicitOptIn(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		body := `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.0","platform":"IOS"}}],"included":[` +
			includedAppStoreReviewDetailJSON() + `]}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"versions", "list", "--app", "123456789",
			"--include", "appStoreReviewDetail", "--output", "json",
			"--include-sensitive",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if !strings.Contains(stdout, appStoreDemoPasswordSentinel) {
		t.Fatalf("expected real review credential with --include-sensitive, got %q", stdout)
	}
	if !strings.Contains(stderr, includeSensitiveWarningText) {
		t.Fatalf("expected plaintext-secret warning, got %q", stderr)
	}
}

func TestVersionsListPaginateKeepsInclude(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	requestCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		if got := req.URL.Query().Get("include"); got != "build,appStoreReviewDetail" {
			t.Fatalf("expected include=build,appStoreReviewDetail on request %d, got %q", requestCount, got)
		}

		var body string
		switch requestCount {
		case 1:
			body = `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.0","platform":"IOS"}}],"included":[{"type":"builds","id":"build-1","attributes":{"version":"42"}},` + includedAppStoreReviewDetailJSON() + `],"links":{"next":"https://api.appstoreconnect.apple.com/v1/apps/123456789/appStoreVersions?cursor=AQ&include=build%2CappStoreReviewDetail"}}`
		case 2:
			body = `{"data":[{"type":"appStoreVersions","id":"version-2","attributes":{"versionString":"1.1","platform":"IOS"}}],"included":[{"type":"builds","id":"build-2","attributes":{"version":"43"}}]}`
		default:
			t.Fatalf("unexpected request %d to %s", requestCount, req.URL.String())
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"versions", "list", "--app", "123456789", "--include", "build,appStoreReviewDetail", "--paginate"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if requestCount != 2 {
		t.Fatalf("expected 2 requests, got %d", requestCount)
	}
	for _, buildID := range []string{"build-1", "build-2"} {
		if !strings.Contains(stdout, `"id":"`+buildID+`"`) {
			t.Fatalf("expected included build %s in aggregated output, got %q", buildID, stdout)
		}
	}
	assertNoSentinel(t, "paginated stdout", appStoreDemoPasswordSentinel, stdout)
	if !strings.Contains(stdout, redactedDemoPasswordText) {
		t.Fatalf("expected paginated review credential to be redacted, got %q", stdout)
	}
}

func TestVersionsListRejectsInvalidIncludeUsage(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		t.Fatalf("unexpected request to %s", req.URL.String())
		return nil, nil
	})

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "include with next",
			args:    []string{"versions", "list", "--next", "https://api.appstoreconnect.apple.com/v1/apps/123456789/appStoreVersions?cursor=AQ", "--include", "build"},
			wantErr: "--next cannot be combined with --include",
		},
		{
			name:    "repeated include flags",
			args:    []string{"versions", "list", "--app", "123456789", "--include", "build", "--include", "appStoreReviewDetail"},
			wantErr: `--include specified multiple times; pass one comma-separated list, for example --include "build,appStoreReviewDetail"`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assertUsageExit(t, test.args, test.wantErr)
		})
	}
}

func TestVersionsListRejectsUnsupportedInclude(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		t.Fatalf("unexpected request to %s", req.URL.String())
		return nil, nil
	})

	tests := []struct {
		name  string
		value string
	}{
		{name: "unknown value", value: "notARelationship"},
		// The app store versions collection endpoint has no ageRatingDeclaration
		// relationship, unlike the single-version endpoint.
		{name: "single version only value", value: "ageRatingDeclaration"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stdout, stderr := captureOutput(t, func() {
				code := cmd.Run([]string{"versions", "list", "--app", "123456789", "--include", test.value}, "1.2.3")
				if code != cmd.ExitUsage {
					t.Fatalf("exit code = %d, want %d", code, cmd.ExitUsage)
				}
			})

			if strings.TrimSpace(stdout) != "" {
				t.Fatalf("stdout = %q, want empty", stdout)
			}
			if !strings.Contains(stderr, "--include must be one of:") {
				t.Fatalf("expected include usage error, got %q", stderr)
			}
			if !strings.Contains(stderr, "appStoreVersionSubmission") {
				t.Fatalf("expected valid include values in error, got %q", stderr)
			}
			if strings.Contains(stderr, "ageRatingDeclaration") {
				t.Fatalf("ageRatingDeclaration is not supported by the collection endpoint, got %q", stderr)
			}
		})
	}
}
