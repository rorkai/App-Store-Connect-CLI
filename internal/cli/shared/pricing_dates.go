package shared

import (
	"sync"
	"time"
	// Embed the IANA time zone database so US Pacific pricing dates work
	// without system zoneinfo (Windows, minimal containers).
	_ "time/tzdata"
)

// pricingTimeZone is the zone App Store Connect uses to decide "today" for
// price schedules. Live checks on 2026-09-27 at 00:03 UTC rejected the UTC
// date (2026-09-27) as a future appPriceSchedules start date and accepted the
// US Pacific date (2026-09-26). Apple's encoded price IDs also place schedule
// boundaries at US Pacific midnight (see docs/API_NOTES.md).
const pricingTimeZone = "America/Los_Angeles"

// pricingLocation loads pricingTimeZone once. The time zone database is
// embedded, so loading cannot fail at run time; the fixed UTC-8 fallback only
// keeps a broken build from panicking.
var pricingLocation = sync.OnceValue(func() *time.Location {
	location, err := time.LoadLocation(pricingTimeZone)
	if err != nil {
		return time.FixedZone("PST", -8*60*60)
	}
	return location
})

// PricingDate returns the App Store Connect pricing date that contains now:
// its US Pacific calendar day. The date is returned as midnight UTC so it
// compares directly with schedule dates parsed from YYYY-MM-DD.
func PricingDate(now time.Time) time.Time {
	local := now.In(pricingLocation())
	return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, time.UTC)
}

// PriceActiveOn reports whether a scheduled price applies on date, a pricing
// date from PricingDate. start and end are the price's schedule dates as
// midnight UTC, or nil when unbounded. Start dates are inclusive and end dates
// are exclusive: App Store Connect ends one price on the date the next one
// starts.
func PriceActiveOn(start, end *time.Time, date time.Time) bool {
	if start != nil && start.After(date) {
		return false
	}
	if end != nil && !end.After(date) {
		return false
	}
	return true
}
