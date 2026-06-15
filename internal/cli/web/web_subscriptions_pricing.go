package web

import (
	"context"
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
	resolveWebSubscriptionPricePointFn = func(ctx context.Context, client *webcore.Client, subscriptionID, territory, customerPrice string) (*webcore.SubscriptionPricePoint, error) {
		return client.ResolveSubscriptionPricePoint(ctx, subscriptionID, territory, customerPrice)
	}
	getWebSubscriptionAdjustedEqualizationsFn = func(ctx context.Context, client *webcore.Client, pricePointID, planType string) (*webcore.SubscriptionAdjustedEqualizationsResult, error) {
		return client.GetSubscriptionAdjustedEqualizations(ctx, pricePointID, planType)
	}
)

type webSubscriptionMonthlyCommitmentBootstrapResult struct {
	SubscriptionID              string `json:"subscriptionId"`
	Territory                   string `json:"territory"`
	PlanAvailabilityID          string `json:"planAvailabilityId"`
	PlanAvailabilityNew         bool   `json:"planAvailabilityCreated"`
	PlanAvailabilityWouldCreate bool   `json:"planAvailabilityWouldCreate,omitempty"`
	UpfrontPricePointID         string `json:"upfrontPricePointId"`
	MonthlyPricePointID         string `json:"monthlyPricePointId"`
	PricesCreated               bool   `json:"pricesCreated"`
	DryRun                      bool   `json:"dryRun"`
	StartDate                   string `json:"startDate,omitempty"`
	PreserveCurrentPrice        bool   `json:"preserveCurrentPrice,omitempty"`
}

// WebSubscriptionsPricingCommand returns the web subscription pricing command group.
func WebSubscriptionsPricingCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web subscriptions pricing", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "pricing",
		ShortUsage: "asc web subscriptions pricing <subcommand> [flags]",
		ShortHelp:  "[experimental] Manage subscription pricing via web sessions.",
		LongHelp: `EXPERIMENTAL / UNOFFICIAL / DISCOURAGED

Manage subscription pricing through Apple's internal web API.

` + webWarningText,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			WebSubscriptionsPricingAdjustedEqualizationsCommand(),
			WebSubscriptionsPricingMonthlyCommitmentCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// WebSubscriptionsPricingMonthlyCommitmentCommand returns the monthly commitment group.
func WebSubscriptionsPricingMonthlyCommitmentCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web subscriptions pricing monthly-commitment", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "monthly-commitment",
		ShortUsage: "asc web subscriptions pricing monthly-commitment <subcommand> [flags]",
		ShortHelp:  "[experimental] Bootstrap monthly-with-commitment pricing.",
		LongHelp: `EXPERIMENTAL / UNOFFICIAL / DISCOURAGED

Bootstrap monthly-with-12-month-commitment pricing through Apple's internal web API.

` + webWarningText,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			WebSubscriptionsPricingMonthlyCommitmentBootstrapCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// WebSubscriptionsPricingMonthlyCommitmentBootstrapCommand creates availability and prices.
func WebSubscriptionsPricingMonthlyCommitmentBootstrapCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web subscriptions pricing monthly-commitment bootstrap", flag.ExitOnError)

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
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "bootstrap",
		ShortUsage: "asc web subscriptions pricing monthly-commitment bootstrap --subscription-id SUB_ID --territory NOR (--upfront-price PRICE | --upfront-price-point-id ID) (--monthly-price PRICE | --monthly-price-point-id ID) [--dry-run | --confirm] [flags]",
		ShortHelp:  "[experimental] Create monthly plan availability and paired prices.",
		LongHelp: `EXPERIMENTAL / UNOFFICIAL / DISCOURAGED

Create MONTHLY plan availability, then attach paired UPFRONT and MONTHLY prices
using the same private inline subscription PATCH as App Store Connect.

Prefer asc subscriptions pricing monthly-commitment enable for normal setup.
Use this private command only when you specifically need App Store Connect's
paired web pricing workflow or a paired scheduled price change.

Prices may be supplied as exact customer prices or price point IDs. Use
--start-date for a scheduled paired price change. --preserve-current-price
applies only to scheduled changes. --dry-run performs all reads and price
resolution but does not mutate App Store Connect.

` + webWarningText,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return shared.UsageError("web subscriptions pricing monthly-commitment bootstrap does not accept positional arguments")
			}
			id := strings.TrimSpace(*subscriptionID)
			territoryID := strings.ToUpper(strings.TrimSpace(*territory))
			upfrontID := strings.TrimSpace(*upfrontPricePointID)
			monthlyID := strings.TrimSpace(*monthlyPricePointID)
			upfrontAmount := strings.TrimSpace(*upfrontPrice)
			monthlyAmount := strings.TrimSpace(*monthlyPrice)
			scheduledDate := strings.TrimSpace(*startDate)
			switch {
			case id == "":
				return shared.UsageError("--subscription-id is required")
			case len(territoryID) != 3:
				return shared.UsageError("--territory must be a three-letter territory ID")
			case territoryID == "USA" || territoryID == "SGP":
				return shared.UsageError("--territory cannot be USA or Singapore for monthly-commitment pricing")
			case (upfrontID == "") == (upfrontAmount == ""):
				return shared.UsageError("exactly one of --upfront-price or --upfront-price-point-id is required")
			case (monthlyID == "") == (monthlyAmount == ""):
				return shared.UsageError("exactly one of --monthly-price or --monthly-price-point-id is required")
			case *preserveCurrentPrice && scheduledDate == "":
				return shared.UsageError("--preserve-current-price requires --start-date")
			case !*dryRun && !*confirm:
				return shared.UsageError("--confirm is required")
			}
			if scheduledDate != "" {
				if _, err := time.Parse("2006-01-02", scheduledDate); err != nil {
					return shared.UsageError("--start-date must use YYYY-MM-DD")
				}
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()
			session, err := resolveWebSessionForCommand(requestCtx, authFlags)
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
				return fmt.Errorf("MONTHLY plan availability %q exists but does not include %s; update its territories before bootstrapping prices", monthlyAvailability.ID, territoryID)
			}
			if *dryRun {
				result := webSubscriptionMonthlyCommitmentBootstrapResult{
					SubscriptionID: id, Territory: territoryID,
					PlanAvailabilityID:          monthlyAvailability.ID,
					PlanAvailabilityWouldCreate: !found,
					UpfrontPricePointID:         upfrontID, MonthlyPricePointID: monthlyID,
					DryRun: true, StartDate: scheduledDate,
					PreserveCurrentPrice: *preserveCurrentPrice,
				}
				return shared.PrintOutput(result, *output.Output, *output.Pretty)
			}
			if !found {
				monthlyAvailability, err = dereferencePlanAvailability(createWebSubscriptionPlanAvailabilityFn(requestCtx, client, id, "MONTHLY", []string{territoryID}, false))
				if err != nil {
					return withWebAuthHint(err, "web subscriptions pricing monthly-commitment bootstrap")
				}
				created = true
			}

			var prices *webcore.SubscriptionPlanPricesResult
			if scheduledDate == "" {
				prices, err = createWebSubscriptionPlanPricesFn(requestCtx, client, id, upfrontID, monthlyID)
			} else {
				prices, err = setWebSubscriptionPlanPricesFn(requestCtx, client, id, []webcore.SubscriptionPlanPrice{
					{PlanType: "UPFRONT", PricePointID: upfrontID, StartDate: scheduledDate, PreserveCurrentPrice: *preserveCurrentPrice},
					{PlanType: "MONTHLY", PricePointID: monthlyID, StartDate: scheduledDate, PreserveCurrentPrice: *preserveCurrentPrice},
				})
			}
			if err != nil {
				return fmt.Errorf("monthly availability ready, but paired price creation failed: %w", err)
			}
			result := webSubscriptionMonthlyCommitmentBootstrapResult{
				SubscriptionID:       id,
				Territory:            territoryID,
				PlanAvailabilityID:   monthlyAvailability.ID,
				PlanAvailabilityNew:  created,
				UpfrontPricePointID:  prices.UpfrontPricePointID,
				MonthlyPricePointID:  prices.MonthlyPricePointID,
				PricesCreated:        true,
				StartDate:            scheduledDate,
				PreserveCurrentPrice: *preserveCurrentPrice,
			}
			return shared.PrintOutputWithRenderers(
				result,
				*output.Output,
				*output.Pretty,
				func() error { return renderWebMonthlyCommitmentBootstrapTable(result) },
				func() error { return renderWebMonthlyCommitmentBootstrapMarkdown(result) },
			)
		},
	}
}

// WebSubscriptionsPricingAdjustedEqualizationsCommand returns the adjusted equalizations group.
func WebSubscriptionsPricingAdjustedEqualizationsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web subscriptions pricing adjusted-equalizations", flag.ExitOnError)
	return &ffcli.Command{
		Name: "adjusted-equalizations", ShortUsage: "asc web subscriptions pricing adjusted-equalizations view [flags]",
		ShortHelp: "[experimental] Inspect Apple's adjusted subscription price matrix.",
		FlagSet:   fs, UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{WebSubscriptionsPricingAdjustedEqualizationsViewCommand()},
		Exec:        func(ctx context.Context, args []string) error { return flag.ErrHelp },
	}
}

// WebSubscriptionsPricingAdjustedEqualizationsViewCommand inspects one price point.
func WebSubscriptionsPricingAdjustedEqualizationsViewCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web subscriptions pricing adjusted-equalizations view", flag.ExitOnError)
	pricePointID := fs.String("price-point-id", "", "Subscription price point ID")
	planType := fs.String("plan-type", "MONTHLY", "Plan type (MONTHLY only)")
	authFlags := bindWebSessionFlags(fs)
	output := shared.BindOutputFlags(fs)
	return &ffcli.Command{
		Name:       "view",
		ShortUsage: "asc web subscriptions pricing adjusted-equalizations view --price-point-id PRICE_POINT_ID [--plan-type MONTHLY] [flags]",
		ShortHelp:  "[experimental] View a generated MONTHLY subscription price matrix.",
		FlagSet:    fs, UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return shared.UsageError("web subscriptions pricing adjusted-equalizations view does not accept positional arguments")
			}
			id := strings.TrimSpace(*pricePointID)
			normalizedPlanType := strings.ToUpper(strings.TrimSpace(*planType))
			if id == "" {
				return shared.UsageError("--price-point-id is required")
			}
			if normalizedPlanType != "MONTHLY" {
				return shared.UsageError(`--plan-type only supports "MONTHLY"; Apple's private endpoint rejects UPFRONT`)
			}
			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()
			session, err := resolveWebSessionForCommand(requestCtx, authFlags)
			if err != nil {
				return err
			}
			result, err := getWebSubscriptionAdjustedEqualizationsFn(requestCtx, newWebClientFn(session), id, normalizedPlanType)
			if err != nil {
				return withWebAuthHint(err, "web subscriptions pricing adjusted-equalizations view")
			}
			return shared.PrintOutput(result, *output.Output, *output.Pretty)
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

func renderWebMonthlyCommitmentBootstrapTable(result webSubscriptionMonthlyCommitmentBootstrapResult) error {
	asc.RenderTable(
		[]string{"Subscription ID", "Territory", "Plan Availability ID", "Availability Created", "Prices Created"},
		[][]string{{
			result.SubscriptionID,
			result.Territory,
			result.PlanAvailabilityID,
			fmt.Sprintf("%t", result.PlanAvailabilityNew),
			fmt.Sprintf("%t", result.PricesCreated),
		}},
	)
	return nil
}

func renderWebMonthlyCommitmentBootstrapMarkdown(result webSubscriptionMonthlyCommitmentBootstrapResult) error {
	fmt.Println("| Subscription ID | Territory | Plan Availability ID | Availability Created | Prices Created |")
	fmt.Println("|---|---|---|---|---|")
	fmt.Printf("| %s | %s | %s | %t | %t |\n", result.SubscriptionID, result.Territory, result.PlanAvailabilityID, result.PlanAvailabilityNew, result.PricesCreated)
	return nil
}
