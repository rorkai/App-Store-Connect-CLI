package asc

import (
	"net/url"
	"strings"
)

// AppPriceSchedulePricesOption configures app price schedule manual/automatic
// price list endpoints.
type AppPriceSchedulePricesOption func(*appPriceSchedulePricesQuery)

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

func WithAppPriceSchedulePricesLimit(limit int) AppPriceSchedulePricesOption {
	return func(q *appPriceSchedulePricesQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

func WithAppPriceSchedulePricesNextURL(next string) AppPriceSchedulePricesOption {
	return func(q *appPriceSchedulePricesQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

func WithAppPriceSchedulePricesStartDate(startDate string) AppPriceSchedulePricesOption {
	return func(q *appPriceSchedulePricesQuery) {
		if strings.TrimSpace(startDate) != "" {
			q.startDate = strings.TrimSpace(startDate)
		}
	}
}

func WithAppPriceSchedulePricesEndDate(endDate string) AppPriceSchedulePricesOption {
	return func(q *appPriceSchedulePricesQuery) {
		if strings.TrimSpace(endDate) != "" {
			q.endDate = strings.TrimSpace(endDate)
		}
	}
}

func WithAppPriceSchedulePricesTerritory(territory string) AppPriceSchedulePricesOption {
	return func(q *appPriceSchedulePricesQuery) {
		if strings.TrimSpace(territory) != "" {
			q.territory = strings.ToUpper(strings.TrimSpace(territory))
		}
	}
}

func WithAppPriceSchedulePricesInclude(include []string) AppPriceSchedulePricesOption {
	return func(q *appPriceSchedulePricesQuery) {
		q.include = normalizeList(include)
	}
}

func WithAppPriceSchedulePricesFields(fields []string) AppPriceSchedulePricesOption {
	return func(q *appPriceSchedulePricesQuery) {
		q.priceFields = normalizeList(fields)
	}
}

func WithAppPriceSchedulePricesPricePointFields(fields []string) AppPriceSchedulePricesOption {
	return func(q *appPriceSchedulePricesQuery) {
		q.pricePointFields = normalizeList(fields)
	}
}

func WithAppPriceSchedulePricesTerritoryFields(fields []string) AppPriceSchedulePricesOption {
	return func(q *appPriceSchedulePricesQuery) {
		q.territoryFields = normalizeList(fields)
	}
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
