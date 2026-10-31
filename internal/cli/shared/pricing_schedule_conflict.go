package shared

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

// App Store Connect error codes returned with HTTP 409 by
// POST /v1/appPriceSchedules. Each was captured live against the disposable
// app 6759231657 on 2026-09-26 and 2026-09-27; the raw bodies are checked in under
// internal/cli/cmdtest/testdata/pricing_schedule_create_409.
const (
	appPriceScheduleInvalidStartDateCode         = "ENTITY_ERROR.INVALID_START_DATE"
	appPriceScheduleBaseTerritoryIntervalMissing = "ENTITY_ERROR.BASE_TERRITORY_INTERVAL_REQUIRED"
	appPriceScheduleEntityNotFoundCode           = "ENTITY_ERROR.NOT_FOUND"
)

// appPriceScheduleConflictInput describes the request whose create failed, so
// the explanation can name the values the operator supplied.
type appPriceScheduleConflictInput struct {
	ErrorPrefix  string
	AppID        string
	PricePointID string
	// PriceSelectionFlag is the flag the price point came from: --price-point,
	// --tier, --price, or --free.
	PriceSelectionFlag string
	BaseTerritoryID    string
	StartDate          string
	StartDateDefaulted bool
	// TemporaryPrice marks a request with several manual prices, where the
	// single-price guidance could blame the wrong flag.
	TemporaryPrice bool
}

// explainAppPriceScheduleConflict turns the App Store Connect 409 causes known
// for POST /v1/appPriceSchedules into specific, actionable errors. The
// original *asc.APIError stays in the chain, so the conflict exit code and the
// api_conflict telemetry kind are unchanged; the cause adds an allowlisted
// diagnostic code and parameter. Every other error, including a 409 with an
// unrecognized code or detail, is wrapped exactly as before.
func explainAppPriceScheduleConflict(err error, input appPriceScheduleConflictInput) error {
	wrapped := fmt.Errorf("%s: %w", input.ErrorPrefix, err)

	apiErr, ok := errors.AsType[*asc.APIError](err)
	if !ok || apiErr == nil || apiErr.StatusCode != http.StatusConflict {
		return wrapped
	}

	entries := apiErr.Entries
	if len(entries) == 0 {
		entries = []asc.APIErrorEntry{{Code: apiErr.Code, Detail: apiErr.Detail}}
	}

	var (
		guidance   string
		diagnostic DiagnosticCode
		parameter  string
	)
	// Each code is matched with the detail of its own errors[] entry, so text
	// from one entry never classifies another entry's code.
	switch {
	case hasAPIErrorEntry(entries, appPriceScheduleInvalidStartDateCode, "start date in the past"):
		guidance, diagnostic, parameter = pastStartDateGuidance(input), DiagnosticInvalidInput, "--start-date"
	case input.TemporaryPrice:
		return wrapped
	case hasAPIErrorEntry(entries, appPriceScheduleInvalidStartDateCode, "entire timeline must be covered"):
		guidance, diagnostic, parameter = futureStartDateGuidance(input), DiagnosticInvalidInput, "--start-date"
	case hasAPIErrorEntry(entries, appPriceScheduleBaseTerritoryIntervalMissing, ""):
		guidance, diagnostic, parameter = baseTerritoryMismatchGuidance(input), DiagnosticConflictingInput, "--base-territory"
	case hasAPIErrorEntry(entries, appPriceScheduleEntityNotFoundCode, "'appPricePoints'"):
		guidance, diagnostic, parameter = pricePointNotFoundGuidance(input), DiagnosticResourceNotFound, input.PriceSelectionFlag
	default:
		return wrapped
	}

	return WithDiagnostic(
		fmt.Errorf("%s: %s\n\nApp Store Connect: %w", input.ErrorPrefix, guidance, err),
		diagnostic,
		parameter,
	)
}

// appPriceScheduleTodayHint records how App Store Connect dates "today". On
// 2026-09-27 at 00:03 UTC it rejected startDate 2026-09-27 (the UTC date) as
// in the future and accepted 2026-09-26 (the US Pacific date).
const appPriceScheduleTodayHint = "App Store Connect used the US Pacific date as today in live checks, " +
	"so for several hours after UTC midnight it rejects the UTC date as in the future."

// appPriceScheduleDefaultDateHint explains a rejected default start date. The
// default is already today's US Pacific date, so a rejection points at the
// local clock or at App Store Connect using another, undocumented time zone.
const appPriceScheduleDefaultDateHint = "App Store Connect used the US Pacific date as today in live checks, " +
	"but its time zone is not documented. Check the system clock, or pass --start-date with the date App Store Connect accepts as today."

func pastStartDateGuidance(input appPriceScheduleConflictInput) string {
	if input.StartDateDefaulted {
		return fmt.Sprintf(
			"App Store Connect treats the default start date %s (today in US Pacific time) as in the past. %s",
			input.StartDate,
			appPriceScheduleDefaultDateHint,
		)
	}
	return fmt.Sprintf("--start-date %s is in the past.", input.StartDate) +
		" Pass --start-date with today's date in US Pacific time. " + appPriceScheduleTodayHint
}

func futureStartDateGuidance(input appPriceScheduleConflictInput) string {
	if input.StartDateDefaulted {
		return fmt.Sprintf(
			"App Store Connect treats the default start date %s (today in US Pacific time) as in the future. %s",
			input.StartDate,
			appPriceScheduleDefaultDateHint,
		)
	}
	return fmt.Sprintf(
		"--start-date %s is in the future. This command replaces the app's whole price schedule with this one price, "+
			"and App Store Connect requires the schedule to cover today, so a future price change cannot be scheduled with it. "+
			"Use today's date in US Pacific time. %s",
		input.StartDate,
		appPriceScheduleTodayHint,
	)
}

func baseTerritoryMismatchGuidance(input appPriceScheduleConflictInput) string {
	territory := strings.TrimSpace(input.BaseTerritoryID)
	return fmt.Sprintf(
		"the price point is not a %[1]s price point, and App Store Connect requires the price to be set in the base territory (%[1]s). "+
			"Pass a --price-point listed by `asc pricing price-points --app %[2]s --territory %[1]s`, or use --price, --tier, or --free, "+
			"which pick the price point in the base territory.",
		territory,
		input.AppID,
	)
}

func pricePointNotFoundGuidance(input appPriceScheduleConflictInput) string {
	if input.PriceSelectionFlag != "--price-point" {
		message := fmt.Sprintf(
			"App Store Connect did not find price point %q, resolved from %s, for app %s.",
			input.PricePointID,
			input.PriceSelectionFlag,
			input.AppID,
		)
		if input.PriceSelectionFlag == "--tier" || input.PriceSelectionFlag == "--price" {
			message += " Retry with --refresh to rebuild the tier cache."
		}
		return message
	}
	return fmt.Sprintf(
		"price point %q was not found for app %s. Price point IDs belong to one app and territory; "+
			"list valid IDs with `asc pricing price-points --app %s --territory %s`.",
		input.PricePointID,
		input.AppID,
		input.AppID,
		input.BaseTerritoryID,
	)
}

// hasAPIErrorEntry reports whether one errors[] entry has code and, when
// detailSubstring is not empty, a detail containing it (case-insensitively).
func hasAPIErrorEntry(entries []asc.APIErrorEntry, code, detailSubstring string) bool {
	detailSubstring = strings.ToLower(detailSubstring)
	for _, entry := range entries {
		if !strings.EqualFold(strings.TrimSpace(entry.Code), code) {
			continue
		}
		if detailSubstring == "" || strings.Contains(strings.ToLower(entry.Detail), detailSubstring) {
			return true
		}
	}
	return false
}
