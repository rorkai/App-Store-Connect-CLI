package cmdtest

import "testing"

func TestCustomPagesListsPaginateFromNext(t *testing.T) {
	runPaginateFromNextCases(t, []paginateFromNextCase{
		{
			args:     []string{"product-pages", "custom-pages", "list"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/apps/app-1/appCustomProductPages?cursor=AQ&limit=200",
			dataType: "appCustomProductPages",
		},
		{
			args:     []string{"product-pages", "custom-pages", "versions", "list", "--custom-page-id", "page-1"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/appCustomProductPages/page-1/appCustomProductPageVersions?cursor=AQ&limit=200",
			dataType: "appCustomProductPageVersions",
		},
		{
			args:     []string{"product-pages", "custom-pages", "localizations", "list", "--custom-page-version-id", "version-1"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/appCustomProductPageVersions/version-1/appCustomProductPageLocalizations?cursor=AQ&limit=200",
			dataType: "appCustomProductPageLocalizations",
		},
		{
			args:     []string{"product-pages", "custom-pages", "localizations", "preview-sets", "list"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/appCustomProductPageLocalizations/loc-1/appPreviewSets?cursor=AQ&limit=200",
			dataType: "appPreviewSets",
		},
		{
			args:     []string{"product-pages", "custom-pages", "localizations", "screenshot-sets", "list"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/appCustomProductPageLocalizations/loc-1/appScreenshotSets?cursor=AQ&limit=200",
			dataType: "appScreenshotSets",
		},
	})
}
