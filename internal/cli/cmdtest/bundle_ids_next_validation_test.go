package cmdtest

import "testing"

func TestBundleIDsRejectsInvalidNextURL(t *testing.T) {
	runInvalidNextURLCases(t, []invalidNextURLCase{
		{args: []string{"bundle-ids", "list"}, wantErr: "bundle-ids list: --next"},
		{args: []string{"bundle-ids", "capabilities", "list"}, wantErr: "bundle-ids capabilities list: --next"},
		{args: []string{"bundle-ids", "profiles", "list"}, wantErr: "bundle-ids profiles list: --next"},
	})
}

func TestBundleIDsPaginateFromNext(t *testing.T) {
	runPaginateFromNextCases(t, []paginateFromNextCase{
		{
			args:     []string{"bundle-ids", "list"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/bundleIds?cursor=AQ&limit=200",
			dataType: "bundleIds",
		},
		{
			args:     []string{"bundle-ids", "capabilities", "list"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/bundleIds/bundle-1/bundleIdCapabilities?cursor=AQ",
			dataType: "bundleIdCapabilities",
		},
		{
			args:     []string{"bundle-ids", "profiles", "list"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/bundleIds/bundle-1/profiles?cursor=AQ&limit=200",
			dataType: "profiles",
		},
	})
}
