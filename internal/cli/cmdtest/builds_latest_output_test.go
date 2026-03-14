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

func TestBuildsLatestSelectsNewestAcrossPlatformPreReleaseVersions(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	const nextPreReleaseURL = "https://api.appstoreconnect.apple.com/v1/preReleaseVersions?page=2"

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/preReleaseVersions" && req.URL.Query().Get("page") == "":
			query := req.URL.Query()
			if query.Get("filter[app]") != "100000001" {
				t.Fatalf("expected filter[app]=100000001, got %q", query.Get("filter[app]"))
			}
			if query.Get("filter[platform]") != "IOS" {
				t.Fatalf("expected filter[platform]=IOS, got %q", query.Get("filter[platform]"))
			}
			if query.Get("limit") != "200" {
				t.Fatalf("expected limit=200, got %q", query.Get("limit"))
			}
			body := `{
				"data":[{"type":"preReleaseVersions","id":"prv-old"}],
				"links":{"next":"` + nextPreReleaseURL + `"}
			}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil

		case req.Method == http.MethodGet && req.URL.String() == nextPreReleaseURL:
			body := `{
				"data":[{"type":"preReleaseVersions","id":"prv-new"}],
				"links":{"next":""}
			}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil

		case req.Method == http.MethodGet && req.URL.Path == "/v1/builds":
			query := req.URL.Query()
			if query.Get("filter[app]") != "100000001" {
				t.Fatalf("expected filter[app]=100000001, got %q", query.Get("filter[app]"))
			}
			if query.Get("sort") != "-uploadedDate" {
				t.Fatalf("expected sort=-uploadedDate, got %q", query.Get("sort"))
			}
			if query.Get("limit") != "1" {
				t.Fatalf("expected limit=1, got %q", query.Get("limit"))
			}

			switch query.Get("filter[preReleaseVersion]") {
			case "prv-old":
				body := `{
					"data":[{"type":"builds","id":"build-old","attributes":{"uploadedDate":"2026-01-01T00:00:00Z"}}]
				}`
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(body)),
					Header:     http.Header{"Content-Type": []string{"application/json"}},
				}, nil
			case "prv-new":
				body := `{
					"data":[{"type":"builds","id":"build-new","attributes":{"uploadedDate":"2026-02-01T00:00:00Z"}}]
				}`
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(body)),
					Header:     http.Header{"Content-Type": []string{"application/json"}},
				}, nil
			default:
				t.Fatalf("unexpected filter[preReleaseVersion]=%q", query.Get("filter[preReleaseVersion]"))
				return nil, nil
			}

		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"builds", "latest", "--app", "100000001", "--platform", "ios"}); err != nil {
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
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &out); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout: %s", err, stdout)
	}
	if out.Data.ID != "build-new" {
		t.Fatalf("expected latest build id build-new, got %q", out.Data.ID)
	}
}

func TestBuildsLatestSelectsNewestAcrossPlatformPreReleaseVersionsWithTimezoneOffsets(t *testing.T) {
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
			if query.Get("filter[platform]") != "IOS" {
				t.Fatalf("expected filter[platform]=IOS, got %q", query.Get("filter[platform]"))
			}
			body := `{
				"data":[
					{"type":"preReleaseVersions","id":"prv-z-older"},
					{"type":"preReleaseVersions","id":"prv-offset-newer"}
				],
				"links":{"next":""}
			}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil

		case req.Method == http.MethodGet && req.URL.Path == "/v1/builds":
			query := req.URL.Query()
			if query.Get("filter[app]") != "100000001" {
				t.Fatalf("expected filter[app]=100000001, got %q", query.Get("filter[app]"))
			}
			if query.Get("sort") != "-uploadedDate" {
				t.Fatalf("expected sort=-uploadedDate, got %q", query.Get("sort"))
			}
			if query.Get("limit") != "1" {
				t.Fatalf("expected limit=1, got %q", query.Get("limit"))
			}

			switch query.Get("filter[preReleaseVersion]") {
			case "prv-z-older":
				// Absolute time: 2026-02-02T07:00:00Z (older by 30 minutes)
				body := `{
					"data":[{"type":"builds","id":"build-z-older","attributes":{"uploadedDate":"2026-02-02T07:00:00Z"}}]
				}`
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(body)),
					Header:     http.Header{"Content-Type": []string{"application/json"}},
				}, nil
			case "prv-offset-newer":
				// Absolute time: 2026-02-02T07:30:00Z (newer), but lexical string order is lower.
				body := `{
					"data":[{"type":"builds","id":"build-offset-newer","attributes":{"uploadedDate":"2026-02-01T23:30:00-08:00"}}]
				}`
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(body)),
					Header:     http.Header{"Content-Type": []string{"application/json"}},
				}, nil
			default:
				t.Fatalf("unexpected filter[preReleaseVersion]=%q", query.Get("filter[preReleaseVersion]"))
				return nil, nil
			}

		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"builds", "latest", "--app", "100000001", "--platform", "ios"}); err != nil {
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
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &out); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout: %s", err, stdout)
	}
	if out.Data.ID != "build-offset-newer" {
		t.Fatalf("expected latest build id build-offset-newer, got %q", out.Data.ID)
	}
}

func TestBuildsLatestReturnsPreReleaseLookupFailure(t *testing.T) {
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
		if req.URL.Path != "/v1/preReleaseVersions" {
			t.Fatalf("expected pre-release versions path, got %s", req.URL.Path)
		}
		body := `{"errors":[{"status":"500","title":"Server Error","detail":"pre-release lookup failed"}]}`
		return &http.Response{
			StatusCode: http.StatusInternalServerError,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, _ := captureOutput(t, func() {
		if err := root.Parse([]string{"builds", "latest", "--app", "100000001", "--platform", "ios"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if runErr == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(runErr.Error(), "builds latest: failed to lookup pre-release versions") {
		t.Fatalf("expected pre-release lookup error, got %v", runErr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
}

func TestBuildsLatestRejectsRepeatedPreReleasePaginationURL(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	const repeatedNextURL = "https://api.appstoreconnect.apple.com/v1/preReleaseVersions?cursor=AQ"

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	requestCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		switch requestCount {
		case 1:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/preReleaseVersions" {
				t.Fatalf("unexpected first request: %s %s", req.Method, req.URL.String())
			}
			body := `{
				"data":[{"type":"preReleaseVersions","id":"prv-1"}],
				"links":{"next":"` + repeatedNextURL + `"}
			}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 2:
			if req.Method != http.MethodGet || req.URL.String() != repeatedNextURL {
				t.Fatalf("unexpected second request: %s %s", req.Method, req.URL.String())
			}
			body := `{
				"data":[{"type":"preReleaseVersions","id":"prv-2"}],
				"links":{"next":"` + repeatedNextURL + `"}
			}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		default:
			t.Fatalf("unexpected extra request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, _ := captureOutput(t, func() {
		if err := root.Parse([]string{"builds", "latest", "--app", "100000001", "--platform", "IOS"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if runErr == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(runErr.Error(), "detected repeated pagination URL") {
		t.Fatalf("expected repeated pagination URL error, got %v", runErr)
	}
	if !strings.Contains(runErr.Error(), "failed to paginate pre-release versions") {
		t.Fatalf("expected pre-release pagination context, got %v", runErr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
}

func TestBuildsLatestOutputErrors(t *testing.T) {
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
		if req.URL.Path != "/v1/builds" {
			t.Fatalf("expected path /v1/builds, got %s", req.URL.Path)
		}
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
			"data":[{"type":"builds","id":"build-1","attributes":{"uploadedDate":"2026-02-01T00:00:00Z"}}]
		}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "unsupported output",
			args:    []string{"builds", "latest", "--app", "100000001", "--output", "yaml"},
			wantErr: "unsupported format: yaml",
		},
		{
			name:    "pretty with table",
			args:    []string{"builds", "latest", "--app", "100000001", "--output", "table", "--pretty"},
			wantErr: "--pretty is only valid with JSON output",
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
				t.Fatalf("expected help error, got %v", runErr)
			}
			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, test.wantErr) {
				t.Fatalf("expected stderr %q, got %q", test.wantErr, stderr)
			}
		})
	}
}

func TestBuildsLatestTableOutput(t *testing.T) {
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
		if req.URL.Path != "/v1/builds" {
			t.Fatalf("expected path /v1/builds, got %s", req.URL.Path)
		}
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
			"data":[{"type":"builds","id":"build-table","attributes":{"uploadedDate":"2026-03-01T00:00:00Z"}}]
		}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"builds", "latest", "--app", "100000001", "--output", "table"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, "2026-03-01T00:00:00Z") {
		t.Fatalf("expected table output to contain uploaded timestamp, got %q", stdout)
	}
}

func TestBuildsLatestNextUsesUploadsAndBuilds(t *testing.T) {
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
			if query.Get("limit") != "1" {
				t.Fatalf("expected limit=1, got %q", query.Get("limit"))
			}
			body := `{
				"data":[{"type":"preReleaseVersions","id":"prv-1"}],
				"links":{"next":""}
			}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil

		case req.Method == http.MethodGet && req.URL.Path == "/v1/builds":
			query := req.URL.Query()
			if query.Get("filter[app]") != "100000001" {
				t.Fatalf("expected filter[app]=100000001, got %q", query.Get("filter[app]"))
			}
			if query.Get("sort") != "-uploadedDate" {
				t.Fatalf("expected sort=-uploadedDate, got %q", query.Get("sort"))
			}
			if query.Get("limit") != "1" {
				t.Fatalf("expected limit=1, got %q", query.Get("limit"))
			}
			if query.Get("filter[preReleaseVersion]") != "prv-1" {
				t.Fatalf("expected filter[preReleaseVersion]=prv-1, got %q", query.Get("filter[preReleaseVersion]"))
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
			if query.Get("filter[cfBundleShortVersionString]") != "1.2.3" {
				t.Fatalf("expected filter[cfBundleShortVersionString]=1.2.3, got %q", query.Get("filter[cfBundleShortVersionString]"))
			}
			if query.Get("filter[platform]") != "IOS" {
				t.Fatalf("expected filter[platform]=IOS, got %q", query.Get("filter[platform]"))
			}
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
		if err := root.Parse([]string{"builds", "latest", "--app", "100000001", "--version", "1.2.3", "--platform", "IOS", "--next"}); err != nil {
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

func TestBuildsLatestNextProcessedOnly(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/preReleaseVersions":
			body := `{
				"data":[{"type":"preReleaseVersions","id":"prv-1"}],
				"links":{"next":""}
			}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil

		case req.Method == http.MethodGet && req.URL.Path == "/v1/builds":
			body := `{
				"data":[{"type":"builds","id":"build-1","attributes":{"version":"55","uploadedDate":"2026-02-01T00:00:00Z"}}]
			}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil

		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/100000001/buildUploads":
			body := `{
				"data":[],
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
		if err := root.Parse([]string{"builds", "latest", "--app", "100000001", "--version", "1.2.3", "--platform", "IOS", "--next"}); err != nil {
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
	if out.LatestProcessedBuildNumber == nil || *out.LatestProcessedBuildNumber != "55" {
		t.Fatalf("expected latestProcessedBuildNumber=55, got %v", out.LatestProcessedBuildNumber)
	}
	if out.LatestUploadBuildNumber != nil {
		t.Fatalf("expected latestUploadBuildNumber to be nil, got %v", out.LatestUploadBuildNumber)
	}
	if out.LatestObservedBuildNumber == nil || *out.LatestObservedBuildNumber != "55" {
		t.Fatalf("expected latestObservedBuildNumber=55, got %v", out.LatestObservedBuildNumber)
	}
	if out.NextBuildNumber != "56" {
		t.Fatalf("expected nextBuildNumber=56, got %q", out.NextBuildNumber)
	}
}

func TestBuildsLatestNextUploadsOnly(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/builds":
			body := `{
				"data":[],
				"links":{"next":""}
			}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil

		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/100000001/buildUploads":
			body := `{
				"data":[{"type":"buildUploads","id":"upload-1","attributes":{"cfBundleVersion":"25"}}],
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
		if err := root.Parse([]string{"builds", "latest", "--app", "100000001", "--next"}); err != nil {
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
	if out.LatestProcessedBuildNumber != nil {
		t.Fatalf("expected latestProcessedBuildNumber to be nil, got %v", out.LatestProcessedBuildNumber)
	}
	if out.LatestUploadBuildNumber == nil || *out.LatestUploadBuildNumber != "25" {
		t.Fatalf("expected latestUploadBuildNumber=25, got %v", out.LatestUploadBuildNumber)
	}
	if out.LatestObservedBuildNumber == nil || *out.LatestObservedBuildNumber != "25" {
		t.Fatalf("expected latestObservedBuildNumber=25, got %v", out.LatestObservedBuildNumber)
	}
	if out.NextBuildNumber != "26" {
		t.Fatalf("expected nextBuildNumber=26, got %q", out.NextBuildNumber)
	}
}

func TestBuildsLatestNextNoHistoryUsesInitial(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/preReleaseVersions":
			body := `{
				"data":[],
				"links":{"next":""}
			}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil

		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/100000001/buildUploads":
			body := `{
				"data":[],
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
		if err := root.Parse([]string{"builds", "latest", "--app", "100000001", "--version", "1.2.3", "--platform", "IOS", "--next", "--initial-build-number", "7"}); err != nil {
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
	if out.LatestProcessedBuildNumber != nil {
		t.Fatalf("expected latestProcessedBuildNumber to be nil, got %v", out.LatestProcessedBuildNumber)
	}
	if out.LatestUploadBuildNumber != nil {
		t.Fatalf("expected latestUploadBuildNumber to be nil, got %v", out.LatestUploadBuildNumber)
	}
	if out.LatestObservedBuildNumber != nil {
		t.Fatalf("expected latestObservedBuildNumber to be nil, got %v", out.LatestObservedBuildNumber)
	}
	if out.NextBuildNumber != "7" {
		t.Fatalf("expected nextBuildNumber=7, got %q", out.NextBuildNumber)
	}
}

func TestBuildsLatestNextSupportsDotSeparatedBuildNumbers(t *testing.T) {
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
				"data":[{"type":"builds","id":"build-dot","attributes":{"version":"1.2.3","uploadedDate":"2026-02-01T00:00:00Z"}}]
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
				"data":[{"type":"buildUploads","id":"upload-dot","attributes":{"cfBundleVersion":"1.2.4"}}],
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
		if err := root.Parse([]string{"builds", "latest", "--app", "100000001", "--next"}); err != nil {
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
	if out.LatestProcessedBuildNumber == nil || *out.LatestProcessedBuildNumber != "1.2.3" {
		t.Fatalf("expected latestProcessedBuildNumber=1.2.3, got %v", out.LatestProcessedBuildNumber)
	}
	if out.LatestUploadBuildNumber == nil || *out.LatestUploadBuildNumber != "1.2.4" {
		t.Fatalf("expected latestUploadBuildNumber=1.2.4, got %v", out.LatestUploadBuildNumber)
	}
	if out.LatestObservedBuildNumber == nil || *out.LatestObservedBuildNumber != "1.2.4" {
		t.Fatalf("expected latestObservedBuildNumber=1.2.4, got %v", out.LatestObservedBuildNumber)
	}
	if out.NextBuildNumber != "1.2.5" {
		t.Fatalf("expected nextBuildNumber=1.2.5, got %q", out.NextBuildNumber)
	}
}

func TestBuildsLatestExcludeExpiredFiltersOutExpiredBuilds(t *testing.T) {
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
		if req.URL.Path != "/v1/builds" {
			t.Fatalf("expected path /v1/builds, got %s", req.URL.Path)
		}
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
		if query.Get("filter[expired]") != "false" {
			t.Fatalf("expected filter[expired]=false, got %q", query.Get("filter[expired]"))
		}
		body := `{
			"data":[{"type":"builds","id":"build-non-expired","attributes":{"version":"100","uploadedDate":"2026-02-01T00:00:00Z","expired":false}}]
		}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"builds", "latest", "--app", "100000001", "--exclude-expired"}); err != nil {
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
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &out); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout: %s", err, stdout)
	}
	if out.Data.ID != "build-non-expired" {
		t.Fatalf("expected latest build id build-non-expired, got %q", out.Data.ID)
	}
}

func TestBuildsLatestNextExcludeExpiredHonorsFilter(t *testing.T) {
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
			if query.Get("filter[expired]") != "false" {
				t.Fatalf("expected filter[expired]=false, got %q", query.Get("filter[expired]"))
			}
			body := `{
				"data":[{"type":"builds","id":"build-1","attributes":{"version":"100","uploadedDate":"2026-02-01T00:00:00Z","expired":false}}]
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
				"data":[{"type":"buildUploads","id":"upload-1","attributes":{"cfBundleVersion":"150"}}],
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
		if err := root.Parse([]string{"builds", "latest", "--app", "100000001", "--next", "--exclude-expired"}); err != nil {
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
	if out.LatestProcessedBuildNumber == nil || *out.LatestProcessedBuildNumber != "100" {
		t.Fatalf("expected latestProcessedBuildNumber=100, got %v", out.LatestProcessedBuildNumber)
	}
	if out.LatestUploadBuildNumber == nil || *out.LatestUploadBuildNumber != "150" {
		t.Fatalf("expected latestUploadBuildNumber=150, got %v", out.LatestUploadBuildNumber)
	}
	if out.LatestObservedBuildNumber == nil || *out.LatestObservedBuildNumber != "150" {
		t.Fatalf("expected latestObservedBuildNumber=150, got %v", out.LatestObservedBuildNumber)
	}
	if out.NextBuildNumber != "151" {
		t.Fatalf("expected nextBuildNumber=151, got %q", out.NextBuildNumber)
	}
}

func TestBuildsLatestNotExpiredAliasHonorsExpiredFilter(t *testing.T) {
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
		if req.URL.Path != "/v1/builds" {
			t.Fatalf("expected path /v1/builds, got %s", req.URL.Path)
		}
		query := req.URL.Query()
		if query.Get("filter[app]") != "100000001" {
			t.Fatalf("expected filter[app]=100000001, got %q", query.Get("filter[app]"))
		}
		if query.Get("filter[expired]") != "false" {
			t.Fatalf("expected filter[expired]=false, got %q", query.Get("filter[expired]"))
		}
		body := `{"data":[{"type":"builds","id":"build-alias","attributes":{"uploadedDate":"2026-03-01T00:00:00Z","expired":false}}]}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"builds", "latest", "--app", "100000001", "--not-expired"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"id":"build-alias"`) {
		t.Fatalf("expected build-alias in output, got %q", stdout)
	}
}

func TestBuildsLatestNotExpiredAliasSinglePreReleasePath(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	callCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		callCount++
		switch callCount {
		case 1:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/preReleaseVersions" {
				t.Fatalf("unexpected first request: %s %s", req.Method, req.URL.String())
			}
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
			body := `{"data":[{"type":"preReleaseVersions","id":"prv-1"}],"links":{"next":""}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 2:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/builds" {
				t.Fatalf("unexpected second request: %s %s", req.Method, req.URL.String())
			}
			query := req.URL.Query()
			if query.Get("filter[preReleaseVersion]") != "prv-1" {
				t.Fatalf("expected filter[preReleaseVersion]=prv-1, got %q", query.Get("filter[preReleaseVersion]"))
			}
			if query.Get("filter[expired]") != "false" {
				t.Fatalf("expected filter[expired]=false, got %q", query.Get("filter[expired]"))
			}
			body := `{"data":[{"type":"builds","id":"build-single","attributes":{"uploadedDate":"2026-03-02T00:00:00Z","expired":false}}]}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		default:
			t.Fatalf("unexpected request count %d", callCount)
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"builds", "latest", "--app", "100000001", "--version", "1.2.3", "--platform", "IOS", "--not-expired"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"id":"build-single"`) {
		t.Fatalf("expected build-single in output, got %q", stdout)
	}
}

func TestBuildsLatestNotExpiredAliasMultiPreReleasePath(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	callCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		callCount++
		switch callCount {
		case 1:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/preReleaseVersions" {
				t.Fatalf("unexpected first request: %s %s", req.Method, req.URL.String())
			}
			query := req.URL.Query()
			if query.Get("filter[platform]") != "IOS" {
				t.Fatalf("expected platform filter IOS, got %q", query.Get("filter[platform]"))
			}
			body := `{"data":[{"type":"preReleaseVersions","id":"prv-1"},{"type":"preReleaseVersions","id":"prv-2"}],"links":{"next":""}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 2:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/builds" {
				t.Fatalf("unexpected second request: %s %s", req.Method, req.URL.String())
			}
			query := req.URL.Query()
			if query.Get("filter[preReleaseVersion]") != "prv-1" {
				t.Fatalf("expected prv-1 query, got %q", query.Get("filter[preReleaseVersion]"))
			}
			if query.Get("filter[expired]") != "false" {
				t.Fatalf("expected filter[expired]=false, got %q", query.Get("filter[expired]"))
			}
			body := `{"data":[{"type":"builds","id":"build-old","attributes":{"uploadedDate":"2026-03-01T00:00:00Z","expired":false}}]}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 3:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/builds" {
				t.Fatalf("unexpected third request: %s %s", req.Method, req.URL.String())
			}
			query := req.URL.Query()
			if query.Get("filter[preReleaseVersion]") != "prv-2" {
				t.Fatalf("expected prv-2 query, got %q", query.Get("filter[preReleaseVersion]"))
			}
			if query.Get("filter[expired]") != "false" {
				t.Fatalf("expected filter[expired]=false, got %q", query.Get("filter[expired]"))
			}
			body := `{"data":[{"type":"builds","id":"build-new","attributes":{"uploadedDate":"2026-03-05T00:00:00Z","expired":false}}]}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		default:
			t.Fatalf("unexpected request count %d", callCount)
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"builds", "latest", "--app", "100000001", "--platform", "IOS", "--not-expired"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"id":"build-new"`) {
		t.Fatalf("expected newest build from multi-pre-release path, got %q", stdout)
	}
}

func TestBuildsLatestSelectsNewestUploadedDateWhenBuildNumberResets(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	const nextBuildsURL = "https://api.appstoreconnect.apple.com/v1/builds?page=2"

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/builds" && req.URL.Query().Get("page") == "":
			query := req.URL.Query()
			if query.Get("filter[app]") != "100000001" {
				t.Fatalf("expected filter[app]=100000001, got %q", query.Get("filter[app]"))
			}
			if query.Get("sort") != "-uploadedDate" {
				t.Fatalf("expected sort=-uploadedDate, got %q", query.Get("sort"))
			}
			body := `{
				"data":[{"type":"builds","id":"build-45-older","attributes":{"version":"45","uploadedDate":"2025-11-19T00:00:00Z"}}],
				"links":{"next":"` + nextBuildsURL + `"}
			}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil

		case req.Method == http.MethodGet && req.URL.String() == nextBuildsURL:
			body := `{
				"data":[{"type":"builds","id":"build-1-newest","attributes":{"version":"1","uploadedDate":"2026-02-01T00:00:00Z"}}],
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
		if err := root.Parse([]string{"builds", "latest", "--app", "100000001"}); err != nil {
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
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &out); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout: %s", err, stdout)
	}
	if out.Data.ID != "build-1-newest" {
		t.Fatalf("expected latest build id build-1-newest, got %q", out.Data.ID)
	}
}

func TestBuildsLatestIncludesExpiredByDefaultWhenItIsNewest(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	const nextBuildsURL = "https://api.appstoreconnect.apple.com/v1/builds?page=2"

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/builds" && req.URL.Query().Get("page") == "":
			query := req.URL.Query()
			if query.Get("filter[app]") != "100000001" {
				t.Fatalf("expected filter[app]=100000001, got %q", query.Get("filter[app]"))
			}
			if query.Get("sort") != "-uploadedDate" {
				t.Fatalf("expected sort=-uploadedDate, got %q", query.Get("sort"))
			}
			if query.Get("filter[expired]") != "" {
				t.Fatalf("did not expect filter[expired], got %q", query.Get("filter[expired]"))
			}
			body := `{
				"data":[{"type":"builds","id":"build-non-expired-older","attributes":{"version":"100","uploadedDate":"2026-01-10T00:00:00Z","expired":false}}],
				"links":{"next":"` + nextBuildsURL + `"}
			}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil

		case req.Method == http.MethodGet && req.URL.String() == nextBuildsURL:
			body := `{
				"data":[{"type":"builds","id":"build-expired-newest","attributes":{"version":"101","uploadedDate":"2026-01-20T00:00:00Z","expired":true}}],
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
		if err := root.Parse([]string{"builds", "latest", "--app", "100000001"}); err != nil {
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
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &out); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout: %s", err, stdout)
	}
	if out.Data.ID != "build-expired-newest" {
		t.Fatalf("expected latest build id build-expired-newest, got %q", out.Data.ID)
	}
}

func TestBuildsLatestExcludeExpiredSelectsNewestNonExpiredAcrossPages(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	const nextBuildsURL = "https://api.appstoreconnect.apple.com/v1/builds?page=2"

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/builds" && req.URL.Query().Get("page") == "":
			query := req.URL.Query()
			if query.Get("filter[app]") != "100000001" {
				t.Fatalf("expected filter[app]=100000001, got %q", query.Get("filter[app]"))
			}
			if query.Get("sort") != "-uploadedDate" {
				t.Fatalf("expected sort=-uploadedDate, got %q", query.Get("sort"))
			}
			if query.Get("filter[expired]") != "false" {
				t.Fatalf("expected filter[expired]=false, got %q", query.Get("filter[expired]"))
			}
			body := `{
				"data":[{"type":"builds","id":"build-non-expired-older","attributes":{"version":"100","uploadedDate":"2026-01-10T00:00:00Z","expired":false}}],
				"links":{"next":"` + nextBuildsURL + `"}
			}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil

		case req.Method == http.MethodGet && req.URL.String() == nextBuildsURL:
			body := `{
				"data":[{"type":"builds","id":"build-non-expired-newest","attributes":{"version":"101","uploadedDate":"2026-01-20T00:00:00Z","expired":false}}],
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
		if err := root.Parse([]string{"builds", "latest", "--app", "100000001", "--exclude-expired"}); err != nil {
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
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &out); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout: %s", err, stdout)
	}
	if out.Data.ID != "build-non-expired-newest" {
		t.Fatalf("expected latest build id build-non-expired-newest, got %q", out.Data.ID)
	}
}

func TestBuildsLatestDoesNotExhaustivelyPaginateWhenOrderingIsStable(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	const secondBuildsURL = "https://api.appstoreconnect.apple.com/v1/builds?page=2"
	const thirdBuildsURL = "https://api.appstoreconnect.apple.com/v1/builds?page=3"

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	requestCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/builds" && req.URL.Query().Get("page") == "":
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
				"data":[{"type":"builds","id":"build-newest","attributes":{"version":"12","uploadedDate":"2026-03-01T00:00:00Z"}}],
				"links":{"next":"` + secondBuildsURL + `"}
			}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil

		case req.Method == http.MethodGet && req.URL.String() == secondBuildsURL:
			body := `{
				"data":[{"type":"builds","id":"build-older","attributes":{"version":"11","uploadedDate":"2026-02-01T00:00:00Z"}}],
				"links":{"next":"` + thirdBuildsURL + `"}
			}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil

		case req.Method == http.MethodGet && req.URL.String() == thirdBuildsURL:
			t.Fatalf("unexpected third page fetch when ordering is stable")
			return nil, nil

		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"builds", "latest", "--app", "100000001"}); err != nil {
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
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
		Links struct {
			Next string `json:"next"`
		} `json:"links"`
	}
	if err := json.Unmarshal([]byte(stdout), &out); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout: %s", err, stdout)
	}
	if out.Data.ID != "build-newest" {
		t.Fatalf("expected latest build id build-newest, got %q", out.Data.ID)
	}
	if out.Links.Next != secondBuildsURL {
		t.Fatalf("expected links.next=%q, got %q", secondBuildsURL, out.Links.Next)
	}
	if requestCount != 2 {
		t.Fatalf("expected exactly 2 build page requests, got %d", requestCount)
	}
}

func TestBuildsLatestDoesNotTreatIDOnlyTieAsOrderingAnomaly(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	const secondBuildsURL = "https://api.appstoreconnect.apple.com/v1/builds?page=2"
	const thirdBuildsURL = "https://api.appstoreconnect.apple.com/v1/builds?page=3"

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	requestCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/builds" && req.URL.Query().Get("page") == "":
			body := `{
				"data":[{"type":"builds","id":"build-a","attributes":{"version":"12","uploadedDate":"2026-03-01T00:00:00Z"}}],
				"links":{"next":"` + secondBuildsURL + `"}
			}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil

		case req.Method == http.MethodGet && req.URL.String() == secondBuildsURL:
			// Same uploadedDate, higher ID; deterministic tie-break may switch choice,
			// but this must not be treated as a strictly newer upload anomaly.
			body := `{
				"data":[{"type":"builds","id":"build-z","attributes":{"version":"12","uploadedDate":"2026-03-01T00:00:00Z"}}],
				"links":{"next":"` + thirdBuildsURL + `"}
			}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil

		case req.Method == http.MethodGet && req.URL.String() == thirdBuildsURL:
			t.Fatalf("unexpected third page fetch for ID-only timestamp tie")
			return nil, nil

		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"builds", "latest", "--app", "100000001"}); err != nil {
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
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &out); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout: %s", err, stdout)
	}
	if out.Data.ID == "" {
		t.Fatalf("expected non-empty latest build id, got %q", out.Data.ID)
	}
	if requestCount != 2 {
		t.Fatalf("expected exactly 2 build page requests, got %d", requestCount)
	}
}

func TestBuildsLatestReturnsFirstPageBestWhenProbePageFails(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	const secondBuildsURL = "https://api.appstoreconnect.apple.com/v1/builds?page=2"

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	requestCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/builds" && req.URL.Query().Get("page") == "":
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
				"data":[{"type":"builds","id":"build-first-page-best","attributes":{"version":"12","uploadedDate":"2026-03-01T00:00:00Z"}}],
				"links":{"next":"` + secondBuildsURL + `"}
			}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil

		case req.Method == http.MethodGet && req.URL.String() == secondBuildsURL:
			body := `{"errors":[{"status":"500","title":"Server Error","detail":"temporary failure"}]}`
			return &http.Response{
				StatusCode: http.StatusInternalServerError,
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
		if err := root.Parse([]string{"builds", "latest", "--app", "100000001"}); err != nil {
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
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &out); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout: %s", err, stdout)
	}
	if out.Data.ID != "build-first-page-best" {
		t.Fatalf("expected latest build id build-first-page-best, got %q", out.Data.ID)
	}
	if requestCount != 2 {
		t.Fatalf("expected exactly 2 build page requests, got %d", requestCount)
	}
}

func TestBuildsLatestReturnsErrorWhenProbePageTimesOut(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	const secondBuildsURL = "https://api.appstoreconnect.apple.com/v1/builds?page=2"

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	requestCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/builds" && req.URL.Query().Get("page") == "":
			body := `{
				"data":[{"type":"builds","id":"build-first-page-best","attributes":{"version":"12","uploadedDate":"2026-03-01T00:00:00Z"}}],
				"links":{"next":"` + secondBuildsURL + `"}
			}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil

		case req.Method == http.MethodGet && req.URL.String() == secondBuildsURL:
			return nil, context.DeadlineExceeded

		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, _ := captureOutput(t, func() {
		if err := root.Parse([]string{"builds", "latest", "--app", "100000001", "--next"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if runErr == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(runErr.Error(), "builds latest: failed to paginate builds: page 2") {
		t.Fatalf("expected pagination timeout error, got %v", runErr)
	}
	if !errors.Is(runErr, context.DeadlineExceeded) {
		t.Fatalf("expected context deadline exceeded cause, got %v", runErr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if requestCount != 2 {
		t.Fatalf("expected exactly 2 build page requests, got %d", requestCount)
	}
}

func TestBuildsLatestReturnsErrorWhenProbePageIsCanceled(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	const secondBuildsURL = "https://api.appstoreconnect.apple.com/v1/builds?page=2"

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	requestCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/builds" && req.URL.Query().Get("page") == "":
			body := `{
				"data":[{"type":"builds","id":"build-first-page-best","attributes":{"version":"12","uploadedDate":"2026-03-01T00:00:00Z"}}],
				"links":{"next":"` + secondBuildsURL + `"}
			}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil

		case req.Method == http.MethodGet && req.URL.String() == secondBuildsURL:
			return nil, context.Canceled

		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, _ := captureOutput(t, func() {
		if err := root.Parse([]string{"builds", "latest", "--app", "100000001"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if runErr == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(runErr.Error(), "builds latest: failed to paginate builds: page 2") {
		t.Fatalf("expected pagination canceled error, got %v", runErr)
	}
	if !errors.Is(runErr, context.Canceled) {
		t.Fatalf("expected context canceled cause, got %v", runErr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if requestCount != 2 {
		t.Fatalf("expected exactly 2 build page requests, got %d", requestCount)
	}
}

func TestBuildsLatestReturnsBestFetchedCandidateWhenLaterProbeFails(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	const secondBuildsURL = "https://api.appstoreconnect.apple.com/v1/builds?page=2"
	const thirdBuildsURL = "https://api.appstoreconnect.apple.com/v1/builds?page=3"

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	requestCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/builds" && req.URL.Query().Get("page") == "":
			body := `{
				"data":[{"type":"builds","id":"build-page1-older","attributes":{"version":"10","uploadedDate":"2026-01-01T00:00:00Z"}}],
				"links":{"next":"` + secondBuildsURL + `"}
			}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil

		case req.Method == http.MethodGet && req.URL.String() == secondBuildsURL:
			body := `{
				"data":[{"type":"builds","id":"build-page2-newer","attributes":{"version":"11","uploadedDate":"2026-02-01T00:00:00Z"}}],
				"links":{"next":"` + thirdBuildsURL + `"}
			}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil

		case req.Method == http.MethodGet && req.URL.String() == thirdBuildsURL:
			body := `{"errors":[{"status":"500","title":"Server Error","detail":"temporary failure"}]}`
			return &http.Response{
				StatusCode: http.StatusInternalServerError,
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
		if err := root.Parse([]string{"builds", "latest", "--app", "100000001"}); err != nil {
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
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &out); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout: %s", err, stdout)
	}
	if out.Data.ID != "build-page2-newer" {
		t.Fatalf("expected latest build id build-page2-newer, got %q", out.Data.ID)
	}
	if requestCount != 3 {
		t.Fatalf("expected exactly 3 build page requests, got %d", requestCount)
	}
}

func TestBuildsLatestKeepsScanningAfterAnomalyUntilPaginationExhausted(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	const secondBuildsURL = "https://api.appstoreconnect.apple.com/v1/builds?page=2"
	const thirdBuildsURL = "https://api.appstoreconnect.apple.com/v1/builds?page=3"
	const fourthBuildsURL = "https://api.appstoreconnect.apple.com/v1/builds?page=4"

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	requestCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/builds" && req.URL.Query().Get("page") == "":
			body := `{
				"data":[{"type":"builds","id":"build-page1-older","attributes":{"version":"10","uploadedDate":"2026-01-01T00:00:00Z"}}],
				"links":{"next":"` + secondBuildsURL + `"}
			}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil

		case req.Method == http.MethodGet && req.URL.String() == secondBuildsURL:
			// Anomaly: page 2 contains a newer build than page 1.
			body := `{
				"data":[{"type":"builds","id":"build-page2-newer","attributes":{"version":"11","uploadedDate":"2026-02-01T00:00:00Z"}}],
				"links":{"next":"` + thirdBuildsURL + `"}
			}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil

		case req.Method == http.MethodGet && req.URL.String() == thirdBuildsURL:
			// Non-monotonic ordering: this page is older, but pagination continues.
			body := `{
				"data":[{"type":"builds","id":"build-page3-older","attributes":{"version":"10","uploadedDate":"2026-01-15T00:00:00Z"}}],
				"links":{"next":"` + fourthBuildsURL + `"}
			}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil

		case req.Method == http.MethodGet && req.URL.String() == fourthBuildsURL:
			body := `{
				"data":[{"type":"builds","id":"build-page4-newest","attributes":{"version":"12","uploadedDate":"2026-03-01T00:00:00Z"}}],
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
		if err := root.Parse([]string{"builds", "latest", "--app", "100000001"}); err != nil {
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
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &out); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout: %s", err, stdout)
	}
	if out.Data.ID != "build-page4-newest" {
		t.Fatalf("expected latest build id build-page4-newest, got %q", out.Data.ID)
	}
	if requestCount != 4 {
		t.Fatalf("expected exactly 4 build page requests, got %d", requestCount)
	}
}

func TestBuildsLatestFailsOnRepeatedProbePaginationURLAfterAnomaly(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	const repeatedBuildsURL = "https://api.appstoreconnect.apple.com/v1/builds?page=2"

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	requestCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/builds" && req.URL.Query().Get("page") == "":
			body := `{
				"data":[{"type":"builds","id":"build-page1-older","attributes":{"version":"10","uploadedDate":"2026-01-01T00:00:00Z"}}],
				"links":{"next":"` + repeatedBuildsURL + `"}
			}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil

		case req.Method == http.MethodGet && req.URL.String() == repeatedBuildsURL:
			// Anomaly: page 2 is newer than page 1, but links.next repeats.
			body := `{
				"data":[{"type":"builds","id":"build-page2-newer","attributes":{"version":"11","uploadedDate":"2026-02-01T00:00:00Z"}}],
				"links":{"next":"` + repeatedBuildsURL + `"}
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

	var runErr error
	stdout, _ := captureOutput(t, func() {
		if err := root.Parse([]string{"builds", "latest", "--app", "100000001"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if runErr == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(runErr.Error(), "failed to paginate builds") {
		t.Fatalf("expected builds pagination context, got %v", runErr)
	}
	if !strings.Contains(runErr.Error(), "detected repeated pagination URL") {
		t.Fatalf("expected repeated pagination URL error, got %v", runErr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if requestCount != 2 {
		t.Fatalf("expected exactly 2 build page requests, got %d", requestCount)
	}
}

func TestBuildsLatestFailsOnRepeatedProbePaginationURLBeforeAnomaly(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	const repeatedBuildsURL = "https://api.appstoreconnect.apple.com/v1/builds?page=2"

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	requestCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/builds" && req.URL.Query().Get("page") == "":
			body := `{
				"data":[{"type":"builds","id":"build-page1-newest","attributes":{"version":"12","uploadedDate":"2026-03-01T00:00:00Z"}}],
				"links":{"next":"` + repeatedBuildsURL + `"}
			}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil

		case req.Method == http.MethodGet && req.URL.String() == repeatedBuildsURL:
			// Older page with self-repeating next link.
			body := `{
				"data":[{"type":"builds","id":"build-page2-older","attributes":{"version":"11","uploadedDate":"2026-02-01T00:00:00Z"}}],
				"links":{"next":"` + repeatedBuildsURL + `"}
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

	var runErr error
	stdout, _ := captureOutput(t, func() {
		if err := root.Parse([]string{"builds", "latest", "--app", "100000001"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if runErr == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(runErr.Error(), "failed to paginate builds") {
		t.Fatalf("expected builds pagination context, got %v", runErr)
	}
	if !strings.Contains(runErr.Error(), "detected repeated pagination URL") {
		t.Fatalf("expected repeated pagination URL error, got %v", runErr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if requestCount != 2 {
		t.Fatalf("expected exactly 2 build page requests, got %d", requestCount)
	}
}

func TestBuildsLatestFailsWhenAnomalyProbeHitsPageScanCap(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	const pagePrefix = "https://api.appstoreconnect.apple.com/v1/builds?page="

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	nextPageByCurrent := map[string]string{
		"2":  "3",
		"3":  "4",
		"4":  "5",
		"5":  "6",
		"6":  "7",
		"7":  "8",
		"8":  "9",
		"9":  "10",
		"10": "11",
	}

	requestCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		if req.Method != http.MethodGet || req.URL.Path != "/v1/builds" {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
		}

		page := req.URL.Query().Get("page")
		switch page {
		case "":
			body := `{
				"data":[{"type":"builds","id":"build-page1-older","attributes":{"version":"10","uploadedDate":"2026-01-01T00:00:00Z"}}],
				"links":{"next":"` + pagePrefix + `2"}
			}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case "2":
			// Anomaly starts here: newer build appears after page 1.
			body := `{
				"data":[{"type":"builds","id":"build-page2-newer","attributes":{"version":"11","uploadedDate":"2026-02-01T00:00:00Z"}}],
				"links":{"next":"` + pagePrefix + `3"}
			}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		default:
			nextPage, ok := nextPageByCurrent[page]
			if !ok {
				t.Fatalf("unexpected page query %q", page)
			}
			body := `{
				"data":[{"type":"builds","id":"build-page` + page + `-older","attributes":{"version":"10","uploadedDate":"2026-01-15T00:00:00Z"}}],
				"links":{"next":"` + pagePrefix + nextPage + `"}
			}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, _ := captureOutput(t, func() {
		if err := root.Parse([]string{"builds", "latest", "--app", "100000001"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if runErr == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(runErr.Error(), "builds latest: failed to paginate builds: reached scan cap of 10 pages") {
		t.Fatalf("expected page cap error, got %v", runErr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if requestCount != 10 {
		t.Fatalf("expected exactly 10 build page requests, got %d", requestCount)
	}
}
