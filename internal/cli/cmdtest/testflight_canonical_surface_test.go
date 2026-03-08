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

func TestTestFlightHelpShowsCanonicalSubcommands(t *testing.T) {
	root := RootCommand("1.2.3")

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"testflight"}); err != nil {
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

	for _, want := range []string{
		"feedback",
		"crashes",
		"groups",
		"testers",
		"distribution",
		"agreements",
		"notifications",
		"config",
	} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("expected help to contain %q, got %q", want, stderr)
		}
	}

	for _, legacy := range []string{
		"beta-feedback",
		"beta-crash-logs",
		"beta-groups",
		"beta-testers",
		"beta-details",
		"beta-license-agreements",
		"beta-notifications",
	} {
		if strings.Contains(stderr, legacy) {
			t.Fatalf("expected help to hide legacy alias %q, got %q", legacy, stderr)
		}
	}
}

func TestRootHelpHidesDeprecatedCompatibilityCommands(t *testing.T) {
	root := RootCommand("1.2.3")

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{}); err != nil {
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
	if strings.Contains(stderr, "feedback:") {
		t.Fatalf("expected root help to hide deprecated feedback command, got %q", stderr)
	}
	if strings.Contains(stderr, "crashes:") {
		t.Fatalf("expected root help to hide deprecated crashes command, got %q", stderr)
	}
	if !strings.Contains(stderr, "testflight:") {
		t.Fatalf("expected root help to still show testflight command, got %q", stderr)
	}
}

func TestDeprecatedHelpShowsCanonicalPathsOnly(t *testing.T) {
	tests := []struct {
		name         string
		args         []string
		wantUsage    string
		wantWarning  string
		wantNotShown []string
	}{
		{
			name:        "feedback root help",
			args:        []string{"feedback"},
			wantUsage:   "asc testflight feedback list [flags]",
			wantWarning: "",
			wantNotShown: []string{
				"asc feedback [flags]",
			},
		},
		{
			name:        "crashes root help",
			args:        []string{"crashes"},
			wantUsage:   "asc testflight crashes list [flags]",
			wantWarning: "",
			wantNotShown: []string{
				"asc crashes [flags]",
			},
		},
		{
			name:        "beta details alias help",
			args:        []string{"testflight", "beta-details"},
			wantUsage:   "asc testflight distribution <subcommand> [flags]",
			wantWarning: "",
			wantNotShown: []string{
				"asc testflight beta-details <subcommand> [flags]",
				"get",
				"update",
			},
		},
		{
			name:        "beta groups alias help",
			args:        []string{"testflight", "beta-groups"},
			wantUsage:   "asc testflight groups <subcommand> [flags]",
			wantWarning: "",
			wantNotShown: []string{
				"asc testflight beta-groups <subcommand> [flags]",
			},
		},
		{
			name:        "beta testers alias help",
			args:        []string{"testflight", "beta-testers"},
			wantUsage:   "asc testflight testers <subcommand> [flags]",
			wantWarning: "",
			wantNotShown: []string{
				"asc testflight beta-testers <subcommand> [flags]",
			},
		},
		{
			name:        "beta agreements alias help",
			args:        []string{"testflight", "beta-license-agreements"},
			wantUsage:   "asc testflight agreements <subcommand> [flags]",
			wantWarning: "",
			wantNotShown: []string{
				"asc testflight beta-license-agreements <subcommand> [flags]",
			},
		},
		{
			name:        "beta notifications alias help",
			args:        []string{"testflight", "beta-notifications"},
			wantUsage:   "asc testflight notifications send --build \"BUILD_ID\"",
			wantWarning: "",
			wantNotShown: []string{
				"asc testflight beta-notifications <subcommand> [flags]",
			},
		},
		{
			name:        "sync alias help",
			args:        []string{"testflight", "sync"},
			wantUsage:   "asc testflight config export [flags]",
			wantWarning: "",
			wantNotShown: []string{
				"asc testflight sync <subcommand> [flags]",
			},
		},
		{
			name:        "metrics beta tester usages alias help",
			args:        []string{"testflight", "metrics", "beta-tester-usages"},
			wantUsage:   "asc testflight metrics app-testers --app \"APP_ID\" [flags]",
			wantWarning: "",
			wantNotShown: []string{
				"asc testflight metrics beta-tester-usages --app \"APP_ID\" [flags]",
			},
		},
		{
			name:        "beta feedback alias help",
			args:        []string{"testflight", "beta-feedback"},
			wantUsage:   "asc testflight feedback <subcommand> | asc testflight crashes <subcommand>",
			wantWarning: betaFeedbackDeprecationWarning,
			wantNotShown: []string{
				"crash-submissions",
				"screenshot-submissions",
				"crash-log",
				"asc testflight beta-feedback <subcommand> [flags]",
			},
		},
		{
			name:        "beta crash logs alias help",
			args:        []string{"testflight", "beta-crash-logs"},
			wantUsage:   "asc testflight crashes log --crash-log-id \"CRASH_LOG_ID\"",
			wantWarning: betaCrashLogsDeprecationWarning,
			wantNotShown: []string{
				"asc testflight beta-crash-logs <subcommand> [flags]",
				"get",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("ASC_APP_ID", "")
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
			if !strings.Contains(stderr, test.wantUsage) {
				t.Fatalf("expected help to contain %q, got %q", test.wantUsage, stderr)
			}
			if test.wantWarning != "" {
				requireStderrContainsWarning(t, stderr, test.wantWarning)
			}
			for _, notShown := range test.wantNotShown {
				if strings.Contains(stderr, notShown) {
					t.Fatalf("expected help to hide %q, got %q", notShown, stderr)
				}
			}
		})
	}
}

func TestTestFlightHelpHidesTestFlightApps(t *testing.T) {
	root := RootCommand("1.2.3")

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"testflight"}); err != nil {
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
	if strings.Contains(stderr, "\n  apps ") {
		t.Fatalf("expected testflight help to hide apps, got %q", stderr)
	}
}

func TestTestFlightAppsShowsRemovedGuidance(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"testflight", "apps", "list"}); err != nil {
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
	if !strings.Contains(stderr, "Error: `asc testflight apps` was removed. Use `asc apps list` instead.") {
		t.Fatalf("expected removed guidance, got %q", stderr)
	}
	if !strings.Contains(stderr, "asc apps <subcommand> [flags]") {
		t.Fatalf("expected deprecated usage redirect, got %q", stderr)
	}
}

func TestTestFlightGroupsHelpHidesRawRelationshipSurface(t *testing.T) {
	root := RootCommand("1.2.3")

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"testflight", "groups"}); err != nil {
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
	if !strings.Contains(stderr, "compatibility") {
		t.Fatalf("expected groups help to contain compatibility, got %q", stderr)
	}
	for _, hidden := range []string{"relationships", "compatible-build-check"} {
		if strings.Contains(stderr, hidden) {
			t.Fatalf("expected groups help to hide %q, got %q", hidden, stderr)
		}
	}
}

func TestTestFlightMetricsHelpShowsCanonicalScopes(t *testing.T) {
	root := RootCommand("1.2.3")

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"testflight", "metrics"}); err != nil {
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
	for _, want := range []string{"public-link", "group-testers", "app-testers"} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("expected metrics help to contain %q, got %q", want, stderr)
		}
	}
	for _, hidden := range []string{"\nbeta-tester-usages", "\n  testers "} {
		if strings.Contains(stderr, hidden) {
			t.Fatalf("expected metrics help to hide %q, got %q", hidden, stderr)
		}
	}
}

func TestTestFlightReviewHelpShowsCanonicalVerbs(t *testing.T) {
	root := RootCommand("1.2.3")

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"testflight", "review"}); err != nil {
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
	for _, want := range []string{"view", "edit", "submit", "submissions"} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("expected review help to contain %q, got %q", want, stderr)
		}
	}
	for _, legacy := range []string{"get\t", "update\t"} {
		if strings.Contains(stderr, legacy) {
			t.Fatalf("expected review help to hide legacy verb %q, got %q", strings.TrimSpace(legacy), stderr)
		}
	}
}

func TestCanonicalTestFlightValidationPaths(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "review view missing app",
			args:    []string{"testflight", "review", "view"},
			wantErr: "--app is required",
		},
		{
			name:    "groups list missing app",
			args:    []string{"testflight", "groups", "list"},
			wantErr: "--app or --global is required",
		},
		{
			name:    "testers view missing id",
			args:    []string{"testflight", "testers", "view"},
			wantErr: "--id is required",
		},
		{
			name:    "distribution view missing build",
			args:    []string{"testflight", "distribution", "view"},
			wantErr: "--build is required",
		},
		{
			name:    "agreements view missing selector",
			args:    []string{"testflight", "agreements", "view"},
			wantErr: "--id or --app is required",
		},
		{
			name:    "notifications send missing build",
			args:    []string{"testflight", "notifications", "send"},
			wantErr: "--build is required",
		},
		{
			name:    "metrics app-testers missing app",
			args:    []string{"testflight", "metrics", "app-testers"},
			wantErr: "--app is required",
		},
		{
			name:    "config export missing app",
			args:    []string{"testflight", "config", "export"},
			wantErr: "--app is required",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("ASC_APP_ID", "")
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
			if !strings.Contains(stderr, test.wantErr) {
				t.Fatalf("expected stderr to contain %q, got %q", test.wantErr, stderr)
			}
		})
	}
}

func TestTestFlightFeedbackViewOutput(t *testing.T) {
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
		if req.URL.Path != "/v1/betaFeedbackScreenshotSubmissions/sub-2" {
			t.Fatalf("expected path /v1/betaFeedbackScreenshotSubmissions/sub-2, got %s", req.URL.Path)
		}
		body := `{"data":{"type":"betaFeedbackScreenshotSubmissions","id":"sub-2"}}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"testflight", "feedback", "view", "--submission-id", "sub-2"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"id":"sub-2"`) {
		t.Fatalf("expected submission id in output, got %q", stdout)
	}
}

func TestTestFlightReviewViewOutput(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/betaAppReviewDetails" {
			t.Fatalf("expected path /v1/betaAppReviewDetails, got %s", req.URL.Path)
		}
		if req.URL.Query().Get("filter[app]") != "app-1" {
			t.Fatalf("expected filter app app-1, got %q", req.URL.Query().Get("filter[app]"))
		}
		body := `{"data":[{"type":"betaAppReviewDetails","id":"detail-1"}],"links":{"next":""}}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"testflight", "review", "view", "--app", "app-1"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"id":"detail-1"`) {
		t.Fatalf("expected detail id in output, got %q", stdout)
	}
}

func TestTestFlightFeedbackDeleteOutput(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodDelete {
			t.Fatalf("expected DELETE, got %s", req.Method)
		}
		if req.URL.Path != "/v1/betaFeedbackScreenshotSubmissions/sub-2" {
			t.Fatalf("expected path /v1/betaFeedbackScreenshotSubmissions/sub-2, got %s", req.URL.Path)
		}
		return &http.Response{
			StatusCode: http.StatusNoContent,
			Body:       io.NopCloser(strings.NewReader("")),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"testflight", "feedback", "delete", "--submission-id", "sub-2", "--confirm"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"id":"sub-2"`) || !strings.Contains(stdout, `"deleted":true`) {
		t.Fatalf("expected delete result in output, got %q", stdout)
	}
}

func TestTestFlightCrashesViewOutput(t *testing.T) {
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
		if req.URL.Path != "/v1/betaFeedbackCrashSubmissions/sub-1" {
			t.Fatalf("expected path /v1/betaFeedbackCrashSubmissions/sub-1, got %s", req.URL.Path)
		}
		body := `{"data":{"type":"betaFeedbackCrashSubmissions","id":"sub-1"}}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"testflight", "crashes", "view", "--submission-id", "sub-1"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"id":"sub-1"`) {
		t.Fatalf("expected crash submission id in output, got %q", stdout)
	}
}

func TestTestFlightDistributionViewOutput(t *testing.T) {
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
		if req.URL.Path != "/v1/buildBetaDetails" {
			t.Fatalf("expected path /v1/buildBetaDetails, got %s", req.URL.Path)
		}
		if req.URL.Query().Get("filter[build]") != "build-1" {
			t.Fatalf("expected build filter build-1, got %q", req.URL.Query().Get("filter[build]"))
		}
		body := `{"data":[{"type":"buildBetaDetails","id":"detail-1"}],"links":{"next":""}}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"testflight", "distribution", "view", "--build", "build-1"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"id":"detail-1"`) {
		t.Fatalf("expected distribution detail id in output, got %q", stdout)
	}
}

func TestTestFlightCrashesLogOutputBySubmissionID(t *testing.T) {
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
		if req.URL.Path != "/v1/betaFeedbackCrashSubmissions/sub-1/crashLog" {
			t.Fatalf("expected path /v1/betaFeedbackCrashSubmissions/sub-1/crashLog, got %s", req.URL.Path)
		}
		body := `{"data":{"type":"betaCrashLogs","id":"log-1"}}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"testflight", "crashes", "log", "--submission-id", "sub-1"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"id":"log-1"`) {
		t.Fatalf("expected crash log id in output, got %q", stdout)
	}
}

func TestTestFlightMetricsAppTestersOutput(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")

	tempDir := t.TempDir()
	keyPath := filepath.Join(tempDir, "key.p8")
	writeECDSAPEM(t, keyPath)
	t.Setenv("ASC_KEY_ID", "TEST_KEY")
	t.Setenv("ASC_ISSUER_ID", "TEST_ISSUER")
	t.Setenv("ASC_PRIVATE_KEY_PATH", keyPath)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/apps/app-1/metrics/betaTesterUsages" {
			t.Fatalf("expected path /v1/apps/app-1/metrics/betaTesterUsages, got %s", req.URL.Path)
		}
		body := `{"data":[{"id":"usage-1"}],"links":{"next":""}}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"testflight", "metrics", "app-testers", "--app", "app-1"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"usage-1"`) {
		t.Fatalf("expected usage in output, got %q", stdout)
	}
}

func TestTestFlightCrashesLogOutputByCrashLogID(t *testing.T) {
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
		if req.URL.Path != "/v1/betaCrashLogs/log-1" {
			t.Fatalf("expected path /v1/betaCrashLogs/log-1, got %s", req.URL.Path)
		}
		body := `{"data":{"type":"betaCrashLogs","id":"log-1"}}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"testflight", "crashes", "log", "--crash-log-id", "log-1"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"id":"log-1"`) {
		t.Fatalf("expected crash log id in output, got %q", stdout)
	}
}

func TestTestFlightCrashesLogRequiresExactlyOneLookupFlag(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "missing all lookup flags",
			args:    []string{"testflight", "crashes", "log"},
			wantErr: "exactly one of --submission-id or --crash-log-id is required",
		},
		{
			name:    "both lookup flags",
			args:    []string{"testflight", "crashes", "log", "--submission-id", "sub-1", "--crash-log-id", "log-1"},
			wantErr: "exactly one of --submission-id or --crash-log-id is required",
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
			if !strings.Contains(stderr, test.wantErr) {
				t.Fatalf("expected stderr to contain %q, got %q", test.wantErr, stderr)
			}
		})
	}
}

func TestLegacyFeedbackAndCrashAliasesWarnAndDelegate(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	tests := []struct {
		name        string
		args        []string
		wantPath    string
		wantWarning string
	}{
		{
			name:        "root feedback shim",
			args:        []string{"feedback", "--app", "123"},
			wantPath:    "/v1/apps/123/betaFeedbackScreenshotSubmissions",
			wantWarning: "Warning: `asc feedback` is deprecated. Use `asc testflight feedback list`.",
		},
		{
			name:        "root crashes shim",
			args:        []string{"crashes", "--app", "123"},
			wantPath:    "/v1/apps/123/betaFeedbackCrashSubmissions",
			wantWarning: "Warning: `asc crashes` is deprecated. Use `asc testflight crashes list`.",
		},
		{
			name:        "beta feedback alias",
			args:        []string{"testflight", "beta-feedback", "crash-log", "get", "--id", "sub-1"},
			wantPath:    "/v1/betaFeedbackCrashSubmissions/sub-1/crashLog",
			wantWarning: "Warning: `asc testflight beta-feedback ...` is deprecated. Use `asc testflight feedback ...` and `asc testflight crashes ...`.",
		},
		{
			name:        "beta crash logs alias",
			args:        []string{"testflight", "beta-crash-logs", "get", "--id", "log-1"},
			wantPath:    "/v1/betaCrashLogs/log-1",
			wantWarning: "Warning: `asc testflight beta-crash-logs ...` is deprecated. Use `asc testflight crashes log`.",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			originalTransport := http.DefaultTransport
			t.Cleanup(func() {
				http.DefaultTransport = originalTransport
			})

			http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.URL.Path != test.wantPath {
					t.Fatalf("expected path %s, got %s", test.wantPath, req.URL.Path)
				}
				body := `{"data":{"type":"stub","id":"ok-1"}}`
				if strings.Contains(test.wantPath, "/v1/apps/123/") {
					body = `{"data":[{"type":"stub","id":"ok-1"}]}`
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
				if err := root.Parse(test.args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				if err := root.Run(context.Background()); err != nil {
					t.Fatalf("run error: %v", err)
				}
			})

			if !strings.Contains(stdout, `"id":"ok-1"`) {
				t.Fatalf("expected delegated output, got %q", stdout)
			}
			if !strings.Contains(stderr, test.wantWarning) {
				t.Fatalf("expected warning %q, got %q", test.wantWarning, stderr)
			}
		})
	}
}
