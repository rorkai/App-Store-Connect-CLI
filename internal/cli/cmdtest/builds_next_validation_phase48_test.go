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

func TestBuildsRejectsInvalidNextURL(t *testing.T) {
	runInvalidNextURLCases(t, []invalidNextURLCase{
		{args: []string{"builds", "icons", "list"}, wantErr: "builds icons list: --next"},
		{args: []string{"builds", "individual-testers", "list"}, wantErr: "builds individual-testers list: --next"},
		{args: []string{"builds", "metrics", "beta-usages"}, wantErr: "builds metrics beta-usages: --next"},
		{args: []string{"builds", "links", "view", "--type", "icons"}, wantErr: "builds links view: --next"},
		{args: []string{"builds", "test-notes", "list", "--build-id", "build-1"}, wantErr: "builds test-notes list: --next"},
		{args: []string{"builds", "uploads", "files", "list"}, wantErr: "builds uploads files list: --next"},
		{args: []string{"builds", "uploads", "list"}, wantErr: "builds uploads list: --next"},
	})
}

func TestBuildsPaginateFromNext(t *testing.T) {
	runPaginateFromNextCases(t, []paginateFromNextCase{
		{
			args:     []string{"builds", "icons", "list", "--build-id", "build-1"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/builds/build-1/icons?cursor=AQ&limit=200",
			dataType: "buildIcons",
		},
		{
			args:     []string{"builds", "icons", "list"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/builds/build-1/icons?cursor=AQ&limit=200",
			dataType: "buildIcons",
		},
		{
			args:     []string{"builds", "individual-testers", "list", "--build-id", "build-1"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/builds/build-1/individualTesters?cursor=AQ&limit=200",
			dataType: "betaTesters",
		},
		{
			args:     []string{"builds", "individual-testers", "list"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/builds/build-1/individualTesters?cursor=AQ&limit=200",
			dataType: "betaTesters",
		},
		{
			args:     []string{"builds", "links", "view", "--type", "icons"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/builds/build-1/relationships/icons?cursor=AQ&limit=200",
			dataType: "buildIcons",
		},
		{
			args:     []string{"builds", "test-notes", "list", "--build-id", "build-1"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/betaBuildLocalizations?cursor=AQ&limit=200",
			dataType: "betaBuildLocalizations",
		},
		{
			args:     []string{"builds", "uploads", "files", "list", "--upload", "upload-1"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/buildUploads/upload-1/buildUploadFiles?cursor=AQ&limit=200",
			dataType: "buildUploadFiles",
		},
		{
			args:     []string{"builds", "uploads", "list"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/apps/app-1/buildUploads?cursor=AQ&limit=200",
			dataType: "buildUploads",
		},
	})
}

func runBuildsFetchFromNext(
	t *testing.T,
	args []string,
	nextURL string,
	body string,
	wantContains string,
) {
	t.Helper()

	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	requestCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		if requestCount != 1 {
			t.Fatalf("unexpected extra request: %s %s", req.Method, req.URL.String())
		}
		if req.Method != http.MethodGet || req.URL.String() != nextURL {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
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
		if err := root.Parse(args); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, wantContains) {
		t.Fatalf("expected output to contain %q, got %q", wantContains, stdout)
	}
}

func TestBuildsRejectsExpiredFilterFromNextWithoutLatest(t *testing.T) {
	commands := []struct {
		args []string
		next string
	}{
		{[]string{"builds", "icons", "list"}, "https://api.appstoreconnect.apple.com/v1/builds/build-1/icons?cursor=AQ"},
		{[]string{"builds", "individual-testers", "list"}, "https://api.appstoreconnect.apple.com/v1/builds/build-1/individualTesters?cursor=AQ"},
		{[]string{"builds", "metrics", "beta-usages"}, "https://api.appstoreconnect.apple.com/v1/builds/build-1/metrics/betaBuildUsages?cursor=AQ"},
	}

	for _, command := range commands {
		for _, expiredFlag := range []string{"--exclude-expired", "--not-expired"} {
			t.Run(strings.Join(command.args, " ")+" "+expiredFlag, func(t *testing.T) {
				args := append(append([]string{}, command.args...), "--next", command.next, expiredFlag)

				root := RootCommand("1.2.3")
				root.FlagSet.SetOutput(io.Discard)

				var runErr error
				stdout, stderr := captureOutput(t, func() {
					if err := root.Parse(args); err != nil {
						t.Fatalf("parse error: %v", err)
					}
					runErr = root.Run(context.Background())
				})

				if !errors.Is(runErr, flag.ErrHelp) {
					t.Fatalf("expected flag.ErrHelp, got %v", runErr)
				}
				wantErr := "Error: --exclude-expired and --not-expired require --latest"
				if stdout != "" {
					t.Fatalf("expected empty stdout, got %q", stdout)
				}
				if !strings.Contains(stderr, wantErr) {
					t.Fatalf("expected stderr to contain %q, got %q", wantErr, stderr)
				}
			})
		}
	}
}

func TestBuildsMetricsBetaUsagesFetchFromNextWithoutBuild(t *testing.T) {
	const nextURL = "https://api.appstoreconnect.apple.com/v1/builds/build-1/metrics/betaBuildUsages?cursor=AQ&limit=50"
	body := `{"data":[{"date":"2026-02-01","sessions":5}],"links":{"next":""}}`

	runBuildsFetchFromNext(
		t,
		[]string{"builds", "metrics", "beta-usages", "--next", nextURL},
		nextURL,
		body,
		`"date":"2026-02-01"`,
	)
}
