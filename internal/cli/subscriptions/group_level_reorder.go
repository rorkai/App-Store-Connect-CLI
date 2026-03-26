package subscriptions

import (
	"context"
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

type subscriptionGroupLevelPlan struct {
	DesiredOrder    []string
	TargetLevels    map[string]int
	SourceFromLevel int
	SourceToLevel   int
	ChangedLevelIDs []string
}

func executeSubscriptionUpdateWithPeer(
	ctx context.Context,
	client *asc.Client,
	subscriptionID string,
	peerID string,
	sourceAttrs asc.SubscriptionUpdateAttributes,
) (*asc.SubscriptionResponse, error) {
	sourceGroupID, err := getSubscriptionForGroupLevelCopy(ctx, client, subscriptionID)
	if err != nil {
		return nil, fmt.Errorf("resolve subscription %q: %w", subscriptionID, err)
	}

	peerGroupID, err := getSubscriptionForGroupLevelCopy(ctx, client, peerID)
	if err != nil {
		return nil, fmt.Errorf("resolve --with subscription %q: %w", peerID, err)
	}
	if sourceGroupID != peerGroupID {
		return nil, fmt.Errorf("--with subscription %q must belong to the same subscription group as --id %q", peerID, subscriptionID)
	}

	siblings, err := listAllSubscriptionsForGroup(ctx, client, sourceGroupID)
	if err != nil {
		return nil, fmt.Errorf("list subscription group siblings: %w", err)
	}

	plan, err := planSubscriptionMoveToPeerSlot(siblings, subscriptionID, peerID)
	if err != nil {
		return nil, err
	}

	resp, err := applySubscriptionGroupLevelPlan(ctx, client, siblings, plan, subscriptionID, sourceAttrs)
	if err != nil {
		return nil, err
	}

	verifiedSiblings, err := listAllSubscriptionsForGroup(ctx, client, sourceGroupID)
	if err != nil {
		return nil, fmt.Errorf("verify subscription order: %w", err)
	}
	if err := verifySubscriptionGroupLevelPlan(verifiedSiblings, plan); err != nil {
		return nil, fmt.Errorf("verify subscription order: %w", err)
	}

	if resp != nil {
		return resp, nil
	}

	requestCtx, cancel := shared.ContextWithTimeout(ctx)
	defer cancel()

	return client.GetSubscription(requestCtx, subscriptionID)
}

func listAllSubscriptionsForGroup(
	ctx context.Context,
	client *asc.Client,
	groupID string,
) ([]asc.Resource[asc.SubscriptionAttributes], error) {
	requestCtx, cancel := shared.ContextWithTimeout(ctx)
	defer cancel()

	firstPage, err := client.GetSubscriptions(requestCtx, groupID, asc.WithSubscriptionsLimit(200))
	if err != nil {
		return nil, err
	}

	if strings.TrimSpace(firstPage.Links.Next) == "" {
		sortSubscriptionsByGroupLevel(firstPage.Data)
		return firstPage.Data, nil
	}

	allPages, err := asc.PaginateAll(ctx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
		pageCtx, cancel := shared.ContextWithTimeout(ctx)
		defer cancel()

		return client.GetSubscriptions(pageCtx, groupID, asc.WithSubscriptionsNextURL(nextURL))
	})
	if err != nil {
		return nil, err
	}

	resp, ok := allPages.(*asc.SubscriptionsResponse)
	if !ok {
		return nil, fmt.Errorf("unexpected pagination response type %T", allPages)
	}

	sortSubscriptionsByGroupLevel(resp.Data)
	return resp.Data, nil
}

func updateSubscriptionGroupLevel(
	ctx context.Context,
	client *asc.Client,
	subID string,
	groupLevel int,
) error {
	requestCtx, cancel := shared.ContextWithTimeout(ctx)
	defer cancel()

	attrs := asc.SubscriptionUpdateAttributes{
		GroupLevel: &groupLevel,
	}
	_, err := client.UpdateSubscription(requestCtx, subID, attrs)
	return err
}

func sortSubscriptionsByGroupLevel(subscriptions []asc.Resource[asc.SubscriptionAttributes]) {
	sort.SliceStable(subscriptions, func(i, j int) bool {
		return subscriptions[i].Attributes.GroupLevel < subscriptions[j].Attributes.GroupLevel
	})
}

func orderedSubscriptionIDs(subscriptions []asc.Resource[asc.SubscriptionAttributes]) []string {
	ordered := make([]string, 0, len(subscriptions))
	for _, subscription := range subscriptions {
		ordered = append(ordered, subscription.ID)
	}
	return ordered
}

func planSubscriptionMoveToPeerSlot(
	subscriptions []asc.Resource[asc.SubscriptionAttributes],
	sourceID string,
	peerID string,
) (*subscriptionGroupLevelPlan, error) {
	currentOrder := orderedSubscriptionIDs(subscriptions)

	sourceIndex := slices.Index(currentOrder, sourceID)
	if sourceIndex < 0 {
		return nil, fmt.Errorf("subscription %q not found in the subscription group", sourceID)
	}

	peerIndex := slices.Index(currentOrder, peerID)
	if peerIndex < 0 {
		return nil, fmt.Errorf("subscription %q not found in the subscription group", peerID)
	}

	withoutSource := make([]string, 0, len(currentOrder)-1)
	for _, id := range currentOrder {
		if id != sourceID {
			withoutSource = append(withoutSource, id)
		}
	}

	desired := make([]string, 0, len(currentOrder))
	desired = append(desired, withoutSource[:peerIndex]...)
	desired = append(desired, sourceID)
	desired = append(desired, withoutSource[peerIndex:]...)

	targetLevels := make(map[string]int, len(desired))
	for idx, id := range desired {
		targetLevels[id] = idx + 1
	}

	changedLevelIDs := make([]string, 0, len(subscriptions))
	sourceFromLevel := 0
	for _, subscription := range subscriptions {
		if subscription.ID == sourceID {
			sourceFromLevel = subscription.Attributes.GroupLevel
		}
		if subscription.Attributes.GroupLevel != targetLevels[subscription.ID] {
			changedLevelIDs = append(changedLevelIDs, subscription.ID)
		}
	}

	return &subscriptionGroupLevelPlan{
		DesiredOrder:    desired,
		TargetLevels:    targetLevels,
		SourceFromLevel: sourceFromLevel,
		SourceToLevel:   targetLevels[sourceID],
		ChangedLevelIDs: changedLevelIDs,
	}, nil
}

func applySubscriptionGroupLevelPlan(
	ctx context.Context,
	client *asc.Client,
	subscriptions []asc.Resource[asc.SubscriptionAttributes],
	plan *subscriptionGroupLevelPlan,
	sourceID string,
	sourceAttrs asc.SubscriptionUpdateAttributes,
) (*asc.SubscriptionResponse, error) {
	changedLevelIDs := make(map[string]struct{}, len(plan.ChangedLevelIDs))
	for _, id := range plan.ChangedLevelIDs {
		changedLevelIDs[id] = struct{}{}
	}

	tempLevel := maxSubscriptionGroupLevel(subscriptions)
	for _, subscription := range subscriptions {
		if _, ok := changedLevelIDs[subscription.ID]; !ok {
			continue
		}
		tempLevel++
		if err := updateSubscriptionGroupLevel(ctx, client, subscription.ID, tempLevel); err != nil {
			return nil, fmt.Errorf("stage subscription %q for reorder: %w", subscription.ID, err)
		}
	}

	for _, subID := range plan.DesiredOrder {
		if subID == sourceID {
			continue
		}
		if _, ok := changedLevelIDs[subID]; !ok {
			continue
		}
		if err := updateSubscriptionGroupLevel(ctx, client, subID, plan.TargetLevels[subID]); err != nil {
			return nil, fmt.Errorf("set subscription %q to group level %d: %w", subID, plan.TargetLevels[subID], err)
		}
	}

	sourceNeedsUpdate := hasNonGroupLevelChanges(sourceAttrs)
	if _, ok := changedLevelIDs[sourceID]; ok {
		sourceNeedsUpdate = true
	}
	if !sourceNeedsUpdate {
		return nil, nil
	}

	finalAttrs := sourceAttrs
	if _, ok := changedLevelIDs[sourceID]; ok {
		level := plan.TargetLevels[sourceID]
		finalAttrs.GroupLevel = &level
	}

	requestCtx, cancel := shared.ContextWithTimeout(ctx)
	defer cancel()

	resp, err := client.UpdateSubscription(requestCtx, sourceID, finalAttrs)
	if err != nil {
		return nil, fmt.Errorf("set subscription %q to group level %d: %w", sourceID, plan.TargetLevels[sourceID], err)
	}
	return resp, nil
}

func verifySubscriptionGroupLevelPlan(
	subscriptions []asc.Resource[asc.SubscriptionAttributes],
	plan *subscriptionGroupLevelPlan,
) error {
	sortSubscriptionsByGroupLevel(subscriptions)

	gotOrder := orderedSubscriptionIDs(subscriptions)
	if !slices.Equal(gotOrder, plan.DesiredOrder) {
		return fmt.Errorf(
			"expected order [%s], got [%s]",
			strings.Join(plan.DesiredOrder, ", "),
			strings.Join(gotOrder, ", "),
		)
	}

	for _, subscription := range subscriptions {
		wantLevel := plan.TargetLevels[subscription.ID]
		if subscription.Attributes.GroupLevel != wantLevel {
			return fmt.Errorf(
				`subscription %q expected groupLevel=%d, got %d`,
				subscription.ID,
				wantLevel,
				subscription.Attributes.GroupLevel,
			)
		}
	}

	return nil
}

func hasNonGroupLevelChanges(attrs asc.SubscriptionUpdateAttributes) bool {
	return attrs.Name != nil ||
		attrs.ReviewNote != nil ||
		attrs.FamilySharable != nil ||
		attrs.SubscriptionPeriod != nil ||
		attrs.AvailableInAllTerritories != nil
}

func maxSubscriptionGroupLevel(subscriptions []asc.Resource[asc.SubscriptionAttributes]) int {
	maxLevel := len(subscriptions)
	for _, subscription := range subscriptions {
		if subscription.Attributes.GroupLevel > maxLevel {
			maxLevel = subscription.Attributes.GroupLevel
		}
	}
	return maxLevel
}
