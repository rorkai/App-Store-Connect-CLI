package cmdtest

import (
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func TestAppScopedCommandsResolveAppByBundleID(t *testing.T) {
	type lookupRequest struct {
		matcher asctest.Matcher
		body    string
	}
	tests := []struct {
		name       string
		args       []string
		bundleID   string
		appID      string
		requests   []lookupRequest
		wantStdout string
		deps       bool // the command is built on shared.Deps: run it on test deps
	}{
		{
			name:     "builds list",
			args:     []string{"builds", "list", "--app", "com.example.lookup"},
			bundleID: "com.example.lookup",
			appID:    "app-lookup",
			requests: []lookupRequest{{
				matcher: asctest.GET("/v1/builds", "filter[app]=app-lookup", "include=preReleaseVersion"),
				body:    `{"data":[{"type":"builds","id":"build-1"}]}`,
			}},
			wantStdout: `"id":"build-1"`,
			deps:       true,
		},
		{
			name:     "builds info latest",
			args:     []string{"builds", "info", "--app", "com.example.latest", "--latest"},
			bundleID: "com.example.latest",
			appID:    "100000001",
			requests: []lookupRequest{
				{
					matcher: asctest.GET("/v1/builds", "filter[app]=100000001", "sort=-uploadedDate", "limit=200"),
					body:    `{"data":[{"type":"builds","id":"build-1","attributes":{"version":"42"}}]}`,
				},
				{
					matcher: asctest.GET("/v1/builds/build-1/preReleaseVersion"),
					body:    `{"data":{"type":"preReleaseVersions","id":"prv-1"}}`,
				},
			},
			wantStdout: `"id":"build-1"`,
			deps:       true,
		},
		{
			name:     "apps view",
			args:     []string{"apps", "view", "--app", "com.example.view"},
			bundleID: "com.example.view",
			appID:    "100000002",
			requests: []lookupRequest{{
				matcher: asctest.GET("/v1/apps/100000002"),
				body:    `{"data":{"type":"apps","id":"100000002"}}`,
			}},
			wantStdout: `"id":"100000002"`,
			deps:       true,
		},
		{
			name:     "testflight crashes list",
			args:     []string{"testflight", "crashes", "list", "--app", "com.example.crashes"},
			bundleID: "com.example.crashes",
			appID:    "app-lookup",
			requests: []lookupRequest{{
				matcher: asctest.GET("/v1/apps/app-lookup/betaFeedbackCrashSubmissions"),
				body:    `{"data":[{"type":"betaFeedbackCrashSubmissions","id":"crash-1"}]}`,
			}},
			wantStdout: `"id":"crash-1"`,
			deps:       true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("ASC_APP_ID", "")
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

			requests := append([]lookupRequest{{
				matcher: asctest.GET("/v1/apps", "filter[bundleId]="+test.bundleID, "limit=2"),
				body:    `{"data":[{"type":"apps","id":"` + test.appID + `"}]}`,
			}}, test.requests...)
			reg := asctest.NewRegistry(t)
			for _, request := range requests {
				reg.Register(request.matcher, asctest.JSON(http.StatusOK, request.body))
			}

			run := func(t *testing.T, args []string) (string, string, error) {
				return runRootWithDeps(t, stubClient(t, reg), args...)
			}
			if !test.deps {
				setupAuth(t)
				reg.Install()
				run = runCommand
			}
			stdout, stderr, err := run(t, test.args)
			if err != nil {
				t.Fatalf("run error: %v", err)
			}

			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}
			if callCount := len(reg.Requests()); callCount != len(requests) {
				t.Fatalf("request count = %d, want %d", callCount, len(requests))
			}
			if !strings.Contains(stdout, test.wantStdout) {
				t.Fatalf("expected %s in output, got %q", test.wantStdout, stdout)
			}
		})
	}
}

func TestBuildsListNextURLSkipsAppLookupForInheritedNonNumericApp(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	const nextURL = "https://api.appstoreconnect.apple.com/v1/builds?cursor=AQ"
	t.Setenv("ASC_APP_ID", "com.example.lookup")

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/builds", "cursor=AQ"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"builds","id":"build-next"}],"links":{"next":""}}`))

	stdout, stderr, err := runCommand(t, []string{
		"builds", "list",
		"--next", nextURL,
	})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"id":"build-next"`) {
		t.Fatalf("expected next-page build output, got %q", stdout)
	}
}
