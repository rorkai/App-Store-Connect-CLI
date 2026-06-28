package validate

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/validation"
)

type validateIAPOptions struct {
	AppID  string
	Strict bool
	Output string
	Pretty bool
}

// ValidateIAPCommand returns the asc validate iap subcommand.
func ValidateIAPCommand() *ffcli.Command {
	fs := flag.NewFlagSet("iap", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID)")
	strict := fs.Bool("strict", false, "Treat warnings as errors (exit non-zero)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "iap",
		ShortUsage: "asc validate iap --app \"APP_ID\" [flags]",
		ShortHelp:  "Validate IAP review readiness (warning-only by default).",
		LongHelp: `Validate review readiness for in-app purchases.

This command is conservative: it emits warnings for IAPs that look unsubmitted or
need action, but it does not block by default (use --strict for CI).

Examples:
  asc validate iap --app "APP_ID"
  asc validate iap --app "APP_ID" --output table
  asc validate iap --app "APP_ID" --strict`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			resolvedAppID := shared.ResolveAppID(*appID)
			if resolvedAppID == "" {
				fmt.Fprintln(os.Stderr, "Error: --app is required (or set ASC_APP_ID)")
				return shared.MissingRequiredUsageError()
			}

			return runValidateIAP(ctx, validateIAPOptions{
				AppID:  resolvedAppID,
				Strict: *strict,
				Output: *output.Output,
				Pretty: *output.Pretty,
			})
		},
	}
}

func runValidateIAP(ctx context.Context, opts validateIAPOptions) error {
	client, err := clientFactory()
	if err != nil {
		return fmt.Errorf("validate iap: %w", err)
	}

	firstCtx, firstCancel := shared.ContextWithTimeout(ctx)
	defer firstCancel()

	firstPage, err := client.GetInAppPurchasesV2(firstCtx, opts.AppID, asc.WithIAPLimit(200))
	if err != nil {
		return fmt.Errorf("validate iap: failed to fetch in-app purchases: %w", err)
	}

	paginated, err := asc.PaginateAll(ctx, firstPage, func(_ context.Context, nextURL string) (asc.PaginatedResponse, error) {
		pageCtx, pageCancel := shared.ContextWithTimeout(ctx)
		defer pageCancel()
		return client.GetInAppPurchasesV2(pageCtx, opts.AppID, asc.WithIAPNextURL(nextURL))
	})
	if err != nil {
		return fmt.Errorf("validate iap: paginate in-app purchases: %w", err)
	}

	resp, ok := paginated.(*asc.InAppPurchasesV2Response)
	if !ok {
		return fmt.Errorf("validate iap: unexpected in-app purchases response type %T", paginated)
	}

	iaps := make([]validation.IAP, 0, len(resp.Data))
	for _, item := range resp.Data {
		attrs := item.Attributes
		iaps = append(iaps, validation.IAP{
			ID:        item.ID,
			Name:      attrs.Name,
			ProductID: attrs.ProductID,
			Type:      attrs.InAppPurchaseType,
			State:     attrs.State,
		})
	}

	report := validation.ValidateIAP(validation.IAPInput{
		AppID: opts.AppID,
		IAPs:  iaps,
	}, opts.Strict)

	if err := shared.PrintOutput(&report, opts.Output, opts.Pretty); err != nil {
		return err
	}

	if report.Summary.Blocking > 0 {
		return shared.NewReportedError(fmt.Errorf("validate iap: found %d blocking issue(s)", report.Summary.Blocking))
	}

	return nil
}
