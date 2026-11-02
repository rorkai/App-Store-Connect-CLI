package cmdtest

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestGameCenterPublicCreateContracts(t *testing.T) {
	tests := []struct {
		name         string
		args         []string
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
			name:         "leaderboard v1 with set ids",
			args:         []string{"game-center", "leaderboards", "create", "--app", "APP_ID", "--reference-name", "High Score", "--vendor-id", "com.example.highscore", "--formatter", "INTEGER", "--sort", "DESC", "--submission-type", "BEST_SCORE", "--set-ids", "set-2,set-1,set-2"},
			postPath:     "/v1/gameCenterLeaderboards",
			resourceType: "gameCenterLeaderboards",
			wantBody:     `{"data":{"type":"gameCenterLeaderboards","attributes":{"referenceName":"High Score","vendorIdentifier":"com.example.highscore","defaultFormatter":"INTEGER","scoreSortType":"DESC","submissionType":"BEST_SCORE"},"relationships":{"gameCenterDetail":{"data":{"type":"gameCenterDetails","id":"detail-1"}},"gameCenterLeaderboardSets":{"data":[{"type":"gameCenterLeaderboardSets","id":"set-2"},{"type":"gameCenterLeaderboardSets","id":"set-1"}]}}}}`,
			wantRequests: 2,
		},
		{
			name:         "leaderboard v2 with set ids",
			args:         []string{"game-center", "leaderboards", "create", "--app", "APP_ID", "--reference-name", "High Score", "--vendor-id", "com.example.highscore", "--formatter", "INTEGER", "--sort", "DESC", "--submission-type", "BEST_SCORE", "--v2", "--set-ids", "set-2,set-1,set-2"},
			postPath:     "/v2/gameCenterLeaderboards",
			resourceType: "gameCenterLeaderboards",
			wantBody:     `{"data":{"type":"gameCenterLeaderboards","attributes":{"referenceName":"High Score","vendorIdentifier":"com.example.highscore","defaultFormatter":"INTEGER","scoreSortType":"DESC","submissionType":"BEST_SCORE"},"relationships":{"gameCenterDetail":{"data":{"type":"gameCenterDetails","id":"detail-1"}},"gameCenterLeaderboardSets":{"data":[{"type":"gameCenterLeaderboardSets","id":"set-2"},{"type":"gameCenterLeaderboardSets","id":"set-1"}]},"versions":{"data":[{"type":"gameCenterLeaderboardVersions","id":"${lbVer1}"}]}}},"included":[{"type":"gameCenterLeaderboardVersions","id":"${lbVer1}"}]}`,
			wantRequests: 2,
		},
		{
			name:         "leaderboard set v1 with leaderboard ids",
			args:         []string{"game-center", "leaderboard-sets", "create", "--app", "APP_ID", "--reference-name", "Season 1", "--vendor-id", "com.example.season1", "--leaderboard-ids", "lb-2,lb-1,lb-2"},
			postPath:     "/v1/gameCenterLeaderboardSets",
			resourceType: "gameCenterLeaderboardSets",
			wantBody:     `{"data":{"type":"gameCenterLeaderboardSets","attributes":{"referenceName":"Season 1","vendorIdentifier":"com.example.season1"},"relationships":{"gameCenterDetail":{"data":{"type":"gameCenterDetails","id":"detail-1"}},"gameCenterLeaderboards":{"data":[{"type":"gameCenterLeaderboards","id":"lb-2"},{"type":"gameCenterLeaderboards","id":"lb-1"}]}}}}`,
			wantRequests: 2,
		},
		{
			name:         "leaderboard set v2 with leaderboard ids",
			args:         []string{"game-center", "leaderboard-sets", "v2", "create", "--group-id", "group-1", "--reference-name", "Group Season", "--vendor-id", "grp.com.example.groupseason", "--leaderboard-ids", "lb-2,lb-1,lb-2"},
			postPath:     "/v2/gameCenterLeaderboardSets",
			resourceType: "gameCenterLeaderboardSets",
			wantBody:     `{"data":{"type":"gameCenterLeaderboardSets","attributes":{"referenceName":"Group Season","vendorIdentifier":"grp.com.example.groupseason"},"relationships":{"gameCenterGroup":{"data":{"type":"gameCenterGroups","id":"group-1"}},"gameCenterLeaderboards":{"data":[{"type":"gameCenterLeaderboards","id":"lb-2"},{"type":"gameCenterLeaderboards","id":"lb-1"}]},"versions":{"data":[{"type":"gameCenterLeaderboardSetVersions","id":"${lbsetVer1}"}]}}},"included":[{"type":"gameCenterLeaderboardSetVersions","id":"${lbsetVer1}"}]}`,
			wantRequests: 1,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

			callCount := &lockedCounter{}
			installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
				callCount.Inc()
				if req.Method == http.MethodGet && req.URL.Path == "/v1/apps/APP_ID/gameCenterDetail" {
					return gameCenterCreateResponse(http.StatusOK, `{"data":{"type":"gameCenterDetails","id":"detail-1","attributes":{}}}`), nil
				}
				if req.Method != http.MethodPost || req.URL.Path != test.postPath {
					t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
				}

				body, err := io.ReadAll(req.Body)
				if err != nil {
					t.Fatalf("read request body: %v", err)
				}
				assertJSONDocumentEqual(t, body, []byte(test.wantBody))

				response := fmt.Sprintf(`{"data":{"type":%q,"id":"created","attributes":{}}}`, test.resourceType)
				return gameCenterCreateResponse(http.StatusCreated, response), nil
			}))

			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)
			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(test.args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				if err := root.Run(context.Background()); err != nil {
					t.Fatalf("run error: %v", err)
				}
			})

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
			if got := callCount.Load(); got != test.wantRequests {
				t.Fatalf("request count = %d, want %d", got, test.wantRequests)
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

func gameCenterCreateResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
	}
}
