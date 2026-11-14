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

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func TestBuildsReadCommandsResolveLatestSelector(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	tests := []struct {
		name         string
		args         []string
		expectedPath string
		responseBody string
		expectStdout string
	}{
		{
			name:         "app view",
			args:         []string{"builds", "app", "view", "--app", "123456789", "--latest", "--output", "json"},
			expectedPath: "/v1/builds/BUILD_123/app",
			responseBody: `{"data":{"type":"apps","id":"app-1"}}`,
			expectStdout: `"id":"app-1"`,
		},
		{
			name:         "pre-release-version view",
			args:         []string{"builds", "pre-release-version", "view", "--app", "123456789", "--latest", "--output", "json"},
			expectedPath: "/v1/builds/BUILD_123/preReleaseVersion",
			responseBody: `{"data":{"type":"preReleaseVersions","id":"prv-1"}}`,
			expectStdout: `"id":"prv-1"`,
		},
		{
			name:         "icons list",
			args:         []string{"builds", "icons", "list", "--app", "123456789", "--latest", "--output", "json"},
			expectedPath: "/v1/builds/BUILD_123/icons",
			responseBody: `{"data":[{"type":"buildIcons","id":"icon-1"}]}`,
			expectStdout: `"id":"icon-1"`,
		},
		{
			name:         "beta-app-review-submission view",
			args:         []string{"builds", "beta-app-review-submission", "view", "--app", "123456789", "--latest", "--output", "json"},
			expectedPath: "/v1/builds/BUILD_123/betaAppReviewSubmission",
			responseBody: `{"data":{"type":"betaAppReviewSubmissions","id":"review-1"}}`,
			expectStdout: `"id":"review-1"`,
		},
		{
			name:         "build-beta-detail view",
			args:         []string{"builds", "build-beta-detail", "view", "--app", "123456789", "--latest", "--output", "json"},
			expectedPath: "/v1/builds/BUILD_123/buildBetaDetail",
			responseBody: `{"data":{"type":"buildBetaDetails","id":"detail-1"}}`,
			expectStdout: `"id":"detail-1"`,
		},
		{
			name:         "links view",
			args:         []string{"builds", "links", "view", "--app", "123456789", "--latest", "--type", "app", "--output", "json"},
			expectedPath: "/v1/builds/BUILD_123/relationships/app",
			responseBody: `{"data":{"type":"apps","id":"app-1"}}`,
			expectStdout: `"id":"app-1"`,
		},
		{
			name:         "individual-testers list",
			args:         []string{"builds", "individual-testers", "list", "--app", "123456789", "--latest", "--output", "json"},
			expectedPath: "/v1/builds/BUILD_123/individualTesters",
			responseBody: `{"data":[{"type":"betaTesters","id":"tester-1"}]}`,
			expectStdout: `"id":"tester-1"`,
		},
		{
			name:         "metrics beta-usages",
			args:         []string{"builds", "metrics", "beta-usages", "--app", "123456789", "--latest", "--output", "json"},
			expectedPath: "/v1/builds/BUILD_123/metrics/betaBuildUsages",
			responseBody: `{"data":[{"type":"betaBuildUsages","id":"usage-1"}]}`,
			expectStdout: `"id":"usage-1"`,
		},
		{
			name:         "app-encryption-declaration view",
			args:         []string{"builds", "app-encryption-declaration", "view", "--app", "123456789", "--latest", "--output", "json"},
			expectedPath: "/v1/builds/BUILD_123/appEncryptionDeclaration",
			responseBody: `{"data":{"type":"appEncryptionDeclarations","id":"enc-1"}}`,
			expectStdout: `"id":"enc-1"`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			reg := asctest.NewRegistry(t)
			reg.Install()
			reg.Register(asctest.GET("/v1/builds", "filter[app]=123456789", "limit=200", "sort=-uploadedDate"),
				asctest.JSON(http.StatusOK, `{
					"data":[{"type":"builds","id":"BUILD_123","attributes":{"uploadedDate":"2026-03-13T00:00:00Z","processingState":"VALID","version":"42"},"relationships":{}}]
				}`))
			reg.Register(asctest.GET(test.expectedPath), asctest.JSON(http.StatusOK, test.responseBody))

			stdout, stderr, err := runCommand(t, test.args)
			if err != nil {
				t.Fatalf("run error: %v", err)
			}

			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}
			if !strings.Contains(stdout, test.expectStdout) {
				t.Fatalf("expected stdout to contain %q, got %q", test.expectStdout, stdout)
			}
		})
	}
}

func TestBuildsMutatingCommandsAcceptUnifiedSelectorsForValidation(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "expire latest requires confirm",
			args:    []string{"builds", "expire", "--app", "123456789", "--latest"},
			wantErr: "Error: --confirm is required to expire build",
		},
		{
			name:    "update build-number requires update flag",
			args:    []string{"builds", "update", "--app", "123456789", "--build-number", "42", "--platform", "IOS"},
			wantErr: "Error: at least one update flag is required",
		},
		{
			name:    "add-groups latest requires group",
			args:    []string{"builds", "add-groups", "--app", "123456789", "--latest"},
			wantErr: "Error: --group is required",
		},
		{
			name:    "remove-groups build-number requires group",
			args:    []string{"builds", "remove-groups", "--app", "123456789", "--build-number", "42", "--platform", "IOS", "--confirm"},
			wantErr: "Error: --group is required",
		},
		{
			name:    "individual-testers add latest requires tester",
			args:    []string{"builds", "individual-testers", "add", "--app", "123456789", "--latest"},
			wantErr: "Error: --tester is required",
		},
		{
			name:    "individual-testers remove build-number requires tester",
			args:    []string{"builds", "individual-testers", "remove", "--app", "123456789", "--build-number", "42", "--platform", "IOS", "--confirm"},
			wantErr: "Error: --tester is required",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
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
				t.Fatalf("expected ErrHelp, got %v", runErr)
			}
			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, test.wantErr) {
				t.Fatalf("expected stderr to contain %q, got %q", test.wantErr, stderr)
			}
			if strings.Contains(stderr, "Error: --build-id is required") {
				t.Fatalf("expected unified selectors to be accepted, got %q", stderr)
			}
		})
	}
}
