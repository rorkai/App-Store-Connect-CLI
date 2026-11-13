package cmdtest

import "testing"

func TestSubscriptionsListsRejectsInvalidNextURL(t *testing.T) {
	runInvalidNextURLCases(t, []invalidNextURLCase{
		{args: []string{"subscriptions", "pricing", "price-points", "list"}, wantErr: "subscriptions pricing price-points list: --next"},
		{args: []string{"subscriptions", "pricing", "availability", "available-territories"}, wantErr: "subscriptions pricing availability available-territories: --next"},
		{args: []string{"subscriptions", "groups", "list"}, wantErr: "subscriptions groups list: --next"},
		{args: []string{"subscriptions", "offers", "introductory", "list"}, wantErr: "subscriptions offers introductory list: --next"},
		{args: []string{"subscriptions", "list"}, wantErr: "subscriptions list: --next"},
		{args: []string{"subscriptions", "offers", "offer-codes", "custom-codes", "list"}, wantErr: "subscriptions offers offer-codes custom-codes list: --next"},
		{args: []string{"subscriptions", "offers", "offer-codes", "one-time-codes", "list"}, wantErr: "subscriptions offers offer-codes one-time-codes list: --next"},
		{args: []string{"subscriptions", "offers", "offer-codes", "prices"}, wantErr: "subscriptions offers offer-codes prices: --next"},
		{args: []string{"subscriptions", "pricing", "prices", "list"}, wantErr: "subscriptions pricing prices list: --next"},
		{args: []string{"subscriptions", "offers", "promotional", "list"}, wantErr: "subscriptions offers promotional list: --next"},
		{args: []string{"subscriptions", "offers", "promotional", "prices"}, wantErr: "subscriptions offers promotional prices: --next"},
	})
}

func TestSubscriptionsListsPaginateFromNext(t *testing.T) {
	runPaginateFromNextCases(t, []paginateFromNextCase{
		{
			args:     []string{"subscriptions", "pricing", "availability", "available-territories"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/subscriptionAvailabilities/avail-1/availableTerritories?cursor=AQ&limit=200",
			dataType: "territories",
		},
		{
			args:     []string{"subscriptions", "offers", "introductory", "list"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/subscriptions/sub-1/introductoryOffers?cursor=AQ&limit=200",
			dataType: "subscriptionIntroductoryOffers",
		},
		{
			args:     []string{"subscriptions", "offers", "offer-codes", "custom-codes", "list"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/subscriptionOfferCodes/offer-1/customCodes?cursor=AQ&limit=200",
			dataType: "subscriptionOfferCodeCustomCodes",
		},
		{
			args:     []string{"subscriptions", "offers", "offer-codes", "one-time-codes", "list"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/subscriptionOfferCodes/offer-1/oneTimeUseCodes?cursor=AQ&limit=200",
			dataType: "subscriptionOfferCodeOneTimeUseCodes",
		},
		{
			args:     []string{"subscriptions", "offers", "offer-codes", "prices"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/subscriptionOfferCodes/offer-1/prices?cursor=AQ&limit=200",
			dataType: "subscriptionOfferCodePrices",
		},
		{
			args:     []string{"subscriptions", "pricing", "prices", "list"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/subscriptions/sub-1/prices?cursor=AQ&limit=200",
			dataType: "subscriptionPrices",
		},
		{
			args:     []string{"subscriptions", "offers", "promotional", "list"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/subscriptions/sub-1/promotionalOffers?cursor=AQ&limit=200",
			dataType: "subscriptionPromotionalOffers",
		},
		{
			args:     []string{"subscriptions", "offers", "promotional", "prices"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/subscriptionPromotionalOffers/offer-1/prices?cursor=AQ&limit=200",
			dataType: "subscriptionPromotionalOfferPrices",
		},
		{
			args:     []string{"subscriptions", "groups", "list"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/apps/app-1/subscriptionGroups?cursor=AQ&limit=200",
			dataType: "subscriptionGroups",
			env:      map[string]string{"ASC_APP_ID": "ambient-app"},
		},
		{
			args:     []string{"subscriptions", "list"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/subscriptionGroups/group-1/subscriptions?cursor=AQ&limit=200",
			dataType: "subscriptions",
			env:      map[string]string{"ASC_APP_ID": "ambient-app"},
		},
	})
}
