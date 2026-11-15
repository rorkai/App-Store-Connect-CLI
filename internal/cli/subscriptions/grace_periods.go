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

// newSubscriptionsGracePeriodsCommand returns the grace periods command group.
func newSubscriptionsGracePeriodsCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("grace-periods", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "grace-periods",
		ShortUsage: "asc subscriptions grace-periods <subcommand> [flags]",
		ShortHelp:  "Inspect subscription grace periods.",
		LongHelp: `Inspect subscription grace periods.

Examples:
  asc subscriptions grace-periods view --id "GRACE_PERIOD_ID"
  asc subscriptions grace-periods update --id "GRACE_PERIOD_ID" --duration SIXTEEN_DAYS --opt-in true`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newSubscriptionsGracePeriodsGetCommand(d),
			newSubscriptionsGracePeriodsUpdateCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newSubscriptionsGracePeriodsGetCommand returns the grace period get subcommand.
func newSubscriptionsGracePeriodsGetCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		Deps:        d,
		FlagSetName: "grace-periods view",
		Name:        "view",
		ShortUsage:  "asc subscriptions grace-periods view --id \"GRACE_PERIOD_ID\"",
		ShortHelp:   "View a subscription grace period by ID.",
		LongHelp: `View a subscription grace period by ID.

Examples:
  asc subscriptions grace-periods view --id "GRACE_PERIOD_ID"`,
		IDUsage:            "Subscription grace period ID",
		IDType:             "subscriptionGracePeriods",
		RequiredDiagnostic: true,
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.GetSubscriptionGracePeriod(ctx, id)
		},
	})
}

// newSubscriptionsGracePeriodsUpdateCommand returns the grace period update subcommand.
func newSubscriptionsGracePeriodsUpdateCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("grace-periods update", flag.ExitOnError)

	gracePeriodID := shared.BindResourceIDFlag(fs, "id", "subscriptionGracePeriods", "Subscription grace period ID")
	var optIn shared.OptionalBool
	fs.Var(&optIn, "opt-in", "Enable grace period opt-in: true or false")
	var sandboxOptIn shared.OptionalBool
	fs.Var(&sandboxOptIn, "sandbox-opt-in", "Enable grace period sandbox opt-in: true or false")
	duration := fs.String("duration", "", "Grace period duration: "+strings.Join(subscriptionGracePeriodDurationValues, ", "))
	renewalType := fs.String("renewal-type", "", "Grace period renewal type: "+strings.Join(subscriptionGracePeriodRenewalTypeValues, ", "))
	output := d.BindOutputFlags(fs)

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
				return d.RequiredFlag("--id")
			}

			durationValue, err := normalizeSubscriptionGracePeriodDuration(*duration, false)
			if err != nil {
				fmt.Fprintln(d.IO.ErrOut, "Error:", err.Error())
				return flag.ErrHelp
			}
			renewalTypeValue, err := normalizeSubscriptionGracePeriodRenewalType(*renewalType, false)
			if err != nil {
				fmt.Fprintln(d.IO.ErrOut, "Error:", err.Error())
				return flag.ErrHelp
			}
			if !optIn.IsSet() && !sandboxOptIn.IsSet() && durationValue == "" && renewalTypeValue == "" {
				return d.UpdateFlagRequired(fs, &optIn, &sandboxOptIn, duration, renewalType)
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("subscriptions grace-periods update: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
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

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}
