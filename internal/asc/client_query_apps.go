package asc

import (
	"net/url"
	"strconv"
	"strings"
)

type appsQuery struct {
	listQuery
	sort                    string
	bundleIDs               []string
	names                   []string
	skus                    []string
	inAppPurchaseFields     []string
	subscriptionGroupFields []string
	include                 []string
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

type betaAppClipInvocationsQuery struct {
	listQuery
}

type betaAppClipInvocationQuery struct {
	include            []string
	localizationsLimit int
}

type accessibilityDeclarationsQuery struct {
	listQuery
	deviceFamilies []string
	states         []string
	fields         []string
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

func buildAppsQuery(query *appsQuery) string {
	values := url.Values{}
	addCSV(values, "filter[bundleId]", query.bundleIDs)
	addCSV(values, "filter[name]", query.names)
	addCSV(values, "filter[sku]", query.skus)
	if query.sort != "" {
		values.Set("sort", query.sort)
	}
	addCSV(values, "fields[inAppPurchases]", query.inAppPurchaseFields)
	addCSV(values, "fields[subscriptionGroups]", query.subscriptionGroupFields)
	addCSV(values, "include", query.include)
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

func buildBetaAppClipInvocationsQuery(query *betaAppClipInvocationsQuery) string {
	values := url.Values{}
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

// BetaAppClipInvocationsOption is a functional option for GetBuildBundleBetaAppClipInvocations.
type BetaAppClipInvocationsOption func(*betaAppClipInvocationsQuery)

// AccessibilityDeclarationsOption is a functional option for accessibility declarations.
type AccessibilityDeclarationsOption func(*accessibilityDeclarationsQuery)

// AppEncryptionDeclarationsOption is a functional option for encryption declarations.
type AppEncryptionDeclarationsOption func(*appEncryptionDeclarationsQuery)

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

// WithAppsInAppPurchaseFields sets fields[inAppPurchases].
func WithAppsInAppPurchaseFields(fields []string) AppsOption {
	return func(q *appsQuery) {
		q.inAppPurchaseFields = normalizeList(fields)
	}
}

// WithAppsSubscriptionGroupFields sets fields[subscriptionGroups].
func WithAppsSubscriptionGroupFields(fields []string) AppsOption {
	return func(q *appsQuery) {
		q.subscriptionGroupFields = normalizeList(fields)
	}
}

// WithAppsInclude includes related resources in app collection responses.
func WithAppsInclude(include []string) AppsOption {
	return func(q *appsQuery) {
		q.include = normalizeList(include)
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

// WithAppClipDefaultExperiencesReleaseWithVersionExists filters default experiences by release version presence.
func WithAppClipDefaultExperiencesReleaseWithVersionExists(exists bool) AppClipDefaultExperiencesOption {
	return func(q *appClipDefaultExperiencesQuery) {
		q.releaseWithVersionExists = &exists
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
