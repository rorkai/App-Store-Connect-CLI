package subscriptions

import (
	"fmt"
	"os"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

var subscriptionPeriodValues = []string{
	string(asc.SubscriptionPeriodOneWeek),
	string(asc.SubscriptionPeriodOneMonth),
	string(asc.SubscriptionPeriodTwoMonths),
	string(asc.SubscriptionPeriodThreeMonths),
	string(asc.SubscriptionPeriodSixMonths),
	string(asc.SubscriptionPeriodOneYear),
}

var subscriptionPeriodMap = map[string]asc.SubscriptionPeriod{
	string(asc.SubscriptionPeriodOneWeek):     asc.SubscriptionPeriodOneWeek,
	string(asc.SubscriptionPeriodOneMonth):    asc.SubscriptionPeriodOneMonth,
	string(asc.SubscriptionPeriodTwoMonths):   asc.SubscriptionPeriodTwoMonths,
	string(asc.SubscriptionPeriodThreeMonths): asc.SubscriptionPeriodThreeMonths,
	string(asc.SubscriptionPeriodSixMonths):   asc.SubscriptionPeriodSixMonths,
	string(asc.SubscriptionPeriodOneYear):     asc.SubscriptionPeriodOneYear,
}

var subscriptionGracePeriodDurationValues = []string{
	string(asc.SubscriptionGracePeriodDurationThreeDays),
	string(asc.SubscriptionGracePeriodDurationSixteenDays),
	string(asc.SubscriptionGracePeriodDurationTwentyEightDays),
}

var subscriptionGracePeriodDurationMap = map[string]string{
	string(asc.SubscriptionGracePeriodDurationThreeDays):       string(asc.SubscriptionGracePeriodDurationThreeDays),
	string(asc.SubscriptionGracePeriodDurationSixteenDays):     string(asc.SubscriptionGracePeriodDurationSixteenDays),
	string(asc.SubscriptionGracePeriodDurationTwentyEightDays): string(asc.SubscriptionGracePeriodDurationTwentyEightDays),
	"DAY_3":  "DAY_3",
	"DAY_16": "DAY_16",
	"DAY_28": "DAY_28",
}

var subscriptionGracePeriodRenewalTypeValues = []string{
	string(asc.SubscriptionGracePeriodRenewalTypeAllRenewals),
	string(asc.SubscriptionGracePeriodRenewalTypePaidToPaidOnly),
}

var subscriptionGracePeriodRenewalTypeMap = map[string]asc.SubscriptionGracePeriodRenewalType{
	string(asc.SubscriptionGracePeriodRenewalTypeAllRenewals):    asc.SubscriptionGracePeriodRenewalTypeAllRenewals,
	string(asc.SubscriptionGracePeriodRenewalTypePaidToPaidOnly): asc.SubscriptionGracePeriodRenewalTypePaidToPaidOnly,
}

var subscriptionOfferDurationValues = []string{
	string(asc.SubscriptionOfferDurationThreeDays),
	string(asc.SubscriptionOfferDurationOneWeek),
	string(asc.SubscriptionOfferDurationTwoWeeks),
	string(asc.SubscriptionOfferDurationOneMonth),
	string(asc.SubscriptionOfferDurationTwoMonths),
	string(asc.SubscriptionOfferDurationThreeMonths),
	string(asc.SubscriptionOfferDurationSixMonths),
	string(asc.SubscriptionOfferDurationOneYear),
}

var subscriptionOfferDurationMap = map[string]asc.SubscriptionOfferDuration{
	string(asc.SubscriptionOfferDurationThreeDays):   asc.SubscriptionOfferDurationThreeDays,
	string(asc.SubscriptionOfferDurationOneWeek):     asc.SubscriptionOfferDurationOneWeek,
	string(asc.SubscriptionOfferDurationTwoWeeks):    asc.SubscriptionOfferDurationTwoWeeks,
	string(asc.SubscriptionOfferDurationOneMonth):    asc.SubscriptionOfferDurationOneMonth,
	string(asc.SubscriptionOfferDurationTwoMonths):   asc.SubscriptionOfferDurationTwoMonths,
	string(asc.SubscriptionOfferDurationThreeMonths): asc.SubscriptionOfferDurationThreeMonths,
	string(asc.SubscriptionOfferDurationSixMonths):   asc.SubscriptionOfferDurationSixMonths,
	string(asc.SubscriptionOfferDurationOneYear):     asc.SubscriptionOfferDurationOneYear,
}

var subscriptionOfferModeValues = []string{
	string(asc.SubscriptionOfferModePayAsYouGo),
	string(asc.SubscriptionOfferModePayUpFront),
	string(asc.SubscriptionOfferModeFreeTrial),
}

var subscriptionOfferModeMap = map[string]asc.SubscriptionOfferMode{
	string(asc.SubscriptionOfferModePayAsYouGo): asc.SubscriptionOfferModePayAsYouGo,
	string(asc.SubscriptionOfferModePayUpFront): asc.SubscriptionOfferModePayUpFront,
	string(asc.SubscriptionOfferModeFreeTrial):  asc.SubscriptionOfferModeFreeTrial,
}

var subscriptionOfferEligibilityValues = []string{
	string(asc.SubscriptionOfferEligibilityStackWithIntroOffers),
	string(asc.SubscriptionOfferEligibilityReplaceIntroOffers),
}

var subscriptionOfferEligibilityMap = map[string]asc.SubscriptionOfferEligibility{
	string(asc.SubscriptionOfferEligibilityStackWithIntroOffers): asc.SubscriptionOfferEligibilityStackWithIntroOffers,
	string(asc.SubscriptionOfferEligibilityReplaceIntroOffers):   asc.SubscriptionOfferEligibilityReplaceIntroOffers,
}

var subscriptionCustomerEligibilityValues = []string{
	string(asc.SubscriptionCustomerEligibilityNew),
	string(asc.SubscriptionCustomerEligibilityExisting),
	string(asc.SubscriptionCustomerEligibilityExpired),
}

var subscriptionCustomerEligibilityMap = map[string]asc.SubscriptionCustomerEligibility{
	string(asc.SubscriptionCustomerEligibilityNew):      asc.SubscriptionCustomerEligibilityNew,
	string(asc.SubscriptionCustomerEligibilityExisting): asc.SubscriptionCustomerEligibilityExisting,
	string(asc.SubscriptionCustomerEligibilityExpired):  asc.SubscriptionCustomerEligibilityExpired,
}

func normalizeSubscriptionOfferDuration(value string) (asc.SubscriptionOfferDuration, error) {
	normalized := strings.ToUpper(strings.TrimSpace(value))
	if normalized == "" {
		return "", fmt.Errorf("--offer-duration is required")
	}
	if duration, ok := subscriptionOfferDurationMap[normalized]; ok {
		return duration, nil
	}
	return "", fmt.Errorf("--offer-duration must be one of: %s", strings.Join(subscriptionOfferDurationValues, ", "))
}

func normalizeSubscriptionPeriod(value string, required bool) (asc.SubscriptionPeriod, error) {
	normalized := strings.ToUpper(strings.TrimSpace(value))
	if normalized == "" {
		if required {
			return "", fmt.Errorf("--subscription-period is required")
		}
		return "", nil
	}
	if period, ok := subscriptionPeriodMap[normalized]; ok {
		return period, nil
	}
	return "", fmt.Errorf("--subscription-period must be one of: %s", strings.Join(subscriptionPeriodValues, ", "))
}

func normalizeSubscriptionGracePeriodDuration(value string, required bool) (string, error) {
	normalized := strings.ToUpper(strings.TrimSpace(value))
	if normalized == "" {
		if required {
			return "", fmt.Errorf("--duration is required")
		}
		return "", nil
	}
	if duration, ok := subscriptionGracePeriodDurationMap[normalized]; ok {
		return duration, nil
	}
	return "", fmt.Errorf("--duration must be one of: %s", strings.Join(subscriptionGracePeriodDurationValues, ", "))
}

func normalizeSubscriptionGracePeriodRenewalType(value string, required bool) (asc.SubscriptionGracePeriodRenewalType, error) {
	normalized := strings.ToUpper(strings.TrimSpace(value))
	if normalized == "" {
		if required {
			return "", fmt.Errorf("--renewal-type is required")
		}
		return "", nil
	}
	if renewalType, ok := subscriptionGracePeriodRenewalTypeMap[normalized]; ok {
		return renewalType, nil
	}
	return "", fmt.Errorf("--renewal-type must be one of: %s", strings.Join(subscriptionGracePeriodRenewalTypeValues, ", "))
}

func normalizeSubscriptionOfferMode(value string) (asc.SubscriptionOfferMode, error) {
	normalized := strings.ToUpper(strings.TrimSpace(value))
	if normalized == "" {
		return "", fmt.Errorf("--offer-mode is required")
	}
	if mode, ok := subscriptionOfferModeMap[normalized]; ok {
		return mode, nil
	}
	return "", fmt.Errorf("--offer-mode must be one of: %s", strings.Join(subscriptionOfferModeValues, ", "))
}

func normalizeSubscriptionOfferEligibility(value string, required bool) (asc.SubscriptionOfferEligibility, error) {
	normalized := strings.ToUpper(strings.TrimSpace(value))
	if normalized == "" {
		if required {
			return "", fmt.Errorf("--offer-eligibility is required")
		}
		return "", nil
	}
	if eligibility, ok := subscriptionOfferEligibilityMap[normalized]; ok {
		return eligibility, nil
	}
	return "", fmt.Errorf("--offer-eligibility must be one of: %s", strings.Join(subscriptionOfferEligibilityValues, ", "))
}

func normalizeSubscriptionCustomerEligibilities(value string) ([]asc.SubscriptionCustomerEligibility, error) {
	values := shared.SplitCSVUpper(value)
	if len(values) == 0 {
		return nil, fmt.Errorf("--customer-eligibilities is required")
	}

	eligibilities := make([]asc.SubscriptionCustomerEligibility, 0, len(values))
	for _, item := range values {
		eligibility, ok := subscriptionCustomerEligibilityMap[item]
		if !ok {
			return nil, fmt.Errorf("--customer-eligibilities must be one of: %s", strings.Join(subscriptionCustomerEligibilityValues, ", "))
		}
		eligibilities = append(eligibilities, eligibility)
	}
	return eligibilities, nil
}

func parseSubscriptionOfferCodePrices(value string, mode asc.SubscriptionOfferMode) ([]asc.SubscriptionOfferCodePrice, error) {
	if mode == asc.SubscriptionOfferModeFreeTrial {
		if strings.Contains(value, ":") {
			return nil, fmt.Errorf("--prices for FREE_TRIAL must use TERRITORY entries without price point IDs")
		}
		territoryIDs, err := shared.NormalizeASCTerritoryCSV(value)
		if err != nil {
			return nil, err
		}
		prices := make([]asc.SubscriptionOfferCodePrice, 0, len(territoryIDs))
		for _, territoryID := range territoryIDs {
			prices = append(prices, asc.SubscriptionOfferCodePrice{TerritoryID: territoryID})
		}
		return prices, nil
	}

	entries, err := shared.ParseASCTerritoryValueCSV(value)
	if err != nil {
		return nil, err
	}
	if len(entries) == 0 {
		return nil, nil
	}

	prices := make([]asc.SubscriptionOfferCodePrice, 0, len(entries))
	for _, entry := range entries {
		prices = append(prices, asc.SubscriptionOfferCodePrice{
			TerritoryID:  entry.TerritoryID,
			PricePointID: entry.Value,
		})
	}

	return prices, nil
}

func openSubscriptionImageFile(path string) (*os.File, os.FileInfo, error) {
	if err := asc.ValidateImageFile(path); err != nil {
		return nil, nil, err
	}
	file, err := shared.OpenExistingNoFollow(path)
	if err != nil {
		return nil, nil, err
	}
	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, nil, err
	}
	return file, info, nil
}
