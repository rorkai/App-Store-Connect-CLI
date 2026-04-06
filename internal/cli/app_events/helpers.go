package app_events

import (
	"context"
	"fmt"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

const appEventAssetUploadDefaultTimeout = 10 * time.Minute

var appEventPurchaseRequirementSanitizer = strings.NewReplacer("_", "", "-", "", " ", "")

func supportedAppEventPurchaseRequirementValues() string {
	return string(asc.AppEventPurchaseRequirementNoCostAssociated)
}

func normalizeAppEventBadge(value string, required bool) (string, error) {
	normalized := strings.ToUpper(strings.TrimSpace(value))
	if normalized == "" {
		if required {
			return "", fmt.Errorf("--event-type is required")
		}
		return "", nil
	}
	if slices.Contains(asc.ValidAppEventBadges, normalized) {
		return normalized, nil
	}
	return "", fmt.Errorf("--event-type must be one of: %s", strings.Join(asc.ValidAppEventBadges, ", "))
}

func normalizeAppEventPriority(value string) (string, error) {
	normalized := strings.ToUpper(strings.TrimSpace(value))
	if normalized == "" {
		return "", nil
	}
	if slices.Contains(asc.ValidAppEventPriorities, normalized) {
		return normalized, nil
	}
	return "", fmt.Errorf("--priority must be one of: %s", strings.Join(asc.ValidAppEventPriorities, ", "))
}

func normalizeAppEventPurpose(value string) (string, error) {
	normalized := strings.ToUpper(strings.TrimSpace(value))
	if normalized == "" {
		return "", nil
	}
	if slices.Contains(asc.ValidAppEventPurposes, normalized) {
		return normalized, nil
	}
	return "", fmt.Errorf("--purpose must be one of: %s", strings.Join(asc.ValidAppEventPurposes, ", "))
}

func normalizeAppEventPurchaseRequirement(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", nil
	}

	token := appEventPurchaseRequirementSanitizer.Replace(strings.ToUpper(trimmed))
	switch token {
	case "NOCOSTASSOCIATED":
		return string(asc.AppEventPurchaseRequirementNoCostAssociated), nil
	case "NOIAPREQUIRED":
		return string(asc.AppEventPurchaseRequirementNoIAPRequired), nil
	case "IAPREQUIRED":
		return string(asc.AppEventPurchaseRequirementIAPRequired), nil
	default:
		return "", fmt.Errorf(
			"--purchase-requirement currently supports only: %s (App Store Connect returns 500 for %s and %s)",
			supportedAppEventPurchaseRequirementValues(),
			string(asc.AppEventPurchaseRequirementNoIAPRequired),
			string(asc.AppEventPurchaseRequirementIAPRequired),
		)
	}
}

func validateAppEventPurchaseRequirement(value string) error {
	switch value {
	case "":
		return nil
	case string(asc.AppEventPurchaseRequirementNoIAPRequired), string(asc.AppEventPurchaseRequirementIAPRequired):
		return fmt.Errorf(
			"--purchase-requirement %s is currently unsupported by App Store Connect API (known 500 UNEXPECTED_ERROR); use %s or omit --purchase-requirement",
			value,
			string(asc.AppEventPurchaseRequirementNoCostAssociated),
		)
	default:
		return nil
	}
}

func normalizeAppEventAssetType(value string) (string, error) {
	normalized := strings.ToUpper(strings.TrimSpace(value))
	if normalized == "" {
		return "", fmt.Errorf("--asset-type is required")
	}
	if slices.Contains(asc.ValidAppEventAssetTypes, normalized) {
		return normalized, nil
	}
	return "", fmt.Errorf("--asset-type must be one of: %s", strings.Join(asc.ValidAppEventAssetTypes, ", "))
}

func normalizeRFC3339(value, flagName string, required bool) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		if required {
			return "", fmt.Errorf("%s is required", flagName)
		}
		return "", nil
	}
	parsed, err := time.Parse(time.RFC3339, trimmed)
	if err != nil {
		return "", fmt.Errorf("%s must be in RFC3339 format", flagName)
	}
	return parsed.Format(time.RFC3339), nil
}

func buildAppEventTerritorySchedule(territories []string, publishStart, start, end string) asc.AppEventTerritorySchedule {
	schedule := asc.AppEventTerritorySchedule{
		EventStart: start,
		EventEnd:   end,
	}
	if len(territories) > 0 {
		schedule.Territories = territories
	}
	if strings.TrimSpace(publishStart) != "" {
		schedule.PublishStart = publishStart
	}
	return schedule
}

func normalizeAppEventTerritorySchedule(start, end, publishStart, territories string) (asc.AppEventTerritorySchedule, bool, error) {
	scheduleProvided := strings.TrimSpace(start) != "" ||
		strings.TrimSpace(end) != "" ||
		strings.TrimSpace(publishStart) != "" ||
		strings.TrimSpace(territories) != ""
	if !scheduleProvided {
		return asc.AppEventTerritorySchedule{}, false, nil
	}

	startValue, err := normalizeRFC3339(start, "--start", true)
	if err != nil {
		return asc.AppEventTerritorySchedule{}, false, err
	}
	endValue, err := normalizeRFC3339(end, "--end", true)
	if err != nil {
		return asc.AppEventTerritorySchedule{}, false, err
	}
	publishValue, err := normalizeRFC3339(publishStart, "--publish-start", false)
	if err != nil {
		return asc.AppEventTerritorySchedule{}, false, err
	}
	territoryValues := shared.SplitCSVUpper(territories)

	return buildAppEventTerritorySchedule(territoryValues, publishValue, startValue, endValue), true, nil
}

func appEventHasTerritorySchedule(event *asc.AppEventResponse, expected asc.AppEventTerritorySchedule) bool {
	if event == nil {
		return false
	}

	expectedTerritories := sortedTerritories(expected.Territories)

	for _, actual := range event.Data.Attributes.TerritorySchedules {
		if equalRFC3339Instant(actual.PublishStart, expected.PublishStart) &&
			equalRFC3339Instant(actual.EventStart, expected.EventStart) &&
			equalRFC3339Instant(actual.EventEnd, expected.EventEnd) &&
			slices.Equal(sortedTerritories(actual.Territories), expectedTerritories) {
			return true
		}
	}

	return false
}

func sortedTerritories(territories []string) []string {
	sorted := slices.Clone(territories)
	slices.Sort(sorted)
	return sorted
}

func equalRFC3339Instant(actual, expected string) bool {
	actual = strings.TrimSpace(actual)
	expected = strings.TrimSpace(expected)
	if actual == "" || expected == "" {
		return actual == expected
	}

	actualTime, actualOK := parseRFC3339Instant(actual)
	expectedTime, expectedOK := parseRFC3339Instant(expected)
	if actualOK && expectedOK {
		return actualTime.Equal(expectedTime)
	}

	return actual == expected
}

func parseRFC3339Instant(value string) (time.Time, bool) {
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}, false
	}
	return parsed, true
}

func resolveAppEventLocalizationID(ctx context.Context, client *asc.Client, eventID, localizationID, locale string) (string, error) {
	localizationID = strings.TrimSpace(localizationID)
	if localizationID != "" {
		return localizationID, nil
	}
	eventID = strings.TrimSpace(eventID)
	if eventID == "" {
		return "", fmt.Errorf("--event-id is required")
	}
	locale = strings.TrimSpace(locale)
	if locale == "" {
		event, err := client.GetAppEvent(ctx, eventID)
		if err != nil {
			return "", err
		}
		locale = strings.TrimSpace(event.Data.Attributes.PrimaryLocale)
	}
	if locale == "" {
		return "", fmt.Errorf("no locale resolved for app event %q (use --locale or --localization-id)", eventID)
	}

	resp, err := client.GetAppEventLocalizations(ctx, eventID, asc.WithAppEventLocalizationsLimit(200))
	if err != nil {
		return "", err
	}
	for _, localization := range resp.Data {
		if strings.EqualFold(localization.Attributes.Locale, locale) {
			return localization.ID, nil
		}
	}
	return "", fmt.Errorf("no localization found for locale %q (use --localization-id to specify)", locale)
}

func openAssetFile(path string) (*os.File, os.FileInfo, error) {
	if err := asc.ValidateAssetFile(path); err != nil {
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

func contextWithAssetUploadTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	return shared.ContextWithResolvedTimeout(ctx, appEventAssetUploadDefaultTimeout)
}
