package shared

import (
	"flag"
	"io"
	"maps"
	"os"
	"slices"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

// includedResource names the sparse-field flag of one resource type and the
// fields[type] values Apple's OpenAPI document publishes for it.
type includedResource struct {
	flagName string
	fields   []string
}

var includedResources = map[string]includedResource{
	"actors":                                 {"actor-fields", []string{"actorType", "userFirstName", "userLastName", "userEmail", "apiKeyId"}},
	"alternativeDistributionPackages":        {"alternative-distribution-package-fields", []string{"sourceFileChecksum", "versions"}},
	"appAssetLibraryImages":                  {"asset-image-fields", []string{"category", "createdDate", "lastModifiedDate", "fileName", "fileSize", "imageAsset", "referenceName", "specId", "state", "stateDetails", "uploadOperations", "placements"}},
	"appAssetLibraryPlacements":              {"placement-fields", []string{"mediaType", "placementType", "placementGroup", "createdDate", "lastModifiedDate", "state", "stateDetails", "image", "video", "appEventLocalization", "appStoreVersionLocalization", "appCustomProductPageLocalization", "appStoreVersionExperimentTreatmentLocalization", "retentionMessageVersion"}},
	"appAssetLibraryVideos":                  {"asset-video-fields", []string{"category", "createdDate", "lastModifiedDate", "fileName", "fileSize", "previewFrameImage", "previewFrameTimeCode", "referenceName", "specId", "state", "stateDetails", "uploadOperations", "videoAsset", "placements"}},
	"appCategories":                          {"category-fields", []string{"platforms", "subcategories", "parent"}},
	"appClipAppStoreReviewDetails":           {"app-clip-review-detail-fields", []string{"invocationUrls", "appClipDefaultExperience"}},
	"appClipDefaultExperienceLocalizations":  {"default-experience-localization-fields", []string{"locale", "subtitle", "appClipDefaultExperience", "appClipHeaderImage"}},
	"appClipDefaultExperiences":              {"app-clip-default-experience-fields", []string{"action", "appClip", "releaseWithAppStoreVersion", "appClipDefaultExperienceLocalizations", "appClipAppStoreReviewDetail"}},
	"appClips":                               {"app-clip-fields", []string{"bundleId", "app", "appClipDefaultExperiences", "appClipAdvancedExperiences"}},
	"appCustomProductPageVersions":           {"custom-product-page-version-fields", []string{"version", "state", "deepLink", "appCustomProductPage", "appCustomProductPageLocalizations"}},
	"appEncryptionDeclarations":              {"encryption-declaration-fields", []string{"appDescription", "createdDate", "usesEncryption", "exempt", "containsProprietaryCryptography", "containsThirdPartyCryptography", "availableOnFrenchStore", "platform", "uploadedDate", "documentUrl", "documentName", "documentType", "appEncryptionDeclarationState", "codeValue", "app", "builds", "appEncryptionDeclarationDocument"}},
	"appEvents":                              {"app-event-fields", []string{"referenceName", "badge", "eventState", "deepLink", "purchaseRequirement", "primaryLocale", "priority", "purpose", "territorySchedules", "archivedTerritorySchedules", "localizations"}},
	"appInfoLocalizations":                   {"app-info-localization-fields", []string{"locale", "name", "subtitle", "privacyPolicyUrl", "privacyChoicesUrl", "privacyPolicyText", "appInfo"}},
	"appPreviewSets":                         {"preview-set-fields", []string{"previewType", "appStoreVersionLocalization", "appCustomProductPageLocalization", "appStoreVersionExperimentTreatmentLocalization", "appPreviews"}},
	"appScreenshotSets":                      {"screenshot-set-fields", []string{"screenshotDisplayType", "appStoreVersionLocalization", "appCustomProductPageLocalization", "appStoreVersionExperimentTreatmentLocalization", "appScreenshots"}},
	"appScreenshots":                         {"screenshot-fields", []string{"fileSize", "fileName", "sourceFileChecksum", "imageAsset", "assetToken", "assetType", "uploadOperations", "assetDeliveryState", "appScreenshotSet"}},
	"appStoreReviewDetails":                  {"review-detail-fields", []string{"contactFirstName", "contactLastName", "contactPhone", "contactEmail", "demoAccountName", "demoAccountPassword", "demoAccountRequired", "notes", "appStoreVersion", "appStoreReviewAttachments"}},
	"appStoreVersionExperiments":             {"experiment-fields", []string{"name", "trafficProportion", "state", "reviewRequired", "startDate", "endDate", "appStoreVersion", "appStoreVersionExperimentTreatments", "platform", "app", "latestControlVersion", "controlVersions"}},
	"appStoreVersionLocalizations":           {"version-localization-fields", []string{"description", "locale", "keywords", "marketingUrl", "promotionalText", "supportUrl", "whatsNew", "appStoreVersion", "appScreenshotSets", "appPreviewSets", "searchKeywords", "placements"}},
	"appStoreVersionPhasedReleases":          {"phased-release-fields", []string{"phasedReleaseState", "startDate", "totalPauseDuration", "currentDayNumber"}},
	"appStoreVersionSubmissions":             {"version-submission-fields", []string{"appStoreVersion"}},
	"appStoreVersions":                       {"app-store-version-fields", []string{"platform", "versionString", "appStoreState", "appVersionState", "copyright", "reviewType", "releaseType", "earliestReleaseDate", "usesIdfa", "downloadable", "createdDate", "app", "appStoreVersionLocalizations", "build", "appStoreVersionPhasedRelease", "gameCenterAppVersion", "routingAppCoverage", "appStoreReviewDetail", "appStoreVersionSubmission", "appClipDefaultExperience", "appStoreVersionExperiments", "appStoreVersionExperimentsV2", "customerReviews", "alternativeDistributionPackage"}},
	"apps":                                   {"app-fields", []string{"accessibilityUrl", "name", "bundleId", "sku", "primaryLocale", "isOrEverWasMadeForKids", "subscriptionStatusUrl", "subscriptionStatusUrlVersion", "subscriptionStatusUrlForSandbox", "subscriptionStatusUrlVersionForSandbox", "contentRightsDeclaration", "streamlinedPurchasingEnabled", "accessibilityDeclarations", "appStoreIcon", "ciProduct", "betaTesters", "betaGroups", "appStoreVersions", "appTags", "preReleaseVersions", "betaAppLocalizations", "builds", "betaLicenseAgreement", "betaAppReviewDetail", "appInfos", "appClips", "appPricePoints", "endUserLicenseAgreement", "appPriceSchedule", "appAvailabilityV2", "inAppPurchases", "subscriptionGroups", "retentionMessages", "gameCenterEnabledVersions", "performanceOverviews", "perfPowerMetrics", "appCustomProductPages", "inAppPurchasesV2", "promotedPurchases", "appEvents", "reviewSubmissions", "subscriptionGracePeriod", "customerReviews", "customerReviewSummarizations", "gameCenterDetail", "appStoreVersionExperimentsV2", "alternativeDistributionKey", "analyticsReportRequests", "marketplaceSearchDetail", "buildUploads", "backgroundAssets", "assetLibrary", "betaFeedbackScreenshotSubmissions", "betaFeedbackCrashSubmissions", "searchKeywords", "webhooks", "androidToIosAppMappingDetails"}}, // Apple rejects spec value appEncryptionDeclarations: 400 "is not a valid field name" or HTTP 500.
	"backgroundAssetVersions":                {"background-asset-version-fields", []string{"createdDate", "platforms", "state", "stateDetails", "version", "locale", "backgroundAsset", "internalBetaRelease", "externalBetaRelease", "appStoreRelease", "assetFile", "manifestFile", "backgroundAssetUploadFiles"}},
	"betaAppReviewSubmissions":               {"beta-app-review-submission-fields", []string{"betaReviewState", "submittedDate", "build"}},
	"betaBuildLocalizations":                 {"beta-build-localization-fields", []string{"whatsNew", "locale", "build"}},
	"betaGroups":                             {"beta-group-fields", []string{"name", "createdDate", "isInternalGroup", "hasAccessToAllBuilds", "publicLinkEnabled", "publicLinkId", "publicLinkLimitEnabled", "publicLinkLimit", "publicLink", "feedbackEnabled", "iosBuildsAvailableForAppleSiliconMac", "iosBuildsAvailableForAppleVision", "app", "builds", "betaTesters", "betaRecruitmentCriteria", "betaRecruitmentCriterionCompatibleBuildCheck"}},
	"betaTesters":                            {"tester-fields", []string{"firstName", "lastName", "email", "inviteType", "state", "appDevices", "apps", "betaGroups", "builds"}},
	"buildBetaDetails":                       {"build-beta-detail-fields", []string{"autoNotifyEnabled", "internalBuildState", "externalBuildState", "build"}},
	"buildBundles":                           {"build-bundle-fields", []string{"bundleId", "bundleType", "sdkBuild", "platformBuild", "fileName", "hasSirikit", "hasOnDemandResources", "hasPrerenderedIcon", "usesLocationServices", "isIosBuildMacAppStoreCompatible", "includesSymbols", "dSYMUrl", "supportedArchitectures", "requiredCapabilities", "deviceProtocols", "locales", "entitlements", "baDownloadAllowance", "baMaxInstallSize", "minimumOsVersion", "appClipDomainCacheStatus", "appClipDomainDebugStatus", "betaAppClipInvocations", "buildBundleFileSizes"}},
	"buildIcons":                             {"build-icon-fields", []string{"iconAsset", "iconType", "masked", "name"}},
	"buildUploads":                           {"build-upload-fields", []string{"cfBundleShortVersionString", "cfBundleVersion", "createdDate", "state", "platform", "uploadedDate", "build", "assetFile", "assetDescriptionFile", "assetSpiFile", "buildUploadFiles"}},
	"builds":                                 {"build-fields", []string{"version", "uploadedDate", "expirationDate", "expired", "minOsVersion", "lsMinimumSystemVersion", "computedMinMacOsVersion", "computedMinVisionOsVersion", "iconAssetToken", "processingState", "buildAudienceType", "usesNonExemptEncryption", "preReleaseVersion", "individualTesters", "betaGroups", "betaBuildLocalizations", "appEncryptionDeclaration", "betaAppReviewSubmission", "app", "buildBetaDetail", "appStoreVersion", "icons", "buildBundles", "buildUpload", "perfPowerMetrics", "diagnosticSignatures"}},
	"bundleIds":                              {"bundle-id-fields", []string{"name", "platform", "identifier", "seedId", "profiles", "bundleIdCapabilities"}}, // Apple rejects spec value app: 400 "'app' does not exist".
	"certificates":                           {"certificate-fields", []string{"name", "certificateType", "displayName", "serialNumber", "platform", "expirationDate", "certificateContent", "activated", "passTypeId"}},
	"devices":                                {"device-fields", []string{"name", "platform", "udid", "deviceClass", "status", "model", "addedDate"}},
	"gameCenterAchievementVersions":          {"game-center-achievement-version-fields", []string{"version", "state", "achievement", "localizations"}},
	"gameCenterActivityVersions":             {"game-center-activity-version-fields", []string{"version", "state", "fallbackUrl", "activity", "localizations", "defaultImage", "releases"}},
	"gameCenterAppVersions":                  {"game-center-app-version-fields", []string{"enabled", "compatibilityVersions", "appStoreVersion"}},
	"gameCenterChallengeVersions":            {"game-center-challenge-version-fields", []string{"version", "state", "challenge", "localizations", "releases", "defaultImage"}},
	"gameCenterLeaderboardSetVersions":       {"game-center-leaderboard-set-version-fields", []string{"version", "state", "leaderboardSet", "localizations"}},
	"gameCenterLeaderboardVersions":          {"game-center-leaderboard-version-fields", []string{"version", "state", "leaderboard", "localizations"}},
	"inAppPurchaseAppStoreReviewScreenshots": {"iap-review-screenshot-fields", []string{"fileSize", "fileName", "sourceFileChecksum", "imageAsset", "assetToken", "assetType", "uploadOperations", "assetDeliveryState", "inAppPurchaseV2"}},
	"inAppPurchaseAvailabilities":            {"iap-availability-fields", []string{"availableInNewTerritories", "availableTerritories"}},
	"inAppPurchaseContents":                  {"iap-content-fields", []string{"fileName", "fileSize", "url", "lastModifiedDate", "inAppPurchaseV2"}},
	"inAppPurchaseImages":                    {"iap-image-fields", []string{"fileSize", "fileName", "sourceFileChecksum", "assetToken", "imageAsset", "uploadOperations", "state", "inAppPurchase"}},
	"inAppPurchaseLocalizations":             {"iap-localization-fields", []string{"name", "locale", "description", "state", "inAppPurchaseV2"}},
	"inAppPurchaseOfferCodes":                {"iap-offer-code-fields", []string{"name", "customerEligibilities", "productionCodeCount", "sandboxCodeCount", "active", "oneTimeUseCodes", "customCodes", "prices"}},
	"inAppPurchasePricePoints":               {"iap-price-point-fields", []string{"customerPrice", "proceeds", "territory", "equalizations"}},
	"inAppPurchasePriceSchedules":            {"iap-price-schedule-fields", []string{"baseTerritory", "manualPrices", "automaticPrices"}},
	"passTypeIds":                            {"pass-type-id-fields", []string{"name", "identifier", "certificates"}},
	"preReleaseVersions":                     {"pre-release-version-fields", []string{"version", "platform", "builds", "app"}},
	"promotedPurchases":                      {"promoted-purchase-fields", []string{"visibleForAllUsers", "enabled", "state", "inAppPurchaseV2", "subscription"}},
	"retentionMessageLocalizations":          {"retention-message-localization-fields", []string{"locale", "header", "body", "retentionMessageVersion"}},
	"retentionMessageVersions":               {"retention-message-version-fields", []string{"state", "retentionMessage", "localizations", "placement"}},
	"retentionMessages":                      {"retention-message-fields", []string{"referenceName", "versions", "subscriptions"}},
	"retentionOfferPrices":                   {"price-fields", []string{"territory", "subscriptionPricePoint"}},
	"routingAppCoverages":                    {"routing-app-coverage-fields", []string{"fileSize", "fileName", "sourceFileChecksum", "uploadOperations", "assetDeliveryState", "appStoreVersion"}},
	"subscriptionPricePoints":                {"price-point-fields", []string{"customerPrice", "proceeds", "proceedsYear2", "territory", "equalizations", "adjustedEqualizations"}},
	"subscriptions":                          {"subscription-fields", []string{"name", "productId", "familySharable", "state", "subscriptionPeriod", "reviewNote", "groupLevel", "multiSeatStatus", "marketSettings", "subscriptionLocalizations", "appStoreReviewScreenshot", "group", "introductoryOffers", "promotionalOffers", "offerCodes", "prices", "pricePoints", "promotedPurchase", "subscriptionAvailability", "winBackOffers", "images", "planAvailabilities", "versions", "retentionOffers", "retentionMessage", "retentionMessageOffers"}},
	"territories":                            {"territory-fields", []string{"currency"}},
}

// IncludedResourceFields returns the fields[type] values published for a resource type.
func IncludedResourceFields(resourceType string) []string {
	return slices.Clone(includedResources[resourceType].fields)
}

type includedFieldFlag struct {
	name         string
	resourceType string
	includes     []string
	fields       []string
	value        *string
}

// IncludedFieldFlags holds the sparse-field flags of the resources a command
// can include.
type IncludedFieldFlags struct {
	fs    *flag.FlagSet
	flags []includedFieldFlag
}

// BindIncludedFieldFlags binds one sparse-field flag per resource type.
// relationships maps each --include value to the resource type it returns.
func BindIncludedFieldFlags(fs *flag.FlagSet, relationships map[string]string) *IncludedFieldFlags {
	return BindIncludedFieldFlagsExcept(fs, relationships, nil)
}

// BindIncludedFieldFlagsExcept is BindIncludedFieldFlags without the
// fields[type] values, keyed by resource type, that Apple rejects on this
// command.
func BindIncludedFieldFlagsExcept(fs *flag.FlagSet, relationships map[string]string, rejected map[string][]string) *IncludedFieldFlags {
	includes := map[string][]string{}
	for _, include := range slices.Sorted(maps.Keys(relationships)) {
		resourceType := relationships[include]
		includes[resourceType] = append(includes[resourceType], include)
	}
	bound := &IncludedFieldFlags{fs: fs}
	for _, resourceType := range slices.Sorted(maps.Keys(includes)) {
		name := includedResources[resourceType].flagName
		fields := slices.DeleteFunc(slices.Clone(includedResources[resourceType].fields), func(value string) bool {
			return slices.Contains(rejected[resourceType], value)
		})
		usage := "fields[" + resourceType + "], comma-separated (requires --include " + strings.Join(includes[resourceType], " or ") + ")"
		bound.flags = append(bound.flags, includedFieldFlag{
			name:         name,
			resourceType: resourceType,
			includes:     includes[resourceType],
			fields:       fields,
			value:        fs.String(name, "", usage),
		})
	}
	return bound
}

// Resolve validates the provided flags against --next and the resolved
// includes, and returns the requested fields per resource type.
func (f *IncludedFieldFlags) Resolve(command, next string, includes []string) (asc.IncludedFields, error) {
	return f.resolveTo(os.Stderr, command, next, includes)
}

// ResolveWith is Resolve with the "Error:" line written to d's stderr.
func (f *IncludedFieldFlags) ResolveWith(d *Deps, command, next string, includes []string) (asc.IncludedFields, error) {
	return f.resolveTo(d.IO.ErrOut, command, next, includes)
}

func (f *IncludedFieldFlags) resolveTo(stderr io.Writer, command, next string, includes []string) (asc.IncludedFields, error) {
	names := make([]string, 0, len(f.flags))
	for _, field := range f.flags {
		names = append(names, field.name)
	}
	if err := rejectNextFlagConflicts(stderr, f.fs, next, command, names...); err != nil {
		return nil, err
	}

	provided := map[string]bool{}
	f.fs.Visit(func(parsed *flag.Flag) { provided[parsed.Name] = true })

	resolved := asc.IncludedFields{}
	for _, field := range f.flags {
		if !provided[field.name] {
			continue
		}
		flagName := "--" + field.name
		values, err := NormalizeSelection(*field.value, field.fields, flagName)
		if err != nil {
			return nil, WithDiagnostic(usageErrorfTo(stderr, "%s: %v", command, err), DiagnosticInvalidInput, flagName)
		}
		if len(values) == 0 {
			return nil, WithDiagnostic(usageErrorfTo(stderr, "%s: %s must not be empty", command, flagName), DiagnosticInvalidInput, flagName)
		}
		if !slices.ContainsFunc(field.includes, func(include string) bool { return HasInclude(includes, include) }) {
			return nil, WithDiagnostic(usageErrorfTo(stderr, "%s: %s requires --include %s", command, flagName, strings.Join(field.includes, " or ")), DiagnosticInvalidInput, flagName)
		}
		resolved[field.resourceType] = values
	}
	return resolved, nil
}
