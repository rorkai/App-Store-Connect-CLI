package cmdtest

import "testing"

const appTagsTerritoriesDeprecationWarning = "Warning: App-tag territories are deprecated in API 4.5; remove territory selections and lookups. Requests are still forwarded for compatibility.\n"

func TestAppTagsRejectsInvalidNextURL(t *testing.T) {
	runInvalidNextURLCases(t, []invalidNextURLCase{
		{args: []string{"app-tags", "list"}, wantErr: "app-tags list: --next"},
	})
}

func TestAppTagsPaginateFromNext(t *testing.T) {
	runPaginateFromNextCases(t, []paginateFromNextCase{
		{
			args:     []string{"app-tags", "list"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/apps/app-1/appTags?cursor=AQ&limit=200",
			dataType: "appTags",
		},
		{
			args:     []string{"app-tags", "links"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/apps/app-1/relationships/appTags?cursor=AQ&limit=200",
			dataType: "appTags",
		},
		{
			args:       []string{"app-tags", "territories"},
			firstURL:   "https://api.appstoreconnect.apple.com/v1/appTags/tag-1/territories?cursor=AQ&limit=200",
			dataType:   "territories",
			wantStderr: appTagsTerritoriesDeprecationWarning,
		},
		{
			args:       []string{"app-tags", "territories-links"},
			firstURL:   "https://api.appstoreconnect.apple.com/v1/appTags/tag-1/relationships/territories?cursor=AQ&limit=200",
			dataType:   "territories",
			wantStderr: appTagsTerritoriesDeprecationWarning,
		},
	})
}
