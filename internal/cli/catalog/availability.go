package catalog

import (
	"context"
	"fmt"
	"net/url"
	"slices"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/ascterritory"
)

type availability struct {
	Territories               []string `json:"territories"`
	AvailableInNewTerritories bool     `json:"availableInNewTerritories"`
}

func validateAvailability(a *availability) error {
	if a == nil {
		return nil
	}
	if len(a.Territories) == 0 {
		return fmt.Errorf("availability.territories must not be empty")
	}
	seen := map[string]bool{}
	for i, t := range a.Territories {
		value, err := ascterritory.Normalize(t)
		if err != nil {
			return err
		}
		if seen[value] {
			return fmt.Errorf("duplicate availability territory %q", value)
		}
		seen[value] = true
		a.Territories[i] = value
	}
	slices.Sort(a.Territories)
	return nil
}

func collect[T any](ctx context.Context, collection string, fetch func(context.Context, string) (*asc.Response[T], error)) ([]asc.Resource[T], error) {
	seen := map[string]bool{}
	expectedTotal := 0
	page := func(ctx context.Context, next string) (*asc.Response[T], error) {
		if next != "" {
			var err error
			next, err = collectionContinuation(next, collection)
			if err != nil {
				return nil, err
			}
		}
		r, err := fetch(ctx, next)
		if err != nil {
			return nil, err
		}
		if r == nil || r.Data == nil {
			return nil, fmt.Errorf("collection response data must be an array")
		}
		expectedTotal = max(expectedTotal, asc.ParsePagingTotal(r.Meta))
		for _, item := range r.Data {
			if item.ID == "" || item.Type == "" {
				return nil, fmt.Errorf("collection response is missing resource identity")
			}
			if seen[item.ID] {
				return nil, fmt.Errorf("duplicate resource %s across collection pages", item.ID)
			}
			seen[item.ID] = true
		}
		return r, nil
	}
	first, err := page(ctx, "")
	if err != nil {
		return nil, err
	}
	all, err := asc.PaginateAll(ctx, first, func(ctx context.Context, next string) (asc.PaginatedResponse, error) { return page(ctx, next) })
	if err != nil {
		return nil, err
	}
	if len(seen) < expectedTotal {
		return nil, fmt.Errorf("incomplete collection %s: received %d of %d resources", collection, len(seen), expectedTotal)
	}
	return all.(*asc.Response[T]).Data, nil
}

func collectionContinuation(next, collection string) (string, error) {
	original, err := url.Parse(collection)
	if err != nil {
		return "", err
	}
	canonical, err := asc.ScopedContinuationURL(next, original.Path)
	if err != nil {
		return "", err
	}
	parsed, err := url.Parse(canonical)
	if err != nil {
		return "", err
	}
	query, err := url.ParseQuery(parsed.RawQuery)
	if err != nil {
		return "", fmt.Errorf("invalid scoped collection query: %w", err)
	}
	expectedQuery := original.Query()
	expectedQuery.Del("limit")
	for key, expected := range expectedQuery {
		if actual, exists := query[key]; exists && !slices.Equal(actual, expected) {
			return "", fmt.Errorf("pagination URL changed scoped collection query %q", key)
		}
		query[key] = expected
	}
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

type availabilitySnapshot struct {
	id          string
	territories []string
	future      bool
}

func readAvailability(ctx context.Context, c *asc.Client, kind, id string) (availabilitySnapshot, error) {
	var snapshot availabilitySnapshot
	if id == "" {
		return snapshot, nil
	}
	var existing []asc.Resource[asc.TerritoryAttributes]
	var err error
	if kind == "subscription" {
		r, readErr := c.GetSubscriptionAvailabilityForSubscription(ctx, id)
		if asc.IsMissingResourceOfType(readErr, "subscriptionAvailabilities") {
			return snapshot, nil
		}
		if readErr != nil {
			return snapshot, readErr
		}
		if r.Data.ID == "" || r.Data.Type != "subscriptionAvailabilities" {
			return snapshot, fmt.Errorf("invalid subscription availability identity")
		}
		snapshot.id, snapshot.future = r.Data.ID, r.Data.Attributes.AvailableInNewTerritories
		existing, err = collect(ctx, "/v1/subscriptionAvailabilities/"+snapshot.id+"/availableTerritories", func(ctx context.Context, next string) (*asc.TerritoriesResponse, error) {
			return c.GetSubscriptionAvailabilityAvailableTerritories(ctx, snapshot.id, asc.Limit[asc.SubscriptionAvailabilityTerritoriesOption](200), asc.NextURL[asc.SubscriptionAvailabilityTerritoriesOption](next))
		})
	} else {
		r, readErr := c.GetInAppPurchaseAvailability(ctx, id)
		if asc.IsMissingResourceOfType(readErr, "inAppPurchaseAvailabilities") {
			return snapshot, nil
		}
		if readErr != nil {
			return snapshot, readErr
		}
		if r.Data.ID == "" || r.Data.Type != "inAppPurchaseAvailabilities" {
			return snapshot, fmt.Errorf("invalid in-app purchase availability identity")
		}
		snapshot.id, snapshot.future = r.Data.ID, r.Data.Attributes.AvailableInNewTerritories
		existing, err = collect(ctx, "/v1/inAppPurchaseAvailabilities/"+snapshot.id+"/availableTerritories", func(ctx context.Context, next string) (*asc.TerritoriesResponse, error) {
			return c.GetInAppPurchaseAvailabilityAvailableTerritories(ctx, snapshot.id, asc.Limit[asc.IAPAvailabilityTerritoriesOption](200), asc.NextURL[asc.IAPAvailabilityTerritoriesOption](next))
		})
	}
	if err != nil {
		return snapshot, err
	}
	for _, territory := range existing {
		if territory.Type != "territories" {
			return snapshot, fmt.Errorf("invalid availability territory type")
		}
		snapshot.territories = append(snapshot.territories, territory.ID)
	}
	slices.Sort(snapshot.territories)
	return snapshot, nil
}

func planAvailability(ctx context.Context, c *asc.Client, kind, key string, id *string, a *availability) (operation, error) {
	op := operation{item: asc.CatalogItem{Kind: kind + "Availability", Key: key, Action: "create", Status: "pending", Changes: map[string]any{"territories": a.Territories, "availableInNewTerritories": a.AvailableInNewTerritories}}}
	planned, err := readAvailability(ctx, c, kind, *id)
	if err != nil {
		return op, err
	}
	matches := func(snapshot availabilitySnapshot) bool {
		return snapshot.id != "" && snapshot.future == a.AvailableInNewTerritories && slices.Equal(snapshot.territories, a.Territories)
	}
	if planned.id != "" {
		op.item.ID = planned.id
		op.item.Action = "update"
		op.destructive = planned.future && !a.AvailableInNewTerritories
		for _, territory := range planned.territories {
			if !slices.Contains(a.Territories, territory) {
				op.destructive = true
			}
		}
		if matches(planned) {
			op.item.Action = "noop"
			op.item.Changes = nil
		}
	}
	op.write = func(ctx context.Context) (string, error) {
		actual, err := readAvailability(ctx, c, kind, *id)
		if err != nil {
			return "", err
		}
		if matches(actual) {
			return actual.id, nil
		}
		if actual.id != planned.id || actual.future != planned.future || !slices.Equal(actual.territories, planned.territories) {
			return "", fmt.Errorf("availability changed after planning; rerun to review the current scope")
		}
		if kind == "subscription" {
			r, err := c.CreateSubscriptionAvailability(ctx, *id, a.Territories, asc.SubscriptionAvailabilityAttributes{AvailableInNewTerritories: a.AvailableInNewTerritories})
			if err != nil {
				return "", err
			}
			return r.Data.ID, nil
		}
		r, err := c.CreateInAppPurchaseAvailability(ctx, *id, a.AvailableInNewTerritories, a.Territories)
		if err != nil {
			return "", err
		}
		return r.Data.ID, nil
	}
	return op, nil
}

func planAvailabilityItems(ctx context.Context, c *asc.Client, m manifest, ids map[string]*string) ([]operation, error) {
	ops := []operation{}
	for _, p := range m.InAppPurchases {
		if p.Availability != nil {
			op, err := planAvailability(ctx, c, "inAppPurchase", p.ProductID, ids[p.ProductID], p.Availability)
			if err != nil {
				return nil, err
			}
			ops = append(ops, op)
		}
	}
	for _, s := range m.Subscriptions {
		if s.Availability != nil {
			op, err := planAvailability(ctx, c, "subscription", s.ProductID, ids[s.ProductID], s.Availability)
			if err != nil {
				return nil, err
			}
			ops = append(ops, op)
		}
	}
	return ops, nil
}
