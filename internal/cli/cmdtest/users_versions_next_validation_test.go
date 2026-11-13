package cmdtest

import "testing"

func TestUsersVersionsListsRejectsInvalidNextURL(t *testing.T) {
	runInvalidNextURLCases(t, []invalidNextURLCase{
		{args: []string{"users", "visible-apps", "list"}, wantErr: "users visible-apps list: --next"},
		{args: []string{"versions", "links", "--type", "appStoreVersionExperiments"}, wantErr: "versions links: --next"},
	})
}

func TestUsersVersionsListsPaginateFromNext(t *testing.T) {
	runPaginateFromNextCases(t, []paginateFromNextCase{
		{
			args:     []string{"users", "invites", "list"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/userInvitations?cursor=AQ&limit=200",
			dataType: "userInvitations",
		},
		{
			args:     []string{"users", "visible-apps", "list"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/users/user-1/visibleApps?cursor=AQ&limit=200",
			dataType: "apps",
		},
		{
			args:     []string{"users", "visible-apps", "view"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/users/user-1/relationships/visibleApps?cursor=AQ&limit=200",
			dataType: "apps",
		},
		{
			args:     []string{"users", "invites", "visible-apps", "list"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/userInvitations/invite-1/visibleApps?cursor=AQ&limit=200",
			dataType: "apps",
		},
		{
			args:     []string{"versions", "links", "--type", "appStoreVersionExperiments"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/appStoreVersions/ver-1/relationships/appStoreVersionExperiments?cursor=AQ&limit=200",
			dataType: "appStoreVersionExperiments",
		},
		{
			args:     []string{"versions", "experiments-v2", "list"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/appStoreVersions/ver-1/appStoreVersionExperimentsV2?cursor=AQ&limit=200",
			dataType: "appStoreVersionExperimentsV2",
		},
		{
			args:     []string{"versions", "customer-reviews", "list"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/appStoreVersions/ver-1/customerReviews?cursor=AQ&limit=200",
			dataType: "customerReviews",
		},
	})
}
