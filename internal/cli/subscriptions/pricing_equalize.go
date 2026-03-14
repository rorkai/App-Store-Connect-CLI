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
	"sync/atomic"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

const defaultEqualizeWorkers = 8

var errEqualizePricePointFound = errors.New("equalize price point found")

// SubscriptionsPricingEqualizeCommand returns the equalize subcommand.
func SubscriptionsPricingEqualizeCommand() *ffcli.Command {
	fs := flag.NewFlagSet("equalize", flag.ExitOnError)

	subscriptionID := fs.String("subscription-id", "", "Subscription ID (required)")
	baseTerritory := fs.String("base-territory", "USA", "Territory to use as the pricing base")
	basePrice := fs.String("base-price", "", "Customer price in the base territory (required)")
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
  asc subscriptions pricing equalize --subscription-id "SUB_ID" --base-price "38.49" --base-territory "USA" --confirm
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
			territory := strings.ToUpper(strings.TrimSpace(*baseTerritory))
			if territory == "" {
				territory = "USA"
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

			if *dryRun {
				return printEqualizeResult(&equalizeResult{
					SubscriptionID: subID,
					BaseTerritory:  territory,
					BasePrice:      price,
					DryRun:         true,
					Territories:    allTerritories,
					Total:          len(allTerritories),
				}, *output.Output, *output.Pretty)
			}

			if err := validateEqualizeAvailability(ctx, client, subID, allTerritories); err != nil {
				return fmt.Errorf("equalize: %w", err)
			}

			// Step 3: Set prices for all territories concurrently
			fmt.Fprintf(os.Stderr, "Setting prices for %d territories (%d workers)...\n", len(allTerritories), numWorkers)
			var succeeded atomic.Int32
			var failed atomic.Int32
			failures := make([]equalizeFailure, 0)
			var mu sync.Mutex

			existingCtx, existingCancel := shared.ContextWithTimeout(ctx)
			existingPrices, err := client.GetSubscriptionPricesRelationships(existingCtx, subID)
			existingCancel()
			if err != nil {
				return fmt.Errorf("equalize: failed to check existing prices: %w", err)
			}

			remainingTerritories := allTerritories
			if len(existingPrices.Data) == 0 && len(allTerritories) > 0 {
				fmt.Fprintf(os.Stderr, "Subscription has no prices; setting initial price in %s first...\n", territory)

				baseTarget := allTerritories[0]
				initialCtx, initialCancel := shared.ContextWithTimeout(ctx)
				_, err := client.SetSubscriptionInitialPrice(initialCtx, subID, baseTarget.PricePointID, baseTarget.Territory, asc.SubscriptionPriceCreateAttributes{})
				initialCancel()
				if err != nil {
					failed.Add(1)
					failures = append(failures, equalizeFailure{
						Territory: baseTarget.Territory,
						Price:     baseTarget.Price,
						Error:     err.Error(),
					})
					result := &equalizeResult{
						SubscriptionID: subID,
						BaseTerritory:  territory,
						BasePrice:      price,
						DryRun:         false,
						Total:          len(allTerritories),
						Succeeded:      int(succeeded.Load()),
						Failed:         int(failed.Load()),
						Failures:       failures,
					}
					fmt.Fprintf(os.Stderr, "Done: %d succeeded, %d failed\n", result.Succeeded, result.Failed)
					if err := printEqualizeResult(result, *output.Output, *output.Pretty); err != nil {
						return err
					}
					return shared.NewReportedError(fmt.Errorf("equalize: failed to set initial price in %s", baseTarget.Territory))
				} else {
					succeeded.Add(1)
					remainingTerritories = allTerritories[1:]
				}
			}

			sem := make(chan struct{}, numWorkers)
			var wg sync.WaitGroup

			for _, eq := range remainingTerritories {
				wg.Add(1)
				go func(e equalization) {
					defer wg.Done()
					sem <- struct{}{}
					defer func() { <-sem }()

					setCtx, setCancel := shared.ContextWithTimeout(ctx)
					defer setCancel()

					_, err := client.CreateSubscriptionPrice(setCtx, subID, e.PricePointID, e.Territory, asc.SubscriptionPriceCreateAttributes{})
					if err != nil {
						failed.Add(1)
						mu.Lock()
						failures = append(failures, equalizeFailure{
							Territory: e.Territory,
							Price:     e.Price,
							Error:     err.Error(),
						})
						mu.Unlock()
						return
					}
					succeeded.Add(1)
				}(eq)
			}

			wg.Wait()

			result := &equalizeResult{
				SubscriptionID: subID,
				BaseTerritory:  territory,
				BasePrice:      price,
				DryRun:         false,
				Total:          len(allTerritories),
				Succeeded:      int(succeeded.Load()),
				Failed:         int(failed.Load()),
				Failures:       failures,
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

type equalizeResult struct {
	SubscriptionID string            `json:"subscriptionId"`
	BaseTerritory  string            `json:"baseTerritory"`
	BasePrice      string            `json:"basePrice"`
	DryRun         bool              `json:"dryRun"`
	Total          int               `json:"total"`
	Succeeded      int               `json:"succeeded,omitempty"`
	Failed         int               `json:"failed,omitempty"`
	Territories    []equalization    `json:"territories,omitempty"`
	Failures       []equalizeFailure `json:"failures,omitempty"`
}

func findPricePoint(ctx context.Context, client *asc.Client, subID, territory, targetPrice string) (string, error) {
	// List price points filtered by the base territory, paginating to find the matching price
	var pricePointID string
	priceFilter := shared.PriceFilter{Price: targetPrice}

	firstCtx, firstCancel := shared.ContextWithTimeout(ctx)
	firstPage, err := client.GetSubscriptionPricePoints(firstCtx, subID,
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
	err = asc.PaginateEach(ctx, firstPage,
		func(_ context.Context, nextURL string) (asc.PaginatedResponse, error) {
			pageCtx, pageCancel := shared.ContextWithTimeout(ctx)
			defer pageCancel()
			return client.GetSubscriptionPricePoints(pageCtx, subID,
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
	resp, err := client.GetSubscriptionPricePointEqualizations(firstCtx, pricePointID,
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
		return client.GetSubscriptionPricePointEqualizations(pageCtx, pricePointID,
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

func validateEqualizeAvailability(ctx context.Context, client *asc.Client, subID string, territories []equalization) error {
	if len(territories) == 0 {
		return nil
	}

	getCtx, getCancel := shared.ContextWithTimeout(ctx)
	availability, err := client.GetSubscriptionAvailabilityForSubscription(getCtx, subID)
	getCancel()
	if err != nil {
		if errors.Is(err, asc.ErrNotFound) {
			return fmt.Errorf("subscription availability is not configured; equalize only updates prices and will not change sale availability. Configure territories first with `asc subscriptions pricing availability set`")
		}
		return fmt.Errorf("failed to fetch availability: %w", err)
	}

	availabilityID := strings.TrimSpace(availability.Data.ID)
	if availabilityID == "" {
		return fmt.Errorf("availability readback returned empty id")
	}

	territoriesCtx, territoriesCancel := shared.ContextWithTimeout(ctx)
	territoriesResp, err := client.GetSubscriptionAvailabilityAvailableTerritories(territoriesCtx, availabilityID, asc.WithSubscriptionAvailabilityTerritoriesLimit(200))
	territoriesCancel()
	if err != nil {
		return fmt.Errorf("failed to fetch availability territories: %w", err)
	}

	available := make(map[string]struct{}, len(territoriesResp.Data))
	for _, territory := range territoriesResp.Data {
		id := strings.ToUpper(strings.TrimSpace(territory.ID))
		if id == "" {
			continue
		}
		available[id] = struct{}{}
	}

	missing := make([]string, 0)
	for _, territory := range territories {
		if _, ok := available[territory.Territory]; !ok {
			missing = append(missing, territory.Territory)
		}
	}
	if len(missing) == 0 {
		return nil
	}

	sort.Strings(missing)
	return fmt.Errorf("subscription availability is missing %d equalized territor%s (%s); equalize only updates prices and will not change sale availability. Configure territories first with `asc subscriptions pricing availability set`", len(missing), pluralizeEqualizeTerritories(len(missing)), summarizeEqualizeTerritories(missing, 8))
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
