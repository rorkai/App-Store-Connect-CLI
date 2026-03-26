package web

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

type analyticsTableSection struct {
	Title   string
	Headers []string
	Rows    [][]string
}

func renderAnalyticsTableSections(sections ...analyticsTableSection) error {
	printed := false
	for _, section := range sections {
		if len(section.Headers) == 0 {
			continue
		}
		rows := normalizeAnalyticsRows(section.Rows, len(section.Headers))
		if len(rows) == 0 {
			continue
		}
		if printed {
			fmt.Println()
		}
		title := strings.TrimSpace(section.Title)
		if title != "" {
			fmt.Println(title)
			fmt.Println(strings.Repeat("-", len(title)))
		}
		asc.RenderTable(section.Headers, rows)
		printed = true
	}
	return nil
}

func analyticsFieldValueSection(title string, rows [][]string) analyticsTableSection {
	return analyticsTableSection{
		Title:   title,
		Headers: []string{"Field", "Value"},
		Rows:    normalizeAnalyticsRows(rows, 2),
	}
}

func analyticsQuerySection(title, appID, startDate, endDate string, extras ...[]string) analyticsTableSection {
	rows := [][]string{{"App ID", strings.TrimSpace(appID)}}
	if dateRange := analyticsDateRangeString(startDate, endDate); dateRange != "" {
		rows = append(rows, []string{"Date Range", dateRange})
	}
	for _, extra := range extras {
		if len(extra) < 2 {
			continue
		}
		label := strings.TrimSpace(extra[0])
		value := strings.TrimSpace(extra[1])
		if label == "" || value == "" {
			continue
		}
		rows = append(rows, []string{label, value})
	}
	return analyticsFieldValueSection(title, rows)
}

func analyticsMeasureSummarySection(title string, results []webcore.AnalyticsMeasureResult) analyticsTableSection {
	rows := make([][]string, 0, len(results))
	for _, result := range results {
		rows = append(rows, []string{
			analyticsMeasureLabel(result.Measure),
			analyticsMeasureValueString(result.Measure, result.Total),
			analyticsMeasureValueString(result.Measure, result.PreviousTotal),
			analyticsPercentChangeString(result.PercentChange),
		})
	}
	return analyticsTableSection{
		Title:   title,
		Headers: []string{"Metric", "Value", "Previous", "Change"},
		Rows:    normalizeAnalyticsRows(rows, 4),
	}
}

func analyticsLatestCohortSection(title string, resp *webcore.AnalyticsCohortsResponse) analyticsTableSection {
	rows := make([][]string, 0)
	for _, row := range analyticsLatestCohortRows(resp) {
		rows = append(rows, []string{
			analyticsPeriodLabel(row.Period),
			analyticsMeasureValueString(row.Measure, row.Value),
		})
	}
	return analyticsTableSection{
		Title:   title,
		Headers: []string{"Period", "Value"},
		Rows:    normalizeAnalyticsRows(rows, 2),
	}
}

func analyticsLatestTimelineSection(title string, results []webcore.AnalyticsTimeseriesResult) analyticsTableSection {
	rows := make([][]string, 0)
	for _, result := range results {
		if result.Group != nil || len(result.Data) == 0 {
			continue
		}
		latest := result.Data[len(result.Data)-1]
		keys := make([]string, 0, len(latest))
		for key, value := range latest {
			if key == "date" || value == nil {
				continue
			}
			keys = append(keys, key)
		}
		sort.Slice(keys, func(i, j int) bool {
			return analyticsMeasureSortLess(keys[i], keys[j])
		})
		for _, key := range keys {
			value := analyticsSeriesValue(latest, key)
			if value == nil {
				continue
			}
			rows = append(rows, []string{
				analyticsMeasureLabel(key),
				analyticsDisplayDate(fmt.Sprint(latest["date"])),
				analyticsMeasureValueString(key, value),
			})
		}
	}
	return analyticsTableSection{
		Title:   title,
		Headers: []string{"Metric", "Latest Date", "Value"},
		Rows:    normalizeAnalyticsRows(rows, 3),
	}
}

func analyticsBreakdownSection(title string, breakdown *webcore.AnalyticsBreakdown) analyticsTableSection {
	if breakdown == nil {
		return analyticsTableSection{}
	}
	if strings.TrimSpace(title) == "" {
		title = breakdown.Name
	}
	items := append([]webcore.AnalyticsBreakdownItem(nil), breakdown.Items...)
	sort.Slice(items, func(i, j int) bool {
		if items[i].Value != items[j].Value {
			return items[i].Value > items[j].Value
		}
		return analyticsBreakdownItemLabel(items[i]) < analyticsBreakdownItemLabel(items[j])
	})
	rows := make([][]string, 0, len(items))
	for idx, item := range items {
		rows = append(rows, []string{
			fmt.Sprintf("%d", idx+1),
			analyticsBreakdownItemLabel(item),
			analyticsFloatValueString(breakdown.Measure, item.Value),
		})
	}
	return analyticsTableSection{
		Title:   title,
		Headers: []string{"Rank", analyticsDimensionLabel(breakdown.Dimension), "Value"},
		Rows:    normalizeAnalyticsRows(rows, 3),
	}
}

func analyticsSourcesPerformanceSection(result *webcore.AnalyticsSourcesPage) analyticsTableSection {
	if result == nil || result.Result == nil {
		return analyticsTableSection{}
	}
	title := fmt.Sprintf("%s by %s", analyticsMeasureLabel(result.Measure), analyticsDimensionLabel(result.GroupDimension))
	if len(result.Result.Results) == 0 {
		return analyticsFieldValueSection(title, [][]string{
			{"Status", "No source data for this date range"},
		})
	}
	type sourceRow struct {
		Label string
		Total *float64
		Avg   *float64
		Last  *float64
	}
	stats := make([]sourceRow, 0, len(result.Result.Results))
	for _, series := range result.Result.Results {
		total, avg, last := analyticsSeriesStats(series.Data, result.Measure)
		stats = append(stats, sourceRow{
			Label: analyticsTimeseriesGroupLabel(series.Group),
			Total: total,
			Avg:   avg,
			Last:  last,
		})
	}
	sort.Slice(stats, func(i, j int) bool {
		leftTotal := analyticsPointerValue(stats[i].Total)
		rightTotal := analyticsPointerValue(stats[j].Total)
		if leftTotal != rightTotal {
			return leftTotal > rightTotal
		}
		return stats[i].Label < stats[j].Label
	})
	rows := make([][]string, 0, len(stats))
	for _, stat := range stats {
		rows = append(rows, []string{
			stat.Label,
			analyticsMeasureValueString(result.Measure, stat.Total),
			analyticsMeasureValueString(result.Measure, stat.Avg),
			analyticsMeasureValueString(result.Measure, stat.Last),
		})
	}
	return analyticsTableSection{
		Title:   title,
		Headers: []string{analyticsDimensionLabel(result.GroupDimension), "Total", "Average / Day", "Latest"},
		Rows:    normalizeAnalyticsRows(rows, 4),
	}
}

func analyticsInAppEventsListSection(result *webcore.AnalyticsInAppEventsPage) analyticsTableSection {
	if result == nil {
		return analyticsTableSection{}
	}
	rows := make([][]string, 0, len(result.Events))
	for _, event := range result.Events {
		rows = append(rows, []string{
			event.Name,
			event.Status,
			analyticsDisplayDate(event.Published),
			analyticsDisplayDate(event.Start),
			analyticsDisplayDate(event.End),
		})
	}
	return analyticsTableSection{
		Title:   "Events",
		Headers: []string{"Event", "Status", "Published", "Start", "End"},
		Rows:    normalizeAnalyticsRows(rows, 5),
	}
}

func analyticsCampaignPerformanceSection(result *webcore.AnalyticsCampaignsPage) analyticsTableSection {
	if result == nil || result.Result == nil {
		return analyticsTableSection{}
	}
	if len(result.Result.Results) == 0 {
		return analyticsFieldValueSection("Campaign Performance", [][]string{
			{"Status", "No campaign data for this date range"},
		})
	}
	measureSet := map[string]struct{}{}
	for _, item := range result.Result.Results {
		for key := range item.Measures {
			measureSet[key] = struct{}{}
		}
	}
	measureKeys := make([]string, 0, len(measureSet))
	for key := range measureSet {
		measureKeys = append(measureKeys, key)
	}
	sort.Slice(measureKeys, func(i, j int) bool {
		return analyticsMeasureSortLess(measureKeys[i], measureKeys[j])
	})
	headers := []string{"Campaign"}
	for _, key := range measureKeys {
		headers = append(headers, analyticsMeasureLabel(key))
	}
	type campaignRow struct {
		Label    string
		Measures map[string]float64
	}
	campaigns := make([]campaignRow, 0, len(result.Result.Results))
	for _, item := range result.Result.Results {
		campaigns = append(campaigns, campaignRow{
			Label:    analyticsCampaignLabel(item),
			Measures: item.Measures,
		})
	}
	sort.Slice(campaigns, func(i, j int) bool {
		leftDownloads := campaigns[i].Measures["totalDownloads"]
		rightDownloads := campaigns[j].Measures["totalDownloads"]
		if leftDownloads != rightDownloads {
			return leftDownloads > rightDownloads
		}
		return campaigns[i].Label < campaigns[j].Label
	})
	rows := make([][]string, 0, len(campaigns))
	for _, campaign := range campaigns {
		row := []string{campaign.Label}
		for _, key := range measureKeys {
			value, ok := campaign.Measures[key]
			if !ok {
				row = append(row, "-")
				continue
			}
			row = append(row, analyticsFloatValueString(key, value))
		}
		rows = append(rows, row)
	}
	return analyticsTableSection{
		Title:   "Campaign Performance",
		Headers: headers,
		Rows:    normalizeAnalyticsRows(rows, len(headers)),
	}
}

func analyticsBenchmarkPeerGroupsSection(result *webcore.AnalyticsBenchmarksSummary) analyticsTableSection {
	if result == nil {
		return analyticsTableSection{}
	}
	rows := make([][]string, 0, len(result.SelectedGroups))
	for _, group := range result.SelectedGroups {
		rows = append(rows, []string{
			group.Title,
			group.ID,
			group.Monetization,
			group.Size,
			analyticsBooleanString(group.MemberOf),
			analyticsBooleanString(group.Primary),
		})
	}
	return analyticsTableSection{
		Title:   "Peer Groups",
		Headers: []string{"Peer Group", "ID", "Monetization", "Size", "Member", "Primary"},
		Rows:    normalizeAnalyticsRows(rows, 6),
	}
}

func analyticsBenchmarkMetricsSection(result *webcore.AnalyticsBenchmarksSummary) analyticsTableSection {
	if result == nil {
		return analyticsTableSection{}
	}
	rows := make([][]string, 0, len(result.Metrics))
	for _, metric := range result.Metrics {
		rows = append(rows, []string{
			metric.Label,
			analyticsMeasureValueString(metric.Key, metric.AppValue),
			analyticsMeasureValueString(metric.Key, metric.P25),
			analyticsMeasureValueString(metric.Key, metric.P50),
			analyticsMeasureValueString(metric.Key, metric.P75),
		})
	}
	return analyticsTableSection{
		Title:   "Benchmark Metrics",
		Headers: []string{"Metric", "App", "25th", "50th", "75th"},
		Rows:    normalizeAnalyticsRows(rows, 5),
	}
}

func analyticsCapabilityStatusSection(payload analyticsPageCapabilityOutput) analyticsTableSection {
	return analyticsFieldValueSection("Tab Availability", [][]string{
		{"App ID", payload.AppID},
		{"Page", payload.Page},
		{"Status", payload.Status},
		{"Reason", payload.Reason},
	})
}

func analyticsCapabilityListSection(title, label string, values []string) analyticsTableSection {
	rows := make([][]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		rows = append(rows, []string{value})
	}
	return analyticsTableSection{
		Title:   title,
		Headers: []string{label},
		Rows:    normalizeAnalyticsRows(rows, 1),
	}
}

func analyticsCapabilityHintsSection(payload analyticsPageCapabilityOutput) analyticsTableSection {
	rows := make([][]string, 0, len(payload.Hints))
	for _, hint := range payload.Hints {
		if strings.TrimSpace(hint.Key) == "" || strings.TrimSpace(hint.Value) == "" {
			continue
		}
		rows = append(rows, []string{hint.Key, hint.Value})
	}
	return analyticsTableSection{
		Title:   "Hints",
		Headers: []string{"Key", "Value"},
		Rows:    normalizeAnalyticsRows(rows, 2),
	}
}

func analyticsRetentionDataSection(payload analyticsRetentionOutput) analyticsTableSection {
	rows := make([][]string, 0)
	if payload.Result != nil {
		for _, cohort := range payload.Result.Results {
			for _, point := range cohort.Data {
				rows = append(rows, []string{
					analyticsDisplayDate(cohort.AppPurchase),
					analyticsDisplayDate(point.Date),
					analyticsMeasureValueString("cohort-subscription-retention-rate", point.RetentionPercentage),
					analyticsNumberString(point.Value),
				})
			}
		}
	}
	return analyticsTableSection{
		Title:   "Retention Data",
		Headers: []string{"Cohort", "Date", "Retention", "Value"},
		Rows:    normalizeAnalyticsRows(rows, 4),
	}
}

func analyticsCohortDataSection(payload analyticsCohortsOutput) analyticsTableSection {
	rows := make([][]string, 0)
	if payload.Result != nil && len(payload.Result.Results) > 0 {
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
					analyticsDisplayDate(fmt.Sprint(dates[i])),
					analyticsPeriodLabel(fmt.Sprint(periods[i])),
					analyticsMeasureLabel(measure),
					analyticsAnyValueString(measure, values[i]),
				})
			}
		}
	}
	return analyticsTableSection{
		Title:   "Cohort Data",
		Headers: []string{"Date", "Period", "Measure", "Value"},
		Rows:    normalizeAnalyticsRows(rows, 4),
	}
}

func analyticsDateRangeString(startDate, endDate string) string {
	start := analyticsDisplayDate(startDate)
	end := analyticsDisplayDate(endDate)
	switch {
	case start == "" && end == "":
		return ""
	case start == "":
		return end
	case end == "":
		return start
	default:
		return start + " to " + end
	}
}

func analyticsDisplayDate(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" || value == "<nil>" {
		return ""
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02"} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed.Format("2006-01-02")
		}
	}
	return value
}

func analyticsDimensionLabel(dimension string) string {
	switch strings.TrimSpace(dimension) {
	case "source":
		return "Source"
	case "campaignId":
		return "Campaign"
	case "inAppEvent":
		return "In-App Event"
	case "appVersion":
		return "App Version"
	case "inAppPurchase":
		return "In-App Purchase"
	case "storefront":
		return "Territory"
	case "subscriptionPlanId":
		return "Subscription"
	case "productPage":
		return "Product Page"
	default:
		return "Item"
	}
}

func analyticsBreakdownItemLabel(item webcore.AnalyticsBreakdownItem) string {
	if label := strings.TrimSpace(item.Label); label != "" {
		return label
	}
	if key := strings.TrimSpace(item.Key); key != "" {
		return key
	}
	return "-"
}

func analyticsCampaignLabel(item webcore.AnalyticsSourcesListItem) string {
	if title := strings.TrimSpace(item.SourceTitle); title != "" {
		return title
	}
	if title := strings.TrimSpace(item.Title); title != "" {
		return title
	}
	if sourceID := strings.TrimSpace(item.SourceID); sourceID != "" {
		return sourceID
	}
	return "-"
}

func analyticsMeasureSortLess(left, right string) bool {
	leftRank := analyticsMeasureSortRank(left)
	rightRank := analyticsMeasureSortRank(right)
	if leftRank != rightRank {
		return leftRank < rightRank
	}
	return analyticsMeasureLabel(left) < analyticsMeasureLabel(right)
}

func analyticsMeasureSortRank(measure string) int {
	switch strings.TrimSpace(measure) {
	case "units":
		return 10
	case "redownloads":
		return 20
	case "totalDownloads":
		return 30
	case "conversionRate", "benchConversionRate":
		return 40
	case "impressionsTotal":
		return 50
	case "pageViewCount":
		return 60
	case "pageViewUnique":
		return 70
	case "updates":
		return 80
	case "eventImpressions":
		return 90
	case "eventOpens":
		return 100
	case "proceeds":
		return 110
	case "payingUsers":
		return 120
	case "iap":
		return 130
	case "sessions":
		return 140
	case "subscription-state-plans-active":
		return 150
	case "subscription-state-paid":
		return 160
	case "revenue-recurring":
		return 170
	case "summary-plans-paid-net":
		return 180
	case "summary-plans-paid-starts":
		return 190
	case "summary-plans-paid-churned":
		return 200
	default:
		return 1000
	}
}

func analyticsFloatValueString(measure string, value float64) string {
	copyValue := value
	return analyticsMeasureValueString(measure, &copyValue)
}

func analyticsPointerValue(value *float64) float64 {
	if value == nil {
		return 0
	}
	return *value
}

func analyticsBooleanString(value bool) string {
	if value {
		return "Yes"
	}
	return "No"
}
