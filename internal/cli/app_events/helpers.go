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

func normalizeAppEventBadge(value string) (string, error) {
	normalized := strings.ToUpper(strings.TrimSpace(value))
	if normalized == "" {
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

// App Store Connect's limits on an event schedule: an event lasts from 15
// minutes to 31 days, and is published at most 14 days before it starts.
const (
	appEventMinDuration        = 15 * time.Minute
	appEventMaxDuration        = 31 * 24 * time.Hour
	appEventMaxPublishLeadTime = 14 * 24 * time.Hour
)

// validateAppEventScheduleWindow checks normalized RFC3339 schedule values
// against App Store Connect's schedule limits, so a schedule it would reject
// fails before any request.
func validateAppEventScheduleWindow(start, end, publishStart string) error {
	startTime, err := time.Parse(time.RFC3339, start)
	if err != nil {
		return fmt.Errorf("--start must be in RFC3339 format")
	}
	endTime, err := time.Parse(time.RFC3339, end)
	if err != nil {
		return fmt.Errorf("--end must be in RFC3339 format")
	}
	if endTime.Sub(startTime) < appEventMinDuration {
		return fmt.Errorf("--end must be at least 15 minutes after --start")
	}
	if endTime.Sub(startTime) > appEventMaxDuration {
		return fmt.Errorf("--end must be at most 31 days after --start (latest: %s)", startTime.Add(appEventMaxDuration).Format(time.RFC3339))
	}
	if publishStart == "" {
		return nil
	}
	publishTime, err := time.Parse(time.RFC3339, publishStart)
	if err != nil {
		return fmt.Errorf("--publish-start must be in RFC3339 format")
	}
	if startTime.Sub(publishTime) > appEventMaxPublishLeadTime {
		return fmt.Errorf("--publish-start must be at most 14 days before --start (earliest: %s)", startTime.Add(-appEventMaxPublishLeadTime).Format(time.RFC3339))
	}
	if publishTime.After(endTime) {
		return fmt.Errorf("--publish-start must not be after --end")
	}
	return nil
}

// appEventSchedule is the schedule given by the event schedule flags.
// allTerritories means --territories all, which create resolves from the
// app's availability before it sends the schedule.
type appEventSchedule struct {
	schedule       asc.AppEventTerritorySchedule
	provided       bool
	allTerritories bool
}

func normalizeAppEventTerritorySchedule(start, end, publishStart, territories string, allowAllTerritories bool) (appEventSchedule, error) {
	scheduleProvided := strings.TrimSpace(start) != "" ||
		strings.TrimSpace(end) != "" ||
		strings.TrimSpace(publishStart) != "" ||
		strings.TrimSpace(territories) != ""
	if !scheduleProvided {
		return appEventSchedule{}, nil
	}

	startValue, err := normalizeRFC3339(start, "--start", true)
	if err != nil {
		return appEventSchedule{}, err
	}
	endValue, err := normalizeRFC3339(end, "--end", true)
	if err != nil {
		return appEventSchedule{}, err
	}
	publishValue, err := normalizeRFC3339(publishStart, "--publish-start", false)
	if err != nil {
		return appEventSchedule{}, err
	}

	if err := validateAppEventScheduleWindow(startValue, endValue, publishValue); err != nil {
		return appEventSchedule{}, err
	}

	territoryValues := shared.SplitCSVUpper(territories)
	allTerritories := slices.Contains(territoryValues, "ALL")
	switch {
	case len(territoryValues) == 0 && allowAllTerritories:
		return appEventSchedule{}, fmt.Errorf("--territories is required with a schedule: pass territory codes such as USA,GBR, or all for every territory the app is available in")
	case len(territoryValues) == 0:
		return appEventSchedule{}, fmt.Errorf("--territories is required with a schedule: pass territory codes such as USA,GBR")
	case allTerritories && !allowAllTerritories:
		return appEventSchedule{}, fmt.Errorf("--territories all is supported only by app-events create; pass territory codes such as USA,GBR")
	case allTerritories && len(territoryValues) > 1:
		return appEventSchedule{}, fmt.Errorf("--territories all cannot be combined with territory codes")
	case allTerritories:
		territoryValues = nil
	}

	return appEventSchedule{
		schedule:       buildAppEventTerritorySchedule(territoryValues, publishValue, startValue, endValue),
		provided:       true,
		allTerritories: allTerritories,
	}, nil
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

func resolveAppEventLocalizationID(ctx context.Context, d *shared.Deps, client *asc.Client, eventID, localizationID, locale string) (string, error) {
	localizationID = strings.TrimSpace(localizationID)
	if localizationID != "" {
		return localizationID, nil
	}
	eventID = strings.TrimSpace(eventID)
	if eventID == "" {
		return "", d.UsageError("--event-id is required")
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
		return "", d.UsageErrorf("no locale resolved for app event %q (use --locale or --localization-id)", eventID)
	}

	resp, err := client.GetAppEventLocalizations(ctx, eventID, asc.Limit[asc.AppEventLocalizationsOption](200))
	if err != nil {
		return "", err
	}
	for _, localization := range resp.Data {
		if strings.EqualFold(localization.Attributes.Locale, locale) {
			return localization.ID, nil
		}
	}
	return "", d.UsageErrorf("no localization found for locale %q (use --localization-id to specify)", locale)
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
