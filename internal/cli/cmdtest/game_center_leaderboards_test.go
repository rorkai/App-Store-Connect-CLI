package cmdtest

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
)

func TestGameCenterLeaderboardsListValidationErrors(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, _ := captureOutput(t, func() {
		if err := root.Parse([]string{"game-center", "leaderboards", "list"}); err != nil {
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

func TestGameCenterLeaderboardsListNoDetailReturnsEmptyList(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	expectedURL := "https://api.appstoreconnect.apple.com/v1/apps/APP_ID/gameCenterDetail"

	callCount := &lockedCounter{}
	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if callCount.Inc() > 1 {
			t.Fatalf("unexpected extra request: %s %s", req.Method, req.URL.String())
		}
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.String() != expectedURL {
			t.Fatalf("expected URL %s, got %s", expectedURL, req.URL.String())
		}

		body := `{"data":{"type":"gameCenterDetails","id":"","attributes":{}},"links":{"self":"` + expectedURL + `"}}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	}))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"game-center", "leaderboards", "list", "--app", "APP_ID"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if !strings.Contains(stderr, "Warning: no Game Center detail exists for this app") {
		t.Fatalf("expected warning in stderr, got %q", stderr)
	}

	var resp struct {
		Data []any `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &resp); err != nil {
		t.Fatalf("failed to parse JSON output: %v\nstdout: %q", err, stdout)
	}
	if len(resp.Data) != 0 {
		t.Fatalf("expected empty data array, got %d items", len(resp.Data))
	}
}

func TestGameCenterLeaderboardsCreateValidationErrors(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")

	tests := []struct {
		name string
		args []string
	}{
		{
			name: "missing app",
			args: []string{"game-center", "leaderboards", "create", "--reference-name", "Test", "--vendor-id", "com.test", "--formatter", "INTEGER", "--sort", "DESC", "--submission-type", "BEST_SCORE"},
		},
		{
			name: "app and group",
			args: []string{"game-center", "leaderboards", "create", "--app", "APP_ID", "--group-id", "GROUP_ID", "--reference-name", "Test", "--vendor-id", "com.test", "--formatter", "INTEGER", "--sort", "DESC", "--submission-type", "BEST_SCORE"},
		},
		{
			name: "group vendor prefix",
			args: []string{"game-center", "leaderboards", "create", "--group-id", "GROUP_ID", "--reference-name", "Test", "--vendor-id", "com.test", "--formatter", "INTEGER", "--sort", "DESC", "--submission-type", "BEST_SCORE"},
		},
		{
			name: "missing reference-name",
			args: []string{"game-center", "leaderboards", "create", "--app", "APP_ID", "--vendor-id", "com.test", "--formatter", "INTEGER", "--sort", "DESC", "--submission-type", "BEST_SCORE"},
		},
		{
			name: "missing vendor-id",
			args: []string{"game-center", "leaderboards", "create", "--app", "APP_ID", "--reference-name", "Test", "--formatter", "INTEGER", "--sort", "DESC", "--submission-type", "BEST_SCORE"},
		},
		{
			name: "missing formatter",
			args: []string{"game-center", "leaderboards", "create", "--app", "APP_ID", "--reference-name", "Test", "--vendor-id", "com.test", "--sort", "DESC", "--submission-type", "BEST_SCORE"},
		},
		{
			name: "missing sort",
			args: []string{"game-center", "leaderboards", "create", "--app", "APP_ID", "--reference-name", "Test", "--vendor-id", "com.test", "--formatter", "INTEGER", "--submission-type", "BEST_SCORE"},
		},
		{
			name: "missing submission-type",
			args: []string{"game-center", "leaderboards", "create", "--app", "APP_ID", "--reference-name", "Test", "--vendor-id", "com.test", "--formatter", "INTEGER", "--sort", "DESC"},
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

func TestGameCenterLeaderboardsCreateRejectsInvalidFormatter(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")

	for _, formatter := range []string{"ELAPSED_TIME_MILLISECOND", "MONEY_WHOLE", "MONEY_POINT_2_PLACE"} {
		t.Run(formatter, func(t *testing.T) {
			stdout, stderr := captureOutput(t, func() {
				args := []string{"game-center", "leaderboards", "create", "--app", "APP_ID", "--reference-name", "Test", "--vendor-id", "com.test", "--formatter", formatter, "--sort", "DESC", "--submission-type", "BEST_SCORE"}
				if exitCode := rootcmd.Run(args, "1.2.3"); exitCode != rootcmd.ExitUsage {
					t.Fatalf("expected exit code %d, got %d", rootcmd.ExitUsage, exitCode)
				}
			})

			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, "Error: --formatter must be one of:") {
				t.Fatalf("expected formatter validation error, got %q", stderr)
			}
		})
	}
}

func TestGameCenterLeaderboardsUpdateScoringRequestBody(t *testing.T) {
	const allScoringFlags = `"defaultFormatter":"ELAPSED_TIME_SECOND","scoreSortType":"ASC","submissionType":"MOST_RECENT_SCORE","scoreRangeStart":"-100","scoreRangeEnd":"5000"`

	tests := []struct {
		name      string
		args      []string
		wantPath  string
		wantAttrs string
	}{
		{
			name:      "v1 sends every scoring flag normalized like create",
			args:      []string{"--formatter", "elapsed_time_second", "--sort", "asc", "--submission-type", "most_recent_score", "--score-range-start", "-100", "--score-range-end", " 5000 "},
			wantPath:  "/v1/gameCenterLeaderboards/lb-1",
			wantAttrs: allScoringFlags,
		},
		{
			name:      "v2 sends every scoring flag to the v2 endpoint",
			args:      []string{"--formatter", "ELAPSED_TIME_SECOND", "--sort", "ASC", "--submission-type", "MOST_RECENT_SCORE", "--score-range-start", "-100", "--score-range-end", "5000", "--v2"},
			wantPath:  "/v2/gameCenterLeaderboards/lb-1",
			wantAttrs: allScoringFlags,
		},
		{
			name:      "omitted flags stay out of the body",
			args:      []string{"--sort", "DESC"},
			wantPath:  "/v1/gameCenterLeaderboards/lb-1",
			wantAttrs: `"scoreSortType":"DESC"`,
		},
		{
			name:      "zero and false values reach the body",
			args:      []string{"--score-range-start=0", "--score-range-end=0", "--archived=false", "--v2"},
			wantPath:  "/v2/gameCenterLeaderboards/lb-1",
			wantAttrs: `"scoreRangeStart":"0","scoreRangeEnd":"0","archived":false`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var requests int
			setGameCenterContractClient(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
				requests++
				if req.Method != http.MethodPatch || req.URL.Path != test.wantPath {
					t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
				}
				body, err := io.ReadAll(req.Body)
				if err != nil {
					t.Fatalf("read request body: %v", err)
				}
				assertJSONDocumentEqual(t, body, []byte(`{"data":{"type":"gameCenterLeaderboards","id":"lb-1","attributes":{`+test.wantAttrs+`}}}`))
				return gameCenterContractJSONResponse(`{"data":{"type":"gameCenterLeaderboards","id":"lb-1","attributes":{}}}`), nil
			}))

			args := append([]string{"game-center", "leaderboards", "update", "--id", "lb-1"}, test.args...)
			if _, stderr, err := runCommand(t, args); err != nil {
				t.Fatalf("run error: %v\nstderr: %s", err, stderr)
			}
			if requests != 1 {
				t.Fatalf("request count = %d, want 1", requests)
			}
		})
	}
}

func TestGameCenterLeaderboardsSubmitValidationErrors(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{
			name: "missing vendor-id",
			args: []string{"game-center", "leaderboards", "submit", "--score", "100", "--bundle-id", "BUNDLE_ID", "--scoped-player-id", "PLAYER_ID"},
		},
		{
			name: "missing score",
			args: []string{"game-center", "leaderboards", "submit", "--vendor-id", "com.example.leaderboard", "--bundle-id", "BUNDLE_ID", "--scoped-player-id", "PLAYER_ID"},
		},
		{
			name: "missing bundle-id",
			args: []string{"game-center", "leaderboards", "submit", "--vendor-id", "com.example.leaderboard", "--score", "100", "--scoped-player-id", "PLAYER_ID"},
		},
		{
			name: "missing scoped-player-id",
			args: []string{"game-center", "leaderboards", "submit", "--vendor-id", "com.example.leaderboard", "--score", "100", "--bundle-id", "BUNDLE_ID"},
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

func TestGameCenterLeaderboardLocalizationsListValidationErrors(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, _ := captureOutput(t, func() {
		if err := root.Parse([]string{"game-center", "leaderboards", "localizations", "list"}); err != nil {
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

func TestGameCenterLeaderboardLocalizationsCreateValidationErrors(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{
			name: "missing leaderboard-id",
			args: []string{"game-center", "leaderboards", "localizations", "create", "--locale", "en-US", "--name", "Test"},
		},
		{
			name: "missing locale",
			args: []string{"game-center", "leaderboards", "localizations", "create", "--leaderboard-id", "LB_ID", "--name", "Test"},
		},
		{
			name: "missing name",
			args: []string{"game-center", "leaderboards", "localizations", "create", "--leaderboard-id", "LB_ID", "--locale", "en-US"},
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

func TestGameCenterLeaderboardsListLimitValidation(t *testing.T) {
	t.Setenv("ASC_APP_ID", "APP_ID")

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, _ := captureOutput(t, func() {
		if err := root.Parse([]string{"game-center", "leaderboards", "list", "--app", "APP_ID", "--limit", "300"}); err != nil {
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
