package cmdtest

import (
	"context"
	"errors"
	"flag"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
)

func TestGameCenterActivitiesListValidationErrors(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, _ := captureOutput(t, func() {
		if err := root.Parse([]string{"game-center", "activities", "list"}); err != nil {
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
}

func TestGameCenterActivitiesCreateValidationErrors(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")

	tests := []struct {
		name string
		args []string
	}{
		{
			name: "missing app",
			args: []string{"game-center", "activities", "create", "--reference-name", "Seasonal"},
		},
		{
			name: "app and group",
			args: []string{"game-center", "activities", "create", "--app", "APP_ID", "--group-id", "GROUP_ID", "--reference-name", "Seasonal", "--vendor-id", "com.example.seasonal"},
		},
		{
			name: "group vendor prefix",
			args: []string{"game-center", "activities", "create", "--group-id", "GROUP_ID", "--reference-name", "Seasonal", "--vendor-id", "com.example.seasonal"},
		},
		{
			name: "missing reference-name",
			args: []string{"game-center", "activities", "create", "--app", "APP_ID"},
		},
		{
			name: "fallback requires initial version",
			args: []string{"game-center", "activities", "create", "--app", "APP_ID", "--reference-name", "Seasonal", "--vendor-id", "com.example.seasonal", "--initial-fallback-url", "https://example.com/fallback"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			stdout, _ := captureOutput(t, func() {
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
		})
	}
}

func TestGameCenterActivitiesUpdateValidationErrors(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{
			name: "missing id",
			args: []string{"game-center", "activities", "update"},
		},
		{
			name: "missing update flags",
			args: []string{"game-center", "activities", "update", "--id", "ACTIVITY_ID"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			stdout, _ := captureOutput(t, func() {
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
		})
	}
}

func TestGameCenterActivitiesDeleteValidationErrors(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{
			name: "missing id",
			args: []string{"game-center", "activities", "delete", "--confirm"},
		},
		{
			name: "missing confirm",
			args: []string{"game-center", "activities", "delete", "--id", "ACTIVITY_ID"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			stdout, _ := captureOutput(t, func() {
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
		})
	}
}

func TestGameCenterActivityAchievementsSetValidationErrors(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{
			name: "missing activity-id",
			args: []string{"game-center", "activities", "achievements", "set", "--ids", "ACH_1"},
		},
		{
			name: "missing ids",
			args: []string{"game-center", "activities", "achievements", "set", "--activity-id", "ACTIVITY_ID"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			stdout, _ := captureOutput(t, func() {
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
		})
	}
}

func TestGameCenterActivityLeaderboardsSetValidationErrors(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{
			name: "missing activity-id",
			args: []string{"game-center", "activities", "leaderboards", "set", "--ids", "LB_1"},
		},
		{
			name: "missing ids",
			args: []string{"game-center", "activities", "leaderboards", "set", "--activity-id", "ACTIVITY_ID"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			stdout, _ := captureOutput(t, func() {
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
		})
	}
}

func TestGameCenterActivityRelationshipsSetRequestWiring(t *testing.T) {
	const achievementsBody = `{"data":[{"type":"gameCenterAchievements","id":"RES_1"},{"type":"gameCenterAchievements","id":"RES_2"}]}`
	const leaderboardsBody = `{"data":[{"type":"gameCenterLeaderboards","id":"RES_1"},{"type":"gameCenterLeaderboards","id":"RES_2"}]}`

	tests := []struct {
		name       string
		resource   string
		extraArgs  []string
		wantMethod string
		wantPath   string
		wantBody   string
	}{
		{"achievements add", "achievements", nil, http.MethodPost, "/v1/gameCenterActivities/ACTIVITY_ID/relationships/achievements", achievementsBody},
		{"achievements remove", "achievements", []string{"--remove", "--confirm"}, http.MethodDelete, "/v1/gameCenterActivities/ACTIVITY_ID/relationships/achievements", achievementsBody},
		{"achievements add v2", "achievements", []string{"--v2"}, http.MethodPost, "/v1/gameCenterActivities/ACTIVITY_ID/relationships/achievementsV2", achievementsBody},
		{"achievements remove v2", "achievements", []string{"--v2", "--remove", "--confirm"}, http.MethodDelete, "/v1/gameCenterActivities/ACTIVITY_ID/relationships/achievementsV2", achievementsBody},
		{"leaderboards add", "leaderboards", nil, http.MethodPost, "/v1/gameCenterActivities/ACTIVITY_ID/relationships/leaderboards", leaderboardsBody},
		{"leaderboards remove", "leaderboards", []string{"--remove", "--confirm"}, http.MethodDelete, "/v1/gameCenterActivities/ACTIVITY_ID/relationships/leaderboards", leaderboardsBody},
		{"leaderboards add v2", "leaderboards", []string{"--v2"}, http.MethodPost, "/v1/gameCenterActivities/ACTIVITY_ID/relationships/leaderboardsV2", leaderboardsBody},
		{"leaderboards remove v2", "leaderboards", []string{"--v2", "--remove", "--confirm"}, http.MethodDelete, "/v1/gameCenterActivities/ACTIVITY_ID/relationships/leaderboardsV2", leaderboardsBody},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

			callCount := &lockedCounter{}
			installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
				callCount.Inc()
				if req.Method != test.wantMethod || req.URL.Path != test.wantPath {
					t.Fatalf("request = %s %s, want %s %s", req.Method, req.URL.Path, test.wantMethod, test.wantPath)
				}
				body, err := io.ReadAll(req.Body)
				if err != nil {
					t.Fatalf("read request body: %v", err)
				}
				if string(body) != test.wantBody+"\n" {
					t.Fatalf("request body = %q, want %q", body, test.wantBody+"\n")
				}
				return gameCenterCreateResponse(http.StatusNoContent, ""), nil
			}))

			args := []string{"game-center", "activities", test.resource, "set", "--activity-id", "ACTIVITY_ID", "--ids", "RES_1,RES_2"}
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)
			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(append(args, test.extraArgs...)); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				if err := root.Run(context.Background()); err != nil {
					t.Fatalf("run error: %v", err)
				}
			})

			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}
			wantOutput := strings.TrimSuffix(test.wantBody, "}") + `,"links":{}}`
			assertJSONDocumentEqual(t, []byte(stdout), []byte(wantOutput))
			if got := callCount.Load(); got != 1 {
				t.Fatalf("request count = %d, want 1", got)
			}
		})
	}
}

func TestGameCenterActivityVersionsListValidationErrors(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, _ := captureOutput(t, func() {
		if err := root.Parse([]string{"game-center", "activities", "versions", "list"}); err != nil {
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
}

func TestGameCenterActivityVersionsCreateValidationErrors(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, _ := captureOutput(t, func() {
		if err := root.Parse([]string{"game-center", "activities", "versions", "create"}); err != nil {
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
}

func TestGameCenterActivityLocalizationsListValidationErrors(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, _ := captureOutput(t, func() {
		if err := root.Parse([]string{"game-center", "activities", "localizations", "list"}); err != nil {
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
}

func TestGameCenterActivityLocalizationsCreateValidationErrors(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{
			name: "missing version-id",
			args: []string{"game-center", "activities", "localizations", "create", "--locale", "en-US", "--name", "Weekly", "--description", "Desc"},
		},
		{
			name: "missing locale",
			args: []string{"game-center", "activities", "localizations", "create", "--version-id", "VER_ID", "--name", "Weekly", "--description", "Desc"},
		},
		{
			name: "missing name",
			args: []string{"game-center", "activities", "localizations", "create", "--version-id", "VER_ID", "--locale", "en-US", "--description", "Desc"},
		},
		{
			name: "missing description",
			args: []string{"game-center", "activities", "localizations", "create", "--version-id", "VER_ID", "--locale", "en-US", "--name", "Weekly"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			stdout, _ := captureOutput(t, func() {
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
		})
	}
}

func TestGameCenterActivityLocalizationsUpdateValidationErrors(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{
			name: "missing id",
			args: []string{"game-center", "activities", "localizations", "update"},
		},
		{
			name: "missing update flags",
			args: []string{"game-center", "activities", "localizations", "update", "--id", "LOC_ID"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			stdout, _ := captureOutput(t, func() {
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
		})
	}
}

func TestGameCenterActivityLocalizationsDeleteValidationErrors(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{
			name: "missing id",
			args: []string{"game-center", "activities", "localizations", "delete", "--confirm"},
		},
		{
			name: "missing confirm",
			args: []string{"game-center", "activities", "localizations", "delete", "--id", "LOC_ID"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			stdout, _ := captureOutput(t, func() {
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
		})
	}
}

func TestGameCenterActivityImagesUploadValidationErrors(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{
			name: "missing target",
			args: []string{"game-center", "activities", "images", "upload", "--file", "test.png"},
		},
		{
			name: "missing file",
			args: []string{"game-center", "activities", "images", "upload", "--localization-id", "LOC_ID"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			stdout, _ := captureOutput(t, func() {
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
		})
	}
}

func TestGameCenterActivityImagesDeleteValidationErrors(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{
			name: "missing id",
			args: []string{"game-center", "activities", "images", "delete", "--confirm"},
		},
		{
			name: "missing confirm",
			args: []string{"game-center", "activities", "images", "delete", "--id", "IMAGE_ID"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			stdout, _ := captureOutput(t, func() {
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
		})
	}
}

func TestGameCenterActivityReleasesListValidationErrors(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, _ := captureOutput(t, func() {
		if err := root.Parse([]string{"game-center", "activities", "releases", "list"}); err != nil {
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
}

func TestGameCenterActivityReleasesCreateValidationErrors(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, _ := captureOutput(t, func() {
		if err := root.Parse([]string{"game-center", "activities", "releases", "create"}); err != nil {
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
}

func TestGameCenterActivityReleasesDeleteValidationErrors(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{
			name: "missing id",
			args: []string{"game-center", "activities", "releases", "delete", "--confirm"},
		},
		{
			name: "missing confirm",
			args: []string{"game-center", "activities", "releases", "delete", "--id", "REL_ID"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			stdout, _ := captureOutput(t, func() {
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
		})
	}
}

func TestGameCenterActivitiesListLimitValidation(t *testing.T) {
	t.Setenv("ASC_APP_ID", "APP_ID")

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, _ := captureOutput(t, func() {
		if err := root.Parse([]string{"game-center", "activities", "list", "--app", "APP_ID", "--limit", "400"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if err == nil {
			t.Fatalf("expected error, got nil")
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
}
