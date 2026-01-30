package iap

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"
)

// IAPSubmitCommand returns the submit subcommand.
func IAPSubmitCommand() *ffcli.Command {
	fs := flag.NewFlagSet("submit", flag.ExitOnError)

	iapID := fs.String("iap-id", "", "In-app purchase ID")
	confirm := fs.Bool("confirm", false, "Confirm submission")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "submit",
		ShortUsage: "asc iap submit --iap-id \"IAP_ID\" --confirm",
		ShortHelp:  "Submit an in-app purchase for review.",
		LongHelp: `Submit an in-app purchase for review.

Examples:
  asc iap submit --iap-id "IAP_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			iapValue := strings.TrimSpace(*iapID)
			if iapValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --iap-id is required")
				return flag.ErrHelp
			}
			if !*confirm {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required")
				return flag.ErrHelp
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("iap submit: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			resp, err := client.CreateInAppPurchaseSubmission(requestCtx, iapValue)
			if err != nil {
				return fmt.Errorf("iap submit: failed to submit: %w", err)
			}

			return printOutput(resp, *output, *pretty)
		},
	}
}
