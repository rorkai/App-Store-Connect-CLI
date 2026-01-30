package asc

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

type listQuery struct {
	limit   int
	nextURL string
}

type feedbackQuery struct {
	listQuery
	deviceModels              []string
	osVersions                []string
	appPlatforms              []string
	devicePlatforms           []string
	buildIDs                  []string
	buildPreReleaseVersionIDs []string
	testerIDs                 []string
	sort                      string
	includeScreenshots        bool
}

type crashQuery struct {
	listQuery
	deviceModels              []string
	osVersions                []string
	appPlatforms              []string
	devicePlatforms           []string
	buildIDs                  []string
	buildPreReleaseVersionIDs []string
	testerIDs                 []string
	sort                      string
}

type reviewQuery struct {
	listQuery
	rating    int
	territory string
	sort      string
}

type appsQuery struct {
	listQuery
	sort      string
	bundleIDs []string
	names     []string
	skus      []string
}

type appSearchKeywordsQuery struct {
	listQuery
	platforms []string
	locales   []string
}

type appClipsQuery struct {
	listQuery
	bundleIDs []string
}

type appClipDefaultExperiencesQuery struct {
	listQuery
	releaseWithVersionExists *bool
}

type appClipDefaultExperienceQuery struct {
	include []string
}

type appClipDefaultExperienceLocalizationsQuery struct {
	listQuery
	locales []string
}

type appClipAdvancedExperiencesQuery struct {
	listQuery
	actions       []string
	statuses      []string
	placeStatuses []string
}

type appTagsQuery struct {
	listQuery
	visibleInAppStore []string
	sort              string
	fields            []string
	include           []string
	territoryFields   []string
	territoryLimit    int
}

type nominationsQuery struct {
	listQuery
	types                     []string
	states                    []string
	relatedApps               []string
	sort                      string
	fields                    []string
	include                   []string
	inAppEventsLimit          int
	relatedAppsLimit          int
	supportedTerritoriesLimit int
}

type buildsQuery struct {
	listQuery
	sort                string
	preReleaseVersionID string
}

type buildUploadsQuery struct {
	listQuery
	cfBundleShortVersions []string
	cfBundleVersions      []string
	platforms             []string
	states                []string
	sort                  string
}

type buildBundlesQuery struct {
	limit int
}

type buildBundleFileSizesQuery struct {
	listQuery
}

type buildUploadFilesQuery struct {
	listQuery
}

type buildIndividualTestersQuery struct {
	listQuery
}

type betaAppClipInvocationsQuery struct {
	listQuery
}

type betaAppClipInvocationQuery struct {
	include            []string
	localizationsLimit int
}

type subscriptionOfferCodeOneTimeUseCodesQuery struct {
	listQuery
}

type marketplaceWebhooksQuery struct {
	listQuery
	fields []string
}
type alternativeDistributionDomainsQuery struct {
	listQuery
	fields []string
}

type alternativeDistributionKeysQuery struct {
	listQuery
	fields    []string
	existsApp *bool
}

type alternativeDistributionPackageVersionsQuery struct {
	listQuery
}

type alternativeDistributionPackageVariantsQuery struct {
	listQuery
	fields []string
}

type alternativeDistributionPackageDeltasQuery struct {
	listQuery
	fields []string
}

type webhooksQuery struct {
	listQuery
	fields    []string
	appFields []string
	include   []string
}

type webhookDeliveriesQuery struct {
	listQuery
	deliveryStates []string
	createdAfter   []string
	createdBefore  []string
	fields         []string
	eventFields    []string
	include        []string
}

type backgroundAssetsQuery struct {
	listQuery
	archived             []string
	assetPackIdentifiers []string
}

type backgroundAssetVersionsQuery struct {
	listQuery
}

type backgroundAssetUploadFilesQuery struct {
	listQuery
}

type winBackOffersQuery struct {
	listQuery
	fields      []string
	priceFields []string
	include     []string
	pricesLimit int
}

type winBackOfferPricesQuery struct {
	listQuery
	territoryIDs                 []string
	fields                       []string
	territoryFields              []string
	subscriptionPricePointFields []string
	include                      []string
}

type appStoreVersionsQuery struct {
	listQuery
	platforms      []string
	versionStrings []string
	states         []string
	include        []string
}

type appStoreVersionQuery struct {
	include []string
}

type reviewSubmissionsQuery struct {
	listQuery
	platforms []string
	states    []string
}

type reviewSubmissionItemsQuery struct {
	listQuery
}

type preReleaseVersionsQuery struct {
	listQuery
	platform string
	version  string
}

type appStoreVersionLocalizationsQuery struct {
	listQuery
	locales []string
}

type betaAppLocalizationsQuery struct {
	listQuery
	locales []string
	appIDs  []string
}

type betaBuildLocalizationsQuery struct {
	listQuery
	locales []string
}

type betaBuildUsagesQuery struct {
	listQuery
}

type betaTesterUsagesQuery struct {
	listQuery
	period string
	appID  string
}

type appInfoLocalizationsQuery struct {
	listQuery
	locales []string
}

type appInfoQuery struct {
	include []string
}

type appCustomProductPagesQuery struct {
	listQuery
}

type appCustomProductPageVersionsQuery struct {
	listQuery
}

type appCustomProductPageLocalizationsQuery struct {
	listQuery
}

type appCustomProductPageLocalizationPreviewSetsQuery struct {
	listQuery
}

type appCustomProductPageLocalizationScreenshotSetsQuery struct {
	listQuery
}

type appStoreVersionLocalizationPreviewSetsQuery struct {
	listQuery
}

type appStoreVersionLocalizationScreenshotSetsQuery struct {
	listQuery
}

type appStoreVersionExperimentsQuery struct {
	listQuery
	states []string
}

type appStoreVersionExperimentsV2Query struct {
	listQuery
	states []string
}

type appStoreVersionExperimentTreatmentsQuery struct {
	listQuery
}

type appStoreVersionExperimentTreatmentLocalizationsQuery struct {
	listQuery
}

type betaGroupsQuery struct {
	listQuery
}

type betaGroupBuildsQuery struct {
	listQuery
}

type betaGroupTestersQuery struct {
	listQuery
}

type betaTestersQuery struct {
	listQuery
	email        string
	groupIDs     []string
	filterBuilds string
}

type bundleIDsQuery struct {
	listQuery
	identifier string
}

type promotedPurchasesQuery struct {
	listQuery
}

type merchantIDsQuery struct {
	listQuery
	name              string
	identifier        string
	sort              string
	fields            []string
	certificateFields []string
	include           []string
	certificatesLimit int
}

type bundleIDCapabilitiesQuery struct {
	listQuery
}

type passTypeIDsQuery struct {
	listQuery
	ids               string
	identifier        string
	name              string
	sort              string
	fields            []string
	certificateFields []string
	include           []string
	certificatesLimit int
}

type passTypeIDQuery struct {
	fields            []string
	certificateFields []string
	include           []string
	certificatesLimit int
}

type passTypeIDCertificatesQuery struct {
	listQuery
	displayNames     []string
	certificateTypes []string
	serialNumbers    []string
	ids              []string
	sort             string
	fields           []string
	passTypeIDFields []string
	include          []string
}

type certificatesQuery struct {
	listQuery
	certificateTypes []string
	include          []string
}

type merchantIDCertificatesQuery struct {
	listQuery
	displayName     string
	certificateType string
	serialNumber    string
	ids             string
	sort            string
	fields          []string
	passTypeFields  []string
	include         []string
}

type profilesQuery struct {
	listQuery
	bundleID     string
	profileTypes []string
	include      []string
}

type usersQuery struct {
	listQuery
	email   string
	roles   []string
	include []string
}

type profileCertificatesQuery struct {
	listQuery
}

type profileDevicesQuery struct {
	listQuery
}

type userVisibleAppsQuery struct {
	listQuery
}

type actorsQuery struct {
	listQuery
	ids    []string
	fields []string
}

type devicesQuery struct {
	listQuery
	names     []string
	platforms []string
	status    string
	udids     []string
	ids       []string
	sort      string
	fields    []string
}

type userInvitationsQuery struct {
	listQuery
}

type territoriesQuery struct {
	listQuery
	fields []string
}

type androidToIosAppMappingDetailsQuery struct {
	listQuery
	fields []string
}

type perfPowerMetricsQuery struct {
	platforms   []string
	metricTypes []string
	deviceTypes []string
}

type diagnosticSignaturesQuery struct {
	listQuery
	diagnosticTypes []string
	fields          []string
}

type diagnosticLogsQuery struct {
	listQuery
}

type territoryAvailabilitiesQuery struct {
	listQuery
}

type linkagesQuery struct {
	listQuery
}

type pricePointsQuery struct {
	listQuery
	territory string
}

type accessibilityDeclarationsQuery struct {
	listQuery
	deviceFamilies []string
	states         []string
	fields         []string
}

type appStoreReviewAttachmentsQuery struct {
	listQuery
	fieldsAttachments   []string
	fieldsReviewDetails []string
	include             []string
}

type appEncryptionDeclarationsQuery struct {
	listQuery
	appID          string
	buildIDs       []string
	fields         []string
	documentFields []string
	include        []string
	buildLimit     int
}

type betaAppReviewDetailsQuery struct {
	listQuery
}

type betaAppReviewSubmissionsQuery struct {
	listQuery
	buildIDs []string
}

type buildBetaDetailsQuery struct {
	listQuery
	buildIDs []string
}

type betaRecruitmentCriterionOptionsQuery struct {
	listQuery
}

func buildReviewQuery(opts []ReviewOption) string {
	query := &reviewQuery{}
	for _, opt := range opts {
		opt(query)
	}

	values := url.Values{}
	if query.territory != "" {
		values.Set("filter[territory]", query.territory)
	}
	if query.rating >= 1 && query.rating <= 5 {
		values.Set("filter[rating]", fmt.Sprintf("%d", query.rating))
	}
	if query.sort != "" {
		values.Set("sort", query.sort)
	}
	addLimit(values, query.limit)

	return values.Encode()
}

func buildAppsQuery(query *appsQuery) string {
	values := url.Values{}
	addCSV(values, "filter[bundleId]", query.bundleIDs)
	addCSV(values, "filter[name]", query.names)
	addCSV(values, "filter[sku]", query.skus)
	if query.sort != "" {
		values.Set("sort", query.sort)
	}
	addLimit(values, query.limit)
	return values.Encode()
}

func buildAppClipsQuery(query *appClipsQuery) string {
	values := url.Values{}
	addCSV(values, "filter[bundleId]", query.bundleIDs)
	addLimit(values, query.limit)
	return values.Encode()
}

func buildAppClipDefaultExperiencesQuery(query *appClipDefaultExperiencesQuery) string {
	values := url.Values{}
	if query.releaseWithVersionExists != nil {
		values.Set("exists[releaseWithAppStoreVersion]", strconv.FormatBool(*query.releaseWithVersionExists))
	}
	addLimit(values, query.limit)
	return values.Encode()
}

func buildAppClipDefaultExperienceQuery(query *appClipDefaultExperienceQuery) string {
	values := url.Values{}
	addCSV(values, "include", query.include)
	return values.Encode()
}

func buildAppClipDefaultExperienceLocalizationsQuery(query *appClipDefaultExperienceLocalizationsQuery) string {
	values := url.Values{}
	addCSV(values, "filter[locale]", query.locales)
	addLimit(values, query.limit)
	return values.Encode()
}

func buildAppClipAdvancedExperiencesQuery(query *appClipAdvancedExperiencesQuery) string {
	values := url.Values{}
	addCSV(values, "filter[action]", query.actions)
	addCSV(values, "filter[status]", query.statuses)
	addCSV(values, "filter[placeStatus]", query.placeStatuses)
	addLimit(values, query.limit)
	return values.Encode()
}

func buildBetaAppClipInvocationQuery(query *betaAppClipInvocationQuery) string {
	values := url.Values{}
	addCSV(values, "include", query.include)
	if query.localizationsLimit > 0 {
		values.Set("limit[betaAppClipInvocationLocalizations]", strconv.Itoa(query.localizationsLimit))
	}
	return values.Encode()
}

func buildAppTagsQuery(query *appTagsQuery) string {
	values := url.Values{}
	addCSV(values, "filter[visibleInAppStore]", query.visibleInAppStore)
	if query.sort != "" {
		values.Set("sort", query.sort)
	}
	addCSV(values, "fields[appTags]", query.fields)
	addCSV(values, "fields[territories]", query.territoryFields)
	addCSV(values, "include", query.include)
	addLimit(values, query.limit)
	if query.territoryLimit > 0 {
		values.Set("limit[territories]", strconv.Itoa(query.territoryLimit))
	}
	return values.Encode()
}

func buildNominationsQuery(query *nominationsQuery) string {
	values := url.Values{}
	addCSV(values, "filter[type]", query.types)
	addCSV(values, "filter[state]", query.states)
	addCSV(values, "filter[relatedApps]", query.relatedApps)
	if query.sort != "" {
		values.Set("sort", query.sort)
	}
	addCSV(values, "fields[nominations]", query.fields)
	addCSV(values, "include", query.include)
	addLimit(values, query.limit)
	if query.inAppEventsLimit > 0 {
		values.Set("limit[inAppEvents]", strconv.Itoa(query.inAppEventsLimit))
	}
	if query.relatedAppsLimit > 0 {
		values.Set("limit[relatedApps]", strconv.Itoa(query.relatedAppsLimit))
	}
	if query.supportedTerritoriesLimit > 0 {
		values.Set("limit[supportedTerritories]", strconv.Itoa(query.supportedTerritoriesLimit))
	}
	return values.Encode()
}

func buildNominationsDetailQuery(query *nominationsQuery) string {
	values := url.Values{}
	addCSV(values, "fields[nominations]", query.fields)
	addCSV(values, "include", query.include)
	if query.inAppEventsLimit > 0 {
		values.Set("limit[inAppEvents]", strconv.Itoa(query.inAppEventsLimit))
	}
	if query.relatedAppsLimit > 0 {
		values.Set("limit[relatedApps]", strconv.Itoa(query.relatedAppsLimit))
	}
	if query.supportedTerritoriesLimit > 0 {
		values.Set("limit[supportedTerritories]", strconv.Itoa(query.supportedTerritoriesLimit))
	}
	return values.Encode()
}

func buildFeedbackQuery(query *feedbackQuery) string {
	values := url.Values{}
	if query.includeScreenshots {
		values.Set("fields[betaFeedbackScreenshotSubmissions]", strings.Join([]string{
			"createdDate",
			"comment",
			"email",
			"deviceModel",
			"osVersion",
			"appPlatform",
			"devicePlatform",
			"screenshots",
		}, ","))
	}
	addCSV(values, "filter[deviceModel]", query.deviceModels)
	addCSV(values, "filter[osVersion]", query.osVersions)
	addCSV(values, "filter[appPlatform]", query.appPlatforms)
	addCSV(values, "filter[devicePlatform]", query.devicePlatforms)
	addCSV(values, "filter[build]", query.buildIDs)
	addCSV(values, "filter[build.preReleaseVersion]", query.buildPreReleaseVersionIDs)
	addCSV(values, "filter[tester]", query.testerIDs)
	if query.sort != "" {
		values.Set("sort", query.sort)
	}
	addLimit(values, query.limit)
	return values.Encode()
}

func buildCrashQuery(query *crashQuery) string {
	values := url.Values{}
	addCSV(values, "filter[deviceModel]", query.deviceModels)
	addCSV(values, "filter[osVersion]", query.osVersions)
	addCSV(values, "filter[appPlatform]", query.appPlatforms)
	addCSV(values, "filter[devicePlatform]", query.devicePlatforms)
	addCSV(values, "filter[build]", query.buildIDs)
	addCSV(values, "filter[build.preReleaseVersion]", query.buildPreReleaseVersionIDs)
	addCSV(values, "filter[tester]", query.testerIDs)
	if query.sort != "" {
		values.Set("sort", query.sort)
	}
	addLimit(values, query.limit)
	return values.Encode()
}

func buildBetaGroupsQuery(query *betaGroupsQuery) string {
	values := url.Values{}
	addLimit(values, query.limit)
	return values.Encode()
}

func buildBetaGroupBuildsQuery(query *betaGroupBuildsQuery) string {
	values := url.Values{}
	addLimit(values, query.limit)
	return values.Encode()
}

func buildBetaGroupTestersQuery(query *betaGroupTestersQuery) string {
	values := url.Values{}
	addLimit(values, query.limit)
	return values.Encode()
}

func buildBetaTestersQuery(appID string, query *betaTestersQuery) string {
	values := url.Values{}
	// API allows only one relationship filter, so prefer builds over apps if provided
	if strings.TrimSpace(query.filterBuilds) != "" {
		values.Set("filter[builds]", strings.TrimSpace(query.filterBuilds))
	} else if strings.TrimSpace(appID) != "" {
		values.Set("filter[apps]", strings.TrimSpace(appID))
	}
	if strings.TrimSpace(query.email) != "" {
		values.Set("filter[email]", strings.TrimSpace(query.email))
	}
	addCSV(values, "filter[betaGroups]", query.groupIDs)
	addLimit(values, query.limit)
	return values.Encode()
}

func buildBundleIDsQuery(query *bundleIDsQuery) string {
	values := url.Values{}
	if strings.TrimSpace(query.identifier) != "" {
		values.Set("filter[identifier]", strings.TrimSpace(query.identifier))
	}
	addLimit(values, query.limit)
	return values.Encode()
}

func buildPromotedPurchasesQuery(query *promotedPurchasesQuery) string {
	values := url.Values{}
	addLimit(values, query.limit)
	return values.Encode()
}

func buildMerchantIDsQuery(query *merchantIDsQuery) string {
	values := url.Values{}
	if strings.TrimSpace(query.name) != "" {
		values.Set("filter[name]", strings.TrimSpace(query.name))
	}
	if strings.TrimSpace(query.identifier) != "" {
		values.Set("filter[identifier]", strings.TrimSpace(query.identifier))
	}
	if strings.TrimSpace(query.sort) != "" {
		values.Set("sort", strings.TrimSpace(query.sort))
	}
	addCSV(values, "fields[merchantIds]", query.fields)
	addCSV(values, "fields[certificates]", query.certificateFields)
	addCSV(values, "include", query.include)
	if query.certificatesLimit > 0 {
		values.Set("limit[certificates]", strconv.Itoa(query.certificatesLimit))
	}
	addLimit(values, query.limit)
	return values.Encode()
}

func buildPassTypeIDsQuery(query *passTypeIDsQuery) string {
	values := url.Values{}
	if strings.TrimSpace(query.ids) != "" {
		values.Set("filter[id]", strings.TrimSpace(query.ids))
	}
	if strings.TrimSpace(query.identifier) != "" {
		values.Set("filter[identifier]", strings.TrimSpace(query.identifier))
	}
	if strings.TrimSpace(query.name) != "" {
		values.Set("filter[name]", strings.TrimSpace(query.name))
	}
	if strings.TrimSpace(query.sort) != "" {
		values.Set("sort", strings.TrimSpace(query.sort))
	}
	addCSV(values, "fields[passTypeIds]", query.fields)
	addCSV(values, "fields[certificates]", query.certificateFields)
	addCSV(values, "include", query.include)
	addLimit(values, query.limit)
	if query.certificatesLimit > 0 {
		values.Set("limit[certificates]", strconv.Itoa(query.certificatesLimit))
	}
	return values.Encode()
}

func buildPassTypeIDQuery(query *passTypeIDQuery) string {
	values := url.Values{}
	addCSV(values, "fields[passTypeIds]", query.fields)
	addCSV(values, "fields[certificates]", query.certificateFields)
	addCSV(values, "include", query.include)
	if query.certificatesLimit > 0 {
		values.Set("limit[certificates]", strconv.Itoa(query.certificatesLimit))
	}
	return values.Encode()
}

func buildBundleIDCapabilitiesQuery(_ *bundleIDCapabilitiesQuery) string {
	// Bundle ID capabilities endpoint does not support limit/pagination params.
	return ""
}

func buildCertificatesQuery(query *certificatesQuery) string {
	values := url.Values{}
	addCSV(values, "filter[certificateType]", query.certificateTypes)
	addCSV(values, "include", query.include)
	addLimit(values, query.limit)
	return values.Encode()
}

func buildMerchantIDCertificatesQuery(query *merchantIDCertificatesQuery) string {
	values := url.Values{}
	if strings.TrimSpace(query.displayName) != "" {
		values.Set("filter[displayName]", strings.TrimSpace(query.displayName))
	}
	if strings.TrimSpace(query.certificateType) != "" {
		values.Set("filter[certificateType]", strings.TrimSpace(query.certificateType))
	}
	if strings.TrimSpace(query.serialNumber) != "" {
		values.Set("filter[serialNumber]", strings.TrimSpace(query.serialNumber))
	}
	if strings.TrimSpace(query.ids) != "" {
		values.Set("filter[id]", strings.TrimSpace(query.ids))
	}
	if strings.TrimSpace(query.sort) != "" {
		values.Set("sort", strings.TrimSpace(query.sort))
	}
	addCSV(values, "fields[certificates]", query.fields)
	addCSV(values, "fields[passTypeIds]", query.passTypeFields)
	addCSV(values, "include", query.include)
	addLimit(values, query.limit)
	return values.Encode()
}

func buildPassTypeIDCertificatesQuery(query *passTypeIDCertificatesQuery) string {
	values := url.Values{}
	addCSV(values, "filter[displayName]", query.displayNames)
	addCSV(values, "filter[certificateType]", query.certificateTypes)
	addCSV(values, "filter[serialNumber]", query.serialNumbers)
	addCSV(values, "filter[id]", query.ids)
	if strings.TrimSpace(query.sort) != "" {
		values.Set("sort", strings.TrimSpace(query.sort))
	}
	addCSV(values, "fields[certificates]", query.fields)
	addCSV(values, "fields[passTypeIds]", query.passTypeIDFields)
	addCSV(values, "include", query.include)
	addLimit(values, query.limit)
	return values.Encode()
}

func buildProfilesQuery(query *profilesQuery) string {
	values := url.Values{}
	if strings.TrimSpace(query.bundleID) != "" {
		values.Set("filter[bundleId]", strings.TrimSpace(query.bundleID))
	}
	addCSV(values, "filter[profileType]", query.profileTypes)
	addCSV(values, "include", query.include)
	addLimit(values, query.limit)
	return values.Encode()
}

func buildUsersQuery(query *usersQuery) string {
	values := url.Values{}
	if strings.TrimSpace(query.email) != "" {
		values.Set("filter[username]", strings.TrimSpace(query.email))
	}
	addCSV(values, "filter[roles]", query.roles)
	addCSV(values, "include", query.include)
	addLimit(values, query.limit)
	return values.Encode()
}

func buildProfileCertificatesQuery(query *profileCertificatesQuery) string {
	values := url.Values{}
	addLimit(values, query.limit)
	return values.Encode()
}

func buildProfileDevicesQuery(query *profileDevicesQuery) string {
	values := url.Values{}
	addLimit(values, query.limit)
	return values.Encode()
}

func buildUserVisibleAppsQuery(query *userVisibleAppsQuery) string {
	values := url.Values{}
	addLimit(values, query.limit)
	return values.Encode()
}

func buildActorsQuery(query *actorsQuery) string {
	values := url.Values{}
	addCSV(values, "filter[id]", query.ids)
	addCSV(values, "fields[actors]", query.fields)
	addLimit(values, query.limit)
	return values.Encode()
}

func buildActorsFieldsQuery(fields []string) string {
	values := url.Values{}
	addCSV(values, "fields[actors]", fields)
	return values.Encode()
}

func buildDevicesQuery(query *devicesQuery) string {
	values := url.Values{}
	addCSV(values, "filter[name]", query.names)
	addCSV(values, "filter[platform]", query.platforms)
	if strings.TrimSpace(query.status) != "" {
		values.Set("filter[status]", strings.TrimSpace(query.status))
	}
	addCSV(values, "filter[udid]", query.udids)
	addCSV(values, "filter[id]", query.ids)
	if query.sort != "" {
		values.Set("sort", query.sort)
	}
	addCSV(values, "fields[devices]", query.fields)
	addLimit(values, query.limit)
	return values.Encode()
}

func buildDevicesFieldsQuery(fields []string) string {
	values := url.Values{}
	addCSV(values, "fields[devices]", fields)
	return values.Encode()
}

func buildAccessibilityDeclarationsQuery(query *accessibilityDeclarationsQuery) string {
	values := url.Values{}
	addCSV(values, "filter[deviceFamily]", query.deviceFamilies)
	addCSV(values, "filter[state]", query.states)
	addCSV(values, "fields[accessibilityDeclarations]", query.fields)
	addLimit(values, query.limit)
	return values.Encode()
}

func buildAccessibilityDeclarationsFieldsQuery(fields []string) string {
	values := url.Values{}
	addCSV(values, "fields[accessibilityDeclarations]", fields)
	return values.Encode()
}

func buildAppStoreReviewAttachmentsQuery(query *appStoreReviewAttachmentsQuery) string {
	values := url.Values{}
	addCSV(values, "fields[appStoreReviewAttachments]", query.fieldsAttachments)
	addCSV(values, "fields[appStoreReviewDetails]", query.fieldsReviewDetails)
	addCSV(values, "include", query.include)
	addLimit(values, query.limit)
	return values.Encode()
}

func buildAppEncryptionDeclarationsQuery(query *appEncryptionDeclarationsQuery) string {
	values := url.Values{}
	if strings.TrimSpace(query.appID) != "" {
		values.Set("filter[app]", strings.TrimSpace(query.appID))
	}
	addCSV(values, "filter[builds]", query.buildIDs)
	addCSV(values, "fields[appEncryptionDeclarations]", query.fields)
	addCSV(values, "fields[appEncryptionDeclarationDocuments]", query.documentFields)
	addCSV(values, "include", query.include)
	addLimit(values, query.limit)
	if query.buildLimit > 0 {
		values.Set("limit[builds]", strconv.Itoa(query.buildLimit))
	}
	return values.Encode()
}

func buildAppEncryptionDeclarationDocumentFieldsQuery(fields []string) string {
	values := url.Values{}
	addCSV(values, "fields[appEncryptionDeclarationDocuments]", fields)
	return values.Encode()
}

func buildUserInvitationsQuery(query *userInvitationsQuery) string {
	values := url.Values{}
	addLimit(values, query.limit)
	return values.Encode()
}

func buildBetaAppReviewDetailsQuery(appID string, query *betaAppReviewDetailsQuery) string {
	values := url.Values{}
	if strings.TrimSpace(appID) != "" {
		values.Set("filter[app]", strings.TrimSpace(appID))
	}
	addLimit(values, query.limit)
	return values.Encode()
}

func buildBetaAppReviewSubmissionsQuery(query *betaAppReviewSubmissionsQuery) string {
	values := url.Values{}
	addCSV(values, "filter[build]", query.buildIDs)
	addLimit(values, query.limit)
	return values.Encode()
}

func buildBuildBetaDetailsQuery(query *buildBetaDetailsQuery) string {
	values := url.Values{}
	addCSV(values, "filter[build]", query.buildIDs)
	addLimit(values, query.limit)
	return values.Encode()
}

func buildBuildBundlesQuery(query *buildBundlesQuery) string {
	values := url.Values{}
	values.Set("include", "buildBundles")
	if query.limit > 0 {
		values.Set("limit[buildBundles]", strconv.Itoa(query.limit))
	}
	return values.Encode()
}

func buildBuildBundleFileSizesQuery(query *buildBundleFileSizesQuery) string {
	values := url.Values{}
	addLimit(values, query.limit)
	return values.Encode()
}

func buildBuildUploadsQuery(query *buildUploadsQuery) string {
	values := url.Values{}
	addCSV(values, "filter[cfBundleShortVersionString]", query.cfBundleShortVersions)
	addCSV(values, "filter[cfBundleVersion]", query.cfBundleVersions)
	addCSV(values, "filter[platform]", query.platforms)
	addCSV(values, "filter[state]", query.states)
	if query.sort != "" {
		values.Set("sort", query.sort)
	}
	addLimit(values, query.limit)
	return values.Encode()
}

func buildBuildUploadFilesQuery(query *buildUploadFilesQuery) string {
	values := url.Values{}
	addLimit(values, query.limit)
	return values.Encode()
}

func buildBuildIndividualTestersQuery(query *buildIndividualTestersQuery) string {
	values := url.Values{}
	addLimit(values, query.limit)
	return values.Encode()
}

func buildBetaAppClipInvocationsQuery(query *betaAppClipInvocationsQuery) string {
	values := url.Values{}
	addLimit(values, query.limit)
	return values.Encode()
}

func buildBetaRecruitmentCriterionOptionsQuery(query *betaRecruitmentCriterionOptionsQuery) string {
	values := url.Values{}
	addLimit(values, query.limit)
	return values.Encode()
}

func buildSubscriptionOfferCodeOneTimeUseCodesQuery(query *subscriptionOfferCodeOneTimeUseCodesQuery) string {
	values := url.Values{}
	addLimit(values, query.limit)
	return values.Encode()
}

func buildMarketplaceSearchDetailsFieldsQuery(fields []string) string {
	values := url.Values{}
	addCSV(values, "fields[marketplaceSearchDetails]", fields)
	return values.Encode()
}

func buildMarketplaceWebhooksQuery(query *marketplaceWebhooksQuery) string {
	values := url.Values{}
	addCSV(values, "fields[marketplaceWebhooks]", query.fields)
	addLimit(values, query.limit)
	return values.Encode()
}

func buildAlternativeDistributionDomainsQuery(query *alternativeDistributionDomainsQuery) string {
	values := url.Values{}
	addCSV(values, "fields[alternativeDistributionDomains]", query.fields)
	addLimit(values, query.limit)
	return values.Encode()
}

func buildAlternativeDistributionKeysQuery(query *alternativeDistributionKeysQuery) string {
	values := url.Values{}
	addCSV(values, "fields[alternativeDistributionKeys]", query.fields)
	if query.existsApp != nil {
		values.Set("exists[app]", strconv.FormatBool(*query.existsApp))
	}
	addLimit(values, query.limit)
	return values.Encode()
}

func buildAlternativeDistributionPackageVersionsQuery(query *alternativeDistributionPackageVersionsQuery) string {
	values := url.Values{}
	addLimit(values, query.limit)
	return values.Encode()
}

func buildAlternativeDistributionPackageVariantsQuery(query *alternativeDistributionPackageVariantsQuery) string {
	values := url.Values{}
	addCSV(values, "fields[alternativeDistributionPackageVariants]", query.fields)
	addLimit(values, query.limit)
	return values.Encode()
}

func buildAlternativeDistributionPackageDeltasQuery(query *alternativeDistributionPackageDeltasQuery) string {
	values := url.Values{}
	addCSV(values, "fields[alternativeDistributionPackageDeltas]", query.fields)
	addLimit(values, query.limit)
	return values.Encode()
}

func buildWebhooksQuery(query *webhooksQuery) string {
	values := url.Values{}
	addCSV(values, "fields[webhooks]", query.fields)
	addCSV(values, "fields[apps]", query.appFields)
	addCSV(values, "include", query.include)
	addLimit(values, query.limit)
	return values.Encode()
}

func buildWebhookDeliveriesQuery(query *webhookDeliveriesQuery) string {
	values := url.Values{}
	addCSV(values, "filter[deliveryState]", query.deliveryStates)
	addCSV(values, "filter[createdDateGreaterThanOrEqualTo]", query.createdAfter)
	addCSV(values, "filter[createdDateLessThan]", query.createdBefore)
	addCSV(values, "fields[webhookDeliveries]", query.fields)
	addCSV(values, "fields[webhookEvents]", query.eventFields)
	addCSV(values, "include", query.include)
	addLimit(values, query.limit)
	return values.Encode()
}

func buildBackgroundAssetsQuery(query *backgroundAssetsQuery) string {
	values := url.Values{}
	addCSV(values, "filter[archived]", query.archived)
	addCSV(values, "filter[assetPackIdentifier]", query.assetPackIdentifiers)
	addLimit(values, query.limit)
	return values.Encode()
}

func buildBackgroundAssetVersionsQuery(query *backgroundAssetVersionsQuery) string {
	values := url.Values{}
	addLimit(values, query.limit)
	return values.Encode()
}

func buildBackgroundAssetUploadFilesQuery(query *backgroundAssetUploadFilesQuery) string {
	values := url.Values{}
	addLimit(values, query.limit)
	return values.Encode()
}

func buildWinBackOffersQuery(query *winBackOffersQuery) string {
	values := url.Values{}
	addCSV(values, "fields[winBackOffers]", query.fields)
	addCSV(values, "fields[winBackOfferPrices]", query.priceFields)
	addCSV(values, "include", query.include)
	addLimit(values, query.limit)
	if query.pricesLimit > 0 {
		values.Set("limit[prices]", strconv.Itoa(query.pricesLimit))
	}
	return values.Encode()
}

func buildWinBackOfferPricesQuery(query *winBackOfferPricesQuery) string {
	values := url.Values{}
	addCSV(values, "filter[territory]", query.territoryIDs)
	addCSV(values, "fields[winBackOfferPrices]", query.fields)
	addCSV(values, "fields[territories]", query.territoryFields)
	addCSV(values, "fields[subscriptionPricePoints]", query.subscriptionPricePointFields)
	addCSV(values, "include", query.include)
	addLimit(values, query.limit)
	return values.Encode()
}

func buildAppSearchKeywordsQuery(query *appSearchKeywordsQuery) string {
	values := url.Values{}
	addCSV(values, "filter[platform]", query.platforms)
	addCSV(values, "filter[locale]", query.locales)
	addLimit(values, query.limit)
	return values.Encode()
}

func buildAppStoreVersionsQuery(query *appStoreVersionsQuery) string {
	values := url.Values{}
	addCSV(values, "filter[platform]", query.platforms)
	addCSV(values, "filter[versionString]", query.versionStrings)
	addCSV(values, "filter[appStoreState]", query.states)
	addCSV(values, "include", query.include)
	addLimit(values, query.limit)
	return values.Encode()
}

func buildAppStoreVersionQuery(query *appStoreVersionQuery) string {
	values := url.Values{}
	addCSV(values, "include", query.include)
	return values.Encode()
}

func buildReviewSubmissionsQuery(query *reviewSubmissionsQuery) string {
	values := url.Values{}
	addCSV(values, "filter[platform]", query.platforms)
	addCSV(values, "filter[state]", query.states)
	addLimit(values, query.limit)
	return values.Encode()
}

func buildReviewSubmissionItemsQuery(query *reviewSubmissionItemsQuery) string {
	values := url.Values{}
	addLimit(values, query.limit)
	return values.Encode()
}

func buildPreReleaseVersionsQuery(appID string, query *preReleaseVersionsQuery) string {
	values := url.Values{}
	if strings.TrimSpace(appID) != "" {
		values.Set("filter[app]", strings.TrimSpace(appID))
	}
	if strings.TrimSpace(query.platform) != "" {
		values.Set("filter[platform]", strings.TrimSpace(query.platform))
	}
	if strings.TrimSpace(query.version) != "" {
		values.Set("filter[version]", strings.TrimSpace(query.version))
	}
	addLimit(values, query.limit)
	return values.Encode()
}

func buildAppStoreVersionLocalizationsQuery(query *appStoreVersionLocalizationsQuery) string {
	values := url.Values{}
	addCSV(values, "filter[locale]", query.locales)
	addLimit(values, query.limit)
	return values.Encode()
}

func buildBetaAppLocalizationsQuery(query *betaAppLocalizationsQuery) string {
	values := url.Values{}
	addCSV(values, "filter[locale]", query.locales)
	addCSV(values, "filter[app]", query.appIDs)
	addLimit(values, query.limit)
	return values.Encode()
}

func buildBetaBuildLocalizationsQuery(query *betaBuildLocalizationsQuery) string {
	values := url.Values{}
	addCSV(values, "filter[locale]", query.locales)
	addLimit(values, query.limit)
	return values.Encode()
}

func buildBetaBuildUsagesQuery(query *betaBuildUsagesQuery) string {
	values := url.Values{}
	addLimit(values, query.limit)
	return values.Encode()
}

func buildBetaTesterUsagesQuery(query *betaTesterUsagesQuery) string {
	values := url.Values{}
	if strings.TrimSpace(query.period) != "" {
		values.Set("period", strings.TrimSpace(query.period))
	}
	if strings.TrimSpace(query.appID) != "" {
		values.Set("filter[apps]", strings.TrimSpace(query.appID))
	}
	addLimit(values, query.limit)
	return values.Encode()
}

func buildAppInfoLocalizationsQuery(query *appInfoLocalizationsQuery) string {
	values := url.Values{}
	addCSV(values, "filter[locale]", query.locales)
	addLimit(values, query.limit)
	return values.Encode()
}

func buildAppInfoQuery(query *appInfoQuery) string {
	values := url.Values{}
	addCSV(values, "include", query.include)
	return values.Encode()
}

func buildAppCustomProductPagesQuery(query *appCustomProductPagesQuery) string {
	values := url.Values{}
	addLimit(values, query.limit)
	return values.Encode()
}

func buildAppCustomProductPageVersionsQuery(query *appCustomProductPageVersionsQuery) string {
	values := url.Values{}
	addLimit(values, query.limit)
	return values.Encode()
}

func buildAppCustomProductPageLocalizationsQuery(query *appCustomProductPageLocalizationsQuery) string {
	values := url.Values{}
	addLimit(values, query.limit)
	return values.Encode()
}

func buildAppCustomProductPageLocalizationPreviewSetsQuery(query *appCustomProductPageLocalizationPreviewSetsQuery) string {
	values := url.Values{}
	addLimit(values, query.limit)
	return values.Encode()
}

func buildAppCustomProductPageLocalizationScreenshotSetsQuery(query *appCustomProductPageLocalizationScreenshotSetsQuery) string {
	values := url.Values{}
	addLimit(values, query.limit)
	return values.Encode()
}

func buildAppStoreVersionLocalizationPreviewSetsQuery(query *appStoreVersionLocalizationPreviewSetsQuery) string {
	values := url.Values{}
	addLimit(values, query.limit)
	return values.Encode()
}

func buildAppStoreVersionLocalizationScreenshotSetsQuery(query *appStoreVersionLocalizationScreenshotSetsQuery) string {
	values := url.Values{}
	addLimit(values, query.limit)
	return values.Encode()
}

func buildAppStoreVersionExperimentsQuery(query *appStoreVersionExperimentsQuery) string {
	values := url.Values{}
	addCSV(values, "filter[state]", query.states)
	addLimit(values, query.limit)
	return values.Encode()
}

func buildAppStoreVersionExperimentsV2Query(query *appStoreVersionExperimentsV2Query) string {
	values := url.Values{}
	addCSV(values, "filter[state]", query.states)
	addLimit(values, query.limit)
	return values.Encode()
}

func buildAppStoreVersionExperimentTreatmentsQuery(query *appStoreVersionExperimentTreatmentsQuery) string {
	values := url.Values{}
	addLimit(values, query.limit)
	return values.Encode()
}

func buildAppStoreVersionExperimentTreatmentLocalizationsQuery(query *appStoreVersionExperimentTreatmentLocalizationsQuery) string {
	values := url.Values{}
	addLimit(values, query.limit)
	return values.Encode()
}

func buildTerritoriesQuery(query *territoriesQuery) string {
	values := url.Values{}
	addCSV(values, "fields[territories]", query.fields)
	addLimit(values, query.limit)
	return values.Encode()
}

func buildAndroidToIosAppMappingDetailsQuery(query *androidToIosAppMappingDetailsQuery) string {
	values := url.Values{}
	addCSV(values, "fields[androidToIosAppMappingDetails]", query.fields)
	addLimit(values, query.limit)
	return values.Encode()
}

func buildAndroidToIosAppMappingDetailQuery(query *androidToIosAppMappingDetailsQuery) string {
	values := url.Values{}
	addCSV(values, "fields[androidToIosAppMappingDetails]", query.fields)
	return values.Encode()
}

func buildPerfPowerMetricsQuery(query *perfPowerMetricsQuery) string {
	values := url.Values{}
	addCSV(values, "filter[platform]", query.platforms)
	addCSV(values, "filter[metricType]", query.metricTypes)
	addCSV(values, "filter[deviceType]", query.deviceTypes)
	return values.Encode()
}

func buildDiagnosticSignaturesQuery(query *diagnosticSignaturesQuery) string {
	values := url.Values{}
	addCSV(values, "filter[diagnosticType]", query.diagnosticTypes)
	addCSV(values, "fields[diagnosticSignatures]", query.fields)
	addLimit(values, query.limit)
	return values.Encode()
}

func buildDiagnosticLogsQuery(query *diagnosticLogsQuery) string {
	values := url.Values{}
	addLimit(values, query.limit)
	return values.Encode()
}

func buildLinkagesQuery(query *linkagesQuery) string {
	values := url.Values{}
	addLimit(values, query.limit)
	return values.Encode()
}

func buildPricePointsQuery(query *pricePointsQuery) string {
	values := url.Values{}
	if strings.TrimSpace(query.territory) != "" {
		values.Set("filter[territory]", strings.TrimSpace(query.territory))
	}
	addLimit(values, query.limit)
	return values.Encode()
}
