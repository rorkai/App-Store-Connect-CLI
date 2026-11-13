package cmdtest

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"path/filepath"
	"reflect"
	"testing"
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

			var body map[string]any
			requests := 0
			installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
				requests++
				if req.Method != http.MethodPatch || req.URL.Path != "/v1/gameCenterDetails/DETAIL_ID" {
					t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
				}
				if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
					t.Fatalf("decode request body: %v", err)
				}
				return jsonHTTPResponse(http.StatusOK, `{"data":{"type":"gameCenterDetails","id":"DETAIL_ID"}}`), nil
			}))

			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)
			args := append([]string{"game-center", "details", "update", "--id", "DETAIL_ID", "--output", "json"}, test.args...)

			captureOutput(t, func() {
				if err := root.Parse(args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				if err := root.Run(context.Background()); err != nil {
					t.Fatalf("run error: %v", err)
				}
			})

			if requests != 1 {
				t.Fatalf("expected 1 request, got %d", requests)
			}
			var want map[string]any
			if err := json.Unmarshal([]byte(`{"data":{"type":"gameCenterDetails","id":"DETAIL_ID","relationships":`+test.want+`}}`), &want); err != nil {
				t.Fatalf("decode expected body: %v", err)
			}
			if !reflect.DeepEqual(body, want) {
				t.Fatalf("request body = %v, want %v", body, want)
			}
		})
	}
}
