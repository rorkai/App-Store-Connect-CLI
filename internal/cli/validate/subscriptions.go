package validate

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/validation"
)

type validateSubscriptionsOptions struct {
	AppID  string
	Strict bool
	Output string
	Pretty bool
}

// SubscriptionsOptions configures a non-printing subscription readiness report.
type SubscriptionsOptions struct {
	AppID  string
	Strict bool
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
		ShortHelp:  "Validate subscription metadata, screenshot delivery, pricing, and availability.",
		LongHelp: `Validate review readiness for auto-renewable subscriptions.

For subscriptions in MISSING_METADATA, this command inspects group and
subscription localizations, App Review screenshot delivery, availability, and
price coverage. It also emits advisory guidance for promotional images Apple
uses for App Store promotion, offer-code redemption pages, and win-back offers.
Use --strict to gate on warnings in CI.

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
				return shared.MissingRequiredUsageError()
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

	report, err := buildSubscriptionsReport(ctx, client, SubscriptionsOptions{
		AppID:  opts.AppID,
		Strict: opts.Strict,
	})
	if err != nil {
		return err
	}

	if err := shared.PrintOutput(&report, opts.Output, opts.Pretty); err != nil {
		return err
	}

	if report.Summary.Blocking > 0 {
		return shared.NewValidationReportedError(fmt.Errorf("validate subscriptions: found %d blocking issue(s)", report.Summary.Blocking))
	}

	return nil
}

// BuildSubscriptionsReport fetches live subscription state and returns the same
// structured report emitted by `asc validate subscriptions` without printing it.
func BuildSubscriptionsReport(ctx context.Context, opts SubscriptionsOptions) (validation.SubscriptionsReport, error) {
	client, err := clientFactory()
	if err != nil {
		return validation.SubscriptionsReport{}, fmt.Errorf("validate subscriptions: %w", err)
	}
	return buildSubscriptionsReport(ctx, client, opts)
}

func buildSubscriptionsReport(ctx context.Context, client *asc.Client, opts SubscriptionsOptions) (validation.SubscriptionsReport, error) {
	ctx = withReadinessRequestGate(ctx)
	pricingCoverageSkipReason := ""
	var appAvailableTerritories []string
	availableTerritories := 0
	var subs []validation.Subscription
	if err := runReadinessTasks(
		ctx,
		func(taskCtx context.Context) error {
			_, territories, count, fetchErr := fetchAvailableTerritoryDetailsFn(taskCtx, client, opts.AppID)
			if fetchErr != nil {
				if reason, ok := availabilityCheckSkipReason(fetchErr); ok {
					pricingCoverageSkipReason = reason
					return nil
				}
				return fmt.Errorf("validate subscriptions: %w", fetchErr)
			}
			appAvailableTerritories = territories
			availableTerritories = count
			return nil
		},
		func(taskCtx context.Context) error {
			var fetchErr error
			subs, fetchErr = fetchSubscriptionsFn(taskCtx, client, opts.AppID)
			if fetchErr != nil {
				return fmt.Errorf("validate subscriptions: %w", fetchErr)
			}
			return nil
		},
	); err != nil {
		return validation.SubscriptionsReport{}, err
	}

	buildCount := 0
	buildCheckSkipped := false
	buildCheckSkipReason := ""
	var buildStatus metadataCheckStatus
	for _, sub := range subs {
		if strings.EqualFold(strings.TrimSpace(sub.State), "MISSING_METADATA") {
			var err error
			buildCount, buildStatus, err = fetchAppBuildCountFn(ctx, client, opts.AppID)
			if err != nil {
				return validation.SubscriptionsReport{}, fmt.Errorf("validate subscriptions: %w", err)
			}
			buildCheckSkipped = !buildStatus.Verified
			buildCheckSkipReason = buildStatus.SkipReason
			break
		}
	}

	report := validation.ValidateSubscriptions(validation.SubscriptionsInput{
		AppID:                     opts.AppID,
		Subscriptions:             subs,
		AvailableTerritories:      availableTerritories,
		AppAvailableTerritories:   appAvailableTerritories,
		PricingCoverageSkipReason: pricingCoverageSkipReason,
		AppBuildCount:             buildCount,
		BuildCheckSkipped:         buildCheckSkipped,
		BuildCheckSkipReason:      buildCheckSkipReason,
	}, opts.Strict)

	return report, nil
}
