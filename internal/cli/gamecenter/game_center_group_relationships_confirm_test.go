package gamecenter

import (
	"context"
	"errors"
	"flag"
	"strconv"
	"testing"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func TestGameCenterGroupRelationshipReplacementsRejectWithoutConfirmBeforeHTTP(t *testing.T) {
	isolateGameCenterAuthEnv(t)

	commands := map[string]func(*shared.Deps) *ffcli.Command{
		"achievements": newGameCenterGroupAchievementsSetCommand,
		"leaderboards": newGameCenterGroupLeaderboardsSetCommand,
	}

	for name, newCommand := range commands {
		for _, v2 := range []bool{false, true} {
			t.Run(name+" v2="+strconv.FormatBool(v2), func(t *testing.T) {
				factoryCalled := false
				d, stderr := newGameCenterTestDeps(&factoryCalled)

				cmd := newCommand(d)
				args := []string{"--group-id", "group-1", "--ids", "resource-1"}
				if v2 {
					args = append(args, "--v2")
				}
				if err := cmd.FlagSet.Parse(args); err != nil {
					t.Fatalf("parse flags: %v", err)
				}

				err := cmd.Exec(context.Background(), []string{})
				if !errors.Is(err, flag.ErrHelp) {
					t.Fatalf("replacement without --confirm must fail validation, got %v", err)
				}
				if diagnostic, ok := shared.DiagnosticFromError(err); !ok || diagnostic.Parameter != "--confirm" {
					t.Fatalf("diagnostic = %+v (ok=%t), want missing parameter %q", diagnostic, ok, "--confirm")
				}
				if stderr.String() != "Error: --confirm is required\n" {
					t.Fatalf("stderr = %q, want exact missing --confirm diagnostic", stderr.String())
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

	commands := map[string]func(*shared.Deps) *ffcli.Command{
		"achievements": newGameCenterGroupAchievementsSetCommand,
		"leaderboards": newGameCenterGroupLeaderboardsSetCommand,
	}

	for name, newCommand := range commands {
		for _, v2 := range []bool{false, true} {
			t.Run(name+" v2="+strconv.FormatBool(v2), func(t *testing.T) {
				d, _ := newGameCenterTestDeps(nil)
				cmd := newCommand(d)
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
		command func(*shared.Deps) *ffcli.Command
		args    []string
	}{
		"group achievements": {
			command: newGameCenterGroupAchievementsSetCommand,
			args:    []string{"--group-id", "group-1", "--ids", "achievement-1"},
		},
		"group leaderboards": {
			command: newGameCenterGroupLeaderboardsSetCommand,
			args:    []string{"--group-id", "group-1", "--ids", "leaderboard-1"},
		},
		"leaderboard-set members": {
			command: newGameCenterLeaderboardSetMembersSetCommand,
			args:    []string{"--set-id", "set-1", "--leaderboard-ids", "leaderboard-1"},
		},
		"leaderboard-set v2 members": {
			command: newGameCenterLeaderboardSetMembersV2SetCommand,
			args:    []string{"--set-id", "set-1", "--leaderboard-ids", "leaderboard-1"},
		},
	}

	for name, test := range commands {
		t.Run(name, func(t *testing.T) {
			args := append(append([]string{}, test.args...), "--confirm=false")
			d, errOut := newGameCenterTestDeps(nil)
			cmd := test.command(d)
			if err := cmd.FlagSet.Parse(args); err != nil {
				t.Fatalf("parse flags: %v", err)
			}
			err := cmd.Exec(context.Background(), nil)
			stderr := errOut.String()
			if !errors.Is(err, flag.ErrHelp) {
				t.Fatalf("explicit false --confirm should fail validation before auth, got %v", err)
			}
			if stderr != "Error: --confirm must be true when specified\n" {
				t.Fatalf("stderr = %q", stderr)
			}
		})
	}
}
