package asc

import (
	"net/url"
	"strings"
)

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

// PromotedPurchasesOption is a functional option for promoted purchases endpoints.
type PromotedPurchasesOption func(*promotedPurchasesQuery)

// TerritoryAvailabilitiesOption is a functional option for GetTerritoryAvailabilities.
type TerritoryAvailabilitiesOption func(*territoryAvailabilitiesQuery)

// LinkagesOption is a functional option for linkages endpoints.
type LinkagesOption func(*linkagesQuery)

// PricePointsOption is a functional option for GetAppPricePoints.
type PricePointsOption func(*pricePointsQuery)

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
