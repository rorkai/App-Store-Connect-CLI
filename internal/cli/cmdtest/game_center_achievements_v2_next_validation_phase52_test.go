package cmdtest

import "testing"

func TestGameCenterAchievementsV2ListsRejectsInvalidNextURL(t *testing.T) {
	runInvalidNextURLCases(t, []invalidNextURLCase{
		{args: []string{"game-center", "achievements", "v2", "list"}, wantErr: "game-center achievements v2 list: --next", deps: true},
		{args: []string{"game-center", "achievements", "v2", "versions", "list"}, wantErr: "game-center achievements v2 versions list: --next", deps: true},
		{args: []string{"game-center", "achievements", "v2", "localizations", "list"}, wantErr: "game-center achievements v2 localizations list: --next", deps: true},
	})
}

func TestGameCenterAchievementsV2ListsPaginateFromNext(t *testing.T) {
	runPaginateFromNextCases(t, []paginateFromNextCase{
		{
			args:     []string{"game-center", "achievements", "v2", "list"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/gameCenterDetails/gc-detail-1/gameCenterAchievementsV2?cursor=AQ&limit=200",
			dataType: "gameCenterAchievements",
			deps:     true,
		},
		{
			args:     []string{"game-center", "achievements", "v2", "versions", "list"},
			firstURL: "https://api.appstoreconnect.apple.com/v2/gameCenterAchievements/ach-v2-1/versions?cursor=AQ&limit=200",
			dataType: "gameCenterAchievementVersions",
			deps:     true,
		},
		{
			args:     []string{"game-center", "achievements", "v2", "localizations", "list"},
			firstURL: "https://api.appstoreconnect.apple.com/v2/gameCenterAchievementVersions/ver-v2-1/localizations?cursor=AQ&limit=200",
			dataType: "gameCenterAchievementLocalizations",
			deps:     true,
		},
	})
}
