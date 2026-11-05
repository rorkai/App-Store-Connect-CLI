package cmdtest

import "testing"

func TestDevicesUsersVersionsRejectsInvalidNextURL(t *testing.T) {
	runInvalidNextURLCases(t, []invalidNextURLCase{
		{args: []string{"devices", "list"}, wantErr: "devices list: --next"},
	})
}

func TestDevicesUsersVersionsPaginateFromNext(t *testing.T) {
	runPaginateFromNextCases(t, []paginateFromNextCase{
		{
			args:     []string{"devices", "list"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/devices?cursor=AQ&limit=200",
			dataType: "devices",
		},
		{
			args:     []string{"users", "list"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/users?cursor=AQ&limit=200",
			dataType: "users",
		},
		{
			args:     []string{"versions", "list"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/apps/app-1/appStoreVersions?cursor=AQ&limit=200",
			dataType: "appStoreVersions",
		},
	})
}
