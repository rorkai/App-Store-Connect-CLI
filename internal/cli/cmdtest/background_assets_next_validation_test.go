package cmdtest

import "testing"

func TestBackgroundAssetsRejectsInvalidNextURL(t *testing.T) {
	runInvalidNextURLCases(t, []invalidNextURLCase{
		{args: []string{"background-assets", "list"}, wantErr: "background-assets list: --next", deps: true},
		{args: []string{"background-assets", "versions", "list"}, wantErr: "background-assets versions list: --next", deps: true},
		{args: []string{"background-assets", "upload-files", "list"}, wantErr: "background-assets upload-files list: --next", deps: true},
	})
}

func TestBackgroundAssetsPaginateFromNext(t *testing.T) {
	runPaginateFromNextCases(t, []paginateFromNextCase{
		{
			args:     []string{"background-assets", "list"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/apps/app-1/backgroundAssets?cursor=AQ&limit=200",
			dataType: "backgroundAssets",
			deps:     true,
		},
		{
			args:     []string{"background-assets", "versions", "list"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/backgroundAssets/asset-1/versions?cursor=AQ&limit=200",
			dataType: "backgroundAssetVersions",
			deps:     true,
		},
		{
			args:     []string{"background-assets", "upload-files", "list"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/backgroundAssetVersions/version-1/backgroundAssetUploadFiles?cursor=AQ&limit=200",
			dataType: "backgroundAssetUploadFiles",
			deps:     true,
		},
	})
}
