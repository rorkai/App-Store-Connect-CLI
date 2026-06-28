package subscriptions

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"math/big"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/ascterritory"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

type subscriptionPriceImportSummary struct {
	SubscriptionID       string                                `json:"subscriptionId"`
	InputFile            string                                `json:"inputFile"`
	DryRun               bool                                  `json:"dryRun"`
	ContinueOnError      bool                                  `json:"continueOnError"`
	DefaultStart         string                                `json:"defaultStartDate,omitempty"`
	DefaultPreserve      bool                                  `json:"defaultPreserved"`
	Total                int                                   `json:"total"`
	Created              int                                   `json:"created"`
	Skipped              int                                   `json:"skipped,omitempty"`
	Reconciled           int                                   `json:"reconciled,omitempty"`
	Failed               int                                   `json:"failed"`
	Failures             []subscriptionPriceImportSummaryError `json:"failures,omitempty"`
	FailureArtifact      string                                `json:"failureArtifactPath,omitempty"`
	FailureArtifactError string                                `json:"failureArtifactError,omitempty"`
	Results              []subscriptionPriceImportResultItem   `json:"results,omitempty"`
}

type subscriptionPriceImportSummaryError struct {
	Row       int    `json:"row"`
	Territory string `json:"territory,omitempty"`
	Price     string `json:"price,omitempty"`
	Error     string `json:"error"`
}

type subscriptionPriceImportResultItem struct {
	Row                  int    `json:"row"`
	Territory            string `json:"territory,omitempty"`
	Price                string `json:"price,omitempty"`
	PricePointID         string `json:"pricePointId,omitempty"`
	StartDate            string `json:"startDate,omitempty"`
	PreserveCurrentPrice bool   `json:"preserveCurrentPrice"`
	PreserveCurrentSet   bool   `json:"preserveCurrentPriceSet"`
	PlanType             string `json:"planType"`
	Status               string `json:"status"`
	Error                string `json:"error,omitempty"`
}

type subscriptionPriceImportFailureArtifact struct {
	SchemaVersion  int                                 `json:"schemaVersion"`
	Command        string                              `json:"command"`
	SubscriptionID string                              `json:"subscriptionId"`
	InputFile      string                              `json:"inputFile"`
	Failed         int                                 `json:"failed"`
	GeneratedAt    string                              `json:"generatedAt"`
	Results        []subscriptionPriceImportResultItem `json:"results"`
}

type subscriptionPriceImportCSVRow struct {
	row                  int
	territory            string
	currencyCode         string
	price                string
	startDate            string
	preserveSet          bool
	preserveCurrentPrice bool
	pricePointID         string
}

type subscriptionPriceImportResolvedRow struct {
	row                  int
	territoryID          string
	price                string
	priceKey             string
	startDate            string
	preserveSet          bool
	preserveCurrentPrice bool
	pricePointID         string
	planType             asc.SubscriptionPlanType
}

type subscriptionPricePointLookupCache struct {
	mu          sync.Mutex
	byTerritory map[string]map[string][]string
}

type subscriptionPriceImportState struct {
	territoryID          string
	pricePointID         string
	startDate            string
	preserveCurrentPrice bool
	planType             asc.SubscriptionPlanType
}

type subscriptionPriceImportStateIndex struct {
	now    time.Time
	states []subscriptionPriceImportState
}

var subscriptionPricesImportKnownColumns = map[string]string{
	"territory":              "territory",
	"countries_or_regions":   "territory",
	"country_or_region":      "territory",
	"currency_code":          "currency_code",
	"price":                  "price",
	"start_date":             "start_date",
	"preserved":              "preserved",
	"preserve_current_price": "preserve_current_price",
	"price_point_id":         "price_point_id",
}

// SubscriptionsPricesImportCommand returns the subscriptions prices import subcommand.
func SubscriptionsPricesImportCommand() *ffcli.Command {
	fs := flag.NewFlagSet("prices import", flag.ExitOnError)

	subID := fs.String("subscription-id", "", "Subscription ID, product ID, or exact current name")
	appID := addSubscriptionLookupAppFlag(fs)
	inputPath := fs.String("input", "", "Input CSV file path (required)")
	startDate := fs.String("start-date", "", "Default start date (YYYY-MM-DD) for rows without start_date")
	preserved := fs.Bool("preserved", false, "Set preserveCurrentPrice=true for rows without preserved columns")
	dryRun := fs.Bool("dry-run", false, "Validate and resolve price points without creating subscription prices")
	continueOnError := fs.Bool("continue-on-error", true, "Continue processing rows after failures (default true)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "import",
		ShortUsage: "asc subscriptions prices import --subscription-id \"SUB_ID\" --input \"./prices.csv\" [flags]",
		ShortHelp:  "Import subscription prices from a CSV file.",
		LongHelp: `Import subscription prices from a CSV file.

CSV is UTF-8 with a required header row.

Required columns:
  territory, price

Optional columns:
  currency_code, start_date, preserved, preserve_current_price, price_point_id

Header aliases:
  Countries or Regions -> territory
  countries_or_regions -> territory
  Currency Code -> currency_code

Examples:
  asc subscriptions prices import --subscription-id "SUB_ID" --input "./prices.csv" --dry-run
  asc subscriptions prices import --subscription-id "SUB_ID" --input "./prices.csv" --start-date "2026-03-01"
  asc subscriptions prices import --subscription-id "SUB_ID" --input "./prices.csv" --preserved`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*subID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --subscription-id is required")
				return shared.MissingRequiredUsageError()
			}

			inputValue := strings.TrimSpace(*inputPath)
			if inputValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --input is required")
				return shared.MissingRequiredUsageError()
			}

			defaultStartDate := ""
			if strings.TrimSpace(*startDate) != "" {
				normalized, err := shared.NormalizeDate(*startDate, "--start-date")
				if err != nil {
					return shared.UsageError(err.Error())
				}
				defaultStartDate = normalized
			}

			rows, err := readSubscriptionPricesImportCSV(inputValue)
			if err != nil {
				return fmt.Errorf("subscriptions prices import: %w", err)
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("subscriptions prices import: %w", err)
			}

			summary := &subscriptionPriceImportSummary{
				SubscriptionID:  id,
				InputFile:       filepath.Clean(inputValue),
				DryRun:          *dryRun,
				ContinueOnError: *continueOnError,
				DefaultStart:    defaultStartDate,
				DefaultPreserve: *preserved,
				Total:           len(rows),
			}

			summary.SubscriptionID, err = shared.RetryReadWithFreshTimeout(ctx, func(requestCtx context.Context) (string, error) {
				return resolveSubscriptionLookupID(requestCtx, client, *appID, summary.SubscriptionID)
			})
			if err != nil {
				return err
			}

			lookupCache := &subscriptionPricePointLookupCache{
				byTerritory: make(map[string]map[string][]string),
			}
			var stateIndex *subscriptionPriceImportStateIndex
			if !*dryRun {
				stateIndex, err = fetchSubscriptionPriceImportState(ctx, client, summary.SubscriptionID)
				if err != nil {
					return fmt.Errorf("subscriptions prices import: fetch existing prices: %w", err)
				}
			}

			for _, csvRow := range rows {
				resolvedRow, rowErr := resolveSubscriptionPriceImportRow(csvRow, defaultStartDate, *preserved)
				if rowErr != nil {
					appendSubscriptionPriceImportFailure(summary, resolvedRow, rowErr)
					if !*continueOnError {
						break
					}
					continue
				}

				pricePointID := resolvedRow.pricePointID
				if pricePointID == "" {
					pricePointID, rowErr = lookupCache.lookupPricePointID(ctx, client, summary.SubscriptionID, resolvedRow.territoryID, resolvedRow.priceKey, resolvedRow.price)
					if rowErr != nil {
						appendSubscriptionPriceImportFailure(summary, resolvedRow, rowErr)
						if !*continueOnError {
							break
						}
						continue
					}
				}

				if *dryRun {
					summary.Created++
					summary.Results = append(summary.Results, subscriptionPriceImportResultItem{
						Row:                  resolvedRow.row,
						Territory:            resolvedRow.territoryID,
						Price:                resolvedRow.price,
						PricePointID:         pricePointID,
						StartDate:            resolvedRow.startDate,
						PreserveCurrentPrice: resolvedRow.preserveCurrentPrice,
						PreserveCurrentSet:   resolvedRow.preserveSet,
						PlanType:             string(resolvedRow.planType),
						Status:               "planned",
					})
					continue
				}

				attrs := asc.SubscriptionPriceCreateAttributes{
					StartDate: resolvedRow.startDate,
					PlanType:  resolvedRow.planType,
				}
				if resolvedRow.preserveSet {
					attrs.Preserved = &resolvedRow.preserveCurrentPrice
				}

				resolvedRow.pricePointID = pricePointID
				status := reconciledMutationSkipped
				if !stateIndex.matches(resolvedRow) {
					status, rowErr = runReconciledMutation(
						ctx,
						func(readbackCtx context.Context) (bool, error) {
							refreshed, err := fetchSubscriptionPriceImportState(readbackCtx, client, summary.SubscriptionID)
							if err != nil {
								return false, err
							}
							stateIndex = refreshed
							return stateIndex.matches(resolvedRow), nil
						},
						func(mutationCtx context.Context) error {
							_, err := client.CreateSubscriptionPrice(mutationCtx, summary.SubscriptionID, pricePointID, resolvedRow.territoryID, attrs)
							return err
						},
					)
				}
				if rowErr != nil {
					appendSubscriptionPriceImportFailure(summary, resolvedRow, rowErr)
					if !*continueOnError {
						break
					}
					continue
				}

				result := subscriptionPriceImportResultItem{
					Row:                  resolvedRow.row,
					Territory:            resolvedRow.territoryID,
					Price:                resolvedRow.price,
					PricePointID:         pricePointID,
					StartDate:            resolvedRow.startDate,
					PreserveCurrentPrice: resolvedRow.preserveCurrentPrice,
					PreserveCurrentSet:   resolvedRow.preserveSet,
					PlanType:             string(resolvedRow.planType),
					Status:               string(status),
				}
				summary.Results = append(summary.Results, result)
				if status != reconciledMutationSkipped {
					stateIndex.add(resolvedRow)
				}
				switch status {
				case reconciledMutationCreated:
					summary.Created++
				case reconciledMutationSkipped:
					summary.Skipped++
				case reconciledMutationReconciled:
					summary.Reconciled++
				}
			}

			if summary.Failed > 0 {
				artifactPath, artifactErr := writeSubscriptionPriceImportFailureArtifact(summary)
				if artifactErr != nil {
					summary.FailureArtifactError = artifactErr.Error()
				} else {
					summary.FailureArtifact = artifactPath
				}
			}

			if err := shared.PrintOutputWithRenderers(
				summary,
				*output.Output,
				*output.Pretty,
				func() error { return renderSubscriptionPriceImportSummaryTables(summary, false) },
				func() error { return renderSubscriptionPriceImportSummaryTables(summary, true) },
			); err != nil {
				return err
			}

			if summary.Failed > 0 {
				rowErr := fmt.Errorf("subscriptions prices import: %d row(s) failed", summary.Failed)
				if summary.FailureArtifactError != "" {
					rowErr = errors.Join(rowErr, fmt.Errorf("write failure artifact: %s", summary.FailureArtifactError))
				}
				return shared.NewReportedError(rowErr)
			}
			return nil
		},
	}
}

func renderSubscriptionPriceImportSummaryTables(summary *subscriptionPriceImportSummary, markdown bool) error {
	if summary == nil {
		return fmt.Errorf("summary is nil")
	}

	render := asc.RenderTable
	if markdown {
		render = asc.RenderMarkdown
	}

	render(
		[]string{"Subscription ID", "Input File", "Dry Run", "Total", "Created", "Skipped", "Reconciled", "Failed", "Failure Artifact", "Failure Artifact Error"},
		[][]string{{
			summary.SubscriptionID,
			summary.InputFile,
			fmt.Sprintf("%t", summary.DryRun),
			fmt.Sprintf("%d", summary.Total),
			fmt.Sprintf("%d", summary.Created),
			fmt.Sprintf("%d", summary.Skipped),
			fmt.Sprintf("%d", summary.Reconciled),
			fmt.Sprintf("%d", summary.Failed),
			summary.FailureArtifact,
			summary.FailureArtifactError,
		}},
	)

	if len(summary.Failures) > 0 {
		rows := make([][]string, 0, len(summary.Failures))
		for _, failure := range summary.Failures {
			rows = append(rows, []string{
				fmt.Sprintf("%d", failure.Row),
				failure.Territory,
				failure.Price,
				failure.Error,
			})
		}
		render([]string{"Row", "Territory", "Price", "Error"}, rows)
	}

	return nil
}

func appendSubscriptionPriceImportFailure(summary *subscriptionPriceImportSummary, row subscriptionPriceImportResolvedRow, err error) {
	if summary == nil || err == nil {
		return
	}
	summary.Failed++
	summary.Failures = append(summary.Failures, subscriptionPriceImportSummaryError{
		Row:       row.row,
		Territory: row.territoryID,
		Price:     row.price,
		Error:     err.Error(),
	})
	summary.Results = append(summary.Results, subscriptionPriceImportResultItem{
		Row:                  row.row,
		Territory:            row.territoryID,
		Price:                row.price,
		PricePointID:         row.pricePointID,
		StartDate:            row.startDate,
		PreserveCurrentPrice: row.preserveCurrentPrice,
		PreserveCurrentSet:   row.preserveSet,
		PlanType:             string(row.planType),
		Status:               "failed",
		Error:                err.Error(),
	})
}

func fetchSubscriptionPriceImportState(ctx context.Context, client *asc.Client, subscriptionID string) (*subscriptionPriceImportStateIndex, error) {
	query := url.Values{
		"fields[subscriptionPrices]": []string{"startDate,preserved,planType,territory,subscriptionPricePoint"},
		"filter[planType]":           []string{string(asc.SubscriptionPlanTypeUpfront)},
		"include":                    []string{"territory,subscriptionPricePoint"},
		"limit":                      []string{"200"},
	}
	fetchPage := func(nextURL string) (*asc.SubscriptionPricesResponse, error) {
		if nextURL != "" {
			mergedNext, err := mergeSubscriptionPricesNextQuery(nextURL, query)
			if err != nil {
				return nil, err
			}
			return shared.RetryReadWithFreshTimeout(ctx, func(requestCtx context.Context) (*asc.SubscriptionPricesResponse, error) {
				return client.GetSubscriptionPrices(requestCtx, subscriptionID, asc.WithSubscriptionPricesNextURL(mergedNext))
			})
		}
		return shared.RetryReadWithFreshTimeout(ctx, func(requestCtx context.Context) (*asc.SubscriptionPricesResponse, error) {
			return client.GetSubscriptionPrices(
				requestCtx,
				subscriptionID,
				asc.WithSubscriptionPricesLimit(200),
				asc.WithSubscriptionPricesPlanType(asc.SubscriptionPlanTypeUpfront),
				asc.WithSubscriptionPricesFields([]string{"startDate", "preserved", "planType", "territory", "subscriptionPricePoint"}),
				asc.WithSubscriptionPricesInclude([]string{"territory", "subscriptionPricePoint"}),
			)
		})
	}

	firstPage, err := fetchPage("")
	if err != nil {
		return nil, err
	}
	index := &subscriptionPriceImportStateIndex{}
	err = asc.PaginateEach(ctx, firstPage, func(_ context.Context, nextURL string) (asc.PaginatedResponse, error) {
		return fetchPage(nextURL)
	}, func(page asc.PaginatedResponse) error {
		prices, ok := page.(*asc.SubscriptionPricesResponse)
		if !ok {
			return fmt.Errorf("unexpected subscription prices response type %T", page)
		}
		for _, price := range prices.Data {
			index.states = append(index.states, subscriptionPriceImportState{
				territoryID:          strings.ToUpper(strings.TrimSpace(extractSubscriptionPriceRelationshipID(price, "territory"))),
				pricePointID:         strings.TrimSpace(extractSubscriptionPriceRelationshipID(price, "subscriptionPricePoint")),
				startDate:            strings.TrimSpace(price.Attributes.StartDate),
				preserveCurrentPrice: price.Attributes.Preserved,
				planType:             price.Attributes.PlanType,
			})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	index.now = subscriptionImportNow()
	return index, nil
}

func (index *subscriptionPriceImportStateIndex) matches(target subscriptionPriceImportResolvedRow) bool {
	if index == nil {
		return false
	}
	targetTerritory := strings.ToUpper(strings.TrimSpace(target.territoryID))
	targetStart := strings.TrimSpace(target.startDate)
	if targetStart != "" {
		for _, state := range index.states {
			if state.territoryID == targetTerritory &&
				state.planType == target.planType &&
				state.pricePointID == target.pricePointID &&
				state.startDate == targetStart {
				return true
			}
		}
		return false
	}

	asOf := dateOnlyUTC(index.now)
	var selected *subscriptionPriceImportState
	selectedStart := time.Time{}
	for _, state := range index.states {
		if state.territoryID != targetTerritory ||
			state.planType != target.planType {
			continue
		}
		start := time.Time{}
		if state.startDate != "" {
			parsed, err := time.Parse(equalizeDateLayout, state.startDate)
			if err != nil || parsed.After(asOf) {
				continue
			}
			start = parsed
		}
		if selected == nil || start.After(selectedStart) ||
			(start.Equal(selectedStart) && selected.preserveCurrentPrice && !state.preserveCurrentPrice) {
			candidate := state
			selected = &candidate
			selectedStart = start
		}
	}
	return selected != nil && selected.pricePointID == target.pricePointID
}

func (index *subscriptionPriceImportStateIndex) add(target subscriptionPriceImportResolvedRow) {
	if index == nil {
		return
	}
	startDate := strings.TrimSpace(target.startDate)
	if startDate == "" {
		startDate = dateOnlyUTC(index.now).Format(equalizeDateLayout)
	}
	index.states = append(index.states, subscriptionPriceImportState{
		territoryID:          strings.ToUpper(strings.TrimSpace(target.territoryID)),
		pricePointID:         strings.TrimSpace(target.pricePointID),
		startDate:            startDate,
		preserveCurrentPrice: target.preserveCurrentPrice,
		planType:             target.planType,
	})
}

func writeSubscriptionPriceImportFailureArtifact(summary *subscriptionPriceImportSummary) (string, error) {
	failures := make([]subscriptionPriceImportResultItem, 0, summary.Failed)
	for _, result := range summary.Results {
		if result.Status == "failed" {
			failures = append(failures, result)
		}
	}
	artifact := subscriptionPriceImportFailureArtifact{
		SchemaVersion:  1,
		Command:        "subscriptions pricing prices import",
		SubscriptionID: summary.SubscriptionID,
		InputFile:      summary.InputFile,
		Failed:         summary.Failed,
		GeneratedAt:    time.Now().UTC().Format(time.RFC3339),
		Results:        failures,
	}
	data, err := json.MarshalIndent(artifact, "", "  ")
	if err != nil {
		return "", err
	}
	path := filepath.Join(".asc", "reports", "subscription-prices-import", fmt.Sprintf("failures-%d.json", time.Now().UTC().UnixNano()))
	if _, err := shared.WriteStreamToFile(path, bytes.NewReader(data)); err != nil {
		return "", err
	}
	return path, nil
}

func readSubscriptionPricesImportCSV(path string) ([]subscriptionPriceImportCSVRow, error) {
	file, err := shared.OpenExistingNoFollow(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.FieldsPerRecord = -1
	reader.LazyQuotes = true

	header, err := reader.Read()
	if err != nil {
		if errors.Is(err, io.EOF) {
			return nil, shared.UsageError("CSV file is empty")
		}
		return nil, fmt.Errorf("read header: %w", err)
	}

	columnIdx, err := parseSubscriptionPricesImportCSVHeader(header)
	if err != nil {
		return nil, err
	}

	rows := make([]subscriptionPriceImportCSVRow, 0)
	dataRowNumber := 0
	for {
		record, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read csv: %w", err)
		}
		if record == nil || isSubscriptionPricesImportRecordEmpty(record) {
			continue
		}
		dataRowNumber++

		row, rowErr := parseSubscriptionPricesImportCSVRow(record, columnIdx, dataRowNumber)
		if rowErr != nil {
			return nil, rowErr
		}
		rows = append(rows, row)
	}

	return rows, nil
}

func parseSubscriptionPricesImportCSVHeader(header []string) (map[string]int, error) {
	if len(header) == 0 {
		return nil, shared.UsageError("CSV header row is required")
	}

	knownIdx := make(map[string]int)
	for idx, raw := range header {
		normalized := normalizeSubscriptionPricesImportHeader(raw)
		canonical, ok := subscriptionPricesImportKnownColumns[normalized]
		if !ok {
			continue
		}
		if _, exists := knownIdx[canonical]; exists {
			return nil, shared.UsageErrorf("duplicate CSV column %q", canonical)
		}
		knownIdx[canonical] = idx
	}

	if _, ok := knownIdx["territory"]; !ok {
		return nil, shared.UsageError(`CSV header must include required column "territory"`)
	}
	if _, ok := knownIdx["price"]; !ok {
		return nil, shared.UsageError(`CSV header must include required column "price"`)
	}
	return knownIdx, nil
}

func parseSubscriptionPricesImportCSVRow(record []string, columnIdx map[string]int, rowNumber int) (subscriptionPriceImportCSVRow, error) {
	get := func(col string) string {
		idx, ok := columnIdx[col]
		if !ok || idx < 0 || idx >= len(record) {
			return ""
		}
		return strings.TrimSpace(record[idx])
	}

	startDate := strings.TrimSpace(get("start_date"))
	if startDate != "" {
		normalized, err := normalizeSubscriptionPriceImportDate(startDate)
		if err != nil {
			return subscriptionPriceImportCSVRow{}, shared.UsageErrorf("row %d: %v", rowNumber, err)
		}
		startDate = normalized
	}

	currencyCode := strings.ToUpper(strings.TrimSpace(get("currency_code")))
	if currencyCode != "" && !isISO4217Code(currencyCode) {
		return subscriptionPriceImportCSVRow{}, shared.UsageErrorf("row %d: currency_code must be a 3-letter ISO 4217 code", rowNumber)
	}

	preserved, preservedSet, err := parseSubscriptionPriceImportBool(get("preserved"))
	if err != nil {
		return subscriptionPriceImportCSVRow{}, shared.UsageErrorf("row %d: preserved must be true or false", rowNumber)
	}
	preserveCurrentPrice, preserveCurrentPriceSet, err := parseSubscriptionPriceImportBool(get("preserve_current_price"))
	if err != nil {
		return subscriptionPriceImportCSVRow{}, shared.UsageErrorf("row %d: preserve_current_price must be true or false", rowNumber)
	}
	switch {
	case preservedSet && preserveCurrentPriceSet && preserved != preserveCurrentPrice:
		return subscriptionPriceImportCSVRow{}, shared.UsageErrorf("row %d: preserved and preserve_current_price must match when both are provided", rowNumber)
	case preserveCurrentPriceSet:
		preserved = preserveCurrentPrice
		preservedSet = true
	}

	return subscriptionPriceImportCSVRow{
		row:                  rowNumber,
		territory:            get("territory"),
		currencyCode:         currencyCode,
		price:                get("price"),
		startDate:            startDate,
		preserveSet:          preservedSet,
		preserveCurrentPrice: preserved,
		pricePointID:         strings.TrimSpace(get("price_point_id")),
	}, nil
}

func resolveSubscriptionPriceImportRow(
	row subscriptionPriceImportCSVRow,
	defaultStartDate string,
	defaultPreserved bool,
) (subscriptionPriceImportResolvedRow, error) {
	resolved := subscriptionPriceImportResolvedRow{
		row:                  row.row,
		price:                strings.TrimSpace(row.price),
		startDate:            row.startDate,
		preserveSet:          row.preserveSet,
		preserveCurrentPrice: row.preserveCurrentPrice,
		pricePointID:         strings.TrimSpace(row.pricePointID),
		planType:             asc.SubscriptionPlanTypeUpfront,
	}

	if resolved.startDate == "" {
		resolved.startDate = defaultStartDate
	}
	if !resolved.preserveSet && defaultPreserved {
		resolved.preserveSet = true
		resolved.preserveCurrentPrice = true
	}

	territoryID, err := resolveSubscriptionPriceImportTerritoryID(row.territory)
	if err != nil {
		return resolved, err
	}
	resolved.territoryID = territoryID

	priceKey, err := normalizeSubscriptionPriceImportPrice(resolved.price)
	if err != nil {
		return resolved, err
	}
	resolved.priceKey = priceKey

	return resolved, nil
}

func (c *subscriptionPricePointLookupCache) lookupPricePointID(
	ctx context.Context,
	client *asc.Client,
	subscriptionID string,
	territoryID string,
	priceKey string,
	rawPrice string,
) (string, error) {
	c.mu.Lock()
	territoryPrices, ok := c.byTerritory[territoryID]
	c.mu.Unlock()

	if !ok {
		fetched, err := fetchSubscriptionPricePointsByTerritory(ctx, client, subscriptionID, territoryID)
		if err != nil {
			return "", err
		}
		c.mu.Lock()
		c.byTerritory[territoryID] = fetched
		territoryPrices = fetched
		c.mu.Unlock()
	}

	ids := territoryPrices[priceKey]
	switch len(ids) {
	case 0:
		return "", fmt.Errorf("row price %q was not found in subscription price points for territory %q", rawPrice, territoryID)
	case 1:
		return ids[0], nil
	default:
		return "", fmt.Errorf("row price %q matched multiple subscription price points in territory %q", rawPrice, territoryID)
	}
}

func fetchSubscriptionPricePointsByTerritory(
	ctx context.Context,
	client *asc.Client,
	subscriptionID string,
	territoryID string,
) (map[string][]string, error) {
	priceByAmount := make(map[string][]string)
	query := url.Values{
		"fields[subscriptionPricePoints]": []string{"customerPrice"},
		"filter[territory]":               []string{strings.ToUpper(strings.TrimSpace(territoryID))},
		"limit":                           []string{"200"},
	}
	fetchPage := func(nextURL string) (*asc.SubscriptionPricePointsResponse, error) {
		if nextURL == "" {
			return shared.RetryReadWithFreshTimeout(ctx, func(requestCtx context.Context) (*asc.SubscriptionPricePointsResponse, error) {
				return client.GetSubscriptionPricePoints(
					requestCtx,
					subscriptionID,
					asc.WithSubscriptionPricePointsTerritory(territoryID),
					asc.WithSubscriptionPricePointsFields([]string{"customerPrice"}),
					asc.WithSubscriptionPricePointsLimit(200),
				)
			})
		}
		mergedNext, err := mergeSubscriptionPricesNextQuery(nextURL, query)
		if err != nil {
			return nil, err
		}
		return shared.RetryReadWithFreshTimeout(ctx, func(requestCtx context.Context) (*asc.SubscriptionPricePointsResponse, error) {
			return client.GetSubscriptionPricePoints(
				requestCtx,
				subscriptionID,
				asc.WithSubscriptionPricePointsNextURL(mergedNext),
			)
		})
	}

	firstPage, err := fetchPage("")
	if err != nil {
		return nil, fmt.Errorf("resolve price points for territory %q: %w", territoryID, err)
	}

	if err := asc.PaginateEach(
		ctx,
		firstPage,
		func(_ context.Context, nextURL string) (asc.PaginatedResponse, error) {
			return fetchPage(nextURL)
		},
		func(page asc.PaginatedResponse) error {
			resp, ok := page.(*asc.SubscriptionPricePointsResponse)
			if !ok {
				return fmt.Errorf("unexpected response type %T", page)
			}
			for _, pricePoint := range resp.Data {
				priceKey, priceErr := normalizeSubscriptionPriceImportPrice(pricePoint.Attributes.CustomerPrice)
				if priceErr != nil {
					continue
				}
				id := strings.TrimSpace(pricePoint.ID)
				if id == "" {
					continue
				}
				priceByAmount[priceKey] = appendUniqueString(priceByAmount[priceKey], id)
			}
			return nil
		},
	); err != nil {
		return nil, fmt.Errorf("resolve price points for territory %q: %w", territoryID, err)
	}

	return priceByAmount, nil
}

func resolveSubscriptionPriceImportTerritoryID(raw string) (string, error) {
	return ascterritory.Normalize(raw)
}

func normalizeSubscriptionPriceImportPrice(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", fmt.Errorf("price is required")
	}
	rat := new(big.Rat)
	if _, ok := rat.SetString(trimmed); !ok {
		return "", fmt.Errorf("price %q is not a valid numeric value", trimmed)
	}
	return rat.RatString(), nil
}

func normalizeSubscriptionPriceImportDate(value string) (string, error) {
	parsed, err := time.Parse("2006-01-02", strings.TrimSpace(value))
	if err != nil {
		return "", fmt.Errorf("start_date must be in YYYY-MM-DD format")
	}
	return parsed.Format("2006-01-02"), nil
}

func parseSubscriptionPriceImportBool(value string) (bool, bool, error) {
	trimmed := strings.TrimSpace(strings.ToLower(value))
	switch trimmed {
	case "":
		return false, false, nil
	case "true":
		return true, true, nil
	case "false":
		return false, true, nil
	default:
		return false, false, fmt.Errorf("must be true or false")
	}
}

func normalizeSubscriptionPricesImportHeader(raw string) string {
	trimmed := strings.TrimSpace(strings.ToLower(raw))
	if trimmed == "" {
		return ""
	}

	var builder strings.Builder
	lastUnderscore := false
	for _, r := range trimmed {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			builder.WriteRune(r)
			lastUnderscore = false
		default:
			if !lastUnderscore {
				builder.WriteRune('_')
				lastUnderscore = true
			}
		}
	}

	return strings.Trim(builder.String(), "_")
}

func isSubscriptionPricesImportRecordEmpty(record []string) bool {
	for _, item := range record {
		if strings.TrimSpace(item) != "" {
			return false
		}
	}
	return true
}

func isISO4217Code(value string) bool {
	if len(value) != 3 {
		return false
	}
	for _, r := range value {
		if r < 'A' || r > 'Z' {
			return false
		}
	}
	return true
}

func appendUniqueString(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	values = append(values, value)
	sort.Strings(values)
	return values
}
