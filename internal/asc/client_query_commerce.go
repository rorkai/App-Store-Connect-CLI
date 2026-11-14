package asc

import (
	"net/url"
	"strconv"
	"strings"
)

type subscriptionOfferCodeOneTimeUseCodesQuery struct {
	listQuery
}

type winBackOffersQuery struct {
	listQuery
	fields      []string
	priceFields []string
	include     []string
	pricesLimit int
}

type winBackOfferPricesQuery struct {
	listQuery
	territoryIDs                 []string
	fields                       []string
	territoryFields              []string
	subscriptionPricePointFields []string
	include                      []string
}

type promotedPurchasesQuery struct {
	listQuery
	iapFields          []string
	subscriptionFields []string
}

type territoryAvailabilitiesQuery struct {
	listQuery
}

type linkagesQuery struct {
	listQuery
}

type pricePointsQuery struct {
	listQuery
	includedFieldsQuery
	territory string
	include   []string
}

type appPriceSchedulePricesQuery struct {
	listQuery
	include          []string
	priceFields      []string
	pricePointFields []string
	territoryFields  []string
}

func buildPromotedPurchasesQuery(query *promotedPurchasesQuery) string {
	values := url.Values{}
	addLimit(values, query.limit)
	addCSV(values, "fields[inAppPurchases]", query.iapFields)
	addCSV(values, "fields[subscriptions]", query.subscriptionFields)
	include := includeWhenFieldsSelected(nil, "inAppPurchaseV2", query.iapFields)
	include = includeWhenFieldsSelected(include, "subscription", query.subscriptionFields)
	addCSV(values, "include", include)
	return values.Encode()
}

func buildSubscriptionOfferCodeOneTimeUseCodesQuery(query *subscriptionOfferCodeOneTimeUseCodesQuery) string {
	values := url.Values{}
	addLimit(values, query.limit)
	return values.Encode()
}

func buildWinBackOffersQuery(query *winBackOffersQuery) string {
	values := url.Values{}
	addCSV(values, "fields[winBackOffers]", query.fields)
	addCSV(values, "fields[winBackOfferPrices]", query.priceFields)
	addCSV(values, "include", query.include)
	addLimit(values, query.limit)
	if query.pricesLimit > 0 {
		values.Set("limit[prices]", strconv.Itoa(query.pricesLimit))
	}
	return values.Encode()
}

func buildWinBackOfferPricesQuery(query *winBackOfferPricesQuery) string {
	values := url.Values{}
	addCSV(values, "filter[territory]", query.territoryIDs)
	addCSV(values, "fields[winBackOfferPrices]", query.fields)
	addCSV(values, "fields[territories]", query.territoryFields)
	addCSV(values, "fields[subscriptionPricePoints]", query.subscriptionPricePointFields)
	addCSV(values, "include", query.include)
	addLimit(values, query.limit)
	return values.Encode()
}

func buildLinkagesQuery(query *linkagesQuery) string {
	values := url.Values{}
	addLimit(values, query.limit)
	return values.Encode()
}

func buildPricePointsQuery(query *pricePointsQuery) string {
	values := url.Values{}
	if strings.TrimSpace(query.territory) != "" {
		values.Set("filter[territory]", strings.TrimSpace(query.territory))
	}
	addCSV(values, "include", query.include)
	addLimit(values, query.limit)
	query.includedFields.AddTo(values)
	return values.Encode()
}

func buildAppPriceSchedulePricesQuery(query *appPriceSchedulePricesQuery) string {
	values := url.Values{}
	addCSV(values, "include", query.include)
	addCSV(values, "fields[appPrices]", query.priceFields)
	addCSV(values, "fields[appPricePoints]", query.pricePointFields)
	addCSV(values, "fields[territories]", query.territoryFields)
	addLimit(values, query.limit)
	return values.Encode()
}

// SubscriptionOfferCodeOneTimeUseCodesOption is a functional option for GetSubscriptionOfferCodeOneTimeUseCodes.
type SubscriptionOfferCodeOneTimeUseCodesOption func(*subscriptionOfferCodeOneTimeUseCodesQuery)

// WinBackOffersOption is a functional option for win-back offer list endpoints.
type WinBackOffersOption func(*winBackOffersQuery)

// WinBackOfferPricesOption is a functional option for win-back offer prices list endpoints.
type WinBackOfferPricesOption func(*winBackOfferPricesQuery)

// PromotedPurchasesOption is a functional option for promoted purchases endpoints.
type PromotedPurchasesOption func(*promotedPurchasesQuery)

// TerritoryAvailabilitiesOption is a functional option for GetTerritoryAvailabilities.
type TerritoryAvailabilitiesOption func(*territoryAvailabilitiesQuery)

// LinkagesOption is a functional option for linkages endpoints.
type LinkagesOption func(*linkagesQuery)

// PricePointsOption is a functional option for GetAppPricePoints.
type PricePointsOption func(*pricePointsQuery)

// AppPriceSchedulePricesOption is a functional option for app price schedule price endpoints.
type AppPriceSchedulePricesOption func(*appPriceSchedulePricesQuery)

// WithWinBackOffersFields sets fields[winBackOffers] for win-back offer responses.
func WithWinBackOffersFields(fields []string) WinBackOffersOption {
	return func(q *winBackOffersQuery) {
		q.fields = normalizeList(fields)
	}
}

// WithWinBackOffersPriceFields sets fields[winBackOfferPrices] for included prices.
func WithWinBackOffersPriceFields(fields []string) WinBackOffersOption {
	return func(q *winBackOffersQuery) {
		q.priceFields = normalizeList(fields)
	}
}

// WithWinBackOffersInclude sets include for win-back offer responses.
func WithWinBackOffersInclude(include []string) WinBackOffersOption {
	return func(q *winBackOffersQuery) {
		q.include = normalizeList(include)
	}
}

// WithWinBackOffersPricesLimit sets limit[prices] for included prices.
func WithWinBackOffersPricesLimit(limit int) WinBackOffersOption {
	return func(q *winBackOffersQuery) {
		if limit > 0 {
			q.pricesLimit = limit
		}
	}
}

// WithWinBackOfferPricesTerritoryFilter filters win-back offer prices by territory ID(s).
func WithWinBackOfferPricesTerritoryFilter(ids []string) WinBackOfferPricesOption {
	return func(q *winBackOfferPricesQuery) {
		q.territoryIDs = normalizeList(ids)
	}
}

// WithWinBackOfferPricesFields sets fields[winBackOfferPrices] for price responses.
func WithWinBackOfferPricesFields(fields []string) WinBackOfferPricesOption {
	return func(q *winBackOfferPricesQuery) {
		q.fields = normalizeList(fields)
	}
}

// WithWinBackOfferPricesTerritoryFields sets fields[territories] for included territories.
func WithWinBackOfferPricesTerritoryFields(fields []string) WinBackOfferPricesOption {
	return func(q *winBackOfferPricesQuery) {
		q.territoryFields = normalizeList(fields)
	}
}

// WithWinBackOfferPricesSubscriptionPricePointFields sets fields[subscriptionPricePoints] for included price points.
func WithWinBackOfferPricesSubscriptionPricePointFields(fields []string) WinBackOfferPricesOption {
	return func(q *winBackOfferPricesQuery) {
		q.subscriptionPricePointFields = normalizeList(fields)
	}
}

// WithWinBackOfferPricesInclude sets include for win-back offer price responses.
func WithWinBackOfferPricesInclude(include []string) WinBackOfferPricesOption {
	return func(q *winBackOfferPricesQuery) {
		q.include = normalizeList(include)
	}
}

// WithPromotedPurchasesIAPFields sets fields[inAppPurchases] for included IAPs.
func WithPromotedPurchasesIAPFields(fields []string) PromotedPurchasesOption {
	return func(q *promotedPurchasesQuery) { q.iapFields = normalizeUniqueList(fields) }
}

// WithPromotedPurchasesSubscriptionFields sets fields[subscriptions] for included subscriptions.
func WithPromotedPurchasesSubscriptionFields(fields []string) PromotedPurchasesOption {
	return func(q *promotedPurchasesQuery) { q.subscriptionFields = normalizeUniqueList(fields) }
}

// WithPricePointsTerritory filters app price points by territory.
func WithPricePointsTerritory(territory string) PricePointsOption {
	return func(q *pricePointsQuery) {
		if strings.TrimSpace(territory) != "" {
			q.territory = strings.ToUpper(strings.TrimSpace(territory))
		}
	}
}

// WithPricePointsInclude sets app price point relationships to include.
func WithPricePointsInclude(include []string) PricePointsOption {
	return func(q *pricePointsQuery) {
		q.include = normalizeList(include)
	}
}

// WithAppPriceSchedulePricesInclude sets include values for app schedule price responses.
func WithAppPriceSchedulePricesInclude(include []string) AppPriceSchedulePricesOption {
	return func(q *appPriceSchedulePricesQuery) {
		q.include = normalizeList(include)
	}
}

// WithAppPriceSchedulePricesFields sets fields[appPrices] for app schedule price responses.
func WithAppPriceSchedulePricesFields(fields []string) AppPriceSchedulePricesOption {
	return func(q *appPriceSchedulePricesQuery) {
		q.priceFields = normalizeList(fields)
	}
}

// WithAppPriceSchedulePricesPricePointFields sets fields[appPricePoints] for included app price points.
func WithAppPriceSchedulePricesPricePointFields(fields []string) AppPriceSchedulePricesOption {
	return func(q *appPriceSchedulePricesQuery) {
		q.pricePointFields = normalizeList(fields)
	}
}

// WithAppPriceSchedulePricesTerritoryFields sets fields[territories] for included territories.
func WithAppPriceSchedulePricesTerritoryFields(fields []string) AppPriceSchedulePricesOption {
	return func(q *appPriceSchedulePricesQuery) {
		q.territoryFields = normalizeList(fields)
	}
}
