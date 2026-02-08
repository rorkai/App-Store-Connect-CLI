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

// SubscriptionsPricingCommand returns a consolidated pricing summary command for subscriptions.
func SubscriptionsPricingCommand() *ffcli.Command {
	fs := flag.NewFlagSet("pricing", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	subscriptionID := fs.String("subscription-id", "", "Subscription ID")
	territory := fs.String("territory", "USA", "Territory for pricing (e.g., USA)")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "pricing",
		ShortUsage: "asc subscriptions pricing [flags]",
		ShortHelp:  "Show consolidated subscription pricing summary.",
		LongHelp: `Show consolidated subscription pricing summary.

Returns current price, proceeds, and proceeds year 2 for each subscription
in the specified territory. Much faster than paginating through all 140K+
price points.

Examples:
  asc subscriptions pricing --app "APP_ID"
  asc subscriptions pricing --subscription-id "SUB_ID"
  asc subscriptions pricing --app "APP_ID" --territory "USA" --output table`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			requestedSubID := strings.TrimSpace(*subscriptionID)
			requestedAppID := strings.TrimSpace(*appID)
			if requestedSubID == "" && shared.ResolveAppID(requestedAppID) == "" {
				fmt.Fprintln(os.Stderr, "Error: --app or --subscription-id is required")
				return flag.ErrHelp
			}
			if requestedSubID != "" && requestedAppID != "" {
				fmt.Fprintln(os.Stderr, "Error: --app and --subscription-id are mutually exclusive")
				return flag.ErrHelp
			}

			territoryFilter := strings.ToUpper(strings.TrimSpace(*territory))
			if territoryFilter == "" {
				territoryFilter = "USA"
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("subscriptions pricing: %w", err)
			}

			var subs []subWithGroup

			if requestedSubID != "" {
				subCtx, subCancel := shared.ContextWithTimeout(ctx)
				resp, err := client.GetSubscription(subCtx, requestedSubID)
				subCancel()
				if err != nil {
					return fmt.Errorf("subscriptions pricing: failed to fetch subscription: %w", err)
				}
				subs = []subWithGroup{{Sub: resp.Data, GroupName: ""}}
			} else {
				resolvedAppID := shared.ResolveAppID(requestedAppID)

				groupsCtx, groupsCancel := shared.ContextWithTimeout(ctx)
				groupsResp, err := client.GetSubscriptionGroups(groupsCtx, resolvedAppID, asc.WithSubscriptionGroupsLimit(200))
				groupsCancel()
				if err != nil {
					return fmt.Errorf("subscriptions pricing: failed to fetch groups: %w", err)
				}

				paginatedGroups, err := asc.PaginateAll(ctx, groupsResp, func(_ context.Context, nextURL string) (asc.PaginatedResponse, error) {
					pageCtx, pageCancel := shared.ContextWithTimeout(ctx)
					defer pageCancel()
					return client.GetSubscriptionGroups(pageCtx, resolvedAppID, asc.WithSubscriptionGroupsNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("subscriptions pricing: paginate groups: %w", err)
				}

				groups, ok := paginatedGroups.(*asc.SubscriptionGroupsResponse)
				if !ok {
					return fmt.Errorf("subscriptions pricing: unexpected groups response type %T", paginatedGroups)
				}

				for _, group := range groups.Data {
					subsCtx, subsCancel := shared.ContextWithTimeout(ctx)
					subsResp, err := client.GetSubscriptions(subsCtx, group.ID, asc.WithSubscriptionsLimit(200))
					subsCancel()
					if err != nil {
						return fmt.Errorf("subscriptions pricing: failed to fetch subscriptions for group %s: %w", group.ID, err)
					}

					paginatedSubs, err := asc.PaginateAll(ctx, subsResp, func(_ context.Context, nextURL string) (asc.PaginatedResponse, error) {
						pageCtx, pageCancel := shared.ContextWithTimeout(ctx)
						defer pageCancel()
						return client.GetSubscriptions(pageCtx, group.ID, asc.WithSubscriptionsNextURL(nextURL))
					})
					if err != nil {
						return fmt.Errorf("subscriptions pricing: paginate subscriptions: %w", err)
					}

					subsResult, ok := paginatedSubs.(*asc.SubscriptionsResponse)
					if !ok {
						return fmt.Errorf("subscriptions pricing: unexpected subscriptions response type %T", paginatedSubs)
					}

					groupName := group.Attributes.ReferenceName
					for _, sub := range subsResult.Data {
						subs = append(subs, subWithGroup{Sub: sub, GroupName: groupName})
					}
				}
			}

			if len(subs) == 0 {
				return printSubscriptionPricingResult(&subscriptionPricingResult{Subscriptions: []subscriptionPriceSummary{}}, *output, *pretty)
			}

			summaries, err := resolveSubscriptionPriceSummaries(ctx, client, subs, territoryFilter)
			if err != nil {
				return fmt.Errorf("subscriptions pricing: %w", err)
			}

			return printSubscriptionPricingResult(&subscriptionPricingResult{Subscriptions: summaries}, *output, *pretty)
		},
	}
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

	workers := defaultSubscriptionPricingWorkers
	if len(subs) < workers {
		workers = len(subs)
	}
	if workers < 1 {
		workers = 1
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	sem := make(chan struct{}, workers)
	results := make([]subscriptionPriceSummary, len(subs))
	errs := make(chan error, len(subs))
	var once sync.Once
	var wg sync.WaitGroup

	for idx := range subs {
		idx := idx
		wg.Add(1)
		go func() {
			defer wg.Done()
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
		}()
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

	// Use the subscription prices endpoint with include=subscriptionPricePoint,territory
	// and filter[territory]=<territory>. This returns just the current price assignment
	// for the target territory with the price point data included -- one API call total.
	pricesCtx, pricesCancel := shared.ContextWithTimeout(ctx)
	pricesResp, err := client.GetSubscriptionPrices(
		pricesCtx,
		sub.Sub.ID,
		asc.WithSubscriptionPricesTerritory(territory),
		asc.WithSubscriptionPricesInclude([]string{"subscriptionPricePoint", "territory"}),
		asc.WithSubscriptionPricesPricePointFields([]string{"customerPrice", "proceeds", "proceedsYear2"}),
		asc.WithSubscriptionPricesTerritoryFields([]string{"currency"}),
		asc.WithSubscriptionPricesLimit(10),
	)
	pricesCancel()
	if err != nil {
		return summary, fmt.Errorf("fetch prices: %w", err)
	}

	// Parse the included resources for price point values and territory currencies
	pricePointValues, currencies := parseSubscriptionPricesIncluded(pricesResp.Included)

	// Find the currency for the target territory
	currency := currencies[strings.ToUpper(territory)]
	if currency == "" {
		currency = territoryToCurrency(territory)
	}

	if value, ok := selectCurrentSubscriptionPriceValue(pricesResp.Data, pricePointValues, time.Now().UTC()); ok {
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
	asOf := dateOnlyUTC(now)

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

		if candidate.startAt.After(asOf) {
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
	normalized := dateOnlyUTC(parsed.UTC())
	return &normalized
}

func dateOnlyUTC(value time.Time) time.Time {
	return time.Date(value.UTC().Year(), value.UTC().Month(), value.UTC().Day(), 0, 0, 0, 0, time.UTC)
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
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "json":
		return shared.PrintOutput(result, "json", pretty)
	case "table":
		if pretty {
			return fmt.Errorf("--pretty is only valid with JSON output")
		}
		return printSubscriptionPricingTable(result)
	case "markdown", "md":
		if pretty {
			return fmt.Errorf("--pretty is only valid with JSON output")
		}
		return printSubscriptionPricingMarkdown(result)
	default:
		return fmt.Errorf("unsupported format: %s", format)
	}
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
