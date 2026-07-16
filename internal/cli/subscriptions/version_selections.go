package subscriptions

import (
	"flag"
	"strings"

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
