package catalog

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/ascterritory"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

type introductoryOffer struct {
	Territory       string                        `json:"territory"`
	OfferMode       asc.SubscriptionOfferMode     `json:"offerMode"`
	Duration        asc.SubscriptionOfferDuration `json:"duration"`
	NumberOfPeriods int                           `json:"numberOfPeriods"`
	CustomerPrice   string                        `json:"customerPrice,omitempty"`
	StartDate       string                        `json:"startDate,omitempty"`
	EndDate         string                        `json:"endDate,omitempty"`
}

func validateOffers(offers []introductoryOffer) error {
	for i := range offers {
		o := &offers[i]
		territory, err := ascterritory.Normalize(o.Territory)
		if err != nil {
			return err
		}
		o.Territory = territory
		if !slices.Contains([]asc.SubscriptionOfferMode{"FREE_TRIAL", "PAY_AS_YOU_GO", "PAY_UP_FRONT"}, o.OfferMode) {
			return fmt.Errorf("introductoryOffers[%d].offerMode is invalid", i)
		}
		if !slices.Contains([]asc.SubscriptionOfferDuration{"THREE_DAYS", "ONE_WEEK", "TWO_WEEKS", "ONE_MONTH", "TWO_MONTHS", "THREE_MONTHS", "SIX_MONTHS", "ONE_YEAR"}, o.Duration) {
			return fmt.Errorf("introductoryOffers[%d].duration is invalid", i)
		}
		if o.NumberOfPeriods < 1 {
			return fmt.Errorf("introductoryOffers[%d].numberOfPeriods must be positive", i)
		}
		if o.OfferMode == asc.SubscriptionOfferModeFreeTrial {
			if o.CustomerPrice != "" {
				return fmt.Errorf("free trials must omit customerPrice")
			}
		} else if !shared.IsPriceDecimal(o.CustomerPrice) {
			return fmt.Errorf("paid offers require a decimal customerPrice")
		}
		for _, d := range []string{o.StartDate, o.EndDate} {
			if d != "" {
				if _, err := time.Parse(time.DateOnly, d); err != nil {
					return fmt.Errorf("introductory offer dates must be YYYY-MM-DD")
				}
			}
		}
		if o.StartDate != "" && o.EndDate != "" && o.EndDate < o.StartDate {
			return fmt.Errorf("introductory offer endDate precedes startDate")
		}
		for _, prior := range offers[:i] {
			if prior.Territory == o.Territory && (prior.EndDate == "" || o.StartDate == "" || prior.EndDate >= o.StartDate) && (o.EndDate == "" || prior.StartDate == "" || o.EndDate >= prior.StartDate) {
				return fmt.Errorf("overlapping introductory offers in %s", territory)
			}
		}
	}
	return nil
}

type existingOffer struct {
	id         string
	territory  string
	point      string
	attributes asc.SubscriptionIntroductoryOfferAttributes
}

func readOffers(ctx context.Context, c *asc.Client, id string) ([]existingOffer, error) {
	if id == "" {
		return nil, nil
	}
	rows, err := collect(ctx, "/v1/subscriptions/"+id+"/introductoryOffers?include=territory,subscriptionPricePoint", func(ctx context.Context, next string) (*asc.SubscriptionIntroductoryOffersResponse, error) {
		if next != "" {
			return c.GetSubscriptionIntroductoryOffers(ctx, id, asc.NextURL[asc.SubscriptionIntroductoryOffersOption](next))
		}
		return c.GetSubscriptionIntroductoryOffers(ctx, id, asc.WithSubscriptionIntroductoryOffersInclude([]string{"territory", "subscriptionPricePoint"}), asc.Limit[asc.SubscriptionIntroductoryOffersOption](200), asc.NextURL[asc.SubscriptionIntroductoryOffersOption](next))
	})
	if err != nil {
		return nil, err
	}
	offers := make([]existingOffer, 0, len(rows))
	for _, row := range rows {
		if row.Type != "subscriptionIntroductoryOffers" {
			return nil, fmt.Errorf("invalid introductory offer type")
		}
		territory, err := relationshipID(row.Relationships, "territory")
		if err != nil {
			return nil, err
		}
		offer := existingOffer{id: row.ID, territory: territory, attributes: row.Attributes}
		if row.Attributes.OfferMode != asc.SubscriptionOfferModeFreeTrial {
			offer.point, err = relationshipID(row.Relationships, "subscriptionPricePoint")
			if err != nil {
				return nil, err
			}
		}
		for _, date := range []string{row.Attributes.StartDate, row.Attributes.EndDate} {
			if date != "" {
				if _, err := time.Parse(time.DateOnly, date); err != nil {
					return nil, fmt.Errorf("invalid introductory offer date for %s", row.ID)
				}
			}
		}
		offers = append(offers, offer)
	}
	return offers, nil
}

func matchingOffer(existing []existingOffer, want introductoryOffer, today string) (*existingOffer, error) {
	var matched *existingOffer
	start := want.StartDate
	if start == "" {
		start = today
	}
	for _, old := range existing {
		if old.territory != want.Territory {
			continue
		}
		a := old.attributes
		startMatches := a.StartDate == want.StartDate || (want.StartDate == "" && a.StartDate <= today && (a.EndDate == "" || a.EndDate >= today))
		overlaps := (a.EndDate == "" || a.EndDate >= start) && (want.EndDate == "" || a.StartDate == "" || want.EndDate >= a.StartDate)
		if !startMatches && !overlaps {
			continue
		}
		if a.TargetSubscriptionPlanType != "" {
			return nil, fmt.Errorf("introductory offer %s has an unsupported plan scope", old.id)
		}
		if !startMatches {
			return nil, fmt.Errorf("introductory offer overlaps existing offer %s in %s", old.id, want.Territory)
		}
		if matched != nil {
			return nil, fmt.Errorf("ambiguous introductory offers in %s", want.Territory)
		}
		matched = &old
	}
	return matched, nil
}

func offerMatches(old existingOffer, want introductoryOffer, point string) bool {
	a := old.attributes
	return a.OfferMode == want.OfferMode && a.Duration == want.Duration && a.NumberOfPeriods == want.NumberOfPeriods && a.EndDate == want.EndDate && old.point == point
}

func planOffers(ctx context.Context, c *asc.Client, s subscription, id *string, today string) ([]operation, error) {
	if s.IntroductoryOffers == nil {
		return nil, nil
	}
	existing, err := readOffers(ctx, c, *id)
	if err != nil {
		return nil, err
	}
	ops := []operation{}
	remove := func(planned existingOffer) operation {
		return operation{item: asc.CatalogItem{Kind: "introductoryOffer", Key: s.ProductID + "/" + planned.id, ID: planned.id, Action: "delete", Status: "pending"}, destructive: true, write: func(ctx context.Context) (string, error) {
			actual, err := readOffers(ctx, c, *id)
			if err != nil {
				return "", err
			}
			for _, current := range actual {
				if current.id != planned.id {
					continue
				}
				if current != planned {
					return "", fmt.Errorf("introductory offer %s changed after planning; rerun to review its scope", planned.id)
				}
				return planned.id, c.DeleteSubscriptionIntroductoryOffer(ctx, planned.id)
			}
			return "", fmt.Errorf("introductory offer %s no longer belongs to subscription %s; rerun to reconcile", planned.id, *id)
		}}
	}
	if len(s.IntroductoryOffers) == 0 {
		for _, old := range existing {
			ops = append(ops, remove(old))
		}
		return ops, nil
	}
	for _, want := range s.IntroductoryOffers {
		old, err := matchingOffer(existing, want, today)
		if err != nil {
			return nil, err
		}
		point := ""
		if *id != "" && want.OfferMode != asc.SubscriptionOfferModeFreeTrial {
			point, err = resolvePrice(ctx, c, "subscription", *id, want.Territory, want.CustomerPrice)
			if err != nil {
				return nil, err
			}
		}
		op := operation{item: asc.CatalogItem{Kind: "introductoryOffer", Key: s.ProductID + "/" + want.Territory + "/" + want.StartDate, Action: "create", Status: "pending", Changes: map[string]any{"offer": want}}}
		if *id == "" && want.OfferMode != asc.SubscriptionOfferModeFreeTrial {
			op.item.Deferred = true
		}
		if old != nil {
			if offerMatches(*old, want, point) {
				op.item.Action = "noop"
				op.item.ID = old.id
				op.item.Changes = nil
			} else {
				ops = append(ops, remove(*old))
			}
		}
		op.write = func(ctx context.Context) (string, error) {
			if want.OfferMode != asc.SubscriptionOfferModeFreeTrial && point == "" {
				var err error
				point, err = resolvePrice(ctx, c, "subscription", *id, want.Territory, want.CustomerPrice)
				if err != nil {
					return "", err
				}
			}
			actual, err := readOffers(ctx, c, *id)
			if err != nil {
				return "", err
			}
			current, err := matchingOffer(actual, want, today)
			if err != nil {
				return "", err
			}
			if current != nil {
				if offerMatches(*current, want, point) {
					return current.id, nil
				}
				return "", fmt.Errorf("introductory offer changed after planning; rerun to review its scope")
			}
			r, err := c.CreateSubscriptionIntroductoryOffer(ctx, *id, asc.SubscriptionIntroductoryOfferCreateAttributes{StartDate: want.StartDate, EndDate: want.EndDate, Duration: want.Duration, OfferMode: want.OfferMode, NumberOfPeriods: want.NumberOfPeriods}, want.Territory, point)
			if err != nil {
				return "", err
			}
			return r.Data.ID, nil
		}
		ops = append(ops, op)
	}
	return ops, nil
}
