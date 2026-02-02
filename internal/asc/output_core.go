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
	case *BetaBuildUsagesResponse:
		return printPrettyRawJSON(v.Data)
	case *BetaTesterUsagesResponse:
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
	case *CustomerReviewSummarizationsResponse:
		return printCustomerReviewSummarizationsMarkdown(v)
	case *CustomerReviewResponse:
		return printReviewsMarkdown(&ReviewsResponse{Data: []Resource[ReviewAttributes]{v.Data}})
	case *AppsResponse:
		return printAppsMarkdown(v)
	case *AppClipsResponse:
		return printAppClipsMarkdown(v)
	case *AppCategoriesResponse:
		return printAppCategoriesMarkdown(v)
	case *AppCategoryResponse:
		return printAppCategoriesMarkdown(&AppCategoriesResponse{Data: []AppCategory{v.Data}})
	case *AppInfosResponse:
		return printAppInfosMarkdown(v)
	case *AppInfoResponse:
		return printAppInfosMarkdown(&AppInfosResponse{Data: []Resource[AppInfoAttributes]{v.Data}})
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
	case *AppClipHeaderImageResponse:
		return printAppClipHeaderImageMarkdown(v)
	case *AppClipAdvancedExperienceImageResponse:
		return printAppClipAdvancedExperienceImageMarkdown(v)
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
	case *BackgroundAssetVersionAppStoreReleaseResponse:
		return printBackgroundAssetVersionAppStoreReleaseMarkdown(v)
	case *BackgroundAssetVersionExternalBetaReleaseResponse:
		return printBackgroundAssetVersionExternalBetaReleaseMarkdown(v)
	case *BackgroundAssetVersionInternalBetaReleaseResponse:
		return printBackgroundAssetVersionInternalBetaReleaseMarkdown(v)
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
	case *AppStoreVersionAgeRatingDeclarationLinkageResponse:
		return printLinkagesMarkdown(&LinkagesResponse{Data: []ResourceData{v.Data}})
	case *AppStoreVersionReviewDetailLinkageResponse:
		return printLinkagesMarkdown(&LinkagesResponse{Data: []ResourceData{v.Data}})
	case *AppStoreVersionAppClipDefaultExperienceLinkageResponse:
		return printLinkagesMarkdown(&LinkagesResponse{Data: []ResourceData{v.Data}})
	case *AppStoreVersionSubmissionLinkageResponse:
		return printLinkagesMarkdown(&LinkagesResponse{Data: []ResourceData{v.Data}})
	case *AppStoreVersionRoutingAppCoverageLinkageResponse:
		return printLinkagesMarkdown(&LinkagesResponse{Data: []ResourceData{v.Data}})
	case *AppStoreVersionAlternativeDistributionPackageLinkageResponse:
		return printLinkagesMarkdown(&LinkagesResponse{Data: []ResourceData{v.Data}})
	case *AppStoreVersionGameCenterAppVersionLinkageResponse:
		return printLinkagesMarkdown(&LinkagesResponse{Data: []ResourceData{v.Data}})
	case *BuildAppLinkageResponse:
		return printLinkagesMarkdown(&LinkagesResponse{Data: []ResourceData{v.Data}})
	case *BuildAppStoreVersionLinkageResponse:
		return printLinkagesMarkdown(&LinkagesResponse{Data: []ResourceData{v.Data}})
	case *BuildBuildBetaDetailLinkageResponse:
		return printLinkagesMarkdown(&LinkagesResponse{Data: []ResourceData{v.Data}})
	case *BuildPreReleaseVersionLinkageResponse:
		return printLinkagesMarkdown(&LinkagesResponse{Data: []ResourceData{v.Data}})
	case *PreReleaseVersionAppLinkageResponse:
		return printLinkagesMarkdown(&LinkagesResponse{Data: []ResourceData{v.Data}})
	case *AppInfoAgeRatingDeclarationLinkageResponse:
		return printLinkagesMarkdown(&LinkagesResponse{Data: []ResourceData{v.Data}})
	case *AppInfoPrimaryCategoryLinkageResponse:
		return printLinkagesMarkdown(&LinkagesResponse{Data: []ResourceData{v.Data}})
	case *AppInfoPrimarySubcategoryOneLinkageResponse:
		return printLinkagesMarkdown(&LinkagesResponse{Data: []ResourceData{v.Data}})
	case *AppInfoPrimarySubcategoryTwoLinkageResponse:
		return printLinkagesMarkdown(&LinkagesResponse{Data: []ResourceData{v.Data}})
	case *AppInfoSecondaryCategoryLinkageResponse:
		return printLinkagesMarkdown(&LinkagesResponse{Data: []ResourceData{v.Data}})
	case *AppInfoSecondarySubcategoryOneLinkageResponse:
		return printLinkagesMarkdown(&LinkagesResponse{Data: []ResourceData{v.Data}})
	case *AppInfoSecondarySubcategoryTwoLinkageResponse:
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
	case *InAppPurchasesResponse:
		return printLegacyInAppPurchasesMarkdown(v)
	case *InAppPurchaseResponse:
		return printLegacyInAppPurchasesMarkdown(&InAppPurchasesResponse{Data: []Resource[InAppPurchaseAttributes]{v.Data}})
	case *InAppPurchasesV2Response:
		return printInAppPurchasesMarkdown(v)
	case *InAppPurchaseV2Response:
		return printInAppPurchasesMarkdown(&InAppPurchasesV2Response{Data: []Resource[InAppPurchaseV2Attributes]{v.Data}})
	case *InAppPurchaseLocalizationsResponse:
		return printInAppPurchaseLocalizationsMarkdown(v)
	case *InAppPurchaseLocalizationResponse:
		return printInAppPurchaseLocalizationsMarkdown(&InAppPurchaseLocalizationsResponse{Data: []Resource[InAppPurchaseLocalizationAttributes]{v.Data}})
	case *InAppPurchaseImagesResponse:
		return printInAppPurchaseImagesMarkdown(v)
	case *InAppPurchaseImageResponse:
		return printInAppPurchaseImagesMarkdown(&InAppPurchaseImagesResponse{Data: []Resource[InAppPurchaseImageAttributes]{v.Data}})
	case *InAppPurchasePricePointsResponse:
		return printInAppPurchasePricePointsMarkdown(v)
	case *InAppPurchasePricesResponse:
		return printInAppPurchasePricesMarkdown(v)
	case *InAppPurchaseOfferPricesResponse:
		return printInAppPurchaseOfferCodePricesMarkdown(v)
	case *InAppPurchaseOfferCodesResponse:
		return printInAppPurchaseOfferCodesMarkdown(v)
	case *InAppPurchaseOfferCodeResponse:
		return printInAppPurchaseOfferCodesMarkdown(&InAppPurchaseOfferCodesResponse{Data: []Resource[InAppPurchaseOfferCodeAttributes]{v.Data}})
	case *InAppPurchaseOfferCodeCustomCodesResponse:
		return printInAppPurchaseOfferCodeCustomCodesMarkdown(v)
	case *InAppPurchaseOfferCodeCustomCodeResponse:
		return printInAppPurchaseOfferCodeCustomCodesMarkdown(&InAppPurchaseOfferCodeCustomCodesResponse{Data: []Resource[InAppPurchaseOfferCodeCustomCodeAttributes]{v.Data}})
	case *InAppPurchaseOfferCodeOneTimeUseCodesResponse:
		return printInAppPurchaseOfferCodeOneTimeUseCodesMarkdown(v)
	case *InAppPurchaseOfferCodeOneTimeUseCodeResponse:
		return printInAppPurchaseOfferCodeOneTimeUseCodesMarkdown(&InAppPurchaseOfferCodeOneTimeUseCodesResponse{Data: []Resource[InAppPurchaseOfferCodeOneTimeUseCodeAttributes]{v.Data}})
	case *InAppPurchaseAvailabilityResponse:
		return printInAppPurchaseAvailabilityMarkdown(v)
	case *InAppPurchaseContentResponse:
		return printInAppPurchaseContentMarkdown(v)
	case *InAppPurchasePriceScheduleResponse:
		return printInAppPurchasePriceScheduleMarkdown(v)
	case *InAppPurchaseAppStoreReviewScreenshotResponse:
		return printInAppPurchaseReviewScreenshotMarkdown(v)
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
	case *SubscriptionPricesResponse:
		return printSubscriptionPricesMarkdown(v)
	case *SubscriptionPriceResponse:
		return printSubscriptionPriceMarkdown(v)
	case *SubscriptionAvailabilityResponse:
		return printSubscriptionAvailabilityMarkdown(v)
	case *SubscriptionGracePeriodResponse:
		return printSubscriptionGracePeriodMarkdown(v)
	case *TerritoriesResponse:
		return printTerritoriesMarkdown(v)
	case *TerritoryResponse:
		return printTerritoriesMarkdown(&TerritoriesResponse{Data: []Resource[TerritoryAttributes]{v.Data}})
	case *TerritoryAgeRatingsResponse:
		return printTerritoryAgeRatingsMarkdown(v)
	case *OfferCodeValuesResult:
		return printOfferCodeValuesMarkdown(v)
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
	case *SubscriptionOfferCodeCustomCodesResponse:
		return printOfferCodeCustomCodesMarkdown(v)
	case *SubscriptionOfferCodeResponse:
		return printSubscriptionOfferCodeMarkdown(v)
	case *WinBackOffersResponse:
		return printWinBackOffersMarkdown(v)
	case *WinBackOfferResponse:
		return printWinBackOffersMarkdown(&WinBackOffersResponse{Data: []Resource[WinBackOfferAttributes]{v.Data}})
	case *WinBackOfferPricesResponse:
		return printWinBackOfferPricesMarkdown(v)
	case *AppStoreVersionsResponse:
		return printAppStoreVersionsMarkdown(v)
	case *AppStoreVersionResponse:
		return printAppStoreVersionsMarkdown(&AppStoreVersionsResponse{Data: []Resource[AppStoreVersionAttributes]{v.Data}})
	case *PreReleaseVersionsResponse:
		return printPreReleaseVersionsMarkdown(v)
	case *BuildResponse:
		return printBuildsMarkdown(&BuildsResponse{Data: []Resource[BuildAttributes]{v.Data}})
	case *BuildIconsResponse:
		return printBuildIconsMarkdown(v)
	case *BuildUploadsResponse:
		return printBuildUploadsMarkdown(v)
	case *BuildUploadResponse:
		return printBuildUploadsMarkdown(&BuildUploadsResponse{Data: []Resource[BuildUploadAttributes]{v.Data}})
	case *BuildUploadFilesResponse:
		return printBuildUploadFilesMarkdown(v)
	case *BuildUploadFileResponse:
		return printBuildUploadFilesMarkdown(&BuildUploadFilesResponse{Data: []Resource[BuildUploadFileAttributes]{v.Data}})
	case *AppClipDomainStatusResult:
		return printAppClipDomainStatusResultMarkdown(v)
	case *SubscriptionOfferCodeOneTimeUseCodeResponse:
		return printOfferCodesMarkdown(&SubscriptionOfferCodeOneTimeUseCodesResponse{Data: []Resource[SubscriptionOfferCodeOneTimeUseCodeAttributes]{v.Data}})
	case *SubscriptionOfferCodeCustomCodeResponse:
		return printOfferCodeCustomCodesMarkdown(&SubscriptionOfferCodeCustomCodesResponse{Data: []Resource[SubscriptionOfferCodeCustomCodeAttributes]{v.Data}})
	case *WinBackOfferDeleteResult:
		return printWinBackOfferDeleteResultMarkdown(v)
	case *SubscriptionPriceDeleteResult:
		return printSubscriptionPriceDeleteResultMarkdown(v)
	case *SubscriptionOfferCodePricesResponse:
		return printOfferCodePricesMarkdown(v)
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
	case *BetaAppLocalizationsResponse:
		return printBetaAppLocalizationsMarkdown(v)
	case *BetaAppLocalizationResponse:
		return printBetaAppLocalizationsMarkdown(&BetaAppLocalizationsResponse{Data: []Resource[BetaAppLocalizationAttributes]{v.Data}})
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
	case *BetaLicenseAgreementsResponse:
		return printBetaLicenseAgreementsMarkdown(v)
	case *BetaLicenseAgreementResponse:
		return printBetaLicenseAgreementsMarkdown(&BetaLicenseAgreementsResponse{Data: []BetaLicenseAgreementResource{v.Data}})
	case *BuildBetaNotificationResponse:
		return printBuildBetaNotificationMarkdown(v)
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
	case *AnalyticsReportsResponse:
		return printAnalyticsReportsMarkdown(v)
	case *AnalyticsReportResponse:
		return printAnalyticsReportsMarkdown(&AnalyticsReportsResponse{Data: []Resource[AnalyticsReportAttributes]{v.Data}, Links: v.Links})
	case *AnalyticsReportInstancesResponse:
		return printAnalyticsReportInstancesMarkdown(v)
	case *AnalyticsReportInstanceResponse:
		return printAnalyticsReportInstancesMarkdown(&AnalyticsReportInstancesResponse{Data: []Resource[AnalyticsReportInstanceAttributes]{v.Data}, Links: v.Links})
	case *AnalyticsReportSegmentsResponse:
		return printAnalyticsReportSegmentsMarkdown(v)
	case *AnalyticsReportSegmentResponse:
		return printAnalyticsReportSegmentsMarkdown(&AnalyticsReportSegmentsResponse{Data: []Resource[AnalyticsReportSegmentAttributes]{v.Data}, Links: v.Links})
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
	case *BuildIndividualTestersUpdateResult:
		return printBuildIndividualTestersUpdateMarkdown(v)
	case *BuildUploadDeleteResult:
		return printBuildUploadDeleteResultMarkdown(v)
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
	case *GameCenterAchievementVersionsResponse:
		return printGameCenterAchievementVersionsMarkdown(v)
	case *GameCenterAchievementVersionResponse:
		return printGameCenterAchievementVersionsMarkdown(&GameCenterAchievementVersionsResponse{Data: []Resource[GameCenterAchievementVersionAttributes]{v.Data}})
	case *GameCenterLeaderboardsResponse:
		return printGameCenterLeaderboardsMarkdown(v)
	case *GameCenterLeaderboardResponse:
		return printGameCenterLeaderboardsMarkdown(&GameCenterLeaderboardsResponse{Data: []Resource[GameCenterLeaderboardAttributes]{v.Data}})
	case *GameCenterLeaderboardDeleteResult:
		return printGameCenterLeaderboardDeleteResultMarkdown(v)
	case *GameCenterLeaderboardVersionsResponse:
		return printGameCenterLeaderboardVersionsMarkdown(v)
	case *GameCenterLeaderboardVersionResponse:
		return printGameCenterLeaderboardVersionsMarkdown(&GameCenterLeaderboardVersionsResponse{Data: []Resource[GameCenterLeaderboardVersionAttributes]{v.Data}})
	case *GameCenterLeaderboardSetsResponse:
		return printGameCenterLeaderboardSetsMarkdown(v)
	case *GameCenterLeaderboardSetResponse:
		return printGameCenterLeaderboardSetsMarkdown(&GameCenterLeaderboardSetsResponse{Data: []Resource[GameCenterLeaderboardSetAttributes]{v.Data}})
	case *GameCenterLeaderboardSetDeleteResult:
		return printGameCenterLeaderboardSetDeleteResultMarkdown(v)
	case *GameCenterLeaderboardSetVersionsResponse:
		return printGameCenterLeaderboardSetVersionsMarkdown(v)
	case *GameCenterLeaderboardSetVersionResponse:
		return printGameCenterLeaderboardSetVersionsMarkdown(&GameCenterLeaderboardSetVersionsResponse{Data: []Resource[GameCenterLeaderboardSetVersionAttributes]{v.Data}})
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
	case *GameCenterLeaderboardEntrySubmissionResponse:
		return printGameCenterLeaderboardEntrySubmissionMarkdown(v)
	case *GameCenterPlayerAchievementSubmissionResponse:
		return printGameCenterPlayerAchievementSubmissionMarkdown(v)
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
	case *GameCenterChallengesResponse:
		return printGameCenterChallengesMarkdown(v)
	case *GameCenterChallengeResponse:
		return printGameCenterChallengesMarkdown(&GameCenterChallengesResponse{Data: []Resource[GameCenterChallengeAttributes]{v.Data}})
	case *GameCenterChallengeDeleteResult:
		return printGameCenterChallengeDeleteResultMarkdown(v)
	case *GameCenterChallengeVersionsResponse:
		return printGameCenterChallengeVersionsMarkdown(v)
	case *GameCenterChallengeVersionResponse:
		return printGameCenterChallengeVersionsMarkdown(&GameCenterChallengeVersionsResponse{Data: []Resource[GameCenterChallengeVersionAttributes]{v.Data}})
	case *GameCenterChallengeLocalizationsResponse:
		return printGameCenterChallengeLocalizationsMarkdown(v)
	case *GameCenterChallengeLocalizationResponse:
		return printGameCenterChallengeLocalizationsMarkdown(&GameCenterChallengeLocalizationsResponse{Data: []Resource[GameCenterChallengeLocalizationAttributes]{v.Data}})
	case *GameCenterChallengeLocalizationDeleteResult:
		return printGameCenterChallengeLocalizationDeleteResultMarkdown(v)
	case *GameCenterChallengeImagesResponse:
		return printGameCenterChallengeImagesMarkdown(v)
	case *GameCenterChallengeImageResponse:
		return printGameCenterChallengeImagesMarkdown(&GameCenterChallengeImagesResponse{Data: []Resource[GameCenterChallengeImageAttributes]{v.Data}})
	case *GameCenterChallengeImageUploadResult:
		return printGameCenterChallengeImageUploadResultMarkdown(v)
	case *GameCenterChallengeImageDeleteResult:
		return printGameCenterChallengeImageDeleteResultMarkdown(v)
	case *GameCenterChallengeVersionReleasesResponse:
		return printGameCenterChallengeReleasesMarkdown(v)
	case *GameCenterChallengeVersionReleaseResponse:
		return printGameCenterChallengeReleasesMarkdown(&GameCenterChallengeVersionReleasesResponse{Data: []Resource[GameCenterChallengeVersionReleaseAttributes]{v.Data}})
	case *GameCenterChallengeVersionReleaseDeleteResult:
		return printGameCenterChallengeReleaseDeleteResultMarkdown(v)
	case *GameCenterActivitiesResponse:
		return printGameCenterActivitiesMarkdown(v)
	case *GameCenterActivityResponse:
		return printGameCenterActivitiesMarkdown(&GameCenterActivitiesResponse{Data: []Resource[GameCenterActivityAttributes]{v.Data}})
	case *GameCenterActivityDeleteResult:
		return printGameCenterActivityDeleteResultMarkdown(v)
	case *GameCenterActivityVersionsResponse:
		return printGameCenterActivityVersionsMarkdown(v)
	case *GameCenterActivityVersionResponse:
		return printGameCenterActivityVersionsMarkdown(&GameCenterActivityVersionsResponse{Data: []Resource[GameCenterActivityVersionAttributes]{v.Data}})
	case *GameCenterActivityLocalizationsResponse:
		return printGameCenterActivityLocalizationsMarkdown(v)
	case *GameCenterActivityLocalizationResponse:
		return printGameCenterActivityLocalizationsMarkdown(&GameCenterActivityLocalizationsResponse{Data: []Resource[GameCenterActivityLocalizationAttributes]{v.Data}})
	case *GameCenterActivityLocalizationDeleteResult:
		return printGameCenterActivityLocalizationDeleteResultMarkdown(v)
	case *GameCenterActivityImagesResponse:
		return printGameCenterActivityImagesMarkdown(v)
	case *GameCenterActivityImageResponse:
		return printGameCenterActivityImagesMarkdown(&GameCenterActivityImagesResponse{Data: []Resource[GameCenterActivityImageAttributes]{v.Data}})
	case *GameCenterActivityImageUploadResult:
		return printGameCenterActivityImageUploadResultMarkdown(v)
	case *GameCenterActivityImageDeleteResult:
		return printGameCenterActivityImageDeleteResultMarkdown(v)
	case *GameCenterActivityVersionReleasesResponse:
		return printGameCenterActivityReleasesMarkdown(v)
	case *GameCenterActivityVersionReleaseResponse:
		return printGameCenterActivityReleasesMarkdown(&GameCenterActivityVersionReleasesResponse{Data: []Resource[GameCenterActivityVersionReleaseAttributes]{v.Data}})
	case *GameCenterActivityVersionReleaseDeleteResult:
		return printGameCenterActivityReleaseDeleteResultMarkdown(v)
	case *GameCenterGroupsResponse:
		return printGameCenterGroupsMarkdown(v)
	case *GameCenterGroupResponse:
		return printGameCenterGroupsMarkdown(&GameCenterGroupsResponse{Data: []Resource[GameCenterGroupAttributes]{v.Data}})
	case *GameCenterGroupDeleteResult:
		return printGameCenterGroupDeleteResultMarkdown(v)
	case *GameCenterAppVersionsResponse:
		return printGameCenterAppVersionsMarkdown(v)
	case *GameCenterAppVersionResponse:
		return printGameCenterAppVersionsMarkdown(&GameCenterAppVersionsResponse{Data: []Resource[GameCenterAppVersionAttributes]{v.Data}})
	case *GameCenterEnabledVersionsResponse:
		return printGameCenterEnabledVersionsMarkdown(v)
	case *GameCenterDetailsResponse:
		return printGameCenterDetailsMarkdown(v)
	case *GameCenterDetailResponse:
		return printGameCenterDetailsMarkdown(&GameCenterDetailsResponse{Data: []Resource[GameCenterDetailAttributes]{v.Data}})
	case *GameCenterMatchmakingQueuesResponse:
		return printGameCenterMatchmakingQueuesMarkdown(v)
	case *GameCenterMatchmakingQueueResponse:
		return printGameCenterMatchmakingQueuesMarkdown(&GameCenterMatchmakingQueuesResponse{Data: []Resource[GameCenterMatchmakingQueueAttributes]{v.Data}})
	case *GameCenterMatchmakingQueueDeleteResult:
		return printGameCenterMatchmakingQueueDeleteResultMarkdown(v)
	case *GameCenterMatchmakingRuleSetsResponse:
		return printGameCenterMatchmakingRuleSetsMarkdown(v)
	case *GameCenterMatchmakingRuleSetResponse:
		return printGameCenterMatchmakingRuleSetsMarkdown(&GameCenterMatchmakingRuleSetsResponse{Data: []Resource[GameCenterMatchmakingRuleSetAttributes]{v.Data}})
	case *GameCenterMatchmakingRuleSetDeleteResult:
		return printGameCenterMatchmakingRuleSetDeleteResultMarkdown(v)
	case *GameCenterMatchmakingRulesResponse:
		return printGameCenterMatchmakingRulesMarkdown(v)
	case *GameCenterMatchmakingRuleResponse:
		return printGameCenterMatchmakingRulesMarkdown(&GameCenterMatchmakingRulesResponse{Data: []Resource[GameCenterMatchmakingRuleAttributes]{v.Data}})
	case *GameCenterMatchmakingRuleDeleteResult:
		return printGameCenterMatchmakingRuleDeleteResultMarkdown(v)
	case *GameCenterMatchmakingTeamsResponse:
		return printGameCenterMatchmakingTeamsMarkdown(v)
	case *GameCenterMatchmakingTeamResponse:
		return printGameCenterMatchmakingTeamsMarkdown(&GameCenterMatchmakingTeamsResponse{Data: []Resource[GameCenterMatchmakingTeamAttributes]{v.Data}})
	case *GameCenterMatchmakingTeamDeleteResult:
		return printGameCenterMatchmakingTeamDeleteResultMarkdown(v)
	case *GameCenterMetricsResponse:
		return printGameCenterMetricsMarkdown(v)
	case *GameCenterMatchmakingRuleSetTestResponse:
		return printGameCenterMatchmakingRuleSetTestMarkdown(v)
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
	case *BetaAppLocalizationDeleteResult:
		return printBetaAppLocalizationDeleteResultMarkdown(v)
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
	case *ScmProvidersResponse:
		return printScmProvidersMarkdown(v)
	case *ScmProviderResponse:
		return printScmProvidersMarkdown(&ScmProvidersResponse{Data: []ScmProviderResource{v.Data}, Links: v.Links})
	case *ScmRepositoriesResponse:
		return printScmRepositoriesMarkdown(v)
	case *ScmGitReferencesResponse:
		return printScmGitReferencesMarkdown(v)
	case *ScmGitReferenceResponse:
		return printScmGitReferencesMarkdown(&ScmGitReferencesResponse{Data: []ScmGitReferenceResource{v.Data}, Links: v.Links})
	case *ScmPullRequestsResponse:
		return printScmPullRequestsMarkdown(v)
	case *ScmPullRequestResponse:
		return printScmPullRequestsMarkdown(&ScmPullRequestsResponse{Data: []ScmPullRequestResource{v.Data}, Links: v.Links})
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
	case *AppKeywordsResponse:
		return printAppKeywordsMarkdown(v)
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
	case *CustomerReviewSummarizationsResponse:
		return printCustomerReviewSummarizationsTable(v)
	case *CustomerReviewResponse:
		return printReviewsTable(&ReviewsResponse{Data: []Resource[ReviewAttributes]{v.Data}})
	case *AppsResponse:
		return printAppsTable(v)
	case *AppClipsResponse:
		return printAppClipsTable(v)
	case *AppCategoriesResponse:
		return printAppCategoriesTable(v)
	case *AppCategoryResponse:
		return printAppCategoriesTable(&AppCategoriesResponse{Data: []AppCategory{v.Data}})
	case *AppInfosResponse:
		return printAppInfosTable(v)
	case *AppInfoResponse:
		return printAppInfosTable(&AppInfosResponse{Data: []Resource[AppInfoAttributes]{v.Data}})
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
	case *AppClipHeaderImageResponse:
		return printAppClipHeaderImageTable(v)
	case *AppClipAdvancedExperienceImageResponse:
		return printAppClipAdvancedExperienceImageTable(v)
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
	case *BackgroundAssetVersionAppStoreReleaseResponse:
		return printBackgroundAssetVersionAppStoreReleaseTable(v)
	case *BackgroundAssetVersionExternalBetaReleaseResponse:
		return printBackgroundAssetVersionExternalBetaReleaseTable(v)
	case *BackgroundAssetVersionInternalBetaReleaseResponse:
		return printBackgroundAssetVersionInternalBetaReleaseTable(v)
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
	case *AppStoreVersionAgeRatingDeclarationLinkageResponse:
		return printLinkagesTable(&LinkagesResponse{Data: []ResourceData{v.Data}})
	case *AppStoreVersionReviewDetailLinkageResponse:
		return printLinkagesTable(&LinkagesResponse{Data: []ResourceData{v.Data}})
	case *AppStoreVersionAppClipDefaultExperienceLinkageResponse:
		return printLinkagesTable(&LinkagesResponse{Data: []ResourceData{v.Data}})
	case *AppStoreVersionSubmissionLinkageResponse:
		return printLinkagesTable(&LinkagesResponse{Data: []ResourceData{v.Data}})
	case *AppStoreVersionRoutingAppCoverageLinkageResponse:
		return printLinkagesTable(&LinkagesResponse{Data: []ResourceData{v.Data}})
	case *AppStoreVersionAlternativeDistributionPackageLinkageResponse:
		return printLinkagesTable(&LinkagesResponse{Data: []ResourceData{v.Data}})
	case *AppStoreVersionGameCenterAppVersionLinkageResponse:
		return printLinkagesTable(&LinkagesResponse{Data: []ResourceData{v.Data}})
	case *BuildAppLinkageResponse:
		return printLinkagesTable(&LinkagesResponse{Data: []ResourceData{v.Data}})
	case *BuildAppStoreVersionLinkageResponse:
		return printLinkagesTable(&LinkagesResponse{Data: []ResourceData{v.Data}})
	case *BuildBuildBetaDetailLinkageResponse:
		return printLinkagesTable(&LinkagesResponse{Data: []ResourceData{v.Data}})
	case *BuildPreReleaseVersionLinkageResponse:
		return printLinkagesTable(&LinkagesResponse{Data: []ResourceData{v.Data}})
	case *PreReleaseVersionAppLinkageResponse:
		return printLinkagesTable(&LinkagesResponse{Data: []ResourceData{v.Data}})
	case *AppInfoAgeRatingDeclarationLinkageResponse:
		return printLinkagesTable(&LinkagesResponse{Data: []ResourceData{v.Data}})
	case *AppInfoPrimaryCategoryLinkageResponse:
		return printLinkagesTable(&LinkagesResponse{Data: []ResourceData{v.Data}})
	case *AppInfoPrimarySubcategoryOneLinkageResponse:
		return printLinkagesTable(&LinkagesResponse{Data: []ResourceData{v.Data}})
	case *AppInfoPrimarySubcategoryTwoLinkageResponse:
		return printLinkagesTable(&LinkagesResponse{Data: []ResourceData{v.Data}})
	case *AppInfoSecondaryCategoryLinkageResponse:
		return printLinkagesTable(&LinkagesResponse{Data: []ResourceData{v.Data}})
	case *AppInfoSecondarySubcategoryOneLinkageResponse:
		return printLinkagesTable(&LinkagesResponse{Data: []ResourceData{v.Data}})
	case *AppInfoSecondarySubcategoryTwoLinkageResponse:
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
	case *InAppPurchasesResponse:
		return printLegacyInAppPurchasesTable(v)
	case *InAppPurchaseResponse:
		return printLegacyInAppPurchasesTable(&InAppPurchasesResponse{Data: []Resource[InAppPurchaseAttributes]{v.Data}})
	case *InAppPurchasesV2Response:
		return printInAppPurchasesTable(v)
	case *InAppPurchaseV2Response:
		return printInAppPurchasesTable(&InAppPurchasesV2Response{Data: []Resource[InAppPurchaseV2Attributes]{v.Data}})
	case *InAppPurchaseLocalizationsResponse:
		return printInAppPurchaseLocalizationsTable(v)
	case *InAppPurchaseLocalizationResponse:
		return printInAppPurchaseLocalizationsTable(&InAppPurchaseLocalizationsResponse{Data: []Resource[InAppPurchaseLocalizationAttributes]{v.Data}})
	case *InAppPurchaseImagesResponse:
		return printInAppPurchaseImagesTable(v)
	case *InAppPurchaseImageResponse:
		return printInAppPurchaseImagesTable(&InAppPurchaseImagesResponse{Data: []Resource[InAppPurchaseImageAttributes]{v.Data}})
	case *InAppPurchasePricePointsResponse:
		return printInAppPurchasePricePointsTable(v)
	case *InAppPurchasePricesResponse:
		return printInAppPurchasePricesTable(v)
	case *InAppPurchaseOfferPricesResponse:
		return printInAppPurchaseOfferCodePricesTable(v)
	case *InAppPurchaseOfferCodesResponse:
		return printInAppPurchaseOfferCodesTable(v)
	case *InAppPurchaseOfferCodeResponse:
		return printInAppPurchaseOfferCodesTable(&InAppPurchaseOfferCodesResponse{Data: []Resource[InAppPurchaseOfferCodeAttributes]{v.Data}})
	case *InAppPurchaseOfferCodeCustomCodesResponse:
		return printInAppPurchaseOfferCodeCustomCodesTable(v)
	case *InAppPurchaseOfferCodeCustomCodeResponse:
		return printInAppPurchaseOfferCodeCustomCodesTable(&InAppPurchaseOfferCodeCustomCodesResponse{Data: []Resource[InAppPurchaseOfferCodeCustomCodeAttributes]{v.Data}})
	case *InAppPurchaseOfferCodeOneTimeUseCodesResponse:
		return printInAppPurchaseOfferCodeOneTimeUseCodesTable(v)
	case *InAppPurchaseOfferCodeOneTimeUseCodeResponse:
		return printInAppPurchaseOfferCodeOneTimeUseCodesTable(&InAppPurchaseOfferCodeOneTimeUseCodesResponse{Data: []Resource[InAppPurchaseOfferCodeOneTimeUseCodeAttributes]{v.Data}})
	case *InAppPurchaseAvailabilityResponse:
		return printInAppPurchaseAvailabilityTable(v)
	case *InAppPurchaseContentResponse:
		return printInAppPurchaseContentTable(v)
	case *InAppPurchasePriceScheduleResponse:
		return printInAppPurchasePriceScheduleTable(v)
	case *InAppPurchaseAppStoreReviewScreenshotResponse:
		return printInAppPurchaseReviewScreenshotTable(v)
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
	case *SubscriptionPricesResponse:
		return printSubscriptionPricesTable(v)
	case *SubscriptionPriceResponse:
		return printSubscriptionPriceTable(v)
	case *SubscriptionAvailabilityResponse:
		return printSubscriptionAvailabilityTable(v)
	case *SubscriptionGracePeriodResponse:
		return printSubscriptionGracePeriodTable(v)
	case *TerritoriesResponse:
		return printTerritoriesTable(v)
	case *TerritoryResponse:
		return printTerritoriesTable(&TerritoriesResponse{Data: []Resource[TerritoryAttributes]{v.Data}})
	case *TerritoryAgeRatingsResponse:
		return printTerritoryAgeRatingsTable(v)
	case *OfferCodeValuesResult:
		return printOfferCodeValuesTable(v)
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
	case *SubscriptionOfferCodeCustomCodesResponse:
		return printOfferCodeCustomCodesTable(v)
	case *SubscriptionOfferCodeResponse:
		return printSubscriptionOfferCodeTable(v)
	case *WinBackOffersResponse:
		return printWinBackOffersTable(v)
	case *WinBackOfferResponse:
		return printWinBackOffersTable(&WinBackOffersResponse{Data: []Resource[WinBackOfferAttributes]{v.Data}})
	case *WinBackOfferPricesResponse:
		return printWinBackOfferPricesTable(v)
	case *AppStoreVersionsResponse:
		return printAppStoreVersionsTable(v)
	case *AppStoreVersionResponse:
		return printAppStoreVersionsTable(&AppStoreVersionsResponse{Data: []Resource[AppStoreVersionAttributes]{v.Data}})
	case *PreReleaseVersionsResponse:
		return printPreReleaseVersionsTable(v)
	case *BuildResponse:
		return printBuildsTable(&BuildsResponse{Data: []Resource[BuildAttributes]{v.Data}})
	case *BuildIconsResponse:
		return printBuildIconsTable(v)
	case *BuildUploadsResponse:
		return printBuildUploadsTable(v)
	case *BuildUploadResponse:
		return printBuildUploadsTable(&BuildUploadsResponse{Data: []Resource[BuildUploadAttributes]{v.Data}})
	case *BuildUploadFilesResponse:
		return printBuildUploadFilesTable(v)
	case *BuildUploadFileResponse:
		return printBuildUploadFilesTable(&BuildUploadFilesResponse{Data: []Resource[BuildUploadFileAttributes]{v.Data}})
	case *AppClipDomainStatusResult:
		return printAppClipDomainStatusResultTable(v)
	case *SubscriptionOfferCodeOneTimeUseCodeResponse:
		return printOfferCodesTable(&SubscriptionOfferCodeOneTimeUseCodesResponse{Data: []Resource[SubscriptionOfferCodeOneTimeUseCodeAttributes]{v.Data}})
	case *SubscriptionOfferCodeCustomCodeResponse:
		return printOfferCodeCustomCodesTable(&SubscriptionOfferCodeCustomCodesResponse{Data: []Resource[SubscriptionOfferCodeCustomCodeAttributes]{v.Data}})
	case *WinBackOfferDeleteResult:
		return printWinBackOfferDeleteResultTable(v)
	case *SubscriptionPriceDeleteResult:
		return printSubscriptionPriceDeleteResultTable(v)
	case *SubscriptionOfferCodePricesResponse:
		return printOfferCodePricesTable(v)
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
	case *BetaAppLocalizationsResponse:
		return printBetaAppLocalizationsTable(v)
	case *BetaAppLocalizationResponse:
		return printBetaAppLocalizationsTable(&BetaAppLocalizationsResponse{Data: []Resource[BetaAppLocalizationAttributes]{v.Data}})
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
	case *BetaLicenseAgreementsResponse:
		return printBetaLicenseAgreementsTable(v)
	case *BetaLicenseAgreementResponse:
		return printBetaLicenseAgreementsTable(&BetaLicenseAgreementsResponse{Data: []BetaLicenseAgreementResource{v.Data}})
	case *BuildBetaNotificationResponse:
		return printBuildBetaNotificationTable(v)
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
	case *AnalyticsReportsResponse:
		return printAnalyticsReportsTable(v)
	case *AnalyticsReportResponse:
		return printAnalyticsReportsTable(&AnalyticsReportsResponse{Data: []Resource[AnalyticsReportAttributes]{v.Data}, Links: v.Links})
	case *AnalyticsReportInstancesResponse:
		return printAnalyticsReportInstancesTable(v)
	case *AnalyticsReportInstanceResponse:
		return printAnalyticsReportInstancesTable(&AnalyticsReportInstancesResponse{Data: []Resource[AnalyticsReportInstanceAttributes]{v.Data}, Links: v.Links})
	case *AnalyticsReportSegmentsResponse:
		return printAnalyticsReportSegmentsTable(v)
	case *AnalyticsReportSegmentResponse:
		return printAnalyticsReportSegmentsTable(&AnalyticsReportSegmentsResponse{Data: []Resource[AnalyticsReportSegmentAttributes]{v.Data}, Links: v.Links})
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
	case *BuildIndividualTestersUpdateResult:
		return printBuildIndividualTestersUpdateTable(v)
	case *BuildUploadDeleteResult:
		return printBuildUploadDeleteResultTable(v)
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
	case *GameCenterAchievementVersionsResponse:
		return printGameCenterAchievementVersionsTable(v)
	case *GameCenterAchievementVersionResponse:
		return printGameCenterAchievementVersionsTable(&GameCenterAchievementVersionsResponse{Data: []Resource[GameCenterAchievementVersionAttributes]{v.Data}})
	case *GameCenterLeaderboardsResponse:
		return printGameCenterLeaderboardsTable(v)
	case *GameCenterLeaderboardResponse:
		return printGameCenterLeaderboardsTable(&GameCenterLeaderboardsResponse{Data: []Resource[GameCenterLeaderboardAttributes]{v.Data}})
	case *GameCenterLeaderboardDeleteResult:
		return printGameCenterLeaderboardDeleteResultTable(v)
	case *GameCenterLeaderboardVersionsResponse:
		return printGameCenterLeaderboardVersionsTable(v)
	case *GameCenterLeaderboardVersionResponse:
		return printGameCenterLeaderboardVersionsTable(&GameCenterLeaderboardVersionsResponse{Data: []Resource[GameCenterLeaderboardVersionAttributes]{v.Data}})
	case *GameCenterLeaderboardSetsResponse:
		return printGameCenterLeaderboardSetsTable(v)
	case *GameCenterLeaderboardSetResponse:
		return printGameCenterLeaderboardSetsTable(&GameCenterLeaderboardSetsResponse{Data: []Resource[GameCenterLeaderboardSetAttributes]{v.Data}})
	case *GameCenterLeaderboardSetDeleteResult:
		return printGameCenterLeaderboardSetDeleteResultTable(v)
	case *GameCenterLeaderboardSetVersionsResponse:
		return printGameCenterLeaderboardSetVersionsTable(v)
	case *GameCenterLeaderboardSetVersionResponse:
		return printGameCenterLeaderboardSetVersionsTable(&GameCenterLeaderboardSetVersionsResponse{Data: []Resource[GameCenterLeaderboardSetVersionAttributes]{v.Data}})
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
	case *GameCenterLeaderboardEntrySubmissionResponse:
		return printGameCenterLeaderboardEntrySubmissionTable(v)
	case *GameCenterPlayerAchievementSubmissionResponse:
		return printGameCenterPlayerAchievementSubmissionTable(v)
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
	case *GameCenterChallengesResponse:
		return printGameCenterChallengesTable(v)
	case *GameCenterChallengeResponse:
		return printGameCenterChallengesTable(&GameCenterChallengesResponse{Data: []Resource[GameCenterChallengeAttributes]{v.Data}})
	case *GameCenterChallengeDeleteResult:
		return printGameCenterChallengeDeleteResultTable(v)
	case *GameCenterChallengeVersionsResponse:
		return printGameCenterChallengeVersionsTable(v)
	case *GameCenterChallengeVersionResponse:
		return printGameCenterChallengeVersionsTable(&GameCenterChallengeVersionsResponse{Data: []Resource[GameCenterChallengeVersionAttributes]{v.Data}})
	case *GameCenterChallengeLocalizationsResponse:
		return printGameCenterChallengeLocalizationsTable(v)
	case *GameCenterChallengeLocalizationResponse:
		return printGameCenterChallengeLocalizationsTable(&GameCenterChallengeLocalizationsResponse{Data: []Resource[GameCenterChallengeLocalizationAttributes]{v.Data}})
	case *GameCenterChallengeLocalizationDeleteResult:
		return printGameCenterChallengeLocalizationDeleteResultTable(v)
	case *GameCenterChallengeImagesResponse:
		return printGameCenterChallengeImagesTable(v)
	case *GameCenterChallengeImageResponse:
		return printGameCenterChallengeImagesTable(&GameCenterChallengeImagesResponse{Data: []Resource[GameCenterChallengeImageAttributes]{v.Data}})
	case *GameCenterChallengeImageUploadResult:
		return printGameCenterChallengeImageUploadResultTable(v)
	case *GameCenterChallengeImageDeleteResult:
		return printGameCenterChallengeImageDeleteResultTable(v)
	case *GameCenterChallengeVersionReleasesResponse:
		return printGameCenterChallengeReleasesTable(v)
	case *GameCenterChallengeVersionReleaseResponse:
		return printGameCenterChallengeReleasesTable(&GameCenterChallengeVersionReleasesResponse{Data: []Resource[GameCenterChallengeVersionReleaseAttributes]{v.Data}})
	case *GameCenterChallengeVersionReleaseDeleteResult:
		return printGameCenterChallengeReleaseDeleteResultTable(v)
	case *GameCenterActivitiesResponse:
		return printGameCenterActivitiesTable(v)
	case *GameCenterActivityResponse:
		return printGameCenterActivitiesTable(&GameCenterActivitiesResponse{Data: []Resource[GameCenterActivityAttributes]{v.Data}})
	case *GameCenterActivityDeleteResult:
		return printGameCenterActivityDeleteResultTable(v)
	case *GameCenterActivityVersionsResponse:
		return printGameCenterActivityVersionsTable(v)
	case *GameCenterActivityVersionResponse:
		return printGameCenterActivityVersionsTable(&GameCenterActivityVersionsResponse{Data: []Resource[GameCenterActivityVersionAttributes]{v.Data}})
	case *GameCenterActivityLocalizationsResponse:
		return printGameCenterActivityLocalizationsTable(v)
	case *GameCenterActivityLocalizationResponse:
		return printGameCenterActivityLocalizationsTable(&GameCenterActivityLocalizationsResponse{Data: []Resource[GameCenterActivityLocalizationAttributes]{v.Data}})
	case *GameCenterActivityLocalizationDeleteResult:
		return printGameCenterActivityLocalizationDeleteResultTable(v)
	case *GameCenterActivityImagesResponse:
		return printGameCenterActivityImagesTable(v)
	case *GameCenterActivityImageResponse:
		return printGameCenterActivityImagesTable(&GameCenterActivityImagesResponse{Data: []Resource[GameCenterActivityImageAttributes]{v.Data}})
	case *GameCenterActivityImageUploadResult:
		return printGameCenterActivityImageUploadResultTable(v)
	case *GameCenterActivityImageDeleteResult:
		return printGameCenterActivityImageDeleteResultTable(v)
	case *GameCenterActivityVersionReleasesResponse:
		return printGameCenterActivityReleasesTable(v)
	case *GameCenterActivityVersionReleaseResponse:
		return printGameCenterActivityReleasesTable(&GameCenterActivityVersionReleasesResponse{Data: []Resource[GameCenterActivityVersionReleaseAttributes]{v.Data}})
	case *GameCenterActivityVersionReleaseDeleteResult:
		return printGameCenterActivityReleaseDeleteResultTable(v)
	case *GameCenterGroupsResponse:
		return printGameCenterGroupsTable(v)
	case *GameCenterGroupResponse:
		return printGameCenterGroupsTable(&GameCenterGroupsResponse{Data: []Resource[GameCenterGroupAttributes]{v.Data}})
	case *GameCenterGroupDeleteResult:
		return printGameCenterGroupDeleteResultTable(v)
	case *GameCenterAppVersionsResponse:
		return printGameCenterAppVersionsTable(v)
	case *GameCenterAppVersionResponse:
		return printGameCenterAppVersionsTable(&GameCenterAppVersionsResponse{Data: []Resource[GameCenterAppVersionAttributes]{v.Data}})
	case *GameCenterEnabledVersionsResponse:
		return printGameCenterEnabledVersionsTable(v)
	case *GameCenterDetailsResponse:
		return printGameCenterDetailsTable(v)
	case *GameCenterDetailResponse:
		return printGameCenterDetailsTable(&GameCenterDetailsResponse{Data: []Resource[GameCenterDetailAttributes]{v.Data}})
	case *GameCenterMatchmakingQueuesResponse:
		return printGameCenterMatchmakingQueuesTable(v)
	case *GameCenterMatchmakingQueueResponse:
		return printGameCenterMatchmakingQueuesTable(&GameCenterMatchmakingQueuesResponse{Data: []Resource[GameCenterMatchmakingQueueAttributes]{v.Data}})
	case *GameCenterMatchmakingQueueDeleteResult:
		return printGameCenterMatchmakingQueueDeleteResultTable(v)
	case *GameCenterMatchmakingRuleSetsResponse:
		return printGameCenterMatchmakingRuleSetsTable(v)
	case *GameCenterMatchmakingRuleSetResponse:
		return printGameCenterMatchmakingRuleSetsTable(&GameCenterMatchmakingRuleSetsResponse{Data: []Resource[GameCenterMatchmakingRuleSetAttributes]{v.Data}})
	case *GameCenterMatchmakingRuleSetDeleteResult:
		return printGameCenterMatchmakingRuleSetDeleteResultTable(v)
	case *GameCenterMatchmakingRulesResponse:
		return printGameCenterMatchmakingRulesTable(v)
	case *GameCenterMatchmakingRuleResponse:
		return printGameCenterMatchmakingRulesTable(&GameCenterMatchmakingRulesResponse{Data: []Resource[GameCenterMatchmakingRuleAttributes]{v.Data}})
	case *GameCenterMatchmakingRuleDeleteResult:
		return printGameCenterMatchmakingRuleDeleteResultTable(v)
	case *GameCenterMatchmakingTeamsResponse:
		return printGameCenterMatchmakingTeamsTable(v)
	case *GameCenterMatchmakingTeamResponse:
		return printGameCenterMatchmakingTeamsTable(&GameCenterMatchmakingTeamsResponse{Data: []Resource[GameCenterMatchmakingTeamAttributes]{v.Data}})
	case *GameCenterMatchmakingTeamDeleteResult:
		return printGameCenterMatchmakingTeamDeleteResultTable(v)
	case *GameCenterMetricsResponse:
		return printGameCenterMetricsTable(v)
	case *GameCenterMatchmakingRuleSetTestResponse:
		return printGameCenterMatchmakingRuleSetTestTable(v)
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
	case *BetaAppLocalizationDeleteResult:
		return printBetaAppLocalizationDeleteResultTable(v)
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
	case *ScmProvidersResponse:
		return printScmProvidersTable(v)
	case *ScmProviderResponse:
		return printScmProvidersTable(&ScmProvidersResponse{Data: []ScmProviderResource{v.Data}, Links: v.Links})
	case *ScmRepositoriesResponse:
		return printScmRepositoriesTable(v)
	case *ScmGitReferencesResponse:
		return printScmGitReferencesTable(v)
	case *ScmGitReferenceResponse:
		return printScmGitReferencesTable(&ScmGitReferencesResponse{Data: []ScmGitReferenceResource{v.Data}, Links: v.Links})
	case *ScmPullRequestsResponse:
		return printScmPullRequestsTable(v)
	case *ScmPullRequestResponse:
		return printScmPullRequestsTable(&ScmPullRequestsResponse{Data: []ScmPullRequestResource{v.Data}, Links: v.Links})
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
	case *AppKeywordsResponse:
		return printAppKeywordsTable(v)
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
