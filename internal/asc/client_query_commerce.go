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
}

type territoryAvailabilitiesQuery struct {
	listQuery
}

type linkagesQuery struct {
	listQuery
}

type pricePointsQuery struct {
	listQuery
	territory string
}

type appPriceSchedulePricesQuery struct {
	listQuery
	startDate        string
	endDate          string
	territory        string
	include          []string
	priceFields      []string
	pricePointFields []string
	territoryFields  []string
}

func buildPromotedPurchasesQuery(query *promotedPurchasesQuery) string {
	values := url.Values{}
	addLimit(values, query.limit)
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
	addLimit(values, query.limit)
	return values.Encode()
}

func buildAppPriceSchedulePricesQuery(query *appPriceSchedulePricesQuery) string {
	values := url.Values{}
	if strings.TrimSpace(query.startDate) != "" {
		values.Set("filter[startDate]", strings.TrimSpace(query.startDate))
	}
	if strings.TrimSpace(query.endDate) != "" {
		values.Set("filter[endDate]", strings.TrimSpace(query.endDate))
	}
	if strings.TrimSpace(query.territory) != "" {
		values.Set("filter[territory]", strings.TrimSpace(query.territory))
	}
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

// WithSubscriptionOfferCodeOneTimeUseCodesLimit sets the max number of offer code batches to return.
func WithSubscriptionOfferCodeOneTimeUseCodesLimit(limit int) SubscriptionOfferCodeOneTimeUseCodesOption {
	return func(q *subscriptionOfferCodeOneTimeUseCodesQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithSubscriptionOfferCodeOneTimeUseCodesNextURL uses a next page URL directly.
func WithSubscriptionOfferCodeOneTimeUseCodesNextURL(next string) SubscriptionOfferCodeOneTimeUseCodesOption {
	return func(q *subscriptionOfferCodeOneTimeUseCodesQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

// WithWinBackOffersLimit sets the max number of win-back offers to return.
func WithWinBackOffersLimit(limit int) WinBackOffersOption {
	return func(q *winBackOffersQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithWinBackOffersNextURL uses a next page URL directly.
func WithWinBackOffersNextURL(next string) WinBackOffersOption {
	return func(q *winBackOffersQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

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

// WithWinBackOfferPricesLimit sets the max number of win-back offer prices to return.
func WithWinBackOfferPricesLimit(limit int) WinBackOfferPricesOption {
	return func(q *winBackOfferPricesQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithWinBackOfferPricesNextURL uses a next page URL directly.
func WithWinBackOfferPricesNextURL(next string) WinBackOfferPricesOption {
	return func(q *winBackOfferPricesQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
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

// WithPromotedPurchasesLimit sets the max number of promoted purchases to return.
func WithPromotedPurchasesLimit(limit int) PromotedPurchasesOption {
	return func(q *promotedPurchasesQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithPromotedPurchasesNextURL uses a next page URL directly.
func WithPromotedPurchasesNextURL(next string) PromotedPurchasesOption {
	return func(q *promotedPurchasesQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

// WithTerritoryAvailabilitiesLimit sets the max number of territory availabilities to return.
func WithTerritoryAvailabilitiesLimit(limit int) TerritoryAvailabilitiesOption {
	return func(q *territoryAvailabilitiesQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithTerritoryAvailabilitiesNextURL uses a next page URL directly.
func WithTerritoryAvailabilitiesNextURL(next string) TerritoryAvailabilitiesOption {
	return func(q *territoryAvailabilitiesQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

// WithLinkagesLimit sets the max number of linkages to return.
func WithLinkagesLimit(limit int) LinkagesOption {
	return func(q *linkagesQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithLinkagesNextURL uses a next page URL directly.
func WithLinkagesNextURL(next string) LinkagesOption {
	return func(q *linkagesQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

// WithPricePointsLimit sets the max number of price points to return.
func WithPricePointsLimit(limit int) PricePointsOption {
	return func(q *pricePointsQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithPricePointsNextURL uses a next page URL directly.
func WithPricePointsNextURL(next string) PricePointsOption {
	return func(q *pricePointsQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

// WithPricePointsTerritory filters app price points by territory.
func WithPricePointsTerritory(territory string) PricePointsOption {
	return func(q *pricePointsQuery) {
		if strings.TrimSpace(territory) != "" {
			q.territory = strings.ToUpper(strings.TrimSpace(territory))
		}
	}
}

// WithAppPriceSchedulePricesLimit sets the max number of schedule prices to return.
func WithAppPriceSchedulePricesLimit(limit int) AppPriceSchedulePricesOption {
	return func(q *appPriceSchedulePricesQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithAppPriceSchedulePricesNextURL uses a next page URL directly.
func WithAppPriceSchedulePricesNextURL(next string) AppPriceSchedulePricesOption {
	return func(q *appPriceSchedulePricesQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

// WithAppPriceSchedulePricesStartDate filters schedule prices by inclusive start date.
func WithAppPriceSchedulePricesStartDate(startDate string) AppPriceSchedulePricesOption {
	return func(q *appPriceSchedulePricesQuery) {
		if strings.TrimSpace(startDate) != "" {
			q.startDate = strings.TrimSpace(startDate)
		}
	}
}

// WithAppPriceSchedulePricesEndDate filters schedule prices by inclusive end date.
func WithAppPriceSchedulePricesEndDate(endDate string) AppPriceSchedulePricesOption {
	return func(q *appPriceSchedulePricesQuery) {
		if strings.TrimSpace(endDate) != "" {
			q.endDate = strings.TrimSpace(endDate)
		}
	}
}

// WithAppPriceSchedulePricesTerritory filters schedule prices by territory code.
func WithAppPriceSchedulePricesTerritory(territory string) AppPriceSchedulePricesOption {
	return func(q *appPriceSchedulePricesQuery) {
		if strings.TrimSpace(territory) != "" {
			q.territory = strings.ToUpper(strings.TrimSpace(territory))
		}
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
