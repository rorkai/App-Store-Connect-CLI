package cmdtest

import "testing"

func TestGameCenterDetailsListsRejectsInvalidNextURL(t *testing.T) {
	runInvalidNextURLCases(t, []invalidNextURLCase{
		{args: []string{"game-center", "details", "app-versions", "list"}, wantErr: "game-center details app-versions list: --next"},
		{args: []string{"game-center", "details", "achievements-v2", "list"}, wantErr: "game-center details achievements-v2 list: --next"},
		{args: []string{"game-center", "details", "leaderboards-v2", "list"}, wantErr: "game-center details leaderboards-v2 list: --next"},
		{args: []string{"game-center", "details", "leaderboard-sets-v2", "list"}, wantErr: "game-center details leaderboard-sets-v2 list: --next"},
		{args: []string{"game-center", "details", "achievement-releases", "list"}, wantErr: "game-center details achievement-releases list: --next"},
		{args: []string{"game-center", "details", "leaderboard-releases", "list"}, wantErr: "game-center details leaderboard-releases list: --next"},
		{args: []string{"game-center", "details", "leaderboard-set-releases", "list"}, wantErr: "game-center details leaderboard-set-releases list: --next"},
	})
}

func TestGameCenterDetailsListsPaginateFromNext(t *testing.T) {
	runPaginateFromNextCases(t, []paginateFromNextCase{
		{
			args:     []string{"game-center", "details", "app-versions", "list"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/gameCenterDetails/detail-1/gameCenterAppVersions?cursor=AQ&limit=200",
			dataType: "gameCenterAppVersions",
		},
		{
			args:     []string{"game-center", "details", "achievements-v2", "list"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/gameCenterDetails/detail-1/gameCenterAchievementsV2?cursor=AQ&limit=200",
			dataType: "gameCenterAchievements",
		},
		{
			args:     []string{"game-center", "details", "leaderboards-v2", "list"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/gameCenterDetails/detail-1/gameCenterLeaderboardsV2?cursor=AQ&limit=200",
			dataType: "gameCenterLeaderboards",
		},
		{
			args:     []string{"game-center", "details", "leaderboard-sets-v2", "list"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/gameCenterDetails/detail-1/gameCenterLeaderboardSetsV2?cursor=AQ&limit=200",
			dataType: "gameCenterLeaderboardSets",
		},
		{
			args:     []string{"game-center", "details", "achievement-releases", "list"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/gameCenterDetails/detail-1/achievementReleases?cursor=AQ&limit=200",
			dataType: "gameCenterAchievementReleases",
		},
		{
			args:     []string{"game-center", "details", "leaderboard-releases", "list"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/gameCenterDetails/detail-1/leaderboardReleases?cursor=AQ&limit=200",
			dataType: "gameCenterLeaderboardReleases",
		},
		{
			args:     []string{"game-center", "details", "leaderboard-set-releases", "list"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/gameCenterDetails/detail-1/leaderboardSetReleases?cursor=AQ&limit=200",
			dataType: "gameCenterLeaderboardSetReleases",
		},
	})
}
