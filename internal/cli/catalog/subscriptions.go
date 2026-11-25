package catalog

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

type subscription struct {
	IntroductoryOffers []introductoryOffer `json:"introductoryOffers,omitempty"`
	Group              string              `json:"group"`
	ProductID          string              `json:"productId"`
	Name               string              `json:"name"`
	SubscriptionPeriod string              `json:"subscriptionPeriod"`
	ReviewNote         *string             `json:"reviewNote,omitempty"`
	GroupLevel         *int                `json:"groupLevel,omitempty"`
	Availability       *availability       `json:"availability,omitempty"`
	Pricing            *pricing            `json:"pricing,omitempty"`
	Localizations      []localization      `json:"localizations,omitempty"`
}

func validateSubscriptions(m manifest, groups, products map[string]bool) error {
	for i, s := range m.Subscriptions {
		if err := validateOffers(s.IntroductoryOffers); err != nil {
			return err
		}
		if err := validateLocalizations("subscription", s.Localizations); err != nil {
			return err
		}
		if err := validatePricing(s.Pricing); err != nil {
			return fmt.Errorf("subscriptions[%d]: %w", i, err)
		}
		if err := validateAvailability(s.Availability); err != nil {
			return fmt.Errorf("subscriptions[%d]: %w", i, err)
		}
		if !groups[s.Group] {
			return fmt.Errorf("subscriptions[%d].group must reference a group in the manifest", i)
		}
		if !validProductID(s.ProductID) {
			return fmt.Errorf("subscriptions[%d].productId is invalid", i)
		}
		if products[s.ProductID] {
			return fmt.Errorf("duplicate productId %q", s.ProductID)
		}
		products[s.ProductID] = true
		if strings.TrimSpace(s.Name) == "" || utf8.RuneCountInString(s.Name) > 64 {
			return fmt.Errorf("subscriptions[%d].name must contain 1 to 64 characters", i)
		}
		if !slices.Contains([]string{"ONE_WEEK", "ONE_MONTH", "TWO_MONTHS", "THREE_MONTHS", "SIX_MONTHS", "ONE_YEAR"}, s.SubscriptionPeriod) {
			return fmt.Errorf("subscriptions[%d].subscriptionPeriod is invalid", i)
		}
		if s.GroupLevel != nil && *s.GroupLevel < 1 {
			return fmt.Errorf("subscriptions[%d].groupLevel must be positive", i)
		}
		if s.ReviewNote != nil && utf8.RuneCountInString(*s.ReviewNote) > 4000 {
			return fmt.Errorf("subscriptions[%d].reviewNote exceeds 4000 characters", i)
		}
	}
	return nil
}

func planSubscriptions(ctx context.Context, c *asc.Client, m manifest, groups map[string]*string, remoteGroups []asc.Resource[asc.SubscriptionGroupAttributes], productRefs map[string]*string) ([]operation, error) {
	if len(m.Subscriptions)+len(m.InAppPurchases) == 0 {
		return nil, nil
	}
	type existing struct {
		resource asc.Resource[asc.SubscriptionAttributes]
		groupID  string
	}
	byProduct := map[string]existing{}
	for _, g := range remoteGroups {
		resources, err := collect(ctx, "/v1/subscriptionGroups/"+g.ID+"/subscriptions", func(ctx context.Context, next string) (*asc.SubscriptionsResponse, error) {
			return c.GetSubscriptions(ctx, g.ID, asc.Limit[asc.SubscriptionsOption](200), asc.NextURL[asc.SubscriptionsOption](next))
		})
		if err != nil {
			return nil, err
		}
		for _, r := range resources {
			if r.Type != asc.ResourceTypeSubscriptions || r.ID == "" || r.Attributes.ProductID == "" {
				return nil, fmt.Errorf("subscription response is missing identity")
			}
			if _, ok := byProduct[r.Attributes.ProductID]; ok {
				return nil, fmt.Errorf("ambiguous productId %q", r.Attributes.ProductID)
			}
			byProduct[r.Attributes.ProductID] = existing{r, g.ID}
		}
	}
	for _, p := range m.InAppPurchases {
		if _, ok := byProduct[p.ProductID]; ok {
			return nil, fmt.Errorf("productId %q already belongs to a subscription", p.ProductID)
		}
	}
	ops := []operation{}
	for _, s := range m.Subscriptions {
		groupID := groups[s.Group]
		id := new(string)
		productRefs[s.ProductID] = id
		op := operation{item: asc.CatalogItem{Kind: "subscription", Key: s.ProductID, Action: "create", Status: "pending", Changes: map[string]any{"name": s.Name, "group": s.Group, "subscriptionPeriod": s.SubscriptionPeriod}}}
		attrs := asc.SubscriptionCreateAttributes{Name: s.Name, ProductID: s.ProductID, SubscriptionPeriod: s.SubscriptionPeriod, GroupLevel: s.GroupLevel}
		if s.ReviewNote != nil {
			attrs.ReviewNote = *s.ReviewNote
			op.item.Changes["reviewNote"] = *s.ReviewNote
		}
		if s.GroupLevel != nil {
			op.item.Changes["groupLevel"] = *s.GroupLevel
		}
		op.write = func(ctx context.Context) (string, error) {
			if *groupID == "" {
				return "", fmt.Errorf("group %q has no resolved ID", s.Group)
			}
			r, e := c.CreateSubscription(ctx, *groupID, attrs)
			if e != nil {
				return "", e
			}
			*id = r.Data.ID
			return r.Data.ID, nil
		}
		if old, ok := byProduct[s.ProductID]; ok {
			if old.groupID != *groupID {
				return nil, fmt.Errorf("productId %q already belongs to group %s; moving groups is unsupported", s.ProductID, old.groupID)
			}
			r := old.resource
			*id = r.ID
			op.item.ID = r.ID
			op.item.Action = "update"
			op.item.Changes = map[string]any{}
			update := asc.SubscriptionUpdateAttributes{}
			if r.Attributes.Name != s.Name {
				update.Name = &s.Name
				op.item.Changes["name"] = s.Name
			}
			if r.Attributes.SubscriptionPeriod != s.SubscriptionPeriod {
				update.SubscriptionPeriod = &s.SubscriptionPeriod
				op.item.Changes["subscriptionPeriod"] = s.SubscriptionPeriod
			}
			if s.GroupLevel != nil && r.Attributes.GroupLevel != *s.GroupLevel {
				update.GroupLevel = s.GroupLevel
				op.item.Changes["groupLevel"] = *s.GroupLevel
			}
			if s.ReviewNote != nil && r.Attributes.ReviewNote != *s.ReviewNote {
				update.ReviewNote = s.ReviewNote
				op.item.Changes["reviewNote"] = *s.ReviewNote
			}
			op.write = func(ctx context.Context) (string, error) {
				r, e := c.UpdateSubscription(ctx, r.ID, update)
				if e != nil {
					return "", e
				}
				return r.Data.ID, nil
			}
			if len(op.item.Changes) == 0 {
				op.item.Action = "noop"
			}
		}
		ops = append(ops, op)
	}
	return ops, nil
}
