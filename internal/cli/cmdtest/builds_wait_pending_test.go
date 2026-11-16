package cmdtest

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// Recorded App Store Connect responses for a build whose upload is still
// moving through processing.
const (
	buildsWaitPendingEmptyCollection = `{"data":[],"links":{"self":"https://api.appstoreconnect.apple.com/v1/builds"}}`

	buildsWaitPendingUploadProcessing = `{
		"data": [
			{
				"type": "buildUploads",
				"id": "upload-78",
				"attributes": {
					"cfBundleShortVersionString": "1.0.12",
					"cfBundleVersion": "78",
					"platform": "IOS",
					"createdDate": "2026-09-30T10:00:00Z",
					"uploadedDate": "2026-09-30T10:01:00Z",
					"state": {"state": "PROCESSING", "errors": [], "warnings": [], "infos": []}
				}
			}
		],
		"links": {"self": "https://api.appstoreconnect.apple.com/v1/apps/123456789/buildUploads"}
	}`

	buildsWaitPendingUploadComplete = `{
		"data": [
			{
				"type": "buildUploads",
				"id": "upload-78",
				"attributes": {
					"cfBundleShortVersionString": "1.0.12",
					"cfBundleVersion": "78",
					"platform": "IOS",
					"createdDate": "2026-09-30T10:00:00Z",
					"uploadedDate": "2026-09-30T10:01:00Z",
					"state": {"state": "COMPLETE", "errors": [], "warnings": [], "infos": []}
				}
			}
		],
		"links": {"self": "https://api.appstoreconnect.apple.com/v1/apps/123456789/buildUploads"}
	}`

	buildsWaitPendingUploadFailed = `{
		"data": [
			{
				"type": "buildUploads",
				"id": "upload-78",
				"attributes": {
					"cfBundleShortVersionString": "1.0.12",
					"cfBundleVersion": "78",
					"platform": "IOS",
					"createdDate": "2026-09-30T10:00:00Z",
					"uploadedDate": "2026-09-30T10:01:00Z",
					"state": {
						"state": "FAILED",
						"errors": [
							{"code": "90189", "description": "Redundant Binary Upload. You've already uploaded a build with build number '78' for version number '1.0.12'."}
						]
					}
				}
			}
		],
		"links": {"self": "https://api.appstoreconnect.apple.com/v1/apps/123456789/buildUploads"}
	}`

	// Two uploads where only the older one predates --since.
	buildsWaitPendingUploadsAroundSince = `{
		"data": [
			{
				"type": "buildUploads",
				"id": "upload-new",
				"attributes": {
					"cfBundleShortVersionString": "1.0.13",
					"cfBundleVersion": "80",
					"platform": "IOS",
					"createdDate": "2026-09-30T12:00:00Z",
					"state": {"state": "AWAITING_UPLOAD"}
				}
			},
			{
				"type": "buildUploads",
				"id": "upload-old",
				"attributes": {
					"cfBundleShortVersionString": "1.0.12",
					"cfBundleVersion": "79",
					"platform": "IOS",
					"createdDate": "2026-09-30T08:00:00Z",
					"uploadedDate": "2026-09-30T08:01:00Z",
					"state": {"state": "COMPLETE"}
				}
			}
		]
	}`

	buildsWaitPendingUploadsBeforeSince = `{
		"data": [
			{
				"type": "buildUploads",
				"id": "upload-old",
				"attributes": {
					"cfBundleShortVersionString": "1.0.12",
					"cfBundleVersion": "79",
					"platform": "IOS",
					"createdDate": "2026-09-30T08:00:00Z",
					"uploadedDate": "2026-09-30T08:01:00Z",
					"state": {"state": "COMPLETE"}
				}
			}
		]
	}`

	buildsWaitPendingBuildsProcessing = `{"data":[{"type":"builds","id":"build-99","attributes":{"uploadedDate":"2026-09-30T10:05:00Z","processingState":"PROCESSING","version":"99"}}]}`
	buildsWaitPendingBuildProcessing  = `{"data":{"type":"builds","id":"build-99","attributes":{"uploadedDate":"2026-09-30T10:05:00Z","processingState":"PROCESSING","version":"99"}}}`
)

type buildsWaitPendingUploadJSON struct {
	ID           string `json:"id"`
	State        string `json:"state"`
	Version      string `json:"version"`
	BuildNumber  string `json:"buildNumber"`
	Platform     string `json:"platform"`
	UploadedDate string `json:"uploadedDate"`
}

type buildsWaitPendingJSON struct {
	Status          string                       `json:"status"`
	Phase           string                       `json:"phase"`
	Summary         string                       `json:"summary"`
	AppID           string                       `json:"appId"`
	BuildID         string                       `json:"buildId"`
	Version         string                       `json:"version"`
	BuildNumber     string                       `json:"buildNumber"`
	Platform        string                       `json:"platform"`
	ProcessingState string                       `json:"processingState"`
	Upload          *buildsWaitPendingUploadJSON `json:"upload"`
	Elapsed         string                       `json:"elapsed"`
	Timeout         string                       `json:"timeout"`
	ResumeCommand   string                       `json:"resumeCommand"`
}

// buildsWaitFakeClock reads a fixed start until the fake API answers its first
// request and 50 seconds later from then on, so reported elapsed times are
// deterministic while the wait itself still expires on its real (millisecond)
// deadline.
type buildsWaitFakeClock struct {
	mu        sync.Mutex
	requested bool
}

func (c *buildsWaitFakeClock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	start := time.Date(2026, time.September, 30, 10, 0, 0, 0, time.UTC)
	if !c.requested {
		return start
	}
	return start.Add(50 * time.Second)
}

// advanceOnRequest returns rt with the clock moved forward before the first
// request it sends.
func (c *buildsWaitFakeClock) advanceOnRequest(rt http.RoundTripper) http.RoundTripper {
	return roundTripFunc(func(req *http.Request) (*http.Response, error) {
		c.mu.Lock()
		c.requested = true
		c.mu.Unlock()
		return rt.RoundTrip(req)
	})
}

// buildsWaitRecordedResponses answers GET requests from recorded bodies keyed
// by path and fails the test on any other request.
func buildsWaitRecordedResponses(t *testing.T, responses map[string]string, inspect func(*http.Request)) http.RoundTripper {
	t.Helper()

	return roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet {
			t.Errorf("unexpected %s %s", req.Method, req.URL.String())
		}
		if inspect != nil {
			inspect(req)
		}
		body, ok := responses[req.URL.Path]
		if !ok {
			t.Errorf("unexpected request path %s", req.URL.Path)
			body = `{"errors":[{"status":"404","code":"NOT_FOUND","title":"not found"}]}`
			return &http.Response{
				StatusCode: http.StatusNotFound,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})
}

// newBuildsWaitPendingDeps returns test deps whose client sends every request
// to rt and whose clock is a buildsWaitFakeClock.
func newBuildsWaitPendingDeps(t *testing.T, rt http.RoundTripper) (*shared.Deps, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()

	clock := &buildsWaitFakeClock{}
	d, stdout, stderr := newTestDeps(t, stubClient(t, clock.advanceOnRequest(rt)))
	d.Now = clock.now
	return d, stdout, stderr
}

func runBuildsWaitPending(t *testing.T, rt http.RoundTripper, args []string) (string, string, int) {
	t.Helper()

	d, stdout, stderr := newBuildsWaitPendingDeps(t, rt)
	code := runWithDepsOnFakeClock(t, d, "1.0.0", args...)
	return stdout.String(), stderr.String(), code
}

func parseBuildsWaitPending(t *testing.T, stdout string) buildsWaitPendingJSON {
	t.Helper()

	var parsed buildsWaitPendingJSON
	if err := json.Unmarshal([]byte(stdout), &parsed); err != nil {
		t.Fatalf("failed to parse pending result %q: %v", stdout, err)
	}
	return parsed
}

func setupBuildsWaitPendingTest(t *testing.T) {
	t.Helper()

	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_MAX_RETRIES", "0")
}

func TestBuildsWaitReportPendingDuringDiscoveryReportsVisibleUpload(t *testing.T) {
	setupBuildsWaitPendingTest(t)

	var uploadQueries []string
	var mu sync.Mutex
	api := buildsWaitRecordedResponses(t, map[string]string{
		"/v1/preReleaseVersions":          buildsWaitPendingEmptyCollection,
		"/v1/apps/123456789/buildUploads": buildsWaitPendingUploadProcessing,
	}, func(req *http.Request) {
		if req.URL.Path == "/v1/apps/123456789/buildUploads" {
			mu.Lock()
			uploadQueries = append(uploadQueries, req.URL.RawQuery)
			mu.Unlock()
		}
	})

	stdout, stderr, code := runBuildsWaitPending(t, api, []string{
		"builds", "wait",
		"--app", "123456789",
		"--build-number", "78",
		"--version", "1.0.12",
		"--platform", "IOS",
		"--timeout", "200ms",
		"--poll-interval", "20ms",
		"--report-pending",
		"--output", "json",
	})

	if code != cmd.ExitPending {
		t.Fatalf("exit code = %d, want %d (ExitPending); stderr=%q", code, cmd.ExitPending, stderr)
	}
	result := parseBuildsWaitPending(t, stdout)
	if result.Status != "pending" || result.Phase != "discovery" {
		t.Fatalf("status/phase = %q/%q, want pending/discovery", result.Status, result.Phase)
	}
	if result.AppID != "123456789" || result.BuildNumber != "78" || result.Version != "1.0.12" || result.Platform != "IOS" {
		t.Fatalf("unexpected selector fields: %+v", result)
	}
	if result.BuildID != "" || result.ProcessingState != "" {
		t.Fatalf("discovery result must not claim a build: %+v", result)
	}
	if result.Upload == nil {
		t.Fatalf("expected the in-flight upload in the result, got %q", stdout)
	}
	wantUpload := buildsWaitPendingUploadJSON{
		ID:           "upload-78",
		State:        "PROCESSING",
		Version:      "1.0.12",
		BuildNumber:  "78",
		Platform:     "IOS",
		UploadedDate: "2026-09-30T10:01:00Z",
	}
	if *result.Upload != wantUpload {
		t.Fatalf("upload = %+v, want %+v", *result.Upload, wantUpload)
	}
	wantSummary := `Build upload "upload-78" (version 1.0.12, build 78, IOS) is PROCESSING and not yet visible as a build.`
	if result.Summary != wantSummary {
		t.Fatalf("summary = %q, want %q", result.Summary, wantSummary)
	}
	if result.Elapsed != "50s" {
		t.Fatalf("elapsed = %q, want 50s from the fake clock", result.Elapsed)
	}
	if result.Timeout != "200ms" {
		t.Fatalf("timeout = %q, want 200ms", result.Timeout)
	}
	wantResume := "asc builds wait --app 123456789 --build-number 78 --version 1.0.12 --platform IOS --output json --poll-interval 20ms --report-pending --timeout 200ms"
	if result.ResumeCommand != wantResume {
		t.Fatalf("resumeCommand = %q, want %q", result.ResumeCommand, wantResume)
	}

	if !strings.Contains(stderr, "Build is still pending after 50s; resume with: "+wantResume+"\n") {
		t.Fatalf("expected pending notice on stderr, got %q", stderr)
	}
	if strings.Contains(stderr, "Error:") {
		t.Fatalf("pending outcome must not be rendered as an error, got %q", stderr)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(uploadQueries) == 0 {
		t.Fatal("expected discovery to read build uploads")
	}
	for _, want := range []string{
		"filter%5BcfBundleVersion%5D=78",
		"filter%5BcfBundleShortVersionString%5D=1.0.12",
		"filter%5Bplatform%5D=IOS",
		"sort=-uploadedDate",
	} {
		if !strings.Contains(uploadQueries[0], want) {
			t.Fatalf("upload query %q missing %q", uploadQueries[0], want)
		}
	}
}

func TestBuildsWaitReportPendingDuringDiscoveryWithoutVisibleUpload(t *testing.T) {
	tests := []struct {
		name       string
		uploads    string
		extraArgs  []string
		wantResume string
		wantUpload string
	}{
		{
			name:       "no upload yet",
			uploads:    buildsWaitPendingEmptyCollection,
			wantResume: "asc builds wait --app 123456789 --latest --poll-interval 20ms --report-pending --timeout 200ms",
		},
		{
			name:       "upload older than since",
			uploads:    buildsWaitPendingUploadsBeforeSince,
			extraArgs:  []string{"--since", "2026-09-30T09:00:00Z"},
			wantResume: "asc builds wait --app 123456789 --latest --since 2026-09-30T09:00:00Z --poll-interval 20ms --report-pending --timeout 200ms",
		},
		{
			name:       "newest upload after since",
			uploads:    buildsWaitPendingUploadsAroundSince,
			extraArgs:  []string{"--since", "2026-09-30T09:00:00Z"},
			wantResume: "asc builds wait --app 123456789 --latest --since 2026-09-30T09:00:00Z --poll-interval 20ms --report-pending --timeout 200ms",
			wantUpload: "upload-new",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupBuildsWaitPendingTest(t)
			api := buildsWaitRecordedResponses(t, map[string]string{
				"/v1/builds":                      buildsWaitPendingEmptyCollection,
				"/v1/apps/123456789/buildUploads": test.uploads,
			}, nil)

			args := []string{
				"builds", "wait",
				"--app", "123456789",
				"--latest",
				"--timeout", "200ms",
				"--poll-interval", "20ms",
				"--report-pending",
			}
			args = append(args, test.extraArgs...)
			stdout, stderr, code := runBuildsWaitPending(t, api, args)

			if code != cmd.ExitPending {
				t.Fatalf("exit code = %d, want %d; stderr=%q", code, cmd.ExitPending, stderr)
			}
			result := parseBuildsWaitPending(t, stdout)
			if result.Status != "pending" || result.Phase != "discovery" {
				t.Fatalf("status/phase = %q/%q, want pending/discovery", result.Status, result.Phase)
			}
			if result.ResumeCommand != test.wantResume {
				t.Fatalf("resumeCommand = %q, want %q", result.ResumeCommand, test.wantResume)
			}
			if test.wantUpload == "" {
				if result.Upload != nil {
					t.Fatalf("expected no upload, got %+v", *result.Upload)
				}
				want := "No build or build upload matching the selector is visible yet."
				if result.Summary != want {
					t.Fatalf("summary = %q, want %q", result.Summary, want)
				}
				return
			}
			if result.Upload == nil || result.Upload.ID != test.wantUpload {
				t.Fatalf("upload = %+v, want %s", result.Upload, test.wantUpload)
			}
			if result.Upload.State != "AWAITING_UPLOAD" || result.Upload.UploadedDate != "" {
				t.Fatalf("unexpected upload state: %+v", *result.Upload)
			}
		})
	}
}

func TestBuildsWaitDiscoveryIgnoresBuildUploadLookupFailures(t *testing.T) {
	setupBuildsWaitPendingTest(t)

	var mu sync.Mutex
	buildPolls := 0
	uploadReads := 0
	api := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		mu.Lock()
		defer mu.Unlock()
		switch req.URL.Path {
		case "/v1/builds":
			buildPolls++
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(buildsWaitPendingEmptyCollection)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case "/v1/apps/123456789/buildUploads":
			uploadReads++
			return &http.Response{
				StatusCode: http.StatusInternalServerError,
				Body:       io.NopCloser(strings.NewReader(`{"errors":[{"status":"500","code":"UNEXPECTED_ERROR","title":"An unexpected error occurred."}]}`)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		default:
			t.Errorf("unexpected request path %s", req.URL.Path)
			return nil, io.ErrUnexpectedEOF
		}
	})

	stdout, stderr, code := runBuildsWaitPending(t, api, []string{
		"builds", "wait",
		"--app", "123456789",
		"--latest",
		"--timeout", "200ms",
		"--poll-interval", "20ms",
		"--report-pending",
	})

	if code != cmd.ExitPending {
		t.Fatalf("exit code = %d, want %d; stderr=%q", code, cmd.ExitPending, stderr)
	}
	result := parseBuildsWaitPending(t, stdout)
	if result.Phase != "discovery" || result.Upload != nil {
		t.Fatalf("expected discovery without upload information, got %+v", result)
	}
	mu.Lock()
	defer mu.Unlock()
	if buildPolls < 2 || uploadReads < 2 {
		t.Fatalf("expected discovery to keep polling past upload lookup failures, got %d build polls and %d upload reads", buildPolls, uploadReads)
	}
}

func TestBuildsWaitReportPendingDuringProcessingResumesByBuildID(t *testing.T) {
	setupBuildsWaitPendingTest(t)
	api := buildsWaitRecordedResponses(t, map[string]string{
		"/v1/builds":          buildsWaitPendingBuildsProcessing,
		"/v1/builds/build-99": buildsWaitPendingBuildProcessing,
	}, nil)

	d, stdoutBuf, stderrBuf := newBuildsWaitPendingDeps(t, api)
	var code int
	// The --timeout deadline runs on the bubble's fake clock, so host load
	// cannot expire it before discovery observes the processing build.
	synctest.Test(t, func(t *testing.T) {
		root := cmd.RootCommandWithDeps("1.0.0", d)
		root.FlagSet.SetOutput(io.Discard)
		err := root.Parse([]string{
			"builds", "wait",
			"--app", "123456789",
			"--latest",
			"--timeout", "200ms",
			"--poll-interval", "20ms",
			"--fail-on-invalid",
			"--report-pending",
			"--output", "json",
			"--pretty",
		})
		if err == nil {
			err = root.Run(context.Background())
		}
		code = cmd.ExitCodeFromError(err)
	})
	stdout, stderr := stdoutBuf.String(), stderrBuf.String()

	if code != cmd.ExitPending {
		t.Fatalf("exit code = %d, want %d; stderr=%q", code, cmd.ExitPending, stderr)
	}
	result := parseBuildsWaitPending(t, stdout)
	if result.Status != "pending" || result.Phase != "processing" {
		t.Fatalf("status/phase = %q/%q, want pending/processing", result.Status, result.Phase)
	}
	if result.BuildID != "build-99" || result.ProcessingState != "PROCESSING" || result.BuildNumber != "99" {
		t.Fatalf("unexpected build state: %+v", result)
	}
	if result.Upload != nil {
		t.Fatalf("processing result must not report an upload, got %+v", *result.Upload)
	}
	wantSummary := `Build "build-99" is PROCESSING; processing has not finished.`
	if result.Summary != wantSummary {
		t.Fatalf("summary = %q, want %q", result.Summary, wantSummary)
	}
	wantResume := "asc builds wait --build-id build-99 --fail-on-invalid --output json --poll-interval 20ms --pretty --report-pending --timeout 200ms"
	if result.ResumeCommand != wantResume {
		t.Fatalf("resumeCommand = %q, want %q", result.ResumeCommand, wantResume)
	}
	if !strings.Contains(stdout, "\n  \"status\": \"pending\"") {
		t.Fatalf("expected --pretty JSON, got %q", stdout)
	}
}

func TestBuildsWaitTimeoutWithoutReportPendingKeepsFailureAndAddsState(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		responses  map[string]string
		wantPrefix string
		wantState  string
		wantResume string
	}{
		{
			name: "discovery",
			args: []string{"builds", "wait", "--app", "123456789", "--build-number", "78", "--platform", "IOS", "--timeout", "200ms", "--poll-interval", "20ms"},
			responses: map[string]string{
				"/v1/builds":                      buildsWaitPendingEmptyCollection,
				"/v1/apps/123456789/buildUploads": buildsWaitPendingUploadComplete,
			},
			wantPrefix: "Error: builds wait: timed out resolving build selector after 0s; ",
			wantState:  `build upload "upload-78" (version 1.0.12, build 78, IOS) is COMPLETE and not yet visible as a build`,
			wantResume: "resume with: asc builds wait --app 123456789 --build-number 78 --platform IOS --poll-interval 20ms --timeout 200ms; add --report-pending to print this state on stdout and exit 7",
		},
		{
			name: "processing",
			args: []string{"builds", "wait", "--build-id", "build-99", "--timeout", "200ms", "--poll-interval", "20ms"},
			responses: map[string]string{
				"/v1/builds/build-99": buildsWaitPendingBuildProcessing,
			},
			wantPrefix: "Error: builds wait: timed out waiting for build build-99 after 0s; ",
			wantState:  `build "build-99" is PROCESSING; processing has not finished`,
			wantResume: "resume with: asc builds wait --build-id build-99 --poll-interval 20ms --timeout 200ms; add --report-pending to print this state on stdout and exit 7",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupBuildsWaitPendingTest(t)
			api := buildsWaitRecordedResponses(t, test.responses, nil)

			stdout, stderr, code := runBuildsWaitPending(t, api, test.args)

			if code != cmd.ExitError {
				t.Fatalf("exit code = %d, want %d; stderr=%q", code, cmd.ExitError, stderr)
			}
			if stdout != "" {
				t.Fatalf("expected empty stdout without --report-pending, got %q", stdout)
			}
			want := test.wantPrefix + test.wantState + "; " + test.wantResume + "\n"
			if !strings.Contains(stderr, want) {
				t.Fatalf("stderr missing %q, got %q", want, stderr)
			}
		})
	}
}

func TestBuildsWaitReportPendingFailedUploadIsAFailure(t *testing.T) {
	setupBuildsWaitPendingTest(t)
	api := buildsWaitRecordedResponses(t, map[string]string{
		"/v1/builds":                      buildsWaitPendingEmptyCollection,
		"/v1/apps/123456789/buildUploads": buildsWaitPendingUploadFailed,
	}, nil)

	stdout, stderr, code := runBuildsWaitPending(t, api, []string{
		"builds", "wait",
		"--app", "123456789",
		"--build-number", "78",
		"--platform", "IOS",
		"--timeout", "200ms",
		"--poll-interval", "20ms",
		"--report-pending",
	})

	if code != cmd.ExitError {
		t.Fatalf("exit code = %d, want %d; stderr=%q", code, cmd.ExitError, stderr)
	}
	if stdout != "" {
		t.Fatalf("a failed upload must not print a pending result, got %q", stdout)
	}
	want := `Error: builds wait: timed out resolving build selector after 0s; build upload "upload-78" failed with state FAILED: 90189 (Redundant Binary Upload.`
	if !strings.Contains(stderr, want) {
		t.Fatalf("stderr missing %q, got %q", want, stderr)
	}
	if !strings.Contains(stderr, "recovery: increase the build number (CFBundleVersion), rebuild, and upload again") {
		t.Fatalf("expected recovery guidance, got %q", stderr)
	}
	if strings.Contains(stderr, "resume with:") {
		t.Fatalf("a failed upload must not suggest resuming, got %q", stderr)
	}
}
