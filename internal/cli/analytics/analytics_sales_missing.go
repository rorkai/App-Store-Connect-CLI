package analytics

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// salesReportPacific approximates Pacific Time, in which Apple dates and
// publishes Sales and Trends reports. Standard time is used all year, so in
// summer a report is treated as published an hour later than Apple's
// "by 8 a.m. PT".
var salesReportPacific = time.FixedZone("PST", -8*60*60)

// salesReportPublishHour is the Pacific hour by which Apple says its Sales and
// Trends reports are generally available.
const salesReportPublishHour = 8

// salesReportRetry holds the flags of the failed request that a suggested
// command repeats with another --date.
type salesReportRetry struct {
	vendor     string
	reportType asc.SalesReportType
	subType    asc.SalesReportSubType
	frequency  asc.SalesReportFrequency
	version    asc.SalesReportVersion
	decompress bool
}

func (r salesReportRetry) command(date string) (string, bool) {
	parts := []string{"asc analytics sales"}
	if r.vendor != "" {
		quoted, ok := shared.ShellQuote(r.vendor)
		if !ok {
			return "", false
		}
		parts = append(parts, "--vendor "+quoted)
	}
	parts = append(parts,
		"--type "+string(r.reportType),
		"--subtype "+string(r.subType),
		"--frequency "+string(r.frequency),
		"--date "+date,
	)
	if r.version != "" {
		parts = append(parts, "--version "+string(r.version))
	}
	if r.decompress {
		parts = append(parts, "--decompress")
	}
	return strings.Join(parts, " "), true
}

// missingSalesReportHint explains a 404 for which Apple has no report file:
// a period that Apple has not published yet, by its published availability
// schedule, or a published period that had no sales.
func missingSalesReportHint(err error, retry salesReportRetry, reportDate string, now time.Time) string {
	freq := retry.frequency
	periodEnd, ok := salesPeriodEnd(freq, reportDate)
	if !ok {
		periodEnd = latestPublishedSalesPeriod(freq, now)
	}
	label := formatSalesPeriod(freq, periodEnd)
	var apiErr *asc.APIError
	noSales := errors.As(err, &apiErr) && apiErr != nil && isNoSalesDetail(apiErr)

	publishedAt := salesPeriodPublishedAt(freq, periodEnd)
	if now.Before(publishedAt) {
		latest := formatSalesPeriod(freq, latestPublishedSalesPeriod(freq, now))
		hint := fmt.Sprintf("Apple has not published the %s report for %s yet: %s reports appear %s, by about 8 a.m. Pacific, so this one should appear on %s. The latest %s report that should be published is %s",
			freq, label, freq, salesPublishSchedule(freq), publishedAt.Format("2006-01-02"), freq, latest)
		if command, ok := retry.command(latest); ok {
			return hint + ": " + command
		}
		return hint + "."
	}
	if noSales {
		return fmt.Sprintf("Apple publishes no %s report for a period without sales, so %s had no sales (for example, it is before the first sale); count it as zero, re-running will not change it. When looping over dates, add --allow-missing to get available=false with exit 0 for such periods.",
			freq, label)
	}
	previous := formatSalesPeriod(freq, previousSalesPeriod(freq, periodEnd))
	hint := fmt.Sprintf("Apple has not published the %s report for %s yet, although it is usually out by now; re-running will not help until it is. The period before is %s", freq, label, previous)
	if command, ok := retry.command(previous); ok {
		hint += ": " + command
	}
	return hint + ", or add --allow-missing to get available=false instead of an error."
}

// salesPeriodEnd returns the last day of the period the request names, as a
// date at midnight in salesReportPacific.
func salesPeriodEnd(freq asc.SalesReportFrequency, reportDate string) (time.Time, bool) {
	switch freq {
	case asc.SalesReportFrequencyMonthly:
		start, err := time.ParseInLocation("2006-01", reportDate, salesReportPacific)
		if err != nil {
			return time.Time{}, false
		}
		return start.AddDate(0, 1, -1), true
	case asc.SalesReportFrequencyYearly:
		start, err := time.ParseInLocation("2006", reportDate, salesReportPacific)
		if err != nil {
			return time.Time{}, false
		}
		return start.AddDate(1, 0, -1), true
	default:
		// DAILY dates name the day and WEEKLY dates the week's Sunday.
		end, err := time.ParseInLocation("2006-01-02", reportDate, salesReportPacific)
		if err != nil {
			return time.Time{}, false
		}
		return end, true
	}
}

// salesPeriodPublishedAt is when Apple's availability schedule says the
// report for the period ending on periodEnd is out: daily reports the next
// day, weekly reports on the Monday after, monthly reports five days after the
// month ends and yearly reports six days after the year ends, by 8 a.m. PT.
func salesPeriodPublishedAt(freq asc.SalesReportFrequency, periodEnd time.Time) time.Time {
	days := 1
	switch freq {
	case asc.SalesReportFrequencyMonthly:
		days = 5
	case asc.SalesReportFrequencyYearly:
		days = 6
	}
	return periodEnd.AddDate(0, 0, days).Add(salesReportPublishHour * time.Hour)
}

func salesPublishSchedule(freq asc.SalesReportFrequency) string {
	switch freq {
	case asc.SalesReportFrequencyWeekly:
		return "on the Monday after the week"
	case asc.SalesReportFrequencyMonthly:
		return "five days after the month ends"
	case asc.SalesReportFrequencyYearly:
		return "six days after the year ends"
	default:
		return "the day after"
	}
}

// latestPublishedSalesPeriod returns the end of the most recent period whose
// report the availability schedule says is out at now.
func latestPublishedSalesPeriod(freq asc.SalesReportFrequency, now time.Time) time.Time {
	local := now.In(salesReportPacific)
	today := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, salesReportPacific)
	var end time.Time
	switch freq {
	case asc.SalesReportFrequencyWeekly:
		end = today.AddDate(0, 0, (7-int(today.Weekday()))%7)
	case asc.SalesReportFrequencyMonthly:
		end = time.Date(today.Year(), today.Month()+1, 0, 0, 0, 0, 0, salesReportPacific)
	case asc.SalesReportFrequencyYearly:
		end = time.Date(today.Year(), time.December, 31, 0, 0, 0, 0, salesReportPacific)
	default:
		end = today
	}
	for now.Before(salesPeriodPublishedAt(freq, end)) {
		end = previousSalesPeriod(freq, end)
	}
	return end
}

func previousSalesPeriod(freq asc.SalesReportFrequency, periodEnd time.Time) time.Time {
	switch freq {
	case asc.SalesReportFrequencyWeekly:
		return periodEnd.AddDate(0, 0, -7)
	case asc.SalesReportFrequencyMonthly:
		return time.Date(periodEnd.Year(), periodEnd.Month(), 0, 0, 0, 0, 0, salesReportPacific)
	case asc.SalesReportFrequencyYearly:
		return time.Date(periodEnd.Year()-1, time.December, 31, 0, 0, 0, 0, salesReportPacific)
	default:
		return periodEnd.AddDate(0, 0, -1)
	}
}

// formatSalesPeriod renders a period end as the --date value Apple expects.
func formatSalesPeriod(freq asc.SalesReportFrequency, periodEnd time.Time) string {
	switch freq {
	case asc.SalesReportFrequencyMonthly:
		return periodEnd.Format("2006-01")
	case asc.SalesReportFrequencyYearly:
		return periodEnd.Format("2006")
	default:
		return periodEnd.Format("2006-01-02")
	}
}
