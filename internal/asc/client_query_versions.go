package asc

import (
	"net/url"
	"slices"
	"strconv"
	"strings"
)

type reviewQuery struct {
	listQuery
	ratings                 []int
	territory               string
	reviewTerritory         string
	sort                    string
	publishedResponseExists *bool
	include                 []string
	fields                  []string
	responseFields          []string
	territoryFields         []string
}

type appStoreVersionsQuery struct {
	listQuery
	includedFieldsQuery
	platforms        []string
	versionStrings   []string
	states           []string
	appVersionStates []string
	include          []string
}

type appStoreVersionQuery struct {
	includedFieldsQuery
	include            []string
	localizationsLimit int
}

type reviewSubmissionsQuery struct {
	listQuery
	includedFieldsQuery
	platforms                  []string
	states                     []string
	appIDs                     []string
	reviewSubmissionItemFields []string
	include                    []string
}

type reviewSubmissionQuery struct {
	includedFieldsQuery
	reviewSubmissionItemFields []string
	include                    []string
}

type reviewSubmissionItemsQuery struct {
	listQuery
	includedFieldsQuery
	fields                         []string
	inAppPurchaseVersionFields     []string
	subscriptionVersionFields      []string
	subscriptionGroupVersionFields []string
	include                        []string
}

// localizationRelatedQuery carries the include and limit[placements] parameters
// that localization reads share.
type localizationRelatedQuery struct {
	include         []string
	placementsLimit int
}

func (q localizationRelatedQuery) addTo(values url.Values) {
	addCSV(values, "include", q.include)
	if q.placementsLimit > 0 {
		values.Set("limit[placements]", strconv.Itoa(q.placementsLimit))
	}
}

type appStoreVersionLocalizationsQuery struct {
	listQuery
	localizationRelatedQuery
	includedFieldsQuery
	locales []string
}

type appInfoQuery struct {
	listQuery
	includedFieldsQuery
	fields                     []string
	ageRatingDeclarationFields []string
	include                    []string
	localizationsLimit         int
}

type appCustomProductPagesQuery struct {
	listQuery
	visible       []string
	fields        []string
	appFields     []string
	versionFields []string
	include       []string
	versionsLimit int
}

type appCustomProductPageLocalizationsQuery struct {
	listQuery
	localizationRelatedQuery
	searchKeywordsLimit int
}

type appCustomProductPageLocalizationScreenshotSetsQuery struct {
	listQuery
	includeMedia   bool
	requestContext RequestContextFunc
}

type appStoreVersionLocalizationPreviewSetsQuery struct {
	listQuery
	includeMedia bool
}

type appStoreVersionLocalizationScreenshotSetsQuery struct {
	listQuery
	includedFieldsQuery
	includeMedia bool
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
	localizationRelatedQuery
}

type appStoreVersionExperimentTreatmentLocalizationScreenshotSetsQuery struct {
	listQuery
	includeMedia   bool
	requestContext RequestContextFunc
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
	if query.reviewTerritory != "" {
		values.Set("filter[reviewTerritory]", query.reviewTerritory)
	}
	if len(query.ratings) > 0 {
		ratings := make([]string, 0, len(query.ratings))
		for _, rating := range query.ratings {
			ratings = append(ratings, strconv.Itoa(rating))
		}
		values.Set("filter[rating]", strings.Join(ratings, ","))
	}
	if query.sort != "" {
		values.Set("sort", query.sort)
	}
	if query.publishedResponseExists != nil {
		values.Set("exists[publishedResponse]", strconv.FormatBool(*query.publishedResponseExists))
	}
	addReviewSelection(values, query)
	addLimit(values, query.limit)

	return values.Encode()
}

// buildCustomerReviewQuery builds the query for GET /v1/customerReviews/{id},
// which accepts only the include and fields parameters.
func buildCustomerReviewQuery(opts []ReviewOption) string {
	query := &reviewQuery{}
	for _, opt := range opts {
		opt(query)
	}

	values := url.Values{}
	addReviewSelection(values, query)
	return values.Encode()
}

func addReviewSelection(values url.Values, query *reviewQuery) {
	addCSV(values, "include", normalizeUniqueList(query.include))
	addCSV(values, "fields[customerReviews]", query.fields)
	addCSV(values, "fields[customerReviewResponses]", query.responseFields)
	addCSV(values, "fields[territories]", query.territoryFields)
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
	query.includedFields.AddTo(values)
	return values.Encode()
}

func buildAppStoreVersionQuery(query *appStoreVersionQuery) string {
	values := url.Values{}
	addCSV(values, "include", query.include)
	if query.localizationsLimit > 0 {
		values.Set("limit[appStoreVersionLocalizations]", strconv.Itoa(query.localizationsLimit))
	}
	query.includedFields.AddTo(values)
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
	query.includedFields.AddTo(values)
	return values.Encode()
}

func buildReviewSubmissionQuery(query *reviewSubmissionQuery) string {
	values := url.Values{}
	addCSV(values, "fields[reviewSubmissionItems]", query.reviewSubmissionItemFields)
	addCSV(values, "include", query.include)
	query.includedFields.AddTo(values)
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
	query.includedFields.AddTo(values)
	return values.Encode()
}

func buildAppStoreVersionLocalizationsQuery(query *appStoreVersionLocalizationsQuery) string {
	values := url.Values{}
	addCSV(values, "filter[locale]", query.locales)
	query.addTo(values)
	addLimit(values, query.limit)
	query.includedFields.AddTo(values)
	return values.Encode()
}

func buildAppInfoQuery(query *appInfoQuery) string {
	values := url.Values{}
	include := normalizeUniqueList(query.include)
	fields := normalizeUniqueList(query.fields)
	if len(fields) > 0 {
		// A primary sparse fieldset must retain every included relationship or
		// ASC can omit the linkage and included resource from the response.
		fields = normalizeUniqueList(append(fields, include...))
	}
	addCSV(values, "fields[appInfos]", fields)
	addCSV(values, "fields[ageRatingDeclarations]", query.ageRatingDeclarationFields)
	addCSV(values, "include", include)
	addLimit(values, query.limit)
	if query.localizationsLimit > 0 {
		values.Set("limit[appInfoLocalizations]", strconv.Itoa(query.localizationsLimit))
	}
	query.includedFields.AddTo(values)
	return values.Encode()
}

func buildAppCustomProductPagesQuery(query *appCustomProductPagesQuery) string {
	values := url.Values{}
	include := normalizeUniqueList(query.include)
	fields := normalizeUniqueList(query.fields)
	if len(fields) > 0 {
		// A primary sparse fieldset must retain every included relationship or
		// ASC can omit the linkage and included resource from the response.
		fields = normalizeUniqueList(append(fields, include...))
	}
	addCSV(values, "filter[visible]", query.visible)
	addCSV(values, "fields[appCustomProductPages]", fields)
	addCSV(values, "fields[apps]", query.appFields)
	addCSV(values, "fields[appCustomProductPageVersions]", query.versionFields)
	addCSV(values, "include", include)
	addLimit(values, query.limit)
	if query.versionsLimit > 0 {
		values.Set("limit[appCustomProductPageVersions]", strconv.Itoa(query.versionsLimit))
	}
	return values.Encode()
}

func buildAppCustomProductPageLocalizationsQuery(query *appCustomProductPageLocalizationsQuery) string {
	values := url.Values{}
	addLimit(values, query.limit)
	query.addTo(values)
	if query.searchKeywordsLimit > 0 {
		addCSV(values, "include", append(slices.Clone(query.include), "searchKeywords"))
		values.Set("limit[searchKeywords]", strconv.Itoa(query.searchKeywordsLimit))
	}
	return values.Encode()
}

func buildAppCustomProductPageLocalizationScreenshotSetsQuery(query *appCustomProductPageLocalizationScreenshotSetsQuery) string {
	values := url.Values{}
	addLimit(values, query.limit)
	if query.includeMedia {
		addIncludedSetMedia(values, "appScreenshots")
	}
	return values.Encode()
}

func buildAppStoreVersionLocalizationPreviewSetsQuery(query *appStoreVersionLocalizationPreviewSetsQuery) string {
	values := url.Values{}
	addLimit(values, query.limit)
	if query.includeMedia {
		addIncludedSetMedia(values, "appPreviews")
	}
	return values.Encode()
}

func buildAppStoreVersionLocalizationScreenshotSetsQuery(query *appStoreVersionLocalizationScreenshotSetsQuery) string {
	values := url.Values{}
	addLimit(values, query.limit)
	if query.includeMedia {
		addIncludedSetMedia(values, "appScreenshots")
	}
	query.includedFields.AddTo(values)
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
	query.addTo(values)
	return values.Encode()
}

func buildAppStoreVersionExperimentTreatmentLocalizationScreenshotSetsQuery(query *appStoreVersionExperimentTreatmentLocalizationScreenshotSetsQuery) string {
	values := url.Values{}
	addLimit(values, query.limit)
	if query.includeMedia {
		addIncludedSetMedia(values, "appScreenshots")
	}
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

// AppInfoOption is a functional option for GetAppInfo.
type AppInfoOption func(*appInfoQuery)

// AppCustomProductPagesOption is a functional option for custom product page list endpoints.
type AppCustomProductPagesOption func(*appCustomProductPagesQuery)

// LocalizationReadOption sets query parameters on a single-localization read.
type LocalizationReadOption func(*localizationRelatedQuery)

// WithLocalizationReadInclude includes related resources.
func WithLocalizationReadInclude(include []string) LocalizationReadOption {
	return func(q *localizationRelatedQuery) {
		q.include = normalizeList(include)
	}
}

// WithLocalizationReadPlacementsLimit sets limit[placements].
func WithLocalizationReadPlacementsLimit(limit int) LocalizationReadOption {
	return func(q *localizationRelatedQuery) {
		q.placementsLimit = limit
	}
}

func localizationReadPath(path string, opts []LocalizationReadOption) string {
	query := localizationRelatedQuery{}
	for _, opt := range opts {
		opt(&query)
	}
	values := url.Values{}
	query.addTo(values)
	if len(values) == 0 {
		return path
	}
	return path + "?" + values.Encode()
}

// AppCustomProductPageLocalizationsOption is a functional option for custom product page localization list endpoints.
type AppCustomProductPageLocalizationsOption func(*appCustomProductPageLocalizationsQuery)

// AppCustomProductPageLocalizationScreenshotSetsOption is a functional option for screenshot set list endpoints.
type AppCustomProductPageLocalizationScreenshotSetsOption func(*appCustomProductPageLocalizationScreenshotSetsQuery)

// AppStoreVersionLocalizationPreviewSetsOption is a functional option for app store version preview sets list endpoints.
type AppStoreVersionLocalizationPreviewSetsOption func(*appStoreVersionLocalizationPreviewSetsQuery)

// AppStoreVersionLocalizationScreenshotSetsOption is a functional option for app store version screenshot sets list endpoints.
type AppStoreVersionLocalizationScreenshotSetsOption func(*appStoreVersionLocalizationScreenshotSetsQuery)

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

// AppStoreReviewAttachmentsOption is a functional option for review attachments.
type AppStoreReviewAttachmentsOption func(*appStoreReviewAttachmentsQuery)

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

// WithRatings filters reviews by one or more star ratings (1-5). Apple models
// filter[rating] as a comma-separated array parameter, so several ratings match
// with OR semantics. Ratings outside 1-5 and repeats are dropped.
func WithRatings(ratings []int) ReviewOption {
	return func(r *reviewQuery) {
		for _, rating := range ratings {
			if rating < 1 || rating > 5 {
				continue
			}
			if slices.Contains(r.ratings, rating) {
				continue
			}
			r.ratings = append(r.ratings, rating)
		}
	}
}

// WithTerritory filters reviews by App Store territory code (e.g. USA, GBR).
func WithTerritory(territory string) ReviewOption {
	return func(r *reviewQuery) {
		if territory != "" {
			r.territory = strings.ToUpper(territory)
		}
	}
}

// WithReviewTerritory filters reviews by related reviewTerritory IDs.
func WithReviewTerritory(territory string) ReviewOption {
	return func(r *reviewQuery) {
		r.reviewTerritory = strings.TrimSpace(territory)
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
		r.include = append(r.include, "response")
	}
}

// WithReviewInclude adds relationships to include in review results.
func WithReviewInclude(include []string) ReviewOption {
	return func(r *reviewQuery) {
		r.include = append(r.include, include...)
	}
}

// WithReviewFields limits fields returned for customer reviews.
func WithReviewFields(fields []string) ReviewOption {
	return func(r *reviewQuery) {
		r.fields = normalizeList(fields)
	}
}

// WithReviewResponseFields limits fields returned for included review responses.
func WithReviewResponseFields(fields []string) ReviewOption {
	return func(r *reviewQuery) {
		r.responseFields = normalizeList(fields)
	}
}

// WithReviewTerritoryFields limits fields returned for included territories.
func WithReviewTerritoryFields(fields []string) ReviewOption {
	return func(r *reviewQuery) {
		r.territoryFields = normalizeList(fields)
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

// WithReviewSubmissionInclude includes related resources for a review submission response.
func WithReviewSubmissionInclude(include []string) ReviewSubmissionOption {
	return func(q *reviewSubmissionQuery) {
		q.include = normalizeList(include)
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

// WithAppStoreVersionLocalizationLocales filters version localizations by locale.
func WithAppStoreVersionLocalizationLocales(locales []string) AppStoreVersionLocalizationsOption {
	return func(q *appStoreVersionLocalizationsQuery) {
		q.locales = normalizeList(locales)
	}
}

// WithAppStoreVersionLocalizationsInclude includes related resources for
// version localizations (appStoreVersion, appScreenshotSets, appPreviewSets,
// searchKeywords, placements).
func WithAppStoreVersionLocalizationsInclude(include []string) AppStoreVersionLocalizationsOption {
	return func(q *appStoreVersionLocalizationsQuery) {
		q.include = normalizeList(include)
	}
}

// WithAppStoreVersionLocalizationsPlacementsLimit sets limit[placements].
func WithAppStoreVersionLocalizationsPlacementsLimit(limit int) AppStoreVersionLocalizationsOption {
	return func(q *appStoreVersionLocalizationsQuery) {
		q.placementsLimit = limit
	}
}

// WithAppInfoFields sets fields[appInfos].
func WithAppInfoFields(fields []string) AppInfoOption {
	return func(q *appInfoQuery) {
		q.fields = normalizeList(fields)
	}
}

// WithAppInfoAgeRatingDeclarationFields sets fields[ageRatingDeclarations].
func WithAppInfoAgeRatingDeclarationFields(fields []string) AppInfoOption {
	return func(q *appInfoQuery) {
		q.ageRatingDeclarationFields = normalizeList(fields)
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

// WithTerritoriesFields sets fields[territories] for territory responses.
func WithTerritoriesFields(fields []string) TerritoriesOption {
	return func(q *territoriesQuery) {
		q.fields = normalizeList(fields)
	}
}

// WithAppCustomProductPagesVisible filters custom product pages by visibility.
func WithAppCustomProductPagesVisible(values []string) AppCustomProductPagesOption {
	return func(q *appCustomProductPagesQuery) {
		q.visible = normalizeList(values)
	}
}

// WithAppCustomProductPagesFields sets fields[appCustomProductPages] for responses.
func WithAppCustomProductPagesFields(fields []string) AppCustomProductPagesOption {
	return func(q *appCustomProductPagesQuery) {
		q.fields = normalizeList(fields)
	}
}

// WithAppCustomProductPagesAppFields sets fields[apps] for included apps.
func WithAppCustomProductPagesAppFields(fields []string) AppCustomProductPagesOption {
	return func(q *appCustomProductPagesQuery) {
		q.appFields = normalizeList(fields)
	}
}

// WithAppCustomProductPagesVersionFields sets fields[appCustomProductPageVersions] for included versions.
func WithAppCustomProductPagesVersionFields(fields []string) AppCustomProductPagesOption {
	return func(q *appCustomProductPagesQuery) {
		q.versionFields = normalizeList(fields)
	}
}

// WithAppCustomProductPagesInclude sets relationships to include in responses.
func WithAppCustomProductPagesInclude(include []string) AppCustomProductPagesOption {
	return func(q *appCustomProductPagesQuery) {
		q.include = normalizeList(include)
	}
}

// WithAppCustomProductPagesVersionsLimit sets the max number of included versions.
func WithAppCustomProductPagesVersionsLimit(limit int) AppCustomProductPagesOption {
	return func(q *appCustomProductPagesQuery) {
		if limit > 0 {
			q.versionsLimit = limit
		}
	}
}

// WithAppCustomProductPageLocalizationsSearchKeywords includes up to limit
// search keywords per localization in each localization's relationships.
func WithAppCustomProductPageLocalizationsSearchKeywords(limit int) AppCustomProductPageLocalizationsOption {
	return func(q *appCustomProductPageLocalizationsQuery) {
		if limit > 0 {
			q.searchKeywordsLimit = limit
		}
	}
}

// WithAppCustomProductPageLocalizationsInclude includes related resources.
func WithAppCustomProductPageLocalizationsInclude(include []string) AppCustomProductPageLocalizationsOption {
	return func(q *appCustomProductPageLocalizationsQuery) {
		q.include = normalizeList(include)
	}
}

// WithAppCustomProductPageLocalizationsPlacementsLimit sets limit[placements].
func WithAppCustomProductPageLocalizationsPlacementsLimit(limit int) AppCustomProductPageLocalizationsOption {
	return func(q *appCustomProductPageLocalizationsQuery) {
		q.placementsLimit = limit
	}
}

// WithAppCustomProductPageLocalizationScreenshotSetsIncludeScreenshots includes each set's screenshots.
func WithAppCustomProductPageLocalizationScreenshotSetsIncludeScreenshots() AppCustomProductPageLocalizationScreenshotSetsOption {
	return func(q *appCustomProductPageLocalizationScreenshotSetsQuery) {
		q.includeMedia = true
	}
}

// WithAppStoreVersionLocalizationPreviewSetsIncludePreviews includes each set's previews.
func WithAppStoreVersionLocalizationPreviewSetsIncludePreviews() AppStoreVersionLocalizationPreviewSetsOption {
	return func(q *appStoreVersionLocalizationPreviewSetsQuery) {
		q.includeMedia = true
	}
}

// WithAppStoreVersionLocalizationScreenshotSetsIncludeScreenshots includes each set's screenshots.
func WithAppStoreVersionLocalizationScreenshotSetsIncludeScreenshots() AppStoreVersionLocalizationScreenshotSetsOption {
	return func(q *appStoreVersionLocalizationScreenshotSetsQuery) {
		q.includeMedia = true
	}
}

// WithAppStoreVersionExperimentTreatmentLocalizationScreenshotSetsIncludeScreenshots includes each set's screenshots.
func WithAppStoreVersionExperimentTreatmentLocalizationScreenshotSetsIncludeScreenshots() AppStoreVersionExperimentTreatmentLocalizationScreenshotSetsOption {
	return func(q *appStoreVersionExperimentTreatmentLocalizationScreenshotSetsQuery) {
		q.includeMedia = true
	}
}

// WithAppStoreVersionExperimentsState filters experiments by state.
func WithAppStoreVersionExperimentsState(states []string) AppStoreVersionExperimentsOption {
	return func(q *appStoreVersionExperimentsQuery) {
		q.states = normalizeUpperList(states)
	}
}

// WithAppStoreVersionExperimentsV2State filters experiments by state (v2).
func WithAppStoreVersionExperimentsV2State(states []string) AppStoreVersionExperimentsV2Option {
	return func(q *appStoreVersionExperimentsV2Query) {
		q.states = normalizeUpperList(states)
	}
}

// WithAppStoreVersionExperimentTreatmentLocalizationsInclude includes related resources.
func WithAppStoreVersionExperimentTreatmentLocalizationsInclude(include []string) AppStoreVersionExperimentTreatmentLocalizationsOption {
	return func(q *appStoreVersionExperimentTreatmentLocalizationsQuery) {
		q.include = normalizeList(include)
	}
}

// WithAppStoreVersionExperimentTreatmentLocalizationsPlacementsLimit sets limit[placements].
func WithAppStoreVersionExperimentTreatmentLocalizationsPlacementsLimit(limit int) AppStoreVersionExperimentTreatmentLocalizationsOption {
	return func(q *appStoreVersionExperimentTreatmentLocalizationsQuery) {
		q.placementsLimit = limit
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
