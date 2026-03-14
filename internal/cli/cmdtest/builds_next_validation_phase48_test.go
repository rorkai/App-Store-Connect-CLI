package cmdtest

import (
	"context"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
)

func runBuildsInvalidNextURLCases(
	t *testing.T,
	argsPrefix []string,
	wantErrPrefix string,
) {
	t.Helper()

	tests := []struct {
		name    string
		next    string
		wantErr string
	}{
		{
			name:    "invalid scheme",
			next:    "http://api.appstoreconnect.apple.com/v1/builds/build-1/icons?cursor=AQ",
			wantErr: wantErrPrefix + " must be an App Store Connect URL",
		},
		{
			name:    "malformed URL",
			next:    "https://api.appstoreconnect.apple.com/%zz",
			wantErr: wantErrPrefix + " must be a valid URL:",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			args := append(append([]string{}, argsPrefix...), "--next", test.next)

			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			var runErr error
			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				runErr = root.Run(context.Background())
			})

			if runErr == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(runErr.Error(), test.wantErr) {
				t.Fatalf("expected error %q, got %v", test.wantErr, runErr)
			}
			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}
		})
	}
}

func runBuildsPaginateFromNext(
	t *testing.T,
	argsPrefix []string,
	firstURL string,
	secondURL string,
	firstBody string,
	secondBody string,
	wantIDs ...string,
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
		switch requestCount {
		case 1:
			if req.Method != http.MethodGet || req.URL.String() != firstURL {
				t.Fatalf("unexpected first request: %s %s", req.Method, req.URL.String())
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(firstBody)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 2:
			if req.Method != http.MethodGet || req.URL.String() != secondURL {
				t.Fatalf("unexpected second request: %s %s", req.Method, req.URL.String())
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(secondBody)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		default:
			t.Fatalf("unexpected extra request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})

	args := append(append([]string{}, argsPrefix...), "--paginate", "--next", firstURL)

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
	for _, id := range wantIDs {
		needle := `"id":"` + id + `"`
		if !strings.Contains(stdout, needle) {
			t.Fatalf("expected output to contain %q, got %q", needle, stdout)
		}
	}
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

func TestBuildsIconsListRejectsInvalidNextURL(t *testing.T) {
	runBuildsInvalidNextURLCases(
		t,
		[]string{"builds", "icons", "list"},
		"builds icons list: --next",
	)
}

func TestBuildsIconsListPaginateFromNext(t *testing.T) {
	const firstURL = "https://api.appstoreconnect.apple.com/v1/builds/build-1/icons?cursor=AQ&limit=200"
	const secondURL = "https://api.appstoreconnect.apple.com/v1/builds/build-1/icons?cursor=BQ&limit=200"

	firstBody := `{"data":[{"type":"buildIcons","id":"build-icon-next-1"}],"links":{"next":"` + secondURL + `"}}`
	secondBody := `{"data":[{"type":"buildIcons","id":"build-icon-next-2"}],"links":{"next":""}}`

	runBuildsPaginateFromNext(
		t,
		[]string{"builds", "icons", "list", "--build", "build-1"},
		firstURL,
		secondURL,
		firstBody,
		secondBody,
		"build-icon-next-1",
		"build-icon-next-2",
	)
}

func TestBuildsIndividualTestersListRejectsInvalidNextURL(t *testing.T) {
	runBuildsInvalidNextURLCases(
		t,
		[]string{"builds", "individual-testers", "list"},
		"builds individual-testers list: --next",
	)
}

func TestBuildsIndividualTestersListPaginateFromNext(t *testing.T) {
	const firstURL = "https://api.appstoreconnect.apple.com/v1/builds/build-1/individualTesters?cursor=AQ&limit=200"
	const secondURL = "https://api.appstoreconnect.apple.com/v1/builds/build-1/individualTesters?cursor=BQ&limit=200"

	firstBody := `{"data":[{"type":"betaTesters","id":"build-individual-tester-next-1"}],"links":{"next":"` + secondURL + `"}}`
	secondBody := `{"data":[{"type":"betaTesters","id":"build-individual-tester-next-2"}],"links":{"next":""}}`

	runBuildsPaginateFromNext(
		t,
		[]string{"builds", "individual-testers", "list", "--build", "build-1"},
		firstURL,
		secondURL,
		firstBody,
		secondBody,
		"build-individual-tester-next-1",
		"build-individual-tester-next-2",
	)
}

func TestBuildsMetricsBetaUsagesRejectsInvalidNextURL(t *testing.T) {
	runBuildsInvalidNextURLCases(
		t,
		[]string{"builds", "metrics", "beta-usages"},
		"builds metrics beta-usages: --next",
	)
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

func TestBuildsRelationshipsGetRejectsInvalidNextURL(t *testing.T) {
	runBuildsInvalidNextURLCases(
		t,
		[]string{"builds", "links", "view", "--type", "icons"},
		"builds links view: --next",
	)
}

func TestBuildsRelationshipsGetPaginateFromNextWithoutBuild(t *testing.T) {
	const firstURL = "https://api.appstoreconnect.apple.com/v1/builds/build-1/relationships/icons?cursor=AQ&limit=200"
	const secondURL = "https://api.appstoreconnect.apple.com/v1/builds/build-1/relationships/icons?cursor=BQ&limit=200"

	firstBody := `{"data":[{"type":"buildIcons","id":"build-relationship-icon-next-1"}],"links":{"next":"` + secondURL + `"}}`
	secondBody := `{"data":[{"type":"buildIcons","id":"build-relationship-icon-next-2"}],"links":{"next":""}}`

	runBuildsPaginateFromNext(
		t,
		[]string{"builds", "links", "view", "--type", "icons"},
		firstURL,
		secondURL,
		firstBody,
		secondBody,
		"build-relationship-icon-next-1",
		"build-relationship-icon-next-2",
	)
}

func TestBuildsTestNotesListRejectsInvalidNextURL(t *testing.T) {
	runBuildsInvalidNextURLCases(
		t,
		[]string{"builds", "test-notes", "list", "--build", "build-1"},
		"builds test-notes list: --next",
	)
}

func TestBuildsTestNotesListPaginateFromNext(t *testing.T) {
	const firstURL = "https://api.appstoreconnect.apple.com/v1/builds/build-1/betaBuildLocalizations?cursor=AQ&limit=200"
	const secondURL = "https://api.appstoreconnect.apple.com/v1/builds/build-1/betaBuildLocalizations?cursor=BQ&limit=200"

	firstBody := `{"data":[{"type":"betaBuildLocalizations","id":"build-test-note-next-1"}],"links":{"next":"` + secondURL + `"}}`
	secondBody := `{"data":[{"type":"betaBuildLocalizations","id":"build-test-note-next-2"}],"links":{"next":""}}`

	runBuildsPaginateFromNext(
		t,
		[]string{"builds", "test-notes", "list", "--build", "build-1"},
		firstURL,
		secondURL,
		firstBody,
		secondBody,
		"build-test-note-next-1",
		"build-test-note-next-2",
	)
}

func TestBuildsUploadsFilesListRejectsInvalidNextURL(t *testing.T) {
	runBuildsInvalidNextURLCases(
		t,
		[]string{"builds", "uploads", "files", "list"},
		"builds uploads files list: --next",
	)
}

func TestBuildsUploadsFilesListPaginateFromNext(t *testing.T) {
	const firstURL = "https://api.appstoreconnect.apple.com/v1/buildUploads/upload-1/buildUploadFiles?cursor=AQ&limit=200"
	const secondURL = "https://api.appstoreconnect.apple.com/v1/buildUploads/upload-1/buildUploadFiles?cursor=BQ&limit=200"

	firstBody := `{"data":[{"type":"buildUploadFiles","id":"build-upload-file-next-1"}],"links":{"next":"` + secondURL + `"}}`
	secondBody := `{"data":[{"type":"buildUploadFiles","id":"build-upload-file-next-2"}],"links":{"next":""}}`

	runBuildsPaginateFromNext(
		t,
		[]string{"builds", "uploads", "files", "list", "--upload", "upload-1"},
		firstURL,
		secondURL,
		firstBody,
		secondBody,
		"build-upload-file-next-1",
		"build-upload-file-next-2",
	)
}

func TestBuildsUploadsListRejectsInvalidNextURL(t *testing.T) {
	runBuildsInvalidNextURLCases(
		t,
		[]string{"builds", "uploads", "list"},
		"builds uploads list: --next",
	)
}

func TestBuildsUploadsListPaginateFromNextWithoutApp(t *testing.T) {
	const firstURL = "https://api.appstoreconnect.apple.com/v1/apps/app-1/buildUploads?cursor=AQ&limit=200"
	const secondURL = "https://api.appstoreconnect.apple.com/v1/apps/app-1/buildUploads?cursor=BQ&limit=200"

	firstBody := `{"data":[{"type":"buildUploads","id":"build-upload-next-1"}],"links":{"next":"` + secondURL + `"}}`
	secondBody := `{"data":[{"type":"buildUploads","id":"build-upload-next-2"}],"links":{"next":""}}`

	runBuildsPaginateFromNext(
		t,
		[]string{"builds", "uploads", "list"},
		firstURL,
		secondURL,
		firstBody,
		secondBody,
		"build-upload-next-1",
		"build-upload-next-2",
	)
}
