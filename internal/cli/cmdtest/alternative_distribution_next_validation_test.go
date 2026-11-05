package cmdtest

import "testing"

func TestAlternativeDistributionPaginateFromNext(t *testing.T) {
	runPaginateFromNextCases(t, []paginateFromNextCase{
		{
			args:     []string{"alternative-distribution", "domains", "list"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/alternativeDistributionDomains?cursor=AQ&limit=200",
			dataType: "alternativeDistributionDomains",
		},
		{
			args:     []string{"alternative-distribution", "keys", "list"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/alternativeDistributionKeys?cursor=AQ&limit=200",
			dataType: "alternativeDistributionKeys",
		},
		{
			args:     []string{"alternative-distribution", "packages", "versions", "list", "--package-id", "pkg-1"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/alternativeDistributionPackages/pkg-1/versions?cursor=AQ&limit=200",
			dataType: "alternativeDistributionPackageVersions",
		},
		{
			args:     []string{"alternative-distribution", "packages", "versions", "deltas", "--version-id", "ver-1"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/alternativeDistributionPackageVersions/ver-1/deltas?cursor=AQ&limit=200",
			dataType: "alternativeDistributionPackageDeltas",
		},
		{
			args:     []string{"alternative-distribution", "packages", "versions", "variants", "--version-id", "ver-1"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/alternativeDistributionPackageVersions/ver-1/variants?cursor=AQ&limit=200",
			dataType: "alternativeDistributionPackageVariants",
		},
	})
}
