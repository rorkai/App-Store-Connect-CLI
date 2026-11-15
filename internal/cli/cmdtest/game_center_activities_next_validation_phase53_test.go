package cmdtest

import "testing"

func TestGameCenterActivitiesListsRejectsInvalidNextURL(t *testing.T) {
	runInvalidNextURLCases(t, []invalidNextURLCase{
		{args: []string{"game-center", "activities", "list"}, wantErr: "game-center activities list: --next", deps: true},
		{args: []string{"game-center", "activities", "versions", "list"}, wantErr: "game-center activities versions list: --next", deps: true},
		{args: []string{"game-center", "activities", "localizations", "list"}, wantErr: "game-center activities localizations list: --next", deps: true},
		{args: []string{"game-center", "activities", "releases", "list"}, wantErr: "game-center activities releases list: --next", deps: true},
	})
}

func TestGameCenterActivitiesListsPaginateFromNext(t *testing.T) {
	runPaginateFromNextCases(t, []paginateFromNextCase{
		{
			args:     []string{"game-center", "activities", "list"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/gameCenterDetails/gc-detail-1/gameCenterActivities?cursor=AQ&limit=200",
			dataType: "gameCenterActivities",
			deps:     true,
		},
		{
			args:     []string{"game-center", "activities", "versions", "list"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/gameCenterActivities/activity-1/versions?cursor=AQ&limit=200",
			dataType: "gameCenterActivityVersions",
			deps:     true,
		},
		{
			args:     []string{"game-center", "activities", "localizations", "list"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/gameCenterActivityVersions/version-1/localizations?cursor=AQ&limit=200",
			dataType: "gameCenterActivityLocalizations",
			deps:     true,
		},
		{
			args:     []string{"game-center", "activities", "releases", "list"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/gameCenterDetails/gc-detail-1/activityReleases?cursor=AQ&limit=200",
			dataType: "gameCenterActivityVersionReleases",
			deps:     true,
		},
	})
}
