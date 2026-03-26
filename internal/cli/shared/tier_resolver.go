package shared

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

// TierEntry represents a single tier in a territory's price point list.
type TierEntry struct {
	Tier          int    `json:"tier"`
	PricePointID  string `json:"pricePointId"`
	CustomerPrice string `json:"customerPrice"`
	Proceeds      string `json:"proceeds"`
}

type pricePointEntry struct {
	id            string
	customerPrice float64
	rawPrice      string
	proceeds      string
}

type tierPage struct {
	entries []pricePointEntry
	nextURL string
}

type tierPageFetcher func(nextURL string) (tierPage, error)

type freePricePointPage struct {
	pricePointID string
	nextURL      string
	found        bool
}

type freePricePointPageFetcher func(nextURL string) (freePricePointPage, error)

const (
	tierCacheScopeSubscription = "subscription"
	tierCacheScopeIAP          = "iap"
)

// ResolveTiers fetches all price points for a territory, sorts by customerPrice ascending,
// and assigns tier numbers starting at 1. Free (0.00) price points are excluded.
func ResolveTiers(ctx context.Context, client *asc.Client, appID, territory string, refresh bool) ([]TierEntry, error) {
	normalizedAppID, normalizedTerritory, err := normalizeTierResolverInputs("app", appID, territory)
	if err != nil {
		return nil, err
	}

	return resolveTiersWithFetcher(
		refresh,
		func() ([]TierEntry, error) {
			return LoadTierCache(normalizedAppID, normalizedTerritory)
		},
		func(tiers []TierEntry) error {
			return SaveTierCache(normalizedAppID, normalizedTerritory, tiers)
		},
		func(nextURL string) (tierPage, error) {
			opts := []asc.PricePointsOption{
				asc.WithPricePointsLimit(200),
				asc.WithPricePointsTerritory(normalizedTerritory),
			}
			if nextURL != "" {
				opts = []asc.PricePointsOption{asc.WithPricePointsNextURL(nextURL)}
			}
			resp, err := client.GetAppPricePoints(ctx, normalizedAppID, opts...)
			if err != nil {
				return tierPage{}, fmt.Errorf("fetch price points: %w", err)
			}

			entries := make([]pricePointEntry, 0, len(resp.Data))
			for _, pp := range resp.Data {
				entry, ok := newPricePointEntry(pp.ID, pp.Attributes.CustomerPrice, pp.Attributes.Proceeds)
				if !ok {
					continue
				}
				entries = append(entries, entry)
			}

			return tierPage{
				entries: entries,
				nextURL: resp.Links.Next,
			}, nil
		},
	)
}

// ResolveSubscriptionTiers resolves subscription price point tiers for a subscription and territory.
func ResolveSubscriptionTiers(ctx context.Context, client *asc.Client, subscriptionID, territory string, refresh bool) ([]TierEntry, error) {
	return resolveScopedTiers(ctx, client, "subscription", subscriptionID, territory, tierCacheScopeSubscription, refresh,
		func(ctx context.Context, client *asc.Client, resourceID, territory, nextURL string) (tierPage, error) {
			opts := []asc.SubscriptionPricePointsOption{
				asc.WithSubscriptionPricePointsLimit(200),
				asc.WithSubscriptionPricePointsTerritory(territory),
			}
			if nextURL != "" {
				opts = []asc.SubscriptionPricePointsOption{asc.WithSubscriptionPricePointsNextURL(nextURL)}
			}
			resp, err := client.GetSubscriptionPricePoints(ctx, resourceID, opts...)
			if err != nil {
				return tierPage{}, fmt.Errorf("fetch price points: %w", err)
			}
			return buildTierPage(resp.Links.Next, func(add func(string, string, string)) {
				for _, pp := range resp.Data {
					add(pp.ID, pp.Attributes.CustomerPrice, pp.Attributes.Proceeds)
				}
			}), nil
		},
	)
}

// ResolveIAPTiers resolves in-app purchase price point tiers for an IAP and territory.
func ResolveIAPTiers(ctx context.Context, client *asc.Client, iapID, territory string, refresh bool) ([]TierEntry, error) {
	return resolveScopedTiers(ctx, client, "in-app purchase", iapID, territory, tierCacheScopeIAP, refresh,
		func(ctx context.Context, client *asc.Client, resourceID, territory, nextURL string) (tierPage, error) {
			opts := []asc.IAPPricePointsOption{
				asc.WithIAPPricePointsLimit(200),
				asc.WithIAPPricePointsTerritory(territory),
			}
			if nextURL != "" {
				opts = []asc.IAPPricePointsOption{asc.WithIAPPricePointsNextURL(nextURL)}
			}
			resp, err := client.GetInAppPurchasePricePoints(ctx, resourceID, opts...)
			if err != nil {
				return tierPage{}, fmt.Errorf("fetch price points: %w", err)
			}
			return buildTierPage(resp.Links.Next, func(add func(string, string, string)) {
				for _, pp := range resp.Data {
					add(pp.ID, pp.Attributes.CustomerPrice, pp.Attributes.Proceeds)
				}
			}), nil
		},
	)
}

type scopedTierPageFetcher func(context.Context, *asc.Client, string, string, string) (tierPage, error)

func resolveScopedTiers(
	ctx context.Context,
	client *asc.Client,
	resourceName string,
	resourceID string,
	territory string,
	cacheScope string,
	refresh bool,
	fetchPage scopedTierPageFetcher,
) ([]TierEntry, error) {
	normalizedResourceID, normalizedTerritory, err := normalizeTierResolverInputs(resourceName, resourceID, territory)
	if err != nil {
		return nil, err
	}

	return resolveTiersWithFetcher(
		refresh,
		func() ([]TierEntry, error) {
			return loadScopedTierCache(cacheScope, normalizedResourceID, normalizedTerritory)
		},
		func(tiers []TierEntry) error {
			return saveScopedTierCache(cacheScope, normalizedResourceID, normalizedTerritory, tiers)
		},
		func(nextURL string) (tierPage, error) {
			return fetchPage(ctx, client, normalizedResourceID, normalizedTerritory, nextURL)
		},
	)
}

func normalizeTierResolverInputs(resourceName, resourceID, territory string) (string, string, error) {
	normalizedResourceID := strings.TrimSpace(resourceID)
	normalizedTerritory := strings.ToUpper(strings.TrimSpace(territory))
	if normalizedResourceID == "" {
		return "", "", fmt.Errorf("%s ID is required for tier resolution", resourceName)
	}
	if normalizedTerritory == "" {
		return "", "", fmt.Errorf("territory is required for tier resolution")
	}
	return normalizedResourceID, normalizedTerritory, nil
}

func newPricePointEntry(id, customerPrice, proceeds string) (pricePointEntry, bool) {
	parsedPrice, err := strconv.ParseFloat(strings.TrimSpace(customerPrice), 64)
	if err != nil || parsedPrice <= 0 {
		return pricePointEntry{}, false
	}
	return pricePointEntry{
		id:            id,
		customerPrice: parsedPrice,
		rawPrice:      customerPrice,
		proceeds:      proceeds,
	}, true
}

func buildTierPage(nextURL string, appendEntries func(add func(string, string, string))) tierPage {
	entries := make([]pricePointEntry, 0)
	appendEntries(func(id, customerPrice, proceeds string) {
		entry, ok := newPricePointEntry(id, customerPrice, proceeds)
		if !ok {
			return
		}
		entries = append(entries, entry)
	})
	return tierPage{
		entries: entries,
		nextURL: nextURL,
	}
}

func resolveTiersWithFetcher(
	refresh bool,
	loadCache func() ([]TierEntry, error),
	saveCache func([]TierEntry) error,
	fetchPage tierPageFetcher,
) ([]TierEntry, error) {
	if !refresh {
		cached, err := loadCache()
		if err == nil && len(cached) > 0 {
			return cached, nil
		}
	}

	var entries []pricePointEntry
	var nextURL string

	for {
		page, err := fetchPage(nextURL)
		if err != nil {
			return nil, err
		}
		entries = append(entries, page.entries...)

		if page.nextURL == "" {
			break
		}
		nextURL = page.nextURL
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].customerPrice < entries[j].customerPrice
	})

	tiers := make([]TierEntry, 0, len(entries))
	for i, e := range entries {
		tiers = append(tiers, TierEntry{
			Tier:          i + 1,
			PricePointID:  e.id,
			CustomerPrice: e.rawPrice,
			Proceeds:      e.proceeds,
		})
	}

	if len(tiers) > 0 {
		_ = saveCache(tiers)
	}

	return tiers, nil
}

// ResolveFreeAppPricePoint finds the free ($0) price point ID for an app in a territory.
func ResolveFreeAppPricePoint(ctx context.Context, client *asc.Client, appID, territory string) (string, error) {
	normalizedAppID, normalizedTerritory, err := normalizeTierResolverInputs("app", appID, territory)
	if err != nil {
		return "", err
	}

	return resolveFreeAppPricePointWithFetcher(normalizedTerritory, func(nextURL string) (freePricePointPage, error) {
		opts := []asc.PricePointsOption{
			asc.WithPricePointsLimit(200),
			asc.WithPricePointsTerritory(normalizedTerritory),
		}
		if nextURL != "" {
			opts = []asc.PricePointsOption{asc.WithPricePointsNextURL(nextURL)}
		}
		resp, err := client.GetAppPricePoints(ctx, normalizedAppID, opts...)
		if err != nil {
			return freePricePointPage{}, fmt.Errorf("fetch price points: %w", err)
		}
		pricePointID, found := findFreePricePointID(resp.Data)
		return freePricePointPage{
			pricePointID: pricePointID,
			nextURL:      resp.Links.Next,
			found:        found,
		}, nil
	})
}

// ResolvePricePointByTier finds the price point ID for a given tier number.
func ResolvePricePointByTier(tiers []TierEntry, tier int) (string, error) {
	for _, t := range tiers {
		if t.Tier == tier {
			return t.PricePointID, nil
		}
	}
	return "", fmt.Errorf("tier %d not found (valid range: 1-%d)", tier, len(tiers))
}

func resolveFreeAppPricePointWithFetcher(territory string, fetchPage freePricePointPageFetcher) (string, error) {
	var nextURL string

	for {
		page, err := fetchPage(nextURL)
		if err != nil {
			return "", err
		}
		if page.found {
			return page.pricePointID, nil
		}
		if page.nextURL == "" {
			break
		}
		nextURL = page.nextURL
	}

	return "", fmt.Errorf("no free ($0) price point found for territory %s", territory)
}

func findFreePricePointID(pricePoints []asc.Resource[asc.AppPricePointV3Attributes]) (string, bool) {
	for _, pp := range pricePoints {
		cp := strings.TrimSpace(pp.Attributes.CustomerPrice)
		parsed, err := strconv.ParseFloat(cp, 64)
		if err == nil && parsed == 0 {
			return pp.ID, true
		}
	}
	return "", false
}

// ResolvePricePointByPrice finds the price point ID for a given customer price.
func ResolvePricePointByPrice(tiers []TierEntry, price string) (string, error) {
	target, err := strconv.ParseFloat(strings.TrimSpace(price), 64)
	if err != nil {
		return "", fmt.Errorf("invalid price %q: %w", price, err)
	}
	if math.IsNaN(target) || math.IsInf(target, 0) {
		return "", fmt.Errorf("price must be a finite number")
	}

	for _, t := range tiers {
		cp, err := strconv.ParseFloat(strings.TrimSpace(t.CustomerPrice), 64)
		if err != nil {
			continue
		}
		if math.Abs(cp-target) < 0.005 {
			return t.PricePointID, nil
		}
	}
	return "", fmt.Errorf("no price point found matching price %s in this territory", price)
}

// ValidatePriceSelectionFlags checks that --price-point, --tier, --price, and --free are mutually exclusive.
// Returns a usage-style error if more than one is set.
func ValidatePriceSelectionFlags(pricePoint string, tier int, price string, free ...bool) error {
	if tier < 0 {
		return fmt.Errorf("--tier must be a positive integer")
	}

	supportsFree := len(free) > 0
	isFree := supportsFree && free[0]
	requiredMessage, mutuallyExclusiveMessage := priceSelectionValidationMessages(supportsFree)

	count := 0
	if strings.TrimSpace(pricePoint) != "" {
		count++
	}
	if tier > 0 {
		count++
	}
	if strings.TrimSpace(price) != "" {
		count++
	}
	if isFree {
		count++
	}
	if count == 0 {
		return fmt.Errorf("%s", requiredMessage)
	}
	if count > 1 {
		return fmt.Errorf("%s", mutuallyExclusiveMessage)
	}
	return nil
}

func priceSelectionValidationMessages(supportsFree bool) (required string, mutuallyExclusive string) {
	if supportsFree {
		return "one of --price-point, --tier, --price, or --free is required",
			"--price-point, --tier, --price, and --free are mutually exclusive"
	}
	return "one of --price-point, --tier, or --price is required",
		"--price-point, --tier, and --price are mutually exclusive"
}
