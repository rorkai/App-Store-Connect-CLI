package asc

import (
	"net/url"
	"strconv"
	"strings"
)

type (
	IAPImagesOption                   func(*iapImagesQuery)
	IAPOfferCodesOption               func(*iapOfferCodesQuery)
	IAPPricePointsOption              func(*iapPricePointsQuery)
	IAPOfferCodeCustomCodesOption     func(*iapOfferCodeCustomCodesQuery)
	IAPOfferCodeOneTimeUseCodesOption func(*iapOfferCodeOneTimeUseCodesQuery)
	IAPOfferCodePricesOption          func(*iapOfferCodePricesQuery)
	IAPAvailabilityTerritoriesOption  func(*iapAvailabilityTerritoriesQuery)
	IAPPriceSchedulePricesOption      func(*iapPriceSchedulePricesQuery)
	IAPPriceScheduleOption            func(*iapPriceScheduleQuery)
)

type iapImagesQuery struct {
	listQuery
	iapFields []string
	include   []string
}

type iapOfferCodesQuery struct {
	listQuery
}

type iapPricePointsQuery struct {
	listQuery
	territory       string
	fields          []string
	territoryFields []string
	include         []string
}

type iapOfferCodeCustomCodesQuery struct {
	listQuery
}

type iapOfferCodeOneTimeUseCodesQuery struct {
	listQuery
}

type iapOfferCodePricesQuery struct {
	listQuery
	fields  []string
	include []string
}

type iapAvailabilityTerritoriesQuery struct {
	listQuery
}

type iapPriceSchedulePricesQuery struct {
	listQuery
	include          []string
	priceFields      []string
	pricePointFields []string
	territoryFields  []string
}

type iapPriceScheduleQuery struct {
	include              []string
	priceScheduleFields  []string
	territoryFields      []string
	inAppPriceFields     []string
	manualPricesLimit    int
	automaticPricesLimit int
}

func WithIAPImagesLimit(limit int) IAPImagesOption {
	return func(q *iapImagesQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

func WithIAPImagesNextURL(next string) IAPImagesOption {
	return func(q *iapImagesQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

// WithIAPImagesIAPFields sets fields[inAppPurchases] for included IAPs.
func WithIAPImagesIAPFields(fields []string) IAPImagesOption {
	return func(q *iapImagesQuery) { q.iapFields = normalizeUniqueList(fields) }
}

// WithIAPImagesInclude sets the exact image relationship include set.
func WithIAPImagesInclude(include []string) IAPImagesOption {
	return func(q *iapImagesQuery) { q.include = normalizeUniqueList(include) }
}

func WithIAPOfferCodesLimit(limit int) IAPOfferCodesOption {
	return func(q *iapOfferCodesQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

func WithIAPOfferCodesNextURL(next string) IAPOfferCodesOption {
	return func(q *iapOfferCodesQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

func WithIAPPricePointsLimit(limit int) IAPPricePointsOption {
	return func(q *iapPricePointsQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

func WithIAPPricePointsNextURL(next string) IAPPricePointsOption {
	return func(q *iapPricePointsQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

func WithIAPPricePointsTerritory(territory string) IAPPricePointsOption {
	return func(q *iapPricePointsQuery) {
		if strings.TrimSpace(territory) != "" {
			q.territory = strings.TrimSpace(territory)
		}
	}
}

func WithIAPPricePointsFields(fields []string) IAPPricePointsOption {
	return func(q *iapPricePointsQuery) {
		q.fields = normalizeList(fields)
	}
}

func WithIAPPricePointsTerritoryFields(fields []string) IAPPricePointsOption {
	return func(q *iapPricePointsQuery) {
		q.territoryFields = normalizeList(fields)
	}
}

func WithIAPPricePointsInclude(include []string) IAPPricePointsOption {
	return func(q *iapPricePointsQuery) {
		q.include = normalizeList(include)
	}
}

func WithIAPOfferCodeCustomCodesLimit(limit int) IAPOfferCodeCustomCodesOption {
	return func(q *iapOfferCodeCustomCodesQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

func WithIAPOfferCodeCustomCodesNextURL(next string) IAPOfferCodeCustomCodesOption {
	return func(q *iapOfferCodeCustomCodesQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

func WithIAPOfferCodeOneTimeUseCodesLimit(limit int) IAPOfferCodeOneTimeUseCodesOption {
	return func(q *iapOfferCodeOneTimeUseCodesQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

func WithIAPOfferCodeOneTimeUseCodesNextURL(next string) IAPOfferCodeOneTimeUseCodesOption {
	return func(q *iapOfferCodeOneTimeUseCodesQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

func WithIAPOfferCodePricesLimit(limit int) IAPOfferCodePricesOption {
	return func(q *iapOfferCodePricesQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

func WithIAPOfferCodePricesNextURL(next string) IAPOfferCodePricesOption {
	return func(q *iapOfferCodePricesQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

// WithIAPOfferCodePricesFields sets fields[inAppPurchaseOfferPrices].
func WithIAPOfferCodePricesFields(fields []string) IAPOfferCodePricesOption {
	return func(q *iapOfferCodePricesQuery) {
		q.fields = normalizeUniqueList(fields)
	}
}

// WithIAPOfferCodePricesInclude sets relationships to include.
func WithIAPOfferCodePricesInclude(include []string) IAPOfferCodePricesOption {
	return func(q *iapOfferCodePricesQuery) {
		q.include = normalizeUniqueList(include)
	}
}

func WithIAPAvailabilityTerritoriesLimit(limit int) IAPAvailabilityTerritoriesOption {
	return func(q *iapAvailabilityTerritoriesQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

func WithIAPAvailabilityTerritoriesNextURL(next string) IAPAvailabilityTerritoriesOption {
	return func(q *iapAvailabilityTerritoriesQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

func WithIAPPriceSchedulePricesLimit(limit int) IAPPriceSchedulePricesOption {
	return func(q *iapPriceSchedulePricesQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

func WithIAPPriceSchedulePricesNextURL(next string) IAPPriceSchedulePricesOption {
	return func(q *iapPriceSchedulePricesQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

func WithIAPPriceSchedulePricesInclude(include []string) IAPPriceSchedulePricesOption {
	return func(q *iapPriceSchedulePricesQuery) {
		q.include = normalizeList(include)
	}
}

func WithIAPPriceSchedulePricesFields(fields []string) IAPPriceSchedulePricesOption {
	return func(q *iapPriceSchedulePricesQuery) {
		q.priceFields = normalizeList(fields)
	}
}

func WithIAPPriceSchedulePricesPricePointFields(fields []string) IAPPriceSchedulePricesOption {
	return func(q *iapPriceSchedulePricesQuery) {
		q.pricePointFields = normalizeList(fields)
	}
}

func WithIAPPriceSchedulePricesTerritoryFields(fields []string) IAPPriceSchedulePricesOption {
	return func(q *iapPriceSchedulePricesQuery) {
		q.territoryFields = normalizeList(fields)
	}
}

func WithIAPPriceScheduleInclude(include []string) IAPPriceScheduleOption {
	return func(q *iapPriceScheduleQuery) {
		q.include = normalizeList(include)
	}
}

func WithIAPPriceScheduleFields(fields []string) IAPPriceScheduleOption {
	return func(q *iapPriceScheduleQuery) {
		q.priceScheduleFields = normalizeList(fields)
	}
}

func WithIAPPriceScheduleTerritoryFields(fields []string) IAPPriceScheduleOption {
	return func(q *iapPriceScheduleQuery) {
		q.territoryFields = normalizeList(fields)
	}
}

func WithIAPPriceSchedulePriceFields(fields []string) IAPPriceScheduleOption {
	return func(q *iapPriceScheduleQuery) {
		q.inAppPriceFields = normalizeList(fields)
	}
}

func WithIAPPriceScheduleManualPricesLimit(limit int) IAPPriceScheduleOption {
	return func(q *iapPriceScheduleQuery) {
		if limit > 0 {
			q.manualPricesLimit = limit
		}
	}
}

func WithIAPPriceScheduleAutomaticPricesLimit(limit int) IAPPriceScheduleOption {
	return func(q *iapPriceScheduleQuery) {
		if limit > 0 {
			q.automaticPricesLimit = limit
		}
	}
}

func buildIAPImagesQuery(query *iapImagesQuery) string {
	values := url.Values{}
	addLimit(values, query.limit)
	addCSV(values, "fields[inAppPurchases]", query.iapFields)
	addCSV(values, "include", includeWhenFieldsSelected(query.include, "inAppPurchase", query.iapFields))
	return values.Encode()
}

func buildIAPOfferCodesQuery(query *iapOfferCodesQuery) string {
	values := url.Values{}
	addLimit(values, query.limit)
	return values.Encode()
}

func buildIAPPricePointsQuery(query *iapPricePointsQuery) string {
	values := url.Values{}
	if strings.TrimSpace(query.territory) != "" {
		values.Set("filter[territory]", strings.TrimSpace(query.territory))
	}
	addCSV(values, "fields[inAppPurchasePricePoints]", query.fields)
	addCSV(values, "fields[territories]", query.territoryFields)
	addCSV(values, "include", query.include)
	addLimit(values, query.limit)
	return values.Encode()
}

func buildIAPOfferCodeCustomCodesQuery(query *iapOfferCodeCustomCodesQuery) string {
	values := url.Values{}
	addLimit(values, query.limit)
	return values.Encode()
}

func buildIAPOfferCodeOneTimeUseCodesQuery(query *iapOfferCodeOneTimeUseCodesQuery) string {
	values := url.Values{}
	addLimit(values, query.limit)
	return values.Encode()
}

func buildIAPOfferCodePricesQuery(query *iapOfferCodePricesQuery) string {
	values := url.Values{}
	addCSV(values, "fields[inAppPurchaseOfferPrices]", query.fields)
	addCSV(values, "include", query.include)
	addLimit(values, query.limit)
	return values.Encode()
}

func buildIAPAvailabilityTerritoriesQuery(query *iapAvailabilityTerritoriesQuery) string {
	values := url.Values{}
	addLimit(values, query.limit)
	return values.Encode()
}

func buildIAPPriceSchedulePricesQuery(query *iapPriceSchedulePricesQuery) string {
	values := url.Values{}
	addCSV(values, "include", query.include)
	addCSV(values, "fields[inAppPurchasePrices]", query.priceFields)
	addCSV(values, "fields[inAppPurchasePricePoints]", query.pricePointFields)
	addCSV(values, "fields[territories]", query.territoryFields)
	addLimit(values, query.limit)
	return values.Encode()
}

func buildIAPPriceScheduleQuery(query *iapPriceScheduleQuery) string {
	values := url.Values{}
	addCSV(values, "include", query.include)
	addCSV(values, "fields[inAppPurchasePriceSchedules]", query.priceScheduleFields)
	addCSV(values, "fields[territories]", query.territoryFields)
	addCSV(values, "fields[inAppPurchasePrices]", query.inAppPriceFields)
	if query.manualPricesLimit > 0 {
		values.Set("limit[manualPrices]", strconv.Itoa(query.manualPricesLimit))
	}
	if query.automaticPricesLimit > 0 {
		values.Set("limit[automaticPrices]", strconv.Itoa(query.automaticPricesLimit))
	}
	return values.Encode()
}
