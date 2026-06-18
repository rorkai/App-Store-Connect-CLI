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

func TestBuildsNextBuildNumberWithFiltersUsesCanonicalQueryShape(t *testing.T) {
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
			if query.Get("limit") != "1" {
				t.Fatalf("expected limit=1, got %q", query.Get("limit"))
			}
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"builds","id":"build-1","attributes":{"version":"100","uploadedDate":"2026-02-01T00:00:00Z"}}]}`), nil

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
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"buildUploads","id":"upload-1","attributes":{"cfBundleVersion":"101"}}],"links":{"next":""}}`), nil

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

func TestBuildsNextBuildNumberVersionFilterIgnoresNearMatchPreReleaseVersions(t *testing.T) {
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
			if query.Get("filter[version]") != "1.1" {
				t.Fatalf("expected filter[version]=1.1, got %q", query.Get("filter[version]"))
			}
			if query.Get("limit") != "200" {
				t.Fatalf("expected limit=200 for version-only next-build-number lookup, got %q", query.Get("limit"))
			}
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"preReleaseVersions","id":"prv-exact","attributes":{"version":"1.1","platform":"MAC_OS"}},{"type":"preReleaseVersions","id":"prv-near","attributes":{"version":"1.1.0","platform":"IOS"}}],"links":{"next":""}}`), nil

		case req.Method == http.MethodGet && req.URL.Path == "/v1/builds":
			query := req.URL.Query()
			if query.Get("filter[preReleaseVersion]") != "prv-exact" {
				t.Fatalf("expected exact pre-release version match only, got %q", query.Get("filter[preReleaseVersion]"))
			}
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"builds","id":"build-exact","attributes":{"version":"100","uploadedDate":"2026-02-01T00:00:00Z"}}]}`), nil

		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/100000001/buildUploads":
			query := req.URL.Query()
			if query.Get("filter[cfBundleShortVersionString]") != "1.1" {
				t.Fatalf("expected filter[cfBundleShortVersionString]=1.1, got %q", query.Get("filter[cfBundleShortVersionString]"))
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
			if query.Get("filter[version]") != "1.1" {
				t.Fatalf("expected filter[version]=1.1, got %q", query.Get("filter[version]"))
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
			if query.Get("filter[cfBundleShortVersionString]") != "1.1" {
				t.Fatalf("expected filter[cfBundleShortVersionString]=1.1, got %q", query.Get("filter[cfBundleShortVersionString]"))
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
			if query.Get("filter[cfBundleShortVersionString]") != "1.1" {
				t.Fatalf("expected filter[cfBundleShortVersionString]=1.1, got %q", query.Get("filter[cfBundleShortVersionString]"))
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
