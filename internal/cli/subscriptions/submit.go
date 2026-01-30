package subscriptions

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

// SubscriptionsSubmitCommand returns the subscriptions submit subcommand.
func SubscriptionsSubmitCommand() *ffcli.Command {
	fs := flag.NewFlagSet("submit", flag.ExitOnError)

	subscriptionID := fs.String("subscription-id", "", "Subscription ID")
	confirm := fs.Bool("confirm", false, "Confirm submission")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "submit",
		ShortUsage: "asc subscriptions submit --subscription-id \"SUB_ID\" --confirm",
		ShortHelp:  "Submit a subscription for review.",
		LongHelp: `Submit a subscription for review.

Examples:
  asc subscriptions submit --subscription-id "SUB_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*subscriptionID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --subscription-id is required")
				return flag.ErrHelp
			}
			if !*confirm {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required")
				return flag.ErrHelp
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("subscriptions submit: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			resp, err := client.CreateSubscriptionSubmission(requestCtx, id)
			if err != nil {
				return fmt.Errorf("subscriptions submit: failed to submit: %w", err)
			}

			return printOutput(resp, *output, *pretty)
		},
	}
}

// SubscriptionsGroupsSubmitCommand returns the group submit subcommand.
func SubscriptionsGroupsSubmitCommand() *ffcli.Command {
	fs := flag.NewFlagSet("groups submit", flag.ExitOnError)

	groupID := fs.String("group-id", "", "Subscription group ID")
	confirm := fs.Bool("confirm", false, "Confirm submission")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "submit",
		ShortUsage: "asc subscriptions groups submit --group-id \"GROUP_ID\" --confirm",
		ShortHelp:  "Submit a subscription group for review.",
		LongHelp: `Submit a subscription group for review.

Examples:
  asc subscriptions groups submit --group-id "GROUP_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*groupID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --group-id is required")
				return flag.ErrHelp
			}
			if !*confirm {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required")
				return flag.ErrHelp
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("subscriptions groups submit: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			resp, err := client.CreateSubscriptionGroupSubmission(requestCtx, id)
			if err != nil {
				return fmt.Errorf("subscriptions groups submit: failed to submit: %w", err)
			}

			return printOutput(resp, *output, *pretty)
		},
	}
}
