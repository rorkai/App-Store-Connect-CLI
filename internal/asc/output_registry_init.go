package asc

func singleLinkageRows(data ResourceData) ([]string, [][]string) {
	return linkagesRows(&LinkagesResponse{Data: []ResourceData{data}})
}

func registerSingleLinkageRows[T any](extract func(*T) ResourceData) {
	registerRows(func(v *T) ([]string, [][]string) {
		return singleLinkageRows(extract(v))
	})
}

func registerIDStateRows[T any](extract func(*T) (string, string), rows func(string, string) ([]string, [][]string)) {
	registerRows(func(v *T) ([]string, [][]string) {
		id, state := extract(v)
		return rows(id, state)
	})
}

func registerIDBoolRows[T any](extract func(*T) (string, bool), rows func(string, bool) ([]string, [][]string)) {
	registerRows(func(v *T) ([]string, [][]string) {
		id, deleted := extract(v)
		return rows(id, deleted)
	})
}

func registerResponseDataRows[T any](rows func([]Resource[T]) ([]string, [][]string)) {
	registerRows(func(v *Response[T]) ([]string, [][]string) {
		return rows(v.Data)
	})
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
	registerIDStateRows(func(v *BackgroundAssetVersionAppStoreReleaseResponse) (string, string) {
		return v.Data.ID, v.Data.Attributes.State
	}, backgroundAssetVersionStateRows)
	registerIDStateRows(func(v *BackgroundAssetVersionExternalBetaReleaseResponse) (string, string) {
		return v.Data.ID, v.Data.Attributes.State
	}, backgroundAssetVersionStateRows)
	registerIDStateRows(func(v *BackgroundAssetVersionInternalBetaReleaseResponse) (string, string) {
		return v.Data.ID, v.Data.Attributes.State
	}, backgroundAssetVersionStateRows)
	registerRows(backgroundAssetUploadFilesRows)
	registerRowsAdapter(func(v *BackgroundAssetUploadFileResponse) *BackgroundAssetUploadFilesResponse {
		return &BackgroundAssetUploadFilesResponse{Data: []Resource[BackgroundAssetUploadFileAttributes]{v.Data}}
	}, backgroundAssetUploadFilesRows)
	registerRows(nominationsRows)
	registerRowsAdapter(func(v *NominationResponse) *NominationsResponse {
		return &NominationsResponse{Data: []Resource[NominationAttributes]{v.Data}}
	}, nominationsRows)
	registerRows(linkagesRows)
	registerSingleLinkageRows(func(v *AppClipDefaultExperienceReviewDetailLinkageResponse) ResourceData { return v.Data })
	registerSingleLinkageRows(func(v *AppClipDefaultExperienceReleaseWithAppStoreVersionLinkageResponse) ResourceData {
		return v.Data
	})
	registerSingleLinkageRows(func(v *AppClipDefaultExperienceLocalizationHeaderImageLinkageResponse) ResourceData {
		return v.Data
	})
	registerSingleLinkageRows(func(v *AppStoreVersionAgeRatingDeclarationLinkageResponse) ResourceData { return v.Data })
	registerSingleLinkageRows(func(v *AppStoreVersionReviewDetailLinkageResponse) ResourceData { return v.Data })
	registerSingleLinkageRows(func(v *AppStoreVersionAppClipDefaultExperienceLinkageResponse) ResourceData { return v.Data })
	registerSingleLinkageRows(func(v *AppStoreVersionSubmissionLinkageResponse) ResourceData { return v.Data })
	registerSingleLinkageRows(func(v *AppStoreVersionRoutingAppCoverageLinkageResponse) ResourceData { return v.Data })
	registerSingleLinkageRows(func(v *AppStoreVersionAlternativeDistributionPackageLinkageResponse) ResourceData {
		return v.Data
	})
	registerSingleLinkageRows(func(v *AppStoreVersionGameCenterAppVersionLinkageResponse) ResourceData { return v.Data })
	registerSingleLinkageRows(func(v *BuildAppLinkageResponse) ResourceData { return v.Data })
	registerSingleLinkageRows(func(v *BuildAppStoreVersionLinkageResponse) ResourceData { return v.Data })
	registerSingleLinkageRows(func(v *BuildBuildBetaDetailLinkageResponse) ResourceData { return v.Data })
	registerSingleLinkageRows(func(v *BuildPreReleaseVersionLinkageResponse) ResourceData { return v.Data })
	registerSingleLinkageRows(func(v *PreReleaseVersionAppLinkageResponse) ResourceData { return v.Data })
	registerSingleLinkageRows(func(v *AppInfoAgeRatingDeclarationLinkageResponse) ResourceData { return v.Data })
	registerSingleLinkageRows(func(v *AppInfoPrimaryCategoryLinkageResponse) ResourceData { return v.Data })
	registerSingleLinkageRows(func(v *AppInfoPrimarySubcategoryOneLinkageResponse) ResourceData { return v.Data })
	registerSingleLinkageRows(func(v *AppInfoPrimarySubcategoryTwoLinkageResponse) ResourceData { return v.Data })
	registerSingleLinkageRows(func(v *AppInfoSecondaryCategoryLinkageResponse) ResourceData { return v.Data })
	registerSingleLinkageRows(func(v *AppInfoSecondarySubcategoryOneLinkageResponse) ResourceData { return v.Data })
	registerSingleLinkageRows(func(v *AppInfoSecondarySubcategoryTwoLinkageResponse) ResourceData { return v.Data })
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
	registerResponseDataRows(betaGroupMetricsRows)
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
	registerRowsAdapter(func(v *GameCenterAchievementResponse) *GameCenterAchievementsResponse {
		return &GameCenterAchievementsResponse{Data: []Resource[GameCenterAchievementAttributes]{v.Data}}
	}, gameCenterAchievementsRows)
	registerRows(gameCenterAchievementDeleteResultRows)
	registerRows(gameCenterAchievementVersionsRows)
	registerRowsAdapter(func(v *GameCenterAchievementVersionResponse) *GameCenterAchievementVersionsResponse {
		return &GameCenterAchievementVersionsResponse{Data: []Resource[GameCenterAchievementVersionAttributes]{v.Data}}
	}, gameCenterAchievementVersionsRows)
	registerRows(gameCenterLeaderboardsRows)
	registerRowsAdapter(func(v *GameCenterLeaderboardResponse) *GameCenterLeaderboardsResponse {
		return &GameCenterLeaderboardsResponse{Data: []Resource[GameCenterLeaderboardAttributes]{v.Data}}
	}, gameCenterLeaderboardsRows)
	registerRows(gameCenterLeaderboardDeleteResultRows)
	registerRows(gameCenterLeaderboardVersionsRows)
	registerRowsAdapter(func(v *GameCenterLeaderboardVersionResponse) *GameCenterLeaderboardVersionsResponse {
		return &GameCenterLeaderboardVersionsResponse{Data: []Resource[GameCenterLeaderboardVersionAttributes]{v.Data}}
	}, gameCenterLeaderboardVersionsRows)
	registerRows(gameCenterLeaderboardSetsRows)
	registerRowsAdapter(func(v *GameCenterLeaderboardSetResponse) *GameCenterLeaderboardSetsResponse {
		return &GameCenterLeaderboardSetsResponse{Data: []Resource[GameCenterLeaderboardSetAttributes]{v.Data}}
	}, gameCenterLeaderboardSetsRows)
	registerRows(gameCenterLeaderboardSetDeleteResultRows)
	registerRows(gameCenterLeaderboardSetVersionsRows)
	registerRowsAdapter(func(v *GameCenterLeaderboardSetVersionResponse) *GameCenterLeaderboardSetVersionsResponse {
		return &GameCenterLeaderboardSetVersionsResponse{Data: []Resource[GameCenterLeaderboardSetVersionAttributes]{v.Data}}
	}, gameCenterLeaderboardSetVersionsRows)
	registerRows(gameCenterLeaderboardLocalizationsRows)
	registerRowsAdapter(func(v *GameCenterLeaderboardLocalizationResponse) *GameCenterLeaderboardLocalizationsResponse {
		return &GameCenterLeaderboardLocalizationsResponse{Data: []Resource[GameCenterLeaderboardLocalizationAttributes]{v.Data}}
	}, gameCenterLeaderboardLocalizationsRows)
	registerRows(gameCenterLeaderboardLocalizationDeleteResultRows)
	registerRows(gameCenterLeaderboardReleasesRows)
	registerRowsAdapter(func(v *GameCenterLeaderboardReleaseResponse) *GameCenterLeaderboardReleasesResponse {
		return &GameCenterLeaderboardReleasesResponse{Data: []Resource[GameCenterLeaderboardReleaseAttributes]{v.Data}}
	}, gameCenterLeaderboardReleasesRows)
	registerRows(gameCenterLeaderboardReleaseDeleteResultRows)
	registerRows(gameCenterLeaderboardEntrySubmissionRows)
	registerRows(gameCenterPlayerAchievementSubmissionRows)
	registerRows(gameCenterLeaderboardSetReleasesRows)
	registerRowsAdapter(func(v *GameCenterLeaderboardSetReleaseResponse) *GameCenterLeaderboardSetReleasesResponse {
		return &GameCenterLeaderboardSetReleasesResponse{Data: []Resource[GameCenterLeaderboardSetReleaseAttributes]{v.Data}}
	}, gameCenterLeaderboardSetReleasesRows)
	registerRows(gameCenterLeaderboardSetReleaseDeleteResultRows)
	registerRows(gameCenterLeaderboardSetLocalizationsRows)
	registerRowsAdapter(func(v *GameCenterLeaderboardSetLocalizationResponse) *GameCenterLeaderboardSetLocalizationsResponse {
		return &GameCenterLeaderboardSetLocalizationsResponse{Data: []Resource[GameCenterLeaderboardSetLocalizationAttributes]{v.Data}}
	}, gameCenterLeaderboardSetLocalizationsRows)
	registerRows(gameCenterLeaderboardSetLocalizationDeleteResultRows)
	registerRows(gameCenterAchievementReleasesRows)
	registerRowsAdapter(func(v *GameCenterAchievementReleaseResponse) *GameCenterAchievementReleasesResponse {
		return &GameCenterAchievementReleasesResponse{Data: []Resource[GameCenterAchievementReleaseAttributes]{v.Data}}
	}, gameCenterAchievementReleasesRows)
	registerRows(gameCenterAchievementReleaseDeleteResultRows)
	registerRows(gameCenterAchievementLocalizationsRows)
	registerRowsAdapter(func(v *GameCenterAchievementLocalizationResponse) *GameCenterAchievementLocalizationsResponse {
		return &GameCenterAchievementLocalizationsResponse{Data: []Resource[GameCenterAchievementLocalizationAttributes]{v.Data}}
	}, gameCenterAchievementLocalizationsRows)
	registerRows(gameCenterAchievementLocalizationDeleteResultRows)
	registerRows(gameCenterLeaderboardImageUploadResultRows)
	registerRows(gameCenterLeaderboardImageDeleteResultRows)
	registerRows(gameCenterAchievementImageUploadResultRows)
	registerRows(gameCenterAchievementImageDeleteResultRows)
	registerRows(gameCenterLeaderboardSetImageUploadResultRows)
	registerRows(gameCenterLeaderboardSetImageDeleteResultRows)
	registerRows(gameCenterChallengesRows)
	registerRowsAdapter(func(v *GameCenterChallengeResponse) *GameCenterChallengesResponse {
		return &GameCenterChallengesResponse{Data: []Resource[GameCenterChallengeAttributes]{v.Data}}
	}, gameCenterChallengesRows)
	registerRows(gameCenterChallengeDeleteResultRows)
	registerRows(gameCenterChallengeVersionsRows)
	registerRowsAdapter(func(v *GameCenterChallengeVersionResponse) *GameCenterChallengeVersionsResponse {
		return &GameCenterChallengeVersionsResponse{Data: []Resource[GameCenterChallengeVersionAttributes]{v.Data}}
	}, gameCenterChallengeVersionsRows)
	registerRows(gameCenterChallengeLocalizationsRows)
	registerRowsAdapter(func(v *GameCenterChallengeLocalizationResponse) *GameCenterChallengeLocalizationsResponse {
		return &GameCenterChallengeLocalizationsResponse{Data: []Resource[GameCenterChallengeLocalizationAttributes]{v.Data}}
	}, gameCenterChallengeLocalizationsRows)
	registerRows(gameCenterChallengeLocalizationDeleteResultRows)
	registerRows(gameCenterChallengeImagesRows)
	registerRowsAdapter(func(v *GameCenterChallengeImageResponse) *GameCenterChallengeImagesResponse {
		return &GameCenterChallengeImagesResponse{Data: []Resource[GameCenterChallengeImageAttributes]{v.Data}}
	}, gameCenterChallengeImagesRows)
	registerRows(gameCenterChallengeImageUploadResultRows)
	registerRows(gameCenterChallengeImageDeleteResultRows)
	registerRows(gameCenterChallengeReleasesRows)
	registerRowsAdapter(func(v *GameCenterChallengeVersionReleaseResponse) *GameCenterChallengeVersionReleasesResponse {
		return &GameCenterChallengeVersionReleasesResponse{Data: []Resource[GameCenterChallengeVersionReleaseAttributes]{v.Data}}
	}, gameCenterChallengeReleasesRows)
	registerRows(gameCenterChallengeReleaseDeleteResultRows)
	registerRows(gameCenterActivitiesRows)
	registerRowsAdapter(func(v *GameCenterActivityResponse) *GameCenterActivitiesResponse {
		return &GameCenterActivitiesResponse{Data: []Resource[GameCenterActivityAttributes]{v.Data}}
	}, gameCenterActivitiesRows)
	registerRows(gameCenterActivityDeleteResultRows)
	registerRows(gameCenterActivityVersionsRows)
	registerRowsAdapter(func(v *GameCenterActivityVersionResponse) *GameCenterActivityVersionsResponse {
		return &GameCenterActivityVersionsResponse{Data: []Resource[GameCenterActivityVersionAttributes]{v.Data}}
	}, gameCenterActivityVersionsRows)
	registerRows(gameCenterActivityLocalizationsRows)
	registerRowsAdapter(func(v *GameCenterActivityLocalizationResponse) *GameCenterActivityLocalizationsResponse {
		return &GameCenterActivityLocalizationsResponse{Data: []Resource[GameCenterActivityLocalizationAttributes]{v.Data}}
	}, gameCenterActivityLocalizationsRows)
	registerRows(gameCenterActivityLocalizationDeleteResultRows)
	registerRows(gameCenterActivityImagesRows)
	registerRowsAdapter(func(v *GameCenterActivityImageResponse) *GameCenterActivityImagesResponse {
		return &GameCenterActivityImagesResponse{Data: []Resource[GameCenterActivityImageAttributes]{v.Data}}
	}, gameCenterActivityImagesRows)
	registerRows(gameCenterActivityImageUploadResultRows)
	registerRows(gameCenterActivityImageDeleteResultRows)
	registerRows(gameCenterActivityReleasesRows)
	registerRowsAdapter(func(v *GameCenterActivityVersionReleaseResponse) *GameCenterActivityVersionReleasesResponse {
		return &GameCenterActivityVersionReleasesResponse{Data: []Resource[GameCenterActivityVersionReleaseAttributes]{v.Data}}
	}, gameCenterActivityReleasesRows)
	registerRows(gameCenterActivityReleaseDeleteResultRows)
	registerRows(gameCenterGroupsRows)
	registerRowsAdapter(func(v *GameCenterGroupResponse) *GameCenterGroupsResponse {
		return &GameCenterGroupsResponse{Data: []Resource[GameCenterGroupAttributes]{v.Data}}
	}, gameCenterGroupsRows)
	registerRows(gameCenterGroupDeleteResultRows)
	registerRows(gameCenterAppVersionsRows)
	registerRowsAdapter(func(v *GameCenterAppVersionResponse) *GameCenterAppVersionsResponse {
		return &GameCenterAppVersionsResponse{Data: []Resource[GameCenterAppVersionAttributes]{v.Data}}
	}, gameCenterAppVersionsRows)
	registerRows(gameCenterEnabledVersionsRows)
	registerRows(gameCenterDetailsRows)
	registerRowsAdapter(func(v *GameCenterDetailResponse) *GameCenterDetailsResponse {
		return &GameCenterDetailsResponse{Data: []Resource[GameCenterDetailAttributes]{v.Data}}
	}, gameCenterDetailsRows)
	registerRows(gameCenterMatchmakingQueuesRows)
	registerRowsAdapter(func(v *GameCenterMatchmakingQueueResponse) *GameCenterMatchmakingQueuesResponse {
		return &GameCenterMatchmakingQueuesResponse{Data: []Resource[GameCenterMatchmakingQueueAttributes]{v.Data}}
	}, gameCenterMatchmakingQueuesRows)
	registerRows(gameCenterMatchmakingQueueDeleteResultRows)
	registerRows(gameCenterMatchmakingRuleSetsRows)
	registerRowsAdapter(func(v *GameCenterMatchmakingRuleSetResponse) *GameCenterMatchmakingRuleSetsResponse {
		return &GameCenterMatchmakingRuleSetsResponse{Data: []Resource[GameCenterMatchmakingRuleSetAttributes]{v.Data}}
	}, gameCenterMatchmakingRuleSetsRows)
	registerRows(gameCenterMatchmakingRuleSetDeleteResultRows)
	registerRows(gameCenterMatchmakingRulesRows)
	registerRowsAdapter(func(v *GameCenterMatchmakingRuleResponse) *GameCenterMatchmakingRulesResponse {
		return &GameCenterMatchmakingRulesResponse{Data: []Resource[GameCenterMatchmakingRuleAttributes]{v.Data}}
	}, gameCenterMatchmakingRulesRows)
	registerRows(gameCenterMatchmakingRuleDeleteResultRows)
	registerRows(gameCenterMatchmakingTeamsRows)
	registerRowsAdapter(func(v *GameCenterMatchmakingTeamResponse) *GameCenterMatchmakingTeamsResponse {
		return &GameCenterMatchmakingTeamsResponse{Data: []Resource[GameCenterMatchmakingTeamAttributes]{v.Data}}
	}, gameCenterMatchmakingTeamsRows)
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
	registerIDBoolRows(func(v *AlternativeDistributionDomainDeleteResult) (string, bool) {
		return v.ID, v.Deleted
	}, alternativeDistributionDeleteResultRows)
	registerIDBoolRows(func(v *AlternativeDistributionKeyDeleteResult) (string, bool) {
		return v.ID, v.Deleted
	}, alternativeDistributionDeleteResultRows)
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
