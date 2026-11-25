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

// Price write decisions that depend on absence must account for every advertised row.
type priceCollectionCount struct {
	read       int
	advertised int
}

func checkPriceCollectionPage[T any](count *priceCollectionCount, page *asc.Response[T], name string) error {
	if page == nil || page.Data == nil {
		return fmt.Errorf("missing %s collection", name)
	}
	count.read += len(page.Data)
	count.advertised = max(count.advertised, asc.ParsePagingTotal(page.Meta))
	if page.Links.Next == "" && count.read < count.advertised {
		return fmt.Errorf("incomplete %s collection: received %d of %d", name, count.read, count.advertised)
	}
	return nil
}

func subscriptionHasExistingPrices(ctx context.Context, client *asc.Client, subscriptionID string) (bool, error) {
	prices, err := client.GetSubscriptionPricesRelationships(asc.WithNoRedirectReads(ctx), subscriptionID)
	if err != nil {
		return false, err
	}
	if prices == nil || prices.Data == nil {
		return false, fmt.Errorf("missing existing price collection")
	}
	if len(prices.Data) == 0 && (prices.Links.Next != "" || asc.ParsePagingTotal(prices.Meta) > 0) {
		return false, fmt.Errorf("unverified existing price membership: empty page reports additional prices")
	}
	return len(prices.Data) > 0, nil
}

func requireUnpricedSubscription(ctx context.Context, client *asc.Client, id string) error {
	exists, err := subscriptionHasExistingPrices(ctx, client, id)
	if err != nil {
		return err
	}
	if exists {
		return fmt.Errorf("subscription %s already has prices; cannot replay the initial price matrix", id)
	}
	return nil
}

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
	var count priceCollectionCount
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
		if err == nil {
			err = checkPriceCollectionPage(&count, resp, "price tier")
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
		if tier.Attributes.MinQuantity == nil || relationships.SubscriptionPricePoint == nil || relationships.SubscriptionPricePoint.Data.Type != asc.ResourceTypeSubscriptionPricePoints {
			return false, fmt.Errorf("price tier %q is missing its minimum quantity or price point", tier.ID)
		}
		actual = append(actual, asc.ProposedSubscriptionPriceTier{
			MinQuantity:              *tier.Attributes.MinQuantity,
			SubscriptionPricePointID: strings.TrimSpace(relationships.SubscriptionPricePoint.Data.ID),
		})
	}
	slices.SortFunc(actual, func(a, b asc.ProposedSubscriptionPriceTier) int {
		return cmp.Compare(a.MinQuantity, b.MinQuantity)
	})
	if err := asc.ValidateProposedSubscriptionPriceTiers(actual); err != nil {
		return false, fmt.Errorf("invalid price tier readback: %w", err)
	}
	return slices.Equal(actual, desired), nil
}

var errSubscriptionVolumeTiersFound = errors.New("subscription has volume price tiers")

func subscriptionHasPriceTiers(ctx context.Context, d *shared.Deps, client *asc.Client, subscriptionID string, territories []string) (bool, error) {
	ctx = asc.WithNoRedirectReads(ctx)
	query := url.Values{"filter[subscription]": {subscriptionID}}
	if len(territories) > 0 {
		query.Set("filter[territory]", strings.Join(territories, ","))
	}
	var count priceCollectionCount
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
		return response, checkPriceCollectionPage(&count, response, "price tier")
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
