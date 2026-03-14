package publish

import (
	"context"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func resolvePublishBetaGroups(ctx context.Context, client *asc.Client, appID string, groups []string) ([]shared.ResolvedBetaGroup, error) {
	return shared.ResolveBetaGroups(ctx, client, appID, groups, shared.ResolveBetaGroupsOptions{})
}

func listAllPublishBetaGroups(ctx context.Context, client *asc.Client, appID string) (*asc.BetaGroupsResponse, error) {
	return shared.ListAllBetaGroups(ctx, client, appID)
}

func resolvePublishBetaGroupIDsFromList(inputGroups []string, groups *asc.BetaGroupsResponse) ([]string, error) {
	resolvedGroups, err := shared.ResolveBetaGroupsFromList(inputGroups, groups, shared.ResolveBetaGroupsOptions{})
	if err != nil {
		return nil, err
	}
	return resolvedPublishBetaGroupIDs(resolvedGroups), nil
}

func resolvedPublishBetaGroupIDs(groups []shared.ResolvedBetaGroup) []string {
	ids := make([]string, 0, len(groups))
	for _, group := range groups {
		ids = append(ids, group.ID)
	}
	return ids
}
