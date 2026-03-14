package cmdtest

import "testing"

func TestSubscriptionsAvailabilityAvailableTerritoriesRejectsInvalidNextURLPhase62(t *testing.T) {
	runGameCenterAchievementsInvalidNextURLCases(
		t,
		[]string{"subscriptions", "pricing", "availability", "available-territories"},
		"subscriptions pricing availability available-territories: --next",
	)
}

func TestSubscriptionsAvailabilityAvailableTerritoriesPaginateFromNextWithoutIDPhase62(t *testing.T) {
	const firstURL = "https://api.appstoreconnect.apple.com/v1/subscriptionAvailabilities/avail-1/availableTerritories?cursor=AQ&limit=200"
	const secondURL = "https://api.appstoreconnect.apple.com/v1/subscriptionAvailabilities/avail-1/availableTerritories?cursor=BQ&limit=200"

	firstBody := `{"data":[{"type":"territories","id":"subscription-availability-territory-next-1"}],"links":{"next":"` + secondURL + `"}}`
	secondBody := `{"data":[{"type":"territories","id":"subscription-availability-territory-next-2"}],"links":{"next":""}}`

	runGameCenterAchievementsPaginateFromNext(
		t,
		[]string{"subscriptions", "pricing", "availability", "available-territories"},
		firstURL,
		secondURL,
		firstBody,
		secondBody,
		"subscription-availability-territory-next-1",
		"subscription-availability-territory-next-2",
	)
}

func TestSubscriptionsGroupsListRejectsInvalidNextURLPhase62(t *testing.T) {
	runGameCenterAchievementsInvalidNextURLCases(
		t,
		[]string{"subscriptions", "groups", "list"},
		"subscriptions groups list: --next",
	)
}

func TestSubscriptionsGroupsListPaginateFromNextWithoutAppPhase62(t *testing.T) {
	const firstURL = "https://api.appstoreconnect.apple.com/v1/apps/app-1/subscriptionGroups?cursor=AQ&limit=200"
	const secondURL = "https://api.appstoreconnect.apple.com/v1/apps/app-1/subscriptionGroups?cursor=BQ&limit=200"

	firstBody := `{"data":[{"type":"subscriptionGroups","id":"subscription-group-next-1"}],"links":{"next":"` + secondURL + `"}}`
	secondBody := `{"data":[{"type":"subscriptionGroups","id":"subscription-group-next-2"}],"links":{"next":""}}`

	runGameCenterAchievementsPaginateFromNext(
		t,
		[]string{"subscriptions", "groups", "list"},
		firstURL,
		secondURL,
		firstBody,
		secondBody,
		"subscription-group-next-1",
		"subscription-group-next-2",
	)
}

func TestSubscriptionsGroupsLocalizationsListRejectsInvalidNextURLPhase62(t *testing.T) {
	runGameCenterAchievementsInvalidNextURLCases(
		t,
		[]string{"subscriptions", "groups", "localizations", "list"},
		"subscriptions groups localizations list: --next",
	)
}

func TestSubscriptionsGroupsLocalizationsListPaginateFromNextWithoutGroupIDPhase62(t *testing.T) {
	const firstURL = "https://api.appstoreconnect.apple.com/v1/subscriptionGroups/group-1/subscriptionGroupLocalizations?cursor=AQ&limit=200"
	const secondURL = "https://api.appstoreconnect.apple.com/v1/subscriptionGroups/group-1/subscriptionGroupLocalizations?cursor=BQ&limit=200"

	firstBody := `{"data":[{"type":"subscriptionGroupLocalizations","id":"subscription-group-localization-next-1"}],"links":{"next":"` + secondURL + `"}}`
	secondBody := `{"data":[{"type":"subscriptionGroupLocalizations","id":"subscription-group-localization-next-2"}],"links":{"next":""}}`

	runGameCenterAchievementsPaginateFromNext(
		t,
		[]string{"subscriptions", "groups", "localizations", "list"},
		firstURL,
		secondURL,
		firstBody,
		secondBody,
		"subscription-group-localization-next-1",
		"subscription-group-localization-next-2",
	)
}

func TestSubscriptionsImagesListRejectsInvalidNextURLPhase62(t *testing.T) {
	runGameCenterAchievementsInvalidNextURLCases(
		t,
		[]string{"subscriptions", "images", "list"},
		"subscriptions images list: --next",
	)
}

func TestSubscriptionsImagesListPaginateFromNextWithoutSubscriptionIDPhase62(t *testing.T) {
	const firstURL = "https://api.appstoreconnect.apple.com/v1/subscriptions/sub-1/images?cursor=AQ&limit=200"
	const secondURL = "https://api.appstoreconnect.apple.com/v1/subscriptions/sub-1/images?cursor=BQ&limit=200"

	firstBody := `{"data":[{"type":"subscriptionImages","id":"subscription-image-next-1"}],"links":{"next":"` + secondURL + `"}}`
	secondBody := `{"data":[{"type":"subscriptionImages","id":"subscription-image-next-2"}],"links":{"next":""}}`

	runGameCenterAchievementsPaginateFromNext(
		t,
		[]string{"subscriptions", "images", "list"},
		firstURL,
		secondURL,
		firstBody,
		secondBody,
		"subscription-image-next-1",
		"subscription-image-next-2",
	)
}

func TestSubscriptionsIntroductoryOffersListRejectsInvalidNextURLPhase62(t *testing.T) {
	runGameCenterAchievementsInvalidNextURLCases(
		t,
		[]string{"subscriptions", "offers", "introductory", "list"},
		"subscriptions offers introductory list: --next",
	)
}

func TestSubscriptionsIntroductoryOffersListPaginateFromNextWithoutSubscriptionIDPhase62(t *testing.T) {
	const firstURL = "https://api.appstoreconnect.apple.com/v1/subscriptions/sub-1/introductoryOffers?cursor=AQ&limit=200"
	const secondURL = "https://api.appstoreconnect.apple.com/v1/subscriptions/sub-1/introductoryOffers?cursor=BQ&limit=200"

	firstBody := `{"data":[{"type":"subscriptionIntroductoryOffers","id":"subscription-intro-offer-next-1"}],"links":{"next":"` + secondURL + `"}}`
	secondBody := `{"data":[{"type":"subscriptionIntroductoryOffers","id":"subscription-intro-offer-next-2"}],"links":{"next":""}}`

	runGameCenterAchievementsPaginateFromNext(
		t,
		[]string{"subscriptions", "offers", "introductory", "list"},
		firstURL,
		secondURL,
		firstBody,
		secondBody,
		"subscription-intro-offer-next-1",
		"subscription-intro-offer-next-2",
	)
}

func TestSubscriptionsListRejectsInvalidNextURLPhase62(t *testing.T) {
	runGameCenterAchievementsInvalidNextURLCases(
		t,
		[]string{"subscriptions", "list"},
		"subscriptions list: --next",
	)
}

func TestSubscriptionsListPaginateFromNextWithoutGroupPhase62(t *testing.T) {
	const firstURL = "https://api.appstoreconnect.apple.com/v1/subscriptionGroups/group-1/subscriptions?cursor=AQ&limit=200"
	const secondURL = "https://api.appstoreconnect.apple.com/v1/subscriptionGroups/group-1/subscriptions?cursor=BQ&limit=200"

	firstBody := `{"data":[{"type":"subscriptions","id":"subscription-next-1"}],"links":{"next":"` + secondURL + `"}}`
	secondBody := `{"data":[{"type":"subscriptions","id":"subscription-next-2"}],"links":{"next":""}}`

	runGameCenterAchievementsPaginateFromNext(
		t,
		[]string{"subscriptions", "list"},
		firstURL,
		secondURL,
		firstBody,
		secondBody,
		"subscription-next-1",
		"subscription-next-2",
	)
}

func TestSubscriptionsLocalizationsListRejectsInvalidNextURLPhase62(t *testing.T) {
	runGameCenterAchievementsInvalidNextURLCases(
		t,
		[]string{"subscriptions", "localizations", "list"},
		"subscriptions localizations list: --next",
	)
}

func TestSubscriptionsLocalizationsListPaginateFromNextWithoutSubscriptionIDPhase62(t *testing.T) {
	const firstURL = "https://api.appstoreconnect.apple.com/v1/subscriptions/sub-1/subscriptionLocalizations?cursor=AQ&limit=200"
	const secondURL = "https://api.appstoreconnect.apple.com/v1/subscriptions/sub-1/subscriptionLocalizations?cursor=BQ&limit=200"

	firstBody := `{"data":[{"type":"subscriptionLocalizations","id":"subscription-localization-next-1"}],"links":{"next":"` + secondURL + `"}}`
	secondBody := `{"data":[{"type":"subscriptionLocalizations","id":"subscription-localization-next-2"}],"links":{"next":""}}`

	runGameCenterAchievementsPaginateFromNext(
		t,
		[]string{"subscriptions", "localizations", "list"},
		firstURL,
		secondURL,
		firstBody,
		secondBody,
		"subscription-localization-next-1",
		"subscription-localization-next-2",
	)
}

func TestSubscriptionsOfferCodesCustomCodesRejectsInvalidNextURLPhase62(t *testing.T) {
	runGameCenterAchievementsInvalidNextURLCases(
		t,
		[]string{"subscriptions", "offers", "offer-codes", "custom-codes", "list"},
		"subscriptions offers offer-codes custom-codes list: --next",
	)
}

func TestSubscriptionsOfferCodesCustomCodesPaginateFromNextWithoutOfferCodeIDPhase62(t *testing.T) {
	const firstURL = "https://api.appstoreconnect.apple.com/v1/subscriptionOfferCodes/offer-1/customCodes?cursor=AQ&limit=200"
	const secondURL = "https://api.appstoreconnect.apple.com/v1/subscriptionOfferCodes/offer-1/customCodes?cursor=BQ&limit=200"

	firstBody := `{"data":[{"type":"subscriptionOfferCodeCustomCodes","id":"subscription-custom-code-next-1"}],"links":{"next":"` + secondURL + `"}}`
	secondBody := `{"data":[{"type":"subscriptionOfferCodeCustomCodes","id":"subscription-custom-code-next-2"}],"links":{"next":""}}`

	runGameCenterAchievementsPaginateFromNext(
		t,
		[]string{"subscriptions", "offers", "offer-codes", "custom-codes", "list"},
		firstURL,
		secondURL,
		firstBody,
		secondBody,
		"subscription-custom-code-next-1",
		"subscription-custom-code-next-2",
	)
}

func TestSubscriptionsOfferCodesOneTimeCodesListRejectsInvalidNextURLPhase62(t *testing.T) {
	runGameCenterAchievementsInvalidNextURLCases(
		t,
		[]string{"subscriptions", "offers", "offer-codes", "one-time-codes", "list"},
		"subscriptions offers offer-codes one-time-codes list: --next",
	)
}

func TestSubscriptionsOfferCodesOneTimeCodesListPaginateFromNextWithoutOfferCodeIDPhase62(t *testing.T) {
	const firstURL = "https://api.appstoreconnect.apple.com/v1/subscriptionOfferCodes/offer-1/oneTimeUseCodes?cursor=AQ&limit=200"
	const secondURL = "https://api.appstoreconnect.apple.com/v1/subscriptionOfferCodes/offer-1/oneTimeUseCodes?cursor=BQ&limit=200"

	firstBody := `{"data":[{"type":"subscriptionOfferCodeOneTimeUseCodes","id":"subscription-one-time-code-next-1"}],"links":{"next":"` + secondURL + `"}}`
	secondBody := `{"data":[{"type":"subscriptionOfferCodeOneTimeUseCodes","id":"subscription-one-time-code-next-2"}],"links":{"next":""}}`

	runGameCenterAchievementsPaginateFromNext(
		t,
		[]string{"subscriptions", "offers", "offer-codes", "one-time-codes", "list"},
		firstURL,
		secondURL,
		firstBody,
		secondBody,
		"subscription-one-time-code-next-1",
		"subscription-one-time-code-next-2",
	)
}

func TestSubscriptionsOfferCodesPricesRejectsInvalidNextURLPhase62(t *testing.T) {
	runGameCenterAchievementsInvalidNextURLCases(
		t,
		[]string{"subscriptions", "offers", "offer-codes", "prices"},
		"subscriptions offers offer-codes prices: --next",
	)
}

func TestSubscriptionsOfferCodesPricesPaginateFromNextWithoutOfferCodeIDPhase62(t *testing.T) {
	const firstURL = "https://api.appstoreconnect.apple.com/v1/subscriptionOfferCodes/offer-1/prices?cursor=AQ&limit=200"
	const secondURL = "https://api.appstoreconnect.apple.com/v1/subscriptionOfferCodes/offer-1/prices?cursor=BQ&limit=200"

	firstBody := `{"data":[{"type":"subscriptionOfferCodePrices","id":"subscription-offer-code-price-next-1"}],"links":{"next":"` + secondURL + `"}}`
	secondBody := `{"data":[{"type":"subscriptionOfferCodePrices","id":"subscription-offer-code-price-next-2"}],"links":{"next":""}}`

	runGameCenterAchievementsPaginateFromNext(
		t,
		[]string{"subscriptions", "offers", "offer-codes", "prices"},
		firstURL,
		secondURL,
		firstBody,
		secondBody,
		"subscription-offer-code-price-next-1",
		"subscription-offer-code-price-next-2",
	)
}

func TestSubscriptionsPricesListRejectsInvalidNextURLPhase62(t *testing.T) {
	runGameCenterAchievementsInvalidNextURLCases(
		t,
		[]string{"subscriptions", "pricing", "prices", "list"},
		"subscriptions pricing prices list: --next",
	)
}

func TestSubscriptionsPricesListPaginateFromNextWithoutIDPhase62(t *testing.T) {
	const firstURL = "https://api.appstoreconnect.apple.com/v1/subscriptions/sub-1/prices?cursor=AQ&limit=200"
	const secondURL = "https://api.appstoreconnect.apple.com/v1/subscriptions/sub-1/prices?cursor=BQ&limit=200"

	firstBody := `{"data":[{"type":"subscriptionPrices","id":"subscription-price-next-1"}],"links":{"next":"` + secondURL + `"}}`
	secondBody := `{"data":[{"type":"subscriptionPrices","id":"subscription-price-next-2"}],"links":{"next":""}}`

	runGameCenterAchievementsPaginateFromNext(
		t,
		[]string{"subscriptions", "pricing", "prices", "list"},
		firstURL,
		secondURL,
		firstBody,
		secondBody,
		"subscription-price-next-1",
		"subscription-price-next-2",
	)
}

func TestSubscriptionsPromotionalOffersListRejectsInvalidNextURLPhase62(t *testing.T) {
	runGameCenterAchievementsInvalidNextURLCases(
		t,
		[]string{"subscriptions", "offers", "promotional", "list"},
		"subscriptions offers promotional list: --next",
	)
}

func TestSubscriptionsPromotionalOffersListPaginateFromNextWithoutSubscriptionIDPhase62(t *testing.T) {
	const firstURL = "https://api.appstoreconnect.apple.com/v1/subscriptions/sub-1/promotionalOffers?cursor=AQ&limit=200"
	const secondURL = "https://api.appstoreconnect.apple.com/v1/subscriptions/sub-1/promotionalOffers?cursor=BQ&limit=200"

	firstBody := `{"data":[{"type":"subscriptionPromotionalOffers","id":"subscription-promotional-offer-next-1"}],"links":{"next":"` + secondURL + `"}}`
	secondBody := `{"data":[{"type":"subscriptionPromotionalOffers","id":"subscription-promotional-offer-next-2"}],"links":{"next":""}}`

	runGameCenterAchievementsPaginateFromNext(
		t,
		[]string{"subscriptions", "offers", "promotional", "list"},
		firstURL,
		secondURL,
		firstBody,
		secondBody,
		"subscription-promotional-offer-next-1",
		"subscription-promotional-offer-next-2",
	)
}

func TestSubscriptionsPromotionalOffersPricesRejectsInvalidNextURLPhase62(t *testing.T) {
	runGameCenterAchievementsInvalidNextURLCases(
		t,
		[]string{"subscriptions", "offers", "promotional", "prices"},
		"subscriptions offers promotional prices: --next",
	)
}

func TestSubscriptionsPromotionalOffersPricesPaginateFromNextWithoutIDPhase62(t *testing.T) {
	const firstURL = "https://api.appstoreconnect.apple.com/v1/subscriptionPromotionalOffers/offer-1/prices?cursor=AQ&limit=200"
	const secondURL = "https://api.appstoreconnect.apple.com/v1/subscriptionPromotionalOffers/offer-1/prices?cursor=BQ&limit=200"

	firstBody := `{"data":[{"type":"subscriptionPromotionalOfferPrices","id":"subscription-promotional-offer-price-next-1"}],"links":{"next":"` + secondURL + `"}}`
	secondBody := `{"data":[{"type":"subscriptionPromotionalOfferPrices","id":"subscription-promotional-offer-price-next-2"}],"links":{"next":""}}`

	runGameCenterAchievementsPaginateFromNext(
		t,
		[]string{"subscriptions", "offers", "promotional", "prices"},
		firstURL,
		secondURL,
		firstBody,
		secondBody,
		"subscription-promotional-offer-price-next-1",
		"subscription-promotional-offer-price-next-2",
	)
}
