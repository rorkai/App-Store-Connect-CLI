package analytics

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

const analyticsMaxLimit = 200

var (
	uuidPattern               = regexp.MustCompile(`^[a-fA-F0-9]{8}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{12}$`)
	salesReportVersionPattern = regexp.MustCompile(`^[0-9]+_[0-9]+$`)
)

func normalizeSalesReportType(value string) (asc.SalesReportType, error) {
	normalized := strings.ToUpper(strings.TrimSpace(value))
	switch normalized {
	case string(asc.SalesReportTypeSales):
		return asc.SalesReportTypeSales, nil
	case string(asc.SalesReportTypePreOrder):
		return asc.SalesReportTypePreOrder, nil
	case string(asc.SalesReportTypeNewsstand):
		return asc.SalesReportTypeNewsstand, nil
	case string(asc.SalesReportTypeSubscription):
		return asc.SalesReportTypeSubscription, nil
	case string(asc.SalesReportTypeSubscriptionEvent):
		return asc.SalesReportTypeSubscriptionEvent, nil
	case string(asc.SalesReportTypeSubscriber):
		return asc.SalesReportTypeSubscriber, nil
	case string(asc.SalesReportTypeSubscriptionOfferCodeRedemption):
		return asc.SalesReportTypeSubscriptionOfferCodeRedemption, nil
	case string(asc.SalesReportTypeInstalls):
		return asc.SalesReportTypeInstalls, nil
	case string(asc.SalesReportTypeFirstAnnual):
		return asc.SalesReportTypeFirstAnnual, nil
	case string(asc.SalesReportTypeWinBackEligibility):
		return asc.SalesReportTypeWinBackEligibility, nil
	default:
		return "", fmt.Errorf("--type must be SALES, PRE_ORDER, NEWSSTAND, SUBSCRIPTION, SUBSCRIPTION_EVENT, SUBSCRIBER, SUBSCRIPTION_OFFER_CODE_REDEMPTION, INSTALLS, FIRST_ANNUAL, or WIN_BACK_ELIGIBILITY")
	}
}

func normalizeSalesReportSubType(value string) (asc.SalesReportSubType, error) {
	normalized := strings.ToUpper(strings.TrimSpace(value))
	switch normalized {
	case string(asc.SalesReportSubTypeSummary):
		return asc.SalesReportSubTypeSummary, nil
	case string(asc.SalesReportSubTypeDetailed):
		return asc.SalesReportSubTypeDetailed, nil
	case string(asc.SalesReportSubTypeSummaryInstallType):
		return asc.SalesReportSubTypeSummaryInstallType, nil
	case string(asc.SalesReportSubTypeSummaryTerritory):
		return asc.SalesReportSubTypeSummaryTerritory, nil
	case string(asc.SalesReportSubTypeSummaryChannel):
		return asc.SalesReportSubTypeSummaryChannel, nil
	default:
		return "", fmt.Errorf("--subtype must be SUMMARY, DETAILED, SUMMARY_INSTALL_TYPE, SUMMARY_TERRITORY, or SUMMARY_CHANNEL")
	}
}

func normalizeSalesReportFrequency(value string) (asc.SalesReportFrequency, error) {
	normalized := strings.ToUpper(strings.TrimSpace(value))
	switch normalized {
	case string(asc.SalesReportFrequencyDaily):
		return asc.SalesReportFrequencyDaily, nil
	case string(asc.SalesReportFrequencyWeekly):
		return asc.SalesReportFrequencyWeekly, nil
	case string(asc.SalesReportFrequencyMonthly):
		return asc.SalesReportFrequencyMonthly, nil
	case string(asc.SalesReportFrequencyYearly):
		return asc.SalesReportFrequencyYearly, nil
	default:
		return "", fmt.Errorf("--frequency must be DAILY, WEEKLY, MONTHLY, or YEARLY")
	}
}

func normalizeSalesReportVersion(value string, reportType asc.SalesReportType, reportSubType asc.SalesReportSubType, frequency asc.SalesReportFrequency) (asc.SalesReportVersion, error) {
	allowed, err := allowedSalesReportVersions(reportType, reportSubType, frequency)
	if err != nil {
		return "", err
	}
	normalized := strings.TrimSpace(value)
	if normalized == "" {
		return defaultSalesReportVersion(reportType, reportSubType, frequency), nil
	}
	if !salesReportVersionPattern.MatchString(normalized) {
		return "", fmt.Errorf("--version must use major_minor format (for example, 1_4)")
	}
	for _, version := range allowed {
		if normalized == string(version) {
			return version, nil
		}
	}
	return "", fmt.Errorf("--version %s is not supported for --type %s --subtype %s --frequency %s; allowed: %s", normalized, reportType, reportSubType, frequency, joinSalesReportVersions(allowed))
}

func defaultSalesReportVersion(reportType asc.SalesReportType, reportSubType asc.SalesReportSubType, frequency asc.SalesReportFrequency) asc.SalesReportVersion {
	if reportType == asc.SalesReportTypeSubscription {
		// Apple currently serves 1_4 in production even though the endpoint table
		// still lists 1_3. Preserve the live-verified default from PR #1842.
		return asc.SalesReportVersion1_4
	}
	versions, err := allowedSalesReportVersions(reportType, reportSubType, frequency)
	if err == nil && len(versions) > 0 {
		return versions[0]
	}
	return ""
}

func validateSalesReportTuple(reportType asc.SalesReportType, reportSubType asc.SalesReportSubType, frequency asc.SalesReportFrequency) error {
	_, err := allowedSalesReportVersions(reportType, reportSubType, frequency)
	return err
}

func allowedSalesReportVersions(reportType asc.SalesReportType, reportSubType asc.SalesReportSubType, frequency asc.SalesReportFrequency) ([]asc.SalesReportVersion, error) {
	versions := []asc.SalesReportVersion(nil)
	switch {
	case reportType == asc.SalesReportTypeFirstAnnual && reportSubType == asc.SalesReportSubTypeDetailed && frequency == asc.SalesReportFrequencyDaily:
		versions = []asc.SalesReportVersion{asc.SalesReportVersion1_0}
	case reportType == asc.SalesReportTypeFirstAnnual && reportSubType == asc.SalesReportSubTypeSummary && frequency == asc.SalesReportFrequencyYearly:
		versions = []asc.SalesReportVersion{asc.SalesReportVersion1_0}
	case reportType == asc.SalesReportTypeInstalls && frequency == asc.SalesReportFrequencyYearly &&
		(reportSubType == asc.SalesReportSubTypeSummaryChannel || reportSubType == asc.SalesReportSubTypeSummaryInstallType || reportSubType == asc.SalesReportSubTypeSummaryTerritory || reportSubType == asc.SalesReportSubTypeDetailed):
		versions = []asc.SalesReportVersion{asc.SalesReportVersion1_0, asc.SalesReportVersion1_1}
	case reportType == asc.SalesReportTypeInstalls && frequency == asc.SalesReportFrequencyMonthly &&
		(reportSubType == asc.SalesReportSubTypeSummary || reportSubType == asc.SalesReportSubTypeDetailed):
		versions = []asc.SalesReportVersion{asc.SalesReportVersion1_2}
	case reportType == asc.SalesReportTypeNewsstand && reportSubType == asc.SalesReportSubTypeDetailed &&
		(frequency == asc.SalesReportFrequencyDaily || frequency == asc.SalesReportFrequencyWeekly):
		versions = []asc.SalesReportVersion{asc.SalesReportVersion1_0}
	case reportType == asc.SalesReportTypePreOrder && reportSubType == asc.SalesReportSubTypeSummary:
		versions = []asc.SalesReportVersion{asc.SalesReportVersion1_0}
	case reportType == asc.SalesReportTypeSales && reportSubType == asc.SalesReportSubTypeSummary:
		versions = []asc.SalesReportVersion{asc.SalesReportVersion1_0}
	case reportType == asc.SalesReportTypeSubscriber && reportSubType == asc.SalesReportSubTypeDetailed && frequency == asc.SalesReportFrequencyDaily:
		versions = []asc.SalesReportVersion{asc.SalesReportVersion1_3}
	case reportType == asc.SalesReportTypeSubscription && reportSubType == asc.SalesReportSubTypeSummary && frequency == asc.SalesReportFrequencyDaily:
		versions = []asc.SalesReportVersion{asc.SalesReportVersion1_3, asc.SalesReportVersion1_4}
	case reportType == asc.SalesReportTypeSubscriptionEvent && reportSubType == asc.SalesReportSubTypeSummary && frequency == asc.SalesReportFrequencyDaily:
		versions = []asc.SalesReportVersion{asc.SalesReportVersion1_3}
	case reportType == asc.SalesReportTypeSubscriptionOfferCodeRedemption && reportSubType == asc.SalesReportSubTypeSummary && frequency == asc.SalesReportFrequencyDaily:
		versions = []asc.SalesReportVersion{asc.SalesReportVersion1_0}
	case reportType == asc.SalesReportTypeWinBackEligibility && reportSubType == asc.SalesReportSubTypeSummary && frequency == asc.SalesReportFrequencyDaily:
		versions = []asc.SalesReportVersion{asc.SalesReportVersion1_0}
	}
	if len(versions) == 0 {
		return nil, fmt.Errorf("unsupported sales report combination: --type %s --subtype %s --frequency %s", reportType, reportSubType, frequency)
	}
	return versions, nil
}

func joinSalesReportVersions(versions []asc.SalesReportVersion) string {
	values := make([]string, 0, len(versions))
	for _, version := range versions {
		values = append(values, string(version))
	}
	return strings.Join(values, ", ")
}

func normalizeAnalyticsAccessType(value string) (asc.AnalyticsAccessType, error) {
	normalized := strings.ToUpper(strings.TrimSpace(value))
	switch normalized {
	case string(asc.AnalyticsAccessTypeOngoing):
		return asc.AnalyticsAccessTypeOngoing, nil
	case string(asc.AnalyticsAccessTypeOneTimeSnapshot):
		return asc.AnalyticsAccessTypeOneTimeSnapshot, nil
	default:
		return "", fmt.Errorf("--access-type must be ONGOING or ONE_TIME_SNAPSHOT")
	}
}

func validateAnalyticsRequestID(value string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("--request-id is required")
	}
	if !uuidPattern.MatchString(strings.TrimSpace(value)) {
		return fmt.Errorf("--request-id must be a valid UUID")
	}
	return nil
}

func analyticsDownloadDefaultOutput(requestID, instanceID string) string {
	return fmt.Sprintf("analytics_report_%s_%s.csv.gz", strings.TrimSpace(requestID), sanitizeAnalyticsFilenameComponent(instanceID))
}

func sanitizeAnalyticsFilenameComponent(value string) string {
	trimmed := strings.TrimSpace(value)
	var sanitized strings.Builder
	sanitized.Grow(len(trimmed))
	for _, r := range trimmed {
		isASCIIAlpha := (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
		isDigit := r >= '0' && r <= '9'
		switch {
		case isASCIIAlpha || isDigit || r == '.' || r == '-' || r == '_':
			sanitized.WriteRune(r)
		default:
			sanitized.WriteByte('_')
		}
	}

	result := strings.Trim(sanitized.String(), "._-")
	if result == "" {
		return "instance"
	}
	return result
}

func normalizeReportDate(value string, frequency asc.SalesReportFrequency) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		if frequency == asc.SalesReportFrequencyDaily {
			return "", nil
		}
		return "", fmt.Errorf("--date is required for %s reports", frequency)
	}
	switch frequency {
	case asc.SalesReportFrequencyMonthly:
		parsed, err := time.Parse("2006-01", trimmed)
		if err == nil {
			return parsed.Format("2006-01"), nil
		}
		parsed, err = time.Parse("2006-01-02", trimmed)
		if err != nil {
			return "", fmt.Errorf("--date must be in YYYY-MM or YYYY-MM-DD format for monthly reports")
		}
		return parsed.Format("2006-01"), nil
	case asc.SalesReportFrequencyYearly:
		parsed, err := time.Parse("2006", trimmed)
		if err == nil {
			return parsed.Format("2006"), nil
		}
		parsed, err = time.Parse("2006-01-02", trimmed)
		if err != nil {
			return "", fmt.Errorf("--date must be in YYYY or YYYY-MM-DD format for yearly reports")
		}
		return parsed.Format("2006"), nil
	case asc.SalesReportFrequencyWeekly:
		parsed, err := time.Parse("2006-01-02", trimmed)
		if err != nil {
			return "", fmt.Errorf("--date must be in YYYY-MM-DD format for weekly reports")
		}
		switch parsed.Weekday() {
		case time.Monday:
			return parsed.AddDate(0, 0, 6).Format("2006-01-02"), nil
		case time.Sunday:
			return parsed.Format("2006-01-02"), nil
		default:
			return "", fmt.Errorf("--date for weekly reports must be a Monday (week start) or Sunday (week end)")
		}
	default:
		parsed, err := time.Parse("2006-01-02", trimmed)
		if err != nil {
			return "", fmt.Errorf("--date must be in YYYY-MM-DD format")
		}
		return parsed.Format("2006-01-02"), nil
	}
}

func normalizeAnalyticsProcessingDateFilter(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", fmt.Errorf("--processing-date must be in YYYY-MM-DD format")
	}
	parsed, err := time.Parse("2006-01-02", trimmed)
	if err != nil {
		return "", fmt.Errorf("--processing-date must be in YYYY-MM-DD format")
	}
	return parsed.Format("2006-01-02"), nil
}

func normalizeAnalyticsDateFilter(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", nil
	}
	parsed, err := time.Parse("2006-01-02", trimmed)
	if err != nil {
		return "", fmt.Errorf("--date must be in YYYY-MM-DD format")
	}
	return parsed.Format("2006-01-02"), nil
}

func matchAnalyticsInstanceDate(attrs asc.AnalyticsReportInstanceAttributes, date string) bool {
	if strings.TrimSpace(date) == "" {
		return true
	}
	if strings.HasPrefix(attrs.ReportDate, date) {
		return true
	}
	return strings.HasPrefix(attrs.ProcessingDate, date)
}

func normalizeAnalyticsGranularities(value string) ([]string, error) {
	parts := strings.Split(value, ",")
	seen := make(map[string]struct{}, len(parts))
	granularities := make([]string, 0, len(parts))
	for _, part := range parts {
		granularity := strings.ToUpper(strings.TrimSpace(part))
		if granularity == "" {
			return nil, fmt.Errorf("--granularity must be a comma-separated list of: DAILY, WEEKLY, MONTHLY")
		}
		switch granularity {
		case "DAILY", "WEEKLY", "MONTHLY":
		default:
			return nil, fmt.Errorf("--granularity must be a comma-separated list of: DAILY, WEEKLY, MONTHLY")
		}
		if _, exists := seen[granularity]; exists {
			continue
		}
		seen[granularity] = struct{}{}
		granularities = append(granularities, granularity)
	}
	return granularities, nil
}

func fetchAnalyticsReports(ctx context.Context, client *asc.Client, requestID string, limit int, next string, paginate bool) ([]asc.Resource[asc.AnalyticsReportAttributes], asc.Links, error) {
	var (
		all   []asc.Resource[asc.AnalyticsReportAttributes]
		links asc.Links
		seen  = make(map[string]bool)
	)

	if strings.TrimSpace(next) != "" {
		resp, err := getAnalyticsReportsPage(ctx, client, requestID, asc.WithAnalyticsReportsNextURL(next))
		if err != nil {
			return nil, asc.Links{}, err
		}
		return resp.Data, resp.Links, nil
	}

	if limit <= 0 {
		limit = analyticsMaxLimit
	}
	nextURL := ""
	for {
		var resp *asc.AnalyticsReportsResponse
		var err error
		if nextURL != "" {
			if seen[nextURL] {
				return nil, asc.Links{}, fmt.Errorf("analytics view: detected repeated pagination URL")
			}
			seen[nextURL] = true
			resp, err = getAnalyticsReportsPage(ctx, client, requestID, asc.WithAnalyticsReportsNextURL(nextURL))
		} else {
			resp, err = getAnalyticsReportsPage(ctx, client, requestID, asc.WithAnalyticsReportsLimit(limit))
		}
		if err != nil {
			return nil, asc.Links{}, err
		}
		if links.Self == "" {
			links.Self = resp.Links.Self
		}
		all = append(all, resp.Data...)
		links.Next = resp.Links.Next
		if !paginate || resp.Links.Next == "" {
			break
		}
		nextURL = resp.Links.Next
	}
	return all, links, nil
}

func fetchAnalyticsReportInstances(ctx context.Context, client *asc.Client, reportID string, opts ...asc.AnalyticsReportInstancesOption) ([]asc.Resource[asc.AnalyticsReportInstanceAttributes], error) {
	var (
		all  []asc.Resource[asc.AnalyticsReportInstanceAttributes]
		next string
		seen = make(map[string]bool)
	)
	for {
		var resp *asc.AnalyticsReportInstancesResponse
		var err error
		if next != "" {
			if seen[next] {
				return nil, fmt.Errorf("analytics view: detected repeated instance pagination URL")
			}
			seen[next] = true
			resp, err = getAnalyticsReportInstancesPage(ctx, client, reportID, asc.WithAnalyticsReportInstancesNextURL(next))
		} else {
			firstPageOpts := make([]asc.AnalyticsReportInstancesOption, 0, len(opts)+1)
			firstPageOpts = append(firstPageOpts, asc.WithAnalyticsReportInstancesLimit(analyticsMaxLimit))
			firstPageOpts = append(firstPageOpts, opts...)
			resp, err = getAnalyticsReportInstancesPage(ctx, client, reportID, firstPageOpts...)
		}
		if err != nil {
			return nil, err
		}
		all = append(all, resp.Data...)
		if resp.Links.Next == "" {
			break
		}
		next = resp.Links.Next
	}
	return all, nil
}

func normalizeCompareDateRange(from, fromEnd string, freq asc.SalesReportFrequency, fromFlag, fromEndFlag string) (string, string, error) {
	from = strings.TrimSpace(from)
	fromEnd = strings.TrimSpace(fromEnd)
	if fromEnd == "" {
		fromEnd = from
	}

	start, startTime, err := normalizeCompareRangeBoundary(from, freq, fromFlag)
	if err != nil {
		return "", "", err
	}
	end, endTime, err := normalizeCompareRangeBoundary(fromEnd, freq, fromEndFlag)
	if err != nil {
		return "", "", err
	}
	if endTime.Before(startTime) {
		return "", "", fmt.Errorf("%s must not be before %s", fromEndFlag, fromFlag)
	}
	return start, end, nil
}

func normalizeCompareRangeBoundary(value string, freq asc.SalesReportFrequency, flagName string) (string, time.Time, error) {
	trimmed := strings.TrimSpace(value)
	switch freq {
	case asc.SalesReportFrequencyYearly:
		parsed, err := time.Parse("2006", trimmed)
		if err != nil {
			return "", time.Time{}, fmt.Errorf("%s must be in YYYY format for yearly frequency", flagName)
		}
		return parsed.Format("2006"), parsed, nil

	case asc.SalesReportFrequencyMonthly:
		parsed, err := time.Parse("2006-01", trimmed)
		if err != nil {
			return "", time.Time{}, fmt.Errorf("%s must be in YYYY-MM format for monthly frequency", flagName)
		}
		return parsed.Format("2006-01"), parsed, nil

	case asc.SalesReportFrequencyWeekly:
		return normalizeWeeklyCompareBoundary(trimmed, flagName)

	default:
		parsed, err := time.Parse("2006-01-02", trimmed)
		if err != nil {
			return "", time.Time{}, fmt.Errorf("%s must be in YYYY-MM-DD format", flagName)
		}
		return parsed.Format("2006-01-02"), parsed, nil
	}
}

func normalizeWeeklyCompareBoundary(value, flagName string) (string, time.Time, error) {
	parsed, err := time.Parse("2006-01-02", strings.TrimSpace(value))
	if err != nil {
		return "", time.Time{}, fmt.Errorf("%s must be in YYYY-MM-DD format for weekly reports", flagName)
	}
	switch parsed.Weekday() {
	case time.Monday:
		weekEnd := parsed.AddDate(0, 0, 6)
		return weekEnd.Format("2006-01-02"), weekEnd, nil
	case time.Sunday:
		return parsed.Format("2006-01-02"), parsed, nil
	default:
		return "", time.Time{}, fmt.Errorf("%s for weekly reports must be a Monday (week start) or Sunday (week end)", flagName)
	}
}

func generateReportDates(start, end string, freq asc.SalesReportFrequency) ([]string, error) {
	switch freq {
	case asc.SalesReportFrequencyYearly:
		s, err := time.Parse("2006", start)
		if err != nil {
			return nil, fmt.Errorf("invalid start year %q", start)
		}
		e, err := time.Parse("2006", end)
		if err != nil {
			return nil, fmt.Errorf("invalid end year %q", end)
		}
		var dates []string
		for cur := s; !cur.After(e); cur = cur.AddDate(1, 0, 0) {
			reportDate, normErr := normalizeReportDate(cur.Format("2006"), asc.SalesReportFrequencyYearly)
			if normErr != nil {
				return nil, normErr
			}
			dates = append(dates, reportDate)
		}
		return dates, nil

	case asc.SalesReportFrequencyMonthly:
		s, err := time.Parse("2006-01", start)
		if err != nil {
			return nil, fmt.Errorf("invalid start month %q", start)
		}
		e, err := time.Parse("2006-01", end)
		if err != nil {
			return nil, fmt.Errorf("invalid end month %q", end)
		}
		var dates []string
		for cur := s; !cur.After(e); cur = cur.AddDate(0, 1, 0) {
			reportDate, normErr := normalizeReportDate(cur.Format("2006-01"), asc.SalesReportFrequencyMonthly)
			if normErr != nil {
				return nil, normErr
			}
			dates = append(dates, reportDate)
		}
		return dates, nil

	case asc.SalesReportFrequencyWeekly:
		s, err := time.Parse("2006-01-02", start)
		if err != nil {
			return nil, fmt.Errorf("invalid start date %q", start)
		}
		e, err := time.Parse("2006-01-02", end)
		if err != nil {
			return nil, fmt.Errorf("invalid end date %q", end)
		}
		var dates []string
		for cur := s; !cur.After(e); cur = cur.AddDate(0, 0, 7) {
			reportDate, normErr := normalizeReportDate(cur.Format("2006-01-02"), asc.SalesReportFrequencyWeekly)
			if normErr != nil {
				return nil, normErr
			}
			dates = append(dates, reportDate)
		}
		return dates, nil

	default:
		s, err := time.Parse("2006-01-02", start)
		if err != nil {
			return nil, fmt.Errorf("invalid start date %q", start)
		}
		e, err := time.Parse("2006-01-02", end)
		if err != nil {
			return nil, fmt.Errorf("invalid end date %q", end)
		}
		var dates []string
		for cur := s; !cur.After(e); cur = cur.AddDate(0, 0, 1) {
			dates = append(dates, cur.Format("2006-01-02"))
		}
		return dates, nil
	}
}

func fetchAnalyticsReportSegments(ctx context.Context, client *asc.Client, instanceID string) ([]asc.Resource[asc.AnalyticsReportSegmentAttributes], error) {
	var (
		all  []asc.Resource[asc.AnalyticsReportSegmentAttributes]
		next string
		seen = make(map[string]bool)
	)
	for {
		var resp *asc.AnalyticsReportSegmentsResponse
		var err error
		if next != "" {
			if seen[next] {
				return nil, fmt.Errorf("analytics view: detected repeated segment pagination URL")
			}
			seen[next] = true
			resp, err = getAnalyticsReportSegmentsPage(ctx, client, instanceID, asc.WithAnalyticsReportSegmentsNextURL(next))
		} else {
			resp, err = getAnalyticsReportSegmentsPage(ctx, client, instanceID, asc.WithAnalyticsReportSegmentsLimit(analyticsMaxLimit))
		}
		if err != nil {
			return nil, err
		}
		all = append(all, resp.Data...)
		if resp.Links.Next == "" {
			break
		}
		next = resp.Links.Next
	}
	return all, nil
}

func getAnalyticsReportsPage(ctx context.Context, client *asc.Client, requestID string, opts ...asc.AnalyticsReportsOption) (*asc.AnalyticsReportsResponse, error) {
	requestCtx, cancel := shared.ContextWithTimeout(ctx)
	defer cancel()
	return client.GetAnalyticsReports(requestCtx, requestID, opts...)
}

func getAnalyticsReportInstancesPage(ctx context.Context, client *asc.Client, reportID string, opts ...asc.AnalyticsReportInstancesOption) (*asc.AnalyticsReportInstancesResponse, error) {
	requestCtx, cancel := shared.ContextWithTimeout(ctx)
	defer cancel()
	return client.GetAnalyticsReportInstances(requestCtx, reportID, opts...)
}

func getAnalyticsReportSegmentsPage(ctx context.Context, client *asc.Client, instanceID string, opts ...asc.AnalyticsReportSegmentsOption) (*asc.AnalyticsReportSegmentsResponse, error) {
	requestCtx, cancel := shared.ContextWithTimeout(ctx)
	defer cancel()
	return client.GetAnalyticsReportSegments(requestCtx, instanceID, opts...)
}

func downloadAnalyticsReportToFile(ctx context.Context, client *asc.Client, downloadURL, outputPath string) (int64, error) {
	requestCtx, cancel := shared.ContextWithTimeout(ctx)
	defer cancel()

	download, err := client.DownloadAnalyticsReport(requestCtx, downloadURL)
	if err != nil {
		return 0, fmt.Errorf("failed to download report: %w", err)
	}
	defer download.Body.Close()

	size, err := shared.WriteStreamToFile(outputPath, download.Body)
	if err != nil {
		return 0, fmt.Errorf("failed to write report: %w", err)
	}
	return size, nil
}
