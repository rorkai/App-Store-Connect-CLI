package insights

import (
	"compress/gzip"
	"context"
	"encoding/csv"
	"errors"
	"flag"
	"fmt"
	"io"
	"maps"
	"math"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

const (
	sourceAnalytics = "analytics"
	sourceSales     = "sales"

	analyticsInstancePageLimit = 200

	analyticsInstanceFetchConcurrency = 4
)

// analyticsInstanceGranularities covers every documented instance granularity so
// the processing-date filter alone decides which instances count for a week.
var analyticsInstanceGranularities = []string{"DAILY", "WEEKLY", "MONTHLY"}

// NewInsightsCommand returns the insights command group built on d.
func NewInsightsCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("insights", flag.ExitOnError)

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
		UsageFunc: d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newInsightsWeeklyCommand(d),
			newInsightsDailyCommand(d),
		},
		Exec: func(_ context.Context, _ []string) error {
			return flag.ErrHelp
		},
	}
}

func newInsightsWeeklyCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("insights weekly", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (required, or ASC_APP_ID env)")
	source := fs.String("source", "", "Insights source: analytics or sales")
	week := fs.String("week", "", "Week start date (YYYY-MM-DD)")
	vendor := fs.String("vendor", "", "Vendor number for sales source (or ASC_VENDOR_NUMBER)")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "weekly",
		ShortUsage: "asc insights weekly --app \"APP_ID\" --source analytics|sales --week \"YYYY-MM-DD\" [flags]",
		ShortHelp:  "Summarize this week vs last week metrics.",
		LongHelp: `Summarize this week vs last week metrics.

The output is deterministic JSON by default and can be rendered as table/markdown.

For --source sales, totals are scoped to the selected app and include linked in-app purchases
and subscriptions by matching Parent Identifier against the app SKU.

Monetary metrics multiply the report's per-unit amounts by Units, so refunds reduce them.
They are totaled per currency and never converted: a metric has one total only when every
amount shares a currency, otherwise it is unavailable and byCurrency lists each currency.

Examples:
  asc insights weekly --app "123456789" --source analytics --week "2026-02-16"
  asc insights weekly --app "123456789" --source sales --week "2026-02-16" --vendor "12345678"
  asc insights weekly --app "123456789" --source sales --week "2026-02-16" --output table`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			resolvedAppID := d.ResolveAppID(*appID)
			if resolvedAppID == "" {
				return d.UsageError("--app is required (or set ASC_APP_ID)")
			}

			sourceName := strings.ToLower(strings.TrimSpace(*source))
			if sourceName == "" {
				return d.UsageError("--source is required")
			}
			if !isAllowedSource(sourceName) {
				return d.UsageErrorf("--source must be one of: %s, %s", sourceAnalytics, sourceSales)
			}

			weekStart, err := normalizeWeekStart(*week)
			if err != nil {
				return d.UsageError(err.Error())
			}

			resolvedVendor := ""
			if sourceName == sourceSales {
				resolvedVendor = d.ResolveVendorNumber(*vendor)
				if resolvedVendor == "" {
					return d.UsageError("--vendor is required for --source sales (or set ASC_VENDOR_NUMBER)")
				}
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("insights weekly: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			resp, err := collectWeeklyInsights(requestCtx, client, d.Now, resolvedAppID, sourceName, resolvedVendor, weekStart)
			if err != nil {
				return fmt.Errorf("insights weekly: %w", err)
			}

			return d.PrintOutputWithRenderers(
				resp,
				*output.Output,
				*output.Pretty,
				func() error { renderWeeklyInsights(d, resp, false); return nil },
				func() error { renderWeeklyInsights(d, resp, true); return nil },
			)
		},
	}
}

func newInsightsDailyCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("insights daily", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (required, or ASC_APP_ID env)")
	vendor := fs.String("vendor", "", "Vendor number for sales source (or ASC_VENDOR_NUMBER)")
	date := fs.String("date", "", "Report date (YYYY-MM-DD)")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "daily",
		ShortUsage: "asc insights daily --app \"APP_ID\" --vendor \"VENDOR\" --date \"YYYY-MM-DD\" [flags]",
		ShortHelp:  "Summarize daily subscription renewal signals from sales exports.",
		LongHelp: `Summarize daily subscription renewal signals from sales exports.

This command is sales-only and app-scoped. It compares the selected day to the previous day.
Renewal metrics are derived from rows where Subscription equals "Renewal" for products linked to the app.

Proceeds metrics multiply the report's per-unit amounts by Units, so refunds reduce them.
They are totaled per currency and never converted: a metric has one total only when every
amount shares a currency, otherwise it is unavailable and byCurrency lists each currency.

Examples:
  asc insights daily --app "123456789" --vendor "12345678" --date "2026-02-20"
  asc insights daily --app "123456789" --vendor "12345678" --date "2026-02-20" --output table`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			resolvedAppID := d.ResolveAppID(*appID)
			if resolvedAppID == "" {
				return d.UsageError("--app is required (or set ASC_APP_ID)")
			}

			resolvedVendor := d.ResolveVendorNumber(*vendor)
			if resolvedVendor == "" {
				return d.UsageError("--vendor is required (or set ASC_VENDOR_NUMBER)")
			}

			reportDate, err := normalizeInsightsDate(*date, "--date")
			if err != nil {
				return d.UsageError(err.Error())
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("insights daily: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			resp, err := collectDailyInsights(requestCtx, client, d.Now, resolvedAppID, resolvedVendor, reportDate)
			if err != nil {
				return fmt.Errorf("insights daily: %w", err)
			}

			return d.PrintOutputWithRenderers(
				resp,
				*output.Output,
				*output.Pretty,
				func() error { renderDailyInsights(d, resp, false); return nil },
				func() error { renderDailyInsights(d, resp, true); return nil },
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
	Name         string                 `json:"name"`
	Unit         string                 `json:"unit,omitempty"`
	Currency     string                 `json:"currency,omitempty"`
	ThisWeek     *float64               `json:"thisWeek,omitempty"`
	LastWeek     *float64               `json:"lastWeek,omitempty"`
	Delta        *float64               `json:"delta,omitempty"`
	DeltaPercent *float64               `json:"deltaPercent,omitempty"`
	Status       string                 `json:"status"`
	Reason       string                 `json:"reason,omitempty"`
	ByCurrency   []weeklyCurrencyAmount `json:"byCurrency,omitempty"`
}

// weeklyCurrencyAmount is one currency's share of a monetary weekly metric.
type weeklyCurrencyAmount struct {
	Currency     string   `json:"currency"`
	ThisWeek     float64  `json:"thisWeek"`
	LastWeek     float64  `json:"lastWeek"`
	Delta        float64  `json:"delta"`
	DeltaPercent *float64 `json:"deltaPercent,omitempty"`
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
	Name         string                `json:"name"`
	Unit         string                `json:"unit,omitempty"`
	Currency     string                `json:"currency,omitempty"`
	ThisDay      *float64              `json:"thisDay,omitempty"`
	PreviousDay  *float64              `json:"previousDay,omitempty"`
	Delta        *float64              `json:"delta,omitempty"`
	DeltaPercent *float64              `json:"deltaPercent,omitempty"`
	Status       string                `json:"status"`
	Reason       string                `json:"reason,omitempty"`
	ByCurrency   []dailyCurrencyAmount `json:"byCurrency,omitempty"`
}

// dailyCurrencyAmount is one currency's share of a monetary daily metric.
type dailyCurrencyAmount struct {
	Currency     string   `json:"currency"`
	ThisDay      float64  `json:"thisDay"`
	PreviousDay  float64  `json:"previousDay"`
	Delta        float64  `json:"delta"`
	DeltaPercent *float64 `json:"deltaPercent,omitempty"`
}

type reportWeekWindow struct {
	start time.Time
	end   time.Time
}

// SalesMetrics holds aggregated metrics from a parsed sales report.
//
// Monetary totals are row amounts (Units x the per-unit column) grouped by
// currency. The *ColumnPresent flags only describe the per-unit column; a total
// also needs the Units column.
type SalesMetrics struct {
	RowCount                                int
	UnitsColumnPresent                      bool
	DownloadUnitsAvailable                  bool
	DeveloperProceedsColumnPresent          bool
	CustomerPriceColumnPresent              bool
	SubscriptionColumnPresent               bool
	UnitsTotal                              float64
	DownloadUnitsTotal                      float64
	MonetizedUnitsTotal                     float64
	DeveloperProceedsByCurrency             CurrencyAmounts
	CustomerPriceByCurrency                 CurrencyAmounts
	SubscriptionRows                        int
	SubscriptionUnitsTotal                  float64
	SubscriptionDeveloperProceedsByCurrency CurrencyAmounts
	RenewalRows                             int
	RenewalUnitsTotal                       float64
	RenewalDeveloperProceedsByCurrency      CurrencyAmounts
}

// CurrencyAmounts holds monetary totals keyed by upper-case ISO currency code.
// Amounts in different currencies are never added together. The empty key
// collects amounts from reports that do not say which currency they are in.
type CurrencyAmounts map[string]float64

// Plus returns the per-currency sum of c and other without modifying either.
func (c CurrencyAmounts) Plus(other CurrencyAmounts) CurrencyAmounts {
	if len(c) == 0 && len(other) == 0 {
		return nil
	}
	sum := make(CurrencyAmounts, max(len(c), len(other)))
	maps.Copy(sum, c)
	for currency, amount := range other {
		sum[currency] += amount
	}
	return sum
}

// add records one row's amount. Zero amounts are skipped so free downloads do
// not make a report look like it holds money in their storefront's currency.
func (c *CurrencyAmounts) add(currency string, amount float64) {
	if amount == 0 {
		return
	}
	if *c == nil {
		*c = make(CurrencyAmounts)
	}
	(*c)[currency] += amount
}

// CurrencyComparison pairs one currency's totals across two compared periods.
type CurrencyComparison struct {
	Currency string
	Current  float64
	Previous float64
}

// CompareCurrencyAmounts pairs the per-currency totals of two periods, sorted
// by currency code. A currency missing from one period compares against zero.
func CompareCurrencyAmounts(current, previous CurrencyAmounts) []CurrencyComparison {
	currencies := make([]string, 0, len(current)+len(previous))
	for currency := range current {
		currencies = append(currencies, currency)
	}
	for currency := range previous {
		if _, seen := current[currency]; !seen {
			currencies = append(currencies, currency)
		}
	}
	slices.Sort(currencies)

	comparisons := make([]CurrencyComparison, 0, len(currencies))
	for _, currency := range currencies {
		comparisons = append(comparisons, CurrencyComparison{
			Currency: currency,
			Current:  current[currency],
			Previous: previous[currency],
		})
	}
	return comparisons
}

// CurrencyLabel names a currency code for people, including the empty code.
func CurrencyLabel(currency string) string {
	if currency == "" {
		return "unknown"
	}
	return currency
}

// MixedCurrencyReason explains why a monetary metric has no single total.
func MixedCurrencyReason(metricLabel string, comparisons []CurrencyComparison) string {
	labels := make([]string, 0, len(comparisons))
	for _, comparison := range comparisons {
		labels = append(labels, CurrencyLabel(comparison.Currency))
	}
	return fmt.Sprintf("%s are in multiple currencies (%s) and cannot be summed; totals are reported per currency", metricLabel, strings.Join(labels, ", "))
}

// Unexported alias so all internal references keep compiling with the old name.
type salesWeekMetrics = SalesMetrics

// SalesScope identifies an app for scoping sales report row filtering.
type SalesScope struct {
	AppID  string
	AppSKU string
}

type salesScope = SalesScope

func collectWeeklyInsights(ctx context.Context, client *asc.Client, now func() time.Time, appID, sourceName, vendor string, weekStart time.Time) (*weeklyInsightsResponse, error) {
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
		GeneratedAt: now().UTC().Format(time.RFC3339),
	}

	switch sourceName {
	case sourceSales:
		resp.Source.VendorNumber = vendor
		scope, reports, reportErrs, appErr := fetchAppSalesReports(ctx, client, appID, [2]asc.SalesReportParams{
			salesSummaryReportParams(vendor, asc.SalesReportFrequencyWeekly, thisWeek.end.Format("2006-01-02")),
			salesSummaryReportParams(vendor, asc.SalesReportFrequencyWeekly, previousWeek.end.Format("2006-01-02")),
		})
		if appErr != nil {
			return nil, appErr
		}
		resp.Source.AppSKU = scope.AppSKU
		resp.Metrics = collectSalesMetrics(reports[0], reportErrs[0], reports[1], reportErrs[1])
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

func collectDailyInsights(ctx context.Context, client *asc.Client, now func() time.Time, appID, vendor string, reportDate time.Time) (*dailyInsightsResponse, error) {
	thisDay := reportDate.Format("2006-01-02")
	previousDay := reportDate.AddDate(0, 0, -1).Format("2006-01-02")

	scope, reports, reportErrs, err := fetchAppSalesReports(ctx, client, appID, [2]asc.SalesReportParams{
		salesSummaryReportParams(vendor, asc.SalesReportFrequencyDaily, thisDay),
		salesSummaryReportParams(vendor, asc.SalesReportFrequencyDaily, previousDay),
	})
	if err != nil {
		return nil, err
	}
	thisData, thisErr := reports[0], reportErrs[0]
	prevData, prevErr := reports[1], reportErrs[1]

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
			AppSKU:        scope.AppSKU,
			ReportType:    string(asc.SalesReportTypeSales),
			ReportSubType: string(asc.SalesReportSubTypeSummary),
			Frequency:     string(asc.SalesReportFrequencyDaily),
			Version:       string(asc.SalesReportVersion1_0),
		},
		Date:         thisDay,
		PreviousDate: previousDay,
		GeneratedAt:  now().UTC().Format(time.RFC3339),
	}

	metrics := make([]dailyMetric, 0, 8)
	metrics = append(metrics, dailyCountMetricFromOptionalTotals(
		"renewal_rows",
		thisData.SubscriptionColumnPresent && prevData.SubscriptionColumnPresent && availabilityReason == "",
		float64(thisData.RenewalRows),
		float64(prevData.RenewalRows),
		resolveSalesReason("subscription column", availabilityReason, thisData.SubscriptionColumnPresent, prevData.SubscriptionColumnPresent),
	))
	metrics = append(metrics, dailyCountMetricFromOptionalTotals(
		"renewal_units",
		thisData.SubscriptionColumnPresent && prevData.SubscriptionColumnPresent &&
			thisData.UnitsColumnPresent && prevData.UnitsColumnPresent &&
			availabilityReason == "",
		thisData.RenewalUnitsTotal,
		prevData.RenewalUnitsTotal,
		resolveSalesReason("renewal units", availabilityReason, thisData.UnitsColumnPresent && thisData.SubscriptionColumnPresent, prevData.UnitsColumnPresent && prevData.SubscriptionColumnPresent),
	))
	thisSubscriptionProceedsAvailable := thisData.SubscriptionColumnPresent && developerProceedsAvailable(thisData)
	prevSubscriptionProceedsAvailable := prevData.SubscriptionColumnPresent && developerProceedsAvailable(prevData)
	metrics = append(metrics, dailyCurrencyMetric(
		"renewal_developer_proceeds",
		"renewal developer proceeds totals",
		thisSubscriptionProceedsAvailable && prevSubscriptionProceedsAvailable && availabilityReason == "",
		thisData.RenewalDeveloperProceedsByCurrency,
		prevData.RenewalDeveloperProceedsByCurrency,
		resolveSalesReason("renewal developer proceeds", availabilityReason, thisSubscriptionProceedsAvailable, prevSubscriptionProceedsAvailable),
	))
	metrics = append(metrics, dailyCountMetricFromOptionalTotals(
		"subscription_rows",
		thisData.SubscriptionColumnPresent && prevData.SubscriptionColumnPresent && availabilityReason == "",
		float64(thisData.SubscriptionRows),
		float64(prevData.SubscriptionRows),
		resolveSalesReason("subscription column", availabilityReason, thisData.SubscriptionColumnPresent, prevData.SubscriptionColumnPresent),
	))
	metrics = append(metrics, dailyCountMetricFromOptionalTotals(
		"subscription_units",
		thisData.SubscriptionColumnPresent && prevData.SubscriptionColumnPresent &&
			thisData.UnitsColumnPresent && prevData.UnitsColumnPresent &&
			availabilityReason == "",
		thisData.SubscriptionUnitsTotal,
		prevData.SubscriptionUnitsTotal,
		resolveSalesReason("subscription units", availabilityReason, thisData.UnitsColumnPresent && thisData.SubscriptionColumnPresent, prevData.UnitsColumnPresent && prevData.SubscriptionColumnPresent),
	))
	metrics = append(metrics, dailyCurrencyMetric(
		"subscription_developer_proceeds",
		"subscription developer proceeds totals",
		thisSubscriptionProceedsAvailable && prevSubscriptionProceedsAvailable && availabilityReason == "",
		thisData.SubscriptionDeveloperProceedsByCurrency,
		prevData.SubscriptionDeveloperProceedsByCurrency,
		resolveSalesReason("subscription developer proceeds", availabilityReason, thisSubscriptionProceedsAvailable, prevSubscriptionProceedsAvailable),
	))
	metrics = append(metrics, dailyCountMetricFromOptionalTotals(
		"monetized_units",
		thisData.UnitsColumnPresent && prevData.UnitsColumnPresent && availabilityReason == "",
		thisData.MonetizedUnitsTotal,
		prevData.MonetizedUnitsTotal,
		resolveSalesReason("monetized units", availabilityReason, thisData.UnitsColumnPresent, prevData.UnitsColumnPresent),
	))
	metrics = append(metrics, dailyCountMetricFromOptionalTotals(
		"report_rows",
		availabilityReason == "",
		float64(thisData.RowCount),
		float64(prevData.RowCount),
		availabilityReason,
	))

	resp.Metrics = metrics
	return resp, nil
}

func collectSalesMetrics(thisData salesWeekMetrics, thisErr error, prevData salesWeekMetrics, prevErr error) []weeklyMetric {
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
	metrics = append(metrics, countMetricFromOptionalTotals(
		"download_units",
		thisData.DownloadUnitsAvailable && prevData.DownloadUnitsAvailable && availabilityReason == "",
		thisData.DownloadUnitsTotal,
		prevData.DownloadUnitsTotal,
		resolveSalesReason("download units", availabilityReason, thisData.DownloadUnitsAvailable, prevData.DownloadUnitsAvailable),
	))
	metrics = append(metrics, countMetricFromOptionalTotals(
		"monetized_units",
		thisData.UnitsColumnPresent && prevData.UnitsColumnPresent && availabilityReason == "",
		thisData.MonetizedUnitsTotal,
		prevData.MonetizedUnitsTotal,
		resolveSalesReason("monetized units", availabilityReason, thisData.UnitsColumnPresent, prevData.UnitsColumnPresent),
	))
	metrics = append(metrics, countMetricFromOptionalTotals(
		"units",
		thisData.UnitsColumnPresent && prevData.UnitsColumnPresent && availabilityReason == "",
		thisData.UnitsTotal,
		prevData.UnitsTotal,
		resolveSalesReason("units", availabilityReason, thisData.UnitsColumnPresent, prevData.UnitsColumnPresent),
	))
	metrics = append(metrics, weeklyCurrencyMetric(
		"developer_proceeds",
		"developer proceeds totals",
		developerProceedsAvailable(thisData) && developerProceedsAvailable(prevData) && availabilityReason == "",
		thisData.DeveloperProceedsByCurrency,
		prevData.DeveloperProceedsByCurrency,
		resolveSalesReason("developer proceeds", availabilityReason, developerProceedsAvailable(thisData), developerProceedsAvailable(prevData)),
	))
	metrics = append(metrics, weeklyCurrencyMetric(
		"customer_price",
		"customer price totals",
		customerPriceAvailable(thisData) && customerPriceAvailable(prevData) && availabilityReason == "",
		thisData.CustomerPriceByCurrency,
		prevData.CustomerPriceByCurrency,
		resolveSalesReason("customer price", availabilityReason, customerPriceAvailable(thisData), customerPriceAvailable(prevData)),
	))

	metrics = append(metrics, countMetricFromOptionalTotals(
		"report_rows",
		availabilityReason == "",
		float64(thisData.RowCount),
		float64(prevData.RowCount),
		availabilityReason,
	))
	metrics = append(metrics, unavailableMetric("active_devices", "count", "not derivable from sales summary exports"))

	return metrics
}

func salesSummaryReportParams(vendor string, frequency asc.SalesReportFrequency, reportDate string) asc.SalesReportParams {
	return asc.SalesReportParams{
		VendorNumber:  vendor,
		ReportType:    asc.SalesReportTypeSales,
		ReportSubType: asc.SalesReportSubTypeSummary,
		Frequency:     frequency,
		ReportDate:    reportDate,
		Version:       asc.SalesReportVersion1_0,
	}
}

// fetchAppSalesReports overlaps the app SKU lookup with both report downloads.
// App lookup errors keep precedence; each report keeps its own error.
func fetchAppSalesReports(ctx context.Context, client *asc.Client, appID string, reports [2]asc.SalesReportParams) (salesScope, [2]salesWeekMetrics, [2]error, error) {
	readCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	var (
		scope   salesScope
		metrics [2]salesWeekMetrics
		errs    [2]error
		workers sync.WaitGroup
	)
	scopeReady := make(chan struct{})
	for i, params := range reports {
		workers.Go(func() {
			download, err := client.GetSalesReport(readCtx, params)
			if err != nil {
				errs[i] = err
				return
			}
			defer download.Body.Close()
			select {
			case <-scopeReady:
				metrics[i], errs[i] = ParseSalesReportMetrics(download.Body, scope)
			case <-readCtx.Done():
				errs[i] = readCtx.Err()
			}
		})
	}
	appResp, err := client.GetApp(ctx, appID)
	if err != nil {
		cancel()
		workers.Wait()
		return salesScope{}, metrics, errs, err
	}
	scope = salesScope{
		AppID:  appID,
		AppSKU: strings.TrimSpace(appResp.Data.Attributes.SKU),
	}
	close(scopeReady)
	workers.Wait()
	return scope, metrics, errs, nil
}

// ParseSalesReportMetrics reads a gzip-compressed sales TSV from reader and
// returns aggregated metrics scoped to the given app.
func ParseSalesReportMetrics(reader io.Reader, scope salesScope) (salesWeekMetrics, error) {
	gzipReader, err := gzip.NewReader(reader)
	if err != nil {
		return salesWeekMetrics{}, fmt.Errorf("read gzip report: %w", err)
	}
	defer gzipReader.Close()

	tsvReader := csv.NewReader(gzipReader)
	tsvReader.Comma = '\t'
	tsvReader.FieldsPerRecord = -1
	tsvReader.LazyQuotes = true

	if strings.TrimSpace(scope.AppSKU) != "" {
		tsvReader.ReuseRecord = true
		headers, err := tsvReader.Read()
		if err == io.EOF {
			return salesWeekMetrics{}, fmt.Errorf("report is empty")
		}
		if err != nil {
			return salesWeekMetrics{}, fmt.Errorf("parse report rows: %w", err)
		}
		accumulator, headerErr := newSalesReportAccumulator(headers, scope)
		for {
			row, err := tsvReader.Read()
			if err == io.EOF {
				break
			}
			if err != nil {
				return salesWeekMetrics{}, fmt.Errorf("parse report rows: %w", err)
			}
			if headerErr == nil {
				accumulator.add(row)
			}
		}
		// Read the entire stream before reporting header errors, matching the
		// buffered path's CSV and gzip checksum error precedence.
		if headerErr != nil {
			return salesWeekMetrics{}, headerErr
		}
		return accumulator.metrics, nil
	}

	// A missing SKU may be discovered after linked IAP rows. Retain the
	// full report so enrichment precedes aggregation for this fallback.
	rows, err := tsvReader.ReadAll()
	if err != nil {
		return salesWeekMetrics{}, fmt.Errorf("parse report rows: %w", err)
	}
	if len(rows) == 0 {
		return salesWeekMetrics{}, fmt.Errorf("report is empty")
	}
	accumulator, err := newSalesReportAccumulator(rows[0], scope)
	if err != nil {
		return salesWeekMetrics{}, err
	}
	accumulator.scope = EnrichSalesScopeFromRows(scope, rows[1:], accumulator.appleIdentifierIdx, accumulator.skuIdx)
	for _, row := range rows[1:] {
		accumulator.add(row)
	}
	return accumulator.metrics, nil
}

// salesReportAccumulator retains column indexes and totals, never input rows.
// Both parser paths use the same ordered aggregation rules.
type salesReportAccumulator struct {
	scope                    salesScope
	metrics                  salesWeekMetrics
	appleIdentifierIdx       int
	parentIdentifierIdx      int
	skuIdx                   int
	productTypeIdentifierIdx int
	subscriptionIdx          int
	unitsIdx                 int
	developerProceedsIdx     int
	proceedsCurrencyIdx      int
	customerPriceIdx         int
	customerCurrencyIdx      int
}

func newSalesReportAccumulator(headers []string, scope salesScope) (salesReportAccumulator, error) {
	a := salesReportAccumulator{scope: scope}
	a.appleIdentifierIdx = findColumnIndex(headers, "appleidentifier")
	a.parentIdentifierIdx = findColumnIndex(headers, "parentidentifier")
	a.skuIdx = findColumnIndex(headers, "sku")
	a.productTypeIdentifierIdx = findColumnIndex(headers, "producttypeidentifier")
	a.subscriptionIdx = findColumnIndex(headers, "subscription")
	a.unitsIdx = findColumnIndex(headers, "units")
	a.developerProceedsIdx = findColumnIndex(headers, "developerproceeds")
	a.proceedsCurrencyIdx = findColumnIndex(headers, "currencyofproceeds")
	a.customerPriceIdx = findColumnIndex(headers, "customerprice")
	a.customerCurrencyIdx = findColumnIndex(headers, "customercurrency")
	if a.appleIdentifierIdx < 0 && a.parentIdentifierIdx < 0 {
		return a, fmt.Errorf("report is missing Apple Identifier and Parent Identifier columns")
	}

	a.metrics = salesWeekMetrics{
		UnitsColumnPresent:             a.unitsIdx >= 0,
		DownloadUnitsAvailable:         a.unitsIdx >= 0 && a.productTypeIdentifierIdx >= 0,
		DeveloperProceedsColumnPresent: a.developerProceedsIdx >= 0,
		CustomerPriceColumnPresent:     a.customerPriceIdx >= 0,
		SubscriptionColumnPresent:      a.subscriptionIdx >= 0,
	}
	return a, nil
}

func (a *salesReportAccumulator) add(row []string) {
	metrics := &a.metrics
	if isEmptyRow(row) {
		return
	}

	appleIdentifier := strings.TrimSpace(valueAtIndex(row, a.appleIdentifierIdx))
	parentIdentifier := strings.TrimSpace(valueAtIndex(row, a.parentIdentifierIdx))
	isAppRow, isMonetizedRow, include := RowMatchesSalesScope(a.scope, appleIdentifier, parentIdentifier)
	if !include {
		return
	}
	subscriptionValue := strings.TrimSpace(valueAtIndex(row, a.subscriptionIdx))
	isSubscriptionRow := subscriptionValue != ""
	isRenewalRow := isRenewalSubscriptionState(subscriptionValue)

	metrics.RowCount++
	if isSubscriptionRow {
		metrics.SubscriptionRows++
	}
	if isRenewalRow {
		metrics.RenewalRows++
	}

	units, unitsOK := 0.0, false
	if a.unitsIdx >= 0 {
		if value, ok := parseNumericValue(valueAtIndex(row, a.unitsIdx)); ok {
			units, unitsOK = value, true
			metrics.UnitsTotal += value
			if isAppRow && isInitialAppDownloadProductType(valueAtIndex(row, a.productTypeIdentifierIdx)) {
				metrics.DownloadUnitsTotal += value
			}
			if isMonetizedRow {
				metrics.MonetizedUnitsTotal += value
			}
			if isSubscriptionRow {
				metrics.SubscriptionUnitsTotal += value
			}
			if isRenewalRow {
				metrics.RenewalUnitsTotal += value
			}
		}
	}
	if !unitsOK {
		// Money columns are per unit, so a row without units has no amount.
		return
	}
	if a.developerProceedsIdx >= 0 {
		if perUnit, ok := parseNumericValue(valueAtIndex(row, a.developerProceedsIdx)); ok {
			amount := salesRowAmount(units, perUnit)
			currency := normalizeCurrencyCode(valueAtIndex(row, a.proceedsCurrencyIdx))
			metrics.DeveloperProceedsByCurrency.add(currency, amount)
			if isSubscriptionRow {
				metrics.SubscriptionDeveloperProceedsByCurrency.add(currency, amount)
			}
			if isRenewalRow {
				metrics.RenewalDeveloperProceedsByCurrency.add(currency, amount)
			}
		}
	}
	if a.customerPriceIdx >= 0 {
		if perUnit, ok := parseNumericValue(valueAtIndex(row, a.customerPriceIdx)); ok {
			currency := normalizeCurrencyCode(valueAtIndex(row, a.customerCurrencyIdx))
			metrics.CustomerPriceByCurrency.add(currency, salesRowAmount(units, perUnit))
		}
	}
}

// salesRowAmount turns a per-unit money column into the row's amount. Apple
// reports Developer Proceeds and Customer Price per unit, and marks refunds
// with negative Units: refund rows keep a positive Developer Proceeds but a
// negative Customer Price. Units therefore carries the sign for both columns.
// https://developer.apple.com/help/app-store-connect/reference/reporting/summary-sales-report/
func salesRowAmount(units, perUnit float64) float64 {
	return units * math.Abs(perUnit)
}

func normalizeCurrencyCode(value string) string {
	return strings.ToUpper(strings.TrimSpace(value))
}

func developerProceedsAvailable(metrics salesWeekMetrics) bool {
	return metrics.UnitsColumnPresent && metrics.DeveloperProceedsColumnPresent
}

func customerPriceAvailable(metrics salesWeekMetrics) bool {
	return metrics.UnitsColumnPresent && metrics.CustomerPriceColumnPresent
}

func isInitialAppDownloadProductType(value string) bool {
	// Apple defines these as first-time app or app-bundle purchases. Redownload
	// and update product types are intentionally excluded from download units.
	// https://developer.apple.com/help/app-store-connect/reference/reporting/product-type-identifiers/
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case "1", "1-B", "1E", "1EP", "1EU", "1F", "1T", "F1", "F1-B":
		return true
	default:
		return false
	}
}

// EnrichSalesScopeFromRows resolves the app SKU from report data when it is
// not already set on scope.
func EnrichSalesScopeFromRows(scope salesScope, rows [][]string, appleIdentifierIdx, skuIdx int) salesScope {
	if strings.TrimSpace(scope.AppSKU) != "" {
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
		if appleIdentifier != strings.TrimSpace(scope.AppID) {
			continue
		}
		sku := strings.TrimSpace(valueAtIndex(row, skuIdx))
		if sku != "" {
			scope.AppSKU = sku
			return scope
		}
	}
	return scope
}

// RowMatchesSalesScope determines whether a report row belongs to the app,
// represents a monetized product, or should be included in aggregation.
func RowMatchesSalesScope(scope salesScope, appleIdentifier, parentIdentifier string) (isAppRow bool, isMonetizedRow bool, include bool) {
	appID := strings.TrimSpace(scope.AppID)
	appSKU := strings.TrimSpace(scope.AppSKU)
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
	requestsResp, err := fetchAllAnalyticsReportRequests(ctx, client, appID)
	if err != nil {
		if isLikelyForbidden(err) {
			return analyticsUnavailableMetrics("analytics source is not permitted for the current API key"), 0, nil
		}
		if isLikelyNotFound(err) {
			return analyticsUnavailableMetrics("analytics data is unavailable for this app"), 0, nil
		}
		return nil, 0, err
	}

	activeRequests := make([]asc.AnalyticsReportRequestResource, 0, len(requestsResp.Data))
	for _, request := range requestsResp.Data {
		if analyticsReportRequestIsActive(request.Attributes) {
			activeRequests = append(activeRequests, request)
		}
	}

	requestCount := len(activeRequests)
	if requestCount == 0 {
		return analyticsUnavailableMetrics("no active analytics report requests found"), 0, nil
	}

	var (
		thisInstances int
		lastInstances int
	)
	thisReportIDs := make(map[string]struct{})
	lastReportIDs := make(map[string]struct{})
	processingDates := weekWindowProcessingDates(thisWeek, previousWeek)

	for _, request := range activeRequests {
		reportsResp, reportsErr := fetchAllAnalyticsReports(ctx, client, request.ID)
		if reportsErr != nil {
			if isLikelyForbidden(reportsErr) {
				return analyticsUnavailableMetrics("analytics report metadata endpoints are not permitted for the current API key"), requestCount, nil
			}
			if isLikelyNotFound(reportsErr) {
				return analyticsUnavailableMetrics("analytics report metadata is unavailable for this app"), requestCount, nil
			}
			return nil, requestCount, reportsErr
		}

		reportInstances, instancesErr := fetchAnalyticsReportsInstances(
			ctx,
			client,
			reportsResp.Data,
			asc.WithAnalyticsReportInstancesProcessingDates(processingDates),
			asc.WithAnalyticsReportInstancesGranularities(analyticsInstanceGranularities),
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

		for index, report := range reportsResp.Data {
			for _, instance := range reportInstances[index] {
				processingDate, ok := parseDateValue(instance.Attributes.ProcessingDate)
				if !ok {
					continue
				}
				if containsDate(thisWeek, processingDate) {
					thisInstances++
					thisReportIDs[report.ID] = struct{}{}
				}
				if containsDate(previousWeek, processingDate) {
					lastInstances++
					lastReportIDs[report.ID] = struct{}{}
				}
			}
		}
	}

	metrics := []weeklyMetric{
		unavailableMetric("completed_requests", "count", "request state and creation date are not exposed by the current API"),
		comparableMetric("reports_available", "count", float64(len(thisReportIDs)), float64(len(lastReportIDs))),
		comparableMetric("instances_available", "count", float64(thisInstances), float64(lastInstances)),
		unavailableMetric("business_conversion_rate", "percent", "not derivable from analytics metadata alone"),
	}
	return metrics, requestCount, nil
}

func fetchAllAnalyticsReportRequests(ctx context.Context, client *asc.Client, appID string) (*asc.AnalyticsReportRequestsResponse, error) {
	first, err := client.GetAnalyticsReportRequests(ctx, appID, asc.Limit[asc.AnalyticsReportRequestsOption](200))
	if err != nil {
		return nil, err
	}

	paginated, err := asc.PaginateAll(ctx, first, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
		return client.GetAnalyticsReportRequests(ctx, appID, asc.NextURL[asc.AnalyticsReportRequestsOption](nextURL))
	})
	if err != nil {
		return nil, err
	}
	requests, ok := paginated.(*asc.AnalyticsReportRequestsResponse)
	if !ok || requests == nil {
		return nil, fmt.Errorf("insights: unexpected analytics report requests pagination response %T", paginated)
	}
	return requests, nil
}

func fetchAllAnalyticsReports(ctx context.Context, client *asc.Client, requestID string) (*asc.AnalyticsReportsResponse, error) {
	first, err := client.GetAnalyticsReports(ctx, requestID, asc.Limit[asc.AnalyticsReportsOption](200))
	if err != nil {
		return nil, err
	}

	paginated, err := asc.PaginateAll(ctx, first, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
		return client.GetAnalyticsReports(ctx, requestID, asc.NextURL[asc.AnalyticsReportsOption](nextURL))
	})
	if err != nil {
		return nil, err
	}
	reports, ok := paginated.(*asc.AnalyticsReportsResponse)
	if !ok || reports == nil {
		return nil, fmt.Errorf("insights: unexpected analytics reports pagination response %T", paginated)
	}
	return reports, nil
}

// weekWindowProcessingDates lists every processing date covered by the given
// comparison windows so App Store Connect filters instances server side.
func weekWindowProcessingDates(windows ...reportWeekWindow) []string {
	dates := make([]string, 0, len(windows)*7)
	seen := make(map[string]struct{}, len(windows)*7)
	for _, window := range windows {
		for day := window.start; !day.After(window.end); day = day.AddDate(0, 0, 1) {
			formatted := day.Format("2006-01-02")
			if _, exists := seen[formatted]; exists {
				continue
			}
			seen[formatted] = struct{}{}
			dates = append(dates, formatted)
		}
	}
	return dates
}

// fetchAnalyticsReportsInstances fetches each report's instances through a
// bounded pool. The first error in report order wins, matching a serial walk.
func fetchAnalyticsReportsInstances(
	ctx context.Context,
	client *asc.Client,
	reports []asc.Resource[asc.AnalyticsReportAttributes],
	opts ...asc.AnalyticsReportInstancesOption,
) ([][]asc.Resource[asc.AnalyticsReportInstanceAttributes], error) {
	type result struct {
		instances []asc.Resource[asc.AnalyticsReportInstanceAttributes]
		err       error
	}
	workCtx, cancel := context.WithCancel(ctx)
	var workers sync.WaitGroup
	defer func() {
		cancel()
		workers.Wait()
	}()

	results := make([]chan result, len(reports))
	for index := range results {
		results[index] = make(chan result, 1)
	}
	jobs := make(chan int)
	workers.Go(func() {
		defer close(jobs)
		for index := range reports {
			jobs <- index
		}
	})
	for range min(analyticsInstanceFetchConcurrency, len(reports)) {
		workers.Go(func() {
			for index := range jobs {
				if err := workCtx.Err(); err != nil {
					results[index] <- result{err: err}
					continue
				}
				instances, err := fetchAnalyticsReportInstances(workCtx, client, reports[index].ID, opts...)
				results[index] <- result{instances: instances, err: err}
			}
		})
	}

	instances := make([][]asc.Resource[asc.AnalyticsReportInstanceAttributes], len(reports))
	for index := range reports {
		result := <-results[index]
		if result.err != nil {
			return nil, result.err
		}
		instances[index] = result.instances
	}
	return instances, nil
}

// fetchAnalyticsReportInstances returns every instance page for a report so
// weekly counts are not truncated when daily and weekly instances share pages.
func fetchAnalyticsReportInstances(
	ctx context.Context,
	client *asc.Client,
	reportID string,
	opts ...asc.AnalyticsReportInstancesOption,
) ([]asc.Resource[asc.AnalyticsReportInstanceAttributes], error) {
	var (
		instances []asc.Resource[asc.AnalyticsReportInstanceAttributes]
		next      string
	)
	seen := make(map[string]struct{})

	for {
		var (
			resp *asc.AnalyticsReportInstancesResponse
			err  error
		)
		if next == "" {
			pageOpts := make([]asc.AnalyticsReportInstancesOption, 0, len(opts)+1)
			pageOpts = append(pageOpts, asc.Limit[asc.AnalyticsReportInstancesOption](analyticsInstancePageLimit))
			pageOpts = append(pageOpts, opts...)
			resp, err = client.GetAnalyticsReportInstances(ctx, reportID, pageOpts...)
		} else {
			identity := asc.PaginationURLIdentity(next)
			if _, repeated := seen[identity]; repeated {
				return nil, fmt.Errorf("insights: detected repeated analytics report instance pagination URL")
			}
			seen[identity] = struct{}{}
			resp, err = client.GetAnalyticsReportInstances(ctx, reportID, asc.NextURL[asc.AnalyticsReportInstancesOption](next))
		}
		if err != nil {
			return nil, err
		}

		instances = append(instances, resp.Data...)
		if resp.Links.Next == "" {
			return instances, nil
		}
		next = resp.Links.Next
	}
}

func analyticsReportRequestIsActive(attributes asc.AnalyticsReportRequestAttributes) bool {
	return attributes.StoppedDueToInactivity == nil || !*attributes.StoppedDueToInactivity
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

func countMetricFromOptionalTotals(name string, available bool, thisValue, lastValue float64, reason string) weeklyMetric {
	if !available {
		return unavailableMetric(name, "count", reason)
	}
	return comparableMetric(name, "count", thisValue, lastValue)
}

func dailyCountMetricFromOptionalTotals(name string, available bool, thisValue, previousValue float64, reason string) dailyMetric {
	if !available {
		return unavailableDailyMetric(name, "count", reason)
	}
	return comparableDailyMetric(name, "count", thisValue, previousValue)
}

// weeklyCurrencyMetric reports a monetary total only when every amount in both
// weeks is in one currency; otherwise the totals stay split in ByCurrency.
func weeklyCurrencyMetric(name, label string, available bool, thisWeek, lastWeek CurrencyAmounts, reason string) weeklyMetric {
	if !available {
		return unavailableMetric(name, "currency", reason)
	}
	comparisons := CompareCurrencyAmounts(thisWeek, lastWeek)
	breakdown := make([]weeklyCurrencyAmount, 0, len(comparisons))
	for _, comparison := range comparisons {
		compared := comparableMetric(name, "currency", comparison.Current, comparison.Previous)
		breakdown = append(breakdown, weeklyCurrencyAmount{
			Currency:     comparison.Currency,
			ThisWeek:     comparison.Current,
			LastWeek:     comparison.Previous,
			Delta:        *compared.Delta,
			DeltaPercent: compared.DeltaPercent,
		})
	}

	switch len(comparisons) {
	case 0:
		return comparableMetric(name, "currency", 0, 0)
	case 1:
		metric := comparableMetric(name, "currency", comparisons[0].Current, comparisons[0].Previous)
		if comparisons[0].Currency != "" {
			metric.Currency = comparisons[0].Currency
			metric.ByCurrency = breakdown
		}
		return metric
	default:
		metric := unavailableMetric(name, "currency", MixedCurrencyReason(label, comparisons))
		metric.ByCurrency = breakdown
		return metric
	}
}

// dailyCurrencyMetric is weeklyCurrencyMetric for day-over-day metrics.
func dailyCurrencyMetric(name, label string, available bool, thisDay, previousDay CurrencyAmounts, reason string) dailyMetric {
	if !available {
		return unavailableDailyMetric(name, "currency", reason)
	}
	comparisons := CompareCurrencyAmounts(thisDay, previousDay)
	breakdown := make([]dailyCurrencyAmount, 0, len(comparisons))
	for _, comparison := range comparisons {
		compared := comparableDailyMetric(name, "currency", comparison.Current, comparison.Previous)
		breakdown = append(breakdown, dailyCurrencyAmount{
			Currency:     comparison.Currency,
			ThisDay:      comparison.Current,
			PreviousDay:  comparison.Previous,
			Delta:        *compared.Delta,
			DeltaPercent: compared.DeltaPercent,
		})
	}

	switch len(comparisons) {
	case 0:
		return comparableDailyMetric(name, "currency", 0, 0)
	case 1:
		metric := comparableDailyMetric(name, "currency", comparisons[0].Current, comparisons[0].Previous)
		if comparisons[0].Currency != "" {
			metric.Currency = comparisons[0].Currency
			metric.ByCurrency = breakdown
		}
		return metric
	default:
		metric := unavailableDailyMetric(name, "currency", MixedCurrencyReason(label, comparisons))
		metric.ByCurrency = breakdown
		return metric
	}
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

func renderWeeklyInsights(d *shared.Deps, resp *weeklyInsightsResponse, markdown bool) {
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
	d.RenderSection("Context", []string{"field", "value"}, contextRows, markdown)

	metricRows := make([][]string, 0, len(resp.Metrics))
	for _, metric := range resp.Metrics {
		metricRows = append(metricRows, []string{
			metric.Name,
			shared.OrNA(metricUnitLabel(metric.Unit, metric.Currency)),
			formatOptionalNumber(metric.ThisWeek),
			formatOptionalNumber(metric.LastWeek),
			formatOptionalNumber(metric.Delta),
			formatOptionalNumber(metric.DeltaPercent),
			metric.Status,
			shared.OrNA(metric.Reason),
		})
		if metric.Status == "ok" {
			continue // The metric row already shows its single currency.
		}
		for _, amount := range metric.ByCurrency {
			metricRows = append(metricRows, []string{
				metric.Name,
				CurrencyLabel(amount.Currency),
				formatOptionalNumber(&amount.ThisWeek),
				formatOptionalNumber(&amount.LastWeek),
				formatOptionalNumber(&amount.Delta),
				formatOptionalNumber(amount.DeltaPercent),
				"ok",
				shared.OrNA(""),
			})
		}
	}
	d.RenderSection("Metrics", []string{"metric", "unit", "thisWeek", "lastWeek", "delta", "deltaPercent", "status", "reason"}, metricRows, markdown)
}

func renderDailyInsights(d *shared.Deps, resp *dailyInsightsResponse, markdown bool) {
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
	d.RenderSection("Context", []string{"field", "value"}, contextRows, markdown)

	metricRows := make([][]string, 0, len(resp.Metrics))
	for _, metric := range resp.Metrics {
		metricRows = append(metricRows, []string{
			metric.Name,
			shared.OrNA(metricUnitLabel(metric.Unit, metric.Currency)),
			formatOptionalNumber(metric.ThisDay),
			formatOptionalNumber(metric.PreviousDay),
			formatOptionalNumber(metric.Delta),
			formatOptionalNumber(metric.DeltaPercent),
			metric.Status,
			shared.OrNA(metric.Reason),
		})
		if metric.Status == "ok" {
			continue // The metric row already shows its single currency.
		}
		for _, amount := range metric.ByCurrency {
			metricRows = append(metricRows, []string{
				metric.Name,
				CurrencyLabel(amount.Currency),
				formatOptionalNumber(&amount.ThisDay),
				formatOptionalNumber(&amount.PreviousDay),
				formatOptionalNumber(&amount.Delta),
				formatOptionalNumber(amount.DeltaPercent),
				"ok",
				shared.OrNA(""),
			})
		}
	}
	d.RenderSection("Metrics", []string{"metric", "unit", "thisDay", "previousDay", "delta", "deltaPercent", "status", "reason"}, metricRows, markdown)
}

// metricUnitLabel shows the currency code as a monetary metric's unit when
// tables and markdown render a single-currency total.
func metricUnitLabel(unit, currency string) string {
	if currency != "" {
		return currency
	}
	return unit
}

func formatOptionalNumber(value *float64) string {
	if value == nil {
		return "n/a"
	}
	return fmt.Sprintf("%.2f", *value)
}
