package cmdtest

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func TestGameCenterPublicCreateContracts(t *testing.T) {
	tests := []struct {
		name         string
		args         []string
		appIDEnv     string
		postPath     string
		resourceType string
		wantBody     string
		wantRequests int
	}{
		{
			name:         "achievement v1",
			args:         []string{"game-center", "achievements", "create", "--app", "APP_ID", "--reference-name", "First Win", "--vendor-id", "com.example.firstwin", "--points", "10"},
			postPath:     "/v1/gameCenterAchievements",
			resourceType: "gameCenterAchievements",
			wantBody:     `{"data":{"type":"gameCenterAchievements","attributes":{"referenceName":"First Win","vendorIdentifier":"com.example.firstwin","points":10,"showBeforeEarned":true,"repeatable":false},"relationships":{"gameCenterDetail":{"data":{"type":"gameCenterDetails","id":"detail-1"}}}}}`,
			wantRequests: 2,
		},
		{
			name:         "achievement v2",
			args:         []string{"game-center", "achievements", "create", "--group-id", "group-1", "--reference-name", "Group Win", "--vendor-id", "grp.com.example.groupwin", "--points", "10", "--v2"},
			postPath:     "/v2/gameCenterAchievements",
			resourceType: "gameCenterAchievements",
			wantBody:     `{"data":{"type":"gameCenterAchievements","attributes":{"referenceName":"Group Win","vendorIdentifier":"grp.com.example.groupwin","points":10,"showBeforeEarned":true,"repeatable":false},"relationships":{"gameCenterGroup":{"data":{"type":"gameCenterGroups","id":"group-1"}},"versions":{"data":[{"type":"gameCenterAchievementVersions","id":"${achVer1}"}]}}},"included":[{"type":"gameCenterAchievementVersions","id":"${achVer1}"}]}`,
			wantRequests: 1,
		},
		{
			name:         "leaderboard v1",
			args:         []string{"game-center", "leaderboards", "create", "--app", "APP_ID", "--reference-name", "High Score", "--vendor-id", "com.example.highscore", "--formatter", "INTEGER", "--sort", "DESC", "--submission-type", "BEST_SCORE"},
			postPath:     "/v1/gameCenterLeaderboards",
			resourceType: "gameCenterLeaderboards",
			wantBody:     `{"data":{"type":"gameCenterLeaderboards","attributes":{"referenceName":"High Score","vendorIdentifier":"com.example.highscore","defaultFormatter":"INTEGER","scoreSortType":"DESC","submissionType":"BEST_SCORE"},"relationships":{"gameCenterDetail":{"data":{"type":"gameCenterDetails","id":"detail-1"}}}}}`,
			wantRequests: 2,
		},
		{
			name:         "leaderboard v2",
			args:         []string{"game-center", "leaderboards", "create", "--group-id", "group-1", "--reference-name", "Group Score", "--vendor-id", "grp.com.example.groupscore", "--formatter", "MONEY_YEN", "--sort", "DESC", "--submission-type", "BEST_SCORE", "--v2"},
			postPath:     "/v2/gameCenterLeaderboards",
			resourceType: "gameCenterLeaderboards",
			wantBody:     `{"data":{"type":"gameCenterLeaderboards","attributes":{"referenceName":"Group Score","vendorIdentifier":"grp.com.example.groupscore","defaultFormatter":"MONEY_YEN","scoreSortType":"DESC","submissionType":"BEST_SCORE"},"relationships":{"gameCenterGroup":{"data":{"type":"gameCenterGroups","id":"group-1"}},"versions":{"data":[{"type":"gameCenterLeaderboardVersions","id":"${lbVer1}"}]}}},"included":[{"type":"gameCenterLeaderboardVersions","id":"${lbVer1}"}]}`,
			wantRequests: 1,
		},
		{
			name:         "leaderboard v1 with recurrence",
			args:         []string{"game-center", "leaderboards", "create", "--app", "APP_ID", "--reference-name", "High Score", "--vendor-id", "com.example.highscore", "--formatter", "INTEGER", "--sort", "DESC", "--submission-type", "BEST_SCORE", "--recurrence-start-date", "2026-12-01T00:00:00Z", "--recurrence-duration", "PT168H", "--recurrence-rule", "FREQ=DAILY;INTERVAL=7"},
			postPath:     "/v1/gameCenterLeaderboards",
			resourceType: "gameCenterLeaderboards",
			wantBody:     `{"data":{"type":"gameCenterLeaderboards","attributes":{"referenceName":"High Score","vendorIdentifier":"com.example.highscore","defaultFormatter":"INTEGER","scoreSortType":"DESC","submissionType":"BEST_SCORE","recurrenceStartDate":"2026-12-01T00:00:00Z","recurrenceDuration":"PT168H","recurrenceRule":"FREQ=DAILY;INTERVAL=7"},"relationships":{"gameCenterDetail":{"data":{"type":"gameCenterDetails","id":"detail-1"}}}}}`,
			wantRequests: 2,
		},
		{
			name:         "leaderboard v2 with recurrence",
			args:         []string{"game-center", "leaderboards", "create", "--group-id", "group-1", "--reference-name", "Group Score", "--vendor-id", "grp.com.example.groupscore", "--formatter", "MONEY_YEN", "--sort", "DESC", "--submission-type", "BEST_SCORE", "--v2", "--recurrence-start-date", "2026-12-01T00:00:00Z", "--recurrence-duration", "PT12H", "--recurrence-rule", "FREQ=HOURLY;INTERVAL=12"},
			postPath:     "/v2/gameCenterLeaderboards",
			resourceType: "gameCenterLeaderboards",
			wantBody:     `{"data":{"type":"gameCenterLeaderboards","attributes":{"referenceName":"Group Score","vendorIdentifier":"grp.com.example.groupscore","defaultFormatter":"MONEY_YEN","scoreSortType":"DESC","submissionType":"BEST_SCORE","recurrenceStartDate":"2026-12-01T00:00:00Z","recurrenceDuration":"PT12H","recurrenceRule":"FREQ=HOURLY;INTERVAL=12"},"relationships":{"gameCenterGroup":{"data":{"type":"gameCenterGroups","id":"group-1"}},"versions":{"data":[{"type":"gameCenterLeaderboardVersions","id":"${lbVer1}"}]}}},"included":[{"type":"gameCenterLeaderboardVersions","id":"${lbVer1}"}]}`,
			wantRequests: 1,
		},
		{
			name:         "achievement v1 activity properties",
			args:         []string{"game-center", "achievements", "create", "--app", "APP_ID", "--reference-name", "First Win", "--vendor-id", "com.example.firstwin", "--points", "10", "--activity-properties", `{"mode":"ranked"}`},
			postPath:     "/v1/gameCenterAchievements",
			resourceType: "gameCenterAchievements",
			wantBody:     `{"data":{"type":"gameCenterAchievements","attributes":{"referenceName":"First Win","vendorIdentifier":"com.example.firstwin","points":10,"showBeforeEarned":true,"repeatable":false,"activityProperties":{"mode":"ranked"}},"relationships":{"gameCenterDetail":{"data":{"type":"gameCenterDetails","id":"detail-1"}}}}}`,
			wantRequests: 2,
		},
		{
			name:         "achievement v2 activity properties",
			args:         []string{"game-center", "achievements", "create", "--group-id", "group-1", "--reference-name", "Group Win", "--vendor-id", "grp.com.example.groupwin", "--points", "10", "--v2", "--activity-properties", `{"mode":"ranked"}`},
			postPath:     "/v2/gameCenterAchievements",
			resourceType: "gameCenterAchievements",
			wantBody:     `{"data":{"type":"gameCenterAchievements","attributes":{"referenceName":"Group Win","vendorIdentifier":"grp.com.example.groupwin","points":10,"showBeforeEarned":true,"repeatable":false,"activityProperties":{"mode":"ranked"}},"relationships":{"gameCenterGroup":{"data":{"type":"gameCenterGroups","id":"group-1"}},"versions":{"data":[{"type":"gameCenterAchievementVersions","id":"${achVer1}"}]}}},"included":[{"type":"gameCenterAchievementVersions","id":"${achVer1}"}]}`,
			wantRequests: 1,
		},
		{
			name:         "leaderboard v1 visibility and activity properties",
			args:         []string{"game-center", "leaderboards", "create", "--app", "APP_ID", "--reference-name", "High Score", "--vendor-id", "com.example.highscore", "--formatter", "INTEGER", "--sort", "DESC", "--submission-type", "BEST_SCORE", "--visibility", "hide_for_all", "--activity-properties", `{"mode":"ranked"}`},
			postPath:     "/v1/gameCenterLeaderboards",
			resourceType: "gameCenterLeaderboards",
			wantBody:     `{"data":{"type":"gameCenterLeaderboards","attributes":{"referenceName":"High Score","vendorIdentifier":"com.example.highscore","defaultFormatter":"INTEGER","scoreSortType":"DESC","submissionType":"BEST_SCORE","visibility":"HIDE_FOR_ALL","activityProperties":{"mode":"ranked"}},"relationships":{"gameCenterDetail":{"data":{"type":"gameCenterDetails","id":"detail-1"}}}}}`,
			wantRequests: 2,
		},
		{
			name:         "leaderboard v2 visibility and activity properties",
			args:         []string{"game-center", "leaderboards", "create", "--group-id", "group-1", "--reference-name", "Group Score", "--vendor-id", "grp.com.example.groupscore", "--formatter", "MONEY_YEN", "--sort", "DESC", "--submission-type", "BEST_SCORE", "--v2", "--visibility", "SHOW_FOR_ALL", "--activity-properties", `{"mode":"ranked"}`},
			postPath:     "/v2/gameCenterLeaderboards",
			resourceType: "gameCenterLeaderboards",
			wantBody:     `{"data":{"type":"gameCenterLeaderboards","attributes":{"referenceName":"Group Score","vendorIdentifier":"grp.com.example.groupscore","defaultFormatter":"MONEY_YEN","scoreSortType":"DESC","submissionType":"BEST_SCORE","visibility":"SHOW_FOR_ALL","activityProperties":{"mode":"ranked"}},"relationships":{"gameCenterGroup":{"data":{"type":"gameCenterGroups","id":"group-1"}},"versions":{"data":[{"type":"gameCenterLeaderboardVersions","id":"${lbVer1}"}]}}},"included":[{"type":"gameCenterLeaderboardVersions","id":"${lbVer1}"}]}`,
			wantRequests: 1,
		},
		{
			name:         "activity properties",
			args:         []string{"game-center", "activities", "create", "--app", "APP_ID", "--reference-name", "Weekly", "--vendor-id", "com.example.weekly", "--properties", `{"mode":"ranked"}`},
			postPath:     "/v1/gameCenterActivities",
			resourceType: "gameCenterActivities",
			wantBody:     `{"data":{"type":"gameCenterActivities","attributes":{"referenceName":"Weekly","vendorIdentifier":"com.example.weekly","properties":{"mode":"ranked"}},"relationships":{"gameCenterDetail":{"data":{"type":"gameCenterDetails","id":"detail-1"}}}}}`,
			wantRequests: 2,
		},
		{
			name:         "leaderboard set v1",
			args:         []string{"game-center", "leaderboard-sets", "create", "--app", "APP_ID", "--reference-name", "Season 1", "--vendor-id", "com.example.season1"},
			postPath:     "/v1/gameCenterLeaderboardSets",
			resourceType: "gameCenterLeaderboardSets",
			wantBody:     `{"data":{"type":"gameCenterLeaderboardSets","attributes":{"referenceName":"Season 1","vendorIdentifier":"com.example.season1"},"relationships":{"gameCenterDetail":{"data":{"type":"gameCenterDetails","id":"detail-1"}}}}}`,
			wantRequests: 2,
		},
		{
			name:         "leaderboard set v2",
			args:         []string{"game-center", "leaderboard-sets", "v2", "create", "--group-id", "group-1", "--reference-name", "Group Season", "--vendor-id", "grp.com.example.groupseason"},
			postPath:     "/v2/gameCenterLeaderboardSets",
			resourceType: "gameCenterLeaderboardSets",
			wantBody:     `{"data":{"type":"gameCenterLeaderboardSets","attributes":{"referenceName":"Group Season","vendorIdentifier":"grp.com.example.groupseason"},"relationships":{"gameCenterGroup":{"data":{"type":"gameCenterGroups","id":"group-1"}},"versions":{"data":[{"type":"gameCenterLeaderboardSetVersions","id":"${lbsetVer1}"}]}}},"included":[{"type":"gameCenterLeaderboardSetVersions","id":"${lbsetVer1}"}]}`,
			wantRequests: 1,
		},
		{
			// Apple returns 409 when gameCenterLeaderboardSets is sent with gameCenterDetail or gameCenterGroup.
			name:         "leaderboard v1 with set ids inherits parent",
			args:         []string{"game-center", "leaderboards", "create", "--reference-name", "High Score", "--vendor-id", "com.example.highscore", "--formatter", "INTEGER", "--sort", "DESC", "--submission-type", "BEST_SCORE", "--set-ids", "set-2,set-1,set-2"},
			appIDEnv:     "APP_ID",
			postPath:     "/v1/gameCenterLeaderboards",
			resourceType: "gameCenterLeaderboards",
			wantBody:     `{"data":{"type":"gameCenterLeaderboards","attributes":{"referenceName":"High Score","vendorIdentifier":"com.example.highscore","defaultFormatter":"INTEGER","scoreSortType":"DESC","submissionType":"BEST_SCORE"},"relationships":{"gameCenterLeaderboardSets":{"data":[{"type":"gameCenterLeaderboardSets","id":"set-2"},{"type":"gameCenterLeaderboardSets","id":"set-1"}]}}}}`,
			wantRequests: 1,
		},
		{
			name:         "leaderboard v2 with set ids inherits parent",
			args:         []string{"game-center", "leaderboards", "create", "--reference-name", "High Score", "--vendor-id", "com.example.highscore", "--formatter", "INTEGER", "--sort", "DESC", "--submission-type", "BEST_SCORE", "--v2", "--set-ids", "set-2,set-1,set-2"},
			postPath:     "/v2/gameCenterLeaderboards",
			resourceType: "gameCenterLeaderboards",
			wantBody:     `{"data":{"type":"gameCenterLeaderboards","attributes":{"referenceName":"High Score","vendorIdentifier":"com.example.highscore","defaultFormatter":"INTEGER","scoreSortType":"DESC","submissionType":"BEST_SCORE"},"relationships":{"gameCenterLeaderboardSets":{"data":[{"type":"gameCenterLeaderboardSets","id":"set-2"},{"type":"gameCenterLeaderboardSets","id":"set-1"}]},"versions":{"data":[{"type":"gameCenterLeaderboardVersions","id":"${lbVer1}"}]}}},"included":[{"type":"gameCenterLeaderboardVersions","id":"${lbVer1}"}]}`,
			wantRequests: 1,
		},
		{
			// Apple returns 409 when gameCenterLeaderboards is sent with gameCenterDetail or gameCenterGroup.
			name:         "leaderboard set v1 with leaderboard ids inherits parent",
			args:         []string{"game-center", "leaderboard-sets", "create", "--reference-name", "Season 1", "--vendor-id", "com.example.season1", "--leaderboard-ids", "lb-2,lb-1,lb-2"},
			appIDEnv:     "APP_ID",
			postPath:     "/v1/gameCenterLeaderboardSets",
			resourceType: "gameCenterLeaderboardSets",
			wantBody:     `{"data":{"type":"gameCenterLeaderboardSets","attributes":{"referenceName":"Season 1","vendorIdentifier":"com.example.season1"},"relationships":{"gameCenterLeaderboards":{"data":[{"type":"gameCenterLeaderboards","id":"lb-2"},{"type":"gameCenterLeaderboards","id":"lb-1"}]}}}}`,
			wantRequests: 1,
		},
		{
			name:         "leaderboard set v2 with leaderboard ids inherits parent",
			args:         []string{"game-center", "leaderboard-sets", "v2", "create", "--reference-name", "Group Season", "--vendor-id", "grp.com.example.groupseason", "--leaderboard-ids", "lb-2,lb-1,lb-2"},
			postPath:     "/v2/gameCenterLeaderboardSets",
			resourceType: "gameCenterLeaderboardSets",
			wantBody:     `{"data":{"type":"gameCenterLeaderboardSets","attributes":{"referenceName":"Group Season","vendorIdentifier":"grp.com.example.groupseason"},"relationships":{"gameCenterLeaderboards":{"data":[{"type":"gameCenterLeaderboards","id":"lb-2"},{"type":"gameCenterLeaderboards","id":"lb-1"}]},"versions":{"data":[{"type":"gameCenterLeaderboardSetVersions","id":"${lbsetVer1}"}]}}},"included":[{"type":"gameCenterLeaderboardSetVersions","id":"${lbsetVer1}"}]}`,
			wantRequests: 1,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
			t.Setenv("ASC_APP_ID", test.appIDEnv)

			reg := asctest.NewRegistry(t)
			reg.Install()
			if test.wantRequests == 2 {
				reg.Register(asctest.GET("/v1/apps/APP_ID/gameCenterDetail"),
					asctest.JSON(http.StatusOK, `{"data":{"type":"gameCenterDetails","id":"detail-1","attributes":{}}}`))
			}
			reg.Register(asctest.POST(test.postPath).JSONBody(test.wantBody),
				asctest.JSON(http.StatusCreated, fmt.Sprintf(`{"data":{"type":%q,"id":"created","attributes":{}}}`, test.resourceType)))

			stdout, stderr, err := runCommand(t, test.args)
			if err != nil {
				t.Fatalf("run error: %v", err)
			}

			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}
			var output struct {
				Data struct {
					ID string `json:"id"`
				} `json:"data"`
			}
			if err := json.Unmarshal([]byte(stdout), &output); err != nil {
				t.Fatalf("decode stdout: %v\nstdout: %q", err, stdout)
			}
			if output.Data.ID != "created" {
				t.Fatalf("created ID = %q, want created", output.Data.ID)
			}
			if got := len(reg.Requests()); got != test.wantRequests {
				t.Fatalf("request count = %d, want %d", got, test.wantRequests)
			}
		})
	}
}

func TestGameCenterCreateMembersRejectExplicitParent(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")

	const setInherits = "the set inherits its parent from the member leaderboards"
	const leaderboardInherits = "the leaderboard inherits its parent from the leaderboard sets"
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "leaderboard set v1 app",
			args:    []string{"game-center", "leaderboard-sets", "create", "--app", "APP_ID", "--reference-name", "Season", "--vendor-id", "com.example.season", "--leaderboard-ids", "lb-1"},
			wantErr: "Error: --app cannot be used with --leaderboard-ids; " + setInherits,
		},
		{
			name:    "leaderboard set v2 app",
			args:    []string{"game-center", "leaderboard-sets", "v2", "create", "--app", "APP_ID", "--reference-name", "Season", "--vendor-id", "com.example.season", "--leaderboard-ids", "lb-1"},
			wantErr: "Error: --app cannot be used with --leaderboard-ids; " + setInherits,
		},
		{
			name:    "leaderboard set v2 group",
			args:    []string{"game-center", "leaderboard-sets", "v2", "create", "--group-id", "GROUP_ID", "--reference-name", "Season", "--vendor-id", "grp.com.example.season", "--leaderboard-ids", "lb-1"},
			wantErr: "Error: --group-id cannot be used with --leaderboard-ids; " + setInherits,
		},
		{
			name:    "leaderboard v1 app",
			args:    []string{"game-center", "leaderboards", "create", "--app", "APP_ID", "--reference-name", "Score", "--vendor-id", "com.example.score", "--formatter", "INTEGER", "--sort", "DESC", "--submission-type", "BEST_SCORE", "--set-ids", "set-1"},
			wantErr: "Error: --app cannot be used with --set-ids; " + leaderboardInherits,
		},
		{
			name:    "leaderboard v2 group",
			args:    []string{"game-center", "leaderboards", "create", "--group-id", "GROUP_ID", "--reference-name", "Score", "--vendor-id", "grp.com.example.score", "--formatter", "INTEGER", "--sort", "DESC", "--submission-type", "BEST_SCORE", "--set-ids", "set-1"},
			wantErr: "Error: --group-id cannot be used with --set-ids; " + leaderboardInherits,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stdout, stderr := captureOutput(t, func() {
				if exitCode := rootcmd.Run(test.args, "1.2.3"); exitCode != rootcmd.ExitUsage {
					t.Fatalf("exit code = %d, want %d", exitCode, rootcmd.ExitUsage)
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

func TestGameCenterUpdateSendsVisibilityAndProperties(t *testing.T) {
	propertiesPath := filepath.Join(t.TempDir(), "properties.json")
	if err := os.WriteFile(propertiesPath, []byte(`{"mode":"casual"}`+"\n"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	tests := []struct {
		name          string
		args          []string
		patchPath     string
		resourceType  string
		wantAttribute string
	}{
		{
			name:          "achievement v1 activity properties",
			args:          []string{"game-center", "achievements", "update", "--id", "res-1", "--activity-properties", `{"mode":"ranked"}`},
			patchPath:     "/v1/gameCenterAchievements/res-1",
			resourceType:  "gameCenterAchievements",
			wantAttribute: `{"activityProperties":{"mode":"ranked"}}`,
		},
		{
			name:          "achievement v2 activity properties from a file",
			args:          []string{"game-center", "achievements", "update", "--id", "res-1", "--v2", "--activity-properties", "@file:" + propertiesPath},
			patchPath:     "/v2/gameCenterAchievements/res-1",
			resourceType:  "gameCenterAchievements",
			wantAttribute: `{"activityProperties":{"mode":"casual"}}`,
		},
		{
			name:          "leaderboard v1 visibility and activity properties",
			args:          []string{"game-center", "leaderboards", "update", "--id", "res-1", "--visibility", "hide_for_all", "--activity-properties", `{"mode":"ranked"}`},
			patchPath:     "/v1/gameCenterLeaderboards/res-1",
			resourceType:  "gameCenterLeaderboards",
			wantAttribute: `{"visibility":"HIDE_FOR_ALL","activityProperties":{"mode":"ranked"}}`,
		},
		{
			name:          "leaderboard v2 visibility",
			args:          []string{"game-center", "leaderboards", "update", "--id", "res-1", "--v2", "--visibility", "SHOW_FOR_ALL"},
			patchPath:     "/v2/gameCenterLeaderboards/res-1",
			resourceType:  "gameCenterLeaderboards",
			wantAttribute: `{"visibility":"SHOW_FOR_ALL"}`,
		},
		{
			name:          "activity properties",
			args:          []string{"game-center", "activities", "update", "--id", "res-1", "--properties", `{"mode":"ranked"}`},
			patchPath:     "/v1/gameCenterActivities/res-1",
			resourceType:  "gameCenterActivities",
			wantAttribute: `{"properties":{"mode":"ranked"}}`,
		},
		{
			name:          "empty object is sent instead of an empty patch",
			args:          []string{"game-center", "activities", "update", "--id", "res-1", "--properties", `{}`},
			patchPath:     "/v1/gameCenterActivities/res-1",
			resourceType:  "gameCenterActivities",
			wantAttribute: `{"properties":{}}`,
		},
		{
			name:          "omitted flags stay out of the request",
			args:          []string{"game-center", "leaderboards", "update", "--id", "res-1", "--reference-name", "New Name"},
			patchPath:     "/v1/gameCenterLeaderboards/res-1",
			resourceType:  "gameCenterLeaderboards",
			wantAttribute: `{"referenceName":"New Name"}`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

			reg := asctest.NewRegistry(t)
			reg.Install()
			reg.Register(asctest.PATCH(test.patchPath).JSONBody(fmt.Sprintf(`{"data":{"type":%q,"id":"res-1","attributes":%s}}`, test.resourceType, test.wantAttribute)),
				asctest.JSON(http.StatusOK, fmt.Sprintf(`{"data":{"type":%q,"id":"res-1","attributes":{}}}`, test.resourceType)))

			_, stderr := captureOutput(t, func() {
				if code := rootcmd.Run(test.args, "1.2.3"); code != rootcmd.ExitSuccess {
					t.Fatalf("exit code = %d, want %d", code, rootcmd.ExitSuccess)
				}
			})
			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}
		})
	}
}

func assertJSONDocumentEqual(t *testing.T, got, want []byte) {
	t.Helper()

	var gotDocument any
	if err := json.Unmarshal(got, &gotDocument); err != nil {
		t.Fatalf("decode actual JSON: %v\nbody: %s", err, got)
	}
	var wantDocument any
	if err := json.Unmarshal(want, &wantDocument); err != nil {
		t.Fatalf("decode expected JSON: %v\nbody: %s", err, want)
	}
	if !reflect.DeepEqual(gotDocument, wantDocument) {
		t.Fatalf("request body mismatch\ngot:  %s\nwant: %s", strings.TrimSpace(string(got)), strings.TrimSpace(string(want)))
	}
}
