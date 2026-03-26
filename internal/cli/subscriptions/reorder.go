package subscriptions

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"sort"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

type subscriptionReorderPlacement string

const (
	subscriptionReorderPlacementBefore subscriptionReorderPlacement = "before"
	subscriptionReorderPlacementAfter  subscriptionReorderPlacement = "after"
	subscriptionReorderPlacementTop    subscriptionReorderPlacement = "top"
	subscriptionReorderPlacementBottom subscriptionReorderPlacement = "bottom"
)

type subscriptionReorderPlan struct {
	DesiredOrder   []string
	FromGroupLevel int
	ToGroupLevel   int
	Changed        bool
}

type subscriptionsReorderResult struct {
	SubscriptionID       string `json:"subscriptionId"`
	GroupID              string `json:"groupId"`
	Placement            string `json:"placement"`
	AnchorSubscriptionID string `json:"anchorSubscriptionId,omitempty"`
	FromGroupLevel       int    `json:"fromGroupLevel"`
	ToGroupLevel         int    `json:"toGroupLevel"`
	Changed              bool   `json:"changed"`
	Verified             bool   `json:"verified"`
}

type subscriptionResourceRelationships struct {
	Group *asc.Relationship `json:"group"`
}

// SubscriptionsReorderCommand returns the subscriptions reorder subcommand.
func SubscriptionsReorderCommand() *ffcli.Command {
	fs := flag.NewFlagSet("reorder", flag.ExitOnError)

	subID := fs.String("id", "", "Subscription ID to reorder")
	beforeID := fs.String("before", "", "Move before another subscription ID")
	afterID := fs.String("after", "", "Move after another subscription ID")
	top := fs.Bool("top", false, "Move to the top of the group")
	bottom := fs.Bool("bottom", false, "Move to the bottom of the group")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "reorder",
		ShortUsage: "asc subscriptions reorder --id \"SUB_ID\" [--before \"SUB_ID\" | --after \"SUB_ID\" | --top | --bottom] [flags]",
		ShortHelp:  "Reorder a subscription within its group.",
		LongHelp: `Reorder a subscription within its group.

Exactly one of --before, --after, --top, or --bottom is required.

Examples:
  asc subscriptions reorder --id "SUB_ID" --before "OTHER_SUB_ID"
  asc subscriptions reorder --id "SUB_ID" --after "OTHER_SUB_ID"
  asc subscriptions reorder --id "SUB_ID" --top
  asc subscriptions reorder --id "SUB_ID" --bottom`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*subID)
			if id == "" {
				return shared.UsageError("--id is required")
			}

			placement, anchorID, err := resolveSubscriptionReorderPlacement(
				id,
				strings.TrimSpace(*beforeID),
				strings.TrimSpace(*afterID),
				*top,
				*bottom,
			)
			if err != nil {
				return err
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("subscriptions reorder: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			source, err := client.GetSubscription(requestCtx, id)
			if err != nil {
				return fmt.Errorf("subscriptions reorder: resolve subscription: %w", err)
			}

			groupID, err := resolveSubscriptionGroupID(source.Data.Relationships)
			if err != nil {
				return fmt.Errorf("subscriptions reorder: %w", err)
			}

			siblings, err := listAllSubscriptionsForGroup(requestCtx, client, groupID)
			if err != nil {
				return fmt.Errorf("subscriptions reorder: list group subscriptions: %w", err)
			}

			currentOrder := orderedSubscriptionIDs(siblings)
			plan, err := planSubscriptionReorder(currentOrder, id, placement, anchorID)
			if err != nil {
				return fmt.Errorf("subscriptions reorder: %w", err)
			}

			result := subscriptionsReorderResult{
				SubscriptionID:       id,
				GroupID:              groupID,
				Placement:            string(placement),
				AnchorSubscriptionID: anchorID,
				FromGroupLevel:       plan.FromGroupLevel,
				ToGroupLevel:         plan.ToGroupLevel,
				Changed:              plan.Changed,
			}

			if !plan.Changed {
				result.Verified = true
				return shared.PrintOutput(result, *output.Output, *output.Pretty)
			}

			targetLevel := plan.ToGroupLevel
			attrs := asc.SubscriptionUpdateAttributes{
				GroupLevel: &targetLevel,
			}
			if _, err := client.UpdateSubscription(requestCtx, id, attrs); err != nil {
				return fmt.Errorf("subscriptions reorder: update subscription: %w", err)
			}

			verifiedSiblings, err := listAllSubscriptionsForGroup(requestCtx, client, groupID)
			if err != nil {
				return fmt.Errorf("subscriptions reorder: verify order: %w", err)
			}

			verifiedOrder := orderedSubscriptionIDs(verifiedSiblings)
			if !slicesEqual(verifiedOrder, plan.DesiredOrder) {
				return fmt.Errorf(
					"subscriptions reorder: verification failed: expected order [%s], got [%s]",
					strings.Join(plan.DesiredOrder, ", "),
					strings.Join(verifiedOrder, ", "),
				)
			}

			result.Verified = true
			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

func resolveSubscriptionReorderPlacement(
	sourceID string,
	beforeID string,
	afterID string,
	top bool,
	bottom bool,
) (subscriptionReorderPlacement, string, error) {
	selected := 0
	if beforeID != "" {
		selected++
	}
	if afterID != "" {
		selected++
	}
	if top {
		selected++
	}
	if bottom {
		selected++
	}

	if selected == 0 {
		return "", "", shared.UsageError("exactly one of --before, --after, --top, or --bottom is required")
	}
	if selected > 1 {
		return "", "", shared.UsageError("--before, --after, --top, and --bottom are mutually exclusive")
	}
	if beforeID == sourceID {
		return "", "", shared.UsageError("--before cannot reference the same subscription as --id")
	}
	if afterID == sourceID {
		return "", "", shared.UsageError("--after cannot reference the same subscription as --id")
	}

	switch {
	case beforeID != "":
		return subscriptionReorderPlacementBefore, beforeID, nil
	case afterID != "":
		return subscriptionReorderPlacementAfter, afterID, nil
	case top:
		return subscriptionReorderPlacementTop, "", nil
	default:
		return subscriptionReorderPlacementBottom, "", nil
	}
}

func resolveSubscriptionGroupID(raw json.RawMessage) (string, error) {
	if len(strings.TrimSpace(string(raw))) == 0 {
		return "", fmt.Errorf("subscription detail did not include a group relationship")
	}

	var relationships subscriptionResourceRelationships
	if err := json.Unmarshal(raw, &relationships); err != nil {
		return "", fmt.Errorf("failed to parse subscription group relationship: %w", err)
	}
	if relationships.Group == nil || strings.TrimSpace(relationships.Group.Data.ID) == "" {
		return "", fmt.Errorf("subscription detail did not include a group relationship")
	}
	return strings.TrimSpace(relationships.Group.Data.ID), nil
}

func listAllSubscriptionsForGroup(
	ctx context.Context,
	client *asc.Client,
	groupID string,
) ([]asc.Resource[asc.SubscriptionAttributes], error) {
	firstPage, err := client.GetSubscriptions(ctx, groupID, asc.WithSubscriptionsLimit(200))
	if err != nil {
		return nil, err
	}

	if strings.TrimSpace(firstPage.Links.Next) == "" {
		sortSubscriptionsByGroupLevel(firstPage.Data)
		return firstPage.Data, nil
	}

	allPages, err := asc.PaginateAll(ctx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
		return client.GetSubscriptions(ctx, groupID, asc.WithSubscriptionsNextURL(nextURL))
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

func planSubscriptionReorder(
	currentOrder []string,
	sourceID string,
	placement subscriptionReorderPlacement,
	anchorID string,
) (*subscriptionReorderPlan, error) {
	sourceIndex := indexOfString(currentOrder, sourceID)
	if sourceIndex < 0 {
		return nil, fmt.Errorf("subscription %q not found in the subscription group", sourceID)
	}

	withoutSource := make([]string, 0, len(currentOrder)-1)
	for _, id := range currentOrder {
		if id != sourceID {
			withoutSource = append(withoutSource, id)
		}
	}

	insertIndex := 0
	switch placement {
	case subscriptionReorderPlacementTop:
		insertIndex = 0
	case subscriptionReorderPlacementBottom:
		insertIndex = len(withoutSource)
	case subscriptionReorderPlacementBefore:
		insertIndex = indexOfString(withoutSource, anchorID)
		if insertIndex < 0 {
			return nil, fmt.Errorf("subscription %q not found in the subscription group", anchorID)
		}
	case subscriptionReorderPlacementAfter:
		insertIndex = indexOfString(withoutSource, anchorID)
		if insertIndex < 0 {
			return nil, fmt.Errorf("subscription %q not found in the subscription group", anchorID)
		}
		insertIndex++
	default:
		return nil, fmt.Errorf("unsupported placement %q", placement)
	}

	desired := make([]string, 0, len(currentOrder))
	desired = append(desired, withoutSource[:insertIndex]...)
	desired = append(desired, sourceID)
	desired = append(desired, withoutSource[insertIndex:]...)

	return &subscriptionReorderPlan{
		DesiredOrder:   desired,
		FromGroupLevel: sourceIndex + 1,
		ToGroupLevel:   indexOfString(desired, sourceID) + 1,
		Changed:        !slicesEqual(currentOrder, desired),
	}, nil
}

func indexOfString(values []string, needle string) int {
	for i, value := range values {
		if value == needle {
			return i
		}
	}
	return -1
}

func slicesEqual(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}
