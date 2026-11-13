package cmdtest

import "testing"

func TestGameCenterLeaderboardsListsRejectsInvalidNextURL(t *testing.T) {
	runInvalidNextURLCases(t, []invalidNextURLCase{
		{args: []string{"game-center", "leaderboards", "list"}, wantErr: "game-center leaderboards list: --next"},
		{args: []string{"game-center", "leaderboards", "localizations", "list"}, wantErr: "game-center leaderboards localizations list: --next"},
		{args: []string{"game-center", "leaderboards", "releases", "list"}, wantErr: "game-center leaderboards releases list: --next"},
		{args: []string{"game-center", "leaderboards", "v2", "list"}, wantErr: "game-center leaderboards v2 list: --next"},
		{args: []string{"game-center", "leaderboards", "v2", "versions", "list"}, wantErr: "game-center leaderboards v2 versions list: --next"},
		{args: []string{"game-center", "leaderboards", "v2", "localizations", "list"}, wantErr: "game-center leaderboards v2 localizations list: --next"},
	})
}

func TestGameCenterLeaderboardsListsPaginateFromNext(t *testing.T) {
	runPaginateFromNextCases(t, []paginateFromNextCase{
		{
			args:     []string{"game-center", "leaderboards", "list"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/gameCenterDetails/detail-1/gameCenterLeaderboards?cursor=AQ&limit=200",
			dataType: "gameCenterLeaderboards",
		},
		{
			args:     []string{"game-center", "leaderboards", "localizations", "list"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/gameCenterLeaderboards/lb-1/localizations?cursor=AQ&limit=200",
			dataType: "gameCenterLeaderboardLocalizations",
		},
		{
			args:     []string{"game-center", "leaderboards", "releases", "list"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/gameCenterLeaderboards/lb-1/releases?cursor=AQ&limit=200",
			dataType: "gameCenterLeaderboardReleases",
		},
		{
			args:     []string{"game-center", "leaderboards", "v2", "list"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/gameCenterDetails/detail-1/gameCenterLeaderboardsV2?cursor=AQ&limit=200",
			dataType: "gameCenterLeaderboards",
		},
		{
			args:     []string{"game-center", "leaderboards", "v2", "versions", "list"},
			firstURL: "https://api.appstoreconnect.apple.com/v2/gameCenterLeaderboards/lb-1/versions?cursor=AQ&limit=200",
			dataType: "gameCenterLeaderboardVersions",
		},
		{
			args:     []string{"game-center", "leaderboards", "v2", "localizations", "list"},
			firstURL: "https://api.appstoreconnect.apple.com/v2/gameCenterLeaderboardVersions/ver-1/localizations?cursor=AQ&limit=200",
			dataType: "gameCenterLeaderboardLocalizations",
		},
	})
}
