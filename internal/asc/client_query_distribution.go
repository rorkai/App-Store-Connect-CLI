package asc

import (
	"net/url"
	"strconv"
	"strings"
)

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
	versionsLocales      []string
}

type backgroundAssetVersionsQuery struct {
	listQuery
	locales []string
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
	addCSV(values, "filter[versions.locale]", query.versionsLocales)
	addLimit(values, query.limit)
	return values.Encode()
}

func buildBackgroundAssetVersionsQuery(query *backgroundAssetVersionsQuery) string {
	values := url.Values{}
	addCSV(values, "filter[locale]", query.locales)
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

// WithAlternativeDistributionDomainsFields sets fields[alternativeDistributionDomains].
func WithAlternativeDistributionDomainsFields(fields []string) AlternativeDistributionDomainsOption {
	return func(q *alternativeDistributionDomainsQuery) {
		q.fields = normalizeList(fields)
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

// WithAlternativeDistributionKeysFields sets fields[alternativeDistributionKeys].
func WithAlternativeDistributionKeysFields(fields []string) AlternativeDistributionKeysOption {
	return func(q *alternativeDistributionKeysQuery) {
		q.fields = normalizeList(fields)
	}
}

// WithAlternativeDistributionKeysExistsApp filters keys by whether they belong to an app.
func WithAlternativeDistributionKeysExistsApp(exists bool) AlternativeDistributionKeysOption {
	return func(q *alternativeDistributionKeysQuery) {
		q.existsApp = &exists
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

// WithAlternativeDistributionPackageVariantsFields sets fields[alternativeDistributionPackageVariants].
func WithAlternativeDistributionPackageVariantsFields(fields []string) AlternativeDistributionPackageVariantsOption {
	return func(q *alternativeDistributionPackageVariantsQuery) {
		q.fields = normalizeList(fields)
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

// WithAlternativeDistributionPackageDeltasFields sets fields[alternativeDistributionPackageDeltas].
func WithAlternativeDistributionPackageDeltasFields(fields []string) AlternativeDistributionPackageDeltasOption {
	return func(q *alternativeDistributionPackageDeltasQuery) {
		q.fields = normalizeList(fields)
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

// WithWebhookDeliveriesFields sets fields[webhookDeliveries].
func WithWebhookDeliveriesFields(fields []string) WebhookDeliveriesOption {
	return func(q *webhookDeliveriesQuery) {
		q.fields = normalizeList(fields)
	}
}

// WithWebhookDeliveriesEventFields sets fields[webhookEvents].
func WithWebhookDeliveriesEventFields(fields []string) WebhookDeliveriesOption {
	return func(q *webhookDeliveriesQuery) {
		q.eventFields = normalizeList(fields)
	}
}

// WithWebhookDeliveriesInclude sets related resources to include.
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

// WithBackgroundAssetsFilterVersionsLocale filters background assets by uploaded version locale.
func WithBackgroundAssetsFilterVersionsLocale(values []string) BackgroundAssetsOption {
	return func(q *backgroundAssetsQuery) {
		q.versionsLocales = normalizeList(values)
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

// WithBackgroundAssetVersionsFilterLocale filters background asset versions by locale.
func WithBackgroundAssetVersionsFilterLocale(values []string) BackgroundAssetVersionsOption {
	return func(q *backgroundAssetVersionsQuery) {
		q.locales = normalizeList(values)
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
