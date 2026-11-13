package subscriptions

import (
	"context"
	"flag"
	"fmt"
	"math/big"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/ascterritory"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// SubscriptionsPricingDeriveCommand returns the derive subcommand.
func SubscriptionsPricingDeriveCommand() *ffcli.Command {
	fs := flag.NewFlagSet("subscriptions pricing derive", flag.ExitOnError)

	sourceSubscriptionID := shared.BindResourceIDFlag(fs, "source-subscription-id", "subscriptions", "Source subscription ID, product ID, or exact current name (required)")
	targetSubscriptionID := shared.BindResourceIDFlag(fs, "target-subscription-id", "subscriptions", "Target subscription ID, product ID, or exact current name (required)")
	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env; required when either subscription selector is a product ID or name)")
	multiplier := fs.String("multiplier", "", "Positive decimal multiplied by each source territory price (required)")
	rounding := fs.String("round", string(shared.PriceRoundingNearest), "Price-point resolution: exact, nearest, up, or down")
	territory := fs.String("territory", "", "Limit derivation to one territory ID, 2-letter code, or name")
	startDate := fs.String("start-date", "", "Start date (YYYY-MM-DD) for scheduled target price changes")
	preserved := fs.Bool("preserved", false, "Preserve current target prices for existing subscribers")
	autoStartDate := fs.Bool("auto-start-date", true, "Automatically schedule approved/live target subscriptions for tomorrow when --start-date is omitted")
	dryRun := fs.Bool("dry-run", false, "Show derived target prices without applying them")
	confirm := fs.Bool("confirm", false, "Confirm applying derived target prices (required unless --dry-run)")
	workers := fs.Int("workers", defaultEqualizeWorkers, "Number of concurrent planning workers and price mutations")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "derive",
		ShortUsage: "asc subscriptions pricing derive [flags]",
		ShortHelp:  "Derive one subscription's territory prices from another.",
		LongHelp: `Derive one subscription's standard territory prices from another.

For every territory with a current source price, multiplies that local price by
--multiplier and resolves the result against the target subscription's Apple
price-point ladder. Apple price ladders are uneven, so the selected target and
achieved multiple are reported for every territory.

Rounding modes:
  exact    Require the calculated price to exist.
  nearest  Choose the closest price; exact ties choose the lower price.
  up       Choose the lowest price greater than or equal to the calculation.
  down     Choose the highest price less than or equal to the calculation.

The source and target must be distinct, already-priced subscriptions. This
command derives standard UPFRONT prices; Monthly with 12-Month Commitment plan
pricing is managed separately under subscriptions pricing monthly-commitment.
Use --territory to limit a preview or apply to one normalized territory.

Examples:
  asc subscriptions pricing derive --source-subscription-id "MONTHLY_ID" --target-subscription-id "YEARLY_ID" --multiplier "10" --dry-run
  asc subscriptions pricing derive --source-subscription-id "MONTHLY_ID" --target-subscription-id "YEARLY_ID" --multiplier "10" --round exact --dry-run
  asc subscriptions pricing derive --source-subscription-id "MONTHLY_ID" --target-subscription-id "YEARLY_ID" --multiplier "10" --territory "SWE" --dry-run
  asc subscriptions pricing derive --source-subscription-id "MONTHLY_ID" --target-subscription-id "YEARLY_ID" --multiplier "10" --round nearest --confirm
  asc subscriptions pricing derive --source-subscription-id "MONTHLY_ID" --target-subscription-id "YEARLY_ID" --multiplier "9.6" --start-date "YYYY-MM-DD" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			sourceID := strings.TrimSpace(*sourceSubscriptionID)
			if sourceID == "" {
				fmt.Fprintln(os.Stderr, "Error: --source-subscription-id is required")
				return shared.MissingRequiredUsageError("--source-subscription-id")
			}
			targetID := strings.TrimSpace(*targetSubscriptionID)
			if targetID == "" {
				fmt.Fprintln(os.Stderr, "Error: --target-subscription-id is required")
				return shared.MissingRequiredUsageError("--target-subscription-id")
			}
			if sourceID == targetID {
				return shared.UsageError("source and target subscriptions must be different")
			}

			multiplierValue, err := parseSubscriptionPriceDeriveMultiplier(*multiplier)
			if err != nil {
				return err
			}
			roundMode, err := shared.ParsePriceRounding(*rounding)
			if err != nil {
				return err
			}
			territoryID := ""
			if flagWasProvided(fs, "territory") && strings.TrimSpace(*territory) == "" {
				return shared.UsageError("invalid --territory: cannot be empty")
			}
			if strings.TrimSpace(*territory) != "" {
				territoryID, err = ascterritory.Normalize(*territory)
				if err != nil {
					return shared.UsageError(fmt.Sprintf("invalid --territory: %v", err))
				}
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
			resolvedAppID := shared.ResolveAppID(*appID)
			if err := shared.RequireAppForStableSelector(resolvedAppID, sourceID, "--source-subscription-id"); err != nil {
				return err
			}
			if err := shared.RequireAppForStableSelector(resolvedAppID, targetID, "--target-subscription-id"); err != nil {
				return err
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("subscriptions pricing derive: %w", err)
			}
			sourceID, err = resolveSubscriptionPriceDeriveLookupID(ctx, client, *appID, sourceID, "--source-subscription-id")
			if err != nil {
				return err
			}
			targetID, err = resolveSubscriptionPriceDeriveLookupID(ctx, client, *appID, targetID, "--target-subscription-id")
			if err != nil {
				return err
			}
			if sourceID == targetID {
				return shared.UsageError("source and target subscriptions must be different")
			}

			resolvedStartDate := explicitStartDate
			targetState := ""
			autoScheduled := false
			if resolvedStartDate == "" && *autoStartDate {
				var scheduleErr error
				resolvedStartDate, targetState, autoScheduled, effectiveAt, scheduleErr = autoScheduleEqualizeStartDate(ctx, client, targetID)
				if scheduleErr != nil {
					return fmt.Errorf("subscriptions pricing derive: %w", scheduleErr)
				}
				if autoScheduled {
					fmt.Fprintf(os.Stderr, "Target subscription state is %s; scheduling price changes for %s\n", targetState, resolvedStartDate)
				}
			}

			fmt.Fprintln(os.Stderr, "Fetching current source and target subscription prices...")
			sourcePrices, err := fetchResolvedSubscriptionPrices(
				ctx, client, sourceID, 200, "", subscriptionPricingToday(), asc.SubscriptionPlanTypeUpfront, territoryID,
			)
			if err != nil {
				return fmt.Errorf("subscriptions pricing derive: fetch source prices: %w", err)
			}
			if len(sourcePrices.Prices) == 0 {
				return fmt.Errorf("subscriptions pricing derive: source subscription has no current UPFRONT territory prices")
			}
			targetPrices, err := fetchResolvedSubscriptionPrices(
				ctx, client, targetID, 200, "", effectiveAt, asc.SubscriptionPlanTypeUpfront, territoryID,
			)
			if err != nil {
				return fmt.Errorf("subscriptions pricing derive: fetch target prices: %w", err)
			}
			targetHasPricing := len(targetPrices.Prices) > 0
			if !targetHasPricing && territoryID != "" {
				globalTargetPrices, preflightErr := fetchResolvedSubscriptionPrices(
					ctx, client, targetID, 200, "", effectiveAt, asc.SubscriptionPlanTypeUpfront, "",
				)
				if preflightErr != nil {
					return fmt.Errorf("subscriptions pricing derive: fetch target pricing preflight: %w", preflightErr)
				}
				targetHasPricing = len(globalTargetPrices.Prices) > 0
			}
			if !targetHasPricing {
				return fmt.Errorf("subscriptions pricing derive: target subscription has no current UPFRONT prices; initialize its pricing before deriving changes")
			}

			fmt.Fprintf(os.Stderr, "Resolving target price points for %d territor%s...\n", len(sourcePrices.Prices), pluralizeEqualizeTerritories(len(sourcePrices.Prices)))
			rows := buildSubscriptionPriceDerivePlan(
				ctx,
				client,
				targetID,
				sourcePrices,
				targetPrices,
				func(string) (*big.Rat, bool) { return multiplierValue, true },
				roundMode,
				*workers,
			)
			result := &asc.SubscriptionPricingDeriveResult{
				SourceSubscriptionID: sourceID,
				TargetSubscriptionID: targetID,
				Multiplier:           shared.FormatExactPriceDecimal(multiplierValue),
				Rounding:             string(roundMode),
				StartDate:            resolvedStartDate,
				AutoScheduled:        autoScheduled,
				Preserved:            *preserved,
				TargetState:          targetState,
				DryRun:               *dryRun,
				Rows:                 rows,
				Verification: asc.SubscriptionPricingDeriveVerification{
					Status: "skipped",
				},
			}
			result.Summary = shared.SummarizePriceLadderRows(rows)

			if result.Summary.Unresolved > 0 {
				if err := shared.PrintOutput(result, *output.Output, *output.Pretty); err != nil {
					return err
				}
				return shared.NewReportedError(fmt.Errorf(
					"subscriptions pricing derive: %d territor%s could not be resolved; no changes were applied",
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
				ctx, client, targetID, result.Rows, *workers, attrs, effectiveAt, territoryID, "derived territory price", "derived target prices",
			)
			if err := shared.PrintOutput(result, *output.Output, *output.Pretty); err != nil {
				return err
			}
			if verificationErr != nil || result.Summary.Failed > 0 {
				if verificationErr == nil {
					verificationErr = fmt.Errorf("%d territory update(s) failed", result.Summary.Failed)
				}
				return shared.NewReportedError(fmt.Errorf("subscriptions pricing derive: %w", verificationErr))
			}
			return nil
		},
	}
}

func resolveSubscriptionPriceDeriveLookupID(
	ctx context.Context,
	client *asc.Client,
	appValue string,
	selector string,
	flagName string,
) (string, error) {
	lookupCtx, cancel := shared.ContextWithTimeout(ctx)
	defer cancel()

	appID := shared.ResolveAppID(appValue)
	if err := shared.RequireAppForStableSelector(appID, selector, flagName); err != nil {
		return "", err
	}
	resolved, err := shared.ResolveSubscriptionID(lookupCtx, client, appID, selector)
	if err != nil {
		return "", shared.WithSelectorFlag(err, flagName)
	}
	return resolved, nil
}

func applyAndVerifySubscriptionPriceDeriveRows(
	ctx context.Context,
	client *asc.Client,
	targetSubscriptionID string,
	rows []asc.SubscriptionPricingDeriveRow,
	workers int,
	attrs asc.SubscriptionPriceCreateAttributes,
	effectiveAt time.Time,
	territory string,
	changeNoun string,
	verifyNoun string,
) (asc.SubscriptionPricingDeriveSummary, asc.SubscriptionPricingDeriveVerification, error) {
	planned := countSubscriptionPriceDeriveRows(rows, "planned")
	fmt.Fprintf(os.Stderr, "Setting %d %s%s (%d worker%s)...\n", planned, changeNoun, pluralizeSubscriptionPriceDeriveCount(planned), workers, pluralizeSubscriptionPriceDeriveCount(workers))
	applySubscriptionPriceDeriveRows(ctx, client, targetSubscriptionID, rows, workers, attrs, effectiveAt)
	appliedCount := countSubscriptionPriceDeriveRows(rows, "applied")

	fmt.Fprintf(os.Stderr, "Verifying %s...\n", verifyNoun)
	verification, err := verifySubscriptionPriceDeriveRows(ctx, client, targetSubscriptionID, effectiveAt, territory, rows)
	summary := shared.SummarizePriceLadderRows(rows)
	summary.Applied = appliedCount
	return summary, verification, err
}

func applySubscriptionPriceDeriveRows(
	ctx context.Context,
	client *asc.Client,
	targetSubscriptionID string,
	rows []asc.SubscriptionPricingDeriveRow,
	workers int,
	attrs asc.SubscriptionPriceCreateAttributes,
	effectiveAt time.Time,
) {
	changes := make([]equalization, 0, len(rows))
	for _, row := range rows {
		if row.Status != "planned" {
			continue
		}
		changes = append(changes, equalization{
			Territory:    row.Territory,
			Price:        row.TargetPrice,
			PricePointID: row.TargetPricePointID,
		})
	}
	_, failures := applyEqualizedPrices(ctx, client, targetSubscriptionID, changes, workers, attrs, effectiveAt)
	failureByTerritory := make(map[string]error, len(failures))
	for _, failure := range failures {
		failureByTerritory[strings.ToUpper(strings.TrimSpace(failure.Target.Territory))] = failure.Err
	}
	for index := range rows {
		row := &rows[index]
		if row.Status != "planned" {
			continue
		}
		if failure := failureByTerritory[row.Territory]; failure != nil {
			row.Status = "failed"
			row.Error = failure.Error()
			continue
		}
		row.Status = "applied"
	}
}

func verifySubscriptionPriceDeriveRows(
	ctx context.Context,
	client *asc.Client,
	targetSubscriptionID string,
	effectiveAt time.Time,
	territory string,
	rows []asc.SubscriptionPricingDeriveRow,
) (asc.SubscriptionPricingDeriveVerification, error) {
	preexistingFailed := countSubscriptionPriceDeriveRows(rows, "failed")
	if countSubscriptionPriceDeriveRows(rows, "applied") == 0 {
		status := "completed"
		if preexistingFailed > 0 {
			status = "failed"
		}
		verification := asc.SubscriptionPricingDeriveVerification{
			Status: status,
			Failed: preexistingFailed,
		}
		if preexistingFailed > 0 {
			return verification, fmt.Errorf("%d territory update(s) failed before verification", preexistingFailed)
		}
		return verification, nil
	}

	resolved, err := fetchResolvedSubscriptionPrices(
		ctx,
		client,
		targetSubscriptionID,
		200,
		"",
		effectiveAt,
		asc.SubscriptionPlanTypeUpfront,
		territory,
	)
	if err != nil {
		for index := range rows {
			if rows[index].Status == "applied" {
				rows[index].Status = "failed"
				rows[index].Error = fmt.Sprintf("verification readback failed: %v", err)
			}
		}
		return asc.SubscriptionPricingDeriveVerification{
			Status: "failed",
			Failed: countSubscriptionPriceDeriveRows(rows, "failed"),
		}, fmt.Errorf("verification readback failed: %w", err)
	}
	currentByTerritory := make(map[string]string, len(resolved.Prices))
	for _, row := range resolved.Prices {
		territory := strings.ToUpper(strings.TrimSpace(row.Territory))
		if territory != "" {
			currentByTerritory[territory] = strings.TrimSpace(row.PricePointID)
		}
	}
	verified := 0
	failed := preexistingFailed
	for index := range rows {
		row := &rows[index]
		if row.Status != "applied" {
			continue
		}
		if currentByTerritory[row.Territory] == row.TargetPricePointID {
			row.Status = "verified"
			verified++
			continue
		}
		row.Status = "failed"
		actual := currentByTerritory[row.Territory]
		if actual == "" {
			actual = "<missing>"
		}
		row.Error = fmt.Sprintf("verification mismatch: expected target price point %s, got %s", row.TargetPricePointID, actual)
		failed++
	}
	status := "completed"
	if failed > 0 {
		status = "failed"
	}
	verification := asc.SubscriptionPricingDeriveVerification{
		Status:   status,
		Verified: verified,
		Failed:   failed,
	}
	if failed > 0 {
		return verification, fmt.Errorf("verification failed for %d territory update(s)", failed)
	}
	return verification, nil
}

func countSubscriptionPriceDeriveRows(rows []asc.SubscriptionPricingDeriveRow, status string) int {
	count := 0
	for _, row := range rows {
		if row.Status == status {
			count++
		}
	}
	return count
}

func pluralizeSubscriptionPriceDeriveCount(count int) string {
	if count == 1 {
		return ""
	}
	return "s"
}

func buildSubscriptionPriceDerivePlan(
	ctx context.Context,
	client *asc.Client,
	targetSubscriptionID string,
	sourcePrices *shared.ResolvedPricesResult,
	targetPrices *shared.ResolvedPricesResult,
	multiplierFor func(territory string) (*big.Rat, bool),
	rounding shared.PriceRounding,
	workers int,
) []asc.SubscriptionPricingDeriveRow {
	if sourcePrices == nil {
		return nil
	}
	territories := make([]string, 0, len(sourcePrices.Prices))
	seenTerritories := make(map[string]struct{}, len(sourcePrices.Prices))
	for _, source := range sourcePrices.Prices {
		territory := strings.ToUpper(strings.TrimSpace(source.Territory))
		if territory == "" {
			continue
		}
		if _, seen := seenTerritories[territory]; seen {
			continue
		}
		seenTerritories[territory] = struct{}{}
		territories = append(territories, territory)
	}
	candidatesByTerritory, candidatesErr := fetchSubscriptionPriceDeriveCandidatesByTerritory(
		ctx,
		client,
		targetSubscriptionID,
		territories,
	)
	targetByTerritory := make(map[string]shared.ResolvedPriceRow)
	if targetPrices != nil {
		for _, row := range targetPrices.Prices {
			territory := strings.ToUpper(strings.TrimSpace(row.Territory))
			if territory != "" {
				targetByTerritory[territory] = row
			}
		}
	}

	rows := make([]asc.SubscriptionPricingDeriveRow, len(sourcePrices.Prices))
	jobs := make(chan int)
	var wg sync.WaitGroup
	for worker := 0; worker < workers; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for index := range jobs {
				source := sourcePrices.Prices[index]
				territory := strings.ToUpper(strings.TrimSpace(source.Territory))
				var current *shared.ResolvedPriceRow
				if row, ok := targetByTerritory[territory]; ok {
					current = &row
				}
				multiplier, _ := multiplierFor(territory)
				rows[index] = shared.PlanPriceLadderRow(source, current, multiplier, candidatesByTerritory[territory], candidatesErr, rounding)
			}
		}()
	}
	for index := range sourcePrices.Prices {
		jobs <- index
	}
	close(jobs)
	wg.Wait()

	sort.Slice(rows, func(i, j int) bool {
		return rows[i].Territory < rows[j].Territory
	})
	return rows
}

func fetchSubscriptionPriceDeriveCandidatesByTerritory(
	ctx context.Context,
	client *asc.Client,
	targetSubscriptionID string,
	territories []string,
) (map[string][]shared.PriceLadderCandidate, error) {
	normalizedTerritories := make([]string, 0, len(territories))
	wantedTerritories := make(map[string]struct{}, len(territories))
	for _, rawTerritory := range territories {
		territory := strings.ToUpper(strings.TrimSpace(rawTerritory))
		if territory == "" {
			continue
		}
		if _, seen := wantedTerritories[territory]; seen {
			continue
		}
		wantedTerritories[territory] = struct{}{}
		normalizedTerritories = append(normalizedTerritories, territory)
	}
	if len(normalizedTerritories) == 0 {
		return map[string][]shared.PriceLadderCandidate{}, nil
	}
	sort.Strings(normalizedTerritories)

	fetchPage := func(nextURL string) (*asc.SubscriptionPricePointsResponse, error) {
		if strings.TrimSpace(nextURL) == "" {
			return shared.RetryReadWithFreshTimeout(ctx, func(requestCtx context.Context) (*asc.SubscriptionPricePointsResponse, error) {
				return client.GetSubscriptionPricePoints(
					requestCtx,
					targetSubscriptionID,
					asc.WithSubscriptionPricePointsTerritories(normalizedTerritories),
					asc.WithSubscriptionPricePointsPlanTypes([]string{string(asc.SubscriptionPlanTypeUpfront)}),
					asc.WithSubscriptionPricePointsFields([]string{"customerPrice", "territory"}),
					asc.WithSubscriptionPricePointsInclude([]string{"territory"}),
					asc.WithSubscriptionPricePointsLimit(8000),
				)
			})
		}
		return shared.RetryReadWithFreshTimeout(ctx, func(requestCtx context.Context) (*asc.SubscriptionPricePointsResponse, error) {
			return client.GetSubscriptionPricePoints(
				requestCtx,
				targetSubscriptionID,
				asc.WithSubscriptionPricePointsNextURL(nextURL),
			)
		})
	}

	firstPage, err := fetchPage("")
	if err != nil {
		return nil, fmt.Errorf("resolve target price points: %w", err)
	}
	candidates := make(map[string][]shared.PriceLadderCandidate, len(normalizedTerritories))
	if err := asc.PaginateEach(
		ctx,
		firstPage,
		func(_ context.Context, nextURL string) (asc.PaginatedResponse, error) {
			return fetchPage(nextURL)
		},
		func(page asc.PaginatedResponse) error {
			response, ok := page.(*asc.SubscriptionPricePointsResponse)
			if !ok {
				return fmt.Errorf("unexpected target price-points response type %T", page)
			}
			for _, pricePoint := range response.Data {
				territory := shared.PricePointTerritoryID(pricePoint.Relationships)
				if territory == "" {
					return fmt.Errorf("target price point %q is missing its territory relationship", pricePoint.ID)
				}
				if _, wanted := wantedTerritories[territory]; !wanted {
					return fmt.Errorf("target price point %q returned unexpected territory %s", pricePoint.ID, territory)
				}
				candidates[territory] = append(candidates[territory], shared.PriceLadderCandidate{
					PricePointID:  strings.TrimSpace(pricePoint.ID),
					CustomerPrice: strings.TrimSpace(pricePoint.Attributes.CustomerPrice),
				})
			}
			return nil
		},
	); err != nil {
		return nil, fmt.Errorf("resolve target price points: %w", err)
	}
	return candidates, nil
}

func parseSubscriptionPriceDeriveMultiplier(value string) (*big.Rat, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		fmt.Fprintln(os.Stderr, "Error: --multiplier is required")
		return nil, shared.MissingRequiredUsageError("--multiplier")
	}
	if !shared.IsPriceDecimal(trimmed) {
		return nil, shared.UsageError("--multiplier must be a positive decimal")
	}
	multiplier := new(big.Rat)
	if _, ok := multiplier.SetString(trimmed); !ok || multiplier.Sign() <= 0 {
		return nil, shared.UsageError("--multiplier must be a positive decimal")
	}
	return multiplier, nil
}
