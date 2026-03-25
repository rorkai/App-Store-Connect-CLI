package web

import (
	"context"
	"flag"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

var (
	newAnalyticsClientFn = webcore.NewAnalyticsClient

	getAnalyticsOverviewFn = func(ctx context.Context, client *webcore.Client, appID, startDate, endDate string) (*webcore.AnalyticsOverview, error) {
		return client.GetAnalyticsOverview(ctx, appID, startDate, endDate)
	}
	getAnalyticsSubscriptionsSummaryFn = func(ctx context.Context, client *webcore.Client, appID, startDate, endDate string) (*webcore.AnalyticsSubscriptionsSummary, error) {
		return client.GetAnalyticsSubscriptionsSummary(ctx, appID, startDate, endDate)
	}
	getAnalyticsMeasuresFn = func(ctx context.Context, client *webcore.Client, req webcore.AnalyticsMeasuresRequest) (*webcore.AnalyticsMeasuresResponse, error) {
		return client.GetAnalyticsMeasures(ctx, req)
	}
	getAnalyticsRetentionFn = func(ctx context.Context, client *webcore.Client, req webcore.AnalyticsRetentionRequest) (*webcore.AnalyticsRetentionResponse, error) {
		return client.GetAnalyticsRetention(ctx, req)
	}
	getAnalyticsCohortsFn = func(ctx context.Context, client *webcore.Client, req webcore.AnalyticsCohortsRequest) (*webcore.AnalyticsCohortsResponse, error) {
		return client.GetAnalyticsCohorts(ctx, req)
	}
)

type analyticsMetricsOutput struct {
	AppID     string                             `json:"appId"`
	StartDate string                             `json:"startDate"`
	EndDate   string                             `json:"endDate"`
	Frequency string                             `json:"frequency"`
	Result    *webcore.AnalyticsMeasuresResponse `json:"result"`
}

type analyticsRetentionOutput struct {
	AppID     string                              `json:"appId"`
	StartDate string                              `json:"startDate"`
	EndDate   string                              `json:"endDate"`
	Frequency string                              `json:"frequency"`
	Result    *webcore.AnalyticsRetentionResponse `json:"result"`
}

type analyticsCohortsOutput struct {
	AppID     string                            `json:"appId"`
	StartDate string                            `json:"startDate"`
	EndDate   string                            `json:"endDate"`
	Frequency string                            `json:"frequency"`
	Measures  []string                          `json:"measures"`
	Periods   []string                          `json:"periods"`
	Result    *webcore.AnalyticsCohortsResponse `json:"result"`
}

// WebAnalyticsCommand returns the analytics command group under `asc web`.
func WebAnalyticsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web analytics", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "analytics",
		ShortUsage: "asc web analytics <subcommand> [flags]",
		ShortHelp:  "[experimental] Recreate App Store Connect analytics web dashboards.",
		LongHelp: `EXPERIMENTAL / UNOFFICIAL / DISCOURAGED

	Query Apple's private analytics web endpoints using a user-owned App Store Connect
	web session. These commands are separate from the official Analytics Reports API.

` + webWarningText + `

Examples:
  asc web analytics overview --app "123456789" --start 2025-12-24 --end 2026-03-23 --apple-id "user@example.com"
  asc web analytics sources --app "123456789" --start 2025-12-24 --end 2026-03-23
  asc web analytics sales --app "123456789" --start 2025-12-24 --end 2026-03-23 --output markdown
  asc web analytics benchmarks --app "123456789"
  asc web analytics subscriptions --app "123456789" --start 2025-12-24 --end 2026-03-23 --output table
  asc web analytics metrics --app "123456789" --start 2025-12-24 --end 2026-03-23 --measures units,redownloads
  asc web analytics retention --app "123456789" --start 2026-02-22 --end 2026-03-23
  asc web analytics cohorts --app "123456789" --start 2025-12-24 --end 2026-03-23 --measures cohort-download-to-paid-rate --periods d1,d7,d35`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			WebAnalyticsOverviewCommand(),
			WebAnalyticsSourcesCommand(),
			WebAnalyticsProductPagesCommand(),
			WebAnalyticsInAppEventsCommand(),
			WebAnalyticsAppClipsCommand(),
			WebAnalyticsCampaignsCommand(),
			WebAnalyticsSalesCommand(),
			WebAnalyticsSubscriptionsCommand(),
			WebAnalyticsOffersCommand(),
			WebAnalyticsBenchmarksCommand(),
			WebAnalyticsMetricsCommand(),
			WebAnalyticsRetentionCommand(),
			WebAnalyticsCohortsCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// WebAnalyticsOverviewCommand recreates the analytics overview dashboard.
func WebAnalyticsOverviewCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web analytics overview", flag.ExitOnError)
	sessionFlags := bindWebSessionFlags(fs)
	output := shared.BindOutputFlags(fs)
	appID := fs.String("app", "", "App ID")
	start := fs.String("start", "", "Start date (YYYY-MM-DD)")
	end := fs.String("end", "", "End date (YYYY-MM-DD)")

	return &ffcli.Command{
		Name:       "overview",
		ShortUsage: "asc web analytics overview [flags]",
		ShortHelp:  "[experimental] Recreate the Analytics overview dashboard.",
		LongHelp: `EXPERIMENTAL / UNOFFICIAL / DISCOURAGED

	Recreate the App Store Connect Analytics overview dashboard: acquisition, sales,
	subscription summary cards, top breakdowns, and retention/cohort payloads.

` + webWarningText,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			resolvedAppID, startDate, endDate, err := resolveAnalyticsQueryFlags(*appID, *start, *end)
			if err != nil {
				return err
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			session, err := resolveWebSessionForCommand(requestCtx, sessionFlags)
			if err != nil {
				return err
			}
			client := newAnalyticsClientFn(session)
			result, err := withWebSpinnerValue("Loading analytics overview", func() (*webcore.AnalyticsOverview, error) {
				return getAnalyticsOverviewFn(requestCtx, client, resolvedAppID, startDate, endDate)
			})
			if err != nil {
				return withWebAuthHint(err, "analytics overview")
			}

			return shared.PrintOutputWithRenderers(
				result,
				*output.Output,
				*output.Pretty,
				func() error { return renderAnalyticsOverviewTable(result) },
				func() error { return renderAnalyticsOverviewMarkdown(result) },
			)
		},
	}
}

// WebAnalyticsSubscriptionsCommand recreates the subscriptions summary dashboard.
func WebAnalyticsSubscriptionsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web analytics subscriptions", flag.ExitOnError)
	sessionFlags := bindWebSessionFlags(fs)
	output := shared.BindOutputFlags(fs)
	appID := fs.String("app", "", "App ID")
	start := fs.String("start", "", "Start date (YYYY-MM-DD)")
	end := fs.String("end", "", "End date (YYYY-MM-DD)")

	return &ffcli.Command{
		Name:       "subscriptions",
		ShortUsage: "asc web analytics subscriptions [flags]",
		ShortHelp:  "[experimental] Recreate the subscriptions summary dashboard.",
		LongHelp: `EXPERIMENTAL / UNOFFICIAL / DISCOURAGED

	Recreate the App Store Connect subscriptions summary view: active/paid plan cards,
	monthly recurring revenue, net plan timeline, active plans by subscription, and
	subscription retention cohorts.

` + webWarningText,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			resolvedAppID, startDate, endDate, err := resolveAnalyticsQueryFlags(*appID, *start, *end)
			if err != nil {
				return err
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			session, err := resolveWebSessionForCommand(requestCtx, sessionFlags)
			if err != nil {
				return err
			}
			client := newAnalyticsClientFn(session)
			result, err := withWebSpinnerValue("Loading subscriptions analytics", func() (*webcore.AnalyticsSubscriptionsSummary, error) {
				return getAnalyticsSubscriptionsSummaryFn(requestCtx, client, resolvedAppID, startDate, endDate)
			})
			if err != nil {
				return withWebAuthHint(err, "analytics subscriptions")
			}

			return shared.PrintOutputWithRenderers(
				result,
				*output.Output,
				*output.Pretty,
				func() error { return renderAnalyticsSubscriptionsTable(result) },
				func() error { return renderAnalyticsSubscriptionsMarkdown(result) },
			)
		},
	}
}

// WebAnalyticsMetricsCommand queries analytics measure series directly.
func WebAnalyticsMetricsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web analytics metrics", flag.ExitOnError)
	sessionFlags := bindWebSessionFlags(fs)
	output := shared.BindOutputFlags(fs)
	appID := fs.String("app", "", "App ID")
	start := fs.String("start", "", "Start date (YYYY-MM-DD)")
	end := fs.String("end", "", "End date (YYYY-MM-DD)")
	measures := fs.String("measures", "", "Comma-separated measure keys")
	frequency := fs.String("frequency", "day", "Frequency: day, week, month")

	return &ffcli.Command{
		Name:       "metrics",
		ShortUsage: "asc web analytics metrics [flags]",
		ShortHelp:  "[experimental] Query private analytics measures.",
		LongHelp: `EXPERIMENTAL / UNOFFICIAL / DISCOURAGED

	Query private App Store Connect analytics measures over a custom date range.

` + webWarningText,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			resolvedAppID, startDate, endDate, err := resolveAnalyticsQueryFlags(*appID, *start, *end)
			if err != nil {
				return err
			}
			measureList := shared.SplitCSV(*measures)
			if len(measureList) == 0 {
				return shared.UsageError("--measures is required")
			}
			resolvedFrequency, err := resolveAnalyticsFrequency(*frequency)
			if err != nil {
				return err
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			session, err := resolveWebSessionForCommand(requestCtx, sessionFlags)
			if err != nil {
				return err
			}
			client := newAnalyticsClientFn(session)
			result, err := withWebSpinnerValue("Loading analytics measures", func() (*webcore.AnalyticsMeasuresResponse, error) {
				return getAnalyticsMeasuresFn(requestCtx, client, webcore.AnalyticsMeasuresRequest{
					AppID:     resolvedAppID,
					StartDate: startDate,
					EndDate:   endDate,
					Measures:  measureList,
					Frequency: resolvedFrequency,
				})
			})
			if err != nil {
				return withWebAuthHint(err, "analytics metrics")
			}

			payload := analyticsMetricsOutput{
				AppID:     resolvedAppID,
				StartDate: startDate,
				EndDate:   endDate,
				Frequency: resolvedFrequency,
				Result:    result,
			}
			return shared.PrintOutputWithRenderers(
				payload,
				*output.Output,
				*output.Pretty,
				func() error { return renderAnalyticsMetricsTable(payload) },
				func() error { return renderAnalyticsMetricsMarkdown(payload) },
			)
		},
	}
}

// WebAnalyticsRetentionCommand queries private analytics retention data.
func WebAnalyticsRetentionCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web analytics retention", flag.ExitOnError)
	sessionFlags := bindWebSessionFlags(fs)
	output := shared.BindOutputFlags(fs)
	appID := fs.String("app", "", "App ID")
	start := fs.String("start", "", "Start date (YYYY-MM-DD)")
	end := fs.String("end", "", "End date (YYYY-MM-DD)")
	frequency := fs.String("frequency", "day", "Frequency: day, week, month")

	return &ffcli.Command{
		Name:       "retention",
		ShortUsage: "asc web analytics retention [flags]",
		ShortHelp:  "[experimental] Query private analytics retention data.",
		LongHelp: `EXPERIMENTAL / UNOFFICIAL / DISCOURAGED

	Query private App Store Connect app retention data over a custom date range.

` + webWarningText,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			resolvedAppID, startDate, endDate, err := resolveAnalyticsQueryFlags(*appID, *start, *end)
			if err != nil {
				return err
			}
			resolvedFrequency, err := resolveAnalyticsFrequency(*frequency)
			if err != nil {
				return err
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			session, err := resolveWebSessionForCommand(requestCtx, sessionFlags)
			if err != nil {
				return err
			}
			client := newAnalyticsClientFn(session)
			result, err := withWebSpinnerValue("Loading analytics retention", func() (*webcore.AnalyticsRetentionResponse, error) {
				return getAnalyticsRetentionFn(requestCtx, client, webcore.AnalyticsRetentionRequest{
					AppID:     resolvedAppID,
					StartDate: startDate,
					EndDate:   endDate,
					Frequency: resolvedFrequency,
				})
			})
			if err != nil {
				return withWebAuthHint(err, "analytics retention")
			}

			payload := analyticsRetentionOutput{
				AppID:     resolvedAppID,
				StartDate: startDate,
				EndDate:   endDate,
				Frequency: resolvedFrequency,
				Result:    result,
			}
			return shared.PrintOutputWithRenderers(
				payload,
				*output.Output,
				*output.Pretty,
				func() error { return renderAnalyticsRetentionTable(payload) },
				func() error { return renderAnalyticsRetentionMarkdown(payload) },
			)
		},
	}
}

// WebAnalyticsCohortsCommand queries private analytics cohorts directly.
func WebAnalyticsCohortsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web analytics cohorts", flag.ExitOnError)
	sessionFlags := bindWebSessionFlags(fs)
	output := shared.BindOutputFlags(fs)
	appID := fs.String("app", "", "App ID")
	start := fs.String("start", "", "Start date (YYYY-MM-DD)")
	end := fs.String("end", "", "End date (YYYY-MM-DD)")
	measures := fs.String("measures", "", "Comma-separated cohort measure keys")
	periods := fs.String("periods", "", "Comma-separated cohort periods (for example d1,d7,d35 or m1,m3,m6,m12)")
	frequency := fs.String("frequency", "day", "Frequency: day, week, month")

	return &ffcli.Command{
		Name:       "cohorts",
		ShortUsage: "asc web analytics cohorts [flags]",
		ShortHelp:  "[experimental] Query private analytics cohort data.",
		LongHelp: `EXPERIMENTAL / UNOFFICIAL / DISCOURAGED

	Query private App Store Connect cohort data such as download-to-paid or
	subscription retention cohorts.

` + webWarningText,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			resolvedAppID, startDate, endDate, err := resolveAnalyticsQueryFlags(*appID, *start, *end)
			if err != nil {
				return err
			}
			measureList := shared.SplitCSV(*measures)
			if len(measureList) == 0 {
				return shared.UsageError("--measures is required")
			}
			periodList := shared.SplitCSV(*periods)
			if len(periodList) == 0 {
				return shared.UsageError("--periods is required")
			}
			resolvedFrequency, err := resolveAnalyticsFrequency(*frequency)
			if err != nil {
				return err
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			session, err := resolveWebSessionForCommand(requestCtx, sessionFlags)
			if err != nil {
				return err
			}
			client := newAnalyticsClientFn(session)
			result, err := withWebSpinnerValue("Loading analytics cohorts", func() (*webcore.AnalyticsCohortsResponse, error) {
				return getAnalyticsCohortsFn(requestCtx, client, webcore.AnalyticsCohortsRequest{
					AppID:     resolvedAppID,
					StartDate: startDate,
					EndDate:   endDate,
					Measures:  measureList,
					Periods:   periodList,
					Frequency: resolvedFrequency,
				})
			})
			if err != nil {
				return withWebAuthHint(err, "analytics cohorts")
			}

			payload := analyticsCohortsOutput{
				AppID:     resolvedAppID,
				StartDate: startDate,
				EndDate:   endDate,
				Frequency: resolvedFrequency,
				Measures:  measureList,
				Periods:   periodList,
				Result:    result,
			}
			return shared.PrintOutputWithRenderers(
				payload,
				*output.Output,
				*output.Pretty,
				func() error { return renderAnalyticsCohortsTable(payload) },
				func() error { return renderAnalyticsCohortsMarkdown(payload) },
			)
		},
	}
}

func resolveAnalyticsQueryFlags(appID, startDate, endDate string) (string, string, string, error) {
	resolvedAppID := strings.TrimSpace(shared.ResolveAppID(appID))
	if resolvedAppID == "" {
		return "", "", "", shared.UsageError("--app is required")
	}
	if err := validateDateFlag("--start", startDate); err != nil {
		return "", "", "", shared.UsageError(err.Error())
	}
	if err := validateDateFlag("--end", endDate); err != nil {
		return "", "", "", shared.UsageError(err.Error())
	}
	startTime, _ := time.Parse("2006-01-02", strings.TrimSpace(startDate))
	endTime, _ := time.Parse("2006-01-02", strings.TrimSpace(endDate))
	if endTime.Before(startTime) {
		return "", "", "", shared.UsageError("--end must be on or after --start")
	}
	return resolvedAppID, strings.TrimSpace(startDate), strings.TrimSpace(endDate), nil
}

func resolveAnalyticsFrequency(value string) (string, error) {
	frequency, err := webcore.NormalizeAnalyticsFrequency(value)
	if err != nil {
		return "", shared.UsageError("--" + err.Error())
	}
	return frequency, nil
}

func analyticsMeasureLabel(measure string) string {
	switch strings.TrimSpace(measure) {
	case "units":
		return "First-Time Downloads"
	case "redownloads":
		return "Redownloads"
	case "totalDownloads":
		return "Total Downloads"
	case "conversionRate", "benchConversionRate":
		return "Conversion Rate"
	case "impressionsTotal":
		return "Impressions"
	case "pageViewUnique":
		return "Product Page Views (Unique Devices)"
	case "pageViewCount":
		return "Product Page Views"
	case "updates":
		return "Updates"
	case "eventImpressions":
		return "Event Impressions"
	case "eventOpens":
		return "App Opens"
	case "appClipViews":
		return "App Clip Views"
	case "appClipInstalls":
		return "App Clip Installs"
	case "appClipSessions":
		return "App Clip Sessions"
	case "appClipCrashes":
		return "App Clip Crashes"
	case "crashes":
		return "Crashes"
	case "proceeds":
		return "Proceeds"
	case "payingUsers":
		return "Paying Users"
	case "iap":
		return "In-App Purchases"
	case "sessions":
		return "Sessions"
	case "subscription-state-plans-active":
		return "Active Plans"
	case "subscription-state-paid":
		return "Paid Plans"
	case "revenue-recurring":
		return "Monthly Recurring Revenue"
	case "summary-plans-paid-net":
		return "Net Paid Plans"
	case "summary-plans-paid-starts":
		return "Paid Plan Starts"
	case "summary-plans-paid-churned":
		return "Paid Plan Churn"
	case "subscription-state-offers":
		return "Active Offers"
	case "subscription-state-offers-freeTrial":
		return "Free Trials"
	case "subscription-state-offers-paidOffer":
		return "Paid Offers"
	case "summary-offers-net":
		return "Net Offers"
	case "summary-offers-activations":
		return "Offer Activations"
	case "summary-offers-reactivations":
		return "Offer Reactivations"
	case "summary-offers-churned":
		return "Offer Churn"
	case "cohort-download-to-paid-rate":
		return "Download to Paid"
	case "cohort-download-proceeds-per-download-average":
		return "Accumulated Proceeds"
	case "cohort-subscription-retention-rate":
		return "Subscription Retention"
	case "arppu", "benchArppu":
		return "Proceeds per Paying User"
	case "crashRate", "benchCrashRate":
		return "Crash Rate"
	case "retentionD1", "benchRetentionD1":
		return "Day 1 Retention"
	case "retentionD7", "benchRetentionD7":
		return "Day 7 Retention"
	case "retentionD28", "benchRetentionD28":
		return "Day 28 Retention"
	case "download-to-paid-rate-d35", "download-to-paid-rate-d35-benchmark":
		return "D35 Download to Paid Conversion"
	case "proceeds-per-download-average-d35", "proceeds-per-download-average-d35-benchmark":
		return "D35 Proceeds per Download"
	default:
		return strings.TrimSpace(measure)
	}
}

func analyticsPeriodLabel(period string) string {
	switch strings.ToLower(strings.TrimSpace(period)) {
	case "d1":
		return "Day 1"
	case "d7":
		return "Day 7"
	case "d14":
		return "Day 14"
	case "d35":
		return "Day 35"
	case "d60":
		return "Day 60"
	case "d90":
		return "Day 90"
	case "d180":
		return "Day 180"
	case "m1":
		return "Month 1"
	case "m2":
		return "Month 2"
	case "m3":
		return "Month 3"
	case "m6":
		return "Month 6"
	case "m9":
		return "Month 9"
	case "m12":
		return "Month 12"
	default:
		return strings.TrimSpace(period)
	}
}

func analyticsNumberString(value *float64) string {
	if value == nil {
		return "-"
	}
	v := *value
	if v == float64(int64(v)) {
		return fmt.Sprintf("%.0f", v)
	}
	return fmt.Sprintf("%.2f", v)
}

func analyticsMeasureValueString(measure string, value *float64) string {
	if value == nil {
		return "-"
	}
	v := *value
	switch strings.TrimSpace(measure) {
	case "conversionRate", "cohort-download-to-paid-rate", "cohort-subscription-retention-rate", "download-to-paid-rate-d35", "download-to-paid-rate-d35-benchmark", "crashRate", "benchCrashRate", "retentionD1", "benchRetentionD1", "retentionD7", "benchRetentionD7", "retentionD28", "benchRetentionD28", "benchConversionRate":
		return fmt.Sprintf("%.2f%%", v)
	case "proceeds", "revenue-recurring", "arppu", "benchArppu", "proceeds-per-download-average-d35", "proceeds-per-download-average-d35-benchmark", "cohort-download-proceeds-per-download-average":
		if v == float64(int64(v)) {
			return fmt.Sprintf("$%.0f", v)
		}
		return fmt.Sprintf("$%.2f", v)
	default:
		return analyticsNumberString(value)
	}
}

func analyticsPercentChangeString(value *float64) string {
	if value == nil {
		return "-"
	}
	// App Store Connect returns percentChange as a ratio, so 3 means +300.0%.
	v := *value * 100
	if v > 0 {
		return fmt.Sprintf("+%.1f%%", v)
	}
	return fmt.Sprintf("%.1f%%", v)
}

func buildAnalyticsMeasureRows(section string, results []webcore.AnalyticsMeasureResult) [][]string {
	rows := make([][]string, 0, len(results))
	for _, result := range results {
		rows = append(rows, []string{
			section,
			analyticsMeasureLabel(result.Measure),
			analyticsMeasureValueString(result.Measure, result.Total),
			analyticsMeasureValueString(result.Measure, result.PreviousTotal),
			analyticsPercentChangeString(result.PercentChange),
		})
	}
	return rows
}

type analyticsCohortValueRow struct {
	Measure string
	Period  string
	Value   *float64
}

func analyticsLatestCohortRows(resp *webcore.AnalyticsCohortsResponse) []analyticsCohortValueRow {
	if resp == nil || len(resp.Results) == 0 {
		return nil
	}
	periods := resp.Results["period"]
	measureKeys := make([]string, 0, len(resp.Results))
	for key := range resp.Results {
		if key == "date" || key == "period" {
			continue
		}
		measureKeys = append(measureKeys, key)
	}
	sort.Strings(measureKeys)

	out := []analyticsCohortValueRow{}
	for _, measureKey := range measureKeys {
		values := resp.Results[measureKey]
		latestByPeriod := map[string]*float64{}
		for i := 0; i < len(periods) && i < len(values); i++ {
			period := strings.TrimSpace(fmt.Sprint(periods[i]))
			if period == "" || period == "<nil>" {
				continue
			}
			raw := values[i]
			if raw == nil {
				continue
			}
			switch v := raw.(type) {
			case float64:
				value := v
				latestByPeriod[period] = &value
			case int:
				value := float64(v)
				latestByPeriod[period] = &value
			}
		}
		for _, period := range sortedAnalyticsPeriods(latestByPeriod) {
			out = append(out, analyticsCohortValueRow{
				Measure: measureKey,
				Period:  period,
				Value:   latestByPeriod[period],
			})
		}
	}
	return out
}

func sortedAnalyticsPeriods(values map[string]*float64) []string {
	periods := make([]string, 0, len(values))
	for period := range values {
		periods = append(periods, period)
	}
	sort.Slice(periods, func(i, j int) bool {
		lPrefix, lValue, lok := analyticsPeriodSortKey(periods[i])
		rPrefix, rValue, rok := analyticsPeriodSortKey(periods[j])
		if lok && rok {
			if lPrefix != rPrefix {
				return lPrefix < rPrefix
			}
			return lValue < rValue
		}
		if lok != rok {
			return lok
		}
		return periods[i] < periods[j]
	})
	return periods
}

func analyticsPeriodSortKey(period string) (int, int, bool) {
	normalized := strings.ToLower(strings.TrimSpace(period))
	if len(normalized) < 2 {
		return 0, 0, false
	}
	value, err := strconv.Atoi(normalized[1:])
	if err != nil {
		return 0, 0, false
	}
	switch normalized[0] {
	case 'd':
		return 0, value, true
	case 'w':
		return 1, value, true
	case 'm':
		return 2, value, true
	default:
		return 0, 0, false
	}
}

func analyticsLatestTimelineRows(results []webcore.AnalyticsTimeseriesResult) [][]string {
	rows := [][]string{}
	for _, result := range results {
		if result.Group != nil {
			continue
		}
		var latest map[string]any
		for _, point := range result.Data {
			latest = point
		}
		if latest == nil {
			continue
		}
		for key, value := range latest {
			if key == "date" || value == nil {
				continue
			}
			number, ok := value.(float64)
			if !ok {
				continue
			}
			valueCopy := number
			rows = append(rows, []string{
				"Timeline",
				analyticsMeasureLabel(key),
				analyticsMeasureValueString(key, &valueCopy),
			})
		}
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i][1] < rows[j][1] })
	return rows
}

func analyticsBreakdownRows(section string, breakdowns []webcore.AnalyticsBreakdown) [][]string {
	rows := [][]string{}
	for _, breakdown := range breakdowns {
		for _, item := range breakdown.Items {
			rows = append(rows, []string{
				section,
				breakdown.Name,
				fmt.Sprintf("%s = %s", item.Label, analyticsNumberString(analyticsFloatPtr(item.Value))),
			})
		}
	}
	return rows
}

func buildAnalyticsOverviewRows(result *webcore.AnalyticsOverview) [][]string {
	rows := [][]string{
		{"Query", "App ID", result.AppID},
		{"Query", "Start", result.StartDate},
		{"Query", "End", result.EndDate},
	}
	rows = append(rows, buildAnalyticsMeasureRows("Acquisition", result.Acquisition)...)
	rows = append(rows, buildAnalyticsMeasureRows("Sales", result.Sales)...)
	rows = append(rows, buildAnalyticsMeasureRows("Subscriptions", result.Subscriptions)...)
	for _, row := range analyticsLatestCohortRows(result.DownloadToPaid) {
		rows = append(rows, []string{
			"Sales",
			fmt.Sprintf("%s %s", analyticsPeriodLabel(row.Period), analyticsMeasureLabel(row.Measure)),
			analyticsMeasureValueString(row.Measure, row.Value),
		})
	}
	rows = append(rows, analyticsLatestTimelineRows(result.PlanTimeline)...)
	rows = append(rows, analyticsBreakdownRows("App Store Features", result.FeatureBreakdowns)...)
	rows = append(rows, analyticsBreakdownRows("App Usage", result.AppUsageBreakdowns)...)
	return rows
}

func renderAnalyticsOverviewTable(result *webcore.AnalyticsOverview) error {
	sections := []analyticsTableSection{
		analyticsQuerySection("Overview", result.AppID, result.StartDate, result.EndDate),
		analyticsMeasureSummarySection("Acquisition", result.Acquisition),
		analyticsMeasureSummarySection("Sales", result.Sales),
		analyticsMeasureSummarySection("Subscriptions", result.Subscriptions),
		analyticsLatestCohortSection("Download to Paid", result.DownloadToPaid),
		analyticsLatestTimelineSection("Paid Plan Timeline", result.PlanTimeline),
	}
	for i := range result.FeatureBreakdowns {
		sections = append(sections, analyticsBreakdownSection("", &result.FeatureBreakdowns[i]))
	}
	for i := range result.AppUsageBreakdowns {
		sections = append(sections, analyticsBreakdownSection("", &result.AppUsageBreakdowns[i]))
	}
	return renderAnalyticsTableSections(sections...)
}

func renderAnalyticsOverviewMarkdown(result *webcore.AnalyticsOverview) error {
	headers := []string{"Section", "Field", "Value", "Previous", "Change"}
	rows := buildAnalyticsOverviewRows(result)
	normalized := normalizeAnalyticsRows(rows, 5)
	asc.RenderMarkdown(headers, normalized)
	return nil
}

func buildAnalyticsSubscriptionsRows(result *webcore.AnalyticsSubscriptionsSummary) [][]string {
	rows := [][]string{
		{"Query", "App ID", result.AppID},
		{"Query", "Start", result.StartDate},
		{"Query", "End", result.EndDate},
	}
	rows = append(rows, buildAnalyticsMeasureRows("Subscriptions", result.Summary)...)
	rows = append(rows, analyticsLatestTimelineRows(result.PlanTimeline)...)
	if result.ActivePlansBySubscription != nil {
		for _, item := range result.ActivePlansBySubscription.Items {
			rows = append(rows, []string{
				"Subscriptions",
				result.ActivePlansBySubscription.Name,
				fmt.Sprintf("%s = %s", item.Label, analyticsNumberString(analyticsFloatPtr(item.Value))),
			})
		}
	}
	for _, row := range analyticsLatestCohortRows(result.SubscriptionRetention) {
		rows = append(rows, []string{
			"Retention",
			fmt.Sprintf("%s %s", analyticsPeriodLabel(row.Period), analyticsMeasureLabel(row.Measure)),
			analyticsMeasureValueString(row.Measure, row.Value),
		})
	}
	return rows
}

func renderAnalyticsSubscriptionsTable(result *webcore.AnalyticsSubscriptionsSummary) error {
	sections := []analyticsTableSection{
		analyticsQuerySection("Subscriptions", result.AppID, result.StartDate, result.EndDate),
		analyticsMeasureSummarySection("Summary Cards", result.Summary),
		analyticsLatestTimelineSection("Paid Plan Timeline", result.PlanTimeline),
		analyticsBreakdownSection("Active Plans by Subscription", result.ActivePlansBySubscription),
		analyticsLatestCohortSection("Subscription Retention", result.SubscriptionRetention),
	}
	return renderAnalyticsTableSections(sections...)
}

func renderAnalyticsSubscriptionsMarkdown(result *webcore.AnalyticsSubscriptionsSummary) error {
	headers := []string{"Section", "Field", "Value", "Previous", "Change"}
	asc.RenderMarkdown(headers, normalizeAnalyticsRows(buildAnalyticsSubscriptionsRows(result), 5))
	return nil
}

func buildAnalyticsMetricsRows(payload analyticsMetricsOutput) [][]string {
	rows := [][]string{
		{"Query", "App ID", payload.AppID, "", ""},
		{"Query", "Start", payload.StartDate, "", ""},
		{"Query", "End", payload.EndDate, "", ""},
		{"Query", "Frequency", payload.Frequency, "", ""},
	}
	if payload.Result == nil {
		return rows
	}
	rows = append(rows, buildAnalyticsMeasureRows("Metrics", payload.Result.Results)...)
	return rows
}

func renderAnalyticsMetricsTable(payload analyticsMetricsOutput) error {
	var results []webcore.AnalyticsMeasureResult
	if payload.Result != nil {
		results = payload.Result.Results
	}
	return renderAnalyticsTableSections(
		analyticsQuerySection(
			"Metrics",
			payload.AppID,
			payload.StartDate,
			payload.EndDate,
			[]string{"Frequency", payload.Frequency},
		),
		analyticsMeasureSummarySection("Measure Summary", results),
	)
}

func renderAnalyticsMetricsMarkdown(payload analyticsMetricsOutput) error {
	headers := []string{"Section", "Field", "Value", "Previous", "Change"}
	asc.RenderMarkdown(headers, normalizeAnalyticsRows(buildAnalyticsMetricsRows(payload), 5))
	return nil
}

func buildAnalyticsRetentionRows(payload analyticsRetentionOutput) [][]string {
	rows := [][]string{
		{"Query", "App ID", payload.AppID},
		{"Query", "Start", payload.StartDate},
		{"Query", "End", payload.EndDate},
		{"Query", "Frequency", payload.Frequency},
	}
	if payload.Result == nil {
		return rows
	}
	for _, cohort := range payload.Result.Results {
		for _, point := range cohort.Data {
			rows = append(rows, []string{
				cohort.AppPurchase,
				point.Date,
				analyticsMeasureValueString("cohort-subscription-retention-rate", point.RetentionPercentage),
				analyticsNumberString(point.Value),
			})
		}
	}
	return rows
}

func renderAnalyticsRetentionTable(payload analyticsRetentionOutput) error {
	return renderAnalyticsTableSections(
		analyticsQuerySection(
			"Retention",
			payload.AppID,
			payload.StartDate,
			payload.EndDate,
			[]string{"Frequency", payload.Frequency},
		),
		analyticsRetentionDataSection(payload),
	)
}

func renderAnalyticsRetentionMarkdown(payload analyticsRetentionOutput) error {
	headers := []string{"Cohort", "Date", "Retention", "Value"}
	asc.RenderMarkdown(headers, normalizeAnalyticsRows(buildAnalyticsRetentionRows(payload), 4))
	return nil
}

func buildAnalyticsCohortRows(payload analyticsCohortsOutput) [][]string {
	rows := [][]string{
		{"Query", "App ID", payload.AppID, ""},
		{"Query", "Start", payload.StartDate, ""},
		{"Query", "End", payload.EndDate, ""},
		{"Query", "Frequency", payload.Frequency, ""},
	}
	if payload.Result == nil || len(payload.Result.Results) == 0 {
		return rows
	}
	dates := payload.Result.Results["date"]
	periods := payload.Result.Results["period"]
	measureKeys := make([]string, 0, len(payload.Result.Results))
	for key := range payload.Result.Results {
		if key == "date" || key == "period" {
			continue
		}
		measureKeys = append(measureKeys, key)
	}
	sort.Strings(measureKeys)
	for _, measure := range measureKeys {
		values := payload.Result.Results[measure]
		maxLen := len(values)
		if len(dates) < maxLen {
			maxLen = len(dates)
		}
		if len(periods) < maxLen {
			maxLen = len(periods)
		}
		for i := 0; i < maxLen; i++ {
			rows = append(rows, []string{
				fmt.Sprint(dates[i]),
				analyticsPeriodLabel(fmt.Sprint(periods[i])),
				analyticsMeasureLabel(measure),
				analyticsAnyValueString(measure, values[i]),
			})
		}
	}
	return rows
}

func analyticsAnyValueString(measure string, raw any) string {
	switch v := raw.(type) {
	case nil:
		return "-"
	case float64:
		value := v
		return analyticsMeasureValueString(measure, &value)
	case int:
		value := float64(v)
		return analyticsMeasureValueString(measure, &value)
	default:
		return strings.TrimSpace(fmt.Sprint(raw))
	}
}

func renderAnalyticsCohortsTable(payload analyticsCohortsOutput) error {
	return renderAnalyticsTableSections(
		analyticsQuerySection(
			"Cohorts",
			payload.AppID,
			payload.StartDate,
			payload.EndDate,
			[]string{"Frequency", payload.Frequency},
			[]string{"Measures", strings.Join(payload.Measures, ", ")},
			[]string{"Periods", strings.Join(payload.Periods, ", ")},
		),
		analyticsCohortDataSection(payload),
	)
}

func renderAnalyticsCohortsMarkdown(payload analyticsCohortsOutput) error {
	headers := []string{"Date", "Period", "Measure", "Value"}
	asc.RenderMarkdown(headers, normalizeAnalyticsRows(buildAnalyticsCohortRows(payload), 4))
	return nil
}

func normalizeAnalyticsRows(rows [][]string, width int) [][]string {
	normalized := make([][]string, 0, len(rows))
	for _, row := range rows {
		if len(row) >= width {
			normalized = append(normalized, row[:width])
			continue
		}
		next := make([]string, width)
		copy(next, row)
		normalized = append(normalized, next)
	}
	return normalized
}

func analyticsFloatPtr(v float64) *float64 {
	return &v
}
