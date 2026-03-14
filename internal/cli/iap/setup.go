package iap

import (
	"context"
	"flag"
	"fmt"
	"strings"
	"time"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

const (
	iapSetupStepCreateIAP           = "create_iap"
	iapSetupStepCreateLocalization  = "create_localization"
	iapSetupStepResolvePricePoint   = "resolve_price_point"
	iapSetupStepCreatePriceSchedule = "create_price_schedule"
	iapSetupStepVerifyState         = "verify_state"
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
	NoVerify         bool
}

func (o iapSetupOptions) hasPricing(startDateInput string) bool {
	return o.BaseTerritory != "" ||
		o.PricePointID != "" ||
		o.Tier > 0 ||
		o.Price != "" ||
		strings.TrimSpace(startDateInput) != "" ||
		o.RefreshTierCache
}

type iapSetupStepResult struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	ID      string `json:"id,omitempty"`
	Message string `json:"message,omitempty"`
}

type iapSetupResult struct {
	Status               string                `json:"status"`
	Error                string                `json:"error,omitempty"`
	AppID                string                `json:"appId"`
	Type                 string                `json:"type"`
	ProductID            string                `json:"productId"`
	ReferenceName        string                `json:"referenceName"`
	Locale               string                `json:"locale,omitempty"`
	BaseTerritory        string                `json:"baseTerritory,omitempty"`
	IAPID                string                `json:"iapId,omitempty"`
	LocalizationID       string                `json:"localizationId,omitempty"`
	PriceScheduleID      string                `json:"priceScheduleId,omitempty"`
	ResolvedPricePointID string                `json:"resolvedPricePointId,omitempty"`
	Verification         *iapSetupVerification `json:"verification,omitempty"`
	FailedStep           string                `json:"failedStep,omitempty"`
	Steps                []iapSetupStepResult  `json:"steps"`
}

type iapSetupVerification struct {
	Status             string    `json:"status"`
	IAPExists          bool      `json:"iapExists,omitempty"`
	LocalizationExists *bool     `json:"localizationExists,omitempty"`
	PriceVerified      *bool     `json:"priceVerified,omitempty"`
	BaseTerritory      string    `json:"baseTerritory,omitempty"`
	CurrentPrice       *iapMoney `json:"currentPrice,omitempty"`
	ScheduledPrice     *iapMoney `json:"scheduledPrice,omitempty"`
	ScheduledStartDate string    `json:"scheduledStartDate,omitempty"`
	EstimatedProceeds  *iapMoney `json:"estimatedProceeds,omitempty"`
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
	noVerify := fs.Bool("no-verify", false, "Skip post-create readback verification for faster execution")
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

By default, setup reads the created state back from App Store Connect and
verifies the resulting IAP, localization, and current price. Use
--no-verify to skip that postcondition check when speed matters more than
confirmed final state.

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
				NoVerify:         *noVerify,
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

			hasPricing := opts.hasPricing(*startDate)
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
		result.Error = err.Error()
		result.FailedStep = iapSetupStepCreateIAP
		result.Steps = append(result.Steps, iapSetupStepResult{
			Name:    iapSetupStepCreateIAP,
			Status:  "failed",
			Message: err.Error(),
		})
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
		result.Error = err.Error()
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
			result.Error = err.Error()
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

	hasPricing := opts.hasPricing(opts.StartDate)
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
	} else {
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
				result.Error = err.Error()
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
				result.Error = err.Error()
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
			result.Error = err.Error()
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
	}

	if opts.NoVerify {
		result.Verification = &iapSetupVerification{Status: "skipped"}
		result.Steps = append(result.Steps, iapSetupStepResult{
			Name:    iapSetupStepVerifyState,
			Status:  "skipped",
			Message: "--no-verify set",
		})
		return result, nil
	}

	verification, verifyStep, err := verifyIAPSetupState(requestCtx, client, result, opts)
	if err != nil {
		result.Status = "error"
		result.Error = err.Error()
		result.FailedStep = iapSetupStepVerifyState
		result.Verification = verification
		result.Steps = append(result.Steps, verifyStep)
		return result, fmt.Errorf("iap setup: verify state: %w", err)
	}
	result.Verification = verification
	result.Steps = append(result.Steps, verifyStep)

	return result, nil
}

func verifyIAPSetupState(ctx context.Context, client *asc.Client, result iapSetupResult, opts iapSetupOptions) (*iapSetupVerification, iapSetupStepResult, error) {
	verification := &iapSetupVerification{
		Status: "verified",
	}
	now := time.Now().UTC()

	iapResp, err := client.GetInAppPurchaseV2(ctx, result.IAPID)
	if err != nil {
		verification.Status = "failed"
		return verification, iapSetupStepResult{
			Name:    iapSetupStepVerifyState,
			Status:  "failed",
			Message: err.Error(),
		}, fmt.Errorf("fetch created iap: %w", err)
	}

	if strings.TrimSpace(iapResp.Data.ID) == "" {
		verification.Status = "failed"
		return verification, iapSetupStepResult{
			Name:    iapSetupStepVerifyState,
			Status:  "failed",
			Message: "created iap readback returned empty id",
		}, fmt.Errorf("created iap readback returned empty id")
	}
	if iapResp.Data.Attributes.Name != opts.ReferenceName {
		verification.Status = "failed"
		return verification, iapSetupStepResult{
			Name:    iapSetupStepVerifyState,
			Status:  "failed",
			Message: fmt.Sprintf("reference name mismatch: got %q", iapResp.Data.Attributes.Name),
		}, fmt.Errorf("reference name mismatch: got %q want %q", iapResp.Data.Attributes.Name, opts.ReferenceName)
	}
	if iapResp.Data.Attributes.ProductID != opts.ProductID {
		verification.Status = "failed"
		return verification, iapSetupStepResult{
			Name:    iapSetupStepVerifyState,
			Status:  "failed",
			Message: fmt.Sprintf("product id mismatch: got %q", iapResp.Data.Attributes.ProductID),
		}, fmt.Errorf("product id mismatch: got %q want %q", iapResp.Data.Attributes.ProductID, opts.ProductID)
	}
	if iapResp.Data.Attributes.InAppPurchaseType != opts.Type {
		verification.Status = "failed"
		return verification, iapSetupStepResult{
			Name:    iapSetupStepVerifyState,
			Status:  "failed",
			Message: fmt.Sprintf("type mismatch: got %q", iapResp.Data.Attributes.InAppPurchaseType),
		}, fmt.Errorf("type mismatch: got %q want %q", iapResp.Data.Attributes.InAppPurchaseType, opts.Type)
	}
	verification.IAPExists = true

	hasLocalization := opts.Locale != "" || opts.DisplayName != "" || opts.Description != ""
	if hasLocalization {
		locResp, err := client.GetInAppPurchaseLocalizations(ctx, result.IAPID, asc.WithIAPLocalizationsLimit(200))
		if err != nil {
			verification.Status = "failed"
			return verification, iapSetupStepResult{
				Name:    iapSetupStepVerifyState,
				Status:  "failed",
				Message: err.Error(),
			}, fmt.Errorf("fetch created localization: %w", err)
		}

		found := false
		for _, item := range locResp.Data {
			if strings.TrimSpace(item.ID) != result.LocalizationID {
				continue
			}
			if item.Attributes.Locale == opts.Locale && item.Attributes.Name == opts.DisplayName && item.Attributes.Description == opts.Description {
				found = true
				break
			}
		}
		if !found {
			verification.Status = "failed"
			return verification, iapSetupStepResult{
				Name:    iapSetupStepVerifyState,
				Status:  "failed",
				Message: "created localization did not match requested locale/name/description",
			}, fmt.Errorf("created localization did not match requested locale/name/description")
		}
		value := true
		verification.LocalizationExists = &value
	}

	hasPricing := opts.hasPricing(opts.StartDate)
	if hasPricing {
		expectedVerificationPrice, err := resolveExpectedIAPSetupVerificationPrice(ctx, client, result.IAPID, opts)
		if err != nil {
			verification.Status = "failed"
			return verification, iapSetupStepResult{
				Name:    iapSetupStepVerifyState,
				Status:  "failed",
				Message: err.Error(),
			}, err
		}

		summary, err := resolveIAPPriceSummary(ctx, client, iapResp.Data, opts.BaseTerritory, now)
		if err != nil {
			verification.Status = "failed"
			return verification, iapSetupStepResult{
				Name:    iapSetupStepVerifyState,
				Status:  "failed",
				Message: err.Error(),
			}, fmt.Errorf("resolve current pricing: %w", err)
		}
		verification.BaseTerritory = summary.BaseTerritory
		verification.CurrentPrice = summary.CurrentPrice
		verification.EstimatedProceeds = summary.EstimatedProceeds
		if strings.ToUpper(strings.TrimSpace(summary.BaseTerritory)) != opts.BaseTerritory {
			verification.Status = "failed"
			return verification, iapSetupStepResult{
				Name:    iapSetupStepVerifyState,
				Status:  "failed",
				Message: fmt.Sprintf("base territory mismatch: got %q", summary.BaseTerritory),
			}, fmt.Errorf("base territory mismatch: got %q want %q", summary.BaseTerritory, opts.BaseTerritory)
		}
		if isFutureSetupStartDate(opts.StartDate, now) {
			scheduledPrice, scheduledStartDate, err := verifyScheduledIAPSetupPrice(ctx, client, result.IAPID, opts.BaseTerritory, opts.StartDate, expectedVerificationPrice)
			if err != nil {
				verification.Status = "failed"
				return verification, iapSetupStepResult{
					Name:    iapSetupStepVerifyState,
					Status:  "failed",
					Message: err.Error(),
				}, err
			}
			value := true
			verification.PriceVerified = &value
			verification.ScheduledPrice = scheduledPrice
			verification.ScheduledStartDate = scheduledStartDate
			verification.CurrentPrice = nil
		} else {
			if summary.CurrentPrice == nil {
				verification.Status = "failed"
				return verification, iapSetupStepResult{
					Name:    iapSetupStepVerifyState,
					Status:  "failed",
					Message: "current price missing after schedule creation",
				}, fmt.Errorf("current price missing after schedule creation")
			}
			if expectedVerificationPrice != "" {
				priceFilter := shared.PriceFilter{Price: expectedVerificationPrice}
				if !priceFilter.MatchesPrice(summary.CurrentPrice.Amount) {
					verification.Status = "failed"
					return verification, iapSetupStepResult{
						Name:    iapSetupStepVerifyState,
						Status:  "failed",
						Message: fmt.Sprintf("current price mismatch: got %q", summary.CurrentPrice.Amount),
					}, fmt.Errorf("current price mismatch: got %q want %q", summary.CurrentPrice.Amount, expectedVerificationPrice)
				}
			}

			value := true
			verification.PriceVerified = &value
		}
	}

	return verification, iapSetupStepResult{
		Name:   iapSetupStepVerifyState,
		Status: "completed",
	}, nil
}

func verifyScheduledIAPSetupPrice(ctx context.Context, client *asc.Client, iapID, territory, startDate, expectedPrice string) (*iapMoney, string, error) {
	scheduleResp, err := client.GetInAppPurchasePriceSchedule(
		ctx,
		iapID,
		asc.WithIAPPriceScheduleInclude([]string{"baseTerritory", "manualPrices", "automaticPrices"}),
		asc.WithIAPPriceScheduleFields([]string{"baseTerritory", "manualPrices", "automaticPrices"}),
		asc.WithIAPPriceScheduleTerritoryFields([]string{"currency"}),
		asc.WithIAPPriceSchedulePriceFields([]string{"startDate", "endDate", "manual", "inAppPurchasePricePoint", "territory"}),
		asc.WithIAPPriceScheduleManualPricesLimit(maxIncludedScheduleLimit),
		asc.WithIAPPriceScheduleAutomaticPricesLimit(maxIncludedScheduleLimit),
	)
	if err != nil {
		return nil, "", fmt.Errorf("fetch price schedule for verification: %w", err)
	}

	entries, territoryCurrencies, err := parseIAPPriceScheduleIncluded(scheduleResp.Included)
	if err != nil {
		return nil, "", fmt.Errorf("parse price schedule for verification: %w", err)
	}
	if scheduleEntriesRequireFullFetch(entries) {
		entries, err = fetchAllSchedulePriceEntries(ctx, client, scheduleResp.Data.ID)
		if err != nil {
			return nil, "", fmt.Errorf("fetch full price schedule entries for verification: %w", err)
		}
	}

	var matched *iapPriceEntry
	for idx := range entries {
		if strings.ToUpper(strings.TrimSpace(entries[idx].TerritoryID)) != territory {
			continue
		}
		if strings.TrimSpace(entries[idx].StartDate) != startDate {
			continue
		}
		matched = &entries[idx]
		break
	}
	if matched == nil {
		return nil, "", fmt.Errorf("no scheduled price entry found for territory %q starting on %q", territory, startDate)
	}

	pointValues, currency, err := fetchIAPPricePointValues(ctx, client, iapID, territory)
	if err != nil {
		return nil, "", err
	}
	if currency == "" {
		currency = territoryCurrencies[territory]
	}
	if currency == "" {
		currency = territory
	}
	value, ok := pointValues[matched.PricePointID]
	if !ok {
		return nil, "", fmt.Errorf("scheduled price point %q was not returned for territory %q", matched.PricePointID, territory)
	}
	if expectedPrice != "" {
		priceFilter := shared.PriceFilter{Price: expectedPrice}
		if !priceFilter.MatchesPrice(value.CustomerPrice) {
			return nil, "", fmt.Errorf("scheduled price mismatch: got %q want %q", value.CustomerPrice, expectedPrice)
		}
	}

	return &iapMoney{
		Amount:   value.CustomerPrice,
		Currency: currency,
	}, matched.StartDate, nil
}

func resolveExpectedIAPSetupVerificationPrice(ctx context.Context, client *asc.Client, iapID string, opts iapSetupOptions) (string, error) {
	if opts.Price != "" {
		return opts.Price, nil
	}

	if opts.Tier == 0 && opts.PricePointID == "" {
		return "", nil
	}

	tiers, err := shared.ResolveIAPTiers(ctx, client, iapID, opts.BaseTerritory, true)
	if err != nil {
		return "", fmt.Errorf("resolve live tiers for verification: %w", err)
	}

	if opts.Tier > 0 {
		for _, tier := range tiers {
			if tier.Tier == opts.Tier {
				return strings.TrimSpace(tier.CustomerPrice), nil
			}
		}
		return "", fmt.Errorf("tier %d not found during verification", opts.Tier)
	}

	for _, tier := range tiers {
		if strings.TrimSpace(tier.PricePointID) == strings.TrimSpace(opts.PricePointID) {
			return strings.TrimSpace(tier.CustomerPrice), nil
		}
	}

	return "", fmt.Errorf("price point %q not found in %s during verification", opts.PricePointID, opts.BaseTerritory)
}

func printIAPSetupResult(result *iapSetupResult, format string, pretty bool) error {
	return shared.PrintOutputWithRenderers(
		result,
		format,
		pretty,
		func() error {
			headers := []string{"Status", "Verification", "IAP ID", "Localization ID", "Price Schedule ID", "Price Point ID", "Current Price", "Failed Step", "Error"}
			rows := [][]string{{
				result.Status,
				iapSetupVerificationStatus(result.Verification),
				result.IAPID,
				result.LocalizationID,
				result.PriceScheduleID,
				result.ResolvedPricePointID,
				iapSetupVerificationCurrentPrice(result.Verification),
				result.FailedStep,
				result.Error,
			}}
			asc.RenderTable(headers, rows)
			return nil
		},
		func() error {
			headers := []string{"Status", "Verification", "IAP ID", "Localization ID", "Price Schedule ID", "Price Point ID", "Current Price", "Failed Step", "Error"}
			rows := [][]string{{
				result.Status,
				iapSetupVerificationStatus(result.Verification),
				result.IAPID,
				result.LocalizationID,
				result.PriceScheduleID,
				result.ResolvedPricePointID,
				iapSetupVerificationCurrentPrice(result.Verification),
				result.FailedStep,
				result.Error,
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

func iapSetupVerificationStatus(verification *iapSetupVerification) string {
	if verification == nil {
		return ""
	}
	return verification.Status
}

func iapSetupVerificationCurrentPrice(verification *iapSetupVerification) string {
	if verification == nil {
		return ""
	}
	if verification.CurrentPrice != nil {
		return formatIAPMoney(verification.CurrentPrice)
	}
	if verification.ScheduledPrice != nil {
		return strings.TrimSpace(formatIAPMoney(verification.ScheduledPrice) + " (effective " + verification.ScheduledStartDate + ")")
	}
	return ""
}

func isFutureSetupStartDate(startDate string, now time.Time) bool {
	startDate = strings.TrimSpace(startDate)
	if startDate == "" {
		return false
	}
	parsed, err := time.Parse(iapPricesDateLayout, startDate)
	if err != nil {
		return false
	}
	return dateOnlyUTC(parsed.UTC()).After(dateOnlyUTC(now))
}
