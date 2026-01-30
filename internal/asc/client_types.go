package asc

import "encoding/json"

// ResourceType represents an ASC resource type.
type ResourceType string

const (
	ResourceTypeApps                                            ResourceType = "apps"
	ResourceTypeAppTags                                         ResourceType = "appTags"
	ResourceTypeBundleIds                                       ResourceType = "bundleIds"
	ResourceTypeBundleIdCapabilities                            ResourceType = "bundleIdCapabilities"
	ResourceTypeMerchantIds                                     ResourceType = "merchantIds"
	ResourceTypePassTypeIds                                     ResourceType = "passTypeIds"
	ResourceTypeAppCategories                                   ResourceType = "appCategories"
	ResourceTypeAppAvailabilities                               ResourceType = "appAvailabilities"
	ResourceTypeAppPricePoints                                  ResourceType = "appPricePoints"
	ResourceTypeAppPriceSchedules                               ResourceType = "appPriceSchedules"
	ResourceTypeAppPrices                                       ResourceType = "appPrices"
	ResourceTypeBuilds                                          ResourceType = "builds"
	ResourceTypeBuildBundles                                    ResourceType = "buildBundles"
	ResourceTypeBuildBundleFileSizes                            ResourceType = "buildBundleFileSizes"
	ResourceTypeBuildUploads                                    ResourceType = "buildUploads"
	ResourceTypeBuildUploadFiles                                ResourceType = "buildUploadFiles"
	ResourceTypeCertificates                                    ResourceType = "certificates"
	ResourceTypeAppStoreVersions                                ResourceType = "appStoreVersions"
	ResourceTypeAppClips                                        ResourceType = "appClips"
	ResourceTypeAppClipDefaultExperiences                       ResourceType = "appClipDefaultExperiences"
	ResourceTypeAppClipDefaultExperienceLocalizations           ResourceType = "appClipDefaultExperienceLocalizations"
	ResourceTypeAppClipAdvancedExperiences                      ResourceType = "appClipAdvancedExperiences"
	ResourceTypeAppClipAdvancedExperienceImages                 ResourceType = "appClipAdvancedExperienceImages"
	ResourceTypeAppClipAdvancedExperienceLocalizations          ResourceType = "appClipAdvancedExperienceLocalizations"
	ResourceTypeAppClipHeaderImages                             ResourceType = "appClipHeaderImages"
	ResourceTypeAppClipAppStoreReviewDetails                    ResourceType = "appClipAppStoreReviewDetails"
	ResourceTypeBackgroundAssets                                ResourceType = "backgroundAssets"
	ResourceTypeBackgroundAssetVersions                         ResourceType = "backgroundAssetVersions"
	ResourceTypeBackgroundAssetUploadFiles                      ResourceType = "backgroundAssetUploadFiles"
	ResourceTypeRoutingAppCoverages                             ResourceType = "routingAppCoverages"
	ResourceTypeAppEncryptionDeclarations                       ResourceType = "appEncryptionDeclarations"
	ResourceTypeAppEncryptionDeclarationDocuments               ResourceType = "appEncryptionDeclarationDocuments"
	ResourceTypeAppStoreVersionPromotions                       ResourceType = "appStoreVersionPromotions"
	ResourceTypeAppStoreVersionExperimentTreatments             ResourceType = "appStoreVersionExperimentTreatments"
	ResourceTypeAppStoreVersionExperimentTreatmentLocalizations ResourceType = "appStoreVersionExperimentTreatmentLocalizations"
	ResourceTypePreReleaseVersions                              ResourceType = "preReleaseVersions"
	ResourceTypeAppStoreVersionSubmissions                      ResourceType = "appStoreVersionSubmissions"
	ResourceTypeAppScreenshotSets                               ResourceType = "appScreenshotSets"
	ResourceTypeAppScreenshots                                  ResourceType = "appScreenshots"
	ResourceTypeAppPreviewSets                                  ResourceType = "appPreviewSets"
	ResourceTypeAppPreviews                                     ResourceType = "appPreviews"
	ResourceTypeReviewSubmissions                               ResourceType = "reviewSubmissions"
	ResourceTypeReviewSubmissionItems                           ResourceType = "reviewSubmissionItems"
	ResourceTypeAppCustomProductPages                           ResourceType = "appCustomProductPages"
	ResourceTypeAppCustomProductPageVersions                    ResourceType = "appCustomProductPageVersions"
	ResourceTypeAppCustomProductPageLocalizations               ResourceType = "appCustomProductPageLocalizations"
	ResourceTypeAppEvents                                       ResourceType = "appEvents"
	ResourceTypeAppEventLocalizations                           ResourceType = "appEventLocalizations"
	ResourceTypeAppEventScreenshots                             ResourceType = "appEventScreenshots"
	ResourceTypeAppEventVideoClips                              ResourceType = "appEventVideoClips"
	ResourceTypeAppStoreVersionExperiments                      ResourceType = "appStoreVersionExperiments"
	ResourceTypeBetaGroups                                      ResourceType = "betaGroups"
	ResourceTypeBetaTesters                                     ResourceType = "betaTesters"
	ResourceTypeBetaTesterInvitations                           ResourceType = "betaTesterInvitations"
	ResourceTypeBetaAppReviewDetails                            ResourceType = "betaAppReviewDetails"
	ResourceTypeBetaAppReviewSubmissions                        ResourceType = "betaAppReviewSubmissions"
	ResourceTypeBetaAppClipInvocations                          ResourceType = "betaAppClipInvocations"
	ResourceTypeBetaAppClipInvocationLocalizations              ResourceType = "betaAppClipInvocationLocalizations"
	ResourceTypeBuildBetaDetails                                ResourceType = "buildBetaDetails"
	ResourceTypeBetaBuildLocalizations                          ResourceType = "betaBuildLocalizations"
	ResourceTypeBetaRecruitmentCriteria                         ResourceType = "betaRecruitmentCriteria"
	ResourceTypeBetaRecruitmentCriterionOptions                 ResourceType = "betaRecruitmentCriterionOptions"
	ResourceTypeSandboxTesters                                  ResourceType = "sandboxTesters"
	ResourceTypeSandboxTestersClearHistory                      ResourceType = "sandboxTestersClearPurchaseHistoryRequest"
	ResourceTypeAppClipDomainStatuses                           ResourceType = "appClipDomainStatuses"
	ResourceTypeAppStoreVersionLocalizations                    ResourceType = "appStoreVersionLocalizations"
	ResourceTypeAppKeywords                                     ResourceType = "appKeywords"
	ResourceTypeAppInfoLocalizations                            ResourceType = "appInfoLocalizations"
	ResourceTypeAppInfos                                        ResourceType = "appInfos"
	ResourceTypeAgeRatingDeclarations                           ResourceType = "ageRatingDeclarations"
	ResourceTypeAccessibilityDeclarations                       ResourceType = "accessibilityDeclarations"
	ResourceTypeDiagnosticSignatures                            ResourceType = "diagnosticSignatures"
	ResourceTypeAndroidToIosAppMappingDetails                   ResourceType = "androidToIosAppMappingDetails"
	ResourceTypeAnalyticsReportRequests                         ResourceType = "analyticsReportRequests"
	ResourceTypeAnalyticsReports                                ResourceType = "analyticsReports"
	ResourceTypeAnalyticsReportInstances                        ResourceType = "analyticsReportInstances"
	ResourceTypeAnalyticsReportSegments                         ResourceType = "analyticsReportSegments"
	ResourceTypeInAppPurchases                                  ResourceType = "inAppPurchases"
	ResourceTypeInAppPurchaseLocalizations                      ResourceType = "inAppPurchaseLocalizations"
	ResourceTypeSubscriptionGroups                              ResourceType = "subscriptionGroups"
	ResourceTypeSubscriptions                                   ResourceType = "subscriptions"
	ResourceTypePromotedPurchases                               ResourceType = "promotedPurchases"
	ResourceTypeSubscriptionPrices                              ResourceType = "subscriptionPrices"
	ResourceTypeSubscriptionAvailabilities                      ResourceType = "subscriptionAvailabilities"
	ResourceTypeSubscriptionPricePoints                         ResourceType = "subscriptionPricePoints"
	ResourceTypeDevices                                         ResourceType = "devices"
	ResourceTypeProfiles                                        ResourceType = "profiles"
	ResourceTypeTerritories                                     ResourceType = "territories"
	ResourceTypeEndUserLicenseAgreements                        ResourceType = "endUserLicenseAgreements"
	ResourceTypeEndAppAvailabilityPreOrders                     ResourceType = "endAppAvailabilityPreOrders"
	ResourceTypeTerritoryAvailabilities                         ResourceType = "territoryAvailabilities"
	ResourceTypeAppStoreReviewDetails                           ResourceType = "appStoreReviewDetails"
	ResourceTypeAppStoreReviewAttachments                       ResourceType = "appStoreReviewAttachments"
	ResourceTypeUsers                                           ResourceType = "users"
	ResourceTypeUserInvitations                                 ResourceType = "userInvitations"
	ResourceTypeActors                                          ResourceType = "actors"
	ResourceTypeSubscriptionOfferCodes                          ResourceType = "subscriptionOfferCodes"
	ResourceTypeSubscriptionOfferCodeOneTimeUseCodes            ResourceType = "subscriptionOfferCodeOneTimeUseCodes"
	ResourceTypeWinBackOffers                                   ResourceType = "winBackOffers"
	ResourceTypeWinBackOfferPrices                              ResourceType = "winBackOfferPrices"
	ResourceTypeNominations                                     ResourceType = "nominations"
	ResourceTypeMarketplaceSearchDetails                        ResourceType = "marketplaceSearchDetails"
	ResourceTypeMarketplaceWebhooks                             ResourceType = "marketplaceWebhooks"
	ResourceTypeWebhooks                                        ResourceType = "webhooks"
	ResourceTypeWebhookDeliveries                               ResourceType = "webhookDeliveries"
	ResourceTypeWebhookPings                                    ResourceType = "webhookPings"
	ResourceTypeAlternativeDistributionDomains                  ResourceType = "alternativeDistributionDomains"
	ResourceTypeAlternativeDistributionKeys                     ResourceType = "alternativeDistributionKeys"
	ResourceTypeAlternativeDistributionPackages                 ResourceType = "alternativeDistributionPackages"
	ResourceTypeGameCenterDetails                               ResourceType = "gameCenterDetails"
	ResourceTypeGameCenterAchievements                          ResourceType = "gameCenterAchievements"
	ResourceTypeGameCenterLeaderboards                          ResourceType = "gameCenterLeaderboards"
	ResourceTypeGameCenterLeaderboardSets                       ResourceType = "gameCenterLeaderboardSets"
	ResourceTypeGameCenterLeaderboardLocalizations              ResourceType = "gameCenterLeaderboardLocalizations"
	ResourceTypeGameCenterAchievementLocalizations              ResourceType = "gameCenterAchievementLocalizations"
	ResourceTypeGameCenterLeaderboardReleases                   ResourceType = "gameCenterLeaderboardReleases"
	ResourceTypeGameCenterAchievementReleases                   ResourceType = "gameCenterAchievementReleases"
	ResourceTypeGameCenterLeaderboardSetReleases                ResourceType = "gameCenterLeaderboardSetReleases"
	ResourceTypeGameCenterLeaderboardImages                     ResourceType = "gameCenterLeaderboardImages"
	ResourceTypeGameCenterLeaderboardSetLocalizations           ResourceType = "gameCenterLeaderboardSetLocalizations"
	ResourceTypeGameCenterAchievementImages                     ResourceType = "gameCenterAchievementImages"
	ResourceTypeGameCenterLeaderboardSetImages                  ResourceType = "gameCenterLeaderboardSetImages"
)

// Resource is a generic ASC API resource wrapper.
type Resource[T any] struct {
	Type          ResourceType    `json:"type"`
	ID            string          `json:"id"`
	Attributes    T               `json:"attributes"`
	Relationships json.RawMessage `json:"relationships,omitempty"`
	Links         json.RawMessage `json:"links,omitempty"`
}

// Response is a generic ASC API response wrapper.
type Response[T any] struct {
	Data     []Resource[T]   `json:"data"`
	Links    Links           `json:"links,omitempty"`
	Included json.RawMessage `json:"included,omitempty"`
	Meta     json.RawMessage `json:"meta,omitempty"`
}

// SingleResponse is a generic ASC API response wrapper for single resources.
type SingleResponse[T any] struct {
	Data     Resource[T]     `json:"data"`
	Links    Links           `json:"links,omitempty"`
	Included json.RawMessage `json:"included,omitempty"`
	Meta     json.RawMessage `json:"meta,omitempty"`
}

// LinkagesResponse is a generic relationship linkages response.
type LinkagesResponse struct {
	Data  []ResourceData  `json:"data"`
	Links Links           `json:"links,omitempty"`
	Meta  json.RawMessage `json:"meta,omitempty"`
}

// SingleResourceResponse is a response with a single resource (not an array).
type SingleResourceResponse[T any] struct {
	Data Resource[T] `json:"data"`
}

// Links represents pagination links
type Links struct {
	Self string `json:"self,omitempty"`
	Next string `json:"next,omitempty"`
	Prev string `json:"prev,omitempty"`
}

// Platform represents an Apple platform.
type Platform string

const (
	PlatformIOS      Platform = "IOS"
	PlatformMacOS    Platform = "MAC_OS"
	PlatformTVOS     Platform = "TV_OS"
	PlatformVisionOS Platform = "VISION_OS"
)

// ChecksumAlgorithm represents the algorithm used for checksums.
type ChecksumAlgorithm string

const (
	ChecksumAlgorithmMD5    ChecksumAlgorithm = "MD5"
	ChecksumAlgorithmSHA256 ChecksumAlgorithm = "SHA_256"
)

// AssetType represents the asset type for build uploads.
type AssetType string

const (
	AssetTypeAsset AssetType = "ASSET"
)

// UTI represents a Uniform Type Identifier used in uploads.
type UTI string

const (
	UTIIPA UTI = "com.apple.ipa"
)

// Relationship represents a generic API relationship.
type Relationship struct {
	Data ResourceData `json:"data"`
}

// RelationshipList represents a relationship containing multiple resources.
type RelationshipList struct {
	Data []ResourceData `json:"data"`
}

// RelationshipRequest represents a relationship list payload.
type RelationshipRequest struct {
	Data []RelationshipData `json:"data"`
}

// RelationshipData represents data in a relationship payload.
type RelationshipData struct {
	Type ResourceType `json:"type"`
	ID   string       `json:"id"`
}

// ResourceData represents the data portion of a resource.
type ResourceData struct {
	Type ResourceType `json:"type"`
	ID   string       `json:"id"`
}
