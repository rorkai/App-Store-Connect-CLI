package cmdtest

import (
	"context"
	"errors"
	"flag"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func TestTestFlightHelpSurface(t *testing.T) {
	tests := []struct {
		name         string
		args         []string
		wantShown    []string
		wantNotShown []string
	}{
		{
			name:         "root help hides deprecated compatibility commands",
			args:         []string{},
			wantShown:    []string{"testflight:"},
			wantNotShown: []string{"feedback:", "crashes:", "beta-app-localizations:"},
		},
		{
			name: "testflight help shows canonical subcommands",
			args: []string{"testflight"},
			wantShown: []string{
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
			},
			wantNotShown: []string{
				"beta-feedback",
				"beta-crash-logs",
				"beta-groups",
				"beta-testers",
				"beta-details",
				"beta-license-agreements",
				"beta-notifications",
				"beta-app-localizations",
				"\n  apps ",
			},
		},
		{
			name:         "unknown command does not suggest deprecated roots",
			args:         []string{"feedbak"},
			wantShown:    []string{"Unknown command: feedbak"},
			wantNotShown: []string{"Did you mean: feedback", "Did you mean: crashes"},
		},
		{
			name:         "feedback root removed",
			args:         []string{"feedback"},
			wantShown:    []string{"Unknown command: feedback"},
			wantNotShown: []string{"asc feedback [flags]"},
		},
		{
			name:         "crashes root removed",
			args:         []string{"crashes"},
			wantShown:    []string{"Unknown command: crashes"},
			wantNotShown: []string{"asc crashes [flags]"},
		},
		{
			name:      "review help shows canonical verbs",
			args:      []string{"testflight", "review"},
			wantShown: []string{"asc testflight review <subcommand> [flags]", "view", "edit", "submit", "submissions"},
			wantNotShown: []string{
				"asc testflight beta-details <subcommand> [flags]",
				"get\t",
				"update\t",
			},
		},
		{
			name:      "groups help hides raw relationship surface",
			args:      []string{"testflight", "groups"},
			wantShown: []string{"asc testflight groups <subcommand> [flags]", "compatibility"},
			wantNotShown: []string{
				"asc testflight beta-groups <subcommand> [flags]",
				"relationships",
				"compatible-build-check",
			},
		},
		{
			name:         "beta groups leaf removed",
			args:         []string{"testflight", "groups", "view"},
			wantShown:    []string{"asc testflight groups view [flags]", "--id is required"},
			wantNotShown: []string{"Get a TestFlight beta group by ID."},
		},
		{
			name:         "beta groups relationships alias removed",
			args:         []string{"testflight", "groups", "relationships"},
			wantShown:    []string{"asc testflight groups <subcommand> [flags]", "links"},
			wantNotShown: []string{"asc testflight groups relationships <subcommand> [flags]"},
		},
		{
			name:         "beta testers alias removed",
			args:         []string{"testflight", "testers"},
			wantShown:    []string{"asc testflight testers <subcommand> [flags]", "invite"},
			wantNotShown: []string{"asc testflight beta-testers <subcommand> [flags]"},
		},
		{
			name:         "beta testers relationships alias removed",
			args:         []string{"testflight", "testers", "relationships"},
			wantShown:    []string{"asc testflight testers <subcommand> [flags]", "links"},
			wantNotShown: []string{"asc testflight testers relationships <subcommand> [flags]"},
		},
		{
			name:         "beta agreements alias removed",
			args:         []string{"testflight", "agreements"},
			wantShown:    []string{"asc testflight agreements <subcommand> [flags]", "edit"},
			wantNotShown: []string{"asc testflight beta-license-agreements <subcommand> [flags]"},
		},
		{
			name:         "beta notifications alias removed",
			args:         []string{"testflight", "notifications"},
			wantShown:    []string{"asc testflight notifications <subcommand> [flags]", "send"},
			wantNotShown: []string{"asc testflight beta-notifications <subcommand> [flags]"},
		},
		{
			name:         "app localizations help shows canonical surface",
			args:         []string{"testflight", "app-localizations"},
			wantShown:    []string{"asc testflight app-localizations <subcommand> [flags]", "list", "view", "app", "create", "update", "delete"},
			wantNotShown: []string{"beta-app-localizations"},
		},
		{
			name:         "beta app localizations leaf removed",
			args:         []string{"testflight", "app-localizations", "get"},
			wantShown:    []string{"asc testflight app-localizations <subcommand> [flags]", "view"},
			wantNotShown: []string{"asc beta-app-localizations get --id \"LOCALIZATION_ID\""},
		},
		{
			name:         "pre-release help shows canonical verbs",
			args:         []string{"testflight", "pre-release"},
			wantShown:    []string{"list", "view", "app", "builds", "links"},
			wantNotShown: []string{"relationships"},
		},
		{
			name:         "pre-release relationships alias removed",
			args:         []string{"testflight", "pre-release", "links"},
			wantShown:    []string{"asc testflight pre-release links <subcommand> [flags]", "view"},
			wantNotShown: []string{"asc testflight pre-release relationships <subcommand> [flags]"},
		},
		{
			name:         "sync alias removed",
			args:         []string{"testflight", "sync"},
			wantShown:    []string{"asc testflight <subcommand> [flags]", "config"},
			wantNotShown: []string{"asc testflight sync <subcommand> [flags]"},
		},
		{
			name:         "metrics help shows canonical scopes",
			args:         []string{"testflight", "metrics"},
			wantShown:    []string{"public-link", "group-testers", "app-testers"},
			wantNotShown: []string{"\nbeta-tester-usages", "\n  testers "},
		},
		{
			name:         "metrics beta tester usages alias removed",
			args:         []string{"testflight", "metrics", "beta-tester-usages"},
			wantShown:    []string{"asc testflight metrics <subcommand> [flags]", "app-testers"},
			wantNotShown: []string{"asc testflight metrics beta-tester-usages --app \"APP_ID\" [flags]"},
		},
		{
			name:      "beta feedback alias removed",
			args:      []string{"testflight", "beta-feedback"},
			wantShown: []string{"asc testflight <subcommand> [flags]", "feedback"},
			wantNotShown: []string{
				"crash-submissions",
				"screenshot-submissions",
				"crash-log",
				"asc testflight beta-feedback <subcommand> [flags]",
			},
		},
		{
			name:         "beta crash logs alias removed",
			args:         []string{"testflight", "beta-crash-logs"},
			wantShown:    []string{"asc testflight <subcommand> [flags]", "crashes"},
			wantNotShown: []string{"asc testflight beta-crash-logs <subcommand> [flags]"},
		},
		{
			name:      "top-level pre-release-versions removed",
			args:      []string{"pre-release-versions"},
			wantShown: []string{"Unknown command: pre-release-versions"},
		},
		{
			name:      "pre-release-versions list removed",
			args:      []string{"pre-release-versions", "list", "--app", "APP_ID"},
			wantShown: []string{"Unknown command: pre-release-versions"},
		},
		{
			name:      "pre-release-versions get removed",
			args:      []string{"pre-release-versions", "get", "--id", "PR_ID"},
			wantShown: []string{"Unknown command: pre-release-versions"},
		},
		{
			name:      "pre-release-versions app get removed",
			args:      []string{"pre-release-versions", "app", "get", "--id", "PR_ID"},
			wantShown: []string{"Unknown command: pre-release-versions"},
		},
		{
			name:      "pre-release-versions relationships get removed",
			args:      []string{"pre-release-versions", "relationships", "get", "--id", "PR_ID", "--type", "app"},
			wantShown: []string{"Unknown command: pre-release-versions"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("ASC_APP_ID", "")
			root := RootCommand("1.2.3")

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
			for _, want := range test.wantShown {
				if !strings.Contains(stderr, want) {
					t.Fatalf("expected help to contain %q, got %q", want, stderr)
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

func TestCanonicalTestFlightValidationPaths(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "distribution view missing build",
			args:    []string{"testflight", "distribution", "view"},
			wantErr: "--build-id is required",
		},
		{
			name:    "config export missing app",
			args:    []string{"testflight", "config", "export"},
			wantErr: "--app is required",
		},
		{
			name:    "app localizations get missing id",
			args:    []string{"testflight", "app-localizations", "view"},
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

func TestTestFlightFeedbackAndCrashesSendRequest(t *testing.T) {
	runTestFlightRequestCases(t, []testFlightRequestCase{
		{
			name:       "feedback view",
			args:       []string{"testflight", "feedback", "view", "--submission-id", "sub-2"},
			wantMethod: http.MethodGet,
			wantPath:   "/v1/betaFeedbackScreenshotSubmissions/sub-2",
			respBody:   `{"data":{"type":"betaFeedbackScreenshotSubmissions","id":"sub-2"}}`,
			wantStdout: []string{`"id":"sub-2"`},
		},
		{
			name:       "feedback list",
			args:       []string{"testflight", "feedback", "list", "--app", "123"},
			wantMethod: http.MethodGet,
			wantPath:   "/v1/apps/123/betaFeedbackScreenshotSubmissions",
			respBody:   `{"data":[{"type":"betaFeedbackScreenshotSubmissions","id":"feedback-1"}],"links":{"next":""}}`,
			wantStdout: []string{`"id":"feedback-1"`},
		},
		{
			name:       "feedback delete",
			args:       []string{"testflight", "feedback", "delete", "--submission-id", "sub-2", "--confirm"},
			wantMethod: http.MethodDelete,
			wantPath:   "/v1/betaFeedbackScreenshotSubmissions/sub-2",
			respStatus: http.StatusNoContent,
			wantStdout: []string{`"id":"sub-2"`, `"deleted":true`},
		},
		{
			name:       "crashes list",
			args:       []string{"testflight", "crashes", "list", "--app", "123"},
			wantMethod: http.MethodGet,
			wantPath:   "/v1/apps/123/betaFeedbackCrashSubmissions",
			respBody:   `{"data":[{"type":"betaFeedbackCrashSubmissions","id":"crash-1"}],"links":{"next":""}}`,
			wantStdout: []string{`"id":"crash-1"`},
		},
		{
			name:       "crashes view",
			args:       []string{"testflight", "crashes", "view", "--submission-id", "sub-1"},
			wantMethod: http.MethodGet,
			wantPath:   "/v1/betaFeedbackCrashSubmissions/sub-1",
			respBody:   `{"data":{"type":"betaFeedbackCrashSubmissions","id":"sub-1"}}`,
			wantStdout: []string{`"id":"sub-1"`},
		},
		{
			name:       "crashes delete",
			args:       []string{"testflight", "crashes", "delete", "--submission-id", "sub-1", "--confirm"},
			wantMethod: http.MethodDelete,
			wantPath:   "/v1/betaFeedbackCrashSubmissions/sub-1",
			respStatus: http.StatusNoContent,
			wantStdout: []string{`"id":"sub-1"`, `"deleted":true`},
		},
		{
			name:       "crashes log by submission id",
			args:       []string{"testflight", "crashes", "log", "--submission-id", "sub-1"},
			wantMethod: http.MethodGet,
			wantPath:   "/v1/betaFeedbackCrashSubmissions/sub-1/crashLog",
			respBody:   `{"data":{"type":"betaCrashLogs","id":"log-1"}}`,
			wantStdout: []string{`"id":"log-1"`},
		},
		{
			name:       "crashes log by crash log id",
			args:       []string{"testflight", "crashes", "log", "--crash-log-id", "log-1"},
			wantMethod: http.MethodGet,
			wantPath:   "/v1/betaCrashLogs/log-1",
			respBody:   `{"data":{"type":"betaCrashLogs","id":"log-1"}}`,
			wantStdout: []string{`"id":"log-1"`},
		},
		{
			name:       "app localizations list",
			args:       []string{"testflight", "app-localizations", "list", "--app", "123"},
			wantMethod: http.MethodGet,
			wantPath:   "/v1/betaAppLocalizations",
			wantQuery:  url.Values{"filter[app]": {"123"}},
			respBody:   `{"data":[{"type":"betaAppLocalizations","id":"loc-1"}],"links":{"next":""}}`,
			wantStdout: []string{`"id":"loc-1"`},
		},
		{
			name:       "app localizations create",
			args:       []string{"testflight", "app-localizations", "create", "--app", "app-1", "--locale", "en-US"},
			wantMethod: http.MethodPost,
			wantPath:   "/v1/betaAppLocalizations",
			wantBody:   `{"data":{"type":"betaAppLocalizations","attributes":{"locale":"en-US"},"relationships":{"app":{"data":{"type":"apps","id":"app-1"}}}}}`,
			respStatus: http.StatusCreated,
			respBody:   `{"data":{"type":"betaAppLocalizations","id":"loc-1","attributes":{"locale":"en-US"}}}`,
			wantStdout: []string{`"id":"loc-1"`},
		},
		{
			name:       "pre-release links view",
			args:       []string{"testflight", "pre-release", "links", "view", "--id", "pr-1", "--type", "app"},
			wantMethod: http.MethodGet,
			wantPath:   "/v1/preReleaseVersions/pr-1/relationships/app",
			respBody:   `{"data":{"type":"apps","id":"app-1"}}`,
			wantStdout: []string{`"id":"app-1"`},
		},
	})
}

func TestTestFlightCrashesLogRequiresExactlyOneLookupFlag(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		wantErr  string
		wantCode shared.DiagnosticCode
	}{
		{
			name:     "missing all lookup flags",
			args:     []string{"testflight", "crashes", "log"},
			wantErr:  "exactly one of --submission-id or --crash-log-id is required",
			wantCode: shared.DiagnosticRequiredInputMissing,
		},
		{
			name:     "both lookup flags",
			args:     []string{"testflight", "crashes", "log", "--submission-id", "sub-1", "--crash-log-id", "log-1"},
			wantErr:  "exactly one of --submission-id or --crash-log-id is required",
			wantCode: shared.DiagnosticConflictingInput,
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
			diagnostic, ok := shared.DiagnosticFromError(runErr)
			if !ok {
				t.Fatalf("expected structured diagnostic, got %v", runErr)
			}
			if diagnostic.Code != test.wantCode || diagnostic.Parameter != "" {
				t.Fatalf("diagnostic = %+v, want code %q with empty parameter", diagnostic, test.wantCode)
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
