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

// SubscriptionPricePointsOption is a functional option for subscription price point list endpoints.
type SubscriptionPricePointsOption func(*subscriptionPricePointsQuery)

// SubscriptionGroupLocalizationsOption is a functional option for subscription group localization list endpoints.
type SubscriptionGroupLocalizationsOption func(*subscriptionGroupLocalizationsQuery)

type subscriptionLocalizationsQuery struct {
	listQuery
}

type subscriptionImagesQuery struct {
	listQuery
}

type subscriptionIntroductoryOffersQuery struct {
	listQuery
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
}

type subscriptionGroupLocalizationsQuery struct {
	listQuery
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

func buildSubscriptionLocalizationsQuery(query *subscriptionLocalizationsQuery) string {
	values := url.Values{}
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
	addLimit(values, query.limit)
	return values.Encode()
}

func buildSubscriptionGroupLocalizationsQuery(query *subscriptionGroupLocalizationsQuery) string {
	values := url.Values{}
	addLimit(values, query.limit)
	return values.Encode()
}
