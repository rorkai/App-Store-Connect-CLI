package subscriptions

import (
	"context"
	"flag"
	"fmt"
	"math/big"
	"os"
	"sort"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/ascterritory"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/pppindex"
)

// SubscriptionsPricingPPPCommand returns the ppp subcommand.
func SubscriptionsPricingPPPCommand() *ffcli.Command {
	fs := flag.NewFlagSet("subscriptions pricing ppp", flag.ExitOnError)

	subscriptionID := shared.BindResourceIDFlag(fs, "subscription-id", "subscriptions", "Subscription ID, product ID, or exact current name (required)")
	appID := addSubscriptionLookupAppFlag(fs)
	baseTerritory := fs.String("base-territory", "USA", "Base territory (accepts alpha-2, alpha-3, or exact English country name)")
	basePrice := fs.String("base-price", "", "Customer price in the base territory (required)")
	rounding := fs.String("round", string(subscriptionPriceDeriveNearest), "Price-point resolution: exact, nearest, up, or down")
	startDate := fs.String("start-date", "", "Start date (YYYY-MM-DD) for scheduled price changes")
	preserved := fs.Bool("preserved", false, "Preserve current prices for existing subscribers")
	autoStartDate := fs.Bool("auto-start-date", true, "Automatically schedule approved/live subscriptions for tomorrow when --start-date is omitted")
	dryRun := fs.Bool("dry-run", false, "Show the purchasing-power plan without applying it")
	confirm := fs.Bool("confirm", false, "Confirm applying the purchasing-power plan (required unless --dry-run)")
	workers := fs.Int("workers", defaultEqualizeWorkers, "Number of concurrent planning workers and price mutations")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "ppp",
		ShortUsage: "asc subscriptions pricing ppp [flags]",
		ShortHelp:  "Plan and apply purchasing-power-parity prices from a base price.",
		LongHelp: `Plan and apply purchasing-power-parity prices from a base price.

Finds the subscription price point for --base-price in --base-territory and
Apple's equalized price for it in every territory the subscription is priced
in. Each equalized price is multiplied by the territory's price level divided
by the base territory's price level (rounded to 4 decimals), then resolved to
the subscription's Apple price-point ladder with --round. The plan reports the
equalized, desired, current, and selected price for every territory.

Rounding modes:
  exact    Require the calculated price to exist.
  nearest  Choose the closest price; exact ties choose the lower price.
  up       Choose the lowest price greater than or equal to the calculation.
  down     Choose the highest price less than or equal to the calculation.

Price levels ship with the CLI: World Bank World Development Indicators PPP
conversion factor, GDP (PA.NUS.PPP) divided by the official exchange rate
(PA.NUS.FCRF), latest year from 2021 through 2025, United States = 1.
License: CC BY 4.0. Series last updated ` + pppindex.AsOf + `.
Territories without a price level are skipped and keep their current price.

The subscription must already have UPFRONT prices; only territories it is
priced in are planned. Sale availability is not changed. --confirm builds a
fresh plan, applies changed territories, and reads the prices back.

Examples:
  asc subscriptions pricing ppp --subscription-id "SUB_ID" --base-price "9.99" --dry-run
  asc subscriptions pricing ppp --subscription-id "SUB_ID" --base-price "9.99" --round down --dry-run --output table
  asc subscriptions pricing ppp --subscription-id "SUB_ID" --base-price "9.99" --confirm
  asc subscriptions pricing ppp --subscription-id "SUB_ID" --base-territory "GBR" --base-price "7.99" --start-date "YYYY-MM-DD" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return shared.UsageError("subscriptions pricing ppp does not accept positional arguments")
			}
			subID := strings.TrimSpace(*subscriptionID)
			if subID == "" {
				fmt.Fprintln(os.Stderr, "Error: --subscription-id is required")
				return shared.MissingRequiredUsageError("--subscription-id")
			}
			price := strings.TrimSpace(*basePrice)
			if price == "" {
				fmt.Fprintln(os.Stderr, "Error: --base-price is required")
				return shared.MissingRequiredUsageError("--base-price")
			}
			basePriceValue, ok := new(big.Rat).SetString(price)
			if !isSubscriptionPriceDeriveDecimal(price) || !ok || basePriceValue.Sign() <= 0 {
				return shared.UsageError("--base-price must be a positive decimal")
			}
			territory, err := ascterritory.Normalize(*baseTerritory)
			if err != nil {
				return shared.UsageError(fmt.Sprintf("invalid --base-territory: %v", err))
			}
			baseLevel, indexed := pppindex.PriceLevel(territory)
			if !indexed {
				return shared.UsageError(fmt.Sprintf("invalid --base-territory: no purchasing-power price level for %s", territory))
			}
			roundMode, err := normalizeSubscriptionPriceDeriveRounding(*rounding)
			if err != nil {
				return err
			}
			if *workers < 1 || *workers > 32 {
				return shared.UsageError("--workers must be between 1 and 32")
			}
			if *dryRun && *confirm {
				return shared.UsageError("--dry-run cannot be combined with --confirm")
			}
			if !*dryRun && !*confirm {
				return shared.UsageError("--confirm is required unless --dry-run is set")
			}
			explicitStartDate, effectiveAt, err := normalizeEqualizeStartDate(*startDate)
			if err != nil {
				return err
			}
			if err := shared.RequireAppForStableSelector(shared.ResolveAppID(*appID), subID, "--subscription-id"); err != nil {
				return err
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("subscriptions pricing ppp: %w", err)
			}
			subID, err = resolveSubscriptionLookupIDWithTimeout(ctx, client, *appID, subID)
			if err != nil {
				return err
			}

			resolvedStartDate := explicitStartDate
			subscriptionState := ""
			autoScheduled := false
			if resolvedStartDate == "" && *autoStartDate {
				var scheduleErr error
				resolvedStartDate, subscriptionState, autoScheduled, effectiveAt, scheduleErr = autoScheduleEqualizeStartDate(ctx, client, subID)
				if scheduleErr != nil {
					return fmt.Errorf("subscriptions pricing ppp: %w", scheduleErr)
				}
				if autoScheduled {
					fmt.Fprintf(os.Stderr, "Subscription state is %s; scheduling price changes for %s\n", subscriptionState, resolvedStartDate)
				}
			}

			fmt.Fprintln(os.Stderr, "Fetching current subscription prices...")
			currentPrices, err := fetchResolvedSubscriptionPrices(
				ctx, client, subID, 200, "", effectiveAt, asc.SubscriptionPlanTypeUpfront, "",
			)
			if err != nil {
				return fmt.Errorf("subscriptions pricing ppp: fetch current prices: %w", err)
			}
			if len(currentPrices.Prices) == 0 {
				return fmt.Errorf("subscriptions pricing ppp: subscription has no current UPFRONT prices; initialize its pricing with `asc subscriptions pricing equalize` first")
			}

			fmt.Fprintf(os.Stderr, "Finding %s price point for %s...\n", territory, price)
			baseCandidates, err := fetchSubscriptionPriceDeriveCandidatesByTerritory(ctx, client, subID, []string{territory})
			if err != nil {
				return fmt.Errorf("subscriptions pricing ppp: %w", err)
			}
			baseResolution, err := resolveSubscriptionPriceDerivePoint(baseCandidates[territory], basePriceValue, subscriptionPriceDeriveExact)
			if err != nil {
				return fmt.Errorf("subscriptions pricing ppp: --base-price %s in %s: %w", price, territory, err)
			}
			basePricePointID := baseResolution.PricePointID
			price = baseResolution.CustomerPrice
			fmt.Fprintln(os.Stderr, "Fetching equalized prices for all territories...")
			equalizations, err := fetchEqualizations(ctx, client, basePricePointID, territory)
			if err != nil {
				return fmt.Errorf("subscriptions pricing ppp: %w", err)
			}
			equalizedByTerritory := make(map[string]equalization, len(equalizations)+1)
			equalizedByTerritory[territory] = equalization{Territory: territory, Price: price, PricePointID: basePricePointID}
			for _, item := range equalizations {
				equalizedByTerritory[strings.ToUpper(strings.TrimSpace(item.Territory))] = item
			}

			sourcePrices := &shared.ResolvedPricesResult{}
			var unequalized []asc.SubscriptionPricingDeriveRow
			for _, current := range currentPrices.Prices {
				currentTerritory := strings.ToUpper(strings.TrimSpace(current.Territory))
				equalized, ok := equalizedByTerritory[currentTerritory]
				if !ok {
					row := asc.SubscriptionPricingDeriveRow{
						Territory:            currentTerritory,
						Currency:             strings.TrimSpace(current.Currency),
						CurrentTargetPrice:   strings.TrimSpace(current.CustomerPrice),
						CurrentTargetPointID: strings.TrimSpace(current.PricePointID),
						Action:               "skip",
						Status:               "skipped",
					}
					if _, indexed := pppindex.PriceLevel(currentTerritory); indexed {
						row.Action = "error"
						row.Status = "unresolved"
						row.Error = fmt.Sprintf("%s: no equalized price for base price point %s", currentTerritory, basePricePointID)
					}
					unequalized = append(unequalized, row)
					continue
				}
				sourcePrices.Prices = append(sourcePrices.Prices, shared.ResolvedPriceRow{
					Territory:     currentTerritory,
					Currency:      current.Currency,
					CustomerPrice: equalized.Price,
					PricePointID:  equalized.PricePointID,
				})
			}

			fmt.Fprintf(os.Stderr, "Resolving price points for %d territor%s...\n", len(sourcePrices.Prices), pluralizeEqualizeTerritories(len(sourcePrices.Prices)))
			rows := buildSubscriptionPriceDerivePlan(
				ctx,
				client,
				subID,
				sourcePrices,
				currentPrices,
				func(target string) (*big.Rat, bool) { return subscriptionPricePPPMultiple(baseLevel, target) },
				roundMode,
				*workers,
			)
			rows = append(rows, unequalized...)
			sort.Slice(rows, func(i, j int) bool { return rows[i].Territory < rows[j].Territory })

			result := &asc.SubscriptionPricingPPPResult{
				SubscriptionID:   subID,
				BaseTerritory:    territory,
				BasePrice:        price,
				BasePricePointID: basePricePointID,
				Index: asc.SubscriptionPricingPPPIndex{
					Source:  pppindex.Source,
					License: pppindex.License,
					AsOf:    pppindex.AsOf,
				},
				Rounding:          string(roundMode),
				StartDate:         resolvedStartDate,
				AutoScheduled:     autoScheduled,
				Preserved:         *preserved,
				SubscriptionState: subscriptionState,
				DryRun:            *dryRun,
				Rows:              rows,
				Verification:      asc.SubscriptionPricingDeriveVerification{Status: "skipped"},
			}
			result.Summary = summarizeSubscriptionPriceDeriveRows(rows)

			if result.Summary.Unresolved > 0 {
				if err := shared.PrintOutput(result, *output.Output, *output.Pretty); err != nil {
					return err
				}
				return shared.NewReportedError(fmt.Errorf(
					"subscriptions pricing ppp: %d territor%s could not be resolved; no changes were applied",
					result.Summary.Unresolved,
					pluralizeEqualizeTerritories(result.Summary.Unresolved),
				))
			}
			if *dryRun {
				return shared.PrintOutput(result, *output.Output, *output.Pretty)
			}

			attrs := asc.SubscriptionPriceCreateAttributes{
				StartDate: result.StartDate,
				PlanType:  asc.SubscriptionPlanTypeUpfront,
			}
			if *preserved {
				attrs.Preserved = preserved
			}
			var verificationErr error
			result.Summary, result.Verification, verificationErr = applyAndVerifySubscriptionPriceDeriveRows(
				ctx, client, subID, result.Rows, *workers, attrs, effectiveAt, "", "territory price", "scheduled prices",
			)
			if err := shared.PrintOutput(result, *output.Output, *output.Pretty); err != nil {
				return err
			}
			if verificationErr != nil {
				return shared.NewReportedError(fmt.Errorf("subscriptions pricing ppp: %w", verificationErr))
			}
			return nil
		},
	}
}

func subscriptionPricePPPMultiple(baseLevel *big.Rat, territory string) (*big.Rat, bool) {
	level, ok := pppindex.PriceLevel(territory)
	if !ok {
		return nil, false
	}
	scaled := new(big.Rat).Quo(level, baseLevel)
	scaled.Mul(scaled, big.NewRat(10000, 1))
	twiceNumerator := new(big.Int).Mul(scaled.Num(), big.NewInt(2))
	twiceNumerator.Add(twiceNumerator, scaled.Denom())
	rounded := twiceNumerator.Quo(twiceNumerator, new(big.Int).Mul(scaled.Denom(), big.NewInt(2)))
	return new(big.Rat).SetFrac(rounded, big.NewInt(10000)), true
}
