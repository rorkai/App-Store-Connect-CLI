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
}

// NewPricingSetCommand builds a pricing set command with shared behavior.
func NewPricingSetCommand(config PricingSetCommandConfig) *ffcli.Command {
	fs := flag.NewFlagSet(config.FlagSetName, flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID)")
	pricePointID := fs.String("price-point", "", "App price point ID")
	baseTerritory := fs.String("base-territory", "", "Base territory ID (e.g., USA)")
	startDate := fs.String("start-date", "", config.StartDateHelp)
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
				return flag.ErrHelp
			}
			pricePointValue := strings.TrimSpace(*pricePointID)
			if pricePointValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --price-point is required")
				return flag.ErrHelp
			}

			baseTerritoryValue := strings.TrimSpace(*baseTerritory)
			if config.RequireBaseTerritory && baseTerritoryValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --base-territory is required")
				return flag.ErrHelp
			}

			startDateValue := strings.TrimSpace(*startDate)
			if startDateValue == "" {
				if config.StartDateDefaultToday {
					startDateValue = time.Now().Format("2006-01-02")
				} else {
					fmt.Fprintln(os.Stderr, "Error: --start-date is required")
					return flag.ErrHelp
				}
			}

			normalizedStartDate, err := normalizePricingStartDate(startDateValue)
			if err != nil {
				return fmt.Errorf("%s: %w", config.ErrorPrefix, err)
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

			resp, err := client.CreateAppPriceSchedule(requestCtx, resolvedAppID, asc.AppPriceScheduleCreateAttributes{
				PricePointID:    pricePointValue,
				StartDate:       normalizedStartDate,
				BaseTerritoryID: baseTerritoryID,
			})
			if err != nil {
				return fmt.Errorf("%s: %w", config.ErrorPrefix, err)
			}

			return printOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

func normalizePricingStartDate(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", fmt.Errorf("--start-date is required")
	}
	parsed, err := time.Parse("2006-01-02", trimmed)
	if err != nil {
		return "", fmt.Errorf("--start-date must be in YYYY-MM-DD format")
	}
	return parsed.Format("2006-01-02"), nil
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
