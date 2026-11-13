package cmdtest

import "testing"

func TestIAPListsRejectsInvalidNextURL(t *testing.T) {
	runInvalidNextURLCases(t, []invalidNextURLCase{
		{args: []string{"iap", "offer-codes", "custom-codes", "list"}, wantErr: "iap offer-codes custom-codes list: --next"},
		{args: []string{"iap", "offer-codes", "one-time-codes", "list"}, wantErr: "iap offer-codes one-time-codes list: --next"},
		{args: []string{"iap", "pricing", "price-points", "list"}, wantErr: "iap price-points list: --next"},
		{args: []string{"iap", "pricing", "schedules", "automatic-prices"}, wantErr: "iap pricing schedules automatic-prices: --next"},
		{args: []string{"iap", "pricing", "schedules", "manual-prices"}, wantErr: "iap pricing schedules manual-prices: --next"},
		{args: []string{"iap", "pricing", "availabilities", "available-territories"}, wantErr: "iap availabilities available-territories: --next"},
	})
}

func TestIAPListsPaginateFromNext(t *testing.T) {
	runPaginateFromNextCases(t, []paginateFromNextCase{
		{
			args:     []string{"iap", "list"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/apps/app-1/inAppPurchasesV2?cursor=AQ&limit=200",
			dataType: "inAppPurchases",
		},
		{
			args:     []string{"iap", "offer-codes", "custom-codes", "list"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/inAppPurchaseOfferCodes/offer-1/customCodes?cursor=AQ&limit=200",
			dataType: "inAppPurchaseOfferCodeCustomCodes",
		},
		{
			args:     []string{"iap", "offer-codes", "one-time-codes", "list"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/inAppPurchaseOfferCodes/offer-1/oneTimeUseCodes?cursor=AQ&limit=200",
			dataType: "inAppPurchaseOfferCodeOneTimeUseCodes",
		},
		{
			args:     []string{"iap", "offer-codes", "prices"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/inAppPurchaseOfferCodes/offer-1/prices?cursor=AQ&fields%5BinAppPurchaseOfferPrices%5D=territory%2CpricePoint&include=territory%2CpricePoint&limit=200",
			dataType: "inAppPurchaseOfferPrices",
		},
		{
			args:     []string{"iap", "pricing", "price-points", "list"},
			firstURL: "https://api.appstoreconnect.apple.com/v2/inAppPurchases/iap-1/pricePoints?cursor=AQ&limit=200",
			dataType: "inAppPurchasePricePoints",
		},
		{
			args:     []string{"iap", "pricing", "schedules", "automatic-prices"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/inAppPurchasePriceSchedules/schedule-1/automaticPrices?cursor=AQ&limit=200",
			dataType: "inAppPurchasePrices",
		},
		{
			args:     []string{"iap", "pricing", "schedules", "manual-prices"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/inAppPurchasePriceSchedules/schedule-1/manualPrices?cursor=AQ&limit=200",
			dataType: "inAppPurchasePrices",
		},
		{
			args:     []string{"iap", "pricing", "availabilities", "available-territories"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/inAppPurchaseAvailabilities/availability-1/availableTerritories?cursor=AQ&limit=200",
			dataType: "territories",
		},
	})
}
