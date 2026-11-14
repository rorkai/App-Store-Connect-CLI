package cmdtest

import "testing"

func TestBuildBundlesRejectsInvalidNextURL(t *testing.T) {
	runInvalidNextURLCases(t, []invalidNextURLCase{
		{args: []string{"build-bundles", "file-sizes", "list", "--id", "bundle-1"}, wantErr: "build-bundles file-sizes list: --next", deps: true},
		{args: []string{"build-bundles", "app-clip", "invocations", "list", "--id", "bundle-1"}, wantErr: "build-bundles app-clip invocations list: --next", deps: true},
	})
}

func TestBuildBundlesPaginateFromNext(t *testing.T) {
	runPaginateFromNextCases(t, []paginateFromNextCase{
		{
			args:     []string{"build-bundles", "file-sizes", "list", "--id", "bundle-1"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/buildBundles/bundle-1/buildBundleFileSizes?cursor=AQ&limit=200",
			dataType: "buildBundleFileSizes",
			deps:     true,
		},
		{
			args:     []string{"build-bundles", "app-clip", "invocations", "list", "--id", "bundle-1"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/buildBundles/bundle-1/betaAppClipInvocations?cursor=AQ&limit=200",
			dataType: "betaAppClipInvocations",
			deps:     true,
		},
	})
}
