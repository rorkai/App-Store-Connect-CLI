package subscriptions

import (
	"fmt"
	"os"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

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

func normalizeSubscriptionOfferDuration(value string, required bool) (asc.SubscriptionOfferDuration, error) {
	normalized := strings.ToUpper(strings.TrimSpace(value))
	if normalized == "" {
		if required {
			return "", fmt.Errorf("--offer-duration is required")
		}
		return "", nil
	}
	if duration, ok := subscriptionOfferDurationMap[normalized]; ok {
		return duration, nil
	}
	return "", fmt.Errorf("--offer-duration must be one of: %s", strings.Join(subscriptionOfferDurationValues, ", "))
}

func normalizeSubscriptionOfferMode(value string, required bool) (asc.SubscriptionOfferMode, error) {
	normalized := strings.ToUpper(strings.TrimSpace(value))
	if normalized == "" {
		if required {
			return "", fmt.Errorf("--offer-mode is required")
		}
		return "", nil
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

func normalizeSubscriptionCustomerEligibilities(value string, required bool) ([]asc.SubscriptionCustomerEligibility, error) {
	values := splitCSVUpper(value)
	if len(values) == 0 {
		if required {
			return nil, fmt.Errorf("--customer-eligibilities is required")
		}
		return nil, nil
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

type deleteResult struct {
	ID      string `json:"id"`
	Deleted bool   `json:"deleted"`
}
