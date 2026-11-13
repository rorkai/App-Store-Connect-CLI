package cmdtest

import "testing"

func TestGameCenterLeaderboardSetsListsRejectsInvalidNextURL(t *testing.T) {
	runInvalidNextURLCases(t, []invalidNextURLCase{
		{args: []string{"game-center", "leaderboard-sets", "list"}, wantErr: "game-center leaderboard-sets list: --next"},
		{args: []string{"game-center", "leaderboard-sets", "localizations", "list"}, wantErr: "game-center leaderboard-sets localizations list: --next"},
		{args: []string{"game-center", "leaderboard-sets", "member-localizations", "list"}, wantErr: "game-center leaderboard-sets member-localizations list: --next"},
		{args: []string{"game-center", "leaderboard-sets", "members", "list"}, wantErr: "game-center leaderboard-sets members list: --next"},
		{args: []string{"game-center", "leaderboard-sets", "releases", "list"}, wantErr: "game-center leaderboard-sets releases list: --next"},
		{args: []string{"game-center", "leaderboard-sets", "v2", "list"}, wantErr: "game-center leaderboard-sets v2 list: --next"},
		{args: []string{"game-center", "leaderboard-sets", "v2", "localizations", "list"}, wantErr: "game-center leaderboard-sets v2 localizations list: --next"},
		{args: []string{"game-center", "leaderboard-sets", "v2", "members", "list"}, wantErr: "game-center leaderboard-sets v2 members list: --next"},
		{args: []string{"game-center", "leaderboard-sets", "v2", "versions", "list"}, wantErr: "game-center leaderboard-sets v2 versions list: --next"},
	})
}

func TestGameCenterLeaderboardSetsListsPaginateFromNext(t *testing.T) {
	runPaginateFromNextCases(t, []paginateFromNextCase{
		{
			args:     []string{"game-center", "leaderboard-sets", "list"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/gameCenterDetails/detail-1/gameCenterLeaderboardSets?cursor=AQ&limit=200",
			dataType: "gameCenterLeaderboardSets",
		},
		{
			args:     []string{"game-center", "leaderboard-sets", "localizations", "list"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/gameCenterLeaderboardSets/set-1/localizations?cursor=AQ&limit=200",
			dataType: "gameCenterLeaderboardSetLocalizations",
		},
		{
			args:     []string{"game-center", "leaderboard-sets", "member-localizations", "list"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/gameCenterLeaderboardSetMemberLocalizations?cursor=AQ&limit=200",
			dataType: "gameCenterLeaderboardSetMemberLocalizations",
		},
		{
			args:     []string{"game-center", "leaderboard-sets", "members", "list"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/gameCenterLeaderboardSets/set-1/gameCenterLeaderboards?cursor=AQ&limit=200",
			dataType: "gameCenterLeaderboards",
		},
		{
			args:     []string{"game-center", "leaderboard-sets", "releases", "list"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/gameCenterLeaderboardSets/set-1/releases?cursor=AQ&limit=200",
			dataType: "gameCenterLeaderboardSetReleases",
		},
		{
			args:     []string{"game-center", "leaderboard-sets", "v2", "list"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/gameCenterDetails/detail-1/gameCenterLeaderboardSetsV2?cursor=AQ&limit=200",
			dataType: "gameCenterLeaderboardSets",
		},
		{
			args:     []string{"game-center", "leaderboard-sets", "v2", "localizations", "list"},
			firstURL: "https://api.appstoreconnect.apple.com/v2/gameCenterLeaderboardSetVersions/ver-1/localizations?cursor=AQ&limit=200",
			dataType: "gameCenterLeaderboardSetLocalizations",
		},
		{
			args:     []string{"game-center", "leaderboard-sets", "v2", "members", "list"},
			firstURL: "https://api.appstoreconnect.apple.com/v2/gameCenterLeaderboardSets/set-1/gameCenterLeaderboards?cursor=AQ&limit=200",
			dataType: "gameCenterLeaderboards",
		},
		{
			args:     []string{"game-center", "leaderboard-sets", "v2", "versions", "list"},
			firstURL: "https://api.appstoreconnect.apple.com/v2/gameCenterLeaderboardSets/set-1/versions?cursor=AQ&limit=200",
			dataType: "gameCenterLeaderboardSetVersions",
		},
	})
}
