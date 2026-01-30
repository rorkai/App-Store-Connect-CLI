package asc

import (
	"net/url"
	"strings"
)

type IAPImagesOption func(*iapImagesQuery)
type IAPOfferCodesOption func(*iapOfferCodesQuery)
type IAPPricePointsOption func(*iapPricePointsQuery)
type IAPOfferCodeCustomCodesOption func(*iapOfferCodeCustomCodesQuery)
type IAPOfferCodeOneTimeUseCodesOption func(*iapOfferCodeOneTimeUseCodesQuery)
type IAPOfferCodePricesOption func(*iapOfferCodePricesQuery)
type IAPAvailabilityTerritoriesOption func(*iapAvailabilityTerritoriesQuery)
type IAPPriceSchedulePricesOption func(*iapPriceSchedulePricesQuery)

type iapImagesQuery struct {
	listQuery
}

type iapOfferCodesQuery struct {
	listQuery
}

type iapPricePointsQuery struct {
	listQuery
}

type iapOfferCodeCustomCodesQuery struct {
	listQuery
}

type iapOfferCodeOneTimeUseCodesQuery struct {
	listQuery
}

type iapOfferCodePricesQuery struct {
	listQuery
}

type iapAvailabilityTerritoriesQuery struct {
	listQuery
}

type iapPriceSchedulePricesQuery struct {
	listQuery
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

func buildIAPImagesQuery(query *iapImagesQuery) string {
	values := url.Values{}
	addLimit(values, query.limit)
	return values.Encode()
}

func buildIAPOfferCodesQuery(query *iapOfferCodesQuery) string {
	values := url.Values{}
	addLimit(values, query.limit)
	return values.Encode()
}

func buildIAPPricePointsQuery(query *iapPricePointsQuery) string {
	values := url.Values{}
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
	addLimit(values, query.limit)
	return values.Encode()
}
