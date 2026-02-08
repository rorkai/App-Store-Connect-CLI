package iap

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

const (
	iapPricesDateLayout      = "2006-01-02"
	defaultIAPPricesWorkers  = 4
	maxIncludedScheduleLimit = 50
)

type iapPricesResult struct {
	IAPs []iapPriceSummary `json:"iaps"`
}

type iapPriceSummary struct {
	ID                string               `json:"id"`
	Name              string               `json:"name"`
	ProductID         string               `json:"productId"`
	Type              string               `json:"type"`
	BaseTerritory     string               `json:"baseTerritory,omitempty"`
	CurrentPrice      *iapMoney            `json:"currentPrice,omitempty"`
	EstimatedProceeds *iapMoney            `json:"estimatedProceeds,omitempty"`
	ScheduledChanges  []iapScheduledChange `json:"scheduledChanges,omitempty"`
}

type iapMoney struct {
	Amount   string `json:"amount"`
	Currency string `json:"currency"`
}

type iapScheduledChange struct {
	Territory      string `json:"territory"`
	FromPricePoint string `json:"fromPricePoint,omitempty"`
	ToPricePoint   string `json:"toPricePoint"`
	EffectiveDate  string `json:"effectiveDate"`
}

type iapPriceEntry struct {
	TerritoryID  string
	PricePointID string
	StartDate    string
	EndDate      string
	Manual       bool
	StartAt      *time.Time
	EndAt        *time.Time
}

type iapPricePointValue struct {
	CustomerPrice string
	Proceeds      string
}

// IAPPricesCommand returns a consolidated pricing summary command for IAPs.
func IAPPricesCommand() *ffcli.Command {
	fs := flag.NewFlagSet("prices", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	iapID := fs.String("iap-id", "", "In-app purchase ID")
	territory := fs.String("territory", "", "Territory filter (e.g., USA)")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "prices",
		ShortUsage: "asc iap prices [flags]",
		ShortHelp:  "Show consolidated in-app purchase pricing summary.",
		LongHelp: `Show consolidated in-app purchase pricing summary.

Examples:
  asc iap prices --app "APP_ID"
  asc iap prices --iap-id "IAP_ID"
  asc iap prices --app "APP_ID" --territory "USA" --output table`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			requestedIAPID := strings.TrimSpace(*iapID)
			requestedAppID := strings.TrimSpace(*appID)
			if requestedIAPID == "" && shared.ResolveAppID(requestedAppID) == "" {
				fmt.Fprintln(os.Stderr, "Error: --app or --iap-id is required")
				return flag.ErrHelp
			}
			if requestedIAPID != "" && requestedAppID != "" {
				fmt.Fprintln(os.Stderr, "Error: --app and --iap-id are mutually exclusive")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("iap prices: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			var iaps []asc.Resource[asc.InAppPurchaseV2Attributes]
			if requestedIAPID != "" {
				resp, err := client.GetInAppPurchaseV2(requestCtx, requestedIAPID)
				if err != nil {
					return fmt.Errorf("iap prices: failed to fetch IAP: %w", err)
				}
				iaps = []asc.Resource[asc.InAppPurchaseV2Attributes]{resp.Data}
			} else {
				resolvedAppID := shared.ResolveAppID(requestedAppID)
				firstPage, err := client.GetInAppPurchasesV2(requestCtx, resolvedAppID, asc.WithIAPLimit(200))
				if err != nil {
					return fmt.Errorf("iap prices: failed to fetch IAP list: %w", err)
				}

				paginated, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetInAppPurchasesV2(ctx, resolvedAppID, asc.WithIAPNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("iap prices: paginate IAP list: %w", err)
				}

				resp, ok := paginated.(*asc.InAppPurchasesV2Response)
				if !ok {
					return fmt.Errorf("iap prices: unexpected pagination response type %T", paginated)
				}
				iaps = resp.Data
			}

			summaries, err := resolveIAPPriceSummaries(
				requestCtx,
				client,
				iaps,
				strings.ToUpper(strings.TrimSpace(*territory)),
				time.Now().UTC(),
			)
			if err != nil {
				return fmt.Errorf("iap prices: %w", err)
			}

			return printIAPPricesResult(&iapPricesResult{IAPs: summaries}, *output, *pretty)
		},
	}
}

func resolveIAPPriceSummaries(
	ctx context.Context,
	client *asc.Client,
	iaps []asc.Resource[asc.InAppPurchaseV2Attributes],
	territoryFilter string,
	now time.Time,
) ([]iapPriceSummary, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("context cancelled: %w", err)
	}

	if len(iaps) == 0 {
		return []iapPriceSummary{}, nil
	}

	workers := defaultIAPPricesWorkers
	if len(iaps) < workers {
		workers = len(iaps)
	}
	if workers < 1 {
		workers = 1
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	sem := make(chan struct{}, workers)
	results := make([]iapPriceSummary, len(iaps))
	errs := make(chan error, len(iaps))
	var once sync.Once
	var wg sync.WaitGroup

	for idx := range iaps {
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

			summary, err := resolveIAPPriceSummary(ctx, client, iaps[idx], territoryFilter, now)
			if err != nil {
				once.Do(cancel)
				errs <- fmt.Errorf("resolve %s: %w", iaps[idx].ID, err)
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

func resolveIAPPriceSummary(
	ctx context.Context,
	client *asc.Client,
	iap asc.Resource[asc.InAppPurchaseV2Attributes],
	territoryFilter string,
	now time.Time,
) (iapPriceSummary, error) {
	scheduleResp, err := client.GetInAppPurchasePriceSchedule(
		ctx,
		iap.ID,
		asc.WithIAPPriceScheduleInclude([]string{"baseTerritory", "manualPrices", "automaticPrices"}),
		asc.WithIAPPriceScheduleFields([]string{"baseTerritory", "manualPrices", "automaticPrices"}),
		asc.WithIAPPriceScheduleTerritoryFields([]string{"currency"}),
		asc.WithIAPPriceSchedulePriceFields([]string{"startDate", "endDate", "manual", "inAppPurchasePricePoint", "territory"}),
		asc.WithIAPPriceScheduleManualPricesLimit(maxIncludedScheduleLimit),
		asc.WithIAPPriceScheduleAutomaticPricesLimit(maxIncludedScheduleLimit),
	)
	if err != nil {
		return iapPriceSummary{}, fmt.Errorf("fetch price schedule: %w", err)
	}

	baseTerritoryID, _ := relationshipID(scheduleResp.Data.Relationships, "baseTerritory")
	baseTerritoryID = strings.ToUpper(strings.TrimSpace(baseTerritoryID))

	entries, territoryCurrencies, err := parseIAPPriceScheduleIncluded(scheduleResp.Included)
	if err != nil {
		return iapPriceSummary{}, err
	}
	fetchedAllScheduleEntries := false
	if scheduleEntriesRequireFullFetch(entries) {
		entries, err = fetchAllSchedulePriceEntries(ctx, client, scheduleResp.Data.ID)
		if err != nil {
			return iapPriceSummary{}, fmt.Errorf("fetch full price schedule entries: %w", err)
		}
		fetchedAllScheduleEntries = true
	}

	targetTerritory := territoryFilter
	if targetTerritory == "" {
		targetTerritory = baseTerritoryID
	}

	if territoryFilter != "" && !entriesContainTerritory(entries, territoryFilter) && !fetchedAllScheduleEntries {
		fallbackEntries, fallbackErr := fetchAllSchedulePriceEntries(ctx, client, scheduleResp.Data.ID)
		if fallbackErr != nil {
			return iapPriceSummary{}, fmt.Errorf("fetch full price schedule entries: %w", fallbackErr)
		}
		entries = fallbackEntries
	}

	currentEntry, hasCurrent := findActivePriceEntry(entries, targetTerritory, now)

	currentPrice := (*iapMoney)(nil)
	estimatedProceeds := (*iapMoney)(nil)
	if hasCurrent && targetTerritory != "" {
		pointValues, currency, err := fetchIAPPricePointValues(ctx, client, iap.ID, targetTerritory)
		if err != nil {
			return iapPriceSummary{}, err
		}
		if _, ok := pointValues[currentEntry.PricePointID]; !ok {
			fallbackValues, fallbackCurrency, fallbackErr := fetchManualSchedulePricePointValues(ctx, client, scheduleResp.Data.ID, targetTerritory)
			if fallbackErr == nil {
				for key, value := range fallbackValues {
					pointValues[key] = value
				}
				if currency == "" {
					currency = fallbackCurrency
				}
			}
		}
		if currency == "" {
			currency = territoryCurrencies[targetTerritory]
		}
		if currency == "" {
			currency = targetTerritory
		}
		if value, ok := pointValues[currentEntry.PricePointID]; ok {
			currentPrice = &iapMoney{
				Amount:   value.CustomerPrice,
				Currency: currency,
			}
			estimatedProceeds = &iapMoney{
				Amount:   value.Proceeds,
				Currency: currency,
			}
		}
	}

	return iapPriceSummary{
		ID:                iap.ID,
		Name:              iap.Attributes.Name,
		ProductID:         iap.Attributes.ProductID,
		Type:              iap.Attributes.InAppPurchaseType,
		BaseTerritory:     baseTerritoryID,
		CurrentPrice:      currentPrice,
		EstimatedProceeds: estimatedProceeds,
		ScheduledChanges:  buildScheduledChanges(entries, now, territoryFilter),
	}, nil
}

func fetchAllSchedulePriceEntries(ctx context.Context, client *asc.Client, scheduleID string) ([]iapPriceEntry, error) {
	manualEntries, err := fetchSchedulePriceEntries(ctx, func(ctx context.Context, opts ...asc.IAPPriceSchedulePricesOption) (*asc.InAppPurchasePricesResponse, error) {
		return client.GetInAppPurchasePriceScheduleManualPrices(ctx, scheduleID, opts...)
	})
	if err != nil {
		return nil, fmt.Errorf("fetch manual prices: %w", err)
	}

	automaticEntries, err := fetchSchedulePriceEntries(ctx, func(ctx context.Context, opts ...asc.IAPPriceSchedulePricesOption) (*asc.InAppPurchasePricesResponse, error) {
		return client.GetInAppPurchasePriceScheduleAutomaticPrices(ctx, scheduleID, opts...)
	})
	if err != nil {
		return nil, fmt.Errorf("fetch automatic prices: %w", err)
	}

	return dedupePriceEntries(append(manualEntries, automaticEntries...)), nil
}

type schedulePriceFetcher func(context.Context, ...asc.IAPPriceSchedulePricesOption) (*asc.InAppPurchasePricesResponse, error)

func fetchSchedulePriceEntries(ctx context.Context, fetch schedulePriceFetcher) ([]iapPriceEntry, error) {
	firstPage, err := fetch(ctx, asc.WithIAPPriceSchedulePricesLimit(200))
	if err != nil {
		return nil, err
	}

	paginated, err := asc.PaginateAll(ctx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
		return fetch(ctx, asc.WithIAPPriceSchedulePricesNextURL(nextURL))
	})
	if err != nil {
		return nil, err
	}

	resp, ok := paginated.(*asc.InAppPurchasePricesResponse)
	if !ok {
		return nil, fmt.Errorf("unexpected schedule prices response type %T", paginated)
	}

	entries := make([]iapPriceEntry, 0, len(resp.Data))
	for _, item := range resp.Data {
		decodedMeta, decodedOK := decodeIAPPriceResourceMetadata(item.ID)

		territoryID, err := relationshipID(item.Relationships, "territory")
		if err != nil || strings.TrimSpace(territoryID) == "" {
			territoryID = decodedMeta.TerritoryID
		}
		pricePointID, err := relationshipID(item.Relationships, "inAppPurchasePricePoint")
		if err != nil || strings.TrimSpace(pricePointID) == "" {
			pricePointID = decodedMeta.PricePointID
		} else if strings.TrimSpace(decodedMeta.PricePointID) != "" {
			pricePointID = decodedMeta.PricePointID
		}
		if strings.TrimSpace(territoryID) == "" || strings.TrimSpace(pricePointID) == "" {
			continue
		}

		startDate := strings.TrimSpace(item.Attributes.StartDate)
		endDate := strings.TrimSpace(item.Attributes.EndDate)
		if decodedOK {
			if startDate == "" {
				startDate = decodedMeta.StartDate
			}
			if endDate == "" {
				endDate = decodedMeta.EndDate
			}
		}
		entries = append(entries, newIAPPriceEntry(
			territoryID,
			pricePointID,
			startDate,
			endDate,
			item.Attributes.Manual,
		))
	}

	return entries, nil
}

func fetchIAPPricePointValues(
	ctx context.Context,
	client *asc.Client,
	iapID string,
	territoryID string,
) (map[string]iapPricePointValue, string, error) {
	firstPage, err := client.GetInAppPurchasePricePoints(
		ctx,
		iapID,
		asc.WithIAPPricePointsTerritory(territoryID),
		asc.WithIAPPricePointsFields([]string{"customerPrice", "proceeds", "territory"}),
		asc.WithIAPPricePointsInclude([]string{"territory"}),
		asc.WithIAPPricePointsTerritoryFields([]string{"currency"}),
		asc.WithIAPPricePointsLimit(8000),
	)
	if err != nil {
		return nil, "", fmt.Errorf("fetch price points: %w", err)
	}

	paginated, err := asc.PaginateAll(ctx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
		return client.GetInAppPurchasePricePoints(ctx, iapID, asc.WithIAPPricePointsNextURL(nextURL))
	})
	if err != nil {
		return nil, "", fmt.Errorf("paginate price points: %w", err)
	}

	resp, ok := paginated.(*asc.InAppPurchasePricePointsResponse)
	if !ok {
		return nil, "", fmt.Errorf("unexpected price points response type %T", paginated)
	}

	values := make(map[string]iapPricePointValue, len(resp.Data))
	for _, item := range resp.Data {
		value := iapPricePointValue{
			CustomerPrice: strings.TrimSpace(item.Attributes.CustomerPrice),
			Proceeds:      strings.TrimSpace(item.Attributes.Proceeds),
		}
		values[item.ID] = value
		_, decodedPricePointID, ok := decodeIAPPriceResourceID(item.ID)
		if ok && strings.TrimSpace(decodedPricePointID) != "" {
			values[decodedPricePointID] = value
		}
	}

	currency := territoryCurrencyFromIncluded(resp.Included, territoryID)
	return values, currency, nil
}

func fetchManualSchedulePricePointValues(
	ctx context.Context,
	client *asc.Client,
	scheduleID string,
	territoryID string,
) (map[string]iapPricePointValue, string, error) {
	page, err := client.GetInAppPurchasePriceScheduleManualPrices(
		ctx,
		scheduleID,
		asc.WithIAPPriceSchedulePricesInclude([]string{"inAppPurchasePricePoint", "territory"}),
		asc.WithIAPPriceSchedulePricesFields([]string{"manual", "inAppPurchasePricePoint", "territory"}),
		asc.WithIAPPriceSchedulePricesPricePointFields([]string{"customerPrice", "proceeds", "territory"}),
		asc.WithIAPPriceSchedulePricesTerritoryFields([]string{"currency"}),
		asc.WithIAPPriceSchedulePricesLimit(200),
	)
	if err != nil {
		return nil, "", fmt.Errorf("fetch manual schedule price point values: %w", err)
	}

	values := make(map[string]iapPricePointValue)
	currency := ""
	seenNext := make(map[string]struct{})

	for {
		pageValues, pageCurrency, err := parseManualSchedulePricePointValues(page.Included, territoryID)
		if err != nil {
			return nil, "", err
		}
		for key, value := range pageValues {
			values[key] = value
		}
		if currency == "" && pageCurrency != "" {
			currency = pageCurrency
		}

		if page.Links.Next == "" {
			break
		}
		if _, exists := seenNext[page.Links.Next]; exists {
			return nil, "", fmt.Errorf("paginate manual schedule price point values: %w", asc.ErrRepeatedPaginationURL)
		}
		seenNext[page.Links.Next] = struct{}{}

		page, err = client.GetInAppPurchasePriceScheduleManualPrices(
			ctx,
			scheduleID,
			asc.WithIAPPriceSchedulePricesNextURL(page.Links.Next),
		)
		if err != nil {
			return nil, "", fmt.Errorf("paginate manual schedule price point values: %w", err)
		}
	}

	return values, currency, nil
}

func parseManualSchedulePricePointValues(
	raw json.RawMessage,
	territoryID string,
) (map[string]iapPricePointValue, string, error) {
	if len(raw) == 0 {
		return map[string]iapPricePointValue{}, "", nil
	}

	var included []struct {
		Type          string          `json:"type"`
		ID            string          `json:"id"`
		Attributes    json.RawMessage `json:"attributes"`
		Relationships json.RawMessage `json:"relationships"`
	}
	if err := json.Unmarshal(raw, &included); err != nil {
		return nil, "", fmt.Errorf("parse manual schedule included resources: %w", err)
	}

	targetTerritory := strings.ToUpper(strings.TrimSpace(territoryID))
	values := make(map[string]iapPricePointValue)
	currencies := make(map[string]string)

	for _, item := range included {
		switch item.Type {
		case string(asc.ResourceTypeTerritories):
			var attrs struct {
				Currency string `json:"currency"`
			}
			if err := json.Unmarshal(item.Attributes, &attrs); err != nil {
				continue
			}
			currency := strings.TrimSpace(attrs.Currency)
			if currency == "" {
				continue
			}
			currencies[strings.ToUpper(strings.TrimSpace(item.ID))] = currency
		case string(asc.ResourceTypeInAppPurchasePricePoints):
			var attrs struct {
				CustomerPrice string `json:"customerPrice"`
				Proceeds      string `json:"proceeds"`
			}
			if err := json.Unmarshal(item.Attributes, &attrs); err != nil {
				continue
			}

			decodedTerritoryID, decodedPricePointID, decoded := decodeIAPPriceResourceID(item.ID)
			pointTerritory := decodedTerritoryID
			if pointTerritory == "" {
				relTerritoryID, relErr := relationshipID(item.Relationships, "territory")
				if relErr == nil {
					pointTerritory = relTerritoryID
				}
			}
			pointTerritory = strings.ToUpper(strings.TrimSpace(pointTerritory))
			if targetTerritory != "" && pointTerritory != targetTerritory {
				continue
			}

			value := iapPricePointValue{
				CustomerPrice: strings.TrimSpace(attrs.CustomerPrice),
				Proceeds:      strings.TrimSpace(attrs.Proceeds),
			}
			if value.CustomerPrice == "" && value.Proceeds == "" {
				continue
			}

			values[item.ID] = value
			if decoded && strings.TrimSpace(decodedPricePointID) != "" {
				values[decodedPricePointID] = value
			}
		}
	}

	return values, currencies[targetTerritory], nil
}

func parseIAPPriceScheduleIncluded(raw json.RawMessage) ([]iapPriceEntry, map[string]string, error) {
	if len(raw) == 0 {
		return nil, map[string]string{}, nil
	}

	var included []struct {
		Type          string          `json:"type"`
		ID            string          `json:"id"`
		Attributes    json.RawMessage `json:"attributes"`
		Relationships json.RawMessage `json:"relationships"`
	}
	if err := json.Unmarshal(raw, &included); err != nil {
		return nil, nil, fmt.Errorf("parse schedule included resources: %w", err)
	}

	entries := make([]iapPriceEntry, 0, len(included))
	currencies := make(map[string]string)
	for _, item := range included {
		switch item.Type {
		case string(asc.ResourceTypeTerritories):
			var attrs struct {
				Currency string `json:"currency"`
			}
			if err := json.Unmarshal(item.Attributes, &attrs); err != nil {
				continue
			}
			currency := strings.TrimSpace(attrs.Currency)
			if currency == "" {
				continue
			}
			currencies[strings.ToUpper(strings.TrimSpace(item.ID))] = currency
		case string(asc.ResourceTypeInAppPurchasePrices):
			var attrs asc.InAppPurchasePriceAttributes
			if err := json.Unmarshal(item.Attributes, &attrs); err != nil {
				return nil, nil, fmt.Errorf("parse in-app purchase price attributes: %w", err)
			}
			decodedMeta, decodedOK := decodeIAPPriceResourceMetadata(item.ID)
			territoryID, err := relationshipID(item.Relationships, "territory")
			if err != nil || strings.TrimSpace(territoryID) == "" {
				territoryID = decodedMeta.TerritoryID
			}
			pricePointID, err := relationshipID(item.Relationships, "inAppPurchasePricePoint")
			if err != nil || strings.TrimSpace(pricePointID) == "" {
				pricePointID = decodedMeta.PricePointID
			} else if strings.TrimSpace(decodedMeta.PricePointID) != "" {
				pricePointID = decodedMeta.PricePointID
			}
			if strings.TrimSpace(territoryID) == "" || strings.TrimSpace(pricePointID) == "" {
				continue
			}

			startDate := strings.TrimSpace(attrs.StartDate)
			endDate := strings.TrimSpace(attrs.EndDate)
			if decodedOK {
				if startDate == "" {
					startDate = decodedMeta.StartDate
				}
				if endDate == "" {
					endDate = decodedMeta.EndDate
				}
			}
			entries = append(entries, newIAPPriceEntry(
				territoryID,
				pricePointID,
				startDate,
				endDate,
				attrs.Manual,
			))
		}
	}

	return dedupePriceEntries(entries), currencies, nil
}

func territoryCurrencyFromIncluded(raw json.RawMessage, territoryID string) string {
	if len(raw) == 0 {
		return ""
	}

	var included []struct {
		Type       string `json:"type"`
		ID         string `json:"id"`
		Attributes struct {
			Currency string `json:"currency"`
		} `json:"attributes"`
	}
	if err := json.Unmarshal(raw, &included); err != nil {
		return ""
	}

	target := strings.ToUpper(strings.TrimSpace(territoryID))
	for _, item := range included {
		if item.Type != string(asc.ResourceTypeTerritories) {
			continue
		}
		if target != "" && strings.ToUpper(strings.TrimSpace(item.ID)) != target {
			continue
		}
		currency := strings.TrimSpace(item.Attributes.Currency)
		if currency != "" {
			return currency
		}
	}
	return ""
}

func entriesContainTerritory(entries []iapPriceEntry, territoryID string) bool {
	territoryID = strings.ToUpper(strings.TrimSpace(territoryID))
	if territoryID == "" {
		return false
	}
	for _, entry := range entries {
		if entry.TerritoryID == territoryID {
			return true
		}
	}
	return false
}

func scheduleEntriesRequireFullFetch(entries []iapPriceEntry) bool {
	if len(entries) == 0 {
		return true
	}

	manualCount := 0
	automaticCount := 0
	for _, entry := range entries {
		if entry.Manual {
			manualCount++
			continue
		}
		automaticCount++
	}

	return manualCount >= maxIncludedScheduleLimit || automaticCount >= maxIncludedScheduleLimit
}

func buildScheduledChanges(entries []iapPriceEntry, now time.Time, territoryFilter string) []iapScheduledChange {
	asOf := dateOnlyUTC(now)
	filter := strings.ToUpper(strings.TrimSpace(territoryFilter))
	futureEntries := make([]iapPriceEntry, 0, len(entries))
	for _, entry := range entries {
		if filter != "" && entry.TerritoryID != filter {
			continue
		}
		if entry.StartAt == nil || !entry.StartAt.After(asOf) {
			continue
		}
		futureEntries = append(futureEntries, entry)
	}

	sort.Slice(futureEntries, func(i, j int) bool {
		if futureEntries[i].StartDate != futureEntries[j].StartDate {
			return futureEntries[i].StartDate < futureEntries[j].StartDate
		}
		if futureEntries[i].TerritoryID != futureEntries[j].TerritoryID {
			return futureEntries[i].TerritoryID < futureEntries[j].TerritoryID
		}
		return futureEntries[i].PricePointID < futureEntries[j].PricePointID
	})

	changes := make([]iapScheduledChange, 0, len(futureEntries))
	seen := make(map[string]struct{}, len(futureEntries))
	for _, entry := range futureEntries {
		fromPricePoint := ""
		previousEntry, ok := findActivePriceEntry(entries, entry.TerritoryID, entry.StartAt.AddDate(0, 0, -1))
		if ok {
			fromPricePoint = previousEntry.PricePointID
		}
		if fromPricePoint == entry.PricePointID {
			continue
		}

		change := iapScheduledChange{
			Territory:      entry.TerritoryID,
			FromPricePoint: fromPricePoint,
			ToPricePoint:   entry.PricePointID,
			EffectiveDate:  entry.StartDate,
		}
		key := strings.Join([]string{
			change.Territory,
			change.FromPricePoint,
			change.ToPricePoint,
			change.EffectiveDate,
		}, "|")
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		changes = append(changes, change)
	}

	return changes
}

func findActivePriceEntry(entries []iapPriceEntry, territoryID string, at time.Time) (iapPriceEntry, bool) {
	territoryID = strings.ToUpper(strings.TrimSpace(territoryID))
	if territoryID == "" {
		return iapPriceEntry{}, false
	}

	at = dateOnlyUTC(at)
	var best iapPriceEntry
	found := false

	for _, entry := range entries {
		if entry.TerritoryID != territoryID {
			continue
		}
		if !entryActiveOn(entry, at) {
			continue
		}
		if !found || iapPriceEntryIsNewer(entry, best) {
			best = entry
			found = true
		}
	}

	return best, found
}

func entryActiveOn(entry iapPriceEntry, at time.Time) bool {
	if entry.StartAt != nil && entry.StartAt.After(at) {
		return false
	}
	if entry.EndAt != nil && entry.EndAt.Before(at) {
		return false
	}
	return true
}

func iapPriceEntryIsNewer(candidate, existing iapPriceEntry) bool {
	switch {
	case candidate.StartAt == nil && existing.StartAt != nil:
		return false
	case candidate.StartAt != nil && existing.StartAt == nil:
		return true
	case candidate.StartAt != nil && existing.StartAt != nil:
		if !candidate.StartAt.Equal(*existing.StartAt) {
			return candidate.StartAt.After(*existing.StartAt)
		}
	}
	if candidate.Manual != existing.Manual {
		return candidate.Manual && !existing.Manual
	}
	return candidate.PricePointID > existing.PricePointID
}

func newIAPPriceEntry(territoryID, pricePointID, startDate, endDate string, manual bool) iapPriceEntry {
	return iapPriceEntry{
		TerritoryID:  strings.ToUpper(strings.TrimSpace(territoryID)),
		PricePointID: strings.TrimSpace(pricePointID),
		StartDate:    strings.TrimSpace(startDate),
		EndDate:      strings.TrimSpace(endDate),
		Manual:       manual,
		StartAt:      parseScheduleDate(startDate),
		EndAt:        parseScheduleDate(endDate),
	}
}

func parseScheduleDate(value string) *time.Time {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	parsed, err := time.Parse(iapPricesDateLayout, value)
	if err != nil {
		return nil
	}
	normalized := parsed.UTC()
	return &normalized
}

func decodeIAPPriceResourceID(resourceID string) (string, string, bool) {
	decodedMeta, ok := decodeIAPPriceResourceMetadata(resourceID)
	territoryID := decodedMeta.TerritoryID
	pricePointID := decodedMeta.PricePointID
	if territoryID == "" || pricePointID == "" {
		return "", "", false
	}
	return territoryID, pricePointID, ok
}

type iapPriceResourceMetadata struct {
	TerritoryID  string
	PricePointID string
	StartDate    string
	EndDate      string
}

func decodeIAPPriceResourceMetadata(resourceID string) (iapPriceResourceMetadata, bool) {
	resourceID = strings.TrimSpace(resourceID)
	if resourceID == "" {
		return iapPriceResourceMetadata{}, false
	}

	decoded, err := base64.RawURLEncoding.DecodeString(resourceID)
	if err != nil {
		decoded, err = base64.URLEncoding.DecodeString(resourceID)
		if err != nil {
			return iapPriceResourceMetadata{}, false
		}
	}

	var payload struct {
		TerritoryID      string  `json:"t"`
		PricePointID     string  `json:"p"`
		StartDateSeconds float64 `json:"sd"`
		EndDateSeconds   float64 `json:"ed"`
	}
	if err := json.Unmarshal(decoded, &payload); err != nil {
		return iapPriceResourceMetadata{}, false
	}

	return iapPriceResourceMetadata{
		TerritoryID:  strings.ToUpper(strings.TrimSpace(payload.TerritoryID)),
		PricePointID: strings.TrimSpace(payload.PricePointID),
		StartDate:    scheduleDateFromUnixSeconds(payload.StartDateSeconds),
		EndDate:      scheduleDateFromUnixSeconds(payload.EndDateSeconds),
	}, true
}

func scheduleDateFromUnixSeconds(seconds float64) string {
	if seconds <= 0 {
		return ""
	}
	return time.Unix(int64(seconds), 0).UTC().Format(iapPricesDateLayout)
}

func relationshipID(relationships json.RawMessage, key string) (string, error) {
	if len(relationships) == 0 {
		return "", fmt.Errorf("missing relationships")
	}

	var references map[string]json.RawMessage
	if err := json.Unmarshal(relationships, &references); err != nil {
		return "", fmt.Errorf("parse relationships: %w", err)
	}
	rawReference, ok := references[key]
	if !ok {
		return "", fmt.Errorf("missing %s relationship", key)
	}

	var reference struct {
		Data asc.ResourceData `json:"data"`
	}
	if err := json.Unmarshal(rawReference, &reference); err != nil {
		return "", fmt.Errorf("parse %s relationship: %w", key, err)
	}

	id := strings.TrimSpace(reference.Data.ID)
	if id == "" {
		return "", fmt.Errorf("missing %s relationship id", key)
	}
	return id, nil
}

func dateOnlyUTC(value time.Time) time.Time {
	return time.Date(value.UTC().Year(), value.UTC().Month(), value.UTC().Day(), 0, 0, 0, 0, time.UTC)
}

func dedupePriceEntries(entries []iapPriceEntry) []iapPriceEntry {
	if len(entries) < 2 {
		return entries
	}

	unique := make([]iapPriceEntry, 0, len(entries))
	seen := make(map[string]struct{}, len(entries))
	for _, entry := range entries {
		key := strings.Join([]string{
			entry.TerritoryID,
			entry.PricePointID,
			entry.StartDate,
			entry.EndDate,
			fmt.Sprintf("%t", entry.Manual),
		}, "|")
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		unique = append(unique, entry)
	}

	return unique
}

func printIAPPricesResult(result *iapPricesResult, format string, pretty bool) error {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "json":
		return shared.PrintOutput(result, "json", pretty)
	case "table":
		if pretty {
			return fmt.Errorf("--pretty is only valid with JSON output")
		}
		return printIAPPricesTable(result)
	case "markdown", "md":
		if pretty {
			return fmt.Errorf("--pretty is only valid with JSON output")
		}
		return printIAPPricesMarkdown(result)
	default:
		return fmt.Errorf("unsupported format: %s", format)
	}
}

func printIAPPricesTable(result *iapPricesResult) error {
	headers := []string{"ID", "Name", "Product ID", "Type", "Base Territory", "Current Price", "Estimated Proceeds", "Scheduled Changes"}
	rows := make([][]string, 0, len(result.IAPs))
	for _, item := range result.IAPs {
		rows = append(rows, []string{
			item.ID,
			compactIAPText(item.Name),
			item.ProductID,
			item.Type,
			item.BaseTerritory,
			formatIAPMoney(item.CurrentPrice),
			formatIAPMoney(item.EstimatedProceeds),
			formatScheduledChanges(item.ScheduledChanges),
		})
	}
	asc.RenderTable(headers, rows)
	return nil
}

func printIAPPricesMarkdown(result *iapPricesResult) error {
	headers := []string{"ID", "Name", "Product ID", "Type", "Base Territory", "Current Price", "Estimated Proceeds", "Scheduled Changes"}
	rows := make([][]string, 0, len(result.IAPs))
	for _, item := range result.IAPs {
		rows = append(rows, []string{
			item.ID,
			compactIAPText(item.Name),
			item.ProductID,
			item.Type,
			item.BaseTerritory,
			formatIAPMoney(item.CurrentPrice),
			formatIAPMoney(item.EstimatedProceeds),
			formatScheduledChanges(item.ScheduledChanges),
		})
	}
	asc.RenderMarkdown(headers, rows)
	return nil
}

func formatIAPMoney(value *iapMoney) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(strings.TrimSpace(value.Amount) + " " + strings.TrimSpace(value.Currency))
}

func formatScheduledChanges(changes []iapScheduledChange) string {
	if len(changes) == 0 {
		return ""
	}
	formatted := make([]string, 0, len(changes))
	for _, change := range changes {
		formatted = append(
			formatted,
			fmt.Sprintf(
				"%s:%s->%s@%s",
				change.Territory,
				change.FromPricePoint,
				change.ToPricePoint,
				change.EffectiveDate,
			),
		)
	}
	return strings.Join(formatted, "; ")
}

func compactIAPText(value string) string {
	return strings.Join(strings.Fields(value), " ")
}
