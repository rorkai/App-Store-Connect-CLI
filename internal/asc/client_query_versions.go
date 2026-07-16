package asc

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

type reviewQuery struct {
	listQuery
	rating                  int
	territory               string
	sort                    string
	publishedResponseExists *bool
	includeResponse         bool
	responseFields          []string
}

type appStoreVersionsQuery struct {
	listQuery
	platforms        []string
	versionStrings   []string
	states           []string
	appVersionStates []string
	include          []string
}

type appStoreVersionQuery struct {
	include            []string
	localizationsLimit int
}

type reviewSubmissionsQuery struct {
	listQuery
	platforms                  []string
	states                     []string
	appIDs                     []string
	reviewSubmissionItemFields []string
	include                    []string
}

type reviewSubmissionQuery struct {
	reviewSubmissionItemFields []string
}

type reviewSubmissionItemsQuery struct {
	listQuery
	fields                         []string
	inAppPurchaseVersionFields     []string
	subscriptionVersionFields      []string
	subscriptionGroupVersionFields []string
	include                        []string
}

type appStoreVersionLocalizationsQuery struct {
	listQuery
	locales []string
}

type appInfoLocalizationsQuery struct {
	listQuery
	locales []string
}

type appInfoQuery struct {
	include            []string
	localizationsLimit int
}

type territoryAgeRatingsQuery struct {
	listQuery
	fields          []string
	territoryFields []string
	include         []string
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

type appStoreVersionExperimentTreatmentLocalizationPreviewSetsQuery struct {
	listQuery
}

type appStoreVersionExperimentTreatmentLocalizationScreenshotSetsQuery struct {
	listQuery
}

type endUserLicenseAgreementTerritoriesQuery struct {
	listQuery
}

type territoriesQuery struct {
	listQuery
	fields []string
}

type appStoreReviewAttachmentsQuery struct {
	listQuery
	fieldsAttachments   []string
	fieldsReviewDetails []string
	include             []string
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
	if query.publishedResponseExists != nil {
		values.Set("exists[publishedResponse]", strconv.FormatBool(*query.publishedResponseExists))
	}
	if query.includeResponse {
		values.Set("include", "response")
	}
	addCSV(values, "fields[customerReviewResponses]", query.responseFields)
	addLimit(values, query.limit)

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

func buildAppStoreVersionsQuery(query *appStoreVersionsQuery) string {
	values := url.Values{}
	addCSV(values, "filter[platform]", query.platforms)
	addCSV(values, "filter[versionString]", query.versionStrings)
	addCSV(values, "filter[appStoreState]", query.states)
	addCSV(values, "filter[appVersionState]", query.appVersionStates)
	addCSV(values, "include", query.include)
	addLimit(values, query.limit)
	return values.Encode()
}

func buildAppStoreVersionQuery(query *appStoreVersionQuery) string {
	values := url.Values{}
	addCSV(values, "include", query.include)
	if query.localizationsLimit > 0 {
		values.Set("limit[appStoreVersionLocalizations]", strconv.Itoa(query.localizationsLimit))
	}
	return values.Encode()
}

func buildReviewSubmissionsQuery(query *reviewSubmissionsQuery) string {
	values := url.Values{}
	addCSV(values, "filter[platform]", query.platforms)
	addCSV(values, "filter[state]", query.states)
	addCSV(values, "filter[app]", query.appIDs)
	addCSV(values, "fields[reviewSubmissionItems]", query.reviewSubmissionItemFields)
	addCSV(values, "include", query.include)
	addLimit(values, query.limit)
	return values.Encode()
}

func buildReviewSubmissionQuery(query *reviewSubmissionQuery) string {
	values := url.Values{}
	addCSV(values, "fields[reviewSubmissionItems]", query.reviewSubmissionItemFields)
	return values.Encode()
}

func buildReviewSubmissionItemsQuery(query *reviewSubmissionItemsQuery) string {
	values := url.Values{}
	addCSV(values, "fields[reviewSubmissionItems]", query.fields)
	addCSV(values, "fields[inAppPurchaseVersions]", query.inAppPurchaseVersionFields)
	addCSV(values, "fields[subscriptionVersions]", query.subscriptionVersionFields)
	addCSV(values, "fields[subscriptionGroupVersions]", query.subscriptionGroupVersionFields)
	addCSV(values, "include", query.include)
	addLimit(values, query.limit)
	return values.Encode()
}

func buildAppStoreVersionLocalizationsQuery(query *appStoreVersionLocalizationsQuery) string {
	values := url.Values{}
	addCSV(values, "filter[locale]", query.locales)
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
	if query.localizationsLimit > 0 {
		values.Set("limit[appInfoLocalizations]", strconv.Itoa(query.localizationsLimit))
	}
	return values.Encode()
}

func buildTerritoryAgeRatingsQuery(query *territoryAgeRatingsQuery) string {
	values := url.Values{}
	addCSV(values, "fields[territoryAgeRatings]", query.fields)
	addCSV(values, "fields[territories]", query.territoryFields)
	addCSV(values, "include", query.include)
	addLimit(values, query.limit)
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

func buildAppStoreVersionExperimentTreatmentLocalizationPreviewSetsQuery(query *appStoreVersionExperimentTreatmentLocalizationPreviewSetsQuery) string {
	values := url.Values{}
	addLimit(values, query.limit)
	return values.Encode()
}

func buildAppStoreVersionExperimentTreatmentLocalizationScreenshotSetsQuery(query *appStoreVersionExperimentTreatmentLocalizationScreenshotSetsQuery) string {
	values := url.Values{}
	addLimit(values, query.limit)
	return values.Encode()
}

func buildEndUserLicenseAgreementTerritoriesQuery(query *endUserLicenseAgreementTerritoriesQuery) string {
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

// ReviewOption is a functional option for GetReviews.
type ReviewOption func(*reviewQuery)

// AppStoreVersionsOption is a functional option for GetAppStoreVersions.
type AppStoreVersionsOption func(*appStoreVersionsQuery)

// AppStoreVersionOption is a functional option for GetAppStoreVersion.
type AppStoreVersionOption func(*appStoreVersionQuery)

// ReviewSubmissionsOption is a functional option for GetReviewSubmissions.
type ReviewSubmissionsOption func(*reviewSubmissionsQuery)

// ReviewSubmissionItemsOption is a functional option for GetReviewSubmissionItems.
type ReviewSubmissionItemsOption func(*reviewSubmissionItemsQuery)

// ReviewSubmissionOption is a functional option for GetReviewSubmission.
type ReviewSubmissionOption func(*reviewSubmissionQuery)

// AppStoreVersionLocalizationsOption is a functional option for version localizations.
type AppStoreVersionLocalizationsOption func(*appStoreVersionLocalizationsQuery)

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

// AppStoreVersionExperimentTreatmentLocalizationPreviewSetsOption is a functional option for treatment localization preview set list endpoints.
type AppStoreVersionExperimentTreatmentLocalizationPreviewSetsOption func(*appStoreVersionExperimentTreatmentLocalizationPreviewSetsQuery)

// AppStoreVersionExperimentTreatmentLocalizationScreenshotSetsOption is a functional option for treatment localization screenshot set list endpoints.
type AppStoreVersionExperimentTreatmentLocalizationScreenshotSetsOption func(*appStoreVersionExperimentTreatmentLocalizationScreenshotSetsQuery)

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

// EndUserLicenseAgreementTerritoriesOption is a functional option for EULA territory lists.
type EndUserLicenseAgreementTerritoriesOption func(*endUserLicenseAgreementTerritoriesQuery)

// AppStoreReviewAttachmentsOption is a functional option for review attachments.
type AppStoreReviewAttachmentsOption func(*appStoreReviewAttachmentsQuery)

// TerritoryAgeRatingsOption is a functional option for territory age ratings.
type TerritoryAgeRatingsOption func(*territoryAgeRatingsQuery)

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

// WithPublishedResponseExists filters reviews by whether a published response exists.
func WithPublishedResponseExists(exists bool) ReviewOption {
	return func(r *reviewQuery) {
		value := exists
		r.publishedResponseExists = &value
	}
}

// WithReviewIncludeResponse includes review response relationships in review results.
func WithReviewIncludeResponse() ReviewOption {
	return func(r *reviewQuery) {
		r.includeResponse = true
	}
}

// WithReviewResponseFields limits fields returned for included review responses.
func WithReviewResponseFields(fields []string) ReviewOption {
	return func(r *reviewQuery) {
		r.responseFields = normalizeList(fields)
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

// WithAppStoreVersionsStates filters versions by state. Deprecated app store
// states are sent as filter[appStoreState]; modern version-only states use
// filter[appVersionState].
func WithAppStoreVersionsStates(states []string) AppStoreVersionsOption {
	return func(q *appStoreVersionsQuery) {
		normalized := normalizeUpperList(states)
		if shouldUseAppVersionStateFilter(normalized) {
			q.appVersionStates = append(q.appVersionStates, normalized...)
			return
		}
		for _, state := range normalized {
			if isAppVersionStateOnly(state) {
				q.appVersionStates = append(q.appVersionStates, state)
				continue
			}
			q.states = append(q.states, state)
		}
	}
}

// WithAppStoreVersionsVersionStates filters versions by app version state.
func WithAppStoreVersionsVersionStates(states []string) AppStoreVersionsOption {
	return func(q *appStoreVersionsQuery) {
		q.appVersionStates = normalizeUpperList(states)
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

// WithAppStoreVersionLocalizationsIncludeLimit sets the maximum number of
// localizations returned through an app store version include.
func WithAppStoreVersionLocalizationsIncludeLimit(limit int) AppStoreVersionOption {
	return func(q *appStoreVersionQuery) {
		if limit > 0 {
			q.localizationsLimit = limit
		}
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

// WithReviewSubmissionsApps filters review submissions by app IDs.
func WithReviewSubmissionsApps(appIDs []string) ReviewSubmissionsOption {
	return func(q *reviewSubmissionsQuery) {
		q.appIDs = normalizeList(appIDs)
	}
}

// WithReviewSubmissionsInclude includes related resources for review submissions responses.
func WithReviewSubmissionsInclude(include []string) ReviewSubmissionsOption {
	return func(q *reviewSubmissionsQuery) {
		q.include = normalizeList(include)
	}
}

// WithReviewSubmissionsItemFields sets fields[reviewSubmissionItems] on list responses.
func WithReviewSubmissionsItemFields(fields []string) ReviewSubmissionsOption {
	return func(q *reviewSubmissionsQuery) {
		q.reviewSubmissionItemFields = normalizeList(fields)
	}
}

// WithReviewSubmissionItemFields sets fields[reviewSubmissionItems] on a detail response.
func WithReviewSubmissionItemFields(fields []string) ReviewSubmissionOption {
	return func(q *reviewSubmissionQuery) {
		q.reviewSubmissionItemFields = normalizeList(fields)
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

// WithReviewSubmissionItemsFields sets fields[reviewSubmissionItems] for item responses.
func WithReviewSubmissionItemsFields(fields []string) ReviewSubmissionItemsOption {
	return func(q *reviewSubmissionItemsQuery) {
		q.fields = normalizeList(fields)
	}
}

// WithReviewSubmissionItemsInAppPurchaseVersionFields sets fields[inAppPurchaseVersions].
func WithReviewSubmissionItemsInAppPurchaseVersionFields(fields []string) ReviewSubmissionItemsOption {
	return func(q *reviewSubmissionItemsQuery) {
		q.inAppPurchaseVersionFields = normalizeList(fields)
	}
}

// WithReviewSubmissionItemsSubscriptionVersionFields sets fields[subscriptionVersions].
func WithReviewSubmissionItemsSubscriptionVersionFields(fields []string) ReviewSubmissionItemsOption {
	return func(q *reviewSubmissionItemsQuery) {
		q.subscriptionVersionFields = normalizeList(fields)
	}
}

// WithReviewSubmissionItemsSubscriptionGroupVersionFields sets fields[subscriptionGroupVersions].
func WithReviewSubmissionItemsSubscriptionGroupVersionFields(fields []string) ReviewSubmissionItemsOption {
	return func(q *reviewSubmissionItemsQuery) {
		q.subscriptionGroupVersionFields = normalizeList(fields)
	}
}

// WithReviewSubmissionItemsInclude sets include for item responses.
func WithReviewSubmissionItemsInclude(include []string) ReviewSubmissionItemsOption {
	return func(q *reviewSubmissionItemsQuery) {
		q.include = normalizeList(include)
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

// WithAppInfoLocalizationsIncludeLimit sets the maximum number of
// localizations returned through an app info include.
func WithAppInfoLocalizationsIncludeLimit(limit int) AppInfoOption {
	return func(q *appInfoQuery) {
		if limit > 0 {
			q.localizationsLimit = limit
		}
	}
}

// WithTerritoryAgeRatingsFields sets fields[territoryAgeRatings].
func WithTerritoryAgeRatingsFields(fields []string) TerritoryAgeRatingsOption {
	return func(q *territoryAgeRatingsQuery) {
		q.fields = normalizeList(fields)
	}
}

// WithTerritoryAgeRatingsTerritoryFields sets fields[territories].
func WithTerritoryAgeRatingsTerritoryFields(fields []string) TerritoryAgeRatingsOption {
	return func(q *territoryAgeRatingsQuery) {
		q.territoryFields = normalizeList(fields)
	}
}

// WithTerritoryAgeRatingsInclude sets include values for territory age ratings.
func WithTerritoryAgeRatingsInclude(include []string) TerritoryAgeRatingsOption {
	return func(q *territoryAgeRatingsQuery) {
		q.include = normalizeList(include)
	}
}

// WithTerritoryAgeRatingsLimit sets the max number of territory age ratings to return.
func WithTerritoryAgeRatingsLimit(limit int) TerritoryAgeRatingsOption {
	return func(q *territoryAgeRatingsQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithTerritoryAgeRatingsNextURL uses a next page URL directly.
func WithTerritoryAgeRatingsNextURL(next string) TerritoryAgeRatingsOption {
	return func(q *territoryAgeRatingsQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
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

// WithEndUserLicenseAgreementTerritoriesLimit sets the max number of territories to return.
func WithEndUserLicenseAgreementTerritoriesLimit(limit int) EndUserLicenseAgreementTerritoriesOption {
	return func(q *endUserLicenseAgreementTerritoriesQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithEndUserLicenseAgreementTerritoriesNextURL uses a next page URL directly.
func WithEndUserLicenseAgreementTerritoriesNextURL(next string) EndUserLicenseAgreementTerritoriesOption {
	return func(q *endUserLicenseAgreementTerritoriesQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
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

// WithAppStoreVersionExperimentTreatmentLocalizationPreviewSetsLimit sets the max number of preview sets to return.
func WithAppStoreVersionExperimentTreatmentLocalizationPreviewSetsLimit(limit int) AppStoreVersionExperimentTreatmentLocalizationPreviewSetsOption {
	return func(q *appStoreVersionExperimentTreatmentLocalizationPreviewSetsQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithAppStoreVersionExperimentTreatmentLocalizationPreviewSetsNextURL uses a next page URL directly.
func WithAppStoreVersionExperimentTreatmentLocalizationPreviewSetsNextURL(next string) AppStoreVersionExperimentTreatmentLocalizationPreviewSetsOption {
	return func(q *appStoreVersionExperimentTreatmentLocalizationPreviewSetsQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

// WithAppStoreVersionExperimentTreatmentLocalizationScreenshotSetsLimit sets the max number of screenshot sets to return.
func WithAppStoreVersionExperimentTreatmentLocalizationScreenshotSetsLimit(limit int) AppStoreVersionExperimentTreatmentLocalizationScreenshotSetsOption {
	return func(q *appStoreVersionExperimentTreatmentLocalizationScreenshotSetsQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithAppStoreVersionExperimentTreatmentLocalizationScreenshotSetsNextURL uses a next page URL directly.
func WithAppStoreVersionExperimentTreatmentLocalizationScreenshotSetsNextURL(next string) AppStoreVersionExperimentTreatmentLocalizationScreenshotSetsOption {
	return func(q *appStoreVersionExperimentTreatmentLocalizationScreenshotSetsQuery) {
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

func isAppVersionStateOnly(state string) bool {
	switch state {
	case "PROCESSING_FOR_DISTRIBUTION", "READY_FOR_DISTRIBUTION":
		return true
	default:
		return false
	}
}

func shouldUseAppVersionStateFilter(states []string) bool {
	hasVersionOnlyState := false
	for _, state := range states {
		if !isAppVersionStateFilterState(state) {
			return false
		}
		if isAppVersionStateOnly(state) {
			hasVersionOnlyState = true
		}
	}
	return hasVersionOnlyState
}

func isAppVersionStateFilterState(state string) bool {
	switch state {
	case "ACCEPTED",
		"DEVELOPER_REJECTED",
		"IN_REVIEW",
		"INVALID_BINARY",
		"METADATA_REJECTED",
		"PENDING_APPLE_RELEASE",
		"PENDING_DEVELOPER_RELEASE",
		"PREPARE_FOR_SUBMISSION",
		"PROCESSING_FOR_DISTRIBUTION",
		"READY_FOR_DISTRIBUTION",
		"READY_FOR_REVIEW",
		"REJECTED",
		"REPLACED_WITH_NEW_VERSION",
		"WAITING_FOR_EXPORT_COMPLIANCE",
		"WAITING_FOR_REVIEW":
		return true
	default:
		return false
	}
}
