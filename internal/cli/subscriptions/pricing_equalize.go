package subscriptions

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/url"
	"regexp"
	"slices"
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

// newSubscriptionsPricingEqualizeCommand returns the equalize subcommand.
func newSubscriptionsPricingEqualizeCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("equalize", flag.ExitOnError)

	subscriptionID := shared.BindResourceIDFlag(fs, "subscription-id", "subscriptions", "Subscription ID, product ID, or exact current name (required)")
	appID := addSubscriptionLookupAppFlag(fs)
	baseTerritory := fs.String("base-territory", "USA", "Pricing base territory (accepts alpha-2, alpha-3, or exact English country name)")
	basePrice := fs.String("base-price", "", "Customer price in the base territory (required)")
	startDate := fs.String("start-date", "", "Start date (YYYY-MM-DD) for scheduled price changes")
	preserved := fs.Bool("preserved", false, "Preserve existing prices")
	autoStartDate := fs.Bool("auto-start-date", true, "Automatically schedule approved/live subscriptions for tomorrow, or the earliest later date App Store Connect accepts, when --start-date is omitted")
	dryRun := fs.Bool("dry-run", false, "Show equalized prices without applying them")
	confirm := fs.Bool("confirm", false, "Confirm applying equalized prices (required unless --dry-run)")
	workers := fs.Int("workers", defaultEqualizeWorkers, "Number of concurrent API requests")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "equalize",
		ShortUsage: "asc subscriptions pricing equalize [flags]",
		ShortHelp:  "Set equalized prices for all territories from a base price.",
		LongHelp: `Set equalized prices for all territories from a base price.

Finds the price point matching the given base territory and price, fetches
Apple's equalized prices for all other territories, and sets them in one
operation. This replaces the manual process of exporting equalizations and
importing a CSV.

Approved or live subscriptions are scheduled for tomorrow (US Pacific) unless
--start-date is set; when App Store Connect requires a later start date, the
command uses the earliest one it accepts. After applying, every territory is
read back on the start date, and any territory that does not resolve to its
equalized price counts as failed.

Prerequisite: the subscription's availability must already include every
pricing territory. Equalize only sets prices; configure territories first with
` + "`asc subscriptions pricing availability edit`" + `.

Examples:
  asc subscriptions pricing equalize --subscription-id "SUB_ID" --base-price "3.49" --dry-run
  asc subscriptions pricing equalize --subscription-id "SUB_ID" --base-price "3.49" --confirm
  asc subscriptions pricing equalize --subscription-id "SUB_ID" --base-price "3.49" --start-date "YYYY-MM-DD" --confirm
  asc subscriptions pricing equalize --subscription-id "SUB_ID" --base-price "38.49" --base-territory "United States" --confirm
  asc subscriptions pricing equalize --subscription-id "SUB_ID" --base-price "3.49" --confirm --workers 16`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			subID := strings.TrimSpace(*subscriptionID)
			if subID == "" {
				return d.RequiredFlag("--subscription-id")
			}
			price := strings.TrimSpace(*basePrice)
			if price == "" {
				return d.RequiredFlag("--base-price")
			}
			if err := shared.ValidateFinitePriceFlag("--base-price", price); err != nil {
				return d.UsageError(err.Error())
			}
			explicitStartDate, effectiveAt, err := normalizeEqualizeStartDate(d, *startDate)
			if err != nil {
				return err
			}
			territoryInput := strings.TrimSpace(*baseTerritory)
			if territoryInput == "" {
				territoryInput = "USA"
			}
			territory, err := ascterritory.Normalize(territoryInput)
			if err != nil {
				return d.UsageError(err.Error())
			}
			if err := d.RequireConfirmUnlessDryRun(*dryRun, *confirm); err != nil {
				return err
			}
			numWorkers := *workers
			if numWorkers < 1 || numWorkers > 32 {
				return d.UsageError("--workers must be between 1 and 32")
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("equalize: %w", err)
			}

			resolveCtx, resolveCancel := d.Timeout(ctx)
			subID, err = resolveSubscriptionLookupID(resolveCtx, d, client, *appID, subID)
			resolveCancel()
			if err != nil {
				return err
			}

			// Step 0: Fail fast if sale availability does not already cover all pricing territories.
			fmt.Fprintln(d.IO.ErrOut, "Checking subscription availability coverage...")
			coveredTerritories, err := validateEqualizeAvailability(ctx, d, client, subID)
			if err != nil {
				return fmt.Errorf("equalize: %w", err)
			}
			fmt.Fprintf(d.IO.ErrOut, "Availability covers %d pricing territories\n", coveredTerritories)

			// Step 1: Find the base price point
			fmt.Fprintf(d.IO.ErrOut, "Finding %s price point for %s...\n", territory, price)
			pricePointID, err := findPricePoint(ctx, d, client, subID, territory, price)
			if err != nil {
				return fmt.Errorf("equalize: %w", err)
			}
			fmt.Fprintf(d.IO.ErrOut, "Found price point: %s\n", pricePointID)

			// Step 2: Get equalizations for all territories
			fmt.Fprintf(d.IO.ErrOut, "Fetching equalized prices for all territories...\n")
			equalizations, err := fetchEqualizations(ctx, d, client, pricePointID, territory)
			if err != nil {
				return fmt.Errorf("equalize: %w", err)
			}
			fmt.Fprintf(d.IO.ErrOut, "Got %d territory equalizations\n", len(equalizations))

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
					existingCtx, existingCancel := d.Timeout(ctx)
					hasExistingPrices, err := subscriptionHasExistingPrices(existingCtx, client, subID)
					existingCancel()
					if err != nil {
						return fmt.Errorf("equalize: failed to check existing prices: %w", err)
					}
					if hasExistingPrices {
						var scheduleErr error
						priceAttrs.StartDate, subscriptionState, autoScheduled, _, scheduleErr = autoScheduleEqualizeStartDate(ctx, d, client, subID)
						if scheduleErr != nil {
							return fmt.Errorf("equalize: %w", scheduleErr)
						}
					}
				}

				return printEqualizeResult(d, &equalizeResult{
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
			fmt.Fprintf(d.IO.ErrOut, "Setting prices for %d territories (%d workers)...\n", len(allTerritories), numWorkers)
			succeeded := 0
			failures := make([]equalizeAttemptFailure, 0)

			existingCtx, existingCancel := d.Timeout(ctx)
			hasExistingPrices, err := subscriptionHasExistingPrices(existingCtx, client, subID)
			existingCancel()
			if err != nil {
				return fmt.Errorf("equalize: failed to check existing prices: %w", err)
			}

			if hasExistingPrices {
				if err := requireSingleTierBulkPriceWrite(ctx, d, client, subID, nil); err != nil {
					return fmt.Errorf("equalize: %w", err)
				}
			}

			if priceAttrs.StartDate == "" && *autoStartDate && hasExistingPrices {
				var scheduleErr error
				priceAttrs.StartDate, subscriptionState, autoScheduled, effectiveAt, scheduleErr = autoScheduleEqualizeStartDate(ctx, d, client, subID)
				if scheduleErr != nil {
					return fmt.Errorf("equalize: %w", scheduleErr)
				}
				if autoScheduled {
					fmt.Fprintf(d.IO.ErrOut, "Subscription state is %s; scheduling price changes for %s\n", subscriptionState, priceAttrs.StartDate)
				}
			}

			buildResult := func() *equalizeResult {
				return &equalizeResult{
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
			}
			stopAfterBaseFailure := func(message string, cause error) error {
				result := buildResult()
				fmt.Fprintf(d.IO.ErrOut, "Done: %d succeeded, %d failed\n", result.Succeeded, result.Failed)
				if err := printEqualizeResult(d, result, *output.Output, *output.Pretty); err != nil {
					return err
				}
				return shared.NewReportedError(shared.NewErrorWithCause(errors.New(message), cause))
			}

			remainingTerritories := allTerritories
			if !hasExistingPrices && len(allTerritories) > 0 {
				fmt.Fprintf(d.IO.ErrOut, "Subscription has no prices; setting initial price in %s first...\n", territory)

				baseTarget := allTerritories[0]
				_, err := runReconciledMutation(
					ctx,
					d,
					func(readbackCtx context.Context) (bool, error) {
						reconciled, _, readErr := reconcileEqualizeFailures(
							readbackCtx,
							d,
							client,
							subID,
							[]equalizeAttemptFailure{{Target: baseTarget}},
							effectiveAt,
							priceAttrs.PlanType,
						)
						return reconciled == 1, readErr
					},
					func(mutationCtx context.Context) error {
						_, mutationErr := client.SetSubscriptionInitialPrice(mutationCtx, subID, baseTarget.PricePointID, baseTarget.Territory, priceAttrs)
						return mutationErr
					},
				)
				if err != nil {
					failures = append(failures, equalizeAttemptFailure{Target: baseTarget, Err: err})
					return stopAfterBaseFailure("equalize: failed to set initial price in "+baseTarget.Territory, err)
				}
				succeeded++
				remainingTerritories = allTerritories[1:]
			} else if autoScheduled && len(allTerritories) > 0 {
				// Apple's minimum lead time for a scheduled price is undocumented
				// and can exceed one day, so the base territory goes first and
				// adopts the earliest start date Apple names when it rejects ours.
				// The probe reads Apple's raw error: applyEqualizedPrices would
				// reconcile a rejection away when the base already has its price.
				baseTarget := allTerritories[0]
				baseSucceeded, baseFailures := runEqualizePricePass(ctx, d, client, subID, allTerritories[:1], 1, priceAttrs)
				if len(baseFailures) > 0 {
					if earliest, ok := appleEarliestSubscriptionPriceStartDate(baseFailures[0].Err); ok && earliest.After(effectiveAt) {
						fmt.Fprintf(d.IO.ErrOut, "App Store Connect rejected %s as too soon; scheduling price changes for %s\n", priceAttrs.StartDate, earliest.Format(equalizeDateLayout))
						priceAttrs.StartDate = earliest.Format(equalizeDateLayout)
						effectiveAt = earliest
					}
					baseSucceeded, baseFailures = applyEqualizedPrices(ctx, d, client, subID, allTerritories[:1], 1, priceAttrs, effectiveAt)
				}
				if len(baseFailures) > 0 {
					failures = append(failures, baseFailures...)
					return stopAfterBaseFailure("equalize: failed to set price in "+baseTarget.Territory, baseFailures[0].Err)
				}
				succeeded += baseSucceeded
				remainingTerritories = allTerritories[1:]
			}

			passSucceeded, passFailures := applyEqualizedPrices(ctx, d, client, subID, remainingTerritories, numWorkers, priceAttrs, effectiveAt)
			succeeded += passSucceeded
			failures = append(failures, passFailures...)

			// A 201 does not prove the price applies: read every territory back on
			// the date the new prices take effect.
			verifiedSucceeded, verifiedFailures, verifyErr := verifyEqualizedPrices(ctx, d, client, subID, allTerritories, failures, effectiveAt, priceAttrs.PlanType)
			if verifyErr == nil {
				succeeded = verifiedSucceeded
				failures = verifiedFailures
			}

			result := buildResult()
			fmt.Fprintf(d.IO.ErrOut, "Done: %d succeeded, %d failed\n", result.Succeeded, result.Failed)

			if err := printEqualizeResult(d, result, *output.Output, *output.Pretty); err != nil {
				return err
			}
			if result.Failed > 0 {
				// Concurrent failures arrive in any order; the territory that sorts
				// first supplies the cause, so the exit code is stable across runs.
				primary := slices.MinFunc(failures, func(a, b equalizeAttemptFailure) int {
					return strings.Compare(a.Target.Territory, b.Target.Territory)
				})
				return shared.NewReportedError(shared.NewErrorWithCause(fmt.Errorf("equalize: %d territory update(s) failed", result.Failed), primary.Err))
			}
			if verifyErr != nil {
				return shared.NewReportedError(fmt.Errorf("equalize: App Store Connect accepted %d territory update(s), but they could not be verified: %w", result.Succeeded, verifyErr))
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

func applyEqualizedPrices(ctx context.Context, d *shared.Deps, client *asc.Client, subID string, targets []equalization, workers int, attrs asc.SubscriptionPriceCreateAttributes, effectiveAt time.Time) (int, []equalizeAttemptFailure) {
	succeeded, failures := runEqualizePricePass(ctx, d, client, subID, targets, workers, attrs)
	retryable, finalFailures := partitionEqualizeFailures(ctx, failures)

	for pass := 1; len(retryable) > 0 && pass <= maxEqualizeRecoveryPasses; pass++ {
		reconciled, unresolved, verifyErr := reconcileEqualizeFailures(ctx, d, client, subID, retryable, effectiveAt, attrs.PlanType)
		if verifyErr != nil {
			fmt.Fprintf(d.IO.ErrOut, "Warning: could not verify %d retryable territory update(s) before replay: %v\n", len(retryable), verifyErr)
			finalFailures = append(finalFailures, retryable...)
			retryable = nil
			break
		}
		succeeded += reconciled
		retryable = unresolved
		if len(retryable) == 0 {
			break
		}

		if delay := maxEqualizeRetryAfter(retryable); delay > 0 {
			fmt.Fprintf(d.IO.ErrOut, "Waiting %s before retrying %d retryable territory update(s)...\n", delay.Round(time.Second), len(retryable))
			if err := sleepWithContext(ctx, d, delay); err != nil {
				finalFailures = append(finalFailures, retryable...)
				return succeeded, finalFailures
			}
		}
		fmt.Fprintf(d.IO.ErrOut, "Retrying %d retryable territory update(s) with %d worker...\n", len(retryable), equalizeRecoveryWorkers)
		retrySucceeded, retryFailures := runEqualizePricePass(ctx, d, client, subID, equalizeTargetsFromFailures(retryable), equalizeRecoveryWorkers, attrs)
		succeeded += retrySucceeded

		retryable, failures = partitionEqualizeFailures(ctx, retryFailures)
		finalFailures = append(finalFailures, failures...)
	}

	if len(retryable) > 0 {
		finalFailures = append(finalFailures, retryable...)
	}

	reconciled, remaining, err := reconcileEqualizeFailures(ctx, d, client, subID, finalFailures, effectiveAt, attrs.PlanType)
	if err != nil {
		fmt.Fprintf(d.IO.ErrOut, "Warning: could not verify %d failed territory update(s): %v\n", len(finalFailures), err)
		return succeeded, finalFailures
	}
	succeeded += reconciled
	return succeeded, remaining
}

var errEqualizePriceNotApplied = errors.New("price not applied")

// verifyEqualizedPrices reads every target territory back on effectiveAt. A
// territory succeeds only when it resolves to its target price point; one that
// does not keeps its submission error, or reports that App Store Connect
// accepted the price without applying it.
func verifyEqualizedPrices(ctx context.Context, d *shared.Deps, client *asc.Client, subID string, targets []equalization, submissionFailures []equalizeAttemptFailure, effectiveAt time.Time, planType asc.SubscriptionPlanType) (int, []equalizeAttemptFailure, error) {
	fmt.Fprintf(d.IO.ErrOut, "Verifying %d territory price(s) on %s...\n", len(targets), effectiveAt.Format(equalizeDateLayout))

	resolved, err := fetchResolvedSubscriptionPrices(ctx, d, client, subID, 200, "", effectiveAt, planType, "")
	if err != nil {
		return 0, nil, err
	}
	currentByTerritory := equalizedPlanPricePoints(resolved.Prices, planType)
	submissionErrByTerritory := make(map[string]error, len(submissionFailures))
	for _, failure := range submissionFailures {
		submissionErrByTerritory[strings.ToUpper(strings.TrimSpace(failure.Target.Territory))] = failure.Err
	}

	succeeded := 0
	failures := make([]equalizeAttemptFailure, 0)
	for _, target := range targets {
		territoryID := strings.ToUpper(strings.TrimSpace(target.Territory))
		current := currentByTerritory[territoryID]
		if current == strings.TrimSpace(target.PricePointID) {
			succeeded++
			continue
		}
		failureErr := submissionErrByTerritory[territoryID]
		if failureErr == nil {
			if current == "" {
				current = "no price"
			}
			failureErr = fmt.Errorf("%w: App Store Connect accepted the price, but %s resolves to %s on %s instead of price point %s", errEqualizePriceNotApplied, territoryID, current, effectiveAt.Format(equalizeDateLayout), target.PricePointID)
		}
		failures = append(failures, equalizeAttemptFailure{Target: target, Err: failureErr})
	}
	return succeeded, failures, nil
}

var appleEarliestStartDatePattern = regexp.MustCompile(`(?i)startDate\b.*\bmust be on or after (\d{4}-\d{2}-\d{2})`)

// appleEarliestSubscriptionPriceStartDate returns the earliest start date App
// Store Connect names when it rejects a subscription price start date as too
// soon, for example "Invalid startDate=2026-11-13, must be on or after
// 2026-11-14" (observed 2026-11-12).
func appleEarliestSubscriptionPriceStartDate(err error) (time.Time, bool) {
	apiErr, ok := errors.AsType[*asc.APIError](err)
	if !ok {
		return time.Time{}, false
	}
	for _, detail := range append([]string{apiErr.Detail}, apiErr.AllDetails...) {
		match := appleEarliestStartDatePattern.FindStringSubmatch(detail)
		if match == nil {
			continue
		}
		earliest, parseErr := time.Parse(equalizeDateLayout, match[1])
		if parseErr != nil {
			continue
		}
		return earliest, true
	}
	return time.Time{}, false
}

func runEqualizePricePass(ctx context.Context, d *shared.Deps, client *asc.Client, subID string, targets []equalization, workers int, attrs asc.SubscriptionPriceCreateAttributes) (int, []equalizeAttemptFailure) {
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
			setCtx, setCancel := d.Timeout(ctx)
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

			setCtx, setCancel := d.Timeout(ctx)
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

func partitionEqualizeFailures(ctx context.Context, failures []equalizeAttemptFailure) (retryable []equalizeAttemptFailure, final []equalizeAttemptFailure) {
	for _, failure := range failures {
		if shared.IsTransientMutationError(ctx, failure.Err) {
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

// normalizeEqualizeStartDate returns the normalized --start-date and the
// pricing date the new prices take effect on: today's US Pacific date when
// --start-date is omitted, or the explicit start date, which must be after
// today's US Pacific date.
func normalizeEqualizeStartDate(d *shared.Deps, value string) (string, time.Time, error) {
	today := subscriptionPricingToday(d)
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", today, nil
	}

	normalized, err := shared.NormalizeDate(trimmed, "--start-date")
	if err != nil {
		return "", time.Time{}, d.UsageError(err.Error())
	}
	parsed, err := time.Parse(equalizeDateLayout, normalized)
	if err != nil {
		return "", time.Time{}, err
	}
	if !parsed.After(today) {
		return "", time.Time{}, d.UsageError("--start-date must be a future date")
	}
	return normalized, parsed, nil
}

// autoScheduleEqualizeStartDate schedules approved or live subscriptions for
// tomorrow, the day after today's US Pacific pricing date. It returns the
// start date and the pricing date the prices take effect on.
func autoScheduleEqualizeStartDate(ctx context.Context, d *shared.Deps, client *asc.Client, subID string) (string, string, bool, time.Time, error) {
	state, err := fetchEqualizeSubscriptionState(ctx, d, client, subID)
	if err != nil {
		return "", "", false, time.Time{}, fmt.Errorf("failed to inspect subscription state for auto scheduling: %w", err)
	}
	today := subscriptionPricingToday(d)
	if !isApprovedOrLiveSubscriptionState(state) {
		return "", state, false, today, nil
	}

	effectiveDate := today.AddDate(0, 0, 1)
	return effectiveDate.Format(equalizeDateLayout), state, true, effectiveDate, nil
}

func fetchEqualizeSubscriptionState(ctx context.Context, d *shared.Deps, client *asc.Client, subID string) (string, error) {
	getCtx, getCancel := d.Timeout(ctx)
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

func reconcileEqualizeFailures(ctx context.Context, d *shared.Deps, client *asc.Client, subID string, failures []equalizeAttemptFailure, effectiveAt time.Time, planType asc.SubscriptionPlanType) (int, []equalizeAttemptFailure, error) {
	if len(failures) == 0 {
		return 0, nil, nil
	}

	fmt.Fprintf(d.IO.ErrOut, "Verifying %d territory update(s) against current prices...\n", len(failures))

	resolved, err := fetchResolvedSubscriptionPrices(ctx, d, client, subID, 200, "", effectiveAt, planType, "")
	if err != nil {
		return 0, failures, err
	}

	currentByTerritory := equalizedPlanPricePoints(resolved.Prices, planType)

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

// equalizedPlanPricePoints maps each territory to its price point in the plan
// equalize writes: planType, or Apple's default UPFRONT plan when it is empty.
// A subscription that also bills monthly has MONTHLY rows for the same
// territories, which must not stand in for the written plan.
func equalizedPlanPricePoints(rows []shared.ResolvedPriceRow, planType asc.SubscriptionPlanType) map[string]string {
	written := asc.SubscriptionPlanTypeUpfront
	if planType != "" {
		written = planType
	}
	byTerritory := make(map[string]string, len(rows))
	for _, row := range rows {
		territoryID := strings.ToUpper(strings.TrimSpace(row.Territory))
		rowPlanType := strings.ToUpper(strings.TrimSpace(row.PlanType))
		if territoryID == "" || (rowPlanType != "" && rowPlanType != string(written)) {
			continue
		}
		byTerritory[territoryID] = strings.TrimSpace(row.PricePointID)
	}
	return byTerritory
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

func sleepWithContext(ctx context.Context, d *shared.Deps, delay time.Duration) error {
	if delay <= 0 {
		return nil
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-d.After(delay):
		return nil
	}
}

func findPricePoint(ctx context.Context, d *shared.Deps, client *asc.Client, subID, territory, targetPrice string) (string, error) {
	// List price points filtered by the base territory, paginating to find the matching price
	var pricePointID string
	priceFilter := shared.PriceFilter{Price: targetPrice}

	firstCtx, firstCancel := d.Timeout(ctx)
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
			pageCtx, pageCancel := d.Timeout(ctx)
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

	return "", shared.WithDiagnostic(
		shared.NewValidationError(fmt.Errorf(
			"no price point found for %s %s; list valid prices with `asc subscriptions pricing price-points list --subscription-id %q --territory %s --paginate`",
			territory, targetPrice, subID, territory,
		)),
		shared.DiagnosticInvalidInput,
		"--base-price",
	)
}

func fetchEqualizations(ctx context.Context, d *shared.Deps, client *asc.Client, pricePointID, baseTerritory string) ([]equalization, error) {
	// Use include=territory so the API populates each price point's
	// relationships with the territory reference, avoiding reliance on
	// opaque price point ID structure.
	firstCtx, firstCancel := d.Timeout(ctx)
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

	var count priceCollectionCount
	if err := checkPriceCollectionPage(&count, resp, "equalization"); err != nil {
		return nil, err
	}
	allPages, err := asc.PaginateAll(ctx, resp, func(_ context.Context, nextURL string) (asc.PaginatedResponse, error) {
		pageCtx, pageCancel := d.Timeout(ctx)
		defer pageCancel()
		canonical, err := client.ValidateNoRedirectReadContinuationQuery(ctx, nextURL, "/v1/subscriptionPricePoints/"+url.PathEscape(pricePointID)+"/equalizations", url.Values{
			"include": {"territory"}, "fields[subscriptionPricePoints]": {"customerPrice,territory"},
		})
		if err != nil {
			return nil, err
		}
		page, err := client.GetSubscriptionPricePointEqualizations(
			pageCtx, pricePointID,
			asc.WithSubscriptionPricePointsNextURL(canonical),
		)
		if err != nil {
			return nil, err
		}
		return page, checkPriceCollectionPage(&count, page, "equalization")
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

func validateEqualizeAvailability(ctx context.Context, d *shared.Deps, client *asc.Client, subID string) (int, error) {
	getCtx, getCancel := d.Timeout(ctx)
	availability, err := client.GetSubscriptionAvailabilityForSubscription(getCtx, subID)
	getCancel()
	if err != nil {
		if errors.Is(err, asc.ErrNotFound) {
			exists, verifyErr := subscriptionExists(ctx, d, client, subID)
			if verifyErr != nil {
				return 0, verifyErr
			}
			if !exists {
				return 0, shared.WithDiagnostic(
					shared.NewErrorWithCause(fmt.Errorf("subscription %q was not found", subID), err),
					shared.DiagnosticResourceNotFound,
					"--subscription-id",
				)
			}
			return 0, equalizeAvailabilityNotReady(errors.New("subscription availability is not configured; equalize only updates prices and will not change sale availability. Configure territories first with `asc subscriptions pricing availability edit`"))
		}
		return 0, fmt.Errorf("failed to fetch availability: %w", err)
	}

	availabilityID := strings.TrimSpace(availability.Data.ID)
	if availabilityID == "" {
		return 0, fmt.Errorf("availability readback returned empty id")
	}

	requiredTerritories, err := fetchPricingTerritories(ctx, d, client)
	if err != nil {
		return 0, err
	}
	if len(requiredTerritories) == 0 {
		return 0, nil
	}

	available, err := fetchSubscriptionAvailabilityTerritories(ctx, d, client, availabilityID)
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
	return 0, equalizeAvailabilityNotReady(fmt.Errorf("subscription availability is missing %d equalized territor%s (%s); equalize only updates prices and will not change sale availability. Configure territories first with `asc subscriptions pricing availability edit`", len(missing), pluralizeEqualizeTerritories(len(missing)), summarizeEqualizeTerritories(missing, 8)))
}

func equalizeAvailabilityNotReady(err error) error {
	return shared.WithDiagnostic(shared.NewValidationError(err), shared.DiagnosticStateNotReady, "--subscription-id")
}

func subscriptionExists(ctx context.Context, d *shared.Deps, client *asc.Client, subID string) (bool, error) {
	getCtx, getCancel := d.Timeout(ctx)
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

func fetchPricingTerritories(ctx context.Context, d *shared.Deps, client *asc.Client) ([]string, error) {
	firstCtx, firstCancel := d.Timeout(ctx)
	firstPage, err := client.GetTerritories(firstCtx, asc.Limit[asc.TerritoriesOption](200))
	firstCancel()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch pricing territories: %w", err)
	}

	allPages, err := asc.PaginateAll(ctx, firstPage, func(_ context.Context, nextURL string) (asc.PaginatedResponse, error) {
		pageCtx, pageCancel := d.Timeout(ctx)
		defer pageCancel()
		return client.GetTerritories(pageCtx, asc.NextURL[asc.TerritoriesOption](nextURL))
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

func fetchSubscriptionAvailabilityTerritories(ctx context.Context, d *shared.Deps, client *asc.Client, availabilityID string) (map[string]struct{}, error) {
	territoriesCtx, territoriesCancel := d.Timeout(ctx)
	firstPage, err := client.GetSubscriptionAvailabilityAvailableTerritories(territoriesCtx, availabilityID, asc.Limit[asc.SubscriptionAvailabilityTerritoriesOption](200))
	territoriesCancel()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch availability territories: %w", err)
	}

	allPages, err := asc.PaginateAll(ctx, firstPage, func(_ context.Context, nextURL string) (asc.PaginatedResponse, error) {
		pageCtx, pageCancel := d.Timeout(ctx)
		defer pageCancel()
		return client.GetSubscriptionAvailabilityAvailableTerritories(pageCtx, availabilityID, asc.NextURL[asc.SubscriptionAvailabilityTerritoriesOption](nextURL))
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
	if territory := shared.PricePointTerritoryID(pricePoint.Relationships); territory != "" {
		return territory, nil
	}
	return "", fmt.Errorf("failed to resolve territory for equalized price point %q; ensure include=territory is set", pricePoint.ID)
}

func printEqualizeResult(d *shared.Deps, result *equalizeResult, format string, pretty bool) error {
	return d.PrintOutputWithRenderers(
		result,
		format,
		pretty,
		func() error { return printEqualizeTable(d, result) },
		func() error { return printEqualizeMarkdown(d, result) },
	)
}

func printEqualizeTable(d *shared.Deps, result *equalizeResult) error {
	if result.DryRun {
		headers := []string{"Territory", "Price", "Price Point ID"}
		rows := make([][]string, 0, len(result.Territories))
		for _, t := range result.Territories {
			rows = append(rows, []string{t.Territory, t.Price, t.PricePointID})
		}
		asc.RenderTableTo(d.IO.Out, headers, rows)
		return nil
	}

	fmt.Fprintf(d.IO.Out, "Subscription: %s\n", result.SubscriptionID)
	fmt.Fprintf(d.IO.Out, "Base: %s @ %s\n", result.BaseTerritory, result.BasePrice)
	if result.StartDate != "" {
		fmt.Fprintf(d.IO.Out, "Start Date: %s\n", result.StartDate)
	}
	if result.AutoScheduled {
		fmt.Fprintf(d.IO.Out, "Auto Scheduled: true (%s)\n", result.SubscriptionState)
	}
	if result.Preserved {
		fmt.Fprintln(d.IO.Out, "Preserved: true")
	}
	fmt.Fprintf(d.IO.Out, "Total: %d, Succeeded: %d, Failed: %d\n", result.Total, result.Succeeded, result.Failed)

	if len(result.Failures) > 0 {
		fmt.Fprintln(d.IO.Out, "\nFailures:")
		headers := []string{"Territory", "Price", "Error"}
		rows := make([][]string, 0, len(result.Failures))
		for _, f := range result.Failures {
			rows = append(rows, []string{f.Territory, f.Price, f.Error})
		}
		asc.RenderTableTo(d.IO.Out, headers, rows)
	}

	return nil
}

func printEqualizeMarkdown(d *shared.Deps, result *equalizeResult) error {
	if result.DryRun {
		headers := []string{"Territory", "Price", "Price Point ID"}
		rows := make([][]string, 0, len(result.Territories))
		for _, t := range result.Territories {
			rows = append(rows, []string{t.Territory, t.Price, t.PricePointID})
		}
		asc.RenderMarkdownTo(d.IO.Out, headers, rows)
		return nil
	}

	fmt.Fprintf(d.IO.Out, "## Equalize Results\n\n")
	fmt.Fprintf(d.IO.Out, "- **Subscription:** %s\n", result.SubscriptionID)
	fmt.Fprintf(d.IO.Out, "- **Base:** %s @ %s\n", result.BaseTerritory, result.BasePrice)
	if result.StartDate != "" {
		fmt.Fprintf(d.IO.Out, "- **Start Date:** %s\n", result.StartDate)
	}
	if result.AutoScheduled {
		fmt.Fprintf(d.IO.Out, "- **Auto Scheduled:** true (%s)\n", result.SubscriptionState)
	}
	if result.Preserved {
		fmt.Fprintf(d.IO.Out, "- **Preserved:** true\n")
	}
	fmt.Fprintf(d.IO.Out, "- **Total:** %d, **Succeeded:** %d, **Failed:** %d\n\n", result.Total, result.Succeeded, result.Failed)

	if len(result.Failures) > 0 {
		headers := []string{"Territory", "Price", "Error"}
		rows := make([][]string, 0, len(result.Failures))
		for _, f := range result.Failures {
			rows = append(rows, []string{f.Territory, f.Price, f.Error})
		}
		asc.RenderMarkdownTo(d.IO.Out, headers, rows)
	}

	return nil
}
