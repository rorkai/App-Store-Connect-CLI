package asc

import (
	"net/url"
)

type appQuery struct {
	inAppPurchaseFields     []string
	subscriptionGroupFields []string
	include                 []string
}

type appInfoLocalizationQuery struct {
	appInfoFields []string
	include       []string
}

type ageRatingDeclarationQuery struct {
	fields []string
}

type ciProductAppQuery struct {
	inAppPurchaseFields     []string
	subscriptionGroupFields []string
	include                 []string
}

// AppOption configures an app detail request.
type AppOption func(*appQuery)

// AppInfoLocalizationOption configures an app info localization detail request.
type AppInfoLocalizationOption func(*appInfoLocalizationQuery)

// AgeRatingDeclarationOption configures an age rating declaration read request.
type AgeRatingDeclarationOption func(*ageRatingDeclarationQuery)

// CiProductAppOption configures the app request for a CI product.
type CiProductAppOption func(*ciProductAppQuery)

func buildAppQuery(query *appQuery) string {
	values := url.Values{}
	addCSV(values, "fields[inAppPurchases]", query.inAppPurchaseFields)
	addCSV(values, "fields[subscriptionGroups]", query.subscriptionGroupFields)
	addCSV(values, "include", query.include)
	return values.Encode()
}

func buildAppInfoLocalizationQuery(query *appInfoLocalizationQuery) string {
	values := url.Values{}
	addCSV(values, "fields[appInfos]", query.appInfoFields)
	addCSV(values, "include", query.include)
	return values.Encode()
}

func buildAgeRatingDeclarationQuery(query *ageRatingDeclarationQuery) string {
	values := url.Values{}
	addCSV(values, "fields[ageRatingDeclarations]", query.fields)
	return values.Encode()
}

func buildCiProductAppQuery(query *ciProductAppQuery) string {
	values := url.Values{}
	addCSV(values, "fields[inAppPurchases]", query.inAppPurchaseFields)
	addCSV(values, "fields[subscriptionGroups]", query.subscriptionGroupFields)
	addCSV(values, "include", query.include)
	return values.Encode()
}

// WithAppInAppPurchaseFields sets fields[inAppPurchases].
func WithAppInAppPurchaseFields(fields []string) AppOption {
	return func(q *appQuery) { q.inAppPurchaseFields = normalizeList(fields) }
}

// WithAppSubscriptionGroupFields sets fields[subscriptionGroups].
func WithAppSubscriptionGroupFields(fields []string) AppOption {
	return func(q *appQuery) { q.subscriptionGroupFields = normalizeList(fields) }
}

// WithAppInclude includes related resources in an app detail response.
func WithAppInclude(include []string) AppOption {
	return func(q *appQuery) { q.include = normalizeList(include) }
}

// WithAppInfoLocalizationAppInfoFields sets fields[appInfos].
func WithAppInfoLocalizationAppInfoFields(fields []string) AppInfoLocalizationOption {
	return func(q *appInfoLocalizationQuery) { q.appInfoFields = normalizeList(fields) }
}

// WithAppInfoLocalizationInclude includes related resources.
func WithAppInfoLocalizationInclude(include []string) AppInfoLocalizationOption {
	return func(q *appInfoLocalizationQuery) { q.include = normalizeList(include) }
}

// WithAgeRatingDeclarationFields sets fields[ageRatingDeclarations].
func WithAgeRatingDeclarationFields(fields []string) AgeRatingDeclarationOption {
	return func(q *ageRatingDeclarationQuery) { q.fields = normalizeList(fields) }
}

// WithCiProductAppInAppPurchaseFields sets fields[inAppPurchases].
func WithCiProductAppInAppPurchaseFields(fields []string) CiProductAppOption {
	return func(q *ciProductAppQuery) { q.inAppPurchaseFields = normalizeList(fields) }
}

// WithCiProductAppSubscriptionGroupFields sets fields[subscriptionGroups].
func WithCiProductAppSubscriptionGroupFields(fields []string) CiProductAppOption {
	return func(q *ciProductAppQuery) { q.subscriptionGroupFields = normalizeList(fields) }
}

// WithCiProductAppInclude includes related resources in the app response.
func WithCiProductAppInclude(include []string) CiProductAppOption {
	return func(q *ciProductAppQuery) { q.include = normalizeList(include) }
}
