package cmdtest

import (
	"context"
	"errors"
	"flag"
	"io"
	"strings"
	"testing"
)

func runValidationTests(t *testing.T, tests []struct {
	name    string
	args    []string
	wantErr string
}) {
	t.Helper()

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(test.args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				err := root.Run(context.Background())
				if !errors.Is(err, flag.ErrHelp) {
					t.Fatalf("expected ErrHelp, got %v", err)
				}
			})

			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, test.wantErr) {
				t.Fatalf("expected error %q, got %q", test.wantErr, stderr)
			}
		})
	}
}

func TestBuildsParityValidationErrors(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "builds relationships missing type",
			args:    []string{"builds", "relationships", "get", "--build", "BUILD_ID"},
			wantErr: "--type is required",
		},
		{
			name:    "builds relationships missing build",
			args:    []string{"builds", "relationships", "get", "--type", "app"},
			wantErr: "--build is required",
		},
		{
			name:    "builds relationships invalid type",
			args:    []string{"builds", "relationships", "get", "--build", "BUILD_ID", "--type", "nope"},
			wantErr: "--type must be one of",
		},
		{
			name:    "builds relationships invalid limit for single",
			args:    []string{"builds", "relationships", "get", "--build", "BUILD_ID", "--type", "app", "--limit", "10"},
			wantErr: "only valid for to-many relationships",
		},
		{
			name:    "builds metrics beta-usages missing build",
			args:    []string{"builds", "metrics", "beta-usages"},
			wantErr: "--build is required",
		},
		{
			name:    "builds metrics beta-usages invalid limit",
			args:    []string{"builds", "metrics", "beta-usages", "--build", "BUILD_ID", "--limit", "300"},
			wantErr: "--limit must be between 1 and 200",
		},
		{
			name:    "builds individual-testers list missing build",
			args:    []string{"builds", "individual-testers", "list"},
			wantErr: "--build is required",
		},
		{
			name:    "builds individual-testers add missing build",
			args:    []string{"builds", "individual-testers", "add", "--tester", "TESTER_ID"},
			wantErr: "--build is required",
		},
		{
			name:    "builds individual-testers add missing tester",
			args:    []string{"builds", "individual-testers", "add", "--build", "BUILD_ID"},
			wantErr: "--tester is required",
		},
		{
			name:    "builds individual-testers remove missing build",
			args:    []string{"builds", "individual-testers", "remove", "--tester", "TESTER_ID"},
			wantErr: "--build is required",
		},
		{
			name:    "builds individual-testers remove missing tester",
			args:    []string{"builds", "individual-testers", "remove", "--build", "BUILD_ID"},
			wantErr: "--tester is required",
		},
		{
			name:    "builds uploads list missing app",
			args:    []string{"builds", "uploads", "list"},
			wantErr: "--app is required",
		},
		{
			name:    "builds uploads list invalid limit",
			args:    []string{"builds", "uploads", "list", "--app", "APP_ID", "--limit", "300"},
			wantErr: "--limit must be between 1 and 200",
		},
		{
			name:    "builds uploads list invalid sort",
			args:    []string{"builds", "uploads", "list", "--app", "APP_ID", "--sort", "nope"},
			wantErr: "--sort must be one of",
		},
		{
			name:    "builds uploads get missing id",
			args:    []string{"builds", "uploads", "get"},
			wantErr: "--id is required",
		},
		{
			name:    "builds uploads delete missing id",
			args:    []string{"builds", "uploads", "delete"},
			wantErr: "--id is required",
		},
		{
			name:    "builds uploads delete missing confirm",
			args:    []string{"builds", "uploads", "delete", "--id", "UPLOAD_ID"},
			wantErr: "--confirm is required",
		},
		{
			name:    "builds uploads files list missing upload",
			args:    []string{"builds", "uploads", "files", "list"},
			wantErr: "--upload is required",
		},
		{
			name:    "builds uploads files get missing id",
			args:    []string{"builds", "uploads", "files", "get"},
			wantErr: "--id is required",
		},
	}

	runValidationTests(t, tests)
}

func TestBetaLocalizationsValidationErrors(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "beta-app-localizations list missing app",
			args:    []string{"beta-app-localizations", "list"},
			wantErr: "--app is required",
		},
		{
			name:    "beta-app-localizations create missing app",
			args:    []string{"beta-app-localizations", "create", "--locale", "en-US"},
			wantErr: "--app is required",
		},
		{
			name:    "beta-app-localizations create missing locale",
			args:    []string{"beta-app-localizations", "create", "--app", "APP_ID"},
			wantErr: "--locale is required",
		},
		{
			name:    "beta-app-localizations update missing id",
			args:    []string{"beta-app-localizations", "update"},
			wantErr: "--id is required",
		},
		{
			name:    "beta-app-localizations update missing updates",
			args:    []string{"beta-app-localizations", "update", "--id", "LOC_ID"},
			wantErr: "at least one update flag is required",
		},
		{
			name:    "beta-app-localizations delete missing id",
			args:    []string{"beta-app-localizations", "delete"},
			wantErr: "--id is required",
		},
		{
			name:    "beta-app-localizations delete missing confirm",
			args:    []string{"beta-app-localizations", "delete", "--id", "LOC_ID"},
			wantErr: "--confirm is required",
		},
		{
			name:    "beta-build-localizations list missing build",
			args:    []string{"beta-build-localizations", "list"},
			wantErr: "--build is required",
		},
		{
			name:    "beta-build-localizations create missing build",
			args:    []string{"beta-build-localizations", "create", "--locale", "en-US", "--whats-new", "Notes"},
			wantErr: "--build is required",
		},
		{
			name:    "beta-build-localizations create missing locale",
			args:    []string{"beta-build-localizations", "create", "--build", "BUILD_ID", "--whats-new", "Notes"},
			wantErr: "--locale is required",
		},
		{
			name:    "beta-build-localizations create missing whats-new",
			args:    []string{"beta-build-localizations", "create", "--build", "BUILD_ID", "--locale", "en-US"},
			wantErr: "--whats-new is required",
		},
		{
			name:    "beta-build-localizations update missing id",
			args:    []string{"beta-build-localizations", "update"},
			wantErr: "--id is required",
		},
		{
			name:    "beta-build-localizations update missing whats-new",
			args:    []string{"beta-build-localizations", "update", "--id", "LOC_ID"},
			wantErr: "at least one update flag is required",
		},
		{
			name:    "beta-build-localizations delete missing id",
			args:    []string{"beta-build-localizations", "delete"},
			wantErr: "--id is required",
		},
		{
			name:    "beta-build-localizations delete missing confirm",
			args:    []string{"beta-build-localizations", "delete", "--id", "LOC_ID"},
			wantErr: "--confirm is required",
		},
	}

	runValidationTests(t, tests)
}

func TestTestFlightRelationshipsValidationErrors(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "beta-groups relationships missing type",
			args:    []string{"testflight", "beta-groups", "relationships", "get", "--group-id", "GROUP_ID"},
			wantErr: "--type is required",
		},
		{
			name:    "beta-groups relationships missing group-id",
			args:    []string{"testflight", "beta-groups", "relationships", "get", "--type", "betaTesters"},
			wantErr: "--group-id is required",
		},
		{
			name:    "beta-groups relationships invalid type",
			args:    []string{"testflight", "beta-groups", "relationships", "get", "--group-id", "GROUP_ID", "--type", "nope"},
			wantErr: "--type must be one of",
		},
		{
			name:    "beta-testers relationships missing type",
			args:    []string{"testflight", "beta-testers", "relationships", "get", "--tester-id", "TESTER_ID"},
			wantErr: "--type is required",
		},
		{
			name:    "beta-testers relationships missing tester-id",
			args:    []string{"testflight", "beta-testers", "relationships", "get", "--type", "apps"},
			wantErr: "--tester-id is required",
		},
		{
			name:    "beta-testers relationships invalid type",
			args:    []string{"testflight", "beta-testers", "relationships", "get", "--tester-id", "TESTER_ID", "--type", "nope"},
			wantErr: "--type must be one of",
		},
		{
			name:    "beta-testers metrics missing tester-id",
			args:    []string{"testflight", "beta-testers", "metrics", "--app", "APP_ID"},
			wantErr: "--tester-id is required",
		},
		{
			name:    "beta-testers metrics missing app",
			args:    []string{"testflight", "beta-testers", "metrics", "--tester-id", "TESTER_ID"},
			wantErr: "--app is required",
		},
		{
			name:    "beta-testers metrics invalid period",
			args:    []string{"testflight", "beta-testers", "metrics", "--tester-id", "TESTER_ID", "--app", "APP_ID", "--period", "P1D"},
			wantErr: "--period must be one of",
		},
		{
			name:    "beta-testers metrics invalid limit",
			args:    []string{"testflight", "beta-testers", "metrics", "--tester-id", "TESTER_ID", "--app", "APP_ID", "--limit", "500"},
			wantErr: "--limit must be between 1 and 200",
		},
	}

	runValidationTests(t, tests)
}

func TestPreReleaseRelationshipsValidationErrors(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "pre-release relationships missing type",
			args:    []string{"pre-release-versions", "relationships", "get", "--id", "PR_ID"},
			wantErr: "--type is required",
		},
		{
			name:    "pre-release relationships missing id",
			args:    []string{"pre-release-versions", "relationships", "get", "--type", "app"},
			wantErr: "--id is required",
		},
		{
			name:    "pre-release relationships invalid type",
			args:    []string{"pre-release-versions", "relationships", "get", "--id", "PR_ID", "--type", "nope"},
			wantErr: "--type must be one of",
		},
		{
			name:    "pre-release relationships invalid limit for single",
			args:    []string{"pre-release-versions", "relationships", "get", "--id", "PR_ID", "--type", "app", "--limit", "10"},
			wantErr: "only valid for to-many relationships",
		},
	}

	runValidationTests(t, tests)
}
