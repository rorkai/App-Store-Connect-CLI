package shared

import "slices"

// AppFieldsList returns the fields[apps] enum, which every App Store Connect
// operation in docs/openapi/latest.json shares, without the values in exclude.
// The spec also lists appEncryptionDeclarations, but Apple production rejects it
// on every endpoint (400 "is not a valid field name" or HTTP 500, Nov 2026).
func AppFieldsList(exclude ...string) []string {
	return slices.DeleteFunc(appFieldsEnum(), func(value string) bool {
		return slices.Contains(exclude, value)
	})
}

func appFieldsEnum() []string {
	return []string{
		"accessibilityUrl",
		"name",
		"bundleId",
		"sku",
		"primaryLocale",
		"isOrEverWasMadeForKids",
		"subscriptionStatusUrl",
		"subscriptionStatusUrlVersion",
		"subscriptionStatusUrlForSandbox",
		"subscriptionStatusUrlVersionForSandbox",
		"contentRightsDeclaration",
		"streamlinedPurchasingEnabled",
		"accessibilityDeclarations",
		"appStoreIcon",
		"ciProduct",
		"betaTesters",
		"betaGroups",
		"appStoreVersions",
		"appTags",
		"preReleaseVersions",
		"betaAppLocalizations",
		"builds",
		"betaLicenseAgreement",
		"betaAppReviewDetail",
		"appInfos",
		"appClips",
		"appPricePoints",
		"endUserLicenseAgreement",
		"appPriceSchedule",
		"appAvailabilityV2",
		"inAppPurchases",
		"subscriptionGroups",
		"gameCenterEnabledVersions",
		"performanceOverviews",
		"perfPowerMetrics",
		"appCustomProductPages",
		"inAppPurchasesV2",
		"promotedPurchases",
		"appEvents",
		"reviewSubmissions",
		"subscriptionGracePeriod",
		"customerReviews",
		"customerReviewSummarizations",
		"gameCenterDetail",
		"appStoreVersionExperimentsV2",
		"alternativeDistributionKey",
		"analyticsReportRequests",
		"marketplaceSearchDetail",
		"buildUploads",
		"backgroundAssets",
		"assetLibrary",
		"betaFeedbackScreenshotSubmissions",
		"betaFeedbackCrashSubmissions",
		"searchKeywords",
		"webhooks",
		"androidToIosAppMappingDetails",
	}
}
