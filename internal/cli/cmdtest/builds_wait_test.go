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

func TestBuildsWaitByBuildIDPollsUntilValid(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	requestCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/builds/build-1" {
			t.Fatalf("expected path /v1/builds/build-1, got %s", req.URL.Path)
		}

		state := "PROCESSING"
		if requestCount >= 2 {
			state = "VALID"
		}
		body := `{"data":{"type":"builds","id":"build-1","attributes":{"processingState":"` + state + `","version":"42"}}}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"builds", "wait", "--build-id", "build-1", "--poll-interval", "1ms", "--timeout", "200ms"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if !strings.Contains(stdout, `"id":"build-1"`) {
		t.Fatalf("expected build output, got %q", stdout)
	}
	waitResult := parseBuildsWaitJSON(t, stdout)
	if waitResult.BuildID != "build-1" {
		t.Fatalf("expected buildId=build-1, got %q", waitResult.BuildID)
	}
	if waitResult.BuildNumber != "42" {
		t.Fatalf("expected buildNumber=42, got %q", waitResult.BuildNumber)
	}
	if waitResult.ProcessingState != "VALID" {
		t.Fatalf("expected processingState=VALID, got %q", waitResult.ProcessingState)
	}
	if strings.TrimSpace(waitResult.Elapsed) == "" {
		t.Fatalf("expected non-empty elapsed in output, got %q", waitResult.Elapsed)
	}
	if !strings.Contains(stderr, "Waiting for build build-1... (PROCESSING") {
		t.Fatalf("expected processing progress output, got %q", stderr)
	}
	if !strings.Contains(stderr, "Waiting for build build-1... (VALID") {
		t.Fatalf("expected terminal-state progress output, got %q", stderr)
	}
}

func TestBuildsWaitByAppAndBuildNumberResolvesThenWaits(t *testing.T) {
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
		switch requestCount {
		case 1:
			if req.Method != http.MethodGet {
				t.Fatalf("expected GET, got %s", req.Method)
			}
			if req.URL.Path != "/v1/builds" {
				t.Fatalf("expected path /v1/builds, got %s", req.URL.Path)
			}
			query := req.URL.Query()
			if query.Get("filter[app]") != "123456789" {
				t.Fatalf("expected filter[app]=123456789, got %q", query.Get("filter[app]"))
			}
			if query.Get("filter[version]") != "42" {
				t.Fatalf("expected filter[version]=42, got %q", query.Get("filter[version]"))
			}
			if query.Get("filter[preReleaseVersion.platform]") != "IOS" {
				t.Fatalf("expected filter[preReleaseVersion.platform]=IOS, got %q", query.Get("filter[preReleaseVersion.platform]"))
			}
			if query.Get("filter[processingState]") != "PROCESSING,FAILED,INVALID,VALID" {
				t.Fatalf(
					"expected filter[processingState]=PROCESSING,FAILED,INVALID,VALID, got %q",
					query.Get("filter[processingState]"),
				)
			}
			if query.Get("limit") != "200" {
				t.Fatalf("expected limit=200, got %q", query.Get("limit"))
			}
			body := `{"data":[{"type":"builds","id":"build-42","attributes":{"processingState":"PROCESSING","version":"42"}}]}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 2:
			if req.Method != http.MethodGet {
				t.Fatalf("expected GET, got %s", req.Method)
			}
			if req.URL.Path != "/v1/builds/build-42" {
				t.Fatalf("expected path /v1/builds/build-42, got %s", req.URL.Path)
			}
			body := `{"data":{"type":"builds","id":"build-42","attributes":{"processingState":"VALID","version":"42"}}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		default:
			t.Fatalf("unexpected request count %d", requestCount)
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"builds", "wait",
			"--app", "123456789",
			"--build-number", "42",
			"--platform", "IOS",
			"--poll-interval", "1ms",
			"--timeout", "200ms",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if !strings.Contains(stdout, `"id":"build-42"`) {
		t.Fatalf("expected build output, got %q", stdout)
	}
	waitResult := parseBuildsWaitJSON(t, stdout)
	if waitResult.BuildID != "build-42" {
		t.Fatalf("expected buildId=build-42, got %q", waitResult.BuildID)
	}
	if waitResult.BuildNumber != "42" {
		t.Fatalf("expected buildNumber=42, got %q", waitResult.BuildNumber)
	}
	if waitResult.ProcessingState != "VALID" {
		t.Fatalf("expected processingState=VALID, got %q", waitResult.ProcessingState)
	}
	if !strings.Contains(stderr, "Waiting for build build-42... (VALID") {
		t.Fatalf("expected wait progress output, got %q", stderr)
	}
}

func TestBuildsWaitByAppLatestDiscoversThenWaits(t *testing.T) {
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
		switch requestCount {
		case 1:
			if req.URL.Path != "/v1/builds" {
				t.Fatalf("expected path /v1/builds, got %s", req.URL.Path)
			}
			query := req.URL.Query()
			if query.Get("filter[app]") != "123456789" {
				t.Fatalf("expected filter[app]=123456789, got %q", query.Get("filter[app]"))
			}
			if query.Get("sort") != "-uploadedDate" {
				t.Fatalf("expected sort=-uploadedDate, got %q", query.Get("sort"))
			}
			if query.Get("limit") != "200" {
				t.Fatalf("expected limit=200, got %q", query.Get("limit"))
			}
			body := `{"data":[]}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 2:
			if req.URL.Path != "/v1/builds" {
				t.Fatalf("expected path /v1/builds, got %s", req.URL.Path)
			}
			body := `{"data":[{"type":"builds","id":"build-99","attributes":{"uploadedDate":"2026-03-02T18:01:00Z","processingState":"PROCESSING","version":"99"}}]}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 3:
			if req.URL.Path != "/v1/builds/build-99" {
				t.Fatalf("expected path /v1/builds/build-99, got %s", req.URL.Path)
			}
			body := `{"data":{"type":"builds","id":"build-99","attributes":{"processingState":"VALID","version":"99"}}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		default:
			t.Fatalf("unexpected request count %d", requestCount)
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"builds", "wait",
			"--app", "123456789",
			"--latest",
			"--poll-interval", "1ms",
			"--timeout", "250ms",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	waitResult := parseBuildsWaitJSON(t, stdout)
	if waitResult.BuildID != "build-99" {
		t.Fatalf("expected buildId=build-99, got %q", waitResult.BuildID)
	}
	if waitResult.BuildNumber != "99" {
		t.Fatalf("expected buildNumber=99, got %q", waitResult.BuildNumber)
	}
	if waitResult.ProcessingState != "VALID" {
		t.Fatalf("expected processingState=VALID, got %q", waitResult.ProcessingState)
	}
	if !strings.Contains(stderr, "Waiting for build discovery") {
		t.Fatalf("expected discovery progress output, got %q", stderr)
	}
	if !strings.Contains(stderr, "Waiting for build build-99... (VALID") {
		t.Fatalf("expected wait progress output, got %q", stderr)
	}
}

func TestBuildsWaitByAppWithSinceSkipsOlderMatch(t *testing.T) {
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
		switch requestCount {
		case 1:
			if req.URL.Path != "/v1/preReleaseVersions" {
				t.Fatalf("expected path /v1/preReleaseVersions, got %s", req.URL.Path)
			}
			query := req.URL.Query()
			if query.Get("filter[version]") != "2.4.0" {
				t.Fatalf("expected filter[version]=2.4.0, got %q", query.Get("filter[version]"))
			}
			body := `{"data":[{"type":"preReleaseVersions","id":"prv-24","attributes":{"version":"2.4.0","platform":"IOS"}}]}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 2:
			if req.URL.Path != "/v1/builds" {
				t.Fatalf("expected path /v1/builds, got %s", req.URL.Path)
			}
			query := req.URL.Query()
			if query.Get("filter[preReleaseVersion]") != "prv-24" {
				t.Fatalf("expected filter[preReleaseVersion]=prv-24, got %q", query.Get("filter[preReleaseVersion]"))
			}
			if query.Get("filter[version]") != "2" {
				t.Fatalf("expected filter[version]=2, got %q", query.Get("filter[version]"))
			}
			body := `{"data":[{"type":"builds","id":"build-old","attributes":{"uploadedDate":"2026-03-02T17:59:00Z","processingState":"PROCESSING","version":"2"}}]}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 3:
			if req.URL.Path != "/v1/preReleaseVersions" {
				t.Fatalf("expected path /v1/preReleaseVersions, got %s", req.URL.Path)
			}
			body := `{"data":[{"type":"preReleaseVersions","id":"prv-24","attributes":{"version":"2.4.0","platform":"IOS"}}]}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 4:
			if req.URL.Path != "/v1/builds" {
				t.Fatalf("expected path /v1/builds, got %s", req.URL.Path)
			}
			body := `{"data":[{"type":"builds","id":"build-new","attributes":{"uploadedDate":"2026-03-02T18:01:00Z","processingState":"PROCESSING","version":"2"}}]}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 5:
			if req.URL.Path != "/v1/builds/build-new" {
				t.Fatalf("expected path /v1/builds/build-new, got %s", req.URL.Path)
			}
			body := `{"data":{"type":"builds","id":"build-new","attributes":{"processingState":"VALID","version":"2"}}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		default:
			t.Fatalf("unexpected request count %d", requestCount)
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, _ := captureOutput(t, func() {
		if err := root.Parse([]string{
			"builds", "wait",
			"--app", "123456789",
			"--version", "2.4.0",
			"--build-number", "2",
			"--since", "2026-03-02T18:00:00Z",
			"--poll-interval", "1ms",
			"--timeout", "250ms",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	waitResult := parseBuildsWaitJSON(t, stdout)
	if waitResult.BuildID != "build-new" {
		t.Fatalf("expected buildId=build-new, got %q", waitResult.BuildID)
	}
	if waitResult.BuildNumber != "2" {
		t.Fatalf("expected buildNumber=2, got %q", waitResult.BuildNumber)
	}
}

func TestBuildsWaitByBuildNumberSinceFiltersBeforeUniqueness(t *testing.T) {
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
		switch requestCount {
		case 1:
			if req.URL.Path != "/v1/builds" {
				t.Fatalf("expected path /v1/builds, got %s", req.URL.Path)
			}
			query := req.URL.Query()
			if query.Get("filter[app]") != "123456789" {
				t.Fatalf("expected filter[app]=123456789, got %q", query.Get("filter[app]"))
			}
			if query.Get("filter[version]") != "42" {
				t.Fatalf("expected filter[version]=42, got %q", query.Get("filter[version]"))
			}
			if query.Get("filter[processingState]") != "PROCESSING,FAILED,INVALID,VALID" {
				t.Fatalf("expected wait processing-state filter, got %q", query.Get("filter[processingState]"))
			}
			if query.Get("sort") != "-uploadedDate" {
				t.Fatalf("expected sort=-uploadedDate, got %q", query.Get("sort"))
			}
			if query.Get("limit") != "200" {
				t.Fatalf("expected limit=200, got %q", query.Get("limit"))
			}
			body := `{
				"data":[
					{"type":"builds","id":"build-new","attributes":{"uploadedDate":"2026-03-02T18:01:00Z","processingState":"PROCESSING","version":"42"}},
					{"type":"builds","id":"build-old","attributes":{"uploadedDate":"2026-03-02T17:59:00Z","processingState":"PROCESSING","version":"42"}}
				]
			}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 2:
			if req.URL.Path != "/v1/builds/build-new" {
				t.Fatalf("expected path /v1/builds/build-new, got %s", req.URL.Path)
			}
			body := `{"data":{"type":"builds","id":"build-new","attributes":{"processingState":"VALID","version":"42"}}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		default:
			t.Fatalf("unexpected request count %d", requestCount)
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"builds", "wait",
			"--app", "123456789",
			"--build-number", "42",
			"--since", "2026-03-02T18:00:00Z",
			"--poll-interval", "1ms",
			"--timeout", "200ms",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	waitResult := parseBuildsWaitJSON(t, stdout)
	if waitResult.BuildID != "build-new" {
		t.Fatalf("expected buildId=build-new, got %q", waitResult.BuildID)
	}
	if waitResult.BuildNumber != "42" {
		t.Fatalf("expected buildNumber=42, got %q", waitResult.BuildNumber)
	}
	if waitResult.ProcessingState != "VALID" {
		t.Fatalf("expected processingState=VALID, got %q", waitResult.ProcessingState)
	}
	if !strings.Contains(stderr, deprecatedImplicitIOSBuildNumberPlatformWarning) {
		t.Fatalf("expected implicit IOS deprecation warning, got %q", stderr)
	}
	if !strings.Contains(stderr, "Waiting for build build-new... (VALID") {
		t.Fatalf("expected wait progress output, got %q", stderr)
	}
}

func TestBuildsWaitRejectsVersionOnlySelector(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"builds", "wait", "--app", "123456789", "--version", "2.4.0"}); err != nil {
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
	if !strings.Contains(stderr, "Error: --latest or --build-number is required") {
		t.Fatalf("expected final selector contract error, got %q", stderr)
	}
}

func TestBuildsWaitRejectsSinceOnlySelector(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"builds", "wait", "--app", "123456789", "--since", "2026-03-02T18:00:00Z"}); err != nil {
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
	if !strings.Contains(stderr, "Error: --latest or --build-number is required") {
		t.Fatalf("expected final selector contract error, got %q", stderr)
	}
}

func TestBuildsWaitRejectsSinceWithBuildID(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"builds", "wait", "--build-id", "build-1", "--since", "2026-03-02T18:00:00Z"}); err != nil {
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
	if !strings.Contains(stderr, "Error: --build-id is mutually exclusive with app-scoped selectors") {
		t.Fatalf("expected build-id mutual exclusivity error, got %q", stderr)
	}
	if !strings.Contains(stderr, "--since") {
		t.Fatalf("expected --since to be called out in mutual exclusivity error, got %q", stderr)
	}
}

func TestBuildsWaitByBuildNumberRequiresUniqueMatch(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/builds" {
			t.Fatalf("expected path /v1/builds, got %s", req.URL.Path)
		}

		query := req.URL.Query()
		if query.Get("filter[app]") != "123456789" {
			t.Fatalf("expected filter[app]=123456789, got %q", query.Get("filter[app]"))
		}
		if query.Get("filter[version]") != "42" {
			t.Fatalf("expected filter[version]=42, got %q", query.Get("filter[version]"))
		}
		if query.Get("filter[preReleaseVersion.platform]") != "IOS" {
			t.Fatalf("expected implicit IOS platform filter, got %q", query.Get("filter[preReleaseVersion.platform]"))
		}
		if query.Get("filter[processingState]") != "PROCESSING,FAILED,INVALID,VALID" {
			t.Fatalf("expected wait processing-state filter, got %q", query.Get("filter[processingState]"))
		}
		if query.Get("sort") != "-uploadedDate" {
			t.Fatalf("expected sort=-uploadedDate, got %q", query.Get("sort"))
		}
		if query.Get("limit") != "200" {
			t.Fatalf("expected limit=200 for uniqueness check, got %q", query.Get("limit"))
		}

		body := `{
			"data":[
				{"type":"builds","id":"build-ios","attributes":{"uploadedDate":"2026-03-02T18:01:00Z","processingState":"PROCESSING","version":"42"}},
				{"type":"builds","id":"build-macos","attributes":{"uploadedDate":"2026-03-02T18:00:30Z","processingState":"PROCESSING","version":"42"}}
			]
		}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"builds", "wait",
			"--app", "123456789",
			"--build-number", "42",
			"--poll-interval", "1ms",
			"--timeout", "200ms",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if runErr == nil {
		t.Fatal("expected unique build-number lookup error")
	}
	if !strings.Contains(runErr.Error(), `multiple builds found for app 123456789 with build number "42"`) {
		t.Fatalf("expected ambiguity error, got %v", runErr)
	}
	if !strings.Contains(runErr.Error(), "add --version, or use --build-id") {
		t.Fatalf("expected actionable ambiguity hint, got %v", runErr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout on ambiguity error, got %q", stdout)
	}
	if !strings.Contains(stderr, deprecatedImplicitIOSBuildNumberPlatformWarning) {
		t.Fatalf("expected implicit IOS deprecation warning, got %q", stderr)
	}
}

func TestBuildsWaitByBuildNumberDiscoveryWarnsOnlyOnce(t *testing.T) {
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
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/builds" {
			t.Fatalf("expected path /v1/builds, got %s", req.URL.Path)
		}

		query := req.URL.Query()
		if query.Get("filter[app]") != "123456789" {
			t.Fatalf("expected filter[app]=123456789, got %q", query.Get("filter[app]"))
		}
		if query.Get("filter[version]") != "42" {
			t.Fatalf("expected filter[version]=42, got %q", query.Get("filter[version]"))
		}
		if query.Get("filter[preReleaseVersion.platform]") != "IOS" {
			t.Fatalf("expected implicit IOS platform filter, got %q", query.Get("filter[preReleaseVersion.platform]"))
		}

		body := `{"data":[]}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"builds", "wait",
			"--app", "123456789",
			"--build-number", "42",
			"--poll-interval", "1ms",
			"--timeout", "50ms",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if runErr == nil {
		t.Fatal("expected timeout error")
	}
	if !strings.Contains(runErr.Error(), "timed out resolving build selector") {
		t.Fatalf("expected selector timeout error, got %v", runErr)
	}
	if requestCount < 2 {
		t.Fatalf("expected multiple discovery polls, got %d", requestCount)
	}
	if got := strings.Count(stderr, deprecatedImplicitIOSBuildNumberPlatformWarning); got != 1 {
		t.Fatalf("expected one implicit IOS warning, got %d in %q", got, stderr)
	}
	if !strings.Contains(stderr, "Waiting for build discovery") {
		t.Fatalf("expected discovery progress output, got %q", stderr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout on timeout, got %q", stdout)
	}
}

func TestBuildsWaitByAppDiscoveryTimeoutReturnsError(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Path != "/v1/builds" {
			t.Fatalf("expected path /v1/builds, got %s", req.URL.Path)
		}
		body := `{"data":[]}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, _ := captureOutput(t, func() {
		if err := root.Parse([]string{
			"builds", "wait",
			"--app", "123456789",
			"--latest",
			"--poll-interval", "1ms",
			"--timeout", "10ms",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if runErr == nil {
		t.Fatal("expected timeout error")
	}
	if !strings.Contains(runErr.Error(), "timed out resolving build selector") {
		t.Fatalf("expected timeout resolving selector error, got %v", runErr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout on timeout, got %q", stdout)
	}
}

func TestBuildsWaitFailOnInvalidReturnsError(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/builds/build-1" {
			t.Fatalf("expected path /v1/builds/build-1, got %s", req.URL.Path)
		}
		body := `{"data":{"type":"builds","id":"build-1","attributes":{"processingState":"INVALID","version":"42"}}}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"builds", "wait", "--build-id", "build-1", "--fail-on-invalid", "--poll-interval", "1ms", "--timeout", "100ms"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if runErr == nil {
		t.Fatal("expected INVALID-state failure error")
	}
	if errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("expected runtime error, got usage error: %v", runErr)
	}
	if !strings.Contains(runErr.Error(), "build processing failed with state INVALID") {
		t.Fatalf("expected INVALID failure, got %v", runErr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout on failure, got %q", stdout)
	}
	if !strings.Contains(stderr, "Waiting for build build-1... (INVALID") {
		t.Fatalf("expected progress output on stderr, got %q", stderr)
	}
}

func TestBuildsWaitFailedStateReturnsError(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/builds/build-1" {
			t.Fatalf("expected path /v1/builds/build-1, got %s", req.URL.Path)
		}
		body := `{"data":{"type":"builds","id":"build-1","attributes":{"processingState":"FAILED","version":"42"}}}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"builds", "wait", "--build-id", "build-1", "--poll-interval", "1ms", "--timeout", "100ms"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if runErr == nil {
		t.Fatal("expected FAILED-state failure error")
	}
	if errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("expected runtime error, got usage error: %v", runErr)
	}
	if !strings.Contains(runErr.Error(), "build processing failed with state FAILED") {
		t.Fatalf("expected FAILED failure, got %v", runErr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout on failure, got %q", stdout)
	}
	if !strings.Contains(stderr, "Waiting for build build-1... (FAILED") {
		t.Fatalf("expected progress output on stderr, got %q", stderr)
	}
}

type buildsWaitJSONResult struct {
	Data struct {
		ID         string `json:"id"`
		Attributes struct {
			Version         string `json:"version"`
			ProcessingState string `json:"processingState"`
		} `json:"attributes"`
	} `json:"data"`
	BuildID         string `json:"buildId"`
	BuildNumber     string `json:"buildNumber"`
	ProcessingState string `json:"processingState"`
	Elapsed         string `json:"elapsed"`
}

func parseBuildsWaitJSON(t *testing.T, stdout string) buildsWaitJSONResult {
	t.Helper()

	var parsed buildsWaitJSONResult
	if err := json.Unmarshal([]byte(stdout), &parsed); err != nil {
		t.Fatalf("failed to parse builds wait output JSON %q: %v", stdout, err)
	}
	return parsed
}
