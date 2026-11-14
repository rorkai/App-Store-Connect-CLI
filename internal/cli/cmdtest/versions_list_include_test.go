package cmdtest

import (
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	cmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func TestVersionsListIncludeEmitsIncludeQuery(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/apps/123456789/appStoreVersions", "include=build,appStoreVersionSubmission"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.0","platform":"IOS"}}],"included":[{"type":"builds","id":"build-1","attributes":{"version":"42"}}]}`))

	stdout, stderr, err := runCommand(t, []string{"versions", "list", "--app", "123456789", "--include", "build,appStoreVersionSubmission"})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if requests := len(reg.Requests()); requests != 1 {
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

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/apps/123456789/appStoreVersions", "include=appStoreReviewDetail"), asctest.JSON(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.0","platform":"IOS"}}],"included":[`+
		includedAppStoreReviewDetailJSON()+`]}`))

	stdout, stderr, err := runCommand(t, []string{
		"versions", "list", "--app", "123456789",
		"--include", "appStoreReviewDetail", "--output", "json",
	})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

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

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/apps/123456789/appStoreVersions", "include=appStoreReviewDetail"), asctest.JSON(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.0","platform":"IOS"}}],"included":[`+
		includedAppStoreReviewDetailJSON()+`]}`))

	stdout, stderr, err := runCommand(t, []string{
		"versions", "list", "--app", "123456789",
		"--include", "appStoreReviewDetail", "--output", "json",
		"--include-sensitive",
	})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

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

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/apps/123456789/appStoreVersions", "include=build,appStoreReviewDetail", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.0","platform":"IOS"}}],"included":[{"type":"builds","id":"build-1","attributes":{"version":"42"}},`+includedAppStoreReviewDetailJSON()+`],"links":{"next":"https://api.appstoreconnect.apple.com/v1/apps/123456789/appStoreVersions?cursor=AQ&include=build%2CappStoreReviewDetail"}}`))
	reg.Register(asctest.GET("/v1/apps/123456789/appStoreVersions", "cursor=AQ", "include=build,appStoreReviewDetail"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"version-2","attributes":{"versionString":"1.1","platform":"IOS"}}],"included":[{"type":"builds","id":"build-2","attributes":{"version":"43"}}]}`))

	stdout, stderr, err := runCommand(t, []string{"versions", "list", "--app", "123456789", "--include", "build,appStoreReviewDetail", "--paginate"})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if requestCount := len(reg.Requests()); requestCount != 2 {
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

	asctest.NewRegistry(t).Install()

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

	asctest.NewRegistry(t).Install()

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
