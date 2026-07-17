package iap

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// IAPSubmitCommand returns the submit subcommand.
func IAPSubmitCommand() *ffcli.Command {
	fs := flag.NewFlagSet("submit", flag.ExitOnError)

	iapID := fs.String("iap-id", "", "In-app purchase ID, product ID, or exact current name")
	appID := addIAPLookupAppFlag(fs)
	confirm := fs.Bool("confirm", false, "Confirm submission")
	output := shared.BindOutputFlags(fs)

	return shared.DeprecatedCommand(&ffcli.Command{
		Name:       "submit",
		ShortUsage: "asc iap submit --iap-id \"IAP_ID\" --confirm",
		ShortHelp:  "Submit an in-app purchase for review.",
		LongHelp: `Submit an in-app purchase for review.

Examples:
  asc iap submit --iap-id "IAP_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			iapValue := strings.TrimSpace(*iapID)
			if iapValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --iap-id is required")
				return shared.MissingRequiredUsageError()
			}
			if !*confirm {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required")
				return shared.MissingRequiredUsageError()
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("iap submit: %w", err)
			}

			iapValue, err = resolveIAPLookupIDWithTimeout(ctx, client, *appID, iapValue)
			if err != nil {
				return err
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.CreateInAppPurchaseSubmission(requestCtx, iapValue) //nolint:staticcheck // Compatibility path retained during the App Store Connect API 4.4.1 deprecation window.
			if err != nil {
				return fmt.Errorf("iap submit: failed to submit: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}, "asc iap submit", `asc review items add --submission "SUBMISSION_ID" --item-type inAppPurchaseVersions --item-id "IAP_VERSION_ID"`)
}
