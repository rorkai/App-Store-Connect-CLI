package web

import (
	"context"
	"flag"
	"fmt"
	"sort"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

var (
	getAnalyticsSourcesPageFn = func(ctx context.Context, client *webcore.Client, appID, startDate, endDate string) (*webcore.AnalyticsSourcesPage, error) {
		return client.GetAnalyticsSourcesPage(ctx, appID, startDate, endDate)
	}
	getAnalyticsInAppEventsPageFn = func(ctx context.Context, client *webcore.Client, appID, startDate, endDate string) (*webcore.AnalyticsInAppEventsPage, error) {
		return client.GetAnalyticsInAppEventsPage(ctx, appID, startDate, endDate)
	}
	getAnalyticsCampaignsPageFn = func(ctx context.Context, client *webcore.Client, appID, startDate, endDate string) (*webcore.AnalyticsCampaignsPage, error) {
		return client.GetAnalyticsCampaignsPage(ctx, appID, startDate, endDate)
	}
	getAnalyticsSalesSummaryFn = func(ctx context.Context, client *webcore.Client, appID, startDate, endDate string) (*webcore.AnalyticsSalesSummary, error) {
		return client.GetAnalyticsSalesSummary(ctx, appID, startDate, endDate)
	}
	getAnalyticsBenchmarksFn = func(ctx context.Context, client *webcore.Client, appID string) (*webcore.AnalyticsBenchmarksSummary, error) {
		return client.GetAnalyticsBenchmarks(ctx, appID)
	}
	getAnalyticsAppInfoFn = func(ctx context.Context, client *webcore.Client, appID string) (*webcore.AnalyticsAppInfoResult, error) {
		return client.GetAnalyticsAppInfo(ctx, appID)
	}
)

type analyticsCapabilityHint struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type analyticsPageCapabilityOutput struct {
	AppID              string                    `json:"appId"`
	Page               string                    `json:"page"`
	Status             string                    `json:"status"`
	Reason             string                    `json:"reason"`
	RelevantMeasures   []string                  `json:"relevantMeasures,omitempty"`
	RelevantDimensions []string                  `json:"relevantDimensions,omitempty"`
	Hints              []analyticsCapabilityHint `json:"hints,omitempty"`
}

// newWebAnalyticsSourcesCommand recreates the Acquisition > Sources page.
func newWebAnalyticsSourcesCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("web analytics sources", flag.ExitOnError)
	sessionFlags := bindWebSessionFlags(fs)
	output := d.BindOutputFlags(fs)
	appID := fs.String("app", "", "App ID")
	start := fs.String("start", "", "Start date (YYYY-MM-DD)")
	end := fs.String("end", "", "End date (YYYY-MM-DD)")

	return &ffcli.Command{
		Name:       "sources",
		ShortUsage: "asc web analytics sources [flags]",
		ShortHelp:  "Recreate the Acquisition > Sources page.",
		LongHelp: `WEB SESSION WORKFLOWS

	Show the default Acquisition > Sources view: Product Page Views (Unique Devices)
	grouped by source over the selected date range.

`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			resolvedAppID, startDate, endDate, err := resolveAnalyticsQueryFlags(d, *appID, *start, *end)
			if err != nil {
				return err
			}
			session, requestCtx, cancel, err := resolveWebSessionForCommandWith(ctx, d, sessionFlags)
			defer cancel()
			if err != nil {
				return err
			}
			client := newAnalyticsClientFn(session)
			result, err := withWebSpinnerValueWith(d, "Loading acquisition sources", func() (*webcore.AnalyticsSourcesPage, error) {
				return getAnalyticsSourcesPageFn(requestCtx, client, resolvedAppID, startDate, endDate)
			})
			if err != nil {
				return withWebAuthHint(err, "analytics sources")
			}

			return d.PrintOutputWithRenderers(
				result,
				*output.Output,
				*output.Pretty,
				func() error { return renderAnalyticsSourcesTable(d, result) },
				func() error { return renderAnalyticsSourcesMarkdown(d, result) },
			)
		},
	}
}

// newWebAnalyticsProductPagesCommand explains Product Pages availability for the app.
func newWebAnalyticsProductPagesCommand(d *shared.Deps) *ffcli.Command {
	cmd := newAnalyticsCapabilityCommand(
		d,
		"product-pages",
		"Inspect Product Pages tab availability.",
		"Product Pages tab availability",
		func(appInfo *webcore.AnalyticsAppInfoResult) analyticsPageCapabilityOutput {
			count := analyticsAppFeatureCount(appInfo, "customProductPage")
			status := "unavailable"
			reason := "App metadata reports zero custom product pages, so App Store Connect disables this tab."
			if count > 0 {
				status = "available"
				reason = "App metadata reports one or more custom product pages."
			}
			return analyticsPageCapabilityOutput{
				AppID:              strings.TrimSpace(appInfo.AdamID),
				Page:               "product-pages",
				Status:             status,
				Reason:             reason,
				RelevantMeasures:   []string{"pageViewUnique"},
				RelevantDimensions: []string{"productPage"},
				Hints: []analyticsCapabilityHint{
					{Key: "customProductPages", Value: fmt.Sprintf("%d", count)},
					{Key: "bundleId", Value: appInfo.BundleID},
				},
			}
		},
	)
	cmd.Subcommands = []*ffcli.Command{newWebAnalyticsExperimentsCommand(d)}
	return cmd
}

// newWebAnalyticsInAppEventsCommand recreates the Acquisition > In-App Events page.
func newWebAnalyticsInAppEventsCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("web analytics in-app-events", flag.ExitOnError)
	sessionFlags := bindWebSessionFlags(fs)
	output := d.BindOutputFlags(fs)
	appID := fs.String("app", "", "App ID")
	start := fs.String("start", "", "Requested start date (YYYY-MM-DD)")
	end := fs.String("end", "", "Requested end date (YYYY-MM-DD)")

	return &ffcli.Command{
		Name:       "in-app-events",
		ShortUsage: "asc web analytics in-app-events [flags]",
		ShortHelp:  "Recreate the Acquisition > In-App Events page.",
		LongHelp: `WEB SESSION WORKFLOWS

	Show the In-App Events tab using the app's event list and the default selected
	event metrics. App Store Connect currently uses the event's lifetime range for
	the selected event metrics.

`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			resolvedAppID, startDate, endDate, err := resolveAnalyticsQueryFlags(d, *appID, *start, *end)
			if err != nil {
				return err
			}
			session, requestCtx, cancel, err := resolveWebSessionForCommandWith(ctx, d, sessionFlags)
			defer cancel()
			if err != nil {
				return err
			}
			client := newAnalyticsClientFn(session)
			result, err := withWebSpinnerValueWith(d, "Loading in-app events analytics", func() (*webcore.AnalyticsInAppEventsPage, error) {
				return getAnalyticsInAppEventsPageFn(requestCtx, client, resolvedAppID, startDate, endDate)
			})
			if err != nil {
				return withWebAuthHint(err, "analytics in-app-events")
			}

			return d.PrintOutputWithRenderers(
				result,
				*output.Output,
				*output.Pretty,
				func() error { return renderAnalyticsInAppEventsTable(d, result) },
				func() error { return renderAnalyticsInAppEventsMarkdown(d, result) },
			)
		},
	}
}

// newWebAnalyticsAppClipsCommand explains App Clip tab availability for the app.
func newWebAnalyticsAppClipsCommand(d *shared.Deps) *ffcli.Command {
	return newAnalyticsCapabilityCommand(
		d,
		"app-clips",
		"Inspect App Clip tab availability.",
		"App Clip tab availability",
		func(appInfo *webcore.AnalyticsAppInfoResult) analyticsPageCapabilityOutput {
			status := "unavailable"
			reason := "App metadata reports no App Clip for this app, so App Store Connect disables this tab."
			if appInfo.HasAppClips {
				status = "available"
				reason = "App metadata reports an App Clip."
			}
			return analyticsPageCapabilityOutput{
				AppID:            strings.TrimSpace(appInfo.AdamID),
				Page:             "app-clips",
				Status:           status,
				Reason:           reason,
				RelevantMeasures: []string{"appClipViews", "appClipInstalls", "appClipSessions", "appClipCrashes"},
				Hints: []analyticsCapabilityHint{
					{Key: "hasAppClips", Value: fmt.Sprintf("%t", appInfo.HasAppClips)},
					{Key: "bundleId", Value: appInfo.BundleID},
				},
			}
		},
	)
}

// newWebAnalyticsCampaignsCommand recreates the Acquisition > Campaigns page.
func newWebAnalyticsCampaignsCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("web analytics campaigns", flag.ExitOnError)
	sessionFlags := bindWebSessionFlags(fs)
	output := d.BindOutputFlags(fs)
	appID := fs.String("app", "", "App ID")
	start := fs.String("start", "", "Start date (YYYY-MM-DD)")
	end := fs.String("end", "", "End date (YYYY-MM-DD)")

	return &ffcli.Command{
		Name:       "campaigns",
		ShortUsage: "asc web analytics campaigns [flags]",
		ShortHelp:  "Recreate the Acquisition > Campaigns page.",
		LongHelp: `WEB SESSION WORKFLOWS

	Show the Campaigns tab using Apple's sources/list endpoint.

`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			resolvedAppID, startDate, endDate, err := resolveAnalyticsQueryFlags(d, *appID, *start, *end)
			if err != nil {
				return err
			}
			session, requestCtx, cancel, err := resolveWebSessionForCommandWith(ctx, d, sessionFlags)
			defer cancel()
			if err != nil {
				return err
			}
			client := newAnalyticsClientFn(session)
			result, err := withWebSpinnerValueWith(d, "Loading campaigns analytics", func() (*webcore.AnalyticsCampaignsPage, error) {
				return getAnalyticsCampaignsPageFn(requestCtx, client, resolvedAppID, startDate, endDate)
			})
			if err != nil {
				return withWebAuthHint(err, "analytics campaigns")
			}

			return d.PrintOutputWithRenderers(
				result,
				*output.Output,
				*output.Pretty,
				func() error { return renderAnalyticsCampaignsTable(d, result) },
				func() error { return renderAnalyticsCampaignsMarkdown(d, result) },
			)
		},
	}
}

// newWebAnalyticsSalesCommand recreates the Monetization > Sales page.
func newWebAnalyticsSalesCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("web analytics sales", flag.ExitOnError)
	sessionFlags := bindWebSessionFlags(fs)
	output := d.BindOutputFlags(fs)
	appID := fs.String("app", "", "App ID")
	start := fs.String("start", "", "Start date (YYYY-MM-DD)")
	end := fs.String("end", "", "End date (YYYY-MM-DD)")

	return &ffcli.Command{
		Name:       "sales",
		ShortUsage: "asc web analytics sales [flags]",
		ShortHelp:  "Recreate the Monetization > Sales page.",
		LongHelp: `WEB SESSION WORKFLOWS

	Show the Sales summary cards, revenue cohort cards, and top revenue breakdowns.

`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			resolvedAppID, startDate, endDate, err := resolveAnalyticsQueryFlags(d, *appID, *start, *end)
			if err != nil {
				return err
			}
			session, requestCtx, cancel, err := resolveWebSessionForCommandWith(ctx, d, sessionFlags)
			defer cancel()
			if err != nil {
				return err
			}
			client := newAnalyticsClientFn(session)
			result, err := withWebSpinnerValueWith(d, "Loading sales analytics", func() (*webcore.AnalyticsSalesSummary, error) {
				return getAnalyticsSalesSummaryFn(requestCtx, client, resolvedAppID, startDate, endDate)
			})
			if err != nil {
				return withWebAuthHint(err, "analytics sales")
			}

			return d.PrintOutputWithRenderers(
				result,
				*output.Output,
				*output.Pretty,
				func() error { return renderAnalyticsSalesTable(d, result) },
				func() error { return renderAnalyticsSalesMarkdown(d, result) },
			)
		},
	}
}

// newWebAnalyticsOffersCommand explains Offers tab capability for the app.
func newWebAnalyticsOffersCommand(d *shared.Deps) *ffcli.Command {
	return newAnalyticsCapabilityCommand(
		d,
		"offers",
		"Inspect Offers tab capability.",
		"Offers tab capability",
		func(appInfo *webcore.AnalyticsAppInfoResult) analyticsPageCapabilityOutput {
			status := "unknown"
			reason := "Analytics app metadata exposes subscription support, but Apple does not publish a dedicated offer-count flag in app metadata. The Offers tab may still be disabled until subscription offers exist."
			if !analyticsAppHasFeature(appInfo, "inAppPurchases.type.subscriptions") {
				status = "unavailable"
				reason = "App metadata reports no subscriptions, so the Offers tab is unavailable."
			}
			return analyticsPageCapabilityOutput{
				AppID:            strings.TrimSpace(appInfo.AdamID),
				Page:             "offers",
				Status:           status,
				Reason:           reason,
				RelevantMeasures: []string{"subscription-state-offers", "subscription-state-offers-freeTrial", "subscription-state-offers-paidOffer", "summary-offers-net", "summary-offers-activations", "summary-offers-reactivations", "summary-offers-churned"},
				Hints: []analyticsCapabilityHint{
					{Key: "hasSubscriptions", Value: fmt.Sprintf("%t", analyticsAppHasFeature(appInfo, "inAppPurchases.type.subscriptions"))},
					{Key: "bundleId", Value: appInfo.BundleID},
				},
			}
		},
	)
}

// newWebAnalyticsBenchmarksCommand recreates the Benchmarks summary page.
func newWebAnalyticsBenchmarksCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("web analytics benchmarks", flag.ExitOnError)
	sessionFlags := bindWebSessionFlags(fs)
	output := d.BindOutputFlags(fs)
	appID := fs.String("app", "", "App ID")

	return &ffcli.Command{
		Name:       "benchmarks",
		ShortUsage: "asc web analytics benchmarks [flags]",
		ShortHelp:  "Recreate the Benchmarks summary page.",
		LongHelp: `WEB SESSION WORKFLOWS

	Show the latest available benchmark week using Apple's analytics v2 endpoints.

`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			resolvedAppID, err := resolveAnalyticsAppIDFlag(d, *appID)
			if err != nil {
				return err
			}
			session, requestCtx, cancel, err := resolveWebSessionForCommandWith(ctx, d, sessionFlags)
			defer cancel()
			if err != nil {
				return err
			}
			client := newAnalyticsClientFn(session)
			result, err := withWebSpinnerValueWith(d, "Loading benchmarks analytics", func() (*webcore.AnalyticsBenchmarksSummary, error) {
				return getAnalyticsBenchmarksFn(requestCtx, client, resolvedAppID)
			})
			if err != nil {
				return withWebAuthHint(err, "analytics benchmarks")
			}

			return d.PrintOutputWithRenderers(
				result,
				*output.Output,
				*output.Pretty,
				func() error { return renderAnalyticsBenchmarksTable(d, result) },
				func() error { return renderAnalyticsBenchmarksMarkdown(d, result) },
			)
		},
	}
}

func newAnalyticsCapabilityCommand(
	d *shared.Deps,
	name string,
	shortHelp string,
	spinnerLabel string,
	build func(appInfo *webcore.AnalyticsAppInfoResult) analyticsPageCapabilityOutput,
) *ffcli.Command {
	fs := d.NewFlagSet("web analytics "+name, flag.ExitOnError)
	sessionFlags := bindWebSessionFlags(fs)
	output := d.BindOutputFlags(fs)
	appID := fs.String("app", "", "App ID")

	return &ffcli.Command{
		Name:       name,
		ShortUsage: "asc web analytics " + name + " [flags]",
		ShortHelp:  shortHelp,
		LongHelp: `WEB SESSION WORKFLOWS

Inspect whether this analytics tab is available for the current app and show the
relevant analytics features or dimensions we observed while mapping the sidebar.

`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			resolvedAppID, err := resolveAnalyticsAppIDFlag(d, *appID)
			if err != nil {
				return err
			}
			session, requestCtx, cancel, err := resolveWebSessionForCommandWith(ctx, d, sessionFlags)
			defer cancel()
			if err != nil {
				return err
			}
			client := newAnalyticsClientFn(session)
			payload, err := withWebSpinnerValueWith(d, spinnerLabel, func() (analyticsPageCapabilityOutput, error) {
				info, err := getAnalyticsAppInfoFn(requestCtx, client, resolvedAppID)
				if err != nil {
					return analyticsPageCapabilityOutput{}, err
				}
				result := build(info)
				if strings.TrimSpace(result.AppID) == "" {
					result.AppID = resolvedAppID
				}
				return result, nil
			})
			if err != nil {
				return withWebAuthHint(err, "analytics "+name)
			}

			return d.PrintOutputWithRenderers(
				payload,
				*output.Output,
				*output.Pretty,
				func() error { return renderAnalyticsCapabilityTable(d, payload) },
				func() error { return renderAnalyticsCapabilityMarkdown(d, payload) },
			)
		},
	}
}

func resolveAnalyticsAppIDFlag(d *shared.Deps, appID string) (string, error) {
	resolvedAppID := strings.TrimSpace(d.ResolveAppID(appID))
	if resolvedAppID == "" {
		return "", d.UsageError("--app is required")
	}
	return resolvedAppID, nil
}

func analyticsAppFeatureCount(appInfo *webcore.AnalyticsAppInfoResult, featureID string) int {
	if appInfo == nil {
		return 0
	}
	featureID = strings.TrimSpace(featureID)
	for _, feature := range appInfo.Features {
		if strings.TrimSpace(feature.ID) == featureID {
			return feature.Count
		}
	}
	return 0
}

func analyticsAppHasFeature(appInfo *webcore.AnalyticsAppInfoResult, featureID string) bool {
	if appInfo == nil {
		return false
	}
	featureID = strings.TrimSpace(featureID)
	for _, feature := range appInfo.Features {
		if strings.TrimSpace(feature.ID) == featureID {
			return true
		}
	}
	return false
}

func analyticsTimeseriesGroupLabel(group any) string {
	payload, ok := group.(map[string]any)
	if !ok {
		return "-"
	}
	if title := strings.TrimSpace(fmt.Sprint(payload["title"])); title != "" && title != "<nil>" {
		return title
	}
	if key := strings.TrimSpace(fmt.Sprint(payload["key"])); key != "" && key != "<nil>" {
		return key
	}
	return "-"
}

func analyticsSeriesValue(point map[string]any, key string) *float64 {
	if point == nil {
		return nil
	}
	switch value := point[key].(type) {
	case float64:
		out := value
		return &out
	case int:
		out := float64(value)
		return &out
	default:
		return nil
	}
}

func analyticsSeriesStats(data []map[string]any, key string) (*float64, *float64, *float64) {
	if len(data) == 0 {
		return nil, nil, nil
	}
	var sum float64
	var count int
	var last *float64
	for _, point := range data {
		value := analyticsSeriesValue(point, key)
		if value == nil {
			continue
		}
		sum += *value
		count++
		copyValue := *value
		last = &copyValue
	}
	if count == 0 {
		return nil, nil, nil
	}
	total := sum
	avg := sum / float64(count)
	return &total, &avg, last
}

func buildAnalyticsSourcesRows(result *webcore.AnalyticsSourcesPage) [][]string {
	rows := [][]string{
		{"Query", "App ID", result.AppID, ""},
		{"Query", "Start", result.StartDate, ""},
		{"Query", "End", result.EndDate, ""},
		{"Query", "Metric", analyticsMeasureLabel(result.Measure), ""},
	}
	if result.Result == nil {
		return rows
	}
	for _, series := range result.Result.Results {
		total, avg, last := analyticsSeriesStats(series.Data, result.Measure)
		rows = append(rows, []string{
			"Source",
			analyticsTimeseriesGroupLabel(series.Group),
			analyticsMeasureValueString(result.Measure, avg),
			fmt.Sprintf("total=%s last=%s", analyticsMeasureValueString(result.Measure, total), analyticsMeasureValueString(result.Measure, last)),
		})
	}
	return rows
}

func renderAnalyticsSourcesTable(d *shared.Deps, result *webcore.AnalyticsSourcesPage) error {
	return renderAnalyticsTableSections(
		d,
		analyticsQuerySection(
			"Sources",
			result.AppID,
			result.StartDate,
			result.EndDate,
			[]string{"Metric", analyticsMeasureLabel(result.Measure)},
		),
		analyticsSourcesPerformanceSection(result),
	)
}

func renderAnalyticsSourcesMarkdown(d *shared.Deps, result *webcore.AnalyticsSourcesPage) error {
	headers := []string{"Section", "Field", "Value", "Notes"}
	asc.RenderMarkdownTo(d.IO.Out, headers, normalizeAnalyticsRows(buildAnalyticsSourcesRows(result), 4))
	return nil
}

func buildAnalyticsInAppEventsRows(result *webcore.AnalyticsInAppEventsPage) [][]string {
	rows := [][]string{
		{"Query", "App ID", result.AppID, ""},
		{"Query", "Requested Start", result.RequestedStartDate, ""},
		{"Query", "Requested End", result.RequestedEndDate, ""},
		{"Query", "Effective Start", result.EffectiveStartTime, ""},
		{"Query", "Effective End", result.EffectiveEndTime, ""},
	}
	if result.SelectedEventID != "" {
		rows = append(rows, []string{"Selected Event", "Event ID", result.SelectedEventID, ""})
	}
	for _, event := range result.Events {
		rows = append(rows, []string{"Event", event.Name, event.Status, fmt.Sprintf("published=%s start=%s end=%s", event.Published, event.Start, event.End)})
	}
	if result.SelectedMetrics != nil {
		rows = append(rows, buildAnalyticsMeasureRows("Selected Event Metrics", result.SelectedMetrics.Results)...)
	}
	return rows
}

func renderAnalyticsInAppEventsTable(d *shared.Deps, result *webcore.AnalyticsInAppEventsPage) error {
	var metricResults []webcore.AnalyticsMeasureResult
	if result.SelectedMetrics != nil {
		metricResults = result.SelectedMetrics.Results
	}
	return renderAnalyticsTableSections(
		d,
		analyticsQuerySection(
			"In-App Events",
			result.AppID,
			result.RequestedStartDate,
			result.RequestedEndDate,
			[]string{"Effective Range", analyticsDateRangeString(result.EffectiveStartTime, result.EffectiveEndTime)},
			[]string{"Selected Event", result.SelectedEventID},
		),
		analyticsInAppEventsListSection(result),
		analyticsMeasureSummarySection("Selected Event Metrics", metricResults),
	)
}

func renderAnalyticsInAppEventsMarkdown(d *shared.Deps, result *webcore.AnalyticsInAppEventsPage) error {
	headers := []string{"Section", "Field", "Value", "Notes", "Change"}
	asc.RenderMarkdownTo(d.IO.Out, headers, normalizeAnalyticsRows(buildAnalyticsInAppEventsRows(result), 5))
	return nil
}

func buildAnalyticsCampaignRows(result *webcore.AnalyticsCampaignsPage) [][]string {
	rows := [][]string{
		{"Query", "App ID", result.AppID, ""},
		{"Query", "Start", result.StartDate, ""},
		{"Query", "End", result.EndDate, ""},
	}
	if result.Result == nil {
		return rows
	}
	if result.Result.Size == 0 {
		rows = append(rows, []string{"Campaigns", "Status", "No campaigns data", "Apple returned zero campaign rows for this range"})
		return rows
	}
	for _, item := range result.Result.Results {
		title := strings.TrimSpace(item.SourceTitle)
		if title == "" {
			title = strings.TrimSpace(item.Title)
		}
		measureKeys := make([]string, 0, len(item.Measures))
		for key := range item.Measures {
			measureKeys = append(measureKeys, key)
		}
		sort.Strings(measureKeys)
		parts := make([]string, 0, len(measureKeys))
		for _, key := range measureKeys {
			value := item.Measures[key]
			copyValue := value
			parts = append(parts, fmt.Sprintf("%s=%s", analyticsMeasureLabel(key), analyticsMeasureValueString(key, &copyValue)))
		}
		rows = append(rows, []string{"Campaign", title, strings.Join(parts, ", "), ""})
	}
	return rows
}

func renderAnalyticsCampaignsTable(d *shared.Deps, result *webcore.AnalyticsCampaignsPage) error {
	return renderAnalyticsTableSections(
		d,
		analyticsQuerySection("Campaigns", result.AppID, result.StartDate, result.EndDate),
		analyticsCampaignPerformanceSection(result),
	)
}

func renderAnalyticsCampaignsMarkdown(d *shared.Deps, result *webcore.AnalyticsCampaignsPage) error {
	headers := []string{"Section", "Field", "Value", "Notes"}
	asc.RenderMarkdownTo(d.IO.Out, headers, normalizeAnalyticsRows(buildAnalyticsCampaignRows(result), 4))
	return nil
}

func buildAnalyticsSalesRows(result *webcore.AnalyticsSalesSummary) [][]string {
	rows := [][]string{
		{"Query", "App ID", result.AppID},
		{"Query", "Start", result.StartDate},
		{"Query", "End", result.EndDate},
	}
	rows = append(rows, buildAnalyticsMeasureRows("Sales", result.Summary)...)
	for _, row := range analyticsLatestCohortRows(result.DownloadToPaid) {
		rows = append(rows, []string{
			"Revenue Cohorts",
			fmt.Sprintf("%s %s", analyticsPeriodLabel(row.Period), analyticsMeasureLabel(row.Measure)),
			analyticsMeasureValueString(row.Measure, row.Value),
		})
	}
	for _, row := range analyticsLatestCohortRows(result.ProceedsPerDownload) {
		rows = append(rows, []string{
			"Revenue Cohorts",
			fmt.Sprintf("%s %s", analyticsPeriodLabel(row.Period), analyticsMeasureLabel(row.Measure)),
			analyticsMeasureValueString(row.Measure, row.Value),
		})
	}
	if result.RevenueByPurchase != nil {
		for _, item := range result.RevenueByPurchase.Items {
			rows = append(rows, []string{
				"Revenue Breakdown",
				result.RevenueByPurchase.Name,
				fmt.Sprintf("%s = %s", item.Label, analyticsNumberString(analyticsFloatPtr(item.Value))),
			})
		}
	}
	if result.RevenueByTerritory != nil {
		for _, item := range result.RevenueByTerritory.Items {
			rows = append(rows, []string{
				"Revenue Breakdown",
				result.RevenueByTerritory.Name,
				fmt.Sprintf("%s = %s", item.Label, analyticsNumberString(analyticsFloatPtr(item.Value))),
			})
		}
	}
	return rows
}

func renderAnalyticsSalesTable(d *shared.Deps, result *webcore.AnalyticsSalesSummary) error {
	return renderAnalyticsTableSections(
		d,
		analyticsQuerySection("Sales", result.AppID, result.StartDate, result.EndDate),
		analyticsMeasureSummarySection("Summary Cards", result.Summary),
		analyticsLatestCohortSection("Download to Paid", result.DownloadToPaid),
		analyticsLatestCohortSection("Proceeds per Download", result.ProceedsPerDownload),
		analyticsBreakdownSection("", result.RevenueByPurchase),
		analyticsBreakdownSection("", result.RevenueByTerritory),
	)
}

func renderAnalyticsSalesMarkdown(d *shared.Deps, result *webcore.AnalyticsSalesSummary) error {
	headers := []string{"Section", "Field", "Value", "Previous", "Change"}
	asc.RenderMarkdownTo(d.IO.Out, headers, normalizeAnalyticsRows(buildAnalyticsSalesRows(result), 5))
	return nil
}

func buildAnalyticsBenchmarkRows(result *webcore.AnalyticsBenchmarksSummary) [][]string {
	rows := [][]string{
		{"Query", "App ID", result.AppID, "", "", ""},
		{"Query", "Category", result.Category, "", "", ""},
		{"Query", "Week Start", result.WeekStart, "", "", ""},
		{"Query", "Week End", result.WeekEnd, "", "", ""},
		{"Query", "Peer Groups", strings.Join(result.PeerGroupIDs, ", "), "", "", ""},
	}
	for _, metric := range result.Metrics {
		rows = append(rows, []string{
			"Benchmark",
			metric.Label,
			analyticsMeasureValueString(metric.Key, metric.AppValue),
			analyticsMeasureValueString(metric.Key, metric.P25),
			analyticsMeasureValueString(metric.Key, metric.P50),
			analyticsMeasureValueString(metric.Key, metric.P75),
		})
	}
	return rows
}

func renderAnalyticsBenchmarksTable(d *shared.Deps, result *webcore.AnalyticsBenchmarksSummary) error {
	return renderAnalyticsTableSections(
		d,
		analyticsQuerySection(
			"Benchmarks",
			result.AppID,
			result.WeekStart,
			result.WeekEnd,
			[]string{"Category", result.Category},
		),
		analyticsBenchmarkPeerGroupsSection(result),
		analyticsBenchmarkMetricsSection(result),
	)
}

func renderAnalyticsBenchmarksMarkdown(d *shared.Deps, result *webcore.AnalyticsBenchmarksSummary) error {
	headers := []string{"Section", "Field", "App", "25th", "50th", "75th"}
	asc.RenderMarkdownTo(d.IO.Out, headers, normalizeAnalyticsRows(buildAnalyticsBenchmarkRows(result), 6))
	return nil
}

func buildAnalyticsCapabilityRows(payload analyticsPageCapabilityOutput) [][]string {
	rows := [][]string{
		{"Page", "App ID", payload.AppID},
		{"Page", "Name", payload.Page},
		{"Page", "Status", payload.Status},
		{"Page", "Reason", payload.Reason},
	}
	for _, dimension := range payload.RelevantDimensions {
		rows = append(rows, []string{"Relevant Dimensions", "Dimension", dimension})
	}
	for _, measure := range payload.RelevantMeasures {
		rows = append(rows, []string{"Relevant Measures", "Measure", measure})
	}
	for _, hint := range payload.Hints {
		rows = append(rows, []string{"Hints", hint.Key, hint.Value})
	}
	return rows
}

func renderAnalyticsCapabilityTable(d *shared.Deps, payload analyticsPageCapabilityOutput) error {
	return renderAnalyticsTableSections(
		d,
		analyticsCapabilityStatusSection(payload),
		analyticsCapabilityListSection("Relevant Measures", "Measure", payload.RelevantMeasures),
		analyticsCapabilityListSection("Relevant Dimensions", "Dimension", payload.RelevantDimensions),
		analyticsCapabilityHintsSection(payload),
	)
}

func renderAnalyticsCapabilityMarkdown(d *shared.Deps, payload analyticsPageCapabilityOutput) error {
	headers := []string{"Section", "Field", "Value"}
	asc.RenderMarkdownTo(d.IO.Out, headers, normalizeAnalyticsRows(buildAnalyticsCapabilityRows(payload), 3))
	return nil
}
