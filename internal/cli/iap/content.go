package iap

import (
	"context"
	"flag"
	"fmt"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// newIAPContentCommand returns the content command group.
func newIAPContentCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("content", flag.ExitOnError)

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
			newIAPContentGetCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newIAPContentGetCommand returns the content get subcommand.
func newIAPContentGetCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("content view", flag.ExitOnError)

	appID := addIAPLookupAppFlag(fs)
	iapID := shared.BindResourceIDFlag(fs, "iap-id", "inAppPurchases", "In-app purchase ID, product ID, or exact current name")
	contentID := shared.BindResourceIDFlag(fs, "content-id", "inAppPurchaseContents", "In-app purchase content ID")
	iapFields := fs.String("iap-fields", "", "fields[inAppPurchases] for the included in-app purchase (comma-separated)")
	output := d.BindOutputFlags(fs)

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
				fmt.Fprintln(d.IO.ErrOut, "Error: --iap-id or --content-id is required")
				return shared.MissingRequiredUsageError("")
			}
			if iapValue != "" && contentValue != "" {
				fmt.Fprintln(d.IO.ErrOut, "Error: --iap-id and --content-id are mutually exclusive")
				return flag.ErrHelp
			}
			fieldValues, err := shared.NormalizeSelection(*iapFields, iapVersionIAPFields, "--iap-fields")
			if err != nil {
				return d.UsageError("iap content view: " + err.Error())
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("iap content view: %w", err)
			}

			if contentValue != "" {
				requestCtx, cancel := d.Timeout(ctx)
				defer cancel()

				resp, err := client.GetInAppPurchaseContentByID(requestCtx, contentValue, asc.WithIAPContentIAPFields(fieldValues))
				if err != nil {
					return fmt.Errorf("iap content view: failed to fetch: %w", err)
				}

				return d.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			iapValue, err = resolveIAPLookupIDWithTimeout(ctx, d, client, *appID, iapValue)
			if err != nil {
				return err
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			resp, err := client.GetInAppPurchaseContent(requestCtx, iapValue, asc.WithIAPContentIAPFields(fieldValues))
			if err != nil {
				return fmt.Errorf("iap content view: failed to fetch: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}
