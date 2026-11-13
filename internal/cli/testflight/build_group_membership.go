package testflight

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

const (
	buildGroupLookupServerFilter         = "server_filter_with_inverse_all_builds"
	buildGroupLookupInverseRelationships = "inverse_relationships"

	buildGroupMembershipExplicit             = "explicit"
	buildGroupMembershipAllBuilds            = "all-builds"
	buildGroupMembershipExplicitAndAllBuilds = "explicit-and-all-builds"
)

var buildGroupSparseFields = []string{"name", "isInternalGroup", "hasAccessToAllBuilds"}

func lookupBuildGroupMembership(
	ctx context.Context,
	client *asc.Client,
	buildID string,
	expectedAppID string,
	internalFilter *bool,
) (*asc.BuildBetaGroupMembershipResult, bool, error) {
	appLinkage, err := client.GetBuildAppRelationship(ctx, buildID)
	if err != nil {
		return nil, false, fmt.Errorf("failed to resolve app for build %q: %w", buildID, err)
	}
	appID := strings.TrimSpace(appLinkage.Data.ID)
	if appID == "" {
		return nil, false, fmt.Errorf("build %q is missing related app ID", buildID)
	}
	if expected := strings.TrimSpace(expectedAppID); expected != "" && expected != appID {
		return nil, false, fmt.Errorf("build %q belongs to app %q, not requested app %q", buildID, appID, expected)
	}

	appOptions := []asc.BetaGroupsOption{
		asc.WithBetaGroupsApps([]string{appID}),
		asc.WithBetaGroupsFields(buildGroupSparseFields),
		asc.Limit[asc.BetaGroupsOption](200),
	}
	if internalFilter != nil {
		appOptions = append(appOptions, asc.WithBetaGroupsIsInternal(*internalFilter))
	}
	appGroups, err := listAllBetaGroups(ctx, client, appOptions...)
	if err != nil {
		return nil, false, fmt.Errorf("failed to list groups for app %q: %w", appID, err)
	}

	groupByID := deduplicateBetaGroups(appGroups.Data)
	explicitGroups, filterErr := listAllBetaGroups(
		ctx,
		client,
		asc.WithBetaGroupsBuilds([]string{buildID}),
		asc.WithBetaGroupsFields(buildGroupSparseFields),
		asc.Limit[asc.BetaGroupsOption](200),
	)
	if filterErr == nil {
		explicitIDs := make(map[string]struct{}, len(explicitGroups.Data))
		for _, group := range explicitGroups.Data {
			groupID := strings.TrimSpace(group.ID)
			if groupID == "" || !matchesInternalFilter(group, internalFilter) {
				continue
			}
			explicitIDs[groupID] = struct{}{}
			if _, ok := groupByID[groupID]; !ok {
				groupByID[groupID] = group
			}
		}

		allBuildGroups := make(map[string]asc.Resource[asc.BetaGroupAttributes])
		for groupID, group := range groupByID {
			if group.Attributes.HasAccessToAllBuilds {
				if _, alreadyExplicit := explicitIDs[groupID]; alreadyExplicit {
					continue
				}
				allBuildGroups[groupID] = group
			}
		}
		allBuildExplicitIDs, failures := lookupBuildGroupMembershipByInverseRelationships(ctx, client, buildID, allBuildGroups)
		for groupID := range allBuildExplicitIDs {
			explicitIDs[groupID] = struct{}{}
		}
		result := buildGroupMembershipResult(buildID, appID, groupByID, explicitIDs, failures, buildGroupLookupServerFilter)
		if len(failures) > 0 {
			return result, false, fmt.Errorf("membership lookup incomplete: %d all-build group relationship lookup failed", len(failures))
		}
		return result, false, nil
	}
	if !isHTTPBadRequest(filterErr) {
		return nil, false, fmt.Errorf("failed to filter groups for build %q: %w", buildID, filterErr)
	}

	explicitIDs, failures := lookupBuildGroupMembershipByInverseRelationships(ctx, client, buildID, groupByID)
	result := buildGroupMembershipResult(buildID, appID, groupByID, explicitIDs, failures, buildGroupLookupInverseRelationships)
	if len(failures) > 0 {
		return result, true, fmt.Errorf("membership lookup incomplete: %d group relationship lookup failed", len(failures))
	}
	return result, true, nil
}

func listAllBetaGroups(ctx context.Context, client *asc.Client, opts ...asc.BetaGroupsOption) (*asc.BetaGroupsResponse, error) {
	firstPage, err := client.ListBetaGroups(ctx, opts...)
	if err != nil {
		return nil, err
	}
	all, err := asc.PaginateAll(ctx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
		return client.ListBetaGroups(ctx, asc.NextURL[asc.BetaGroupsOption](nextURL))
	})
	if err != nil {
		return nil, err
	}
	groups, ok := all.(*asc.BetaGroupsResponse)
	if !ok {
		return nil, fmt.Errorf("unexpected beta groups response type %T", all)
	}
	return groups, nil
}

func deduplicateBetaGroups(groups []asc.Resource[asc.BetaGroupAttributes]) map[string]asc.Resource[asc.BetaGroupAttributes] {
	result := make(map[string]asc.Resource[asc.BetaGroupAttributes], len(groups))
	for _, group := range groups {
		groupID := strings.TrimSpace(group.ID)
		if groupID == "" {
			continue
		}
		if _, exists := result[groupID]; !exists {
			result[groupID] = group
		}
	}
	return result
}

func matchesInternalFilter(group asc.Resource[asc.BetaGroupAttributes], internalFilter *bool) bool {
	return internalFilter == nil || group.Attributes.IsInternalGroup == *internalFilter
}

func lookupBuildGroupMembershipByInverseRelationships(
	ctx context.Context,
	client *asc.Client,
	buildID string,
	groups map[string]asc.Resource[asc.BetaGroupAttributes],
) (map[string]struct{}, []asc.BuildBetaGroupMembershipFailure) {
	groupIDs := make([]string, 0, len(groups))
	for groupID := range groups {
		groupIDs = append(groupIDs, groupID)
	}
	sort.Strings(groupIDs)

	explicitIDs := make(map[string]struct{})
	failures := make([]asc.BuildBetaGroupMembershipFailure, 0)
	for _, groupID := range groupIDs {
		contains, err := betaGroupContainsBuild(ctx, client, groupID, buildID)
		if err != nil {
			failures = append(failures, asc.BuildBetaGroupMembershipFailure{
				GroupID:   groupID,
				GroupName: groups[groupID].Attributes.Name,
				Error:     err.Error(),
			})
			continue
		}
		if contains {
			explicitIDs[groupID] = struct{}{}
		}
	}
	return explicitIDs, failures
}

func betaGroupContainsBuild(ctx context.Context, client *asc.Client, groupID, buildID string) (bool, error) {
	page, err := client.GetBetaGroupBuildsRelationships(ctx, groupID, asc.WithLinkagesLimit(200))
	if err != nil {
		return false, err
	}
	seenNext := make(map[string]struct{})
	pageNumber := 1
	for {
		for _, linkage := range page.Data {
			if strings.TrimSpace(linkage.ID) == buildID {
				return true, nil
			}
		}
		nextURL := strings.TrimSpace(page.Links.Next)
		if nextURL == "" {
			return false, nil
		}
		if _, seen := seenNext[nextURL]; seen {
			return false, fmt.Errorf("page %d: %w", pageNumber+1, asc.ErrRepeatedPaginationURL)
		}
		seenNext[nextURL] = struct{}{}
		pageNumber++
		page, err = client.GetBetaGroupBuildsRelationships(ctx, groupID, asc.WithLinkagesNextURL(nextURL))
		if err != nil {
			return false, fmt.Errorf("page %d: %w", pageNumber, err)
		}
	}
}

func buildGroupMembershipResult(
	buildID string,
	appID string,
	groups map[string]asc.Resource[asc.BetaGroupAttributes],
	explicitIDs map[string]struct{},
	failures []asc.BuildBetaGroupMembershipFailure,
	lookupMethod string,
) *asc.BuildBetaGroupMembershipResult {
	memberships := make([]asc.BuildBetaGroupMembershipGroup, 0, len(groups))
	for groupID, group := range groups {
		_, explicit := explicitIDs[groupID]
		allBuilds := group.Attributes.HasAccessToAllBuilds
		membership := ""
		switch {
		case explicit && allBuilds:
			membership = buildGroupMembershipExplicitAndAllBuilds
		case explicit:
			membership = buildGroupMembershipExplicit
		case allBuilds:
			membership = buildGroupMembershipAllBuilds
		default:
			continue
		}

		groupType := "external"
		if group.Attributes.IsInternalGroup {
			groupType = "internal"
		}
		memberships = append(memberships, asc.BuildBetaGroupMembershipGroup{
			ID:                   groupID,
			Name:                 group.Attributes.Name,
			Type:                 groupType,
			Membership:           membership,
			HasAccessToAllBuilds: allBuilds,
		})
	}
	sort.Slice(memberships, func(i, j int) bool {
		leftName := strings.ToLower(strings.TrimSpace(memberships[i].Name))
		rightName := strings.ToLower(strings.TrimSpace(memberships[j].Name))
		if leftName == rightName {
			return memberships[i].ID < memberships[j].ID
		}
		return leftName < rightName
	})
	if failures == nil {
		failures = []asc.BuildBetaGroupMembershipFailure{}
	}
	return &asc.BuildBetaGroupMembershipResult{
		BuildID:      buildID,
		AppID:        appID,
		Complete:     len(failures) == 0,
		LookupMethod: lookupMethod,
		GroupCount:   len(memberships),
		Groups:       memberships,
		Failures:     failures,
	}
}

func isHTTPBadRequest(err error) bool {
	var apiErr *asc.APIError
	return errors.As(err, &apiErr) && apiErr.HTTPStatusCode() == 400
}
