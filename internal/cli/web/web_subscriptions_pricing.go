package web

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"strings"
	"time"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

var (
	createWebSubscriptionPlanAvailabilityFn = func(ctx context.Context, client *webcore.Client, subscriptionID, planType string, territoryIDs []string, availableInNewTerritories bool) (*webcore.SubscriptionPlanAvailability, error) {
		return client.CreateSubscriptionPlanAvailability(ctx, subscriptionID, planType, territoryIDs, availableInNewTerritories)
	}
	createWebSubscriptionPlanPricesFn = func(ctx context.Context, client *webcore.Client, subscriptionID, upfrontPricePointID, monthlyPricePointID string) (*webcore.SubscriptionPlanPricesResult, error) {
		return client.CreateSubscriptionPlanPrices(ctx, subscriptionID, upfrontPricePointID, monthlyPricePointID)
	}
	setWebSubscriptionPlanPricesFn = func(ctx context.Context, client *webcore.Client, subscriptionID string, prices []webcore.SubscriptionPlanPrice) (*webcore.SubscriptionPlanPricesResult, error) {
		return client.SetSubscriptionPlanPrices(ctx, subscriptionID, prices)
	}
	listWebSubscriptionPricesFn = func(ctx context.Context, client *webcore.Client, subscriptionID, territory string) ([]webcore.SubscriptionPrice, error) {
		return client.ListSubscriptionPrices(ctx, subscriptionID, territory)
	}
	resolveWebSubscriptionPricePointFn = func(ctx context.Context, client *webcore.Client, subscriptionID, territory, customerPrice string) (*webcore.SubscriptionPricePoint, error) {
		return client.ResolveSubscriptionPricePoint(ctx, subscriptionID, territory, customerPrice)
	}
	getWebSubscriptionAdjustedEqualizationsFn = func(ctx context.Context, client *webcore.Client, pricePointID, planType string) (*webcore.SubscriptionAdjustedEqualizationsResult, error) {
		return client.GetSubscriptionAdjustedEqualizations(ctx, pricePointID, planType)
	}
)

// newWebSubscriptionsPricingCommand returns the web subscription pricing command group.
func newWebSubscriptionsPricingCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("web subscriptions pricing", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "pricing",
		ShortUsage: "asc web subscriptions pricing <subcommand> [flags]",
		ShortHelp:  "Manage subscription pricing via web sessions.",
		LongHelp: `WEB SESSION WORKFLOWS

Manage subscription pricing through Apple's internal web API.

`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newWebSubscriptionsPricingAdjustedEqualizationsCommand(d),
			newWebSubscriptionsPricingMonthlyCommitmentCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newWebSubscriptionsPricingMonthlyCommitmentCommand returns the monthly commitment group.
func newWebSubscriptionsPricingMonthlyCommitmentCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("web subscriptions pricing monthly-commitment", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "monthly-commitment",
		ShortUsage: "asc web subscriptions pricing monthly-commitment <subcommand> [flags]",
		ShortHelp:  "Bootstrap monthly-with-commitment pricing.",
		LongHelp: `WEB SESSION WORKFLOWS

Bootstrap monthly-with-12-month-commitment pricing through Apple's internal web API.

`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newWebSubscriptionsPricingMonthlyCommitmentBootstrapCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newWebSubscriptionsPricingMonthlyCommitmentBootstrapCommand creates availability and prices.
func newWebSubscriptionsPricingMonthlyCommitmentBootstrapCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("web subscriptions pricing monthly-commitment bootstrap", flag.ExitOnError)

	subscriptionID := fs.String("subscription-id", "", "Subscription ID")
	territory := fs.String("territory", "", "Three-letter territory ID, for example NOR")
	upfrontPricePointID := fs.String("upfront-price-point-id", "", "UPFRONT subscription price point ID")
	monthlyPricePointID := fs.String("monthly-price-point-id", "", "MONTHLY subscription price point ID")
	upfrontPrice := fs.String("upfront-price", "", "UPFRONT customer price to resolve in --territory")
	monthlyPrice := fs.String("monthly-price", "", "MONTHLY customer price to resolve in --territory")
	startDate := fs.String("start-date", "", "Schedule both prices on YYYY-MM-DD")
	preserveCurrentPrice := fs.Bool("preserve-current-price", false, "Preserve current pricing for existing subscribers; requires --start-date")
	dryRun := fs.Bool("dry-run", false, "Resolve and print the plan without creating or changing resources")
	confirm := fs.Bool("confirm", false, "Confirm creating monthly availability and paired prices")
	authFlags := bindWebSessionFlags(fs)
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "bootstrap",
		ShortUsage: "asc web subscriptions pricing monthly-commitment bootstrap --subscription-id SUB_ID --territory NOR (--upfront-price PRICE | --upfront-price-point-id ID) (--monthly-price PRICE | --monthly-price-point-id ID) [--dry-run | --confirm] [flags]",
		ShortHelp:  "Create monthly plan availability and paired prices.",
		LongHelp: `WEB SESSION WORKFLOWS

Create MONTHLY plan availability, then attach paired UPFRONT and MONTHLY prices
using the same inline subscription PATCH as App Store Connect.

Prefer asc subscriptions pricing monthly-commitment enable for normal setup.
Use this command when you specifically need App Store Connect's
paired web pricing workflow or a paired scheduled price change.

Prices may be supplied as exact customer prices or price point IDs. Use
--start-date for a scheduled paired price change. --preserve-current-price
applies only to scheduled changes. --dry-run performs all reads and price
resolution but does not mutate App Store Connect.

`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*subscriptionID)
			territoryID := strings.ToUpper(strings.TrimSpace(*territory))
			upfrontID := strings.TrimSpace(*upfrontPricePointID)
			monthlyID := strings.TrimSpace(*monthlyPricePointID)
			upfrontAmount := strings.TrimSpace(*upfrontPrice)
			monthlyAmount := strings.TrimSpace(*monthlyPrice)
			scheduledDate := strings.TrimSpace(*startDate)
			switch {
			case id == "":
				return d.UsageError("--subscription-id is required")
			case len(territoryID) != 3:
				return d.UsageError("--territory must be a three-letter territory ID")
			case territoryID == "USA":
				return d.UsageError("--territory cannot be USA for monthly-commitment pricing")
			case (upfrontID == "") == (upfrontAmount == ""):
				return d.UsageError("exactly one of --upfront-price or --upfront-price-point-id is required")
			case (monthlyID == "") == (monthlyAmount == ""):
				return d.UsageError("exactly one of --monthly-price or --monthly-price-point-id is required")
			case *preserveCurrentPrice && scheduledDate == "":
				return d.UsageError("--preserve-current-price requires --start-date")
			case !*dryRun && !*confirm:
				return d.RequireConfirmUnlessDryRun(*dryRun, *confirm)
			}
			if scheduledDate != "" {
				if _, err := time.Parse("2006-01-02", scheduledDate); err != nil {
					return d.UsageError("--start-date must use YYYY-MM-DD")
				}
			}

			session, requestCtx, cancel, err := resolveWebSessionForCommandWith(ctx, d, authFlags)
			defer cancel()
			if err != nil {
				return err
			}
			client := newWebClientFn(session)
			if upfrontAmount != "" {
				point, err := resolveWebSubscriptionPricePointFn(requestCtx, client, id, territoryID, upfrontAmount)
				if err != nil {
					return fmt.Errorf("resolve upfront price: %w", err)
				}
				upfrontID = point.ID
			}
			if monthlyAmount != "" {
				point, err := resolveWebSubscriptionPricePointFn(requestCtx, client, id, territoryID, monthlyAmount)
				if err != nil {
					return fmt.Errorf("resolve monthly price: %w", err)
				}
				monthlyID = point.ID
			}

			availabilities, err := listWebSubscriptionPlanAvailabilitiesFn(requestCtx, client, id)
			if err != nil {
				return withWebAuthHint(err, "web subscriptions pricing monthly-commitment bootstrap")
			}
			monthlyAvailability, found := findPlanAvailabilityByType(availabilities, "MONTHLY")
			created := false
			if found && availabilityExcludesTerritory(monthlyAvailability, territoryID) {
				return fmt.Errorf(
					"MONTHLY plan availability %q exists but does not include %s; add it with 'asc subscriptions pricing plan-availability set --subscription-id %s --plan-type MONTHLY --territories <complete list including %s> --confirm' before bootstrapping prices",
					monthlyAvailability.ID,
					territoryID,
					id,
					territoryID,
				)
			}
			if *dryRun {
				result := asc.WebSubscriptionMonthlyCommitmentBootstrapResult{
					SubscriptionID: id, Territory: territoryID,
					PlanAvailabilityID:          monthlyAvailability.ID,
					PlanAvailabilityWouldCreate: !found,
					UpfrontPricePointID:         upfrontID, MonthlyPricePointID: monthlyID,
					DryRun: true, StartDate: scheduledDate,
					PreserveCurrentPrice: *preserveCurrentPrice,
				}
				return d.PrintOutput(&result, *output.Output, *output.Pretty)
			}
			if !found {
				monthlyAvailability, err = dereferencePlanAvailability(createWebSubscriptionPlanAvailabilityFn(requestCtx, client, id, "MONTHLY", []string{territoryID}, false))
				if err != nil {
					return withWebAuthHint(err, "web subscriptions pricing monthly-commitment bootstrap")
				}
				created = true
			}

			result := asc.WebSubscriptionMonthlyCommitmentBootstrapResult{
				SubscriptionID:       id,
				Territory:            territoryID,
				PlanAvailabilityID:   monthlyAvailability.ID,
				PlanAvailabilityNew:  created,
				UpfrontPricePointID:  upfrontID,
				MonthlyPricePointID:  monthlyID,
				CompletedStage:       asc.WebMonthlyCommitmentStagePlanAvailability,
				StartDate:            scheduledDate,
				PreserveCurrentPrice: *preserveCurrentPrice,
			}
			if scheduledDate == "" {
				_, err = createWebSubscriptionPlanPricesFn(requestCtx, client, id, upfrontID, monthlyID)
			} else {
				_, err = setWebSubscriptionPlanPricesFn(requestCtx, client, id, []webcore.SubscriptionPlanPrice{
					{PlanType: "UPFRONT", PricePointID: upfrontID, StartDate: scheduledDate, PreserveCurrentPrice: *preserveCurrentPrice},
					{PlanType: "MONTHLY", PricePointID: monthlyID, StartDate: scheduledDate, PreserveCurrentPrice: *preserveCurrentPrice},
				})
			}
			if err != nil {
				runErr := fmt.Errorf("monthly availability ready, but paired price creation failed: %w", err)
				result.Failure = runErr.Error()
				return printMonthlyCommitmentBootstrapReceipt(d, result, *output.Output, *output.Pretty, runErr)
			}
			result.PricesCreated = true
			result.CompletedStage = asc.WebMonthlyCommitmentStagePrices
			verifyCtx, verifyCancel := d.Timeout(ctx)
			defer verifyCancel()
			if verifyErr := verifyMonthlyCommitmentBootstrap(verifyCtx, d, client, result); verifyErr != nil {
				result.Failure = verifyErr.Error()
				return printMonthlyCommitmentBootstrapReceipt(d, result, *output.Output, *output.Pretty, verifyErr)
			}
			result.Verified = true
			result.CompletedStage = asc.WebMonthlyCommitmentStageVerified
			return printMonthlyCommitmentBootstrapReceipt(d, result, *output.Output, *output.Pretty, nil)
		},
	}
}

// newWebSubscriptionsPricingAdjustedEqualizationsCommand returns the adjusted equalizations group.
func newWebSubscriptionsPricingAdjustedEqualizationsCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("web subscriptions pricing adjusted-equalizations", flag.ExitOnError)
	return &ffcli.Command{
		Name: "adjusted-equalizations", ShortUsage: "asc web subscriptions pricing adjusted-equalizations view [flags]",
		ShortHelp: "Inspect Apple's adjusted subscription price matrix.",
		FlagSet:   fs, UsageFunc: d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{newWebSubscriptionsPricingAdjustedEqualizationsViewCommand(d)},
		Exec:        func(ctx context.Context, args []string) error { return flag.ErrHelp },
	}
}

// newWebSubscriptionsPricingAdjustedEqualizationsViewCommand inspects one price point.
func newWebSubscriptionsPricingAdjustedEqualizationsViewCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("web subscriptions pricing adjusted-equalizations view", flag.ExitOnError)
	pricePointID := fs.String("price-point-id", "", "Subscription price point ID")
	planType := fs.String("plan-type", "MONTHLY", "Plan type (MONTHLY only)")
	authFlags := bindWebSessionFlags(fs)
	output := d.BindOutputFlags(fs)
	return &ffcli.Command{
		Name:       "view",
		ShortUsage: "asc web subscriptions pricing adjusted-equalizations view --price-point-id PRICE_POINT_ID [--plan-type MONTHLY] [flags]",
		ShortHelp:  "View a generated MONTHLY subscription price matrix.",
		FlagSet:    fs, UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*pricePointID)
			normalizedPlanType := strings.ToUpper(strings.TrimSpace(*planType))
			if id == "" {
				return d.UsageError("--price-point-id is required")
			}
			if normalizedPlanType != "MONTHLY" {
				return d.UsageError(`--plan-type only supports "MONTHLY"; Apple's endpoint rejects UPFRONT`)
			}
			session, requestCtx, cancel, err := resolveWebSessionForCommandWith(ctx, d, authFlags)
			defer cancel()
			if err != nil {
				return err
			}
			result, err := getWebSubscriptionAdjustedEqualizationsFn(requestCtx, newWebClientFn(session), id, normalizedPlanType)
			if err != nil {
				return withWebAuthHint(err, "web subscriptions pricing adjusted-equalizations view")
			}
			if !result.Available {
				row := []string{result.PricePointID, "false", result.Code, result.Detail, strings.Join(result.MissingTerritories, ", ")}
				return d.PrintOutputRows(result, *output.Output, *output.Pretty, []string{"Price Point ID", "Available", "Code", "Detail", "Missing Territories"}, [][]string{row})
			}
			rows := make([][]string, 0, len(result.Equalizations))
			for _, eq := range result.Equalizations {
				rows = append(rows, []string{eq.ID, eq.Territory, eq.CustomerPrice, eq.Currency})
			}
			return d.PrintOutputRows(result, *output.Output, *output.Pretty, []string{"ID", "Territory", "Customer Price", "Currency"}, rows)
		},
	}
}

func findPlanAvailabilityByType(availabilities []webcore.SubscriptionPlanAvailability, planType string) (webcore.SubscriptionPlanAvailability, bool) {
	for _, availability := range availabilities {
		if strings.EqualFold(strings.TrimSpace(availability.PlanType), planType) {
			return availability, true
		}
	}
	return webcore.SubscriptionPlanAvailability{}, false
}

func containsTerritory(territories []string, territory string) bool {
	for _, candidate := range territories {
		if strings.EqualFold(strings.TrimSpace(candidate), territory) {
			return true
		}
	}
	return false
}

func availabilityExcludesTerritory(availability webcore.SubscriptionPlanAvailability, territory string) bool {
	return availability.AvailableTerritoriesLoaded &&
		len(availability.AvailableTerritories) < webcore.SubscriptionPlanAvailabilityTerritoryLimit &&
		!containsTerritory(availability.AvailableTerritories, territory)
}

func dereferencePlanAvailability(availability *webcore.SubscriptionPlanAvailability, err error) (webcore.SubscriptionPlanAvailability, error) {
	if err != nil {
		return webcore.SubscriptionPlanAvailability{}, err
	}
	if availability == nil || strings.TrimSpace(availability.ID) == "" {
		return webcore.SubscriptionPlanAvailability{}, fmt.Errorf("apple returned an empty plan availability")
	}
	return *availability, nil
}

func printMonthlyCommitmentBootstrapReceipt(d *shared.Deps, result asc.WebSubscriptionMonthlyCommitmentBootstrapResult, format string, pretty bool, runErr error) error {
	if err := d.PrintOutput(&result, format, pretty); err != nil {
		if runErr != nil {
			return errors.Join(err, runErr)
		}
		return err
	}
	if runErr != nil {
		return shared.NewReportedError(runErr)
	}
	return nil
}

func verifyMonthlyCommitmentBootstrap(ctx context.Context, d *shared.Deps, client *webcore.Client, result asc.WebSubscriptionMonthlyCommitmentBootstrapResult) error {
	availabilities, err := listWebSubscriptionPlanAvailabilitiesFn(ctx, client, result.SubscriptionID)
	if err != nil {
		return fmt.Errorf("read back MONTHLY plan availability: %w", err)
	}
	monthlyAvailability, found := findPlanAvailabilityByType(availabilities, "MONTHLY")
	if !found {
		return fmt.Errorf("MONTHLY plan availability was missing after write")
	}
	if strings.TrimSpace(result.PlanAvailabilityID) != "" && !strings.EqualFold(strings.TrimSpace(monthlyAvailability.ID), strings.TrimSpace(result.PlanAvailabilityID)) {
		return fmt.Errorf("MONTHLY plan availability %q does not match written id %q", monthlyAvailability.ID, result.PlanAvailabilityID)
	}
	if availabilityExcludesTerritory(monthlyAvailability, result.Territory) {
		return fmt.Errorf("MONTHLY plan availability %q does not include %s after write", monthlyAvailability.ID, result.Territory)
	}

	prices, err := listWebSubscriptionPricesFn(ctx, client, result.SubscriptionID, result.Territory)
	if err != nil {
		return fmt.Errorf("read back subscription prices: %w", err)
	}
	startDate := webcore.NormalizeSubscriptionPriceStartDate(result.StartDate)
	now := d.Now().UTC()
	if _, ok := webcore.FindSubscriptionPrice(prices, "UPFRONT", result.Territory, result.UpfrontPricePointID, startDate, now); !ok {
		return fmt.Errorf("UPFRONT price record for %s did not match price point %s", result.Territory, result.UpfrontPricePointID)
	}
	if _, ok := webcore.FindSubscriptionPrice(prices, "MONTHLY", result.Territory, result.MonthlyPricePointID, startDate, now); !ok {
		return fmt.Errorf("MONTHLY price record for %s did not match price point %s", result.Territory, result.MonthlyPricePointID)
	}
	return nil
}
