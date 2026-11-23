package cmdtest

import (
	"errors"
	"flag"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func TestTestFlightReviewSubmissionsListMissingBuild(t *testing.T) {
	stdout, stderr, err := runRootWithDeps(t, nil, "testflight", "review", "submissions", "list")
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected ErrHelp, got %v", err)
	}

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "Error: --build-id is required") {
		t.Fatalf("expected error %q, got %q", "Error: --build-id is required", stderr)
	}
}

func TestTestFlightReviewSubmissionsListOutputTable(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	reg := asctest.NewRegistry(t)
	reg.Register(asctest.GET("/v1/betaAppReviewSubmissions", "filter[build]=build-1", "limit=2"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"betaAppReviewSubmissions","id":"submission-1","attributes":{"betaReviewState":"WAITING_FOR_REVIEW","submittedDate":"2024-01-01T00:00:00Z"}}],"links":{"next":""}}`))

	stdout, stderr, err := runRootWithDeps(t, stubClient(t, reg), "testflight", "review", "submissions", "list", "--build-id", "build-1", "--limit", "2", "--output", "table")
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, "submission-1") {
		t.Fatalf("expected submission in output, got %q", stdout)
	}
	if !strings.Contains(stdout, "WAITING_FOR_REVIEW") {
		t.Fatalf("expected beta review state in output, got %q", stdout)
	}
}

func TestTestFlightReviewSubmissionsListScopesToApp(t *testing.T) {
	type stub struct {
		matcher asctest.Matcher
		body    string
	}
	recentBuilds := asctest.GET("/v1/builds", "filter[app]=100000001", "sort=-uploadedDate", "limit=200")
	tests := []struct {
		name       string
		args       []string
		stubs      []stub
		wantStdout string
		wantErr    string
	}{
		{
			name: "app lists its recent builds' submissions",
			args: []string{"--app", "100000001"},
			stubs: []stub{
				{matcher: recentBuilds, body: `{"data":[{"type":"builds","id":"build-2"},{"type":"builds","id":"build-1"}]}`},
				{matcher: asctest.GET("/v1/betaAppReviewSubmissions", "filter[build]=build-2,build-1"), body: `{"data":[{"type":"betaAppReviewSubmissions","id":"submission-2"}]}`},
			},
			wantStdout: `"id":"submission-2"`,
		},
		{
			name:       "app without builds has no submissions",
			args:       []string{"--app", "100000001"},
			stubs:      []stub{{matcher: recentBuilds, body: `{"data":[]}`}},
			wantStdout: `{"data":[]`,
		},
		{
			name: "build of the app",
			args: []string{"--app", "100000001", "--build-id", "build-1"},
			stubs: []stub{
				{matcher: asctest.GET("/v1/builds/build-1/relationships/app"), body: `{"data":{"type":"apps","id":"100000001"}}`},
				{matcher: asctest.GET("/v1/betaAppReviewSubmissions", "filter[build]=build-1"), body: `{"data":[{"type":"betaAppReviewSubmissions","id":"submission-1"}]}`},
			},
			wantStdout: `"id":"submission-1"`,
		},
		{
			name:    "build of another app",
			args:    []string{"--app", "100000001", "--build-id", "build-9"},
			stubs:   []stub{{matcher: asctest.GET("/v1/builds/build-9/relationships/app"), body: `{"data":{"type":"apps","id":"200000002"}}`}},
			wantErr: `build "build-9" belongs to app "200000002", not "100000001"`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("ASC_APP_ID", "")
			reg := asctest.NewRegistry(t)
			for _, stub := range test.stubs {
				reg.Register(stub.matcher, asctest.JSON(http.StatusOK, stub.body))
			}

			args := append([]string{"testflight", "review", "submissions", "list", "--output", "json"}, test.args...)
			stdout, _, err := runRootWithDeps(t, stubClient(t, reg), args...)

			if test.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantErr) {
					t.Fatalf("err = %v, want %q", err, test.wantErr)
				}
				if stdout != "" {
					t.Fatalf("expected no output for another app's build, got %q", stdout)
				}
				return
			}
			if err != nil {
				t.Fatalf("run error: %v", err)
			}
			if !strings.Contains(stdout, test.wantStdout) {
				t.Fatalf("stdout = %q, want %s", stdout, test.wantStdout)
			}
		})
	}
}
