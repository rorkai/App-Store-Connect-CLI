package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"
)

// GetSubscriptions fetches subscription groups, then subscriptions for each group concurrently.
func (a *App) GetSubscriptions(appID string) (SubscriptionsResponse, error) {
	if strings.TrimSpace(appID) == "" {
		return SubscriptionsResponse{Error: "app ID is required"}, nil
	}
	ascPath, err := a.resolveASCPath()
	if err != nil {
		return SubscriptionsResponse{Error: err.Error()}, nil
	}
	ctx, cancel := context.WithTimeout(a.contextOrBackground(), 30*time.Second)
	defer cancel()
	return a.loadSubscriptions(ctx, ascPath, appID), nil
}

func (a *App) loadSubscriptions(ctx context.Context, ascPath string, appID string) SubscriptionsResponse {
	// Step 1: get groups
	out, err := a.runASCCombinedOutput(ctx, ascPath, "subscriptions", "groups", "list", "--app", appID, "--paginate", "--output", "json")
	if err != nil {
		return SubscriptionsResponse{Error: strings.TrimSpace(string(out))}
	}
	type groupItem struct {
		ID         string `json:"id"`
		Attributes struct {
			ReferenceName string `json:"referenceName"`
		} `json:"attributes"`
	}
	var groupEnv struct {
		Data []groupItem `json:"data"`
	}
	if json.Unmarshal(out, &groupEnv) != nil {
		return SubscriptionsResponse{Error: "failed to parse groups"}
	}

	groupSubscriptions := make([][]SubscriptionItem, len(groupEnv.Data))
	var (
		groupErr   string
		groupErrMu sync.Mutex
	)
	recordGroupErr := func(groupName, message string) {
		groupErrMu.Lock()
		defer groupErrMu.Unlock()
		if groupErr != "" {
			return
		}
		groupLabel := strings.TrimSpace(groupName)
		detail := strings.TrimSpace(message)
		if groupLabel == "" {
			groupLabel = "unknown group"
		}
		if detail == "" {
			groupErr = fmt.Sprintf("failed to load subscriptions for %s", groupLabel)
			return
		}
		groupErr = fmt.Sprintf("failed to load subscriptions for %s: %s", groupLabel, detail)
	}
	runWithConcurrency(boundedStudioConcurrency(len(groupEnv.Data)), len(groupEnv.Data), func(i int) {
		group := groupEnv.Data[i]
		out, err := a.runASCCombinedOutput(ctx, ascPath, "subscriptions", "list", "--group-id", group.ID, "--paginate", "--output", "json")
		if err != nil {
			recordGroupErr(group.Attributes.ReferenceName, string(out))
			return
		}
		type rawSub struct {
			ID         string `json:"id"`
			Attributes struct {
				Name               string `json:"name"`
				ProductID          string `json:"productId"`
				State              string `json:"state"`
				SubscriptionPeriod string `json:"subscriptionPeriod"`
				ReviewNote         string `json:"reviewNote"`
				GroupLevel         int    `json:"groupLevel"`
			} `json:"attributes"`
		}
		var env struct {
			Data []rawSub `json:"data"`
		}
		if json.Unmarshal(out, &env) != nil {
			recordGroupErr(group.Attributes.ReferenceName, "failed to parse response")
			return
		}
		items := make([]SubscriptionItem, 0, len(env.Data))
		for _, s := range env.Data {
			items = append(items, SubscriptionItem{
				ID:                 s.ID,
				GroupName:          group.Attributes.ReferenceName,
				Name:               s.Attributes.Name,
				ProductID:          s.Attributes.ProductID,
				State:              s.Attributes.State,
				SubscriptionPeriod: s.Attributes.SubscriptionPeriod,
				ReviewNote:         s.Attributes.ReviewNote,
				GroupLevel:         s.Attributes.GroupLevel,
			})
		}
		groupSubscriptions[i] = items
	})
	var all []SubscriptionItem
	for _, items := range groupSubscriptions {
		all = append(all, items...)
	}
	return SubscriptionsResponse{Subscriptions: all, Error: groupErr}
}

// GetPricingOverview fetches availability + subscription pricing summary in parallel.
func (a *App) GetPricingOverview(appID string) (PricingOverview, error) {
	if strings.TrimSpace(appID) == "" {
		return PricingOverview{Error: "app ID is required"}, nil
	}
	ascPath, err := a.resolveASCPath()
	if err != nil {
		return PricingOverview{Error: err.Error()}, nil
	}
	ctx, cancel := context.WithTimeout(a.contextOrBackground(), 30*time.Second)
	defer cancel()

	type availResult struct {
		available   bool
		territories []TerritoryAvailability
		err         error
	}
	type pricingResult struct {
		items []SubPricingItem
	}
	type priceResult struct {
		price    string
		proceeds string
		currency string
	}
	availCh := make(chan availResult, 1)
	pricingCh := make(chan pricingResult, 1)
	priceCh := make(chan priceResult, 1)

	// Current app price (first manual price -> decode base64 ID to get price point, then look up tier)
	go func() {
		scheduleID, err := a.fetchPricingScheduleID(ctx, ascPath, appID)
		if err != nil || scheduleID == "" {
			priceCh <- priceResult{}
			return
		}

		price, err := a.fetchCurrentAppPrice(ctx, ascPath, appID, scheduleID)
		if err != nil {
			priceCh <- priceResult{}
			return
		}
		priceCh <- priceResult{
			price:    price.Price,
			proceeds: price.Proceeds,
			currency: price.Currency,
		}
	}()

	// Availability + territories (sequential: need avail ID first, but it's the app ID)
	go func() {
		// 1. Get availability flag and resource ID
		out, err := a.runASCCombinedOutput(ctx, ascPath, "pricing", "availability", "view", "--app", appID, "--output", "json")
		if err != nil {
			availCh <- availResult{err: fmt.Errorf("%s", strings.TrimSpace(string(out)))}
			return
		}
		availabilityID, availableInNewTerritories, err := parseAvailabilityViewOutput(out)
		if err != nil {
			availCh <- availResult{err: fmt.Errorf("failed to parse availability: %w", err)}
			return
		}

		// 2. Get territory availabilities
		var territories []TerritoryAvailability
		if availabilityID != "" {
			out2, err := a.runASCCombinedOutput(ctx, ascPath, "pricing", "availability", "territory-availabilities",
				"--availability", availabilityID, "--paginate", "--output", "json")
			if err == nil {
				type rawTerrItem struct {
					Attributes struct {
						Available   bool   `json:"available"`
						ReleaseDate string `json:"releaseDate"`
					} `json:"attributes"`
					Relationships struct {
						Territory struct {
							Data struct {
								ID string `json:"id"`
							} `json:"data"`
						} `json:"territory"`
					} `json:"relationships"`
				}
				var terrEnv struct {
					Data []rawTerrItem `json:"data"`
				}
				if json.Unmarshal(out2, &terrEnv) == nil {
					for _, t := range terrEnv.Data {
						territories = append(territories, TerritoryAvailability{
							Territory:   t.Relationships.Territory.Data.ID,
							Available:   t.Attributes.Available,
							ReleaseDate: t.Attributes.ReleaseDate,
						})
					}
				}
			}
		}

		availCh <- availResult{
			available:   availableInNewTerritories,
			territories: territories,
		}
	}()

	// Subscription pricing summary
	go func() {
		out, err := a.runASCCombinedOutput(ctx, ascPath, "subscriptions", "pricing", "summary", "--app", appID, "--output", "json")
		if err != nil {
			pricingCh <- pricingResult{} // not an error -- app may have no subscriptions
			return
		}
		type rawSub struct {
			Name         string `json:"name"`
			ProductID    string `json:"productId"`
			Period       string `json:"subscriptionPeriod"`
			State        string `json:"state"`
			GroupName    string `json:"groupName"`
			CurrentPrice struct {
				Amount   string `json:"amount"`
				Currency string `json:"currency"`
			} `json:"currentPrice"`
			Proceeds struct {
				Amount string `json:"amount"`
			} `json:"proceeds"`
		}
		var env struct {
			Subscriptions []rawSub `json:"subscriptions"`
		}
		if json.Unmarshal(out, &env) != nil {
			pricingCh <- pricingResult{}
			return
		}
		items := make([]SubPricingItem, 0, len(env.Subscriptions))
		for _, s := range env.Subscriptions {
			items = append(items, SubPricingItem{
				Name:      s.Name,
				ProductID: s.ProductID,
				Period:    s.Period,
				State:     s.State,
				GroupName: s.GroupName,
				Price:     s.CurrentPrice.Amount,
				Currency:  s.CurrentPrice.Currency,
				Proceeds:  s.Proceeds.Amount,
			})
		}
		pricingCh <- pricingResult{items: items}
	}()

	avail := <-availCh
	pricing := <-pricingCh
	price := <-priceCh

	if avail.err != nil {
		return PricingOverview{Error: avail.err.Error()}, nil
	}

	return PricingOverview{
		AvailableInNewTerritories: avail.available,
		CurrentPrice:              price.price,
		CurrentProceeds:           price.proceeds,
		BaseCurrency:              price.currency,
		Territories:               avail.territories,
		SubscriptionPricing:       pricing.items,
	}, nil
}

// GetOfferCodes fetches offer codes for all subscriptions of an app concurrently.
func (a *App) GetOfferCodes(appID string) (OfferCodesResponse, error) {
	if strings.TrimSpace(appID) == "" {
		return OfferCodesResponse{Error: "app ID is required"}, nil
	}
	ascPath, err := a.resolveASCPath()
	if err != nil {
		return OfferCodesResponse{Error: err.Error()}, nil
	}
	ctx, cancel := context.WithTimeout(a.contextOrBackground(), 30*time.Second)
	defer cancel()

	// First get subscriptions to know which sub IDs to query
	subsResp := a.loadSubscriptions(ctx, ascPath, appID)
	if subsResp.Error != "" && len(subsResp.Subscriptions) == 0 {
		return OfferCodesResponse{Error: subsResp.Error}, nil
	}

	type offerResult struct {
		codes []OfferCode
	}
	offersBySubscription := make([]offerResult, len(subsResp.Subscriptions))
	offerErr := strings.TrimSpace(subsResp.Error)
	var offerErrMu sync.Mutex
	recordOfferErr := func(subscriptionName, message string) {
		offerErrMu.Lock()
		defer offerErrMu.Unlock()
		if offerErr != "" {
			return
		}
		subscriptionLabel := strings.TrimSpace(subscriptionName)
		detail := strings.TrimSpace(message)
		if subscriptionLabel == "" {
			subscriptionLabel = "unknown subscription"
		}
		if detail == "" {
			offerErr = fmt.Sprintf("failed to load offer codes for %s", subscriptionLabel)
			return
		}
		offerErr = fmt.Sprintf("failed to load offer codes for %s: %s", subscriptionLabel, detail)
	}
	runWithConcurrency(boundedStudioConcurrency(len(subsResp.Subscriptions)), len(subsResp.Subscriptions), func(i int) {
		sub := subsResp.Subscriptions[i]
		out, err := a.runASCCombinedOutput(ctx, ascPath, "subscriptions", "offers", "offer-codes", "list",
			"--subscription-id", sub.ID, "--paginate", "--output", "json")
		if err != nil {
			recordOfferErr(sub.Name, string(out))
			return
		}
		type rawCode struct {
			Attributes struct {
				Name                  string   `json:"name"`
				OfferEligibility      string   `json:"offerEligibility"`
				CustomerEligibilities []string `json:"customerEligibilities"`
				Duration              string   `json:"duration"`
				OfferMode             string   `json:"offerMode"`
				NumberOfPeriods       int      `json:"numberOfPeriods"`
				TotalNumberOfCodes    int      `json:"totalNumberOfCodes"`
				ProductionCodeCount   int      `json:"productionCodeCount"`
			} `json:"attributes"`
		}
		var env struct {
			Data []rawCode `json:"data"`
		}
		if json.Unmarshal(out, &env) != nil {
			recordOfferErr(sub.Name, "failed to parse response")
			return
		}
		codes := make([]OfferCode, 0, len(env.Data))
		for _, c := range env.Data {
			codes = append(codes, OfferCode{
				SubscriptionName: sub.Name,
				SubscriptionID:   sub.ID,
				Name:             c.Attributes.Name,
				Eligibility:      c.Attributes.OfferEligibility,
				Customers:        c.Attributes.CustomerEligibilities,
				Duration:         c.Attributes.Duration,
				OfferMode:        c.Attributes.OfferMode,
				Periods:          c.Attributes.NumberOfPeriods,
				TotalCodes:       c.Attributes.TotalNumberOfCodes,
				ProductionCodes:  c.Attributes.ProductionCodeCount,
			})
		}
		offersBySubscription[i] = offerResult{codes: codes}
	})

	var all []OfferCode
	for _, result := range offersBySubscription {
		all = append(all, result.codes...)
	}
	return OfferCodesResponse{OfferCodes: all, Error: offerErr}, nil
}

func (a *App) fetchPricingScheduleID(ctx context.Context, ascPath, appID string) (string, error) {
	out, err := a.runASCCombinedOutput(ctx, ascPath, "pricing", "schedule", "view", "--app", appID, "--output", "json")
	if err != nil {
		return "", err
	}
	return parseResourceIDOutput(out)
}

func (a *App) fetchSchedulePriceReference(ctx context.Context, ascPath, scheduleID, priceMode string) (appPriceReference, bool, error) {
	out, err := a.runASCCombinedOutput(ctx, ascPath, "pricing", "schedule", priceMode, "--schedule", scheduleID, "--output", "json")
	if err != nil {
		return appPriceReference{}, false, err
	}
	return parseFirstAppPriceReference(out)
}

func (a *App) fetchCurrentAppPrice(ctx context.Context, ascPath, appID, scheduleID string) (appPricePointLookupResult, error) {
	for _, priceMode := range []string{"manual-prices", "automatic-prices"} {
		ref, found, err := a.fetchSchedulePriceReference(ctx, ascPath, scheduleID, priceMode)
		if err != nil {
			continue
		}
		if !found {
			continue
		}

		out, err := a.runASCCombinedOutput(ctx, ascPath, "pricing", "price-points", "--app", appID, "--territory", ref.Territory, "--output", "json")
		if err != nil {
			return appPricePointLookupResult{}, err
		}

		price, matched, err := parseAppPricePointLookup(out, ref.Territory, ref.PricePoint)
		if err != nil {
			return appPricePointLookupResult{}, err
		}
		if matched {
			return price, nil
		}
	}

	return appPricePointLookupResult{}, nil
}
