package subscriptions

import (
	"context"
	"encoding/csv"
	"errors"
	"flag"
	"fmt"
	"io"
	"math/big"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/peterbourgon/ff/v3/ffcli"
	"golang.org/x/text/language"
	"golang.org/x/text/language/display"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

type subscriptionPriceImportSummary struct {
	SubscriptionID  string                                `json:"subscriptionId"`
	InputFile       string                                `json:"inputFile"`
	DryRun          bool                                  `json:"dryRun"`
	ContinueOnError bool                                  `json:"continueOnError"`
	DefaultStart    string                                `json:"defaultStartDate,omitempty"`
	DefaultPreserve bool                                  `json:"defaultPreserved"`
	Total           int                                   `json:"total"`
	Created         int                                   `json:"created"`
	Failed          int                                   `json:"failed"`
	Failures        []subscriptionPriceImportSummaryError `json:"failures,omitempty"`
}

type subscriptionPriceImportSummaryError struct {
	Row       int    `json:"row"`
	Territory string `json:"territory,omitempty"`
	Price     string `json:"price,omitempty"`
	Error     string `json:"error"`
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
}

type subscriptionPricePointLookupCache struct {
	mu          sync.Mutex
	byTerritory map[string]map[string][]string
}

type territoryNameMapResult struct {
	id        string
	ambiguous bool
}

var (
	subscriptionPriceImportTerritoryNamesOnce sync.Once
	subscriptionPriceImportTerritoryNames     map[string]territoryNameMapResult
)

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

	subID := fs.String("id", "", "Subscription ID")
	inputPath := fs.String("input", "", "Input CSV file path (required)")
	startDate := fs.String("start-date", "", "Default start date (YYYY-MM-DD) for rows without start_date")
	preserved := fs.Bool("preserved", false, "Set preserveCurrentPrice=true for rows without preserved columns")
	dryRun := fs.Bool("dry-run", false, "Validate and resolve price points without creating subscription prices")
	continueOnError := fs.Bool("continue-on-error", true, "Continue processing rows after failures (default true)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "import",
		ShortUsage: "asc subscriptions prices import --id \"SUB_ID\" --input \"./prices.csv\" [flags]",
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
  asc subscriptions prices import --id "SUB_ID" --input "./prices.csv" --dry-run
  asc subscriptions prices import --id "SUB_ID" --input "./prices.csv" --start-date "2026-03-01"
  asc subscriptions prices import --id "SUB_ID" --input "./prices.csv" --preserved`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*subID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			inputValue := strings.TrimSpace(*inputPath)
			if inputValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --input is required")
				return flag.ErrHelp
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

			lookupCache := &subscriptionPricePointLookupCache{
				byTerritory: make(map[string]map[string][]string),
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
					pricePointID, rowErr = lookupCache.lookupPricePointID(ctx, client, id, resolvedRow.territoryID, resolvedRow.priceKey, resolvedRow.price)
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
					continue
				}

				attrs := asc.SubscriptionPriceCreateAttributes{
					StartDate: resolvedRow.startDate,
				}
				if resolvedRow.preserveSet {
					attrs.Preserved = &resolvedRow.preserveCurrentPrice
				}

				createCtx, createCancel := shared.ContextWithTimeout(ctx)
				_, rowErr = client.CreateSubscriptionPrice(createCtx, id, pricePointID, resolvedRow.territoryID, attrs)
				createCancel()
				if rowErr != nil {
					appendSubscriptionPriceImportFailure(summary, resolvedRow, rowErr)
					if !*continueOnError {
						break
					}
					continue
				}

				summary.Created++
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
				return shared.NewReportedError(fmt.Errorf("subscriptions prices import: %d row(s) failed", summary.Failed))
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
		[]string{"Subscription ID", "Input File", "Dry Run", "Total", "Created", "Failed"},
		[][]string{{
			summary.SubscriptionID,
			summary.InputFile,
			fmt.Sprintf("%t", summary.DryRun),
			fmt.Sprintf("%d", summary.Total),
			fmt.Sprintf("%d", summary.Created),
			fmt.Sprintf("%d", summary.Failed),
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
}

func readSubscriptionPricesImportCSV(path string) ([]subscriptionPriceImportCSVRow, error) {
	file, err := shared.OpenExistingNoFollow(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()

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
	seenNext := make(map[string]struct{})
	nextURL := ""

	for {
		pageCtx, pageCancel := shared.ContextWithTimeout(ctx)
		var (
			resp *asc.SubscriptionPricePointsResponse
			err  error
		)
		if nextURL == "" {
			resp, err = client.GetSubscriptionPricePoints(
				pageCtx,
				subscriptionID,
				asc.WithSubscriptionPricePointsTerritory(territoryID),
				asc.WithSubscriptionPricePointsLimit(200),
			)
		} else {
			resp, err = client.GetSubscriptionPricePoints(
				pageCtx,
				subscriptionID,
				asc.WithSubscriptionPricePointsNextURL(nextURL),
			)
		}
		pageCancel()
		if err != nil {
			return nil, fmt.Errorf("resolve price points for territory %q: %w", territoryID, err)
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

		if strings.TrimSpace(resp.Links.Next) == "" {
			break
		}
		if _, seen := seenNext[resp.Links.Next]; seen {
			return nil, fmt.Errorf("resolve price points for territory %q: repeated next URL", territoryID)
		}
		seenNext[resp.Links.Next] = struct{}{}
		nextURL = resp.Links.Next
	}

	return priceByAmount, nil
}

func resolveSubscriptionPriceImportTerritoryID(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", fmt.Errorf("territory is required")
	}

	upper := strings.ToUpper(trimmed)
	if isThreeLetterCode(upper) {
		return upper, nil
	}

	// Accept alpha-2 inputs when users pass "US"/"GB" from spreadsheets.
	if len(upper) == 2 {
		if region, err := language.ParseRegion(upper); err == nil {
			if iso3 := strings.ToUpper(strings.TrimSpace(region.ISO3())); isThreeLetterCode(iso3) {
				return iso3, nil
			}
		}
	}

	key := normalizeSubscriptionPriceImportTerritoryName(trimmed)
	entry, ok := subscriptionPriceImportTerritoryNameMap()[key]
	if !ok {
		return "", fmt.Errorf("territory %q could not be mapped to an App Store Connect territory ID", trimmed)
	}
	if entry.ambiguous || entry.id == "" {
		return "", fmt.Errorf("territory %q is ambiguous; use a 3-letter territory ID like USA", trimmed)
	}
	return entry.id, nil
}

func subscriptionPriceImportTerritoryNameMap() map[string]territoryNameMapResult {
	subscriptionPriceImportTerritoryNamesOnce.Do(func() {
		m := make(map[string]territoryNameMapResult)
		regionNamer := display.English.Regions()

		for a := 'A'; a <= 'Z'; a++ {
			for b := 'A'; b <= 'Z'; b++ {
				for c := 'A'; c <= 'Z'; c++ {
					code := string([]rune{a, b, c})
					region, err := language.ParseRegion(code)
					if err != nil {
						continue
					}
					iso3 := strings.ToUpper(strings.TrimSpace(region.ISO3()))
					if iso3 != code {
						continue
					}
					name := strings.TrimSpace(regionNamer.Name(region))
					if name == "" || strings.EqualFold(name, code) {
						continue
					}
					key := normalizeSubscriptionPriceImportTerritoryName(name)
					if key == "" {
						continue
					}
					existing, exists := m[key]
					switch {
					case !exists:
						m[key] = territoryNameMapResult{id: iso3}
					case existing.id != iso3:
						m[key] = territoryNameMapResult{ambiguous: true}
					}
				}
			}
		}

		for alias, id := range subscriptionPriceImportTerritoryAliases() {
			key := normalizeSubscriptionPriceImportTerritoryName(alias)
			if key == "" {
				continue
			}
			m[key] = territoryNameMapResult{id: strings.ToUpper(strings.TrimSpace(id))}
		}

		subscriptionPriceImportTerritoryNames = m
	})

	return subscriptionPriceImportTerritoryNames
}

func subscriptionPriceImportTerritoryAliases() map[string]string {
	return map[string]string{
		"uae":                                   "ARE",
		"uk":                                    "GBR",
		"united states of america":              "USA",
		"republic of korea":                     "KOR",
		"korea republic of":                     "KOR",
		"korea south":                           "KOR",
		"democratic people's republic of korea": "PRK",
		"korea north":                           "PRK",
		"taiwan province of china":              "TWN",
		"russian federation":                    "RUS",
		"bolivia plurinational state of":        "BOL",
		"venezuela bolivarian republic of":      "VEN",
		"iran islamic republic of":              "IRN",
		"moldova republic of":                   "MDA",
		"tanzania united republic of":           "TZA",
		"lao people's democratic republic":      "LAO",
		"viet nam":                              "VNM",
		"syrian arab republic":                  "SYR",
		"palestine state of":                    "PSE",
		"brunei darussalam":                     "BRN",
		"czechia":                               "CZE",
		"eswatini":                              "SWZ",
		"cape verde":                            "CPV",
		"curacao":                               "CUW",
		"kosovo":                                "XKS",
		"hong kong sar china":                   "HKG",
		"macao sar china":                       "MAC",
		"myanmar":                               "MMR",
		"turkiye":                               "TUR",
		"cote d ivoire":                         "CIV",
		"cote divoire":                          "CIV",
		"congo democratic republic of the":      "COD",
		"democratic republic of the congo":      "COD",
		"congo republic of the":                 "COG",
		"republic of the congo":                 "COG",
		"micronesia federated states of":        "FSM",
		"macedonia the former yugoslav republic of": "MKD",
		"north macedonia": "MKD",
	}
}

func normalizeSubscriptionPriceImportTerritoryName(value string) string {
	trimmed := strings.TrimSpace(strings.ToLower(value))
	if trimmed == "" {
		return ""
	}

	var builder strings.Builder
	lastSpace := false
	for _, r := range trimmed {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			builder.WriteRune(r)
			lastSpace = false
			continue
		}
		if !lastSpace {
			builder.WriteRune(' ')
			lastSpace = true
		}
	}
	return strings.Join(strings.Fields(builder.String()), " ")
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

func isThreeLetterCode(value string) bool {
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
