package cmdtest

import (
	"context"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
)

func TestGameCenterActivitiesValidationErrors(t *testing.T) {
	runGameCenterUsageCases(t, []gameCenterUsageCase{
		{[]string{"game-center", "activities", "list"}, "--app is required (or set ASC_APP_ID)"},
		{[]string{"game-center", "activities", "create", "--reference-name", "Seasonal"}, "--app is required (or set ASC_APP_ID)"},
		{[]string{"game-center", "activities", "create", "--app", "APP_ID", "--group-id", "GROUP_ID", "--reference-name", "Seasonal", "--vendor-id", "com.example.seasonal"}, "--app cannot be used with --group-id"},
		{[]string{"game-center", "activities", "create", "--group-id", "GROUP_ID", "--reference-name", "Seasonal", "--vendor-id", "com.example.seasonal"}, "--vendor-id must start with \"grp.\" when using --group-id"},
		{[]string{"game-center", "activities", "create", "--app", "APP_ID"}, "--reference-name is required"},
		{[]string{"game-center", "activities", "create", "--app", "APP_ID", "--reference-name", "Seasonal", "--vendor-id", "com.example.seasonal", "--initial-fallback-url", "https://example.com/fallback"}, "--initial-fallback-url requires --create-initial-version true"},
		{[]string{"game-center", "activities", "update"}, "--id is required"},
		{[]string{"game-center", "activities", "update", "--id", "ACTIVITY_ID"}, "at least one update flag is required"},
		{[]string{"game-center", "activities", "delete", "--confirm"}, "--id is required"},
		{[]string{"game-center", "activities", "delete", "--id", "ACTIVITY_ID"}, "--confirm is required"},
		{[]string{"game-center", "activities", "achievements", "set", "--ids", "ACH_1"}, "--activity-id is required"},
		{[]string{"game-center", "activities", "achievements", "set", "--activity-id", "ACTIVITY_ID"}, "--ids is required"},
		{[]string{"game-center", "activities", "leaderboards", "set", "--ids", "LB_1"}, "--activity-id is required"},
		{[]string{"game-center", "activities", "leaderboards", "set", "--activity-id", "ACTIVITY_ID"}, "--ids is required"},
		{[]string{"game-center", "activities", "versions", "list"}, "--activity-id is required"},
		{[]string{"game-center", "activities", "versions", "create"}, "--activity-id is required"},
		{[]string{"game-center", "activities", "localizations", "list"}, "--version-id is required"},
		{[]string{"game-center", "activities", "localizations", "create", "--locale", "en-US", "--name", "Weekly", "--description", "Desc"}, "--version-id is required"},
		{[]string{"game-center", "activities", "localizations", "create", "--version-id", "VER_ID", "--name", "Weekly", "--description", "Desc"}, "--locale is required"},
		{[]string{"game-center", "activities", "localizations", "create", "--version-id", "VER_ID", "--locale", "en-US", "--description", "Desc"}, "--name is required"},
		{[]string{"game-center", "activities", "localizations", "create", "--version-id", "VER_ID", "--locale", "en-US", "--name", "Weekly"}, "--description is required"},
		{[]string{"game-center", "activities", "localizations", "update"}, "--id is required"},
		{[]string{"game-center", "activities", "localizations", "update", "--id", "LOC_ID"}, "at least one update flag is required"},
		{[]string{"game-center", "activities", "localizations", "delete", "--confirm"}, "--id is required"},
		{[]string{"game-center", "activities", "localizations", "delete", "--id", "LOC_ID"}, "--confirm is required"},
		{[]string{"game-center", "activities", "images", "upload", "--localization-id", "LOC_ID"}, "--file is required"},
		{[]string{"game-center", "activities", "images", "delete", "--confirm"}, "--id is required"},
		{[]string{"game-center", "activities", "images", "delete", "--id", "IMAGE_ID"}, "--confirm is required"},
		{[]string{"game-center", "activities", "releases", "list"}, "--app is required (or set ASC_APP_ID)"},
		{[]string{"game-center", "activities", "releases", "create"}, "--version-id is required"},
		{[]string{"game-center", "activities", "releases", "delete", "--confirm"}, "--id is required"},
		{[]string{"game-center", "activities", "releases", "delete", "--id", "REL_ID"}, "--confirm is required"},
		{[]string{"game-center", "activities", "list", "--app", "APP_ID", "--limit", "400"}, "game-center activities list: --limit must be between 1 and 200"},
	})
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
				return jsonHTTPResponse(http.StatusNoContent, ""), nil
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
