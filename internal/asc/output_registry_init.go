package asc

func singleLinkageRows(data ResourceData) ([]string, [][]string) {
	return linkagesRows(&LinkagesResponse{Data: []ResourceData{data}})
}

//nolint:gochecknoinits // registry init is the idiomatic way to populate a type map
func init() {
	registerRows(feedbackRows)
	registerRows(crashesRows)
	registerRows(reviewsRows)
	registerRows(customerReviewSummarizationsRows)
	registerRowsAdapter(func(v *CustomerReviewResponse) *ReviewsResponse {
		return &ReviewsResponse{Data: []Resource[ReviewAttributes]{v.Data}}
	}, reviewsRows)
	registerRows(appsRows)
	registerRows(appsWallRows)
	registerRows(appClipsRows)
	registerRows(appCategoriesRows)
	registerRowsAdapter(func(v *AppCategoryResponse) *AppCategoriesResponse {
		return &AppCategoriesResponse{Data: []AppCategory{v.Data}}
	}, appCategoriesRows)
	registerRows(appInfosRows)
	registerRowsAdapter(func(v *AppInfoResponse) *AppInfosResponse {
		return &AppInfosResponse{Data: []Resource[AppInfoAttributes]{v.Data}}
	}, appInfosRows)
	registerRowsAdapter(func(v *AppResponse) *AppsResponse {
		return &AppsResponse{Data: []Resource[AppAttributes]{v.Data}}
	}, appsRows)
	registerRowsAdapter(func(v *AppClipResponse) *AppClipsResponse {
		return &AppClipsResponse{Data: []Resource[AppClipAttributes]{v.Data}}
	}, appClipsRows)
	registerRows(appClipDefaultExperiencesRows)
	registerRowsAdapter(func(v *AppClipDefaultExperienceResponse) *AppClipDefaultExperiencesResponse {
		return &AppClipDefaultExperiencesResponse{Data: []Resource[AppClipDefaultExperienceAttributes]{v.Data}}
	}, appClipDefaultExperiencesRows)
	registerRows(appClipDefaultExperienceLocalizationsRows)
	registerRowsAdapter(func(v *AppClipDefaultExperienceLocalizationResponse) *AppClipDefaultExperienceLocalizationsResponse {
		return &AppClipDefaultExperienceLocalizationsResponse{Data: []Resource[AppClipDefaultExperienceLocalizationAttributes]{v.Data}}
	}, appClipDefaultExperienceLocalizationsRows)
	registerRows(appClipHeaderImageRows)
	registerRows(appClipAdvancedExperienceImageRows)
	registerRows(appClipAdvancedExperiencesRows)
	registerRowsAdapter(func(v *AppClipAdvancedExperienceResponse) *AppClipAdvancedExperiencesResponse {
		return &AppClipAdvancedExperiencesResponse{Data: []Resource[AppClipAdvancedExperienceAttributes]{v.Data}}
	}, appClipAdvancedExperiencesRows)
	registerRows(appSetupInfoResultRows)
	registerRows(appTagsRows)
	registerRowsAdapter(func(v *AppTagResponse) *AppTagsResponse {
		return &AppTagsResponse{Data: []Resource[AppTagAttributes]{v.Data}}
	}, appTagsRows)
	registerRows(marketplaceSearchDetailsRows)
	registerRowsAdapter(func(v *MarketplaceSearchDetailResponse) *MarketplaceSearchDetailsResponse {
		return &MarketplaceSearchDetailsResponse{Data: []Resource[MarketplaceSearchDetailAttributes]{v.Data}}
	}, marketplaceSearchDetailsRows)
	registerRows(marketplaceWebhooksRows)
	registerRowsAdapter(func(v *MarketplaceWebhookResponse) *MarketplaceWebhooksResponse {
		return &MarketplaceWebhooksResponse{Data: []Resource[MarketplaceWebhookAttributes]{v.Data}}
	}, marketplaceWebhooksRows)
	registerRows(webhooksRows)
	registerRowsAdapter(func(v *WebhookResponse) *WebhooksResponse {
		return &WebhooksResponse{Data: []Resource[WebhookAttributes]{v.Data}}
	}, webhooksRows)
	registerRows(webhookDeliveriesRows)
	registerRowsAdapter(func(v *WebhookDeliveryResponse) *WebhookDeliveriesResponse {
		return &WebhookDeliveriesResponse{Data: []Resource[WebhookDeliveryAttributes]{v.Data}}
	}, webhookDeliveriesRows)
	registerRows(alternativeDistributionDomainsRows)
	registerRowsAdapter(func(v *AlternativeDistributionDomainResponse) *AlternativeDistributionDomainsResponse {
		return &AlternativeDistributionDomainsResponse{Data: []Resource[AlternativeDistributionDomainAttributes]{v.Data}}
	}, alternativeDistributionDomainsRows)
	registerRows(alternativeDistributionKeysRows)
	registerRowsAdapter(func(v *AlternativeDistributionKeyResponse) *AlternativeDistributionKeysResponse {
		return &AlternativeDistributionKeysResponse{Data: []Resource[AlternativeDistributionKeyAttributes]{v.Data}}
	}, alternativeDistributionKeysRows)
	registerRows(alternativeDistributionPackageRows)
	registerRows(alternativeDistributionPackageVersionsRows)
	registerRowsAdapter(func(v *AlternativeDistributionPackageVersionResponse) *AlternativeDistributionPackageVersionsResponse {
		return &AlternativeDistributionPackageVersionsResponse{Data: []Resource[AlternativeDistributionPackageVersionAttributes]{v.Data}}
	}, alternativeDistributionPackageVersionsRows)
	registerRows(alternativeDistributionPackageVariantsRows)
	registerRowsAdapter(func(v *AlternativeDistributionPackageVariantResponse) *AlternativeDistributionPackageVariantsResponse {
		return &AlternativeDistributionPackageVariantsResponse{Data: []Resource[AlternativeDistributionPackageVariantAttributes]{v.Data}}
	}, alternativeDistributionPackageVariantsRows)
	registerRows(alternativeDistributionPackageDeltasRows)
	registerRowsAdapter(func(v *AlternativeDistributionPackageDeltaResponse) *AlternativeDistributionPackageDeltasResponse {
		return &AlternativeDistributionPackageDeltasResponse{Data: []Resource[AlternativeDistributionPackageDeltaAttributes]{v.Data}}
	}, alternativeDistributionPackageDeltasRows)
	registerRows(backgroundAssetsRows)
	registerRowsAdapter(func(v *BackgroundAssetResponse) *BackgroundAssetsResponse {
		return &BackgroundAssetsResponse{Data: []Resource[BackgroundAssetAttributes]{v.Data}}
	}, backgroundAssetsRows)
	registerRows(backgroundAssetVersionsRows)
	registerRowsAdapter(func(v *BackgroundAssetVersionResponse) *BackgroundAssetVersionsResponse {
		return &BackgroundAssetVersionsResponse{Data: []Resource[BackgroundAssetVersionAttributes]{v.Data}}
	}, backgroundAssetVersionsRows)
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
	registerRowsAdapter(func(v *BackgroundAssetUploadFileResponse) *BackgroundAssetUploadFilesResponse {
		return &BackgroundAssetUploadFilesResponse{Data: []Resource[BackgroundAssetUploadFileAttributes]{v.Data}}
	}, backgroundAssetUploadFilesRows)
	registerRows(nominationsRows)
	registerRowsAdapter(func(v *NominationResponse) *NominationsResponse {
		return &NominationsResponse{Data: []Resource[NominationAttributes]{v.Data}}
	}, nominationsRows)
	registerRows(linkagesRows)
	registerRows(func(v *AppClipDefaultExperienceReviewDetailLinkageResponse) ([]string, [][]string) {
		return singleLinkageRows(v.Data)
	})
	registerRows(func(v *AppClipDefaultExperienceReleaseWithAppStoreVersionLinkageResponse) ([]string, [][]string) {
		return singleLinkageRows(v.Data)
	})
	registerRows(func(v *AppClipDefaultExperienceLocalizationHeaderImageLinkageResponse) ([]string, [][]string) {
		return singleLinkageRows(v.Data)
	})
	registerRows(func(v *AppStoreVersionAgeRatingDeclarationLinkageResponse) ([]string, [][]string) {
		return singleLinkageRows(v.Data)
	})
	registerRows(func(v *AppStoreVersionReviewDetailLinkageResponse) ([]string, [][]string) {
		return singleLinkageRows(v.Data)
	})
	registerRows(func(v *AppStoreVersionAppClipDefaultExperienceLinkageResponse) ([]string, [][]string) {
		return singleLinkageRows(v.Data)
	})
	registerRows(func(v *AppStoreVersionSubmissionLinkageResponse) ([]string, [][]string) {
		return singleLinkageRows(v.Data)
	})
	registerRows(func(v *AppStoreVersionRoutingAppCoverageLinkageResponse) ([]string, [][]string) {
		return singleLinkageRows(v.Data)
	})
	registerRows(func(v *AppStoreVersionAlternativeDistributionPackageLinkageResponse) ([]string, [][]string) {
		return singleLinkageRows(v.Data)
	})
	registerRows(func(v *AppStoreVersionGameCenterAppVersionLinkageResponse) ([]string, [][]string) {
		return singleLinkageRows(v.Data)
	})
	registerRows(func(v *BuildAppLinkageResponse) ([]string, [][]string) {
		return singleLinkageRows(v.Data)
	})
	registerRows(func(v *BuildAppStoreVersionLinkageResponse) ([]string, [][]string) {
		return singleLinkageRows(v.Data)
	})
	registerRows(func(v *BuildBuildBetaDetailLinkageResponse) ([]string, [][]string) {
		return singleLinkageRows(v.Data)
	})
	registerRows(func(v *BuildPreReleaseVersionLinkageResponse) ([]string, [][]string) {
		return singleLinkageRows(v.Data)
	})
	registerRows(func(v *PreReleaseVersionAppLinkageResponse) ([]string, [][]string) {
		return singleLinkageRows(v.Data)
	})
	registerRows(func(v *AppInfoAgeRatingDeclarationLinkageResponse) ([]string, [][]string) {
		return singleLinkageRows(v.Data)
	})
	registerRows(func(v *AppInfoPrimaryCategoryLinkageResponse) ([]string, [][]string) {
		return singleLinkageRows(v.Data)
	})
	registerRows(func(v *AppInfoPrimarySubcategoryOneLinkageResponse) ([]string, [][]string) {
		return singleLinkageRows(v.Data)
	})
	registerRows(func(v *AppInfoPrimarySubcategoryTwoLinkageResponse) ([]string, [][]string) {
		return singleLinkageRows(v.Data)
	})
	registerRows(func(v *AppInfoSecondaryCategoryLinkageResponse) ([]string, [][]string) {
		return singleLinkageRows(v.Data)
	})
	registerRows(func(v *AppInfoSecondarySubcategoryOneLinkageResponse) ([]string, [][]string) {
		return singleLinkageRows(v.Data)
	})
	registerRows(func(v *AppInfoSecondarySubcategoryTwoLinkageResponse) ([]string, [][]string) {
		return singleLinkageRows(v.Data)
	})
	registerRows(bundleIDsRows)
	registerRowsAdapter(func(v *BundleIDResponse) *BundleIDsResponse {
		return &BundleIDsResponse{Data: []Resource[BundleIDAttributes]{v.Data}}
	}, bundleIDsRows)
	registerRows(merchantIDsRows)
	registerRowsAdapter(func(v *MerchantIDResponse) *MerchantIDsResponse {
		return &MerchantIDsResponse{Data: []Resource[MerchantIDAttributes]{v.Data}}
	}, merchantIDsRows)
	registerRows(passTypeIDsRows)
	registerRowsAdapter(func(v *PassTypeIDResponse) *PassTypeIDsResponse {
		return &PassTypeIDsResponse{Data: []Resource[PassTypeIDAttributes]{v.Data}}
	}, passTypeIDsRows)
	registerRows(certificatesRows)
	registerRowsAdapter(func(v *CertificateResponse) *CertificatesResponse {
		return &CertificatesResponse{Data: []Resource[CertificateAttributes]{v.Data}}
	}, certificatesRows)
	registerRows(profilesRows)
	registerRowsAdapter(func(v *ProfileResponse) *ProfilesResponse {
		return &ProfilesResponse{Data: []Resource[ProfileAttributes]{v.Data}}
	}, profilesRows)
	registerRows(legacyInAppPurchasesRows)
	registerRowsAdapter(func(v *InAppPurchaseResponse) *InAppPurchasesResponse {
		return &InAppPurchasesResponse{Data: []Resource[InAppPurchaseAttributes]{v.Data}}
	}, legacyInAppPurchasesRows)
	registerRows(inAppPurchasesRows)
	registerRowsAdapter(func(v *InAppPurchaseV2Response) *InAppPurchasesV2Response {
		return &InAppPurchasesV2Response{Data: []Resource[InAppPurchaseV2Attributes]{v.Data}}
	}, inAppPurchasesRows)
	registerRows(inAppPurchaseLocalizationsRows)
	registerRowsAdapter(func(v *InAppPurchaseLocalizationResponse) *InAppPurchaseLocalizationsResponse {
		return &InAppPurchaseLocalizationsResponse{Data: []Resource[InAppPurchaseLocalizationAttributes]{v.Data}}
	}, inAppPurchaseLocalizationsRows)
	registerRows(inAppPurchaseImagesRows)
	registerRowsAdapter(func(v *InAppPurchaseImageResponse) *InAppPurchaseImagesResponse {
		return &InAppPurchaseImagesResponse{Data: []Resource[InAppPurchaseImageAttributes]{v.Data}}
	}, inAppPurchaseImagesRows)
	registerRows(inAppPurchasePricePointsRows)
	registerRowsErr(inAppPurchasePricesRows)
	registerRowsErr(inAppPurchaseOfferCodePricesRows)
	registerRows(inAppPurchaseOfferCodesRows)
	registerRowsAdapter(func(v *InAppPurchaseOfferCodeResponse) *InAppPurchaseOfferCodesResponse {
		return &InAppPurchaseOfferCodesResponse{Data: []Resource[InAppPurchaseOfferCodeAttributes]{v.Data}}
	}, inAppPurchaseOfferCodesRows)
	registerRows(inAppPurchaseOfferCodeCustomCodesRows)
	registerRowsAdapter(func(v *InAppPurchaseOfferCodeCustomCodeResponse) *InAppPurchaseOfferCodeCustomCodesResponse {
		return &InAppPurchaseOfferCodeCustomCodesResponse{Data: []Resource[InAppPurchaseOfferCodeCustomCodeAttributes]{v.Data}}
	}, inAppPurchaseOfferCodeCustomCodesRows)
	registerRows(inAppPurchaseOfferCodeOneTimeUseCodesRows)
	registerRowsAdapter(func(v *InAppPurchaseOfferCodeOneTimeUseCodeResponse) *InAppPurchaseOfferCodeOneTimeUseCodesResponse {
		return &InAppPurchaseOfferCodeOneTimeUseCodesResponse{Data: []Resource[InAppPurchaseOfferCodeOneTimeUseCodeAttributes]{v.Data}}
	}, inAppPurchaseOfferCodeOneTimeUseCodesRows)
	registerRows(inAppPurchaseAvailabilityRows)
	registerRows(inAppPurchaseContentRows)
	registerRows(inAppPurchasePriceScheduleRows)
	registerRows(inAppPurchaseReviewScreenshotRows)
	registerRows(appEventsRows)
	registerRowsAdapter(func(v *AppEventResponse) *AppEventsResponse {
		return &AppEventsResponse{Data: []Resource[AppEventAttributes]{v.Data}}
	}, appEventsRows)
	registerRows(appEventLocalizationsRows)
	registerRowsAdapter(func(v *AppEventLocalizationResponse) *AppEventLocalizationsResponse {
		return &AppEventLocalizationsResponse{Data: []Resource[AppEventLocalizationAttributes]{v.Data}}
	}, appEventLocalizationsRows)
	registerRows(appEventScreenshotsRows)
	registerRowsAdapter(func(v *AppEventScreenshotResponse) *AppEventScreenshotsResponse {
		return &AppEventScreenshotsResponse{Data: []Resource[AppEventScreenshotAttributes]{v.Data}}
	}, appEventScreenshotsRows)
	registerRows(appEventVideoClipsRows)
	registerRowsAdapter(func(v *AppEventVideoClipResponse) *AppEventVideoClipsResponse {
		return &AppEventVideoClipsResponse{Data: []Resource[AppEventVideoClipAttributes]{v.Data}}
	}, appEventVideoClipsRows)
	registerRows(subscriptionGroupsRows)
	registerRowsAdapter(func(v *SubscriptionGroupResponse) *SubscriptionGroupsResponse {
		return &SubscriptionGroupsResponse{Data: []Resource[SubscriptionGroupAttributes]{v.Data}}
	}, subscriptionGroupsRows)
	registerRows(subscriptionsRows)
	registerRowsAdapter(func(v *SubscriptionResponse) *SubscriptionsResponse {
		return &SubscriptionsResponse{Data: []Resource[SubscriptionAttributes]{v.Data}}
	}, subscriptionsRows)
	registerRows(promotedPurchasesRows)
	registerRowsAdapter(func(v *PromotedPurchaseResponse) *PromotedPurchasesResponse {
		return &PromotedPurchasesResponse{Data: []Resource[PromotedPurchaseAttributes]{v.Data}}
	}, promotedPurchasesRows)
	registerRowsErr(subscriptionPricesRows)
	registerRows(subscriptionPriceRows)
	registerRows(subscriptionAvailabilityRows)
	registerRows(subscriptionGracePeriodRows)
	registerRows(territoriesRows)
	registerRowsAdapter(func(v *TerritoryResponse) *TerritoriesResponse {
		return &TerritoriesResponse{Data: []Resource[TerritoryAttributes]{v.Data}}
	}, territoriesRows)
	registerRowsErr(territoryAgeRatingsRows)
	registerRows(offerCodeValuesRows)
	registerRows(appPricePointsRows)
	registerRows(appPriceScheduleRows)
	registerRows(appPricesRows)
	registerRows(buildsRows)
	registerRows(buildBundlesRows)
	registerRows(buildBundleFileSizesRows)
	registerRows(betaAppClipInvocationsRows)
	registerRowsAdapter(func(v *BetaAppClipInvocationResponse) *BetaAppClipInvocationsResponse {
		return &BetaAppClipInvocationsResponse{Data: []Resource[BetaAppClipInvocationAttributes]{v.Data}}
	}, betaAppClipInvocationsRows)
	registerRows(betaAppClipInvocationLocalizationsRows)
	registerRowsAdapter(func(v *BetaAppClipInvocationLocalizationResponse) *BetaAppClipInvocationLocalizationsResponse {
		return &BetaAppClipInvocationLocalizationsResponse{Data: []Resource[BetaAppClipInvocationLocalizationAttributes]{v.Data}}
	}, betaAppClipInvocationLocalizationsRows)
	registerRows(offerCodesRows)
	registerRows(offerCodeCustomCodesRows)
	registerRows(subscriptionOfferCodeRows)
	registerRows(winBackOffersRows)
	registerRowsAdapter(func(v *WinBackOfferResponse) *WinBackOffersResponse {
		return &WinBackOffersResponse{Data: []Resource[WinBackOfferAttributes]{v.Data}}
	}, winBackOffersRows)
	registerRowsErr(winBackOfferPricesRows)
	registerRows(appStoreVersionsRows)
	registerRowsAdapter(func(v *AppStoreVersionResponse) *AppStoreVersionsResponse {
		return &AppStoreVersionsResponse{Data: []Resource[AppStoreVersionAttributes]{v.Data}}
	}, appStoreVersionsRows)
	registerRows(preReleaseVersionsRows)
	registerRowsAdapter(func(v *BuildResponse) *BuildsResponse {
		return &BuildsResponse{Data: []Resource[BuildAttributes]{v.Data}}
	}, buildsRows)
	registerRows(buildIconsRows)
	registerRows(buildUploadsRows)
	registerRows(buildsLatestNextRows)
	registerRowsAdapter(func(v *BuildUploadResponse) *BuildUploadsResponse {
		return &BuildUploadsResponse{Data: []Resource[BuildUploadAttributes]{v.Data}}
	}, buildUploadsRows)
	registerRows(buildUploadFilesRows)
	registerRowsAdapter(func(v *BuildUploadFileResponse) *BuildUploadFilesResponse {
		return &BuildUploadFilesResponse{Data: []Resource[BuildUploadFileAttributes]{v.Data}}
	}, buildUploadFilesRows)
	registerDirect(func(v *AppClipDomainStatusResult, render func([]string, [][]string)) error {
		h, r := appClipDomainStatusMainRows(v)
		render(h, r)
		if len(v.Domains) > 0 {
			dh, dr := appClipDomainStatusDomainRows(v)
			render(dh, dr)
		}
		return nil
	})
	registerRowsAdapter(func(v *SubscriptionOfferCodeOneTimeUseCodeResponse) *SubscriptionOfferCodeOneTimeUseCodesResponse {
		return &SubscriptionOfferCodeOneTimeUseCodesResponse{Data: []Resource[SubscriptionOfferCodeOneTimeUseCodeAttributes]{v.Data}}
	}, offerCodesRows)
	registerRowsAdapter(func(v *SubscriptionOfferCodeCustomCodeResponse) *SubscriptionOfferCodeCustomCodesResponse {
		return &SubscriptionOfferCodeCustomCodesResponse{Data: []Resource[SubscriptionOfferCodeCustomCodeAttributes]{v.Data}}
	}, offerCodeCustomCodesRows)
	registerRows(winBackOfferDeleteResultRows)
	registerRows(subscriptionPriceDeleteResultRows)
	registerRowsErr(offerCodePricesRows)
	registerRows(appAvailabilityRows)
	registerRows(territoryAvailabilitiesRows)
	registerRows(endAppAvailabilityPreOrderRows)
	registerRowsAdapter(func(v *PreReleaseVersionResponse) *PreReleaseVersionsResponse {
		return &PreReleaseVersionsResponse{Data: []PreReleaseVersion{v.Data}}
	}, preReleaseVersionsRows)
	registerRows(appStoreVersionLocalizationsRows)
	registerRowsAdapter(func(v *AppStoreVersionLocalizationResponse) *AppStoreVersionLocalizationsResponse {
		return &AppStoreVersionLocalizationsResponse{Data: []Resource[AppStoreVersionLocalizationAttributes]{v.Data}}
	}, appStoreVersionLocalizationsRows)
	registerRows(betaAppLocalizationsRows)
	registerRowsAdapter(func(v *BetaAppLocalizationResponse) *BetaAppLocalizationsResponse {
		return &BetaAppLocalizationsResponse{Data: []Resource[BetaAppLocalizationAttributes]{v.Data}}
	}, betaAppLocalizationsRows)
	registerRows(betaBuildLocalizationsRows)
	registerRowsAdapter(func(v *BetaBuildLocalizationResponse) *BetaBuildLocalizationsResponse {
		return &BetaBuildLocalizationsResponse{Data: []Resource[BetaBuildLocalizationAttributes]{v.Data}}
	}, betaBuildLocalizationsRows)
	registerRows(appInfoLocalizationsRows)
	registerRows(appScreenshotSetsRows)
	registerRowsAdapter(func(v *AppScreenshotSetResponse) *AppScreenshotSetsResponse {
		return &AppScreenshotSetsResponse{Data: []Resource[AppScreenshotSetAttributes]{v.Data}}
	}, appScreenshotSetsRows)
	registerRows(appScreenshotsRows)
	registerRowsAdapter(func(v *AppScreenshotResponse) *AppScreenshotsResponse {
		return &AppScreenshotsResponse{Data: []Resource[AppScreenshotAttributes]{v.Data}}
	}, appScreenshotsRows)
	registerRows(appPreviewSetsRows)
	registerRowsAdapter(func(v *AppPreviewSetResponse) *AppPreviewSetsResponse {
		return &AppPreviewSetsResponse{Data: []Resource[AppPreviewSetAttributes]{v.Data}}
	}, appPreviewSetsRows)
	registerRows(appPreviewsRows)
	registerRowsAdapter(func(v *AppPreviewResponse) *AppPreviewsResponse {
		return &AppPreviewsResponse{Data: []Resource[AppPreviewAttributes]{v.Data}}
	}, appPreviewsRows)
	registerRows(betaGroupsRows)
	registerRowsAdapter(func(v *BetaGroupResponse) *BetaGroupsResponse {
		return &BetaGroupsResponse{Data: []Resource[BetaGroupAttributes]{v.Data}}
	}, betaGroupsRows)
	registerRows(betaTestersRows)
	registerRowsAdapter(func(v *BetaTesterResponse) *BetaTestersResponse {
		return &BetaTestersResponse{Data: []Resource[BetaTesterAttributes]{v.Data}}
	}, betaTestersRows)
	registerRows(usersRows)
	registerRowsAdapter(func(v *UserResponse) *UsersResponse {
		return &UsersResponse{Data: []Resource[UserAttributes]{v.Data}}
	}, usersRows)
	registerRows(actorsRows)
	registerRowsAdapter(func(v *ActorResponse) *ActorsResponse {
		return &ActorsResponse{Data: []Resource[ActorAttributes]{v.Data}}
	}, actorsRows)
	registerRows(devicesRows)
	registerRows(deviceLocalUDIDRows)
	registerRowsAdapter(func(v *DeviceResponse) *DevicesResponse {
		return &DevicesResponse{Data: []Resource[DeviceAttributes]{v.Data}}
	}, devicesRows)
	registerRows(userInvitationsRows)
	registerRowsAdapter(func(v *UserInvitationResponse) *UserInvitationsResponse {
		return &UserInvitationsResponse{Data: []Resource[UserInvitationAttributes]{v.Data}}
	}, userInvitationsRows)
	registerRows(userDeleteResultRows)
	registerRows(userInvitationRevokeResultRows)
	registerRows(betaAppReviewDetailsRows)
	registerRowsAdapter(func(v *BetaAppReviewDetailResponse) *BetaAppReviewDetailsResponse {
		return &BetaAppReviewDetailsResponse{Data: []Resource[BetaAppReviewDetailAttributes]{v.Data}}
	}, betaAppReviewDetailsRows)
	registerRows(betaAppReviewSubmissionsRows)
	registerRowsAdapter(func(v *BetaAppReviewSubmissionResponse) *BetaAppReviewSubmissionsResponse {
		return &BetaAppReviewSubmissionsResponse{Data: []Resource[BetaAppReviewSubmissionAttributes]{v.Data}}
	}, betaAppReviewSubmissionsRows)
	registerRows(buildBetaDetailsRows)
	registerRowsAdapter(func(v *BuildBetaDetailResponse) *BuildBetaDetailsResponse {
		return &BuildBetaDetailsResponse{Data: []Resource[BuildBetaDetailAttributes]{v.Data}}
	}, buildBetaDetailsRows)
	registerRows(betaLicenseAgreementsRows)
	registerRowsAdapter(func(v *BetaLicenseAgreementResponse) *BetaLicenseAgreementsResponse {
		return &BetaLicenseAgreementsResponse{Data: []BetaLicenseAgreementResource{v.Data}}
	}, betaLicenseAgreementsRows)
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
	registerRowsAdapter(func(v *SandboxTesterResponse) *SandboxTestersResponse {
		return &SandboxTestersResponse{Data: []Resource[SandboxTesterAttributes]{v.Data}}
	}, sandboxTestersRows)
	registerRows(bundleIDCapabilitiesRows)
	registerRowsAdapter(func(v *BundleIDCapabilityResponse) *BundleIDCapabilitiesResponse {
		return &BundleIDCapabilitiesResponse{Data: []Resource[BundleIDCapabilityAttributes]{v.Data}}
	}, bundleIDCapabilitiesRows)
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
	registerRows(screenshotSizesRows)
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
	registerRowsAdapter(func(v *AnalyticsReportRequestResponse) *AnalyticsReportRequestsResponse {
		return &AnalyticsReportRequestsResponse{Data: []AnalyticsReportRequestResource{v.Data}, Links: v.Links}
	}, analyticsReportRequestsRows)
	registerRows(analyticsReportDownloadResultRows)
	registerRows(analyticsReportGetResultRows)
	registerRows(analyticsReportsRows)
	registerRowsAdapter(func(v *AnalyticsReportResponse) *AnalyticsReportsResponse {
		return &AnalyticsReportsResponse{Data: []Resource[AnalyticsReportAttributes]{v.Data}, Links: v.Links}
	}, analyticsReportsRows)
	registerRows(analyticsReportInstancesRows)
	registerRowsAdapter(func(v *AnalyticsReportInstanceResponse) *AnalyticsReportInstancesResponse {
		return &AnalyticsReportInstancesResponse{Data: []Resource[AnalyticsReportInstanceAttributes]{v.Data}, Links: v.Links}
	}, analyticsReportInstancesRows)
	registerRows(analyticsReportSegmentsRows)
	registerRowsAdapter(func(v *AnalyticsReportSegmentResponse) *AnalyticsReportSegmentsResponse {
		return &AnalyticsReportSegmentsResponse{Data: []Resource[AnalyticsReportSegmentAttributes]{v.Data}, Links: v.Links}
	}, analyticsReportSegmentsRows)
	registerRows(appStoreVersionSubmissionRows)
	registerRows(appStoreVersionSubmissionCreateRows)
	registerRows(appStoreVersionSubmissionStatusRows)
	registerRows(appStoreVersionSubmissionCancelRows)
	registerRows(appStoreVersionDetailRows)
	registerRows(appStoreVersionAttachBuildRows)
	registerRows(reviewSubmissionsRows)
	registerRowsAdapter(func(v *ReviewSubmissionResponse) *ReviewSubmissionsResponse {
		return &ReviewSubmissionsResponse{Data: []ReviewSubmissionResource{v.Data}, Links: v.Links}
	}, reviewSubmissionsRows)
	registerRows(reviewSubmissionItemsRows)
	registerRowsAdapter(func(v *ReviewSubmissionItemResponse) *ReviewSubmissionItemsResponse {
		return &ReviewSubmissionItemsResponse{Data: []ReviewSubmissionItemResource{v.Data}, Links: v.Links}
	}, reviewSubmissionItemsRows)
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
	registerRowsAdapter(func(v *CiProductResponse) *CiProductsResponse {
		return &CiProductsResponse{Data: []CiProductResource{v.Data}}
	}, ciProductsRows)
	registerRows(ciWorkflowsRows)
	registerRowsAdapter(func(v *CiWorkflowResponse) *CiWorkflowsResponse {
		return &CiWorkflowsResponse{Data: []CiWorkflowResource{v.Data}}
	}, ciWorkflowsRows)
	registerRows(scmProvidersRows)
	registerRowsAdapter(func(v *ScmProviderResponse) *ScmProvidersResponse {
		return &ScmProvidersResponse{Data: []ScmProviderResource{v.Data}, Links: v.Links}
	}, scmProvidersRows)
	registerRows(scmRepositoriesRows)
	registerRows(scmGitReferencesRows)
	registerRowsAdapter(func(v *ScmGitReferenceResponse) *ScmGitReferencesResponse {
		return &ScmGitReferencesResponse{Data: []ScmGitReferenceResource{v.Data}, Links: v.Links}
	}, scmGitReferencesRows)
	registerRows(scmPullRequestsRows)
	registerRowsAdapter(func(v *ScmPullRequestResponse) *ScmPullRequestsResponse {
		return &ScmPullRequestsResponse{Data: []ScmPullRequestResource{v.Data}, Links: v.Links}
	}, scmPullRequestsRows)
	registerRows(ciBuildRunsRows)
	registerRowsAdapter(func(v *CiBuildRunResponse) *CiBuildRunsResponse {
		return &CiBuildRunsResponse{Data: []CiBuildRunResource{v.Data}}
	}, ciBuildRunsRows)
	registerRows(ciBuildActionsRows)
	registerRowsAdapter(func(v *CiBuildActionResponse) *CiBuildActionsResponse {
		return &CiBuildActionsResponse{Data: []CiBuildActionResource{v.Data}}
	}, ciBuildActionsRows)
	registerRows(ciMacOsVersionsRows)
	registerRowsAdapter(func(v *CiMacOsVersionResponse) *CiMacOsVersionsResponse {
		return &CiMacOsVersionsResponse{Data: []CiMacOsVersionResource{v.Data}}
	}, ciMacOsVersionsRows)
	registerRows(ciXcodeVersionsRows)
	registerRowsAdapter(func(v *CiXcodeVersionResponse) *CiXcodeVersionsResponse {
		return &CiXcodeVersionsResponse{Data: []CiXcodeVersionResource{v.Data}}
	}, ciXcodeVersionsRows)
	registerRows(ciArtifactsRows)
	registerRowsAdapter(func(v *CiArtifactResponse) *CiArtifactsResponse {
		return &CiArtifactsResponse{Data: []CiArtifactResource{v.Data}}
	}, ciArtifactsRows)
	registerRows(ciTestResultsRows)
	registerRowsAdapter(func(v *CiTestResultResponse) *CiTestResultsResponse {
		return &CiTestResultsResponse{Data: []CiTestResultResource{v.Data}}
	}, ciTestResultsRows)
	registerRows(ciIssuesRows)
	registerRowsAdapter(func(v *CiIssueResponse) *CiIssuesResponse {
		return &CiIssuesResponse{Data: []CiIssueResource{v.Data}}
	}, ciIssuesRows)
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
	registerRowsAdapter(func(v *AndroidToIosAppMappingDetailResponse) *AndroidToIosAppMappingDetailsResponse {
		return &AndroidToIosAppMappingDetailsResponse{Data: []Resource[AndroidToIosAppMappingDetailAttributes]{v.Data}}
	}, androidToIosAppMappingDetailsRows)
	registerRows(androidToIosAppMappingDeleteResultRows)
	registerRows(func(v *AlternativeDistributionDomainDeleteResult) ([]string, [][]string) {
		return alternativeDistributionDeleteResultRows(v.ID, v.Deleted)
	})
	registerRows(func(v *AlternativeDistributionKeyDeleteResult) ([]string, [][]string) {
		return alternativeDistributionDeleteResultRows(v.ID, v.Deleted)
	})
	registerRows(appCustomProductPagesRows)
	registerRowsAdapter(func(v *AppCustomProductPageResponse) *AppCustomProductPagesResponse {
		return &AppCustomProductPagesResponse{Data: []Resource[AppCustomProductPageAttributes]{v.Data}}
	}, appCustomProductPagesRows)
	registerRows(appCustomProductPageVersionsRows)
	registerRowsAdapter(func(v *AppCustomProductPageVersionResponse) *AppCustomProductPageVersionsResponse {
		return &AppCustomProductPageVersionsResponse{Data: []Resource[AppCustomProductPageVersionAttributes]{v.Data}}
	}, appCustomProductPageVersionsRows)
	registerRows(appCustomProductPageLocalizationsRows)
	registerRowsAdapter(func(v *AppCustomProductPageLocalizationResponse) *AppCustomProductPageLocalizationsResponse {
		return &AppCustomProductPageLocalizationsResponse{Data: []Resource[AppCustomProductPageLocalizationAttributes]{v.Data}}
	}, appCustomProductPageLocalizationsRows)
	registerRows(appKeywordsRows)
	registerRows(appStoreVersionExperimentsRows)
	registerRowsAdapter(func(v *AppStoreVersionExperimentResponse) *AppStoreVersionExperimentsResponse {
		return &AppStoreVersionExperimentsResponse{Data: []Resource[AppStoreVersionExperimentAttributes]{v.Data}}
	}, appStoreVersionExperimentsRows)
	registerRows(appStoreVersionExperimentsV2Rows)
	registerRowsAdapter(func(v *AppStoreVersionExperimentV2Response) *AppStoreVersionExperimentsV2Response {
		return &AppStoreVersionExperimentsV2Response{Data: []Resource[AppStoreVersionExperimentV2Attributes]{v.Data}}
	}, appStoreVersionExperimentsV2Rows)
	registerRows(appStoreVersionExperimentTreatmentsRows)
	registerRowsAdapter(func(v *AppStoreVersionExperimentTreatmentResponse) *AppStoreVersionExperimentTreatmentsResponse {
		return &AppStoreVersionExperimentTreatmentsResponse{Data: []Resource[AppStoreVersionExperimentTreatmentAttributes]{v.Data}}
	}, appStoreVersionExperimentTreatmentsRows)
	registerRows(appStoreVersionExperimentTreatmentLocalizationsRows)
	registerRowsAdapter(func(v *AppStoreVersionExperimentTreatmentLocalizationResponse) *AppStoreVersionExperimentTreatmentLocalizationsResponse {
		return &AppStoreVersionExperimentTreatmentLocalizationsResponse{Data: []Resource[AppStoreVersionExperimentTreatmentLocalizationAttributes]{v.Data}}
	}, appStoreVersionExperimentTreatmentLocalizationsRows)
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
