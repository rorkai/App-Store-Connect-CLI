package cmdtest

import "testing"

func TestGameCenterMatchmakingListsRejectsInvalidNextURL(t *testing.T) {
	runInvalidNextURLCases(t, []invalidNextURLCase{
		{args: []string{"game-center", "matchmaking", "queues", "list"}, wantErr: "game-center matchmaking queues list: --next"},
		{args: []string{"game-center", "matchmaking", "rule-sets", "list"}, wantErr: "game-center matchmaking rule-sets list: --next"},
		{args: []string{"game-center", "matchmaking", "rule-sets", "queues", "list"}, wantErr: "game-center matchmaking rule-sets queues list: --next"},
		{args: []string{"game-center", "matchmaking", "rules", "list"}, wantErr: "game-center matchmaking rules list: --next"},
		{args: []string{"game-center", "matchmaking", "teams", "list"}, wantErr: "game-center matchmaking teams list: --next"},
	})
}

func TestGameCenterMatchmakingListsPaginateFromNext(t *testing.T) {
	runPaginateFromNextCases(t, []paginateFromNextCase{
		{
			args:     []string{"game-center", "matchmaking", "queues", "list"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/gameCenterMatchmakingQueues?cursor=AQ&limit=200",
			dataType: "gameCenterMatchmakingQueues",
		},
		{
			args:     []string{"game-center", "matchmaking", "rule-sets", "list"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/gameCenterMatchmakingRuleSets?cursor=AQ&limit=200",
			dataType: "gameCenterMatchmakingRuleSets",
		},
		{
			args:     []string{"game-center", "matchmaking", "rule-sets", "queues", "list"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/gameCenterMatchmakingRuleSets/rule-set-1/matchmakingQueues?cursor=AQ&limit=200",
			dataType: "gameCenterMatchmakingQueues",
		},
		{
			args:     []string{"game-center", "matchmaking", "rules", "list"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/gameCenterMatchmakingRuleSets/rule-set-1/rules?cursor=AQ&limit=200",
			dataType: "gameCenterMatchmakingRules",
		},
		{
			args:     []string{"game-center", "matchmaking", "teams", "list"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/gameCenterMatchmakingRuleSets/rule-set-1/teams?cursor=AQ&limit=200",
			dataType: "gameCenterMatchmakingTeams",
		},
	})
}
