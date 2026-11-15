package gamecenter

import (
	"context"
	"errors"
	"flag"
	"path/filepath"
	"testing"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func isolateGameCenterAuthEnv(t *testing.T) {
	t.Helper()
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_PROFILE", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_KEY_ID", "")
	t.Setenv("ASC_ISSUER_ID", "")
	t.Setenv("ASC_PRIVATE_KEY_PATH", "")
	t.Setenv("ASC_PRIVATE_KEY", "")
	t.Setenv("ASC_PRIVATE_KEY_B64", "")
	t.Setenv("ASC_STRICT_AUTH", "")
}

func TestGameCenterActivitySetCommandsRequireConfirmForRemove(t *testing.T) {
	isolateGameCenterAuthEnv(t)

	commands := map[string]func(*shared.Deps) *ffcli.Command{
		"achievements": newGameCenterActivityAchievementsSetCommand,
		"leaderboards": newGameCenterActivityLeaderboardsSetCommand,
	}

	for name, newCommand := range commands {
		t.Run(name+" confirm without remove fails validation", func(t *testing.T) {
			d, stderr := newGameCenterTestDeps(nil)
			cmd := newCommand(d)
			if err := cmd.FlagSet.Parse([]string{
				"--activity-id", "activity-1", "--ids", "res-1", "--confirm",
			}); err != nil {
				t.Fatalf("parse flags: %v", err)
			}
			err := cmd.Exec(context.Background(), []string{})
			if !errors.Is(err, flag.ErrHelp) {
				t.Fatalf("--confirm without --remove should fail validation, got %v", err)
			}
			if err.Error() != "--confirm requires --remove" {
				t.Fatalf("error = %q, want %q", err.Error(), "--confirm requires --remove")
			}
			if stderr.String() != "Error: --confirm requires --remove\n" {
				t.Fatalf("stderr = %q, want exact conflict diagnostic", stderr.String())
			}
		})

		t.Run(name+" remove without confirm fails validation", func(t *testing.T) {
			d, stderr := newGameCenterTestDeps(nil)
			cmd := newCommand(d)
			if err := cmd.FlagSet.Parse([]string{
				"--activity-id", "activity-1", "--ids", "res-1", "--remove",
			}); err != nil {
				t.Fatalf("parse flags: %v", err)
			}
			err := cmd.Exec(context.Background(), []string{})
			if !errors.Is(err, flag.ErrHelp) {
				t.Fatalf("remove without --confirm should fail validation, got %v", err)
			}
			if err.Error() != "--confirm" {
				t.Fatalf("error = %q, want missing parameter %q", err.Error(), "--confirm")
			}
			if stderr.String() != "Error: --confirm is required with --remove\n" {
				t.Fatalf("stderr = %q, want exact missing-confirm diagnostic", stderr.String())
			}
		})

		t.Run(name+" remove with confirm passes validation", func(t *testing.T) {
			d, _ := newGameCenterTestDeps(nil)
			cmd := newCommand(d)
			if err := cmd.FlagSet.Parse([]string{
				"--activity-id", "activity-1", "--ids", "res-1", "--remove", "--confirm",
			}); err != nil {
				t.Fatalf("parse flags: %v", err)
			}
			err := cmd.Exec(context.Background(), []string{})
			if errors.Is(err, flag.ErrHelp) {
				t.Fatalf("remove with --confirm should pass validation, got %v", err)
			}
		})

		t.Run(name+" add mode needs no confirm", func(t *testing.T) {
			d, _ := newGameCenterTestDeps(nil)
			cmd := newCommand(d)
			if err := cmd.FlagSet.Parse([]string{
				"--activity-id", "activity-1", "--ids", "res-1",
			}); err != nil {
				t.Fatalf("parse flags: %v", err)
			}
			err := cmd.Exec(context.Background(), []string{})
			if errors.Is(err, flag.ErrHelp) {
				t.Fatalf("additive set should pass validation without --confirm, got %v", err)
			}
		})
	}
}
