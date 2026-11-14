package asc

import (
	"net/url"
	"strings"
)

type marketplaceWebhooksQuery struct {
	listQuery
	fields []string
}

type alternativeDistributionDomainsQuery struct {
	listQuery
}

type alternativeDistributionKeysQuery struct {
	listQuery
}

type alternativeDistributionPackageVersionsQuery struct {
	listQuery
}

type alternativeDistributionPackageVariantsQuery struct {
	listQuery
}

type alternativeDistributionPackageDeltasQuery struct {
	listQuery
}

type webhooksQuery struct {
	listQuery
}

type webhookDeliveriesQuery struct {
	listQuery
	createdAfter  []string
	createdBefore []string
}

type backgroundAssetsQuery struct {
	listQuery
	archived             []string
	assetPackIdentifiers []string
	versionsLocales      []string
	versionsPlatforms    []string
	sort                 string
}

type backgroundAssetVersionsQuery struct {
	listQuery
	locales            []string
	platforms          []string
	states             []string
	versions           []string
	internalBetaStates []string
	externalBetaStates []string
	appStoreStates     []string
	sort               string
}

type backgroundAssetUploadFilesQuery struct {
	listQuery
}

type androidToIosAppMappingDetailsQuery struct {
	listQuery
	fields []string
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
	addLimit(values, query.limit)
	return values.Encode()
}

func buildAlternativeDistributionKeysQuery(query *alternativeDistributionKeysQuery) string {
	values := url.Values{}
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
	addLimit(values, query.limit)
	return values.Encode()
}

func buildAlternativeDistributionPackageDeltasQuery(query *alternativeDistributionPackageDeltasQuery) string {
	values := url.Values{}
	addLimit(values, query.limit)
	return values.Encode()
}

func buildWebhooksQuery(query *webhooksQuery) string {
	values := url.Values{}
	addLimit(values, query.limit)
	return values.Encode()
}

func buildWebhookDeliveriesQuery(query *webhookDeliveriesQuery) string {
	values := url.Values{}
	addCSV(values, "filter[createdDateGreaterThanOrEqualTo]", query.createdAfter)
	addCSV(values, "filter[createdDateLessThan]", query.createdBefore)
	addLimit(values, query.limit)
	return values.Encode()
}

func buildBackgroundAssetsQuery(query *backgroundAssetsQuery) string {
	values := url.Values{}
	addCSV(values, "filter[archived]", query.archived)
	addCSV(values, "filter[assetPackIdentifier]", query.assetPackIdentifiers)
	addCSV(values, "filter[versions.locale]", query.versionsLocales)
	addCSV(values, "filter[versions.platforms]", query.versionsPlatforms)
	if query.sort != "" {
		values.Set("sort", query.sort)
	}
	addLimit(values, query.limit)
	return values.Encode()
}

func buildBackgroundAssetVersionsQuery(query *backgroundAssetVersionsQuery) string {
	values := url.Values{}
	addCSV(values, "filter[locale]", query.locales)
	addCSV(values, "filter[platforms]", query.platforms)
	addCSV(values, "filter[state]", query.states)
	addCSV(values, "filter[version]", query.versions)
	addCSV(values, "filter[internalBetaRelease.state]", query.internalBetaStates)
	addCSV(values, "filter[externalBetaRelease.state]", query.externalBetaStates)
	addCSV(values, "filter[appStoreRelease.state]", query.appStoreStates)
	if query.sort != "" {
		values.Set("sort", query.sort)
	}
	addLimit(values, query.limit)
	return values.Encode()
}

func buildBackgroundAssetUploadFilesQuery(query *backgroundAssetUploadFilesQuery) string {
	values := url.Values{}
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

// AndroidToIosAppMappingDetailsOption is a functional option for Android-to-iOS mappings.
type AndroidToIosAppMappingDetailsOption func(*androidToIosAppMappingDetailsQuery)

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

// WithMarketplaceWebhooksFields sets fields[marketplaceWebhooks] for webhook responses.
func WithMarketplaceWebhooksFields(fields []string) MarketplaceWebhooksOption {
	return func(q *marketplaceWebhooksQuery) {
		q.fields = normalizeList(fields)
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

// WithBackgroundAssetsFilterVersionsLocale filters background assets by uploaded version locale.
func WithBackgroundAssetsFilterVersionsLocale(values []string) BackgroundAssetsOption {
	return func(q *backgroundAssetsQuery) {
		q.versionsLocales = normalizeList(values)
	}
}

// WithBackgroundAssetsFilterVersionsPlatforms filters background assets by version platform.
func WithBackgroundAssetsFilterVersionsPlatforms(values []string) BackgroundAssetsOption {
	return func(q *backgroundAssetsQuery) {
		q.versionsPlatforms = normalizeList(values)
	}
}

// WithBackgroundAssetsSort sets the sort order for background assets.
func WithBackgroundAssetsSort(sort string) BackgroundAssetsOption {
	return func(q *backgroundAssetsQuery) {
		q.sort = strings.TrimSpace(sort)
	}
}

// WithBackgroundAssetVersionsFilterLocale filters background asset versions by locale.
func WithBackgroundAssetVersionsFilterLocale(values []string) BackgroundAssetVersionsOption {
	return func(q *backgroundAssetVersionsQuery) {
		q.locales = normalizeList(values)
	}
}

// WithBackgroundAssetVersionsFilterPlatforms filters background asset versions by platform.
func WithBackgroundAssetVersionsFilterPlatforms(values []string) BackgroundAssetVersionsOption {
	return func(q *backgroundAssetVersionsQuery) {
		q.platforms = normalizeList(values)
	}
}

// WithBackgroundAssetVersionsFilterState filters background asset versions by state.
func WithBackgroundAssetVersionsFilterState(values []string) BackgroundAssetVersionsOption {
	return func(q *backgroundAssetVersionsQuery) {
		q.states = normalizeList(values)
	}
}

// WithBackgroundAssetVersionsFilterVersion filters background asset versions by version.
func WithBackgroundAssetVersionsFilterVersion(values []string) BackgroundAssetVersionsOption {
	return func(q *backgroundAssetVersionsQuery) {
		q.versions = normalizeList(values)
	}
}

// WithBackgroundAssetVersionsFilterInternalBetaState filters background asset versions by internal beta release state.
func WithBackgroundAssetVersionsFilterInternalBetaState(values []string) BackgroundAssetVersionsOption {
	return func(q *backgroundAssetVersionsQuery) {
		q.internalBetaStates = normalizeList(values)
	}
}

// WithBackgroundAssetVersionsFilterExternalBetaState filters background asset versions by external beta release state.
func WithBackgroundAssetVersionsFilterExternalBetaState(values []string) BackgroundAssetVersionsOption {
	return func(q *backgroundAssetVersionsQuery) {
		q.externalBetaStates = normalizeList(values)
	}
}

// WithBackgroundAssetVersionsFilterAppStoreState filters background asset versions by App Store release state.
func WithBackgroundAssetVersionsFilterAppStoreState(values []string) BackgroundAssetVersionsOption {
	return func(q *backgroundAssetVersionsQuery) {
		q.appStoreStates = normalizeList(values)
	}
}

// WithBackgroundAssetVersionsSort sets the sort order for background asset versions.
func WithBackgroundAssetVersionsSort(sort string) BackgroundAssetVersionsOption {
	return func(q *backgroundAssetVersionsQuery) {
		q.sort = strings.TrimSpace(sort)
	}
}

// WithAndroidToIosAppMappingDetailsFields sets fields[androidToIosAppMappingDetails].
func WithAndroidToIosAppMappingDetailsFields(fields []string) AndroidToIosAppMappingDetailsOption {
	return func(q *androidToIosAppMappingDetailsQuery) {
		q.fields = normalizeList(fields)
	}
}
