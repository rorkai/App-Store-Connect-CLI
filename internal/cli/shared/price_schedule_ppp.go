package shared

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"math/big"
	"os"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/ascterritory"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/pppindex"
)

// PricePointTerritoryID returns the territory ID from a price point's
// relationships, or "" when the territory relationship was not included.
func PricePointTerritoryID(raw json.RawMessage) string {
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

// ScheduledPricePoint is one manual price in a replacement price schedule.
type ScheduledPricePoint struct {
	PricePointID string
	StartDate    string
	EndDate      string
}

// PriceSchedulePPPSurface adapts an app or in-app purchase price schedule to
// the purchasing-power planner.
type PriceSchedulePPPSurface struct {
	AppID string
	IAPID string
	// CurrentPrices returns each territory's effective price on the pricing
	// date containing at.
	CurrentPrices func(ctx context.Context, at time.Time) ([]ResolvedPriceRow, error)
	// ReadSchedule returns the complete forward schedule for scoped replacements.
	ReadSchedule func(ctx context.Context) (PPPScheduleSnapshot, error)
	// LadderBatch returns the price points of a batch of territories, keyed by
	// territory, reading every page.
	LadderBatch func(ctx context.Context, territories []string) (map[string][]PriceLadderCandidate, error)
	// LadderBatchSize is how many territories one LadderBatch call filters on.
	LadderBatchSize int
	// Equalizations returns Apple's equalized price points for the base price
	// point, keyed by territory.
	Equalizations func(ctx context.Context, pricePointID string) (map[string][]PriceLadderCandidate, error)
	// ReplaceSchedule creates a price schedule holding exactly prices.
	ReplaceSchedule func(ctx context.Context, baseTerritory string, prices []ScheduledPricePoint) error
	// PricePointKey maps a price point ID to a comparable key when current
	// prices name price points differently from the ladder; nil compares IDs.
	PricePointKey func(id string) string
}

// PriceSchedulePPPResource binds a command's resource flags.
type PriceSchedulePPPResource struct {
	// Validate checks the resource flags before any request.
	Validate func() error
	// Resolve looks up the resource and returns its surface.
	Resolve func(ctx context.Context, client *asc.Client) (PriceSchedulePPPSurface, error)
}

// PriceSchedulePPPCommandConfig configures pricing ppp and iap pricing ppp.
type PriceSchedulePPPCommandConfig struct {
	FlagSetName  string
	ShortUsage   string
	ResourceNoun string
	Examples     string
	ErrorPrefix  string
	BindResource func(fs *flag.FlagSet) PriceSchedulePPPResource
}

// NewPriceSchedulePPPCommand builds a purchasing-power-parity command for
// resources priced with price schedules.
func NewPriceSchedulePPPCommand(config PriceSchedulePPPCommandConfig) *ffcli.Command {
	fs := flag.NewFlagSet(config.FlagSetName, flag.ExitOnError)

	resource := config.BindResource(fs)
	baseTerritory := fs.String("base-territory", "USA", "Base territory (accepts alpha-2, alpha-3, or exact English country name)")
	selection := BindOnceCSVFlag(fs, "territory", "Only change these territories (comma-separated alpha-2, alpha-3, or English names)")
	basePrice := fs.String("base-price", "", "Customer price in the base territory (required)")
	rounding := fs.String("round", string(PriceRoundingNearest), "Price-point resolution: exact, nearest, up, or down")
	startDate := fs.String("start-date", "", "Future start date (YYYY-MM-DD) for the new prices; current prices apply until then")
	dryRun := fs.Bool("dry-run", false, "Show the purchasing-power plan without applying it")
	confirm := fs.Bool("confirm", false, "Confirm replacing the price schedule (required unless --dry-run)")
	workers := fs.Int("workers", 4, "Number of concurrent price-point ladder reads")
	output := BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "ppp",
		ShortUsage: config.ShortUsage,
		ShortHelp:  "Plan and apply purchasing-power-parity prices from a base price.",
		LongHelp: fmt.Sprintf(`Plan and apply purchasing-power-parity prices from a base price.

Finds the %[1]s price point for --base-price in --base-territory and Apple's
equalized price for it in every territory the %[1]s is priced in. Each
equalized price is multiplied by the territory's price level divided by the
base territory's price level (rounded to 4 decimals), then resolved to that
territory's Apple price-point ladder with --round.

Rounding modes:
  exact    Require the calculated price to exist.
  nearest  Choose the closest price; exact ties choose the lower price.
  up       Choose the lowest price greater than or equal to the calculation.
  down     Choose the highest price less than or equal to the calculation.

Price levels ship with the CLI: World Bank World Development Indicators PPP
conversion factor, GDP (PA.NUS.PPP) divided by the official exchange rate
(PA.NUS.FCRF), latest year from 2021 through 2025, United States = 1.
License: CC BY 4.0. Series last updated %[2]s.
Territories without a price level are skipped and keep their current price.

--territory limits PPP changes to the listed currently priced countries. Excluded
manual timelines and automatic pricing mode are preserved, using the original
schedule base. Changes to that base timeline require omitting --territory for a
global PPP update. Automatic amounts can still follow Apple FX/tax updates.

Without --territory, --confirm replaces the whole price schedule: App Store Connect drops every
manual price not in the request, and those territories fall back to automatic
equalization. The new schedule therefore holds a manual price for every
territory the %[1]s is priced in: the base territory, each planned
territory, and each skipped territory at its current price. Future scheduled
price changes are not kept. With --start-date, current prices stay until that
date. Any territory that cannot be resolved stops the run before any change.
After applying, prices are read back and compared with the plan.

Examples:
%[3]s`, config.ResourceNoun, pppindex.AsOf, config.Examples),
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := resource.Validate(); err != nil {
				return err
			}
			price := strings.TrimSpace(*basePrice)
			if price == "" {
				fmt.Fprintln(os.Stderr, "Error: --base-price is required")
				return MissingRequiredUsageError("--base-price")
			}
			basePriceValue, ok := new(big.Rat).SetString(price)
			if !IsPriceDecimal(price) || !ok || basePriceValue.Sign() <= 0 {
				return UsageError("--base-price must be a positive decimal")
			}
			territory, err := ascterritory.Normalize(*baseTerritory)
			if err != nil {
				return UsageError(fmt.Sprintf("invalid --base-territory: %v", err))
			}
			baseLevel, indexed := pppindex.PriceLevel(territory)
			if !indexed {
				return UsageError(fmt.Sprintf("invalid --base-territory: no purchasing-power price level for %s", territory))
			}
			roundMode, err := ParsePriceRounding(*rounding)
			if err != nil {
				return err
			}
			if *workers < 1 || *workers > 32 {
				return UsageError("--workers must be between 1 and 32")
			}
			if *dryRun && *confirm {
				return UsageError("--dry-run cannot be combined with --confirm")
			}
			if !*dryRun && !*confirm {
				return UsageError("--confirm is required unless --dry-run is set")
			}
			effectiveAt := PricingNow()
			start := strings.TrimSpace(*startDate)
			if start != "" {
				start, err = NormalizeDate(start, "--start-date")
				if err != nil {
					return UsageError(err.Error())
				}
				parsed, err := time.Parse("2006-01-02", start)
				if err != nil {
					return UsageError(err.Error())
				}
				if !parsed.After(PricingDate(effectiveAt)) {
					return UsageError("--start-date must be a future date")
				}
				effectiveAt = time.Date(parsed.Year(), parsed.Month(), parsed.Day(), 12, 0, 0, 0, pricingLocation())
			}

			selected, err := PPPTerritorySelection(selection)
			if err != nil {
				return err
			}
			client, err := GetASCClient()
			if err != nil {
				return fmt.Errorf("%s: %w", config.ErrorPrefix, err)
			}
			surface, err := resource.Resolve(ctx, client)
			if err != nil {
				return err
			}
			result, err := runPriceSchedulePPP(ctx, surface, priceSchedulePPPRun{
				selected:      selected,
				baseTerritory: territory,
				baseLevel:     baseLevel,
				basePrice:     basePriceValue,
				rounding:      roundMode,
				startDate:     start,
				effectiveAt:   effectiveAt,
				dryRun:        *dryRun,
				workers:       *workers,
			})
			if err != nil {
				return fmt.Errorf("%s: %w", config.ErrorPrefix, err)
			}
			if err := PrintOutput(result.output, *output.Output, *output.Pretty); err != nil {
				return err
			}
			if result.err != nil {
				return NewReportedError(fmt.Errorf("%s: %w", config.ErrorPrefix, result.err))
			}
			return nil
		},
	}
}

type priceSchedulePPPRun struct {
	selected      []string
	baseTerritory string
	baseLevel     *big.Rat
	basePrice     *big.Rat
	rounding      PriceRounding
	startDate     string
	effectiveAt   time.Time
	dryRun        bool
	workers       int
}

type priceSchedulePPPOutcome struct {
	output *asc.PriceSchedulePPPResult
	// err is a failure reported after printing output.
	err error
}

func runPriceSchedulePPP(ctx context.Context, surface PriceSchedulePPPSurface, run priceSchedulePPPRun) (priceSchedulePPPOutcome, error) {
	key := surface.PricePointKey
	if key == nil {
		key = strings.TrimSpace
	}

	fmt.Fprintln(os.Stderr, "Fetching current prices...")
	var snapshot PPPScheduleSnapshot
	var current []ResolvedPriceRow
	var err error
	if len(run.selected) > 0 {
		snapshot, err = surface.ReadSchedule(ctx)
		if err == nil {
			current, err = SelectPPPCurrentPrices(snapshot.Prices, run.selected, PricingNow())
		}
	} else {
		current, err = surface.CurrentPrices(ctx, PricingNow())
	}
	if err != nil {
		return priceSchedulePPPOutcome{}, fmt.Errorf("fetch current prices: %w", err)
	}
	if len(current) == 0 {
		return priceSchedulePPPOutcome{}, fmt.Errorf("no current prices; set a price schedule first")
	}
	currentByTerritory := make(map[string]ResolvedPriceRow, len(current)+1)
	for _, row := range current {
		if len(run.selected) > 0 {
			if effective, ok := pppPriceAt(snapshot.Prices, row.Territory, run.effectiveAt); ok {
				row = effective
			} else {
				return priceSchedulePPPOutcome{}, fmt.Errorf("no effective price for %s at requested date", row.Territory)
			}
		}
		currentByTerritory[strings.ToUpper(strings.TrimSpace(row.Territory))] = row
	}
	territories := make([]string, 0, len(currentByTerritory)+1)
	for territory := range currentByTerritory {
		territories = append(territories, territory)
	}
	if _, ok := currentByTerritory[run.baseTerritory]; !ok {
		territories = append(territories, run.baseTerritory)
	}
	sort.Strings(territories)
	targetTerritories := append([]string(nil), territories...)
	if len(run.selected) > 0 {
		targetTerritories = run.selected
		for _, row := range snapshot.Prices {
			if row.Manual != nil && *row.Manual && !slices.Contains(territories, row.Territory) {
				territories = append(territories, row.Territory)
			}
		}
		sort.Strings(territories)
	}

	fmt.Fprintf(os.Stderr, "Fetching price points for %d territories...\n", len(territories))
	ladders, err := fetchPriceLadders(ctx, surface, territories, run.workers)
	if err != nil {
		return priceSchedulePPPOutcome{}, fmt.Errorf("fetch price points: %w", err)
	}
	baseResolution, err := ResolvePriceLadderPoint(ladders[run.baseTerritory], run.basePrice, PriceRoundingExact)
	if err != nil {
		return priceSchedulePPPOutcome{}, fmt.Errorf("--base-price %s in %s: %w", FormatExactPriceDecimal(run.basePrice), run.baseTerritory, err)
	}

	fmt.Fprintln(os.Stderr, "Fetching equalized prices for all territories...")
	equalized, err := surface.Equalizations(ctx, baseResolution.PricePointID)
	if err != nil {
		return priceSchedulePPPOutcome{}, fmt.Errorf("fetch equalizations: %w", err)
	}
	equalized[run.baseTerritory] = []PriceLadderCandidate{{PricePointID: baseResolution.PricePointID, CustomerPrice: baseResolution.CustomerPrice}}

	rows := make([]asc.SubscriptionPricingDeriveRow, 0, len(territories))
	for _, territory := range targetTerritories {
		var currentRow *ResolvedPriceRow
		if row, ok := currentByTerritory[territory]; ok {
			row.PricePointID = ladderPricePointID(ladders[territory], row.PricePointID, key)
			currentRow = &row
		}
		_, indexed := pppindex.PriceLevel(territory)
		sources := equalized[territory]
		if len(sources) == 0 || !indexed {
			row := asc.SubscriptionPricingDeriveRow{Territory: territory, Action: "skip", Status: "skipped"}
			if currentRow != nil {
				row.Currency = strings.TrimSpace(currentRow.Currency)
				row.CurrentTargetPrice = strings.TrimSpace(currentRow.CustomerPrice)
				row.CurrentTargetPointID = currentRow.PricePointID
			}
			switch {
			case indexed:
				row.Action, row.Status = "error", "unresolved"
				row.Error = fmt.Sprintf("%s: no equalized price for base price point %s", territory, baseResolution.PricePointID)
			case !priceLadderHasID(ladders[territory], row.CurrentTargetPointID):
				row.Action, row.Status = "error", "unresolved"
				row.Error = fmt.Sprintf("%s: current price point %q is not on the territory's price-point ladder", territory, row.CurrentTargetPointID)
			}
			rows = append(rows, row)
			continue
		}
		sourceRow := ResolvedPriceRow{Territory: territory, CustomerPrice: sources[0].CustomerPrice, PricePointID: sources[0].PricePointID}
		if currentRow != nil {
			sourceRow.Currency = currentRow.Currency
		}
		multiplier, _ := pppindex.RelativeLevel(run.baseLevel, territory)
		rows = append(rows, PlanPriceLadderRow(sourceRow, currentRow, multiplier, ladders[territory], nil, run.rounding))
	}

	result := &asc.PriceSchedulePPPResult{
		AppID:            surface.AppID,
		IAPID:            surface.IAPID,
		BaseTerritory:    run.baseTerritory,
		BasePrice:        baseResolution.CustomerPrice,
		BasePricePointID: baseResolution.PricePointID,
		Index:            asc.SubscriptionPricingPPPIndex{Source: pppindex.Source, License: pppindex.License, AsOf: pppindex.AsOf},
		Rounding:         string(run.rounding),
		StartDate:        run.startDate,
		DryRun:           run.dryRun,
		Rows:             rows,
		Verification:     asc.SubscriptionPricingDeriveVerification{Status: "skipped"},
	}
	result.Summary = SummarizePriceLadderRows(rows)
	if result.Summary.Unresolved > 0 {
		return priceSchedulePPPOutcome{output: result, err: fmt.Errorf(
			"%d territories could not be resolved; no changes were applied", result.Summary.Unresolved,
		)}, nil
	}
	prices := scheduledPPPPrices(rows, run.startDate)
	base := run.baseTerritory
	if len(run.selected) > 0 {
		if result.Summary.Planned == 0 {
			return priceSchedulePPPOutcome{output: result}, nil
		}
		prices, err = scopedPPPPrices(snapshot, rows, run, ladders, key)
		if err != nil {
			return priceSchedulePPPOutcome{}, err
		}
		base = snapshot.BaseTerritory
	}
	if run.dryRun {
		return priceSchedulePPPOutcome{output: result}, nil
	}
	if len(run.selected) > 0 {
		fresh, err := surface.ReadSchedule(ctx)
		if err != nil {
			return priceSchedulePPPOutcome{}, fmt.Errorf("pre-write schedule read: %w", err)
		}
		if !samePPPSnapshot(snapshot, fresh, key) {
			return priceSchedulePPPOutcome{}, fmt.Errorf("price schedule changed during planning; no changes were applied")
		}
	}
	fmt.Fprintf(os.Stderr, "Replacing the price schedule with %d manual prices...\n", len(prices))
	if err := surface.ReplaceSchedule(ctx, base, prices); err != nil {
		for index := range result.Rows {
			if result.Rows[index].Status == "planned" {
				result.Rows[index].Status = "failed"
				result.Rows[index].Error = err.Error()
			}
		}
		result.Summary = SummarizePriceLadderRows(result.Rows)
		result.Verification.Status = "failed"
		return priceSchedulePPPOutcome{output: result, err: fmt.Errorf("replace price schedule: %w", err)}, nil
	}
	for index := range result.Rows {
		if result.Rows[index].Status == "planned" {
			result.Rows[index].Status = "applied"
		}
	}
	applied := SummarizePriceLadderRows(result.Rows).Applied

	fmt.Fprintln(os.Stderr, "Verifying prices...")
	verifyErr := verifyPriceSchedulePPP(ctx, surface, key, run.effectiveAt, result)
	if len(run.selected) > 0 {
		if err := verifyScopedPPPSchedule(ctx, surface, snapshot, rows, prices, ladders, run.startDate, key); err != nil {
			result.Verification.Status = "failed"
			verifyErr = err
		}
	}
	result.Summary = SummarizePriceLadderRows(result.Rows)
	result.Summary.Applied = applied
	return priceSchedulePPPOutcome{output: result, err: verifyErr}, nil
}

// scheduledPPPPrices lists one manual price per territory: the selected price
// for planned and unchanged territories and the current price for skipped
// ones. With a start date, a changed territory keeps its current price until
// that date.
func scheduledPPPPrices(rows []asc.SubscriptionPricingDeriveRow, startDate string) []ScheduledPricePoint {
	prices := make([]ScheduledPricePoint, 0, len(rows))
	for _, row := range rows {
		target := row.TargetPricePointID
		if row.Status == "skipped" {
			target = row.CurrentTargetPointID
		}
		if startDate == "" || row.CurrentTargetPointID == target {
			prices = append(prices, ScheduledPricePoint{PricePointID: target})
			continue
		}
		if row.CurrentTargetPointID != "" {
			prices = append(prices, ScheduledPricePoint{PricePointID: row.CurrentTargetPointID, EndDate: startDate})
		}
		prices = append(prices, ScheduledPricePoint{PricePointID: target, StartDate: startDate})
	}
	return prices
}

func verifyPriceSchedulePPP(
	ctx context.Context,
	surface PriceSchedulePPPSurface,
	key func(string) string,
	effectiveAt time.Time,
	result *asc.PriceSchedulePPPResult,
) error {
	readback, err := surface.CurrentPrices(ctx, effectiveAt)
	if err != nil {
		for index := range result.Rows {
			if result.Rows[index].Status == "applied" {
				result.Rows[index].Status = "failed"
				result.Rows[index].Error = fmt.Sprintf("verification readback failed: %v", err)
			}
		}
		result.Verification = asc.SubscriptionPricingDeriveVerification{Status: "failed", Failed: SummarizePriceLadderRows(result.Rows).Failed}
		return fmt.Errorf("verification readback failed: %w", err)
	}
	actualByTerritory := make(map[string]string, len(readback))
	for _, row := range readback {
		actualByTerritory[strings.ToUpper(strings.TrimSpace(row.Territory))] = strings.TrimSpace(row.PricePointID)
	}
	verified, failed := 0, 0
	for index := range result.Rows {
		row := &result.Rows[index]
		expected := row.TargetPricePointID
		if row.Status == "skipped" {
			expected = row.CurrentTargetPointID
		}
		actual := actualByTerritory[row.Territory]
		if actual != "" && key(actual) == key(expected) {
			verified++
			if row.Status == "applied" {
				row.Status = "verified"
			}
			continue
		}
		if actual == "" {
			actual = "<missing>"
		}
		row.Status = "failed"
		row.Error = fmt.Sprintf("verification mismatch: expected price point %s, got %s", expected, actual)
		failed++
	}
	result.Verification = asc.SubscriptionPricingDeriveVerification{Status: "completed", Verified: verified, Failed: failed}
	if failed > 0 {
		result.Verification.Status = "failed"
		return fmt.Errorf("verification failed for %d territories", failed)
	}
	return nil
}

func fetchPriceLadders(ctx context.Context, surface PriceSchedulePPPSurface, territories []string, workers int) (map[string][]PriceLadderCandidate, error) {
	batchSize := surface.LadderBatchSize
	if batchSize < 1 {
		batchSize = 1
	}
	var batches [][]string
	for start := 0; start < len(territories); start += batchSize {
		batches = append(batches, territories[start:min(start+batchSize, len(territories))])
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	var (
		mu       sync.Mutex
		ladders  = make(map[string][]PriceLadderCandidate, len(territories))
		firstErr error
		wg       sync.WaitGroup
	)
	jobs := make(chan []string)
	for worker := 0; worker < min(workers, len(batches)); worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for batch := range jobs {
				found, err := surface.LadderBatch(ctx, batch)
				mu.Lock()
				if err != nil && firstErr == nil {
					firstErr = err
					cancel()
				}
				for territory, candidates := range found {
					ladders[territory] = append(ladders[territory], candidates...)
				}
				mu.Unlock()
			}
		}()
	}
	for _, batch := range batches {
		select {
		case jobs <- batch:
		case <-ctx.Done():
		}
	}
	close(jobs)
	wg.Wait()
	if firstErr != nil {
		return nil, firstErr
	}
	return ladders, ctx.Err()
}

func ladderPricePointID(ladder []PriceLadderCandidate, id string, key func(string) string) string {
	id = strings.TrimSpace(id)
	for _, candidate := range ladder {
		if candidate.PricePointID == id || key(candidate.PricePointID) == key(id) {
			return candidate.PricePointID
		}
	}
	return id
}

func priceLadderHasID(ladder []PriceLadderCandidate, id string) bool {
	for _, candidate := range ladder {
		if id != "" && candidate.PricePointID == id {
			return true
		}
	}
	return false
}

// CollectPricePointsByTerritory reads every page of a price-point collection
// requested with include=territory and groups the points by territory. When
// territories is set, a point from any other territory fails the read.
func CollectPricePointsByTerritory[A any](
	ctx context.Context,
	territories []string,
	fetch func(ctx context.Context, next string) (*asc.Response[A], error),
	customerPrice func(A) string,
) (map[string][]PriceLadderCandidate, error) {
	wanted := make(map[string]bool, len(territories))
	for _, territory := range territories {
		wanted[territory] = true
	}
	read := func(next string) (*asc.Response[A], error) {
		return RetryReadWithFreshTimeout(ctx, func(requestCtx context.Context) (*asc.Response[A], error) {
			return fetch(requestCtx, next)
		})
	}
	firstPage, err := read("")
	if err != nil {
		return nil, err
	}
	byTerritory := make(map[string][]PriceLadderCandidate)
	err = asc.PaginateEach(ctx, firstPage, func(_ context.Context, next string) (asc.PaginatedResponse, error) {
		return read(next)
	}, func(page asc.PaginatedResponse) error {
		response, ok := page.(*asc.Response[A])
		if !ok {
			return fmt.Errorf("unexpected price-points response type %T", page)
		}
		for _, pricePoint := range response.Data {
			territory := PricePointTerritoryID(pricePoint.Relationships)
			if territory == "" {
				return fmt.Errorf("price point %q is missing its territory relationship", pricePoint.ID)
			}
			if len(wanted) > 0 && !wanted[territory] {
				return fmt.Errorf("price point %q returned unexpected territory %s", pricePoint.ID, territory)
			}
			byTerritory[territory] = append(byTerritory[territory], PriceLadderCandidate{
				PricePointID:  strings.TrimSpace(pricePoint.ID),
				CustomerPrice: strings.TrimSpace(customerPrice(pricePoint.Attributes)),
			})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return byTerritory, nil
}
