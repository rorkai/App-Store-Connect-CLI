package asc

// This file provides type aliases to internal/asc/types for backward compatibility.
// All callers continue to use asc.ResourceType, asc.Resource[T], etc. without changes.
// The canonical definitions now live in internal/asc/types/.

import "github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc/types"

// Core types — aliases to types package.
type (
	ResourceType        = types.ResourceType
	Links               = types.Links
	Platform            = types.Platform
	ChecksumAlgorithm   = types.ChecksumAlgorithm
	AssetType           = types.AssetType
	UTI                 = types.UTI
	Relationship        = types.Relationship
	RelationshipList    = types.RelationshipList
	RelationshipRequest = types.RelationshipRequest
	RelationshipData    = types.RelationshipData
	ResourceData        = types.ResourceData
)

// Generic resource types — aliases to types package.
type (
	Resource[T any]               = types.Resource[T]
	Response[T any]               = types.Response[T]
	SingleResponse[T any]         = types.SingleResponse[T]
	LinkagesResponse              = types.LinkagesResponse
	SingleResourceResponse[T any] = types.SingleResourceResponse[T]
)

// Pagination interface — alias to types package.
type PaginatedResponse = types.PaginatedResponse

// ParsePagingTotal extracts the total count from a response's paging metadata.
var ParsePagingTotal = types.ParsePagingTotal

// ParsePagingTotalOK extracts the total count from a response's paging metadata.
// Returns (total, true) when the field is present, (0, false) when absent or unparseable.
var ParsePagingTotalOK = types.ParsePagingTotalOK

// ResourceType constants — re-exported from types package.
const (
	ResourceTypeApps                                            = types.ResourceTypeApps
	ResourceTypeAppTags                                         = types.ResourceTypeAppTags
	ResourceTypeBundleIds                                       = types.ResourceTypeBundleIds
	ResourceTypeBundleIdCapabilities                            = types.ResourceTypeBundleIdCapabilities
	ResourceTypeMerchantIds                                     = types.ResourceTypeMerchantIds
	ResourceTypePassTypeIds                                     = types.ResourceTypePassTypeIds
	ResourceTypeAppCategories                                   = types.ResourceTypeAppCategories
	ResourceTypeAppAvailabilities                               = types.ResourceTypeAppAvailabilities
	ResourceTypeAppPricePoints                                  = types.ResourceTypeAppPricePoints
	ResourceTypeAppPriceSchedules                               = types.ResourceTypeAppPriceSchedules
	ResourceTypeAppPrices                                       = types.ResourceTypeAppPrices
	ResourceTypeBuilds                                          = types.ResourceTypeBuilds
	ResourceTypeBuildBundles                                    = types.ResourceTypeBuildBundles
	ResourceTypeBuildBundleFileSizes                            = types.ResourceTypeBuildBundleFileSizes
	ResourceTypeBuildIcons                                      = types.ResourceTypeBuildIcons
	ResourceTypeBuildUploads                                    = types.ResourceTypeBuildUploads
	ResourceTypeBuildUploadFiles                                = types.ResourceTypeBuildUploadFiles
	ResourceTypeCertificates                                    = types.ResourceTypeCertificates
	ResourceTypeAppStoreVersions                                = types.ResourceTypeAppStoreVersions
	ResourceTypeAppClips                                        = types.ResourceTypeAppClips
	ResourceTypeAppClipDefaultExperiences                       = types.ResourceTypeAppClipDefaultExperiences
	ResourceTypeAppClipDefaultExperienceLocalizations           = types.ResourceTypeAppClipDefaultExperienceLocalizations
	ResourceTypeAppClipAdvancedExperiences                      = types.ResourceTypeAppClipAdvancedExperiences
	ResourceTypeAppClipAdvancedExperienceImages                 = types.ResourceTypeAppClipAdvancedExperienceImages
	ResourceTypeAppClipAdvancedExperienceLocalizations          = types.ResourceTypeAppClipAdvancedExperienceLocalizations
	ResourceTypeAppClipHeaderImages                             = types.ResourceTypeAppClipHeaderImages
	ResourceTypeAppClipAppStoreReviewDetails                    = types.ResourceTypeAppClipAppStoreReviewDetails
	ResourceTypeBackgroundAssets                                = types.ResourceTypeBackgroundAssets
	ResourceTypeBackgroundAssetVersions                         = types.ResourceTypeBackgroundAssetVersions
	ResourceTypeBackgroundAssetUploadFiles                      = types.ResourceTypeBackgroundAssetUploadFiles
	ResourceTypeBackgroundAssetVersionAppStoreReleases          = types.ResourceTypeBackgroundAssetVersionAppStoreReleases
	ResourceTypeBackgroundAssetVersionExternalBetaReleases      = types.ResourceTypeBackgroundAssetVersionExternalBetaReleases
	ResourceTypeBackgroundAssetVersionInternalBetaReleases      = types.ResourceTypeBackgroundAssetVersionInternalBetaReleases
	ResourceTypeRoutingAppCoverages                             = types.ResourceTypeRoutingAppCoverages
	ResourceTypeAppEncryptionDeclarations                       = types.ResourceTypeAppEncryptionDeclarations
	ResourceTypeAppEncryptionDeclarationDocuments               = types.ResourceTypeAppEncryptionDeclarationDocuments
	ResourceTypeAppStoreVersionPromotions                       = types.ResourceTypeAppStoreVersionPromotions
	ResourceTypeAppStoreVersionExperimentTreatments             = types.ResourceTypeAppStoreVersionExperimentTreatments
	ResourceTypeAppStoreVersionExperimentTreatmentLocalizations = types.ResourceTypeAppStoreVersionExperimentTreatmentLocalizations
	ResourceTypePreReleaseVersions                              = types.ResourceTypePreReleaseVersions
	ResourceTypeAppStoreVersionSubmissions                      = types.ResourceTypeAppStoreVersionSubmissions
	ResourceTypeAppScreenshotSets                               = types.ResourceTypeAppScreenshotSets
	ResourceTypeAppScreenshots                                  = types.ResourceTypeAppScreenshots
	ResourceTypeAppPreviewSets                                  = types.ResourceTypeAppPreviewSets
	ResourceTypeAppPreviews                                     = types.ResourceTypeAppPreviews
	ResourceTypeReviewSubmissions                               = types.ResourceTypeReviewSubmissions
	ResourceTypeReviewSubmissionItems                           = types.ResourceTypeReviewSubmissionItems
	ResourceTypeAppCustomProductPages                           = types.ResourceTypeAppCustomProductPages
	ResourceTypeAppCustomProductPageVersions                    = types.ResourceTypeAppCustomProductPageVersions
	ResourceTypeAppCustomProductPageLocalizations               = types.ResourceTypeAppCustomProductPageLocalizations
	ResourceTypeAppEvents                                       = types.ResourceTypeAppEvents
	ResourceTypeAppEventLocalizations                           = types.ResourceTypeAppEventLocalizations
	ResourceTypeAppEventScreenshots                             = types.ResourceTypeAppEventScreenshots
	ResourceTypeAppEventVideoClips                              = types.ResourceTypeAppEventVideoClips
	ResourceTypeAppStoreVersionExperiments                      = types.ResourceTypeAppStoreVersionExperiments
	ResourceTypeBetaGroups                                      = types.ResourceTypeBetaGroups
	ResourceTypeBetaTesters                                     = types.ResourceTypeBetaTesters
	ResourceTypeBetaTesterInvitations                           = types.ResourceTypeBetaTesterInvitations
	ResourceTypeBetaAppReviewDetails                            = types.ResourceTypeBetaAppReviewDetails
	ResourceTypeBetaAppReviewSubmissions                        = types.ResourceTypeBetaAppReviewSubmissions
	ResourceTypeBetaLicenseAgreements                           = types.ResourceTypeBetaLicenseAgreements
	ResourceTypeBetaAppClipInvocations                          = types.ResourceTypeBetaAppClipInvocations
	ResourceTypeBetaAppClipInvocationLocalizations              = types.ResourceTypeBetaAppClipInvocationLocalizations
	ResourceTypeBuildBetaDetails                                = types.ResourceTypeBuildBetaDetails
	ResourceTypeBuildBetaNotifications                          = types.ResourceTypeBuildBetaNotifications
	ResourceTypeBetaAppLocalizations                            = types.ResourceTypeBetaAppLocalizations
	ResourceTypeBetaBuildLocalizations                          = types.ResourceTypeBetaBuildLocalizations
	ResourceTypeBetaRecruitmentCriteria                         = types.ResourceTypeBetaRecruitmentCriteria
	ResourceTypeBetaRecruitmentCriterionOptions                 = types.ResourceTypeBetaRecruitmentCriterionOptions
	ResourceTypeSandboxTesters                                  = types.ResourceTypeSandboxTesters
	ResourceTypeSandboxTestersClearHistory                      = types.ResourceTypeSandboxTestersClearHistory
	ResourceTypeAppClipDomainStatuses                           = types.ResourceTypeAppClipDomainStatuses
	ResourceTypeAppStoreVersionLocalizations                    = types.ResourceTypeAppStoreVersionLocalizations
	ResourceTypeAppKeywords                                     = types.ResourceTypeAppKeywords
	ResourceTypeAppInfoLocalizations                            = types.ResourceTypeAppInfoLocalizations
	ResourceTypeAppInfos                                        = types.ResourceTypeAppInfos
	ResourceTypeAgeRatingDeclarations                           = types.ResourceTypeAgeRatingDeclarations
	ResourceTypeAccessibilityDeclarations                       = types.ResourceTypeAccessibilityDeclarations
	ResourceTypeDiagnosticSignatures                            = types.ResourceTypeDiagnosticSignatures
	ResourceTypeAndroidToIosAppMappingDetails                   = types.ResourceTypeAndroidToIosAppMappingDetails
	ResourceTypeAnalyticsReportRequests                         = types.ResourceTypeAnalyticsReportRequests
	ResourceTypeAnalyticsReports                                = types.ResourceTypeAnalyticsReports
	ResourceTypeAnalyticsReportInstances                        = types.ResourceTypeAnalyticsReportInstances
	ResourceTypeAnalyticsReportSegments                         = types.ResourceTypeAnalyticsReportSegments
	ResourceTypeInAppPurchases                                  = types.ResourceTypeInAppPurchases
	ResourceTypeInAppPurchaseLocalizations                      = types.ResourceTypeInAppPurchaseLocalizations
	ResourceTypeInAppPurchaseImages                             = types.ResourceTypeInAppPurchaseImages
	ResourceTypeInAppPurchaseAppStoreReviewScreenshots          = types.ResourceTypeInAppPurchaseAppStoreReviewScreenshots
	ResourceTypeInAppPurchaseAvailabilities                     = types.ResourceTypeInAppPurchaseAvailabilities
	ResourceTypeInAppPurchaseContents                           = types.ResourceTypeInAppPurchaseContents
	ResourceTypeInAppPurchasePricePoints                        = types.ResourceTypeInAppPurchasePricePoints
	ResourceTypeInAppPurchasePriceSchedules                     = types.ResourceTypeInAppPurchasePriceSchedules
	ResourceTypeInAppPurchasePrices                             = types.ResourceTypeInAppPurchasePrices
	ResourceTypeInAppPurchaseOfferCodes                         = types.ResourceTypeInAppPurchaseOfferCodes
	ResourceTypeInAppPurchaseOfferCodeCustomCodes               = types.ResourceTypeInAppPurchaseOfferCodeCustomCodes
	ResourceTypeInAppPurchaseOfferCodeOneTimeUseCodes           = types.ResourceTypeInAppPurchaseOfferCodeOneTimeUseCodes
	ResourceTypeInAppPurchaseOfferPrices                        = types.ResourceTypeInAppPurchaseOfferPrices
	ResourceTypeInAppPurchaseSubmissions                        = types.ResourceTypeInAppPurchaseSubmissions
	ResourceTypeSubscriptionGroups                              = types.ResourceTypeSubscriptionGroups
	ResourceTypeSubscriptionGroupLocalizations                  = types.ResourceTypeSubscriptionGroupLocalizations
	ResourceTypeSubscriptionGroupSubmissions                    = types.ResourceTypeSubscriptionGroupSubmissions
	ResourceTypeSubscriptions                                   = types.ResourceTypeSubscriptions
	ResourceTypeSubscriptionLocalizations                       = types.ResourceTypeSubscriptionLocalizations
	ResourceTypeSubscriptionImages                              = types.ResourceTypeSubscriptionImages
	ResourceTypeSubscriptionIntroductoryOffers                  = types.ResourceTypeSubscriptionIntroductoryOffers
	ResourceTypeSubscriptionPromotionalOffers                   = types.ResourceTypeSubscriptionPromotionalOffers
	ResourceTypeSubscriptionPromotionalOfferPrices              = types.ResourceTypeSubscriptionPromotionalOfferPrices
	ResourceTypeSubscriptionOfferCodeCustomCodes                = types.ResourceTypeSubscriptionOfferCodeCustomCodes
	ResourceTypeSubscriptionOfferCodePrices                     = types.ResourceTypeSubscriptionOfferCodePrices
	ResourceTypeSubscriptionSubmissions                         = types.ResourceTypeSubscriptionSubmissions
	ResourceTypeSubscriptionAppStoreReviewScreenshots           = types.ResourceTypeSubscriptionAppStoreReviewScreenshots
	ResourceTypeSubscriptionGracePeriods                        = types.ResourceTypeSubscriptionGracePeriods
	ResourceTypePromotedPurchases                               = types.ResourceTypePromotedPurchases
	ResourceTypeSubscriptionPrices                              = types.ResourceTypeSubscriptionPrices
	ResourceTypeSubscriptionAvailabilities                      = types.ResourceTypeSubscriptionAvailabilities
	ResourceTypeSubscriptionPricePoints                         = types.ResourceTypeSubscriptionPricePoints
	ResourceTypeDevices                                         = types.ResourceTypeDevices
	ResourceTypeProfiles                                        = types.ResourceTypeProfiles
	ResourceTypeTerritories                                     = types.ResourceTypeTerritories
	ResourceTypeTerritoryAgeRatings                             = types.ResourceTypeTerritoryAgeRatings
	ResourceTypeEndUserLicenseAgreements                        = types.ResourceTypeEndUserLicenseAgreements
	ResourceTypeEndAppAvailabilityPreOrders                     = types.ResourceTypeEndAppAvailabilityPreOrders
	ResourceTypeTerritoryAvailabilities                         = types.ResourceTypeTerritoryAvailabilities
	ResourceTypeAppStoreReviewDetails                           = types.ResourceTypeAppStoreReviewDetails
	ResourceTypeAppStoreReviewAttachments                       = types.ResourceTypeAppStoreReviewAttachments
	ResourceTypeCustomerReviewSummarizations                    = types.ResourceTypeCustomerReviewSummarizations
	ResourceTypeUsers                                           = types.ResourceTypeUsers
	ResourceTypeUserInvitations                                 = types.ResourceTypeUserInvitations
	ResourceTypeActors                                          = types.ResourceTypeActors
	ResourceTypeSubscriptionOfferCodes                          = types.ResourceTypeSubscriptionOfferCodes
	ResourceTypeSubscriptionOfferCodeOneTimeUseCodes            = types.ResourceTypeSubscriptionOfferCodeOneTimeUseCodes
	ResourceTypeWinBackOffers                                   = types.ResourceTypeWinBackOffers
	ResourceTypeWinBackOfferPrices                              = types.ResourceTypeWinBackOfferPrices
	ResourceTypeNominations                                     = types.ResourceTypeNominations
	ResourceTypeMarketplaceSearchDetails                        = types.ResourceTypeMarketplaceSearchDetails
	ResourceTypeMarketplaceWebhooks                             = types.ResourceTypeMarketplaceWebhooks
	ResourceTypeWebhooks                                        = types.ResourceTypeWebhooks
	ResourceTypeWebhookDeliveries                               = types.ResourceTypeWebhookDeliveries
	ResourceTypeWebhookPings                                    = types.ResourceTypeWebhookPings
	ResourceTypeAlternativeDistributionDomains                  = types.ResourceTypeAlternativeDistributionDomains
	ResourceTypeAlternativeDistributionKeys                     = types.ResourceTypeAlternativeDistributionKeys
	ResourceTypeAlternativeDistributionPackages                 = types.ResourceTypeAlternativeDistributionPackages
	ResourceTypeGameCenterDetails                               = types.ResourceTypeGameCenterDetails
	ResourceTypeGameCenterAppVersions                           = types.ResourceTypeGameCenterAppVersions
	ResourceTypeGameCenterEnabledVersions                       = types.ResourceTypeGameCenterEnabledVersions
	ResourceTypeGameCenterAchievements                          = types.ResourceTypeGameCenterAchievements
	ResourceTypeGameCenterAchievementVersions                   = types.ResourceTypeGameCenterAchievementVersions
	ResourceTypeGameCenterLeaderboards                          = types.ResourceTypeGameCenterLeaderboards
	ResourceTypeGameCenterLeaderboardVersions                   = types.ResourceTypeGameCenterLeaderboardVersions
	ResourceTypeGameCenterLeaderboardSets                       = types.ResourceTypeGameCenterLeaderboardSets
	ResourceTypeGameCenterLeaderboardSetVersions                = types.ResourceTypeGameCenterLeaderboardSetVersions
	ResourceTypeGameCenterLeaderboardLocalizations              = types.ResourceTypeGameCenterLeaderboardLocalizations
	ResourceTypeGameCenterAchievementLocalizations              = types.ResourceTypeGameCenterAchievementLocalizations
	ResourceTypeGameCenterLeaderboardReleases                   = types.ResourceTypeGameCenterLeaderboardReleases
	ResourceTypeGameCenterAchievementReleases                   = types.ResourceTypeGameCenterAchievementReleases
	ResourceTypeGameCenterLeaderboardSetReleases                = types.ResourceTypeGameCenterLeaderboardSetReleases
	ResourceTypeGameCenterLeaderboardImages                     = types.ResourceTypeGameCenterLeaderboardImages
	ResourceTypeGameCenterLeaderboardSetLocalizations           = types.ResourceTypeGameCenterLeaderboardSetLocalizations
	ResourceTypeGameCenterLeaderboardSetMemberLocalizations     = types.ResourceTypeGameCenterLeaderboardSetMemberLocalizations
	ResourceTypeGameCenterAchievementImages                     = types.ResourceTypeGameCenterAchievementImages
	ResourceTypeGameCenterLeaderboardSetImages                  = types.ResourceTypeGameCenterLeaderboardSetImages
	ResourceTypeGameCenterChallenges                            = types.ResourceTypeGameCenterChallenges
	ResourceTypeGameCenterChallengeVersions                     = types.ResourceTypeGameCenterChallengeVersions
	ResourceTypeGameCenterChallengeLocalizations                = types.ResourceTypeGameCenterChallengeLocalizations
	ResourceTypeGameCenterChallengeImages                       = types.ResourceTypeGameCenterChallengeImages
	ResourceTypeGameCenterChallengeVersionReleases              = types.ResourceTypeGameCenterChallengeVersionReleases
	ResourceTypeGameCenterActivities                            = types.ResourceTypeGameCenterActivities
	ResourceTypeGameCenterActivityVersions                      = types.ResourceTypeGameCenterActivityVersions
	ResourceTypeGameCenterActivityLocalizations                 = types.ResourceTypeGameCenterActivityLocalizations
	ResourceTypeGameCenterActivityImages                        = types.ResourceTypeGameCenterActivityImages
	ResourceTypeGameCenterActivityVersionReleases               = types.ResourceTypeGameCenterActivityVersionReleases
	ResourceTypeGameCenterGroups                                = types.ResourceTypeGameCenterGroups
	ResourceTypeGameCenterMatchmakingQueues                     = types.ResourceTypeGameCenterMatchmakingQueues
	ResourceTypeGameCenterMatchmakingRuleSets                   = types.ResourceTypeGameCenterMatchmakingRuleSets
	ResourceTypeGameCenterMatchmakingRules                      = types.ResourceTypeGameCenterMatchmakingRules
	ResourceTypeGameCenterMatchmakingTeams                      = types.ResourceTypeGameCenterMatchmakingTeams
	ResourceTypeGameCenterMatchmakingRuleSetTests               = types.ResourceTypeGameCenterMatchmakingRuleSetTests
	ResourceTypeGameCenterLeaderboardEntrySubmissions           = types.ResourceTypeGameCenterLeaderboardEntrySubmissions
	ResourceTypeGameCenterPlayerAchievementSubmissions          = types.ResourceTypeGameCenterPlayerAchievementSubmissions
	ResourceTypeGameCenterMatchmakingTestRequests               = types.ResourceTypeGameCenterMatchmakingTestRequests
	ResourceTypeGameCenterMatchmakingTestPlayerProperties       = types.ResourceTypeGameCenterMatchmakingTestPlayerProperties
)

// Platform constants — re-exported from types package.
const (
	PlatformIOS      = types.PlatformIOS
	PlatformMacOS    = types.PlatformMacOS
	PlatformTVOS     = types.PlatformTVOS
	PlatformVisionOS = types.PlatformVisionOS
)

// ChecksumAlgorithm constants — re-exported from types package.
const (
	ChecksumAlgorithmMD5    = types.ChecksumAlgorithmMD5
	ChecksumAlgorithmSHA256 = types.ChecksumAlgorithmSHA256
)

// AssetType constants — re-exported from types package.
const (
	AssetTypeAsset = types.AssetTypeAsset
)

// UTI constants — re-exported from types package.
const (
	UTIIPA = types.UTIIPA
	UTIPKG = types.UTIPKG
)
