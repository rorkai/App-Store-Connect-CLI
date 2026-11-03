package cmdtest

import (
	"context"
	"errors"
	"flag"
	"io"
	"strings"
	"testing"
)

func TestGameCenterLeaderboardSetsV2ListValidationErrors(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"game-center", "leaderboard-sets", "v2", "list"}); err != nil {
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
	if !strings.Contains(stderr, "Error: --app is required (or set ASC_APP_ID)") {
		t.Fatalf("expected missing app error, got %q", stderr)
	}
}

func TestGameCenterLeaderboardSetsV2CreateValidationErrors(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "missing app",
			args:    []string{"game-center", "leaderboard-sets", "v2", "create", "--reference-name", "Season", "--vendor-id", "com.example.season"},
			wantErr: "Error: --app is required (or set ASC_APP_ID)",
		},
		{
			name:    "app and group",
			args:    []string{"game-center", "leaderboard-sets", "v2", "create", "--app", "APP_ID", "--group-id", "GROUP_ID", "--reference-name", "Season", "--vendor-id", "com.example.season"},
			wantErr: "Error: --app cannot be used with --group-id",
		},
		{
			name:    "group vendor prefix",
			args:    []string{"game-center", "leaderboard-sets", "v2", "create", "--group-id", "GROUP_ID", "--reference-name", "Season", "--vendor-id", "com.example.season"},
			wantErr: "Error: --vendor-id must start with \"grp.\" when using --group-id",
		},
		{
			name:    "missing reference-name",
			args:    []string{"game-center", "leaderboard-sets", "v2", "create", "--app", "APP_ID", "--vendor-id", "com.example.season"},
			wantErr: "Error: --reference-name is required",
		},
		{
			name:    "missing vendor-id",
			args:    []string{"game-center", "leaderboard-sets", "v2", "create", "--app", "APP_ID", "--reference-name", "Season"},
			wantErr: "Error: --vendor-id is required",
		},
		{
			name:    "empty leaderboard-ids",
			args:    []string{"game-center", "leaderboard-sets", "v2", "create", "--app", "APP_ID", "--reference-name", "Season", "--vendor-id", "com.example.season", "--leaderboard-ids", " , "},
			wantErr: "Error: --leaderboard-ids must include at least one ID",
		},
	}

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

func TestGameCenterLeaderboardSetMembersV2ListValidationErrors(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"game-center", "leaderboard-sets", "v2", "members", "list"}); err != nil {
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
	if !strings.Contains(stderr, "Error: --set-id is required") {
		t.Fatalf("expected missing set-id error, got %q", stderr)
	}
}

func TestGameCenterLeaderboardSetVersionsV2ListValidationErrors(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"game-center", "leaderboard-sets", "v2", "versions", "list"}); err != nil {
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
	if !strings.Contains(stderr, "Error: --set-id is required") {
		t.Fatalf("expected missing set-id error, got %q", stderr)
	}
}

func TestGameCenterLeaderboardSetLocalizationsV2CreateValidationErrors(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "missing version-id",
			args:    []string{"game-center", "leaderboard-sets", "v2", "localizations", "create", "--locale", "en-US", "--name", "Season"},
			wantErr: "Error: --version-id is required",
		},
		{
			name:    "missing locale",
			args:    []string{"game-center", "leaderboard-sets", "v2", "localizations", "create", "--version-id", "VER_ID", "--name", "Season"},
			wantErr: "Error: --locale is required",
		},
		{
			name:    "missing name",
			args:    []string{"game-center", "leaderboard-sets", "v2", "localizations", "create", "--version-id", "VER_ID", "--locale", "en-US"},
			wantErr: "Error: --name is required",
		},
	}

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

func TestGameCenterLeaderboardSetImagesV2ViewValidationErrors(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"game-center", "leaderboard-sets", "v2", "images", "view"}); err != nil {
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
	if !strings.Contains(stderr, "Error: --id or --localization-id is required") {
		t.Fatalf("expected missing id/localization error, got %q", stderr)
	}
}

func TestGameCenterLeaderboardSetImagesV2DeleteValidationErrors(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "missing id",
			args:    []string{"game-center", "leaderboard-sets", "v2", "images", "delete", "--confirm"},
			wantErr: "Error: --id is required",
		},
		{
			name:    "missing confirm",
			args:    []string{"game-center", "leaderboard-sets", "v2", "images", "delete", "--id", "IMAGE_ID"},
			wantErr: "Error: --confirm is required",
		},
	}

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
