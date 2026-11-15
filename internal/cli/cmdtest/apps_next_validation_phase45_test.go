package cmdtest

import "testing"

func TestAppsRejectsInvalidNextURL(t *testing.T) {
	runInvalidNextURLCases(t, []invalidNextURLCase{
		{args: []string{"apps", "list"}, wantErr: "apps list: --next", deps: true},
	})
}

func TestAppsPaginateFromNext(t *testing.T) {
	runPaginateFromNextCases(t, []paginateFromNextCase{
		{
			args:     []string{"apps", "list"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/apps?cursor=AQ&limit=200",
			dataType: "apps",
			deps:     true,
		},
		{
			args:     []string{"apps", "search-keywords", "list"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/apps/app-1/searchKeywords?cursor=AQ&limit=200",
			dataType: "appKeywords",
			deps:     true,
		},
		{
			args:     []string{"apps", "app-encryption-declarations", "list"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/appEncryptionDeclarations?cursor=AQ&limit=200",
			dataType: "appEncryptionDeclarations",
			deps:     true,
		},
		{
			args:     []string{"apps", "info", "territory-age-ratings", "list"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/appInfos/app-info-1/territoryAgeRatings?cursor=AQ&limit=200",
			dataType: "appInfoTerritoryAgeRatings",
			deps:     true,
		},
	})
}
