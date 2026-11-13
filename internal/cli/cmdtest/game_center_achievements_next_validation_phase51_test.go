package cmdtest

import "testing"

func TestGameCenterAchievementsListsRejectsInvalidNextURL(t *testing.T) {
	runInvalidNextURLCases(t, []invalidNextURLCase{
		{args: []string{"game-center", "achievements", "localizations", "list"}, wantErr: "game-center achievements localizations list: --next"},
		{args: []string{"game-center", "achievements", "releases", "list"}, wantErr: "game-center achievements releases list: --next"},
	})
}

func TestGameCenterAchievementsListsPaginateFromNext(t *testing.T) {
	runPaginateFromNextCases(t, []paginateFromNextCase{
		{
			args:     []string{"game-center", "achievements", "list"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/gameCenterDetails/gc-detail-1/gameCenterAchievements?cursor=AQ&limit=200",
			dataType: "gameCenterAchievements",
		},
		{
			args:     []string{"game-center", "achievements", "localizations", "list"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/gameCenterAchievements/ach-1/localizations?cursor=AQ&limit=200",
			dataType: "gameCenterAchievementLocalizations",
		},
		{
			args:     []string{"game-center", "achievements", "releases", "list"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/gameCenterAchievements/ach-1/releases?cursor=AQ&limit=200",
			dataType: "gameCenterAchievementReleases",
		},
	})
}
