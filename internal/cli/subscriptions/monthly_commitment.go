package subscriptions

import (
	"context"
	"flag"
	"fmt"
	"math/big"
	"os"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/ascterritory"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

type subscriptionBillingMode string

const (
	subscriptionBillingModeUpfront           subscriptionBillingMode = "upfront"
	subscriptionBillingModeMonthlyCommitment subscriptionBillingMode = "monthly-commitment"
)

var monthlyCommitmentExcludedTerritories = map[string]struct{}{
	"USA": {},
	"SGP": {},
}

// SubscriptionsPricingMonthlyCommitmentCommand returns the monthly-commitment command group.
func SubscriptionsPricingMonthlyCommitmentCommand() *ffcli.Command {
	fs := flag.NewFlagSet("pricing monthly-commitment", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "monthly-commitment",
		ShortUsage: "asc subscriptions pricing monthly-commitment <subcommand> [flags]",
		ShortHelp:  "Manage Monthly with 12-Month Commitment subscription availability.",
		LongHelp: `Manage Monthly with 12-Month Commitment subscription availability.

App Store Connect API 4.4 exposes subscriptionPlanAvailabilities for configuring
monthly subscriptions with a 12-month commitment. The subscription must use
subscriptionPeriod ONE_YEAR. USA and Singapore are excluded by Apple.

Examples:
  asc subscriptions pricing monthly-commitment enable --subscription-id "SUB_ID" --price "9.99" --price-territory "Norway" --territories "Norway,Germany,France"
  asc subscriptions pricing monthly-commitment disable --subscription-id "SUB_ID" --territories "Norway"
  asc subscriptions pricing monthly-commitment list --subscription-id "SUB_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			SubscriptionsPricingMonthlyCommitmentEnableCommand(),
			SubscriptionsPricingMonthlyCommitmentDisableCommand(),
			SubscriptionsPricingMonthlyCommitmentListCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// SubscriptionsPricingMonthlyCommitmentEnableCommand enables monthly commitment billing.
func SubscriptionsPricingMonthlyCommitmentEnableCommand() *ffcli.Command {
	fs := flag.NewFlagSet("pricing monthly-commitment enable", flag.ExitOnError)

	subscriptionID := fs.String("subscription-id", "", "Subscription ID, product ID, or exact current name")
	appID := addSubscriptionLookupAppFlag(fs)
	price := fs.String("price", "", "Monthly customer price; total commitment is price x 12")
	priceTerritory := fs.String("price-territory", "", "Territory used to compare the upfront annual price")
	territories := fs.String("territories", "", "Territories to enable, comma-separated; USA and Singapore are excluded")
	availableInNew := fs.Bool("available-in-new-territories", false, "Include new eligible territories automatically")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "enable",
		ShortUsage: "asc subscriptions pricing monthly-commitment enable [flags]",
		ShortHelp:  "Enable monthly-commitment availability.",
		LongHelp: `Enable Monthly with 12-Month Commitment availability.

The subscription must use subscriptionPeriod ONE_YEAR. USA and Singapore are
removed from --territories because Apple excludes those storefronts. The CLI
also checks that the 12-payment total is at least the upfront annual price and
no more than 1.5x the upfront annual price when the current upfront price can be
read from App Store Connect.

Examples:
  asc subscriptions pricing monthly-commitment enable --subscription-id "SUB_ID" --price "9.99" --price-territory "Norway" --territories "Norway,Germany,France"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := shared.RecoverBoolFlagTailArgs(fs, args, availableInNew); err != nil {
				return err
			}
			if len(args) > 0 {
				return shared.UsageError("subscriptions pricing monthly-commitment enable does not accept positional arguments")
			}

			id := strings.TrimSpace(*subscriptionID)
			if id == "" {
				return shared.UsageError("--subscription-id is required")
			}
			monthlyPrice := strings.TrimSpace(*price)
			if monthlyPrice == "" {
				return shared.UsageError("--price is required")
			}
			if err := shared.ValidateFinitePriceFlag("--price", monthlyPrice); err != nil {
				return shared.UsageError(err.Error())
			}
			priceTerritoryID, err := normalizeMonthlyCommitmentTerritory(*priceTerritory, "--price-territory")
			if err != nil {
				return shared.UsageError(err.Error())
			}
			if _, excluded := monthlyCommitmentExcludedTerritories[priceTerritoryID]; excluded {
				return shared.UsageError("--price-territory cannot be USA or Singapore for monthly-commitment billing")
			}

			territoryIDs, err := shared.NormalizeASCTerritoryCSV(*territories)
			if err != nil {
				return shared.UsageError(err.Error())
			}
			if len(territoryIDs) == 0 {
				return shared.UsageError("--territories is required")
			}
			territoryIDs, excluded := filterMonthlyCommitmentTerritories(territoryIDs)
			printMonthlyCommitmentTerritoryWarning(excluded)
			if len(territoryIDs) == 0 {
				return shared.UsageError("no eligible monthly-commitment territories remain after excluding USA and Singapore")
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("subscriptions pricing monthly-commitment enable: %w", err)
			}

			id, err = resolveSubscriptionLookupIDWithTimeout(ctx, client, *appID, id)
			if err != nil {
				return err
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			subResp, err := client.GetSubscription(requestCtx, id)
			if err != nil {
				return fmt.Errorf("subscriptions pricing monthly-commitment enable: failed to fetch subscription: %w", err)
			}
			if subResp.Data.Attributes.SubscriptionPeriod != string(asc.SubscriptionPeriodOneYear) {
				return shared.UsageError("--subscription-id must refer to a ONE_YEAR subscription for monthly-commitment billing")
			}

			summary, err := resolveSubscriptionPriceSummary(requestCtx, client, subWithGroup{Sub: subResp.Data}, priceTerritoryID)
			if err != nil {
				return fmt.Errorf("subscriptions pricing monthly-commitment enable: failed to fetch upfront price: %w", err)
			}
			if summary.CurrentPrice == nil || strings.TrimSpace(summary.CurrentPrice.Amount) == "" {
				return fmt.Errorf("subscriptions pricing monthly-commitment enable: current upfront price is missing for %s", priceTerritoryID)
			}
			if err := validateMonthlyCommitmentPriceRange(monthlyPrice, summary.CurrentPrice.Amount); err != nil {
				return fmt.Errorf("subscriptions pricing monthly-commitment enable: %w", err)
			}

			availableInNewValue := *availableInNew
			attrs := asc.SubscriptionPlanAvailabilityAttributes{
				AvailableInNewTerritories: &availableInNewValue,
				PlanType:                  asc.SubscriptionPlanTypeMonthly,
			}

			existing, err := client.GetSubscriptionPlanAvailabilitiesForSubscription(requestCtx, id)
			if err != nil {
				return fmt.Errorf("subscriptions pricing monthly-commitment enable: failed to fetch plan availabilities: %w", err)
			}

			if monthlyPlan, ok := findMonthlySubscriptionPlanAvailability(existing); ok {
				resp, err := client.UpdateSubscriptionPlanAvailability(requestCtx, monthlyPlan.ID, territoryIDs, &attrs)
				if err != nil {
					return fmt.Errorf("subscriptions pricing monthly-commitment enable: failed to update plan availability: %w", err)
				}
				return shared.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := client.CreateSubscriptionPlanAvailability(requestCtx, id, territoryIDs, attrs)
			if err != nil {
				return fmt.Errorf("subscriptions pricing monthly-commitment enable: failed to create plan availability: %w", err)
			}
			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// SubscriptionsPricingMonthlyCommitmentDisableCommand disables monthly commitment billing.
func SubscriptionsPricingMonthlyCommitmentDisableCommand() *ffcli.Command {
	fs := flag.NewFlagSet("pricing monthly-commitment disable", flag.ExitOnError)

	subscriptionID := fs.String("subscription-id", "", "Subscription ID, product ID, or exact current name")
	appID := addSubscriptionLookupAppFlag(fs)
	territories := fs.String("territories", "", "Territories to disable, comma-separated; USA and Singapore are excluded")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "disable",
		ShortUsage: "asc subscriptions pricing monthly-commitment disable [flags]",
		ShortHelp:  "Disable monthly-commitment availability.",
		LongHelp: `Disable Monthly with 12-Month Commitment availability.

Examples:
  asc subscriptions pricing monthly-commitment disable --subscription-id "SUB_ID" --territories "Norway"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return shared.UsageError("subscriptions pricing monthly-commitment disable does not accept positional arguments")
			}
			id := strings.TrimSpace(*subscriptionID)
			if id == "" {
				return shared.UsageError("--subscription-id is required")
			}
			territoryIDs, err := shared.NormalizeASCTerritoryCSV(*territories)
			if err != nil {
				return shared.UsageError(err.Error())
			}
			if len(territoryIDs) == 0 {
				return shared.UsageError("--territories is required")
			}
			territoryIDs, excluded := filterMonthlyCommitmentTerritories(territoryIDs)
			printMonthlyCommitmentTerritoryWarning(excluded)
			if len(territoryIDs) == 0 {
				return shared.UsageError("no eligible monthly-commitment territories remain after excluding USA and Singapore")
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("subscriptions pricing monthly-commitment disable: %w", err)
			}
			id, err = resolveSubscriptionLookupIDWithTimeout(ctx, client, *appID, id)
			if err != nil {
				return err
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			existing, err := client.GetSubscriptionPlanAvailabilitiesForSubscription(requestCtx, id)
			if err != nil {
				return fmt.Errorf("subscriptions pricing monthly-commitment disable: failed to fetch plan availabilities: %w", err)
			}
			monthlyPlan, ok := findMonthlySubscriptionPlanAvailability(existing)
			if !ok {
				return fmt.Errorf("subscriptions pricing monthly-commitment disable: no monthly-commitment plan availability found for subscription %q", id)
			}

			resp, err := client.UpdateSubscriptionPlanAvailability(requestCtx, monthlyPlan.ID, nil, nil)
			if err != nil {
				return fmt.Errorf("subscriptions pricing monthly-commitment disable: failed to update plan availability: %w", err)
			}
			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// SubscriptionsPricingMonthlyCommitmentListCommand lists monthly commitment billing.
func SubscriptionsPricingMonthlyCommitmentListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("pricing monthly-commitment list", flag.ExitOnError)

	subscriptionID := fs.String("subscription-id", "", "Subscription ID, product ID, or exact current name")
	appID := addSubscriptionLookupAppFlag(fs)
	planType := fs.String("plan-type", "", "Filter by plan type: MONTHLY or UPFRONT")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc subscriptions pricing monthly-commitment list --subscription-id \"SUB_ID\" [--plan-type MONTHLY|UPFRONT]",
		ShortHelp:  "List monthly-commitment plan availability.",
		LongHelp: `List Monthly with 12-Month Commitment plan availability.

Use --plan-type to filter results by MONTHLY or UPFRONT. Filtering is applied
client-side because the planAvailabilities list endpoint does not expose
filter[planType] in the public OpenAPI schema.

Examples:
  asc subscriptions pricing monthly-commitment list --subscription-id "SUB_ID"
  asc subscriptions pricing monthly-commitment list --subscription-id "SUB_ID" --plan-type MONTHLY`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return shared.UsageError("subscriptions pricing monthly-commitment list does not accept positional arguments")
			}
			id := strings.TrimSpace(*subscriptionID)
			if id == "" {
				return shared.UsageError("--subscription-id is required")
			}
			var planTypeFilter asc.SubscriptionPlanType
			planTypeProvided := false
			fs.Visit(func(f *flag.Flag) {
				if f.Name == "plan-type" {
					planTypeProvided = true
				}
			})
			if planTypeProvided {
				if strings.TrimSpace(*planType) == "" {
					return shared.UsageError("invalid value for --plan-type: cannot be empty")
				}
				normalized, err := normalizeSubscriptionPlanType(*planType)
				if err != nil {
					return shared.UsageError(err.Error())
				}
				planTypeFilter = normalized
			}
			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("subscriptions pricing monthly-commitment list: %w", err)
			}
			id, err = resolveSubscriptionLookupIDWithTimeout(ctx, client, *appID, id)
			if err != nil {
				return err
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.SubscriptionPlanAvailabilitiesOption{}
			if planTypeFilter != "" {
				opts = append(opts, asc.WithSubscriptionPlanAvailabilitiesPlanTypes(planTypeFilter))
			}

			resp, err := client.GetSubscriptionPlanAvailabilitiesForSubscription(requestCtx, id, opts...)
			if err != nil {
				return fmt.Errorf("subscriptions pricing monthly-commitment list: failed to fetch: %w", err)
			}
			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

func findMonthlySubscriptionPlanAvailability(resp *asc.SubscriptionPlanAvailabilitiesResponse) (asc.Resource[asc.SubscriptionPlanAvailabilityAttributes], bool) {
	if resp == nil {
		return asc.Resource[asc.SubscriptionPlanAvailabilityAttributes]{}, false
	}
	for _, item := range resp.Data {
		if item.Attributes.PlanType == asc.SubscriptionPlanTypeMonthly {
			return item, true
		}
	}
	return asc.Resource[asc.SubscriptionPlanAvailabilityAttributes]{}, false
}

func normalizeSubscriptionPlanType(value string) (asc.SubscriptionPlanType, error) {
	normalized := strings.ToUpper(strings.TrimSpace(value))
	switch normalized {
	case string(asc.SubscriptionPlanTypeMonthly):
		return asc.SubscriptionPlanTypeMonthly, nil
	case string(asc.SubscriptionPlanTypeUpfront):
		return asc.SubscriptionPlanTypeUpfront, nil
	default:
		return "", fmt.Errorf("--plan-type must be one of: MONTHLY, UPFRONT")
	}
}

func normalizeSubscriptionBillingMode(value string) (subscriptionBillingMode, error) {
	normalized := strings.ToLower(strings.TrimSpace(value))
	normalized = strings.ReplaceAll(normalized, "_", "-")
	if normalized == "" {
		return subscriptionBillingModeUpfront, nil
	}
	switch normalized {
	case "standard", string(subscriptionBillingModeUpfront):
		return subscriptionBillingModeUpfront, nil
	case "monthly-commitment", "monthly-with-12-month-commitment", "installment-billed-yearly":
		return subscriptionBillingModeMonthlyCommitment, nil
	default:
		return "", fmt.Errorf("--billing-mode must be one of: upfront, monthly-commitment")
	}
}

func normalizeMonthlyCommitmentTerritory(value string, flagName string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", fmt.Errorf("%s is required", flagName)
	}
	territory, err := ascterritory.Normalize(trimmed)
	if err != nil {
		return "", err
	}
	return territory, nil
}

func filterMonthlyCommitmentTerritories(territories []string) ([]string, []string) {
	eligible := make([]string, 0, len(territories))
	excluded := make([]string, 0, 2)
	seenExcluded := make(map[string]struct{}, 2)
	for _, territory := range territories {
		id := strings.ToUpper(strings.TrimSpace(territory))
		if id == "" {
			continue
		}
		if _, ok := monthlyCommitmentExcludedTerritories[id]; ok {
			if _, seen := seenExcluded[id]; !seen {
				excluded = append(excluded, id)
				seenExcluded[id] = struct{}{}
			}
			continue
		}
		eligible = append(eligible, id)
	}
	return eligible, excluded
}

func printMonthlyCommitmentTerritoryWarning(excluded []string) {
	if len(excluded) == 0 {
		return
	}
	fmt.Fprintf(os.Stderr, "Warning: monthly-commitment billing is unavailable in %s; removed from --territories.\n", strings.Join(excluded, ","))
}

func validateMonthlyCommitmentPriceRange(monthlyPrice string, upfrontPrice string) error {
	monthly, err := parsePositiveMoneyRat(monthlyPrice, "--price")
	if err != nil {
		return err
	}
	upfront, err := parsePositiveMoneyRat(upfrontPrice, "upfront price")
	if err != nil {
		return err
	}

	total := new(big.Rat).Mul(monthly, big.NewRat(12, 1))
	max := new(big.Rat).Mul(upfront, big.NewRat(3, 2))
	if total.Cmp(upfront) < 0 || total.Cmp(max) > 0 {
		return fmt.Errorf(
			"monthly commitment total %s is outside the allowed range [%s, %s] based on upfront price %s",
			formatMoneyRat(total),
			formatMoneyRat(upfront),
			formatMoneyRat(max),
			formatMoneyRat(upfront),
		)
	}
	return nil
}

func parsePositiveMoneyRat(value string, label string) (*big.Rat, error) {
	trimmed := strings.TrimSpace(value)
	rat, ok := new(big.Rat).SetString(trimmed)
	if !ok || rat.Sign() <= 0 {
		return nil, fmt.Errorf("%s must be a positive decimal price", label)
	}
	return rat, nil
}

func formatMoneyRat(value *big.Rat) string {
	if value == nil {
		return ""
	}
	return value.FloatString(2)
}
