package cmdtest

import (
	"context"
	"errors"
	"flag"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	buildsLegacyBuildWarning  = "Warning: `--build` is deprecated. Use `--build-id`."
	buildsLegacyIDWarning     = "Warning: `--id` as a build selector is deprecated. Use `--build-id`."
	buildsLegacyNewestWarning = "Warning: `--newest` is deprecated. Use `--latest`."
)

func TestBuildsSelectorAliasesWarnAndMatchCanonicalValidationPaths(t *testing.T) {
	run := func(t *testing.T, args []string) (string, string, error) {
		t.Helper()

		root := RootCommand("1.2.3")
		root.FlagSet.SetOutput(io.Discard)

		var runErr error
		stdout, stderr := captureOutput(t, func() {
			if err := root.Parse(args); err != nil {
				t.Fatalf("parse error: %v", err)
			}
			runErr = root.Run(context.Background())
		})

		return stdout, stderr, runErr
	}

	tests := []struct {
		name          string
		canonicalArgs []string
		aliasArgs     []string
		warning       string
		wantErr       string
	}{
		{
			name:          "wait build alias",
			canonicalArgs: []string{"builds", "wait", "--build-id", "BUILD_123", "--poll-interval", "0"},
			aliasArgs:     []string{"builds", "wait", "--build", "BUILD_123", "--poll-interval", "0"},
			warning:       buildsLegacyBuildWarning,
			wantErr:       "Error: --poll-interval must be greater than 0",
		},
		{
			name:          "wait newest alias",
			canonicalArgs: []string{"builds", "wait", "--app", "APP_123", "--latest", "--poll-interval", "0"},
			aliasArgs:     []string{"builds", "wait", "--app", "APP_123", "--newest", "--poll-interval", "0"},
			warning:       buildsLegacyNewestWarning,
			wantErr:       "Error: --poll-interval must be greater than 0",
		},
		{
			name:          "dsyms build alias",
			canonicalArgs: []string{"builds", "dsyms", "--build-id", "BUILD_123", "--app", "APP_123"},
			aliasArgs:     []string{"builds", "dsyms", "--build", "BUILD_123", "--app", "APP_123"},
			warning:       buildsLegacyBuildWarning,
			wantErr:       "Error: --build-id cannot be combined with --app, --latest, --build-number, --version, --platform, --processing-state, --exclude-expired, or --not-expired",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			canonicalStdout, canonicalStderr, canonicalErr := run(t, test.canonicalArgs)
			if !errors.Is(canonicalErr, flag.ErrHelp) {
				t.Fatalf("expected canonical ErrHelp, got %v", canonicalErr)
			}
			if canonicalStdout != "" {
				t.Fatalf("expected empty canonical stdout, got %q", canonicalStdout)
			}
			if !strings.Contains(canonicalStderr, test.wantErr) {
				t.Fatalf("expected canonical stderr to contain %q, got %q", test.wantErr, canonicalStderr)
			}

			aliasStdout, aliasStderr, aliasErr := run(t, test.aliasArgs)
			if !errors.Is(aliasErr, flag.ErrHelp) {
				t.Fatalf("expected alias ErrHelp, got %v", aliasErr)
			}
			if aliasStdout != "" {
				t.Fatalf("expected empty alias stdout, got %q", aliasStdout)
			}
			requireStderrContainsWarning(t, aliasStderr, test.warning)
			if strings.Contains(aliasStderr, "flag provided but not defined") {
				t.Fatalf("expected deprecated alias warning instead of parse error, got %q", aliasStderr)
			}

			if stripDeprecatedCommandWarnings(aliasStderr) != stripDeprecatedCommandWarnings(canonicalStderr) {
				t.Fatalf("expected alias stderr to match canonical stderr apart from warnings, canonical=%q alias=%q", canonicalStderr, aliasStderr)
			}
		})
	}
}

func TestBuildsWaitRejectsConflictingLatestAndNewest(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{
			name: "latest false newest true",
			args: []string{"builds", "wait", "--app", "APP_123", "--latest=false", "--newest=true", "--poll-interval", "0"},
		},
		{
			name: "latest true newest false",
			args: []string{"builds", "wait", "--app", "APP_123", "--latest=true", "--newest=false", "--poll-interval", "0"},
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
			if !strings.Contains(stderr, "Error: --newest conflicts with --latest; use only --latest") {
				t.Fatalf("expected conflict error, got %q", stderr)
			}
			if strings.Contains(stderr, "flag provided but not defined") {
				t.Fatalf("expected usage conflict instead of parse error, got %q", stderr)
			}
		})
	}
}

func TestBuildsSelectorAliasesWarnAndMatchCanonicalFetchPaths(t *testing.T) {
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

		switch req.URL.Path {
		case "/v1/builds/BUILD_123":
			return jsonHTTPResponse(http.StatusOK, `{"data":{"type":"builds","id":"BUILD_123","relationships":{}}}`), nil
		case "/v1/builds/BUILD_123/preReleaseVersion":
			return jsonHTTPResponse(http.StatusOK, `{"data":{"type":"preReleaseVersions","id":"prv-1"}}`), nil
		case "/v1/builds/BUILD_123/app":
			return jsonHTTPResponse(http.StatusOK, `{"data":{"type":"apps","id":"app-1"}}`), nil
		case "/v1/builds/BUILD_123/icons":
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"buildIcons","id":"icon-1"}]}`), nil
		case "/v1/builds/BUILD_123/betaAppReviewSubmission":
			return jsonHTTPResponse(http.StatusOK, `{"data":{"type":"betaAppReviewSubmissions","id":"review-1"}}`), nil
		case "/v1/builds/BUILD_123/buildBetaDetail":
			return jsonHTTPResponse(http.StatusOK, `{"data":{"type":"buildBetaDetails","id":"detail-1"}}`), nil
		case "/v1/builds/BUILD_123/relationships/app":
			return jsonHTTPResponse(http.StatusOK, `{"data":{"type":"apps","id":"app-1"}}`), nil
		case "/v1/builds/BUILD_123/individualTesters":
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"betaTesters","id":"tester-1"}]}`), nil
		case "/v1/builds/BUILD_123/metrics/betaBuildUsages":
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"betaBuildUsages","id":"usage-1"}]}`), nil
		case "/v1/builds/BUILD_123/appEncryptionDeclaration":
			return jsonHTTPResponse(http.StatusOK, `{"data":{"type":"appEncryptionDeclarations","id":"enc-1"}}`), nil
		default:
			t.Fatalf("unexpected request path %s", req.URL.Path)
			return nil, nil
		}
	})

	run := func(t *testing.T, args []string) (string, string) {
		t.Helper()

		root := RootCommand("1.2.3")
		root.FlagSet.SetOutput(io.Discard)

		return captureOutput(t, func() {
			if err := root.Parse(args); err != nil {
				t.Fatalf("parse error: %v", err)
			}
			if err := root.Run(context.Background()); err != nil {
				t.Fatalf("run error: %v", err)
			}
		})
	}

	tests := []struct {
		name          string
		canonicalArgs []string
		aliasArgs     []string
		warning       string
	}{
		{
			name:          "info build alias",
			canonicalArgs: []string{"builds", "info", "--build-id", "BUILD_123", "--output", "json"},
			aliasArgs:     []string{"builds", "info", "--build", "BUILD_123", "--output", "json"},
			warning:       buildsLegacyBuildWarning,
		},
		{
			name:          "app view id alias",
			canonicalArgs: []string{"builds", "app", "view", "--build-id", "BUILD_123", "--output", "json"},
			aliasArgs:     []string{"builds", "app", "view", "--id", "BUILD_123", "--output", "json"},
			warning:       buildsLegacyIDWarning,
		},
		{
			name:          "pre-release-version view id alias",
			canonicalArgs: []string{"builds", "pre-release-version", "view", "--build-id", "BUILD_123", "--output", "json"},
			aliasArgs:     []string{"builds", "pre-release-version", "view", "--id", "BUILD_123", "--output", "json"},
			warning:       buildsLegacyIDWarning,
		},
		{
			name:          "icons list build alias",
			canonicalArgs: []string{"builds", "icons", "list", "--build-id", "BUILD_123", "--output", "json"},
			aliasArgs:     []string{"builds", "icons", "list", "--build", "BUILD_123", "--output", "json"},
			warning:       buildsLegacyBuildWarning,
		},
		{
			name:          "beta-app-review-submission view id alias",
			canonicalArgs: []string{"builds", "beta-app-review-submission", "view", "--build-id", "BUILD_123", "--output", "json"},
			aliasArgs:     []string{"builds", "beta-app-review-submission", "view", "--id", "BUILD_123", "--output", "json"},
			warning:       buildsLegacyIDWarning,
		},
		{
			name:          "build-beta-detail view build alias",
			canonicalArgs: []string{"builds", "build-beta-detail", "view", "--build-id", "BUILD_123", "--output", "json"},
			aliasArgs:     []string{"builds", "build-beta-detail", "view", "--build", "BUILD_123", "--output", "json"},
			warning:       buildsLegacyBuildWarning,
		},
		{
			name:          "links view build alias",
			canonicalArgs: []string{"builds", "links", "view", "--build-id", "BUILD_123", "--type", "app", "--output", "json"},
			aliasArgs:     []string{"builds", "links", "view", "--build", "BUILD_123", "--type", "app", "--output", "json"},
			warning:       buildsLegacyBuildWarning,
		},
		{
			name:          "individual-testers list build alias",
			canonicalArgs: []string{"builds", "individual-testers", "list", "--build-id", "BUILD_123", "--output", "json"},
			aliasArgs:     []string{"builds", "individual-testers", "list", "--build", "BUILD_123", "--output", "json"},
			warning:       buildsLegacyBuildWarning,
		},
		{
			name:          "metrics beta-usages build alias",
			canonicalArgs: []string{"builds", "metrics", "beta-usages", "--build-id", "BUILD_123", "--output", "json"},
			aliasArgs:     []string{"builds", "metrics", "beta-usages", "--build", "BUILD_123", "--output", "json"},
			warning:       buildsLegacyBuildWarning,
		},
		{
			name:          "app-encryption-declaration view id alias",
			canonicalArgs: []string{"builds", "app-encryption-declaration", "view", "--build-id", "BUILD_123", "--output", "json"},
			aliasArgs:     []string{"builds", "app-encryption-declaration", "view", "--id", "BUILD_123", "--output", "json"},
			warning:       buildsLegacyIDWarning,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			canonicalStdout, canonicalStderr := run(t, test.canonicalArgs)
			aliasStdout, aliasStderr := run(t, test.aliasArgs)

			if canonicalStderr != "" {
				t.Fatalf("expected canonical stderr to be empty, got %q", canonicalStderr)
			}
			requireStderrContainsWarning(t, aliasStderr, test.warning)
			assertOnlyDeprecatedCommandWarnings(t, aliasStderr)

			if canonicalStdout != aliasStdout {
				t.Fatalf("expected canonical and alias stdout to match, canonical=%q alias=%q", canonicalStdout, aliasStdout)
			}
		})
	}
}

func TestBuildsSelectorAliasesRejectConflictingBuildValues(t *testing.T) {
	run := func(t *testing.T, args []string) (string, string, error) {
		t.Helper()

		root := RootCommand("1.2.3")
		root.FlagSet.SetOutput(io.Discard)

		var runErr error
		stdout, stderr := captureOutput(t, func() {
			if err := root.Parse(args); err != nil {
				t.Fatalf("parse error: %v", err)
			}
			runErr = root.Run(context.Background())
		})

		return stdout, stderr, runErr
	}

	tests := []struct {
		name string
		args []string
	}{
		{
			name: "info conflicting build values",
			args: []string{"builds", "info", "--build-id", "BUILD_CANON", "--build", "BUILD_LEGACY"},
		},
		{
			name: "wait conflicting build values",
			args: []string{"builds", "wait", "--build-id", "BUILD_CANON", "--build", "BUILD_LEGACY", "--poll-interval", "1ms"},
		},
		{
			name: "dsyms conflicting build values",
			args: []string{"builds", "dsyms", "--build-id", "BUILD_CANON", "--build", "BUILD_LEGACY"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stdout, stderr, runErr := run(t, test.args)
			if !errors.Is(runErr, flag.ErrHelp) {
				t.Fatalf("expected ErrHelp, got %v", runErr)
			}
			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, "Error: --build conflicts with --build-id; use only --build-id") {
				t.Fatalf("expected conflicting build selector error, got %q", stderr)
			}
			if strings.Contains(stderr, buildsLegacyBuildWarning) {
				t.Fatalf("expected conflict to fail before deprecation warning, got %q", stderr)
			}
		})
	}
}

func TestBuildsWaitSelectorAliasesWarnAndMatchCanonicalSuccessPaths(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	t.Run("build alias", func(t *testing.T) {
		run := func(args []string) (string, string) {
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
			return captureOutput(t, func() {
				if err := root.Parse(args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				if err := root.Run(context.Background()); err != nil {
					t.Fatalf("run error: %v", err)
				}
			})
		}

		canonicalStdout, canonicalStderr := run([]string{"builds", "wait", "--build-id", "build-1", "--poll-interval", "1ms", "--timeout", "200ms"})
		aliasStdout, aliasStderr := run([]string{"builds", "wait", "--build", "build-1", "--poll-interval", "1ms", "--timeout", "200ms"})

		requireStderrContainsWarning(t, aliasStderr, buildsLegacyBuildWarning)
		if canonicalStdout != aliasStdout {
			t.Fatalf("expected canonical and alias stdout to match, canonical=%q alias=%q", canonicalStdout, aliasStdout)
		}
		if stripDeprecatedCommandWarnings(aliasStderr) != stripDeprecatedCommandWarnings(canonicalStderr) {
			t.Fatalf("expected alias stderr to match canonical stderr apart from warnings, canonical=%q alias=%q", canonicalStderr, aliasStderr)
		}

		waitResult := parseBuildsWaitJSON(t, aliasStdout)
		if waitResult.BuildID != "build-1" || waitResult.ProcessingState != "VALID" {
			t.Fatalf("expected legacy alias to resolve build-1 in VALID state, got %+v", waitResult)
		}
	})

	t.Run("newest alias", func(t *testing.T) {
		run := func(args []string) (string, string) {
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
					if query.Get("limit") != "1" {
						t.Fatalf("expected limit=1, got %q", query.Get("limit"))
					}
					body := `{"data":[{"type":"builds","id":"build-99","attributes":{"uploadedDate":"2026-03-02T18:01:00Z","processingState":"PROCESSING","version":"99"}}]}`
					return &http.Response{
						StatusCode: http.StatusOK,
						Body:       io.NopCloser(strings.NewReader(body)),
						Header:     http.Header{"Content-Type": []string{"application/json"}},
					}, nil
				case 2:
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
			return captureOutput(t, func() {
				if err := root.Parse(args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				if err := root.Run(context.Background()); err != nil {
					t.Fatalf("run error: %v", err)
				}
			})
		}

		canonicalStdout, canonicalStderr := run([]string{"builds", "wait", "--app", "123456789", "--latest", "--poll-interval", "1ms", "--timeout", "200ms"})
		aliasStdout, aliasStderr := run([]string{"builds", "wait", "--app", "123456789", "--newest", "--poll-interval", "1ms", "--timeout", "200ms"})

		requireStderrContainsWarning(t, aliasStderr, buildsLegacyNewestWarning)
		if canonicalStdout != aliasStdout {
			t.Fatalf("expected canonical and alias stdout to match, canonical=%q alias=%q", canonicalStdout, aliasStdout)
		}
		if stripDeprecatedCommandWarnings(aliasStderr) != stripDeprecatedCommandWarnings(canonicalStderr) {
			t.Fatalf("expected alias stderr to match canonical stderr apart from warnings, canonical=%q alias=%q", canonicalStderr, aliasStderr)
		}

		waitResult := parseBuildsWaitJSON(t, aliasStdout)
		if waitResult.BuildID != "build-99" || waitResult.ProcessingState != "VALID" {
			t.Fatalf("expected legacy newest alias to resolve build-99 in VALID state, got %+v", waitResult)
		}
	})
}

func TestBuildsDsymsBuildAliasWarnsAndMatchesCanonicalSuccess(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_APP_ID", "default-app")

	outputDir := filepath.Join(t.TempDir(), "dsyms")
	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	run := func(args []string) (string, string) {
		http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
			switch {
			case req.Method == http.MethodGet && req.URL.Path == "/v1/builds/build-1" && req.URL.RawQuery == "":
				return jsonHTTPResponse(http.StatusOK, `{"data":{"type":"builds","id":"build-1","attributes":{"version":"42","processingState":"VALID"}}}`), nil
			case req.Method == http.MethodGet && req.URL.Path == "/v1/builds/build-1" && req.URL.RawQuery == "include=buildBundles":
				return jsonHTTPResponse(http.StatusOK, `{"data":{"type":"builds","id":"build-1","attributes":{"version":"42"}},"included":[{"type":"buildBundles","id":"bundle-1","attributes":{"bundleId":"com.example.app","dSYMUrl":"https://downloads.example.com/app.dSYM.zip"}}]}`), nil
			case req.Method == http.MethodGet && req.URL.Host == "downloads.example.com" && req.URL.Path == "/app.dSYM.zip":
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader("zipdata")),
					Header:     http.Header{"Content-Type": []string{"application/zip"}},
				}, nil
			default:
				t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
				return nil, nil
			}
		})

		root := RootCommand("1.2.3")
		root.FlagSet.SetOutput(io.Discard)
		return captureOutput(t, func() {
			if err := root.Parse(args); err != nil {
				t.Fatalf("parse error: %v", err)
			}
			if err := root.Run(context.Background()); err != nil {
				t.Fatalf("run error: %v", err)
			}
		})
	}

	canonicalStdout, canonicalStderr := run([]string{"builds", "dsyms", "--build-id", "build-1", "--output-dir", outputDir})
	if err := os.RemoveAll(outputDir); err != nil {
		t.Fatalf("remove output dir between canonical and alias runs: %v", err)
	}
	aliasStdout, aliasStderr := run([]string{"builds", "dsyms", "--build", "build-1", "--output-dir", outputDir})

	requireStderrContainsWarning(t, aliasStderr, buildsLegacyBuildWarning)
	if canonicalStdout != aliasStdout {
		t.Fatalf("expected canonical and alias stdout to match, canonical=%q alias=%q", canonicalStdout, aliasStdout)
	}
	if stripDeprecatedCommandWarnings(aliasStderr) != stripDeprecatedCommandWarnings(canonicalStderr) {
		t.Fatalf("expected alias stderr to match canonical stderr apart from warnings, canonical=%q alias=%q", canonicalStderr, aliasStderr)
	}
}
