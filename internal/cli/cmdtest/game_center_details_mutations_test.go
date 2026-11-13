package cmdtest

import (
	"net/http"
	"path/filepath"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func TestGameCenterDetailsMutationValidationErrors(t *testing.T) {
	runGameCenterUsageCases(t, []gameCenterUsageCase{
		{[]string{"game-center", "details", "create"}, "--app is required (or set ASC_APP_ID)"},
		{[]string{"game-center", "details", "update", "--game-center-group-id", "GROUP_ID"}, "--id is required"},
		{[]string{"game-center", "details", "update", "--id", "DETAIL_ID"}, "at least one update flag is required (--game-center-group-id, --default-leaderboard-id, --default-leaderboard-v2-id, --default-group-leaderboard-id, --default-group-leaderboard-v2-id)"},
		{[]string{"game-center", "details", "update", "--id", "DETAIL_ID", "--default-leaderboard-id", "LB_V1", "--default-leaderboard-v2-id", "LB_V2"}, "--default-leaderboard-id and --default-leaderboard-v2-id set the same default; pass one"},
		{[]string{"game-center", "details", "update", "--id", "DETAIL_ID", "--default-group-leaderboard-id", "GROUP_LB_V1", "--default-group-leaderboard-v2-id", "GROUP_LB_V2"}, "--default-group-leaderboard-id and --default-group-leaderboard-v2-id set the same default; pass one"},
	})
}

func TestGameCenterDetailsUpdateSendsOnlyRequestedRelationships(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "group with v2 default and v1 group default",
			args: []string{
				"--game-center-group-id", "GROUP_ID",
				"--default-leaderboard-v2-id", "https://api.appstoreconnect.apple.com/v2/gameCenterLeaderboards/LB_V2",
				"--default-group-leaderboard-id", "GROUP_LB_V1",
			},
			want: `{
				"gameCenterGroup": {"data": {"type": "gameCenterGroups", "id": "GROUP_ID"}},
				"defaultLeaderboardV2": {"data": {"type": "gameCenterLeaderboards", "id": "LB_V2"}},
				"defaultGroupLeaderboard": {"data": {"type": "gameCenterLeaderboards", "id": "GROUP_LB_V1"}}
			}`,
		},
		{
			name: "one v2 default leaves the others out",
			args: []string{"--default-group-leaderboard-v2-id", "GROUP_LB_V2"},
			want: `{"defaultGroupLeaderboardV2": {"data": {"type": "gameCenterLeaderboards", "id": "GROUP_LB_V2"}}}`,
		},
		{
			name: "existing flag request is unchanged",
			args: []string{"--default-leaderboard-id", "LB_V1"},
			want: `{"defaultLeaderboard": {"data": {"type": "gameCenterLeaderboards", "id": "LB_V1"}}}`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

			reg := asctest.NewRegistry(t)
			reg.Install()
			reg.Register(asctest.PATCH("/v1/gameCenterDetails/DETAIL_ID").JSONBody(`{"data":{"type":"gameCenterDetails","id":"DETAIL_ID","relationships":`+test.want+`}}`),
				asctest.JSON(http.StatusOK, `{"data":{"type":"gameCenterDetails","id":"DETAIL_ID"}}`))

			args := append([]string{"game-center", "details", "update", "--id", "DETAIL_ID", "--output", "json"}, test.args...)
			if _, _, err := runCommand(t, args); err != nil {
				t.Fatalf("run error: %v", err)
			}

			if requests := len(reg.Requests()); requests != 1 {
				t.Fatalf("expected 1 request, got %d", requests)
			}
		})
	}
}
