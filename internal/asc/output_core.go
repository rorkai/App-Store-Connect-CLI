package asc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
)

// PrintJSON prints data as minified JSON (best for AI agents)
func PrintJSON(data interface{}) error {
	enc := json.NewEncoder(os.Stdout)
	return enc.Encode(data)
}

// PrintPrettyJSON prints data as indented JSON (best for debugging).
func PrintPrettyJSON(data interface{}) error {
	switch v := data.(type) {
	case *PerfPowerMetricsResponse:
		return printPrettyRawJSON(v.Data)
	case *DiagnosticLogsResponse:
		return printPrettyRawJSON(v.Data)
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(data)
}

func printPrettyRawJSON(data json.RawMessage) error {
	if len(data) == 0 {
		_, err := os.Stdout.Write([]byte("null\n"))
		return err
	}

	var buf bytes.Buffer
	if err := json.Indent(&buf, data, "", "  "); err != nil {
		return fmt.Errorf("pretty-print json: %w", err)
	}
	buf.WriteByte('\n')
	_, err := os.Stdout.Write(buf.Bytes())
	return err
}

// PrintMarkdown prints data as Markdown table
func PrintMarkdown(data interface{}) error {
	switch v := data.(type) {
	case *FeedbackResponse:
		return printFeedbackMarkdown(v)
	case *CrashesResponse:
		return printCrashesMarkdown(v)
	case *ReviewsResponse:
		return printReviewsMarkdown(v)
	case *AppsResponse:
		return printAppsMarkdown(v)
	case *AppClipsResponse:
		return printAppClipsMarkdown(v)
	case *AppCategoriesResponse:
		return printAppCategoriesMarkdown(v)
	case *AppResponse:
		return printAppsMarkdown(&AppsResponse{Data: []Resource[AppAttributes]{v.Data}})
	case *AppClipResponse:
		return printAppClipsMarkdown(&AppClipsResponse{Data: []Resource[AppClipAttributes]{v.Data}})
	case *AppClipDefaultExperiencesResponse:
		return printAppClipDefaultExperiencesMarkdown(v)
	case *AppClipDefaultExperienceResponse:
		return printAppClipDefaultExperiencesMarkdown(&AppClipDefaultExperiencesResponse{Data: []Resource[AppClipDefaultExperienceAttributes]{v.Data}})
	case *AppClipDefaultExperienceLocalizationsResponse:
		return printAppClipDefaultExperienceLocalizationsMarkdown(v)
	case *AppClipDefaultExperienceLocalizationResponse:
		return printAppClipDefaultExperienceLocalizationsMarkdown(&AppClipDefaultExperienceLocalizationsResponse{Data: []Resource[AppClipDefaultExperienceLocalizationAttributes]{v.Data}})
	case *AppClipAdvancedExperiencesResponse:
		return printAppClipAdvancedExperiencesMarkdown(v)
	case *AppClipAdvancedExperienceResponse:
		return printAppClipAdvancedExperiencesMarkdown(&AppClipAdvancedExperiencesResponse{Data: []Resource[AppClipAdvancedExperienceAttributes]{v.Data}})
	case *AppSetupInfoResult:
		return printAppSetupInfoResultMarkdown(v)
	case *AppTagsResponse:
		return printAppTagsMarkdown(v)
	case *AppTagResponse:
		return printAppTagsMarkdown(&AppTagsResponse{Data: []Resource[AppTagAttributes]{v.Data}})
	case *MarketplaceSearchDetailsResponse:
		return printMarketplaceSearchDetailsMarkdown(v)
	case *MarketplaceSearchDetailResponse:
		return printMarketplaceSearchDetailMarkdown(v)
	case *MarketplaceWebhooksResponse:
		return printMarketplaceWebhooksMarkdown(v)
	case *MarketplaceWebhookResponse:
		return printMarketplaceWebhookMarkdown(v)
	case *WebhooksResponse:
		return printWebhooksMarkdown(v)
	case *WebhookResponse:
		return printWebhooksMarkdown(&WebhooksResponse{Data: []Resource[WebhookAttributes]{v.Data}})
	case *WebhookDeliveriesResponse:
		return printWebhookDeliveriesMarkdown(v)
	case *WebhookDeliveryResponse:
		return printWebhookDeliveriesMarkdown(&WebhookDeliveriesResponse{Data: []Resource[WebhookDeliveryAttributes]{v.Data}})
	case *AlternativeDistributionDomainsResponse:
		return printAlternativeDistributionDomainsMarkdown(v)
	case *AlternativeDistributionDomainResponse:
		return printAlternativeDistributionDomainsMarkdown(&AlternativeDistributionDomainsResponse{Data: []Resource[AlternativeDistributionDomainAttributes]{v.Data}})
	case *AlternativeDistributionKeysResponse:
		return printAlternativeDistributionKeysMarkdown(v)
	case *AlternativeDistributionKeyResponse:
		return printAlternativeDistributionKeysMarkdown(&AlternativeDistributionKeysResponse{Data: []Resource[AlternativeDistributionKeyAttributes]{v.Data}})
	case *AlternativeDistributionPackageResponse:
		return printAlternativeDistributionPackageMarkdown(v)
	case *AlternativeDistributionPackageVersionsResponse:
		return printAlternativeDistributionPackageVersionsMarkdown(v)
	case *AlternativeDistributionPackageVersionResponse:
		return printAlternativeDistributionPackageVersionsMarkdown(&AlternativeDistributionPackageVersionsResponse{Data: []Resource[AlternativeDistributionPackageVersionAttributes]{v.Data}})
	case *AlternativeDistributionPackageVariantsResponse:
		return printAlternativeDistributionPackageVariantsMarkdown(v)
	case *AlternativeDistributionPackageVariantResponse:
		return printAlternativeDistributionPackageVariantsMarkdown(&AlternativeDistributionPackageVariantsResponse{Data: []Resource[AlternativeDistributionPackageVariantAttributes]{v.Data}})
	case *AlternativeDistributionPackageDeltasResponse:
		return printAlternativeDistributionPackageDeltasMarkdown(v)
	case *AlternativeDistributionPackageDeltaResponse:
		return printAlternativeDistributionPackageDeltasMarkdown(&AlternativeDistributionPackageDeltasResponse{Data: []Resource[AlternativeDistributionPackageDeltaAttributes]{v.Data}})
	case *BackgroundAssetsResponse:
		return printBackgroundAssetsMarkdown(v)
	case *BackgroundAssetResponse:
		return printBackgroundAssetsMarkdown(&BackgroundAssetsResponse{Data: []Resource[BackgroundAssetAttributes]{v.Data}})
	case *BackgroundAssetVersionsResponse:
		return printBackgroundAssetVersionsMarkdown(v)
	case *BackgroundAssetVersionResponse:
		return printBackgroundAssetVersionsMarkdown(&BackgroundAssetVersionsResponse{Data: []Resource[BackgroundAssetVersionAttributes]{v.Data}})
	case *BackgroundAssetUploadFilesResponse:
		return printBackgroundAssetUploadFilesMarkdown(v)
	case *BackgroundAssetUploadFileResponse:
		return printBackgroundAssetUploadFilesMarkdown(&BackgroundAssetUploadFilesResponse{Data: []Resource[BackgroundAssetUploadFileAttributes]{v.Data}})
	case *NominationsResponse:
		return printNominationsMarkdown(v)
	case *NominationResponse:
		return printNominationsMarkdown(&NominationsResponse{Data: []Resource[NominationAttributes]{v.Data}})
	case *LinkagesResponse:
		return printLinkagesMarkdown(v)
	case *AppClipDefaultExperienceReviewDetailLinkageResponse:
		return printLinkagesMarkdown(&LinkagesResponse{Data: []ResourceData{v.Data}})
	case *AppClipDefaultExperienceReleaseWithAppStoreVersionLinkageResponse:
		return printLinkagesMarkdown(&LinkagesResponse{Data: []ResourceData{v.Data}})
	case *AppClipDefaultExperienceLocalizationHeaderImageLinkageResponse:
		return printLinkagesMarkdown(&LinkagesResponse{Data: []ResourceData{v.Data}})
	case *BundleIDsResponse:
		return printBundleIDsMarkdown(v)
	case *BundleIDResponse:
		return printBundleIDsMarkdown(&BundleIDsResponse{Data: []Resource[BundleIDAttributes]{v.Data}})
	case *MerchantIDsResponse:
		return printMerchantIDsMarkdown(v)
	case *MerchantIDResponse:
		return printMerchantIDsMarkdown(&MerchantIDsResponse{Data: []Resource[MerchantIDAttributes]{v.Data}})
	case *PassTypeIDsResponse:
		return printPassTypeIDsMarkdown(v)
	case *PassTypeIDResponse:
		return printPassTypeIDsMarkdown(&PassTypeIDsResponse{Data: []Resource[PassTypeIDAttributes]{v.Data}})
	case *CertificatesResponse:
		return printCertificatesMarkdown(v)
	case *CertificateResponse:
		return printCertificatesMarkdown(&CertificatesResponse{Data: []Resource[CertificateAttributes]{v.Data}})
	case *ProfilesResponse:
		return printProfilesMarkdown(v)
	case *ProfileResponse:
		return printProfilesMarkdown(&ProfilesResponse{Data: []Resource[ProfileAttributes]{v.Data}})
	case *InAppPurchasesV2Response:
		return printInAppPurchasesMarkdown(v)
	case *InAppPurchaseV2Response:
		return printInAppPurchasesMarkdown(&InAppPurchasesV2Response{Data: []Resource[InAppPurchaseV2Attributes]{v.Data}})
	case *InAppPurchaseLocalizationsResponse:
		return printInAppPurchaseLocalizationsMarkdown(v)
	case *AppEventsResponse:
		return printAppEventsMarkdown(v)
	case *AppEventResponse:
		return printAppEventsMarkdown(&AppEventsResponse{Data: []Resource[AppEventAttributes]{v.Data}})
	case *AppEventLocalizationsResponse:
		return printAppEventLocalizationsMarkdown(v)
	case *AppEventLocalizationResponse:
		return printAppEventLocalizationsMarkdown(&AppEventLocalizationsResponse{Data: []Resource[AppEventLocalizationAttributes]{v.Data}})
	case *AppEventScreenshotsResponse:
		return printAppEventScreenshotsMarkdown(v)
	case *AppEventScreenshotResponse:
		return printAppEventScreenshotsMarkdown(&AppEventScreenshotsResponse{Data: []Resource[AppEventScreenshotAttributes]{v.Data}})
	case *AppEventVideoClipsResponse:
		return printAppEventVideoClipsMarkdown(v)
	case *AppEventVideoClipResponse:
		return printAppEventVideoClipsMarkdown(&AppEventVideoClipsResponse{Data: []Resource[AppEventVideoClipAttributes]{v.Data}})
	case *SubscriptionGroupsResponse:
		return printSubscriptionGroupsMarkdown(v)
	case *SubscriptionGroupResponse:
		return printSubscriptionGroupsMarkdown(&SubscriptionGroupsResponse{Data: []Resource[SubscriptionGroupAttributes]{v.Data}})
	case *SubscriptionsResponse:
		return printSubscriptionsMarkdown(v)
	case *SubscriptionResponse:
		return printSubscriptionsMarkdown(&SubscriptionsResponse{Data: []Resource[SubscriptionAttributes]{v.Data}})
	case *PromotedPurchasesResponse:
		return printPromotedPurchasesMarkdown(v)
	case *PromotedPurchaseResponse:
		return printPromotedPurchasesMarkdown(&PromotedPurchasesResponse{Data: []Resource[PromotedPurchaseAttributes]{v.Data}})
	case *SubscriptionPriceResponse:
		return printSubscriptionPriceMarkdown(v)
	case *SubscriptionAvailabilityResponse:
		return printSubscriptionAvailabilityMarkdown(v)
	case *TerritoriesResponse:
		return printTerritoriesMarkdown(v)
	case *AppPricePointsV3Response:
		return printAppPricePointsMarkdown(v)
	case *AppPriceScheduleResponse:
		return printAppPriceScheduleMarkdown(v)
	case *AppPricesResponse:
		return printAppPricesMarkdown(v)
	case *BuildsResponse:
		return printBuildsMarkdown(v)
	case *BuildBundlesResponse:
		return printBuildBundlesMarkdown(v)
	case *BuildBundleFileSizesResponse:
		return printBuildBundleFileSizesMarkdown(v)
	case *BetaAppClipInvocationsResponse:
		return printBetaAppClipInvocationsMarkdown(v)
	case *BetaAppClipInvocationResponse:
		return printBetaAppClipInvocationsMarkdown(&BetaAppClipInvocationsResponse{Data: []Resource[BetaAppClipInvocationAttributes]{v.Data}})
	case *BetaAppClipInvocationLocalizationsResponse:
		return printBetaAppClipInvocationLocalizationsMarkdown(v)
	case *BetaAppClipInvocationLocalizationResponse:
		return printBetaAppClipInvocationLocalizationsMarkdown(&BetaAppClipInvocationLocalizationsResponse{Data: []Resource[BetaAppClipInvocationLocalizationAttributes]{v.Data}})
	case *SubscriptionOfferCodeOneTimeUseCodesResponse:
		return printOfferCodesMarkdown(v)
	case *WinBackOffersResponse:
		return printWinBackOffersMarkdown(v)
	case *WinBackOfferResponse:
		return printWinBackOffersMarkdown(&WinBackOffersResponse{Data: []Resource[WinBackOfferAttributes]{v.Data}})
	case *WinBackOfferPricesResponse:
		return printWinBackOfferPricesMarkdown(v)
	case *AppStoreVersionsResponse:
		return printAppStoreVersionsMarkdown(v)
	case *PreReleaseVersionsResponse:
		return printPreReleaseVersionsMarkdown(v)
	case *BuildResponse:
		return printBuildsMarkdown(&BuildsResponse{Data: []Resource[BuildAttributes]{v.Data}})
	case *AppClipDomainStatusResult:
		return printAppClipDomainStatusResultMarkdown(v)
	case *SubscriptionOfferCodeOneTimeUseCodeResponse:
		return printOfferCodesMarkdown(&SubscriptionOfferCodeOneTimeUseCodesResponse{Data: []Resource[SubscriptionOfferCodeOneTimeUseCodeAttributes]{v.Data}})
	case *WinBackOfferDeleteResult:
		return printWinBackOfferDeleteResultMarkdown(v)
	case *AppAvailabilityV2Response:
		return printAppAvailabilityMarkdown(v)
	case *TerritoryAvailabilitiesResponse:
		return printTerritoryAvailabilitiesMarkdown(v)
	case *EndAppAvailabilityPreOrderResponse:
		return printEndAppAvailabilityPreOrderMarkdown(v)
	case *PreReleaseVersionResponse:
		return printPreReleaseVersionsMarkdown(&PreReleaseVersionsResponse{Data: []PreReleaseVersion{v.Data}})
	case *AppStoreVersionLocalizationsResponse:
		return printAppStoreVersionLocalizationsMarkdown(v)
	case *AppStoreVersionLocalizationResponse:
		return printAppStoreVersionLocalizationsMarkdown(&AppStoreVersionLocalizationsResponse{Data: []Resource[AppStoreVersionLocalizationAttributes]{v.Data}})
	case *BetaBuildLocalizationsResponse:
		return printBetaBuildLocalizationsMarkdown(v)
	case *BetaBuildLocalizationResponse:
		return printBetaBuildLocalizationsMarkdown(&BetaBuildLocalizationsResponse{Data: []Resource[BetaBuildLocalizationAttributes]{v.Data}})
	case *AppInfoLocalizationsResponse:
		return printAppInfoLocalizationsMarkdown(v)
	case *AppScreenshotSetsResponse:
		return printAppScreenshotSetsMarkdown(v)
	case *AppScreenshotSetResponse:
		return printAppScreenshotSetsMarkdown(&AppScreenshotSetsResponse{Data: []Resource[AppScreenshotSetAttributes]{v.Data}})
	case *AppScreenshotsResponse:
		return printAppScreenshotsMarkdown(v)
	case *AppScreenshotResponse:
		return printAppScreenshotsMarkdown(&AppScreenshotsResponse{Data: []Resource[AppScreenshotAttributes]{v.Data}})
	case *AppPreviewSetsResponse:
		return printAppPreviewSetsMarkdown(v)
	case *AppPreviewSetResponse:
		return printAppPreviewSetsMarkdown(&AppPreviewSetsResponse{Data: []Resource[AppPreviewSetAttributes]{v.Data}})
	case *AppPreviewsResponse:
		return printAppPreviewsMarkdown(v)
	case *AppPreviewResponse:
		return printAppPreviewsMarkdown(&AppPreviewsResponse{Data: []Resource[AppPreviewAttributes]{v.Data}})
	case *BetaGroupsResponse:
		return printBetaGroupsMarkdown(v)
	case *BetaGroupResponse:
		return printBetaGroupsMarkdown(&BetaGroupsResponse{Data: []Resource[BetaGroupAttributes]{v.Data}})
	case *BetaTestersResponse:
		return printBetaTestersMarkdown(v)
	case *BetaTesterResponse:
		return printBetaTesterMarkdown(v)
	case *UsersResponse:
		return printUsersMarkdown(v)
	case *UserResponse:
		return printUsersMarkdown(&UsersResponse{Data: []Resource[UserAttributes]{v.Data}})
	case *ActorsResponse:
		return printActorsMarkdown(v)
	case *ActorResponse:
		return printActorsMarkdown(&ActorsResponse{Data: []Resource[ActorAttributes]{v.Data}})
	case *DevicesResponse:
		return printDevicesMarkdown(v)
	case *DeviceLocalUDIDResult:
		return printDeviceLocalUDIDMarkdown(v)
	case *DeviceResponse:
		return printDevicesMarkdown(&DevicesResponse{Data: []Resource[DeviceAttributes]{v.Data}})
	case *UserInvitationsResponse:
		return printUserInvitationsMarkdown(v)
	case *UserInvitationResponse:
		return printUserInvitationsMarkdown(&UserInvitationsResponse{Data: []Resource[UserInvitationAttributes]{v.Data}})
	case *UserDeleteResult:
		return printUserDeleteResultMarkdown(v)
	case *UserInvitationRevokeResult:
		return printUserInvitationRevokeResultMarkdown(v)
	case *BetaAppReviewDetailsResponse:
		return printBetaAppReviewDetailsMarkdown(v)
	case *BetaAppReviewDetailResponse:
		return printBetaAppReviewDetailMarkdown(v)
	case *BetaAppReviewSubmissionsResponse:
		return printBetaAppReviewSubmissionsMarkdown(v)
	case *BetaAppReviewSubmissionResponse:
		return printBetaAppReviewSubmissionMarkdown(v)
	case *BuildBetaDetailsResponse:
		return printBuildBetaDetailsMarkdown(v)
	case *BuildBetaDetailResponse:
		return printBuildBetaDetailMarkdown(v)
	case *AgeRatingDeclarationResponse:
		return printAgeRatingDeclarationMarkdown(v)
	case *AccessibilityDeclarationsResponse:
		return printAccessibilityDeclarationsMarkdown(v)
	case *AccessibilityDeclarationResponse:
		return printAccessibilityDeclarationMarkdown(v)
	case *AppStoreReviewDetailResponse:
		return printAppStoreReviewDetailMarkdown(v)
	case *AppStoreReviewAttachmentsResponse:
		return printAppStoreReviewAttachmentsMarkdown(v)
	case *AppStoreReviewAttachmentResponse:
		return printAppStoreReviewAttachmentMarkdown(v)
	case *AppClipAppStoreReviewDetailResponse:
		return printAppClipAppStoreReviewDetailMarkdown(v)
	case *RoutingAppCoverageResponse:
		return printRoutingAppCoverageMarkdown(v)
	case *AppEncryptionDeclarationsResponse:
		return printAppEncryptionDeclarationsMarkdown(v)
	case *AppEncryptionDeclarationResponse:
		return printAppEncryptionDeclarationMarkdown(v)
	case *AppEncryptionDeclarationDocumentResponse:
		return printAppEncryptionDeclarationDocumentMarkdown(v)
	case *BetaRecruitmentCriterionOptionsResponse:
		return printBetaRecruitmentCriterionOptionsMarkdown(v)
	case *BetaRecruitmentCriteriaResponse:
		return printBetaRecruitmentCriteriaMarkdown(v)
	case *Response[BetaGroupMetricAttributes]:
		return printBetaGroupMetricsMarkdown(v.Data)
	case *SandboxTestersResponse:
		return printSandboxTestersMarkdown(v)
	case *SandboxTesterResponse:
		return printSandboxTestersMarkdown(&SandboxTestersResponse{Data: []Resource[SandboxTesterAttributes]{v.Data}})
	case *BundleIDCapabilitiesResponse:
		return printBundleIDCapabilitiesMarkdown(v)
	case *BundleIDCapabilityResponse:
		return printBundleIDCapabilitiesMarkdown(&BundleIDCapabilitiesResponse{Data: []Resource[BundleIDCapabilityAttributes]{v.Data}})
	case *LocalizationDownloadResult:
		return printLocalizationDownloadResultMarkdown(v)
	case *LocalizationUploadResult:
		return printLocalizationUploadResultMarkdown(v)
	case *BuildUploadResult:
		return printBuildUploadResultMarkdown(v)
	case *BuildExpireAllResult:
		return printBuildExpireAllResultMarkdown(v)
	case *AppScreenshotListResult:
		return printAppScreenshotListResultMarkdown(v)
	case *AppPreviewListResult:
		return printAppPreviewListResultMarkdown(v)
	case *AppScreenshotUploadResult:
		return printAppScreenshotUploadResultMarkdown(v)
	case *AppPreviewUploadResult:
		return printAppPreviewUploadResultMarkdown(v)
	case *AppClipAdvancedExperienceImageUploadResult:
		return printAppClipAdvancedExperienceImageUploadResultMarkdown(v)
	case *AppClipHeaderImageUploadResult:
		return printAppClipHeaderImageUploadResultMarkdown(v)
	case *AssetDeleteResult:
		return printAssetDeleteResultMarkdown(v)
	case *AppClipDefaultExperienceDeleteResult:
		return printAppClipDefaultExperienceDeleteResultMarkdown(v)
	case *AppClipDefaultExperienceLocalizationDeleteResult:
		return printAppClipDefaultExperienceLocalizationDeleteResultMarkdown(v)
	case *AppClipAdvancedExperienceDeleteResult:
		return printAppClipAdvancedExperienceDeleteResultMarkdown(v)
	case *AppClipAdvancedExperienceImageDeleteResult:
		return printAppClipAdvancedExperienceImageDeleteResultMarkdown(v)
	case *AppClipHeaderImageDeleteResult:
		return printAppClipHeaderImageDeleteResultMarkdown(v)
	case *BetaAppClipInvocationDeleteResult:
		return printBetaAppClipInvocationDeleteResultMarkdown(v)
	case *BetaAppClipInvocationLocalizationDeleteResult:
		return printBetaAppClipInvocationLocalizationDeleteResultMarkdown(v)
	case *TestFlightPublishResult:
		return printTestFlightPublishResultMarkdown(v)
	case *AppStorePublishResult:
		return printAppStorePublishResultMarkdown(v)
	case *SalesReportResult:
		return printSalesReportResultMarkdown(v)
	case *FinanceReportResult:
		return printFinanceReportResultMarkdown(v)
	case *FinanceRegionsResult:
		return printFinanceRegionsMarkdown(v)
	case *AnalyticsReportRequestResult:
		return printAnalyticsReportRequestResultMarkdown(v)
	case *AnalyticsReportRequestsResponse:
		return printAnalyticsReportRequestsMarkdown(v)
	case *AnalyticsReportRequestResponse:
		return printAnalyticsReportRequestsMarkdown(&AnalyticsReportRequestsResponse{Data: []AnalyticsReportRequestResource{v.Data}, Links: v.Links})
	case *AnalyticsReportDownloadResult:
		return printAnalyticsReportDownloadResultMarkdown(v)
	case *AnalyticsReportGetResult:
		return printAnalyticsReportGetResultMarkdown(v)
	case *AppStoreVersionSubmissionResult:
		return printAppStoreVersionSubmissionMarkdown(v)
	case *AppStoreVersionSubmissionCreateResult:
		return printAppStoreVersionSubmissionCreateMarkdown(v)
	case *AppStoreVersionSubmissionStatusResult:
		return printAppStoreVersionSubmissionStatusMarkdown(v)
	case *AppStoreVersionSubmissionCancelResult:
		return printAppStoreVersionSubmissionCancelMarkdown(v)
	case *AppStoreVersionDetailResult:
		return printAppStoreVersionDetailMarkdown(v)
	case *AppStoreVersionAttachBuildResult:
		return printAppStoreVersionAttachBuildMarkdown(v)
	case *ReviewSubmissionsResponse:
		return printReviewSubmissionsMarkdown(v)
	case *ReviewSubmissionResponse:
		return printReviewSubmissionsMarkdown(&ReviewSubmissionsResponse{Data: []ReviewSubmissionResource{v.Data}, Links: v.Links})
	case *ReviewSubmissionItemsResponse:
		return printReviewSubmissionItemsMarkdown(v)
	case *ReviewSubmissionItemResponse:
		return printReviewSubmissionItemsMarkdown(&ReviewSubmissionItemsResponse{Data: []ReviewSubmissionItemResource{v.Data}, Links: v.Links})
	case *ReviewSubmissionItemDeleteResult:
		return printReviewSubmissionItemDeleteResultMarkdown(v)
	case *AppStoreVersionReleaseRequestResult:
		return printAppStoreVersionReleaseRequestMarkdown(v)
	case *AppStoreVersionPromotionCreateResult:
		return printAppStoreVersionPromotionCreateMarkdown(v)
	case *AppStoreVersionPhasedReleaseResponse:
		return printAppStoreVersionPhasedReleaseMarkdown(v)
	case *AppStoreVersionPhasedReleaseDeleteResult:
		return printAppStoreVersionPhasedReleaseDeleteResultMarkdown(v)
	case *BuildBetaGroupsUpdateResult:
		return printBuildBetaGroupsUpdateMarkdown(v)
	case *InAppPurchaseDeleteResult:
		return printInAppPurchaseDeleteResultMarkdown(v)
	case *AppEventDeleteResult:
		return printAppEventDeleteResultMarkdown(v)
	case *AppEventLocalizationDeleteResult:
		return printAppEventLocalizationDeleteResultMarkdown(v)
	case *AppEventSubmissionResult:
		return printAppEventSubmissionResultMarkdown(v)
	case *GameCenterAchievementsResponse:
		return printGameCenterAchievementsMarkdown(v)
	case *GameCenterAchievementResponse:
		return printGameCenterAchievementsMarkdown(&GameCenterAchievementsResponse{Data: []Resource[GameCenterAchievementAttributes]{v.Data}})
	case *GameCenterAchievementDeleteResult:
		return printGameCenterAchievementDeleteResultMarkdown(v)
	case *GameCenterLeaderboardsResponse:
		return printGameCenterLeaderboardsMarkdown(v)
	case *GameCenterLeaderboardResponse:
		return printGameCenterLeaderboardsMarkdown(&GameCenterLeaderboardsResponse{Data: []Resource[GameCenterLeaderboardAttributes]{v.Data}})
	case *GameCenterLeaderboardDeleteResult:
		return printGameCenterLeaderboardDeleteResultMarkdown(v)
	case *GameCenterLeaderboardSetsResponse:
		return printGameCenterLeaderboardSetsMarkdown(v)
	case *GameCenterLeaderboardSetResponse:
		return printGameCenterLeaderboardSetsMarkdown(&GameCenterLeaderboardSetsResponse{Data: []Resource[GameCenterLeaderboardSetAttributes]{v.Data}})
	case *GameCenterLeaderboardSetDeleteResult:
		return printGameCenterLeaderboardSetDeleteResultMarkdown(v)
	case *GameCenterLeaderboardLocalizationsResponse:
		return printGameCenterLeaderboardLocalizationsMarkdown(v)
	case *GameCenterLeaderboardLocalizationResponse:
		return printGameCenterLeaderboardLocalizationsMarkdown(&GameCenterLeaderboardLocalizationsResponse{Data: []Resource[GameCenterLeaderboardLocalizationAttributes]{v.Data}})
	case *GameCenterLeaderboardLocalizationDeleteResult:
		return printGameCenterLeaderboardLocalizationDeleteResultMarkdown(v)
	case *GameCenterLeaderboardReleasesResponse:
		return printGameCenterLeaderboardReleasesMarkdown(v)
	case *GameCenterLeaderboardReleaseResponse:
		return printGameCenterLeaderboardReleasesMarkdown(&GameCenterLeaderboardReleasesResponse{Data: []Resource[GameCenterLeaderboardReleaseAttributes]{v.Data}})
	case *GameCenterLeaderboardReleaseDeleteResult:
		return printGameCenterLeaderboardReleaseDeleteResultMarkdown(v)
	case *GameCenterAchievementReleasesResponse:
		return printGameCenterAchievementReleasesMarkdown(v)
	case *GameCenterAchievementReleaseResponse:
		return printGameCenterAchievementReleasesMarkdown(&GameCenterAchievementReleasesResponse{Data: []Resource[GameCenterAchievementReleaseAttributes]{v.Data}})
	case *GameCenterAchievementReleaseDeleteResult:
		return printGameCenterAchievementReleaseDeleteResultMarkdown(v)
	case *GameCenterLeaderboardSetReleasesResponse:
		return printGameCenterLeaderboardSetReleasesMarkdown(v)
	case *GameCenterLeaderboardSetReleaseResponse:
		return printGameCenterLeaderboardSetReleasesMarkdown(&GameCenterLeaderboardSetReleasesResponse{Data: []Resource[GameCenterLeaderboardSetReleaseAttributes]{v.Data}})
	case *GameCenterLeaderboardSetReleaseDeleteResult:
		return printGameCenterLeaderboardSetReleaseDeleteResultMarkdown(v)
	case *GameCenterLeaderboardSetLocalizationsResponse:
		return printGameCenterLeaderboardSetLocalizationsMarkdown(v)
	case *GameCenterLeaderboardSetLocalizationResponse:
		return printGameCenterLeaderboardSetLocalizationsMarkdown(&GameCenterLeaderboardSetLocalizationsResponse{Data: []Resource[GameCenterLeaderboardSetLocalizationAttributes]{v.Data}})
	case *GameCenterLeaderboardSetLocalizationDeleteResult:
		return printGameCenterLeaderboardSetLocalizationDeleteResultMarkdown(v)
	case *GameCenterAchievementLocalizationsResponse:
		return printGameCenterAchievementLocalizationsMarkdown(v)
	case *GameCenterAchievementLocalizationResponse:
		return printGameCenterAchievementLocalizationsMarkdown(&GameCenterAchievementLocalizationsResponse{Data: []Resource[GameCenterAchievementLocalizationAttributes]{v.Data}})
	case *GameCenterAchievementLocalizationDeleteResult:
		return printGameCenterAchievementLocalizationDeleteResultMarkdown(v)
	case *GameCenterLeaderboardImageUploadResult:
		return printGameCenterLeaderboardImageUploadResultMarkdown(v)
	case *GameCenterLeaderboardImageDeleteResult:
		return printGameCenterLeaderboardImageDeleteResultMarkdown(v)
	case *GameCenterAchievementImageUploadResult:
		return printGameCenterAchievementImageUploadResultMarkdown(v)
	case *GameCenterAchievementImageDeleteResult:
		return printGameCenterAchievementImageDeleteResultMarkdown(v)
	case *GameCenterLeaderboardSetImageUploadResult:
		return printGameCenterLeaderboardSetImageUploadResultMarkdown(v)
	case *GameCenterLeaderboardSetImageDeleteResult:
		return printGameCenterLeaderboardSetImageDeleteResultMarkdown(v)
	case *SubscriptionGroupDeleteResult:
		return printSubscriptionGroupDeleteResultMarkdown(v)
	case *SubscriptionDeleteResult:
		return printSubscriptionDeleteResultMarkdown(v)
	case *BetaTesterDeleteResult:
		return printBetaTesterDeleteResultMarkdown(v)
	case *BetaTesterGroupsUpdateResult:
		return printBetaTesterGroupsUpdateResultMarkdown(v)
	case *AppStoreVersionLocalizationDeleteResult:
		return printAppStoreVersionLocalizationDeleteResultMarkdown(v)
	case *BetaBuildLocalizationDeleteResult:
		return printBetaBuildLocalizationDeleteResultMarkdown(v)
	case *BetaTesterInvitationResult:
		return printBetaTesterInvitationResultMarkdown(v)
	case *PromotedPurchaseDeleteResult:
		return printPromotedPurchaseDeleteResultMarkdown(v)
	case *AppPromotedPurchasesLinkResult:
		return printAppPromotedPurchasesLinkResultMarkdown(v)
	case *SandboxTesterClearHistoryResult:
		return printSandboxTesterClearHistoryResultMarkdown(v)
	case *BundleIDDeleteResult:
		return printBundleIDDeleteResultMarkdown(v)
	case *MarketplaceSearchDetailDeleteResult:
		return printMarketplaceSearchDetailDeleteResultMarkdown(v)
	case *MarketplaceWebhookDeleteResult:
		return printMarketplaceWebhookDeleteResultMarkdown(v)
	case *WebhookDeleteResult:
		return printWebhookDeleteResultMarkdown(v)
	case *WebhookPingResponse:
		return printWebhookPingMarkdown(v)
	case *MerchantIDDeleteResult:
		return printMerchantIDDeleteResultMarkdown(v)
	case *PassTypeIDDeleteResult:
		return printPassTypeIDDeleteResultMarkdown(v)
	case *BundleIDCapabilityDeleteResult:
		return printBundleIDCapabilityDeleteResultMarkdown(v)
	case *CertificateRevokeResult:
		return printCertificateRevokeResultMarkdown(v)
	case *ProfileDeleteResult:
		return printProfileDeleteResultMarkdown(v)
	case *ProfileDownloadResult:
		return printProfileDownloadResultMarkdown(v)
	case *SigningFetchResult:
		return printSigningFetchResultMarkdown(v)
	case *XcodeCloudRunResult:
		return printXcodeCloudRunResultMarkdown(v)
	case *XcodeCloudStatusResult:
		return printXcodeCloudStatusResultMarkdown(v)
	case *CiProductsResponse:
		return printCiProductsMarkdown(v)
	case *CiProductResponse:
		return printCiProductsMarkdown(&CiProductsResponse{Data: []CiProductResource{v.Data}})
	case *CiWorkflowsResponse:
		return printCiWorkflowsMarkdown(v)
	case *CiWorkflowResponse:
		return printCiWorkflowsMarkdown(&CiWorkflowsResponse{Data: []CiWorkflowResource{v.Data}})
	case *ScmRepositoriesResponse:
		return printScmRepositoriesMarkdown(v)
	case *CiBuildRunsResponse:
		return printCiBuildRunsMarkdown(v)
	case *CiBuildRunResponse:
		return printCiBuildRunsMarkdown(&CiBuildRunsResponse{Data: []CiBuildRunResource{v.Data}})
	case *CiBuildActionsResponse:
		return printCiBuildActionsMarkdown(v)
	case *CiBuildActionResponse:
		return printCiBuildActionsMarkdown(&CiBuildActionsResponse{Data: []CiBuildActionResource{v.Data}})
	case *CiMacOsVersionsResponse:
		return printCiMacOsVersionsMarkdown(v)
	case *CiMacOsVersionResponse:
		return printCiMacOsVersionsMarkdown(&CiMacOsVersionsResponse{Data: []CiMacOsVersionResource{v.Data}})
	case *CiXcodeVersionsResponse:
		return printCiXcodeVersionsMarkdown(v)
	case *CiXcodeVersionResponse:
		return printCiXcodeVersionsMarkdown(&CiXcodeVersionsResponse{Data: []CiXcodeVersionResource{v.Data}})
	case *CiArtifactsResponse:
		return printCiArtifactsMarkdown(v)
	case *CiArtifactResponse:
		return printCiArtifactMarkdown(v)
	case *CiTestResultsResponse:
		return printCiTestResultsMarkdown(v)
	case *CiTestResultResponse:
		return printCiTestResultMarkdown(v)
	case *CiIssuesResponse:
		return printCiIssuesMarkdown(v)
	case *CiIssueResponse:
		return printCiIssueMarkdown(v)
	case *CiArtifactDownloadResult:
		return printCiArtifactDownloadResultMarkdown(v)
	case *CiWorkflowDeleteResult:
		return printCiWorkflowDeleteResultMarkdown(v)
	case *CiProductDeleteResult:
		return printCiProductDeleteResultMarkdown(v)
	case *EndUserLicenseAgreementResponse:
		return printEndUserLicenseAgreementMarkdown(v)
	case *EndUserLicenseAgreementDeleteResult:
		return printEndUserLicenseAgreementDeleteResultMarkdown(v)
	case *CustomerReviewResponseResponse:
		return printCustomerReviewResponseMarkdown(v)
	case *CustomerReviewResponseDeleteResult:
		return printCustomerReviewResponseDeleteResultMarkdown(v)
	case *AccessibilityDeclarationDeleteResult:
		return printAccessibilityDeclarationDeleteResultMarkdown(v)
	case *AppStoreReviewAttachmentDeleteResult:
		return printAppStoreReviewAttachmentDeleteResultMarkdown(v)
	case *RoutingAppCoverageDeleteResult:
		return printRoutingAppCoverageDeleteResultMarkdown(v)
	case *NominationDeleteResult:
		return printNominationDeleteResultMarkdown(v)
	case *AppEncryptionDeclarationBuildsUpdateResult:
		return printAppEncryptionDeclarationBuildsUpdateResultMarkdown(v)
	case *AndroidToIosAppMappingDetailsResponse:
		return printAndroidToIosAppMappingDetailsMarkdown(v)
	case *AndroidToIosAppMappingDetailResponse:
		return printAndroidToIosAppMappingDetailsMarkdown(&AndroidToIosAppMappingDetailsResponse{Data: []Resource[AndroidToIosAppMappingDetailAttributes]{v.Data}})
	case *AndroidToIosAppMappingDeleteResult:
		return printAndroidToIosAppMappingDeleteResultMarkdown(v)
	case *AlternativeDistributionDomainDeleteResult:
		return printAlternativeDistributionDomainDeleteResultMarkdown(v)
	case *AlternativeDistributionKeyDeleteResult:
		return printAlternativeDistributionKeyDeleteResultMarkdown(v)
	case *AppCustomProductPagesResponse:
		return printAppCustomProductPagesMarkdown(v)
	case *AppCustomProductPageResponse:
		return printAppCustomProductPagesMarkdown(&AppCustomProductPagesResponse{Data: []Resource[AppCustomProductPageAttributes]{v.Data}})
	case *AppCustomProductPageVersionsResponse:
		return printAppCustomProductPageVersionsMarkdown(v)
	case *AppCustomProductPageVersionResponse:
		return printAppCustomProductPageVersionsMarkdown(&AppCustomProductPageVersionsResponse{Data: []Resource[AppCustomProductPageVersionAttributes]{v.Data}})
	case *AppCustomProductPageLocalizationsResponse:
		return printAppCustomProductPageLocalizationsMarkdown(v)
	case *AppCustomProductPageLocalizationResponse:
		return printAppCustomProductPageLocalizationsMarkdown(&AppCustomProductPageLocalizationsResponse{Data: []Resource[AppCustomProductPageLocalizationAttributes]{v.Data}})
	case *AppStoreVersionExperimentsResponse:
		return printAppStoreVersionExperimentsMarkdown(v)
	case *AppStoreVersionExperimentResponse:
		return printAppStoreVersionExperimentsMarkdown(&AppStoreVersionExperimentsResponse{Data: []Resource[AppStoreVersionExperimentAttributes]{v.Data}})
	case *AppStoreVersionExperimentsV2Response:
		return printAppStoreVersionExperimentsV2Markdown(v)
	case *AppStoreVersionExperimentV2Response:
		return printAppStoreVersionExperimentsV2Markdown(&AppStoreVersionExperimentsV2Response{Data: []Resource[AppStoreVersionExperimentV2Attributes]{v.Data}})
	case *AppStoreVersionExperimentTreatmentsResponse:
		return printAppStoreVersionExperimentTreatmentsMarkdown(v)
	case *AppStoreVersionExperimentTreatmentResponse:
		return printAppStoreVersionExperimentTreatmentsMarkdown(&AppStoreVersionExperimentTreatmentsResponse{Data: []Resource[AppStoreVersionExperimentTreatmentAttributes]{v.Data}})
	case *AppStoreVersionExperimentTreatmentLocalizationsResponse:
		return printAppStoreVersionExperimentTreatmentLocalizationsMarkdown(v)
	case *AppStoreVersionExperimentTreatmentLocalizationResponse:
		return printAppStoreVersionExperimentTreatmentLocalizationsMarkdown(&AppStoreVersionExperimentTreatmentLocalizationsResponse{Data: []Resource[AppStoreVersionExperimentTreatmentLocalizationAttributes]{v.Data}})
	case *AppCustomProductPageDeleteResult:
		return printAppCustomProductPageDeleteResultMarkdown(v)
	case *AppCustomProductPageLocalizationDeleteResult:
		return printAppCustomProductPageLocalizationDeleteResultMarkdown(v)
	case *AppStoreVersionExperimentDeleteResult:
		return printAppStoreVersionExperimentDeleteResultMarkdown(v)
	case *AppStoreVersionExperimentTreatmentDeleteResult:
		return printAppStoreVersionExperimentTreatmentDeleteResultMarkdown(v)
	case *AppStoreVersionExperimentTreatmentLocalizationDeleteResult:
		return printAppStoreVersionExperimentTreatmentLocalizationDeleteResultMarkdown(v)
	case *PerfPowerMetricsResponse:
		return printPerfPowerMetricsMarkdown(v)
	case *DiagnosticSignaturesResponse:
		return printDiagnosticSignaturesMarkdown(v)
	case *DiagnosticLogsResponse:
		return printDiagnosticLogsMarkdown(v)
	case *PerformanceDownloadResult:
		return printPerformanceDownloadResultMarkdown(v)
	default:
		return PrintJSON(data)
	}
}

// PrintTable prints data as a formatted table
func PrintTable(data interface{}) error {
	switch v := data.(type) {
	case *FeedbackResponse:
		return printFeedbackTable(v)
	case *CrashesResponse:
		return printCrashesTable(v)
	case *ReviewsResponse:
		return printReviewsTable(v)
	case *AppsResponse:
		return printAppsTable(v)
	case *AppClipsResponse:
		return printAppClipsTable(v)
	case *AppCategoriesResponse:
		return printAppCategoriesTable(v)
	case *AppResponse:
		return printAppsTable(&AppsResponse{Data: []Resource[AppAttributes]{v.Data}})
	case *AppClipResponse:
		return printAppClipsTable(&AppClipsResponse{Data: []Resource[AppClipAttributes]{v.Data}})
	case *AppClipDefaultExperiencesResponse:
		return printAppClipDefaultExperiencesTable(v)
	case *AppClipDefaultExperienceResponse:
		return printAppClipDefaultExperiencesTable(&AppClipDefaultExperiencesResponse{Data: []Resource[AppClipDefaultExperienceAttributes]{v.Data}})
	case *AppClipDefaultExperienceLocalizationsResponse:
		return printAppClipDefaultExperienceLocalizationsTable(v)
	case *AppClipDefaultExperienceLocalizationResponse:
		return printAppClipDefaultExperienceLocalizationsTable(&AppClipDefaultExperienceLocalizationsResponse{Data: []Resource[AppClipDefaultExperienceLocalizationAttributes]{v.Data}})
	case *AppClipAdvancedExperiencesResponse:
		return printAppClipAdvancedExperiencesTable(v)
	case *AppClipAdvancedExperienceResponse:
		return printAppClipAdvancedExperiencesTable(&AppClipAdvancedExperiencesResponse{Data: []Resource[AppClipAdvancedExperienceAttributes]{v.Data}})
	case *AppSetupInfoResult:
		return printAppSetupInfoResultTable(v)
	case *AppTagsResponse:
		return printAppTagsTable(v)
	case *AppTagResponse:
		return printAppTagsTable(&AppTagsResponse{Data: []Resource[AppTagAttributes]{v.Data}})
	case *MarketplaceSearchDetailsResponse:
		return printMarketplaceSearchDetailsTable(v)
	case *MarketplaceSearchDetailResponse:
		return printMarketplaceSearchDetailTable(v)
	case *MarketplaceWebhooksResponse:
		return printMarketplaceWebhooksTable(v)
	case *MarketplaceWebhookResponse:
		return printMarketplaceWebhookTable(v)
	case *WebhooksResponse:
		return printWebhooksTable(v)
	case *WebhookResponse:
		return printWebhooksTable(&WebhooksResponse{Data: []Resource[WebhookAttributes]{v.Data}})
	case *WebhookDeliveriesResponse:
		return printWebhookDeliveriesTable(v)
	case *WebhookDeliveryResponse:
		return printWebhookDeliveriesTable(&WebhookDeliveriesResponse{Data: []Resource[WebhookDeliveryAttributes]{v.Data}})
	case *AlternativeDistributionDomainsResponse:
		return printAlternativeDistributionDomainsTable(v)
	case *AlternativeDistributionDomainResponse:
		return printAlternativeDistributionDomainsTable(&AlternativeDistributionDomainsResponse{Data: []Resource[AlternativeDistributionDomainAttributes]{v.Data}})
	case *AlternativeDistributionKeysResponse:
		return printAlternativeDistributionKeysTable(v)
	case *AlternativeDistributionKeyResponse:
		return printAlternativeDistributionKeysTable(&AlternativeDistributionKeysResponse{Data: []Resource[AlternativeDistributionKeyAttributes]{v.Data}})
	case *AlternativeDistributionPackageResponse:
		return printAlternativeDistributionPackageTable(v)
	case *AlternativeDistributionPackageVersionsResponse:
		return printAlternativeDistributionPackageVersionsTable(v)
	case *AlternativeDistributionPackageVersionResponse:
		return printAlternativeDistributionPackageVersionsTable(&AlternativeDistributionPackageVersionsResponse{Data: []Resource[AlternativeDistributionPackageVersionAttributes]{v.Data}})
	case *AlternativeDistributionPackageVariantsResponse:
		return printAlternativeDistributionPackageVariantsTable(v)
	case *AlternativeDistributionPackageVariantResponse:
		return printAlternativeDistributionPackageVariantsTable(&AlternativeDistributionPackageVariantsResponse{Data: []Resource[AlternativeDistributionPackageVariantAttributes]{v.Data}})
	case *AlternativeDistributionPackageDeltasResponse:
		return printAlternativeDistributionPackageDeltasTable(v)
	case *AlternativeDistributionPackageDeltaResponse:
		return printAlternativeDistributionPackageDeltasTable(&AlternativeDistributionPackageDeltasResponse{Data: []Resource[AlternativeDistributionPackageDeltaAttributes]{v.Data}})
	case *BackgroundAssetsResponse:
		return printBackgroundAssetsTable(v)
	case *BackgroundAssetResponse:
		return printBackgroundAssetsTable(&BackgroundAssetsResponse{Data: []Resource[BackgroundAssetAttributes]{v.Data}})
	case *BackgroundAssetVersionsResponse:
		return printBackgroundAssetVersionsTable(v)
	case *BackgroundAssetVersionResponse:
		return printBackgroundAssetVersionsTable(&BackgroundAssetVersionsResponse{Data: []Resource[BackgroundAssetVersionAttributes]{v.Data}})
	case *BackgroundAssetUploadFilesResponse:
		return printBackgroundAssetUploadFilesTable(v)
	case *BackgroundAssetUploadFileResponse:
		return printBackgroundAssetUploadFilesTable(&BackgroundAssetUploadFilesResponse{Data: []Resource[BackgroundAssetUploadFileAttributes]{v.Data}})
	case *NominationsResponse:
		return printNominationsTable(v)
	case *NominationResponse:
		return printNominationsTable(&NominationsResponse{Data: []Resource[NominationAttributes]{v.Data}})
	case *LinkagesResponse:
		return printLinkagesTable(v)
	case *AppClipDefaultExperienceReviewDetailLinkageResponse:
		return printLinkagesTable(&LinkagesResponse{Data: []ResourceData{v.Data}})
	case *AppClipDefaultExperienceReleaseWithAppStoreVersionLinkageResponse:
		return printLinkagesTable(&LinkagesResponse{Data: []ResourceData{v.Data}})
	case *AppClipDefaultExperienceLocalizationHeaderImageLinkageResponse:
		return printLinkagesTable(&LinkagesResponse{Data: []ResourceData{v.Data}})
	case *BundleIDsResponse:
		return printBundleIDsTable(v)
	case *BundleIDResponse:
		return printBundleIDsTable(&BundleIDsResponse{Data: []Resource[BundleIDAttributes]{v.Data}})
	case *MerchantIDsResponse:
		return printMerchantIDsTable(v)
	case *MerchantIDResponse:
		return printMerchantIDsTable(&MerchantIDsResponse{Data: []Resource[MerchantIDAttributes]{v.Data}})
	case *PassTypeIDsResponse:
		return printPassTypeIDsTable(v)
	case *PassTypeIDResponse:
		return printPassTypeIDsTable(&PassTypeIDsResponse{Data: []Resource[PassTypeIDAttributes]{v.Data}})
	case *CertificatesResponse:
		return printCertificatesTable(v)
	case *CertificateResponse:
		return printCertificatesTable(&CertificatesResponse{Data: []Resource[CertificateAttributes]{v.Data}})
	case *ProfilesResponse:
		return printProfilesTable(v)
	case *ProfileResponse:
		return printProfilesTable(&ProfilesResponse{Data: []Resource[ProfileAttributes]{v.Data}})
	case *InAppPurchasesV2Response:
		return printInAppPurchasesTable(v)
	case *InAppPurchaseV2Response:
		return printInAppPurchasesTable(&InAppPurchasesV2Response{Data: []Resource[InAppPurchaseV2Attributes]{v.Data}})
	case *InAppPurchaseLocalizationsResponse:
		return printInAppPurchaseLocalizationsTable(v)
	case *AppEventsResponse:
		return printAppEventsTable(v)
	case *AppEventResponse:
		return printAppEventsTable(&AppEventsResponse{Data: []Resource[AppEventAttributes]{v.Data}})
	case *AppEventLocalizationsResponse:
		return printAppEventLocalizationsTable(v)
	case *AppEventLocalizationResponse:
		return printAppEventLocalizationsTable(&AppEventLocalizationsResponse{Data: []Resource[AppEventLocalizationAttributes]{v.Data}})
	case *AppEventScreenshotsResponse:
		return printAppEventScreenshotsTable(v)
	case *AppEventScreenshotResponse:
		return printAppEventScreenshotsTable(&AppEventScreenshotsResponse{Data: []Resource[AppEventScreenshotAttributes]{v.Data}})
	case *AppEventVideoClipsResponse:
		return printAppEventVideoClipsTable(v)
	case *AppEventVideoClipResponse:
		return printAppEventVideoClipsTable(&AppEventVideoClipsResponse{Data: []Resource[AppEventVideoClipAttributes]{v.Data}})
	case *SubscriptionGroupsResponse:
		return printSubscriptionGroupsTable(v)
	case *SubscriptionGroupResponse:
		return printSubscriptionGroupsTable(&SubscriptionGroupsResponse{Data: []Resource[SubscriptionGroupAttributes]{v.Data}})
	case *SubscriptionsResponse:
		return printSubscriptionsTable(v)
	case *SubscriptionResponse:
		return printSubscriptionsTable(&SubscriptionsResponse{Data: []Resource[SubscriptionAttributes]{v.Data}})
	case *PromotedPurchasesResponse:
		return printPromotedPurchasesTable(v)
	case *PromotedPurchaseResponse:
		return printPromotedPurchasesTable(&PromotedPurchasesResponse{Data: []Resource[PromotedPurchaseAttributes]{v.Data}})
	case *SubscriptionPriceResponse:
		return printSubscriptionPriceTable(v)
	case *SubscriptionAvailabilityResponse:
		return printSubscriptionAvailabilityTable(v)
	case *TerritoriesResponse:
		return printTerritoriesTable(v)
	case *AppPricePointsV3Response:
		return printAppPricePointsTable(v)
	case *AppPriceScheduleResponse:
		return printAppPriceScheduleTable(v)
	case *AppPricesResponse:
		return printAppPricesTable(v)
	case *BuildsResponse:
		return printBuildsTable(v)
	case *BuildBundlesResponse:
		return printBuildBundlesTable(v)
	case *BuildBundleFileSizesResponse:
		return printBuildBundleFileSizesTable(v)
	case *BetaAppClipInvocationsResponse:
		return printBetaAppClipInvocationsTable(v)
	case *BetaAppClipInvocationResponse:
		return printBetaAppClipInvocationsTable(&BetaAppClipInvocationsResponse{Data: []Resource[BetaAppClipInvocationAttributes]{v.Data}})
	case *BetaAppClipInvocationLocalizationsResponse:
		return printBetaAppClipInvocationLocalizationsTable(v)
	case *BetaAppClipInvocationLocalizationResponse:
		return printBetaAppClipInvocationLocalizationsTable(&BetaAppClipInvocationLocalizationsResponse{Data: []Resource[BetaAppClipInvocationLocalizationAttributes]{v.Data}})
	case *SubscriptionOfferCodeOneTimeUseCodesResponse:
		return printOfferCodesTable(v)
	case *WinBackOffersResponse:
		return printWinBackOffersTable(v)
	case *WinBackOfferResponse:
		return printWinBackOffersTable(&WinBackOffersResponse{Data: []Resource[WinBackOfferAttributes]{v.Data}})
	case *WinBackOfferPricesResponse:
		return printWinBackOfferPricesTable(v)
	case *AppStoreVersionsResponse:
		return printAppStoreVersionsTable(v)
	case *PreReleaseVersionsResponse:
		return printPreReleaseVersionsTable(v)
	case *BuildResponse:
		return printBuildsTable(&BuildsResponse{Data: []Resource[BuildAttributes]{v.Data}})
	case *AppClipDomainStatusResult:
		return printAppClipDomainStatusResultTable(v)
	case *SubscriptionOfferCodeOneTimeUseCodeResponse:
		return printOfferCodesTable(&SubscriptionOfferCodeOneTimeUseCodesResponse{Data: []Resource[SubscriptionOfferCodeOneTimeUseCodeAttributes]{v.Data}})
	case *WinBackOfferDeleteResult:
		return printWinBackOfferDeleteResultTable(v)
	case *AppAvailabilityV2Response:
		return printAppAvailabilityTable(v)
	case *TerritoryAvailabilitiesResponse:
		return printTerritoryAvailabilitiesTable(v)
	case *EndAppAvailabilityPreOrderResponse:
		return printEndAppAvailabilityPreOrderTable(v)
	case *PreReleaseVersionResponse:
		return printPreReleaseVersionsTable(&PreReleaseVersionsResponse{Data: []PreReleaseVersion{v.Data}})
	case *AppStoreVersionLocalizationsResponse:
		return printAppStoreVersionLocalizationsTable(v)
	case *AppStoreVersionLocalizationResponse:
		return printAppStoreVersionLocalizationsTable(&AppStoreVersionLocalizationsResponse{Data: []Resource[AppStoreVersionLocalizationAttributes]{v.Data}})
	case *BetaBuildLocalizationsResponse:
		return printBetaBuildLocalizationsTable(v)
	case *BetaBuildLocalizationResponse:
		return printBetaBuildLocalizationsTable(&BetaBuildLocalizationsResponse{Data: []Resource[BetaBuildLocalizationAttributes]{v.Data}})
	case *AppInfoLocalizationsResponse:
		return printAppInfoLocalizationsTable(v)
	case *AppScreenshotSetsResponse:
		return printAppScreenshotSetsTable(v)
	case *AppScreenshotSetResponse:
		return printAppScreenshotSetsTable(&AppScreenshotSetsResponse{Data: []Resource[AppScreenshotSetAttributes]{v.Data}})
	case *AppScreenshotsResponse:
		return printAppScreenshotsTable(v)
	case *AppScreenshotResponse:
		return printAppScreenshotsTable(&AppScreenshotsResponse{Data: []Resource[AppScreenshotAttributes]{v.Data}})
	case *AppPreviewSetsResponse:
		return printAppPreviewSetsTable(v)
	case *AppPreviewSetResponse:
		return printAppPreviewSetsTable(&AppPreviewSetsResponse{Data: []Resource[AppPreviewSetAttributes]{v.Data}})
	case *AppPreviewsResponse:
		return printAppPreviewsTable(v)
	case *AppPreviewResponse:
		return printAppPreviewsTable(&AppPreviewsResponse{Data: []Resource[AppPreviewAttributes]{v.Data}})
	case *BetaGroupsResponse:
		return printBetaGroupsTable(v)
	case *BetaGroupResponse:
		return printBetaGroupsTable(&BetaGroupsResponse{Data: []Resource[BetaGroupAttributes]{v.Data}})
	case *BetaTestersResponse:
		return printBetaTestersTable(v)
	case *BetaTesterResponse:
		return printBetaTesterTable(v)
	case *UsersResponse:
		return printUsersTable(v)
	case *UserResponse:
		return printUsersTable(&UsersResponse{Data: []Resource[UserAttributes]{v.Data}})
	case *ActorsResponse:
		return printActorsTable(v)
	case *ActorResponse:
		return printActorsTable(&ActorsResponse{Data: []Resource[ActorAttributes]{v.Data}})
	case *DevicesResponse:
		return printDevicesTable(v)
	case *DeviceLocalUDIDResult:
		return printDeviceLocalUDIDTable(v)
	case *DeviceResponse:
		return printDevicesTable(&DevicesResponse{Data: []Resource[DeviceAttributes]{v.Data}})
	case *UserInvitationsResponse:
		return printUserInvitationsTable(v)
	case *UserInvitationResponse:
		return printUserInvitationsTable(&UserInvitationsResponse{Data: []Resource[UserInvitationAttributes]{v.Data}})
	case *UserDeleteResult:
		return printUserDeleteResultTable(v)
	case *UserInvitationRevokeResult:
		return printUserInvitationRevokeResultTable(v)
	case *BetaAppReviewDetailsResponse:
		return printBetaAppReviewDetailsTable(v)
	case *BetaAppReviewDetailResponse:
		return printBetaAppReviewDetailTable(v)
	case *BetaAppReviewSubmissionsResponse:
		return printBetaAppReviewSubmissionsTable(v)
	case *BetaAppReviewSubmissionResponse:
		return printBetaAppReviewSubmissionTable(v)
	case *BuildBetaDetailsResponse:
		return printBuildBetaDetailsTable(v)
	case *BuildBetaDetailResponse:
		return printBuildBetaDetailTable(v)
	case *AgeRatingDeclarationResponse:
		return printAgeRatingDeclarationTable(v)
	case *AccessibilityDeclarationsResponse:
		return printAccessibilityDeclarationsTable(v)
	case *AccessibilityDeclarationResponse:
		return printAccessibilityDeclarationTable(v)
	case *AppStoreReviewDetailResponse:
		return printAppStoreReviewDetailTable(v)
	case *AppStoreReviewAttachmentsResponse:
		return printAppStoreReviewAttachmentsTable(v)
	case *AppStoreReviewAttachmentResponse:
		return printAppStoreReviewAttachmentTable(v)
	case *AppClipAppStoreReviewDetailResponse:
		return printAppClipAppStoreReviewDetailTable(v)
	case *RoutingAppCoverageResponse:
		return printRoutingAppCoverageTable(v)
	case *AppEncryptionDeclarationsResponse:
		return printAppEncryptionDeclarationsTable(v)
	case *AppEncryptionDeclarationResponse:
		return printAppEncryptionDeclarationTable(v)
	case *AppEncryptionDeclarationDocumentResponse:
		return printAppEncryptionDeclarationDocumentTable(v)
	case *BetaRecruitmentCriterionOptionsResponse:
		return printBetaRecruitmentCriterionOptionsTable(v)
	case *BetaRecruitmentCriteriaResponse:
		return printBetaRecruitmentCriteriaTable(v)
	case *Response[BetaGroupMetricAttributes]:
		return printBetaGroupMetricsTable(v.Data)
	case *SandboxTestersResponse:
		return printSandboxTestersTable(v)
	case *SandboxTesterResponse:
		return printSandboxTestersTable(&SandboxTestersResponse{Data: []Resource[SandboxTesterAttributes]{v.Data}})
	case *BundleIDCapabilitiesResponse:
		return printBundleIDCapabilitiesTable(v)
	case *BundleIDCapabilityResponse:
		return printBundleIDCapabilitiesTable(&BundleIDCapabilitiesResponse{Data: []Resource[BundleIDCapabilityAttributes]{v.Data}})
	case *LocalizationDownloadResult:
		return printLocalizationDownloadResultTable(v)
	case *LocalizationUploadResult:
		return printLocalizationUploadResultTable(v)
	case *BuildUploadResult:
		return printBuildUploadResultTable(v)
	case *BuildExpireAllResult:
		return printBuildExpireAllResultTable(v)
	case *AppScreenshotListResult:
		return printAppScreenshotListResultTable(v)
	case *AppPreviewListResult:
		return printAppPreviewListResultTable(v)
	case *AppScreenshotUploadResult:
		return printAppScreenshotUploadResultTable(v)
	case *AppPreviewUploadResult:
		return printAppPreviewUploadResultTable(v)
	case *AppClipAdvancedExperienceImageUploadResult:
		return printAppClipAdvancedExperienceImageUploadResultTable(v)
	case *AppClipHeaderImageUploadResult:
		return printAppClipHeaderImageUploadResultTable(v)
	case *AssetDeleteResult:
		return printAssetDeleteResultTable(v)
	case *AppClipDefaultExperienceDeleteResult:
		return printAppClipDefaultExperienceDeleteResultTable(v)
	case *AppClipDefaultExperienceLocalizationDeleteResult:
		return printAppClipDefaultExperienceLocalizationDeleteResultTable(v)
	case *AppClipAdvancedExperienceDeleteResult:
		return printAppClipAdvancedExperienceDeleteResultTable(v)
	case *AppClipAdvancedExperienceImageDeleteResult:
		return printAppClipAdvancedExperienceImageDeleteResultTable(v)
	case *AppClipHeaderImageDeleteResult:
		return printAppClipHeaderImageDeleteResultTable(v)
	case *BetaAppClipInvocationDeleteResult:
		return printBetaAppClipInvocationDeleteResultTable(v)
	case *BetaAppClipInvocationLocalizationDeleteResult:
		return printBetaAppClipInvocationLocalizationDeleteResultTable(v)
	case *TestFlightPublishResult:
		return printTestFlightPublishResultTable(v)
	case *AppStorePublishResult:
		return printAppStorePublishResultTable(v)
	case *SalesReportResult:
		return printSalesReportResultTable(v)
	case *FinanceReportResult:
		return printFinanceReportResultTable(v)
	case *FinanceRegionsResult:
		return printFinanceRegionsTable(v)
	case *AnalyticsReportRequestResult:
		return printAnalyticsReportRequestResultTable(v)
	case *AnalyticsReportRequestsResponse:
		return printAnalyticsReportRequestsTable(v)
	case *AnalyticsReportRequestResponse:
		return printAnalyticsReportRequestsTable(&AnalyticsReportRequestsResponse{Data: []AnalyticsReportRequestResource{v.Data}, Links: v.Links})
	case *AnalyticsReportDownloadResult:
		return printAnalyticsReportDownloadResultTable(v)
	case *AnalyticsReportGetResult:
		return printAnalyticsReportGetResultTable(v)
	case *AppStoreVersionSubmissionResult:
		return printAppStoreVersionSubmissionTable(v)
	case *AppStoreVersionSubmissionCreateResult:
		return printAppStoreVersionSubmissionCreateTable(v)
	case *AppStoreVersionSubmissionStatusResult:
		return printAppStoreVersionSubmissionStatusTable(v)
	case *AppStoreVersionSubmissionCancelResult:
		return printAppStoreVersionSubmissionCancelTable(v)
	case *AppStoreVersionDetailResult:
		return printAppStoreVersionDetailTable(v)
	case *AppStoreVersionAttachBuildResult:
		return printAppStoreVersionAttachBuildTable(v)
	case *ReviewSubmissionsResponse:
		return printReviewSubmissionsTable(v)
	case *ReviewSubmissionResponse:
		return printReviewSubmissionsTable(&ReviewSubmissionsResponse{Data: []ReviewSubmissionResource{v.Data}, Links: v.Links})
	case *ReviewSubmissionItemsResponse:
		return printReviewSubmissionItemsTable(v)
	case *ReviewSubmissionItemResponse:
		return printReviewSubmissionItemsTable(&ReviewSubmissionItemsResponse{Data: []ReviewSubmissionItemResource{v.Data}, Links: v.Links})
	case *ReviewSubmissionItemDeleteResult:
		return printReviewSubmissionItemDeleteResultTable(v)
	case *AppStoreVersionReleaseRequestResult:
		return printAppStoreVersionReleaseRequestTable(v)
	case *AppStoreVersionPromotionCreateResult:
		return printAppStoreVersionPromotionCreateTable(v)
	case *AppStoreVersionPhasedReleaseResponse:
		return printAppStoreVersionPhasedReleaseTable(v)
	case *AppStoreVersionPhasedReleaseDeleteResult:
		return printAppStoreVersionPhasedReleaseDeleteResultTable(v)
	case *BuildBetaGroupsUpdateResult:
		return printBuildBetaGroupsUpdateTable(v)
	case *InAppPurchaseDeleteResult:
		return printInAppPurchaseDeleteResultTable(v)
	case *AppEventDeleteResult:
		return printAppEventDeleteResultTable(v)
	case *AppEventLocalizationDeleteResult:
		return printAppEventLocalizationDeleteResultTable(v)
	case *AppEventSubmissionResult:
		return printAppEventSubmissionResultTable(v)
	case *GameCenterAchievementsResponse:
		return printGameCenterAchievementsTable(v)
	case *GameCenterAchievementResponse:
		return printGameCenterAchievementsTable(&GameCenterAchievementsResponse{Data: []Resource[GameCenterAchievementAttributes]{v.Data}})
	case *GameCenterAchievementDeleteResult:
		return printGameCenterAchievementDeleteResultTable(v)
	case *GameCenterLeaderboardsResponse:
		return printGameCenterLeaderboardsTable(v)
	case *GameCenterLeaderboardResponse:
		return printGameCenterLeaderboardsTable(&GameCenterLeaderboardsResponse{Data: []Resource[GameCenterLeaderboardAttributes]{v.Data}})
	case *GameCenterLeaderboardDeleteResult:
		return printGameCenterLeaderboardDeleteResultTable(v)
	case *GameCenterLeaderboardSetsResponse:
		return printGameCenterLeaderboardSetsTable(v)
	case *GameCenterLeaderboardSetResponse:
		return printGameCenterLeaderboardSetsTable(&GameCenterLeaderboardSetsResponse{Data: []Resource[GameCenterLeaderboardSetAttributes]{v.Data}})
	case *GameCenterLeaderboardSetDeleteResult:
		return printGameCenterLeaderboardSetDeleteResultTable(v)
	case *GameCenterLeaderboardLocalizationsResponse:
		return printGameCenterLeaderboardLocalizationsTable(v)
	case *GameCenterLeaderboardLocalizationResponse:
		return printGameCenterLeaderboardLocalizationsTable(&GameCenterLeaderboardLocalizationsResponse{Data: []Resource[GameCenterLeaderboardLocalizationAttributes]{v.Data}})
	case *GameCenterLeaderboardLocalizationDeleteResult:
		return printGameCenterLeaderboardLocalizationDeleteResultTable(v)
	case *GameCenterLeaderboardReleasesResponse:
		return printGameCenterLeaderboardReleasesTable(v)
	case *GameCenterLeaderboardReleaseResponse:
		return printGameCenterLeaderboardReleasesTable(&GameCenterLeaderboardReleasesResponse{Data: []Resource[GameCenterLeaderboardReleaseAttributes]{v.Data}})
	case *GameCenterLeaderboardReleaseDeleteResult:
		return printGameCenterLeaderboardReleaseDeleteResultTable(v)
	case *GameCenterLeaderboardSetReleasesResponse:
		return printGameCenterLeaderboardSetReleasesTable(v)
	case *GameCenterLeaderboardSetReleaseResponse:
		return printGameCenterLeaderboardSetReleasesTable(&GameCenterLeaderboardSetReleasesResponse{Data: []Resource[GameCenterLeaderboardSetReleaseAttributes]{v.Data}})
	case *GameCenterLeaderboardSetReleaseDeleteResult:
		return printGameCenterLeaderboardSetReleaseDeleteResultTable(v)
	case *GameCenterLeaderboardSetLocalizationsResponse:
		return printGameCenterLeaderboardSetLocalizationsTable(v)
	case *GameCenterLeaderboardSetLocalizationResponse:
		return printGameCenterLeaderboardSetLocalizationsTable(&GameCenterLeaderboardSetLocalizationsResponse{Data: []Resource[GameCenterLeaderboardSetLocalizationAttributes]{v.Data}})
	case *GameCenterLeaderboardSetLocalizationDeleteResult:
		return printGameCenterLeaderboardSetLocalizationDeleteResultTable(v)
	case *GameCenterAchievementReleasesResponse:
		return printGameCenterAchievementReleasesTable(v)
	case *GameCenterAchievementReleaseResponse:
		return printGameCenterAchievementReleasesTable(&GameCenterAchievementReleasesResponse{Data: []Resource[GameCenterAchievementReleaseAttributes]{v.Data}})
	case *GameCenterAchievementReleaseDeleteResult:
		return printGameCenterAchievementReleaseDeleteResultTable(v)
	case *GameCenterAchievementLocalizationsResponse:
		return printGameCenterAchievementLocalizationsTable(v)
	case *GameCenterAchievementLocalizationResponse:
		return printGameCenterAchievementLocalizationsTable(&GameCenterAchievementLocalizationsResponse{Data: []Resource[GameCenterAchievementLocalizationAttributes]{v.Data}})
	case *GameCenterAchievementLocalizationDeleteResult:
		return printGameCenterAchievementLocalizationDeleteResultTable(v)
	case *GameCenterLeaderboardImageUploadResult:
		return printGameCenterLeaderboardImageUploadResultTable(v)
	case *GameCenterLeaderboardImageDeleteResult:
		return printGameCenterLeaderboardImageDeleteResultTable(v)
	case *GameCenterAchievementImageUploadResult:
		return printGameCenterAchievementImageUploadResultTable(v)
	case *GameCenterAchievementImageDeleteResult:
		return printGameCenterAchievementImageDeleteResultTable(v)
	case *GameCenterLeaderboardSetImageUploadResult:
		return printGameCenterLeaderboardSetImageUploadResultTable(v)
	case *GameCenterLeaderboardSetImageDeleteResult:
		return printGameCenterLeaderboardSetImageDeleteResultTable(v)
	case *SubscriptionGroupDeleteResult:
		return printSubscriptionGroupDeleteResultTable(v)
	case *SubscriptionDeleteResult:
		return printSubscriptionDeleteResultTable(v)
	case *BetaTesterDeleteResult:
		return printBetaTesterDeleteResultTable(v)
	case *BetaTesterGroupsUpdateResult:
		return printBetaTesterGroupsUpdateResultTable(v)
	case *AppStoreVersionLocalizationDeleteResult:
		return printAppStoreVersionLocalizationDeleteResultTable(v)
	case *BetaBuildLocalizationDeleteResult:
		return printBetaBuildLocalizationDeleteResultTable(v)
	case *BetaTesterInvitationResult:
		return printBetaTesterInvitationResultTable(v)
	case *PromotedPurchaseDeleteResult:
		return printPromotedPurchaseDeleteResultTable(v)
	case *AppPromotedPurchasesLinkResult:
		return printAppPromotedPurchasesLinkResultTable(v)
	case *SandboxTesterClearHistoryResult:
		return printSandboxTesterClearHistoryResultTable(v)
	case *BundleIDDeleteResult:
		return printBundleIDDeleteResultTable(v)
	case *MarketplaceSearchDetailDeleteResult:
		return printMarketplaceSearchDetailDeleteResultTable(v)
	case *MarketplaceWebhookDeleteResult:
		return printMarketplaceWebhookDeleteResultTable(v)
	case *WebhookDeleteResult:
		return printWebhookDeleteResultTable(v)
	case *WebhookPingResponse:
		return printWebhookPingTable(v)
	case *MerchantIDDeleteResult:
		return printMerchantIDDeleteResultTable(v)
	case *PassTypeIDDeleteResult:
		return printPassTypeIDDeleteResultTable(v)
	case *BundleIDCapabilityDeleteResult:
		return printBundleIDCapabilityDeleteResultTable(v)
	case *CertificateRevokeResult:
		return printCertificateRevokeResultTable(v)
	case *ProfileDeleteResult:
		return printProfileDeleteResultTable(v)
	case *EndUserLicenseAgreementResponse:
		return printEndUserLicenseAgreementTable(v)
	case *EndUserLicenseAgreementDeleteResult:
		return printEndUserLicenseAgreementDeleteResultTable(v)
	case *ProfileDownloadResult:
		return printProfileDownloadResultTable(v)
	case *SigningFetchResult:
		return printSigningFetchResultTable(v)
	case *XcodeCloudRunResult:
		return printXcodeCloudRunResultTable(v)
	case *XcodeCloudStatusResult:
		return printXcodeCloudStatusResultTable(v)
	case *CiProductsResponse:
		return printCiProductsTable(v)
	case *CiProductResponse:
		return printCiProductsTable(&CiProductsResponse{Data: []CiProductResource{v.Data}})
	case *CiWorkflowsResponse:
		return printCiWorkflowsTable(v)
	case *CiWorkflowResponse:
		return printCiWorkflowsTable(&CiWorkflowsResponse{Data: []CiWorkflowResource{v.Data}})
	case *ScmRepositoriesResponse:
		return printScmRepositoriesTable(v)
	case *CiBuildRunsResponse:
		return printCiBuildRunsTable(v)
	case *CiBuildRunResponse:
		return printCiBuildRunsTable(&CiBuildRunsResponse{Data: []CiBuildRunResource{v.Data}})
	case *CiBuildActionsResponse:
		return printCiBuildActionsTable(v)
	case *CiBuildActionResponse:
		return printCiBuildActionsTable(&CiBuildActionsResponse{Data: []CiBuildActionResource{v.Data}})
	case *CiMacOsVersionsResponse:
		return printCiMacOsVersionsTable(v)
	case *CiMacOsVersionResponse:
		return printCiMacOsVersionsTable(&CiMacOsVersionsResponse{Data: []CiMacOsVersionResource{v.Data}})
	case *CiXcodeVersionsResponse:
		return printCiXcodeVersionsTable(v)
	case *CiXcodeVersionResponse:
		return printCiXcodeVersionsTable(&CiXcodeVersionsResponse{Data: []CiXcodeVersionResource{v.Data}})
	case *CiArtifactsResponse:
		return printCiArtifactsTable(v)
	case *CiArtifactResponse:
		return printCiArtifactTable(v)
	case *CiTestResultsResponse:
		return printCiTestResultsTable(v)
	case *CiTestResultResponse:
		return printCiTestResultTable(v)
	case *CiIssuesResponse:
		return printCiIssuesTable(v)
	case *CiIssueResponse:
		return printCiIssueTable(v)
	case *CiArtifactDownloadResult:
		return printCiArtifactDownloadResultTable(v)
	case *CiWorkflowDeleteResult:
		return printCiWorkflowDeleteResultTable(v)
	case *CiProductDeleteResult:
		return printCiProductDeleteResultTable(v)
	case *CustomerReviewResponseResponse:
		return printCustomerReviewResponseTable(v)
	case *CustomerReviewResponseDeleteResult:
		return printCustomerReviewResponseDeleteResultTable(v)
	case *AccessibilityDeclarationDeleteResult:
		return printAccessibilityDeclarationDeleteResultTable(v)
	case *AppStoreReviewAttachmentDeleteResult:
		return printAppStoreReviewAttachmentDeleteResultTable(v)
	case *RoutingAppCoverageDeleteResult:
		return printRoutingAppCoverageDeleteResultTable(v)
	case *NominationDeleteResult:
		return printNominationDeleteResultTable(v)
	case *AppEncryptionDeclarationBuildsUpdateResult:
		return printAppEncryptionDeclarationBuildsUpdateResultTable(v)
	case *AndroidToIosAppMappingDetailsResponse:
		return printAndroidToIosAppMappingDetailsTable(v)
	case *AndroidToIosAppMappingDetailResponse:
		return printAndroidToIosAppMappingDetailsTable(&AndroidToIosAppMappingDetailsResponse{Data: []Resource[AndroidToIosAppMappingDetailAttributes]{v.Data}})
	case *AndroidToIosAppMappingDeleteResult:
		return printAndroidToIosAppMappingDeleteResultTable(v)
	case *AlternativeDistributionDomainDeleteResult:
		return printAlternativeDistributionDomainDeleteResultTable(v)
	case *AlternativeDistributionKeyDeleteResult:
		return printAlternativeDistributionKeyDeleteResultTable(v)
	case *AppCustomProductPagesResponse:
		return printAppCustomProductPagesTable(v)
	case *AppCustomProductPageResponse:
		return printAppCustomProductPagesTable(&AppCustomProductPagesResponse{Data: []Resource[AppCustomProductPageAttributes]{v.Data}})
	case *AppCustomProductPageVersionsResponse:
		return printAppCustomProductPageVersionsTable(v)
	case *AppCustomProductPageVersionResponse:
		return printAppCustomProductPageVersionsTable(&AppCustomProductPageVersionsResponse{Data: []Resource[AppCustomProductPageVersionAttributes]{v.Data}})
	case *AppCustomProductPageLocalizationsResponse:
		return printAppCustomProductPageLocalizationsTable(v)
	case *AppCustomProductPageLocalizationResponse:
		return printAppCustomProductPageLocalizationsTable(&AppCustomProductPageLocalizationsResponse{Data: []Resource[AppCustomProductPageLocalizationAttributes]{v.Data}})
	case *AppStoreVersionExperimentsResponse:
		return printAppStoreVersionExperimentsTable(v)
	case *AppStoreVersionExperimentResponse:
		return printAppStoreVersionExperimentsTable(&AppStoreVersionExperimentsResponse{Data: []Resource[AppStoreVersionExperimentAttributes]{v.Data}})
	case *AppStoreVersionExperimentsV2Response:
		return printAppStoreVersionExperimentsV2Table(v)
	case *AppStoreVersionExperimentV2Response:
		return printAppStoreVersionExperimentsV2Table(&AppStoreVersionExperimentsV2Response{Data: []Resource[AppStoreVersionExperimentV2Attributes]{v.Data}})
	case *AppStoreVersionExperimentTreatmentsResponse:
		return printAppStoreVersionExperimentTreatmentsTable(v)
	case *AppStoreVersionExperimentTreatmentResponse:
		return printAppStoreVersionExperimentTreatmentsTable(&AppStoreVersionExperimentTreatmentsResponse{Data: []Resource[AppStoreVersionExperimentTreatmentAttributes]{v.Data}})
	case *AppStoreVersionExperimentTreatmentLocalizationsResponse:
		return printAppStoreVersionExperimentTreatmentLocalizationsTable(v)
	case *AppStoreVersionExperimentTreatmentLocalizationResponse:
		return printAppStoreVersionExperimentTreatmentLocalizationsTable(&AppStoreVersionExperimentTreatmentLocalizationsResponse{Data: []Resource[AppStoreVersionExperimentTreatmentLocalizationAttributes]{v.Data}})
	case *AppCustomProductPageDeleteResult:
		return printAppCustomProductPageDeleteResultTable(v)
	case *AppCustomProductPageLocalizationDeleteResult:
		return printAppCustomProductPageLocalizationDeleteResultTable(v)
	case *AppStoreVersionExperimentDeleteResult:
		return printAppStoreVersionExperimentDeleteResultTable(v)
	case *AppStoreVersionExperimentTreatmentDeleteResult:
		return printAppStoreVersionExperimentTreatmentDeleteResultTable(v)
	case *AppStoreVersionExperimentTreatmentLocalizationDeleteResult:
		return printAppStoreVersionExperimentTreatmentLocalizationDeleteResultTable(v)
	case *PerfPowerMetricsResponse:
		return printPerfPowerMetricsTable(v)
	case *DiagnosticSignaturesResponse:
		return printDiagnosticSignaturesTable(v)
	case *DiagnosticLogsResponse:
		return printDiagnosticLogsTable(v)
	case *PerformanceDownloadResult:
		return printPerformanceDownloadResultTable(v)
	default:
		return PrintJSON(data)
	}
}
