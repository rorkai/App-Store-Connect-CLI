package pricing

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// AppPriceRequestRunner runs one App Store Connect request. Callers that share
// a request limit across concurrent reads pass a runner that applies it; the
// runner must bound request with a timeout derived from ctx.
type AppPriceRequestRunner func(ctx context.Context, request func(context.Context) error) error

// appPriceRequestRunner returns a runner that bounds each request with timeout.
func appPriceRequestRunner(timeout func(context.Context) (context.Context, context.CancelFunc)) AppPriceRequestRunner {
	return func(ctx context.Context, request func(context.Context) error) error {
		callCtx, cancel := timeout(ctx)
		defer cancel()
		return request(callCtx)
	}
}

// AppPriceActiveOn reports whether an app price with startDate and endDate
// applies on now's App Store Connect pricing date (the US Pacific date, see
// shared.PricingDate). Start dates are inclusive and end dates are exclusive,
// so a price stops applying on its end date. An empty date is unbounded. known
// is false when a date is not a YYYY-MM-DD date.
func AppPriceActiveOn(startDate, endDate string, now time.Time) (active bool, known bool) {
	start, ok := parseAppPriceDateStrict(startDate)
	if !ok {
		return false, false
	}
	end, ok := parseAppPriceDateStrict(endDate)
	if !ok {
		return false, false
	}
	return shared.PriceActiveOn(start, end, shared.PricingDate(now)), true
}

// appPriceEndedOn reports whether an app price with endDate no longer applies
// on now's App Store Connect pricing date, using the same rules as
// AppPriceActiveOn. An empty endDate never ends. known is false when endDate is
// not a YYYY-MM-DD date.
func appPriceEndedOn(endDate string, now time.Time) (ended bool, known bool) {
	active, known := AppPriceActiveOn("", endDate, now)
	return known && !active, known
}

// parseAppPriceDateStrict parses a YYYY-MM-DD schedule date as midnight UTC.
// An empty value is unbounded (nil); ok is false for any other unparseable
// value.
func parseAppPriceDateStrict(value string) (date *time.Time, ok bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, true
	}
	parsed, err := time.Parse(appPriceDateLayout, value)
	if err != nil {
		return nil, false
	}
	return &parsed, true
}

// AppBasePriceStatus reports whether an app price schedule has a price for its
// base territory, using the same schedule reads as `asc pricing current`.
type AppBasePriceStatus struct {
	// Configured is false when App Store Connect reports that the app's price
	// schedule was never created.
	Configured bool
	// BaseTerritory is the schedule's base territory ID, such as USA.
	BaseTerritory string
	// HasPrice is true when the base territory has a manual price that is
	// active now or scheduled to start later. A free price counts.
	HasPrice bool
}

// FetchAppBasePriceStatus reads the base territory and manual prices of the
// price schedule scheduleID, running each request through run. A nil run
// bounds each request with shared.ContextWithTimeout. A schedule that App
// Store Connect reports as never configured returns Configured=false without
// an error; every other request or decoding failure is returned so callers can
// treat the price as unverified.
func FetchAppBasePriceStatus(ctx context.Context, client *asc.Client, scheduleID string, run AppPriceRequestRunner) (AppBasePriceStatus, error) {
	scheduleID = strings.TrimSpace(scheduleID)
	if client == nil || scheduleID == "" {
		return AppBasePriceStatus{}, fmt.Errorf("app price schedule ID is required")
	}
	if run == nil {
		run = appPriceRequestRunner(shared.ContextWithTimeout)
	}

	var baseTerritoryResp *asc.TerritoryResponse
	err := run(ctx, func(callCtx context.Context) error {
		var requestErr error
		baseTerritoryResp, requestErr = client.GetAppPriceScheduleBaseTerritory(callCtx, scheduleID)
		return requestErr
	})
	if err != nil {
		if isAppPriceScheduleNotConfigured(err) {
			return AppBasePriceStatus{}, nil
		}
		return AppBasePriceStatus{}, fmt.Errorf("get base territory: %w", err)
	}
	baseTerritory := strings.ToUpper(strings.TrimSpace(baseTerritoryResp.Data.ID))
	if baseTerritory == "" {
		return AppBasePriceStatus{}, fmt.Errorf("base territory missing from response")
	}

	rawPrices := 0
	entries, _, _, err := fetchAppSchedulePriceEntries(ctx, run, func(callCtx context.Context, opts ...asc.AppPriceSchedulePricesOption) (*asc.AppPricesResponse, error) {
		resp, err := client.GetAppPriceScheduleManualPrices(callCtx, scheduleID, opts...)
		if resp != nil {
			rawPrices += len(resp.Data)
		}
		return resp, err
	})
	if err != nil {
		if isAppPriceScheduleNotConfigured(err) {
			return AppBasePriceStatus{BaseTerritory: baseTerritory}, nil
		}
		return AppBasePriceStatus{}, fmt.Errorf("fetch manual prices: %w", err)
	}

	status := AppBasePriceStatus{
		Configured:    true,
		BaseTerritory: baseTerritory,
		HasPrice:      hasCurrentOrScheduledPrice(entries, baseTerritory, shared.PricingNow()),
	}
	if !status.HasPrice && len(entries) < rawPrices {
		// Some manual prices could not be attributed to a territory, so the base
		// territory may still have one.
		return AppBasePriceStatus{}, fmt.Errorf("could not resolve the territory of %d manual price(s)", rawPrices-len(entries))
	}
	return status, nil
}

func hasCurrentOrScheduledPrice(entries []appPriceEntry, territoryID string, now time.Time) bool {
	territoryID = strings.ToUpper(strings.TrimSpace(territoryID))
	for _, entry := range entries {
		if entry.TerritoryID != territoryID {
			continue
		}
		// A price whose end date cannot be read is not treated as ended, so an
		// unreadable date never reports a missing price.
		if ended, known := appPriceEndedOn(entry.EndDate, now); !ended || !known {
			return true
		}
	}
	return false
}
