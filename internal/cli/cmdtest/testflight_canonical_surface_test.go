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
		"app-localizations",
		"pre-release",
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
		"beta-app-localizations",
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
	if strings.Contains(stderr, "beta-app-localizations:") {
		t.Fatalf("expected root help to hide deprecated beta-app-localizations command, got %q", stderr)
	}
	if !strings.Contains(stderr, "testflight:") {
		t.Fatalf("expected root help to still show testflight command, got %q", stderr)
	}
}

func TestRemovedTestFlightAliasPathsPreferCanonicalParentHelp(t *testing.T) {
	tests := []struct {
		name               string
		args               []string
		wantCanonicalUsage string
		wantRemovedMessage string
		wantCanonicalHint  string
		wantNotShown       []string
	}{
		{
			name:               "feedback root removed",
			args:               []string{"feedback"},
			wantRemovedMessage: "Unknown command: feedback",
			wantNotShown: []string{
				"asc feedback [flags]",
			},
		},
		{
			name:               "crashes root removed",
			args:               []string{"crashes"},
			wantRemovedMessage: "Unknown command: crashes",
			wantNotShown: []string{
				"asc crashes [flags]",
			},
		},
		{
			name:               "beta details alias removed",
			args:               []string{"testflight", "review"},
			wantCanonicalUsage: "asc testflight review <subcommand> [flags]",
			wantCanonicalHint:  "submit",
			wantNotShown: []string{
				"asc testflight beta-details <subcommand> [flags]",
			},
		},
		{
			name:               "beta groups alias removed",
			args:               []string{"testflight", "groups"},
			wantCanonicalUsage: "asc testflight groups <subcommand> [flags]",
			wantCanonicalHint:  "compatibility",
			wantNotShown: []string{
				"asc testflight beta-groups <subcommand> [flags]",
			},
		},
		{
			name:               "beta groups leaf removed",
			args:               []string{"testflight", "groups", "view"},
			wantCanonicalUsage: "asc testflight groups view [flags]",
			wantCanonicalHint:  "--id is required",
			wantNotShown: []string{
				"Get a TestFlight beta group by ID.",
			},
		},
		{
			name:               "beta groups relationships alias removed",
			args:               []string{"testflight", "groups", "relationships"},
			wantCanonicalUsage: "asc testflight groups <subcommand> [flags]",
			wantCanonicalHint:  "links",
			wantNotShown: []string{
				"asc testflight groups relationships <subcommand> [flags]",
			},
		},
		{
			name:               "beta testers alias removed",
			args:               []string{"testflight", "testers"},
			wantCanonicalUsage: "asc testflight testers <subcommand> [flags]",
			wantCanonicalHint:  "invite",
			wantNotShown: []string{
				"asc testflight beta-testers <subcommand> [flags]",
			},
		},
		{
			name:               "beta testers relationships alias removed",
			args:               []string{"testflight", "testers", "relationships"},
			wantCanonicalUsage: "asc testflight testers <subcommand> [flags]",
			wantCanonicalHint:  "links",
			wantNotShown: []string{
				"asc testflight testers relationships <subcommand> [flags]",
			},
		},
		{
			name:               "beta agreements alias removed",
			args:               []string{"testflight", "agreements"},
			wantCanonicalUsage: "asc testflight agreements <subcommand> [flags]",
			wantCanonicalHint:  "edit",
			wantNotShown: []string{
				"asc testflight beta-license-agreements <subcommand> [flags]",
			},
		},
		{
			name:               "beta notifications alias removed",
			args:               []string{"testflight", "notifications"},
			wantCanonicalUsage: "asc testflight notifications <subcommand> [flags]",
			wantCanonicalHint:  "send",
			wantNotShown: []string{
				"asc testflight beta-notifications <subcommand> [flags]",
			},
		},
		{
			name:               "beta app localizations root removed",
			args:               []string{"testflight", "app-localizations"},
			wantCanonicalUsage: "asc testflight app-localizations <subcommand> [flags]",
			wantCanonicalHint:  "view",
			wantNotShown: []string{
				"asc beta-app-localizations <subcommand> [flags]",
			},
		},
		{
			name:               "beta app localizations leaf removed",
			args:               []string{"testflight", "app-localizations", "get"},
			wantCanonicalUsage: "asc testflight app-localizations <subcommand> [flags]",
			wantCanonicalHint:  "view",
			wantNotShown: []string{
				"asc beta-app-localizations get --id \"LOCALIZATION_ID\"",
			},
		},
		{
			name:               "pre-release relationships alias removed",
			args:               []string{"testflight", "pre-release", "links"},
			wantCanonicalUsage: "asc testflight pre-release links <subcommand> [flags]",
			wantCanonicalHint:  "view",
			wantNotShown: []string{
				"asc testflight pre-release relationships <subcommand> [flags]",
			},
		},
		{
			name:               "sync alias removed",
			args:               []string{"testflight", "sync"},
			wantCanonicalUsage: "asc testflight <subcommand> [flags]",
			wantCanonicalHint:  "config",
			wantNotShown: []string{
				"asc testflight sync <subcommand> [flags]",
			},
		},
		{
			name:               "metrics beta tester usages alias removed",
			args:               []string{"testflight", "metrics", "beta-tester-usages"},
			wantCanonicalUsage: "asc testflight metrics <subcommand> [flags]",
			wantCanonicalHint:  "app-testers",
			wantNotShown: []string{
				"asc testflight metrics beta-tester-usages --app \"APP_ID\" [flags]",
			},
		},
		{
			name:               "beta feedback alias removed",
			args:               []string{"testflight", "beta-feedback"},
			wantCanonicalUsage: "asc testflight <subcommand> [flags]",
			wantCanonicalHint:  "feedback",
			wantNotShown: []string{
				"crash-submissions",
				"screenshot-submissions",
				"crash-log",
				"asc testflight beta-feedback <subcommand> [flags]",
			},
		},
		{
			name:               "beta crash logs alias removed",
			args:               []string{"testflight", "beta-crash-logs"},
			wantCanonicalUsage: "asc testflight <subcommand> [flags]",
			wantCanonicalHint:  "crashes",
			wantNotShown: []string{
				"asc testflight beta-crash-logs <subcommand> [flags]",
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
			if test.wantRemovedMessage != "" {
				if !strings.Contains(stderr, test.wantRemovedMessage) {
					t.Fatalf("expected removed command message %q, got %q", test.wantRemovedMessage, stderr)
				}
			} else {
				if !strings.Contains(stderr, test.wantCanonicalUsage) {
					t.Fatalf("expected help to contain %q, got %q", test.wantCanonicalUsage, stderr)
				}
				if test.wantCanonicalHint != "" && !strings.Contains(stderr, test.wantCanonicalHint) {
					t.Fatalf("expected help to contain canonical hint %q, got %q", test.wantCanonicalHint, stderr)
				}
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

func TestUnknownCommandDoesNotSuggestDeprecatedRootCommands(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"feedbak"}); err != nil {
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
	if strings.Contains(stderr, "Did you mean: feedback") || strings.Contains(stderr, "Did you mean: crashes") {
		t.Fatalf("expected no deprecated root suggestion, got %q", stderr)
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

func TestTestFlightGroupsCompatibilityHelpUsesCanonicalCopy(t *testing.T) {
	root := RootCommand("1.2.3")

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"testflight", "groups", "compatibility"}); err != nil {
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
	if !strings.Contains(stderr, "View recruitment compatibility for a group.") {
		t.Fatalf("expected canonical compatibility copy, got %q", stderr)
	}
	if strings.Contains(stderr, "recruitment criteria") {
		t.Fatalf("expected help without leaked schema phrasing, got %q", stderr)
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
			wantErr: "--build-id is required",
		},
		{
			name:    "agreements view missing selector",
			args:    []string{"testflight", "agreements", "view"},
			wantErr: "--id or --app is required",
		},
		{
			name:    "notifications send missing build",
			args:    []string{"testflight", "notifications", "send"},
			wantErr: "--build-id is required",
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
		{
			name:    "app localizations list missing app",
			args:    []string{"testflight", "app-localizations", "list"},
			wantErr: "--app is required",
		},
		{
			name:    "app localizations get missing id",
			args:    []string{"testflight", "app-localizations", "view"},
			wantErr: "--id is required",
		},
		{
			name:    "app localizations create missing locale",
			args:    []string{"testflight", "app-localizations", "create", "--app", "APP_ID"},
			wantErr: "--locale is required",
		},
		{
			name:    "pre-release list missing app",
			args:    []string{"testflight", "pre-release", "list"},
			wantErr: "--app is required",
		},
		{
			name:    "pre-release view missing id",
			args:    []string{"testflight", "pre-release", "view"},
			wantErr: "--id is required",
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

func TestTestFlightFeedbackListOutputHasNoDeprecationWarning(t *testing.T) {
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
		if req.URL.Path != "/v1/apps/123/betaFeedbackScreenshotSubmissions" {
			t.Fatalf("expected path /v1/apps/123/betaFeedbackScreenshotSubmissions, got %s", req.URL.Path)
		}
		body := `{"data":[{"type":"betaFeedbackScreenshotSubmissions","id":"feedback-1"}],"links":{"next":""}}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"testflight", "feedback", "list", "--app", "123"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"id":"feedback-1"`) {
		t.Fatalf("expected feedback list output, got %q", stdout)
	}
}

func TestTestFlightCrashesListOutputHasNoDeprecationWarning(t *testing.T) {
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
		if req.URL.Path != "/v1/apps/123/betaFeedbackCrashSubmissions" {
			t.Fatalf("expected path /v1/apps/123/betaFeedbackCrashSubmissions, got %s", req.URL.Path)
		}
		body := `{"data":[{"type":"betaFeedbackCrashSubmissions","id":"crash-1"}],"links":{"next":""}}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"testflight", "crashes", "list", "--app", "123"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"id":"crash-1"`) {
		t.Fatalf("expected crashes list output, got %q", stdout)
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
		if err := root.Parse([]string{"testflight", "distribution", "view", "--build-id", "build-1"}); err != nil {
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

func TestTestFlightAppLocalizationsHelpShowsCanonicalSurface(t *testing.T) {
	root := RootCommand("1.2.3")

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"testflight", "app-localizations"}); err != nil {
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
	for _, want := range []string{"list", "view", "app", "create", "update", "delete"} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("expected app-localizations help to contain %q, got %q", want, stderr)
		}
	}
	if strings.Contains(stderr, "beta-app-localizations") {
		t.Fatalf("expected canonical help without legacy root path, got %q", stderr)
	}
}

func TestTestFlightAppLocalizationsListOutputHasNoDeprecationWarning(t *testing.T) {
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
		if req.URL.Path != "/v1/betaAppLocalizations" {
			t.Fatalf("expected path /v1/betaAppLocalizations, got %s", req.URL.Path)
		}
		if req.URL.Query().Get("filter[app]") != "123" {
			t.Fatalf("expected filter[app]=123, got %q", req.URL.Query().Get("filter[app]"))
		}
		body := `{"data":[{"type":"betaAppLocalizations","id":"loc-1"}],"links":{"next":""}}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"testflight", "app-localizations", "list", "--app", "123"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"id":"loc-1"`) {
		t.Fatalf("expected output, got %q", stdout)
	}
}

func TestTestFlightAppLocalizationsCreateOutput(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/betaAppLocalizations" {
			t.Fatalf("expected path /v1/betaAppLocalizations, got %s", req.URL.Path)
		}
		body := `{"data":{"type":"betaAppLocalizations","id":"loc-1","attributes":{"locale":"en-US"}}}`
		return &http.Response{
			StatusCode: http.StatusCreated,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"testflight", "app-localizations", "create", "--app", "app-1", "--locale", "en-US"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"id":"loc-1"`) {
		t.Fatalf("expected created localization in output, got %q", stdout)
	}
}

func TestTestFlightPreReleaseHelpShowsCanonicalVerbs(t *testing.T) {
	root := RootCommand("1.2.3")

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"testflight", "pre-release"}); err != nil {
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
	for _, want := range []string{"list", "view", "app", "builds", "links"} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("expected pre-release help to contain %q, got %q", want, stderr)
		}
	}
	if strings.Contains(stderr, "relationships") {
		t.Fatalf("expected pre-release help to hide deprecated relationships alias, got %q", stderr)
	}
}

func TestTopLevelPreReleaseVersionsRemoved(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"pre-release-versions"}); err != nil {
			runErr = err
			return
		}
		runErr = root.Run(context.Background())
	})

	if !errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("expected ErrHelp, got %v", runErr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "Unknown command: pre-release-versions") {
		t.Fatalf("expected removed root to be unknown, got %q", stderr)
	}
}

func TestRemovedPreReleaseVersionsCommandsShowMigrationGuidance(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "list command",
			args:    []string{"pre-release-versions", "list", "--app", "APP_ID"},
			wantErr: "Unknown command: pre-release-versions",
		},
		{
			name:    "view command",
			args:    []string{"pre-release-versions", "get", "--id", "PR_ID"},
			wantErr: "Unknown command: pre-release-versions",
		},
		{
			name:    "app view command",
			args:    []string{"pre-release-versions", "app", "get", "--id", "PR_ID"},
			wantErr: "Unknown command: pre-release-versions",
		},
		{
			name:    "relationships view command",
			args:    []string{"pre-release-versions", "relationships", "get", "--id", "PR_ID", "--type", "app"},
			wantErr: "Unknown command: pre-release-versions",
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

func TestTestFlightPreReleaseLinksViewHasNoDeprecationWarning(t *testing.T) {
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
		if req.URL.Path != "/v1/preReleaseVersions/pr-1/relationships/app" {
			t.Fatalf("expected path /v1/preReleaseVersions/pr-1/relationships/app, got %s", req.URL.Path)
		}
		body := `{"data":{"type":"apps","id":"app-1"}}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"testflight", "pre-release", "links", "view", "--id", "pr-1", "--type", "app"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"id":"app-1"`) {
		t.Fatalf("expected delegated output, got %q", stdout)
	}
}
