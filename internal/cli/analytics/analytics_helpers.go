package analytics

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

const analyticsMaxLimit = 200

var uuidPattern = regexp.MustCompile(`^[a-fA-F0-9]{8}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{12}$`)

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
	default:
		return "", fmt.Errorf("--type must be SALES, PRE_ORDER, NEWSSTAND, SUBSCRIPTION, or SUBSCRIPTION_EVENT")
	}
}

func normalizeSalesReportSubType(value string) (asc.SalesReportSubType, error) {
	normalized := strings.ToUpper(strings.TrimSpace(value))
	switch normalized {
	case string(asc.SalesReportSubTypeSummary):
		return asc.SalesReportSubTypeSummary, nil
	case string(asc.SalesReportSubTypeDetailed):
		return asc.SalesReportSubTypeDetailed, nil
	default:
		return "", fmt.Errorf("--subtype must be SUMMARY or DETAILED")
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

func normalizeSalesReportVersion(value string) (asc.SalesReportVersion, error) {
	normalized := strings.TrimSpace(value)
	if normalized == "" {
		return asc.SalesReportVersion1_0, nil
	}
	switch normalized {
	case string(asc.SalesReportVersion1_0):
		return asc.SalesReportVersion1_0, nil
	case string(asc.SalesReportVersion1_1):
		return asc.SalesReportVersion1_1, nil
	case string(asc.SalesReportVersion1_3):
		return asc.SalesReportVersion1_3, nil
	default:
		return "", fmt.Errorf("--version must be 1_0, 1_1, or 1_3")
	}
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

func normalizeAnalyticsRequestState(value string) (asc.AnalyticsReportRequestState, error) {
	normalized := strings.ToUpper(strings.TrimSpace(value))
	switch normalized {
	case string(asc.AnalyticsReportRequestStateProcessing):
		return asc.AnalyticsReportRequestStateProcessing, nil
	case string(asc.AnalyticsReportRequestStateCompleted):
		return asc.AnalyticsReportRequestStateCompleted, nil
	case string(asc.AnalyticsReportRequestStateFailed):
		return asc.AnalyticsReportRequestStateFailed, nil
	default:
		return "", fmt.Errorf("--state must be PROCESSING, COMPLETED, or FAILED")
	}
}

func validateUUIDFlag(flagName, value string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("%s is required", flagName)
	}
	if !uuidPattern.MatchString(strings.TrimSpace(value)) {
		return fmt.Errorf("%s must be a valid UUID", flagName)
	}
	return nil
}

func normalizeReportDate(value string, frequency asc.SalesReportFrequency) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", fmt.Errorf("--date is required")
	}
	switch frequency {
	case asc.SalesReportFrequencyMonthly:
		parsed, err := time.Parse("2006-01", trimmed)
		if err != nil {
			return "", fmt.Errorf("--date must be in YYYY-MM format for monthly reports")
		}
		return parsed.Format("2006-01"), nil
	case asc.SalesReportFrequencyYearly:
		parsed, err := time.Parse("2006", trimmed)
		if err != nil {
			return "", fmt.Errorf("--date must be in YYYY format for yearly reports")
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

func fetchAnalyticsReports(ctx context.Context, client *asc.Client, requestID string, limit int, next string, paginate bool) ([]asc.Resource[asc.AnalyticsReportAttributes], asc.Links, error) {
	var (
		all   []asc.Resource[asc.AnalyticsReportAttributes]
		links asc.Links
		seen  = make(map[string]bool)
	)

	if strings.TrimSpace(next) != "" {
		resp, err := client.GetAnalyticsReports(ctx, requestID, asc.WithAnalyticsReportsNextURL(next))
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
				return nil, asc.Links{}, fmt.Errorf("analytics get: detected repeated pagination URL")
			}
			seen[nextURL] = true
			resp, err = client.GetAnalyticsReports(ctx, requestID, asc.WithAnalyticsReportsNextURL(nextURL))
		} else {
			resp, err = client.GetAnalyticsReports(ctx, requestID, asc.WithAnalyticsReportsLimit(limit))
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

func fetchAnalyticsReportInstances(ctx context.Context, client *asc.Client, reportID string) ([]asc.Resource[asc.AnalyticsReportInstanceAttributes], error) {
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
				return nil, fmt.Errorf("analytics get: detected repeated instance pagination URL")
			}
			seen[next] = true
			resp, err = client.GetAnalyticsReportInstances(ctx, reportID, asc.WithAnalyticsReportInstancesNextURL(next))
		} else {
			resp, err = client.GetAnalyticsReportInstances(ctx, reportID, asc.WithAnalyticsReportInstancesLimit(analyticsMaxLimit))
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

func normalizeCompareDateRange(from, fromEnd string, freq asc.SalesReportFrequency) (string, string, error) {
	from = strings.TrimSpace(from)
	fromEnd = strings.TrimSpace(fromEnd)
	if fromEnd == "" {
		fromEnd = from
	}

	switch freq {
	case asc.SalesReportFrequencyYearly:
		start, err := time.Parse("2006", from)
		if err != nil {
			return "", "", fmt.Errorf("--from must be in YYYY format for yearly frequency")
		}
		end, err := time.Parse("2006", fromEnd)
		if err != nil {
			return "", "", fmt.Errorf("--from-end must be in YYYY format for yearly frequency")
		}
		if end.Before(start) {
			return "", "", fmt.Errorf("--from-end must not be before --from")
		}
		return start.Format("2006"), end.Format("2006"), nil

	case asc.SalesReportFrequencyMonthly:
		start, err := time.Parse("2006-01", from)
		if err != nil {
			return "", "", fmt.Errorf("--from must be in YYYY-MM format for monthly frequency")
		}
		end, err := time.Parse("2006-01", fromEnd)
		if err != nil {
			return "", "", fmt.Errorf("--from-end must be in YYYY-MM format for monthly frequency")
		}
		if end.Before(start) {
			return "", "", fmt.Errorf("--from-end must not be before --from")
		}
		return start.Format("2006-01"), end.Format("2006-01"), nil

	default:
		start, err := time.Parse("2006-01-02", from)
		if err != nil {
			return "", "", fmt.Errorf("--from must be in YYYY-MM-DD format")
		}
		end, err := time.Parse("2006-01-02", fromEnd)
		if err != nil {
			return "", "", fmt.Errorf("--from-end must be in YYYY-MM-DD format")
		}
		if end.Before(start) {
			return "", "", fmt.Errorf("--from-end must not be before --from")
		}
		return start.Format("2006-01-02"), end.Format("2006-01-02"), nil
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
			dates = append(dates, cur.Format("2006"))
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
			dates = append(dates, cur.Format("2006-01"))
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
				return nil, fmt.Errorf("analytics get: detected repeated segment pagination URL")
			}
			seen[next] = true
			resp, err = client.GetAnalyticsReportSegments(ctx, instanceID, asc.WithAnalyticsReportSegmentsNextURL(next))
		} else {
			resp, err = client.GetAnalyticsReportSegments(ctx, instanceID, asc.WithAnalyticsReportSegmentsLimit(analyticsMaxLimit))
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
