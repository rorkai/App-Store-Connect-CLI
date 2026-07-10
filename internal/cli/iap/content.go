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

// IAPContentCommand returns the content command group.
func IAPContentCommand() *ffcli.Command {
	fs := flag.NewFlagSet("content", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "content",
		ShortUsage: "asc iap content <subcommand> [flags]",
		ShortHelp:  "Fetch in-app purchase content metadata.",
		LongHelp: `Fetch in-app purchase content metadata.

Examples:
  asc iap content view --iap-id "IAP_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			IAPContentGetCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// IAPContentGetCommand returns the content get subcommand.
func IAPContentGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("content view", flag.ExitOnError)

	appID := addIAPLookupAppFlag(fs)
	iapID := fs.String("iap-id", "", "In-app purchase ID, product ID, or exact current name")
	contentID := fs.String("content-id", "", "In-app purchase content ID")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "view",
		ShortUsage: "asc iap content view --iap-id \"IAP_ID\"",
		ShortHelp:  "View in-app purchase content metadata.",
		LongHelp: `View in-app purchase content metadata.

Examples:
  asc iap content view --iap-id "IAP_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			iapValue := strings.TrimSpace(*iapID)
			contentValue := strings.TrimSpace(*contentID)
			if iapValue == "" && contentValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --iap-id or --content-id is required")
				return shared.MissingRequiredUsageError()
			}
			if iapValue != "" && contentValue != "" {
				fmt.Fprintln(os.Stderr, "Error: --iap-id and --content-id are mutually exclusive")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("iap content view: %w", err)
			}

			if contentValue != "" {
				requestCtx, cancel := shared.ContextWithTimeout(ctx)
				defer cancel()

				resp, err := client.GetInAppPurchaseContentByID(requestCtx, contentValue)
				if err != nil {
					return fmt.Errorf("iap content view: failed to fetch: %w", err)
				}

				return shared.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			iapValue, err = resolveIAPLookupIDWithTimeout(ctx, client, *appID, iapValue)
			if err != nil {
				return err
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetInAppPurchaseContent(requestCtx, iapValue)
			if err != nil {
				return fmt.Errorf("iap content view: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}
