package subscriptions

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"sort"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/ascterritory"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	validatecmd "github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/validate"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/validation"
)

var errSubscriptionsSetupExistingResourceFound = errors.New("existing subscription setup resource found")

const (
	subscriptionsSetupStepEnsureGroup             = "ensure_group"
	subscriptionsSetupStepCreateGroupLocalization = "create_group_localization"
	subscriptionsSetupStepCreateSubscription      = "create_subscription"
	subscriptionsSetupStepCreateLocalization      = "create_localization"
	subscriptionsSetupStepUploadReviewScreenshot  = "upload_review_screenshot"
	subscriptionsSetupStepResolvePricePoint       = "resolve_price_point"
	subscriptionsSetupStepSetPrice                = "set_price"
	subscriptionsSetupStepSetAvailability         = "set_availability"
	subscriptionsSetupStepVerifyState             = "verify_state"
)

type subscriptionsSetupOptions struct {
	AppID                     string
	GroupID                   string
	GroupReferenceName        string
	GroupLocale               string
	GroupDisplayName          string
	GroupCustomAppName        string
	ReferenceName             string
	ProductID                 string
	SubscriptionPeriod        asc.SubscriptionPeriod
	FamilySharable            bool
	Locale                    string
	DisplayName               string
	Description               string
	ReviewScreenshot          string
	PriceTerritory            string
	PricePointID              string
	Tier                      int
	Price                     string
	StartDate                 string
	RefreshTierCache          bool
	Territories               []string
	AvailableInNewTerritories bool
	EnableMonthlyCommitment   bool
	NoVerify                  bool
	Repair                    bool
}

func (o subscriptionsSetupOptions) hasPricing(startDateInput string) bool {
	return o.PriceTerritory != "" ||
		o.PricePointID != "" ||
		o.Tier > 0 ||
		o.Price != "" ||
		strings.TrimSpace(startDateInput) != "" ||
		o.RefreshTierCache
}

func (o subscriptionsSetupOptions) hasLocalization() bool {
	return o.Locale != "" || o.DisplayName != "" || o.Description != ""
}

func (o subscriptionsSetupOptions) hasGroupLocalization() bool {
	return o.GroupLocale != "" || o.GroupDisplayName != "" || o.GroupCustomAppName != ""
}

func (o subscriptionsSetupOptions) hasAvailability() bool {
	return len(o.Territories) > 0 || o.AvailableInNewTerritories
}

type subscriptionsSetupStepResult struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	ID      string `json:"id,omitempty"`
	Message string `json:"message,omitempty"`
}

type subscriptionsSetupResult struct {
	Status               string                               `json:"status"`
	Error                string                               `json:"error,omitempty"`
	AppID                string                               `json:"appId,omitempty"`
	GroupID              string                               `json:"groupId,omitempty"`
	GroupReferenceName   string                               `json:"groupReferenceName,omitempty"`
	GroupLocalizationID  string                               `json:"groupLocalizationId,omitempty"`
	SubscriptionID       string                               `json:"subscriptionId,omitempty"`
	ReferenceName        string                               `json:"referenceName,omitempty"`
	ProductID            string                               `json:"productId,omitempty"`
	SubscriptionPeriod   string                               `json:"subscriptionPeriod,omitempty"`
	Locale               string                               `json:"locale,omitempty"`
	PriceTerritory       string                               `json:"priceTerritory,omitempty"`
	LocalizationID       string                               `json:"localizationId,omitempty"`
	ReviewScreenshotID   string                               `json:"reviewScreenshotId,omitempty"`
	AvailabilityID       string                               `json:"availabilityId,omitempty"`
	ResolvedPricePointID string                               `json:"resolvedPricePointId,omitempty"`
	Verification         *subscriptionsSetupVerification      `json:"verification,omitempty"`
	Diagnostics          []validation.SubscriptionDiagnostics `json:"diagnostics,omitempty"`
	FailedStep           string                               `json:"failedStep,omitempty"`
	Steps                []subscriptionsSetupStepResult       `json:"steps"`
}

type subscriptionsSetupVerification struct {
	Status                   string    `json:"status"`
	SubscriptionState        string    `json:"subscriptionState,omitempty"`
	GroupExists              *bool     `json:"groupExists,omitempty"`
	SubscriptionExists       bool      `json:"subscriptionExists,omitempty"`
	LocalizationExists       *bool     `json:"localizationExists,omitempty"`
	GroupLocalizationExists  *bool     `json:"groupLocalizationExists,omitempty"`
	ReviewScreenshotVerified *bool     `json:"reviewScreenshotVerified,omitempty"`
	ReviewScreenshotState    string    `json:"reviewScreenshotState,omitempty"`
	PriceVerified            *bool     `json:"priceVerified,omitempty"`
	AvailabilityVerified     *bool     `json:"availabilityVerified,omitempty"`
	PriceTerritory           string    `json:"priceTerritory,omitempty"`
	CurrentPrice             *subMoney `json:"currentPrice,omitempty"`
	ScheduledPrice           *subMoney `json:"scheduledPrice,omitempty"`
	ScheduledStartDate       string    `json:"scheduledStartDate,omitempty"`
	Territories              []string  `json:"territories,omitempty"`
	PriceCoverageVerified    *bool     `json:"priceCoverageVerified,omitempty"`
	PricedTerritories        []string  `json:"pricedTerritories,omitempty"`
	MissingPriceTerritories  []string  `json:"missingPriceTerritories,omitempty"`
}

// SubscriptionsSetupCommand returns the high-level subscriptions bootstrap workflow command.
func SubscriptionsSetupCommand() *ffcli.Command {
	fs := flag.NewFlagSet("setup", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	groupID := fs.String("group-id", "", "Existing subscription group ID")
	groupReferenceName := fs.String("group-reference-name", "", "Reference name for a new subscription group")
	groupRefNameAlias := fs.String("group-ref-name", "", "Reference name alias for a new subscription group")
	groupLocale := fs.String("group-locale", "", "Locale for the subscription group localization (e.g., en-US)")
	groupDisplayName := fs.String("group-display-name", "", "Localized display name for the subscription group")
	groupCustomAppName := fs.String("group-custom-app-name", "", "Optional custom app name for the subscription group localization")

	referenceName := fs.String("reference-name", "", "Subscription reference name")
	refNameAlias := fs.String("ref-name", "", "Subscription reference name alias")
	productID := fs.String("product-id", "", "Product ID (e.g., com.example.sub.monthly)")
	subscriptionPeriod := fs.String("subscription-period", "", "Subscription period: "+strings.Join(subscriptionPeriodValues, ", "))
	familySharable := fs.Bool("family-sharable", false, "Enable Family Sharing (cannot be undone)")

	locale := fs.String("locale", "", "Locale for the first subscription localization (e.g., en-US)")
	displayName := fs.String("display-name", "", "Display name for the first subscription localization")
	nameAlias := fs.String("name", "", "Display name alias")
	description := fs.String("description", "", "Description for the first subscription localization")
	reviewScreenshot := fs.String("review-screenshot", "", "Path to the App Review screenshot for the subscription")

	priceTerritory := fs.String("price-territory", "", "Territory used to resolve and verify the initial subscription price (accepts alpha-2, alpha-3, or exact English country name)")
	pricePointID := fs.String("price-point-id", "", "Explicit price point ID for the initial subscription price")
	tier := fs.Int("tier", 0, "Pricing tier number for the initial subscription price")
	price := fs.String("price", "", "Customer price for the initial subscription price")
	startDate := fs.String("start-date", "", "Start date for the initial subscription price (YYYY-MM-DD)")
	refresh := fs.Bool("refresh", false, "Force refresh of the subscription price-point tier cache when resolving --tier or --price")

	territories := fs.String("territories", "", "Availability territories to enable after creation (comma-separated; accepts alpha-2, alpha-3, or exact English country names)")
	availableInNewTerritories := fs.Bool("available-in-new-territories", false, "Include new territories automatically when creating availability")
	enableMonthlyCommitment := fs.Bool("enable-monthly-commitment", false, "Also configure Monthly with 12-Month Commitment availability for ONE_YEAR subscriptions")
	noVerify := fs.Bool("no-verify", false, "Skip post-create readback verification for faster execution")
	repair := fs.Bool("repair", false, "Atomically rebuild and re-save the complete equalized price matrix")
	output := shared.BindOutputFlags(fs)

	shared.HideFlagFromHelp(fs.Lookup("group-ref-name"))
	shared.HideFlagFromHelp(fs.Lookup("ref-name"))
	shared.HideFlagFromHelp(fs.Lookup("name"))

	return &ffcli.Command{
		Name:       "setup",
		ShortUsage: "asc subscriptions setup [flags]",
		ShortHelp:  "Create or reconcile a subscription with complete App Store metadata.",
		LongHelp: `Create or reconcile a subscription and its required App Store metadata,
including group localization, subscription localization, review screenshot,
pricing, and availability.

The setup command is create-oriented: use it when you want a one-shot happy
path for a new subscription. Existing low-level commands remain available
for partial updates, repair flows, and advanced cases.

Setup uses Apple's price-point equalizations to materialize the complete App
Store price matrix. Sale availability remains the independently selected subset.

By default, setup reads Apple's final subscription state back and verifies the
resulting metadata, screenshot delivery, pricing, and availability. A final
MISSING_METADATA state fails the command and includes deep diagnostics when
--app is available. Use --repair to atomically rebuild and re-save that complete
matrix even when the selected base price is unchanged. Use --no-verify only
when intentionally skipping these postcondition checks.

Examples:
  asc subscriptions setup --app "APP_ID" --group-reference-name "Pro" --reference-name "Pro Monthly" --product-id "com.example.pro.monthly" --subscription-period ONE_MONTH
  asc subscriptions setup --app "APP_ID" --group-reference-name "Pro" --group-locale "en-US" --group-display-name "Premium" --reference-name "Pro Monthly" --product-id "com.example.pro.monthly" --subscription-period ONE_MONTH --locale "en-US" --display-name "Pro Monthly" --description "Unlock everything" --review-screenshot "./review.png"
  asc subscriptions setup --app "APP_ID" --group-reference-name "Pro" --reference-name "Pro Monthly" --product-id "com.example.pro.monthly" --price "3.99" --price-territory "United States" --territories "US,Canada"
  asc subscriptions setup --app "APP_ID" --group-reference-name "Pro" --reference-name "Pro Yearly" --product-id "com.example.pro.yearly" --subscription-period ONE_YEAR --enable-monthly-commitment
  asc subscriptions setup --group-id "GROUP_ID" --reference-name "Pro Monthly" --product-id "com.example.pro.monthly" --subscription-period ONE_MONTH --no-verify`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := shared.RecoverBoolFlagTailArgs(fs, args, availableInNewTerritories); err != nil {
				return err
			}

			groupReferenceNameValue, err := resolveSubscriptionsSetupAlias(*groupReferenceName, *groupRefNameAlias, "--group-reference-name", "--group-ref-name")
			if err != nil {
				return shared.UsageError(err.Error())
			}
			referenceNameValue, err := resolveSubscriptionsSetupAlias(*referenceName, *refNameAlias, "--reference-name", "--ref-name")
			if err != nil {
				return shared.UsageError(err.Error())
			}
			displayNameValue, err := resolveSubscriptionsSetupAlias(*displayName, *nameAlias, "--display-name", "--name")
			if err != nil {
				return shared.UsageError(err.Error())
			}

			priceTerritoryValue := strings.TrimSpace(*priceTerritory)
			if priceTerritoryValue != "" {
				priceTerritoryValue, err = ascterritory.Normalize(priceTerritoryValue)
				if err != nil {
					return shared.UsageError(err.Error())
				}
			}
			territoryValues, err := shared.NormalizeASCTerritoryCSV(*territories)
			if err != nil {
				return shared.UsageError(err.Error())
			}

			opts := subscriptionsSetupOptions{
				AppID:                     shared.ResolveAppID(*appID),
				GroupID:                   strings.TrimSpace(*groupID),
				GroupReferenceName:        groupReferenceNameValue,
				GroupLocale:               strings.TrimSpace(*groupLocale),
				GroupDisplayName:          strings.TrimSpace(*groupDisplayName),
				GroupCustomAppName:        strings.TrimSpace(*groupCustomAppName),
				ReferenceName:             referenceNameValue,
				ProductID:                 strings.TrimSpace(*productID),
				FamilySharable:            *familySharable,
				Locale:                    strings.TrimSpace(*locale),
				DisplayName:               displayNameValue,
				Description:               strings.TrimSpace(*description),
				ReviewScreenshot:          strings.TrimSpace(*reviewScreenshot),
				PriceTerritory:            priceTerritoryValue,
				PricePointID:              strings.TrimSpace(*pricePointID),
				Tier:                      *tier,
				Price:                     strings.TrimSpace(*price),
				Territories:               territoryValues,
				AvailableInNewTerritories: *availableInNewTerritories,
				EnableMonthlyCommitment:   *enableMonthlyCommitment,
				RefreshTierCache:          *refresh,
				NoVerify:                  *noVerify,
				Repair:                    *repair,
			}

			if opts.GroupID == "" && opts.GroupReferenceName == "" {
				return shared.UsageError("one of --group-id or --group-reference-name is required")
			}
			if opts.GroupID != "" && opts.GroupReferenceName != "" {
				return shared.UsageError("--group-id and --group-reference-name are mutually exclusive")
			}
			if opts.GroupID == "" && opts.AppID == "" {
				return shared.UsageError("--app is required when creating a new group")
			}
			if opts.ReferenceName == "" {
				return shared.UsageError("--reference-name is required")
			}
			if opts.ProductID == "" {
				return shared.UsageError("--product-id is required")
			}

			normalizedPeriod, err := normalizeSubscriptionPeriod(*subscriptionPeriod, false)
			if err != nil {
				return shared.UsageError(err.Error())
			}
			opts.SubscriptionPeriod = normalizedPeriod
			if opts.EnableMonthlyCommitment && opts.SubscriptionPeriod != asc.SubscriptionPeriodOneYear {
				return shared.UsageError("--enable-monthly-commitment requires --subscription-period ONE_YEAR")
			}
			if opts.EnableMonthlyCommitment {
				return shared.UsageError("--enable-monthly-commitment is not supported by setup yet; create the ONE_YEAR subscription first, then run `asc subscriptions pricing monthly-commitment enable`")
			}

			if opts.hasLocalization() {
				if opts.Locale == "" {
					return shared.UsageError("--locale is required when localization flags are provided")
				}
				if opts.DisplayName == "" {
					return shared.UsageError("--display-name is required when localization flags are provided")
				}
			}
			if opts.hasGroupLocalization() {
				if opts.GroupLocale == "" {
					return shared.UsageError("--group-locale is required when group localization flags are provided")
				}
				if opts.GroupDisplayName == "" {
					return shared.UsageError("--group-display-name is required when group localization flags are provided")
				}
			}
			if opts.Repair && !opts.hasPricing(*startDate) {
				return shared.UsageError("--repair requires pricing flags")
			}
			if opts.ReviewScreenshot != "" {
				file, _, err := openSubscriptionImageFile(opts.ReviewScreenshot)
				if err != nil {
					return shared.UsageError(fmt.Sprintf("invalid --review-screenshot: %v", err))
				}
				_ = file.Close()
			}

			if err := shared.ValidateFinitePriceFlag("--price", opts.Price); err != nil {
				return shared.UsageError(err.Error())
			}
			if opts.Tier < 0 {
				return shared.UsageError("--tier must be a positive integer")
			}
			hasPricing := opts.hasPricing(*startDate)
			if hasPricing {
				if opts.PriceTerritory == "" {
					return shared.UsageError("--price-territory is required when pricing flags are provided")
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
				normalizedStartDate, err := shared.NormalizeDate(*startDate, "--start-date")
				if err != nil {
					return shared.UsageError(err.Error())
				}
				opts.StartDate = normalizedStartDate
			}

			if opts.hasAvailability() && len(subscriptionsSetupAvailabilityTerritories(opts)) == 0 {
				return shared.UsageError("--territories is required when availability flags are provided unless --price-territory can be used to derive availability")
			}

			result, runErr := executeSubscriptionsSetup(ctx, opts)
			if printErr := printSubscriptionsSetupResult(&result, *output.Output, *output.Pretty); printErr != nil {
				return printErr
			}
			if runErr != nil {
				return shared.NewReportedError(runErr)
			}
			return nil
		},
	}
}

func executeSubscriptionsSetup(ctx context.Context, opts subscriptionsSetupOptions) (subscriptionsSetupResult, error) {
	availabilityTerritories := subscriptionsSetupAvailabilityTerritories(opts)

	result := subscriptionsSetupResult{
		Status:             "ok",
		AppID:              opts.AppID,
		GroupID:            opts.GroupID,
		GroupReferenceName: opts.GroupReferenceName,
		ReferenceName:      opts.ReferenceName,
		ProductID:          opts.ProductID,
		SubscriptionPeriod: string(opts.SubscriptionPeriod),
		Locale:             opts.Locale,
		PriceTerritory:     opts.PriceTerritory,
		Steps:              make([]subscriptionsSetupStepResult, 0, 7),
	}

	client, err := shared.GetASCClient()
	if err != nil {
		result.Status = "error"
		result.Error = err.Error()
		result.FailedStep = subscriptionsSetupStepEnsureGroup
		result.Steps = append(result.Steps, subscriptionsSetupStepResult{
			Name:    subscriptionsSetupStepEnsureGroup,
			Status:  "failed",
			Message: err.Error(),
		})
		return result, fmt.Errorf("subscriptions setup: %w", err)
	}

	reusedGroup := strings.TrimSpace(opts.GroupID) != ""
	if strings.TrimSpace(opts.GroupID) == "" {
		groupCtx, groupCancel := shared.ContextWithTimeout(ctx)
		groupID, found, err := findExistingSubscriptionSetupGroup(groupCtx, client, opts.AppID, opts.GroupReferenceName)
		groupCancel()
		if err != nil {
			result.Status = "error"
			result.Error = err.Error()
			result.FailedStep = subscriptionsSetupStepEnsureGroup
			result.Steps = append(result.Steps, subscriptionsSetupStepResult{
				Name:    subscriptionsSetupStepEnsureGroup,
				Status:  "failed",
				Message: err.Error(),
			})
			return result, fmt.Errorf("subscriptions setup: failed to find existing group: %w", err)
		}
		if found {
			reusedGroup = true
			result.GroupID = groupID
			result.Steps = append(result.Steps, subscriptionsSetupStepResult{
				Name:    subscriptionsSetupStepEnsureGroup,
				Status:  "completed",
				ID:      result.GroupID,
				Message: "used existing group",
			})
		} else {
			groupCtx, groupCancel := shared.ContextWithTimeout(ctx)
			groupResp, err := client.CreateSubscriptionGroup(groupCtx, opts.AppID, asc.SubscriptionGroupCreateAttributes{
				ReferenceName: opts.GroupReferenceName,
			})
			groupCancel()
			if err != nil {
				result.Status = "error"
				result.Error = err.Error()
				result.FailedStep = subscriptionsSetupStepEnsureGroup
				result.Steps = append(result.Steps, subscriptionsSetupStepResult{
					Name:    subscriptionsSetupStepEnsureGroup,
					Status:  "failed",
					Message: err.Error(),
				})
				return result, fmt.Errorf("subscriptions setup: failed to create group: %w", err)
			}
			result.GroupID = strings.TrimSpace(groupResp.Data.ID)
			result.Steps = append(result.Steps, subscriptionsSetupStepResult{
				Name:   subscriptionsSetupStepEnsureGroup,
				Status: "completed",
				ID:     result.GroupID,
			})
		}
	} else {
		result.Steps = append(result.Steps, subscriptionsSetupStepResult{
			Name:    subscriptionsSetupStepEnsureGroup,
			Status:  "completed",
			ID:      result.GroupID,
			Message: "used existing group",
		})
	}

	if !opts.hasGroupLocalization() {
		result.Steps = append(result.Steps, subscriptionsSetupStepResult{
			Name:    subscriptionsSetupStepCreateGroupLocalization,
			Status:  "skipped",
			Message: "no group localization flags provided",
		})
	} else {
		groupLocCtx, groupLocCancel := shared.ContextWithTimeout(ctx)
		groupLocalization, found, err := findExistingSubscriptionSetupGroupLocalization(groupLocCtx, client, result.GroupID, opts.GroupLocale)
		groupLocCancel()
		if err != nil {
			return failSubscriptionsSetupStep(result, subscriptionsSetupStepCreateGroupLocalization, err, "failed to inspect group localization")
		}
		if found {
			if err := validateExistingSubscriptionSetupGroupLocalization(groupLocalization, opts); err != nil {
				return failSubscriptionsSetupStep(result, subscriptionsSetupStepCreateGroupLocalization, err, "group localization does not match")
			}
			result.GroupLocalizationID = strings.TrimSpace(groupLocalization.ID)
			result.Steps = append(result.Steps, subscriptionsSetupStepResult{
				Name:    subscriptionsSetupStepCreateGroupLocalization,
				Status:  "completed",
				ID:      result.GroupLocalizationID,
				Message: "used existing group localization",
			})
		} else {
			attrs := asc.SubscriptionGroupLocalizationCreateAttributes{
				Locale: opts.GroupLocale,
				Name:   opts.GroupDisplayName,
			}
			if opts.GroupCustomAppName != "" {
				attrs.CustomAppName = opts.GroupCustomAppName
			}
			groupLocCtx, groupLocCancel := shared.ContextWithTimeout(ctx)
			groupLocalizationResp, err := client.CreateSubscriptionGroupLocalization(groupLocCtx, result.GroupID, attrs)
			groupLocCancel()
			if err != nil {
				return failSubscriptionsSetupStep(result, subscriptionsSetupStepCreateGroupLocalization, err, "failed to create group localization")
			}
			result.GroupLocalizationID = strings.TrimSpace(groupLocalizationResp.Data.ID)
			result.Steps = append(result.Steps, subscriptionsSetupStepResult{
				Name:   subscriptionsSetupStepCreateGroupLocalization,
				Status: "completed",
				ID:     result.GroupLocalizationID,
			})
		}
	}

	subAttrs := asc.SubscriptionCreateAttributes{
		Name:      opts.ReferenceName,
		ProductID: opts.ProductID,
	}
	if opts.SubscriptionPeriod != "" {
		subAttrs.SubscriptionPeriod = string(opts.SubscriptionPeriod)
	}
	if opts.FamilySharable {
		val := true
		subAttrs.FamilySharable = &val
	}

	reusedSubscription := false
	if reusedGroup {
		subCtx, subCancel := shared.ContextWithTimeout(ctx)
		existingSub, found, err := findExistingSubscriptionSetupSubscription(subCtx, client, result.GroupID, opts.ProductID)
		subCancel()
		if err != nil {
			result.Status = "error"
			result.Error = err.Error()
			result.FailedStep = subscriptionsSetupStepCreateSubscription
			result.Steps = append(result.Steps, subscriptionsSetupStepResult{
				Name:    subscriptionsSetupStepCreateSubscription,
				Status:  "failed",
				Message: err.Error(),
			})
			return result, fmt.Errorf("subscriptions setup: failed to find existing subscription: %w", err)
		}
		if found {
			if err := validateExistingSubscriptionSetupSubscription(existingSub, subAttrs, opts.FamilySharable); err != nil {
				result.Status = "error"
				result.Error = err.Error()
				result.FailedStep = subscriptionsSetupStepCreateSubscription
				result.Steps = append(result.Steps, subscriptionsSetupStepResult{
					Name:    subscriptionsSetupStepCreateSubscription,
					Status:  "failed",
					Message: err.Error(),
				})
				return result, fmt.Errorf("subscriptions setup: %w", err)
			}
			reusedSubscription = true
			result.SubscriptionID = strings.TrimSpace(existingSub.ID)
			result.Steps = append(result.Steps, subscriptionsSetupStepResult{
				Name:    subscriptionsSetupStepCreateSubscription,
				Status:  "completed",
				ID:      result.SubscriptionID,
				Message: "used existing subscription",
			})
		}
	}
	if !reusedSubscription {
		subCtx, subCancel := shared.ContextWithTimeout(ctx)
		subResp, err := client.CreateSubscription(subCtx, result.GroupID, subAttrs)
		subCancel()
		if err != nil {
			result.Status = "error"
			result.Error = err.Error()
			result.FailedStep = subscriptionsSetupStepCreateSubscription
			result.Steps = append(result.Steps, subscriptionsSetupStepResult{
				Name:    subscriptionsSetupStepCreateSubscription,
				Status:  "failed",
				Message: err.Error(),
			})
			return result, fmt.Errorf("subscriptions setup: failed to create subscription: %w", err)
		}

		result.SubscriptionID = strings.TrimSpace(subResp.Data.ID)
		result.Steps = append(result.Steps, subscriptionsSetupStepResult{
			Name:   subscriptionsSetupStepCreateSubscription,
			Status: "completed",
			ID:     result.SubscriptionID,
		})
	}

	var preflightPricePointID string
	var preflightFoundPrice bool
	var preflightHasExistingPrices bool
	pricePreflightDone := false
	if reusedSubscription && opts.hasPricing(opts.StartDate) {
		pricePointCtx, pricePointCancel := shared.ContextWithTimeout(ctx)
		resolvedPricePointID, err := resolveExpectedSubscriptionSetupPricePoint(pricePointCtx, client, result.SubscriptionID, opts)
		pricePointCancel()
		if err != nil {
			result.Status = "error"
			result.Error = err.Error()
			result.FailedStep = subscriptionsSetupStepResolvePricePoint
			result.Steps = append(result.Steps, subscriptionsSetupStepResult{
				Name:    subscriptionsSetupStepResolvePricePoint,
				Status:  "failed",
				Message: err.Error(),
			})
			return result, err
		}
		preflightPricePointID = resolvedPricePointID
		priceAttrs := asc.SubscriptionPriceCreateAttributes{
			StartDate: opts.StartDate,
			PlanType:  asc.SubscriptionPlanTypeUpfront,
		}
		priceCtx, priceCancel := shared.ContextWithTimeout(ctx)
		preflightFoundPrice, preflightHasExistingPrices, err = findExistingSubscriptionSetupPrice(priceCtx, client, result.SubscriptionID, resolvedPricePointID, opts.PriceTerritory, priceAttrs)
		priceCancel()
		if err != nil {
			result.Status = "error"
			result.Error = err.Error()
			result.FailedStep = subscriptionsSetupStepSetPrice
			result.Steps = append(result.Steps, subscriptionsSetupStepResult{
				Name:    subscriptionsSetupStepSetPrice,
				Status:  "failed",
				Message: err.Error(),
			})
			return result, fmt.Errorf("subscriptions setup: failed to find existing price: %w", err)
		}
		pricePreflightDone = true
		if preflightHasExistingPrices && !preflightFoundPrice {
			err := mismatchedExistingSubscriptionSetupPriceError(result.SubscriptionID)
			result.Status = "error"
			result.Error = err.Error()
			result.FailedStep = subscriptionsSetupStepSetPrice
			result.Steps = append(result.Steps, subscriptionsSetupStepResult{
				Name:    subscriptionsSetupStepSetPrice,
				Status:  "failed",
				Message: err.Error(),
			})
			return result, err
		}
	}

	var preflightAvailabilityID string
	var preflightFoundAvailability bool
	var preflightHasAvailability bool
	availabilityPreflightDone := false
	if reusedSubscription && len(availabilityTerritories) > 0 {
		availabilityAttrs := asc.SubscriptionAvailabilityAttributes{
			AvailableInNewTerritories: opts.AvailableInNewTerritories,
		}
		availabilityCtx, availabilityCancel := shared.ContextWithTimeout(ctx)
		preflightAvailabilityID, preflightFoundAvailability, preflightHasAvailability, err = findExistingSubscriptionSetupAvailability(availabilityCtx, client, result.SubscriptionID, availabilityTerritories, availabilityAttrs)
		availabilityCancel()
		if err != nil {
			result.Status = "error"
			result.Error = err.Error()
			result.FailedStep = subscriptionsSetupStepSetAvailability
			result.Steps = append(result.Steps, subscriptionsSetupStepResult{
				Name:    subscriptionsSetupStepSetAvailability,
				Status:  "failed",
				Message: err.Error(),
			})
			return result, fmt.Errorf("subscriptions setup: failed to find existing availability: %w", err)
		}
		availabilityPreflightDone = true
		if preflightHasAvailability && !preflightFoundAvailability {
			err := mismatchedExistingSubscriptionSetupAvailabilityError(result.SubscriptionID)
			result.Status = "error"
			result.Error = err.Error()
			result.FailedStep = subscriptionsSetupStepSetAvailability
			result.Steps = append(result.Steps, subscriptionsSetupStepResult{
				Name:    subscriptionsSetupStepSetAvailability,
				Status:  "failed",
				Message: err.Error(),
			})
			return result, err
		}
	}

	if !opts.hasLocalization() {
		result.Steps = append(result.Steps, subscriptionsSetupStepResult{
			Name:    subscriptionsSetupStepCreateLocalization,
			Status:  "skipped",
			Message: "no localization flags provided",
		})
	} else {
		reusedLocalization := false
		if reusedSubscription {
			locCtx, locCancel := shared.ContextWithTimeout(ctx)
			localization, found, err := findExistingSubscriptionSetupLocalization(locCtx, client, result.SubscriptionID, opts.Locale)
			locCancel()
			if err != nil {
				result.Status = "error"
				result.Error = err.Error()
				result.FailedStep = subscriptionsSetupStepCreateLocalization
				result.Steps = append(result.Steps, subscriptionsSetupStepResult{
					Name:    subscriptionsSetupStepCreateLocalization,
					Status:  "failed",
					Message: err.Error(),
				})
				return result, fmt.Errorf("subscriptions setup: failed to find existing localization: %w", err)
			}
			if found {
				if err := validateExistingSubscriptionSetupLocalization(localization, opts); err != nil {
					result.Status = "error"
					result.Error = err.Error()
					result.FailedStep = subscriptionsSetupStepCreateLocalization
					result.Steps = append(result.Steps, subscriptionsSetupStepResult{
						Name:    subscriptionsSetupStepCreateLocalization,
						Status:  "failed",
						Message: err.Error(),
					})
					return result, fmt.Errorf("subscriptions setup: %w", err)
				}
				reusedLocalization = true
				result.LocalizationID = strings.TrimSpace(localization.ID)
				result.Steps = append(result.Steps, subscriptionsSetupStepResult{
					Name:    subscriptionsSetupStepCreateLocalization,
					Status:  "completed",
					ID:      result.LocalizationID,
					Message: "used existing localization",
				})
			}
		}
		if !reusedLocalization {
			locCtx, locCancel := shared.ContextWithTimeout(ctx)
			locResp, err := client.CreateSubscriptionLocalization(locCtx, result.SubscriptionID, asc.SubscriptionLocalizationCreateAttributes{
				Name:        opts.DisplayName,
				Locale:      opts.Locale,
				Description: opts.Description,
			})
			locCancel()
			if err != nil {
				result.Status = "error"
				result.Error = err.Error()
				result.FailedStep = subscriptionsSetupStepCreateLocalization
				result.Steps = append(result.Steps, subscriptionsSetupStepResult{
					Name:    subscriptionsSetupStepCreateLocalization,
					Status:  "failed",
					Message: err.Error(),
				})
				return result, fmt.Errorf("subscriptions setup: failed to create localization: %w", err)
			}
			result.LocalizationID = strings.TrimSpace(locResp.Data.ID)
			result.Steps = append(result.Steps, subscriptionsSetupStepResult{
				Name:   subscriptionsSetupStepCreateLocalization,
				Status: "completed",
				ID:     result.LocalizationID,
			})
		}
	}

	if opts.ReviewScreenshot == "" {
		result.Steps = append(result.Steps, subscriptionsSetupStepResult{
			Name:    subscriptionsSetupStepUploadReviewScreenshot,
			Status:  "skipped",
			Message: "no review screenshot provided",
		})
	} else {
		file, info, err := openSubscriptionImageFile(opts.ReviewScreenshot)
		if err != nil {
			return failSubscriptionsSetupStep(result, subscriptionsSetupStepUploadReviewScreenshot, err, "invalid review screenshot")
		}
		_ = file.Close()
		checksum, err := asc.ComputeFileChecksum(opts.ReviewScreenshot, asc.ChecksumAlgorithmMD5)
		if err != nil {
			return failSubscriptionsSetupStep(result, subscriptionsSetupStepUploadReviewScreenshot, err, "review screenshot checksum failed")
		}
		screenshotResp, err := createOrResumeSubscriptionReviewScreenshot(ctx, client, result.SubscriptionID, opts.ReviewScreenshot, info, checksum.Hash)
		if err != nil {
			return failSubscriptionsSetupStep(result, subscriptionsSetupStepUploadReviewScreenshot, err, "failed to upload review screenshot")
		}
		result.ReviewScreenshotID = strings.TrimSpace(screenshotResp.Data.ID)
		result.Steps = append(result.Steps, subscriptionsSetupStepResult{
			Name:   subscriptionsSetupStepUploadReviewScreenshot,
			Status: "completed",
			ID:     result.ReviewScreenshotID,
		})
	}

	if !opts.hasPricing(opts.StartDate) {
		result.Steps = append(
			result.Steps,
			subscriptionsSetupStepResult{
				Name:    subscriptionsSetupStepResolvePricePoint,
				Status:  "skipped",
				Message: "no pricing flags provided",
			},
			subscriptionsSetupStepResult{
				Name:    subscriptionsSetupStepSetPrice,
				Status:  "skipped",
				Message: "no pricing flags provided",
			},
		)
	} else {
		resolvedPricePointID := preflightPricePointID
		if !pricePreflightDone {
			pricePointCtx, pricePointCancel := shared.ContextWithTimeout(ctx)
			var err error
			resolvedPricePointID, err = resolveExpectedSubscriptionSetupPricePoint(pricePointCtx, client, result.SubscriptionID, opts)
			pricePointCancel()
			if err != nil {
				result.Status = "error"
				result.Error = err.Error()
				result.FailedStep = subscriptionsSetupStepResolvePricePoint
				result.Steps = append(result.Steps, subscriptionsSetupStepResult{
					Name:    subscriptionsSetupStepResolvePricePoint,
					Status:  "failed",
					Message: err.Error(),
				})
				return result, err
			}
		}
		result.ResolvedPricePointID = resolvedPricePointID
		result.Steps = append(result.Steps, subscriptionsSetupStepResult{
			Name:   subscriptionsSetupStepResolvePricePoint,
			Status: "completed",
			ID:     result.ResolvedPricePointID,
		})

		priceAttrs := asc.SubscriptionPriceCreateAttributes{
			StartDate: opts.StartDate,
			PlanType:  asc.SubscriptionPlanTypeUpfront,
		}
		found := false
		hasExistingPrices := false
		if reusedSubscription {
			if pricePreflightDone {
				found = preflightFoundPrice
				hasExistingPrices = preflightHasExistingPrices
			} else {
				priceCtx, priceCancel := shared.ContextWithTimeout(ctx)
				found, hasExistingPrices, err = findExistingSubscriptionSetupPrice(priceCtx, client, result.SubscriptionID, result.ResolvedPricePointID, opts.PriceTerritory, priceAttrs)
				priceCancel()
				if err != nil {
					result.Status = "error"
					result.Error = err.Error()
					result.FailedStep = subscriptionsSetupStepSetPrice
					result.Steps = append(result.Steps, subscriptionsSetupStepResult{
						Name:    subscriptionsSetupStepSetPrice,
						Status:  "failed",
						Message: err.Error(),
					})
					return result, fmt.Errorf("subscriptions setup: failed to find existing price: %w", err)
				}
			}
		}
		if hasExistingPrices && !found && !opts.Repair {
			err := mismatchedExistingSubscriptionSetupPriceError(result.SubscriptionID)
			result.Status = "error"
			result.Error = err.Error()
			result.FailedStep = subscriptionsSetupStepSetPrice
			result.Steps = append(result.Steps, subscriptionsSetupStepResult{
				Name:    subscriptionsSetupStepSetPrice,
				Status:  "failed",
				Message: err.Error(),
			})
			return result, err
		}
		matrixTerritories, mutated, err := ensureSubscriptionsSetupPriceMatrix(ctx, client, result.SubscriptionID, result.ResolvedPricePointID, opts.PriceTerritory, priceAttrs, opts.Repair)
		if err != nil {
			return failSubscriptionsSetupStep(result, subscriptionsSetupStepSetPrice, err, "failed to materialize complete price matrix")
		}
		message := fmt.Sprintf("verified complete price matrix across %d territories", len(matrixTerritories))
		if mutated {
			message = fmt.Sprintf("materialized complete price matrix across %d territories", len(matrixTerritories))
		}
		result.Steps = append(result.Steps, subscriptionsSetupStepResult{
			Name:    subscriptionsSetupStepSetPrice,
			Status:  "completed",
			ID:      result.SubscriptionID,
			Message: message,
		})
	}

	if len(availabilityTerritories) == 0 {
		result.Steps = append(result.Steps, subscriptionsSetupStepResult{
			Name:    subscriptionsSetupStepSetAvailability,
			Status:  "skipped",
			Message: "no availability flags provided",
		})
	} else {
		availabilityAttrs := asc.SubscriptionAvailabilityAttributes{
			AvailableInNewTerritories: opts.AvailableInNewTerritories,
		}
		existingAvailabilityID := ""
		found := false
		hasExistingAvailability := false
		if reusedSubscription {
			if availabilityPreflightDone {
				existingAvailabilityID = preflightAvailabilityID
				found = preflightFoundAvailability
				hasExistingAvailability = preflightHasAvailability
			} else {
				availabilityCtx, availabilityCancel := shared.ContextWithTimeout(ctx)
				existingAvailabilityID, found, hasExistingAvailability, err = findExistingSubscriptionSetupAvailability(availabilityCtx, client, result.SubscriptionID, availabilityTerritories, availabilityAttrs)
				availabilityCancel()
				if err != nil {
					result.Status = "error"
					result.Error = err.Error()
					result.FailedStep = subscriptionsSetupStepSetAvailability
					result.Steps = append(result.Steps, subscriptionsSetupStepResult{
						Name:    subscriptionsSetupStepSetAvailability,
						Status:  "failed",
						Message: err.Error(),
					})
					return result, fmt.Errorf("subscriptions setup: failed to find existing availability: %w", err)
				}
			}
		}
		if found {
			result.AvailabilityID = existingAvailabilityID
			result.Steps = append(result.Steps, subscriptionsSetupStepResult{
				Name:    subscriptionsSetupStepSetAvailability,
				Status:  "completed",
				ID:      result.AvailabilityID,
				Message: "used existing availability",
			})
		} else if hasExistingAvailability {
			err := mismatchedExistingSubscriptionSetupAvailabilityError(result.SubscriptionID)
			result.Status = "error"
			result.Error = err.Error()
			result.FailedStep = subscriptionsSetupStepSetAvailability
			result.Steps = append(result.Steps, subscriptionsSetupStepResult{
				Name:    subscriptionsSetupStepSetAvailability,
				Status:  "failed",
				Message: err.Error(),
			})
			return result, err
		} else {
			availabilityCtx, availabilityCancel := shared.ContextWithTimeout(ctx)
			availabilityResp, err := client.CreateSubscriptionAvailability(availabilityCtx, result.SubscriptionID, availabilityTerritories, availabilityAttrs)
			availabilityCancel()
			if err != nil {
				result.Status = "error"
				result.Error = err.Error()
				result.FailedStep = subscriptionsSetupStepSetAvailability
				result.Steps = append(result.Steps, subscriptionsSetupStepResult{
					Name:    subscriptionsSetupStepSetAvailability,
					Status:  "failed",
					Message: err.Error(),
				})
				return result, fmt.Errorf("subscriptions setup: failed to set availability: %w", err)
			}
			result.AvailabilityID = strings.TrimSpace(availabilityResp.Data.ID)
			result.Steps = append(result.Steps, subscriptionsSetupStepResult{
				Name:    subscriptionsSetupStepSetAvailability,
				Status:  "completed",
				ID:      result.AvailabilityID,
				Message: subscriptionsSetupAvailabilityMessage(opts, availabilityTerritories),
			})
		}
	}

	if opts.NoVerify {
		result.Verification = &subscriptionsSetupVerification{Status: "skipped"}
		result.Steps = append(result.Steps, subscriptionsSetupStepResult{
			Name:    subscriptionsSetupStepVerifyState,
			Status:  "skipped",
			Message: "--no-verify set",
		})
		return result, nil
	}

	verification, verifyStep, err := verifySubscriptionsSetupState(ctx, client, result, opts, availabilityTerritories, reusedSubscription)
	if err != nil {
		result.Status = "error"
		result.Error = err.Error()
		result.FailedStep = subscriptionsSetupStepVerifyState
		result.Verification = verification
		if verification != nil && strings.EqualFold(strings.TrimSpace(verification.SubscriptionState), "MISSING_METADATA") && opts.AppID != "" {
			diagnostics, diagnosticsErr := subscriptionsSetupDiagnostics(ctx, opts.AppID, result.SubscriptionID)
			if diagnosticsErr != nil {
				verifyStep.Message = fmt.Sprintf("%s; deep diagnostics unavailable: %v", verifyStep.Message, diagnosticsErr)
			} else {
				result.Diagnostics = diagnostics
			}
		}
		result.Steps = append(result.Steps, verifyStep)
		return result, fmt.Errorf("subscriptions setup: verify state: %w", err)
	}
	result.Verification = verification
	result.Steps = append(result.Steps, verifyStep)

	return result, nil
}

func verifySubscriptionsSetupState(ctx context.Context, client *asc.Client, result subscriptionsSetupResult, opts subscriptionsSetupOptions, availabilityTerritories []string, reusedSubscription bool) (*subscriptionsSetupVerification, subscriptionsSetupStepResult, error) {
	verification := &subscriptionsSetupVerification{Status: "verified"}

	groupCtx, groupCancel := shared.ContextWithTimeout(ctx)
	groupResp, err := client.GetSubscriptionGroup(groupCtx, result.GroupID)
	groupCancel()
	if err != nil {
		verification.Status = "failed"
		return verification, subscriptionsSetupStepResult{Name: subscriptionsSetupStepVerifyState, Status: "failed", Message: err.Error()}, fmt.Errorf("fetch created group: %w", err)
	}
	groupExists := strings.TrimSpace(groupResp.Data.ID) != ""
	verification.GroupExists = &groupExists
	if opts.GroupReferenceName != "" && groupResp.Data.Attributes.ReferenceName != opts.GroupReferenceName {
		verification.Status = "failed"
		return verification, subscriptionsSetupStepResult{Name: subscriptionsSetupStepVerifyState, Status: "failed", Message: fmt.Sprintf("group reference mismatch: got %q", groupResp.Data.Attributes.ReferenceName)}, fmt.Errorf("group reference mismatch: got %q want %q", groupResp.Data.Attributes.ReferenceName, opts.GroupReferenceName)
	}
	if opts.hasGroupLocalization() {
		groupLocCtx, groupLocCancel := shared.ContextWithTimeout(ctx)
		groupLocalization, found, err := findExistingSubscriptionSetupGroupLocalization(groupLocCtx, client, result.GroupID, opts.GroupLocale)
		groupLocCancel()
		if err != nil {
			verification.Status = "failed"
			return verification, subscriptionsSetupStepResult{Name: subscriptionsSetupStepVerifyState, Status: "failed", Message: err.Error()}, fmt.Errorf("fetch created group localization: %w", err)
		}
		if !found || strings.TrimSpace(groupLocalization.ID) != result.GroupLocalizationID {
			verification.Status = "failed"
			return verification, subscriptionsSetupStepResult{Name: subscriptionsSetupStepVerifyState, Status: "failed", Message: "created group localization was not found"}, fmt.Errorf("created group localization was not found")
		}
		value := true
		verification.GroupLocalizationExists = &value
	}

	subCtx, subCancel := shared.ContextWithTimeout(ctx)
	subResp, err := client.GetSubscription(subCtx, result.SubscriptionID)
	subCancel()
	if err != nil {
		verification.Status = "failed"
		return verification, subscriptionsSetupStepResult{Name: subscriptionsSetupStepVerifyState, Status: "failed", Message: err.Error()}, fmt.Errorf("fetch created subscription: %w", err)
	}
	if strings.TrimSpace(subResp.Data.ID) == "" {
		verification.Status = "failed"
		return verification, subscriptionsSetupStepResult{Name: subscriptionsSetupStepVerifyState, Status: "failed", Message: "created subscription readback returned empty id"}, fmt.Errorf("created subscription readback returned empty id")
	}
	verification.SubscriptionState = strings.TrimSpace(subResp.Data.Attributes.State)
	if subResp.Data.Attributes.Name != opts.ReferenceName {
		verification.Status = "failed"
		return verification, subscriptionsSetupStepResult{Name: subscriptionsSetupStepVerifyState, Status: "failed", Message: fmt.Sprintf("reference name mismatch: got %q", subResp.Data.Attributes.Name)}, fmt.Errorf("reference name mismatch: got %q want %q", subResp.Data.Attributes.Name, opts.ReferenceName)
	}
	if subResp.Data.Attributes.ProductID != opts.ProductID {
		verification.Status = "failed"
		return verification, subscriptionsSetupStepResult{Name: subscriptionsSetupStepVerifyState, Status: "failed", Message: fmt.Sprintf("product id mismatch: got %q", subResp.Data.Attributes.ProductID)}, fmt.Errorf("product id mismatch: got %q want %q", subResp.Data.Attributes.ProductID, opts.ProductID)
	}
	if opts.SubscriptionPeriod != "" && subResp.Data.Attributes.SubscriptionPeriod != string(opts.SubscriptionPeriod) {
		verification.Status = "failed"
		return verification, subscriptionsSetupStepResult{Name: subscriptionsSetupStepVerifyState, Status: "failed", Message: fmt.Sprintf("subscription period mismatch: got %q", subResp.Data.Attributes.SubscriptionPeriod)}, fmt.Errorf("subscription period mismatch: got %q want %q", subResp.Data.Attributes.SubscriptionPeriod, opts.SubscriptionPeriod)
	}
	if opts.FamilySharable && !subResp.Data.Attributes.FamilySharable {
		verification.Status = "failed"
		return verification, subscriptionsSetupStepResult{Name: subscriptionsSetupStepVerifyState, Status: "failed", Message: "family-sharable mismatch: expected true"}, fmt.Errorf("family-sharable mismatch: expected true")
	}
	verification.SubscriptionExists = true

	if opts.hasLocalization() {
		locCtx, locCancel := shared.ContextWithTimeout(ctx)
		locResp, err := client.GetSubscriptionLocalizations(locCtx, result.SubscriptionID, asc.WithSubscriptionLocalizationsLimit(200))
		locCancel()
		if err != nil {
			verification.Status = "failed"
			return verification, subscriptionsSetupStepResult{Name: subscriptionsSetupStepVerifyState, Status: "failed", Message: err.Error()}, fmt.Errorf("fetch created localization: %w", err)
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
			return verification, subscriptionsSetupStepResult{Name: subscriptionsSetupStepVerifyState, Status: "failed", Message: "created localization did not match requested locale/name/description"}, fmt.Errorf("created localization did not match requested locale/name/description")
		}
		value := true
		verification.LocalizationExists = &value
	}

	if opts.ReviewScreenshot != "" {
		screenshotCtx, screenshotCancel := shared.ContextWithTimeout(ctx)
		screenshotResp, err := client.GetSubscriptionAppStoreReviewScreenshotForSubscription(
			screenshotCtx,
			result.SubscriptionID,
			asc.WithSubscriptionAppStoreReviewScreenshotFields(subscriptionReviewScreenshotFields),
		)
		screenshotCancel()
		if err != nil {
			verification.Status = "failed"
			return verification, subscriptionsSetupStepResult{Name: subscriptionsSetupStepVerifyState, Status: "failed", Message: err.Error()}, fmt.Errorf("fetch review screenshot: %w", err)
		}
		if strings.TrimSpace(screenshotResp.Data.ID) == "" || strings.TrimSpace(screenshotResp.Data.ID) != result.ReviewScreenshotID {
			verification.Status = "failed"
			return verification, subscriptionsSetupStepResult{Name: subscriptionsSetupStepVerifyState, Status: "failed", Message: "review screenshot readback did not match uploaded screenshot"}, fmt.Errorf("review screenshot readback did not match uploaded screenshot")
		}
		verification.ReviewScreenshotState = subscriptionReviewScreenshotDeliveryState(screenshotResp)
		if verification.ReviewScreenshotState != "COMPLETE" {
			verification.Status = "failed"
			message := fmt.Sprintf("review screenshot delivery is %q, expected COMPLETE", verification.ReviewScreenshotState)
			if verification.ReviewScreenshotState == "FAILED" {
				message = subscriptionReviewScreenshotDeliveryError(screenshotResp).Error()
			}
			return verification, subscriptionsSetupStepResult{Name: subscriptionsSetupStepVerifyState, Status: "failed", Message: message}, fmt.Errorf("%s", message)
		}
		value := true
		verification.ReviewScreenshotVerified = &value
	}

	if opts.hasPricing(opts.StartDate) {
		pricePointCtx, pricePointCancel := shared.ContextWithTimeout(ctx)
		expectedPrice, err := resolveExpectedSubscriptionSetupVerificationPrice(pricePointCtx, client, result.SubscriptionID, opts)
		pricePointCancel()
		if err != nil {
			verification.Status = "failed"
			return verification, subscriptionsSetupStepResult{Name: subscriptionsSetupStepVerifyState, Status: "failed", Message: err.Error()}, err
		}
		summary, err := resolveSubscriptionPriceSummary(ctx, client, subWithGroup{Sub: subResp.Data}, opts.PriceTerritory)
		if err != nil {
			verification.Status = "failed"
			return verification, subscriptionsSetupStepResult{Name: subscriptionsSetupStepVerifyState, Status: "failed", Message: err.Error()}, fmt.Errorf("resolve current pricing: %w", err)
		}
		verification.PriceTerritory = opts.PriceTerritory
		verification.CurrentPrice = summary.CurrentPrice
		if summary.CurrentPrice == nil {
			verification.Status = "failed"
			return verification, subscriptionsSetupStepResult{Name: subscriptionsSetupStepVerifyState, Status: "failed", Message: "current price missing after setup"}, fmt.Errorf("current price missing after setup")
		}
		if expectedPrice != "" {
			priceFilter := shared.PriceFilter{Price: expectedPrice}
			if !priceFilter.MatchesPrice(summary.CurrentPrice.Amount) {
				verification.Status = "failed"
				return verification, subscriptionsSetupStepResult{Name: subscriptionsSetupStepVerifyState, Status: "failed", Message: fmt.Sprintf("current price mismatch: got %q", summary.CurrentPrice.Amount)}, fmt.Errorf("current price mismatch: got %q want %q", summary.CurrentPrice.Amount, expectedPrice)
			}
		}
		value := true
		verification.PriceVerified = &value
	}

	if len(availabilityTerritories) > 0 || reusedSubscription {
		availabilityCtx, availabilityCancel := shared.ContextWithTimeout(ctx)
		availabilityResp, err := client.GetSubscriptionAvailabilityForSubscription(availabilityCtx, result.SubscriptionID)
		availabilityCancel()
		if err != nil {
			if asc.IsNotFound(err) && len(availabilityTerritories) == 0 {
				availabilityResp = nil
			} else {
				verification.Status = "failed"
				return verification, subscriptionsSetupStepResult{Name: subscriptionsSetupStepVerifyState, Status: "failed", Message: err.Error()}, fmt.Errorf("fetch created availability: %w", err)
			}
		}
		if availabilityResp != nil {
			resultID := strings.TrimSpace(availabilityResp.Data.ID)
			if resultID == "" {
				verification.Status = "failed"
				return verification, subscriptionsSetupStepResult{Name: subscriptionsSetupStepVerifyState, Status: "failed", Message: "created availability readback returned empty id"}, fmt.Errorf("created availability readback returned empty id")
			}
			if len(availabilityTerritories) > 0 && availabilityResp.Data.Attributes.AvailableInNewTerritories != opts.AvailableInNewTerritories {
				verification.Status = "failed"
				return verification, subscriptionsSetupStepResult{Name: subscriptionsSetupStepVerifyState, Status: "failed", Message: "available-in-new-territories mismatch"}, fmt.Errorf("available-in-new-territories mismatch")
			}
			territoriesCtx, territoriesCancel := shared.ContextWithTimeout(ctx)
			actualSet, actualTerritories, err := fetchSubscriptionSetupAvailabilityTerritories(territoriesCtx, client, resultID)
			territoriesCancel()
			if err != nil {
				verification.Status = "failed"
				return verification, subscriptionsSetupStepResult{Name: subscriptionsSetupStepVerifyState, Status: "failed", Message: err.Error()}, fmt.Errorf("fetch availability territories: %w", err)
			}
			for _, expected := range availabilityTerritories {
				if _, ok := actualSet[expected]; !ok {
					verification.Status = "failed"
					return verification, subscriptionsSetupStepResult{Name: subscriptionsSetupStepVerifyState, Status: "failed", Message: fmt.Sprintf("missing availability territory %q", expected)}, fmt.Errorf("missing availability territory %q", expected)
				}
			}
			value := true
			verification.AvailabilityVerified = &value
			sort.Strings(actualTerritories)
			verification.Territories = actualTerritories
		}
	}

	if opts.hasPricing(opts.StartDate) {
		equalizations, err := fetchEqualizations(ctx, client, result.ResolvedPricePointID, opts.PriceTerritory)
		if err != nil {
			verification.Status = "failed"
			return verification, subscriptionsSetupStepResult{Name: subscriptionsSetupStepVerifyState, Status: "failed", Message: err.Error()}, fmt.Errorf("fetch expected price matrix: %w", err)
		}
		priceAttrs := asc.SubscriptionPriceCreateAttributes{StartDate: opts.StartDate, PlanType: asc.SubscriptionPlanTypeUpfront}
		expectedMatrix, err := buildSubscriptionSetupPriceMatrix(result.ResolvedPricePointID, opts.PriceTerritory, priceAttrs, equalizations)
		if err != nil {
			verification.Status = "failed"
			return verification, subscriptionsSetupStepResult{Name: subscriptionsSetupStepVerifyState, Status: "failed", Message: err.Error()}, err
		}
		expectedTerritories := make([]string, 0, len(expectedMatrix))
		for _, target := range expectedMatrix {
			expectedTerritories = append(expectedTerritories, target.TerritoryID)
		}
		priceCtx, priceCancel := shared.ContextWithTimeout(ctx)
		priceState, err := fetchSubscriptionPriceImportState(priceCtx, client, result.SubscriptionID)
		priceCancel()
		if err != nil {
			verification.Status = "failed"
			return verification, subscriptionsSetupStepResult{Name: subscriptionsSetupStepVerifyState, Status: "failed", Message: err.Error()}, fmt.Errorf("fetch subscription price coverage: %w", err)
		}
		verification.PricedTerritories, verification.MissingPriceTerritories = subscriptionSetupPriceCoverage(priceState.states, expectedTerritories)
		covered := len(verification.MissingPriceTerritories) == 0
		if covered {
			for _, target := range expectedMatrix {
				if !priceState.matches(subscriptionPriceImportResolvedRow{
					territoryID:  target.TerritoryID,
					pricePointID: target.PricePointID,
					startDate:    strings.TrimSpace(target.Attributes.StartDate),
					planType:     target.Attributes.PlanType,
				}) {
					covered = false
					verification.MissingPriceTerritories = append(verification.MissingPriceTerritories, target.TerritoryID)
				}
			}
		}
		verification.PriceCoverageVerified = &covered
		if !covered {
			verification.Status = "failed"
			sort.Strings(verification.MissingPriceTerritories)
			message := fmt.Sprintf("incomplete or mismatched App Store price matrix for territories: %s", strings.Join(verification.MissingPriceTerritories, ","))
			return verification, subscriptionsSetupStepResult{Name: subscriptionsSetupStepVerifyState, Status: "failed", Message: message}, fmt.Errorf("%s", message)
		}
	} else if len(verification.Territories) > 0 {
		priceCtx, priceCancel := shared.ContextWithTimeout(ctx)
		priceState, err := fetchSubscriptionPriceImportState(priceCtx, client, result.SubscriptionID)
		priceCancel()
		if err != nil {
			verification.Status = "failed"
			return verification, subscriptionsSetupStepResult{Name: subscriptionsSetupStepVerifyState, Status: "failed", Message: err.Error()}, fmt.Errorf("fetch subscription price coverage: %w", err)
		}
		verification.PricedTerritories, verification.MissingPriceTerritories = subscriptionSetupPriceCoverage(priceState.states, verification.Territories)
		covered := len(verification.MissingPriceTerritories) == 0
		verification.PriceCoverageVerified = &covered
		if !covered {
			verification.Status = "failed"
			message := fmt.Sprintf("missing price coverage for enabled availability territories: %s", strings.Join(verification.MissingPriceTerritories, ","))
			return verification, subscriptionsSetupStepResult{Name: subscriptionsSetupStepVerifyState, Status: "failed", Message: message}, fmt.Errorf("%s", message)
		}
	}

	if !subscriptionSetupStateIsComplete(verification.SubscriptionState) {
		verification.Status = "failed"
		state := strings.ToUpper(strings.TrimSpace(verification.SubscriptionState))
		if state == "" {
			state = "UNKNOWN"
		}
		message := fmt.Sprintf("Apple reports subscription state %s after setup", state)
		return verification, subscriptionsSetupStepResult{Name: subscriptionsSetupStepVerifyState, Status: "failed", Message: message}, fmt.Errorf("apple reports subscription state %s after setup", state)
	}

	return verification, subscriptionsSetupStepResult{Name: subscriptionsSetupStepVerifyState, Status: "completed"}, nil
}

func subscriptionSetupStateIsComplete(state string) bool {
	switch strings.ToUpper(strings.TrimSpace(state)) {
	case "READY_TO_SUBMIT", "WAITING_FOR_REVIEW", "IN_REVIEW", "PENDING_BINARY_APPROVAL", "APPROVED":
		return true
	default:
		return false
	}
}

func subscriptionSetupPriceCoverage(states []subscriptionPriceImportState, availabilityTerritories []string) ([]string, []string) {
	pricedSet := make(map[string]struct{})
	for _, state := range states {
		territory := strings.ToUpper(strings.TrimSpace(state.territoryID))
		if territory != "" {
			pricedSet[territory] = struct{}{}
		}
	}
	priced := make([]string, 0, len(pricedSet))
	for territory := range pricedSet {
		priced = append(priced, territory)
	}
	sort.Strings(priced)
	missing := make([]string, 0)
	for _, rawTerritory := range availabilityTerritories {
		territory := strings.ToUpper(strings.TrimSpace(rawTerritory))
		if _, ok := pricedSet[territory]; !ok {
			missing = append(missing, territory)
		}
	}
	sort.Strings(missing)
	return priced, missing
}

func subscriptionsSetupDiagnostics(ctx context.Context, appID, subscriptionID string) ([]validation.SubscriptionDiagnostics, error) {
	report, err := validatecmd.BuildSubscriptionsReport(ctx, validatecmd.SubscriptionsOptions{AppID: appID})
	if err != nil {
		return nil, err
	}
	for _, diagnostic := range report.Diagnostics {
		if strings.TrimSpace(diagnostic.SubscriptionID) == strings.TrimSpace(subscriptionID) {
			return []validation.SubscriptionDiagnostics{diagnostic}, nil
		}
	}
	return nil, fmt.Errorf("deep diagnostics did not include subscription %q; App Store Connect may still be propagating the new resource", subscriptionID)
}

func failSubscriptionsSetupStep(result subscriptionsSetupResult, step string, err error, message string) (subscriptionsSetupResult, error) {
	result.Status = "error"
	result.Error = err.Error()
	result.FailedStep = step
	result.Steps = append(result.Steps, subscriptionsSetupStepResult{
		Name:    step,
		Status:  "failed",
		Message: err.Error(),
	})
	return result, fmt.Errorf("subscriptions setup: %s: %w", message, err)
}

func findExistingSubscriptionSetupGroupLocalization(ctx context.Context, client *asc.Client, groupID, locale string) (asc.Resource[asc.SubscriptionGroupLocalizationAttributes], bool, error) {
	locale = strings.TrimSpace(locale)
	if locale == "" {
		return asc.Resource[asc.SubscriptionGroupLocalizationAttributes]{}, false, nil
	}
	firstPage, err := client.GetSubscriptionGroupLocalizations(ctx, groupID, asc.WithSubscriptionGroupLocalizationsLimit(200))
	if err != nil {
		return asc.Resource[asc.SubscriptionGroupLocalizationAttributes]{}, false, err
	}
	var found asc.Resource[asc.SubscriptionGroupLocalizationAttributes]
	if err := asc.PaginateEach(
		ctx,
		firstPage,
		func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
			return client.GetSubscriptionGroupLocalizations(ctx, groupID, asc.WithSubscriptionGroupLocalizationsNextURL(nextURL))
		},
		func(page asc.PaginatedResponse) error {
			resp, ok := page.(*asc.SubscriptionGroupLocalizationsResponse)
			if !ok {
				return fmt.Errorf("unexpected subscription group localizations pagination type %T", page)
			}
			for _, localization := range resp.Data {
				if strings.TrimSpace(localization.Attributes.Locale) == locale {
					found = localization
					return errSubscriptionsSetupExistingResourceFound
				}
			}
			return nil
		},
	); err != nil && !errors.Is(err, errSubscriptionsSetupExistingResourceFound) {
		return asc.Resource[asc.SubscriptionGroupLocalizationAttributes]{}, false, err
	}
	return found, strings.TrimSpace(found.ID) != "", nil
}

func validateExistingSubscriptionSetupGroupLocalization(localization asc.Resource[asc.SubscriptionGroupLocalizationAttributes], opts subscriptionsSetupOptions) error {
	if strings.TrimSpace(localization.Attributes.Name) != opts.GroupDisplayName {
		return fmt.Errorf("existing group localization %q has a different name", localization.ID)
	}
	if opts.GroupCustomAppName != "" && strings.TrimSpace(localization.Attributes.CustomAppName) != opts.GroupCustomAppName {
		return fmt.Errorf("existing group localization %q has a different custom app name", localization.ID)
	}
	return nil
}

func ensureSubscriptionsSetupPriceMatrix(ctx context.Context, client *asc.Client, subscriptionID, basePricePointID, baseTerritory string, attrs asc.SubscriptionPriceCreateAttributes, repair bool) ([]string, bool, error) {
	baseTerritory = strings.ToUpper(strings.TrimSpace(baseTerritory))
	equalizations, err := fetchEqualizations(ctx, client, basePricePointID, baseTerritory)
	if err != nil {
		return nil, false, err
	}
	matrix, err := buildSubscriptionSetupPriceMatrix(basePricePointID, baseTerritory, attrs, equalizations)
	if err != nil {
		return nil, false, err
	}
	territories := make([]string, 0, len(matrix))
	for _, target := range matrix {
		territories = append(territories, target.TerritoryID)
	}

	state, err := fetchSubscriptionPriceImportState(ctx, client, subscriptionID)
	if err != nil {
		return nil, false, err
	}
	complete := true
	for _, target := range matrix {
		resolved := subscriptionPriceImportResolvedRow{
			territoryID:  target.TerritoryID,
			pricePointID: target.PricePointID,
			startDate:    strings.TrimSpace(attrs.StartDate),
			planType:     attrs.PlanType,
		}
		if !state.matches(resolved) {
			complete = false
			break
		}
	}
	if complete && !repair {
		return territories, false, nil
	}
	priceCtx, priceCancel := shared.ContextWithTimeout(ctx)
	_, err = client.SetSubscriptionPriceMatrix(priceCtx, subscriptionID, matrix)
	priceCancel()
	if err != nil {
		return nil, false, err
	}
	return territories, true, nil
}

func buildSubscriptionSetupPriceMatrix(basePricePointID, baseTerritory string, attrs asc.SubscriptionPriceCreateAttributes, equalizations []equalization) ([]asc.SubscriptionInlinePrice, error) {
	basePricePointID = strings.TrimSpace(basePricePointID)
	baseTerritory = strings.ToUpper(strings.TrimSpace(baseTerritory))
	if basePricePointID == "" || baseTerritory == "" {
		return nil, fmt.Errorf("base price point and territory are required")
	}
	byTerritory := map[string]string{baseTerritory: basePricePointID}
	for _, target := range equalizations {
		territory := strings.ToUpper(strings.TrimSpace(target.Territory))
		pricePointID := strings.TrimSpace(target.PricePointID)
		if territory == "" || pricePointID == "" {
			return nil, fmt.Errorf("equalized territory and price point are required")
		}
		if existing, ok := byTerritory[territory]; ok && existing != pricePointID {
			return nil, fmt.Errorf("conflicting equalized price points for territory %s", territory)
		}
		byTerritory[territory] = pricePointID
	}
	territories := make([]string, 0, len(byTerritory))
	for territory := range byTerritory {
		territories = append(territories, territory)
	}
	sort.Strings(territories)
	matrix := make([]asc.SubscriptionInlinePrice, 0, len(territories))
	for _, territory := range territories {
		matrix = append(matrix, asc.SubscriptionInlinePrice{
			TerritoryID:  territory,
			PricePointID: byTerritory[territory],
			Attributes:   attrs,
		})
	}
	return matrix, nil
}

func subscriptionSetupHasPricedTerritory(states []subscriptionPriceImportState, territory string) bool {
	territory = strings.ToUpper(strings.TrimSpace(territory))
	for _, state := range states {
		if strings.ToUpper(strings.TrimSpace(state.territoryID)) == territory {
			return true
		}
	}
	return false
}

func findExistingSubscriptionSetupGroup(ctx context.Context, client *asc.Client, appID, referenceName string) (string, bool, error) {
	referenceName = strings.TrimSpace(referenceName)
	if referenceName == "" {
		return "", false, nil
	}
	firstPage, err := client.GetSubscriptionGroups(ctx, appID, asc.WithSubscriptionGroupsLimit(200))
	if err != nil {
		return "", false, err
	}
	if firstPage == nil {
		return "", false, nil
	}

	var foundIDs []string
	if err := asc.PaginateEach(
		ctx,
		firstPage,
		func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
			return client.GetSubscriptionGroups(ctx, appID, asc.WithSubscriptionGroupsNextURL(nextURL))
		},
		func(page asc.PaginatedResponse) error {
			resp, ok := page.(*asc.SubscriptionGroupsResponse)
			if !ok {
				return fmt.Errorf("unexpected subscription groups pagination type %T", page)
			}
			for _, group := range resp.Data {
				if strings.TrimSpace(group.Attributes.ReferenceName) != referenceName {
					continue
				}
				foundID := strings.TrimSpace(group.ID)
				if foundID != "" {
					foundIDs = append(foundIDs, foundID)
				}
			}
			return nil
		},
	); err != nil {
		return "", false, err
	}
	if len(foundIDs) > 1 {
		return "", false, fmt.Errorf("multiple subscription groups match reference name %q; pass --group-id to choose one", referenceName)
	}
	var foundID string
	if len(foundIDs) == 1 {
		foundID = foundIDs[0]
	}
	return foundID, foundID != "", nil
}

func findExistingSubscriptionSetupSubscription(ctx context.Context, client *asc.Client, groupID, productID string) (asc.Resource[asc.SubscriptionAttributes], bool, error) {
	productID = strings.TrimSpace(productID)
	if productID == "" {
		return asc.Resource[asc.SubscriptionAttributes]{}, false, nil
	}
	firstPage, err := client.GetSubscriptions(ctx, groupID, asc.WithSubscriptionsLimit(200), asc.WithSubscriptionsProductIDs([]string{productID}))
	if err != nil {
		return asc.Resource[asc.SubscriptionAttributes]{}, false, err
	}
	if firstPage == nil {
		return asc.Resource[asc.SubscriptionAttributes]{}, false, nil
	}

	var foundSubscription asc.Resource[asc.SubscriptionAttributes]
	if err := asc.PaginateEach(
		ctx,
		firstPage,
		func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
			return client.GetSubscriptions(ctx, groupID, asc.WithSubscriptionsNextURL(nextURL))
		},
		func(page asc.PaginatedResponse) error {
			resp, ok := page.(*asc.SubscriptionsResponse)
			if !ok {
				return fmt.Errorf("unexpected subscriptions pagination type %T", page)
			}
			for _, subscription := range resp.Data {
				if strings.TrimSpace(subscription.Attributes.ProductID) != productID {
					continue
				}
				foundSubscription = subscription
				return errSubscriptionsSetupExistingResourceFound
			}
			return nil
		},
	); err != nil && !errors.Is(err, errSubscriptionsSetupExistingResourceFound) {
		return asc.Resource[asc.SubscriptionAttributes]{}, false, err
	}
	return foundSubscription, strings.TrimSpace(foundSubscription.ID) != "", nil
}

func findExistingSubscriptionSetupLocalization(ctx context.Context, client *asc.Client, subscriptionID, locale string) (asc.Resource[asc.SubscriptionLocalizationAttributes], bool, error) {
	locale = strings.TrimSpace(locale)
	if locale == "" {
		return asc.Resource[asc.SubscriptionLocalizationAttributes]{}, false, nil
	}
	firstPage, err := client.GetSubscriptionLocalizations(ctx, subscriptionID, asc.WithSubscriptionLocalizationsLimit(200))
	if err != nil {
		return asc.Resource[asc.SubscriptionLocalizationAttributes]{}, false, err
	}
	if firstPage == nil {
		return asc.Resource[asc.SubscriptionLocalizationAttributes]{}, false, nil
	}

	var foundLocalization asc.Resource[asc.SubscriptionLocalizationAttributes]
	if err := asc.PaginateEach(
		ctx,
		firstPage,
		func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
			return client.GetSubscriptionLocalizations(ctx, subscriptionID, asc.WithSubscriptionLocalizationsNextURL(nextURL))
		},
		func(page asc.PaginatedResponse) error {
			resp, ok := page.(*asc.SubscriptionLocalizationsResponse)
			if !ok {
				return fmt.Errorf("unexpected subscription localizations pagination type %T", page)
			}
			for _, localization := range resp.Data {
				if strings.TrimSpace(localization.Attributes.Locale) != locale {
					continue
				}
				foundLocalization = localization
				return errSubscriptionsSetupExistingResourceFound
			}
			return nil
		},
	); err != nil && !errors.Is(err, errSubscriptionsSetupExistingResourceFound) {
		return asc.Resource[asc.SubscriptionLocalizationAttributes]{}, false, err
	}
	return foundLocalization, strings.TrimSpace(foundLocalization.ID) != "", nil
}

func findExistingSubscriptionSetupPrice(ctx context.Context, client *asc.Client, subID, pricePointID, territoryID string, attrs asc.SubscriptionPriceCreateAttributes) (bool, bool, error) {
	pricePointID = strings.TrimSpace(pricePointID)
	territoryID = strings.ToUpper(strings.TrimSpace(territoryID))
	if pricePointID == "" {
		return false, false, nil
	}

	opts := []asc.SubscriptionPricesOption{
		asc.WithSubscriptionPricesLimit(200),
		asc.WithSubscriptionPricesInclude([]string{"subscriptionPricePoint", "territory"}),
	}
	firstPage, err := client.GetSubscriptionPrices(ctx, subID, opts...)
	if err != nil {
		return false, false, err
	}
	if firstPage == nil {
		return false, false, nil
	}

	var foundPrice asc.Resource[asc.SubscriptionPriceAttributes]
	hasExistingPrices := false
	if err := asc.PaginateEach(
		ctx,
		firstPage,
		func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
			return client.GetSubscriptionPrices(ctx, subID, asc.WithSubscriptionPricesNextURL(nextURL))
		},
		func(page asc.PaginatedResponse) error {
			resp, ok := page.(*asc.SubscriptionPricesResponse)
			if !ok {
				return fmt.Errorf("unexpected subscription prices pagination type %T", page)
			}
			for _, price := range resp.Data {
				hasExistingPrices = true
				if subscriptionSetupPriceMatchesTarget(price, pricePointID, territoryID, attrs) {
					foundPrice = price
					return errSubscriptionsSetupExistingResourceFound
				}
			}
			return nil
		},
	); err != nil && !errors.Is(err, errSubscriptionsSetupExistingResourceFound) {
		return false, false, err
	}
	return strings.TrimSpace(foundPrice.ID) != "", hasExistingPrices, nil
}

func mismatchedExistingSubscriptionSetupPriceError(subscriptionID string) error {
	return fmt.Errorf("existing subscription %q already has prices but none match the requested price point, territory, start date, and upfront plan type; use subscriptions prices add to add the requested price", subscriptionID)
}

func subscriptionSetupPriceMatchesTarget(price asc.Resource[asc.SubscriptionPriceAttributes], pricePointID, territoryID string, attrs asc.SubscriptionPriceCreateAttributes) bool {
	var relationships subscriptionSetupPriceRelationships
	if len(price.Relationships) > 0 {
		if err := json.Unmarshal(price.Relationships, &relationships); err != nil {
			return false
		}
	}
	if relationships.SubscriptionPricePoint == nil || relationships.SubscriptionPricePoint.Data.ID != pricePointID {
		return false
	}

	actualTerritory := ""
	if relationships.Territory != nil {
		actualTerritory = strings.ToUpper(strings.TrimSpace(relationships.Territory.Data.ID))
	}
	if strings.ToUpper(strings.TrimSpace(territoryID)) != actualTerritory {
		return false
	}
	if strings.TrimSpace(price.Attributes.StartDate) != strings.TrimSpace(attrs.StartDate) {
		return false
	}
	if attrs.PlanType != "" && price.Attributes.PlanType != attrs.PlanType {
		return false
	}
	return true
}

type subscriptionSetupPriceRelationships struct {
	SubscriptionPricePoint *asc.Relationship `json:"subscriptionPricePoint"`
	Territory              *asc.Relationship `json:"territory"`
}

func findExistingSubscriptionSetupAvailability(ctx context.Context, client *asc.Client, subID string, territories []string, attrs asc.SubscriptionAvailabilityAttributes) (string, bool, bool, error) {
	resp, err := client.GetSubscriptionAvailabilityForSubscription(ctx, subID)
	if err != nil {
		if errors.Is(err, asc.ErrNotFound) {
			return "", false, false, nil
		}
		return "", false, false, err
	}
	if resp == nil || strings.TrimSpace(resp.Data.ID) == "" {
		return "", false, false, nil
	}
	availabilityID := strings.TrimSpace(resp.Data.ID)
	if resp.Data.Attributes.AvailableInNewTerritories != attrs.AvailableInNewTerritories {
		return availabilityID, false, true, nil
	}

	actual, _, err := fetchSubscriptionSetupAvailabilityTerritories(ctx, client, availabilityID)
	if err != nil {
		return "", false, true, err
	}
	for _, territory := range territories {
		if _, ok := actual[strings.ToUpper(strings.TrimSpace(territory))]; !ok {
			return availabilityID, false, true, nil
		}
	}
	return availabilityID, true, true, nil
}

func mismatchedExistingSubscriptionSetupAvailabilityError(subscriptionID string) error {
	return fmt.Errorf("existing subscription %q already has availability but it does not match the requested territories and available-in-new-territories setting; update availability or choose a different product ID", subscriptionID)
}

func fetchSubscriptionSetupAvailabilityTerritories(ctx context.Context, client *asc.Client, availabilityID string) (map[string]struct{}, []string, error) {
	firstPage, err := client.GetSubscriptionAvailabilityAvailableTerritories(ctx, availabilityID, asc.WithSubscriptionAvailabilityTerritoriesLimit(200))
	if err != nil {
		return nil, nil, err
	}
	actualSet := map[string]struct{}{}
	actualTerritories := []string{}
	if firstPage == nil {
		return actualSet, actualTerritories, nil
	}

	err = asc.PaginateEach(
		ctx,
		firstPage,
		func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
			return client.GetSubscriptionAvailabilityAvailableTerritories(ctx, availabilityID, asc.WithSubscriptionAvailabilityTerritoriesNextURL(nextURL))
		},
		func(page asc.PaginatedResponse) error {
			resp, ok := page.(*asc.TerritoriesResponse)
			if !ok {
				return fmt.Errorf("unexpected subscription availability territories pagination type %T", page)
			}
			for _, item := range resp.Data {
				id := strings.ToUpper(strings.TrimSpace(item.ID))
				if id == "" {
					continue
				}
				if _, seen := actualSet[id]; !seen {
					actualTerritories = append(actualTerritories, id)
				}
				actualSet[id] = struct{}{}
			}
			return nil
		},
	)
	if err != nil {
		return nil, nil, err
	}
	return actualSet, actualTerritories, nil
}

func validateExistingSubscriptionSetupSubscription(subscription asc.Resource[asc.SubscriptionAttributes], target asc.SubscriptionCreateAttributes, expectedFamilySharable bool) error {
	if strings.TrimSpace(subscription.Attributes.Name) != strings.TrimSpace(target.Name) {
		return fmt.Errorf("existing subscription %q has a different reference name; update it or choose a different product ID", strings.TrimSpace(subscription.ID))
	}
	if target.SubscriptionPeriod != "" && strings.TrimSpace(subscription.Attributes.SubscriptionPeriod) != strings.TrimSpace(target.SubscriptionPeriod) {
		return fmt.Errorf("existing subscription %q has a different subscription period; update it or choose a different product ID", strings.TrimSpace(subscription.ID))
	}
	if subscription.Attributes.FamilySharable != expectedFamilySharable {
		return fmt.Errorf("existing subscription %q has a different family sharing setting; update it or choose a different product ID", strings.TrimSpace(subscription.ID))
	}
	return nil
}

func validateExistingSubscriptionSetupLocalization(localization asc.Resource[asc.SubscriptionLocalizationAttributes], opts subscriptionsSetupOptions) error {
	if strings.TrimSpace(localization.Attributes.Locale) != strings.TrimSpace(opts.Locale) {
		return fmt.Errorf("existing subscription localization %q has a different locale; update it or choose a different locale", strings.TrimSpace(localization.ID))
	}
	if strings.TrimSpace(opts.DisplayName) != "" && strings.TrimSpace(localization.Attributes.Name) != strings.TrimSpace(opts.DisplayName) {
		return fmt.Errorf("existing subscription localization %q has a different display name; update it or choose a different locale", strings.TrimSpace(localization.ID))
	}
	if strings.TrimSpace(localization.Attributes.Description) != strings.TrimSpace(opts.Description) {
		return fmt.Errorf("existing subscription localization %q has a different description; update it or choose a different locale", strings.TrimSpace(localization.ID))
	}
	return nil
}

func subscriptionsSetupAvailabilityTerritories(opts subscriptionsSetupOptions) []string {
	if len(opts.Territories) > 0 {
		return opts.Territories
	}
	if opts.hasPricing(opts.StartDate) && opts.PriceTerritory != "" {
		return []string{opts.PriceTerritory}
	}
	return nil
}

func subscriptionsSetupAvailabilityMessage(opts subscriptionsSetupOptions, territories []string) string {
	if len(opts.Territories) > 0 {
		return ""
	}
	if len(territories) == 1 && opts.PriceTerritory != "" {
		return fmt.Sprintf("auto-enabled pricing territory %q", territories[0])
	}
	return ""
}

func resolveExpectedSubscriptionSetupPricePoint(ctx context.Context, client *asc.Client, subID string, opts subscriptionsSetupOptions) (string, error) {
	if opts.PricePointID != "" {
		return opts.PricePointID, nil
	}

	tiers, err := shared.ResolveSubscriptionTiers(ctx, client, subID, opts.PriceTerritory, opts.RefreshTierCache)
	if err != nil {
		return "", fmt.Errorf("subscriptions setup: resolve price point: %w", err)
	}
	if opts.Tier > 0 {
		id, err := shared.ResolvePricePointByTier(tiers, opts.Tier)
		if err != nil {
			return "", fmt.Errorf("subscriptions setup: %w", err)
		}
		return id, nil
	}
	id, err := shared.ResolvePricePointByPrice(tiers, opts.Price)
	if err != nil {
		return "", fmt.Errorf("subscriptions setup: %w", err)
	}
	return id, nil
}

func resolveExpectedSubscriptionSetupVerificationPrice(ctx context.Context, client *asc.Client, subID string, opts subscriptionsSetupOptions) (string, error) {
	if opts.Price != "" {
		return opts.Price, nil
	}
	if opts.Tier == 0 && opts.PricePointID == "" {
		return "", nil
	}
	tiers, err := shared.ResolveSubscriptionTiers(ctx, client, subID, opts.PriceTerritory, true)
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
	return "", fmt.Errorf("price point %q not found in %s during verification", opts.PricePointID, opts.PriceTerritory)
}

func printSubscriptionsSetupResult(result *subscriptionsSetupResult, format string, pretty bool) error {
	headers, rows := subscriptionsSetupResultRows(result)
	diagnosticHeaders, diagnosticRows := subscriptionsSetupDiagnosticRows(result.Diagnostics)
	return shared.PrintOutputWithRenderers(
		result,
		format,
		pretty,
		func() error {
			asc.RenderTable(headers, rows)
			if len(diagnosticRows) > 0 {
				asc.RenderTable(diagnosticHeaders, diagnosticRows)
			}
			return nil
		},
		func() error {
			asc.RenderMarkdown(headers, rows)
			if len(diagnosticRows) > 0 {
				asc.RenderMarkdown(diagnosticHeaders, diagnosticRows)
			}
			return nil
		},
	)
}

func subscriptionsSetupDiagnosticRows(diagnostics []validation.SubscriptionDiagnostics) ([]string, [][]string) {
	headers := []string{"Subscription ID", "Conclusion", "Diagnostic", "Status", "Blocking", "Evidence", "Remediation"}
	rows := make([][]string, 0)
	for _, diagnostic := range diagnostics {
		for _, row := range diagnostic.Rows {
			rows = append(rows, []string{
				diagnostic.SubscriptionID,
				diagnostic.Conclusion,
				row.Label,
				string(row.Status),
				fmt.Sprintf("%t", row.Blocking),
				row.Evidence,
				row.Remediation,
			})
		}
	}
	return headers, rows
}

func subscriptionsSetupResultRows(result *subscriptionsSetupResult) ([]string, [][]string) {
	headers := []string{"Status", "Verification", "Apple State", "Group ID", "Group Localization ID", "Subscription ID", "Localization ID", "Review Screenshot ID", "Availability ID", "Price Point ID", "Price Coverage", "Current Price", "Failed Step", "Error"}
	rows := [][]string{{
		result.Status,
		subscriptionsSetupVerificationStatus(result.Verification),
		subscriptionsSetupVerificationState(result.Verification),
		result.GroupID,
		result.GroupLocalizationID,
		result.SubscriptionID,
		result.LocalizationID,
		result.ReviewScreenshotID,
		result.AvailabilityID,
		result.ResolvedPricePointID,
		subscriptionsSetupVerificationPriceCoverage(result.Verification),
		subscriptionsSetupVerificationCurrentPrice(result.Verification),
		result.FailedStep,
		result.Error,
	}}
	return headers, rows
}

func subscriptionsSetupVerificationState(verification *subscriptionsSetupVerification) string {
	if verification == nil {
		return ""
	}
	return verification.SubscriptionState
}

func subscriptionsSetupVerificationPriceCoverage(verification *subscriptionsSetupVerification) string {
	if verification == nil || verification.PriceCoverageVerified == nil {
		return ""
	}
	if *verification.PriceCoverageVerified {
		return "verified"
	}
	return "missing: " + strings.Join(verification.MissingPriceTerritories, ",")
}

func resolveSubscriptionsSetupAlias(primary, alias, primaryName, aliasName string) (string, error) {
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

func subscriptionsSetupVerificationStatus(verification *subscriptionsSetupVerification) string {
	if verification == nil {
		return ""
	}
	return verification.Status
}

func subscriptionsSetupVerificationCurrentPrice(verification *subscriptionsSetupVerification) string {
	if verification == nil {
		return ""
	}
	if verification.CurrentPrice != nil {
		return formatSubMoney(verification.CurrentPrice)
	}
	if verification.ScheduledPrice != nil {
		return strings.TrimSpace(formatSubMoney(verification.ScheduledPrice) + " (effective " + verification.ScheduledStartDate + ")")
	}
	return ""
}
