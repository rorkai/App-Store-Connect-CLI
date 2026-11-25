package publish

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"unicode/utf8"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func missingPublishBuildGroups(ctx context.Context, client *asc.Client, buildID string, groups []shared.ResolvedBetaGroup, uploaded bool) ([]shared.ResolvedBetaGroup, error) {
	missing := make([]shared.ResolvedBetaGroup, 0, len(groups))
	for _, group := range groups {
		if uploaded && group.IsInternalGroup {
			missing = append(missing, group)
			continue
		}
		if err := shared.ValidateBetaGroupRequestID(group.ID); err != nil {
			return nil, err
		}
		first, err := client.GetBetaGroupBuildsRelationships(ctx, group.ID, asc.Limit[asc.LinkagesOption](200))
		if err != nil {
			return nil, fmt.Errorf("read group %s builds: %w", group.ID, err)
		}
		if err := validatePublishBuildMemberships(first); err != nil {
			return nil, err
		}
		all, err := asc.PaginateAll(ctx, first, func(ctx context.Context, next string) (asc.PaginatedResponse, error) {
			boundNext, err := shared.BindCollectionNextURL(ctx, client, next, "/v1/betaGroups/"+group.ID+"/relationships/builds", nil)
			if err != nil {
				return nil, err
			}
			page, err := client.GetBetaGroupBuildsRelationships(ctx, group.ID, asc.NextURL[asc.LinkagesOption](boundNext))
			if err == nil {
				err = validatePublishBuildMemberships(page)
			}
			return page, err
		})
		if err != nil {
			return nil, err
		}
		links, ok := all.(*asc.LinkagesResponse)
		if !ok || links == nil {
			return nil, fmt.Errorf("invalid group builds response")
		}
		found := false
		for _, link := range links.Data {
			if link.ID == buildID {
				found = true
				break
			}
		}
		if !found {
			missing = append(missing, group)
		}
	}
	return missing, nil
}

func publishNotificationRecovery(buildID string) *asc.TestFlightNotificationRecovery {
	return &asc.TestFlightNotificationRecovery{
		BuildID: buildID,
		State:   "unknown",
		Message: "Notification delivery is unknown, including automatic notifications. Check App Store Connect before any fresh build-wide notification request; do not replay an ambiguous send.",
	}
}

func validatePublishBuildMemberships(page *asc.LinkagesResponse) error {
	if page == nil || page.Data == nil {
		return fmt.Errorf("missing beta group builds response data")
	}
	for _, link := range page.Data {
		if link.Type != "builds" || link.ID == "" || link.ID != strings.TrimSpace(link.ID) {
			return fmt.Errorf("invalid beta group build relationship identity")
		}
	}
	return nil
}

func boundedPublishFailure(failure error) string {
	const limit = 2048
	message := shared.SanitizeTerminal(failure.Error())
	if len(message) <= limit {
		return message
	}
	end := limit - len("...")
	for end > 0 && !utf8.RuneStart(message[end]) {
		end--
	}
	return message[:end] + "..."
}

func validatePublishBuildIdentity(build *asc.BuildResponse, expectedID string) error {
	if expectedID == "" || expectedID == "." || expectedID == ".." || url.PathEscape(expectedID) != expectedID || build == nil || build.Data.Type != "builds" || build.Data.ID != expectedID {
		return fmt.Errorf("build response does not match requested build %s", expectedID)
	}
	return nil
}

func writePublishFailure(d *shared.Deps, output string, pretty bool, result *asc.TestFlightPublishResult, failure error) error {
	result.Failure = publishFailureMessage(result.FailureStage)
	if err := d.PrintOutput(result, output, pretty); err != nil {
		return errors.Join(failure, fmt.Errorf("print publish failure result: %w", err))
	}
	reported := errors.New(boundedPublishFailure(failure))
	if result.NotificationRecovery != nil {
		reported = &shared.HintedError{Err: reported, Hint: result.NotificationRecovery.Message}
	}
	return shared.NewReportedError(shared.NewErrorWithCause(reported, failure))
}

func writePublishPreflightFailure(d *shared.Deps, output string, pretty bool, buildID, stage, code string, failure error) error {
	return writePublishFailure(d, output, pretty, &asc.TestFlightPublishResult{
		Status:       "failed",
		BuildID:      buildID,
		FailureStage: stage,
		FailureCode:  code,
	}, failure)
}

func publishFailureMessage(stage string) string {
	switch stage {
	case "app_resolution":
		return "The selected app could not be verified."
	case "group_discovery":
		return "The requested beta groups could not be verified."
	case "build_ownership":
		return "The selected build could not be verified for the selected app."
	case "build_lookup", "build_identity":
		return "The selected build could not be verified."
	case publishFailureStageBuildProcessing:
		return "Build processing could not be verified."
	case publishFailureStageTestNotes:
		return "Test notes could not be saved."
	case publishFailureStageBetaDistribution:
		return "Beta group assignment could not be completed."
	case publishFailureStageBetaReview:
		return "Beta review submission could not be completed."
	case publishFailureStageNotification:
		return "Notification delivery is unknown."
	default:
		return "TestFlight publishing could not be completed."
	}
}
