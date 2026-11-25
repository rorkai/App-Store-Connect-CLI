package subscriptions

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"slices"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

var errSubscriptionPriceTierIntentRequired = errors.New("--additional-price-tiers is required when changing a price with volume tiers or unverified tier linkage. Supply all desired tiers or [] for a single-tier price")

func subscriptionPriceRequiresTierIntent(price asc.Resource[asc.SubscriptionPriceAttributes]) bool {
	if price.Attributes.PlanType == asc.SubscriptionPlanTypeMonthly {
		return false
	}
	var relationships struct {
		AdditionalPriceTiers struct {
			Data  []json.RawMessage `json:"data"`
			Links asc.Links         `json:"links"`
			Meta  struct {
				Paging struct {
					Total int `json:"total"`
				} `json:"paging"`
			} `json:"meta"`
		} `json:"additionalPriceTiers"`
	}
	if err := json.Unmarshal(price.Relationships, &relationships); err != nil {
		return true
	}
	tiers := relationships.AdditionalPriceTiers
	return tiers.Data == nil || len(tiers.Data) > 0 || tiers.Links.Next != "" || tiers.Meta.Paging.Total != 0
}

func subscriptionPriceTiersMatch(ctx context.Context, client *asc.Client, priceID string, desired []asc.ProposedSubscriptionPriceTier) (bool, error) {
	fetch := func(next string) (*asc.SubscriptionPriceTiersResponse, error) {
		if next != "" {
			canonical, err := client.ValidateNoRedirectReadContinuationQuery(ctx, next, "/v1/subscriptionPrices/"+url.PathEscape(priceID)+"/additionalPriceTiers", url.Values{"include": {"subscriptionPricePoint"}})
			if err != nil {
				return nil, err
			}
			next, err = mergeSubscriptionPricesNextQuery(canonical, url.Values{"include": {"subscriptionPricePoint"}})
			if err != nil {
				return nil, err
			}
		}
		resp, err := client.GetSubscriptionPriceAdditionalPriceTiers(ctx, priceID,
			asc.Limit[asc.SubscriptionPriceTiersOption](200), asc.WithSubscriptionPriceTiersInclude([]string{"subscriptionPricePoint"}), asc.NextURL[asc.SubscriptionPriceTiersOption](next))
		if err == nil && (resp == nil || resp.Data == nil) {
			return nil, fmt.Errorf("subscription tier readback requires a data array")
		}
		return resp, err
	}
	first, err := fetch("")
	if err != nil {
		return false, err
	}
	all, err := asc.PaginateAll(ctx, first, func(_ context.Context, next string) (asc.PaginatedResponse, error) {
		return fetch(next)
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

var errSubscriptionVolumeTiersFound = errors.New("subscription has volume price tiers")

func subscriptionHasPriceTiers(ctx context.Context, d *shared.Deps, client *asc.Client, subscriptionID string, territories []string) (bool, error) {
	ctx = asc.WithNoRedirectReads(ctx)
	query := url.Values{"filter[subscription]": {subscriptionID}}
	if len(territories) > 0 {
		query.Set("filter[territory]", strings.Join(territories, ","))
	}
	fetch := func(next string) (*asc.SubscriptionPriceTiersResponse, error) {
		opts := []asc.SubscriptionPriceTiersOption{
			asc.WithSubscriptionPriceTiersSubscriptions([]string{subscriptionID}),
			asc.WithSubscriptionPriceTiersTerritories(territories),
			asc.Limit[asc.SubscriptionPriceTiersOption](1),
		}
		if next != "" {
			canonical, err := client.ValidateNoRedirectReadContinuationQuery(ctx, next, "/v1/subscriptionPriceTiers", query)
			if err != nil {
				return nil, err
			}
			canonical, err = mergeSubscriptionPricesNextQuery(canonical, query)
			if err != nil {
				return nil, err
			}
			opts = []asc.SubscriptionPriceTiersOption{asc.NextURL[asc.SubscriptionPriceTiersOption](canonical)}
		}
		response, err := shared.RetryReadWithFreshTimeoutWith(ctx, d, func(requestCtx context.Context) (*asc.SubscriptionPriceTiersResponse, error) {
			return client.GetSubscriptionPriceTiers(requestCtx, opts...)
		})
		if err != nil {
			return nil, err
		}
		if response == nil || response.Data == nil {
			return nil, fmt.Errorf("cannot verify volume price tiers for subscription %s: missing tier collection", subscriptionID)
		}
		return response, nil
	}
	first, err := fetch("")
	if err != nil {
		return false, err
	}
	err = asc.PaginateEach(ctx, first, func(_ context.Context, next string) (asc.PaginatedResponse, error) {
		return fetch(next)
	}, func(page asc.PaginatedResponse) error {
		if len(page.(*asc.SubscriptionPriceTiersResponse).Data) > 0 {
			return errSubscriptionVolumeTiersFound
		}
		return nil
	})
	if errors.Is(err, errSubscriptionVolumeTiersFound) {
		return true, nil
	}
	return false, err
}

func requireSingleTierBulkPriceWrite(ctx context.Context, d *shared.Deps, client *asc.Client, subscriptionID string, territories []string) error {
	hasTiers, err := subscriptionHasPriceTiers(ctx, d, client, subscriptionID, territories)
	if err != nil {
		return err
	}
	if hasTiers {
		return fmt.Errorf("subscription %s has volume price tiers. Use asc subscriptions pricing prices set --additional-price-tiers with all desired tiers or [] for a single-tier price", subscriptionID)
	}
	return nil
}
