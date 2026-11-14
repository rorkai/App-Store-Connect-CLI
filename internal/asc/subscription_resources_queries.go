package asc

import (
	"net/url"
	"strings"
)

// SubscriptionIntroductoryOffersOption is a functional option for introductory offers list endpoints.
type SubscriptionIntroductoryOffersOption func(*subscriptionIntroductoryOffersQuery)

// SubscriptionPricePointsOption configures the subscription-scoped price-points endpoint.
// It intentionally excludes equalization-only filters such as filter[subscription].
type SubscriptionPricePointsOption interface {
	applySubscriptionPricePoints(*subscriptionPricePointsQuery)
	subscriptionPricePointsOption()
}

// SubscriptionPricePointEqualizationsOption configures equalizations and adjusted-equalizations endpoints.
type SubscriptionPricePointEqualizationsOption interface {
	applySubscriptionPricePoints(*subscriptionPricePointsQuery)
	subscriptionPricePointEqualizationsOption()
}

// SubscriptionPricePointsCommonOption is accepted by both subscription-scoped and equalization endpoints.
type SubscriptionPricePointsCommonOption func(*subscriptionPricePointsQuery)

func (option SubscriptionPricePointsCommonOption) applySubscriptionPricePoints(query *subscriptionPricePointsQuery) {
	option(query)
}

func (SubscriptionPricePointsCommonOption) subscriptionPricePointsOption()             {}
func (SubscriptionPricePointsCommonOption) subscriptionPricePointEqualizationsOption() {}

type subscriptionPricePointEqualizationsOnlyOption func(*subscriptionPricePointsQuery)

func (option subscriptionPricePointEqualizationsOnlyOption) applySubscriptionPricePoints(query *subscriptionPricePointsQuery) {
	option(query)
}

func (subscriptionPricePointEqualizationsOnlyOption) subscriptionPricePointEqualizationsOption() {}

// SubscriptionPricesOption is a functional option for subscription price list endpoints.
type SubscriptionPricesOption func(*subscriptionPricesQuery)

// SubscriptionAppStoreReviewScreenshotOption configures the subscription screenshot relationship endpoint.
type SubscriptionAppStoreReviewScreenshotOption func(*subscriptionAppStoreReviewScreenshotQuery)

// SubscriptionOfferCodeOption configures a subscription offer code detail read.
type SubscriptionOfferCodeOption func(*subscriptionRelatedFieldsQuery)

// SubscriptionPromotionalOfferOption configures a subscription promotional offer detail read.
type SubscriptionPromotionalOfferOption func(*subscriptionRelatedFieldsQuery)

type subscriptionRelatedFieldsQuery struct {
	parentFields []string
	include      []string
}

type subscriptionIntroductoryOffersQuery struct {
	listQuery
	territories        []string
	fields             []string
	subscriptionFields []string
	pricePointFields   []string
	include            []string
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
	pricePointIDs    []string
	include          []string
	priceFields      []string
	pricePointFields []string
	territoryFields  []string
}

type subscriptionAppStoreReviewScreenshotQuery struct {
	fields             []string
	subscriptionFields []string
	include            []string
}

// WithSubscriptionIntroductoryOffersTerritories filters introductory offers by territory IDs.
func WithSubscriptionIntroductoryOffersTerritories(territories []string) SubscriptionIntroductoryOffersOption {
	return func(q *subscriptionIntroductoryOffersQuery) {
		q.territories = normalizeUpperList(territories)
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

// WithSubscriptionIntroductoryOffersSubscriptionFields sets fields[subscriptions] for included subscriptions.
func WithSubscriptionIntroductoryOffersSubscriptionFields(fields []string) SubscriptionIntroductoryOffersOption {
	return func(q *subscriptionIntroductoryOffersQuery) {
		q.subscriptionFields = normalizeList(fields)
	}
}

// WithSubscriptionIntroductoryOffersPricePointFields sets fields[subscriptionPricePoints] for included price points.
func WithSubscriptionIntroductoryOffersPricePointFields(fields []string) SubscriptionIntroductoryOffersOption {
	return func(q *subscriptionIntroductoryOffersQuery) {
		q.pricePointFields = normalizeList(fields)
	}
}

// WithSubscriptionPricePointsLimit sets the max number of price points to return.
func WithSubscriptionPricePointsLimit(limit int) SubscriptionPricePointsCommonOption {
	return SubscriptionPricePointsCommonOption(func(q *subscriptionPricePointsQuery) {
		if limit > 0 {
			q.limit = limit
		}
	})
}

// WithSubscriptionPricePointsNextURL uses a next page URL directly.
func WithSubscriptionPricePointsNextURL(next string) SubscriptionPricePointsCommonOption {
	return SubscriptionPricePointsCommonOption(func(q *subscriptionPricePointsQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	})
}

// WithSubscriptionPricePointsTerritory filters price points by territory (e.g., "USA").
func WithSubscriptionPricePointsTerritory(territory string) SubscriptionPricePointsCommonOption {
	return SubscriptionPricePointsCommonOption(func(q *subscriptionPricePointsQuery) {
		if strings.TrimSpace(territory) != "" {
			q.territories = []string{strings.ToUpper(strings.TrimSpace(territory))}
		}
	})
}

// WithSubscriptionPricePointsTerritories filters price points by territory IDs.
func WithSubscriptionPricePointsTerritories(territories []string) SubscriptionPricePointsCommonOption {
	return SubscriptionPricePointsCommonOption(func(q *subscriptionPricePointsQuery) {
		q.territories = normalizeList(territories)
	})
}

// WithSubscriptionPricePointsSubscriptions filters equalizations by subscription IDs.
func WithSubscriptionPricePointsSubscriptions(subscriptionIDs []string) SubscriptionPricePointEqualizationsOption {
	return subscriptionPricePointEqualizationsOnlyOption(func(q *subscriptionPricePointsQuery) {
		q.subscriptionIDs = normalizeList(subscriptionIDs)
	})
}

// WithSubscriptionPricePointsUpfrontPricePointIDs filters by upfront price point IDs.
func WithSubscriptionPricePointsUpfrontPricePointIDs(pricePointIDs []string) SubscriptionPricePointsCommonOption {
	return SubscriptionPricePointsCommonOption(func(q *subscriptionPricePointsQuery) {
		q.upfrontPricePointIDs = normalizeList(pricePointIDs)
	})
}

// WithSubscriptionPricePointsPlanTypes filters by billing plan types.
func WithSubscriptionPricePointsPlanTypes(planTypes []string) SubscriptionPricePointsCommonOption {
	return SubscriptionPricePointsCommonOption(func(q *subscriptionPricePointsQuery) {
		q.planTypes = normalizeList(planTypes)
	})
}

// WithSubscriptionPricePointsInclude sets the relationships to include (e.g., "territory").
func WithSubscriptionPricePointsInclude(include []string) SubscriptionPricePointsCommonOption {
	return SubscriptionPricePointsCommonOption(func(q *subscriptionPricePointsQuery) {
		q.include = normalizeList(include)
	})
}

// WithSubscriptionPricePointsFields sets fields for returned subscriptionPricePoints resources.
func WithSubscriptionPricePointsFields(fields []string) SubscriptionPricePointsCommonOption {
	return SubscriptionPricePointsCommonOption(func(q *subscriptionPricePointsQuery) {
		q.pricePointFields = normalizeList(fields)
	})
}

// WithSubscriptionPricePointsTerritoryFields sets fields for included territories.
func WithSubscriptionPricePointsTerritoryFields(fields []string) SubscriptionPricePointsCommonOption {
	return SubscriptionPricePointsCommonOption(func(q *subscriptionPricePointsQuery) {
		q.territoryFields = normalizeList(fields)
	})
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

// WithSubscriptionPricesPricePointIDs filters subscription prices by price point IDs.
func WithSubscriptionPricesPricePointIDs(pricePointIDs []string) SubscriptionPricesOption {
	return func(q *subscriptionPricesQuery) {
		q.pricePointIDs = normalizeList(pricePointIDs)
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

// WithSubscriptionAppStoreReviewScreenshotFields limits returned screenshot fields.
func WithSubscriptionAppStoreReviewScreenshotFields(fields []string) SubscriptionAppStoreReviewScreenshotOption {
	return func(q *subscriptionAppStoreReviewScreenshotQuery) {
		q.fields = normalizeList(fields)
	}
}

// WithSubscriptionAppStoreReviewScreenshotSubscriptionFields sets fields[subscriptions] for included subscriptions.
// It is accepted by both screenshot detail and subscription relationship reads.
func WithSubscriptionAppStoreReviewScreenshotSubscriptionFields(fields []string) SubscriptionAppStoreReviewScreenshotOption {
	return func(q *subscriptionAppStoreReviewScreenshotQuery) {
		q.subscriptionFields = normalizeList(fields)
	}
}

// WithSubscriptionAppStoreReviewScreenshotInclude sets relationships to include.
// It is accepted by both screenshot detail and subscription relationship reads.
func WithSubscriptionAppStoreReviewScreenshotInclude(include []string) SubscriptionAppStoreReviewScreenshotOption {
	return func(q *subscriptionAppStoreReviewScreenshotQuery) {
		q.include = normalizeList(include)
	}
}

// WithSubscriptionOfferCodeSubscriptionFields sets fields[subscriptions] on an offer code detail read.
func WithSubscriptionOfferCodeSubscriptionFields(fields []string) SubscriptionOfferCodeOption {
	return func(q *subscriptionRelatedFieldsQuery) {
		q.parentFields = normalizeList(fields)
	}
}

// WithSubscriptionOfferCodeInclude sets relationships to include on an offer code detail read.
func WithSubscriptionOfferCodeInclude(include []string) SubscriptionOfferCodeOption {
	return func(q *subscriptionRelatedFieldsQuery) {
		q.include = normalizeList(include)
	}
}

// WithSubscriptionPromotionalOfferSubscriptionFields sets fields[subscriptions] on a promotional offer detail read.
func WithSubscriptionPromotionalOfferSubscriptionFields(fields []string) SubscriptionPromotionalOfferOption {
	return func(q *subscriptionRelatedFieldsQuery) {
		q.parentFields = normalizeList(fields)
	}
}

// WithSubscriptionPromotionalOfferInclude sets relationships to include on a promotional offer detail read.
func WithSubscriptionPromotionalOfferInclude(include []string) SubscriptionPromotionalOfferOption {
	return func(q *subscriptionRelatedFieldsQuery) {
		q.include = normalizeList(include)
	}
}

func buildSubscriptionIntroductoryOffersQuery(query *subscriptionIntroductoryOffersQuery) string {
	values := url.Values{}
	addCSV(values, "filter[territory]", query.territories)
	addCSV(values, "fields[subscriptionIntroductoryOffers]", query.fields)
	addCSV(values, "fields[subscriptions]", query.subscriptionFields)
	addCSV(values, "fields[subscriptionPricePoints]", query.pricePointFields)
	addCSV(values, "include", query.include)
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

func subscriptionPricePointsQueryHasModifiers(query *subscriptionPricePointsQuery) bool {
	return query.limit != 0 || len(query.territories) != 0 || len(query.subscriptionIDs) != 0 ||
		len(query.upfrontPricePointIDs) != 0 || len(query.planTypes) != 0 || len(query.include) != 0 ||
		len(query.pricePointFields) != 0 || len(query.territoryFields) != 0
}

func buildSubscriptionPricesQuery(query *subscriptionPricesQuery) string {
	values := url.Values{}
	if strings.TrimSpace(query.territory) != "" {
		values.Set("filter[territory]", strings.TrimSpace(query.territory))
	}
	if query.planType != "" {
		values.Set("filter[planType]", string(query.planType))
	}
	addCSV(values, "filter[subscriptionPricePoint]", query.pricePointIDs)
	addCSV(values, "include", query.include)
	addCSV(values, "fields[subscriptionPrices]", query.priceFields)
	addCSV(values, "fields[subscriptionPricePoints]", query.pricePointFields)
	addCSV(values, "fields[territories]", query.territoryFields)
	addLimit(values, query.limit)
	return values.Encode()
}

func buildSubscriptionAppStoreReviewScreenshotQuery(query *subscriptionAppStoreReviewScreenshotQuery) string {
	values := url.Values{}
	addCSV(values, "fields[subscriptionAppStoreReviewScreenshots]", query.fields)
	addCSV(values, "fields[subscriptions]", query.subscriptionFields)
	addCSV(values, "include", query.include)
	return values.Encode()
}

func buildSubscriptionRelatedFieldsQuery(parentResource string, query *subscriptionRelatedFieldsQuery) string {
	values := url.Values{}
	addCSV(values, "fields["+parentResource+"]", query.parentFields)
	addCSV(values, "include", query.include)
	return values.Encode()
}

func subscriptionLocalizationsQueryHasModifiers(query *subscriptionLocalizationsQuery) bool {
	return query.limit != 0
}

func subscriptionImagesQueryHasModifiers(query *subscriptionImagesQuery) bool {
	return query.limit != 0
}

func subscriptionIntroductoryOffersQueryHasModifiers(query *subscriptionIntroductoryOffersQuery) bool {
	return query.limit != 0 || len(query.territories) != 0 || len(query.fields) != 0 || len(query.subscriptionFields) != 0 ||
		len(query.pricePointFields) != 0 || len(query.include) != 0
}

func subscriptionPromotionalOffersQueryHasModifiers(query *subscriptionPromotionalOffersQuery) bool {
	return query.limit != 0 || len(query.subscriptionFields) != 0 || len(query.include) != 0
}

func subscriptionPromotionalOfferPricesQueryHasModifiers(query *subscriptionPromotionalOfferPricesQuery) bool {
	return query.limit != 0 || len(query.pricePointFields) != 0 || len(query.include) != 0
}

func subscriptionOfferCodesQueryHasModifiers(query *subscriptionOfferCodesQuery) bool {
	return query.limit != 0 || len(query.subscriptionFields) != 0 || len(query.include) != 0
}

func subscriptionOfferCodePricesQueryHasModifiers(query *subscriptionOfferCodePricesQuery) bool {
	return query.limit != 0 || len(query.pricePointFields) != 0 || len(query.include) != 0
}

func subscriptionGroupLocalizationsQueryHasModifiers(query *subscriptionGroupLocalizationsQuery) bool {
	return query.limit != 0
}
