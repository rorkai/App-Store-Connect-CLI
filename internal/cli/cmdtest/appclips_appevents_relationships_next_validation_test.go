package cmdtest

import "testing"

func TestAppClipsAppEventsRelationshipsPaginateFromNext(t *testing.T) {
	runPaginateFromNextCases(t, []paginateFromNextCase{
		{
			args:     []string{"app-clips", "advanced-experiences-links"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/appClips/clip-1/relationships/appClipAdvancedExperiences?cursor=AQ&limit=200",
			dataType: "appClipAdvancedExperiences",
		},
		{
			args:     []string{"app-clips", "default-experiences-links"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/appClips/clip-1/relationships/appClipDefaultExperiences?cursor=AQ&limit=200",
			dataType: "appClipDefaultExperiences",
		},
		{
			args:     []string{"app-clips", "default-experiences", "localizations", "list", "--experience-id", "exp-1"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/appClipDefaultExperiences/exp-1/appClipDefaultExperienceLocalizations?cursor=AQ&limit=200",
			dataType: "appClipDefaultExperienceLocalizations",
		},
		{
			args:     []string{"app-events", "localizations", "screenshots-links"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/appEventLocalizations/loc-1/relationships/appEventScreenshots?cursor=AQ&limit=200",
			dataType: "appEventScreenshots",
		},
		{
			args:     []string{"app-events", "localizations", "video-clips-links"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/appEventLocalizations/loc-1/relationships/appEventVideoClips?cursor=AQ&limit=200",
			dataType: "appEventVideoClips",
		},
	})
}
