package iap

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"
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
  asc iap content get --iap-id "IAP_ID"`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
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
	fs := flag.NewFlagSet("content get", flag.ExitOnError)

	iapID := fs.String("iap-id", "", "In-app purchase ID")
	contentID := fs.String("content-id", "", "In-app purchase content ID")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc iap content get --iap-id \"IAP_ID\"",
		ShortHelp:  "Get in-app purchase content metadata.",
		LongHelp: `Get in-app purchase content metadata.

Examples:
  asc iap content get --iap-id "IAP_ID"`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			iapValue := strings.TrimSpace(*iapID)
			contentValue := strings.TrimSpace(*contentID)
			if iapValue == "" && contentValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --iap-id or --content-id is required")
				return flag.ErrHelp
			}
			if iapValue != "" && contentValue != "" {
				fmt.Fprintln(os.Stderr, "Error: --iap-id and --content-id are mutually exclusive")
				return flag.ErrHelp
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("iap content get: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			if contentValue != "" {
				resp, err := client.GetInAppPurchaseContentByID(requestCtx, contentValue)
				if err != nil {
					return fmt.Errorf("iap content get: failed to fetch: %w", err)
				}

				return printOutput(resp, *output, *pretty)
			}

			resp, err := client.GetInAppPurchaseContent(requestCtx, iapValue)
			if err != nil {
				return fmt.Errorf("iap content get: failed to fetch: %w", err)
			}

			return printOutput(resp, *output, *pretty)
		},
	}
}
