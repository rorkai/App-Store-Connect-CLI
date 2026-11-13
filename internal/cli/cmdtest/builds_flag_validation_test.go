package cmdtest

import (
	"errors"
	"net/http"
	"path/filepath"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
)

func TestBuildsInvalidInputFailsBeforeNetwork(t *testing.T) {
	const (
		processingStateErr = "Error: --processing-state must be one of VALID, PROCESSING, FAILED, INVALID, or all"
		betaReviewStateErr = "Error: --beta-review-state must be a comma-separated list of: WAITING_FOR_REVIEW, IN_REVIEW, REJECTED, APPROVED"
		includeErr         = "Error: --include must be a comma-separated list of: preReleaseVersion, individualTesters, betaGroups, betaBuildLocalizations, appEncryptionDeclaration, betaAppReviewSubmission, app, buildBetaDetail, appStoreVersion, icons, buildBundles, buildUpload"
	)

	tests := []struct {
		name     string
		args     []string
		wantExit int
		wantErr  string
	}{
		{
			name:     "test-notes update invalid locale",
			args:     []string{"builds", "test-notes", "update", "--build-id", "build-1", "--locale", "!!!bad!!!", "--whats-new", "test"},
			wantExit: rootcmd.ExitError,
			wantErr:  `Error: builds test-notes update: invalid locale "!!!bad!!!": must match pattern like en or en-US`,
		},
		{
			name:     "list invalid limit",
			args:     []string{"builds", "list", "--app", "123456789", "--limit", "999"},
			wantExit: rootcmd.ExitUsage,
			wantErr:  "Error: builds: --limit must be between 1 and 200",
		},
		{
			name:     "wait build-id with since",
			args:     []string{"builds", "wait", "--build-id", "build-1", "--since", "2026-03-02T18:00:00Z"},
			wantExit: rootcmd.ExitUsage,
			wantErr:  "Error: --build-id is mutually exclusive with app-scoped selectors (--app, --latest, --version, --build-number, --platform, --since)",
		},
		{
			name:     "list invalid platform",
			args:     []string{"builds", "list", "--app", "123456789", "--platform", "ANDROID"},
			wantExit: rootcmd.ExitUsage,
			wantErr:  "Error: --platform must be one of: IOS, MAC_OS, TV_OS, VISION_OS",
		},
		{
			name:     "list invalid processing state",
			args:     []string{"builds", "list", "--app", "123456789", "--processing-state", "WRONG"},
			wantExit: rootcmd.ExitUsage,
			wantErr:  processingStateErr,
		},
		{
			name:     "info latest invalid processing state",
			args:     []string{"builds", "info", "--app", "123456789", "--latest", "--processing-state", "WRONG"},
			wantExit: rootcmd.ExitUsage,
			wantErr:  processingStateErr,
		},
		{
			name:     "update invalid uses-non-exempt-encryption",
			args:     []string{"builds", "update", "--build-id", "build-99", "--uses-non-exempt-encryption=maybe"},
			wantExit: rootcmd.ExitUsage,
			wantErr:  "Error: --uses-non-exempt-encryption must be 'true' or 'false'",
		},
		{
			name:     "list unknown beta review state",
			args:     []string{"builds", "list", "--app", "123456789", "--beta-review-state", "WAITING_FOR_REVIEW,PENDING"},
			wantExit: rootcmd.ExitUsage,
			wantErr:  betaReviewStateErr,
		},
		{
			name:     "list blank beta review state",
			args:     []string{"builds", "list", "--app", "123456789", "--beta-review-state", ","},
			wantExit: rootcmd.ExitUsage,
			wantErr:  betaReviewStateErr,
		},
		{
			name:     "count blank beta review state",
			args:     []string{"builds", "count", "--app", "123456789", "--beta-review-state", ","},
			wantExit: rootcmd.ExitUsage,
			wantErr:  betaReviewStateErr,
		},
		{
			name:     "count unknown beta review state",
			args:     []string{"builds", "count", "--app", "123456789", "--beta-review-state", "PENDING"},
			wantExit: rootcmd.ExitUsage,
			wantErr:  betaReviewStateErr,
		},
		{
			name:     "list unknown include",
			args:     []string{"builds", "list", "--app", "123456789", "--include", "app,buildBundle"},
			wantExit: rootcmd.ExitUsage,
			wantErr:  includeErr + ` (unknown value "buildBundle")`,
		},
		{
			name:     "list blank include",
			args:     []string{"builds", "list", "--app", "123456789", "--include", ","},
			wantExit: rootcmd.ExitUsage,
			wantErr:  includeErr,
		},
		{
			name:     "list unknown sort key",
			args:     []string{"builds", "list", "--app", "123456789", "--sort", "expirationDate"},
			wantExit: rootcmd.ExitUsage,
			wantErr:  "Error: builds: --sort must be one of: version, -version, uploadedDate, -uploadedDate, preReleaseVersion, -preReleaseVersion",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
			t.Setenv("ASC_APP_ID", "")
			installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
				t.Errorf("unexpected HTTP request: %s %s", req.Method, req.URL.String())
				return nil, errors.New("unexpected HTTP request")
			}))

			assertExitStderr(t, test.args, test.wantExit, test.wantErr)
		})
	}
}
