package asc

import "strings"

// FeedbackOption is a functional option for GetFeedback.
type FeedbackOption func(*feedbackQuery)

// CrashOption is a functional option for GetCrashes.
type CrashOption func(*crashQuery)

// ReviewOption is a functional option for GetReviews.
type ReviewOption func(*reviewQuery)

// AppsOption is a functional option for GetApps.
type AppsOption func(*appsQuery)

// AppSearchKeywordsOption is a functional option for GetAppSearchKeywords.
type AppSearchKeywordsOption func(*appSearchKeywordsQuery)

// AppClipsOption is a functional option for GetAppClips.
type AppClipsOption func(*appClipsQuery)

// AppClipDefaultExperiencesOption is a functional option for GetAppClipDefaultExperiences.
type AppClipDefaultExperiencesOption func(*appClipDefaultExperiencesQuery)

// AppClipDefaultExperienceOption is a functional option for GetAppClipDefaultExperience.
type AppClipDefaultExperienceOption func(*appClipDefaultExperienceQuery)

// AppClipDefaultExperienceLocalizationsOption is a functional option for GetAppClipDefaultExperienceLocalizations.
type AppClipDefaultExperienceLocalizationsOption func(*appClipDefaultExperienceLocalizationsQuery)

// AppClipAdvancedExperiencesOption is a functional option for GetAppClipAdvancedExperiences.
type AppClipAdvancedExperiencesOption func(*appClipAdvancedExperiencesQuery)

// BetaAppClipInvocationOption is a functional option for GetBetaAppClipInvocation.
type BetaAppClipInvocationOption func(*betaAppClipInvocationQuery)

// AppTagsOption is a functional option for GetAppTags.
type AppTagsOption func(*appTagsQuery)

// NominationsOption is a functional option for nominations endpoints.
type NominationsOption func(*nominationsQuery)

// BuildsOption is a functional option for GetBuilds.
type BuildsOption func(*buildsQuery)

// BuildBundlesOption is a functional option for GetBuildBundlesForBuild.
type BuildBundlesOption func(*buildBundlesQuery)

// BuildBundleFileSizesOption is a functional option for GetBuildBundleFileSizes.
type BuildBundleFileSizesOption func(*buildBundleFileSizesQuery)

// BuildUploadsOption is a functional option for GetBuildUploads.
type BuildUploadsOption func(*buildUploadsQuery)

// BuildUploadFilesOption is a functional option for GetBuildUploadFiles.
type BuildUploadFilesOption func(*buildUploadFilesQuery)

// BuildIndividualTestersOption is a functional option for GetBuildIndividualTesters.
type BuildIndividualTestersOption func(*buildIndividualTestersQuery)

// BetaAppClipInvocationsOption is a functional option for GetBuildBundleBetaAppClipInvocations.
type BetaAppClipInvocationsOption func(*betaAppClipInvocationsQuery)

// SubscriptionOfferCodeOneTimeUseCodesOption is a functional option for GetSubscriptionOfferCodeOneTimeUseCodes.
type SubscriptionOfferCodeOneTimeUseCodesOption func(*subscriptionOfferCodeOneTimeUseCodesQuery)

// WinBackOffersOption is a functional option for win-back offer list endpoints.
type WinBackOffersOption func(*winBackOffersQuery)

// WinBackOfferPricesOption is a functional option for win-back offer prices list endpoints.
type WinBackOfferPricesOption func(*winBackOfferPricesQuery)

// AppStoreVersionsOption is a functional option for GetAppStoreVersions.
type AppStoreVersionsOption func(*appStoreVersionsQuery)

// AppStoreVersionOption is a functional option for GetAppStoreVersion.
type AppStoreVersionOption func(*appStoreVersionQuery)

// ReviewSubmissionsOption is a functional option for GetReviewSubmissions.
type ReviewSubmissionsOption func(*reviewSubmissionsQuery)

// ReviewSubmissionItemsOption is a functional option for GetReviewSubmissionItems.
type ReviewSubmissionItemsOption func(*reviewSubmissionItemsQuery)

// PreReleaseVersionsOption is a functional option for GetPreReleaseVersions.
type PreReleaseVersionsOption func(*preReleaseVersionsQuery)

// BetaGroupsOption is a functional option for GetBetaGroups.
type BetaGroupsOption func(*betaGroupsQuery)

// BetaGroupBuildsOption is a functional option for GetBetaGroupBuilds.
type BetaGroupBuildsOption func(*betaGroupBuildsQuery)

// BetaGroupTestersOption is a functional option for GetBetaGroupTesters.
type BetaGroupTestersOption func(*betaGroupTestersQuery)

// BetaTestersOption is a functional option for GetBetaTesters.
type BetaTestersOption func(*betaTestersQuery)

// BetaTesterUsagesOption is a functional option for beta tester usage metrics.
type BetaTesterUsagesOption func(*betaTesterUsagesQuery)

// BundleIDsOption is a functional option for GetBundleIDs.
type BundleIDsOption func(*bundleIDsQuery)

// BundleIDCapabilitiesOption is a functional option for GetBundleIDCapabilities.
type BundleIDCapabilitiesOption func(*bundleIDCapabilitiesQuery)

// PromotedPurchasesOption is a functional option for promoted purchases endpoints.
type PromotedPurchasesOption func(*promotedPurchasesQuery)

// MerchantIDsOption is a functional option for GetMerchantIDs.
type MerchantIDsOption func(*merchantIDsQuery)

// MerchantIDCertificatesOption is a functional option for GetMerchantIDCertificates.
type MerchantIDCertificatesOption func(*merchantIDCertificatesQuery)

// PassTypeIDsOption is a functional option for GetPassTypeIDs.
type PassTypeIDsOption func(*passTypeIDsQuery)

// PassTypeIDOption is a functional option for GetPassTypeID.
type PassTypeIDOption func(*passTypeIDQuery)

// PassTypeIDCertificatesOption is a functional option for GetPassTypeIDCertificates.
type PassTypeIDCertificatesOption func(*passTypeIDCertificatesQuery)

// CertificatesOption is a functional option for GetCertificates.
type CertificatesOption func(*certificatesQuery)

// DevicesOption is a functional option for GetDevices.
type DevicesOption func(*devicesQuery)

// ProfilesOption is a functional option for GetProfiles.
type ProfilesOption func(*profilesQuery)

// UsersOption is a functional option for GetUsers.
type UsersOption func(*usersQuery)

// ProfileCertificatesOption is a functional option for GetProfileCertificates.
type ProfileCertificatesOption func(*profileCertificatesQuery)

// ProfileDevicesOption is a functional option for GetProfileDevices.
type ProfileDevicesOption func(*profileDevicesQuery)

// UserVisibleAppsOption is a functional option for GetUserVisibleApps.
type UserVisibleAppsOption func(*userVisibleAppsQuery)

// ActorsOption is a functional option for GetActors.
type ActorsOption func(*actorsQuery)

// UserInvitationsOption is a functional option for GetUserInvitations.
type UserInvitationsOption func(*userInvitationsQuery)

// BetaAppReviewDetailsOption is a functional option for beta app review details.
type BetaAppReviewDetailsOption func(*betaAppReviewDetailsQuery)

// BetaAppReviewSubmissionsOption is a functional option for beta app review submissions.
type BetaAppReviewSubmissionsOption func(*betaAppReviewSubmissionsQuery)

// BuildBetaDetailsOption is a functional option for build beta details.
type BuildBetaDetailsOption func(*buildBetaDetailsQuery)

// BetaRecruitmentCriterionOptionsOption is a functional option for recruitment options.
type BetaRecruitmentCriterionOptionsOption func(*betaRecruitmentCriterionOptionsQuery)

// AppStoreVersionLocalizationsOption is a functional option for version localizations.
type AppStoreVersionLocalizationsOption func(*appStoreVersionLocalizationsQuery)

// BetaAppLocalizationsOption is a functional option for beta app localizations.
type BetaAppLocalizationsOption func(*betaAppLocalizationsQuery)

// BetaBuildLocalizationsOption is a functional option for beta build localizations.
type BetaBuildLocalizationsOption func(*betaBuildLocalizationsQuery)

// BetaBuildUsagesOption is a functional option for beta build usage metrics.
type BetaBuildUsagesOption func(*betaBuildUsagesQuery)

// AppInfoLocalizationsOption is a functional option for app info localizations.
type AppInfoLocalizationsOption func(*appInfoLocalizationsQuery)

// AppInfoOption is a functional option for GetAppInfo.
type AppInfoOption func(*appInfoQuery)

// AppCustomProductPagesOption is a functional option for custom product page list endpoints.
type AppCustomProductPagesOption func(*appCustomProductPagesQuery)

// AppCustomProductPageVersionsOption is a functional option for custom product page version list endpoints.
type AppCustomProductPageVersionsOption func(*appCustomProductPageVersionsQuery)

// AppCustomProductPageLocalizationsOption is a functional option for custom product page localization list endpoints.
type AppCustomProductPageLocalizationsOption func(*appCustomProductPageLocalizationsQuery)

// AppCustomProductPageLocalizationPreviewSetsOption is a functional option for preview set list endpoints.
type AppCustomProductPageLocalizationPreviewSetsOption func(*appCustomProductPageLocalizationPreviewSetsQuery)

// AppCustomProductPageLocalizationScreenshotSetsOption is a functional option for screenshot set list endpoints.
type AppCustomProductPageLocalizationScreenshotSetsOption func(*appCustomProductPageLocalizationScreenshotSetsQuery)

// AppStoreVersionLocalizationPreviewSetsOption is a functional option for app store version preview sets list endpoints.
type AppStoreVersionLocalizationPreviewSetsOption func(*appStoreVersionLocalizationPreviewSetsQuery)

// AppStoreVersionLocalizationScreenshotSetsOption is a functional option for app store version screenshot sets list endpoints.
type AppStoreVersionLocalizationScreenshotSetsOption func(*appStoreVersionLocalizationScreenshotSetsQuery)

// AppStoreVersionExperimentsOption is a functional option for app store version experiment list endpoints (v1).
type AppStoreVersionExperimentsOption func(*appStoreVersionExperimentsQuery)

// AppStoreVersionExperimentsV2Option is a functional option for app store version experiment list endpoints (v2).
type AppStoreVersionExperimentsV2Option func(*appStoreVersionExperimentsV2Query)

// AppStoreVersionExperimentTreatmentsOption is a functional option for experiment treatment list endpoints.
type AppStoreVersionExperimentTreatmentsOption func(*appStoreVersionExperimentTreatmentsQuery)

// AppStoreVersionExperimentTreatmentLocalizationsOption is a functional option for treatment localization list endpoints.
type AppStoreVersionExperimentTreatmentLocalizationsOption func(*appStoreVersionExperimentTreatmentLocalizationsQuery)

// TerritoriesOption is a functional option for GetTerritories.
type TerritoriesOption func(*territoriesQuery)

// AndroidToIosAppMappingDetailsOption is a functional option for Android-to-iOS mappings.
type AndroidToIosAppMappingDetailsOption func(*androidToIosAppMappingDetailsQuery)

// PerfPowerMetricsOption is a functional option for performance/power metrics.
type PerfPowerMetricsOption func(*perfPowerMetricsQuery)

// DiagnosticSignaturesOption is a functional option for diagnostic signatures.
type DiagnosticSignaturesOption func(*diagnosticSignaturesQuery)

// DiagnosticLogsOption is a functional option for diagnostic logs.
type DiagnosticLogsOption func(*diagnosticLogsQuery)

// TerritoryAvailabilitiesOption is a functional option for GetTerritoryAvailabilities.
type TerritoryAvailabilitiesOption func(*territoryAvailabilitiesQuery)

// LinkagesOption is a functional option for linkages endpoints.
type LinkagesOption func(*linkagesQuery)

// PricePointsOption is a functional option for GetAppPricePoints.
type PricePointsOption func(*pricePointsQuery)

// AccessibilityDeclarationsOption is a functional option for accessibility declarations.
type AccessibilityDeclarationsOption func(*accessibilityDeclarationsQuery)

// AppStoreReviewAttachmentsOption is a functional option for review attachments.
type AppStoreReviewAttachmentsOption func(*appStoreReviewAttachmentsQuery)

// AppEncryptionDeclarationsOption is a functional option for encryption declarations.
type AppEncryptionDeclarationsOption func(*appEncryptionDeclarationsQuery)

// MarketplaceWebhooksOption is a functional option for marketplace webhooks.
type MarketplaceWebhooksOption func(*marketplaceWebhooksQuery)

// AlternativeDistributionDomainsOption is a functional option for alternative distribution domains.
type AlternativeDistributionDomainsOption func(*alternativeDistributionDomainsQuery)

// AlternativeDistributionKeysOption is a functional option for alternative distribution keys.
type AlternativeDistributionKeysOption func(*alternativeDistributionKeysQuery)

// AlternativeDistributionPackageVersionsOption is a functional option for package versions list endpoints.
type AlternativeDistributionPackageVersionsOption func(*alternativeDistributionPackageVersionsQuery)

// AlternativeDistributionPackageVariantsOption is a functional option for package variant list endpoints.
type AlternativeDistributionPackageVariantsOption func(*alternativeDistributionPackageVariantsQuery)

// AlternativeDistributionPackageDeltasOption is a functional option for package delta list endpoints.
type AlternativeDistributionPackageDeltasOption func(*alternativeDistributionPackageDeltasQuery)

// WebhooksOption is a functional option for webhooks list endpoints.
type WebhooksOption func(*webhooksQuery)

// WebhookDeliveriesOption is a functional option for webhook deliveries endpoints.
type WebhookDeliveriesOption func(*webhookDeliveriesQuery)

// BackgroundAssetsOption is a functional option for background assets list endpoints.
type BackgroundAssetsOption func(*backgroundAssetsQuery)

// BackgroundAssetVersionsOption is a functional option for background asset versions list endpoints.
type BackgroundAssetVersionsOption func(*backgroundAssetVersionsQuery)

// BackgroundAssetUploadFilesOption is a functional option for background asset upload files list endpoints.
type BackgroundAssetUploadFilesOption func(*backgroundAssetUploadFilesQuery)

// WithFeedbackDeviceModels filters feedback by device model(s).
func WithFeedbackDeviceModels(models []string) FeedbackOption {
	return func(q *feedbackQuery) {
		q.deviceModels = normalizeList(models)
	}
}

// WithFeedbackOSVersions filters feedback by OS version(s).
func WithFeedbackOSVersions(versions []string) FeedbackOption {
	return func(q *feedbackQuery) {
		q.osVersions = normalizeList(versions)
	}
}

// WithFeedbackAppPlatforms filters feedback by app platform(s).
func WithFeedbackAppPlatforms(platforms []string) FeedbackOption {
	return func(q *feedbackQuery) {
		q.appPlatforms = normalizeUpperList(platforms)
	}
}

// WithFeedbackDevicePlatforms filters feedback by device platform(s).
func WithFeedbackDevicePlatforms(platforms []string) FeedbackOption {
	return func(q *feedbackQuery) {
		q.devicePlatforms = normalizeUpperList(platforms)
	}
}

// WithFeedbackBuildIDs filters feedback by build ID(s).
func WithFeedbackBuildIDs(ids []string) FeedbackOption {
	return func(q *feedbackQuery) {
		q.buildIDs = normalizeList(ids)
	}
}

// WithFeedbackBuildPreReleaseVersionIDs filters feedback by pre-release version ID(s).
func WithFeedbackBuildPreReleaseVersionIDs(ids []string) FeedbackOption {
	return func(q *feedbackQuery) {
		q.buildPreReleaseVersionIDs = normalizeList(ids)
	}
}

// WithFeedbackTesterIDs filters feedback by tester ID(s).
func WithFeedbackTesterIDs(ids []string) FeedbackOption {
	return func(q *feedbackQuery) {
		q.testerIDs = normalizeList(ids)
	}
}

// WithFeedbackLimit sets the max number of feedback items to return.
func WithFeedbackLimit(limit int) FeedbackOption {
	return func(q *feedbackQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithFeedbackNextURL uses a next page URL directly.
func WithFeedbackNextURL(next string) FeedbackOption {
	return func(q *feedbackQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

// WithFeedbackSort sets the sort order for feedback.
func WithFeedbackSort(sort string) FeedbackOption {
	return func(q *feedbackQuery) {
		if strings.TrimSpace(sort) != "" {
			q.sort = strings.TrimSpace(sort)
		}
	}
}

// WithFeedbackIncludeScreenshots includes screenshot URLs in feedback responses.
func WithFeedbackIncludeScreenshots() FeedbackOption {
	return func(q *feedbackQuery) {
		q.includeScreenshots = true
	}
}

// WithCrashDeviceModels filters crashes by device model(s).
func WithCrashDeviceModels(models []string) CrashOption {
	return func(q *crashQuery) {
		q.deviceModels = normalizeList(models)
	}
}

// WithCrashOSVersions filters crashes by OS version(s).
func WithCrashOSVersions(versions []string) CrashOption {
	return func(q *crashQuery) {
		q.osVersions = normalizeList(versions)
	}
}

// WithCrashAppPlatforms filters crashes by app platform(s).
func WithCrashAppPlatforms(platforms []string) CrashOption {
	return func(q *crashQuery) {
		q.appPlatforms = normalizeUpperList(platforms)
	}
}

// WithCrashDevicePlatforms filters crashes by device platform(s).
func WithCrashDevicePlatforms(platforms []string) CrashOption {
	return func(q *crashQuery) {
		q.devicePlatforms = normalizeUpperList(platforms)
	}
}

// WithCrashBuildIDs filters crashes by build ID(s).
func WithCrashBuildIDs(ids []string) CrashOption {
	return func(q *crashQuery) {
		q.buildIDs = normalizeList(ids)
	}
}

// WithCrashBuildPreReleaseVersionIDs filters crashes by pre-release version ID(s).
func WithCrashBuildPreReleaseVersionIDs(ids []string) CrashOption {
	return func(q *crashQuery) {
		q.buildPreReleaseVersionIDs = normalizeList(ids)
	}
}

// WithSubscriptionOfferCodeOneTimeUseCodesLimit sets the max number of offer code batches to return.
func WithSubscriptionOfferCodeOneTimeUseCodesLimit(limit int) SubscriptionOfferCodeOneTimeUseCodesOption {
	return func(q *subscriptionOfferCodeOneTimeUseCodesQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithSubscriptionOfferCodeOneTimeUseCodesNextURL uses a next page URL directly.
func WithSubscriptionOfferCodeOneTimeUseCodesNextURL(next string) SubscriptionOfferCodeOneTimeUseCodesOption {
	return func(q *subscriptionOfferCodeOneTimeUseCodesQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

// WithMarketplaceWebhooksLimit sets the max number of marketplace webhooks to return.
func WithMarketplaceWebhooksLimit(limit int) MarketplaceWebhooksOption {
	return func(q *marketplaceWebhooksQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithMarketplaceWebhooksNextURL uses a next page URL directly.
func WithMarketplaceWebhooksNextURL(next string) MarketplaceWebhooksOption {
	return func(q *marketplaceWebhooksQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

// WithMarketplaceWebhooksFields sets fields[marketplaceWebhooks] for webhook responses.
func WithMarketplaceWebhooksFields(fields []string) MarketplaceWebhooksOption {
	return func(q *marketplaceWebhooksQuery) {
		q.fields = normalizeList(fields)
	}
}

// WithAlternativeDistributionDomainsLimit sets the max number of domains to return.
func WithAlternativeDistributionDomainsLimit(limit int) AlternativeDistributionDomainsOption {
	return func(q *alternativeDistributionDomainsQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithAlternativeDistributionDomainsNextURL uses a next page URL directly.
func WithAlternativeDistributionDomainsNextURL(next string) AlternativeDistributionDomainsOption {
	return func(q *alternativeDistributionDomainsQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

// WithAlternativeDistributionKeysLimit sets the max number of keys to return.
func WithAlternativeDistributionKeysLimit(limit int) AlternativeDistributionKeysOption {
	return func(q *alternativeDistributionKeysQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithAlternativeDistributionKeysNextURL uses a next page URL directly.
func WithAlternativeDistributionKeysNextURL(next string) AlternativeDistributionKeysOption {
	return func(q *alternativeDistributionKeysQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

// WithAlternativeDistributionPackageVersionsLimit sets the max number of package versions to return.
func WithAlternativeDistributionPackageVersionsLimit(limit int) AlternativeDistributionPackageVersionsOption {
	return func(q *alternativeDistributionPackageVersionsQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithAlternativeDistributionPackageVersionsNextURL uses a next page URL directly.
func WithAlternativeDistributionPackageVersionsNextURL(next string) AlternativeDistributionPackageVersionsOption {
	return func(q *alternativeDistributionPackageVersionsQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

// WithAlternativeDistributionPackageVariantsLimit sets the max number of package variants to return.
func WithAlternativeDistributionPackageVariantsLimit(limit int) AlternativeDistributionPackageVariantsOption {
	return func(q *alternativeDistributionPackageVariantsQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithAlternativeDistributionPackageVariantsNextURL uses a next page URL directly.
func WithAlternativeDistributionPackageVariantsNextURL(next string) AlternativeDistributionPackageVariantsOption {
	return func(q *alternativeDistributionPackageVariantsQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

// WithAlternativeDistributionPackageDeltasLimit sets the max number of package deltas to return.
func WithAlternativeDistributionPackageDeltasLimit(limit int) AlternativeDistributionPackageDeltasOption {
	return func(q *alternativeDistributionPackageDeltasQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithAlternativeDistributionPackageDeltasNextURL uses a next page URL directly.
func WithAlternativeDistributionPackageDeltasNextURL(next string) AlternativeDistributionPackageDeltasOption {
	return func(q *alternativeDistributionPackageDeltasQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

// WithWebhooksLimit sets the max number of webhooks to return.
func WithWebhooksLimit(limit int) WebhooksOption {
	return func(q *webhooksQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithWebhooksNextURL uses a next page URL directly.
func WithWebhooksNextURL(next string) WebhooksOption {
	return func(q *webhooksQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

// WithWebhooksFields sets fields[webhooks] for webhook responses.
func WithWebhooksFields(fields []string) WebhooksOption {
	return func(q *webhooksQuery) {
		q.fields = normalizeList(fields)
	}
}

// WithWebhooksAppFields sets fields[apps] for webhook responses.
func WithWebhooksAppFields(fields []string) WebhooksOption {
	return func(q *webhooksQuery) {
		q.appFields = normalizeList(fields)
	}
}

// WithWebhooksInclude sets include for webhook responses.
func WithWebhooksInclude(include []string) WebhooksOption {
	return func(q *webhooksQuery) {
		q.include = normalizeList(include)
	}
}

// WithWebhookDeliveriesLimit sets the max number of webhook deliveries to return.
func WithWebhookDeliveriesLimit(limit int) WebhookDeliveriesOption {
	return func(q *webhookDeliveriesQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithWebhookDeliveriesNextURL uses a next page URL directly.
func WithWebhookDeliveriesNextURL(next string) WebhookDeliveriesOption {
	return func(q *webhookDeliveriesQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

// WithWebhookDeliveriesDeliveryStates filters deliveries by state.
func WithWebhookDeliveriesDeliveryStates(states []string) WebhookDeliveriesOption {
	return func(q *webhookDeliveriesQuery) {
		q.deliveryStates = normalizeUpperList(states)
	}
}

// WithWebhookDeliveriesCreatedAfter filters deliveries created after or equal to a timestamp.
func WithWebhookDeliveriesCreatedAfter(values []string) WebhookDeliveriesOption {
	return func(q *webhookDeliveriesQuery) {
		q.createdAfter = normalizeList(values)
	}
}

// WithWebhookDeliveriesCreatedBefore filters deliveries created before a timestamp.
func WithWebhookDeliveriesCreatedBefore(values []string) WebhookDeliveriesOption {
	return func(q *webhookDeliveriesQuery) {
		q.createdBefore = normalizeList(values)
	}
}

// WithWebhookDeliveriesFields sets fields[webhookDeliveries] for delivery responses.
func WithWebhookDeliveriesFields(fields []string) WebhookDeliveriesOption {
	return func(q *webhookDeliveriesQuery) {
		q.fields = normalizeList(fields)
	}
}

// WithWebhookDeliveriesEventFields sets fields[webhookEvents] for delivery responses.
func WithWebhookDeliveriesEventFields(fields []string) WebhookDeliveriesOption {
	return func(q *webhookDeliveriesQuery) {
		q.eventFields = normalizeList(fields)
	}
}

// WithWebhookDeliveriesInclude sets include for delivery responses.
func WithWebhookDeliveriesInclude(include []string) WebhookDeliveriesOption {
	return func(q *webhookDeliveriesQuery) {
		q.include = normalizeList(include)
	}
}

// WithBackgroundAssetsLimit sets the max number of background assets to return.
func WithBackgroundAssetsLimit(limit int) BackgroundAssetsOption {
	return func(q *backgroundAssetsQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithBackgroundAssetsNextURL uses a next page URL directly.
func WithBackgroundAssetsNextURL(next string) BackgroundAssetsOption {
	return func(q *backgroundAssetsQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

// WithBackgroundAssetsFilterArchived filters background assets by archived state.
func WithBackgroundAssetsFilterArchived(values []string) BackgroundAssetsOption {
	return func(q *backgroundAssetsQuery) {
		q.archived = normalizeList(values)
	}
}

// WithBackgroundAssetsFilterAssetPackIdentifier filters background assets by asset pack identifier.
func WithBackgroundAssetsFilterAssetPackIdentifier(values []string) BackgroundAssetsOption {
	return func(q *backgroundAssetsQuery) {
		q.assetPackIdentifiers = normalizeList(values)
	}
}

// WithBackgroundAssetVersionsLimit sets the max number of background asset versions to return.
func WithBackgroundAssetVersionsLimit(limit int) BackgroundAssetVersionsOption {
	return func(q *backgroundAssetVersionsQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithBackgroundAssetVersionsNextURL uses a next page URL directly.
func WithBackgroundAssetVersionsNextURL(next string) BackgroundAssetVersionsOption {
	return func(q *backgroundAssetVersionsQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

// WithBackgroundAssetUploadFilesLimit sets the max number of background asset upload files to return.
func WithBackgroundAssetUploadFilesLimit(limit int) BackgroundAssetUploadFilesOption {
	return func(q *backgroundAssetUploadFilesQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithBackgroundAssetUploadFilesNextURL uses a next page URL directly.
func WithBackgroundAssetUploadFilesNextURL(next string) BackgroundAssetUploadFilesOption {
	return func(q *backgroundAssetUploadFilesQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

// WithWinBackOffersLimit sets the max number of win-back offers to return.
func WithWinBackOffersLimit(limit int) WinBackOffersOption {
	return func(q *winBackOffersQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithWinBackOffersNextURL uses a next page URL directly.
func WithWinBackOffersNextURL(next string) WinBackOffersOption {
	return func(q *winBackOffersQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

// WithWinBackOffersFields sets fields[winBackOffers] for win-back offer responses.
func WithWinBackOffersFields(fields []string) WinBackOffersOption {
	return func(q *winBackOffersQuery) {
		q.fields = normalizeList(fields)
	}
}

// WithWinBackOffersPriceFields sets fields[winBackOfferPrices] for included prices.
func WithWinBackOffersPriceFields(fields []string) WinBackOffersOption {
	return func(q *winBackOffersQuery) {
		q.priceFields = normalizeList(fields)
	}
}

// WithWinBackOffersInclude sets include for win-back offer responses.
func WithWinBackOffersInclude(include []string) WinBackOffersOption {
	return func(q *winBackOffersQuery) {
		q.include = normalizeList(include)
	}
}

// WithWinBackOffersPricesLimit sets limit[prices] for included prices.
func WithWinBackOffersPricesLimit(limit int) WinBackOffersOption {
	return func(q *winBackOffersQuery) {
		if limit > 0 {
			q.pricesLimit = limit
		}
	}
}

// WithWinBackOfferPricesLimit sets the max number of win-back offer prices to return.
func WithWinBackOfferPricesLimit(limit int) WinBackOfferPricesOption {
	return func(q *winBackOfferPricesQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithWinBackOfferPricesNextURL uses a next page URL directly.
func WithWinBackOfferPricesNextURL(next string) WinBackOfferPricesOption {
	return func(q *winBackOfferPricesQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

// WithWinBackOfferPricesTerritoryFilter filters win-back offer prices by territory ID(s).
func WithWinBackOfferPricesTerritoryFilter(ids []string) WinBackOfferPricesOption {
	return func(q *winBackOfferPricesQuery) {
		q.territoryIDs = normalizeList(ids)
	}
}

// WithWinBackOfferPricesFields sets fields[winBackOfferPrices] for price responses.
func WithWinBackOfferPricesFields(fields []string) WinBackOfferPricesOption {
	return func(q *winBackOfferPricesQuery) {
		q.fields = normalizeList(fields)
	}
}

// WithWinBackOfferPricesTerritoryFields sets fields[territories] for included territories.
func WithWinBackOfferPricesTerritoryFields(fields []string) WinBackOfferPricesOption {
	return func(q *winBackOfferPricesQuery) {
		q.territoryFields = normalizeList(fields)
	}
}

// WithWinBackOfferPricesSubscriptionPricePointFields sets fields[subscriptionPricePoints] for included price points.
func WithWinBackOfferPricesSubscriptionPricePointFields(fields []string) WinBackOfferPricesOption {
	return func(q *winBackOfferPricesQuery) {
		q.subscriptionPricePointFields = normalizeList(fields)
	}
}

// WithWinBackOfferPricesInclude sets include for win-back offer price responses.
func WithWinBackOfferPricesInclude(include []string) WinBackOfferPricesOption {
	return func(q *winBackOfferPricesQuery) {
		q.include = normalizeList(include)
	}
}

// WithCrashTesterIDs filters crashes by tester ID(s).
func WithCrashTesterIDs(ids []string) CrashOption {
	return func(q *crashQuery) {
		q.testerIDs = normalizeList(ids)
	}
}

// WithCrashLimit sets the max number of crash items to return.
func WithCrashLimit(limit int) CrashOption {
	return func(q *crashQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithAccessibilityDeclarationsDeviceFamilies filters declarations by device family.
func WithAccessibilityDeclarationsDeviceFamilies(families []string) AccessibilityDeclarationsOption {
	return func(q *accessibilityDeclarationsQuery) {
		q.deviceFamilies = normalizeUpperList(families)
	}
}

// WithAccessibilityDeclarationsStates filters declarations by state.
func WithAccessibilityDeclarationsStates(states []string) AccessibilityDeclarationsOption {
	return func(q *accessibilityDeclarationsQuery) {
		q.states = normalizeUpperList(states)
	}
}

// WithAccessibilityDeclarationsFields includes specific fields.
func WithAccessibilityDeclarationsFields(fields []string) AccessibilityDeclarationsOption {
	return func(q *accessibilityDeclarationsQuery) {
		q.fields = normalizeList(fields)
	}
}

// WithAccessibilityDeclarationsLimit sets the max number of declarations to return.
func WithAccessibilityDeclarationsLimit(limit int) AccessibilityDeclarationsOption {
	return func(q *accessibilityDeclarationsQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithAccessibilityDeclarationsNextURL uses a next page URL directly.
func WithAccessibilityDeclarationsNextURL(next string) AccessibilityDeclarationsOption {
	return func(q *accessibilityDeclarationsQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

// WithAppStoreReviewAttachmentsFields includes specific attachment fields.
func WithAppStoreReviewAttachmentsFields(fields []string) AppStoreReviewAttachmentsOption {
	return func(q *appStoreReviewAttachmentsQuery) {
		q.fieldsAttachments = normalizeList(fields)
	}
}

// WithAppStoreReviewAttachmentReviewDetailFields includes fields for review detail when included.
func WithAppStoreReviewAttachmentReviewDetailFields(fields []string) AppStoreReviewAttachmentsOption {
	return func(q *appStoreReviewAttachmentsQuery) {
		q.fieldsReviewDetails = normalizeList(fields)
	}
}

// WithAppStoreReviewAttachmentsInclude includes related resources.
func WithAppStoreReviewAttachmentsInclude(include []string) AppStoreReviewAttachmentsOption {
	return func(q *appStoreReviewAttachmentsQuery) {
		q.include = normalizeList(include)
	}
}

// WithAppStoreReviewAttachmentsLimit sets the max number of attachments to return.
func WithAppStoreReviewAttachmentsLimit(limit int) AppStoreReviewAttachmentsOption {
	return func(q *appStoreReviewAttachmentsQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithAppStoreReviewAttachmentsNextURL uses a next page URL directly.
func WithAppStoreReviewAttachmentsNextURL(next string) AppStoreReviewAttachmentsOption {
	return func(q *appStoreReviewAttachmentsQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

// WithAppEncryptionDeclarationsBuildIDs filters declarations by build IDs.
func WithAppEncryptionDeclarationsBuildIDs(ids []string) AppEncryptionDeclarationsOption {
	return func(q *appEncryptionDeclarationsQuery) {
		q.buildIDs = normalizeList(ids)
	}
}

// WithAppEncryptionDeclarationsFields includes specific declaration fields.
func WithAppEncryptionDeclarationsFields(fields []string) AppEncryptionDeclarationsOption {
	return func(q *appEncryptionDeclarationsQuery) {
		q.fields = normalizeList(fields)
	}
}

// WithAppEncryptionDeclarationsDocumentFields includes document fields when included.
func WithAppEncryptionDeclarationsDocumentFields(fields []string) AppEncryptionDeclarationsOption {
	return func(q *appEncryptionDeclarationsQuery) {
		q.documentFields = normalizeList(fields)
	}
}

// WithAppEncryptionDeclarationsInclude includes related resources.
func WithAppEncryptionDeclarationsInclude(include []string) AppEncryptionDeclarationsOption {
	return func(q *appEncryptionDeclarationsQuery) {
		q.include = normalizeList(include)
	}
}

// WithAppEncryptionDeclarationsLimit sets the max number of declarations to return.
func WithAppEncryptionDeclarationsLimit(limit int) AppEncryptionDeclarationsOption {
	return func(q *appEncryptionDeclarationsQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithAppEncryptionDeclarationsBuildLimit sets the max number of related builds when included.
func WithAppEncryptionDeclarationsBuildLimit(limit int) AppEncryptionDeclarationsOption {
	return func(q *appEncryptionDeclarationsQuery) {
		if limit > 0 {
			q.buildLimit = limit
		}
	}
}

// WithAppEncryptionDeclarationsNextURL uses a next page URL directly.
func WithAppEncryptionDeclarationsNextURL(next string) AppEncryptionDeclarationsOption {
	return func(q *appEncryptionDeclarationsQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

// WithCrashNextURL uses a next page URL directly.
func WithCrashNextURL(next string) CrashOption {
	return func(q *crashQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

// WithCrashSort sets the sort order for crashes.
func WithCrashSort(sort string) CrashOption {
	return func(q *crashQuery) {
		if strings.TrimSpace(sort) != "" {
			q.sort = strings.TrimSpace(sort)
		}
	}
}

// WithRating filters reviews by star rating (1-5).
func WithRating(rating int) ReviewOption {
	return func(r *reviewQuery) {
		if rating >= 1 && rating <= 5 {
			r.rating = rating
		}
	}
}

// WithTerritory filters reviews by territory code (e.g. US, GBR).
func WithTerritory(territory string) ReviewOption {
	return func(r *reviewQuery) {
		if territory != "" {
			r.territory = strings.ToUpper(territory)
		}
	}
}

// WithReviewSort sets the sort order for reviews.
func WithReviewSort(sort string) ReviewOption {
	return func(r *reviewQuery) {
		if strings.TrimSpace(sort) != "" {
			r.sort = strings.TrimSpace(sort)
		}
	}
}

// WithLimit sets the max number of reviews to return.
func WithLimit(limit int) ReviewOption {
	return func(r *reviewQuery) {
		if limit > 0 {
			r.limit = limit
		}
	}
}

// WithNextURL uses a next page URL directly.
func WithNextURL(next string) ReviewOption {
	return func(r *reviewQuery) {
		if strings.TrimSpace(next) != "" {
			r.nextURL = strings.TrimSpace(next)
		}
	}
}

// WithAppsLimit sets the max number of apps to return.
func WithAppsLimit(limit int) AppsOption {
	return func(q *appsQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithAppsNextURL uses a next page URL directly.
func WithAppsNextURL(next string) AppsOption {
	return func(q *appsQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

// WithAppsSort sets the sort order for apps.
func WithAppsSort(sort string) AppsOption {
	return func(q *appsQuery) {
		if strings.TrimSpace(sort) != "" {
			q.sort = strings.TrimSpace(sort)
		}
	}
}

// WithAppsBundleIDs filters apps by bundle ID(s).
func WithAppsBundleIDs(bundleIDs []string) AppsOption {
	return func(q *appsQuery) {
		q.bundleIDs = normalizeList(bundleIDs)
	}
}

// WithAppsNames filters apps by name(s).
func WithAppsNames(names []string) AppsOption {
	return func(q *appsQuery) {
		q.names = normalizeList(names)
	}
}

// WithAppsSKUs filters apps by SKU(s).
func WithAppsSKUs(skus []string) AppsOption {
	return func(q *appsQuery) {
		q.skus = normalizeList(skus)
	}
}

// WithAppSearchKeywordsLimit sets the max number of app keywords to return.
func WithAppSearchKeywordsLimit(limit int) AppSearchKeywordsOption {
	return func(q *appSearchKeywordsQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithAppSearchKeywordsNextURL uses a next page URL directly.
func WithAppSearchKeywordsNextURL(next string) AppSearchKeywordsOption {
	return func(q *appSearchKeywordsQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

// WithAppSearchKeywordsPlatforms filters app keywords by platform(s).
func WithAppSearchKeywordsPlatforms(platforms []string) AppSearchKeywordsOption {
	return func(q *appSearchKeywordsQuery) {
		q.platforms = normalizeUpperList(platforms)
	}
}

// WithAppSearchKeywordsLocales filters app keywords by locale(s).
func WithAppSearchKeywordsLocales(locales []string) AppSearchKeywordsOption {
	return func(q *appSearchKeywordsQuery) {
		q.locales = normalizeList(locales)
	}
}

// WithAppClipsLimit sets the max number of App Clips to return.
func WithAppClipsLimit(limit int) AppClipsOption {
	return func(q *appClipsQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithAppClipsNextURL uses a next page URL directly.
func WithAppClipsNextURL(next string) AppClipsOption {
	return func(q *appClipsQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

// WithAppClipsBundleIDs filters App Clips by bundle ID(s).
func WithAppClipsBundleIDs(bundleIDs []string) AppClipsOption {
	return func(q *appClipsQuery) {
		q.bundleIDs = normalizeList(bundleIDs)
	}
}

// WithAppClipDefaultExperiencesLimit sets the max number of default experiences to return.
func WithAppClipDefaultExperiencesLimit(limit int) AppClipDefaultExperiencesOption {
	return func(q *appClipDefaultExperiencesQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithAppClipDefaultExperiencesNextURL uses a next page URL directly.
func WithAppClipDefaultExperiencesNextURL(next string) AppClipDefaultExperiencesOption {
	return func(q *appClipDefaultExperiencesQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

// WithAppClipDefaultExperiencesReleaseWithVersionExists filters by releaseWithAppStoreVersion existence.
func WithAppClipDefaultExperiencesReleaseWithVersionExists(value bool) AppClipDefaultExperiencesOption {
	return func(q *appClipDefaultExperiencesQuery) {
		q.releaseWithVersionExists = &value
	}
}

// WithAppClipDefaultExperienceInclude sets include for default experience detail.
func WithAppClipDefaultExperienceInclude(include []string) AppClipDefaultExperienceOption {
	return func(q *appClipDefaultExperienceQuery) {
		q.include = normalizeList(include)
	}
}

// WithAppClipDefaultExperienceLocalizationsLimit sets the max number of localizations to return.
func WithAppClipDefaultExperienceLocalizationsLimit(limit int) AppClipDefaultExperienceLocalizationsOption {
	return func(q *appClipDefaultExperienceLocalizationsQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithAppClipDefaultExperienceLocalizationsNextURL uses a next page URL directly.
func WithAppClipDefaultExperienceLocalizationsNextURL(next string) AppClipDefaultExperienceLocalizationsOption {
	return func(q *appClipDefaultExperienceLocalizationsQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

// WithAppClipDefaultExperienceLocalizationsLocales filters localizations by locale(s).
func WithAppClipDefaultExperienceLocalizationsLocales(locales []string) AppClipDefaultExperienceLocalizationsOption {
	return func(q *appClipDefaultExperienceLocalizationsQuery) {
		q.locales = normalizeList(locales)
	}
}

// WithAppClipAdvancedExperiencesLimit sets the max number of advanced experiences to return.
func WithAppClipAdvancedExperiencesLimit(limit int) AppClipAdvancedExperiencesOption {
	return func(q *appClipAdvancedExperiencesQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithAppClipAdvancedExperiencesNextURL uses a next page URL directly.
func WithAppClipAdvancedExperiencesNextURL(next string) AppClipAdvancedExperiencesOption {
	return func(q *appClipAdvancedExperiencesQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

// WithAppClipAdvancedExperiencesActions filters advanced experiences by action(s).
func WithAppClipAdvancedExperiencesActions(actions []string) AppClipAdvancedExperiencesOption {
	return func(q *appClipAdvancedExperiencesQuery) {
		q.actions = normalizeList(actions)
	}
}

// WithAppClipAdvancedExperiencesStatuses filters advanced experiences by status(es).
func WithAppClipAdvancedExperiencesStatuses(statuses []string) AppClipAdvancedExperiencesOption {
	return func(q *appClipAdvancedExperiencesQuery) {
		q.statuses = normalizeList(statuses)
	}
}

// WithAppClipAdvancedExperiencesPlaceStatuses filters advanced experiences by place status(es).
func WithAppClipAdvancedExperiencesPlaceStatuses(placeStatuses []string) AppClipAdvancedExperiencesOption {
	return func(q *appClipAdvancedExperiencesQuery) {
		q.placeStatuses = normalizeList(placeStatuses)
	}
}

// WithBetaAppClipInvocationInclude sets include for beta App Clip invocation detail.
func WithBetaAppClipInvocationInclude(include []string) BetaAppClipInvocationOption {
	return func(q *betaAppClipInvocationQuery) {
		q.include = normalizeList(include)
	}
}

// WithBetaAppClipInvocationLocalizationsLimit sets limit for included localizations.
func WithBetaAppClipInvocationLocalizationsLimit(limit int) BetaAppClipInvocationOption {
	return func(q *betaAppClipInvocationQuery) {
		if limit > 0 {
			q.localizationsLimit = limit
		}
	}
}

// WithAppTagsLimit sets the max number of app tags to return.
func WithAppTagsLimit(limit int) AppTagsOption {
	return func(q *appTagsQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithAppTagsNextURL uses a next page URL directly.
func WithAppTagsNextURL(next string) AppTagsOption {
	return func(q *appTagsQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

// WithAppTagsVisibleInAppStore filters app tags by visibility.
func WithAppTagsVisibleInAppStore(values []string) AppTagsOption {
	return func(q *appTagsQuery) {
		q.visibleInAppStore = normalizeList(values)
	}
}

// WithAppTagsSort sets the sort order for app tags.
func WithAppTagsSort(sort string) AppTagsOption {
	return func(q *appTagsQuery) {
		if strings.TrimSpace(sort) != "" {
			q.sort = strings.TrimSpace(sort)
		}
	}
}

// WithAppTagsFields sets fields[appTags] for app tag responses.
func WithAppTagsFields(fields []string) AppTagsOption {
	return func(q *appTagsQuery) {
		q.fields = normalizeList(fields)
	}
}

// WithAppTagsInclude sets include for app tag responses.
func WithAppTagsInclude(include []string) AppTagsOption {
	return func(q *appTagsQuery) {
		q.include = normalizeList(include)
	}
}

// WithAppTagsTerritoryFields sets fields[territories] for included territory responses.
func WithAppTagsTerritoryFields(fields []string) AppTagsOption {
	return func(q *appTagsQuery) {
		q.territoryFields = normalizeList(fields)
	}
}

// WithAppTagsTerritoryLimit sets limit[territories] for included territories.
func WithAppTagsTerritoryLimit(limit int) AppTagsOption {
	return func(q *appTagsQuery) {
		if limit > 0 {
			q.territoryLimit = limit
		}
	}
}

// WithNominationsLimit sets the max number of nominations to return.
func WithNominationsLimit(limit int) NominationsOption {
	return func(q *nominationsQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithNominationsNextURL uses a next page URL directly.
func WithNominationsNextURL(next string) NominationsOption {
	return func(q *nominationsQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

// WithNominationsTypes filters nominations by type.
func WithNominationsTypes(types []string) NominationsOption {
	return func(q *nominationsQuery) {
		q.types = normalizeUpperList(types)
	}
}

// WithNominationsStates filters nominations by state.
func WithNominationsStates(states []string) NominationsOption {
	return func(q *nominationsQuery) {
		q.states = normalizeUpperList(states)
	}
}

// WithNominationsRelatedApps filters nominations by related app ID(s).
func WithNominationsRelatedApps(appIDs []string) NominationsOption {
	return func(q *nominationsQuery) {
		q.relatedApps = normalizeList(appIDs)
	}
}

// WithNominationsSort sets the sort order for nominations.
func WithNominationsSort(sort string) NominationsOption {
	return func(q *nominationsQuery) {
		if strings.TrimSpace(sort) != "" {
			q.sort = strings.TrimSpace(sort)
		}
	}
}

// WithNominationsFields sets fields[nominations] for nominations responses.
func WithNominationsFields(fields []string) NominationsOption {
	return func(q *nominationsQuery) {
		q.fields = normalizeList(fields)
	}
}

// WithNominationsInclude sets include for nominations responses.
func WithNominationsInclude(include []string) NominationsOption {
	return func(q *nominationsQuery) {
		q.include = normalizeList(include)
	}
}

// WithNominationsInAppEventsLimit sets limit[inAppEvents] for included in-app events.
func WithNominationsInAppEventsLimit(limit int) NominationsOption {
	return func(q *nominationsQuery) {
		if limit > 0 {
			q.inAppEventsLimit = limit
		}
	}
}

// WithNominationsRelatedAppsLimit sets limit[relatedApps] for included related apps.
func WithNominationsRelatedAppsLimit(limit int) NominationsOption {
	return func(q *nominationsQuery) {
		if limit > 0 {
			q.relatedAppsLimit = limit
		}
	}
}

// WithNominationsSupportedTerritoriesLimit sets limit[supportedTerritories] for included territories.
func WithNominationsSupportedTerritoriesLimit(limit int) NominationsOption {
	return func(q *nominationsQuery) {
		if limit > 0 {
			q.supportedTerritoriesLimit = limit
		}
	}
}

// WithBuildsLimit sets the max number of builds to return.
func WithBuildsLimit(limit int) BuildsOption {
	return func(q *buildsQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithBuildsNextURL uses a next page URL directly.
func WithBuildsNextURL(next string) BuildsOption {
	return func(q *buildsQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

// WithBuildsSort sets the sort order for builds.
func WithBuildsSort(sort string) BuildsOption {
	return func(q *buildsQuery) {
		if strings.TrimSpace(sort) != "" {
			q.sort = strings.TrimSpace(sort)
		}
	}
}

// WithBuildsPreReleaseVersion filters builds by pre-release version ID.
func WithBuildsPreReleaseVersion(preReleaseVersionID string) BuildsOption {
	return func(q *buildsQuery) {
		if strings.TrimSpace(preReleaseVersionID) != "" {
			q.preReleaseVersionID = strings.TrimSpace(preReleaseVersionID)
		}
	}
}

// WithBuildBundlesLimit sets the max number of included build bundles to return.
func WithBuildBundlesLimit(limit int) BuildBundlesOption {
	return func(q *buildBundlesQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithBuildBundleFileSizesLimit sets the max number of file size items to return.
func WithBuildBundleFileSizesLimit(limit int) BuildBundleFileSizesOption {
	return func(q *buildBundleFileSizesQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithBuildBundleFileSizesNextURL uses a next page URL directly.
func WithBuildBundleFileSizesNextURL(next string) BuildBundleFileSizesOption {
	return func(q *buildBundleFileSizesQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

// WithBuildUploadsLimit sets the max number of build uploads to return.
func WithBuildUploadsLimit(limit int) BuildUploadsOption {
	return func(q *buildUploadsQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithBuildUploadsNextURL uses a next page URL directly.
func WithBuildUploadsNextURL(next string) BuildUploadsOption {
	return func(q *buildUploadsQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

// WithBuildUploadsCFBundleShortVersionStrings filters build uploads by CFBundleShortVersionString.
func WithBuildUploadsCFBundleShortVersionStrings(values []string) BuildUploadsOption {
	return func(q *buildUploadsQuery) {
		q.cfBundleShortVersions = normalizeList(values)
	}
}

// WithBuildUploadsCFBundleVersions filters build uploads by CFBundleVersion.
func WithBuildUploadsCFBundleVersions(values []string) BuildUploadsOption {
	return func(q *buildUploadsQuery) {
		q.cfBundleVersions = normalizeList(values)
	}
}

// WithBuildUploadsPlatforms filters build uploads by platform(s).
func WithBuildUploadsPlatforms(platforms []string) BuildUploadsOption {
	return func(q *buildUploadsQuery) {
		q.platforms = normalizeUpperList(platforms)
	}
}

// WithBuildUploadsStates filters build uploads by upload state(s).
func WithBuildUploadsStates(states []string) BuildUploadsOption {
	return func(q *buildUploadsQuery) {
		q.states = normalizeUpperList(states)
	}
}

// WithBuildUploadsSort sets the sort order for build uploads.
func WithBuildUploadsSort(sort string) BuildUploadsOption {
	return func(q *buildUploadsQuery) {
		if strings.TrimSpace(sort) != "" {
			q.sort = strings.TrimSpace(sort)
		}
	}
}

// WithBuildUploadFilesLimit sets the max number of build upload files to return.
func WithBuildUploadFilesLimit(limit int) BuildUploadFilesOption {
	return func(q *buildUploadFilesQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithBuildUploadFilesNextURL uses a next page URL directly.
func WithBuildUploadFilesNextURL(next string) BuildUploadFilesOption {
	return func(q *buildUploadFilesQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

// WithBuildIndividualTestersLimit sets the max number of build individual testers to return.
func WithBuildIndividualTestersLimit(limit int) BuildIndividualTestersOption {
	return func(q *buildIndividualTestersQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithBuildIndividualTestersNextURL uses a next page URL directly.
func WithBuildIndividualTestersNextURL(next string) BuildIndividualTestersOption {
	return func(q *buildIndividualTestersQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

// WithBetaAppClipInvocationsLimit sets the max number of App Clip invocations to return.
func WithBetaAppClipInvocationsLimit(limit int) BetaAppClipInvocationsOption {
	return func(q *betaAppClipInvocationsQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithBetaAppClipInvocationsNextURL uses a next page URL directly.
func WithBetaAppClipInvocationsNextURL(next string) BetaAppClipInvocationsOption {
	return func(q *betaAppClipInvocationsQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

// WithAppStoreVersionsLimit sets the max number of versions to return.
func WithAppStoreVersionsLimit(limit int) AppStoreVersionsOption {
	return func(q *appStoreVersionsQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithAppStoreVersionsNextURL uses a next page URL directly.
func WithAppStoreVersionsNextURL(next string) AppStoreVersionsOption {
	return func(q *appStoreVersionsQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

// WithAppStoreVersionsPlatforms filters versions by platform.
func WithAppStoreVersionsPlatforms(platforms []string) AppStoreVersionsOption {
	return func(q *appStoreVersionsQuery) {
		q.platforms = normalizeUpperList(platforms)
	}
}

// WithAppStoreVersionsVersionStrings filters versions by version string.
func WithAppStoreVersionsVersionStrings(versions []string) AppStoreVersionsOption {
	return func(q *appStoreVersionsQuery) {
		q.versionStrings = normalizeList(versions)
	}
}

// WithAppStoreVersionsStates filters versions by app store state.
func WithAppStoreVersionsStates(states []string) AppStoreVersionsOption {
	return func(q *appStoreVersionsQuery) {
		q.states = normalizeUpperList(states)
	}
}

// WithAppStoreVersionsInclude includes related resources for versions.
func WithAppStoreVersionsInclude(include []string) AppStoreVersionsOption {
	return func(q *appStoreVersionsQuery) {
		q.include = normalizeList(include)
	}
}

// WithAppStoreVersionInclude includes related resources for a version.
func WithAppStoreVersionInclude(include []string) AppStoreVersionOption {
	return func(q *appStoreVersionQuery) {
		q.include = normalizeList(include)
	}
}

// WithReviewSubmissionsLimit sets the max number of review submissions to return.
func WithReviewSubmissionsLimit(limit int) ReviewSubmissionsOption {
	return func(q *reviewSubmissionsQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithReviewSubmissionsNextURL uses a next page URL directly.
func WithReviewSubmissionsNextURL(next string) ReviewSubmissionsOption {
	return func(q *reviewSubmissionsQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

// WithReviewSubmissionsPlatforms filters review submissions by platform.
func WithReviewSubmissionsPlatforms(platforms []string) ReviewSubmissionsOption {
	return func(q *reviewSubmissionsQuery) {
		q.platforms = normalizeUpperList(platforms)
	}
}

// WithReviewSubmissionsStates filters review submissions by state.
func WithReviewSubmissionsStates(states []string) ReviewSubmissionsOption {
	return func(q *reviewSubmissionsQuery) {
		q.states = normalizeUpperList(states)
	}
}

// WithReviewSubmissionItemsLimit sets the max number of review submission items to return.
func WithReviewSubmissionItemsLimit(limit int) ReviewSubmissionItemsOption {
	return func(q *reviewSubmissionItemsQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithReviewSubmissionItemsNextURL uses a next page URL directly.
func WithReviewSubmissionItemsNextURL(next string) ReviewSubmissionItemsOption {
	return func(q *reviewSubmissionItemsQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

// WithPreReleaseVersionsPlatform filters pre-release versions by platform.
func WithPreReleaseVersionsPlatform(platform string) PreReleaseVersionsOption {
	return func(q *preReleaseVersionsQuery) {
		normalized := normalizeUpperCSVString(platform)
		if normalized != "" {
			q.platform = normalized
		}
	}
}

// WithPreReleaseVersionsVersion filters pre-release versions by version string.
func WithPreReleaseVersionsVersion(version string) PreReleaseVersionsOption {
	return func(q *preReleaseVersionsQuery) {
		normalized := normalizeCSVString(version)
		if normalized != "" {
			q.version = normalized
		}
	}
}

// WithPreReleaseVersionsLimit sets the max number of pre-release versions to return.
func WithPreReleaseVersionsLimit(limit int) PreReleaseVersionsOption {
	return func(q *preReleaseVersionsQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithPreReleaseVersionsNextURL uses a next page URL directly.
func WithPreReleaseVersionsNextURL(next string) PreReleaseVersionsOption {
	return func(q *preReleaseVersionsQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

// WithBetaGroupsLimit sets the max number of beta groups to return.
func WithBetaGroupsLimit(limit int) BetaGroupsOption {
	return func(q *betaGroupsQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithBetaGroupsNextURL uses a next page URL directly.
func WithBetaGroupsNextURL(next string) BetaGroupsOption {
	return func(q *betaGroupsQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

// WithBetaGroupBuildsLimit sets the max number of builds to return for a group.
func WithBetaGroupBuildsLimit(limit int) BetaGroupBuildsOption {
	return func(q *betaGroupBuildsQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithBetaGroupBuildsNextURL uses a next page URL directly.
func WithBetaGroupBuildsNextURL(next string) BetaGroupBuildsOption {
	return func(q *betaGroupBuildsQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

// WithBetaGroupTestersLimit sets the max number of testers to return for a group.
func WithBetaGroupTestersLimit(limit int) BetaGroupTestersOption {
	return func(q *betaGroupTestersQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithBetaGroupTestersNextURL uses a next page URL directly.
func WithBetaGroupTestersNextURL(next string) BetaGroupTestersOption {
	return func(q *betaGroupTestersQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

// WithBetaTestersLimit sets the max number of beta testers to return.
func WithBetaTestersLimit(limit int) BetaTestersOption {
	return func(q *betaTestersQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithBetaTestersNextURL uses a next page URL directly.
func WithBetaTestersNextURL(next string) BetaTestersOption {
	return func(q *betaTestersQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

// WithBetaTestersEmail filters beta testers by email.
func WithBetaTestersEmail(email string) BetaTestersOption {
	return func(q *betaTestersQuery) {
		q.email = strings.TrimSpace(email)
	}
}

// WithBetaTestersGroupIDs filters beta testers by beta group ID(s).
func WithBetaTestersGroupIDs(ids []string) BetaTestersOption {
	return func(q *betaTestersQuery) {
		q.groupIDs = normalizeList(ids)
	}
}

// WithBetaTestersBuildID filters beta testers by build ID.
func WithBetaTestersBuildID(buildID string) BetaTestersOption {
	return func(q *betaTestersQuery) {
		q.filterBuilds = strings.TrimSpace(buildID)
	}
}

// WithBetaTesterUsagesLimit sets the max number of beta tester usage records to return.
func WithBetaTesterUsagesLimit(limit int) BetaTesterUsagesOption {
	return func(q *betaTesterUsagesQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithBetaTesterUsagesNextURL uses a next page URL directly.
func WithBetaTesterUsagesNextURL(next string) BetaTesterUsagesOption {
	return func(q *betaTesterUsagesQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

// WithBetaTesterUsagesPeriod sets the reporting period for beta tester usage metrics.
func WithBetaTesterUsagesPeriod(period string) BetaTesterUsagesOption {
	return func(q *betaTesterUsagesQuery) {
		if strings.TrimSpace(period) != "" {
			q.period = strings.TrimSpace(period)
		}
	}
}

// WithBetaTesterUsagesAppID filters beta tester usage metrics by app ID.
func WithBetaTesterUsagesAppID(appID string) BetaTesterUsagesOption {
	return func(q *betaTesterUsagesQuery) {
		if strings.TrimSpace(appID) != "" {
			q.appID = strings.TrimSpace(appID)
		}
	}
}

// WithBundleIDsLimit sets the max number of bundle IDs to return.
func WithBundleIDsLimit(limit int) BundleIDsOption {
	return func(q *bundleIDsQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithBundleIDsNextURL uses a next page URL directly.
func WithBundleIDsNextURL(next string) BundleIDsOption {
	return func(q *bundleIDsQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

// WithBundleIDsFilterIdentifier filters bundle IDs by identifier (supports CSV).
func WithBundleIDsFilterIdentifier(identifier string) BundleIDsOption {
	return func(q *bundleIDsQuery) {
		normalized := normalizeCSVString(identifier)
		if normalized != "" {
			q.identifier = normalized
		}
	}
}

// WithPromotedPurchasesLimit sets the max number of promoted purchases to return.
func WithPromotedPurchasesLimit(limit int) PromotedPurchasesOption {
	return func(q *promotedPurchasesQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithPromotedPurchasesNextURL uses a next page URL directly.
func WithPromotedPurchasesNextURL(next string) PromotedPurchasesOption {
	return func(q *promotedPurchasesQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

// WithMerchantIDsLimit sets the max number of merchant IDs to return.
func WithMerchantIDsLimit(limit int) MerchantIDsOption {
	return func(q *merchantIDsQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithMerchantIDsNextURL uses a next page URL directly.
func WithMerchantIDsNextURL(next string) MerchantIDsOption {
	return func(q *merchantIDsQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

// WithMerchantIDsFilterIdentifier filters merchant IDs by identifier (supports CSV).
func WithMerchantIDsFilterIdentifier(identifier string) MerchantIDsOption {
	return func(q *merchantIDsQuery) {
		normalized := normalizeCSVString(identifier)
		if normalized != "" {
			q.identifier = normalized
		}
	}
}

// WithMerchantIDsFilterName filters merchant IDs by name (supports CSV).
func WithMerchantIDsFilterName(name string) MerchantIDsOption {
	return func(q *merchantIDsQuery) {
		normalized := normalizeCSVString(name)
		if normalized != "" {
			q.name = normalized
		}
	}
}

// WithMerchantIDsSort sets the sort order for merchant IDs.
func WithMerchantIDsSort(sort string) MerchantIDsOption {
	return func(q *merchantIDsQuery) {
		if strings.TrimSpace(sort) != "" {
			q.sort = strings.TrimSpace(sort)
		}
	}
}

// WithMerchantIDsFields sets fields[merchantIds] for merchant ID responses.
func WithMerchantIDsFields(fields []string) MerchantIDsOption {
	return func(q *merchantIDsQuery) {
		q.fields = normalizeList(fields)
	}
}

// WithMerchantIDsCertificateFields sets fields[certificates] for included certificates.
func WithMerchantIDsCertificateFields(fields []string) MerchantIDsOption {
	return func(q *merchantIDsQuery) {
		q.certificateFields = normalizeList(fields)
	}
}

// WithMerchantIDsInclude sets include for merchant ID responses.
func WithMerchantIDsInclude(include []string) MerchantIDsOption {
	return func(q *merchantIDsQuery) {
		q.include = normalizeList(include)
	}
}

// WithMerchantIDsCertificatesLimit sets limit[certificates] for included certificates.
func WithMerchantIDsCertificatesLimit(limit int) MerchantIDsOption {
	return func(q *merchantIDsQuery) {
		if limit > 0 {
			q.certificatesLimit = limit
		}
	}
}

// WithMerchantIDCertificatesLimit sets the max number of certificates to return.
func WithMerchantIDCertificatesLimit(limit int) MerchantIDCertificatesOption {
	return func(q *merchantIDCertificatesQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithMerchantIDCertificatesNextURL uses a next page URL directly.
func WithMerchantIDCertificatesNextURL(next string) MerchantIDCertificatesOption {
	return func(q *merchantIDCertificatesQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

// WithMerchantIDCertificatesFilterDisplayName filters certificates by display name (supports CSV).
func WithMerchantIDCertificatesFilterDisplayName(displayName string) MerchantIDCertificatesOption {
	return func(q *merchantIDCertificatesQuery) {
		normalized := normalizeCSVString(displayName)
		if normalized != "" {
			q.displayName = normalized
		}
	}
}

// WithMerchantIDCertificatesFilterCertificateTypes filters certificates by type (supports CSV).
func WithMerchantIDCertificatesFilterCertificateTypes(types string) MerchantIDCertificatesOption {
	return func(q *merchantIDCertificatesQuery) {
		normalized := normalizeUpperCSVString(types)
		if normalized != "" {
			q.certificateType = normalized
		}
	}
}

// WithMerchantIDCertificatesFilterSerialNumbers filters certificates by serial number (supports CSV).
func WithMerchantIDCertificatesFilterSerialNumbers(serialNumbers string) MerchantIDCertificatesOption {
	return func(q *merchantIDCertificatesQuery) {
		normalized := normalizeCSVString(serialNumbers)
		if normalized != "" {
			q.serialNumber = normalized
		}
	}
}

// WithMerchantIDCertificatesFilterIDs filters certificates by ID (supports CSV).
func WithMerchantIDCertificatesFilterIDs(ids string) MerchantIDCertificatesOption {
	return func(q *merchantIDCertificatesQuery) {
		normalized := normalizeCSVString(ids)
		if normalized != "" {
			q.ids = normalized
		}
	}
}

// WithMerchantIDCertificatesSort sets the sort order for merchant ID certificates.
func WithMerchantIDCertificatesSort(sort string) MerchantIDCertificatesOption {
	return func(q *merchantIDCertificatesQuery) {
		if strings.TrimSpace(sort) != "" {
			q.sort = strings.TrimSpace(sort)
		}
	}
}

// WithMerchantIDCertificatesFields sets fields[certificates] for certificate responses.
func WithMerchantIDCertificatesFields(fields []string) MerchantIDCertificatesOption {
	return func(q *merchantIDCertificatesQuery) {
		q.fields = normalizeList(fields)
	}
}

// WithMerchantIDCertificatesPassTypeFields sets fields[passTypeIds] for included pass type IDs.
func WithMerchantIDCertificatesPassTypeFields(fields []string) MerchantIDCertificatesOption {
	return func(q *merchantIDCertificatesQuery) {
		q.passTypeFields = normalizeList(fields)
	}
}

// WithMerchantIDCertificatesInclude sets include for merchant ID certificates responses.
func WithMerchantIDCertificatesInclude(include []string) MerchantIDCertificatesOption {
	return func(q *merchantIDCertificatesQuery) {
		q.include = normalizeList(include)
	}
}

// WithPassTypeIDsLimit sets the max number of pass type IDs to return.
func WithPassTypeIDsLimit(limit int) PassTypeIDsOption {
	return func(q *passTypeIDsQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithPassTypeIDsNextURL uses a next page URL directly.
func WithPassTypeIDsNextURL(next string) PassTypeIDsOption {
	return func(q *passTypeIDsQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

// WithPassTypeIDsFilterIDs filters pass type IDs by ID(s).
func WithPassTypeIDsFilterIDs(ids []string) PassTypeIDsOption {
	return func(q *passTypeIDsQuery) {
		normalized := normalizeList(ids)
		if len(normalized) > 0 {
			q.ids = strings.Join(normalized, ",")
		}
	}
}

// WithPassTypeIDsFilterName filters pass type IDs by name (supports CSV).
func WithPassTypeIDsFilterName(name string) PassTypeIDsOption {
	return func(q *passTypeIDsQuery) {
		normalized := normalizeCSVString(name)
		if normalized != "" {
			q.name = normalized
		}
	}
}

// WithPassTypeIDsFilterIdentifier filters pass type IDs by identifier (supports CSV).
func WithPassTypeIDsFilterIdentifier(identifier string) PassTypeIDsOption {
	return func(q *passTypeIDsQuery) {
		normalized := normalizeCSVString(identifier)
		if normalized != "" {
			q.identifier = normalized
		}
	}
}

// WithPassTypeIDsSort sets the sort order for pass type IDs.
func WithPassTypeIDsSort(sort string) PassTypeIDsOption {
	return func(q *passTypeIDsQuery) {
		if strings.TrimSpace(sort) != "" {
			q.sort = strings.TrimSpace(sort)
		}
	}
}

// WithPassTypeIDsFields sets fields[passTypeIds] for pass type ID responses.
func WithPassTypeIDsFields(fields []string) PassTypeIDsOption {
	return func(q *passTypeIDsQuery) {
		q.fields = normalizeList(fields)
	}
}

// WithPassTypeIDsCertificateFields sets fields[certificates] for included certificates.
func WithPassTypeIDsCertificateFields(fields []string) PassTypeIDsOption {
	return func(q *passTypeIDsQuery) {
		q.certificateFields = normalizeList(fields)
	}
}

// WithPassTypeIDsInclude sets include for pass type ID responses.
func WithPassTypeIDsInclude(include []string) PassTypeIDsOption {
	return func(q *passTypeIDsQuery) {
		q.include = normalizeList(include)
	}
}

// WithPassTypeIDsCertificatesLimit sets limit[certificates] for included certificates.
func WithPassTypeIDsCertificatesLimit(limit int) PassTypeIDsOption {
	return func(q *passTypeIDsQuery) {
		if limit > 0 {
			q.certificatesLimit = limit
		}
	}
}

// WithPassTypeIDFields sets fields[passTypeIds] for pass type ID responses.
func WithPassTypeIDFields(fields []string) PassTypeIDOption {
	return func(q *passTypeIDQuery) {
		q.fields = normalizeList(fields)
	}
}

// WithPassTypeIDCertificateFields sets fields[certificates] for included certificates.
func WithPassTypeIDCertificateFields(fields []string) PassTypeIDOption {
	return func(q *passTypeIDQuery) {
		q.certificateFields = normalizeList(fields)
	}
}

// WithPassTypeIDInclude sets include for pass type ID responses.
func WithPassTypeIDInclude(include []string) PassTypeIDOption {
	return func(q *passTypeIDQuery) {
		q.include = normalizeList(include)
	}
}

// WithPassTypeIDCertificatesIncludeLimit sets limit[certificates] for included certificates.
func WithPassTypeIDCertificatesIncludeLimit(limit int) PassTypeIDOption {
	return func(q *passTypeIDQuery) {
		if limit > 0 {
			q.certificatesLimit = limit
		}
	}
}

// WithPassTypeIDCertificatesLimit sets the max number of certificates to return.
func WithPassTypeIDCertificatesLimit(limit int) PassTypeIDCertificatesOption {
	return func(q *passTypeIDCertificatesQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithPassTypeIDCertificatesNextURL uses a next page URL directly.
func WithPassTypeIDCertificatesNextURL(next string) PassTypeIDCertificatesOption {
	return func(q *passTypeIDCertificatesQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

// WithPassTypeIDCertificatesFilterDisplayNames filters certificates by display name(s).
func WithPassTypeIDCertificatesFilterDisplayNames(names []string) PassTypeIDCertificatesOption {
	return func(q *passTypeIDCertificatesQuery) {
		q.displayNames = normalizeList(names)
	}
}

// WithPassTypeIDCertificatesFilterCertificateTypes filters certificates by type(s).
func WithPassTypeIDCertificatesFilterCertificateTypes(types []string) PassTypeIDCertificatesOption {
	return func(q *passTypeIDCertificatesQuery) {
		q.certificateTypes = normalizeUpperList(types)
	}
}

// WithPassTypeIDCertificatesFilterSerialNumbers filters certificates by serial number(s).
func WithPassTypeIDCertificatesFilterSerialNumbers(serials []string) PassTypeIDCertificatesOption {
	return func(q *passTypeIDCertificatesQuery) {
		q.serialNumbers = normalizeList(serials)
	}
}

// WithPassTypeIDCertificatesFilterIDs filters certificates by ID(s).
func WithPassTypeIDCertificatesFilterIDs(ids []string) PassTypeIDCertificatesOption {
	return func(q *passTypeIDCertificatesQuery) {
		q.ids = normalizeList(ids)
	}
}

// WithPassTypeIDCertificatesSort sets the sort order for certificates.
func WithPassTypeIDCertificatesSort(sort string) PassTypeIDCertificatesOption {
	return func(q *passTypeIDCertificatesQuery) {
		if strings.TrimSpace(sort) != "" {
			q.sort = strings.TrimSpace(sort)
		}
	}
}

// WithPassTypeIDCertificatesFields sets fields[certificates] for certificate responses.
func WithPassTypeIDCertificatesFields(fields []string) PassTypeIDCertificatesOption {
	return func(q *passTypeIDCertificatesQuery) {
		q.fields = normalizeList(fields)
	}
}

// WithPassTypeIDCertificatesPassTypeIDFields sets fields[passTypeIds] for included pass type IDs.
func WithPassTypeIDCertificatesPassTypeIDFields(fields []string) PassTypeIDCertificatesOption {
	return func(q *passTypeIDCertificatesQuery) {
		q.passTypeIDFields = normalizeList(fields)
	}
}

// WithPassTypeIDCertificatesInclude sets include for certificate responses.
func WithPassTypeIDCertificatesInclude(include []string) PassTypeIDCertificatesOption {
	return func(q *passTypeIDCertificatesQuery) {
		q.include = normalizeList(include)
	}
}

// WithBundleIDCapabilitiesLimit sets the max number of capabilities to return.
func WithBundleIDCapabilitiesLimit(limit int) BundleIDCapabilitiesOption {
	return func(q *bundleIDCapabilitiesQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithBundleIDCapabilitiesNextURL uses a next page URL directly.
func WithBundleIDCapabilitiesNextURL(next string) BundleIDCapabilitiesOption {
	return func(q *bundleIDCapabilitiesQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

// WithCertificatesLimit sets the max number of certificates to return.
func WithCertificatesLimit(limit int) CertificatesOption {
	return func(q *certificatesQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithCertificatesNextURL uses a next page URL directly.
func WithCertificatesNextURL(next string) CertificatesOption {
	return func(q *certificatesQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

// WithCertificatesTypes filters certificates by type.
func WithCertificatesTypes(types []string) CertificatesOption {
	return func(q *certificatesQuery) {
		q.certificateTypes = normalizeUpperList(types)
	}
}

// WithCertificatesInclude sets include for certificate responses.
func WithCertificatesInclude(include []string) CertificatesOption {
	return func(q *certificatesQuery) {
		q.include = normalizeList(include)
	}
}

// WithCertificatesFilterType filters certificates by certificate type (supports CSV).
func WithCertificatesFilterType(certType string) CertificatesOption {
	return func(q *certificatesQuery) {
		normalized := normalizeUpperCSVString(certType)
		if normalized == "" {
			return
		}
		q.certificateTypes = strings.Split(normalized, ",")
	}
}

// WithProfilesLimit sets the max number of profiles to return.
func WithProfilesLimit(limit int) ProfilesOption {
	return func(q *profilesQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithProfilesNextURL uses a next page URL directly.
func WithProfilesNextURL(next string) ProfilesOption {
	return func(q *profilesQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

// WithProfilesTypes filters profiles by profile type.
func WithProfilesTypes(types []string) ProfilesOption {
	return func(q *profilesQuery) {
		q.profileTypes = normalizeUpperList(types)
	}
}

// WithProfilesInclude sets include for profile responses.
func WithProfilesInclude(include []string) ProfilesOption {
	return func(q *profilesQuery) {
		q.include = normalizeList(include)
	}
}

// WithProfilesFilterBundleID filters profiles by bundle ID.
func WithProfilesFilterBundleID(bundleID string) ProfilesOption {
	return func(q *profilesQuery) {
		if strings.TrimSpace(bundleID) != "" {
			q.bundleID = strings.TrimSpace(bundleID)
		}
	}
}

// WithProfilesFilterType filters profiles by profile type (supports CSV).
func WithProfilesFilterType(profileType string) ProfilesOption {
	return func(q *profilesQuery) {
		normalized := normalizeUpperCSVString(profileType)
		if normalized == "" {
			return
		}
		q.profileTypes = strings.Split(normalized, ",")
	}
}

// WithProfileCertificatesLimit sets the max number of profile certificates to return.
func WithProfileCertificatesLimit(limit int) ProfileCertificatesOption {
	return func(q *profileCertificatesQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithProfileCertificatesNextURL uses a next page URL directly.
func WithProfileCertificatesNextURL(next string) ProfileCertificatesOption {
	return func(q *profileCertificatesQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

// WithProfileDevicesLimit sets the max number of profile devices to return.
func WithProfileDevicesLimit(limit int) ProfileDevicesOption {
	return func(q *profileDevicesQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithProfileDevicesNextURL uses a next page URL directly.
func WithProfileDevicesNextURL(next string) ProfileDevicesOption {
	return func(q *profileDevicesQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

// WithUsersLimit sets the max number of users to return.
func WithUsersLimit(limit int) UsersOption {
	return func(q *usersQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithUsersNextURL uses a next page URL directly.
func WithUsersNextURL(next string) UsersOption {
	return func(q *usersQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

// WithUsersEmail filters users by email/username.
func WithUsersEmail(email string) UsersOption {
	return func(q *usersQuery) {
		q.email = strings.TrimSpace(email)
	}
}

// WithUsersRoles filters users by roles.
func WithUsersRoles(roles []string) UsersOption {
	return func(q *usersQuery) {
		q.roles = normalizeList(roles)
	}
}

// WithUsersInclude sets include for user responses.
func WithUsersInclude(include []string) UsersOption {
	return func(q *usersQuery) {
		q.include = normalizeList(include)
	}
}

// WithUserVisibleAppsLimit sets the max number of visible apps to return.
func WithUserVisibleAppsLimit(limit int) UserVisibleAppsOption {
	return func(q *userVisibleAppsQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithUserVisibleAppsNextURL uses a next page URL directly.
func WithUserVisibleAppsNextURL(next string) UserVisibleAppsOption {
	return func(q *userVisibleAppsQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

// WithActorsLimit sets the max number of actors to return.
func WithActorsLimit(limit int) ActorsOption {
	return func(q *actorsQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithActorsNextURL uses a next page URL directly.
func WithActorsNextURL(next string) ActorsOption {
	return func(q *actorsQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

// WithActorsIDs filters actors by id(s).
func WithActorsIDs(ids []string) ActorsOption {
	return func(q *actorsQuery) {
		q.ids = normalizeList(ids)
	}
}

// WithActorsFields limits actor fields in the response.
func WithActorsFields(fields []string) ActorsOption {
	return func(q *actorsQuery) {
		q.fields = normalizeList(fields)
	}
}

// WithDevicesLimit sets the max number of devices to return.
func WithDevicesLimit(limit int) DevicesOption {
	return func(q *devicesQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithDevicesFilterUDIDs filters devices by UDID(s).
func WithDevicesFilterUDIDs(udids []string) DevicesOption {
	return WithDevicesUDIDs(udids)
}

// WithDevicesFilterPlatforms filters devices by platform(s).
func WithDevicesFilterPlatforms(platforms []string) DevicesOption {
	return func(q *devicesQuery) {
		normalized := normalizeUpperList(platforms)
		if len(normalized) == 0 {
			return
		}
		q.platforms = normalized
	}
}

// WithDevicesFilterStatuses filters devices by status (e.g., ENABLED, DISABLED).
func WithDevicesFilterStatuses(statuses []string) DevicesOption {
	return func(q *devicesQuery) {
		normalized := normalizeUpperList(statuses)
		if len(normalized) == 0 {
			return
		}
		q.status = strings.Join(normalized, ",")
	}
}

// WithDevicesNextURL uses a next page URL directly.
func WithDevicesNextURL(next string) DevicesOption {
	return func(q *devicesQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

// WithDevicesNames filters devices by name(s).
func WithDevicesNames(names []string) DevicesOption {
	return func(q *devicesQuery) {
		q.names = normalizeList(names)
	}
}

// WithDevicesPlatform filters devices by platform.
func WithDevicesPlatform(platform string) DevicesOption {
	return func(q *devicesQuery) {
		if strings.TrimSpace(platform) != "" {
			q.platforms = []string{strings.ToUpper(strings.TrimSpace(platform))}
		}
	}
}

// WithDevicesPlatforms filters devices by platform(s).
func WithDevicesPlatforms(platforms []string) DevicesOption {
	return func(q *devicesQuery) {
		q.platforms = normalizeUpperList(platforms)
	}
}

// WithDevicesStatus filters devices by status.
func WithDevicesStatus(status string) DevicesOption {
	return func(q *devicesQuery) {
		if strings.TrimSpace(status) != "" {
			q.status = strings.ToUpper(strings.TrimSpace(status))
		}
	}
}

// WithDevicesUDIDs filters devices by UDID(s).
func WithDevicesUDIDs(udids []string) DevicesOption {
	return func(q *devicesQuery) {
		q.udids = normalizeList(udids)
	}
}

// WithDevicesIDs filters devices by ID(s).
func WithDevicesIDs(ids []string) DevicesOption {
	return func(q *devicesQuery) {
		q.ids = normalizeList(ids)
	}
}

// WithDevicesSort sets the sort order for devices.
func WithDevicesSort(sort string) DevicesOption {
	return func(q *devicesQuery) {
		if strings.TrimSpace(sort) != "" {
			q.sort = strings.TrimSpace(sort)
		}
	}
}

// WithDevicesFields sets fields[devices] for device responses.
func WithDevicesFields(fields []string) DevicesOption {
	return func(q *devicesQuery) {
		q.fields = normalizeList(fields)
	}
}

// WithUserInvitationsLimit sets the max number of invitations to return.
func WithUserInvitationsLimit(limit int) UserInvitationsOption {
	return func(q *userInvitationsQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithUserInvitationsNextURL uses a next page URL directly.
func WithUserInvitationsNextURL(next string) UserInvitationsOption {
	return func(q *userInvitationsQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

// WithBetaAppReviewDetailsLimit sets the max number of review detail records to return.
func WithBetaAppReviewDetailsLimit(limit int) BetaAppReviewDetailsOption {
	return func(q *betaAppReviewDetailsQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithBetaAppReviewDetailsNextURL uses a next page URL directly.
func WithBetaAppReviewDetailsNextURL(next string) BetaAppReviewDetailsOption {
	return func(q *betaAppReviewDetailsQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

// WithBetaAppReviewSubmissionsLimit sets the max number of submissions to return.
func WithBetaAppReviewSubmissionsLimit(limit int) BetaAppReviewSubmissionsOption {
	return func(q *betaAppReviewSubmissionsQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithBetaAppReviewSubmissionsNextURL uses a next page URL directly.
func WithBetaAppReviewSubmissionsNextURL(next string) BetaAppReviewSubmissionsOption {
	return func(q *betaAppReviewSubmissionsQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

// WithBetaAppReviewSubmissionsBuildIDs filters submissions by build ID(s).
func WithBetaAppReviewSubmissionsBuildIDs(ids []string) BetaAppReviewSubmissionsOption {
	return func(q *betaAppReviewSubmissionsQuery) {
		q.buildIDs = normalizeList(ids)
	}
}

// WithBuildBetaDetailsLimit sets the max number of build beta details to return.
func WithBuildBetaDetailsLimit(limit int) BuildBetaDetailsOption {
	return func(q *buildBetaDetailsQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithBuildBetaDetailsNextURL uses a next page URL directly.
func WithBuildBetaDetailsNextURL(next string) BuildBetaDetailsOption {
	return func(q *buildBetaDetailsQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

// WithBuildBetaDetailsBuildIDs filters build beta details by build ID(s).
func WithBuildBetaDetailsBuildIDs(ids []string) BuildBetaDetailsOption {
	return func(q *buildBetaDetailsQuery) {
		q.buildIDs = normalizeList(ids)
	}
}

// WithBetaRecruitmentCriterionOptionsLimit sets the max number of criterion options to return.
func WithBetaRecruitmentCriterionOptionsLimit(limit int) BetaRecruitmentCriterionOptionsOption {
	return func(q *betaRecruitmentCriterionOptionsQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithBetaRecruitmentCriterionOptionsNextURL uses a next page URL directly.
func WithBetaRecruitmentCriterionOptionsNextURL(next string) BetaRecruitmentCriterionOptionsOption {
	return func(q *betaRecruitmentCriterionOptionsQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

// WithAppStoreVersionLocalizationsLimit sets the max number of localizations to return.
func WithAppStoreVersionLocalizationsLimit(limit int) AppStoreVersionLocalizationsOption {
	return func(q *appStoreVersionLocalizationsQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithAppStoreVersionLocalizationsNextURL uses a next page URL directly.
func WithAppStoreVersionLocalizationsNextURL(next string) AppStoreVersionLocalizationsOption {
	return func(q *appStoreVersionLocalizationsQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

// WithAppStoreVersionLocalizationLocales filters version localizations by locale.
func WithAppStoreVersionLocalizationLocales(locales []string) AppStoreVersionLocalizationsOption {
	return func(q *appStoreVersionLocalizationsQuery) {
		q.locales = normalizeList(locales)
	}
}

// WithBetaAppLocalizationsLimit sets the max number of beta app localizations to return.
func WithBetaAppLocalizationsLimit(limit int) BetaAppLocalizationsOption {
	return func(q *betaAppLocalizationsQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithBetaAppLocalizationsNextURL uses a next page URL directly.
func WithBetaAppLocalizationsNextURL(next string) BetaAppLocalizationsOption {
	return func(q *betaAppLocalizationsQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

// WithBetaAppLocalizationLocales filters beta app localizations by locale.
func WithBetaAppLocalizationLocales(locales []string) BetaAppLocalizationsOption {
	return func(q *betaAppLocalizationsQuery) {
		q.locales = normalizeList(locales)
	}
}

// WithBetaAppLocalizationAppIDs filters beta app localizations by app ID(s).
func WithBetaAppLocalizationAppIDs(ids []string) BetaAppLocalizationsOption {
	return func(q *betaAppLocalizationsQuery) {
		q.appIDs = normalizeList(ids)
	}
}

// WithBetaBuildLocalizationsLimit sets the max number of beta build localizations to return.
func WithBetaBuildLocalizationsLimit(limit int) BetaBuildLocalizationsOption {
	return func(q *betaBuildLocalizationsQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithBetaBuildLocalizationsNextURL uses a next page URL directly.
func WithBetaBuildLocalizationsNextURL(next string) BetaBuildLocalizationsOption {
	return func(q *betaBuildLocalizationsQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

// WithBetaBuildLocalizationLocales filters beta build localizations by locale.
func WithBetaBuildLocalizationLocales(locales []string) BetaBuildLocalizationsOption {
	return func(q *betaBuildLocalizationsQuery) {
		q.locales = normalizeList(locales)
	}
}

// WithBetaBuildUsagesLimit sets the max number of beta build usage records to return.
func WithBetaBuildUsagesLimit(limit int) BetaBuildUsagesOption {
	return func(q *betaBuildUsagesQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithBetaBuildUsagesNextURL uses a next page URL directly.
func WithBetaBuildUsagesNextURL(next string) BetaBuildUsagesOption {
	return func(q *betaBuildUsagesQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

// WithAppInfoLocalizationsLimit sets the max number of app info localizations to return.
func WithAppInfoLocalizationsLimit(limit int) AppInfoLocalizationsOption {
	return func(q *appInfoLocalizationsQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithAppInfoLocalizationsNextURL uses a next page URL directly.
func WithAppInfoLocalizationsNextURL(next string) AppInfoLocalizationsOption {
	return func(q *appInfoLocalizationsQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

// WithAppInfoLocalizationLocales filters app info localizations by locale.
func WithAppInfoLocalizationLocales(locales []string) AppInfoLocalizationsOption {
	return func(q *appInfoLocalizationsQuery) {
		q.locales = normalizeList(locales)
	}
}

// WithAppInfoInclude includes related resources for an app info.
func WithAppInfoInclude(include []string) AppInfoOption {
	return func(q *appInfoQuery) {
		q.include = normalizeList(include)
	}
}

// WithTerritoriesLimit sets the max number of territories to return.
func WithTerritoriesLimit(limit int) TerritoriesOption {
	return func(q *territoriesQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithTerritoriesNextURL uses a next page URL directly.
func WithTerritoriesNextURL(next string) TerritoriesOption {
	return func(q *territoriesQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

// WithTerritoriesFields sets fields[territories] for territory responses.
func WithTerritoriesFields(fields []string) TerritoriesOption {
	return func(q *territoriesQuery) {
		q.fields = normalizeList(fields)
	}
}

// WithAndroidToIosAppMappingDetailsLimit sets the max number of mappings to return.
func WithAndroidToIosAppMappingDetailsLimit(limit int) AndroidToIosAppMappingDetailsOption {
	return func(q *androidToIosAppMappingDetailsQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithAndroidToIosAppMappingDetailsNextURL uses a next page URL directly.
func WithAndroidToIosAppMappingDetailsNextURL(next string) AndroidToIosAppMappingDetailsOption {
	return func(q *androidToIosAppMappingDetailsQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

// WithAndroidToIosAppMappingDetailsFields sets fields[androidToIosAppMappingDetails].
func WithAndroidToIosAppMappingDetailsFields(fields []string) AndroidToIosAppMappingDetailsOption {
	return func(q *androidToIosAppMappingDetailsQuery) {
		q.fields = normalizeList(fields)
	}
}

// WithPerfPowerMetricsPlatforms filters metrics by platform(s).
func WithPerfPowerMetricsPlatforms(platforms []string) PerfPowerMetricsOption {
	return func(q *perfPowerMetricsQuery) {
		q.platforms = normalizeUpperList(platforms)
	}
}

// WithPerfPowerMetricsMetricTypes filters metrics by metric type(s).
func WithPerfPowerMetricsMetricTypes(types []string) PerfPowerMetricsOption {
	return func(q *perfPowerMetricsQuery) {
		q.metricTypes = normalizeUpperList(types)
	}
}

// WithPerfPowerMetricsDeviceTypes filters metrics by device type(s).
func WithPerfPowerMetricsDeviceTypes(types []string) PerfPowerMetricsOption {
	return func(q *perfPowerMetricsQuery) {
		q.deviceTypes = normalizeList(types)
	}
}

// WithDiagnosticSignaturesLimit sets the max number of diagnostic signatures to return.
func WithDiagnosticSignaturesLimit(limit int) DiagnosticSignaturesOption {
	return func(q *diagnosticSignaturesQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithDiagnosticSignaturesNextURL uses a next page URL directly.
func WithDiagnosticSignaturesNextURL(next string) DiagnosticSignaturesOption {
	return func(q *diagnosticSignaturesQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

// WithDiagnosticSignaturesDiagnosticTypes filters diagnostic signatures by type.
func WithDiagnosticSignaturesDiagnosticTypes(types []string) DiagnosticSignaturesOption {
	return func(q *diagnosticSignaturesQuery) {
		q.diagnosticTypes = normalizeUpperList(types)
	}
}

// WithDiagnosticSignaturesFields sets fields[diagnosticSignatures] for diagnostic signatures.
func WithDiagnosticSignaturesFields(fields []string) DiagnosticSignaturesOption {
	return func(q *diagnosticSignaturesQuery) {
		q.fields = normalizeList(fields)
	}
}

// WithDiagnosticLogsLimit sets the max number of diagnostic logs to return.
func WithDiagnosticLogsLimit(limit int) DiagnosticLogsOption {
	return func(q *diagnosticLogsQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithTerritoryAvailabilitiesLimit sets the max number of territory availabilities to return.
func WithTerritoryAvailabilitiesLimit(limit int) TerritoryAvailabilitiesOption {
	return func(q *territoryAvailabilitiesQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithTerritoryAvailabilitiesNextURL uses a next page URL directly.
func WithTerritoryAvailabilitiesNextURL(next string) TerritoryAvailabilitiesOption {
	return func(q *territoryAvailabilitiesQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

// WithLinkagesLimit sets the max number of linkages to return.
func WithLinkagesLimit(limit int) LinkagesOption {
	return func(q *linkagesQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithLinkagesNextURL uses a next page URL directly.
func WithLinkagesNextURL(next string) LinkagesOption {
	return func(q *linkagesQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

// WithPricePointsLimit sets the max number of price points to return.
func WithPricePointsLimit(limit int) PricePointsOption {
	return func(q *pricePointsQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithPricePointsNextURL uses a next page URL directly.
func WithPricePointsNextURL(next string) PricePointsOption {
	return func(q *pricePointsQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

// WithPricePointsTerritory filters app price points by territory.
func WithPricePointsTerritory(territory string) PricePointsOption {
	return func(q *pricePointsQuery) {
		if strings.TrimSpace(territory) != "" {
			q.territory = strings.ToUpper(strings.TrimSpace(territory))
		}
	}
}

// WithAppCustomProductPagesLimit sets the max number of custom product pages to return.
func WithAppCustomProductPagesLimit(limit int) AppCustomProductPagesOption {
	return func(q *appCustomProductPagesQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithAppCustomProductPagesNextURL uses a next page URL directly.
func WithAppCustomProductPagesNextURL(next string) AppCustomProductPagesOption {
	return func(q *appCustomProductPagesQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

// WithAppCustomProductPageVersionsLimit sets the max number of versions to return.
func WithAppCustomProductPageVersionsLimit(limit int) AppCustomProductPageVersionsOption {
	return func(q *appCustomProductPageVersionsQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithAppCustomProductPageVersionsNextURL uses a next page URL directly.
func WithAppCustomProductPageVersionsNextURL(next string) AppCustomProductPageVersionsOption {
	return func(q *appCustomProductPageVersionsQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

// WithAppCustomProductPageLocalizationsLimit sets the max number of localizations to return.
func WithAppCustomProductPageLocalizationsLimit(limit int) AppCustomProductPageLocalizationsOption {
	return func(q *appCustomProductPageLocalizationsQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithAppCustomProductPageLocalizationsNextURL uses a next page URL directly.
func WithAppCustomProductPageLocalizationsNextURL(next string) AppCustomProductPageLocalizationsOption {
	return func(q *appCustomProductPageLocalizationsQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

// WithAppCustomProductPageLocalizationPreviewSetsLimit sets the max number of preview sets to return.
func WithAppCustomProductPageLocalizationPreviewSetsLimit(limit int) AppCustomProductPageLocalizationPreviewSetsOption {
	return func(q *appCustomProductPageLocalizationPreviewSetsQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithAppCustomProductPageLocalizationPreviewSetsNextURL uses a next page URL directly.
func WithAppCustomProductPageLocalizationPreviewSetsNextURL(next string) AppCustomProductPageLocalizationPreviewSetsOption {
	return func(q *appCustomProductPageLocalizationPreviewSetsQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

// WithAppCustomProductPageLocalizationScreenshotSetsLimit sets the max number of screenshot sets to return.
func WithAppCustomProductPageLocalizationScreenshotSetsLimit(limit int) AppCustomProductPageLocalizationScreenshotSetsOption {
	return func(q *appCustomProductPageLocalizationScreenshotSetsQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithAppCustomProductPageLocalizationScreenshotSetsNextURL uses a next page URL directly.
func WithAppCustomProductPageLocalizationScreenshotSetsNextURL(next string) AppCustomProductPageLocalizationScreenshotSetsOption {
	return func(q *appCustomProductPageLocalizationScreenshotSetsQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

// WithAppStoreVersionLocalizationPreviewSetsLimit sets the max number of preview sets to return.
func WithAppStoreVersionLocalizationPreviewSetsLimit(limit int) AppStoreVersionLocalizationPreviewSetsOption {
	return func(q *appStoreVersionLocalizationPreviewSetsQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithAppStoreVersionLocalizationPreviewSetsNextURL uses a next page URL directly.
func WithAppStoreVersionLocalizationPreviewSetsNextURL(next string) AppStoreVersionLocalizationPreviewSetsOption {
	return func(q *appStoreVersionLocalizationPreviewSetsQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

// WithAppStoreVersionLocalizationScreenshotSetsLimit sets the max number of screenshot sets to return.
func WithAppStoreVersionLocalizationScreenshotSetsLimit(limit int) AppStoreVersionLocalizationScreenshotSetsOption {
	return func(q *appStoreVersionLocalizationScreenshotSetsQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithAppStoreVersionLocalizationScreenshotSetsNextURL uses a next page URL directly.
func WithAppStoreVersionLocalizationScreenshotSetsNextURL(next string) AppStoreVersionLocalizationScreenshotSetsOption {
	return func(q *appStoreVersionLocalizationScreenshotSetsQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

// WithAppStoreVersionExperimentsLimit sets the max number of experiments to return.
func WithAppStoreVersionExperimentsLimit(limit int) AppStoreVersionExperimentsOption {
	return func(q *appStoreVersionExperimentsQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithAppStoreVersionExperimentsNextURL uses a next page URL directly.
func WithAppStoreVersionExperimentsNextURL(next string) AppStoreVersionExperimentsOption {
	return func(q *appStoreVersionExperimentsQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

// WithAppStoreVersionExperimentsState filters experiments by state.
func WithAppStoreVersionExperimentsState(states []string) AppStoreVersionExperimentsOption {
	return func(q *appStoreVersionExperimentsQuery) {
		q.states = normalizeUpperList(states)
	}
}

// WithAppStoreVersionExperimentsV2Limit sets the max number of experiments to return (v2).
func WithAppStoreVersionExperimentsV2Limit(limit int) AppStoreVersionExperimentsV2Option {
	return func(q *appStoreVersionExperimentsV2Query) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithAppStoreVersionExperimentsV2NextURL uses a next page URL directly (v2).
func WithAppStoreVersionExperimentsV2NextURL(next string) AppStoreVersionExperimentsV2Option {
	return func(q *appStoreVersionExperimentsV2Query) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

// WithAppStoreVersionExperimentsV2State filters experiments by state (v2).
func WithAppStoreVersionExperimentsV2State(states []string) AppStoreVersionExperimentsV2Option {
	return func(q *appStoreVersionExperimentsV2Query) {
		q.states = normalizeUpperList(states)
	}
}

// WithAppStoreVersionExperimentTreatmentsLimit sets the max number of treatments to return.
func WithAppStoreVersionExperimentTreatmentsLimit(limit int) AppStoreVersionExperimentTreatmentsOption {
	return func(q *appStoreVersionExperimentTreatmentsQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithAppStoreVersionExperimentTreatmentsNextURL uses a next page URL directly.
func WithAppStoreVersionExperimentTreatmentsNextURL(next string) AppStoreVersionExperimentTreatmentsOption {
	return func(q *appStoreVersionExperimentTreatmentsQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

// WithAppStoreVersionExperimentTreatmentLocalizationsLimit sets the max number of treatment localizations to return.
func WithAppStoreVersionExperimentTreatmentLocalizationsLimit(limit int) AppStoreVersionExperimentTreatmentLocalizationsOption {
	return func(q *appStoreVersionExperimentTreatmentLocalizationsQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithAppStoreVersionExperimentTreatmentLocalizationsNextURL uses a next page URL directly.
func WithAppStoreVersionExperimentTreatmentLocalizationsNextURL(next string) AppStoreVersionExperimentTreatmentLocalizationsOption {
	return func(q *appStoreVersionExperimentTreatmentLocalizationsQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}
