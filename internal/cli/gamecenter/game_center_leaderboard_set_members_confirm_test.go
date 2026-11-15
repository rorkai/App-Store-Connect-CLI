package gamecenter

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"strings"
	"testing"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func TestGameCenterLeaderboardSetMembersReplacementRejectsWithoutConfirmBeforeHTTP(t *testing.T) {
	isolateGameCenterAuthEnv(t)

	commands := map[string]func(*shared.Deps) *ffcli.Command{
		"v1": newGameCenterLeaderboardSetMembersSetCommand,
		"v2": newGameCenterLeaderboardSetMembersV2SetCommand,
	}

	for name, newCommand := range commands {
		for _, leaderboardIDs := range []string{"leaderboard-1", ""} {
			t.Run(name+" ids="+leaderboardIDs, func(t *testing.T) {
				factoryCalled := false
				d, stderr := newGameCenterTestDeps(&factoryCalled)

				cmd := newCommand(d)
				if err := cmd.FlagSet.Parse([]string{
					"--set-id", "set-1", "--leaderboard-ids", leaderboardIDs,
				}); err != nil {
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

func TestGameCenterLeaderboardSetMembersRequiresLeaderboardIDs(t *testing.T) {
	isolateGameCenterAuthEnv(t)

	commands := map[string]func(*shared.Deps) *ffcli.Command{
		"v1": newGameCenterLeaderboardSetMembersSetCommand,
		"v2": newGameCenterLeaderboardSetMembersV2SetCommand,
	}

	for name, newCommand := range commands {
		t.Run(name, func(t *testing.T) {
			d, stderr := newGameCenterTestDeps(nil)
			cmd := newCommand(d)
			if err := cmd.FlagSet.Parse([]string{
				"--set-id", "set-1", "--confirm",
			}); err != nil {
				t.Fatalf("parse flags: %v", err)
			}

			err := cmd.Exec(context.Background(), []string{})
			if !errors.Is(err, flag.ErrHelp) {
				t.Fatalf("missing --leaderboard-ids should fail validation before auth, got %v", err)
			}
			if err.Error() != "--leaderboard-ids" {
				t.Fatalf("error = %q, want missing parameter %q", err.Error(), "--leaderboard-ids")
			}
			if stderr.String() != "Error: --leaderboard-ids is required\n" {
				t.Fatalf("stderr = %q, want exact missing-ID diagnostic", stderr.String())
			}
		})
	}
}

func TestGameCenterLeaderboardSetMembersConfirmPassesValidation(t *testing.T) {
	isolateGameCenterAuthEnv(t)

	commands := map[string]func(*shared.Deps) *ffcli.Command{
		"v1": newGameCenterLeaderboardSetMembersSetCommand,
		"v2": newGameCenterLeaderboardSetMembersV2SetCommand,
	}

	for name, newCommand := range commands {
		for _, leaderboardIDs := range []string{"leaderboard-1", ""} {
			t.Run(name+" ids="+leaderboardIDs, func(t *testing.T) {
				d, _ := newGameCenterTestDeps(nil)
				cmd := newCommand(d)
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

// newGameCenterTestDeps returns deps whose client fails and sets called when
// it is reached, and the buffer that receives their stderr.
func newGameCenterTestDeps(called *bool) (*shared.Deps, *bytes.Buffer) {
	var stdout, stderr bytes.Buffer
	return &shared.Deps{
		Client: func() (*asc.Client, error) {
			if called != nil {
				*called = true
			}
			return nil, errors.New("test client called")
		},
		IO: &shared.IO{In: strings.NewReader(""), Out: &stdout, ErrOut: &stderr},
	}, &stderr
}
