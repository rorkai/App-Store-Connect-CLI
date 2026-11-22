package validation

import (
	"fmt"
	"strings"
)

func pricingChecks(appID string, priceScheduleID string, basePriceMissing bool, baseTerritory string, skipReason string) []CheckResult {
	if strings.TrimSpace(skipReason) != "" {
		return []CheckResult{{
			ID:           "pricing.schedule.unverified",
			Severity:     SeverityWarning,
			Field:        "appPriceSchedule",
			ResourceType: "app",
			ResourceID:   strings.TrimSpace(appID),
			Message:      "could not verify app price schedule",
			Remediation:  strings.TrimSpace(skipReason),
		}}
	}
	if strings.TrimSpace(priceScheduleID) != "" {
		if !basePriceMissing {
			return nil
		}
		territory := strings.ToUpper(strings.TrimSpace(baseTerritory))
		message := "app has no price set"
		remediationTerritory := "USA"
		if territory != "" {
			message = fmt.Sprintf("app has no price set for base territory %s", territory)
			remediationTerritory = territory
		}
		return []CheckResult{{
			ID:           "pricing.base_price.missing",
			Severity:     SeverityError,
			Field:        "manualPrices",
			ResourceType: "appPriceSchedule",
			ResourceID:   strings.TrimSpace(priceScheduleID),
			Message:      message,
			Remediation:  fmt.Sprintf("Set a price (Free counts): asc pricing schedule create --app %s --free --base-territory %q", strings.TrimSpace(appID), remediationTerritory),
		}}
	}
	return []CheckResult{
		{
			ID:           "pricing.schedule.missing",
			Severity:     SeverityError,
			Field:        "appPriceSchedule",
			ResourceType: "app",
			ResourceID:   strings.TrimSpace(appID),
			Message:      "app price schedule is missing",
			Remediation:  "Set the price the developer chooses, Free included (list price points with asc pricing price-points)",
		},
	}
}

func availabilityChecks(appID string, availabilityID string, availableTerritories int, skipReason string) []CheckResult {
	if strings.TrimSpace(skipReason) != "" {
		return []CheckResult{{
			ID:           "availability.unverified",
			Severity:     SeverityWarning,
			Field:        "appAvailabilityV2",
			ResourceType: "app",
			ResourceID:   strings.TrimSpace(appID),
			Message:      "could not verify app availability",
			Remediation:  strings.TrimSpace(skipReason),
		}}
	}
	if strings.TrimSpace(availabilityID) == "" {
		return []CheckResult{
			{
				ID:           "availability.missing",
				Severity:     SeverityError,
				Field:        "appAvailabilityV2",
				ResourceType: "app",
				ResourceID:   strings.TrimSpace(appID),
				Message:      "app availability is missing",
				Remediation:  "Make the app available in at least one territory",
			},
		}
	}

	if availableTerritories > 0 {
		return nil
	}

	return []CheckResult{
		{
			ID:           "availability.territories.none",
			Severity:     SeverityError,
			Field:        "territoryAvailabilities",
			ResourceType: "appAvailabilityV2",
			ResourceID:   strings.TrimSpace(availabilityID),
			Message:      fmt.Sprintf("no available territories configured (available=%d)", availableTerritories),
			Remediation:  "Enable at least one territory",
		},
	}
}
