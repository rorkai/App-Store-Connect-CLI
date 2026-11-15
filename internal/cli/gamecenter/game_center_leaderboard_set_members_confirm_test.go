package gamecenter

import (
	"context"
	"errors"
	"flag"
	"testing"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func TestGameCenterLeaderboardSetMembersReplacementRejectsWithoutConfirmBeforeHTTP(t *testing.T) {
	isolateGameCenterAuthEnv(t)

	commands := map[string]func() *ffcli.Command{
		"v1": GameCenterLeaderboardSetMembersSetCommand,
		"v2": GameCenterLeaderboardSetMembersV2SetCommand,
	}

	for name, newCommand := range commands {
		for _, leaderboardIDs := range []string{"leaderboard-1", ""} {
			t.Run(name+" ids="+leaderboardIDs, func(t *testing.T) {
				factoryCalled := false
				restore := shared.SetASCClientFactoryForTesting(func() (*asc.Client, error) {
					factoryCalled = true
					return nil, errors.New("poison client factory called")
				})
				t.Cleanup(restore)

				cmd := newCommand()
				if err := cmd.FlagSet.Parse([]string{
					"--set-id", "set-1", "--leaderboard-ids", leaderboardIDs,
				}); err != nil {
					t.Fatalf("parse flags: %v", err)
				}

				var err error
				stderr := captureGameCenterStderr(t, func() {
					err = cmd.Exec(context.Background(), []string{})
				})
				if !errors.Is(err, flag.ErrHelp) {
					t.Fatalf("replacement without --confirm must fail validation, got %v", err)
				}
				if diagnostic, ok := shared.DiagnosticFromError(err); !ok || diagnostic.Parameter != "--confirm" {
					t.Fatalf("diagnostic = %+v (ok=%t), want missing parameter %q", diagnostic, ok, "--confirm")
				}
				if stderr != "Error: --confirm is required\n" {
					t.Fatalf("stderr = %q, want exact missing --confirm diagnostic", stderr)
				}
				if factoryCalled {
					t.Fatal("client factory called before --confirm validation")
				}
			})
		}
	}
}

func TestGameCenterLeaderboardSetMembersRequiresLeaderboardIDs(t *testing.T) {
	isolateGameCenterAuthEnv(t)

	commands := map[string]func() *ffcli.Command{
		"v1": GameCenterLeaderboardSetMembersSetCommand,
		"v2": GameCenterLeaderboardSetMembersV2SetCommand,
	}

	for name, newCommand := range commands {
		t.Run(name, func(t *testing.T) {
			cmd := newCommand()
			if err := cmd.FlagSet.Parse([]string{
				"--set-id", "set-1", "--confirm",
			}); err != nil {
				t.Fatalf("parse flags: %v", err)
			}

			var err error
			stderr := captureGameCenterStderr(t, func() {
				err = cmd.Exec(context.Background(), []string{})
			})
			if !errors.Is(err, flag.ErrHelp) {
				t.Fatalf("missing --leaderboard-ids should fail validation before auth, got %v", err)
			}
			if err.Error() != "--leaderboard-ids" {
				t.Fatalf("error = %q, want missing parameter %q", err.Error(), "--leaderboard-ids")
			}
			if stderr != "Error: --leaderboard-ids is required\n" {
				t.Fatalf("stderr = %q, want exact missing-ID diagnostic", stderr)
			}
		})
	}
}

func TestGameCenterLeaderboardSetMembersConfirmPassesValidation(t *testing.T) {
	isolateGameCenterAuthEnv(t)

	commands := map[string]func() *ffcli.Command{
		"v1": GameCenterLeaderboardSetMembersSetCommand,
		"v2": GameCenterLeaderboardSetMembersV2SetCommand,
	}

	for name, newCommand := range commands {
		for _, leaderboardIDs := range []string{"leaderboard-1", ""} {
			t.Run(name+" ids="+leaderboardIDs, func(t *testing.T) {
				cmd := newCommand()
				if err := cmd.FlagSet.Parse([]string{
					"--set-id", "set-1", "--leaderboard-ids", leaderboardIDs, "--confirm",
				}); err != nil {
					t.Fatalf("parse flags: %v", err)
				}

				err := cmd.Exec(context.Background(), []string{})
				if errors.Is(err, flag.ErrHelp) {
					t.Fatalf("replacement with --confirm should pass validation before auth, got %v", err)
				}
			})
		}
	}
}
