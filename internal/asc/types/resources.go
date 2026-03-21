// Package types contains the core data types for the App Store Connect API.
// These are pure data structures with JSON serialization and pagination support.
package types

import (
	"bytes"
	"encoding/json"
)

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
	ResourceTypeBuildIcons                                      ResourceType = "buildIcons"
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
	ResourceTypeBackgroundAssetVersionAppStoreReleases          ResourceType = "backgroundAssetVersionAppStoreReleases"
	ResourceTypeBackgroundAssetVersionExternalBetaReleases      ResourceType = "backgroundAssetVersionExternalBetaReleases"
	ResourceTypeBackgroundAssetVersionInternalBetaReleases      ResourceType = "backgroundAssetVersionInternalBetaReleases"
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
	ResourceTypeBetaLicenseAgreements                           ResourceType = "betaLicenseAgreements"
	ResourceTypeBetaAppClipInvocations                          ResourceType = "betaAppClipInvocations"
	ResourceTypeBetaAppClipInvocationLocalizations              ResourceType = "betaAppClipInvocationLocalizations"
	ResourceTypeBuildBetaDetails                                ResourceType = "buildBetaDetails"
	ResourceTypeBuildBetaNotifications                          ResourceType = "buildBetaNotifications"
	ResourceTypeBetaAppLocalizations                            ResourceType = "betaAppLocalizations"
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
	ResourceTypeInAppPurchaseImages                             ResourceType = "inAppPurchaseImages"
	ResourceTypeInAppPurchaseAppStoreReviewScreenshots          ResourceType = "inAppPurchaseAppStoreReviewScreenshots"
	ResourceTypeInAppPurchaseAvailabilities                     ResourceType = "inAppPurchaseAvailabilities"
	ResourceTypeInAppPurchaseContents                           ResourceType = "inAppPurchaseContents"
	ResourceTypeInAppPurchasePricePoints                        ResourceType = "inAppPurchasePricePoints"
	ResourceTypeInAppPurchasePriceSchedules                     ResourceType = "inAppPurchasePriceSchedules"
	ResourceTypeInAppPurchasePrices                             ResourceType = "inAppPurchasePrices"
	ResourceTypeInAppPurchaseOfferCodes                         ResourceType = "inAppPurchaseOfferCodes"
	ResourceTypeInAppPurchaseOfferCodeCustomCodes               ResourceType = "inAppPurchaseOfferCodeCustomCodes"
	ResourceTypeInAppPurchaseOfferCodeOneTimeUseCodes           ResourceType = "inAppPurchaseOfferCodeOneTimeUseCodes"
	ResourceTypeInAppPurchaseOfferPrices                        ResourceType = "inAppPurchaseOfferPrices"
	ResourceTypeInAppPurchaseSubmissions                        ResourceType = "inAppPurchaseSubmissions"
	ResourceTypeSubscriptionGroups                              ResourceType = "subscriptionGroups"
	ResourceTypeSubscriptionGroupLocalizations                  ResourceType = "subscriptionGroupLocalizations"
	ResourceTypeSubscriptionGroupSubmissions                    ResourceType = "subscriptionGroupSubmissions"
	ResourceTypeSubscriptions                                   ResourceType = "subscriptions"
	ResourceTypeSubscriptionLocalizations                       ResourceType = "subscriptionLocalizations"
	ResourceTypeSubscriptionImages                              ResourceType = "subscriptionImages"
	ResourceTypeSubscriptionIntroductoryOffers                  ResourceType = "subscriptionIntroductoryOffers"
	ResourceTypeSubscriptionPromotionalOffers                   ResourceType = "subscriptionPromotionalOffers"
	ResourceTypeSubscriptionPromotionalOfferPrices              ResourceType = "subscriptionPromotionalOfferPrices"
	ResourceTypeSubscriptionOfferCodeCustomCodes                ResourceType = "subscriptionOfferCodeCustomCodes"
	ResourceTypeSubscriptionOfferCodePrices                     ResourceType = "subscriptionOfferCodePrices"
	ResourceTypeSubscriptionSubmissions                         ResourceType = "subscriptionSubmissions"
	ResourceTypeSubscriptionAppStoreReviewScreenshots           ResourceType = "subscriptionAppStoreReviewScreenshots"
	ResourceTypeSubscriptionGracePeriods                        ResourceType = "subscriptionGracePeriods"
	ResourceTypePromotedPurchases                               ResourceType = "promotedPurchases"
	ResourceTypeSubscriptionPrices                              ResourceType = "subscriptionPrices"
	ResourceTypeSubscriptionAvailabilities                      ResourceType = "subscriptionAvailabilities"
	ResourceTypeSubscriptionPricePoints                         ResourceType = "subscriptionPricePoints"
	ResourceTypeDevices                                         ResourceType = "devices"
	ResourceTypeProfiles                                        ResourceType = "profiles"
	ResourceTypeTerritories                                     ResourceType = "territories"
	ResourceTypeTerritoryAgeRatings                             ResourceType = "territoryAgeRatings"
	ResourceTypeEndUserLicenseAgreements                        ResourceType = "endUserLicenseAgreements"
	ResourceTypeEndAppAvailabilityPreOrders                     ResourceType = "endAppAvailabilityPreOrders"
	ResourceTypeTerritoryAvailabilities                         ResourceType = "territoryAvailabilities"
	ResourceTypeAppStoreReviewDetails                           ResourceType = "appStoreReviewDetails"
	ResourceTypeAppStoreReviewAttachments                       ResourceType = "appStoreReviewAttachments"
	ResourceTypeCustomerReviewSummarizations                    ResourceType = "customerReviewSummarizations"
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
	ResourceTypeGameCenterAppVersions                           ResourceType = "gameCenterAppVersions"
	ResourceTypeGameCenterEnabledVersions                       ResourceType = "gameCenterEnabledVersions"
	ResourceTypeGameCenterAchievements                          ResourceType = "gameCenterAchievements"
	ResourceTypeGameCenterAchievementVersions                   ResourceType = "gameCenterAchievementVersions"
	ResourceTypeGameCenterLeaderboards                          ResourceType = "gameCenterLeaderboards"
	ResourceTypeGameCenterLeaderboardVersions                   ResourceType = "gameCenterLeaderboardVersions"
	ResourceTypeGameCenterLeaderboardSets                       ResourceType = "gameCenterLeaderboardSets"
	ResourceTypeGameCenterLeaderboardSetVersions                ResourceType = "gameCenterLeaderboardSetVersions"
	ResourceTypeGameCenterLeaderboardLocalizations              ResourceType = "gameCenterLeaderboardLocalizations"
	ResourceTypeGameCenterAchievementLocalizations              ResourceType = "gameCenterAchievementLocalizations"
	ResourceTypeGameCenterLeaderboardReleases                   ResourceType = "gameCenterLeaderboardReleases"
	ResourceTypeGameCenterAchievementReleases                   ResourceType = "gameCenterAchievementReleases"
	ResourceTypeGameCenterLeaderboardSetReleases                ResourceType = "gameCenterLeaderboardSetReleases"
	ResourceTypeGameCenterLeaderboardImages                     ResourceType = "gameCenterLeaderboardImages"
	ResourceTypeGameCenterLeaderboardSetLocalizations           ResourceType = "gameCenterLeaderboardSetLocalizations"
	ResourceTypeGameCenterLeaderboardSetMemberLocalizations     ResourceType = "gameCenterLeaderboardSetMemberLocalizations"
	ResourceTypeGameCenterAchievementImages                     ResourceType = "gameCenterAchievementImages"
	ResourceTypeGameCenterLeaderboardSetImages                  ResourceType = "gameCenterLeaderboardSetImages"
	ResourceTypeGameCenterChallenges                            ResourceType = "gameCenterChallenges"
	ResourceTypeGameCenterChallengeVersions                     ResourceType = "gameCenterChallengeVersions"
	ResourceTypeGameCenterChallengeLocalizations                ResourceType = "gameCenterChallengeLocalizations"
	ResourceTypeGameCenterChallengeImages                       ResourceType = "gameCenterChallengeImages"
	ResourceTypeGameCenterChallengeVersionReleases              ResourceType = "gameCenterChallengeVersionReleases"
	ResourceTypeGameCenterActivities                            ResourceType = "gameCenterActivities"
	ResourceTypeGameCenterActivityVersions                      ResourceType = "gameCenterActivityVersions"
	ResourceTypeGameCenterActivityLocalizations                 ResourceType = "gameCenterActivityLocalizations"
	ResourceTypeGameCenterActivityImages                        ResourceType = "gameCenterActivityImages"
	ResourceTypeGameCenterActivityVersionReleases               ResourceType = "gameCenterActivityVersionReleases"
	ResourceTypeGameCenterGroups                                ResourceType = "gameCenterGroups"
	ResourceTypeGameCenterMatchmakingQueues                     ResourceType = "gameCenterMatchmakingQueues"
	ResourceTypeGameCenterMatchmakingRuleSets                   ResourceType = "gameCenterMatchmakingRuleSets"
	ResourceTypeGameCenterMatchmakingRules                      ResourceType = "gameCenterMatchmakingRules"
	ResourceTypeGameCenterMatchmakingTeams                      ResourceType = "gameCenterMatchmakingTeams"
	ResourceTypeGameCenterMatchmakingRuleSetTests               ResourceType = "gameCenterMatchmakingRuleSetTests"
	ResourceTypeGameCenterLeaderboardEntrySubmissions           ResourceType = "gameCenterLeaderboardEntrySubmissions"
	ResourceTypeGameCenterPlayerAchievementSubmissions          ResourceType = "gameCenterPlayerAchievementSubmissions"
	ResourceTypeGameCenterMatchmakingTestRequests               ResourceType = "gameCenterMatchmakingTestRequests"
	ResourceTypeGameCenterMatchmakingTestPlayerProperties       ResourceType = "gameCenterMatchmakingTestPlayerProperties"
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
	Links    Links           `json:"links"`
	Included json.RawMessage `json:"included,omitempty"`
	Meta     json.RawMessage `json:"meta,omitempty"`
}

// GetLinks returns the links field for pagination.
func (r *Response[T]) GetLinks() *Links {
	return &r.Links
}

// GetData returns the data field for aggregation.
func (r *Response[T]) GetData() any {
	return r.Data
}

// ParsePagingTotal extracts the total count from a response's paging metadata.
// App Store Connect responses include {"meta":{"paging":{"total":N}}} which
// provides the total count even when limit is less than the total.
// Returns 0 if the metadata is absent or unparseable.
func ParsePagingTotal(meta json.RawMessage) int {
	total, _ := ParsePagingTotalOK(meta)
	return total
}

// ParsePagingTotalOK extracts the total count from a response's paging metadata.
// Returns (total, true) when the field is present, (0, false) when absent or unparseable.
// Use this when you need to distinguish "zero builds" from "API did not return a total".
func ParsePagingTotalOK(meta json.RawMessage) (int, bool) {
	if len(meta) == 0 {
		return 0, false
	}
	var parsed struct {
		Paging struct {
			Total *int `json:"total"`
		} `json:"paging"`
	}
	if err := json.Unmarshal(meta, &parsed); err != nil {
		return 0, false
	}
	if parsed.Paging.Total == nil {
		return 0, false
	}
	return *parsed.Paging.Total, true
}

// SingleResponse is a generic ASC API response wrapper for single resources.
type SingleResponse[T any] struct {
	Data     Resource[T]     `json:"data"`
	Links    Links           `json:"links"`
	Included json.RawMessage `json:"included,omitempty"`
	Meta     json.RawMessage `json:"meta,omitempty"`
}

// LinkagesResponse is a generic relationship linkages response.
type LinkagesResponse struct {
	Data  []ResourceData  `json:"data"`
	Links Links           `json:"links"`
	Meta  json.RawMessage `json:"meta,omitempty"`
}

// GetLinks returns the links field for pagination.
func (r *LinkagesResponse) GetLinks() *Links {
	return &r.Links
}

// GetData returns the data field for aggregation.
func (r *LinkagesResponse) GetData() any {
	return r.Data
}

// SingleResourceResponse is a response with a single resource (not an array).
type SingleResourceResponse[T any] struct {
	Data Resource[T] `json:"data"`
}

// PaginatedResponse represents a response that supports pagination.
type PaginatedResponse interface {
	GetLinks() *Links
	GetData() any
}

// Links represents pagination links.
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
	UTIPKG UTI = "com.apple.pkg"
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

// MarshalJSON ensures relationship list payloads always encode `data` as an array.
//
// In JSON:API, to-many relationship payloads use `data: []` to represent an empty
// set. Go's `encoding/json` encodes nil slices as `null`, which is not a valid
// representation for a relationship list.
func (r RelationshipRequest) MarshalJSON() ([]byte, error) {
	if r.Data == nil {
		r.Data = []RelationshipData{}
	}
	type alias RelationshipRequest
	return json.Marshal(alias(r))
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

// UnmarshalJSON allows relationship responses to represent {"data":null}.
//
// App Store Connect uses JSON:API relationships, which may return a null "data"
// when the relationship is unset (e.g. primary category not configured yet).
// Modeling ResourceData as a struct is still convenient, but it must tolerate
// nulls to avoid turning a "missing relationship" into a hard parse error.
func (r *ResourceData) UnmarshalJSON(data []byte) error {
	if r == nil {
		return nil
	}

	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		*r = ResourceData{}
		return nil
	}

	type resourceData ResourceData
	var tmp resourceData
	if err := json.Unmarshal(data, &tmp); err != nil {
		return err
	}
	*r = ResourceData(tmp)
	return nil
}
