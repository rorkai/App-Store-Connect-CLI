package analytics

import (
	"context"
	"flag"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/insights"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

type compareResponse struct {
	AppID       string          `json:"appId"`
	Source      compareSource   `json:"source"`
	Baseline    comparePeriod   `json:"baseline"`
	Comparison  comparePeriod   `json:"comparison"`
	Metrics     []compareMetric `json:"metrics"`
	GeneratedAt string          `json:"generatedAt"`
}

type compareSource struct {
	Name          string `json:"name"`
	VendorNumber  string `json:"vendorNumber,omitempty"`
	AppSKU        string `json:"appSku,omitempty"`
	ReportType    string `json:"reportType,omitempty"`
	ReportSubType string `json:"reportSubType,omitempty"`
	Frequency     string `json:"frequency,omitempty"`
}

type comparePeriod struct {
	Start        string `json:"start"`
	End          string `json:"end"`
	ReportsFound int    `json:"reportsFound"`
}

type compareMetric struct {
	Name         string                  `json:"name"`
	Unit         string                  `json:"unit,omitempty"`
	Currency     string                  `json:"currency,omitempty"`
	Baseline     *float64                `json:"baseline,omitempty"`
	Comparison   *float64                `json:"comparison,omitempty"`
	Delta        *float64                `json:"delta,omitempty"`
	DeltaPercent *float64                `json:"deltaPercent,omitempty"`
	Status       string                  `json:"status"`
	Reason       string                  `json:"reason,omitempty"`
	ByCurrency   []compareCurrencyAmount `json:"byCurrency,omitempty"`
}

// compareCurrencyAmount is one currency's share of a monetary compare metric.
type compareCurrencyAmount struct {
	Currency     string   `json:"currency"`
	Baseline     float64  `json:"baseline"`
	Comparison   float64  `json:"comparison"`
	Delta        float64  `json:"delta"`
	DeltaPercent *float64 `json:"deltaPercent,omitempty"`
}

// newAnalyticsCompareCommand returns the compare subcommand for analytics.
func newAnalyticsCompareCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("compare", flag.ExitOnError)

	source := fs.String("source", "", "Data source: sales")
	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	vendor := fs.String("vendor", "", "Vendor number (or ASC_VENDOR_NUMBER env)")
	from := fs.String("from", "", "Baseline period start (YYYY-MM-DD, YYYY-MM, or YYYY)")
	fromEnd := fs.String("from-end", "", "Baseline period end (defaults to --from)")
	to := fs.String("to", "", "Comparison period start")
	toEnd := fs.String("to-end", "", "Comparison period end (defaults to --to)")
	frequency := fs.String("frequency", "", "Report frequency: DAILY, WEEKLY, MONTHLY, YEARLY")
	reportType := fs.String("type", "SALES", "Report type: SALES, PRE_ORDER, NEWSSTAND, SUBSCRIPTION, SUBSCRIPTION_EVENT")
	reportSubType := fs.String("subtype", "SUMMARY", "Report subtype: SUMMARY, DETAILED")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "compare",
		ShortUsage: "asc analytics compare [flags]",
		ShortHelp:  "Compare sales metrics between two date ranges.",
		LongHelp: `Compare sales metrics between two date ranges.

Fetches reports for each date in both ranges, aggregates metrics, and produces
delta and percentage-change output scoped to the specified app.

For WEEKLY frequency, each boundary must be a Monday (week start) or Sunday
(week end). All requested report dates in both ranges must be available and
parse successfully before comparison output is produced.

Monetary metrics multiply the report's per-unit amounts by Units, so refunds reduce them.
They are totaled per currency and never converted: a metric has one total only when every
amount shares a currency, otherwise it is unavailable and byCurrency lists each currency.
Totals are in whole cents. A paid row with a blank currency makes its metric unavailable.

Examples:
  asc analytics compare --source sales --vendor "12345678" --app "123456789" --from "2026-01-01" --to "2026-02-01" --frequency DAILY
  asc analytics compare --source sales --vendor "12345678" --app "123456789" --from "2026-01" --to "2026-02" --frequency MONTHLY --output table
  asc analytics compare --source sales --vendor "12345678" --app "123456789" --from "2026-01-01" --from-end "2026-01-31" --to "2026-02-01" --to-end "2026-02-28" --frequency DAILY --output json --pretty`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			sourceName := strings.ToLower(strings.TrimSpace(*source))
			if sourceName == "" {
				return d.UsageError("--source is required")
			}
			if sourceName != "sales" {
				return d.UsageError("--source must be sales")
			}

			resolvedAppID := d.ResolveAppID(*appID)
			if resolvedAppID == "" {
				return d.UsageError("--app is required (or set ASC_APP_ID)")
			}

			resolvedVendor := d.ResolveVendorNumber(*vendor)
			if resolvedVendor == "" {
				return d.UsageError("--vendor is required (or set ASC_VENDOR_NUMBER)")
			}

			if strings.TrimSpace(*from) == "" {
				return d.UsageError("--from is required")
			}
			if strings.TrimSpace(*to) == "" {
				return d.UsageError("--to is required")
			}
			if strings.TrimSpace(*frequency) == "" {
				return d.UsageError("--frequency is required")
			}

			freq, err := normalizeSalesReportFrequency(*frequency)
			if err != nil {
				return d.UsageError(err.Error())
			}

			salesType, err := normalizeAnalyticsCompareReportType(*reportType)
			if err != nil {
				return d.UsageError(err.Error())
			}
			subType, err := normalizeAnalyticsCompareReportSubType(*reportSubType)
			if err != nil {
				return d.UsageError(err.Error())
			}
			if err := validateSalesReportTuple(salesType, subType, freq); err != nil {
				return d.UsageError(err.Error())
			}

			baselineStart, baselineEnd, err := normalizeCompareDateRange(*from, *fromEnd, freq, "--from", "--from-end")
			if err != nil {
				return d.UsageError(err.Error())
			}

			compStart, compEnd, err := normalizeCompareDateRange(*to, *toEnd, freq, "--to", "--to-end")
			if err != nil {
				return d.UsageError(err.Error())
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("analytics compare: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			appResp, err := client.GetApp(requestCtx, resolvedAppID)
			if err != nil {
				return fmt.Errorf("analytics compare: %w", err)
			}
			scope := insights.SalesScope{
				AppID:  resolvedAppID,
				AppSKU: strings.TrimSpace(appResp.Data.Attributes.SKU),
			}

			baselineDates, err := generateReportDates(baselineStart, baselineEnd, freq)
			if err != nil {
				return fmt.Errorf("analytics compare: %w", err)
			}
			compDates, err := generateReportDates(compStart, compEnd, freq)
			if err != nil {
				return fmt.Errorf("analytics compare: %w", err)
			}

			periods := fetchAndAggregate(requestCtx, client, resolvedVendor, scope, [][]string{baselineDates, compDates}, salesType, subType, freq)
			baseline, comparison := periods[0], periods[1]
			if baseline.err != nil || comparison.err != nil {
				return fmt.Errorf("analytics compare: %s", joinCompareErrors(baseline.err, comparison.err))
			}

			resp := &compareResponse{
				AppID: resolvedAppID,
				Source: compareSource{
					Name:          sourceName,
					VendorNumber:  resolvedVendor,
					AppSKU:        scope.AppSKU,
					ReportType:    string(salesType),
					ReportSubType: string(subType),
					Frequency:     string(freq),
				},
				Baseline:    comparePeriod{Start: baselineStart, End: baselineEnd, ReportsFound: baseline.found},
				Comparison:  comparePeriod{Start: compStart, End: compEnd, ReportsFound: comparison.found},
				Metrics:     buildCompareMetrics(baseline.metrics, comparison.metrics),
				GeneratedAt: d.Now().UTC().Format(time.RFC3339),
			}

			return d.PrintOutputWithRenderers(
				resp,
				*output.Output,
				*output.Pretty,
				func() error { renderCompareOutput(d, resp, false); return nil },
				func() error { renderCompareOutput(d, resp, true); return nil },
			)
		},
	}
}

func normalizeAnalyticsCompareReportType(value string) (asc.SalesReportType, error) {
	reportType, err := normalizeSalesReportType(value)
	if err != nil {
		return "", err
	}
	switch reportType {
	case asc.SalesReportTypeSales,
		asc.SalesReportTypePreOrder,
		asc.SalesReportTypeNewsstand,
		asc.SalesReportTypeSubscription,
		asc.SalesReportTypeSubscriptionEvent:
		return reportType, nil
	default:
		return "", fmt.Errorf("--type for analytics compare must be SALES, PRE_ORDER, NEWSSTAND, SUBSCRIPTION, or SUBSCRIPTION_EVENT")
	}
}

func normalizeAnalyticsCompareReportSubType(value string) (asc.SalesReportSubType, error) {
	reportSubType, err := normalizeSalesReportSubType(value)
	if err != nil {
		return "", err
	}
	switch reportSubType {
	case asc.SalesReportSubTypeSummary, asc.SalesReportSubTypeDetailed:
		return reportSubType, nil
	default:
		return "", fmt.Errorf("--subtype for analytics compare must be SUMMARY or DETAILED")
	}
}

const salesReportFetchConcurrency = 4

type salesReportFetchResult struct {
	metrics insights.SalesMetrics
	err     error
}

type salesPeriodResult struct {
	metrics insights.SalesMetrics
	found   int
	err     error
}

// fetchAndAggregate downloads every period's reports through one bounded pool
// and reduces each period in date order. A period's error cancels only that
// period's remaining downloads.
func fetchAndAggregate(ctx context.Context, client *asc.Client, vendor string, scope insights.SalesScope, periods [][]string, salesType asc.SalesReportType, subType asc.SalesReportSubType, freq asc.SalesReportFrequency) []salesPeriodResult {
	type job struct{ period, index int }
	workCtx, cancel := context.WithCancel(ctx)
	var workers sync.WaitGroup
	defer func() {
		cancel()
		workers.Wait()
	}()

	periodCtxs := make([]context.Context, len(periods))
	results := make([][]chan salesReportFetchResult, len(periods))
	cancels := make([]context.CancelFunc, len(periods))
	total := 0
	for period, dates := range periods {
		periodCtxs[period], cancels[period] = context.WithCancel(workCtx)
		results[period] = make([]chan salesReportFetchResult, len(dates))
		for index := range dates {
			results[period][index] = make(chan salesReportFetchResult, 1)
		}
		total += len(dates)
	}
	jobs := make(chan job)
	workers.Go(func() {
		defer close(jobs)
		for period, dates := range periods {
			for index := range dates {
				select {
				case jobs <- job{period: period, index: index}:
				case <-workCtx.Done():
					return
				}
			}
		}
	})
	for range min(salesReportFetchConcurrency, total) {
		workers.Go(func() {
			for job := range jobs {
				periodCtx := periodCtxs[job.period]
				if periodCtx.Err() != nil {
					continue
				}
				date := periods[job.period][job.index]
				download, err := client.GetSalesReport(periodCtx, asc.SalesReportParams{
					VendorNumber:  vendor,
					ReportType:    salesType,
					ReportSubType: subType,
					Frequency:     freq,
					ReportDate:    date,
					Version:       defaultSalesReportVersion(salesType, subType, freq),
				})
				if err != nil {
					results[job.period][job.index] <- salesReportFetchResult{err: fmt.Errorf("download report %s: %w", date, err)}
					continue
				}
				metrics, parseErr := insights.ParseSalesReportMetrics(download.Body, scope)
				_ = download.Body.Close()
				if parseErr != nil {
					parseErr = fmt.Errorf("parse report %s: %w", date, parseErr)
				}
				results[job.period][job.index] <- salesReportFetchResult{metrics: metrics, err: parseErr}
			}
		})
	}

	reduced := make([]salesPeriodResult, len(periods))
	for period, dates := range periods {
		metrics, found, err := reduceSalesPeriod(ctx, dates, results[period])
		cancels[period]()
		reduced[period] = salesPeriodResult{metrics: metrics, found: found, err: err}
	}
	return reduced
}

func reduceSalesPeriod(ctx context.Context, dates []string, results []chan salesReportFetchResult) (insights.SalesMetrics, int, error) {
	var aggregate insights.SalesMetrics
	found := 0
	missingDates := make([]string, 0)
	// Reduce in date order so completion timing cannot change totals or errors.
	for index, date := range dates {
		var result salesReportFetchResult
		select {
		case result = <-results[index]:
		case <-ctx.Done():
			return aggregate, found, fmt.Errorf("download report %s: %w", date, ctx.Err())
		}
		if result.err != nil {
			if asc.IsNotFound(result.err) {
				missingDates = append(missingDates, date)
				continue
			}
			return aggregate, found, result.err
		}

		// Seed from the first parsed report so availability flags reflect real coverage.
		if found == 0 {
			aggregate = result.metrics
		} else {
			aggregate = aggregateSalesMetrics(aggregate, result.metrics)
		}
		found++
	}

	if found == 0 {
		if len(missingDates) > 0 {
			return aggregate, 0, fmt.Errorf("no reports available for the requested period (missing %s)", summarizeMissingReportDates(missingDates))
		}
		return aggregate, 0, fmt.Errorf("no reports available for the requested period")
	}
	if found != len(dates) {
		return aggregate, found, fmt.Errorf("requested range is incomplete: found %d of %d reports (missing %s)", found, len(dates), summarizeMissingReportDates(missingDates))
	}
	return aggregate, found, nil
}

func joinCompareErrors(baselineErr, compErr error) string {
	switch {
	case baselineErr != nil && compErr != nil:
		return fmt.Sprintf("baseline period: %v; comparison period: %v", baselineErr, compErr)
	case baselineErr != nil:
		return fmt.Sprintf("baseline period: %v", baselineErr)
	default:
		return fmt.Sprintf("comparison period: %v", compErr)
	}
}

func summarizeMissingReportDates(dates []string) string {
	switch len(dates) {
	case 0:
		return "no report dates"
	case 1:
		return dates[0]
	case 2:
		return fmt.Sprintf("%s and %s", dates[0], dates[1])
	case 3:
		return fmt.Sprintf("%s, %s, and %s", dates[0], dates[1], dates[2])
	default:
		return fmt.Sprintf("%s, %s, %s, and %d more", dates[0], dates[1], dates[2], len(dates)-3)
	}
}

func aggregateSalesMetrics(a, b insights.SalesMetrics) insights.SalesMetrics {
	return insights.SalesMetrics{
		RowCount:                                a.RowCount + b.RowCount,
		UnitsColumnPresent:                      a.UnitsColumnPresent && b.UnitsColumnPresent,
		DownloadUnitsAvailable:                  a.DownloadUnitsAvailable && b.DownloadUnitsAvailable,
		DeveloperProceedsColumnPresent:          a.DeveloperProceedsColumnPresent && b.DeveloperProceedsColumnPresent,
		CustomerPriceColumnPresent:              a.CustomerPriceColumnPresent && b.CustomerPriceColumnPresent,
		SubscriptionColumnPresent:               a.SubscriptionColumnPresent && b.SubscriptionColumnPresent,
		UnitsTotal:                              a.UnitsTotal + b.UnitsTotal,
		DownloadUnitsTotal:                      a.DownloadUnitsTotal + b.DownloadUnitsTotal,
		MonetizedUnitsTotal:                     a.MonetizedUnitsTotal + b.MonetizedUnitsTotal,
		DeveloperProceedsByCurrency:             a.DeveloperProceedsByCurrency.Plus(b.DeveloperProceedsByCurrency),
		CustomerPriceByCurrency:                 a.CustomerPriceByCurrency.Plus(b.CustomerPriceByCurrency),
		SubscriptionRows:                        a.SubscriptionRows + b.SubscriptionRows,
		SubscriptionUnitsTotal:                  a.SubscriptionUnitsTotal + b.SubscriptionUnitsTotal,
		SubscriptionDeveloperProceedsByCurrency: a.SubscriptionDeveloperProceedsByCurrency.Plus(b.SubscriptionDeveloperProceedsByCurrency),
		RenewalRows:                             a.RenewalRows + b.RenewalRows,
		RenewalUnitsTotal:                       a.RenewalUnitsTotal + b.RenewalUnitsTotal,
		RenewalDeveloperProceedsByCurrency:      a.RenewalDeveloperProceedsByCurrency.Plus(b.RenewalDeveloperProceedsByCurrency),

		DeveloperProceedsMissingCurrencyRows:             a.DeveloperProceedsMissingCurrencyRows + b.DeveloperProceedsMissingCurrencyRows,
		CustomerPriceMissingCurrencyRows:                 a.CustomerPriceMissingCurrencyRows + b.CustomerPriceMissingCurrencyRows,
		SubscriptionDeveloperProceedsMissingCurrencyRows: a.SubscriptionDeveloperProceedsMissingCurrencyRows + b.SubscriptionDeveloperProceedsMissingCurrencyRows,
		RenewalDeveloperProceedsMissingCurrencyRows:      a.RenewalDeveloperProceedsMissingCurrencyRows + b.RenewalDeveloperProceedsMissingCurrencyRows,
	}
}

func buildCompareMetrics(baseline, comparison insights.SalesMetrics) []compareMetric {
	metrics := []compareMetric{
		compareMetricFromValues("download_units", "count", "download units",
			baseline.DownloadUnitsTotal, comparison.DownloadUnitsTotal,
			baseline.DownloadUnitsAvailable, comparison.DownloadUnitsAvailable),
		compareMetricFromValues("monetized_units", "count", "monetized units",
			baseline.MonetizedUnitsTotal, comparison.MonetizedUnitsTotal,
			baseline.UnitsColumnPresent, comparison.UnitsColumnPresent),
		compareMetricFromValues("units", "count", "units",
			baseline.UnitsTotal, comparison.UnitsTotal,
			baseline.UnitsColumnPresent, comparison.UnitsColumnPresent),
		compareCurrencyMetric("developer_proceeds", "developer proceeds",
			baseline.DeveloperProceedsByCurrency, comparison.DeveloperProceedsByCurrency,
			baseline.UnitsColumnPresent && baseline.DeveloperProceedsColumnPresent,
			comparison.UnitsColumnPresent && comparison.DeveloperProceedsColumnPresent,
			insights.MissingCurrencyReason("developer proceeds", insights.ProceedsCurrencyColumn,
				insights.MissingCurrencyPeriod{Rows: baseline.DeveloperProceedsMissingCurrencyRows, Phrase: "in the baseline period"},
				insights.MissingCurrencyPeriod{Rows: comparison.DeveloperProceedsMissingCurrencyRows, Phrase: "in the comparison period"})),
		compareCurrencyMetric("customer_price", "customer price",
			baseline.CustomerPriceByCurrency, comparison.CustomerPriceByCurrency,
			baseline.UnitsColumnPresent && baseline.CustomerPriceColumnPresent,
			comparison.UnitsColumnPresent && comparison.CustomerPriceColumnPresent,
			insights.MissingCurrencyReason("customer price", insights.CustomerCurrencyColumn,
				insights.MissingCurrencyPeriod{Rows: baseline.CustomerPriceMissingCurrencyRows, Phrase: "in the baseline period"},
				insights.MissingCurrencyPeriod{Rows: comparison.CustomerPriceMissingCurrencyRows, Phrase: "in the comparison period"})),
		compareMetricFromValues("subscription_units", "count", "subscription units",
			baseline.SubscriptionUnitsTotal, comparison.SubscriptionUnitsTotal,
			baseline.SubscriptionColumnPresent && baseline.UnitsColumnPresent,
			comparison.SubscriptionColumnPresent && comparison.UnitsColumnPresent),
		compareMetricFromValues("renewal_units", "count", "renewal units",
			baseline.RenewalUnitsTotal, comparison.RenewalUnitsTotal,
			baseline.SubscriptionColumnPresent && baseline.UnitsColumnPresent,
			comparison.SubscriptionColumnPresent && comparison.UnitsColumnPresent),
		compareMetricFromValues("report_rows", "count", "report rows",
			float64(baseline.RowCount), float64(comparison.RowCount),
			true, true),
	}
	return metrics
}

func compareMetricAvailabilityReason(metricLabel string, baselineAvailable, comparisonAvailable bool) string {
	switch {
	case !baselineAvailable && !comparisonAvailable:
		return fmt.Sprintf("cannot derive %s for either period from the available report columns", metricLabel)
	case !baselineAvailable:
		return fmt.Sprintf("cannot derive %s for the baseline period from the available report columns", metricLabel)
	default:
		return fmt.Sprintf("cannot derive %s for the comparison period from the available report columns", metricLabel)
	}
}

func compareMetricFromValues(name, unit, metricLabel string, baselineValue, comparisonValue float64, baselineAvailable, comparisonAvailable bool) compareMetric {
	if !baselineAvailable || !comparisonAvailable {
		return compareMetric{
			Name:   name,
			Unit:   unit,
			Status: "unavailable",
			Reason: compareMetricAvailabilityReason(metricLabel, baselineAvailable, comparisonAvailable),
		}
	}
	delta := comparisonValue - baselineValue
	if unit == "currency" {
		delta = insights.RoundMoney(delta)
	}
	m := compareMetric{
		Name:       name,
		Unit:       unit,
		Baseline:   ptrFloat(baselineValue),
		Comparison: ptrFloat(comparisonValue),
		Delta:      ptrFloat(delta),
		Status:     "ok",
	}
	if baselineValue == 0 {
		if comparisonValue == 0 {
			m.DeltaPercent = ptrFloat(0)
		} else {
			m.Reason = "baseline is zero; percentage change is undefined"
		}
		return m
	}
	m.DeltaPercent = ptrFloat(delta / baselineValue * 100)
	return m
}

// compareCurrencyMetric reports a monetary total only when every amount in both
// periods is in one currency; otherwise the totals stay split in ByCurrency.
func compareCurrencyMetric(name, metricLabel string, baseline, comparison insights.CurrencyAmounts, baselineAvailable, comparisonAvailable bool, missingCurrencyReason string) compareMetric {
	if !baselineAvailable || !comparisonAvailable {
		return compareMetricFromValues(name, "currency", metricLabel, 0, 0, baselineAvailable, comparisonAvailable)
	}
	if missingCurrencyReason != "" {
		return compareMetric{Name: name, Unit: "currency", Status: "unavailable", Reason: missingCurrencyReason}
	}
	// Baseline is the earlier period, so it is the "previous" side.
	comparisons := insights.CompareCurrencyAmounts(comparison, baseline)
	breakdown := make([]compareCurrencyAmount, 0, len(comparisons))
	for _, c := range comparisons {
		compared := compareMetricFromValues(name, "currency", metricLabel, c.Previous, c.Current, true, true)
		breakdown = append(breakdown, compareCurrencyAmount{
			Currency:     c.Currency,
			Baseline:     c.Previous,
			Comparison:   c.Current,
			Delta:        *compared.Delta,
			DeltaPercent: compared.DeltaPercent,
		})
	}

	switch len(comparisons) {
	case 0:
		return compareMetricFromValues(name, "currency", metricLabel, 0, 0, true, true)
	case 1:
		m := compareMetricFromValues(name, "currency", metricLabel, comparisons[0].Previous, comparisons[0].Current, true, true)
		if comparisons[0].Currency != "" {
			m.Currency = comparisons[0].Currency
			m.ByCurrency = breakdown
		}
		return m
	default:
		return compareMetric{
			Name:       name,
			Unit:       "currency",
			Status:     "unavailable",
			Reason:     insights.MixedCurrencyReason(metricLabel+" totals", comparisons),
			ByCurrency: breakdown,
		}
	}
}

func ptrFloat(v float64) *float64 {
	return &v
}

func renderCompareOutput(d *shared.Deps, resp *compareResponse, markdown bool) {
	contextRows := [][]string{
		{"appId", resp.AppID},
		{"source", resp.Source.Name},
		{"baseline", fmt.Sprintf("%s to %s (%d reports)", resp.Baseline.Start, resp.Baseline.End, resp.Baseline.ReportsFound)},
		{"comparison", fmt.Sprintf("%s to %s (%d reports)", resp.Comparison.Start, resp.Comparison.End, resp.Comparison.ReportsFound)},
		{"frequency", resp.Source.Frequency},
		{"generatedAt", resp.GeneratedAt},
	}
	if strings.TrimSpace(resp.Source.VendorNumber) != "" {
		contextRows = append(contextRows, []string{"vendorNumber", resp.Source.VendorNumber})
	}
	if strings.TrimSpace(resp.Source.AppSKU) != "" {
		contextRows = append(contextRows, []string{"appSku", resp.Source.AppSKU})
	}
	d.RenderSection("Context", []string{"field", "value"}, contextRows, markdown)

	metricRows := make([][]string, 0, len(resp.Metrics))
	for _, metric := range resp.Metrics {
		unit := metric.Unit
		if metric.Currency != "" {
			unit = metric.Currency
		}
		metricRows = append(metricRows, []string{
			metric.Name,
			shared.OrNA(unit),
			formatOptionalFloat(metric.Baseline),
			formatOptionalFloat(metric.Comparison),
			formatOptionalFloat(metric.Delta),
			formatOptionalFloat(metric.DeltaPercent),
			metric.Status,
			shared.OrNA(metric.Reason),
		})
		if metric.Status == "ok" {
			continue // The metric row already shows its single currency.
		}
		for _, amount := range metric.ByCurrency {
			metricRows = append(metricRows, []string{
				metric.Name,
				insights.CurrencyLabel(amount.Currency),
				formatOptionalFloat(&amount.Baseline),
				formatOptionalFloat(&amount.Comparison),
				formatOptionalFloat(&amount.Delta),
				formatOptionalFloat(amount.DeltaPercent),
				"ok",
				shared.OrNA(""),
			})
		}
	}
	d.RenderSection("Metrics", []string{"metric", "unit", "baseline", "comparison", "delta", "deltaPercent", "status", "reason"}, metricRows, markdown)
}

func formatOptionalFloat(v *float64) string {
	if v == nil {
		return "n/a"
	}
	return fmt.Sprintf("%.2f", *v)
}
