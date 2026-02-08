package asc

//nolint:gochecknoinits // registry init is the idiomatic way to populate a type map
func init() {
	registerRows(feedbackRows)
	registerRows(crashesRows)
	registerRows(reviewsRows)
	registerRows(customerReviewSummarizationsRows)
	registerRows(func(v *CustomerReviewResponse) ([]string, [][]string) {
		return reviewsRows(&ReviewsResponse{Data: []Resource[ReviewAttributes]{v.Data}})
	})
	registerRows(appsRows)
	registerRows(appClipsRows)
	registerRows(appCategoriesRows)
	registerRows(func(v *AppCategoryResponse) ([]string, [][]string) {
		return appCategoriesRows(&AppCategoriesResponse{Data: []AppCategory{v.Data}})
	})
	registerRows(appInfosRows)
	registerRows(func(v *AppInfoResponse) ([]string, [][]string) {
		return appInfosRows(&AppInfosResponse{Data: []Resource[AppInfoAttributes]{v.Data}})
	})
	registerRows(func(v *AppResponse) ([]string, [][]string) {
		return appsRows(&AppsResponse{Data: []Resource[AppAttributes]{v.Data}})
	})
	registerRows(func(v *AppClipResponse) ([]string, [][]string) {
		return appClipsRows(&AppClipsResponse{Data: []Resource[AppClipAttributes]{v.Data}})
	})
	registerRows(appClipDefaultExperiencesRows)
	registerRows(func(v *AppClipDefaultExperienceResponse) ([]string, [][]string) {
		return appClipDefaultExperiencesRows(&AppClipDefaultExperiencesResponse{Data: []Resource[AppClipDefaultExperienceAttributes]{v.Data}})
	})
	registerRows(appClipDefaultExperienceLocalizationsRows)
	registerRows(func(v *AppClipDefaultExperienceLocalizationResponse) ([]string, [][]string) {
		return appClipDefaultExperienceLocalizationsRows(&AppClipDefaultExperienceLocalizationsResponse{Data: []Resource[AppClipDefaultExperienceLocalizationAttributes]{v.Data}})
	})
	registerRows(appClipHeaderImageRows)
	registerRows(appClipAdvancedExperienceImageRows)
	registerRows(appClipAdvancedExperiencesRows)
	registerRows(func(v *AppClipAdvancedExperienceResponse) ([]string, [][]string) {
		return appClipAdvancedExperiencesRows(&AppClipAdvancedExperiencesResponse{Data: []Resource[AppClipAdvancedExperienceAttributes]{v.Data}})
	})
	registerRows(appSetupInfoResultRows)
	registerRows(appTagsRows)
	registerRows(func(v *AppTagResponse) ([]string, [][]string) {
		return appTagsRows(&AppTagsResponse{Data: []Resource[AppTagAttributes]{v.Data}})
	})
	registerRows(marketplaceSearchDetailsRows)
	registerRows(func(v *MarketplaceSearchDetailResponse) ([]string, [][]string) {
		return marketplaceSearchDetailsRows(&MarketplaceSearchDetailsResponse{Data: []Resource[MarketplaceSearchDetailAttributes]{v.Data}})
	})
	registerRows(marketplaceWebhooksRows)
	registerRows(func(v *MarketplaceWebhookResponse) ([]string, [][]string) {
		return marketplaceWebhooksRows(&MarketplaceWebhooksResponse{Data: []Resource[MarketplaceWebhookAttributes]{v.Data}})
	})
	registerRows(webhooksRows)
	registerRows(func(v *WebhookResponse) ([]string, [][]string) {
		return webhooksRows(&WebhooksResponse{Data: []Resource[WebhookAttributes]{v.Data}})
	})
	registerRows(webhookDeliveriesRows)
	registerRows(func(v *WebhookDeliveryResponse) ([]string, [][]string) {
		return webhookDeliveriesRows(&WebhookDeliveriesResponse{Data: []Resource[WebhookDeliveryAttributes]{v.Data}})
	})
	registerRows(alternativeDistributionDomainsRows)
	registerRows(func(v *AlternativeDistributionDomainResponse) ([]string, [][]string) {
		return alternativeDistributionDomainsRows(&AlternativeDistributionDomainsResponse{Data: []Resource[AlternativeDistributionDomainAttributes]{v.Data}})
	})
	registerRows(alternativeDistributionKeysRows)
	registerRows(func(v *AlternativeDistributionKeyResponse) ([]string, [][]string) {
		return alternativeDistributionKeysRows(&AlternativeDistributionKeysResponse{Data: []Resource[AlternativeDistributionKeyAttributes]{v.Data}})
	})
	registerRows(alternativeDistributionPackageRows)
	registerRows(alternativeDistributionPackageVersionsRows)
	registerRows(func(v *AlternativeDistributionPackageVersionResponse) ([]string, [][]string) {
		return alternativeDistributionPackageVersionsRows(&AlternativeDistributionPackageVersionsResponse{Data: []Resource[AlternativeDistributionPackageVersionAttributes]{v.Data}})
	})
	registerRows(alternativeDistributionPackageVariantsRows)
	registerRows(func(v *AlternativeDistributionPackageVariantResponse) ([]string, [][]string) {
		return alternativeDistributionPackageVariantsRows(&AlternativeDistributionPackageVariantsResponse{Data: []Resource[AlternativeDistributionPackageVariantAttributes]{v.Data}})
	})
	registerRows(alternativeDistributionPackageDeltasRows)
	registerRows(func(v *AlternativeDistributionPackageDeltaResponse) ([]string, [][]string) {
		return alternativeDistributionPackageDeltasRows(&AlternativeDistributionPackageDeltasResponse{Data: []Resource[AlternativeDistributionPackageDeltaAttributes]{v.Data}})
	})
	registerRows(backgroundAssetsRows)
	registerRows(func(v *BackgroundAssetResponse) ([]string, [][]string) {
		return backgroundAssetsRows(&BackgroundAssetsResponse{Data: []Resource[BackgroundAssetAttributes]{v.Data}})
	})
	registerRows(backgroundAssetVersionsRows)
	registerRows(func(v *BackgroundAssetVersionResponse) ([]string, [][]string) {
		return backgroundAssetVersionsRows(&BackgroundAssetVersionsResponse{Data: []Resource[BackgroundAssetVersionAttributes]{v.Data}})
	})
	registerRows(func(v *BackgroundAssetVersionAppStoreReleaseResponse) ([]string, [][]string) {
		return backgroundAssetVersionStateRows(v.Data.ID, v.Data.Attributes.State)
	})
	registerRows(func(v *BackgroundAssetVersionExternalBetaReleaseResponse) ([]string, [][]string) {
		return backgroundAssetVersionStateRows(v.Data.ID, v.Data.Attributes.State)
	})
	registerRows(func(v *BackgroundAssetVersionInternalBetaReleaseResponse) ([]string, [][]string) {
		return backgroundAssetVersionStateRows(v.Data.ID, v.Data.Attributes.State)
	})
	registerRows(backgroundAssetUploadFilesRows)
	registerRows(func(v *BackgroundAssetUploadFileResponse) ([]string, [][]string) {
		return backgroundAssetUploadFilesRows(&BackgroundAssetUploadFilesResponse{Data: []Resource[BackgroundAssetUploadFileAttributes]{v.Data}})
	})
	registerRows(nominationsRows)
	registerRows(func(v *NominationResponse) ([]string, [][]string) {
		return nominationsRows(&NominationsResponse{Data: []Resource[NominationAttributes]{v.Data}})
	})
	registerRows(linkagesRows)
	registerRows(func(v *AppClipDefaultExperienceReviewDetailLinkageResponse) ([]string, [][]string) {
		return linkagesRows(&LinkagesResponse{Data: []ResourceData{v.Data}})
	})
	registerRows(func(v *AppClipDefaultExperienceReleaseWithAppStoreVersionLinkageResponse) ([]string, [][]string) {
		return linkagesRows(&LinkagesResponse{Data: []ResourceData{v.Data}})
	})
	registerRows(func(v *AppClipDefaultExperienceLocalizationHeaderImageLinkageResponse) ([]string, [][]string) {
		return linkagesRows(&LinkagesResponse{Data: []ResourceData{v.Data}})
	})
	registerRows(func(v *AppStoreVersionAgeRatingDeclarationLinkageResponse) ([]string, [][]string) {
		return linkagesRows(&LinkagesResponse{Data: []ResourceData{v.Data}})
	})
	registerRows(func(v *AppStoreVersionReviewDetailLinkageResponse) ([]string, [][]string) {
		return linkagesRows(&LinkagesResponse{Data: []ResourceData{v.Data}})
	})
	registerRows(func(v *AppStoreVersionAppClipDefaultExperienceLinkageResponse) ([]string, [][]string) {
		return linkagesRows(&LinkagesResponse{Data: []ResourceData{v.Data}})
	})
	registerRows(func(v *AppStoreVersionSubmissionLinkageResponse) ([]string, [][]string) {
		return linkagesRows(&LinkagesResponse{Data: []ResourceData{v.Data}})
	})
	registerRows(func(v *AppStoreVersionRoutingAppCoverageLinkageResponse) ([]string, [][]string) {
		return linkagesRows(&LinkagesResponse{Data: []ResourceData{v.Data}})
	})
	registerRows(func(v *AppStoreVersionAlternativeDistributionPackageLinkageResponse) ([]string, [][]string) {
		return linkagesRows(&LinkagesResponse{Data: []ResourceData{v.Data}})
	})
	registerRows(func(v *AppStoreVersionGameCenterAppVersionLinkageResponse) ([]string, [][]string) {
		return linkagesRows(&LinkagesResponse{Data: []ResourceData{v.Data}})
	})
	registerRows(func(v *BuildAppLinkageResponse) ([]string, [][]string) {
		return linkagesRows(&LinkagesResponse{Data: []ResourceData{v.Data}})
	})
	registerRows(func(v *BuildAppStoreVersionLinkageResponse) ([]string, [][]string) {
		return linkagesRows(&LinkagesResponse{Data: []ResourceData{v.Data}})
	})
	registerRows(func(v *BuildBuildBetaDetailLinkageResponse) ([]string, [][]string) {
		return linkagesRows(&LinkagesResponse{Data: []ResourceData{v.Data}})
	})
	registerRows(func(v *BuildPreReleaseVersionLinkageResponse) ([]string, [][]string) {
		return linkagesRows(&LinkagesResponse{Data: []ResourceData{v.Data}})
	})
	registerRows(func(v *PreReleaseVersionAppLinkageResponse) ([]string, [][]string) {
		return linkagesRows(&LinkagesResponse{Data: []ResourceData{v.Data}})
	})
	registerRows(func(v *AppInfoAgeRatingDeclarationLinkageResponse) ([]string, [][]string) {
		return linkagesRows(&LinkagesResponse{Data: []ResourceData{v.Data}})
	})
	registerRows(func(v *AppInfoPrimaryCategoryLinkageResponse) ([]string, [][]string) {
		return linkagesRows(&LinkagesResponse{Data: []ResourceData{v.Data}})
	})
	registerRows(func(v *AppInfoPrimarySubcategoryOneLinkageResponse) ([]string, [][]string) {
		return linkagesRows(&LinkagesResponse{Data: []ResourceData{v.Data}})
	})
	registerRows(func(v *AppInfoPrimarySubcategoryTwoLinkageResponse) ([]string, [][]string) {
		return linkagesRows(&LinkagesResponse{Data: []ResourceData{v.Data}})
	})
	registerRows(func(v *AppInfoSecondaryCategoryLinkageResponse) ([]string, [][]string) {
		return linkagesRows(&LinkagesResponse{Data: []ResourceData{v.Data}})
	})
	registerRows(func(v *AppInfoSecondarySubcategoryOneLinkageResponse) ([]string, [][]string) {
		return linkagesRows(&LinkagesResponse{Data: []ResourceData{v.Data}})
	})
	registerRows(func(v *AppInfoSecondarySubcategoryTwoLinkageResponse) ([]string, [][]string) {
		return linkagesRows(&LinkagesResponse{Data: []ResourceData{v.Data}})
	})
	registerRows(bundleIDsRows)
	registerRows(func(v *BundleIDResponse) ([]string, [][]string) {
		return bundleIDsRows(&BundleIDsResponse{Data: []Resource[BundleIDAttributes]{v.Data}})
	})
	registerRows(merchantIDsRows)
	registerRows(func(v *MerchantIDResponse) ([]string, [][]string) {
		return merchantIDsRows(&MerchantIDsResponse{Data: []Resource[MerchantIDAttributes]{v.Data}})
	})
	registerRows(passTypeIDsRows)
	registerRows(func(v *PassTypeIDResponse) ([]string, [][]string) {
		return passTypeIDsRows(&PassTypeIDsResponse{Data: []Resource[PassTypeIDAttributes]{v.Data}})
	})
	registerRows(certificatesRows)
	registerRows(func(v *CertificateResponse) ([]string, [][]string) {
		return certificatesRows(&CertificatesResponse{Data: []Resource[CertificateAttributes]{v.Data}})
	})
	registerRows(profilesRows)
	registerRows(func(v *ProfileResponse) ([]string, [][]string) {
		return profilesRows(&ProfilesResponse{Data: []Resource[ProfileAttributes]{v.Data}})
	})
	registerRows(legacyInAppPurchasesRows)
	registerRows(func(v *InAppPurchaseResponse) ([]string, [][]string) {
		return legacyInAppPurchasesRows(&InAppPurchasesResponse{Data: []Resource[InAppPurchaseAttributes]{v.Data}})
	})
	registerRows(inAppPurchasesRows)
	registerRows(func(v *InAppPurchaseV2Response) ([]string, [][]string) {
		return inAppPurchasesRows(&InAppPurchasesV2Response{Data: []Resource[InAppPurchaseV2Attributes]{v.Data}})
	})
	registerRows(inAppPurchaseLocalizationsRows)
	registerRows(func(v *InAppPurchaseLocalizationResponse) ([]string, [][]string) {
		return inAppPurchaseLocalizationsRows(&InAppPurchaseLocalizationsResponse{Data: []Resource[InAppPurchaseLocalizationAttributes]{v.Data}})
	})
	registerRows(inAppPurchaseImagesRows)
	registerRows(func(v *InAppPurchaseImageResponse) ([]string, [][]string) {
		return inAppPurchaseImagesRows(&InAppPurchaseImagesResponse{Data: []Resource[InAppPurchaseImageAttributes]{v.Data}})
	})
	registerRows(inAppPurchasePricePointsRows)
	registerRowsErr(inAppPurchasePricesRows)
	registerRowsErr(inAppPurchaseOfferCodePricesRows)
	registerRows(inAppPurchaseOfferCodesRows)
	registerRows(func(v *InAppPurchaseOfferCodeResponse) ([]string, [][]string) {
		return inAppPurchaseOfferCodesRows(&InAppPurchaseOfferCodesResponse{Data: []Resource[InAppPurchaseOfferCodeAttributes]{v.Data}})
	})
	registerRows(inAppPurchaseOfferCodeCustomCodesRows)
	registerRows(func(v *InAppPurchaseOfferCodeCustomCodeResponse) ([]string, [][]string) {
		return inAppPurchaseOfferCodeCustomCodesRows(&InAppPurchaseOfferCodeCustomCodesResponse{Data: []Resource[InAppPurchaseOfferCodeCustomCodeAttributes]{v.Data}})
	})
	registerRows(inAppPurchaseOfferCodeOneTimeUseCodesRows)
	registerRows(func(v *InAppPurchaseOfferCodeOneTimeUseCodeResponse) ([]string, [][]string) {
		return inAppPurchaseOfferCodeOneTimeUseCodesRows(&InAppPurchaseOfferCodeOneTimeUseCodesResponse{Data: []Resource[InAppPurchaseOfferCodeOneTimeUseCodeAttributes]{v.Data}})
	})
	registerRows(inAppPurchaseAvailabilityRows)
	registerRows(inAppPurchaseContentRows)
	registerRows(inAppPurchasePriceScheduleRows)
	registerRows(inAppPurchaseReviewScreenshotRows)
	registerRows(appEventsRows)
	registerRows(func(v *AppEventResponse) ([]string, [][]string) {
		return appEventsRows(&AppEventsResponse{Data: []Resource[AppEventAttributes]{v.Data}})
	})
	registerRows(appEventLocalizationsRows)
	registerRows(func(v *AppEventLocalizationResponse) ([]string, [][]string) {
		return appEventLocalizationsRows(&AppEventLocalizationsResponse{Data: []Resource[AppEventLocalizationAttributes]{v.Data}})
	})
	registerRows(appEventScreenshotsRows)
	registerRows(func(v *AppEventScreenshotResponse) ([]string, [][]string) {
		return appEventScreenshotsRows(&AppEventScreenshotsResponse{Data: []Resource[AppEventScreenshotAttributes]{v.Data}})
	})
	registerRows(appEventVideoClipsRows)
	registerRows(func(v *AppEventVideoClipResponse) ([]string, [][]string) {
		return appEventVideoClipsRows(&AppEventVideoClipsResponse{Data: []Resource[AppEventVideoClipAttributes]{v.Data}})
	})
	registerRows(subscriptionGroupsRows)
	registerRows(func(v *SubscriptionGroupResponse) ([]string, [][]string) {
		return subscriptionGroupsRows(&SubscriptionGroupsResponse{Data: []Resource[SubscriptionGroupAttributes]{v.Data}})
	})
	registerRows(subscriptionsRows)
	registerRows(func(v *SubscriptionResponse) ([]string, [][]string) {
		return subscriptionsRows(&SubscriptionsResponse{Data: []Resource[SubscriptionAttributes]{v.Data}})
	})
	registerRows(promotedPurchasesRows)
	registerRows(func(v *PromotedPurchaseResponse) ([]string, [][]string) {
		return promotedPurchasesRows(&PromotedPurchasesResponse{Data: []Resource[PromotedPurchaseAttributes]{v.Data}})
	})
	registerRowsErr(subscriptionPricesRows)
	registerRows(subscriptionPriceRows)
	registerRows(subscriptionAvailabilityRows)
	registerRows(subscriptionGracePeriodRows)
	registerRows(territoriesRows)
	registerRows(func(v *TerritoryResponse) ([]string, [][]string) {
		return territoriesRows(&TerritoriesResponse{Data: []Resource[TerritoryAttributes]{v.Data}})
	})
	registerRowsErr(territoryAgeRatingsRows)
	registerRows(offerCodeValuesRows)
	registerRows(appPricePointsRows)
	registerRows(appPriceScheduleRows)
	registerRows(appPricesRows)
	registerRows(buildsRows)
	registerRows(buildBundlesRows)
	registerRows(buildBundleFileSizesRows)
	registerRows(betaAppClipInvocationsRows)
	registerRows(func(v *BetaAppClipInvocationResponse) ([]string, [][]string) {
		return betaAppClipInvocationsRows(&BetaAppClipInvocationsResponse{Data: []Resource[BetaAppClipInvocationAttributes]{v.Data}})
	})
	registerRows(betaAppClipInvocationLocalizationsRows)
	registerRows(func(v *BetaAppClipInvocationLocalizationResponse) ([]string, [][]string) {
		return betaAppClipInvocationLocalizationsRows(&BetaAppClipInvocationLocalizationsResponse{Data: []Resource[BetaAppClipInvocationLocalizationAttributes]{v.Data}})
	})
	registerRows(offerCodesRows)
	registerRows(offerCodeCustomCodesRows)
	registerRows(subscriptionOfferCodeRows)
	registerRows(winBackOffersRows)
	registerRows(func(v *WinBackOfferResponse) ([]string, [][]string) {
		return winBackOffersRows(&WinBackOffersResponse{Data: []Resource[WinBackOfferAttributes]{v.Data}})
	})
	registerRowsErr(winBackOfferPricesRows)
	registerRows(appStoreVersionsRows)
	registerRows(func(v *AppStoreVersionResponse) ([]string, [][]string) {
		return appStoreVersionsRows(&AppStoreVersionsResponse{Data: []Resource[AppStoreVersionAttributes]{v.Data}})
	})
	registerRows(preReleaseVersionsRows)
	registerRows(func(v *BuildResponse) ([]string, [][]string) {
		return buildsRows(&BuildsResponse{Data: []Resource[BuildAttributes]{v.Data}})
	})
	registerRows(buildIconsRows)
	registerRows(buildUploadsRows)
	registerRows(func(v *BuildUploadResponse) ([]string, [][]string) {
		return buildUploadsRows(&BuildUploadsResponse{Data: []Resource[BuildUploadAttributes]{v.Data}})
	})
	registerRows(buildUploadFilesRows)
	registerRows(func(v *BuildUploadFileResponse) ([]string, [][]string) {
		return buildUploadFilesRows(&BuildUploadFilesResponse{Data: []Resource[BuildUploadFileAttributes]{v.Data}})
	})
	registerDirect(func(v *AppClipDomainStatusResult, render func([]string, [][]string)) error {
		h, r := appClipDomainStatusMainRows(v)
		render(h, r)
		if len(v.Domains) > 0 {
			dh, dr := appClipDomainStatusDomainRows(v)
			render(dh, dr)
		}
		return nil
	})
	registerRows(func(v *SubscriptionOfferCodeOneTimeUseCodeResponse) ([]string, [][]string) {
		return offerCodesRows(&SubscriptionOfferCodeOneTimeUseCodesResponse{Data: []Resource[SubscriptionOfferCodeOneTimeUseCodeAttributes]{v.Data}})
	})
	registerRows(func(v *SubscriptionOfferCodeCustomCodeResponse) ([]string, [][]string) {
		return offerCodeCustomCodesRows(&SubscriptionOfferCodeCustomCodesResponse{Data: []Resource[SubscriptionOfferCodeCustomCodeAttributes]{v.Data}})
	})
	registerRows(winBackOfferDeleteResultRows)
	registerRows(subscriptionPriceDeleteResultRows)
	registerRowsErr(offerCodePricesRows)
	registerRows(appAvailabilityRows)
	registerRows(territoryAvailabilitiesRows)
	registerRows(endAppAvailabilityPreOrderRows)
	registerRows(func(v *PreReleaseVersionResponse) ([]string, [][]string) {
		return preReleaseVersionsRows(&PreReleaseVersionsResponse{Data: []PreReleaseVersion{v.Data}})
	})
	registerRows(appStoreVersionLocalizationsRows)
	registerRows(func(v *AppStoreVersionLocalizationResponse) ([]string, [][]string) {
		return appStoreVersionLocalizationsRows(&AppStoreVersionLocalizationsResponse{Data: []Resource[AppStoreVersionLocalizationAttributes]{v.Data}})
	})
	registerRows(betaAppLocalizationsRows)
	registerRows(func(v *BetaAppLocalizationResponse) ([]string, [][]string) {
		return betaAppLocalizationsRows(&BetaAppLocalizationsResponse{Data: []Resource[BetaAppLocalizationAttributes]{v.Data}})
	})
	registerRows(betaBuildLocalizationsRows)
	registerRows(func(v *BetaBuildLocalizationResponse) ([]string, [][]string) {
		return betaBuildLocalizationsRows(&BetaBuildLocalizationsResponse{Data: []Resource[BetaBuildLocalizationAttributes]{v.Data}})
	})
	registerRows(appInfoLocalizationsRows)
	registerRows(appScreenshotSetsRows)
	registerRows(func(v *AppScreenshotSetResponse) ([]string, [][]string) {
		return appScreenshotSetsRows(&AppScreenshotSetsResponse{Data: []Resource[AppScreenshotSetAttributes]{v.Data}})
	})
	registerRows(appScreenshotsRows)
	registerRows(func(v *AppScreenshotResponse) ([]string, [][]string) {
		return appScreenshotsRows(&AppScreenshotsResponse{Data: []Resource[AppScreenshotAttributes]{v.Data}})
	})
	registerRows(appPreviewSetsRows)
	registerRows(func(v *AppPreviewSetResponse) ([]string, [][]string) {
		return appPreviewSetsRows(&AppPreviewSetsResponse{Data: []Resource[AppPreviewSetAttributes]{v.Data}})
	})
	registerRows(appPreviewsRows)
	registerRows(func(v *AppPreviewResponse) ([]string, [][]string) {
		return appPreviewsRows(&AppPreviewsResponse{Data: []Resource[AppPreviewAttributes]{v.Data}})
	})
	registerRows(betaGroupsRows)
	registerRows(func(v *BetaGroupResponse) ([]string, [][]string) {
		return betaGroupsRows(&BetaGroupsResponse{Data: []Resource[BetaGroupAttributes]{v.Data}})
	})
	registerRows(betaTestersRows)
	registerRows(func(v *BetaTesterResponse) ([]string, [][]string) {
		return betaTestersRows(&BetaTestersResponse{Data: []Resource[BetaTesterAttributes]{v.Data}})
	})
	registerRows(usersRows)
	registerRows(func(v *UserResponse) ([]string, [][]string) {
		return usersRows(&UsersResponse{Data: []Resource[UserAttributes]{v.Data}})
	})
	registerRows(actorsRows)
	registerRows(func(v *ActorResponse) ([]string, [][]string) {
		return actorsRows(&ActorsResponse{Data: []Resource[ActorAttributes]{v.Data}})
	})
	registerRows(devicesRows)
	registerRows(deviceLocalUDIDRows)
	registerRows(func(v *DeviceResponse) ([]string, [][]string) {
		return devicesRows(&DevicesResponse{Data: []Resource[DeviceAttributes]{v.Data}})
	})
	registerRows(userInvitationsRows)
	registerRows(func(v *UserInvitationResponse) ([]string, [][]string) {
		return userInvitationsRows(&UserInvitationsResponse{Data: []Resource[UserInvitationAttributes]{v.Data}})
	})
	registerRows(userDeleteResultRows)
	registerRows(userInvitationRevokeResultRows)
	registerRows(betaAppReviewDetailsRows)
	registerRows(func(v *BetaAppReviewDetailResponse) ([]string, [][]string) {
		return betaAppReviewDetailsRows(&BetaAppReviewDetailsResponse{Data: []Resource[BetaAppReviewDetailAttributes]{v.Data}})
	})
	registerRows(betaAppReviewSubmissionsRows)
	registerRows(func(v *BetaAppReviewSubmissionResponse) ([]string, [][]string) {
		return betaAppReviewSubmissionsRows(&BetaAppReviewSubmissionsResponse{Data: []Resource[BetaAppReviewSubmissionAttributes]{v.Data}})
	})
	registerRows(buildBetaDetailsRows)
	registerRows(func(v *BuildBetaDetailResponse) ([]string, [][]string) {
		return buildBetaDetailsRows(&BuildBetaDetailsResponse{Data: []Resource[BuildBetaDetailAttributes]{v.Data}})
	})
	registerRows(betaLicenseAgreementsRows)
	registerRows(func(v *BetaLicenseAgreementResponse) ([]string, [][]string) {
		return betaLicenseAgreementsRows(&BetaLicenseAgreementsResponse{Data: []BetaLicenseAgreementResource{v.Data}})
	})
	registerRows(buildBetaNotificationRows)
	registerRows(ageRatingDeclarationRows)
	registerRows(accessibilityDeclarationsRows)
	registerRows(accessibilityDeclarationRows)
	registerRows(appStoreReviewDetailRows)
	registerRows(appStoreReviewAttachmentsRows)
	registerRows(appStoreReviewAttachmentRows)
	registerRows(appClipAppStoreReviewDetailRows)
	registerRows(routingAppCoverageRows)
	registerRows(appEncryptionDeclarationsRows)
	registerRows(appEncryptionDeclarationRows)
	registerRows(appEncryptionDeclarationDocumentRows)
	registerRows(betaRecruitmentCriterionOptionsRows)
	registerRows(betaRecruitmentCriteriaRows)
	registerRows(betaRecruitmentCriteriaDeleteResultRows)
	registerRows(func(v *Response[BetaGroupMetricAttributes]) ([]string, [][]string) {
		return betaGroupMetricsRows(v.Data)
	})
	registerRows(sandboxTestersRows)
	registerRows(func(v *SandboxTesterResponse) ([]string, [][]string) {
		return sandboxTestersRows(&SandboxTestersResponse{Data: []Resource[SandboxTesterAttributes]{v.Data}})
	})
	registerRows(bundleIDCapabilitiesRows)
	registerRows(func(v *BundleIDCapabilityResponse) ([]string, [][]string) {
		return bundleIDCapabilitiesRows(&BundleIDCapabilitiesResponse{Data: []Resource[BundleIDCapabilityAttributes]{v.Data}})
	})
	registerRows(localizationDownloadResultRows)
	registerRows(localizationUploadResultRows)
	registerDirect(func(v *BuildUploadResult, render func([]string, [][]string)) error {
		h, r := buildUploadResultRows(v)
		render(h, r)
		if len(v.Operations) > 0 {
			oh, or := buildUploadOperationsRows(v.Operations)
			render(oh, or)
		}
		return nil
	})
	registerRows(buildExpireAllResultRows)
	registerRows(appScreenshotListResultRows)
	registerRows(appPreviewListResultRows)
	registerDirect(func(v *AppScreenshotUploadResult, render func([]string, [][]string)) error {
		h, r := appScreenshotUploadResultMainRows(v)
		render(h, r)
		if len(v.Results) > 0 {
			ih, ir := assetUploadResultItemRows(v.Results)
			render(ih, ir)
		}
		return nil
	})
	registerDirect(func(v *AppPreviewUploadResult, render func([]string, [][]string)) error {
		h, r := appPreviewUploadResultMainRows(v)
		render(h, r)
		if len(v.Results) > 0 {
			ih, ir := assetUploadResultItemRows(v.Results)
			render(ih, ir)
		}
		return nil
	})
	registerRows(appClipAdvancedExperienceImageUploadResultRows)
	registerRows(appClipHeaderImageUploadResultRows)
	registerRows(assetDeleteResultRows)
	registerRows(appClipDefaultExperienceDeleteResultRows)
	registerRows(appClipDefaultExperienceLocalizationDeleteResultRows)
	registerRows(appClipAdvancedExperienceDeleteResultRows)
	registerRows(appClipAdvancedExperienceImageDeleteResultRows)
	registerRows(appClipHeaderImageDeleteResultRows)
	registerRows(betaAppClipInvocationDeleteResultRows)
	registerRows(betaAppClipInvocationLocalizationDeleteResultRows)
	registerRows(testFlightPublishResultRows)
	registerRows(appStorePublishResultRows)
	registerRows(salesReportResultRows)
	registerRows(financeReportResultRows)
	registerRows(financeRegionsRows)
	registerRows(analyticsReportRequestResultRows)
	registerRows(analyticsReportRequestDeleteResultRows)
	registerRows(analyticsReportRequestsRows)
	registerRows(func(v *AnalyticsReportRequestResponse) ([]string, [][]string) {
		return analyticsReportRequestsRows(&AnalyticsReportRequestsResponse{Data: []AnalyticsReportRequestResource{v.Data}, Links: v.Links})
	})
	registerRows(analyticsReportDownloadResultRows)
	registerRows(analyticsReportGetResultRows)
	registerRows(analyticsReportsRows)
	registerRows(func(v *AnalyticsReportResponse) ([]string, [][]string) {
		return analyticsReportsRows(&AnalyticsReportsResponse{Data: []Resource[AnalyticsReportAttributes]{v.Data}, Links: v.Links})
	})
	registerRows(analyticsReportInstancesRows)
	registerRows(func(v *AnalyticsReportInstanceResponse) ([]string, [][]string) {
		return analyticsReportInstancesRows(&AnalyticsReportInstancesResponse{Data: []Resource[AnalyticsReportInstanceAttributes]{v.Data}, Links: v.Links})
	})
	registerRows(analyticsReportSegmentsRows)
	registerRows(func(v *AnalyticsReportSegmentResponse) ([]string, [][]string) {
		return analyticsReportSegmentsRows(&AnalyticsReportSegmentsResponse{Data: []Resource[AnalyticsReportSegmentAttributes]{v.Data}, Links: v.Links})
	})
	registerRows(appStoreVersionSubmissionRows)
	registerRows(appStoreVersionSubmissionCreateRows)
	registerRows(appStoreVersionSubmissionStatusRows)
	registerRows(appStoreVersionSubmissionCancelRows)
	registerRows(appStoreVersionDetailRows)
	registerRows(appStoreVersionAttachBuildRows)
	registerRows(reviewSubmissionsRows)
	registerRows(func(v *ReviewSubmissionResponse) ([]string, [][]string) {
		return reviewSubmissionsRows(&ReviewSubmissionsResponse{Data: []ReviewSubmissionResource{v.Data}, Links: v.Links})
	})
	registerRows(reviewSubmissionItemsRows)
	registerRows(func(v *ReviewSubmissionItemResponse) ([]string, [][]string) {
		return reviewSubmissionItemsRows(&ReviewSubmissionItemsResponse{Data: []ReviewSubmissionItemResource{v.Data}, Links: v.Links})
	})
	registerRows(reviewSubmissionItemDeleteResultRows)
	registerRows(appStoreVersionReleaseRequestRows)
	registerRows(appStoreVersionPromotionCreateRows)
	registerRows(appStoreVersionPhasedReleaseRows)
	registerRows(appStoreVersionPhasedReleaseDeleteResultRows)
	registerRows(buildBetaGroupsUpdateRows)
	registerRows(buildIndividualTestersUpdateRows)
	registerRows(buildUploadDeleteResultRows)
	registerRows(inAppPurchaseDeleteResultRows)
	registerRows(appEventDeleteResultRows)
	registerRows(appEventLocalizationDeleteResultRows)
	registerRows(appEventSubmissionResultRows)
	registerRows(gameCenterAchievementsRows)
	registerRows(func(v *GameCenterAchievementResponse) ([]string, [][]string) {
		return gameCenterAchievementsRows(&GameCenterAchievementsResponse{Data: []Resource[GameCenterAchievementAttributes]{v.Data}})
	})
	registerRows(gameCenterAchievementDeleteResultRows)
	registerRows(gameCenterAchievementVersionsRows)
	registerRows(func(v *GameCenterAchievementVersionResponse) ([]string, [][]string) {
		return gameCenterAchievementVersionsRows(&GameCenterAchievementVersionsResponse{Data: []Resource[GameCenterAchievementVersionAttributes]{v.Data}})
	})
	registerRows(gameCenterLeaderboardsRows)
	registerRows(func(v *GameCenterLeaderboardResponse) ([]string, [][]string) {
		return gameCenterLeaderboardsRows(&GameCenterLeaderboardsResponse{Data: []Resource[GameCenterLeaderboardAttributes]{v.Data}})
	})
	registerRows(gameCenterLeaderboardDeleteResultRows)
	registerRows(gameCenterLeaderboardVersionsRows)
	registerRows(func(v *GameCenterLeaderboardVersionResponse) ([]string, [][]string) {
		return gameCenterLeaderboardVersionsRows(&GameCenterLeaderboardVersionsResponse{Data: []Resource[GameCenterLeaderboardVersionAttributes]{v.Data}})
	})
	registerRows(gameCenterLeaderboardSetsRows)
	registerRows(func(v *GameCenterLeaderboardSetResponse) ([]string, [][]string) {
		return gameCenterLeaderboardSetsRows(&GameCenterLeaderboardSetsResponse{Data: []Resource[GameCenterLeaderboardSetAttributes]{v.Data}})
	})
	registerRows(gameCenterLeaderboardSetDeleteResultRows)
	registerRows(gameCenterLeaderboardSetVersionsRows)
	registerRows(func(v *GameCenterLeaderboardSetVersionResponse) ([]string, [][]string) {
		return gameCenterLeaderboardSetVersionsRows(&GameCenterLeaderboardSetVersionsResponse{Data: []Resource[GameCenterLeaderboardSetVersionAttributes]{v.Data}})
	})
	registerRows(gameCenterLeaderboardLocalizationsRows)
	registerRows(func(v *GameCenterLeaderboardLocalizationResponse) ([]string, [][]string) {
		return gameCenterLeaderboardLocalizationsRows(&GameCenterLeaderboardLocalizationsResponse{Data: []Resource[GameCenterLeaderboardLocalizationAttributes]{v.Data}})
	})
	registerRows(gameCenterLeaderboardLocalizationDeleteResultRows)
	registerRows(gameCenterLeaderboardReleasesRows)
	registerRows(func(v *GameCenterLeaderboardReleaseResponse) ([]string, [][]string) {
		return gameCenterLeaderboardReleasesRows(&GameCenterLeaderboardReleasesResponse{Data: []Resource[GameCenterLeaderboardReleaseAttributes]{v.Data}})
	})
	registerRows(gameCenterLeaderboardReleaseDeleteResultRows)
	registerRows(gameCenterLeaderboardEntrySubmissionRows)
	registerRows(gameCenterPlayerAchievementSubmissionRows)
	registerRows(gameCenterLeaderboardSetReleasesRows)
	registerRows(func(v *GameCenterLeaderboardSetReleaseResponse) ([]string, [][]string) {
		return gameCenterLeaderboardSetReleasesRows(&GameCenterLeaderboardSetReleasesResponse{Data: []Resource[GameCenterLeaderboardSetReleaseAttributes]{v.Data}})
	})
	registerRows(gameCenterLeaderboardSetReleaseDeleteResultRows)
	registerRows(gameCenterLeaderboardSetLocalizationsRows)
	registerRows(func(v *GameCenterLeaderboardSetLocalizationResponse) ([]string, [][]string) {
		return gameCenterLeaderboardSetLocalizationsRows(&GameCenterLeaderboardSetLocalizationsResponse{Data: []Resource[GameCenterLeaderboardSetLocalizationAttributes]{v.Data}})
	})
	registerRows(gameCenterLeaderboardSetLocalizationDeleteResultRows)
	registerRows(gameCenterAchievementReleasesRows)
	registerRows(func(v *GameCenterAchievementReleaseResponse) ([]string, [][]string) {
		return gameCenterAchievementReleasesRows(&GameCenterAchievementReleasesResponse{Data: []Resource[GameCenterAchievementReleaseAttributes]{v.Data}})
	})
	registerRows(gameCenterAchievementReleaseDeleteResultRows)
	registerRows(gameCenterAchievementLocalizationsRows)
	registerRows(func(v *GameCenterAchievementLocalizationResponse) ([]string, [][]string) {
		return gameCenterAchievementLocalizationsRows(&GameCenterAchievementLocalizationsResponse{Data: []Resource[GameCenterAchievementLocalizationAttributes]{v.Data}})
	})
	registerRows(gameCenterAchievementLocalizationDeleteResultRows)
	registerRows(gameCenterLeaderboardImageUploadResultRows)
	registerRows(gameCenterLeaderboardImageDeleteResultRows)
	registerRows(gameCenterAchievementImageUploadResultRows)
	registerRows(gameCenterAchievementImageDeleteResultRows)
	registerRows(gameCenterLeaderboardSetImageUploadResultRows)
	registerRows(gameCenterLeaderboardSetImageDeleteResultRows)
	registerRows(gameCenterChallengesRows)
	registerRows(func(v *GameCenterChallengeResponse) ([]string, [][]string) {
		return gameCenterChallengesRows(&GameCenterChallengesResponse{Data: []Resource[GameCenterChallengeAttributes]{v.Data}})
	})
	registerRows(gameCenterChallengeDeleteResultRows)
	registerRows(gameCenterChallengeVersionsRows)
	registerRows(func(v *GameCenterChallengeVersionResponse) ([]string, [][]string) {
		return gameCenterChallengeVersionsRows(&GameCenterChallengeVersionsResponse{Data: []Resource[GameCenterChallengeVersionAttributes]{v.Data}})
	})
	registerRows(gameCenterChallengeLocalizationsRows)
	registerRows(func(v *GameCenterChallengeLocalizationResponse) ([]string, [][]string) {
		return gameCenterChallengeLocalizationsRows(&GameCenterChallengeLocalizationsResponse{Data: []Resource[GameCenterChallengeLocalizationAttributes]{v.Data}})
	})
	registerRows(gameCenterChallengeLocalizationDeleteResultRows)
	registerRows(gameCenterChallengeImagesRows)
	registerRows(func(v *GameCenterChallengeImageResponse) ([]string, [][]string) {
		return gameCenterChallengeImagesRows(&GameCenterChallengeImagesResponse{Data: []Resource[GameCenterChallengeImageAttributes]{v.Data}})
	})
	registerRows(gameCenterChallengeImageUploadResultRows)
	registerRows(gameCenterChallengeImageDeleteResultRows)
	registerRows(gameCenterChallengeReleasesRows)
	registerRows(func(v *GameCenterChallengeVersionReleaseResponse) ([]string, [][]string) {
		return gameCenterChallengeReleasesRows(&GameCenterChallengeVersionReleasesResponse{Data: []Resource[GameCenterChallengeVersionReleaseAttributes]{v.Data}})
	})
	registerRows(gameCenterChallengeReleaseDeleteResultRows)
	registerRows(gameCenterActivitiesRows)
	registerRows(func(v *GameCenterActivityResponse) ([]string, [][]string) {
		return gameCenterActivitiesRows(&GameCenterActivitiesResponse{Data: []Resource[GameCenterActivityAttributes]{v.Data}})
	})
	registerRows(gameCenterActivityDeleteResultRows)
	registerRows(gameCenterActivityVersionsRows)
	registerRows(func(v *GameCenterActivityVersionResponse) ([]string, [][]string) {
		return gameCenterActivityVersionsRows(&GameCenterActivityVersionsResponse{Data: []Resource[GameCenterActivityVersionAttributes]{v.Data}})
	})
	registerRows(gameCenterActivityLocalizationsRows)
	registerRows(func(v *GameCenterActivityLocalizationResponse) ([]string, [][]string) {
		return gameCenterActivityLocalizationsRows(&GameCenterActivityLocalizationsResponse{Data: []Resource[GameCenterActivityLocalizationAttributes]{v.Data}})
	})
	registerRows(gameCenterActivityLocalizationDeleteResultRows)
	registerRows(gameCenterActivityImagesRows)
	registerRows(func(v *GameCenterActivityImageResponse) ([]string, [][]string) {
		return gameCenterActivityImagesRows(&GameCenterActivityImagesResponse{Data: []Resource[GameCenterActivityImageAttributes]{v.Data}})
	})
	registerRows(gameCenterActivityImageUploadResultRows)
	registerRows(gameCenterActivityImageDeleteResultRows)
	registerRows(gameCenterActivityReleasesRows)
	registerRows(func(v *GameCenterActivityVersionReleaseResponse) ([]string, [][]string) {
		return gameCenterActivityReleasesRows(&GameCenterActivityVersionReleasesResponse{Data: []Resource[GameCenterActivityVersionReleaseAttributes]{v.Data}})
	})
	registerRows(gameCenterActivityReleaseDeleteResultRows)
	registerRows(gameCenterGroupsRows)
	registerRows(func(v *GameCenterGroupResponse) ([]string, [][]string) {
		return gameCenterGroupsRows(&GameCenterGroupsResponse{Data: []Resource[GameCenterGroupAttributes]{v.Data}})
	})
	registerRows(gameCenterGroupDeleteResultRows)
	registerRows(gameCenterAppVersionsRows)
	registerRows(func(v *GameCenterAppVersionResponse) ([]string, [][]string) {
		return gameCenterAppVersionsRows(&GameCenterAppVersionsResponse{Data: []Resource[GameCenterAppVersionAttributes]{v.Data}})
	})
	registerRows(gameCenterEnabledVersionsRows)
	registerRows(gameCenterDetailsRows)
	registerRows(func(v *GameCenterDetailResponse) ([]string, [][]string) {
		return gameCenterDetailsRows(&GameCenterDetailsResponse{Data: []Resource[GameCenterDetailAttributes]{v.Data}})
	})
	registerRows(gameCenterMatchmakingQueuesRows)
	registerRows(func(v *GameCenterMatchmakingQueueResponse) ([]string, [][]string) {
		return gameCenterMatchmakingQueuesRows(&GameCenterMatchmakingQueuesResponse{Data: []Resource[GameCenterMatchmakingQueueAttributes]{v.Data}})
	})
	registerRows(gameCenterMatchmakingQueueDeleteResultRows)
	registerRows(gameCenterMatchmakingRuleSetsRows)
	registerRows(func(v *GameCenterMatchmakingRuleSetResponse) ([]string, [][]string) {
		return gameCenterMatchmakingRuleSetsRows(&GameCenterMatchmakingRuleSetsResponse{Data: []Resource[GameCenterMatchmakingRuleSetAttributes]{v.Data}})
	})
	registerRows(gameCenterMatchmakingRuleSetDeleteResultRows)
	registerRows(gameCenterMatchmakingRulesRows)
	registerRows(func(v *GameCenterMatchmakingRuleResponse) ([]string, [][]string) {
		return gameCenterMatchmakingRulesRows(&GameCenterMatchmakingRulesResponse{Data: []Resource[GameCenterMatchmakingRuleAttributes]{v.Data}})
	})
	registerRows(gameCenterMatchmakingRuleDeleteResultRows)
	registerRows(gameCenterMatchmakingTeamsRows)
	registerRows(func(v *GameCenterMatchmakingTeamResponse) ([]string, [][]string) {
		return gameCenterMatchmakingTeamsRows(&GameCenterMatchmakingTeamsResponse{Data: []Resource[GameCenterMatchmakingTeamAttributes]{v.Data}})
	})
	registerRows(gameCenterMatchmakingTeamDeleteResultRows)
	registerRows(gameCenterMetricsRows)
	registerRows(gameCenterMatchmakingRuleSetTestRows)
	registerRows(subscriptionGroupDeleteResultRows)
	registerRows(subscriptionDeleteResultRows)
	registerRows(betaTesterDeleteResultRows)
	registerRows(betaTesterGroupsUpdateResultRows)
	registerRows(betaTesterAppsUpdateResultRows)
	registerRows(betaTesterBuildsUpdateResultRows)
	registerRows(appBetaTestersUpdateResultRows)
	registerRows(betaFeedbackSubmissionDeleteResultRows)
	registerRows(appStoreVersionLocalizationDeleteResultRows)
	registerRows(betaAppLocalizationDeleteResultRows)
	registerRows(betaBuildLocalizationDeleteResultRows)
	registerRows(betaTesterInvitationResultRows)
	registerRows(promotedPurchaseDeleteResultRows)
	registerRows(appPromotedPurchasesLinkResultRows)
	registerRows(sandboxTesterClearHistoryResultRows)
	registerRows(bundleIDDeleteResultRows)
	registerRows(marketplaceSearchDetailDeleteResultRows)
	registerRows(marketplaceWebhookDeleteResultRows)
	registerRows(webhookDeleteResultRows)
	registerRows(webhookPingRows)
	registerRows(merchantIDDeleteResultRows)
	registerRows(passTypeIDDeleteResultRows)
	registerRows(bundleIDCapabilityDeleteResultRows)
	registerRows(certificateRevokeResultRows)
	registerRows(profileDeleteResultRows)
	registerRows(endUserLicenseAgreementRows)
	registerRows(endUserLicenseAgreementDeleteResultRows)
	registerRows(profileDownloadResultRows)
	registerRows(signingFetchResultRows)
	registerRows(xcodeCloudRunResultRows)
	registerRows(xcodeCloudStatusResultRows)
	registerRows(ciProductsRows)
	registerRows(func(v *CiProductResponse) ([]string, [][]string) {
		return ciProductsRows(&CiProductsResponse{Data: []CiProductResource{v.Data}})
	})
	registerRows(ciWorkflowsRows)
	registerRows(func(v *CiWorkflowResponse) ([]string, [][]string) {
		return ciWorkflowsRows(&CiWorkflowsResponse{Data: []CiWorkflowResource{v.Data}})
	})
	registerRows(scmProvidersRows)
	registerRows(func(v *ScmProviderResponse) ([]string, [][]string) {
		return scmProvidersRows(&ScmProvidersResponse{Data: []ScmProviderResource{v.Data}, Links: v.Links})
	})
	registerRows(scmRepositoriesRows)
	registerRows(scmGitReferencesRows)
	registerRows(func(v *ScmGitReferenceResponse) ([]string, [][]string) {
		return scmGitReferencesRows(&ScmGitReferencesResponse{Data: []ScmGitReferenceResource{v.Data}, Links: v.Links})
	})
	registerRows(scmPullRequestsRows)
	registerRows(func(v *ScmPullRequestResponse) ([]string, [][]string) {
		return scmPullRequestsRows(&ScmPullRequestsResponse{Data: []ScmPullRequestResource{v.Data}, Links: v.Links})
	})
	registerRows(ciBuildRunsRows)
	registerRows(func(v *CiBuildRunResponse) ([]string, [][]string) {
		return ciBuildRunsRows(&CiBuildRunsResponse{Data: []CiBuildRunResource{v.Data}})
	})
	registerRows(ciBuildActionsRows)
	registerRows(func(v *CiBuildActionResponse) ([]string, [][]string) {
		return ciBuildActionsRows(&CiBuildActionsResponse{Data: []CiBuildActionResource{v.Data}})
	})
	registerRows(ciMacOsVersionsRows)
	registerRows(func(v *CiMacOsVersionResponse) ([]string, [][]string) {
		return ciMacOsVersionsRows(&CiMacOsVersionsResponse{Data: []CiMacOsVersionResource{v.Data}})
	})
	registerRows(ciXcodeVersionsRows)
	registerRows(func(v *CiXcodeVersionResponse) ([]string, [][]string) {
		return ciXcodeVersionsRows(&CiXcodeVersionsResponse{Data: []CiXcodeVersionResource{v.Data}})
	})
	registerRows(ciArtifactsRows)
	registerRows(func(v *CiArtifactResponse) ([]string, [][]string) {
		return ciArtifactsRows(&CiArtifactsResponse{Data: []CiArtifactResource{v.Data}})
	})
	registerRows(ciTestResultsRows)
	registerRows(func(v *CiTestResultResponse) ([]string, [][]string) {
		return ciTestResultsRows(&CiTestResultsResponse{Data: []CiTestResultResource{v.Data}})
	})
	registerRows(ciIssuesRows)
	registerRows(func(v *CiIssueResponse) ([]string, [][]string) {
		return ciIssuesRows(&CiIssuesResponse{Data: []CiIssueResource{v.Data}})
	})
	registerRows(ciArtifactDownloadResultRows)
	registerRows(ciWorkflowDeleteResultRows)
	registerRows(ciProductDeleteResultRows)
	registerRows(customerReviewResponseRows)
	registerRows(customerReviewResponseDeleteResultRows)
	registerRows(accessibilityDeclarationDeleteResultRows)
	registerRows(appStoreReviewAttachmentDeleteResultRows)
	registerRows(routingAppCoverageDeleteResultRows)
	registerRows(nominationDeleteResultRows)
	registerRows(appEncryptionDeclarationBuildsUpdateResultRows)
	registerRows(androidToIosAppMappingDetailsRows)
	registerRows(func(v *AndroidToIosAppMappingDetailResponse) ([]string, [][]string) {
		return androidToIosAppMappingDetailsRows(&AndroidToIosAppMappingDetailsResponse{Data: []Resource[AndroidToIosAppMappingDetailAttributes]{v.Data}})
	})
	registerRows(androidToIosAppMappingDeleteResultRows)
	registerRows(func(v *AlternativeDistributionDomainDeleteResult) ([]string, [][]string) {
		return alternativeDistributionDeleteResultRows(v.ID, v.Deleted)
	})
	registerRows(func(v *AlternativeDistributionKeyDeleteResult) ([]string, [][]string) {
		return alternativeDistributionDeleteResultRows(v.ID, v.Deleted)
	})
	registerRows(appCustomProductPagesRows)
	registerRows(func(v *AppCustomProductPageResponse) ([]string, [][]string) {
		return appCustomProductPagesRows(&AppCustomProductPagesResponse{Data: []Resource[AppCustomProductPageAttributes]{v.Data}})
	})
	registerRows(appCustomProductPageVersionsRows)
	registerRows(func(v *AppCustomProductPageVersionResponse) ([]string, [][]string) {
		return appCustomProductPageVersionsRows(&AppCustomProductPageVersionsResponse{Data: []Resource[AppCustomProductPageVersionAttributes]{v.Data}})
	})
	registerRows(appCustomProductPageLocalizationsRows)
	registerRows(func(v *AppCustomProductPageLocalizationResponse) ([]string, [][]string) {
		return appCustomProductPageLocalizationsRows(&AppCustomProductPageLocalizationsResponse{Data: []Resource[AppCustomProductPageLocalizationAttributes]{v.Data}})
	})
	registerRows(appKeywordsRows)
	registerRows(appStoreVersionExperimentsRows)
	registerRows(func(v *AppStoreVersionExperimentResponse) ([]string, [][]string) {
		return appStoreVersionExperimentsRows(&AppStoreVersionExperimentsResponse{Data: []Resource[AppStoreVersionExperimentAttributes]{v.Data}})
	})
	registerRows(appStoreVersionExperimentsV2Rows)
	registerRows(func(v *AppStoreVersionExperimentV2Response) ([]string, [][]string) {
		return appStoreVersionExperimentsV2Rows(&AppStoreVersionExperimentsV2Response{Data: []Resource[AppStoreVersionExperimentV2Attributes]{v.Data}})
	})
	registerRows(appStoreVersionExperimentTreatmentsRows)
	registerRows(func(v *AppStoreVersionExperimentTreatmentResponse) ([]string, [][]string) {
		return appStoreVersionExperimentTreatmentsRows(&AppStoreVersionExperimentTreatmentsResponse{Data: []Resource[AppStoreVersionExperimentTreatmentAttributes]{v.Data}})
	})
	registerRows(appStoreVersionExperimentTreatmentLocalizationsRows)
	registerRows(func(v *AppStoreVersionExperimentTreatmentLocalizationResponse) ([]string, [][]string) {
		return appStoreVersionExperimentTreatmentLocalizationsRows(&AppStoreVersionExperimentTreatmentLocalizationsResponse{Data: []Resource[AppStoreVersionExperimentTreatmentLocalizationAttributes]{v.Data}})
	})
	registerRows(appCustomProductPageDeleteResultRows)
	registerRows(appCustomProductPageLocalizationDeleteResultRows)
	registerRows(appStoreVersionExperimentDeleteResultRows)
	registerRows(appStoreVersionExperimentTreatmentDeleteResultRows)
	registerRows(appStoreVersionExperimentTreatmentLocalizationDeleteResultRows)
	registerRowsErr(perfPowerMetricsRows)
	registerRows(diagnosticSignaturesRows)
	registerRowsErr(diagnosticLogsRows)
	registerRows(performanceDownloadResultRows)
	registerRows(notarySubmissionStatusRows)
	registerRows(notarySubmissionsListRows)
	registerRows(notarySubmissionLogsRows)
}
