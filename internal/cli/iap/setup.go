package iap

import (
	"context"
	"flag"
	"fmt"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

const (
	iapSetupStepCreateIAP           = "create_iap"
	iapSetupStepCreateLocalization  = "create_localization"
	iapSetupStepResolvePricePoint   = "resolve_price_point"
	iapSetupStepCreatePriceSchedule = "create_price_schedule"
)

type iapSetupOptions struct {
	AppID            string
	Type             string
	ReferenceName    string
	ProductID        string
	FamilySharable   bool
	Locale           string
	DisplayName      string
	Description      string
	BaseTerritory    string
	PricePointID     string
	Tier             int
	Price            string
	StartDate        string
	RefreshTierCache bool
}

type iapSetupStepResult struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	ID      string `json:"id,omitempty"`
	Message string `json:"message,omitempty"`
}

type iapSetupResult struct {
	Status               string               `json:"status"`
	AppID                string               `json:"appId"`
	Type                 string               `json:"type"`
	ProductID            string               `json:"productId"`
	ReferenceName        string               `json:"referenceName"`
	Locale               string               `json:"locale,omitempty"`
	BaseTerritory        string               `json:"baseTerritory,omitempty"`
	IAPID                string               `json:"iapId,omitempty"`
	LocalizationID       string               `json:"localizationId,omitempty"`
	PriceScheduleID      string               `json:"priceScheduleId,omitempty"`
	ResolvedPricePointID string               `json:"resolvedPricePointId,omitempty"`
	FailedStep           string               `json:"failedStep,omitempty"`
	Steps                []iapSetupStepResult `json:"steps"`
}

// IAPSetupCommand returns the high-level IAP bootstrap workflow command.
func IAPSetupCommand() *ffcli.Command {
	fs := flag.NewFlagSet("setup", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	iapType := fs.String("type", "", "IAP type: CONSUMABLE, NON_CONSUMABLE, NON_RENEWING_SUBSCRIPTION")
	referenceName := fs.String("reference-name", "", "Reference name")
	refNameAlias := fs.String("ref-name", "", "Reference name alias")
	productID := fs.String("product-id", "", "Product ID (e.g., com.example.product)")
	familySharable := fs.Bool("family-sharable", false, "Enable Family Sharing (cannot be undone)")

	locale := fs.String("locale", "", "Locale for the first localization (e.g., en-US)")
	displayName := fs.String("display-name", "", "Display name for the first localization")
	nameAlias := fs.String("name", "", "Display name alias")
	description := fs.String("description", "", "Description for the first localization")

	baseTerritory := fs.String("base-territory", "", "Base territory ID for the initial price schedule (e.g., USA)")
	pricePointID := fs.String("price-point-id", "", "Explicit price point ID for the initial price schedule")
	tier := fs.Int("tier", 0, "Pricing tier number for the initial price schedule")
	price := fs.String("price", "", "Customer price for the initial price schedule")
	startDate := fs.String("start-date", "", "Start date for the initial price schedule (YYYY-MM-DD)")
	refresh := fs.Bool("refresh", false, "Force refresh of the price-point tier cache when resolving --tier or --price")
	output := shared.BindOutputFlags(fs)

	shared.HideFlagFromHelp(fs.Lookup("ref-name"))
	shared.HideFlagFromHelp(fs.Lookup("name"))

	return &ffcli.Command{
		Name:       "setup",
		ShortUsage: "asc iap setup [flags]",
		ShortHelp:  "Create an in-app purchase with optional localization and pricing.",
		LongHelp: `Create a new in-app purchase and optionally bootstrap its first
localization and price schedule in one workflow.

The setup command is create-oriented: use it when you want a one-shot happy
path for a new IAP. Existing low-level commands remain available for partial
updates, repair flows, and advanced cases.

Examples:
  asc iap setup --app "APP_ID" --type NON_CONSUMABLE --reference-name "Pro Lifetime" --product-id "com.example.lifetime"
  asc iap setup --app "APP_ID" --type NON_CONSUMABLE --reference-name "Pro Lifetime" --product-id "com.example.lifetime" --locale "en-US" --display-name "Second Draft Pro" --description "Unlock everything"
  asc iap setup --app "APP_ID" --type NON_CONSUMABLE --reference-name "Pro Lifetime" --product-id "com.example.lifetime" --locale "en-US" --display-name "Second Draft Pro" --price "3.99" --base-territory "USA" --start-date "2026-03-01"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return shared.UsageError("iap setup does not accept positional arguments")
			}

			referenceNameValue, err := resolveIAPSetupAlias(*referenceName, *refNameAlias, "--reference-name", "--ref-name")
			if err != nil {
				return shared.UsageError(err.Error())
			}
			displayNameValue, err := resolveIAPSetupAlias(*displayName, *nameAlias, "--display-name", "--name")
			if err != nil {
				return shared.UsageError(err.Error())
			}

			opts := iapSetupOptions{
				AppID:            shared.ResolveAppID(*appID),
				ReferenceName:    referenceNameValue,
				ProductID:        strings.TrimSpace(*productID),
				FamilySharable:   *familySharable,
				Locale:           strings.TrimSpace(*locale),
				DisplayName:      displayNameValue,
				Description:      strings.TrimSpace(*description),
				BaseTerritory:    strings.ToUpper(strings.TrimSpace(*baseTerritory)),
				PricePointID:     strings.TrimSpace(*pricePointID),
				Tier:             *tier,
				Price:            strings.TrimSpace(*price),
				RefreshTierCache: *refresh,
			}

			if opts.AppID == "" {
				return shared.UsageError("--app is required (or set ASC_APP_ID)")
			}

			normalizedType, err := normalizeIAPType(*iapType)
			if err != nil {
				return shared.UsageError(err.Error())
			}
			opts.Type = normalizedType

			if opts.ReferenceName == "" {
				return shared.UsageError("--reference-name is required")
			}
			if opts.ProductID == "" {
				return shared.UsageError("--product-id is required")
			}

			hasLocalization := opts.Locale != "" || opts.DisplayName != "" || opts.Description != ""
			if hasLocalization {
				if opts.Locale == "" {
					return shared.UsageError("--locale is required when localization flags are provided")
				}
				if opts.DisplayName == "" {
					return shared.UsageError("--display-name is required when localization flags are provided")
				}
			}

			if err := shared.ValidateFinitePriceFlag("--price", opts.Price); err != nil {
				return shared.UsageError(err.Error())
			}
			if opts.Tier < 0 {
				return shared.UsageError("--tier must be a positive integer")
			}

			hasPricing := opts.BaseTerritory != "" || opts.PricePointID != "" || opts.Tier > 0 || opts.Price != "" || strings.TrimSpace(*startDate) != "" || opts.RefreshTierCache
			if hasPricing {
				if opts.BaseTerritory == "" {
					return shared.UsageError("--base-territory is required when pricing flags are provided")
				}
				selectorCount := 0
				if opts.PricePointID != "" {
					selectorCount++
				}
				if opts.Tier > 0 {
					selectorCount++
				}
				if opts.Price != "" {
					selectorCount++
				}
				if selectorCount == 0 {
					return shared.UsageError("one of --price-point-id, --tier, or --price is required when pricing flags are provided")
				}
				if selectorCount > 1 {
					return shared.UsageError("--price-point-id, --tier, and --price are mutually exclusive")
				}
			}

			if strings.TrimSpace(*startDate) != "" {
				normalizedStartDate, err := normalizeIAPDate(*startDate, "--start-date")
				if err != nil {
					return shared.UsageError(err.Error())
				}
				opts.StartDate = normalizedStartDate
			}

			result, runErr := executeIAPSetup(ctx, opts)
			if printErr := printIAPSetupResult(&result, *output.Output, *output.Pretty); printErr != nil {
				return printErr
			}
			if runErr != nil {
				return shared.NewReportedError(runErr)
			}
			return nil
		},
	}
}

func executeIAPSetup(ctx context.Context, opts iapSetupOptions) (iapSetupResult, error) {
	result := iapSetupResult{
		Status:        "ok",
		AppID:         opts.AppID,
		Type:          opts.Type,
		ProductID:     opts.ProductID,
		ReferenceName: opts.ReferenceName,
		Locale:        opts.Locale,
		BaseTerritory: opts.BaseTerritory,
		Steps:         make([]iapSetupStepResult, 0, 4),
	}

	client, err := shared.GetASCClient()
	if err != nil {
		result.Status = "error"
		result.FailedStep = iapSetupStepCreateIAP
		return result, fmt.Errorf("iap setup: %w", err)
	}

	requestCtx, cancel := shared.ContextWithTimeout(ctx)
	defer cancel()

	iapResp, err := client.CreateInAppPurchaseV2(requestCtx, opts.AppID, asc.InAppPurchaseV2CreateAttributes{
		Name:              opts.ReferenceName,
		ProductID:         opts.ProductID,
		InAppPurchaseType: opts.Type,
		FamilySharable:    opts.FamilySharable,
	})
	if err != nil {
		result.Status = "error"
		result.FailedStep = iapSetupStepCreateIAP
		result.Steps = append(result.Steps, iapSetupStepResult{
			Name:    iapSetupStepCreateIAP,
			Status:  "failed",
			Message: err.Error(),
		})
		return result, fmt.Errorf("iap setup: failed to create iap: %w", err)
	}

	result.IAPID = strings.TrimSpace(iapResp.Data.ID)
	result.Steps = append(result.Steps, iapSetupStepResult{
		Name:   iapSetupStepCreateIAP,
		Status: "completed",
		ID:     result.IAPID,
	})

	if opts.Locale == "" && opts.DisplayName == "" && opts.Description == "" {
		result.Steps = append(result.Steps, iapSetupStepResult{
			Name:    iapSetupStepCreateLocalization,
			Status:  "skipped",
			Message: "no localization flags provided",
		})
	} else {
		localizationResp, err := client.CreateInAppPurchaseLocalization(requestCtx, result.IAPID, asc.InAppPurchaseLocalizationCreateAttributes{
			Name:        opts.DisplayName,
			Locale:      opts.Locale,
			Description: opts.Description,
		})
		if err != nil {
			result.Status = "error"
			result.FailedStep = iapSetupStepCreateLocalization
			result.Steps = append(result.Steps, iapSetupStepResult{
				Name:    iapSetupStepCreateLocalization,
				Status:  "failed",
				Message: err.Error(),
			})
			return result, fmt.Errorf("iap setup: failed to create localization: %w", err)
		}

		result.LocalizationID = strings.TrimSpace(localizationResp.Data.ID)
		result.Steps = append(result.Steps, iapSetupStepResult{
			Name:   iapSetupStepCreateLocalization,
			Status: "completed",
			ID:     result.LocalizationID,
		})
	}

	hasPricing := opts.BaseTerritory != "" || opts.PricePointID != "" || opts.Tier > 0 || opts.Price != "" || opts.StartDate != "" || opts.RefreshTierCache
	if !hasPricing {
		result.Steps = append(result.Steps,
			iapSetupStepResult{
				Name:    iapSetupStepResolvePricePoint,
				Status:  "skipped",
				Message: "no pricing flags provided",
			},
			iapSetupStepResult{
				Name:    iapSetupStepCreatePriceSchedule,
				Status:  "skipped",
				Message: "no pricing flags provided",
			},
		)
		return result, nil
	}

	resolvedPricePointID := opts.PricePointID
	if resolvedPricePointID != "" {
		result.Steps = append(result.Steps, iapSetupStepResult{
			Name:    iapSetupStepResolvePricePoint,
			Status:  "completed",
			ID:      resolvedPricePointID,
			Message: "used explicit price point id",
		})
	} else {
		tiers, err := shared.ResolveIAPTiers(requestCtx, client, result.IAPID, opts.BaseTerritory, opts.RefreshTierCache)
		if err != nil {
			result.Status = "error"
			result.FailedStep = iapSetupStepResolvePricePoint
			result.Steps = append(result.Steps, iapSetupStepResult{
				Name:    iapSetupStepResolvePricePoint,
				Status:  "failed",
				Message: err.Error(),
			})
			return result, fmt.Errorf("iap setup: resolve price point: %w", err)
		}
		if opts.Tier > 0 {
			resolvedPricePointID, err = shared.ResolvePricePointByTier(tiers, opts.Tier)
		} else {
			resolvedPricePointID, err = shared.ResolvePricePointByPrice(tiers, opts.Price)
		}
		if err != nil {
			result.Status = "error"
			result.FailedStep = iapSetupStepResolvePricePoint
			result.Steps = append(result.Steps, iapSetupStepResult{
				Name:    iapSetupStepResolvePricePoint,
				Status:  "failed",
				Message: err.Error(),
			})
			return result, fmt.Errorf("iap setup: resolve price point: %w", err)
		}
		result.Steps = append(result.Steps, iapSetupStepResult{
			Name:   iapSetupStepResolvePricePoint,
			Status: "completed",
			ID:     resolvedPricePointID,
		})
	}
	result.ResolvedPricePointID = strings.TrimSpace(resolvedPricePointID)

	priceScheduleResp, err := client.CreateInAppPurchasePriceSchedule(requestCtx, result.IAPID, asc.InAppPurchasePriceScheduleCreateAttributes{
		BaseTerritoryID: opts.BaseTerritory,
		Prices: []asc.InAppPurchasePriceSchedulePrice{
			{
				PricePointID: result.ResolvedPricePointID,
				StartDate:    opts.StartDate,
			},
		},
	})
	if err != nil {
		result.Status = "error"
		result.FailedStep = iapSetupStepCreatePriceSchedule
		result.Steps = append(result.Steps, iapSetupStepResult{
			Name:    iapSetupStepCreatePriceSchedule,
			Status:  "failed",
			Message: err.Error(),
		})
		return result, fmt.Errorf("iap setup: failed to create price schedule: %w", err)
	}

	result.PriceScheduleID = strings.TrimSpace(priceScheduleResp.Data.ID)
	result.Steps = append(result.Steps, iapSetupStepResult{
		Name:   iapSetupStepCreatePriceSchedule,
		Status: "completed",
		ID:     result.PriceScheduleID,
	})

	return result, nil
}

func printIAPSetupResult(result *iapSetupResult, format string, pretty bool) error {
	return shared.PrintOutputWithRenderers(
		result,
		format,
		pretty,
		func() error {
			headers := []string{"Status", "IAP ID", "Localization ID", "Price Schedule ID", "Price Point ID", "Failed Step"}
			rows := [][]string{{
				result.Status,
				result.IAPID,
				result.LocalizationID,
				result.PriceScheduleID,
				result.ResolvedPricePointID,
				result.FailedStep,
			}}
			asc.RenderTable(headers, rows)
			return nil
		},
		func() error {
			headers := []string{"Status", "IAP ID", "Localization ID", "Price Schedule ID", "Price Point ID", "Failed Step"}
			rows := [][]string{{
				result.Status,
				result.IAPID,
				result.LocalizationID,
				result.PriceScheduleID,
				result.ResolvedPricePointID,
				result.FailedStep,
			}}
			asc.RenderMarkdown(headers, rows)
			return nil
		},
	)
}

func resolveIAPSetupAlias(primary, alias, primaryName, aliasName string) (string, error) {
	trimmedPrimary := strings.TrimSpace(primary)
	trimmedAlias := strings.TrimSpace(alias)
	if trimmedPrimary == "" {
		return trimmedAlias, nil
	}
	if trimmedAlias == "" || trimmedAlias == trimmedPrimary {
		return trimmedPrimary, nil
	}
	return "", fmt.Errorf("%s and %s must match when both are provided", primaryName, aliasName)
}
