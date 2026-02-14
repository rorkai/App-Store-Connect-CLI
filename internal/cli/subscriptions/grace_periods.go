package subscriptions

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// SubscriptionsGracePeriodsCommand returns the grace periods command group.
func SubscriptionsGracePeriodsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("grace-periods", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "grace-periods",
		ShortUsage: "asc subscriptions grace-periods <subcommand> [flags]",
		ShortHelp:  "Inspect subscription grace periods.",
		LongHelp: `Inspect subscription grace periods.

Examples:
  asc subscriptions grace-periods get --id "GRACE_PERIOD_ID"
  asc subscriptions grace-periods update --id "GRACE_PERIOD_ID" --duration SIXTEEN_DAYS --opt-in true`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			SubscriptionsGracePeriodsGetCommand(),
			SubscriptionsGracePeriodsUpdateCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// SubscriptionsGracePeriodsGetCommand returns the grace period get subcommand.
func SubscriptionsGracePeriodsGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("grace-periods get", flag.ExitOnError)

	gracePeriodID := fs.String("id", "", "Subscription grace period ID")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc subscriptions grace-periods get --id \"GRACE_PERIOD_ID\"",
		ShortHelp:  "Get a subscription grace period by ID.",
		LongHelp: `Get a subscription grace period by ID.

Examples:
  asc subscriptions grace-periods get --id "GRACE_PERIOD_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*gracePeriodID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("subscriptions grace-periods get: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetSubscriptionGracePeriod(requestCtx, id)
			if err != nil {
				return fmt.Errorf("subscriptions grace-periods get: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// SubscriptionsGracePeriodsUpdateCommand returns the grace period update subcommand.
func SubscriptionsGracePeriodsUpdateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("grace-periods update", flag.ExitOnError)

	gracePeriodID := fs.String("id", "", "Subscription grace period ID")
	var optIn shared.OptionalBool
	fs.Var(&optIn, "opt-in", "Enable grace period opt-in: true or false")
	var sandboxOptIn shared.OptionalBool
	fs.Var(&sandboxOptIn, "sandbox-opt-in", "Enable grace period sandbox opt-in: true or false")
	duration := fs.String("duration", "", "Grace period duration: "+strings.Join(subscriptionGracePeriodDurationValues, ", "))
	renewalType := fs.String("renewal-type", "", "Grace period renewal type: "+strings.Join(subscriptionGracePeriodRenewalTypeValues, ", "))
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "update",
		ShortUsage: "asc subscriptions grace-periods update [flags]",
		ShortHelp:  "Update a subscription grace period.",
		LongHelp: `Update a subscription grace period.

Examples:
  asc subscriptions grace-periods update --id "GRACE_PERIOD_ID" --duration SIXTEEN_DAYS --opt-in true`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*gracePeriodID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			durationValue, err := normalizeSubscriptionGracePeriodDuration(*duration, false)
			if err != nil {
				fmt.Fprintln(os.Stderr, "Error:", err.Error())
				return flag.ErrHelp
			}
			renewalTypeValue, err := normalizeSubscriptionGracePeriodRenewalType(*renewalType, false)
			if err != nil {
				fmt.Fprintln(os.Stderr, "Error:", err.Error())
				return flag.ErrHelp
			}
			if !optIn.IsSet() && !sandboxOptIn.IsSet() && durationValue == "" && renewalTypeValue == "" {
				fmt.Fprintln(os.Stderr, "Error: at least one update flag is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("subscriptions grace-periods update: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			attrs := asc.SubscriptionGracePeriodUpdateAttributes{}
			if optIn.IsSet() {
				value := optIn.Value()
				attrs.OptIn = &value
			}
			if sandboxOptIn.IsSet() {
				value := sandboxOptIn.Value()
				attrs.SandboxOptIn = &value
			}
			if durationValue != "" {
				value := durationValue
				attrs.Duration = &value
			}
			if renewalTypeValue != "" {
				value := string(renewalTypeValue)
				attrs.RenewalType = &value
			}

			resp, err := client.UpdateSubscriptionGracePeriod(requestCtx, id, attrs)
			if err != nil {
				return fmt.Errorf("subscriptions grace-periods update: failed to update: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}
