package cmdtest

import "testing"

func TestAppEventsPaginateFromNext(t *testing.T) {
	runPaginateFromNextCases(t, []paginateFromNextCase{
		{
			args:     []string{"app-events", "list"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/apps/app-1/appEvents?cursor=AQ&limit=200",
			dataType: "appEvents",
		},
		{
			args:     []string{"app-events", "localizations", "list"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/appEvents/event-1/localizations?cursor=AQ&limit=200",
			dataType: "appEventLocalizations",
		},
		{
			args:     []string{"app-events", "screenshots", "list"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/appEventLocalizations/loc-1/appEventScreenshots?cursor=AQ&limit=200",
			dataType: "appEventScreenshots",
		},
		{
			args:     []string{"app-events", "video-clips", "list"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/appEventLocalizations/loc-1/appEventVideoClips?cursor=AQ&limit=200",
			dataType: "appEventVideoClips",
		},
	})
}
