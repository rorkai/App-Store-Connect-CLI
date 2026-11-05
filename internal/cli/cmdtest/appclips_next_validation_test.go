package cmdtest

import "testing"

func TestAppClipsPaginateFromNext(t *testing.T) {
	runPaginateFromNextCases(t, []paginateFromNextCase{
		{
			args:     []string{"app-clips", "list", "--app", "app-1"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/apps/app-1/appClips?cursor=AQ&limit=200",
			dataType: "appClips",
		},
		{
			args:     []string{"app-clips", "advanced-experiences", "list", "--app-clip-id", "clip-1"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/appClips/clip-1/appClipAdvancedExperiences?cursor=AQ&limit=200",
			dataType: "appClipAdvancedExperiences",
		},
		{
			args:     []string{"app-clips", "default-experiences", "list", "--app-clip-id", "clip-1"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/appClips/clip-1/appClipDefaultExperiences?cursor=AQ&limit=200",
			dataType: "appClipDefaultExperiences",
		},
		{
			args:     []string{"app-clips", "invocations", "list", "--build-bundle-id", "bundle-1"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/buildBundles/bundle-1/betaAppClipInvocations?cursor=AQ&limit=200",
			dataType: "betaAppClipInvocations",
		},
	})
}
