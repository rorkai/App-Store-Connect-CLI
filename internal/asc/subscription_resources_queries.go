package asc

import (
	"fmt"
	"net/url"
	"strings"
)

// SubscriptionLocalizationsOption is a functional option for subscription localizations list endpoints.
type SubscriptionLocalizationsOption func(*subscriptionLocalizationsQuery)

// SubscriptionImagesOption is a functional option for subscription images list endpoints.
type SubscriptionImagesOption func(*subscriptionImagesQuery)

// SubscriptionIntroductoryOffersOption is a functional option for introductory offers list endpoints.
type SubscriptionIntroductoryOffersOption func(*subscriptionIntroductoryOffersQuery)

// SubscriptionPromotionalOffersOption is a functional option for promotional offers list endpoints.
type SubscriptionPromotionalOffersOption func(*subscriptionPromotionalOffersQuery)

// SubscriptionPromotionalOfferPricesOption is a functional option for promotional offer prices list endpoints.
type SubscriptionPromotionalOfferPricesOption func(*subscriptionPromotionalOfferPricesQuery)

// SubscriptionOfferCodesOption is a functional option for offer codes list endpoints.
type SubscriptionOfferCodesOption func(*subscriptionOfferCodesQuery)

// SubscriptionOfferCodeCustomCodesOption is a functional option for offer code custom codes list endpoints.
type SubscriptionOfferCodeCustomCodesOption func(*subscriptionOfferCodeCustomCodesQuery)

// SubscriptionOfferCodePricesOption is a functional option for offer code prices list endpoints.
type SubscriptionOfferCodePricesOption func(*subscriptionOfferCodePricesQuery)

// SubscriptionPricePointsOption is a functional option for subscription price point list endpoints.
type SubscriptionPricePointsOption func(*subscriptionPricePointsQuery)

// SubscriptionPricesOption is a functional option for subscription price list endpoints.
type SubscriptionPricesOption func(*subscriptionPricesQuery)

// SubscriptionGroupLocalizationsOption is a functional option for subscription group localization list endpoints.
type SubscriptionGroupLocalizationsOption func(*subscriptionGroupLocalizationsQuery)

// SubscriptionAppStoreReviewScreenshotOption configures the subscription screenshot relationship endpoint.
type SubscriptionAppStoreReviewScreenshotOption func(*subscriptionAppStoreReviewScreenshotQuery)

type subscriptionLocalizationsQuery struct {
	listQuery
	fields []string
}

type subscriptionImagesQuery struct {
	listQuery
}

type subscriptionIntroductoryOffersQuery struct {
	listQuery
	fields  []string
	include []string
}

type subscriptionPromotionalOffersQuery struct {
	listQuery
}

type subscriptionPromotionalOfferPricesQuery struct {
	listQuery
}

type subscriptionOfferCodesQuery struct {
	listQuery
}

type subscriptionOfferCodeCustomCodesQuery struct {
	listQuery
}

type subscriptionOfferCodePricesQuery struct {
	listQuery
}

type subscriptionPricePointsQuery struct {
	listQuery
	territories          []string
	subscriptionIDs      []string
	upfrontPricePointIDs []string
	planTypes            []string
	include              []string
	pricePointFields     []string
	territoryFields      []string
}

type subscriptionPricesQuery struct {
	listQuery
	territory        string
	planType         SubscriptionPlanType
	include          []string
	priceFields      []string
	pricePointFields []string
	territoryFields  []string
}

type subscriptionGroupLocalizationsQuery struct {
	listQuery
	fields []string
}

type subscriptionAppStoreReviewScreenshotQuery struct {
	fields []string
}

// WithSubscriptionLocalizationsLimit sets the max number of localizations to return.
func WithSubscriptionLocalizationsLimit(limit int) SubscriptionLocalizationsOption {
	return func(q *subscriptionLocalizationsQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithSubscriptionLocalizationsNextURL uses a next page URL directly.
func WithSubscriptionLocalizationsNextURL(next string) SubscriptionLocalizationsOption {
	return func(q *subscriptionLocalizationsQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

// WithSubscriptionLocalizationsFields sets fields for returned subscription localizations.
func WithSubscriptionLocalizationsFields(fields []string) SubscriptionLocalizationsOption {
	return func(q *subscriptionLocalizationsQuery) {
		q.fields = normalizeList(fields)
	}
}

// WithSubscriptionImagesLimit sets the max number of images to return.
func WithSubscriptionImagesLimit(limit int) SubscriptionImagesOption {
	return func(q *subscriptionImagesQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithSubscriptionImagesNextURL uses a next page URL directly.
func WithSubscriptionImagesNextURL(next string) SubscriptionImagesOption {
	return func(q *subscriptionImagesQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

// WithSubscriptionIntroductoryOffersLimit sets the max number of offers to return.
func WithSubscriptionIntroductoryOffersLimit(limit int) SubscriptionIntroductoryOffersOption {
	return func(q *subscriptionIntroductoryOffersQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithSubscriptionIntroductoryOffersNextURL uses a next page URL directly.
func WithSubscriptionIntroductoryOffersNextURL(next string) SubscriptionIntroductoryOffersOption {
	return func(q *subscriptionIntroductoryOffersQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

// WithSubscriptionIntroductoryOffersFields sets fields for returned introductory offers.
func WithSubscriptionIntroductoryOffersFields(fields []string) SubscriptionIntroductoryOffersOption {
	return func(q *subscriptionIntroductoryOffersQuery) {
		q.fields = normalizeList(fields)
	}
}

// WithSubscriptionIntroductoryOffersInclude sets introductory offer relationships to include.
func WithSubscriptionIntroductoryOffersInclude(include []string) SubscriptionIntroductoryOffersOption {
	return func(q *subscriptionIntroductoryOffersQuery) {
		q.include = normalizeList(include)
	}
}

// WithSubscriptionPromotionalOffersLimit sets the max number of offers to return.
func WithSubscriptionPromotionalOffersLimit(limit int) SubscriptionPromotionalOffersOption {
	return func(q *subscriptionPromotionalOffersQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithSubscriptionPromotionalOffersNextURL uses a next page URL directly.
func WithSubscriptionPromotionalOffersNextURL(next string) SubscriptionPromotionalOffersOption {
	return func(q *subscriptionPromotionalOffersQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

// WithSubscriptionPromotionalOfferPricesLimit sets the max number of prices to return.
func WithSubscriptionPromotionalOfferPricesLimit(limit int) SubscriptionPromotionalOfferPricesOption {
	return func(q *subscriptionPromotionalOfferPricesQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithSubscriptionPromotionalOfferPricesNextURL uses a next page URL directly.
func WithSubscriptionPromotionalOfferPricesNextURL(next string) SubscriptionPromotionalOfferPricesOption {
	return func(q *subscriptionPromotionalOfferPricesQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

// WithSubscriptionOfferCodesLimit sets the max number of offer codes to return.
func WithSubscriptionOfferCodesLimit(limit int) SubscriptionOfferCodesOption {
	return func(q *subscriptionOfferCodesQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithSubscriptionOfferCodesNextURL uses a next page URL directly.
func WithSubscriptionOfferCodesNextURL(next string) SubscriptionOfferCodesOption {
	return func(q *subscriptionOfferCodesQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

// WithSubscriptionOfferCodeCustomCodesLimit sets the max number of custom codes to return.
func WithSubscriptionOfferCodeCustomCodesLimit(limit int) SubscriptionOfferCodeCustomCodesOption {
	return func(q *subscriptionOfferCodeCustomCodesQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithSubscriptionOfferCodeCustomCodesNextURL uses a next page URL directly.
func WithSubscriptionOfferCodeCustomCodesNextURL(next string) SubscriptionOfferCodeCustomCodesOption {
	return func(q *subscriptionOfferCodeCustomCodesQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

// WithSubscriptionOfferCodePricesLimit sets the max number of prices to return.
func WithSubscriptionOfferCodePricesLimit(limit int) SubscriptionOfferCodePricesOption {
	return func(q *subscriptionOfferCodePricesQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithSubscriptionOfferCodePricesNextURL uses a next page URL directly.
func WithSubscriptionOfferCodePricesNextURL(next string) SubscriptionOfferCodePricesOption {
	return func(q *subscriptionOfferCodePricesQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

// WithSubscriptionPricePointsLimit sets the max number of price points to return.
func WithSubscriptionPricePointsLimit(limit int) SubscriptionPricePointsOption {
	return func(q *subscriptionPricePointsQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithSubscriptionPricePointsNextURL uses a next page URL directly.
func WithSubscriptionPricePointsNextURL(next string) SubscriptionPricePointsOption {
	return func(q *subscriptionPricePointsQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

// WithSubscriptionPricePointsTerritory filters price points by territory (e.g., "USA").
func WithSubscriptionPricePointsTerritory(territory string) SubscriptionPricePointsOption {
	return func(q *subscriptionPricePointsQuery) {
		if strings.TrimSpace(territory) != "" {
			q.territories = []string{strings.ToUpper(strings.TrimSpace(territory))}
		}
	}
}

// WithSubscriptionPricePointsTerritories filters price points by territory IDs.
func WithSubscriptionPricePointsTerritories(territories []string) SubscriptionPricePointsOption {
	return func(q *subscriptionPricePointsQuery) {
		q.territories = normalizeList(territories)
	}
}

// WithSubscriptionPricePointsSubscriptions filters equalizations by subscription IDs.
func WithSubscriptionPricePointsSubscriptions(subscriptionIDs []string) SubscriptionPricePointsOption {
	return func(q *subscriptionPricePointsQuery) {
		q.subscriptionIDs = normalizeList(subscriptionIDs)
	}
}

// WithSubscriptionPricePointsUpfrontPricePointIDs filters by upfront price point IDs.
func WithSubscriptionPricePointsUpfrontPricePointIDs(pricePointIDs []string) SubscriptionPricePointsOption {
	return func(q *subscriptionPricePointsQuery) {
		q.upfrontPricePointIDs = normalizeList(pricePointIDs)
	}
}

// WithSubscriptionPricePointsPlanTypes filters by billing plan types.
func WithSubscriptionPricePointsPlanTypes(planTypes []string) SubscriptionPricePointsOption {
	return func(q *subscriptionPricePointsQuery) {
		q.planTypes = normalizeList(planTypes)
	}
}

// WithSubscriptionPricePointsInclude sets the relationships to include (e.g., "territory").
func WithSubscriptionPricePointsInclude(include []string) SubscriptionPricePointsOption {
	return func(q *subscriptionPricePointsQuery) {
		q.include = normalizeList(include)
	}
}

// WithSubscriptionPricePointsFields sets fields for returned subscriptionPricePoints resources.
func WithSubscriptionPricePointsFields(fields []string) SubscriptionPricePointsOption {
	return func(q *subscriptionPricePointsQuery) {
		q.pricePointFields = normalizeList(fields)
	}
}

// WithSubscriptionPricePointsTerritoryFields sets fields for included territories.
func WithSubscriptionPricePointsTerritoryFields(fields []string) SubscriptionPricePointsOption {
	return func(q *subscriptionPricePointsQuery) {
		q.territoryFields = normalizeList(fields)
	}
}

// WithSubscriptionPricesLimit sets the max number of prices to return.
func WithSubscriptionPricesLimit(limit int) SubscriptionPricesOption {
	return func(q *subscriptionPricesQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithSubscriptionPricesNextURL uses a next page URL directly.
func WithSubscriptionPricesNextURL(next string) SubscriptionPricesOption {
	return func(q *subscriptionPricesQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

// WithSubscriptionPricesTerritory filters subscription prices by territory (e.g., "USA").
func WithSubscriptionPricesTerritory(territory string) SubscriptionPricesOption {
	return func(q *subscriptionPricesQuery) {
		if strings.TrimSpace(territory) != "" {
			q.territory = strings.ToUpper(strings.TrimSpace(territory))
		}
	}
}

// WithSubscriptionPricesPlanType filters subscription prices by plan type (MONTHLY or UPFRONT).
func WithSubscriptionPricesPlanType(planType SubscriptionPlanType) SubscriptionPricesOption {
	return func(q *subscriptionPricesQuery) {
		if planType != "" {
			q.planType = planType
		}
	}
}

// WithSubscriptionPricesInclude sets the relationships to include (e.g., "subscriptionPricePoint", "territory").
func WithSubscriptionPricesInclude(include []string) SubscriptionPricesOption {
	return func(q *subscriptionPricesQuery) {
		q.include = normalizeList(include)
	}
}

// WithSubscriptionPricesFields sets fields for returned subscription prices.
func WithSubscriptionPricesFields(fields []string) SubscriptionPricesOption {
	return func(q *subscriptionPricesQuery) {
		q.priceFields = normalizeList(fields)
	}
}

// WithSubscriptionPricesPricePointFields sets fields for included subscriptionPricePoints.
func WithSubscriptionPricesPricePointFields(fields []string) SubscriptionPricesOption {
	return func(q *subscriptionPricesQuery) {
		q.pricePointFields = normalizeList(fields)
	}
}

// WithSubscriptionPricesTerritoryFields sets fields for included territories.
func WithSubscriptionPricesTerritoryFields(fields []string) SubscriptionPricesOption {
	return func(q *subscriptionPricesQuery) {
		q.territoryFields = normalizeList(fields)
	}
}

// WithSubscriptionGroupLocalizationsLimit sets the max number of group localizations to return.
func WithSubscriptionGroupLocalizationsLimit(limit int) SubscriptionGroupLocalizationsOption {
	return func(q *subscriptionGroupLocalizationsQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithSubscriptionGroupLocalizationsNextURL uses a next page URL directly.
func WithSubscriptionGroupLocalizationsNextURL(next string) SubscriptionGroupLocalizationsOption {
	return func(q *subscriptionGroupLocalizationsQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

// WithSubscriptionGroupLocalizationsFields sets fields for returned subscription group localizations.
func WithSubscriptionGroupLocalizationsFields(fields []string) SubscriptionGroupLocalizationsOption {
	return func(q *subscriptionGroupLocalizationsQuery) {
		q.fields = normalizeList(fields)
	}
}

// WithSubscriptionAppStoreReviewScreenshotFields limits returned screenshot fields.
func WithSubscriptionAppStoreReviewScreenshotFields(fields []string) SubscriptionAppStoreReviewScreenshotOption {
	return func(q *subscriptionAppStoreReviewScreenshotQuery) {
		q.fields = normalizeList(fields)
	}
}

func buildSubscriptionLocalizationsQuery(query *subscriptionLocalizationsQuery) string {
	values := url.Values{}
	addCSV(values, "fields[subscriptionLocalizations]", query.fields)
	addLimit(values, query.limit)
	return values.Encode()
}

func buildSubscriptionImagesQuery(query *subscriptionImagesQuery) string {
	values := url.Values{}
	addLimit(values, query.limit)
	return values.Encode()
}

func buildSubscriptionIntroductoryOffersQuery(query *subscriptionIntroductoryOffersQuery) string {
	values := url.Values{}
	addCSV(values, "fields[subscriptionIntroductoryOffers]", query.fields)
	addCSV(values, "include", query.include)
	addLimit(values, query.limit)
	return values.Encode()
}

func buildSubscriptionPromotionalOffersQuery(query *subscriptionPromotionalOffersQuery) string {
	values := url.Values{}
	addLimit(values, query.limit)
	return values.Encode()
}

func buildSubscriptionPromotionalOfferPricesQuery(query *subscriptionPromotionalOfferPricesQuery) string {
	values := url.Values{}
	addLimit(values, query.limit)
	return values.Encode()
}

func buildSubscriptionOfferCodesQuery(query *subscriptionOfferCodesQuery) string {
	values := url.Values{}
	addLimit(values, query.limit)
	return values.Encode()
}

func buildSubscriptionOfferCodeCustomCodesQuery(query *subscriptionOfferCodeCustomCodesQuery) string {
	values := url.Values{}
	addLimit(values, query.limit)
	return values.Encode()
}

func buildSubscriptionOfferCodePricesQuery(query *subscriptionOfferCodePricesQuery) string {
	values := url.Values{}
	addLimit(values, query.limit)
	return values.Encode()
}

func buildSubscriptionPricePointsQuery(query *subscriptionPricePointsQuery) string {
	values := url.Values{}
	addCSV(values, "filter[territory]", query.territories)
	addCSV(values, "filter[subscription]", query.subscriptionIDs)
	addCSV(values, "filter[upfrontPricePointId]", query.upfrontPricePointIDs)
	addCSV(values, "filter[planType]", query.planTypes)
	addCSV(values, "include", query.include)
	addCSV(values, "fields[subscriptionPricePoints]", query.pricePointFields)
	addCSV(values, "fields[territories]", query.territoryFields)
	addLimit(values, query.limit)
	return values.Encode()
}

func mergeSubscriptionPricePointsQuery(rawURL string, query *subscriptionPricePointsQuery) (string, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("invalid pagination URL: %w", err)
	}
	values := parsed.Query()
	extra, err := url.ParseQuery(buildSubscriptionPricePointsQuery(query))
	if err != nil {
		return "", fmt.Errorf("build subscription price points query: %w", err)
	}
	for key, items := range extra {
		// Pagination URLs are opaque cursors. Preserve Apple's page-size choice
		// instead of replacing it with the caller's first-page limit.
		if key == "limit" {
			continue
		}
		values.Del(key)
		for _, item := range items {
			values.Add(key, item)
		}
	}
	parsed.RawQuery = values.Encode()
	return parsed.String(), nil
}

func buildSubscriptionPricesQuery(query *subscriptionPricesQuery) string {
	values := url.Values{}
	if strings.TrimSpace(query.territory) != "" {
		values.Set("filter[territory]", strings.TrimSpace(query.territory))
	}
	if query.planType != "" {
		values.Set("filter[planType]", string(query.planType))
	}
	addCSV(values, "include", query.include)
	addCSV(values, "fields[subscriptionPrices]", query.priceFields)
	addCSV(values, "fields[subscriptionPricePoints]", query.pricePointFields)
	addCSV(values, "fields[territories]", query.territoryFields)
	addLimit(values, query.limit)
	return values.Encode()
}

func buildSubscriptionGroupLocalizationsQuery(query *subscriptionGroupLocalizationsQuery) string {
	values := url.Values{}
	addCSV(values, "fields[subscriptionGroupLocalizations]", query.fields)
	addLimit(values, query.limit)
	return values.Encode()
}

func buildSubscriptionAppStoreReviewScreenshotQuery(query *subscriptionAppStoreReviewScreenshotQuery) string {
	values := url.Values{}
	addCSV(values, "fields[subscriptionAppStoreReviewScreenshots]", query.fields)
	return values.Encode()
}
