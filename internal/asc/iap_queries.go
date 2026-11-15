package asc

import (
	"net/url"
	"strings"
)

type IAPPricePointsOption func(*iapPricePointsQuery)

type iapPricePointsQuery struct {
	listQuery
	includedFieldsQuery
	territory       string
	fields          []string
	territoryFields []string
	include         []string
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

func buildIAPPricePointsQuery(query *iapPricePointsQuery) string {
	values := url.Values{}
	if strings.TrimSpace(query.territory) != "" {
		values.Set("filter[territory]", strings.TrimSpace(query.territory))
	}
	addCSV(values, "fields[inAppPurchasePricePoints]", query.fields)
	addCSV(values, "fields[territories]", query.territoryFields)
	addCSV(values, "include", query.include)
	addLimit(values, query.limit)
	query.includedFields.AddTo(values)
	return values.Encode()
}
