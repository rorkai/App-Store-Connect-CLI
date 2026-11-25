package subscriptions

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

var errSubscriptionPriceTierIntentRequired = errors.New("--additional-price-tiers is required when changing a price with volume tiers. Supply all desired tiers or [] for a single-tier price")

func subscriptionPriceHasAdditionalTiers(price asc.Resource[asc.SubscriptionPriceAttributes]) bool {
	var relationships struct {
		AdditionalPriceTiers *asc.RelationshipList `json:"additionalPriceTiers"`
	}
	if err := json.Unmarshal(price.Relationships, &relationships); err != nil {
		return false
	}
	return relationships.AdditionalPriceTiers != nil && len(relationships.AdditionalPriceTiers.Data) > 0
}

func subscriptionPriceTiersMatch(ctx context.Context, client *asc.Client, priceID string, desired []asc.ProposedSubscriptionPriceTier) (bool, error) {
	first, err := client.GetSubscriptionPriceAdditionalPriceTiers(ctx, priceID,
		asc.Limit[asc.SubscriptionPriceTiersOption](200),
		asc.WithSubscriptionPriceTiersInclude([]string{"subscriptionPricePoint"}),
	)
	if err != nil {
		return false, err
	}
	all, err := asc.PaginateAll(ctx, first, func(ctx context.Context, next string) (asc.PaginatedResponse, error) {
		return client.GetSubscriptionPriceAdditionalPriceTiers(ctx, priceID, asc.NextURL[asc.SubscriptionPriceTiersOption](next))
	})
	if err != nil {
		return false, err
	}
	result := all.(*asc.SubscriptionPriceTiersResponse)
	actual := make([]asc.ProposedSubscriptionPriceTier, 0, len(result.Data))
	for _, tier := range result.Data {
		var relationships struct {
			SubscriptionPricePoint *asc.Relationship `json:"subscriptionPricePoint"`
		}
		if err := json.Unmarshal(tier.Relationships, &relationships); err != nil {
			return false, err
		}
		if tier.Attributes.MinQuantity == nil || relationships.SubscriptionPricePoint == nil {
			return false, nil
		}
		actual = append(actual, asc.ProposedSubscriptionPriceTier{
			MinQuantity:              *tier.Attributes.MinQuantity,
			SubscriptionPricePointID: strings.TrimSpace(relationships.SubscriptionPricePoint.Data.ID),
		})
	}
	slices.SortFunc(actual, func(a, b asc.ProposedSubscriptionPriceTier) int {
		return cmp.Compare(a.MinQuantity, b.MinQuantity)
	})
	return slices.Equal(actual, desired), nil
}
