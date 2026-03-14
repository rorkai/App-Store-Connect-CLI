package validate

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/validation"
)

type validateSubscriptionsOptions struct {
	AppID  string
	Strict bool
	Output string
	Pretty bool
}

// ValidateSubscriptionsCommand returns the asc validate subscriptions subcommand.
func ValidateSubscriptionsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("subscriptions", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID)")
	strict := fs.Bool("strict", false, "Treat warnings as errors (exit non-zero)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "subscriptions",
		ShortUsage: "asc validate subscriptions --app \"APP_ID\" [flags]",
		ShortHelp:  "Validate subscription review readiness and promotional image guidance.",
		LongHelp: `Validate review readiness for auto-renewable subscriptions.

This command is conservative: it emits warnings for subscriptions that need
review attention or are missing promotional images Apple uses for App Store
promotion, offer-code redemption pages, and win-back offers. Use --strict to
gate on warnings in CI.

Examples:
  asc validate subscriptions --app "APP_ID"
  asc validate subscriptions --app "APP_ID" --output table
  asc validate subscriptions --app "APP_ID" --strict`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			resolvedAppID := shared.ResolveAppID(*appID)
			if resolvedAppID == "" {
				fmt.Fprintln(os.Stderr, "Error: --app is required (or set ASC_APP_ID)")
				return flag.ErrHelp
			}

			return runValidateSubscriptions(ctx, validateSubscriptionsOptions{
				AppID:  resolvedAppID,
				Strict: *strict,
				Output: *output.Output,
				Pretty: *output.Pretty,
			})
		},
	}
}

func runValidateSubscriptions(ctx context.Context, opts validateSubscriptionsOptions) error {
	client, err := clientFactory()
	if err != nil {
		return fmt.Errorf("validate subscriptions: %w", err)
	}

	requestCtx, cancel := shared.ContextWithTimeout(ctx)
	defer cancel()

	pricingCoverageSkipReason := ""
	_, availableTerritories, err := fetchAvailableTerritories(requestCtx, client, opts.AppID)
	if err != nil {
		if reason, ok := availabilityCheckSkipReason(err); ok {
			pricingCoverageSkipReason = reason
		} else {
			return fmt.Errorf("validate subscriptions: %w", err)
		}
	}

	subs, err := fetchSubscriptionsFn(ctx, client, opts.AppID)
	if err != nil {
		return fmt.Errorf("validate subscriptions: %w", err)
	}

	report := validation.ValidateSubscriptions(validation.SubscriptionsInput{
		AppID:                     opts.AppID,
		Subscriptions:             subs,
		AvailableTerritories:      availableTerritories,
		PricingCoverageSkipReason: pricingCoverageSkipReason,
	}, opts.Strict)

	if err := shared.PrintOutput(&report, opts.Output, opts.Pretty); err != nil {
		return err
	}

	if report.Summary.Blocking > 0 {
		return shared.NewReportedError(fmt.Errorf("validate subscriptions: found %d blocking issue(s)", report.Summary.Blocking))
	}

	return nil
}
