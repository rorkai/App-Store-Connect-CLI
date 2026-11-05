package cmdtest

import (
	"errors"
	"strings"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// TestGameCenterRelationshipReplacementRequiresConfirm locks the 5.0.0
// contract: replacing Game Center group or leaderboard-set relationships
// without --confirm is a usage error before authentication or HTTP, not a
// warning that continues.
func TestGameCenterRelationshipReplacementRequiresConfirm(t *testing.T) {
	setupUsageExitCodeEnv(t)

	tests := []struct {
		name string
		args []string
	}{
		{
			name: "groups achievements set",
			args: []string{"game-center", "groups", "achievements", "set", "--group-id", "group-1", "--ids", "achievement-1"},
		},
		{
			name: "groups leaderboards set",
			args: []string{"game-center", "groups", "leaderboards", "set", "--group-id", "group-1", "--ids", "leaderboard-1"},
		},
		{
			name: "leaderboard-sets members set",
			args: []string{"game-center", "leaderboard-sets", "members", "set", "--set-id", "set-1", "--leaderboard-ids", "leaderboard-1"},
		},
		{
			name: "leaderboard-sets v2 members set",
			args: []string{"game-center", "leaderboard-sets", "v2", "members", "set", "--set-id", "set-1", "--leaderboard-ids", "leaderboard-1"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			factoryCalled := false
			restore := shared.SetASCClientFactoryForTesting(func() (*asc.Client, error) {
				factoryCalled = true
				return nil, errors.New("poison client factory called")
			})
			t.Cleanup(restore)

			var code int
			stdout, stderr := captureOutput(t, func() {
				code = rootcmd.Run(test.args, "5.0.0")
			})

			if code != rootcmd.ExitUsage {
				t.Fatalf("exit code = %d, want %d", code, rootcmd.ExitUsage)
			}
			if stdout != "" {
				t.Fatalf("stdout = %q, want empty", stdout)
			}
			assertUsageDiagnosticFirstLine(t, stderr, "--confirm is required")
			if strings.Contains(stderr, "Warning:") {
				t.Fatalf("stderr = %q, must not carry the retired compatibility warning", stderr)
			}
			if factoryCalled {
				t.Fatal("client factory called before --confirm validation")
			}
		})
	}
}

// TestGameCenterRemovedFlagsAreUnknown locks the 5.0.0 removal of flags that
// 4.x kept registered only to return migration guidance: the pagination flags
// on the singleton `details list` and --challenge-enabled on `details
// create|update`. None of them is registered any more, so the failure carries
// the migration step and makes no HTTP request.
func TestGameCenterRemovedFlagsAreUnknown(t *testing.T) {
	setupUsageExitCodeEnv(t)

	tests := []struct {
		name     string
		command  string
		args     []string
		flag     string
		guidance string
	}{
		{name: "details list --limit", command: "asc game-center details list", args: []string{"game-center", "details", "list", "--app", "app-1", "--limit", "5"}, flag: "--limit", guidance: "each app has a single Game Center detail"},
		{name: "details list --next", command: "asc game-center details list", args: []string{"game-center", "details", "list", "--app", "app-1", "--next", "https://api.appstoreconnect.apple.com/v1/gameCenterDetails?cursor=x"}, flag: "--next", guidance: "each app has a single Game Center detail"},
		{name: "details list --paginate", command: "asc game-center details list", args: []string{"game-center", "details", "list", "--app", "app-1", "--paginate"}, flag: "--paginate", guidance: "each app has a single Game Center detail"},
		{name: "details create --challenge-enabled", command: "asc game-center details create", args: []string{"game-center", "details", "create", "--app", "app-1", "--challenge-enabled", "true"}, flag: "--challenge-enabled", guidance: "App Store Connect no longer accepts `challengeEnabled`"},
		{name: "details update --challenge-enabled", command: "asc game-center details update", args: []string{"game-center", "details", "update", "--id", "detail-1", "--challenge-enabled", "true"}, flag: "--challenge-enabled", guidance: "App Store Connect no longer accepts `challengeEnabled`"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			factoryCalled := false
			restore := shared.SetASCClientFactoryForTesting(func() (*asc.Client, error) {
				factoryCalled = true
				return nil, errors.New("poison client factory called")
			})
			t.Cleanup(restore)

			var code int
			stdout, stderr := captureOutput(t, func() {
				code = rootcmd.Run(test.args, "5.0.0")
			})

			if code != rootcmd.ExitUsage {
				t.Fatalf("exit code = %d, want %d", code, rootcmd.ExitUsage)
			}
			if stdout != "" {
				t.Fatalf("stdout = %q, want empty", stdout)
			}
			want := "Error: `" + test.flag + "` was removed in 5.0.0; " + test.guidance + " (see migrate-to-5-0)\nFor help:\n  " + test.command + " --help\n"
			if stderr != want {
				t.Fatalf("stderr = %q, want removed-flag guidance %q", stderr, want)
			}
			if factoryCalled {
				t.Fatalf("client factory called for removed %s", test.flag)
			}
		})
	}
}

// TestGameCenterGroupChallengesSetIsRemoved locks the 5.0.0 removal of the
// unsupported `groups challenges set` stub. App Store Connect exposes group
// challenge relationships read-only, so the verb is gone rather than a
// registered command that always failed.
func TestGameCenterGroupChallengesSetIsRemoved(t *testing.T) {
	setupUsageExitCodeEnv(t)

	root := RootCommand("5.0.0")
	if command := findSubcommand(root, "game-center", "groups", "challenges", "set"); command != nil {
		t.Fatal("removed command `asc game-center groups challenges set` is still registered")
	}
	if findSubcommand(root, "game-center", "groups", "challenges", "list") == nil {
		t.Fatal("`asc game-center groups challenges list` must stay registered")
	}

	factoryCalled := false
	restore := shared.SetASCClientFactoryForTesting(func() (*asc.Client, error) {
		factoryCalled = true
		return nil, errors.New("poison client factory called")
	})
	t.Cleanup(restore)

	var code int
	stdout, stderr := captureOutput(t, func() {
		code = rootcmd.Run([]string{"game-center", "groups", "challenges", "set", "--group-id", "group-1", "--ids", "challenge-1"}, "5.0.0")
	})

	if code != rootcmd.ExitUsage {
		t.Fatalf("exit code = %d, want %d", code, rootcmd.ExitUsage)
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	if !strings.HasPrefix(stderr, "Error: unknown command `asc game-center groups challenges set`\n") {
		t.Fatalf("stderr = %q, want generic unknown-command failure", stderr)
	}
	if strings.Contains(strings.ToLower(stderr), "deprecated") {
		t.Fatalf("stderr = %q, must not carry retired deprecation guidance", stderr)
	}
	if factoryCalled {
		t.Fatal("client factory called for removed command")
	}
}
