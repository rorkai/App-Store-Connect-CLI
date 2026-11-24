package publish

import (
	"context"
	"fmt"

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
		first, err := client.GetBetaGroupBuildsRelationships(ctx, group.ID, asc.Limit[asc.LinkagesOption](200))
		if err != nil {
			return nil, fmt.Errorf("read group %s builds: %w", group.ID, err)
		}
		all, err := asc.PaginateAll(ctx, first, func(ctx context.Context, next string) (asc.PaginatedResponse, error) {
			return client.GetBetaGroupBuildsRelationships(ctx, group.ID, asc.NextURL[asc.LinkagesOption](next))
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

func publishNotificationRecovery(buildID, state string) *asc.TestFlightNotificationRecovery {
	message := "Existing memberships do not establish whether testers were notified. Check notification delivery before sending; do not replay an ambiguous send."
	if state == "not_attempted" {
		message = "Group assignment succeeded, but checking notification settings failed before a notification was sent. Check those settings before sending."
	}
	return &asc.TestFlightNotificationRecovery{
		BuildID:   buildID,
		State:     state,
		Message:   message,
		Command:   "asc",
		Arguments: []string{"testflight", "notifications", "send", "--build-id", buildID},
	}
}

func validatePublishBuildIdentity(build *asc.BuildResponse, expectedID string) error {
	if expectedID == "" || build == nil || build.Data.Type != "builds" || build.Data.ID != expectedID {
		return fmt.Errorf("build response does not match requested build %s", expectedID)
	}
	return nil
}
