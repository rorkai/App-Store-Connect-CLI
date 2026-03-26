package shared

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

type betaGroupsClient interface {
	GetBetaGroups(ctx context.Context, appID string, opts ...asc.BetaGroupsOption) (*asc.BetaGroupsResponse, error)
}

type buildBetaGroupsMutationClient interface {
	AddBetaGroupsToBuildWithNotify(ctx context.Context, buildID string, groupIDs []string, notify bool) (asc.BuildBetaGroupsNotificationAction, error)
}

// ResolvedBetaGroup captures the canonical ID and metadata for a beta group.
type ResolvedBetaGroup struct {
	ID                   string
	Name                 string
	IsInternalGroup      bool
	HasAccessToAllBuilds bool
}

func (g ResolvedBetaGroup) NameForDisplay() string {
	name := strings.TrimSpace(g.Name)
	if name != "" {
		return name
	}
	return g.ID
}

// ResolveBetaGroupsOptions controls beta-group name resolution behavior.
type ResolveBetaGroupsOptions struct {
	SkipInternal            bool
	IncludeSkipInternalHint bool
}

// AddBuildBetaGroupsOptions controls how resolved groups are assigned to a build.
type AddBuildBetaGroupsOptions struct {
	SkipInternal              bool
	SkipInternalWithAllBuilds bool
	Notify                    bool
}

// AddBuildBetaGroupsResult reports the final add/skipped group sets.
type AddBuildBetaGroupsResult struct {
	AddedGroupIDs                  []string
	SkippedInternalGroups          []ResolvedBetaGroup
	SkippedInternalAllBuildsGroups []ResolvedBetaGroup
	NotificationAction             asc.BuildBetaGroupsNotificationAction
}

// ResolveBetaGroups lists an app's beta groups and resolves the provided IDs or names.
func ResolveBetaGroups(ctx context.Context, client betaGroupsClient, appID string, groups []string, opts ResolveBetaGroupsOptions) ([]ResolvedBetaGroup, error) {
	allGroups, err := ListAllBetaGroups(ctx, client, appID)
	if err != nil {
		return nil, fmt.Errorf("failed to list beta groups: %w", err)
	}
	return ResolveBetaGroupsFromList(groups, allGroups, opts)
}

// ResolveBetaGroupsFromList resolves IDs or names against a provided beta-groups payload.
func ResolveBetaGroupsFromList(inputGroups []string, groups *asc.BetaGroupsResponse, opts ResolveBetaGroupsOptions) ([]ResolvedBetaGroup, error) {
	if groups == nil {
		return nil, fmt.Errorf("no beta groups returned for app")
	}

	groupIDs := make(map[string]struct{}, len(groups.Data))
	groupNameToIDs := make(map[string][]string)
	groupInternal := make(map[string]bool, len(groups.Data))
	groupsByID := make(map[string]asc.Resource[asc.BetaGroupAttributes], len(groups.Data))
	for _, item := range groups.Data {
		id := strings.TrimSpace(item.ID)
		if id == "" {
			continue
		}
		groupIDs[id] = struct{}{}
		groupInternal[id] = item.Attributes.IsInternalGroup
		groupsByID[id] = item

		name := strings.TrimSpace(item.Attributes.Name)
		if name == "" {
			continue
		}
		key := strings.ToLower(name)
		if !slices.Contains(groupNameToIDs[key], id) {
			groupNameToIDs[key] = append(groupNameToIDs[key], id)
		}
	}

	resolvedIDs := make([]string, 0, len(inputGroups))
	seen := make(map[string]struct{}, len(inputGroups))
	for _, raw := range inputGroups {
		group := strings.TrimSpace(raw)
		if group == "" {
			continue
		}

		resolvedID := ""
		if _, ok := groupIDs[group]; ok {
			resolvedID = group
		} else {
			matches := groupNameToIDs[strings.ToLower(group)]
			switch len(matches) {
			case 0:
				return nil, fmt.Errorf("beta group %q not found", group)
			case 1:
				resolvedID = matches[0]
			default:
				externalMatches := filterExternalGroupIDs(matches, groupInternal)
				if opts.SkipInternal && len(externalMatches) == 1 {
					resolvedID = externalMatches[0]
					break
				}

				hint := "Use the group ID to disambiguate."
				if opts.IncludeSkipInternalHint && !opts.SkipInternal && len(externalMatches) == 1 && len(externalMatches) < len(matches) {
					hint = "Use the group ID to disambiguate, or --skip-internal to exclude internal groups."
				}
				return nil, fmt.Errorf("%s\n%s", formatAmbiguousBetaGroupError(group, matches, groupInternal), hint)
			}
		}

		if _, ok := seen[resolvedID]; ok {
			continue
		}
		seen[resolvedID] = struct{}{}
		resolvedIDs = append(resolvedIDs, resolvedID)
	}

	if len(resolvedIDs) == 0 {
		return nil, fmt.Errorf("at least one beta group is required")
	}

	resolvedGroups := make([]ResolvedBetaGroup, 0, len(resolvedIDs))
	for _, resolvedID := range resolvedIDs {
		group, ok := groupsByID[resolvedID]
		if !ok {
			return nil, fmt.Errorf("resolved beta group %q not found in app group list", resolvedID)
		}
		resolvedGroups = append(resolvedGroups, ResolvedBetaGroup{
			ID:                   resolvedID,
			Name:                 strings.TrimSpace(group.Attributes.Name),
			IsInternalGroup:      group.Attributes.IsInternalGroup,
			HasAccessToAllBuilds: group.Attributes.HasAccessToAllBuilds,
		})
	}

	return resolvedGroups, nil
}

// AddBuildBetaGroups applies resolved beta groups to a build, optionally skipping internal groups.
func AddBuildBetaGroups(ctx context.Context, client buildBetaGroupsMutationClient, buildID string, groups []ResolvedBetaGroup, opts AddBuildBetaGroupsOptions) (*AddBuildBetaGroupsResult, error) {
	groupIDsToAdd := make([]string, 0, len(groups))
	skippedInternalGroups := make([]ResolvedBetaGroup, 0, len(groups))
	skippedInternalAllBuildsGroups := make([]ResolvedBetaGroup, 0, len(groups))
	for _, group := range groups {
		if group.IsInternalGroup && opts.SkipInternal {
			skippedInternalGroups = append(skippedInternalGroups, group)
			continue
		}
		if group.IsInternalGroup && group.HasAccessToAllBuilds && opts.SkipInternalWithAllBuilds {
			skippedInternalAllBuildsGroups = append(skippedInternalAllBuildsGroups, group)
			continue
		}
		groupIDsToAdd = append(groupIDsToAdd, group.ID)
	}

	if len(groupIDsToAdd) == 0 {
		return &AddBuildBetaGroupsResult{
			AddedGroupIDs:                  []string{},
			SkippedInternalGroups:          skippedInternalGroups,
			SkippedInternalAllBuildsGroups: skippedInternalAllBuildsGroups,
			NotificationAction:             asc.BuildBetaGroupsNotificationActionNone,
		}, nil
	}

	notificationAction, err := client.AddBetaGroupsToBuildWithNotify(ctx, buildID, groupIDsToAdd, opts.Notify)
	if err != nil {
		return nil, err
	}

	return &AddBuildBetaGroupsResult{
		AddedGroupIDs:                  groupIDsToAdd,
		SkippedInternalGroups:          skippedInternalGroups,
		SkippedInternalAllBuildsGroups: skippedInternalAllBuildsGroups,
		NotificationAction:             notificationAction,
	}, nil
}

// ListAllBetaGroups fetches and paginates all beta groups for an app.
func ListAllBetaGroups(ctx context.Context, client betaGroupsClient, appID string) (*asc.BetaGroupsResponse, error) {
	firstPage, err := client.GetBetaGroups(ctx, appID, asc.WithBetaGroupsLimit(200))
	if err != nil {
		return nil, err
	}
	if firstPage == nil || firstPage.Links.Next == "" {
		return firstPage, nil
	}

	paginated, err := asc.PaginateAll(ctx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
		return client.GetBetaGroups(ctx, appID, asc.WithBetaGroupsNextURL(nextURL))
	})
	if err != nil {
		return nil, err
	}

	allGroups, ok := paginated.(*asc.BetaGroupsResponse)
	if !ok {
		return nil, fmt.Errorf("unexpected beta groups pagination type %T", paginated)
	}
	return allGroups, nil
}

func filterExternalGroupIDs(matchIDs []string, internalByID map[string]bool) []string {
	external := make([]string, 0, len(matchIDs))
	for _, id := range matchIDs {
		if !internalByID[id] {
			external = append(external, id)
		}
	}
	return external
}

func formatAmbiguousBetaGroupError(name string, matchIDs []string, internalByID map[string]bool) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%q matches %d beta groups:", name, len(matchIDs))
	for _, id := range matchIDs {
		kind := "external"
		if internalByID[id] {
			kind = "internal"
		}
		fmt.Fprintf(&b, "\n  %s (%s)", id, kind)
	}
	return b.String()
}
