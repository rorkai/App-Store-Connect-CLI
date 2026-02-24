package insights

import (
	"compress/gzip"
	"context"
	"encoding/csv"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

const (
	sourceAnalytics = "analytics"
	sourceSales     = "sales"
)

// InsightsCommand returns the insights command group.
func InsightsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("insights", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "insights",
		ShortUsage: "asc insights <subcommand> [flags]",
		ShortHelp:  "Generate weekly and daily insights from App Store data sources.",
		LongHelp: `Generate weekly and daily insights from App Store data sources.

Examples:
  asc insights weekly --app "123456789" --source analytics --week "2026-02-16"
  asc insights weekly --app "123456789" --source sales --week "2026-02-16" --vendor "12345678"
  asc insights daily --app "123456789" --vendor "12345678" --date "2026-02-20"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			insightsWeeklyCommand(),
			insightsDailyCommand(),
		},
		Exec: func(_ context.Context, _ []string) error {
			return flag.ErrHelp
		},
	}
}

func insightsWeeklyCommand() *ffcli.Command {
	fs := flag.NewFlagSet("insights weekly", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (required, or ASC_APP_ID env)")
	source := fs.String("source", "", "Insights source: analytics or sales")
	week := fs.String("week", "", "Week start date (YYYY-MM-DD)")
	vendor := fs.String("vendor", "", "Vendor number for sales source (or ASC_VENDOR_NUMBER)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "weekly",
		ShortUsage: "asc insights weekly --app \"APP_ID\" --source analytics|sales --week \"YYYY-MM-DD\" [flags]",
		ShortHelp:  "Summarize this week vs last week metrics.",
		LongHelp: `Summarize this week vs last week metrics.

The output is deterministic JSON by default and can be rendered as table/markdown.

For --source sales, totals are scoped to the selected app and include linked in-app purchases
and subscriptions by matching Parent Identifier against the app SKU.

Examples:
  asc insights weekly --app "123456789" --source analytics --week "2026-02-16"
  asc insights weekly --app "123456789" --source sales --week "2026-02-16" --vendor "12345678"
  asc insights weekly --app "123456789" --source sales --week "2026-02-16" --output table`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return shared.UsageErrorf("unexpected argument(s): %s", strings.Join(args, " "))
			}

			resolvedAppID := shared.ResolveAppID(*appID)
			if resolvedAppID == "" {
				return shared.UsageError("--app is required (or set ASC_APP_ID)")
			}

			sourceName := strings.ToLower(strings.TrimSpace(*source))
			if sourceName == "" {
				return shared.UsageError("--source is required")
			}
			if !isAllowedSource(sourceName) {
				return shared.UsageErrorf("--source must be one of: %s, %s", sourceAnalytics, sourceSales)
			}

			weekStart, err := normalizeWeekStart(*week)
			if err != nil {
				return shared.UsageError(err.Error())
			}

			resolvedVendor := ""
			if sourceName == sourceSales {
				resolvedVendor = shared.ResolveVendorNumber(*vendor)
				if resolvedVendor == "" {
					return shared.UsageError("--vendor is required for --source sales (or set ASC_VENDOR_NUMBER)")
				}
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("insights weekly: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := collectWeeklyInsights(requestCtx, client, resolvedAppID, sourceName, resolvedVendor, weekStart)
			if err != nil {
				return fmt.Errorf("insights weekly: %w", err)
			}

			return shared.PrintOutputWithRenderers(
				resp,
				*output.Output,
				*output.Pretty,
				func() error { renderWeeklyInsights(resp, false); return nil },
				func() error { renderWeeklyInsights(resp, true); return nil },
			)
		},
	}
}

func insightsDailyCommand() *ffcli.Command {
	fs := flag.NewFlagSet("insights daily", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (required, or ASC_APP_ID env)")
	vendor := fs.String("vendor", "", "Vendor number for sales source (or ASC_VENDOR_NUMBER)")
	date := fs.String("date", "", "Report date (YYYY-MM-DD)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "daily",
		ShortUsage: "asc insights daily --app \"APP_ID\" --vendor \"VENDOR\" --date \"YYYY-MM-DD\" [flags]",
		ShortHelp:  "Summarize daily subscription renewal signals from sales exports.",
		LongHelp: `Summarize daily subscription renewal signals from sales exports.

This command is sales-only and app-scoped. It compares the selected day to the previous day.
Renewal metrics are derived from rows where Subscription equals "Renewal" for products linked to the app.

Examples:
  asc insights daily --app "123456789" --vendor "12345678" --date "2026-02-20"
  asc insights daily --app "123456789" --vendor "12345678" --date "2026-02-20" --output table`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return shared.UsageErrorf("unexpected argument(s): %s", strings.Join(args, " "))
			}

			resolvedAppID := shared.ResolveAppID(*appID)
			if resolvedAppID == "" {
				return shared.UsageError("--app is required (or set ASC_APP_ID)")
			}

			resolvedVendor := shared.ResolveVendorNumber(*vendor)
			if resolvedVendor == "" {
				return shared.UsageError("--vendor is required (or set ASC_VENDOR_NUMBER)")
			}

			reportDate, err := normalizeInsightsDate(*date, "--date")
			if err != nil {
				return shared.UsageError(err.Error())
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("insights daily: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := collectDailyInsights(requestCtx, client, resolvedAppID, resolvedVendor, reportDate)
			if err != nil {
				return fmt.Errorf("insights daily: %w", err)
			}

			return shared.PrintOutputWithRenderers(
				resp,
				*output.Output,
				*output.Pretty,
				func() error { renderDailyInsights(resp, false); return nil },
				func() error { renderDailyInsights(resp, true); return nil },
			)
		},
	}
}

type weeklyInsightsResponse struct {
	AppID        string               `json:"appId"`
	Source       weeklyInsightsSource `json:"source"`
	Week         weekRange            `json:"week"`
	PreviousWeek weekRange            `json:"previousWeek"`
	Metrics      []weeklyMetric       `json:"metrics"`
	GeneratedAt  string               `json:"generatedAt"`
}

type weeklyInsightsSource struct {
	Name            string `json:"name"`
	VendorNumber    string `json:"vendorNumber,omitempty"`
	AppSKU          string `json:"appSku,omitempty"`
	RequestsScanned int    `json:"requestsScanned,omitempty"`
}

type weekRange struct {
	Start string `json:"start"`
	End   string `json:"end"`
}

type weeklyMetric struct {
	Name         string   `json:"name"`
	Unit         string   `json:"unit,omitempty"`
	ThisWeek     *float64 `json:"thisWeek,omitempty"`
	LastWeek     *float64 `json:"lastWeek,omitempty"`
	Delta        *float64 `json:"delta,omitempty"`
	DeltaPercent *float64 `json:"deltaPercent,omitempty"`
	Status       string   `json:"status"`
	Reason       string   `json:"reason,omitempty"`
}

type dailyInsightsResponse struct {
	AppID        string              `json:"appId"`
	Source       dailyInsightsSource `json:"source"`
	Date         string              `json:"date"`
	PreviousDate string              `json:"previousDate"`
	Metrics      []dailyMetric       `json:"metrics"`
	GeneratedAt  string              `json:"generatedAt"`
}

type dailyInsightsSource struct {
	Name          string `json:"name"`
	VendorNumber  string `json:"vendorNumber,omitempty"`
	AppSKU        string `json:"appSku,omitempty"`
	ReportType    string `json:"reportType,omitempty"`
	ReportSubType string `json:"reportSubType,omitempty"`
	Frequency     string `json:"frequency,omitempty"`
	Version       string `json:"version,omitempty"`
}

type dailyMetric struct {
	Name         string   `json:"name"`
	Unit         string   `json:"unit,omitempty"`
	ThisDay      *float64 `json:"thisDay,omitempty"`
	PreviousDay  *float64 `json:"previousDay,omitempty"`
	Delta        *float64 `json:"delta,omitempty"`
	DeltaPercent *float64 `json:"deltaPercent,omitempty"`
	Status       string   `json:"status"`
	Reason       string   `json:"reason,omitempty"`
}

type reportWeekWindow struct {
	start time.Time
	end   time.Time
}

type salesWeekMetrics struct {
	rowCount                       int
	unitsColumnPresent             bool
	developerProceedsColumnPresent bool
	customerPriceColumnPresent     bool
	subscriptionColumnPresent      bool
	unitsTotal                     float64
	downloadUnitsTotal             float64
	monetizedUnitsTotal            float64
	developerProceedsTotal         float64
	customerPriceTotal             float64
	subscriptionRows               int
	subscriptionUnitsTotal         float64
	subscriptionDeveloperProceeds  float64
	subscriptionCustomerPrice      float64
	renewalRows                    int
	renewalUnitsTotal              float64
	renewalDeveloperProceeds       float64
	renewalCustomerPrice           float64
}

type salesScope struct {
	appID  string
	appSKU string
}

func collectWeeklyInsights(ctx context.Context, client *asc.Client, appID, sourceName, vendor string, weekStart time.Time) (*weeklyInsightsResponse, error) {
	thisWeek := weekWindowFromStart(weekStart)
	previousWeek := weekWindowFromStart(weekStart.AddDate(0, 0, -7))

	resp := &weeklyInsightsResponse{
		AppID: appID,
		Source: weeklyInsightsSource{
			Name: sourceName,
		},
		Week: weekRange{
			Start: thisWeek.start.Format("2006-01-02"),
			End:   thisWeek.end.Format("2006-01-02"),
		},
		PreviousWeek: weekRange{
			Start: previousWeek.start.Format("2006-01-02"),
			End:   previousWeek.end.Format("2006-01-02"),
		},
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
	}

	switch sourceName {
	case sourceSales:
		resp.Source.VendorNumber = vendor
		appResp, appErr := client.GetApp(ctx, appID)
		if appErr != nil {
			return nil, appErr
		}
		scope := salesScope{
			appID:  appID,
			appSKU: strings.TrimSpace(appResp.Data.Attributes.SKU),
		}
		resp.Source.AppSKU = scope.appSKU
		metrics := collectSalesMetrics(ctx, client, vendor, scope, thisWeek, previousWeek)
		resp.Metrics = metrics
	case sourceAnalytics:
		metrics, requestsScanned, err := collectAnalyticsMetrics(ctx, client, appID, thisWeek, previousWeek)
		if err != nil {
			return nil, err
		}
		resp.Source.RequestsScanned = requestsScanned
		resp.Metrics = metrics
	default:
		return nil, fmt.Errorf("unsupported source %q", sourceName)
	}

	return resp, nil
}

func collectDailyInsights(ctx context.Context, client *asc.Client, appID, vendor string, reportDate time.Time) (*dailyInsightsResponse, error) {
	appResp, err := client.GetApp(ctx, appID)
	if err != nil {
		return nil, err
	}

	scope := salesScope{
		appID:  appID,
		appSKU: strings.TrimSpace(appResp.Data.Attributes.SKU),
	}
	thisDay := reportDate.Format("2006-01-02")
	previousDay := reportDate.AddDate(0, 0, -1).Format("2006-01-02")

	thisData, thisErr := fetchSalesDayMetrics(ctx, client, vendor, thisDay, scope)
	prevData, prevErr := fetchSalesDayMetrics(ctx, client, vendor, previousDay, scope)

	availabilityReason := ""
	if thisErr != nil || prevErr != nil {
		reasons := make([]string, 0, 2)
		if thisErr != nil {
			reasons = append(reasons, fmt.Sprintf("selected day: %v", thisErr))
		}
		if prevErr != nil {
			reasons = append(reasons, fmt.Sprintf("previous day: %v", prevErr))
		}
		availabilityReason = strings.Join(reasons, "; ")
	}

	resp := &dailyInsightsResponse{
		AppID: appID,
		Source: dailyInsightsSource{
			Name:          sourceSales,
			VendorNumber:  vendor,
			AppSKU:        scope.appSKU,
			ReportType:    string(asc.SalesReportTypeSales),
			ReportSubType: string(asc.SalesReportSubTypeSummary),
			Frequency:     string(asc.SalesReportFrequencyDaily),
			Version:       string(asc.SalesReportVersion1_0),
		},
		Date:         thisDay,
		PreviousDate: previousDay,
		GeneratedAt:  time.Now().UTC().Format(time.RFC3339),
	}

	metrics := make([]dailyMetric, 0, 8)
	metrics = append(metrics, dailyMetricFromOptionalTotals(
		"renewal_rows",
		"count",
		thisData.subscriptionColumnPresent && prevData.subscriptionColumnPresent && availabilityReason == "",
		float64(thisData.renewalRows),
		float64(prevData.renewalRows),
		resolveSalesReason("subscription column", availabilityReason, thisData.subscriptionColumnPresent, prevData.subscriptionColumnPresent),
	))
	metrics = append(metrics, dailyMetricFromOptionalTotals(
		"renewal_units",
		"count",
		thisData.subscriptionColumnPresent && prevData.subscriptionColumnPresent &&
			thisData.unitsColumnPresent && prevData.unitsColumnPresent &&
			availabilityReason == "",
		thisData.renewalUnitsTotal,
		prevData.renewalUnitsTotal,
		resolveSalesReason("renewal units", availabilityReason, thisData.unitsColumnPresent && thisData.subscriptionColumnPresent, prevData.unitsColumnPresent && prevData.subscriptionColumnPresent),
	))
	metrics = append(metrics, dailyMetricFromOptionalTotals(
		"renewal_developer_proceeds",
		"currency",
		thisData.subscriptionColumnPresent && prevData.subscriptionColumnPresent &&
			thisData.developerProceedsColumnPresent && prevData.developerProceedsColumnPresent &&
			availabilityReason == "",
		thisData.renewalDeveloperProceeds,
		prevData.renewalDeveloperProceeds,
		resolveSalesReason("renewal developer proceeds", availabilityReason, thisData.developerProceedsColumnPresent && thisData.subscriptionColumnPresent, prevData.developerProceedsColumnPresent && prevData.subscriptionColumnPresent),
	))
	metrics = append(metrics, dailyMetricFromOptionalTotals(
		"subscription_rows",
		"count",
		thisData.subscriptionColumnPresent && prevData.subscriptionColumnPresent && availabilityReason == "",
		float64(thisData.subscriptionRows),
		float64(prevData.subscriptionRows),
		resolveSalesReason("subscription column", availabilityReason, thisData.subscriptionColumnPresent, prevData.subscriptionColumnPresent),
	))
	metrics = append(metrics, dailyMetricFromOptionalTotals(
		"subscription_units",
		"count",
		thisData.subscriptionColumnPresent && prevData.subscriptionColumnPresent &&
			thisData.unitsColumnPresent && prevData.unitsColumnPresent &&
			availabilityReason == "",
		thisData.subscriptionUnitsTotal,
		prevData.subscriptionUnitsTotal,
		resolveSalesReason("subscription units", availabilityReason, thisData.unitsColumnPresent && thisData.subscriptionColumnPresent, prevData.unitsColumnPresent && prevData.subscriptionColumnPresent),
	))
	metrics = append(metrics, dailyMetricFromOptionalTotals(
		"subscription_developer_proceeds",
		"currency",
		thisData.subscriptionColumnPresent && prevData.subscriptionColumnPresent &&
			thisData.developerProceedsColumnPresent && prevData.developerProceedsColumnPresent &&
			availabilityReason == "",
		thisData.subscriptionDeveloperProceeds,
		prevData.subscriptionDeveloperProceeds,
		resolveSalesReason("subscription developer proceeds", availabilityReason, thisData.developerProceedsColumnPresent && thisData.subscriptionColumnPresent, prevData.developerProceedsColumnPresent && prevData.subscriptionColumnPresent),
	))
	metrics = append(metrics, dailyMetricFromOptionalTotals(
		"monetized_units",
		"count",
		thisData.unitsColumnPresent && prevData.unitsColumnPresent && availabilityReason == "",
		thisData.monetizedUnitsTotal,
		prevData.monetizedUnitsTotal,
		resolveSalesReason("monetized units", availabilityReason, thisData.unitsColumnPresent, prevData.unitsColumnPresent),
	))
	metrics = append(metrics, dailyMetricFromOptionalTotals(
		"report_rows",
		"count",
		availabilityReason == "",
		float64(thisData.rowCount),
		float64(prevData.rowCount),
		availabilityReason,
	))

	resp.Metrics = metrics
	return resp, nil
}

func collectSalesMetrics(ctx context.Context, client *asc.Client, vendor string, scope salesScope, thisWeek, previousWeek reportWeekWindow) []weeklyMetric {
	thisData, thisErr := fetchSalesWeekMetrics(ctx, client, vendor, thisWeek.end.Format("2006-01-02"), scope)
	prevData, prevErr := fetchSalesWeekMetrics(ctx, client, vendor, previousWeek.end.Format("2006-01-02"), scope)

	availabilityReason := ""
	if thisErr != nil || prevErr != nil {
		reasons := make([]string, 0, 2)
		if thisErr != nil {
			reasons = append(reasons, fmt.Sprintf("this week: %v", thisErr))
		}
		if prevErr != nil {
			reasons = append(reasons, fmt.Sprintf("last week: %v", prevErr))
		}
		availabilityReason = strings.Join(reasons, "; ")
	}

	metrics := make([]weeklyMetric, 0, 7)
	metrics = append(metrics, metricFromOptionalTotals(
		"download_units",
		"count",
		thisData.unitsColumnPresent && prevData.unitsColumnPresent && availabilityReason == "",
		thisData.downloadUnitsTotal,
		prevData.downloadUnitsTotal,
		resolveSalesReason("download units", availabilityReason, thisData.unitsColumnPresent, prevData.unitsColumnPresent),
	))
	metrics = append(metrics, metricFromOptionalTotals(
		"monetized_units",
		"count",
		thisData.unitsColumnPresent && prevData.unitsColumnPresent && availabilityReason == "",
		thisData.monetizedUnitsTotal,
		prevData.monetizedUnitsTotal,
		resolveSalesReason("monetized units", availabilityReason, thisData.unitsColumnPresent, prevData.unitsColumnPresent),
	))
	metrics = append(metrics, metricFromOptionalTotals(
		"units",
		"count",
		thisData.unitsColumnPresent && prevData.unitsColumnPresent && availabilityReason == "",
		thisData.unitsTotal,
		prevData.unitsTotal,
		resolveSalesReason("units", availabilityReason, thisData.unitsColumnPresent, prevData.unitsColumnPresent),
	))
	metrics = append(metrics, metricFromOptionalTotals(
		"developer_proceeds",
		"currency",
		thisData.developerProceedsColumnPresent && prevData.developerProceedsColumnPresent && availabilityReason == "",
		thisData.developerProceedsTotal,
		prevData.developerProceedsTotal,
		resolveSalesReason("developer proceeds", availabilityReason, thisData.developerProceedsColumnPresent, prevData.developerProceedsColumnPresent),
	))
	metrics = append(metrics, metricFromOptionalTotals(
		"customer_price",
		"currency",
		thisData.customerPriceColumnPresent && prevData.customerPriceColumnPresent && availabilityReason == "",
		thisData.customerPriceTotal,
		prevData.customerPriceTotal,
		resolveSalesReason("customer price", availabilityReason, thisData.customerPriceColumnPresent, prevData.customerPriceColumnPresent),
	))

	metrics = append(metrics, metricFromOptionalTotals(
		"report_rows",
		"count",
		availabilityReason == "",
		float64(thisData.rowCount),
		float64(prevData.rowCount),
		availabilityReason,
	))
	metrics = append(metrics, unavailableMetric("active_devices", "count", "not derivable from sales summary exports"))

	return metrics
}

func fetchSalesWeekMetrics(ctx context.Context, client *asc.Client, vendor, reportDate string, scope salesScope) (salesWeekMetrics, error) {
	download, err := client.GetSalesReport(ctx, asc.SalesReportParams{
		VendorNumber:  vendor,
		ReportType:    asc.SalesReportTypeSales,
		ReportSubType: asc.SalesReportSubTypeSummary,
		Frequency:     asc.SalesReportFrequencyWeekly,
		ReportDate:    reportDate,
		Version:       asc.SalesReportVersion1_0,
	})
	if err != nil {
		return salesWeekMetrics{}, err
	}
	defer func() { _ = download.Body.Close() }()

	metrics, err := parseSalesReportMetrics(download.Body, scope)
	if err != nil {
		return salesWeekMetrics{}, err
	}
	return metrics, nil
}

func fetchSalesDayMetrics(ctx context.Context, client *asc.Client, vendor, reportDate string, scope salesScope) (salesWeekMetrics, error) {
	download, err := client.GetSalesReport(ctx, asc.SalesReportParams{
		VendorNumber:  vendor,
		ReportType:    asc.SalesReportTypeSales,
		ReportSubType: asc.SalesReportSubTypeSummary,
		Frequency:     asc.SalesReportFrequencyDaily,
		ReportDate:    reportDate,
		Version:       asc.SalesReportVersion1_0,
	})
	if err != nil {
		return salesWeekMetrics{}, err
	}
	defer func() { _ = download.Body.Close() }()

	metrics, err := parseSalesReportMetrics(download.Body, scope)
	if err != nil {
		return salesWeekMetrics{}, err
	}
	return metrics, nil
}

func parseSalesReportMetrics(reader io.Reader, scope salesScope) (salesWeekMetrics, error) {
	gzipReader, err := gzip.NewReader(reader)
	if err != nil {
		return salesWeekMetrics{}, fmt.Errorf("read gzip report: %w", err)
	}
	defer func() { _ = gzipReader.Close() }()

	tsvReader := csv.NewReader(gzipReader)
	tsvReader.Comma = '\t'
	tsvReader.FieldsPerRecord = -1
	tsvReader.LazyQuotes = true

	rows, err := tsvReader.ReadAll()
	if err != nil {
		return salesWeekMetrics{}, fmt.Errorf("parse report rows: %w", err)
	}
	if len(rows) == 0 {
		return salesWeekMetrics{}, fmt.Errorf("report is empty")
	}

	headers := rows[0]
	appleIdentifierIdx := findColumnIndex(headers, "appleidentifier")
	parentIdentifierIdx := findColumnIndex(headers, "parentidentifier")
	skuIdx := findColumnIndex(headers, "sku")
	subscriptionIdx := findColumnIndex(headers, "subscription")
	unitsIdx := findColumnIndex(headers, "units")
	developerProceedsIdx := findColumnIndex(headers, "developerproceeds")
	customerPriceIdx := findColumnIndex(headers, "customerprice")
	if appleIdentifierIdx < 0 && parentIdentifierIdx < 0 {
		return salesWeekMetrics{}, fmt.Errorf("report is missing Apple Identifier and Parent Identifier columns")
	}

	scope = enrichSalesScopeFromRows(scope, rows[1:], appleIdentifierIdx, skuIdx)
	metrics := salesWeekMetrics{
		unitsColumnPresent:             unitsIdx >= 0,
		developerProceedsColumnPresent: developerProceedsIdx >= 0,
		customerPriceColumnPresent:     customerPriceIdx >= 0,
		subscriptionColumnPresent:      subscriptionIdx >= 0,
	}
	for _, row := range rows[1:] {
		if isEmptyRow(row) {
			continue
		}

		appleIdentifier := strings.TrimSpace(valueAtIndex(row, appleIdentifierIdx))
		parentIdentifier := strings.TrimSpace(valueAtIndex(row, parentIdentifierIdx))
		isAppRow, isMonetizedRow, include := rowMatchesSalesScope(scope, appleIdentifier, parentIdentifier)
		if !include {
			continue
		}
		subscriptionValue := strings.TrimSpace(valueAtIndex(row, subscriptionIdx))
		isSubscriptionRow := subscriptionValue != ""
		isRenewalRow := isRenewalSubscriptionState(subscriptionValue)

		metrics.rowCount++
		if isSubscriptionRow {
			metrics.subscriptionRows++
		}
		if isRenewalRow {
			metrics.renewalRows++
		}

		if unitsIdx >= 0 {
			if value, ok := parseNumericValue(valueAtIndex(row, unitsIdx)); ok {
				metrics.unitsTotal += value
				if isAppRow {
					metrics.downloadUnitsTotal += value
				}
				if isMonetizedRow {
					metrics.monetizedUnitsTotal += value
				}
				if isSubscriptionRow {
					metrics.subscriptionUnitsTotal += value
				}
				if isRenewalRow {
					metrics.renewalUnitsTotal += value
				}
			}
		}
		if developerProceedsIdx >= 0 {
			if value, ok := parseNumericValue(valueAtIndex(row, developerProceedsIdx)); ok {
				metrics.developerProceedsTotal += value
				if isSubscriptionRow {
					metrics.subscriptionDeveloperProceeds += value
				}
				if isRenewalRow {
					metrics.renewalDeveloperProceeds += value
				}
			}
		}
		if customerPriceIdx >= 0 {
			if value, ok := parseNumericValue(valueAtIndex(row, customerPriceIdx)); ok {
				metrics.customerPriceTotal += value
				if isSubscriptionRow {
					metrics.subscriptionCustomerPrice += value
				}
				if isRenewalRow {
					metrics.renewalCustomerPrice += value
				}
			}
		}
	}

	return metrics, nil
}

func enrichSalesScopeFromRows(scope salesScope, rows [][]string, appleIdentifierIdx, skuIdx int) salesScope {
	if strings.TrimSpace(scope.appSKU) != "" {
		return scope
	}
	if appleIdentifierIdx < 0 || skuIdx < 0 {
		return scope
	}
	for _, row := range rows {
		if isEmptyRow(row) {
			continue
		}
		appleIdentifier := strings.TrimSpace(valueAtIndex(row, appleIdentifierIdx))
		if appleIdentifier != strings.TrimSpace(scope.appID) {
			continue
		}
		sku := strings.TrimSpace(valueAtIndex(row, skuIdx))
		if sku != "" {
			scope.appSKU = sku
			return scope
		}
	}
	return scope
}

func rowMatchesSalesScope(scope salesScope, appleIdentifier, parentIdentifier string) (isAppRow bool, isMonetizedRow bool, include bool) {
	appID := strings.TrimSpace(scope.appID)
	appSKU := strings.TrimSpace(scope.appSKU)
	appleIdentifier = strings.TrimSpace(appleIdentifier)
	parentIdentifier = strings.TrimSpace(parentIdentifier)

	isAppRow = appID != "" && appleIdentifier == appID
	isMonetizedRow = false
	if appSKU != "" && parentIdentifier == appSKU {
		isMonetizedRow = true
	}
	if appID != "" && parentIdentifier == appID {
		isMonetizedRow = true
	}
	return isAppRow, isMonetizedRow, isAppRow || isMonetizedRow
}

func isRenewalSubscriptionState(value string) bool {
	normalized := strings.ToLower(strings.TrimSpace(value))
	if normalized == "" {
		return false
	}
	return strings.Contains(normalized, "renew")
}

func collectAnalyticsMetrics(ctx context.Context, client *asc.Client, appID string, thisWeek, previousWeek reportWeekWindow) ([]weeklyMetric, int, error) {
	requestsResp, err := client.GetAnalyticsReportRequests(
		ctx,
		appID,
		asc.WithAnalyticsReportRequestsLimit(200),
	)
	if err != nil {
		if isLikelyForbidden(err) {
			return analyticsUnavailableMetrics("analytics source is not permitted for the current API key"), 0, nil
		}
		if isLikelyNotFound(err) {
			return analyticsUnavailableMetrics("analytics data is unavailable for this app"), 0, nil
		}
		return nil, 0, err
	}

	completedRequests := make([]asc.AnalyticsReportRequestResource, 0, len(requestsResp.Data))
	for _, request := range requestsResp.Data {
		if request.Attributes.State == asc.AnalyticsReportRequestStateCompleted {
			completedRequests = append(completedRequests, request)
		}
	}

	requestCount := len(completedRequests)
	if requestCount == 0 {
		return analyticsUnavailableMetrics("no completed analytics report requests found"), 0, nil
	}

	var (
		thisCompletedRequests int
		lastCompletedRequests int
		thisInstances         int
		lastInstances         int
	)
	thisReportIDs := make(map[string]struct{})
	lastReportIDs := make(map[string]struct{})

	for _, request := range completedRequests {
		if createdAt, ok := parseDateValue(request.Attributes.CreatedDate); ok {
			if containsDate(thisWeek, createdAt) {
				thisCompletedRequests++
			}
			if containsDate(previousWeek, createdAt) {
				lastCompletedRequests++
			}
		}

		reportsResp, reportsErr := client.GetAnalyticsReports(
			ctx,
			request.ID,
			asc.WithAnalyticsReportsLimit(200),
		)
		if reportsErr != nil {
			if isLikelyForbidden(reportsErr) {
				return analyticsUnavailableMetrics("analytics report metadata endpoints are not permitted for the current API key"), requestCount, nil
			}
			if isLikelyNotFound(reportsErr) {
				return analyticsUnavailableMetrics("analytics report metadata is unavailable for this app"), requestCount, nil
			}
			return nil, requestCount, reportsErr
		}

		for _, report := range reportsResp.Data {
			instancesResp, instancesErr := client.GetAnalyticsReportInstances(
				ctx,
				report.ID,
				asc.WithAnalyticsReportInstancesLimit(200),
			)
			if instancesErr != nil {
				if isLikelyForbidden(instancesErr) {
					return analyticsUnavailableMetrics("analytics report instance endpoints are not permitted for the current API key"), requestCount, nil
				}
				if isLikelyNotFound(instancesErr) {
					return analyticsUnavailableMetrics("analytics report instances are unavailable for this app"), requestCount, nil
				}
				return nil, requestCount, instancesErr
			}

			for _, instance := range instancesResp.Data {
				reportDate, ok := parseDateValue(instance.Attributes.ReportDate)
				if !ok {
					continue
				}
				if containsDate(thisWeek, reportDate) {
					thisInstances++
					thisReportIDs[report.ID] = struct{}{}
				}
				if containsDate(previousWeek, reportDate) {
					lastInstances++
					lastReportIDs[report.ID] = struct{}{}
				}
			}
		}
	}

	metrics := []weeklyMetric{
		comparableMetric("completed_requests", "count", float64(thisCompletedRequests), float64(lastCompletedRequests)),
		comparableMetric("reports_available", "count", float64(len(thisReportIDs)), float64(len(lastReportIDs))),
		comparableMetric("instances_available", "count", float64(thisInstances), float64(lastInstances)),
		unavailableMetric("business_conversion_rate", "percent", "not derivable from analytics metadata alone"),
	}
	return metrics, requestCount, nil
}

func analyticsUnavailableMetrics(reason string) []weeklyMetric {
	return []weeklyMetric{
		unavailableMetric("completed_requests", "count", reason),
		unavailableMetric("reports_available", "count", reason),
		unavailableMetric("instances_available", "count", reason),
		unavailableMetric("business_conversion_rate", "percent", "not derivable from analytics metadata alone"),
	}
}

func isLikelyForbidden(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, asc.ErrForbidden) {
		return true
	}

	var apiErr *asc.APIError
	if errors.As(err, &apiErr) {
		if apiErr.StatusCode == http.StatusForbidden {
			return true
		}
		if strings.Contains(strings.ToLower(strings.TrimSpace(apiErr.Code)), "forbidden") {
			return true
		}
		if strings.Contains(strings.ToLower(strings.TrimSpace(apiErr.Title)), "forbidden") {
			return true
		}
	}

	return strings.Contains(strings.ToLower(err.Error()), "forbidden")
}

func isLikelyNotFound(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, asc.ErrNotFound) || asc.IsNotFound(err) {
		return true
	}

	var apiErr *asc.APIError
	if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound {
		return true
	}

	message := strings.ToLower(err.Error())
	return strings.Contains(message, "not found")
}

func resolveSalesReason(metricName, availabilityReason string, thisAvailable, lastAvailable bool) string {
	if availabilityReason != "" {
		return availabilityReason
	}
	if !thisAvailable && !lastAvailable {
		return fmt.Sprintf("%s column is missing or non-numeric in both weeks", metricName)
	}
	if !thisAvailable {
		return fmt.Sprintf("%s column is missing or non-numeric for this week", metricName)
	}
	if !lastAvailable {
		return fmt.Sprintf("%s column is missing or non-numeric for last week", metricName)
	}
	return ""
}

func metricFromOptionalTotals(name, unit string, available bool, thisValue, lastValue float64, reason string) weeklyMetric {
	if !available {
		return unavailableMetric(name, unit, reason)
	}
	return comparableMetric(name, unit, thisValue, lastValue)
}

func dailyMetricFromOptionalTotals(name, unit string, available bool, thisValue, previousValue float64, reason string) dailyMetric {
	if !available {
		return unavailableDailyMetric(name, unit, reason)
	}
	return comparableDailyMetric(name, unit, thisValue, previousValue)
}

func comparableMetric(name, unit string, thisValue, lastValue float64) weeklyMetric {
	metric := weeklyMetric{
		Name:     name,
		Unit:     unit,
		ThisWeek: ptrFloat64(thisValue),
		LastWeek: ptrFloat64(lastValue),
		Delta:    ptrFloat64(thisValue - lastValue),
		Status:   "ok",
	}
	if lastValue != 0 {
		deltaPercent := ((thisValue - lastValue) / lastValue) * 100
		metric.DeltaPercent = ptrFloat64(deltaPercent)
	}
	return metric
}

func unavailableMetric(name, unit, reason string) weeklyMetric {
	return weeklyMetric{
		Name:   name,
		Unit:   unit,
		Status: "unavailable",
		Reason: reason,
	}
}

func comparableDailyMetric(name, unit string, thisValue, previousValue float64) dailyMetric {
	metric := dailyMetric{
		Name:        name,
		Unit:        unit,
		ThisDay:     ptrFloat64(thisValue),
		PreviousDay: ptrFloat64(previousValue),
		Delta:       ptrFloat64(thisValue - previousValue),
		Status:      "ok",
	}
	if previousValue != 0 {
		deltaPercent := ((thisValue - previousValue) / previousValue) * 100
		metric.DeltaPercent = ptrFloat64(deltaPercent)
	}
	return metric
}

func unavailableDailyMetric(name, unit, reason string) dailyMetric {
	return dailyMetric{
		Name:   name,
		Unit:   unit,
		Status: "unavailable",
		Reason: reason,
	}
}

func ptrFloat64(value float64) *float64 {
	return &value
}

func parseNumericValue(value string) (float64, bool) {
	normalized := strings.TrimSpace(value)
	if normalized == "" {
		return 0, false
	}

	negative := false
	if strings.HasPrefix(normalized, "(") && strings.HasSuffix(normalized, ")") {
		negative = true
		normalized = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(normalized, "("), ")"))
	}

	replacements := []string{",", "$", "€", "£", "¥"}
	for _, token := range replacements {
		normalized = strings.ReplaceAll(normalized, token, "")
	}

	parsed, err := strconv.ParseFloat(normalized, 64)
	if err != nil {
		return 0, false
	}
	if negative {
		parsed = -parsed
	}
	return parsed, true
}

func findColumnIndex(headers []string, target string) int {
	normalizedTarget := normalizeColumnName(target)
	for index, header := range headers {
		if normalizeColumnName(header) == normalizedTarget {
			return index
		}
	}
	return -1
}

func normalizeColumnName(value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	normalized = strings.ReplaceAll(normalized, " ", "")
	normalized = strings.ReplaceAll(normalized, "_", "")
	normalized = strings.ReplaceAll(normalized, "-", "")
	normalized = strings.ReplaceAll(normalized, "/", "")
	return normalized
}

func valueAtIndex(values []string, index int) string {
	if index < 0 || index >= len(values) {
		return ""
	}
	return values[index]
}

func isEmptyRow(values []string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return false
		}
	}
	return true
}

func weekWindowFromStart(start time.Time) reportWeekWindow {
	startUTC := time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, time.UTC)
	return reportWeekWindow{
		start: startUTC,
		end:   startUTC.AddDate(0, 0, 6),
	}
}

func containsDate(window reportWeekWindow, date time.Time) bool {
	candidate := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, time.UTC)
	if candidate.Before(window.start) {
		return false
	}
	if candidate.After(window.end) {
		return false
	}
	return true
}

func parseDateValue(value string) (time.Time, bool) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return time.Time{}, false
	}
	layouts := []string{
		"2006-01-02",
		time.RFC3339,
		time.RFC3339Nano,
	}
	for _, layout := range layouts {
		parsed, err := time.Parse(layout, trimmed)
		if err == nil {
			return parsed.UTC(), true
		}
	}
	return time.Time{}, false
}

func normalizeWeekStart(value string) (time.Time, error) {
	return normalizeInsightsDate(value, "--week")
}

func normalizeInsightsDate(value, flagName string) (time.Time, error) {
	normalized, err := shared.NormalizeDate(value, flagName)
	if err != nil {
		return time.Time{}, err
	}
	parsed, parseErr := time.Parse("2006-01-02", normalized)
	if parseErr != nil {
		return time.Time{}, fmt.Errorf("%s must be in YYYY-MM-DD format", flagName)
	}
	return parsed.UTC(), nil
}

func isAllowedSource(name string) bool {
	return slices.Contains([]string{sourceAnalytics, sourceSales}, name)
}

func renderWeeklyInsights(resp *weeklyInsightsResponse, markdown bool) {
	contextRows := [][]string{
		{"appId", resp.AppID},
		{"source", resp.Source.Name},
		{"week", fmt.Sprintf("%s to %s", resp.Week.Start, resp.Week.End)},
		{"previousWeek", fmt.Sprintf("%s to %s", resp.PreviousWeek.Start, resp.PreviousWeek.End)},
		{"generatedAt", resp.GeneratedAt},
	}
	if strings.TrimSpace(resp.Source.VendorNumber) != "" {
		contextRows = append(contextRows, []string{"vendorNumber", resp.Source.VendorNumber})
	}
	if resp.Source.RequestsScanned > 0 {
		contextRows = append(contextRows, []string{"requestsScanned", strconv.Itoa(resp.Source.RequestsScanned)})
	}
	shared.RenderSection("Context", []string{"field", "value"}, contextRows, markdown)

	metricRows := make([][]string, 0, len(resp.Metrics))
	for _, metric := range resp.Metrics {
		metricRows = append(metricRows, []string{
			metric.Name,
			shared.OrNA(metric.Unit),
			formatOptionalNumber(metric.ThisWeek),
			formatOptionalNumber(metric.LastWeek),
			formatOptionalNumber(metric.Delta),
			formatOptionalNumber(metric.DeltaPercent),
			metric.Status,
			shared.OrNA(metric.Reason),
		})
	}
	shared.RenderSection("Metrics", []string{"metric", "unit", "thisWeek", "lastWeek", "delta", "deltaPercent", "status", "reason"}, metricRows, markdown)
}

func renderDailyInsights(resp *dailyInsightsResponse, markdown bool) {
	contextRows := [][]string{
		{"appId", resp.AppID},
		{"source", resp.Source.Name},
		{"date", resp.Date},
		{"previousDate", resp.PreviousDate},
		{"generatedAt", resp.GeneratedAt},
	}
	if strings.TrimSpace(resp.Source.VendorNumber) != "" {
		contextRows = append(contextRows, []string{"vendorNumber", resp.Source.VendorNumber})
	}
	if strings.TrimSpace(resp.Source.AppSKU) != "" {
		contextRows = append(contextRows, []string{"appSku", resp.Source.AppSKU})
	}
	if strings.TrimSpace(resp.Source.ReportType) != "" {
		contextRows = append(contextRows, []string{"reportType", resp.Source.ReportType})
	}
	if strings.TrimSpace(resp.Source.ReportSubType) != "" {
		contextRows = append(contextRows, []string{"reportSubType", resp.Source.ReportSubType})
	}
	if strings.TrimSpace(resp.Source.Frequency) != "" {
		contextRows = append(contextRows, []string{"frequency", resp.Source.Frequency})
	}
	if strings.TrimSpace(resp.Source.Version) != "" {
		contextRows = append(contextRows, []string{"version", resp.Source.Version})
	}
	shared.RenderSection("Context", []string{"field", "value"}, contextRows, markdown)

	metricRows := make([][]string, 0, len(resp.Metrics))
	for _, metric := range resp.Metrics {
		metricRows = append(metricRows, []string{
			metric.Name,
			shared.OrNA(metric.Unit),
			formatOptionalNumber(metric.ThisDay),
			formatOptionalNumber(metric.PreviousDay),
			formatOptionalNumber(metric.Delta),
			formatOptionalNumber(metric.DeltaPercent),
			metric.Status,
			shared.OrNA(metric.Reason),
		})
	}
	shared.RenderSection("Metrics", []string{"metric", "unit", "thisDay", "previousDay", "delta", "deltaPercent", "status", "reason"}, metricRows, markdown)
}

func formatOptionalNumber(value *float64) string {
	if value == nil {
		return "n/a"
	}
	return fmt.Sprintf("%.2f", *value)
}
