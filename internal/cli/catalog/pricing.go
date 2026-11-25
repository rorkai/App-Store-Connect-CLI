package catalog

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/ascterritory"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

type pricing struct {
	BaseTerritory string           `json:"baseTerritory"`
	CustomerPrice string           `json:"customerPrice"`
	Equalize      bool             `json:"equalize"`
	Territories   []territoryPrice `json:"territories,omitempty"`
	StartDate     string           `json:"startDate,omitempty"`
}

type territoryPrice struct {
	Territory     string `json:"territory"`
	CustomerPrice string `json:"customerPrice"`
}

func validatePricing(p *pricing) error {
	if p == nil {
		return nil
	}
	var err error
	p.BaseTerritory, err = ascterritory.Normalize(p.BaseTerritory)
	if err != nil {
		return err
	}
	if !shared.IsPriceDecimal(p.CustomerPrice) {
		return fmt.Errorf("pricing.customerPrice must be a decimal string")
	}
	if p.Equalize && len(p.Territories) > 0 {
		return fmt.Errorf("pricing: equalize and explicit territories are mutually exclusive")
	}
	seen := map[string]bool{p.BaseTerritory: true}
	for i := range p.Territories {
		t := &p.Territories[i]
		t.Territory, err = ascterritory.Normalize(t.Territory)
		if err != nil {
			return err
		}
		if seen[t.Territory] {
			return fmt.Errorf("duplicate pricing territory %q", t.Territory)
		}
		seen[t.Territory] = true
		if !shared.IsPriceDecimal(t.CustomerPrice) {
			return fmt.Errorf("pricing customerPrice for %s must be a decimal string", t.Territory)
		}
	}
	if p.StartDate != "" {
		if _, err := time.Parse(time.DateOnly, p.StartDate); err != nil {
			return fmt.Errorf("pricing.startDate must be YYYY-MM-DD")
		}
	}
	return nil
}

func relationshipID(data json.RawMessage, key string) (string, error) {
	var relationships map[string]json.RawMessage
	if err := json.Unmarshal(data, &relationships); err != nil {
		return "", fmt.Errorf("invalid %s relationship: %w", key, err)
	}
	var relationship asc.Relationship
	if raw, ok := relationships[key]; ok {
		if err := json.Unmarshal(raw, &relationship); err != nil {
			return "", fmt.Errorf("invalid %s relationship: %w", key, err)
		}
	}
	id := relationship.Data.ID
	if id == "" {
		return "", fmt.Errorf("missing %s relationship ID", key)
	}
	return id, nil
}

func resolvePrice(ctx context.Context, c *asc.Client, kind, id, territory, price string) (string, error) {
	want, ok := new(big.Rat).SetString(price)
	if !ok {
		return "", fmt.Errorf("invalid customer price")
	}
	var matches []string
	if kind == "subscription" {
		points, err := collect(ctx, "/v1/subscriptions/"+id+"/pricePoints?filter%5Bterritory%5D="+url.QueryEscape(territory), func(ctx context.Context, next string) (*asc.SubscriptionPricePointsResponse, error) {
			if next != "" {
				return c.GetSubscriptionPricePoints(ctx, id, asc.WithSubscriptionPricePointsNextURL(next))
			}
			return c.GetSubscriptionPricePoints(ctx, id, asc.WithSubscriptionPricePointsTerritory(territory), asc.WithSubscriptionPricePointsLimit(200), asc.WithSubscriptionPricePointsNextURL(next))
		})
		if err != nil {
			return "", err
		}
		for _, p := range points {
			if value, ok := new(big.Rat).SetString(p.Attributes.CustomerPrice); ok && want.Cmp(value) == 0 {
				matches = append(matches, p.ID)
			}
		}
	} else {
		points, err := collect(ctx, "/v2/inAppPurchases/"+id+"/pricePoints?filter%5Bterritory%5D="+url.QueryEscape(territory), func(ctx context.Context, next string) (*asc.InAppPurchasePricePointsResponse, error) {
			return c.GetInAppPurchasePricePoints(ctx, id, asc.WithIAPPricePointsTerritory(territory), asc.Limit[asc.IAPPricePointsOption](200), asc.NextURL[asc.IAPPricePointsOption](next))
		})
		if err != nil {
			return "", err
		}
		for _, p := range points {
			if value, ok := new(big.Rat).SetString(p.Attributes.CustomerPrice); ok && want.Cmp(value) == 0 {
				matches = append(matches, p.ID)
			}
		}
	}
	if len(matches) != 1 {
		return "", fmt.Errorf("customer price %s in %s resolved to %d price points; an exact unambiguous price is required", price, territory, len(matches))
	}
	return matches[0], nil
}

func resolvePrices(ctx context.Context, c *asc.Client, kind, id string, p *pricing) (map[string]string, error) {
	base, err := resolvePrice(ctx, c, kind, id, p.BaseTerritory, p.CustomerPrice)
	if err != nil {
		return nil, err
	}
	points := map[string]string{p.BaseTerritory: base}
	for _, t := range p.Territories {
		point, err := resolvePrice(ctx, c, kind, id, t.Territory, t.CustomerPrice)
		if err != nil {
			return nil, err
		}
		points[t.Territory] = point
	}
	if p.Equalize && kind == "subscription" {
		equalized, err := collect(ctx, "/v1/subscriptionPricePoints/"+base+"/equalizations?include=territory", func(ctx context.Context, next string) (*asc.SubscriptionPricePointsResponse, error) {
			if next != "" {
				return c.GetSubscriptionPricePointEqualizations(ctx, base, asc.WithSubscriptionPricePointsNextURL(next))
			}
			return c.GetSubscriptionPricePointEqualizations(ctx, base, asc.WithSubscriptionPricePointsInclude([]string{"territory"}), asc.WithSubscriptionPricePointsLimit(200), asc.WithSubscriptionPricePointsNextURL(next))
		})
		if err != nil {
			return nil, err
		}
		for _, point := range equalized {
			territory, err := relationshipID(point.Relationships, "territory")
			if err != nil {
				return nil, err
			}
			if old, ok := points[territory]; ok && old != point.ID {
				return nil, fmt.Errorf("ambiguous equalization for %s", territory)
			}
			points[territory] = point.ID
		}
	}
	return points, nil
}

func planPricing(ctx context.Context, c *asc.Client, kind, key string, id *string, p *pricing, today string) (operation, error) {
	op := operation{item: asc.CatalogItem{Kind: kind + "Pricing", Key: key, Action: "create", Status: "pending", Changes: map[string]any{"pricing": p}}}
	if *id == "" {
		op.item.Action = "resolve"
		op.item.Deferred = true
	}
	var points map[string]string
	var err error
	if *id != "" {
		points, err = resolvePrices(ctx, c, kind, *id, p)
		if err != nil {
			return op, err
		}
		comparison, err := comparePricing(ctx, c, kind, *id, p, points, today)
		if err != nil {
			return op, err
		}
		if comparison.matches {
			op.item.Action = "noop"
			op.item.ID = *id
			op.item.Changes = nil
		} else if comparison.exists {
			op.item.Action = "update"
		}
		if !comparison.matches && comparison.replacementBlocked != "" {
			return op, fmt.Errorf("cannot replace %s pricing for %s: %s", kind, key, comparison.replacementBlocked)
		}
	}
	op.write = func(ctx context.Context) (string, error) {
		if points == nil {
			var err error
			points, err = resolvePrices(ctx, c, kind, *id, p)
			if err != nil {
				return "", err
			}
		}
		comparison, err := comparePricing(ctx, c, kind, *id, p, points, today)
		if err != nil {
			return "", err
		}
		if comparison.matches {
			return *id, nil
		}
		if comparison.replacementBlocked != "" {
			return "", fmt.Errorf("cannot replace %s pricing for %s: %s", kind, key, comparison.replacementBlocked)
		}
		territories := make([]string, 0, len(points))
		for t := range points {
			territories = append(territories, t)
		}
		slices.Sort(territories)
		if kind == "subscription" {
			entries := make([]asc.SubscriptionInlinePrice, 0, len(points))
			for _, t := range territories {
				entries = append(entries, asc.SubscriptionInlinePrice{PricePointID: points[t], TerritoryID: t, Attributes: asc.SubscriptionPriceCreateAttributes{StartDate: p.StartDate}})
			}
			r, err := c.SetSubscriptionPriceMatrix(ctx, *id, entries)
			if err != nil {
				return "", err
			}
			return r.Data.ID, nil
		}
		entries := make([]asc.InAppPurchasePriceSchedulePrice, 0, len(points))
		for _, t := range territories {
			entries = append(entries, asc.InAppPurchasePriceSchedulePrice{PricePointID: points[t], StartDate: p.StartDate})
		}
		r, err := c.CreateInAppPurchasePriceSchedule(ctx, *id, asc.InAppPurchasePriceScheduleCreateAttributes{BaseTerritoryID: p.BaseTerritory, Prices: entries})
		if err != nil {
			return "", err
		}
		return r.Data.ID, nil
	}
	return op, nil
}

type pricingComparison struct {
	matches            bool
	exists             bool
	replacementBlocked string
}

func comparePricing(ctx context.Context, c *asc.Client, kind, id string, p *pricing, points map[string]string, today string) (pricingComparison, error) {
	result := pricingComparison{}
	type currentPrice struct{ date, point string }
	current := map[string]currentPrice{}
	seenPeriods := map[string]bool{}
	seenTerritories := map[string]bool{}
	block := func(reason string) {
		if result.replacementBlocked == "" {
			result.replacementBlocked = reason
		}
	}
	add := func(territory, point, date, endDate string) error {
		for _, value := range []string{date, endDate} {
			if value != "" {
				if _, err := time.Parse(time.DateOnly, value); err != nil {
					return fmt.Errorf("invalid price date %q", value)
				}
			}
		}
		period := territory + "/" + date
		if seenPeriods[period] {
			return fmt.Errorf("ambiguous price records for %s at %q", territory, date)
		}
		seenPeriods[period] = true
		if seenTerritories[territory] {
			block("multiple price periods must be preserved")
		}
		seenTerritories[territory] = true
		if _, managed := points[territory]; !managed {
			block("prices outside the requested territories must be preserved")
		}
		if date > today {
			block("future prices must be preserved")
		}
		if endDate != "" {
			block("bounded price periods must be preserved")
		}
		if endDate != "" && endDate <= today {
			return nil
		}
		if p.StartDate != "" && date != p.StartDate {
			return nil
		}
		if p.StartDate == "" && date > today {
			return nil
		}
		if prior, ok := current[territory]; !ok || date > prior.date {
			current[territory] = currentPrice{date, point}
		}
		return nil
	}
	baseMatches := true
	if kind == "subscription" {
		const include = "territory,subscriptionPricePoint,additionalPriceTiers"
		prices, err := collect(ctx, "/v1/subscriptions/"+id+"/prices?include="+include, func(ctx context.Context, next string) (*asc.SubscriptionPricesResponse, error) {
			return c.GetSubscriptionPrices(ctx, id, asc.WithSubscriptionPricesInclude(strings.Split(include, ",")), asc.Limit[asc.SubscriptionPricesOption](200), asc.NextURL[asc.SubscriptionPricesOption](next))
		})
		if err != nil {
			return result, err
		}
		result.exists = len(prices) > 0
		for _, price := range prices {
			if price.Attributes.PlanType != "" || price.Attributes.Preserved {
				block("preserved or alternate-plan prices must be preserved")
				continue
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
				return result, fmt.Errorf("invalid additional price tiers relationship: %w", err)
			}
			tiers := relationships.AdditionalPriceTiers
			if tiers.Data == nil || len(tiers.Data) > 0 || tiers.Links.Next != "" || tiers.Meta.Paging.Total != 0 {
				block("additional price tiers must be preserved; complete empty tier linkage is required")
			}
			territory, err := relationshipID(price.Relationships, "territory")
			if err != nil {
				return result, err
			}
			point, err := relationshipID(price.Relationships, "subscriptionPricePoint")
			if err != nil {
				return result, err
			}
			if err := add(territory, point, price.Attributes.StartDate, ""); err != nil {
				return result, err
			}
		}
	} else {
		schedule, err := c.GetInAppPurchasePriceSchedule(ctx, id, asc.WithIAPPriceScheduleInclude([]string{"baseTerritory"}))
		if asc.IsMissingResourceOfType(err, "inAppPurchasePriceSchedules") {
			return result, nil
		}
		if err != nil {
			return result, err
		}
		result.exists = true
		if schedule.Data.ID == "" {
			return result, fmt.Errorf("price schedule has no resource ID")
		}
		base, err := relationshipID(schedule.Data.Relationships, "baseTerritory")
		if err != nil {
			return result, err
		}
		baseMatches = base == p.BaseTerritory
		if !baseMatches {
			block("changing the base territory changes automatic prices outside the manifest")
		}
		prices, err := collect(ctx, "/v1/inAppPurchasePriceSchedules/"+schedule.Data.ID+"/manualPrices?include=territory,inAppPurchasePricePoint", func(ctx context.Context, next string) (*asc.InAppPurchasePricesResponse, error) {
			return c.GetInAppPurchasePriceScheduleManualPrices(ctx, schedule.Data.ID, asc.WithIAPPriceSchedulePricesInclude([]string{"territory", "inAppPurchasePricePoint"}), asc.Limit[asc.IAPPriceSchedulePricesOption](200), asc.NextURL[asc.IAPPriceSchedulePricesOption](next))
		})
		if err != nil {
			return result, err
		}
		if len(prices) == 0 {
			block("existing schedule has no manual price rows; replacement scope is unknown")
		}
		for _, price := range prices {
			territory, err := relationshipID(price.Relationships, "territory")
			if err != nil {
				return result, err
			}
			point, err := relationshipID(price.Relationships, "inAppPurchasePricePoint")
			if err != nil {
				return result, err
			}
			if err := add(territory, point, price.Attributes.StartDate, price.Attributes.EndDate); err != nil {
				return result, err
			}
		}
	}
	if result.exists && p.StartDate > today {
		block("a scheduled price change must preserve the current schedule")
	}
	result.matches = baseMatches
	for territory, point := range points {
		if current[territory].point != point {
			result.matches = false
		}
	}
	if kind == "inAppPurchase" && len(current) != len(points) {
		result.matches = false
	}
	return result, nil
}

func planPricingItems(ctx context.Context, c *asc.Client, m manifest, ids map[string]*string, today string) ([]operation, error) {
	ops := []operation{}
	for _, p := range m.InAppPurchases {
		if p.Pricing != nil {
			op, err := planPricing(ctx, c, "inAppPurchase", p.ProductID, ids[p.ProductID], p.Pricing, today)
			if err != nil {
				return nil, err
			}
			ops = append(ops, op)
		}
	}
	for _, s := range m.Subscriptions {
		if s.Pricing != nil {
			op, err := planPricing(ctx, c, "subscription", s.ProductID, ids[s.ProductID], s.Pricing, today)
			if err != nil {
				return nil, err
			}
			ops = append(ops, op)
		}
	}
	return ops, nil
}
