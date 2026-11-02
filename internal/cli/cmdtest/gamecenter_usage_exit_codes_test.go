package cmdtest

import (
	"path/filepath"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
)

// TestGameCenterInputValidationReturnsUsageExitCode locks the usage-error
// contract for Game Center flag validation: every pre-request flag check must
// print "Error: <message>" to stderr and exit with code 2, not the generic
// runtime failure code.
//
// The table covers both the per-command checks and the two shared metrics
// helpers (runDetailsMetrics and runMetricsQueue), which format the command
// path into the diagnostic instead of hard-coding it.
func TestGameCenterInputValidationReturnsUsageExitCode(t *testing.T) {
	setupUsageExitCodeEnv(t)

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "achievements list limit above maximum",
			args:    []string{"game-center", "achievements", "list", "--app", "123456789", "--limit", "201"},
			wantErr: "game-center achievements list: --limit must be between 1 and 200",
		},
		{
			name:    "achievements list limit below minimum",
			args:    []string{"game-center", "achievements", "list", "--app", "123456789", "--limit", "-1"},
			wantErr: "game-center achievements list: --limit must be between 1 and 200",
		},
		{
			name:    "achievements list non-App-Store-Connect next",
			args:    []string{"game-center", "achievements", "list", "--next", "http://api.appstoreconnect.apple.com/v1/gameCenterAchievements"},
			wantErr: "game-center achievements list: --next must be an App Store Connect URL",
		},
		{
			name:    "achievements list malformed next",
			args:    []string{"game-center", "achievements", "list", "--next", malformedNextURL},
			wantErr: "game-center achievements list: --next must be a valid URL: " + malformedNextURLParseError,
		},
		{
			name:    "groups list limit above maximum",
			args:    []string{"game-center", "groups", "list", "--limit", "201"},
			wantErr: "game-center groups list: --limit must be between 1 and 200",
		},
		{
			name:    "leaderboard-sets v2 list limit above maximum",
			args:    []string{"game-center", "leaderboard-sets", "v2", "list", "--limit", "201"},
			wantErr: "game-center leaderboard-sets v2 list: --limit must be between 1 and 200",
		},
		{
			name:    "enabled-versions compatible-versions limit above maximum",
			args:    []string{"game-center", "enabled-versions", "compatible-versions", "--limit", "201"},
			wantErr: "game-center enabled-versions compatible-versions: --limit must be between 1 and 200",
		},
		{
			name:    "app-versions compatibility list invalid next",
			args:    []string{"game-center", "app-versions", "compatibility", "list", "--next", "http://api.appstoreconnect.apple.com/v1/x"},
			wantErr: "game-center app-versions compatibility list: --next must be an App Store Connect URL",
		},
		{
			name:    "details metrics limit above maximum",
			args:    []string{"game-center", "details", "metrics", "classic-matchmaking", "--limit", "201"},
			wantErr: "game-center details metrics classic-matchmaking: --limit must be between 1 and 200",
		},
		{
			name:    "details metrics invalid next",
			args:    []string{"game-center", "details", "metrics", "classic-matchmaking", "--next", "http://api.appstoreconnect.apple.com/v1/x"},
			wantErr: "game-center details metrics classic-matchmaking: --next must be an App Store Connect URL",
		},
		{
			name:    "matchmaking metrics limit above maximum",
			args:    []string{"game-center", "matchmaking", "metrics", "queue-sizes", "--limit", "201"},
			wantErr: "game-center matchmaking metrics queue-sizes: --limit must be between 1 and 200",
		},
		{
			name:    "matchmaking metrics invalid next",
			args:    []string{"game-center", "matchmaking", "metrics", "queue-sizes", "--next", "http://api.appstoreconnect.apple.com/v1/x"},
			wantErr: "game-center matchmaking metrics queue-sizes: --next must be an App Store Connect URL",
		},
		{
			name:    "matchmaking queues list limit above maximum",
			args:    []string{"game-center", "matchmaking", "queues", "list", "--limit", "201"},
			wantErr: "game-center matchmaking queues list: --limit must be between 1 and 200",
		},
		{
			name:    "challenges create v2 without leaderboard",
			args:    []string{"game-center", "challenges", "create", "--app", "123456789", "--reference-name", "Weekly", "--vendor-id", "com.example.weekly", "--v2"},
			wantErr: "--v2 requires --leaderboard-id",
		},
		{
			name:    "challenges update v2 without leaderboard",
			args:    []string{"game-center", "challenges", "update", "--id", "CHALLENGE_ID", "--archived", "true", "--v2"},
			wantErr: "--v2 requires --leaderboard-id",
		},
		{
			name:    "achievements view without id",
			args:    []string{"game-center", "achievements", "view"},
			wantErr: "--id is required",
		},
		{
			name:    "achievements group-achievement view without id",
			args:    []string{"game-center", "achievements", "group-achievement", "view"},
			wantErr: "--id is required",
		},
		{
			name:    "achievements v2 versions view without id",
			args:    []string{"game-center", "achievements", "v2", "versions", "view"},
			wantErr: "--id is required",
		},
		{
			name:    "achievements v2 localizations view without id",
			args:    []string{"game-center", "achievements", "v2", "localizations", "view"},
			wantErr: "--id is required",
		},
		{
			name:    "achievements localizations view without id",
			args:    []string{"game-center", "achievements", "localizations", "view"},
			wantErr: "--id is required",
		},
		{
			name:    "achievements localizations image view without id",
			args:    []string{"game-center", "achievements", "localizations", "image", "view"},
			wantErr: "--id is required",
		},
		{
			name:    "achievements localizations achievement view without id",
			args:    []string{"game-center", "achievements", "localizations", "achievement", "view"},
			wantErr: "--id is required",
		},
		{
			name:    "achievements images view without id",
			args:    []string{"game-center", "achievements", "images", "view"},
			wantErr: "--id is required",
		},
		{
			name:    "leaderboards view without id",
			args:    []string{"game-center", "leaderboards", "view"},
			wantErr: "--id is required",
		},
		{
			name:    "leaderboards group-leaderboard view without id",
			args:    []string{"game-center", "leaderboards", "group-leaderboard", "view"},
			wantErr: "--id is required",
		},
		{
			name:    "leaderboards v2 versions view without id",
			args:    []string{"game-center", "leaderboards", "v2", "versions", "view"},
			wantErr: "--id is required",
		},
		{
			name:    "leaderboards v2 localizations view without id",
			args:    []string{"game-center", "leaderboards", "v2", "localizations", "view"},
			wantErr: "--id is required",
		},
		{
			name:    "leaderboards localizations view without id",
			args:    []string{"game-center", "leaderboards", "localizations", "view"},
			wantErr: "--id is required",
		},
		{
			name:    "leaderboards localizations image view without id",
			args:    []string{"game-center", "leaderboards", "localizations", "image", "view"},
			wantErr: "--id is required",
		},
		{
			name:    "leaderboard-sets view without id",
			args:    []string{"game-center", "leaderboard-sets", "view"},
			wantErr: "--id is required",
		},
		{
			name:    "leaderboard-sets group-leaderboard-set view without id",
			args:    []string{"game-center", "leaderboard-sets", "group-leaderboard-set", "view"},
			wantErr: "--id is required",
		},
		{
			name:    "leaderboard-sets v2 view without id",
			args:    []string{"game-center", "leaderboard-sets", "v2", "view"},
			wantErr: "--id is required",
		},
		{
			name:    "leaderboard-sets v2 versions view without id",
			args:    []string{"game-center", "leaderboard-sets", "v2", "versions", "view"},
			wantErr: "--id is required",
		},
		{
			name:    "leaderboard-sets v2 localizations view without id",
			args:    []string{"game-center", "leaderboard-sets", "v2", "localizations", "view"},
			wantErr: "--id is required",
		},
		{
			name:    "leaderboard-sets member-localizations view without id",
			args:    []string{"game-center", "leaderboard-sets", "member-localizations", "view"},
			wantErr: "--id is required",
		},
		{
			name:    "leaderboard-sets member-localizations leaderboard view without id",
			args:    []string{"game-center", "leaderboard-sets", "member-localizations", "leaderboard", "view"},
			wantErr: "--id is required",
		},
		{
			name:    "leaderboard-sets member-localizations leaderboard-set view without id",
			args:    []string{"game-center", "leaderboard-sets", "member-localizations", "leaderboard-set", "view"},
			wantErr: "--id is required",
		},
		{
			name:    "leaderboard-sets localizations view without id",
			args:    []string{"game-center", "leaderboard-sets", "localizations", "view"},
			wantErr: "--id is required",
		},
		{
			name:    "leaderboard-sets localizations image view without id",
			args:    []string{"game-center", "leaderboard-sets", "localizations", "image", "view"},
			wantErr: "--id is required",
		},
		{
			name:    "challenges view without id",
			args:    []string{"game-center", "challenges", "view"},
			wantErr: "--id is required",
		},
		{
			name:    "challenges versions view without id",
			args:    []string{"game-center", "challenges", "versions", "view"},
			wantErr: "--id is required",
		},
		{
			name:    "challenges versions default-image view without id",
			args:    []string{"game-center", "challenges", "versions", "default-image", "view"},
			wantErr: "--id is required",
		},
		{
			name:    "challenges localizations view without id",
			args:    []string{"game-center", "challenges", "localizations", "view"},
			wantErr: "--id is required",
		},
		{
			name:    "challenges localizations image view without id",
			args:    []string{"game-center", "challenges", "localizations", "image", "view"},
			wantErr: "--id is required",
		},
		{
			name:    "challenges images view without id",
			args:    []string{"game-center", "challenges", "images", "view"},
			wantErr: "--id is required",
		},
		{
			name:    "activities view without id",
			args:    []string{"game-center", "activities", "view"},
			wantErr: "--id is required",
		},
		{
			name:    "activities versions view without id",
			args:    []string{"game-center", "activities", "versions", "view"},
			wantErr: "--id is required",
		},
		{
			name:    "activities versions default-image view without id",
			args:    []string{"game-center", "activities", "versions", "default-image", "view"},
			wantErr: "--id is required",
		},
		{
			name:    "activities localizations view without id",
			args:    []string{"game-center", "activities", "localizations", "view"},
			wantErr: "--id is required",
		},
		{
			name:    "activities localizations image view without id",
			args:    []string{"game-center", "activities", "localizations", "image", "view"},
			wantErr: "--id is required",
		},
		{
			name:    "activities images view without id",
			args:    []string{"game-center", "activities", "images", "view"},
			wantErr: "--id is required",
		},
		{
			name:    "matchmaking queues view without id",
			args:    []string{"game-center", "matchmaking", "queues", "view"},
			wantErr: "--id is required",
		},
		{
			name:    "matchmaking rule-sets view without id",
			args:    []string{"game-center", "matchmaking", "rule-sets", "view"},
			wantErr: "--id is required",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assertUsageExitCode(t, test.args, test.wantErr)
		})
	}
}

// setupUsageExitCodeEnv isolates auth and app-id state so a validation failure
// cannot be masked by a credential lookup or an ambient ASC_APP_ID.
func setupUsageExitCodeEnv(t *testing.T) {
	t.Helper()

	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")
}

// malformedNextURL and malformedNextURLParseError spell out the url.Parse
// diagnostic shared.ValidateNextURL wraps. The message is written out rather
// than recomputed because staticcheck rejects url.Parse on a constant invalid
// URL (SA1007).
const (
	malformedNextURL           = "https://api.appstoreconnect.apple.com/%zz"
	malformedNextURLParseError = `parse "` + malformedNextURL + `": invalid URL escape "%zz"`
)

// assertUsageExitCode runs one invalid invocation and asserts the full usage
// contract: a usage-class error, exit code 2, no stdout, and exactly one
// "Error: <message>" diagnostic on stderr.
func assertUsageExitCode(t *testing.T, args []string, wantErr string) {
	t.Helper()

	stdout, stderr, runErr := runCommand(t, args)

	if runErr == nil {
		t.Fatal("expected error, got nil")
	}
	if got := rootcmd.ExitCodeFromError(runErr); got != rootcmd.ExitUsage {
		t.Fatalf("exit code = %d, want %d (err=%v)", got, rootcmd.ExitUsage, runErr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	assertUsageErrorStderr(t, stderr, wantErr)
}
