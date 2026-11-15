package cmdtest

import "testing"

func TestGameCenterGroupsListsRejectsInvalidNextURL(t *testing.T) {
	runInvalidNextURLCases(t, []invalidNextURLCase{
		{args: []string{"game-center", "groups", "list"}, wantErr: "game-center groups list: --next", deps: true},
		{args: []string{"game-center", "groups", "achievements", "list"}, wantErr: "game-center groups achievements list: --next", deps: true},
		{args: []string{"game-center", "groups", "leaderboards", "list"}, wantErr: "game-center groups leaderboards list: --next", deps: true},
		{args: []string{"game-center", "groups", "leaderboard-sets", "list"}, wantErr: "game-center groups leaderboard-sets list: --next", deps: true},
		{args: []string{"game-center", "groups", "activities", "list"}, wantErr: "game-center groups activities list: --next", deps: true},
		{args: []string{"game-center", "groups", "challenges", "list"}, wantErr: "game-center groups challenges list: --next", deps: true},
		{args: []string{"game-center", "groups", "details", "list"}, wantErr: "game-center groups details list: --next", deps: true},
	})
}

func TestGameCenterGroupsListsPaginateFromNext(t *testing.T) {
	runPaginateFromNextCases(t, []paginateFromNextCase{
		{
			args:     []string{"game-center", "groups", "list"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/gameCenterGroups?cursor=AQ&limit=200",
			dataType: "gameCenterGroups",
			deps:     true,
		},
		{
			args:     []string{"game-center", "groups", "achievements", "list"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/gameCenterGroups/group-1/gameCenterAchievements?cursor=AQ&limit=200",
			dataType: "gameCenterAchievements",
			deps:     true,
		},
		{
			args:     []string{"game-center", "groups", "leaderboards", "list"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/gameCenterGroups/group-1/gameCenterLeaderboards?cursor=AQ&limit=200",
			dataType: "gameCenterLeaderboards",
			deps:     true,
		},
		{
			args:     []string{"game-center", "groups", "leaderboard-sets", "list"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/gameCenterGroups/group-1/gameCenterLeaderboardSets?cursor=AQ&limit=200",
			dataType: "gameCenterLeaderboardSets",
			deps:     true,
		},
		{
			args:     []string{"game-center", "groups", "activities", "list"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/gameCenterGroups/group-1/gameCenterActivities?cursor=AQ&limit=200",
			dataType: "gameCenterActivities",
			deps:     true,
		},
		{
			args:     []string{"game-center", "groups", "challenges", "list"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/gameCenterGroups/group-1/gameCenterChallenges?cursor=AQ&limit=200",
			dataType: "gameCenterChallenges",
			deps:     true,
		},
		{
			args:     []string{"game-center", "groups", "details", "list"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/gameCenterGroups/group-1/gameCenterDetails?cursor=AQ&limit=200",
			dataType: "gameCenterDetails",
			deps:     true,
		},
	})
}
