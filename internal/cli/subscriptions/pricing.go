package subscriptions

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/ascterritory"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

const (
	defaultSubscriptionPricingWorkers = 4
	subscriptionPricingDateLayout     = "2006-01-02"
)

type subWithGroup struct {
	Sub       asc.Resource[asc.SubscriptionAttributes]
	GroupName string
}

type subscriptionPricingResult struct {
	Subscriptions []subscriptionPriceSummary `json:"subscriptions"`
}

type subscriptionPriceSummary struct {
	ID                 string    `json:"id"`
	Name               string    `json:"name"`
	ProductID          string    `json:"productId"`
	SubscriptionPeriod string    `json:"subscriptionPeriod,omitempty"`
	State              string    `json:"state,omitempty"`
	GroupName          string    `json:"groupName,omitempty"`
	CurrentPrice       *subMoney `json:"currentPrice,omitempty"`
	Proceeds           *subMoney `json:"proceeds,omitempty"`
	ProceedsYear2      *subMoney `json:"proceedsYear2,omitempty"`
}

type subMoney struct {
	Amount   string `json:"amount"`
	Currency string `json:"currency"`
}

// SubscriptionsPricingSummaryCommand returns the pricing summary subcommand.
func SubscriptionsPricingSummaryCommand() *ffcli.Command {
	return buildSubscriptionsPricingSummaryCommand(
		"summary",
		"asc subscriptions pricing summary [flags]",
		"Show consolidated subscription pricing summary.",
		`Show consolidated subscription pricing summary.

Returns current price, proceeds, and proceeds year 2 for each subscription
in the specified territory. Much faster than paginating through all 140K+
price points.

Examples:
  asc subscriptions pricing summary --app "APP_ID"
  asc subscriptions pricing summary --subscription-id "SUB_ID"
  asc subscriptions pricing summary --app "APP_ID" --territory "United States" --output table`,
	)
}

func buildSubscriptionsPricingSummaryCommand(
	name string,
	shortUsage string,
	shortHelp string,
	longHelp string,
) *ffcli.Command {
	fs := flag.NewFlagSet(name, flag.ExitOnError)

	appID := fs.String("app", "", subscriptionLookupAppUsage)
	subscriptionID := shared.BindResourceIDFlag(fs, "subscription-id", "subscriptions", "Subscription ID, product ID, or exact current name")
	territory := fs.String("territory", "USA", "Territory for pricing (accepts alpha-2, alpha-3, or exact English country name)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       name,
		ShortUsage: shortUsage,
		ShortHelp:  shortHelp,
		LongHelp:   longHelp,
		FlagSet:    fs,
		UsageFunc:  shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			requestedSubID := strings.TrimSpace(*subscriptionID)
			requestedAppID := strings.TrimSpace(*appID)
			resolvedAppID := shared.ResolveAppID(requestedAppID)
			if requestedSubID == "" && resolvedAppID == "" {
				fmt.Fprintln(os.Stderr, "Error: --app or --subscription-id is required")
				return shared.MissingRequiredUsageError("")
			}

			territoryInput := strings.TrimSpace(*territory)
			if territoryInput == "" {
				territoryInput = "USA"
			}
			territoryFilter, err := ascterritory.Normalize(territoryInput)
			if err != nil {
				return shared.UsageError(err.Error())
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("subscriptions pricing: %w", err)
			}

			var subs []subWithGroup

			if requestedSubID != "" {
				resolveCtx, resolveCancel := shared.ContextWithTimeout(ctx)
				requestedSubID, err = resolveSubscriptionLookupID(resolveCtx, client, requestedAppID, requestedSubID)
				resolveCancel()
				if err != nil {
					return err
				}

				subCtx, subCancel := shared.ContextWithTimeout(ctx)
				resp, err := client.GetSubscription(subCtx, requestedSubID)
				subCancel()
				if err != nil {
					return fmt.Errorf("subscriptions pricing: failed to fetch subscription: %w", err)
				}
				subs = []subWithGroup{{Sub: resp.Data, GroupName: ""}}
			} else {
				groupsCtx, groupsCancel := shared.ContextWithTimeout(ctx)
				groupsResp, err := client.GetSubscriptionGroups(groupsCtx, resolvedAppID, asc.Limit[asc.SubscriptionGroupsOption](200))
				groupsCancel()
				if err != nil {
					return fmt.Errorf("subscriptions pricing: failed to fetch groups: %w", err)
				}

				paginatedGroups, err := asc.PaginateAll(ctx, groupsResp, func(_ context.Context, nextURL string) (asc.PaginatedResponse, error) {
					pageCtx, pageCancel := shared.ContextWithTimeout(ctx)
					defer pageCancel()
					return client.GetSubscriptionGroups(pageCtx, resolvedAppID, asc.NextURL[asc.SubscriptionGroupsOption](nextURL))
				})
				if err != nil {
					return fmt.Errorf("subscriptions pricing: paginate groups: %w", err)
				}

				groups, ok := paginatedGroups.(*asc.SubscriptionGroupsResponse)
				if !ok {
					return fmt.Errorf("subscriptions pricing: unexpected groups response type %T", paginatedGroups)
				}

				subs, err = fetchSubscriptionPricingGroups(ctx, client, groups.Data)
				if err != nil {
					return err
				}
			}

			if len(subs) == 0 {
				return printSubscriptionPricingResult(&subscriptionPricingResult{Subscriptions: []subscriptionPriceSummary{}}, *output.Output, *output.Pretty)
			}

			summaries, err := resolveSubscriptionPriceSummaries(ctx, client, subs, territoryFilter)
			if err != nil {
				return fmt.Errorf("subscriptions pricing: %w", err)
			}

			return printSubscriptionPricingResult(&subscriptionPricingResult{Subscriptions: summaries}, *output.Output, *output.Pretty)
		},
	}
}

func fetchSubscriptionPricingGroup(ctx context.Context, client *asc.Client, group asc.Resource[asc.SubscriptionGroupAttributes]) ([]subWithGroup, error) {
	subsCtx, subsCancel := shared.ContextWithTimeout(ctx)
	subsResp, err := client.GetSubscriptions(subsCtx, group.ID, asc.Limit[asc.SubscriptionsOption](200))
	subsCancel()
	if err != nil {
		return nil, fmt.Errorf("subscriptions pricing: failed to fetch subscriptions for group %s: %w", group.ID, err)
	}
	paginatedSubs, err := asc.PaginateAll(ctx, subsResp, func(pageParent context.Context, nextURL string) (asc.PaginatedResponse, error) {
		pageCtx, pageCancel := shared.ContextWithTimeout(pageParent)
		defer pageCancel()
		return client.GetSubscriptions(pageCtx, group.ID, asc.NextURL[asc.SubscriptionsOption](nextURL))
	})
	if err != nil {
		return nil, fmt.Errorf("subscriptions pricing: paginate subscriptions: %w", err)
	}
	subsResult, ok := paginatedSubs.(*asc.SubscriptionsResponse)
	if !ok {
		return nil, fmt.Errorf("subscriptions pricing: unexpected subscriptions response type %T", paginatedSubs)
	}
	result := make([]subWithGroup, 0, len(subsResult.Data))
	for _, sub := range subsResult.Data {
		result = append(result, subWithGroup{Sub: sub, GroupName: group.Attributes.ReferenceName})
	}
	return result, nil
}

func fetchSubscriptionPricingGroups(ctx context.Context, client *asc.Client, groups []asc.Resource[asc.SubscriptionGroupAttributes]) ([]subWithGroup, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	type groupResult struct {
		items []subWithGroup
		err   error
	}
	var results []subWithGroup
	// Enumeration finishes before price summaries start, keeping both phases
	// within the same request budget. Consume each wave in group order so
	// output and the first reported error do not depend on response timing.
	for start := 0; start < len(groups); start += defaultSubscriptionPricingWorkers {
		end := min(start+defaultSubscriptionPricingWorkers, len(groups))
		waveCtx, cancel := context.WithCancel(ctx)
		completions := make([]chan groupResult, end-start)
		var wg sync.WaitGroup
		for index := range completions {
			completions[index] = make(chan groupResult, 1)
			wg.Go(func() {
				items, err := fetchSubscriptionPricingGroup(waveCtx, client, groups[start+index])
				completions[index] <- groupResult{items: items, err: err}
			})
		}
		for _, completion := range completions {
			result := <-completion
			if result.err != nil {
				// Cancel and join peers as soon as the earliest ordered error
				// is known; waiting for the whole wave first can stall here.
				cancel()
				wg.Wait()
				return nil, result.err
			}
			results = append(results, result.items...)
		}
		cancel()
		wg.Wait()
	}
	return results, nil
}

func resolveSubscriptionPriceSummaries(
	ctx context.Context,
	client *asc.Client,
	subs []subWithGroup,
	territory string,
) ([]subscriptionPriceSummary, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("context cancelled: %w", err)
	}

	if len(subs) == 0 {
		return []subscriptionPriceSummary{}, nil
	}

	workers := max(min(len(subs), defaultSubscriptionPricingWorkers), 1)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	sem := make(chan struct{}, workers)
	results := make([]subscriptionPriceSummary, len(subs))
	errs := make(chan error, len(subs))
	var once sync.Once
	var wg sync.WaitGroup

	for idx := range subs {
		wg.Go(func() {
			select {
			case sem <- struct{}{}:
			case <-ctx.Done():
				return
			}
			defer func() { <-sem }()

			summary, err := resolveSubscriptionPriceSummary(ctx, client, subs[idx], territory)
			if err != nil {
				once.Do(cancel)
				errs <- fmt.Errorf("resolve %s: %w", subs[idx].Sub.ID, err)
				return
			}
			results[idx] = summary
		})
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		if err != nil {
			return nil, err
		}
	}

	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("context cancelled: %w", err)
	}

	return results, nil
}

func resolveSubscriptionPriceSummary(
	ctx context.Context,
	client *asc.Client,
	sub subWithGroup,
	territory string,
) (subscriptionPriceSummary, error) {
	summary := subscriptionPriceSummary{
		ID:                 sub.Sub.ID,
		Name:               sub.Sub.Attributes.Name,
		ProductID:          sub.Sub.Attributes.ProductID,
		SubscriptionPeriod: sub.Sub.Attributes.SubscriptionPeriod,
		State:              sub.Sub.Attributes.State,
		GroupName:          sub.GroupName,
	}

	// The endpoint returns the territory's full price history unsorted, so every
	// page must be read before the current price can be chosen.
	pricesCtx, pricesCancel := shared.ContextWithTimeout(ctx)
	firstPage, err := client.GetSubscriptionPrices(
		pricesCtx,
		sub.Sub.ID,
		asc.WithSubscriptionPricesTerritory(territory),
		asc.WithSubscriptionPricesPlanType(asc.SubscriptionPlanTypeUpfront),
		asc.WithSubscriptionPricesInclude([]string{"subscriptionPricePoint", "territory"}),
		asc.WithSubscriptionPricesPricePointFields([]string{"customerPrice", "proceeds", "proceedsYear2"}),
		asc.WithSubscriptionPricesTerritoryFields([]string{"currency"}),
		asc.Limit[asc.SubscriptionPricesOption](200),
	)
	pricesCancel()
	if err != nil {
		return summary, fmt.Errorf("fetch prices: %w", err)
	}
	allPages, err := asc.PaginateAll(ctx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
		pageCtx, pageCancel := shared.ContextWithTimeout(ctx)
		defer pageCancel()
		return client.GetSubscriptionPrices(pageCtx, sub.Sub.ID, asc.NextURL[asc.SubscriptionPricesOption](nextURL))
	})
	if err != nil {
		return summary, fmt.Errorf("fetch prices: %w", err)
	}
	pricesResp, ok := allPages.(*asc.SubscriptionPricesResponse)
	if !ok {
		return summary, fmt.Errorf("fetch prices: unexpected response type %T", allPages)
	}

	// Parse the included resources for price point values and territory currencies
	pricePointValues, currencies := parseSubscriptionPricesIncluded(pricesResp.Included)

	// Find the currency for the target territory
	currency := currencies[strings.ToUpper(territory)]
	if currency == "" {
		currency = territoryToCurrency(territory)
	}

	if value, ok := selectCurrentSubscriptionPriceValue(pricesResp.Data, pricePointValues, shared.PricingNow()); ok {
		if value.CustomerPrice != "" {
			summary.CurrentPrice = &subMoney{Amount: value.CustomerPrice, Currency: currency}
		}
		if value.Proceeds != "" {
			summary.Proceeds = &subMoney{Amount: value.Proceeds, Currency: currency}
		}
		if value.ProceedsYear2 != "" {
			summary.ProceedsYear2 = &subMoney{Amount: value.ProceedsYear2, Currency: currency}
		}
	}

	return summary, nil
}

type subscriptionPricePointValue struct {
	CustomerPrice string
	Proceeds      string
	ProceedsYear2 string
}

type subscriptionPriceCandidate struct {
	value     subscriptionPricePointValue
	startAt   *time.Time
	preserved bool
}

func selectCurrentSubscriptionPriceValue(
	prices []asc.Resource[asc.SubscriptionPriceAttributes],
	pricePointValues map[string]subscriptionPricePointValue,
	now time.Time,
) (subscriptionPricePointValue, bool) {
	today := shared.PricingDate(now)

	var bestCurrent *subscriptionPriceCandidate
	var bestFuture *subscriptionPriceCandidate
	var bestUndated *subscriptionPriceCandidate

	for _, price := range prices {
		ppID := extractSubscriptionPricePointID(price)
		if ppID == "" {
			continue
		}

		value, ok := pricePointValues[ppID]
		if !ok {
			continue
		}

		candidate := subscriptionPriceCandidate{
			value:     value,
			startAt:   parseSubscriptionPricingDate(price.Attributes.StartDate),
			preserved: price.Attributes.Preserved,
		}

		if candidate.startAt == nil {
			if bestUndated == nil || (!candidate.preserved && bestUndated.preserved) {
				copyCandidate := candidate
				bestUndated = &copyCandidate
			}
			continue
		}

		if !shared.PriceActiveOn(candidate.startAt, nil, today) {
			if bestFuture == nil || candidate.startAt.Before(*bestFuture.startAt) || (candidate.startAt.Equal(*bestFuture.startAt) && !candidate.preserved && bestFuture.preserved) {
				copyCandidate := candidate
				bestFuture = &copyCandidate
			}
			continue
		}

		if bestCurrent == nil || candidate.startAt.After(*bestCurrent.startAt) || (candidate.startAt.Equal(*bestCurrent.startAt) && !candidate.preserved && bestCurrent.preserved) {
			copyCandidate := candidate
			bestCurrent = &copyCandidate
		}
	}

	switch {
	case bestCurrent != nil:
		return bestCurrent.value, true
	case bestUndated != nil:
		return bestUndated.value, true
	case bestFuture != nil:
		return bestFuture.value, true
	default:
		return subscriptionPricePointValue{}, false
	}
}

func parseSubscriptionPricingDate(value string) *time.Time {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	parsed, err := time.Parse(subscriptionPricingDateLayout, value)
	if err != nil {
		return nil
	}
	return &parsed
}

// subscriptionPricingToday returns today's App Store Connect pricing date, the
// US Pacific calendar day, from the clock pricing commands share.
//
// Subscription prices carry only a startDate, so each price ends on the date
// the next one starts: the latest price that has started on a pricing date is
// the one that applies, matching the exclusive end dates of app and in-app
// purchase schedules.
func subscriptionPricingToday() time.Time {
	return shared.PricingDate(shared.PricingNow())
}

func parseSubscriptionPricesIncluded(raw json.RawMessage) (map[string]subscriptionPricePointValue, map[string]string) {
	values := make(map[string]subscriptionPricePointValue)
	currencies := make(map[string]string)

	if len(raw) == 0 {
		return values, currencies
	}

	var included []struct {
		Type       string          `json:"type"`
		ID         string          `json:"id"`
		Attributes json.RawMessage `json:"attributes"`
	}
	if err := json.Unmarshal(raw, &included); err != nil {
		return values, currencies
	}

	for _, item := range included {
		switch item.Type {
		case "subscriptionPricePoints":
			var attrs asc.SubscriptionPricePointAttributes
			if err := json.Unmarshal(item.Attributes, &attrs); err != nil {
				continue
			}
			values[item.ID] = subscriptionPricePointValue{
				CustomerPrice: strings.TrimSpace(attrs.CustomerPrice),
				Proceeds:      strings.TrimSpace(attrs.Proceeds),
				ProceedsYear2: strings.TrimSpace(attrs.ProceedsYear2),
			}
		case "territories":
			var attrs struct {
				Currency string `json:"currency"`
			}
			if err := json.Unmarshal(item.Attributes, &attrs); err != nil {
				continue
			}
			if currency := strings.TrimSpace(attrs.Currency); currency != "" {
				currencies[strings.ToUpper(strings.TrimSpace(item.ID))] = currency
			}
		}
	}

	return values, currencies
}

func extractSubscriptionPricePointID(price asc.Resource[asc.SubscriptionPriceAttributes]) string {
	if price.Relationships == nil {
		return ""
	}

	var rels struct {
		SubscriptionPricePoint *asc.Relationship `json:"subscriptionPricePoint"`
	}

	rawRels, err := json.Marshal(price.Relationships)
	if err != nil {
		return ""
	}
	if err := json.Unmarshal(rawRels, &rels); err != nil {
		return ""
	}

	if rels.SubscriptionPricePoint == nil {
		return ""
	}

	return strings.TrimSpace(rels.SubscriptionPricePoint.Data.ID)
}

// territoryToCurrency maps common territories to their currency codes.
func territoryToCurrency(territory string) string {
	currencies := map[string]string{
		"USA": "USD", "CAN": "CAD", "GBR": "GBP", "AUS": "AUD",
		"JPN": "JPY", "DEU": "EUR", "FRA": "EUR", "ITA": "EUR",
		"ESP": "EUR", "NLD": "EUR", "BEL": "EUR", "AUT": "EUR",
		"FIN": "EUR", "GRC": "EUR", "IRL": "EUR", "PRT": "EUR",
		"CHN": "CNY", "KOR": "KRW", "BRA": "BRL", "MEX": "MXN",
		"IND": "INR", "RUS": "RUB", "CHE": "CHF", "SWE": "SEK",
		"NOR": "NOK", "DNK": "DKK", "POL": "PLN", "TUR": "TRY",
		"ZAF": "ZAR", "SGP": "SGD", "HKG": "HKD", "TWN": "TWD",
		"THA": "THB", "MYS": "MYR", "IDN": "IDR", "PHL": "PHP",
		"VNM": "VND", "NZL": "NZD", "SAU": "SAR", "ARE": "AED",
		"ISR": "ILS", "EGY": "EGP", "COL": "COP", "CHL": "CLP",
		"PER": "PEN", "ARG": "ARS",
	}
	if c, ok := currencies[strings.ToUpper(territory)]; ok {
		return c
	}
	return territory
}

func printSubscriptionPricingResult(result *subscriptionPricingResult, format string, pretty bool) error {
	return shared.PrintOutputWithRenderers(
		result,
		format,
		pretty,
		func() error { return printSubscriptionPricingTable(result) },
		func() error { return printSubscriptionPricingMarkdown(result) },
	)
}

func printSubscriptionPricingTable(result *subscriptionPricingResult) error {
	headers := []string{"ID", "Name", "Product ID", "Period", "State", "Group", "Current Price", "Proceeds", "Proceeds Y2"}
	rows := make([][]string, 0, len(result.Subscriptions))
	for _, item := range result.Subscriptions {
		rows = append(rows, []string{
			item.ID,
			compactSubText(item.Name),
			item.ProductID,
			item.SubscriptionPeriod,
			item.State,
			compactSubText(item.GroupName),
			formatSubMoney(item.CurrentPrice),
			formatSubMoney(item.Proceeds),
			formatSubMoney(item.ProceedsYear2),
		})
	}
	asc.RenderTable(headers, rows)
	return nil
}

func printSubscriptionPricingMarkdown(result *subscriptionPricingResult) error {
	headers := []string{"ID", "Name", "Product ID", "Period", "State", "Group", "Current Price", "Proceeds", "Proceeds Y2"}
	rows := make([][]string, 0, len(result.Subscriptions))
	for _, item := range result.Subscriptions {
		rows = append(rows, []string{
			item.ID,
			compactSubText(item.Name),
			item.ProductID,
			item.SubscriptionPeriod,
			item.State,
			compactSubText(item.GroupName),
			formatSubMoney(item.CurrentPrice),
			formatSubMoney(item.Proceeds),
			formatSubMoney(item.ProceedsYear2),
		})
	}
	asc.RenderMarkdown(headers, rows)
	return nil
}

func formatSubMoney(value *subMoney) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(strings.TrimSpace(value.Amount) + " " + strings.TrimSpace(value.Currency))
}

func compactSubText(value string) string {
	return strings.Join(strings.Fields(value), " ")
}
