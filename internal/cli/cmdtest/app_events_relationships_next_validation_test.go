package cmdtest

import "testing"

func TestAppEventsRelationshipsPaginateFromNext(t *testing.T) {
	runPaginateFromNextCases(t, []paginateFromNextCase{
		{
			args:     []string{"app-events", "links"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/appEvents/event-1/relationships/localizations?cursor=AQ&limit=200",
			dataType: "appEventLocalizations",
			deps:     true,
		},
		{
			args:     []string{"app-events", "screenshots", "links"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/appEventLocalizations/loc-1/relationships/appEventScreenshots?cursor=AQ&limit=200",
			dataType: "appEventScreenshots",
			deps:     true,
		},
		{
			args:     []string{"app-events", "video-clips", "links"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/appEventLocalizations/loc-1/relationships/appEventVideoClips?cursor=AQ&limit=200",
			dataType: "appEventVideoClips",
			deps:     true,
		},
		{
			args:     []string{"app-events", "localizations", "screenshots", "list", "--localization-id", "loc-1"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/appEventLocalizations/loc-1/appEventScreenshots?cursor=AQ&limit=200",
			dataType: "appEventScreenshots",
			deps:     true,
		},
		{
			args:     []string{"app-events", "localizations", "video-clips", "list", "--localization-id", "loc-1"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/appEventLocalizations/loc-1/appEventVideoClips?cursor=AQ&limit=200",
			dataType: "appEventVideoClips",
			deps:     true,
		},
	})
}
