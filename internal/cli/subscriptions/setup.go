package subscriptions

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
	subscriptionsSetupStepEnsureGroup        = "ensure_group"
	subscriptionsSetupStepCreateSubscription = "create_subscription"
	subscriptionsSetupStepCreateLocalization = "create_localization"
	subscriptionsSetupStepResolvePricePoint  = "resolve_price_point"
	subscriptionsSetupStepSetPrice           = "set_price"
	subscriptionsSetupStepSetAvailability    = "set_availability"
	subscriptionsSetupStepVerifyState        = "verify_state"
)

type subscriptionsSetupOptions struct {
	AppID                     string
	GroupID                   string
	GroupReferenceName        string
	ReferenceName             string
	ProductID                 string
	SubscriptionPeriod        asc.SubscriptionPeriod
	FamilySharable            bool
	Locale                    string
	DisplayName               string
	Description               string
	PriceTerritory            string
	PricePointID              string
	Tier                      int
	Price                     string
	StartDate                 string
	RefreshTierCache          bool
	Territories               []string
	AvailableInNewTerritories bool
	NoVerify                  bool
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
	Status               string                          `json:"status"`
	Error                string                          `json:"error,omitempty"`
	AppID                string                          `json:"appId,omitempty"`
	GroupID              string                          `json:"groupId,omitempty"`
	GroupReferenceName   string                          `json:"groupReferenceName,omitempty"`
	SubscriptionID       string                          `json:"subscriptionId,omitempty"`
	ReferenceName        string                          `json:"referenceName,omitempty"`
	ProductID            string                          `json:"productId,omitempty"`
	SubscriptionPeriod   string                          `json:"subscriptionPeriod,omitempty"`
	Locale               string                          `json:"locale,omitempty"`
	PriceTerritory       string                          `json:"priceTerritory,omitempty"`
	LocalizationID       string                          `json:"localizationId,omitempty"`
	AvailabilityID       string                          `json:"availabilityId,omitempty"`
	ResolvedPricePointID string                          `json:"resolvedPricePointId,omitempty"`
	Verification         *subscriptionsSetupVerification `json:"verification,omitempty"`
	FailedStep           string                          `json:"failedStep,omitempty"`
	Steps                []subscriptionsSetupStepResult  `json:"steps"`
}

type subscriptionsSetupVerification struct {
	Status               string    `json:"status"`
	GroupExists          *bool     `json:"groupExists,omitempty"`
	SubscriptionExists   bool      `json:"subscriptionExists,omitempty"`
	LocalizationExists   *bool     `json:"localizationExists,omitempty"`
	PriceVerified        *bool     `json:"priceVerified,omitempty"`
	AvailabilityVerified *bool     `json:"availabilityVerified,omitempty"`
	PriceTerritory       string    `json:"priceTerritory,omitempty"`
	CurrentPrice         *subMoney `json:"currentPrice,omitempty"`
	ScheduledPrice       *subMoney `json:"scheduledPrice,omitempty"`
	ScheduledStartDate   string    `json:"scheduledStartDate,omitempty"`
	Territories          []string  `json:"territories,omitempty"`
}

// SubscriptionsSetupCommand returns the high-level subscriptions bootstrap workflow command.
func SubscriptionsSetupCommand() *ffcli.Command {
	fs := flag.NewFlagSet("setup", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	groupID := fs.String("group-id", "", "Existing subscription group ID")
	groupReferenceName := fs.String("group-reference-name", "", "Reference name for a new subscription group")
	groupRefNameAlias := fs.String("group-ref-name", "", "Reference name alias for a new subscription group")

	referenceName := fs.String("reference-name", "", "Subscription reference name")
	refNameAlias := fs.String("ref-name", "", "Subscription reference name alias")
	productID := fs.String("product-id", "", "Product ID (e.g., com.example.sub.monthly)")
	subscriptionPeriod := fs.String("subscription-period", "", "Subscription period: "+strings.Join(subscriptionPeriodValues, ", "))
	familySharable := fs.Bool("family-sharable", false, "Enable Family Sharing (cannot be undone)")

	locale := fs.String("locale", "", "Locale for the first subscription localization (e.g., en-US)")
	displayName := fs.String("display-name", "", "Display name for the first subscription localization")
	nameAlias := fs.String("name", "", "Display name alias")
	description := fs.String("description", "", "Description for the first subscription localization")

	priceTerritory := fs.String("price-territory", "", "Territory used to resolve and verify the initial subscription price (e.g., USA)")
	pricePointID := fs.String("price-point-id", "", "Explicit price point ID for the initial subscription price")
	tier := fs.Int("tier", 0, "Pricing tier number for the initial subscription price")
	price := fs.String("price", "", "Customer price for the initial subscription price")
	startDate := fs.String("start-date", "", "Start date for the initial subscription price (YYYY-MM-DD)")
	refresh := fs.Bool("refresh", false, "Force refresh of the subscription price-point tier cache when resolving --tier or --price")

	territories := fs.String("territories", "", "Availability territories to enable after creation, comma-separated")
	availableInNewTerritories := fs.Bool("available-in-new-territories", false, "Include new territories automatically when creating availability")
	noVerify := fs.Bool("no-verify", false, "Skip post-create readback verification for faster execution")
	output := shared.BindOutputFlags(fs)

	shared.HideFlagFromHelp(fs.Lookup("group-ref-name"))
	shared.HideFlagFromHelp(fs.Lookup("ref-name"))
	shared.HideFlagFromHelp(fs.Lookup("name"))

	return &ffcli.Command{
		Name:       "setup",
		ShortUsage: "asc subscriptions setup [flags]",
		ShortHelp:  "Create a subscription with optional group, localization, pricing, and availability.",
		LongHelp: `Create a new subscription and optionally bootstrap its group,
first localization, initial pricing, and availability in one workflow.

The setup command is create-oriented: use it when you want a one-shot happy
path for a new subscription. Existing low-level commands remain available
for partial updates, repair flows, and advanced cases.

By default, setup reads the created state back from App Store Connect and
verifies the resulting group, subscription, localization, pricing, and
availability. Use --no-verify to skip that postcondition check when speed
matters more than confirmed final state.

Examples:
  asc subscriptions setup --app "APP_ID" --group-reference-name "Pro" --reference-name "Pro Monthly" --product-id "com.example.pro.monthly" --subscription-period ONE_MONTH
  asc subscriptions setup --app "APP_ID" --group-reference-name "Pro" --reference-name "Pro Monthly" --product-id "com.example.pro.monthly" --subscription-period ONE_MONTH --locale "en-US" --display-name "Pro Monthly" --description "Unlock everything"
  asc subscriptions setup --app "APP_ID" --group-reference-name "Pro" --reference-name "Pro Monthly" --product-id "com.example.pro.monthly" --price "3.99" --price-territory "USA" --territories "USA,CAN"
  asc subscriptions setup --group-id "GROUP_ID" --reference-name "Pro Monthly" --product-id "com.example.pro.monthly" --subscription-period ONE_MONTH --no-verify`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return shared.UsageError("subscriptions setup does not accept positional arguments")
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

			opts := subscriptionsSetupOptions{
				AppID:                     shared.ResolveAppID(*appID),
				GroupID:                   strings.TrimSpace(*groupID),
				GroupReferenceName:        groupReferenceNameValue,
				ReferenceName:             referenceNameValue,
				ProductID:                 strings.TrimSpace(*productID),
				FamilySharable:            *familySharable,
				Locale:                    strings.TrimSpace(*locale),
				DisplayName:               displayNameValue,
				Description:               strings.TrimSpace(*description),
				PriceTerritory:            strings.ToUpper(strings.TrimSpace(*priceTerritory)),
				PricePointID:              strings.TrimSpace(*pricePointID),
				Tier:                      *tier,
				Price:                     strings.TrimSpace(*price),
				Territories:               shared.SplitCSVUpper(*territories),
				AvailableInNewTerritories: *availableInNewTerritories,
				RefreshTierCache:          *refresh,
				NoVerify:                  *noVerify,
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

			if opts.hasLocalization() {
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

			if opts.hasAvailability() && len(opts.Territories) == 0 {
				return shared.UsageError("--territories is required when availability flags are provided")
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

	if strings.TrimSpace(opts.GroupID) == "" {
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
	} else {
		result.Steps = append(result.Steps, subscriptionsSetupStepResult{
			Name:    subscriptionsSetupStepEnsureGroup,
			Status:  "completed",
			ID:      result.GroupID,
			Message: "used existing group",
		})
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

	if !opts.hasLocalization() {
		result.Steps = append(result.Steps, subscriptionsSetupStepResult{
			Name:    subscriptionsSetupStepCreateLocalization,
			Status:  "skipped",
			Message: "no localization flags provided",
		})
	} else {
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

	if !opts.hasPricing(opts.StartDate) {
		result.Steps = append(result.Steps,
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
		result.ResolvedPricePointID = resolvedPricePointID
		result.Steps = append(result.Steps, subscriptionsSetupStepResult{
			Name:   subscriptionsSetupStepResolvePricePoint,
			Status: "completed",
			ID:     result.ResolvedPricePointID,
		})

		priceCtx, priceCancel := shared.ContextWithTimeout(ctx)
		_, err = client.SetSubscriptionInitialPrice(priceCtx, result.SubscriptionID, result.ResolvedPricePointID, opts.PriceTerritory, asc.SubscriptionPriceCreateAttributes{
			StartDate: opts.StartDate,
		})
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
			return result, fmt.Errorf("subscriptions setup: failed to set initial price: %w", err)
		}
		result.Steps = append(result.Steps, subscriptionsSetupStepResult{
			Name:   subscriptionsSetupStepSetPrice,
			Status: "completed",
			ID:     result.SubscriptionID,
		})
	}

	if !opts.hasAvailability() {
		result.Steps = append(result.Steps, subscriptionsSetupStepResult{
			Name:    subscriptionsSetupStepSetAvailability,
			Status:  "skipped",
			Message: "no availability flags provided",
		})
	} else {
		availabilityCtx, availabilityCancel := shared.ContextWithTimeout(ctx)
		availabilityResp, err := client.CreateSubscriptionAvailability(availabilityCtx, result.SubscriptionID, opts.Territories, asc.SubscriptionAvailabilityAttributes{
			AvailableInNewTerritories: opts.AvailableInNewTerritories,
		})
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
			Name:   subscriptionsSetupStepSetAvailability,
			Status: "completed",
			ID:     result.AvailabilityID,
		})
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

	verification, verifyStep, err := verifySubscriptionsSetupState(ctx, client, result, opts)
	if err != nil {
		result.Status = "error"
		result.Error = err.Error()
		result.FailedStep = subscriptionsSetupStepVerifyState
		result.Verification = verification
		result.Steps = append(result.Steps, verifyStep)
		return result, fmt.Errorf("subscriptions setup: verify state: %w", err)
	}
	result.Verification = verification
	result.Steps = append(result.Steps, verifyStep)

	return result, nil
}

func verifySubscriptionsSetupState(ctx context.Context, client *asc.Client, result subscriptionsSetupResult, opts subscriptionsSetupOptions) (*subscriptionsSetupVerification, subscriptionsSetupStepResult, error) {
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

	if opts.hasAvailability() {
		availabilityCtx, availabilityCancel := shared.ContextWithTimeout(ctx)
		availabilityResp, err := client.GetSubscriptionAvailabilityForSubscription(availabilityCtx, result.SubscriptionID)
		availabilityCancel()
		if err != nil {
			verification.Status = "failed"
			return verification, subscriptionsSetupStepResult{Name: subscriptionsSetupStepVerifyState, Status: "failed", Message: err.Error()}, fmt.Errorf("fetch created availability: %w", err)
		}
		resultID := strings.TrimSpace(availabilityResp.Data.ID)
		if resultID == "" {
			verification.Status = "failed"
			return verification, subscriptionsSetupStepResult{Name: subscriptionsSetupStepVerifyState, Status: "failed", Message: "created availability readback returned empty id"}, fmt.Errorf("created availability readback returned empty id")
		}
		if availabilityResp.Data.Attributes.AvailableInNewTerritories != opts.AvailableInNewTerritories {
			verification.Status = "failed"
			return verification, subscriptionsSetupStepResult{Name: subscriptionsSetupStepVerifyState, Status: "failed", Message: "available-in-new-territories mismatch"}, fmt.Errorf("available-in-new-territories mismatch")
		}
		territoriesCtx, territoriesCancel := shared.ContextWithTimeout(ctx)
		territoriesResp, err := client.GetSubscriptionAvailabilityAvailableTerritories(territoriesCtx, resultID, asc.WithSubscriptionAvailabilityTerritoriesLimit(200))
		territoriesCancel()
		if err != nil {
			verification.Status = "failed"
			return verification, subscriptionsSetupStepResult{Name: subscriptionsSetupStepVerifyState, Status: "failed", Message: err.Error()}, fmt.Errorf("fetch availability territories: %w", err)
		}
		actualTerritories := make([]string, 0, len(territoriesResp.Data))
		actualSet := map[string]struct{}{}
		for _, item := range territoriesResp.Data {
			id := strings.ToUpper(strings.TrimSpace(item.ID))
			if id == "" {
				continue
			}
			actualTerritories = append(actualTerritories, id)
			actualSet[id] = struct{}{}
		}
		for _, expected := range opts.Territories {
			if _, ok := actualSet[expected]; !ok {
				verification.Status = "failed"
				return verification, subscriptionsSetupStepResult{Name: subscriptionsSetupStepVerifyState, Status: "failed", Message: fmt.Sprintf("missing availability territory %q", expected)}, fmt.Errorf("missing availability territory %q", expected)
			}
		}
		value := true
		verification.AvailabilityVerified = &value
		verification.Territories = actualTerritories
	}

	return verification, subscriptionsSetupStepResult{Name: subscriptionsSetupStepVerifyState, Status: "completed"}, nil
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
	return shared.PrintOutputWithRenderers(
		result,
		format,
		pretty,
		func() error {
			asc.RenderTable(headers, rows)
			return nil
		},
		func() error {
			asc.RenderMarkdown(headers, rows)
			return nil
		},
	)
}

func subscriptionsSetupResultRows(result *subscriptionsSetupResult) ([]string, [][]string) {
	headers := []string{"Status", "Verification", "Group ID", "Subscription ID", "Localization ID", "Availability ID", "Price Point ID", "Current Price", "Failed Step", "Error"}
	rows := [][]string{{
		result.Status,
		subscriptionsSetupVerificationStatus(result.Verification),
		result.GroupID,
		result.SubscriptionID,
		result.LocalizationID,
		result.AvailabilityID,
		result.ResolvedPricePointID,
		subscriptionsSetupVerificationCurrentPrice(result.Verification),
		result.FailedStep,
		result.Error,
	}}
	return headers, rows
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
