package asc

import "net/url"

// IAPReviewScreenshotOption configures review screenshot detail and related-resource reads.
type IAPReviewScreenshotOption func(*iapReviewScreenshotQuery)

// IAPContentOption configures content detail and related-resource reads.
type IAPContentOption func(*iapContentQuery)

// PromotedPurchaseGetOption configures promoted purchase detail and related-resource reads.
type PromotedPurchaseGetOption func(*promotedPurchaseGetQuery)

type iapReviewScreenshotQuery struct {
	screenshotFields []string
	iapFields        []string
	include          []string
}

type iapContentQuery struct {
	iapFields []string
	include   []string
}

type promotedPurchaseGetQuery struct {
	iapFields          []string
	subscriptionFields []string
	include            []string
}

// WithIAPReviewScreenshotFields sets fields[inAppPurchaseAppStoreReviewScreenshots].
func WithIAPReviewScreenshotFields(fields []string) IAPReviewScreenshotOption {
	return func(q *iapReviewScreenshotQuery) { q.screenshotFields = normalizeUniqueList(fields) }
}

// WithIAPReviewScreenshotIAPFields sets fields[inAppPurchases] for included IAPs.
func WithIAPReviewScreenshotIAPFields(fields []string) IAPReviewScreenshotOption {
	return func(q *iapReviewScreenshotQuery) { q.iapFields = normalizeUniqueList(fields) }
}

// WithIAPReviewScreenshotInclude sets the exact screenshot relationship include set.
func WithIAPReviewScreenshotInclude(include []string) IAPReviewScreenshotOption {
	return func(q *iapReviewScreenshotQuery) { q.include = normalizeUniqueList(include) }
}

// WithIAPContentIAPFields sets fields[inAppPurchases] for included IAPs.
func WithIAPContentIAPFields(fields []string) IAPContentOption {
	return func(q *iapContentQuery) { q.iapFields = normalizeUniqueList(fields) }
}

// WithIAPContentInclude sets the exact content relationship include set.
func WithIAPContentInclude(include []string) IAPContentOption {
	return func(q *iapContentQuery) { q.include = normalizeUniqueList(include) }
}

// WithPromotedPurchaseIAPFields sets fields[inAppPurchases] for an included IAP.
func WithPromotedPurchaseIAPFields(fields []string) PromotedPurchaseGetOption {
	return func(q *promotedPurchaseGetQuery) { q.iapFields = normalizeUniqueList(fields) }
}

// WithPromotedPurchaseSubscriptionFields sets fields[subscriptions] for an included subscription.
func WithPromotedPurchaseSubscriptionFields(fields []string) PromotedPurchaseGetOption {
	return func(q *promotedPurchaseGetQuery) { q.subscriptionFields = normalizeUniqueList(fields) }
}

// WithPromotedPurchaseInclude sets the exact promoted-purchase relationship include set.
func WithPromotedPurchaseInclude(include []string) PromotedPurchaseGetOption {
	return func(q *promotedPurchaseGetQuery) { q.include = normalizeUniqueList(include) }
}

func buildIAPReviewScreenshotQuery(query *iapReviewScreenshotQuery) string {
	values := url.Values{}
	addCSV(values, "fields[inAppPurchaseAppStoreReviewScreenshots]", query.screenshotFields)
	addCSV(values, "fields[inAppPurchases]", query.iapFields)
	addCSV(values, "include", includeWhenFieldsSelected(query.include, "inAppPurchaseV2", query.iapFields))
	return values.Encode()
}

func buildIAPContentQuery(query *iapContentQuery) string {
	values := url.Values{}
	addCSV(values, "fields[inAppPurchases]", query.iapFields)
	addCSV(values, "include", includeWhenFieldsSelected(query.include, "inAppPurchaseV2", query.iapFields))
	return values.Encode()
}

func buildPromotedPurchaseGetQuery(query *promotedPurchaseGetQuery) string {
	values := url.Values{}
	addCSV(values, "fields[inAppPurchases]", query.iapFields)
	addCSV(values, "fields[subscriptions]", query.subscriptionFields)
	include := includeWhenFieldsSelected(query.include, "inAppPurchaseV2", query.iapFields)
	include = includeWhenFieldsSelected(include, "subscription", query.subscriptionFields)
	addCSV(values, "include", include)
	return values.Encode()
}

func includeWhenFieldsSelected(include []string, relationship string, fields []string) []string {
	if len(fields) == 0 {
		return normalizeUniqueList(include)
	}
	return normalizeUniqueList(append(append([]string{}, include...), relationship))
}
