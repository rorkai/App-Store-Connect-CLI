package analytics

import (
	"context"
	"flag"
	"fmt"
	"strings"
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
	Name         string   `json:"name"`
	Unit         string   `json:"unit,omitempty"`
	Baseline     *float64 `json:"baseline,omitempty"`
	Comparison   *float64 `json:"comparison,omitempty"`
	Delta        *float64 `json:"delta,omitempty"`
	DeltaPercent *float64 `json:"deltaPercent,omitempty"`
	Status       string   `json:"status"`
	Reason       string   `json:"reason,omitempty"`
}

// AnalyticsCompareCommand returns the compare subcommand for analytics.
func AnalyticsCompareCommand() *ffcli.Command {
	fs := flag.NewFlagSet("compare", flag.ExitOnError)

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
	output := shared.BindOutputFlags(fs)

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

Examples:
  asc analytics compare --source sales --vendor "12345678" --app "123456789" --from "2026-01-01" --to "2026-02-01" --frequency DAILY
  asc analytics compare --source sales --vendor "12345678" --app "123456789" --from "2026-01" --to "2026-02" --frequency MONTHLY --output table
  asc analytics compare --source sales --vendor "12345678" --app "123456789" --from "2026-01-01" --from-end "2026-01-31" --to "2026-02-01" --to-end "2026-02-28" --frequency DAILY --output json --pretty`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return shared.UsageErrorf("unexpected argument(s): %s", strings.Join(args, " "))
			}

			sourceName := strings.ToLower(strings.TrimSpace(*source))
			if sourceName == "" {
				return shared.UsageError("--source is required")
			}
			if sourceName != "sales" {
				return shared.UsageError("--source must be sales")
			}

			resolvedAppID := shared.ResolveAppID(*appID)
			if resolvedAppID == "" {
				return shared.UsageError("--app is required (or set ASC_APP_ID)")
			}

			resolvedVendor := shared.ResolveVendorNumber(*vendor)
			if resolvedVendor == "" {
				return shared.UsageError("--vendor is required (or set ASC_VENDOR_NUMBER)")
			}

			if strings.TrimSpace(*from) == "" {
				return shared.UsageError("--from is required")
			}
			if strings.TrimSpace(*to) == "" {
				return shared.UsageError("--to is required")
			}
			if strings.TrimSpace(*frequency) == "" {
				return shared.UsageError("--frequency is required")
			}

			freq, err := normalizeSalesReportFrequency(*frequency)
			if err != nil {
				return shared.UsageError(err.Error())
			}

			salesType, err := normalizeSalesReportType(*reportType)
			if err != nil {
				return shared.UsageError(err.Error())
			}
			subType, err := normalizeSalesReportSubType(*reportSubType)
			if err != nil {
				return shared.UsageError(err.Error())
			}

			baselineStart, baselineEnd, err := normalizeCompareDateRange(*from, *fromEnd, freq, "--from", "--from-end")
			if err != nil {
				return shared.UsageError(err.Error())
			}

			compStart, compEnd, err := normalizeCompareDateRange(*to, *toEnd, freq, "--to", "--to-end")
			if err != nil {
				return shared.UsageError(err.Error())
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("analytics compare: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
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

			baselineMetrics, baselineCount, baselineErr := fetchAndAggregate(requestCtx, client, resolvedVendor, scope, baselineDates, salesType, subType, freq)
			compMetrics, compCount, compErr := fetchAndAggregate(requestCtx, client, resolvedVendor, scope, compDates, salesType, subType, freq)
			if baselineErr != nil || compErr != nil {
				return fmt.Errorf("analytics compare: %s", joinCompareErrors(baselineErr, compErr))
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
				Baseline:    comparePeriod{Start: baselineStart, End: baselineEnd, ReportsFound: baselineCount},
				Comparison:  comparePeriod{Start: compStart, End: compEnd, ReportsFound: compCount},
				Metrics:     buildCompareMetrics(baselineMetrics, compMetrics),
				GeneratedAt: time.Now().UTC().Format(time.RFC3339),
			}

			return shared.PrintOutputWithRenderers(
				resp,
				*output.Output,
				*output.Pretty,
				func() error { renderCompareOutput(resp, false); return nil },
				func() error { renderCompareOutput(resp, true); return nil },
			)
		},
	}
}

func fetchAndAggregate(ctx context.Context, client *asc.Client, vendor string, scope insights.SalesScope, dates []string, salesType asc.SalesReportType, subType asc.SalesReportSubType, freq asc.SalesReportFrequency) (insights.SalesMetrics, int, error) {
	var aggregate insights.SalesMetrics
	found := 0
	missingDates := make([]string, 0)

	for _, date := range dates {
		download, err := client.GetSalesReport(ctx, asc.SalesReportParams{
			VendorNumber:  vendor,
			ReportType:    salesType,
			ReportSubType: subType,
			Frequency:     freq,
			ReportDate:    date,
			Version:       asc.SalesReportVersion1_0,
		})
		if err != nil {
			if asc.IsNotFound(err) {
				missingDates = append(missingDates, date)
				continue
			}
			return aggregate, found, fmt.Errorf("download report %s: %w", date, err)
		}

		metrics, parseErr := insights.ParseSalesReportMetrics(download.Body, scope)
		_ = download.Body.Close()
		if parseErr != nil {
			return aggregate, found, fmt.Errorf("parse report %s: %w", date, parseErr)
		}

		// Seed from the first parsed report so availability flags reflect real coverage.
		if found == 0 {
			aggregate = metrics
		} else {
			aggregate = aggregateSalesMetrics(aggregate, metrics)
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
		RowCount:                       a.RowCount + b.RowCount,
		UnitsColumnPresent:             a.UnitsColumnPresent && b.UnitsColumnPresent,
		DeveloperProceedsColumnPresent: a.DeveloperProceedsColumnPresent && b.DeveloperProceedsColumnPresent,
		CustomerPriceColumnPresent:     a.CustomerPriceColumnPresent && b.CustomerPriceColumnPresent,
		SubscriptionColumnPresent:      a.SubscriptionColumnPresent && b.SubscriptionColumnPresent,
		UnitsTotal:                     a.UnitsTotal + b.UnitsTotal,
		DownloadUnitsTotal:             a.DownloadUnitsTotal + b.DownloadUnitsTotal,
		MonetizedUnitsTotal:            a.MonetizedUnitsTotal + b.MonetizedUnitsTotal,
		DeveloperProceedsTotal:         a.DeveloperProceedsTotal + b.DeveloperProceedsTotal,
		CustomerPriceTotal:             a.CustomerPriceTotal + b.CustomerPriceTotal,
		SubscriptionRows:               a.SubscriptionRows + b.SubscriptionRows,
		SubscriptionUnitsTotal:         a.SubscriptionUnitsTotal + b.SubscriptionUnitsTotal,
		SubscriptionDeveloperProceeds:  a.SubscriptionDeveloperProceeds + b.SubscriptionDeveloperProceeds,
		SubscriptionCustomerPrice:      a.SubscriptionCustomerPrice + b.SubscriptionCustomerPrice,
		RenewalRows:                    a.RenewalRows + b.RenewalRows,
		RenewalUnitsTotal:              a.RenewalUnitsTotal + b.RenewalUnitsTotal,
		RenewalDeveloperProceeds:       a.RenewalDeveloperProceeds + b.RenewalDeveloperProceeds,
		RenewalCustomerPrice:           a.RenewalCustomerPrice + b.RenewalCustomerPrice,
	}
}

func buildCompareMetrics(baseline, comparison insights.SalesMetrics) []compareMetric {
	metrics := []compareMetric{
		compareMetricFromValues("download_units", "count", "download units",
			baseline.DownloadUnitsTotal, comparison.DownloadUnitsTotal,
			baseline.UnitsColumnPresent, comparison.UnitsColumnPresent),
		compareMetricFromValues("monetized_units", "count", "monetized units",
			baseline.MonetizedUnitsTotal, comparison.MonetizedUnitsTotal,
			baseline.UnitsColumnPresent, comparison.UnitsColumnPresent),
		compareMetricFromValues("units", "count", "units",
			baseline.UnitsTotal, comparison.UnitsTotal,
			baseline.UnitsColumnPresent, comparison.UnitsColumnPresent),
		compareMetricFromValues("developer_proceeds", "currency", "developer proceeds",
			baseline.DeveloperProceedsTotal, comparison.DeveloperProceedsTotal,
			baseline.DeveloperProceedsColumnPresent, comparison.DeveloperProceedsColumnPresent),
		compareMetricFromValues("customer_price", "currency", "customer price",
			baseline.CustomerPriceTotal, comparison.CustomerPriceTotal,
			baseline.CustomerPriceColumnPresent, comparison.CustomerPriceColumnPresent),
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
	m := compareMetric{
		Name:       name,
		Unit:       unit,
		Baseline:   ptrFloat(baselineValue),
		Comparison: ptrFloat(comparisonValue),
		Delta:      ptrFloat(comparisonValue - baselineValue),
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
	deltaPercent := ((comparisonValue - baselineValue) / baselineValue) * 100
	m.DeltaPercent = ptrFloat(deltaPercent)
	return m
}

func ptrFloat(v float64) *float64 {
	return &v
}

func renderCompareOutput(resp *compareResponse, markdown bool) {
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
	shared.RenderSection("Context", []string{"field", "value"}, contextRows, markdown)

	metricRows := make([][]string, 0, len(resp.Metrics))
	for _, metric := range resp.Metrics {
		metricRows = append(metricRows, []string{
			metric.Name,
			shared.OrNA(metric.Unit),
			formatOptionalFloat(metric.Baseline),
			formatOptionalFloat(metric.Comparison),
			formatOptionalFloat(metric.Delta),
			formatOptionalFloat(metric.DeltaPercent),
			metric.Status,
			shared.OrNA(metric.Reason),
		})
	}
	shared.RenderSection("Metrics", []string{"metric", "unit", "baseline", "comparison", "delta", "deltaPercent", "status", "reason"}, metricRows, markdown)
}

func formatOptionalFloat(v *float64) string {
	if v == nil {
		return "n/a"
	}
	return fmt.Sprintf("%.2f", *v)
}
