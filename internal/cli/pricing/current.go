package pricing

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"maps"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

type appCurrentPricingResult struct {
	AppID         string                     `json:"appId"`
	BaseTerritory string                     `json:"baseTerritory"`
	Territory     string                     `json:"territory,omitempty"`
	CustomerPrice string                     `json:"customerPrice,omitempty"`
	Proceeds      string                     `json:"proceeds,omitempty"`
	Currency      string                     `json:"currency,omitempty"`
	IsFree        bool                       `json:"isFree"`
	Territories   []appCurrentTerritoryPrice `json:"territories,omitempty"`
}

type appCurrentTerritoryPrice struct {
	Territory     string `json:"territory"`
	CustomerPrice string `json:"customerPrice,omitempty"`
	Proceeds      string `json:"proceeds,omitempty"`
	Currency      string `json:"currency,omitempty"`
}

type appPricePointValue struct {
	CustomerPrice string
	Proceeds      string
}

type appPriceResourceMetadata struct {
	TerritoryID  string
	PricePointID string
	StartDate    string
	EndDate      string
}

// PricingCurrentCommand returns the current pricing command.
func PricingCurrentCommand() *ffcli.Command {
	fs := flag.NewFlagSet("pricing current", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID)")
	allTerritories := fs.Bool("all-territories", false, "Show current prices for all territories")
	territory := fs.String("territory", "", "Comma-separated territory filter(s); accepts alpha-2, alpha-3, or exact English country names (e.g., US,USA,France)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "current",
		ShortUsage: `asc pricing current --app "APP_ID" [--all-territories] [--territory "US,France"]`,
		ShortHelp:  "Show the current app price.",
		LongHelp: `Show the current app price.

Examples:
  asc pricing current --app "123456789"
  asc pricing current --app "123456789" --territory "US,France"
  asc pricing current --app "123456789" --all-territories --output table`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			resolvedAppID := shared.ResolveAppID(*appID)
			if resolvedAppID == "" {
				fmt.Fprintln(os.Stderr, "Error: --app is required (or set ASC_APP_ID)")
				return shared.MissingRequiredUsageError()
			}

			if *allTerritories && strings.TrimSpace(*territory) != "" {
				fmt.Fprintln(os.Stderr, "Error: --territory and --all-territories are mutually exclusive")
				return flag.ErrHelp
			}
			requestedTerritories, err := shared.NormalizeASCTerritoryCSV(*territory)
			if err != nil {
				return shared.UsageError(err.Error())
			}
			requestedTerritories = uniqueUpperList(requestedTerritories)
			if strings.TrimSpace(*territory) != "" && len(requestedTerritories) == 0 {
				fmt.Fprintln(os.Stderr, "Error: --territory must include at least one territory code")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("pricing current: %w", err)
			}

			scheduleResp, err := getAppPriceScheduleWithTimeout(ctx, client, resolvedAppID)
			if err != nil {
				return fmt.Errorf("pricing current: get app price schedule: %w", err)
			}

			scheduleID := strings.TrimSpace(scheduleResp.Data.ID)
			if scheduleID == "" {
				return fmt.Errorf("pricing current: app price schedule ID missing")
			}

			baseTerritoryResp, err := getAppPriceScheduleBaseTerritoryWithTimeout(ctx, client, scheduleID)
			if err != nil {
				return fmt.Errorf("pricing current: get base territory: %w", err)
			}

			baseTerritory := strings.ToUpper(strings.TrimSpace(baseTerritoryResp.Data.ID))
			if baseTerritory == "" {
				return fmt.Errorf("pricing current: base territory missing from response")
			}

			manualEntries, manualValues, manualCurrencies, err := fetchAppSchedulePriceEntries(ctx, func(callCtx context.Context, opts ...asc.AppPriceSchedulePricesOption) (*asc.AppPricesResponse, error) {
				return client.GetAppPriceScheduleManualPrices(callCtx, scheduleID, opts...)
			})
			if err != nil {
				return fmt.Errorf("pricing current: fetch manual prices: %w", err)
			}

			values := manualValues
			currencies := manualCurrencies
			entries := manualEntries

			needAutomatic := *allTerritories
			if !needAutomatic {
				for _, territoryID := range requestedTerritories {
					if territoryID != baseTerritory {
						needAutomatic = true
						break
					}
				}
			}

			if needAutomatic {
				automaticEntries, automaticValues, automaticCurrencies, err := fetchAppSchedulePriceEntries(ctx, func(callCtx context.Context, opts ...asc.AppPriceSchedulePricesOption) (*asc.AppPricesResponse, error) {
					return client.GetAppPriceScheduleAutomaticPrices(callCtx, scheduleID, opts...)
				})
				if err != nil {
					return fmt.Errorf("pricing current: fetch automatic prices: %w", err)
				}
				entries = append(entries, automaticEntries...)
				maps.Copy(values, automaticValues)
				maps.Copy(currencies, automaticCurrencies)
			}

			entries = dedupeAppPriceEntries(entries)
			now := time.Now().UTC()

			result, err := buildAppCurrentPricingResult(
				resolvedAppID,
				baseTerritory,
				entries,
				values,
				currencies,
				requestedTerritories,
				*allTerritories,
				now,
			)
			if err != nil {
				return fmt.Errorf("pricing current: %w", err)
			}

			return printAppCurrentPricingResult(result, *output.Output, *output.Pretty)
		},
	}
}

type appSchedulePricePageFetcher func(context.Context, ...asc.AppPriceSchedulePricesOption) (*asc.AppPricesResponse, error)

func fetchAppSchedulePriceEntries(
	ctx context.Context,
	fetch appSchedulePricePageFetcher,
) ([]appPriceEntry, map[string]appPricePointValue, map[string]string, error) {
	const limit = 200

	fetchPage := func(nextURL string) (*asc.AppPricesResponse, error) {
		callCtx, cancel := shared.ContextWithTimeout(ctx)
		defer cancel()

		if strings.TrimSpace(nextURL) != "" {
			mergedNextURL, err := shared.MergeNextURLQuery(nextURL, appSchedulePricesQuery(limit))
			if err != nil {
				return nil, err
			}
			return fetch(callCtx, asc.WithAppPriceSchedulePricesNextURL(mergedNextURL))
		}

		return fetch(
			callCtx,
			asc.WithAppPriceSchedulePricesInclude([]string{"appPricePoint", "territory"}),
			asc.WithAppPriceSchedulePricesFields([]string{"manual", "startDate", "endDate", "appPricePoint", "territory"}),
			asc.WithAppPriceSchedulePricesPricePointFields([]string{"customerPrice", "proceeds", "territory"}),
			asc.WithAppPriceSchedulePricesTerritoryFields([]string{"currency"}),
			asc.WithAppPriceSchedulePricesLimit(limit),
		)
	}

	firstPage, err := fetchPage("")
	if err != nil {
		return nil, nil, nil, err
	}

	entries := make([]appPriceEntry, 0, len(firstPage.Data))
	values := make(map[string]appPricePointValue)
	currencies := make(map[string]string)

	if err := asc.PaginateEach(
		ctx,
		firstPage,
		func(_ context.Context, nextURL string) (asc.PaginatedResponse, error) {
			return fetchPage(nextURL)
		},
		func(page asc.PaginatedResponse) error {
			typed, ok := page.(*asc.AppPricesResponse)
			if !ok {
				return fmt.Errorf("unexpected app price response type %T", page)
			}

			entries = append(entries, parseAppPriceEntries(typed.Data)...)

			pageValues, pageCurrencies, err := parseIncludedAppPriceData(typed.Included)
			if err != nil {
				return err
			}
			maps.Copy(values, pageValues)
			maps.Copy(currencies, pageCurrencies)
			return nil
		},
	); err != nil {
		return nil, nil, nil, err
	}

	return entries, values, currencies, nil
}

func parseAppPriceEntries(resources []asc.Resource[asc.AppPriceAttributes]) []appPriceEntry {
	entries := make([]appPriceEntry, 0, len(resources))
	for _, item := range resources {
		decodedMeta, decodedOK := decodeAppPriceResourceMetadata(item.ID)

		territoryID := strings.ToUpper(strings.TrimSpace(appPriceRelationshipID(item.Relationships, "territory")))
		if territoryID == "" {
			territoryID = decodedMeta.TerritoryID
		}

		pricePointID := strings.TrimSpace(appPriceRelationshipID(item.Relationships, "appPricePoint"))
		if pricePointID == "" {
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

		entries = append(entries, newAppPriceEntry(
			territoryID,
			pricePointID,
			startDate,
			endDate,
			item.Attributes.Manual,
		))
	}
	return entries
}

func parseIncludedAppPriceData(raw json.RawMessage) (map[string]appPricePointValue, map[string]string, error) {
	values := make(map[string]appPricePointValue)
	currencies := make(map[string]string)
	if len(raw) == 0 {
		return values, currencies, nil
	}

	var included []struct {
		Type       string          `json:"type"`
		ID         string          `json:"id"`
		Attributes json.RawMessage `json:"attributes"`
	}
	if err := json.Unmarshal(raw, &included); err != nil {
		return nil, nil, fmt.Errorf("parse included app pricing resources: %w", err)
	}

	for _, item := range included {
		switch item.Type {
		case string(asc.ResourceTypeAppPricePoints):
			var attrs struct {
				CustomerPrice string `json:"customerPrice"`
				Proceeds      string `json:"proceeds"`
			}
			if err := json.Unmarshal(item.Attributes, &attrs); err != nil {
				return nil, nil, fmt.Errorf("parse app price point attributes: %w", err)
			}

			value := appPricePointValue{
				CustomerPrice: strings.TrimSpace(attrs.CustomerPrice),
				Proceeds:      strings.TrimSpace(attrs.Proceeds),
			}
			values[item.ID] = value

			decodedTerritoryID, decodedPricePointID, ok := decodeAppPriceResourceID(item.ID)
			if ok && strings.TrimSpace(decodedTerritoryID) != "" && strings.TrimSpace(decodedPricePointID) != "" {
				values[appPricePointLookupKey(decodedTerritoryID, decodedPricePointID)] = value
			}

		case string(asc.ResourceTypeTerritories):
			var attrs struct {
				Currency string `json:"currency"`
			}
			if err := json.Unmarshal(item.Attributes, &attrs); err != nil {
				return nil, nil, fmt.Errorf("parse territory attributes: %w", err)
			}

			currency := strings.TrimSpace(attrs.Currency)
			if currency != "" {
				currencies[strings.ToUpper(strings.TrimSpace(item.ID))] = currency
			}
		}
	}

	return values, currencies, nil
}

func resolveCurrentTerritoryPrice(
	entries []appPriceEntry,
	values map[string]appPricePointValue,
	currencies map[string]string,
	territoryID string,
	now time.Time,
) (appCurrentTerritoryPrice, bool, error) {
	currentEntry, found := findActiveAppPriceEntry(entries, territoryID, now)
	if !found {
		return appCurrentTerritoryPrice{}, false, nil
	}

	value, ok := values[appPricePointLookupKey(territoryID, currentEntry.PricePointID)]
	if !ok {
		value, ok = values[currentEntry.PricePointID]
	}
	if !ok {
		return appCurrentTerritoryPrice{}, false, fmt.Errorf("current price point %q missing for territory %s", currentEntry.PricePointID, territoryID)
	}

	currency := currencies[strings.ToUpper(strings.TrimSpace(territoryID))]
	if currency == "" {
		currency = territoryID
	}

	return appCurrentTerritoryPrice{
		Territory:     strings.ToUpper(strings.TrimSpace(territoryID)),
		CustomerPrice: value.CustomerPrice,
		Proceeds:      value.Proceeds,
		Currency:      currency,
	}, true, nil
}

func buildAppCurrentPricingResult(
	appID string,
	baseTerritory string,
	entries []appPriceEntry,
	values map[string]appPricePointValue,
	currencies map[string]string,
	requestedTerritories []string,
	allTerritories bool,
	now time.Time,
) (*appCurrentPricingResult, error) {
	baseCurrent, foundBase, err := resolveCurrentTerritoryPrice(entries, values, currencies, baseTerritory, now)
	if err != nil {
		return nil, err
	}
	if !foundBase {
		return nil, fmt.Errorf("no current price found for base territory %s", baseTerritory)
	}

	targetTerritories := requestedTerritories
	if allTerritories {
		targetTerritories = territoriesFromActiveEntries(entries, now)
	}
	if len(targetTerritories) == 0 {
		targetTerritories = []string{baseTerritory}
	}

	currentPrices := make([]appCurrentTerritoryPrice, 0, len(targetTerritories))
	missingTerritories := make([]string, 0)
	for _, territoryID := range targetTerritories {
		price, found, err := resolveCurrentTerritoryPrice(entries, values, currencies, territoryID, now)
		if err != nil {
			return nil, err
		}
		if !found {
			missingTerritories = append(missingTerritories, territoryID)
			continue
		}
		currentPrices = append(currentPrices, price)
	}

	if len(missingTerritories) > 0 {
		return nil, fmt.Errorf("no current price found for territories: %s", strings.Join(missingTerritories, ", "))
	}

	sortCurrentTerritoryPrices(currentPrices, baseTerritory)

	result := &appCurrentPricingResult{
		AppID:         appID,
		BaseTerritory: baseTerritory,
		IsFree:        amountIsZero(baseCurrent.CustomerPrice),
	}

	if len(currentPrices) == 1 && !allTerritories {
		price := currentPrices[0]
		if len(requestedTerritories) > 0 || price.Territory != baseTerritory {
			result.Territory = price.Territory
		}
		result.CustomerPrice = price.CustomerPrice
		result.Proceeds = price.Proceeds
		result.Currency = price.Currency
	} else {
		result.Territories = currentPrices
	}

	return result, nil
}

func findActiveAppPriceEntry(entries []appPriceEntry, territoryID string, at time.Time) (appPriceEntry, bool) {
	territoryID = strings.ToUpper(strings.TrimSpace(territoryID))
	if territoryID == "" {
		return appPriceEntry{}, false
	}

	at = dateOnlyUTC(at)
	var best appPriceEntry
	found := false

	for _, entry := range entries {
		if entry.TerritoryID != territoryID {
			continue
		}
		if !appPriceEntryActiveOn(entry, at) {
			continue
		}
		if !found || appPriceEntryIsNewer(entry, best) {
			best = entry
			found = true
		}
	}

	return best, found
}

func decodeAppPriceResourceID(resourceID string) (string, string, bool) {
	decodedMeta, ok := decodeAppPriceResourceMetadata(resourceID)
	if decodedMeta.TerritoryID == "" || decodedMeta.PricePointID == "" {
		return "", "", false
	}
	return decodedMeta.TerritoryID, decodedMeta.PricePointID, ok
}

func decodeAppPriceResourceMetadata(resourceID string) (appPriceResourceMetadata, bool) {
	resourceID = strings.TrimSpace(resourceID)
	if resourceID == "" {
		return appPriceResourceMetadata{}, false
	}

	decoded, err := base64.RawURLEncoding.DecodeString(resourceID)
	if err != nil {
		decoded, err = base64.URLEncoding.DecodeString(resourceID)
		if err != nil {
			return appPriceResourceMetadata{}, false
		}
	}

	var payload struct {
		TerritoryID      string  `json:"t"`
		PricePointID     string  `json:"p"`
		StartDateSeconds float64 `json:"sd"`
		EndDateSeconds   float64 `json:"ed"`
	}
	if err := json.Unmarshal(decoded, &payload); err != nil {
		return appPriceResourceMetadata{}, false
	}

	return appPriceResourceMetadata{
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
	return time.Unix(int64(seconds), 0).UTC().Format(appPriceDateLayout)
}

func dedupeAppPriceEntries(entries []appPriceEntry) []appPriceEntry {
	if len(entries) < 2 {
		return entries
	}

	unique := make([]appPriceEntry, 0, len(entries))
	seen := make(map[string]struct{}, len(entries))
	for _, entry := range entries {
		key := strings.Join([]string{
			entry.TerritoryID,
			entry.PricePointID,
			entry.StartDate,
			entry.EndDate,
			strconv.FormatBool(entry.Manual),
		}, "|")
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		unique = append(unique, entry)
	}
	return unique
}

func uniqueUpperList(values []string) []string {
	if len(values) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(values))
	unique := make([]string, 0, len(values))
	for _, item := range values {
		trimmed := strings.ToUpper(strings.TrimSpace(item))
		if trimmed == "" {
			continue
		}
		if _, exists := seen[trimmed]; exists {
			continue
		}
		seen[trimmed] = struct{}{}
		unique = append(unique, trimmed)
	}
	return unique
}

func territoriesFromActiveEntries(entries []appPriceEntry, at time.Time) []string {
	territories := make([]string, 0, len(entries))
	seen := make(map[string]struct{}, len(entries))
	for _, entry := range entries {
		if entry.TerritoryID == "" {
			continue
		}
		if !appPriceEntryActiveOn(entry, dateOnlyUTC(at)) {
			continue
		}
		if _, exists := seen[entry.TerritoryID]; exists {
			continue
		}
		seen[entry.TerritoryID] = struct{}{}
		territories = append(territories, entry.TerritoryID)
	}
	return territories
}

func sortCurrentTerritoryPrices(prices []appCurrentTerritoryPrice, baseTerritory string) {
	sort.Slice(prices, func(i, j int) bool {
		left := prices[i].Territory
		right := prices[j].Territory
		switch {
		case left == baseTerritory && right != baseTerritory:
			return true
		case left != baseTerritory && right == baseTerritory:
			return false
		default:
			return left < right
		}
	})
}

func appPricePointLookupKey(territoryID, pricePointID string) string {
	territoryID = strings.ToUpper(strings.TrimSpace(territoryID))
	pricePointID = strings.TrimSpace(pricePointID)
	if territoryID == "" || pricePointID == "" {
		return ""
	}
	return territoryID + "|" + pricePointID
}

func amountIsZero(value string) bool {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return false
	}
	parsed, err := strconv.ParseFloat(trimmed, 64)
	if err != nil {
		return false
	}
	return parsed == 0
}

func getAppPriceScheduleWithTimeout(ctx context.Context, client *asc.Client, appID string) (*asc.AppPriceScheduleResponse, error) {
	callCtx, cancel := shared.ContextWithTimeout(ctx)
	defer cancel()
	return client.GetAppPriceSchedule(callCtx, appID)
}

func getAppPriceScheduleBaseTerritoryWithTimeout(ctx context.Context, client *asc.Client, scheduleID string) (*asc.TerritoryResponse, error) {
	callCtx, cancel := shared.ContextWithTimeout(ctx)
	defer cancel()
	return client.GetAppPriceScheduleBaseTerritory(callCtx, scheduleID)
}

func printAppCurrentPricingResult(result *appCurrentPricingResult, format string, pretty bool) error {
	return shared.PrintOutputWithRenderers(
		result,
		format,
		pretty,
		func() error { return printAppCurrentPricingTable(result) },
		func() error { return printAppCurrentPricingMarkdown(result) },
	)
}

func printAppCurrentPricingTable(result *appCurrentPricingResult) error {
	headers, rows := appCurrentPricingRows(result)
	asc.RenderTable(headers, rows)
	return nil
}

func printAppCurrentPricingMarkdown(result *appCurrentPricingResult) error {
	headers, rows := appCurrentPricingRows(result)
	asc.RenderMarkdown(headers, rows)
	return nil
}

func appCurrentPricingRows(result *appCurrentPricingResult) ([]string, [][]string) {
	headers := []string{"Territory", "Price", "Proceeds", "Currency"}
	rows := make([][]string, 0)

	if len(result.Territories) > 0 {
		for _, territory := range result.Territories {
			rows = append(rows, []string{
				territory.Territory,
				territory.CustomerPrice,
				territory.Proceeds,
				territory.Currency,
			})
		}
		return headers, rows
	}

	territoryID := result.Territory
	if territoryID == "" {
		territoryID = result.BaseTerritory
	}
	rows = append(rows, []string{
		territoryID,
		result.CustomerPrice,
		result.Proceeds,
		result.Currency,
	})
	return headers, rows
}
