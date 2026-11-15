package cmdtest

import "testing"

func TestGameCenterVersionsListsRejectsInvalidNextURL(t *testing.T) {
	runInvalidNextURLCases(t, []invalidNextURLCase{
		{args: []string{"game-center", "app-versions", "list"}, wantErr: "game-center app-versions list: --next", deps: true},
		{args: []string{"game-center", "enabled-versions", "list"}, wantErr: "game-center enabled-versions list: --next", deps: true},
		{args: []string{"game-center", "enabled-versions", "compatible-versions"}, wantErr: "game-center enabled-versions compatible-versions: --next", deps: true},
	})
}

func TestGameCenterVersionsListsPaginateFromNext(t *testing.T) {
	runPaginateFromNextCases(t, []paginateFromNextCase{
		{
			args:     []string{"game-center", "app-versions", "list"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/gameCenterDetails/gc-detail-1/gameCenterAppVersions?cursor=AQ&limit=200",
			dataType: "gameCenterAppVersions",
			deps:     true,
		},
		{
			args:     []string{"game-center", "app-versions", "compatibility", "list"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/gameCenterAppVersions/gcav-1/compatibilityVersions?cursor=AQ&limit=200",
			dataType: "gameCenterAppVersions",
			deps:     true,
		},
		{
			args:     []string{"game-center", "enabled-versions", "list"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/apps/app-1/gameCenterEnabledVersions?cursor=AQ&limit=200",
			dataType: "gameCenterEnabledVersions",
			deps:     true,
		},
		{
			args:     []string{"game-center", "enabled-versions", "compatible-versions"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/gameCenterEnabledVersions/enabled-1/compatibleVersions?cursor=AQ&limit=200",
			dataType: "gameCenterEnabledVersions",
			deps:     true,
		},
	})
}
