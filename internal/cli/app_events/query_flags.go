package app_events

import (
	"flag"
	"fmt"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func appEventFlagWasProvided(fs *flag.FlagSet, name string) bool {
	provided := false
	fs.Visit(func(f *flag.Flag) {
		provided = provided || f.Name == name
	})
	return provided
}

func validateAppEventCSV(value, flagName string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("%s must not be empty", flagName)
	}
	for _, item := range strings.Split(value, ",") {
		if strings.TrimSpace(item) == "" {
			return fmt.Errorf("%s must not contain empty values", flagName)
		}
	}
	return nil
}

func normalizeAppEventStates(value string) ([]string, error) {
	values := shared.SplitCSVUpper(value)
	if len(values) == 0 {
		return nil, nil
	}
	for _, value := range values {
		if !containsString(appEventStateList(), value) {
			return nil, fmt.Errorf("--event-state must be one of: %s", strings.Join(appEventStateList(), ", "))
		}
	}
	return values, nil
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func appEventStateList() []string {
	return []string{
		"DRAFT",
		"READY_FOR_REVIEW",
		"WAITING_FOR_REVIEW",
		"IN_REVIEW",
		"REJECTED",
		"ACCEPTED",
		"APPROVED",
		"PUBLISHED",
		"PAST",
		"ARCHIVED",
	}
}

func appEventFieldsList() []string {
	return []string{
		"referenceName",
		"badge",
		"eventState",
		"deepLink",
		"purchaseRequirement",
		"primaryLocale",
		"priority",
		"purpose",
		"territorySchedules",
		"archivedTerritorySchedules",
		"localizations",
	}
}

func appEventLocalizationFieldsList() []string {
	return []string{
		"locale",
		"name",
		"shortDescription",
		"longDescription",
		"appEvent",
		"appEventScreenshots",
		"appEventVideoClips",
		"placements",
	}
}

func appEventIncludeList() []string {
	return []string{"localizations"}
}

func appEventLocalizationIncludeList() []string {
	return []string{"appEvent", "appEventScreenshots", "appEventVideoClips", "placements"}
}
