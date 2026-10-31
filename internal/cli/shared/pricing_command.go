package shared

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/ascterritory"
)

// PricingSetCommandConfig configures pricing set commands.
type PricingSetCommandConfig struct {
	FlagSetName           string
	CommandName           string
	ShortUsage            string
	ShortHelp             string
	LongHelp              string
	ErrorPrefix           string
	StartDateHelp         string
	StartDateDefaultToday bool
	RequireBaseTerritory  bool
	ResolveBaseTerritory  bool
	// TemporaryPrice adds --end-date, --return-price-point, and --confirm.
	TemporaryPrice bool
}

// NewPricingSetCommand builds a pricing set command with shared behavior.
func NewPricingSetCommand(config PricingSetCommandConfig) *ffcli.Command {
	fs := flag.NewFlagSet(config.FlagSetName, flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID)")
	pricePointID := BindResourceIDFlag(fs, "price-point", "appPricePoints", "App price point ID")
	tier := fs.Int("tier", 0, "Pricing tier number (1-based, mutually exclusive with --price-point, --price, and --free)")
	price := fs.String("price", "", "Customer price (e.g., 0.99, mutually exclusive with --price-point, --tier, and --free) to select price point")
	free := fs.Bool("free", false, "Set app price to Free ($0), mutually exclusive with --price-point, --tier, and --price")
	baseTerritory := fs.String("base-territory", "", "Base territory input (accepts alpha-2, alpha-3, or exact English country name; e.g., US, USA, United States)")
	startDate := fs.String("start-date", "", config.StartDateHelp)
	refresh := fs.Bool("refresh", false, "Force refresh of tier cache")
	endDate, returnPricePoint, confirm := new(string), new(string), new(bool)
	if config.TemporaryPrice {
		fs.StringVar(endDate, "end-date", "", "Date the price ends (YYYY-MM-DD, US Pacific time, exclusive); requires --return-price-point and --confirm")
		returnPricePoint = BindResourceIDFlag(fs, "return-price-point", "appPricePoints", "Base territory price point that applies from --end-date, and until --start-date when that is in the future")
		fs.BoolVar(confirm, "confirm", false, "Confirm replacing the whole price schedule (required with --end-date)")
	}
	output := BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       config.CommandName,
		ShortUsage: config.ShortUsage,
		ShortHelp:  config.ShortHelp,
		LongHelp:   config.LongHelp,
		FlagSet:    fs,
		UsageFunc:  DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			resolvedAppID := resolveAppID(*appID)
			if resolvedAppID == "" {
				fmt.Fprintln(os.Stderr, "Error: --app is required (or set ASC_APP_ID)")
				return MissingRequiredUsageError("--app")
			}
			pricePointValue := strings.TrimSpace(*pricePointID)
			tierValue := *tier
			priceValue := strings.TrimSpace(*price)
			freeValue := *free

			if err := ValidatePriceSelectionFlags(pricePointValue, tierValue, priceValue, freeValue); err != nil {
				fmt.Fprintln(os.Stderr, "Error:", err)
				return reportedUsageErrHelp(err)
			}
			if err := ValidateFinitePriceFlag("--price", priceValue); err != nil {
				fmt.Fprintln(os.Stderr, "Error:", err)
				return WithDiagnostic(flag.ErrHelp, DiagnosticInvalidInput, "--price")
			}

			baseTerritoryInput := strings.TrimSpace(*baseTerritory)
			if requiresExplicitBaseTerritory(config, baseTerritoryInput, tierValue, priceValue, freeValue) {
				fmt.Fprintln(os.Stderr, "Error: --base-territory is required")
				return MissingRequiredUsageError("--base-territory")
			}
			baseTerritoryValue := baseTerritoryInput
			if baseTerritoryInput != "" {
				normalizedBaseTerritory, normalizeErr := ascterritory.Normalize(baseTerritoryInput)
				if normalizeErr != nil {
					return WithDiagnostic(UsageError(normalizeErr.Error()), DiagnosticInvalidInput, "--base-territory")
				}
				baseTerritoryValue = normalizedBaseTerritory
			}

			startDateValue := strings.TrimSpace(*startDate)
			startDateDefaulted := startDateValue == ""
			if startDateDefaulted && !config.StartDateDefaultToday {
				fmt.Fprintln(os.Stderr, "Error: --start-date is required")
				return MissingRequiredUsageError("--start-date")
			}

			var normalizedStartDate string
			if !startDateDefaulted {
				normalized, normalizeErr := normalizePricingStartDate(startDateValue)
				if normalizeErr != nil {
					return WithDiagnostic(
						UsageError(normalizeErr.Error()),
						DiagnosticInvalidInput,
						"--start-date",
					)
				}
				normalizedStartDate = normalized
			}

			returnPricePointValue := strings.TrimSpace(*returnPricePoint)
			normalizedEndDate, err := validateTemporaryPriceFlags(fs, *endDate, returnPricePointValue, *confirm)
			if err != nil {
				return err
			}
			if normalizedEndDate != "" {
				firstDay := normalizedStartDate
				if startDateDefaulted {
					firstDay = pricingDefaultStartDate(pricingNow())
				}
				if err := validatePricingEndDate(firstDay, normalizedEndDate); err != nil {
					return err
				}
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("%s: %w", config.ErrorPrefix, err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			baseTerritoryID := baseTerritoryValue
			if config.ResolveBaseTerritory {
				baseTerritoryID, err = resolveBaseTerritoryID(requestCtx, client, resolvedAppID, baseTerritoryValue)
				if err != nil {
					return fmt.Errorf("%s: %w", config.ErrorPrefix, err)
				}
			}

			if freeValue {
				resolvedID, err := ResolveFreeAppPricePoint(requestCtx, client, resolvedAppID, baseTerritoryID)
				if err != nil {
					return fmt.Errorf("%s: %w", config.ErrorPrefix, err)
				}
				pricePointValue = resolvedID
			} else if tierValue > 0 || priceValue != "" {
				tiers, err := ResolveTiers(requestCtx, client, resolvedAppID, baseTerritoryID, *refresh)
				if err != nil {
					return fmt.Errorf("resolve tiers: %w", err)
				}

				var resolvedID string
				if tierValue > 0 {
					resolvedID, err = ResolvePricePointByTier(tiers, tierValue)
				} else {
					resolvedID, err = ResolvePricePointByPrice(tiers, priceValue)
				}
				if err != nil {
					return fmt.Errorf("%s: %w", config.ErrorPrefix, err)
				}
				pricePointValue = resolvedID
			}

			// Resolve the default after every preliminary lookup so a request
			// that crosses US Pacific midnight still sends today's date, never
			// a past one Apple would reject.
			if startDateDefaulted {
				normalizedStartDate = pricingDefaultStartDate(pricingNow())
				fmt.Fprintf(
					os.Stderr,
					"Note: --start-date not set; using %s (today in US Pacific time, which App Store Connect uses). Apple requires today or later.\n",
					normalizedStartDate,
				)
			}

			attrs := asc.AppPriceScheduleCreateAttributes{
				PricePointID:    pricePointValue,
				StartDate:       normalizedStartDate,
				BaseTerritoryID: baseTerritoryID,
			}
			if normalizedEndDate != "" {
				if err := validatePricingEndDate(normalizedStartDate, normalizedEndDate); err != nil {
					return err
				}
				attrs.Prices = temporaryPriceTimeline(pricePointValue, returnPricePointValue, normalizedStartDate, normalizedEndDate, pricingDefaultStartDate(pricingNow()))
				if len(attrs.Prices) == 3 {
					fmt.Fprintf(
						os.Stderr,
						"Note: --return-price-point also applies from today until %s, because --start-date is in the future.\n",
						normalizedStartDate,
					)
				}
			}

			resp, err := client.CreateAppPriceSchedule(requestCtx, resolvedAppID, attrs)
			if err != nil {
				return explainAppPriceScheduleConflict(err, appPriceScheduleConflictInput{
					ErrorPrefix:        config.ErrorPrefix,
					AppID:              resolvedAppID,
					PricePointID:       pricePointValue,
					PriceSelectionFlag: priceSelectionFlag(tierValue, priceValue, freeValue),
					BaseTerritoryID:    baseTerritoryID,
					StartDate:          normalizedStartDate,
					StartDateDefaulted: startDateDefaulted,
					TemporaryPrice:     normalizedEndDate != "",
				})
			}

			return printOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// priceSelectionFlag names the flag the operator used to choose the price.
// ValidatePriceSelectionFlags has already ensured exactly one is set.
func priceSelectionFlag(tier int, price string, free bool) string {
	switch {
	case free:
		return "--free"
	case tier > 0:
		return "--tier"
	case price != "":
		return "--price"
	default:
		return "--price-point"
	}
}

// pricingStartDateLayout is the date-only layout App Store Connect expects for
// appPrices startDate and endDate ("format": "date" in the OpenAPI snapshot).
const pricingStartDateLayout = "2006-01-02"

// pricingDefaultStartDate returns today's App Store Connect pricing date (the
// US Pacific date).
func pricingDefaultStartDate(now time.Time) string {
	return PricingDate(now).Format(pricingStartDateLayout)
}

func normalizePricingStartDate(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", fmt.Errorf("--start-date is required")
	}
	return parsePricingDate("--start-date", trimmed)
}

func parsePricingDate(flagName, value string) (string, error) {
	parsed, err := time.Parse(pricingStartDateLayout, value)
	if err != nil {
		return "", fmt.Errorf("%s must be in YYYY-MM-DD format", flagName)
	}
	return parsed.Format(pricingStartDateLayout), nil
}

// validateTemporaryPriceFlags returns the normalized end date, or "" for a
// price without one.
func validateTemporaryPriceFlags(fs *flag.FlagSet, endDate, returnPricePoint string, confirm bool) (string, error) {
	endDateSet := false
	fs.Visit(func(f *flag.Flag) {
		endDateSet = endDateSet || f.Name == "end-date"
	})
	if !endDateSet {
		if returnPricePoint != "" {
			return "", WithDiagnostic(UsageError("--return-price-point requires --end-date"), DiagnosticInvalidInput, "--return-price-point")
		}
		if confirm {
			return "", WithDiagnostic(UsageError("--confirm requires --end-date"), DiagnosticInvalidInput, "--confirm")
		}
		return "", nil
	}
	normalized, err := parsePricingDate("--end-date", strings.TrimSpace(endDate))
	if err != nil {
		return "", WithDiagnostic(UsageError(err.Error()), DiagnosticInvalidInput, "--end-date")
	}
	if returnPricePoint == "" {
		fmt.Fprintln(os.Stderr, "Error: --end-date requires --return-price-point, the price that applies after the end date")
		return "", MissingRequiredUsageError("--return-price-point")
	}
	if !confirm {
		fmt.Fprintln(os.Stderr, "Error: --confirm is required with --end-date because this replaces the app's whole price schedule, dropping existing scheduled prices and manual prices in other territories")
		return "", MissingRequiredUsageError("--confirm")
	}
	return normalized, nil
}

func validatePricingEndDate(startDate, endDate string) error {
	if endDate > startDate {
		return nil
	}
	return WithDiagnostic(
		UsageError(fmt.Sprintf("--end-date must be after the start date %s", startDate)),
		DiagnosticInvalidInput,
		"--end-date",
	)
}

// temporaryPriceTimeline builds the base territory timeline around a temporary
// price. App Store Connect rejects a schedule that does not cover today, and
// ends one price on the date the next one starts, so the return price fills
// both sides of the temporary price. The first entry of a future price has no
// start date, as in Apple's POST /v1/appPriceSchedules example.
func temporaryPriceTimeline(pricePoint, returnPricePoint, startDate, endDate, today string) []asc.AppPriceSchedulePrice {
	timeline := make([]asc.AppPriceSchedulePrice, 0, 3)
	if startDate > today {
		timeline = append(timeline, asc.AppPriceSchedulePrice{PricePointID: returnPricePoint, EndDate: startDate})
	}
	return append(timeline,
		asc.AppPriceSchedulePrice{PricePointID: pricePoint, StartDate: startDate, EndDate: endDate},
		asc.AppPriceSchedulePrice{PricePointID: returnPricePoint, StartDate: endDate},
	)
}

func requiresExplicitBaseTerritory(config PricingSetCommandConfig, baseTerritory string, tier int, price string, free bool) bool {
	if strings.TrimSpace(baseTerritory) != "" {
		return false
	}
	if free && config.ResolveBaseTerritory && !config.RequireBaseTerritory {
		return false
	}
	return config.RequireBaseTerritory || tier > 0 || strings.TrimSpace(price) != "" || free
}

func resolveBaseTerritoryID(ctx context.Context, client *asc.Client, appID string, baseTerritory string) (string, error) {
	trimmed := strings.ToUpper(strings.TrimSpace(baseTerritory))
	if trimmed != "" {
		return trimmed, nil
	}

	schedule, err := client.GetAppPriceSchedule(ctx, appID)
	if err != nil {
		if asc.IsNotFound(err) {
			return "", fmt.Errorf("--base-territory is required when app price schedule is missing")
		}
		return "", fmt.Errorf("get app price schedule: %w", err)
	}

	scheduleID := strings.TrimSpace(schedule.Data.ID)
	if scheduleID == "" {
		return "", fmt.Errorf("app price schedule ID missing")
	}

	territoryResp, err := client.GetAppPriceScheduleBaseTerritory(ctx, scheduleID)
	if err != nil {
		return "", fmt.Errorf("get base territory: %w", err)
	}

	territoryID := strings.ToUpper(strings.TrimSpace(territoryResp.Data.ID))
	if territoryID == "" {
		return "", fmt.Errorf("base territory ID missing from response")
	}

	return territoryID, nil
}
