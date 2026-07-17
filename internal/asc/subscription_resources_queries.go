package asc

import (
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

// SubscriptionGroupLocalizationsOption is a functional option for subscription group localization list endpoints.
type SubscriptionGroupLocalizationsOption func(*subscriptionGroupLocalizationsQuery)

// SubscriptionAppStoreReviewScreenshotOption configures the subscription screenshot relationship endpoint.
type SubscriptionAppStoreReviewScreenshotOption func(*subscriptionAppStoreReviewScreenshotQuery)

// SubscriptionGroupLocalizationOption configures a v1 subscription group localization detail read.
type SubscriptionGroupLocalizationOption func(*subscriptionRelatedFieldsQuery)

// SubscriptionImageOption configures a v1 subscription image detail read.
type SubscriptionImageOption func(*subscriptionRelatedFieldsQuery)

// SubscriptionLocalizationOption configures a v1 subscription localization detail read.
type SubscriptionLocalizationOption func(*subscriptionRelatedFieldsQuery)

// SubscriptionOfferCodeOption configures a subscription offer code detail read.
type SubscriptionOfferCodeOption func(*subscriptionRelatedFieldsQuery)

// SubscriptionPromotionalOfferOption configures a subscription promotional offer detail read.
type SubscriptionPromotionalOfferOption func(*subscriptionRelatedFieldsQuery)

// SubscriptionPricePointOption configures a subscription price point detail read.
type SubscriptionPricePointOption func(*subscriptionPricePointDetailQuery)

type subscriptionRelatedFieldsQuery struct {
	parentFields []string
	include      []string
}

type subscriptionPricePointDetailQuery struct {
	fields []string
}

type subscriptionLocalizationsQuery struct {
	listQuery
	fields             []string
	subscriptionFields []string
	include            []string
}

type subscriptionImagesQuery struct {
	listQuery
	subscriptionFields []string
	include            []string
}

type subscriptionIntroductoryOffersQuery struct {
	listQuery
	fields             []string
	subscriptionFields []string
	pricePointFields   []string
	include            []string
}

type subscriptionPromotionalOffersQuery struct {
	listQuery
	subscriptionFields []string
	include            []string
}

type subscriptionPromotionalOfferPricesQuery struct {
	listQuery
	pricePointFields []string
	include          []string
}

type subscriptionOfferCodesQuery struct {
	listQuery
	subscriptionFields []string
	include            []string
}

type subscriptionOfferCodeCustomCodesQuery struct {
	listQuery
}

type subscriptionOfferCodePricesQuery struct {
	listQuery
	pricePointFields []string
	include          []string
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
	fields      []string
	groupFields []string
	include     []string
}

type subscriptionAppStoreReviewScreenshotQuery struct {
	fields             []string
	subscriptionFields []string
	include            []string
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

// WithSubscriptionLocalizationsSubscriptionFields sets fields[subscriptions] for included subscriptions.
func WithSubscriptionLocalizationsSubscriptionFields(fields []string) SubscriptionLocalizationsOption {
	return func(q *subscriptionLocalizationsQuery) {
		q.subscriptionFields = normalizeList(fields)
	}
}

// WithSubscriptionLocalizationsInclude sets relationships to include.
func WithSubscriptionLocalizationsInclude(include []string) SubscriptionLocalizationsOption {
	return func(q *subscriptionLocalizationsQuery) {
		q.include = normalizeList(include)
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

// WithSubscriptionImagesSubscriptionFields sets fields[subscriptions] for included subscriptions.
func WithSubscriptionImagesSubscriptionFields(fields []string) SubscriptionImagesOption {
	return func(q *subscriptionImagesQuery) {
		q.subscriptionFields = normalizeList(fields)
	}
}

// WithSubscriptionImagesInclude sets relationships to include.
func WithSubscriptionImagesInclude(include []string) SubscriptionImagesOption {
	return func(q *subscriptionImagesQuery) {
		q.include = normalizeList(include)
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

// WithSubscriptionPromotionalOffersSubscriptionFields sets fields[subscriptions] for included subscriptions.
func WithSubscriptionPromotionalOffersSubscriptionFields(fields []string) SubscriptionPromotionalOffersOption {
	return func(q *subscriptionPromotionalOffersQuery) {
		q.subscriptionFields = normalizeList(fields)
	}
}

// WithSubscriptionPromotionalOffersInclude sets relationships to include.
func WithSubscriptionPromotionalOffersInclude(include []string) SubscriptionPromotionalOffersOption {
	return func(q *subscriptionPromotionalOffersQuery) {
		q.include = normalizeList(include)
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

// WithSubscriptionPromotionalOfferPricesPricePointFields sets fields[subscriptionPricePoints] for included price points.
func WithSubscriptionPromotionalOfferPricesPricePointFields(fields []string) SubscriptionPromotionalOfferPricesOption {
	return func(q *subscriptionPromotionalOfferPricesQuery) {
		q.pricePointFields = normalizeList(fields)
	}
}

// WithSubscriptionPromotionalOfferPricesInclude sets relationships to include.
func WithSubscriptionPromotionalOfferPricesInclude(include []string) SubscriptionPromotionalOfferPricesOption {
	return func(q *subscriptionPromotionalOfferPricesQuery) {
		q.include = normalizeList(include)
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

// WithSubscriptionOfferCodesSubscriptionFields sets fields[subscriptions] for included subscriptions.
func WithSubscriptionOfferCodesSubscriptionFields(fields []string) SubscriptionOfferCodesOption {
	return func(q *subscriptionOfferCodesQuery) {
		q.subscriptionFields = normalizeList(fields)
	}
}

// WithSubscriptionOfferCodesInclude sets relationships to include.
func WithSubscriptionOfferCodesInclude(include []string) SubscriptionOfferCodesOption {
	return func(q *subscriptionOfferCodesQuery) {
		q.include = normalizeList(include)
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

// WithSubscriptionOfferCodePricesPricePointFields sets fields[subscriptionPricePoints] for included price points.
func WithSubscriptionOfferCodePricesPricePointFields(fields []string) SubscriptionOfferCodePricesOption {
	return func(q *subscriptionOfferCodePricesQuery) {
		q.pricePointFields = normalizeList(fields)
	}
}

// WithSubscriptionOfferCodePricesInclude sets relationships to include.
func WithSubscriptionOfferCodePricesInclude(include []string) SubscriptionOfferCodePricesOption {
	return func(q *subscriptionOfferCodePricesQuery) {
		q.include = normalizeList(include)
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

// WithSubscriptionGroupLocalizationsGroupFields sets fields[subscriptionGroups] for included groups.
func WithSubscriptionGroupLocalizationsGroupFields(fields []string) SubscriptionGroupLocalizationsOption {
	return func(q *subscriptionGroupLocalizationsQuery) {
		q.groupFields = normalizeList(fields)
	}
}

// WithSubscriptionGroupLocalizationsInclude sets relationships to include.
func WithSubscriptionGroupLocalizationsInclude(include []string) SubscriptionGroupLocalizationsOption {
	return func(q *subscriptionGroupLocalizationsQuery) {
		q.include = normalizeList(include)
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

// WithSubscriptionGroupLocalizationGroupFields sets fields[subscriptionGroups] on a group localization detail read.
func WithSubscriptionGroupLocalizationGroupFields(fields []string) SubscriptionGroupLocalizationOption {
	return func(q *subscriptionRelatedFieldsQuery) {
		q.parentFields = normalizeList(fields)
	}
}

// WithSubscriptionGroupLocalizationInclude sets relationships to include on a group localization detail read.
func WithSubscriptionGroupLocalizationInclude(include []string) SubscriptionGroupLocalizationOption {
	return func(q *subscriptionRelatedFieldsQuery) {
		q.include = normalizeList(include)
	}
}

// WithSubscriptionImageSubscriptionFields sets fields[subscriptions] on a subscription image detail read.
func WithSubscriptionImageSubscriptionFields(fields []string) SubscriptionImageOption {
	return func(q *subscriptionRelatedFieldsQuery) {
		q.parentFields = normalizeList(fields)
	}
}

// WithSubscriptionImageInclude sets relationships to include on a subscription image detail read.
func WithSubscriptionImageInclude(include []string) SubscriptionImageOption {
	return func(q *subscriptionRelatedFieldsQuery) {
		q.include = normalizeList(include)
	}
}

// WithSubscriptionLocalizationSubscriptionFields sets fields[subscriptions] on a subscription localization detail read.
func WithSubscriptionLocalizationSubscriptionFields(fields []string) SubscriptionLocalizationOption {
	return func(q *subscriptionRelatedFieldsQuery) {
		q.parentFields = normalizeList(fields)
	}
}

// WithSubscriptionLocalizationInclude sets relationships to include on a subscription localization detail read.
func WithSubscriptionLocalizationInclude(include []string) SubscriptionLocalizationOption {
	return func(q *subscriptionRelatedFieldsQuery) {
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

// WithSubscriptionPricePointFields sets fields[subscriptionPricePoints] on a price point detail read.
func WithSubscriptionPricePointFields(fields []string) SubscriptionPricePointOption {
	return func(q *subscriptionPricePointDetailQuery) {
		q.fields = normalizeList(fields)
	}
}

func buildSubscriptionLocalizationsQuery(query *subscriptionLocalizationsQuery) string {
	values := url.Values{}
	addCSV(values, "fields[subscriptionLocalizations]", query.fields)
	addCSV(values, "fields[subscriptions]", query.subscriptionFields)
	addCSV(values, "include", query.include)
	addLimit(values, query.limit)
	return values.Encode()
}

func buildSubscriptionImagesQuery(query *subscriptionImagesQuery) string {
	values := url.Values{}
	addCSV(values, "fields[subscriptions]", query.subscriptionFields)
	addCSV(values, "include", query.include)
	addLimit(values, query.limit)
	return values.Encode()
}

func buildSubscriptionIntroductoryOffersQuery(query *subscriptionIntroductoryOffersQuery) string {
	values := url.Values{}
	addCSV(values, "fields[subscriptionIntroductoryOffers]", query.fields)
	addCSV(values, "fields[subscriptions]", query.subscriptionFields)
	addCSV(values, "fields[subscriptionPricePoints]", query.pricePointFields)
	addCSV(values, "include", query.include)
	addLimit(values, query.limit)
	return values.Encode()
}

func buildSubscriptionPromotionalOffersQuery(query *subscriptionPromotionalOffersQuery) string {
	values := url.Values{}
	addCSV(values, "fields[subscriptions]", query.subscriptionFields)
	addCSV(values, "include", query.include)
	addLimit(values, query.limit)
	return values.Encode()
}

func buildSubscriptionPromotionalOfferPricesQuery(query *subscriptionPromotionalOfferPricesQuery) string {
	values := url.Values{}
	addCSV(values, "fields[subscriptionPricePoints]", query.pricePointFields)
	addCSV(values, "include", query.include)
	addLimit(values, query.limit)
	return values.Encode()
}

func buildSubscriptionOfferCodesQuery(query *subscriptionOfferCodesQuery) string {
	values := url.Values{}
	addCSV(values, "fields[subscriptions]", query.subscriptionFields)
	addCSV(values, "include", query.include)
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
	addCSV(values, "fields[subscriptionGroups]", query.groupFields)
	addCSV(values, "include", query.include)
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

func buildSubscriptionPricePointDetailQuery(query *subscriptionPricePointDetailQuery) string {
	values := url.Values{}
	addCSV(values, "fields[subscriptionPricePoints]", query.fields)
	return values.Encode()
}

func subscriptionLocalizationsQueryHasModifiers(query *subscriptionLocalizationsQuery) bool {
	return query.limit != 0 || len(query.fields) != 0 || len(query.subscriptionFields) != 0 || len(query.include) != 0
}

func subscriptionImagesQueryHasModifiers(query *subscriptionImagesQuery) bool {
	return query.limit != 0 || len(query.subscriptionFields) != 0 || len(query.include) != 0
}

func subscriptionIntroductoryOffersQueryHasModifiers(query *subscriptionIntroductoryOffersQuery) bool {
	return query.limit != 0 || len(query.fields) != 0 || len(query.subscriptionFields) != 0 ||
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
	return query.limit != 0 || len(query.fields) != 0 || len(query.groupFields) != 0 || len(query.include) != 0
}
