package cmdtest

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func TestGameCenterLeaderboardsValidationErrors(t *testing.T) {
	runGameCenterUsageCases(t, []gameCenterUsageCase{
		{[]string{"game-center", "leaderboards", "list"}, "--app is required (or set ASC_APP_ID)"},
		{[]string{"game-center", "leaderboards", "create", "--reference-name", "Test", "--vendor-id", "com.test", "--formatter", "INTEGER", "--sort", "DESC", "--submission-type", "BEST_SCORE"}, "--app is required (or set ASC_APP_ID)"},
		{[]string{"game-center", "leaderboards", "create", "--app", "APP_ID", "--group-id", "GROUP_ID", "--reference-name", "Test", "--vendor-id", "com.test", "--formatter", "INTEGER", "--sort", "DESC", "--submission-type", "BEST_SCORE"}, "--app cannot be used with --group-id"},
		{[]string{"game-center", "leaderboards", "create", "--group-id", "GROUP_ID", "--reference-name", "Test", "--vendor-id", "com.test", "--formatter", "INTEGER", "--sort", "DESC", "--submission-type", "BEST_SCORE"}, "--vendor-id must start with \"grp.\" when using --group-id"},
		{[]string{"game-center", "leaderboards", "create", "--app", "APP_ID", "--vendor-id", "com.test", "--formatter", "INTEGER", "--sort", "DESC", "--submission-type", "BEST_SCORE"}, "--reference-name is required"},
		{[]string{"game-center", "leaderboards", "create", "--app", "APP_ID", "--reference-name", "Test", "--formatter", "INTEGER", "--sort", "DESC", "--submission-type", "BEST_SCORE"}, "--vendor-id is required"},
		{[]string{"game-center", "leaderboards", "create", "--app", "APP_ID", "--reference-name", "Test", "--vendor-id", "com.test", "--sort", "DESC", "--submission-type", "BEST_SCORE"}, "--formatter is required"},
		{[]string{"game-center", "leaderboards", "create", "--app", "APP_ID", "--reference-name", "Test", "--vendor-id", "com.test", "--formatter", "INTEGER", "--submission-type", "BEST_SCORE"}, "--sort is required"},
		{[]string{"game-center", "leaderboards", "create", "--app", "APP_ID", "--reference-name", "Test", "--vendor-id", "com.test", "--formatter", "INTEGER", "--sort", "DESC"}, "--submission-type is required"},
		{[]string{"game-center", "leaderboards", "create", "--app", "APP_ID", "--reference-name", "Test", "--vendor-id", "com.test", "--formatter", "INTEGER", "--sort", "DESC", "--submission-type", "BEST_SCORE", "--set-ids", " , "}, "--set-ids must include at least one ID"},
		{[]string{"game-center", "leaderboards", "submit", "--score", "100", "--bundle-id", "BUNDLE_ID", "--scoped-player-id", "PLAYER_ID"}, "--vendor-id is required"},
		{[]string{"game-center", "leaderboards", "submit", "--vendor-id", "com.example.leaderboard", "--bundle-id", "BUNDLE_ID", "--scoped-player-id", "PLAYER_ID"}, "--score is required"},
		{[]string{"game-center", "leaderboards", "submit", "--vendor-id", "com.example.leaderboard", "--score", "100", "--scoped-player-id", "PLAYER_ID"}, "--bundle-id is required"},
		{[]string{"game-center", "leaderboards", "submit", "--vendor-id", "com.example.leaderboard", "--score", "100", "--bundle-id", "BUNDLE_ID"}, "--scoped-player-id is required"},
		{[]string{"game-center", "leaderboards", "localizations", "list"}, "--leaderboard-id is required"},
		{[]string{"game-center", "leaderboards", "localizations", "create", "--locale", "en-US", "--name", "Test"}, "--leaderboard-id is required"},
		{[]string{"game-center", "leaderboards", "localizations", "create", "--leaderboard-id", "LB_ID", "--name", "Test"}, "--locale is required"},
		{[]string{"game-center", "leaderboards", "localizations", "create", "--leaderboard-id", "LB_ID", "--locale", "en-US"}, "--name is required"},
		{[]string{"game-center", "leaderboards", "list", "--app", "APP_ID", "--limit", "300"}, "game-center leaderboards list: --limit must be between 1 and 200"},
	})
}

func TestGameCenterLeaderboardsListNoDetailReturnsEmptyList(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	expectedURL := "https://api.appstoreconnect.apple.com/v1/apps/APP_ID/gameCenterDetail"

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/apps/APP_ID/gameCenterDetail"),
		asctest.JSON(http.StatusOK, `{"data":{"type":"gameCenterDetails","id":"","attributes":{}},"links":{"self":"`+expectedURL+`"}}`))

	stdout, stderr, err := runCommand(t, []string{"game-center", "leaderboards", "list", "--app", "APP_ID"})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

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
			reg := asctest.NewRegistry(t)
			reg.Register(asctest.PATCH(test.wantPath).JSONBody(`{"data":{"type":"gameCenterLeaderboards","id":"lb-1","attributes":{`+test.wantAttrs+`}}}`),
				asctest.JSON(http.StatusOK, `{"data":{"type":"gameCenterLeaderboards","id":"lb-1","attributes":{}}}`))
			setGameCenterContractClient(t, reg)

			args := append([]string{"game-center", "leaderboards", "update", "--id", "lb-1"}, test.args...)
			if _, stderr, err := runCommand(t, args); err != nil {
				t.Fatalf("run error: %v\nstderr: %s", err, stderr)
			}
			if requests := len(reg.Requests()); requests != 1 {
				t.Fatalf("request count = %d, want 1", requests)
			}
		})
	}
}
