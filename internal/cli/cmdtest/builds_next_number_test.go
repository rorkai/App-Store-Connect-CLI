package cmdtest

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func TestBuildsNextBuildNumberUsesUploadsAndBuilds(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/builds":
			query := req.URL.Query()
			if query.Get("filter[app]") != "100000001" {
				t.Fatalf("expected filter[app]=100000001, got %q", query.Get("filter[app]"))
			}
			if query.Get("sort") != "-uploadedDate" {
				t.Fatalf("expected sort=-uploadedDate, got %q", query.Get("sort"))
			}
			if query.Get("limit") != "200" {
				t.Fatalf("expected limit=200, got %q", query.Get("limit"))
			}
			body := `{
				"data":[{"type":"builds","id":"build-1","attributes":{"version":"100","uploadedDate":"2026-02-01T00:00:00Z"}}]
			}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil

		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/100000001/buildUploads":
			query := req.URL.Query()
			if query.Get("filter[state]") != "AWAITING_UPLOAD,PROCESSING,COMPLETE" {
				t.Fatalf("expected filter[state]=AWAITING_UPLOAD,PROCESSING,COMPLETE, got %q", query.Get("filter[state]"))
			}
			body := `{
				"data":[{"type":"buildUploads","id":"upload-1","attributes":{"cfBundleVersion":"101"}}],
				"links":{"next":""}
			}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil

		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"builds", "next-build-number", "--app", "100000001"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var out struct {
		LatestProcessedBuildNumber *string  `json:"latestProcessedBuildNumber"`
		LatestUploadBuildNumber    *string  `json:"latestUploadBuildNumber"`
		LatestObservedBuildNumber  *string  `json:"latestObservedBuildNumber"`
		NextBuildNumber            string   `json:"nextBuildNumber"`
		SourcesConsidered          []string `json:"sourcesConsidered"`
	}
	if err := json.Unmarshal([]byte(stdout), &out); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout: %s", err, stdout)
	}
	if out.LatestProcessedBuildNumber == nil || *out.LatestProcessedBuildNumber != "100" {
		t.Fatalf("expected latestProcessedBuildNumber=100, got %v", out.LatestProcessedBuildNumber)
	}
	if out.LatestUploadBuildNumber == nil || *out.LatestUploadBuildNumber != "101" {
		t.Fatalf("expected latestUploadBuildNumber=101, got %v", out.LatestUploadBuildNumber)
	}
	if out.LatestObservedBuildNumber == nil || *out.LatestObservedBuildNumber != "101" {
		t.Fatalf("expected latestObservedBuildNumber=101, got %v", out.LatestObservedBuildNumber)
	}
	if out.NextBuildNumber != "102" {
		t.Fatalf("expected nextBuildNumber=102, got %q", out.NextBuildNumber)
	}
	if len(out.SourcesConsidered) != 2 {
		t.Fatalf("expected two sources considered, got %v", out.SourcesConsidered)
	}
}

func TestBuildsNextBuildNumberRejectsInvalidInitialBuildNumber(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_PROFILE", "")
	t.Setenv("ASC_KEY_ID", "")
	t.Setenv("ASC_ISSUER_ID", "")
	t.Setenv("ASC_PRIVATE_KEY_PATH", "")
	t.Setenv("ASC_PRIVATE_KEY", "")
	t.Setenv("ASC_PRIVATE_KEY_B64", "")

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"builds", "next-build-number", "--app", "100000001", "--initial-build-number", "0"}); err != nil {
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
	if !strings.Contains(stderr, "Error: --initial-build-number must be >= 1") {
		t.Fatalf("expected initial build number validation error, got %q", stderr)
	}
	if strings.Contains(stderr, "missing authentication") {
		t.Fatalf("expected validation before auth resolution, got %q", stderr)
	}
}

func TestBuildsNextBuildNumberExplainsUnavailableUploadHistory(t *testing.T) {
	tests := []struct {
		name         string
		status       int
		responseBody string
		paginate     bool
		wantCause    error
		wantExitCode int
	}{
		{
			name:         "initial request forbidden",
			status:       http.StatusForbidden,
			responseBody: `{"errors":[{"status":"403","code":"FORBIDDEN","title":"Forbidden"}]}`,
			wantCause:    asc.ErrForbidden,
			wantExitCode: cmd.ExitAuth,
		},
		{
			name:         "initial request not found",
			status:       http.StatusNotFound,
			responseBody: `{"errors":[{"status":"404","code":"NOT_FOUND","title":"Not Found"}]}`,
			wantCause:    asc.ErrNotFound,
			wantExitCode: cmd.ExitNotFound,
		},
		{
			name:         "next page forbidden",
			status:       http.StatusForbidden,
			responseBody: `{"errors":[{"status":"403","code":"FORBIDDEN","title":"Forbidden"}]}`,
			paginate:     true,
			wantCause:    asc.ErrForbidden,
			wantExitCode: cmd.ExitAuth,
		},
		{
			name:         "next page not found",
			status:       http.StatusNotFound,
			responseBody: `{"errors":[{"status":"404","code":"NOT_FOUND","title":"Not Found"}]}`,
			paginate:     true,
			wantCause:    asc.ErrNotFound,
			wantExitCode: cmd.ExitNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
			// Build Upload API 404s are retried; exhaust the budget quickly so
			// the terminal exit code and guidance are asserted after retries.
			t.Setenv("ASC_MAX_RETRIES", "1")
			t.Setenv("ASC_BASE_DELAY", "1ms")
			t.Setenv("ASC_MAX_DELAY", "1ms")

			originalTransport := http.DefaultTransport
			t.Cleanup(func() { http.DefaultTransport = originalTransport })
			appChecks := 0
			http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
				switch {
				case req.Method == http.MethodGet && req.URL.Path == "/v1/builds":
					return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"builds","id":"build-1","attributes":{"version":"100","uploadedDate":"2026-02-01T00:00:00Z"}}]}`), nil
				case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/100000001/buildUploads" && tt.paginate && req.URL.Query().Get("cursor") == "":
					return jsonHTTPResponse(http.StatusOK, `{"data":[],"links":{"next":"https://api.appstoreconnect.apple.com/v1/apps/100000001/buildUploads?cursor=next-page"}}`), nil
				case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/100000001/buildUploads":
					return jsonHTTPResponse(tt.status, tt.responseBody), nil
				case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/100000001" && tt.status == http.StatusNotFound:
					appChecks++
					return jsonHTTPResponse(http.StatusNotFound, tt.responseBody), nil
				default:
					t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
					return nil, nil
				}
			})

			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			var runErr error
			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse([]string{"builds", "next-build-number", "--app", "100000001"}); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				runErr = root.Run(context.Background())
			})

			if runErr == nil {
				t.Fatal("expected unavailable upload history to fail")
			}
			if !errors.Is(runErr, tt.wantCause) {
				t.Fatalf("expected %v in error chain, got %v", tt.wantCause, runErr)
			}
			if got := cmd.ExitCodeFromError(runErr); got != tt.wantExitCode {
				t.Fatalf("exit code = %d, want %d", got, tt.wantExitCode)
			}
			for _, want := range []string{
				`build upload history is unavailable for app "100000001"`,
				"refusing to guess because an in-flight upload may already use the next number",
				`asc builds uploads list --app "100000001" --paginate`,
			} {
				if !strings.Contains(runErr.Error(), want) {
					t.Fatalf("expected error to contain %q, got %v", want, runErr)
				}
			}
			if stdout != "" || stderr != "" {
				t.Fatalf("expected no partial output, got stdout=%q stderr=%q", stdout, stderr)
			}
			wantAppChecks := 0
			if tt.status == http.StatusNotFound {
				wantAppChecks = 1
			}
			if appChecks != wantAppChecks {
				t.Fatalf("app existence checks = %d, want %d", appChecks, wantAppChecks)
			}
		})
	}
}

func TestBuildsNextBuildNumberWithFiltersUsesCanonicalQueryShape(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	chronologicalBuildRequests := 0
	maximumBuildRequests := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/preReleaseVersions":
			query := req.URL.Query()
			if query.Get("filter[app]") != "100000001" {
				t.Fatalf("expected filter[app]=100000001, got %q", query.Get("filter[app]"))
			}
			if query.Get("filter[version]") != "1.2.3" {
				t.Fatalf("expected filter[version]=1.2.3, got %q", query.Get("filter[version]"))
			}
			if query.Get("filter[platform]") != "IOS" {
				t.Fatalf("expected filter[platform]=IOS, got %q", query.Get("filter[platform]"))
			}
			if query.Get("limit") != "200" {
				t.Fatalf("expected limit=200, got %q", query.Get("limit"))
			}
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"preReleaseVersions","id":"prv-1","attributes":{"version":"1.2.3","platform":"IOS"}}]}`), nil

		case req.Method == http.MethodGet && req.URL.Path == "/v1/builds":
			query := req.URL.Query()
			if query.Get("filter[app]") != "100000001" {
				t.Fatalf("expected filter[app]=100000001, got %q", query.Get("filter[app]"))
			}
			if query.Get("filter[preReleaseVersion]") != "prv-1" {
				t.Fatalf("expected filter[preReleaseVersion]=prv-1, got %q", query.Get("filter[preReleaseVersion]"))
			}
			if query.Get("filter[processingState]") != "VALID" {
				t.Fatalf("expected filter[processingState]=VALID, got %q", query.Get("filter[processingState]"))
			}
			if query.Get("filter[expired]") != "false" {
				t.Fatalf("expected filter[expired]=false, got %q", query.Get("filter[expired]"))
			}
			if query.Get("sort") != "-uploadedDate" {
				t.Fatalf("expected sort=-uploadedDate, got %q", query.Get("sort"))
			}
			switch query.Get("limit") {
			case "1":
				chronologicalBuildRequests++
				return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"builds","id":"build-new-50","attributes":{"version":"50","uploadedDate":"2026-02-02T00:00:00Z"}}]}`), nil
			case "200":
				maximumBuildRequests++
				return jsonHTTPResponse(http.StatusOK, `{"data":[
					{"type":"builds","id":"build-new-50","attributes":{"version":"50","uploadedDate":"2026-02-02T00:00:00Z"}},
					{"type":"builds","id":"build-old-100","attributes":{"version":"100","uploadedDate":"2026-02-01T00:00:00Z"}}
				]}`), nil
			default:
				t.Fatalf("expected limit=1 or limit=200, got %q", query.Get("limit"))
				return nil, nil
			}

		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/100000001/buildUploads":
			query := req.URL.Query()
			if query.Get("filter[state]") != "AWAITING_UPLOAD,PROCESSING,COMPLETE" {
				t.Fatalf("expected filter[state]=AWAITING_UPLOAD,PROCESSING,COMPLETE, got %q", query.Get("filter[state]"))
			}
			if query.Get("filter[cfBundleShortVersionString]") != "1.2.3" {
				t.Fatalf("expected filter[cfBundleShortVersionString]=1.2.3, got %q", query.Get("filter[cfBundleShortVersionString]"))
			}
			if query.Get("filter[platform]") != "IOS" {
				t.Fatalf("expected filter[platform]=IOS, got %q", query.Get("filter[platform]"))
			}
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"buildUploads","id":"upload-1","attributes":{"cfBundleVersion":"90"}}],"links":{"next":""}}`), nil

		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"builds", "next-build-number",
			"--app", "100000001",
			"--version", "1.2.3",
			"--platform", "ios",
			"--processing-state", "valid",
			"--exclude-expired",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var out struct {
		LatestProcessedBuildNumber *string `json:"latestProcessedBuildNumber"`
		LatestUploadBuildNumber    *string `json:"latestUploadBuildNumber"`
		LatestObservedBuildNumber  *string `json:"latestObservedBuildNumber"`
		NextBuildNumber            string  `json:"nextBuildNumber"`
	}
	if err := json.Unmarshal([]byte(stdout), &out); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout: %s", err, stdout)
	}
	if out.LatestProcessedBuildNumber == nil || *out.LatestProcessedBuildNumber != "50" {
		t.Fatalf("expected latestProcessedBuildNumber=50, got %v", out.LatestProcessedBuildNumber)
	}
	if out.LatestUploadBuildNumber == nil || *out.LatestUploadBuildNumber != "90" {
		t.Fatalf("expected latestUploadBuildNumber=90, got %v", out.LatestUploadBuildNumber)
	}
	if out.LatestObservedBuildNumber == nil || *out.LatestObservedBuildNumber != "100" {
		t.Fatalf("expected latestObservedBuildNumber=100, got %v", out.LatestObservedBuildNumber)
	}
	if out.NextBuildNumber != "101" {
		t.Fatalf("expected nextBuildNumber=101, got %q", out.NextBuildNumber)
	}
	if chronologicalBuildRequests != 1 || maximumBuildRequests != 1 {
		t.Fatalf("build requests = chronological:%d maximum:%d, want 1 each", chronologicalBuildRequests, maximumBuildRequests)
	}
}

func TestBuildsNextBuildNumberScansEveryEquivalentVersionUpload(t *testing.T) {
	setupAuth(t)
	shared.ResetEquivalentVersionNotesForTest()
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	preReleaseRequests := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/preReleaseVersions":
			preReleaseRequests++
			wantVersion := "76.54.0"
			body := `{"data":[],"links":{"next":""}}`
			if preReleaseRequests == 2 {
				wantVersion = "76.54"
				body = `{"data":[{"type":"preReleaseVersions","id":"prv-equivalent","attributes":{"version":"76.54","platform":"IOS"}}],"links":{"next":""}}`
			}
			if got := req.URL.Query().Get("filter[version]"); got != wantVersion {
				t.Fatalf("filter[version] = %q, want %q", got, wantVersion)
			}
			return jsonHTTPResponse(http.StatusOK, body), nil

		case req.Method == http.MethodGet && req.URL.Path == "/v1/builds":
			if got := req.URL.Query().Get("filter[preReleaseVersion]"); got != "prv-equivalent" {
				t.Fatalf("filter[preReleaseVersion] = %q, want prv-equivalent", got)
			}
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"builds","id":"build-10","attributes":{"version":"10","uploadedDate":"2026-02-01T00:00:00Z"}}]}`), nil

		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/100000001/buildUploads":
			if got := req.URL.Query().Get("filter[cfBundleShortVersionString]"); got != "76.54.0,76.54" {
				t.Fatalf("filter[cfBundleShortVersionString] = %q, want %q", got, "76.54.0,76.54")
			}
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"buildUploads","id":"upload-20","attributes":{"cfBundleShortVersionString":"76.54","cfBundleVersion":"20"}},{"type":"buildUploads","id":"upload-30","attributes":{"cfBundleShortVersionString":"76.54.0","cfBundleVersion":"30"}}],"links":{"next":""}}`), nil

		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"builds", "next-build-number",
			"--app", "100000001",
			"--version", "76.54.0",
			"--platform", "ios",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if preReleaseRequests != 2 {
		t.Fatalf("pre-release requests = %d, want 2", preReleaseRequests)
	}
	var out struct {
		LatestUploadBuildNumber *string `json:"latestUploadBuildNumber"`
		NextBuildNumber         string  `json:"nextBuildNumber"`
	}
	if err := json.Unmarshal([]byte(stdout), &out); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout: %s", err, stdout)
	}
	if out.LatestUploadBuildNumber == nil || *out.LatestUploadBuildNumber != "30" {
		t.Fatalf("latestUploadBuildNumber = %v, want 30", out.LatestUploadBuildNumber)
	}
	if out.NextBuildNumber != "31" {
		t.Fatalf("nextBuildNumber = %q, want 31", out.NextBuildNumber)
	}
	if !strings.Contains(stderr, `matched version "76.54" for requested "76.54.0"`) {
		t.Fatalf("expected equivalent-version note, got %q", stderr)
	}
}

func TestBuildsNextBuildNumberSkipsNonPositiveBuildUploadNumber(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/preReleaseVersions":
			query := req.URL.Query()
			if query.Get("filter[app]") != "100000001" {
				t.Fatalf("expected filter[app]=100000001, got %q", query.Get("filter[app]"))
			}
			if query.Get("filter[version]") != "1.2.3" {
				t.Fatalf("expected filter[version]=1.2.3, got %q", query.Get("filter[version]"))
			}
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"preReleaseVersions","id":"prv-1","attributes":{"version":"1.2.3","platform":"IOS"}}]}`), nil

		case req.Method == http.MethodGet && req.URL.Path == "/v1/builds":
			query := req.URL.Query()
			if query.Get("filter[expired]") != "false" {
				t.Fatalf("expected filter[expired]=false, got %q", query.Get("filter[expired]"))
			}
			return jsonHTTPResponse(http.StatusOK, `{"data":[]}`), nil

		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/100000001/buildUploads":
			query := req.URL.Query()
			if query.Get("filter[cfBundleShortVersionString]") != "1.2.3" {
				t.Fatalf("expected filter[cfBundleShortVersionString]=1.2.3, got %q", query.Get("filter[cfBundleShortVersionString]"))
			}
			return jsonHTTPResponse(http.StatusOK, `{
				"data":[
					{"type":"buildUploads","id":"expired-upload","attributes":{"cfBundleVersion":"0"}},
					{"type":"buildUploads","id":"spaced-zero-upload","attributes":{"cfBundleVersion":"0 .1"}}
				],
				"links":{"next":""}
			}`), nil

		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"builds", "next-build-number",
			"--app", "100000001",
			"--version", "1.2.3",
			"--exclude-expired",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var out struct {
		LatestUploadBuildNumber *string  `json:"latestUploadBuildNumber"`
		NextBuildNumber         string   `json:"nextBuildNumber"`
		SourcesConsidered       []string `json:"sourcesConsidered"`
	}
	if err := json.Unmarshal([]byte(stdout), &out); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout: %s", err, stdout)
	}
	if out.LatestUploadBuildNumber != nil {
		t.Fatalf("expected invalid upload number to be ignored, got %v", *out.LatestUploadBuildNumber)
	}
	if out.NextBuildNumber != "1" {
		t.Fatalf("expected nextBuildNumber=1, got %q", out.NextBuildNumber)
	}
	if len(out.SourcesConsidered) != 0 {
		t.Fatalf("expected no sources considered, got %v", out.SourcesConsidered)
	}
}

func TestBuildsNextBuildNumberVersionFilterCollectsEquivalentPlatforms(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	var buildFilters []string
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/preReleaseVersions":
			query := req.URL.Query()
			if query.Get("filter[app]") != "100000001" {
				t.Fatalf("expected filter[app]=100000001, got %q", query.Get("filter[app]"))
			}
			if query.Get("filter[version]") != "1.1,1.1.0" {
				t.Fatalf("expected filter[version]=1.1,1.1.0, got %q", query.Get("filter[version]"))
			}
			if query.Get("limit") != "200" {
				t.Fatalf("expected limit=200 for version-only next-build-number lookup, got %q", query.Get("limit"))
			}
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"preReleaseVersions","id":"prv-exact","attributes":{"version":"1.1","platform":"MAC_OS"}},{"type":"preReleaseVersions","id":"prv-near","attributes":{"version":"1.1.0","platform":"IOS"}}],"links":{"next":""}}`), nil

		case req.Method == http.MethodGet && req.URL.Path == "/v1/builds":
			query := req.URL.Query()
			filter := query.Get("filter[preReleaseVersion]")
			buildFilters = append(buildFilters, filter)
			if filter == "prv-near" {
				return jsonHTTPResponse(http.StatusOK, `{"data":[]}`), nil
			}
			if filter != "prv-exact" && filter != "prv-exact,prv-near" {
				t.Fatalf("unexpected pre-release version filter %q", filter)
			}
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"builds","id":"build-exact","attributes":{"version":"100","uploadedDate":"2026-02-01T00:00:00Z"}}]}`), nil

		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/100000001/buildUploads":
			query := req.URL.Query()
			if query.Get("filter[cfBundleShortVersionString]") != "1.1,1.1.0" {
				t.Fatalf("expected filter[cfBundleShortVersionString]=1.1,1.1.0, got %q", query.Get("filter[cfBundleShortVersionString]"))
			}
			if query.Get("filter[platform]") != "" {
				t.Fatalf("did not expect platform filter for version-only next-build-number lookup, got %q", query.Get("filter[platform]"))
			}
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"buildUploads","id":"upload-1","attributes":{"cfBundleVersion":"101"}}],"links":{"next":""}}`), nil

		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"builds", "next-build-number", "--app", "100000001", "--version", "1.1"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var out struct {
		LatestProcessedBuildNumber *string `json:"latestProcessedBuildNumber"`
		LatestUploadBuildNumber    *string `json:"latestUploadBuildNumber"`
		NextBuildNumber            string  `json:"nextBuildNumber"`
	}
	if err := json.Unmarshal([]byte(stdout), &out); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout: %s", err, stdout)
	}
	if out.LatestProcessedBuildNumber == nil || *out.LatestProcessedBuildNumber != "100" {
		t.Fatalf("expected latestProcessedBuildNumber=100, got %v", out.LatestProcessedBuildNumber)
	}
	if out.LatestUploadBuildNumber == nil || *out.LatestUploadBuildNumber != "101" {
		t.Fatalf("expected latestUploadBuildNumber=101, got %v", out.LatestUploadBuildNumber)
	}
	if out.NextBuildNumber != "102" {
		t.Fatalf("expected nextBuildNumber=102, got %q", out.NextBuildNumber)
	}
	if len(buildFilters) != 3 || buildFilters[0] != "prv-exact" || buildFilters[1] != "prv-near" || buildFilters[2] != "prv-exact,prv-near" {
		t.Fatalf("expected both equivalent platform trains in latest and history lookups, got %v", buildFilters)
	}
}

func TestBuildsNextBuildNumberVersionFilterKeepsServerMatchedPreReleaseVersionsWithoutAttributes(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/preReleaseVersions":
			query := req.URL.Query()
			if query.Get("filter[app]") != "100000001" {
				t.Fatalf("expected filter[app]=100000001, got %q", query.Get("filter[app]"))
			}
			if query.Get("filter[version]") != "1.1,1.1.0" {
				t.Fatalf("expected filter[version]=1.1,1.1.0, got %q", query.Get("filter[version]"))
			}
			if query.Get("limit") != "200" {
				t.Fatalf("expected limit=200 for version-only next-build-number lookup, got %q", query.Get("limit"))
			}
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"preReleaseVersions","id":"prv-server","attributes":{}}],"links":{"next":""}}`), nil

		case req.Method == http.MethodGet && req.URL.Path == "/v1/builds":
			query := req.URL.Query()
			if query.Get("filter[preReleaseVersion]") != "prv-server" {
				t.Fatalf("expected server-matched pre-release version to be preserved, got %q", query.Get("filter[preReleaseVersion]"))
			}
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"builds","id":"build-server","attributes":{"version":"100","uploadedDate":"2026-02-01T00:00:00Z"}}]}`), nil

		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/100000001/buildUploads":
			query := req.URL.Query()
			if query.Get("filter[cfBundleShortVersionString]") != "1.1,1.1.0" {
				t.Fatalf("expected filter[cfBundleShortVersionString]=1.1,1.1.0, got %q", query.Get("filter[cfBundleShortVersionString]"))
			}
			if query.Get("filter[platform]") != "" {
				t.Fatalf("did not expect platform filter for version-only next-build-number lookup, got %q", query.Get("filter[platform]"))
			}
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"buildUploads","id":"upload-1","attributes":{"cfBundleVersion":"101"}}],"links":{"next":""}}`), nil

		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"builds", "next-build-number", "--app", "100000001", "--version", "1.1"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var out struct {
		LatestProcessedBuildNumber *string `json:"latestProcessedBuildNumber"`
		LatestUploadBuildNumber    *string `json:"latestUploadBuildNumber"`
		NextBuildNumber            string  `json:"nextBuildNumber"`
	}
	if err := json.Unmarshal([]byte(stdout), &out); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout: %s", err, stdout)
	}
	if out.LatestProcessedBuildNumber == nil || *out.LatestProcessedBuildNumber != "100" {
		t.Fatalf("expected latestProcessedBuildNumber=100, got %v", out.LatestProcessedBuildNumber)
	}
	if out.LatestUploadBuildNumber == nil || *out.LatestUploadBuildNumber != "101" {
		t.Fatalf("expected latestUploadBuildNumber=101, got %v", out.LatestUploadBuildNumber)
	}
	if out.NextBuildNumber != "102" {
		t.Fatalf("expected nextBuildNumber=102, got %q", out.NextBuildNumber)
	}
}

func TestBuildsNextBuildNumberVersionAndPlatformPaginatesPastNearMatches(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	const nextURL = "https://api.appstoreconnect.apple.com/v1/preReleaseVersions?cursor=page-2"

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.URL.String() == nextURL:
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"preReleaseVersions","id":"prv-exact","attributes":{"version":"1.1","platform":"IOS"}}],"links":{"next":""}}`), nil

		case req.Method == http.MethodGet && req.URL.Path == "/v1/preReleaseVersions":
			query := req.URL.Query()
			if query.Get("filter[app]") != "100000001" {
				t.Fatalf("expected filter[app]=100000001, got %q", query.Get("filter[app]"))
			}
			if query.Get("filter[version]") != "1.1" {
				t.Fatalf("expected filter[version]=1.1, got %q", query.Get("filter[version]"))
			}
			if query.Get("filter[platform]") != "IOS" {
				t.Fatalf("expected filter[platform]=IOS, got %q", query.Get("filter[platform]"))
			}
			if query.Get("limit") != "200" {
				t.Fatalf("expected limit=200 for version+platform next-build-number lookup, got %q", query.Get("limit"))
			}
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"preReleaseVersions","id":"prv-near","attributes":{"version":"1.1.0","platform":"IOS"}}],"links":{"next":"`+nextURL+`"}}`), nil

		case req.Method == http.MethodGet && req.URL.Path == "/v1/builds":
			query := req.URL.Query()
			if query.Get("filter[preReleaseVersion]") != "prv-exact" {
				t.Fatalf("expected exact pre-release version match after pagination, got %q", query.Get("filter[preReleaseVersion]"))
			}
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"builds","id":"build-exact","attributes":{"version":"100","uploadedDate":"2026-02-01T00:00:00Z"}}]}`), nil

		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/100000001/buildUploads":
			query := req.URL.Query()
			if query.Get("filter[cfBundleShortVersionString]") != "1.1,1.1.0" {
				t.Fatalf("expected filter[cfBundleShortVersionString]=1.1,1.1.0, got %q", query.Get("filter[cfBundleShortVersionString]"))
			}
			if query.Get("filter[platform]") != "IOS" {
				t.Fatalf("expected filter[platform]=IOS, got %q", query.Get("filter[platform]"))
			}
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"buildUploads","id":"upload-1","attributes":{"cfBundleVersion":"101"}}],"links":{"next":""}}`), nil

		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"builds", "next-build-number", "--app", "100000001", "--version", "1.1", "--platform", "IOS"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var out struct {
		LatestProcessedBuildNumber *string `json:"latestProcessedBuildNumber"`
		LatestUploadBuildNumber    *string `json:"latestUploadBuildNumber"`
		NextBuildNumber            string  `json:"nextBuildNumber"`
	}
	if err := json.Unmarshal([]byte(stdout), &out); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout: %s", err, stdout)
	}
	if out.LatestProcessedBuildNumber == nil || *out.LatestProcessedBuildNumber != "100" {
		t.Fatalf("expected latestProcessedBuildNumber=100, got %v", out.LatestProcessedBuildNumber)
	}
	if out.LatestUploadBuildNumber == nil || *out.LatestUploadBuildNumber != "101" {
		t.Fatalf("expected latestUploadBuildNumber=101, got %v", out.LatestUploadBuildNumber)
	}
	if out.NextBuildNumber != "102" {
		t.Fatalf("expected nextBuildNumber=102, got %q", out.NextBuildNumber)
	}
}

func TestBuildsHelpShowsNextBuildNumberAndHidesLatestAlias(t *testing.T) {
	usage := usageForCommand(t, "builds")
	if !strings.Contains(usage, "\n  next-build-number") {
		t.Fatalf("expected builds help to list next-build-number, got %q", usage)
	}
	if strings.Contains(usage, "\n  latest\t") || strings.Contains(usage, "\n  latest ") {
		t.Fatalf("expected deprecated latest alias to stay hidden from builds help, got %q", usage)
	}
}

func TestBuildsNextBuildNumberRetriesBuildUploadsNotFoundForExistingApp(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_MAX_RETRIES", "3")
	t.Setenv("ASC_BASE_DELAY", "1ms")
	t.Setenv("ASC_MAX_DELAY", "1ms")

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	uploadAttempts := 0
	appChecks := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/builds":
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"builds","id":"build-1","attributes":{"version":"100","uploadedDate":"2026-02-01T00:00:00Z"}}]}`), nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/100000001/buildUploads":
			uploadAttempts++
			if uploadAttempts <= 2 {
				return jsonHTTPResponse(http.StatusNotFound, `{"errors":[{"status":"404","code":"NOT_FOUND","title":"The specified resource does not exist","detail":"There is no resource of type 'apps' with id '100000001'"}]}`), nil
			}
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"buildUploads","id":"upload-1","attributes":{"cfBundleVersion":"101"}}],"links":{"next":""}}`), nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/100000001":
			appChecks++
			return jsonHTTPResponse(http.StatusOK, `{"data":{"type":"apps","id":"100000001"}}`), nil
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"builds", "next-build-number", "--app", "100000001"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if uploadAttempts != 3 {
		t.Fatalf("expected 3 buildUploads attempts (404, 404, 200), got %d", uploadAttempts)
	}
	if appChecks != 1 {
		t.Fatalf("expected one app existence check, got %d", appChecks)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	var out struct {
		LatestUploadBuildNumber *string `json:"latestUploadBuildNumber"`
		NextBuildNumber         string  `json:"nextBuildNumber"`
	}
	if err := json.Unmarshal([]byte(stdout), &out); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout: %s", err, stdout)
	}
	if out.LatestUploadBuildNumber == nil || *out.LatestUploadBuildNumber != "101" {
		t.Fatalf("expected latestUploadBuildNumber=101, got %v", out.LatestUploadBuildNumber)
	}
	if out.NextBuildNumber != "102" {
		t.Fatalf("expected nextBuildNumber=102, got %q", out.NextBuildNumber)
	}
}
