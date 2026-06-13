package subscriptions

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
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

const (
	defaultEqualizeWorkers    = 8
	equalizeRecoveryWorkers   = 1
	equalizeDateLayout        = "2006-01-02"
	maxEqualizeRecoveryPasses = 2
)

var errEqualizePricePointFound = errors.New("equalize price point found")

var equalizeNow = time.Now

// SubscriptionsPricingEqualizeCommand returns the equalize subcommand.
func SubscriptionsPricingEqualizeCommand() *ffcli.Command {
	fs := flag.NewFlagSet("equalize", flag.ExitOnError)

	subscriptionID := fs.String("subscription-id", "", "Subscription ID, product ID, or exact current name (required)")
	appID := addSubscriptionLookupAppFlag(fs)
	baseTerritory := fs.String("base-territory", "USA", "Pricing base territory (accepts alpha-2, alpha-3, or exact English country name)")
	basePrice := fs.String("base-price", "", "Customer price in the base territory (required)")
	startDate := fs.String("start-date", "", "Start date (YYYY-MM-DD) for scheduled price changes")
	preserved := fs.Bool("preserved", false, "Preserve existing prices")
	autoStartDate := fs.Bool("auto-start-date", true, "Automatically schedule approved/live subscriptions for tomorrow when --start-date is omitted")
	dryRun := fs.Bool("dry-run", false, "Show equalized prices without applying them")
	confirm := fs.Bool("confirm", false, "Confirm applying equalized prices (required unless --dry-run)")
	workers := fs.Int("workers", defaultEqualizeWorkers, "Number of concurrent API requests")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "equalize",
		ShortUsage: "asc subscriptions pricing equalize [flags]",
		ShortHelp:  "Set equalized prices for all territories from a base price.",
		LongHelp: `Set equalized prices for all territories from a base price.

Finds the price point matching the given base territory and price, fetches
Apple's equalized prices for all other territories, and sets them in one
operation. This replaces the manual process of exporting equalizations and
importing a CSV.

Examples:
  asc subscriptions pricing equalize --subscription-id "SUB_ID" --base-price "3.49" --confirm
  asc subscriptions pricing equalize --subscription-id "SUB_ID" --base-price "3.49" --start-date "2026-04-01" --confirm
  asc subscriptions pricing equalize --subscription-id "SUB_ID" --base-price "38.49" --base-territory "United States" --confirm
  asc subscriptions pricing equalize --subscription-id "SUB_ID" --base-price "3.49" --dry-run
  asc subscriptions pricing equalize --subscription-id "SUB_ID" --base-price "3.49" --confirm --workers 16`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return shared.UsageError("subscriptions pricing equalize does not accept positional arguments")
			}
			subID := strings.TrimSpace(*subscriptionID)
			if subID == "" {
				fmt.Fprintln(os.Stderr, "Error: --subscription-id is required")
				return flag.ErrHelp
			}
			price := strings.TrimSpace(*basePrice)
			if price == "" {
				fmt.Fprintln(os.Stderr, "Error: --base-price is required")
				return flag.ErrHelp
			}
			if err := shared.ValidateFinitePriceFlag("--base-price", price); err != nil {
				return shared.UsageError(err.Error())
			}
			explicitStartDate, effectiveAt, err := normalizeEqualizeStartDate(*startDate)
			if err != nil {
				return err
			}
			territoryInput := strings.TrimSpace(*baseTerritory)
			if territoryInput == "" {
				territoryInput = "USA"
			}
			territory, err := ascterritory.Normalize(territoryInput)
			if err != nil {
				return shared.UsageError(err.Error())
			}
			if !*dryRun && !*confirm {
				return shared.UsageError("--confirm is required unless --dry-run is set")
			}
			numWorkers := *workers
			if numWorkers < 1 || numWorkers > 32 {
				return shared.UsageError("--workers must be between 1 and 32")
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("equalize: %w", err)
			}

			resolveCtx, resolveCancel := shared.ContextWithTimeout(ctx)
			subID, err = resolveSubscriptionLookupID(resolveCtx, client, *appID, subID)
			resolveCancel()
			if err != nil {
				return err
			}

			// Step 0: Fail fast if sale availability does not already cover all pricing territories.
			fmt.Fprintln(os.Stderr, "Checking subscription availability coverage...")
			coveredTerritories, err := validateEqualizeAvailability(ctx, client, subID)
			if err != nil {
				return fmt.Errorf("equalize: %w", err)
			}
			fmt.Fprintf(os.Stderr, "Availability covers %d pricing territories\n", coveredTerritories)

			// Step 1: Find the base price point
			fmt.Fprintf(os.Stderr, "Finding %s price point for %s...\n", territory, price)
			pricePointID, err := findPricePoint(ctx, client, subID, territory, price)
			if err != nil {
				return fmt.Errorf("equalize: %w", err)
			}
			fmt.Fprintf(os.Stderr, "Found price point: %s\n", pricePointID)

			// Step 2: Get equalizations for all territories
			fmt.Fprintf(os.Stderr, "Fetching equalized prices for all territories...\n")
			equalizations, err := fetchEqualizations(ctx, client, pricePointID, territory)
			if err != nil {
				return fmt.Errorf("equalize: %w", err)
			}
			fmt.Fprintf(os.Stderr, "Got %d territory equalizations\n", len(equalizations))

			allTerritories := make([]equalization, 0, len(equalizations)+1)
			allTerritories = append(allTerritories, equalization{
				Territory:    territory,
				Price:        price,
				PricePointID: pricePointID,
			})
			allTerritories = append(allTerritories, equalizations...)

			priceAttrs := asc.SubscriptionPriceCreateAttributes{
				StartDate: explicitStartDate,
			}
			if *preserved {
				priceAttrs.Preserved = preserved
			}

			subscriptionState := ""
			autoScheduled := false

			if *dryRun {
				if priceAttrs.StartDate == "" && *autoStartDate {
					existingCtx, existingCancel := shared.ContextWithTimeout(ctx)
					existingPrices, err := client.GetSubscriptionPricesRelationships(existingCtx, subID)
					existingCancel()
					if err != nil {
						return fmt.Errorf("equalize: failed to check existing prices: %w", err)
					}
					if len(existingPrices.Data) > 0 {
						var scheduleErr error
						priceAttrs.StartDate, subscriptionState, autoScheduled, _, scheduleErr = autoScheduleEqualizeStartDate(ctx, client, subID)
						if scheduleErr != nil {
							return fmt.Errorf("equalize: %w", scheduleErr)
						}
					}
				}

				return printEqualizeResult(&equalizeResult{
					SubscriptionID:    subID,
					BaseTerritory:     territory,
					BasePrice:         price,
					StartDate:         priceAttrs.StartDate,
					AutoScheduled:     autoScheduled,
					Preserved:         *preserved,
					SubscriptionState: subscriptionState,
					DryRun:            true,
					Territories:       allTerritories,
					Total:             len(allTerritories),
				}, *output.Output, *output.Pretty)
			}

			// Step 3: Set prices for all territories concurrently
			fmt.Fprintf(os.Stderr, "Setting prices for %d territories (%d workers)...\n", len(allTerritories), numWorkers)
			succeeded := 0
			failures := make([]equalizeAttemptFailure, 0)

			existingCtx, existingCancel := shared.ContextWithTimeout(ctx)
			existingPrices, err := client.GetSubscriptionPricesRelationships(existingCtx, subID)
			existingCancel()
			if err != nil {
				return fmt.Errorf("equalize: failed to check existing prices: %w", err)
			}

			if priceAttrs.StartDate == "" && *autoStartDate && len(existingPrices.Data) > 0 {
				var scheduleErr error
				priceAttrs.StartDate, subscriptionState, autoScheduled, effectiveAt, scheduleErr = autoScheduleEqualizeStartDate(ctx, client, subID)
				if scheduleErr != nil {
					return fmt.Errorf("equalize: %w", scheduleErr)
				}
				if autoScheduled {
					fmt.Fprintf(os.Stderr, "Subscription state is %s; scheduling price changes for %s\n", subscriptionState, priceAttrs.StartDate)
				}
			}

			remainingTerritories := allTerritories
			if len(existingPrices.Data) == 0 && len(allTerritories) > 0 {
				fmt.Fprintf(os.Stderr, "Subscription has no prices; setting initial price in %s first...\n", territory)

				baseTarget := allTerritories[0]
				initialCtx, initialCancel := shared.ContextWithTimeout(ctx)
				_, err := client.SetSubscriptionInitialPrice(initialCtx, subID, baseTarget.PricePointID, baseTarget.Territory, priceAttrs)
				initialCancel()
				if err != nil {
					initialFailures := []equalizeAttemptFailure{{
						Target: baseTarget,
						Err:    err,
					}}
					reconciled, remaining, verifyErr := reconcileEqualizeFailures(ctx, client, subID, initialFailures, effectiveAt)
					if verifyErr != nil {
						fmt.Fprintf(os.Stderr, "Warning: could not verify failed initial price update: %v\n", verifyErr)
					}
					if len(remaining) == 0 {
						succeeded += reconciled
						remainingTerritories = allTerritories[1:]
					} else {
						failures = append(failures, remaining...)
						result := &equalizeResult{
							SubscriptionID:    subID,
							BaseTerritory:     territory,
							BasePrice:         price,
							StartDate:         priceAttrs.StartDate,
							AutoScheduled:     autoScheduled,
							Preserved:         *preserved,
							SubscriptionState: subscriptionState,
							DryRun:            false,
							Total:             len(allTerritories),
							Succeeded:         succeeded,
							Failed:            len(failures),
							Failures:          renderEqualizeFailures(failures),
						}
						fmt.Fprintf(os.Stderr, "Done: %d succeeded, %d failed\n", result.Succeeded, result.Failed)
						if err := printEqualizeResult(result, *output.Output, *output.Pretty); err != nil {
							return err
						}
						return shared.NewReportedError(fmt.Errorf("equalize: failed to set initial price in %s", baseTarget.Territory))
					}
				} else {
					succeeded++
					remainingTerritories = allTerritories[1:]
				}
			}

			passSucceeded, passFailures := applyEqualizedPrices(ctx, client, subID, remainingTerritories, numWorkers, priceAttrs, effectiveAt)
			succeeded += passSucceeded
			if len(passFailures) > 0 {
				failures = append(failures, passFailures...)
			}

			result := &equalizeResult{
				SubscriptionID:    subID,
				BaseTerritory:     territory,
				BasePrice:         price,
				StartDate:         priceAttrs.StartDate,
				AutoScheduled:     autoScheduled,
				Preserved:         *preserved,
				SubscriptionState: subscriptionState,
				DryRun:            false,
				Total:             len(allTerritories),
				Succeeded:         succeeded,
				Failed:            len(failures),
				Failures:          renderEqualizeFailures(failures),
			}

			fmt.Fprintf(os.Stderr, "Done: %d succeeded, %d failed\n", result.Succeeded, result.Failed)

			if err := printEqualizeResult(result, *output.Output, *output.Pretty); err != nil {
				return err
			}
			if result.Failed > 0 {
				return shared.NewReportedError(fmt.Errorf("equalize: %d territory update(s) failed", result.Failed))
			}
			return nil
		},
	}
}

type equalization struct {
	Territory    string `json:"territory"`
	Price        string `json:"price"`
	PricePointID string `json:"pricePointId"`
}

type equalizeFailure struct {
	Territory string `json:"territory"`
	Price     string `json:"price"`
	Error     string `json:"error"`
}

type equalizeAttemptFailure struct {
	Target equalization
	Err    error
}

type equalizeResult struct {
	SubscriptionID    string            `json:"subscriptionId"`
	BaseTerritory     string            `json:"baseTerritory"`
	BasePrice         string            `json:"basePrice"`
	StartDate         string            `json:"startDate,omitempty"`
	AutoScheduled     bool              `json:"autoScheduled,omitempty"`
	Preserved         bool              `json:"preserved,omitempty"`
	SubscriptionState string            `json:"subscriptionState,omitempty"`
	DryRun            bool              `json:"dryRun"`
	Total             int               `json:"total"`
	Succeeded         int               `json:"succeeded,omitempty"`
	Failed            int               `json:"failed,omitempty"`
	Territories       []equalization    `json:"territories,omitempty"`
	Failures          []equalizeFailure `json:"failures,omitempty"`
}

func applyEqualizedPrices(ctx context.Context, client *asc.Client, subID string, targets []equalization, workers int, attrs asc.SubscriptionPriceCreateAttributes, effectiveAt time.Time) (int, []equalizeAttemptFailure) {
	succeeded, failures := runEqualizePricePass(ctx, client, subID, targets, workers, attrs)
	retryable, finalFailures := partitionEqualizeFailures(failures)

	for pass := 1; len(retryable) > 0 && pass <= maxEqualizeRecoveryPasses; pass++ {
		if delay := maxEqualizeRetryAfter(retryable); delay > 0 {
			fmt.Fprintf(os.Stderr, "Waiting %s before retrying %d retryable territory update(s)...\n", delay.Round(time.Second), len(retryable))
			if err := sleepWithContext(ctx, delay); err != nil {
				finalFailures = append(finalFailures, retryable...)
				return succeeded, finalFailures
			}
		}
		fmt.Fprintf(os.Stderr, "Retrying %d retryable territory update(s) with %d worker...\n", len(retryable), equalizeRecoveryWorkers)
		retrySucceeded, retryFailures := runEqualizePricePass(ctx, client, subID, equalizeTargetsFromFailures(retryable), equalizeRecoveryWorkers, attrs)
		succeeded += retrySucceeded

		retryable, failures = partitionEqualizeFailures(retryFailures)
		finalFailures = append(finalFailures, failures...)
	}

	if len(retryable) > 0 {
		finalFailures = append(finalFailures, retryable...)
	}

	reconciled, remaining, err := reconcileEqualizeFailures(ctx, client, subID, finalFailures, effectiveAt)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not verify %d failed territory update(s): %v\n", len(finalFailures), err)
		return succeeded, finalFailures
	}
	succeeded += reconciled
	return succeeded, remaining
}

func runEqualizePricePass(ctx context.Context, client *asc.Client, subID string, targets []equalization, workers int, attrs asc.SubscriptionPriceCreateAttributes) (int, []equalizeAttemptFailure) {
	if len(targets) == 0 {
		return 0, nil
	}
	if workers < 1 {
		workers = 1
	}
	if workers == 1 {
		succeeded := 0
		failures := make([]equalizeAttemptFailure, 0)
		for _, target := range targets {
			setCtx, setCancel := shared.ContextWithTimeout(ctx)
			_, err := client.CreateSubscriptionPrice(setCtx, subID, target.PricePointID, target.Territory, attrs)
			setCancel()
			if err != nil {
				failures = append(failures, equalizeAttemptFailure{
					Target: target,
					Err:    err,
				})
				continue
			}
			succeeded++
		}
		return succeeded, failures
	}

	type priceUpdateResult struct {
		target equalization
		err    error
	}

	results := make(chan priceUpdateResult, len(targets))
	sem := make(chan struct{}, workers)
	var wg sync.WaitGroup

	for _, target := range targets {
		wg.Add(1)
		go func(t equalization) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			setCtx, setCancel := shared.ContextWithTimeout(ctx)
			defer setCancel()

			_, err := client.CreateSubscriptionPrice(setCtx, subID, t.PricePointID, t.Territory, attrs)
			results <- priceUpdateResult{target: t, err: err}
		}(target)
	}

	wg.Wait()
	close(results)

	succeeded := 0
	failures := make([]equalizeAttemptFailure, 0)
	for result := range results {
		if result.err != nil {
			failures = append(failures, equalizeAttemptFailure{
				Target: result.target,
				Err:    result.err,
			})
			continue
		}
		succeeded++
	}

	return succeeded, failures
}

func partitionEqualizeFailures(failures []equalizeAttemptFailure) (retryable []equalizeAttemptFailure, final []equalizeAttemptFailure) {
	for _, failure := range failures {
		if asc.IsRetryable(failure.Err) {
			retryable = append(retryable, failure)
			continue
		}
		final = append(final, failure)
	}
	return retryable, final
}

func equalizeTargetsFromFailures(failures []equalizeAttemptFailure) []equalization {
	targets := make([]equalization, 0, len(failures))
	for _, failure := range failures {
		targets = append(targets, failure.Target)
	}
	return targets
}

func renderEqualizeFailures(failures []equalizeAttemptFailure) []equalizeFailure {
	rendered := make([]equalizeFailure, 0, len(failures))
	for _, failure := range failures {
		rendered = append(rendered, equalizeFailure{
			Territory: failure.Target.Territory,
			Price:     failure.Target.Price,
			Error:     failure.Err.Error(),
		})
	}
	return rendered
}

func normalizeEqualizeStartDate(value string) (string, time.Time, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", equalizeNow(), nil
	}

	normalized, err := shared.NormalizeDate(trimmed, "--start-date")
	if err != nil {
		return "", time.Time{}, shared.UsageError(err.Error())
	}
	parsed, err := time.Parse(equalizeDateLayout, normalized)
	if err != nil {
		return "", time.Time{}, err
	}
	if !parsed.After(dateOnlyUTC(equalizeNow())) {
		return "", time.Time{}, shared.UsageError("--start-date must be a future date")
	}
	return normalized, parsed, nil
}

func autoScheduleEqualizeStartDate(ctx context.Context, client *asc.Client, subID string) (string, string, bool, time.Time, error) {
	state, err := fetchEqualizeSubscriptionState(ctx, client, subID)
	if err != nil {
		return "", "", false, time.Time{}, fmt.Errorf("failed to inspect subscription state for auto scheduling: %w", err)
	}
	if !isApprovedOrLiveSubscriptionState(state) {
		return "", state, false, equalizeNow(), nil
	}

	effectiveAt := equalizeNow().UTC().AddDate(0, 0, 1)
	startDate := effectiveAt.Format(equalizeDateLayout)
	parsed, err := time.Parse(equalizeDateLayout, startDate)
	if err != nil {
		return "", state, false, time.Time{}, err
	}
	return startDate, state, true, parsed, nil
}

func fetchEqualizeSubscriptionState(ctx context.Context, client *asc.Client, subID string) (string, error) {
	getCtx, getCancel := shared.ContextWithTimeout(ctx)
	defer getCancel()

	resp, err := client.GetSubscription(getCtx, subID)
	if err != nil {
		return "", err
	}
	return strings.ToUpper(strings.TrimSpace(resp.Data.Attributes.State)), nil
}

func isApprovedOrLiveSubscriptionState(state string) bool {
	switch strings.ToUpper(strings.TrimSpace(state)) {
	case "APPROVED", "READY_FOR_SALE":
		return true
	default:
		return false
	}
}

func reconcileEqualizeFailures(ctx context.Context, client *asc.Client, subID string, failures []equalizeAttemptFailure, effectiveAt time.Time) (int, []equalizeAttemptFailure, error) {
	if len(failures) == 0 {
		return 0, nil, nil
	}

	fmt.Fprintf(os.Stderr, "Verifying %d territory update(s) against current prices...\n", len(failures))

	verifyCtx, verifyCancel := shared.ContextWithTimeout(ctx)
	defer verifyCancel()

	resolved, err := fetchResolvedSubscriptionPrices(verifyCtx, client, subID, 200, "", effectiveAt, "")
	if err != nil {
		return 0, failures, err
	}

	currentByTerritory := make(map[string]string, len(resolved.Prices))
	for _, row := range resolved.Prices {
		territoryID := strings.ToUpper(strings.TrimSpace(row.Territory))
		if territoryID == "" {
			continue
		}
		currentByTerritory[territoryID] = strings.TrimSpace(row.PricePointID)
	}

	reconciled := 0
	remaining := make([]equalizeAttemptFailure, 0, len(failures))
	for _, failure := range failures {
		if currentByTerritory[strings.ToUpper(strings.TrimSpace(failure.Target.Territory))] == strings.TrimSpace(failure.Target.PricePointID) {
			reconciled++
			continue
		}
		remaining = append(remaining, failure)
	}

	return reconciled, remaining, nil
}

func maxEqualizeRetryAfter(failures []equalizeAttemptFailure) time.Duration {
	var maxDelay time.Duration
	for _, failure := range failures {
		if delay := asc.GetRetryAfter(failure.Err); delay > maxDelay {
			maxDelay = delay
		}
	}
	return maxDelay
}

func sleepWithContext(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return nil
	}

	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func findPricePoint(ctx context.Context, client *asc.Client, subID, territory, targetPrice string) (string, error) {
	// List price points filtered by the base territory, paginating to find the matching price
	var pricePointID string
	priceFilter := shared.PriceFilter{Price: targetPrice}

	firstCtx, firstCancel := shared.ContextWithTimeout(ctx)
	firstPage, err := client.GetSubscriptionPricePoints(
		firstCtx, subID,
		asc.WithSubscriptionPricePointsTerritory(territory),
		asc.WithSubscriptionPricePointsLimit(200),
	)
	firstCancel()
	if err != nil {
		return "", fmt.Errorf("failed to fetch price points: %w", err)
	}

	// Check first page
	for _, pp := range firstPage.Data {
		if priceFilter.MatchesPrice(pp.Attributes.CustomerPrice) {
			pricePointID = pp.ID
			return pricePointID, nil
		}
	}

	// Paginate through remaining pages
	err = asc.PaginateEach(
		ctx, firstPage,
		func(_ context.Context, nextURL string) (asc.PaginatedResponse, error) {
			pageCtx, pageCancel := shared.ContextWithTimeout(ctx)
			defer pageCancel()
			return client.GetSubscriptionPricePoints(
				pageCtx, subID,
				asc.WithSubscriptionPricePointsNextURL(nextURL),
			)
		},
		func(page asc.PaginatedResponse) error {
			typed, ok := page.(*asc.SubscriptionPricePointsResponse)
			if !ok {
				return nil
			}
			for _, pp := range typed.Data {
				if priceFilter.MatchesPrice(pp.Attributes.CustomerPrice) {
					pricePointID = pp.ID
					return errEqualizePricePointFound
				}
			}
			return nil
		},
	)

	if pricePointID != "" {
		return pricePointID, nil
	}

	if err != nil && !errors.Is(err, errEqualizePricePointFound) {
		return "", err
	}

	return "", fmt.Errorf("no price point found for %s %s", territory, targetPrice)
}

func fetchEqualizations(ctx context.Context, client *asc.Client, pricePointID, baseTerritory string) ([]equalization, error) {
	// Use include=territory so the API populates each price point's
	// relationships with the territory reference, avoiding reliance on
	// opaque price point ID structure.
	firstCtx, firstCancel := shared.ContextWithTimeout(ctx)
	resp, err := client.GetSubscriptionPricePointEqualizations(
		firstCtx, pricePointID,
		asc.WithSubscriptionPricePointsInclude([]string{"territory"}),
		asc.WithSubscriptionPricePointsFields([]string{"customerPrice", "territory"}),
		asc.WithSubscriptionPricePointsLimit(200),
	)
	firstCancel()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch equalizations: %w", err)
	}

	allPages, err := asc.PaginateAll(ctx, resp, func(_ context.Context, nextURL string) (asc.PaginatedResponse, error) {
		pageCtx, pageCancel := shared.ContextWithTimeout(ctx)
		defer pageCancel()
		return client.GetSubscriptionPricePointEqualizations(
			pageCtx, pricePointID,
			asc.WithSubscriptionPricePointsNextURL(nextURL),
		)
	})
	if err != nil {
		return nil, fmt.Errorf("paginate equalizations: %w", err)
	}

	typed, ok := allPages.(*asc.SubscriptionPricePointsResponse)
	if !ok {
		return nil, fmt.Errorf("unexpected response type %T", allPages)
	}

	var result []equalization
	for _, pp := range typed.Data {
		territory, err := equalizationTerritoryID(pp)
		if err != nil {
			return nil, err
		}
		if strings.EqualFold(territory, baseTerritory) {
			continue
		}
		result = append(result, equalization{
			Territory:    territory,
			Price:        strings.TrimSpace(pp.Attributes.CustomerPrice),
			PricePointID: pp.ID,
		})
	}

	return result, nil
}

func validateEqualizeAvailability(ctx context.Context, client *asc.Client, subID string) (int, error) {
	getCtx, getCancel := shared.ContextWithTimeout(ctx)
	availability, err := client.GetSubscriptionAvailabilityForSubscription(getCtx, subID)
	getCancel()
	if err != nil {
		if errors.Is(err, asc.ErrNotFound) {
			exists, verifyErr := subscriptionExists(ctx, client, subID)
			if verifyErr != nil {
				return 0, verifyErr
			}
			if !exists {
				return 0, fmt.Errorf("subscription %q was not found", subID)
			}
			return 0, fmt.Errorf("subscription availability is not configured; equalize only updates prices and will not change sale availability. Configure territories first with `asc subscriptions pricing availability edit`")
		}
		return 0, fmt.Errorf("failed to fetch availability: %w", err)
	}

	availabilityID := strings.TrimSpace(availability.Data.ID)
	if availabilityID == "" {
		return 0, fmt.Errorf("availability readback returned empty id")
	}

	requiredTerritories, err := fetchPricingTerritories(ctx, client)
	if err != nil {
		return 0, err
	}
	if len(requiredTerritories) == 0 {
		return 0, nil
	}

	available, err := fetchSubscriptionAvailabilityTerritories(ctx, client, availabilityID)
	if err != nil {
		return 0, err
	}

	missing := make([]string, 0)
	for _, territoryID := range requiredTerritories {
		if _, ok := available[territoryID]; !ok {
			missing = append(missing, territoryID)
		}
	}
	if len(missing) == 0 {
		return len(requiredTerritories), nil
	}

	sort.Strings(missing)
	return 0, fmt.Errorf("subscription availability is missing %d equalized territor%s (%s); equalize only updates prices and will not change sale availability. Configure territories first with `asc subscriptions pricing availability edit`", len(missing), pluralizeEqualizeTerritories(len(missing)), summarizeEqualizeTerritories(missing, 8))
}

func subscriptionExists(ctx context.Context, client *asc.Client, subID string) (bool, error) {
	getCtx, getCancel := shared.ContextWithTimeout(ctx)
	defer getCancel()

	_, err := client.GetSubscription(getCtx, subID)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, asc.ErrNotFound) {
		return false, nil
	}
	return false, fmt.Errorf("failed to verify subscription: %w", err)
}

func fetchPricingTerritories(ctx context.Context, client *asc.Client) ([]string, error) {
	firstCtx, firstCancel := shared.ContextWithTimeout(ctx)
	firstPage, err := client.GetTerritories(firstCtx, asc.WithTerritoriesLimit(200))
	firstCancel()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch pricing territories: %w", err)
	}

	allPages, err := asc.PaginateAll(ctx, firstPage, func(_ context.Context, nextURL string) (asc.PaginatedResponse, error) {
		pageCtx, pageCancel := shared.ContextWithTimeout(ctx)
		defer pageCancel()
		return client.GetTerritories(pageCtx, asc.WithTerritoriesNextURL(nextURL))
	})
	if err != nil {
		return nil, fmt.Errorf("paginate pricing territories: %w", err)
	}

	typed, ok := allPages.(*asc.TerritoriesResponse)
	if !ok {
		return nil, fmt.Errorf("unexpected pricing territories response type %T", allPages)
	}

	territories := make([]string, 0, len(typed.Data))
	seen := make(map[string]struct{}, len(typed.Data))
	for _, territory := range typed.Data {
		id := strings.ToUpper(strings.TrimSpace(territory.ID))
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		territories = append(territories, id)
	}
	sort.Strings(territories)
	return territories, nil
}

func fetchSubscriptionAvailabilityTerritories(ctx context.Context, client *asc.Client, availabilityID string) (map[string]struct{}, error) {
	territoriesCtx, territoriesCancel := shared.ContextWithTimeout(ctx)
	firstPage, err := client.GetSubscriptionAvailabilityAvailableTerritories(territoriesCtx, availabilityID, asc.WithSubscriptionAvailabilityTerritoriesLimit(200))
	territoriesCancel()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch availability territories: %w", err)
	}

	allPages, err := asc.PaginateAll(ctx, firstPage, func(_ context.Context, nextURL string) (asc.PaginatedResponse, error) {
		pageCtx, pageCancel := shared.ContextWithTimeout(ctx)
		defer pageCancel()
		return client.GetSubscriptionAvailabilityAvailableTerritories(pageCtx, availabilityID, asc.WithSubscriptionAvailabilityTerritoriesNextURL(nextURL))
	})
	if err != nil {
		return nil, fmt.Errorf("paginate availability territories: %w", err)
	}

	typed, ok := allPages.(*asc.TerritoriesResponse)
	if !ok {
		return nil, fmt.Errorf("unexpected availability territories response type %T", allPages)
	}

	available := make(map[string]struct{}, len(typed.Data))
	for _, territory := range typed.Data {
		id := strings.ToUpper(strings.TrimSpace(territory.ID))
		if id == "" {
			continue
		}
		available[id] = struct{}{}
	}
	return available, nil
}

func pluralizeEqualizeTerritories(n int) string {
	if n == 1 {
		return "y"
	}
	return "ies"
}

func summarizeEqualizeTerritories(territories []string, limit int) string {
	if len(territories) == 0 {
		return ""
	}
	if limit <= 0 || len(territories) <= limit {
		return strings.Join(territories, ", ")
	}
	return fmt.Sprintf("%s, and %d more", strings.Join(territories[:limit], ", "), len(territories)-limit)
}

func equalizationTerritoryID(pricePoint asc.Resource[asc.SubscriptionPricePointAttributes]) (string, error) {
	if territory := territoryFromPricePointRelationships(pricePoint.Relationships); territory != "" {
		return territory, nil
	}
	return "", fmt.Errorf("failed to resolve territory for equalized price point %q; ensure include=territory is set", pricePoint.ID)
}

func territoryFromPricePointRelationships(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}

	var relationships struct {
		Territory *asc.Relationship `json:"territory"`
	}
	if err := json.Unmarshal(raw, &relationships); err != nil {
		return ""
	}
	if relationships.Territory == nil {
		return ""
	}
	return strings.ToUpper(strings.TrimSpace(relationships.Territory.Data.ID))
}

func printEqualizeResult(result *equalizeResult, format string, pretty bool) error {
	return shared.PrintOutputWithRenderers(
		result,
		format,
		pretty,
		func() error { return printEqualizeTable(result) },
		func() error { return printEqualizeMarkdown(result) },
	)
}

func printEqualizeTable(result *equalizeResult) error {
	if result.DryRun {
		headers := []string{"Territory", "Price", "Price Point ID"}
		rows := make([][]string, 0, len(result.Territories))
		for _, t := range result.Territories {
			rows = append(rows, []string{t.Territory, t.Price, t.PricePointID})
		}
		asc.RenderTable(headers, rows)
		return nil
	}

	fmt.Printf("Subscription: %s\n", result.SubscriptionID)
	fmt.Printf("Base: %s @ %s\n", result.BaseTerritory, result.BasePrice)
	if result.StartDate != "" {
		fmt.Printf("Start Date: %s\n", result.StartDate)
	}
	if result.AutoScheduled {
		fmt.Printf("Auto Scheduled: true (%s)\n", result.SubscriptionState)
	}
	if result.Preserved {
		fmt.Println("Preserved: true")
	}
	fmt.Printf("Total: %d, Succeeded: %d, Failed: %d\n", result.Total, result.Succeeded, result.Failed)

	if len(result.Failures) > 0 {
		fmt.Println("\nFailures:")
		headers := []string{"Territory", "Price", "Error"}
		rows := make([][]string, 0, len(result.Failures))
		for _, f := range result.Failures {
			rows = append(rows, []string{f.Territory, f.Price, f.Error})
		}
		asc.RenderTable(headers, rows)
	}

	return nil
}

func printEqualizeMarkdown(result *equalizeResult) error {
	if result.DryRun {
		headers := []string{"Territory", "Price", "Price Point ID"}
		rows := make([][]string, 0, len(result.Territories))
		for _, t := range result.Territories {
			rows = append(rows, []string{t.Territory, t.Price, t.PricePointID})
		}
		asc.RenderMarkdown(headers, rows)
		return nil
	}

	fmt.Printf("## Equalize Results\n\n")
	fmt.Printf("- **Subscription:** %s\n", result.SubscriptionID)
	fmt.Printf("- **Base:** %s @ %s\n", result.BaseTerritory, result.BasePrice)
	if result.StartDate != "" {
		fmt.Printf("- **Start Date:** %s\n", result.StartDate)
	}
	if result.AutoScheduled {
		fmt.Printf("- **Auto Scheduled:** true (%s)\n", result.SubscriptionState)
	}
	if result.Preserved {
		fmt.Printf("- **Preserved:** true\n")
	}
	fmt.Printf("- **Total:** %d, **Succeeded:** %d, **Failed:** %d\n\n", result.Total, result.Succeeded, result.Failed)

	if len(result.Failures) > 0 {
		headers := []string{"Territory", "Price", "Error"}
		rows := make([][]string, 0, len(result.Failures))
		for _, f := range result.Failures {
			rows = append(rows, []string{f.Territory, f.Price, f.Error})
		}
		asc.RenderMarkdown(headers, rows)
	}

	return nil
}
