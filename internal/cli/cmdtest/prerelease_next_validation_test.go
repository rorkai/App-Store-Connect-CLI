package cmdtest

import "testing"

func TestPreReleaseListsRejectsInvalidNextURL(t *testing.T) {
	runInvalidNextURLCases(t, []invalidNextURLCase{
		{args: []string{"testflight", "pre-release", "list"}, wantErr: "testflight pre-release list: --next", deps: true},
		{args: []string{"testflight", "pre-release", "builds", "list", "--id", "pr-1"}, wantErr: "testflight pre-release builds list: --next", deps: true},
		{args: []string{"testflight", "pre-release", "links", "view", "--type", "builds"}, wantErr: "testflight pre-release links view: --next", deps: true},
	})
}

func TestPreReleaseListsPaginateFromNext(t *testing.T) {
	runPaginateFromNextCases(t, []paginateFromNextCase{
		{
			args:     []string{"testflight", "pre-release", "list"},
			deps:     true,
			firstURL: "https://api.appstoreconnect.apple.com/v1/apps/app-1/preReleaseVersions?cursor=AQ&limit=200",
			dataType: "preReleaseVersions",
		},
		{
			args:     []string{"testflight", "pre-release", "builds", "list", "--id", "pr-1"},
			deps:     true,
			firstURL: "https://api.appstoreconnect.apple.com/v1/preReleaseVersions/pr-1/builds?cursor=AQ&limit=200",
			dataType: "builds",
		},
		{
			args:     []string{"testflight", "pre-release", "links", "view", "--type", "builds"},
			deps:     true,
			firstURL: "https://api.appstoreconnect.apple.com/v1/preReleaseVersions/pr-1/relationships/builds?cursor=AQ&limit=200",
			dataType: "builds",
		},
	})
}
