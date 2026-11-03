package subscriptions

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

type subscriptionIntroductoryOfferImportCSVRow struct {
	row             int
	territory       string
	offerMode       string
	offerDuration   string
	numberOfPeriods int
	hasPeriods      bool
	startDate       string
	endDate         string
	pricePointID    string
	planType        asc.SubscriptionPlanType
}

type subscriptionIntroductoryOfferImportDefaults struct {
	offerMode       string
	offerDuration   string
	numberOfPeriods int
	hasPeriods      bool
	startDate       string
	endDate         string
	planType        asc.SubscriptionPlanType
}

type subscriptionIntroductoryOfferImportResolvedRow struct {
	row             int
	territory       string
	offerMode       string
	offerDuration   string
	numberOfPeriods int
	startDate       string
	endDate         string
	pricePointID    string
	planType        asc.SubscriptionPlanType
}

var subscriptionIntroductoryOffersImportKnownColumns = map[string]string{
	"territory":                     "territory",
	"offer_mode":                    "offer_mode",
	"offer_duration":                "offer_duration",
	"number_of_periods":             "number_of_periods",
	"start_date":                    "start_date",
	"end_date":                      "end_date",
	"price_point_id":                "price_point_id",
	"price_point":                   "price_point_id",
	"target_subscription_plan_type": "target_subscription_plan_type",
}

func readSubscriptionIntroductoryOffersImportCSV(path string) ([]subscriptionIntroductoryOfferImportCSVRow, error) {
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

	columnIdx, err := parseSubscriptionIntroductoryOffersImportCSVHeader(header)
	if err != nil {
		return nil, err
	}

	rows := make([]subscriptionIntroductoryOfferImportCSVRow, 0)
	dataRowNumber := 0
	for {
		record, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read csv: %w", err)
		}
		if record == nil || isSubscriptionIntroductoryOffersImportRecordEmpty(record) {
			continue
		}
		dataRowNumber++
		row, err := parseSubscriptionIntroductoryOffersImportCSVRow(record, columnIdx, dataRowNumber)
		if err != nil {
			return nil, err
		}
		rows = append(rows, row)
	}

	return rows, nil
}

func parseSubscriptionIntroductoryOffersImportCSVHeader(header []string) (map[string]int, error) {
	if len(header) == 0 {
		return nil, shared.UsageError("CSV header row is required")
	}

	columnIdx := make(map[string]int)
	for idx, raw := range header {
		normalized := normalizeSubscriptionIntroductoryOffersImportHeader(raw)
		if normalized == "" {
			continue
		}
		canonical, ok := subscriptionIntroductoryOffersImportKnownColumns[normalized]
		if !ok {
			return nil, shared.UsageErrorf("unknown CSV column %q", strings.TrimSpace(raw))
		}
		if _, exists := columnIdx[canonical]; exists {
			return nil, shared.UsageErrorf("duplicate CSV column %q", canonical)
		}
		columnIdx[canonical] = idx
	}
	if _, ok := columnIdx["territory"]; !ok {
		return nil, shared.UsageError(`CSV header must include required column "territory"`)
	}

	return columnIdx, nil
}

func parseSubscriptionIntroductoryOffersImportCSVRow(record []string, columnIdx map[string]int, rowNumber int) (subscriptionIntroductoryOfferImportCSVRow, error) {
	get := func(column string) string {
		idx, ok := columnIdx[column]
		if !ok || idx < 0 || idx >= len(record) {
			return ""
		}
		return strings.TrimSpace(record[idx])
	}

	startDate := get("start_date")
	if startDate != "" {
		normalized, err := shared.NormalizeDate(startDate, "--start-date")
		if err != nil {
			return subscriptionIntroductoryOfferImportCSVRow{}, shared.UsageErrorf("row %d: %v", rowNumber, err)
		}
		startDate = normalized
	}

	endDate := get("end_date")
	if endDate != "" {
		normalized, err := shared.NormalizeDate(endDate, "--end-date")
		if err != nil {
			return subscriptionIntroductoryOfferImportCSVRow{}, shared.UsageErrorf("row %d: %v", rowNumber, err)
		}
		endDate = normalized
	}

	offerMode := get("offer_mode")
	if offerMode != "" {
		normalized, err := normalizeSubscriptionOfferMode(offerMode)
		if err != nil {
			return subscriptionIntroductoryOfferImportCSVRow{}, shared.UsageErrorf("row %d: %v", rowNumber, err)
		}
		offerMode = string(normalized)
	}

	offerDuration := get("offer_duration")
	if offerDuration != "" {
		normalized, err := normalizeSubscriptionOfferDuration(offerDuration)
		if err != nil {
			return subscriptionIntroductoryOfferImportCSVRow{}, shared.UsageErrorf("row %d: %v", rowNumber, err)
		}
		offerDuration = string(normalized)
	}

	numberOfPeriodsRaw := get("number_of_periods")
	numberOfPeriods := 0
	hasPeriods := false
	if numberOfPeriodsRaw != "" {
		parsed, err := strconv.Atoi(numberOfPeriodsRaw)
		if err != nil {
			return subscriptionIntroductoryOfferImportCSVRow{}, shared.UsageErrorf("row %d: number_of_periods must be a positive integer", rowNumber)
		}
		if parsed <= 0 {
			return subscriptionIntroductoryOfferImportCSVRow{}, shared.UsageErrorf("row %d: number_of_periods must be a positive integer", rowNumber)
		}
		numberOfPeriods = parsed
		hasPeriods = true
	}

	var planType asc.SubscriptionPlanType
	if value := get("target_subscription_plan_type"); value != "" {
		normalized, err := shared.NormalizeSubscriptionPlanType(value, "target_subscription_plan_type")
		if err != nil {
			return subscriptionIntroductoryOfferImportCSVRow{}, shared.UsageErrorf("row %d: %v", rowNumber, err)
		}
		planType = normalized
	}

	territoryID, err := normalizeSubscriptionIntroductoryOfferImportTerritoryID(get("territory"))
	if err != nil {
		return subscriptionIntroductoryOfferImportCSVRow{}, shared.UsageErrorf("row %d: %v", rowNumber, err)
	}

	return subscriptionIntroductoryOfferImportCSVRow{
		row:             rowNumber,
		territory:       territoryID,
		offerMode:       offerMode,
		offerDuration:   offerDuration,
		numberOfPeriods: numberOfPeriods,
		hasPeriods:      hasPeriods,
		startDate:       startDate,
		endDate:         endDate,
		pricePointID:    get("price_point_id"),
		planType:        planType,
	}, nil
}

func normalizeSubscriptionIntroductoryOffersImportHeader(value string) string {
	return normalizeSubscriptionPricesImportHeader(value)
}

func isSubscriptionIntroductoryOffersImportRecordEmpty(record []string) bool {
	for _, value := range record {
		if strings.TrimSpace(value) != "" {
			return false
		}
	}
	return true
}

func buildSubscriptionIntroductoryOfferImportDefaults(offerDuration, offerMode string, numberOfPeriods int, startDate, endDate string, planType asc.SubscriptionPlanType) subscriptionIntroductoryOfferImportDefaults {
	defaults := subscriptionIntroductoryOfferImportDefaults{
		offerDuration: strings.TrimSpace(offerDuration),
		offerMode:     strings.TrimSpace(offerMode),
		startDate:     strings.TrimSpace(startDate),
		endDate:       strings.TrimSpace(endDate),
		planType:      planType,
	}
	if numberOfPeriods > 0 {
		defaults.numberOfPeriods = numberOfPeriods
		defaults.hasPeriods = true
	}
	return defaults
}

func resolveSubscriptionIntroductoryOfferImportRows(
	rows []subscriptionIntroductoryOfferImportCSVRow,
	defaults subscriptionIntroductoryOfferImportDefaults,
) ([]subscriptionIntroductoryOfferImportResolvedRow, error) {
	resolved := make([]subscriptionIntroductoryOfferImportResolvedRow, 0, len(rows))
	for _, row := range rows {
		offerMode := row.offerMode
		if offerMode == "" {
			offerMode = defaults.offerMode
		}
		if offerMode == "" {
			return nil, shared.UsageErrorf("row %d: offer_mode is required", row.row)
		}

		offerDuration := row.offerDuration
		if offerDuration == "" {
			offerDuration = defaults.offerDuration
		}
		if offerDuration == "" {
			return nil, shared.UsageErrorf("row %d: offer_duration is required", row.row)
		}

		numberOfPeriods := row.numberOfPeriods
		hasPeriods := row.hasPeriods
		if !hasPeriods && defaults.hasPeriods {
			numberOfPeriods = defaults.numberOfPeriods
			hasPeriods = true
		}
		if !hasPeriods {
			return nil, shared.UsageErrorf("row %d: number_of_periods is required", row.row)
		}

		startDate := row.startDate
		if startDate == "" {
			startDate = defaults.startDate
		}

		endDate := row.endDate
		if endDate == "" {
			endDate = defaults.endDate
		}

		planType := row.planType
		if planType == "" {
			planType = defaults.planType
		}
		if planType == "" {
			planType = asc.SubscriptionPlanTypeUpfront
		}

		resolved = append(resolved, subscriptionIntroductoryOfferImportResolvedRow{
			row:             row.row,
			territory:       row.territory,
			offerMode:       offerMode,
			offerDuration:   offerDuration,
			numberOfPeriods: numberOfPeriods,
			startDate:       startDate,
			endDate:         endDate,
			pricePointID:    row.pricePointID,
			planType:        planType,
		})
	}
	return resolved, nil
}
