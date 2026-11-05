package gamecenter

import (
	"context"
	"errors"
	"flag"
	"strconv"
	"testing"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func TestGameCenterGroupRelationshipReplacementsRejectWithoutConfirmBeforeHTTP(t *testing.T) {
	isolateGameCenterAuthEnv(t)

	commands := map[string]func() *ffcli.Command{
		"achievements": GameCenterGroupAchievementsSetCommand,
		"leaderboards": GameCenterGroupLeaderboardsSetCommand,
	}

	for name, newCommand := range commands {
		for _, v2 := range []bool{false, true} {
			t.Run(name+" v2="+strconv.FormatBool(v2), func(t *testing.T) {
				factoryCalled := false
				restore := shared.SetASCClientFactoryForTesting(func() (*asc.Client, error) {
					factoryCalled = true
					return nil, errors.New("poison client factory called")
				})
				t.Cleanup(restore)

				cmd := newCommand()
				args := []string{"--group-id", "group-1", "--ids", "resource-1"}
				if v2 {
					args = append(args, "--v2")
				}
				if err := cmd.FlagSet.Parse(args); err != nil {
					t.Fatalf("parse flags: %v", err)
				}

				var err error
				stderr := captureGameCenterStderr(t, func() {
					err = cmd.Exec(context.Background(), []string{})
				})
				if !errors.Is(err, flag.ErrHelp) {
					t.Fatalf("replacement without --confirm must fail validation, got %v", err)
				}
				if err.Error() != "--confirm" {
					t.Fatalf("error = %q, want missing parameter %q", err.Error(), "--confirm")
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

func TestGameCenterGroupRelationshipReplacementConfirmPassesValidation(t *testing.T) {
	isolateGameCenterAuthEnv(t)

	commands := map[string]func() *ffcli.Command{
		"achievements": GameCenterGroupAchievementsSetCommand,
		"leaderboards": GameCenterGroupLeaderboardsSetCommand,
	}

	for name, newCommand := range commands {
		for _, v2 := range []bool{false, true} {
			t.Run(name+" v2="+strconv.FormatBool(v2), func(t *testing.T) {
				cmd := newCommand()
				args := []string{"--group-id", "group-1", "--ids", "resource-1", "--confirm"}
				if v2 {
					args = append(args, "--v2")
				}
				if err := cmd.FlagSet.Parse(args); err != nil {
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

func TestGameCenterRelationshipReplacementRejectsExplicitFalseConfirm(t *testing.T) {
	isolateGameCenterAuthEnv(t)
	commands := map[string]struct {
		command func() *ffcli.Command
		args    []string
	}{
		"group achievements": {
			command: GameCenterGroupAchievementsSetCommand,
			args:    []string{"--group-id", "group-1", "--ids", "achievement-1"},
		},
		"group leaderboards": {
			command: GameCenterGroupLeaderboardsSetCommand,
			args:    []string{"--group-id", "group-1", "--ids", "leaderboard-1"},
		},
		"leaderboard-set members": {
			command: GameCenterLeaderboardSetMembersSetCommand,
			args:    []string{"--set-id", "set-1", "--leaderboard-ids", "leaderboard-1"},
		},
		"leaderboard-set v2 members": {
			command: GameCenterLeaderboardSetMembersV2SetCommand,
			args:    []string{"--set-id", "set-1", "--leaderboard-ids", "leaderboard-1"},
		},
	}

	for name, test := range commands {
		t.Run(name, func(t *testing.T) {
			cmd := test.command()
			args := append(append([]string{}, test.args...), "--confirm=false")
			if err := cmd.FlagSet.Parse(args); err != nil {
				t.Fatalf("parse flags: %v", err)
			}
			var err error
			stderr := captureGameCenterStderr(t, func() {
				err = cmd.Exec(context.Background(), nil)
			})
			if !errors.Is(err, flag.ErrHelp) {
				t.Fatalf("explicit false --confirm should fail validation before auth, got %v", err)
			}
			if stderr != "Error: --confirm must be true when specified\n" {
				t.Fatalf("stderr = %q", stderr)
			}
		})
	}
}
