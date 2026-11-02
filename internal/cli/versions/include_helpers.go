package versions

import "github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"

func normalizeAppStoreVersionInclude(value string) ([]string, error) {
	return shared.NormalizeSelection(value, appStoreVersionIncludeList(), "--include")
}

func normalizeAppStoreVersionsInclude(value string) ([]string, error) {
	return shared.NormalizeSelection(value, appStoreVersionsIncludeList(), "--include")
}

// appStoreVersionIncludedResources maps each --include value to the resource it
// returns.
var appStoreVersionIncludedResources = map[string]string{
	"app":                            "apps",
	"appStoreVersionLocalizations":   "appStoreVersionLocalizations",
	"build":                          "builds",
	"appStoreVersionPhasedRelease":   "appStoreVersionPhasedReleases",
	"gameCenterAppVersion":           "gameCenterAppVersions",
	"routingAppCoverage":             "routingAppCoverages",
	"appStoreReviewDetail":           "appStoreReviewDetails",
	"appStoreVersionSubmission":      "appStoreVersionSubmissions",
	"appClipDefaultExperience":       "appClipDefaultExperiences",
	"appStoreVersionExperiments":     "appStoreVersionExperiments",
	"appStoreVersionExperimentsV2":   "appStoreVersionExperiments",
	"alternativeDistributionPackage": "alternativeDistributionPackages",
}

// appStoreVersionIncludeList returns the include values a single app store
// version accepts. ageRatingDeclaration has no include relationship on the API
// and is served through a separate request.
func appStoreVersionIncludeList() []string {
	return append([]string{"ageRatingDeclaration"}, appStoreVersionsIncludeList()...)
}

// appStoreVersionsIncludeList returns the include values the app store versions
// collection endpoint accepts.
func appStoreVersionsIncludeList() []string {
	return []string{
		"app",
		"appStoreVersionLocalizations",
		"build",
		"appStoreVersionPhasedRelease",
		"gameCenterAppVersion",
		"routingAppCoverage",
		"appStoreReviewDetail",
		"appStoreVersionSubmission",
		"appClipDefaultExperience",
		"appStoreVersionExperiments",
		"appStoreVersionExperimentsV2",
		"alternativeDistributionPackage",
	}
}
