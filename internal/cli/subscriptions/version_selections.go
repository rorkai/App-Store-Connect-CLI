package subscriptions

import (
	"flag"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func normalizeSelectionFlag(fs *flag.FlagSet, value, flagName string, allowed []string) ([]string, error) {
	provided := flagWasProvided(fs, strings.TrimPrefix(flagName, "--"))
	values, err := shared.NormalizeSelection(value, allowed, flagName)
	if err != nil {
		return nil, shared.UsageError(err.Error())
	}
	if provided && len(values) == 0 {
		return nil, shared.UsageErrorf("%s must not be empty", flagName)
	}
	return values, nil
}

func subscriptionVersionFieldsList() []string {
	return []string{"version", "state", "subscription", "image", "images", "localizations"}
}

func subscriptionFieldsList() []string {
	return []string{
		"name",
		"productId",
		"familySharable",
		"marketSettings",
		"multiSeatStatus",
		"state",
		"subscriptionPeriod",
		"reviewNote",
		"groupLevel",
		"subscriptionLocalizations",
		"appStoreReviewScreenshot",
		"group",
		"introductoryOffers",
		"promotionalOffers",
		"offerCodes",
		"prices",
		"pricePoints",
		"promotedPurchase",
		"subscriptionAvailability",
		"winBackOffers",
		"images",
		"planAvailabilities",
		"versions",
	}
}

func subscriptionGroupLocalizationFieldsList() []string {
	return []string{"name", "customAppName", "locale", "state", "subscriptionGroup"}
}

type subscriptionRelatedFieldFlag struct {
	name     string
	resource string
	allowed  []string
	set      func(*asc.SubscriptionRelatedFields, []string)
}

var subscriptionRelatedFieldFlags = []subscriptionRelatedFieldFlag{
	{
		"localization-fields", "subscriptionLocalizations",
		[]string{"name", "locale", "description", "state", "subscription"},
		func(f *asc.SubscriptionRelatedFields, v []string) { f.Localizations = v },
	},
	{
		"review-screenshot-fields", "subscriptionAppStoreReviewScreenshots",
		[]string{"fileSize", "fileName", "sourceFileChecksum", "imageAsset", "assetToken", "assetType", "uploadOperations", "assetDeliveryState", "subscription"},
		func(f *asc.SubscriptionRelatedFields, v []string) { f.ReviewScreenshots = v },
	},
	{
		"group-fields", "subscriptionGroups",
		subscriptionGroupVersionGroupFields,
		func(f *asc.SubscriptionRelatedFields, v []string) { f.Groups = v },
	},
	{
		"introductory-offer-fields", "subscriptionIntroductoryOffers",
		[]string{"startDate", "endDate", "duration", "offerMode", "numberOfPeriods", "targetSubscriptionPlanType", "subscription", "territory", "subscriptionPricePoint"},
		func(f *asc.SubscriptionRelatedFields, v []string) { f.IntroductoryOffers = v },
	},
	{
		"promotional-offer-fields", "subscriptionPromotionalOffers",
		[]string{"duration", "name", "numberOfPeriods", "offerCode", "offerMode", "targetSubscriptionPlanType", "subscription", "prices"},
		func(f *asc.SubscriptionRelatedFields, v []string) { f.PromotionalOffers = v },
	},
	{
		"offer-code-fields", "subscriptionOfferCodes",
		[]string{"name", "customerEligibilities", "offerEligibility", "duration", "offerMode", "numberOfPeriods", "totalNumberOfCodes", "productionCodeCount", "sandboxCodeCount", "active", "autoRenewEnabled", "targetSubscriptionPlanType", "subscription", "oneTimeUseCodes", "customCodes", "prices"},
		func(f *asc.SubscriptionRelatedFields, v []string) { f.OfferCodes = v },
	},
	{
		"price-fields", "subscriptionPrices",
		subscriptionPriceFieldsList(),
		func(f *asc.SubscriptionRelatedFields, v []string) { f.Prices = v },
	},
	{
		"promoted-purchase-fields", "promotedPurchases",
		[]string{"visibleForAllUsers", "enabled", "state", "inAppPurchaseV2", "subscription"},
		func(f *asc.SubscriptionRelatedFields, v []string) { f.PromotedPurchases = v },
	},
	{
		"availability-fields", "subscriptionAvailabilities",
		[]string{"availableInNewTerritories", "availableTerritories"},
		func(f *asc.SubscriptionRelatedFields, v []string) { f.Availabilities = v },
	},
	{
		"win-back-offer-fields", "winBackOffers",
		[]string{"referenceName", "offerId", "duration", "offerMode", "periodCount", "customerEligibilityPaidSubscriptionDurationInMonths", "customerEligibilityTimeSinceLastSubscribedInMonths", "customerEligibilityWaitBetweenOffersInMonths", "startDate", "endDate", "priority", "promotionIntent", "targetSubscriptionPlanType", "prices"},
		func(f *asc.SubscriptionRelatedFields, v []string) { f.WinBackOffers = v },
	},
	{
		"image-fields", "subscriptionImages",
		[]string{"fileSize", "fileName", "sourceFileChecksum", "assetToken", "imageAsset", "uploadOperations", "state", "subscription"},
		func(f *asc.SubscriptionRelatedFields, v []string) { f.Images = v },
	},
	{
		"plan-availability-fields", "subscriptionPlanAvailabilities",
		[]string{"availableInNewTerritories", "planType", "availableTerritories"},
		func(f *asc.SubscriptionRelatedFields, v []string) { f.PlanAvailabilities = v },
	},
}

func bindSubscriptionRelatedFieldFlags(fs *flag.FlagSet) map[string]*string {
	values := make(map[string]*string, len(subscriptionRelatedFieldFlags))
	for _, field := range subscriptionRelatedFieldFlags {
		values[field.name] = fs.String(field.name, "", "Sparse fields for included "+field.resource)
	}
	return values
}

func subscriptionRelatedFieldFlagNames() []string {
	names := make([]string, 0, len(subscriptionRelatedFieldFlags))
	for _, field := range subscriptionRelatedFieldFlags {
		names = append(names, field.name)
	}
	return names
}

func subscriptionRelatedFieldValues(fs *flag.FlagSet, raw map[string]*string) (asc.SubscriptionRelatedFields, error) {
	var selected asc.SubscriptionRelatedFields
	for _, field := range subscriptionRelatedFieldFlags {
		values, err := normalizeSelectionFlag(fs, *raw[field.name], "--"+field.name, field.allowed)
		if err != nil {
			return selected, err
		}
		field.set(&selected, values)
	}
	return selected, nil
}

func subscriptionVersionImageFieldsList() []string {
	return []string{"fileSize", "fileName", "assetToken", "imageAsset", "uploadOperations", "assetDeliveryState"}
}

func subscriptionVersionLocalizationFieldsList() []string {
	return []string{"name", "locale", "description", "version"}
}

func subscriptionVersionIncludeList() []string {
	return []string{"subscription", "image", "images", "localizations"}
}

func subscriptionLocalizationV2IncludeList() []string {
	return []string{"version"}
}

func subscriptionIntroductoryOfferIncludeList() []string {
	return []string{"subscription", "territory", "subscriptionPricePoint"}
}

func subscriptionIncludeList() []string {
	return []string{
		"subscriptionLocalizations",
		"appStoreReviewScreenshot",
		"group",
		"introductoryOffers",
		"promotionalOffers",
		"offerCodes",
		"prices",
		"promotedPurchase",
		"subscriptionAvailability",
		"winBackOffers",
		"images",
		"planAvailabilities",
		"versions",
	}
}
