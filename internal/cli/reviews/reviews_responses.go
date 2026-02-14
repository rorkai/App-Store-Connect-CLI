package reviews

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// ReviewsRespondCommand returns the reviews respond subcommand.
func ReviewsRespondCommand() *ffcli.Command {
	fs := flag.NewFlagSet("respond", flag.ExitOnError)

	reviewID := fs.String("review-id", "", "Customer review ID (required)")
	response := fs.String("response", "", "Response body text (required)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "respond",
		ShortUsage: "asc reviews respond [flags]",
		ShortHelp:  "Create a response to a customer review.",
		LongHelp: `Create a response to a customer review.

This command creates a developer response to a customer review on the App Store.
Responses are visible to all App Store users.

Examples:
  asc reviews respond --review-id "REVIEW_ID" --response "Thanks for your feedback!"
  asc reviews respond --review-id "REVIEW_ID" --response "We appreciate your review." --output table`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if strings.TrimSpace(*reviewID) == "" {
				fmt.Fprintln(os.Stderr, "Error: --review-id is required")
				return flag.ErrHelp
			}
			if strings.TrimSpace(*response) == "" {
				fmt.Fprintln(os.Stderr, "Error: --response is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("reviews respond: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.CreateCustomerReviewResponse(requestCtx, strings.TrimSpace(*reviewID), strings.TrimSpace(*response))
			if err != nil {
				return fmt.Errorf("reviews respond: failed to create response: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// ReviewsResponseCommand returns the reviews response parent command.
func ReviewsResponseCommand() *ffcli.Command {
	fs := flag.NewFlagSet("response", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "response",
		ShortUsage: "asc reviews response <subcommand> [flags]",
		ShortHelp:  "Manage customer review responses.",
		LongHelp: `Manage customer review responses.

Examples:
  asc reviews response get --id "RESPONSE_ID"
  asc reviews response delete --id "RESPONSE_ID" --confirm
  asc reviews response for-review --review-id "REVIEW_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			ReviewsResponseGetCommand(),
			ReviewsResponseDeleteCommand(),
			ReviewsResponseForReviewCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// ReviewsResponseGetCommand returns the reviews response get subcommand.
func ReviewsResponseGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("get", flag.ExitOnError)

	responseID := fs.String("id", "", "Customer review response ID (required)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc reviews response get [flags]",
		ShortHelp:  "Get a customer review response by ID.",
		LongHelp: `Get a customer review response by ID.

Examples:
  asc reviews response get --id "RESPONSE_ID"
  asc reviews response get --id "RESPONSE_ID" --output table`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if strings.TrimSpace(*responseID) == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("reviews response get: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetCustomerReviewResponse(requestCtx, strings.TrimSpace(*responseID))
			if err != nil {
				return fmt.Errorf("reviews response get: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// ReviewsResponseDeleteCommand returns the reviews response delete subcommand.
func ReviewsResponseDeleteCommand() *ffcli.Command {
	fs := flag.NewFlagSet("delete", flag.ExitOnError)

	responseID := fs.String("id", "", "Customer review response ID (required)")
	confirm := fs.Bool("confirm", false, "Confirm deletion")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "delete",
		ShortUsage: "asc reviews response delete [flags]",
		ShortHelp:  "Delete a customer review response.",
		LongHelp: `Delete a customer review response.

This action removes your response from the review and cannot be undone.

Examples:
  asc reviews response delete --id "RESPONSE_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if strings.TrimSpace(*responseID) == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}
			if !*confirm {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("reviews response delete: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			if err := client.DeleteCustomerReviewResponse(requestCtx, strings.TrimSpace(*responseID)); err != nil {
				return fmt.Errorf("reviews response delete: failed to delete: %w", err)
			}

			result := &asc.CustomerReviewResponseDeleteResult{
				ID:      strings.TrimSpace(*responseID),
				Deleted: true,
			}

			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

// ReviewsResponseForReviewCommand returns the reviews response for-review subcommand.
func ReviewsResponseForReviewCommand() *ffcli.Command {
	fs := flag.NewFlagSet("for-review", flag.ExitOnError)

	reviewID := fs.String("review-id", "", "Customer review ID (required)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "for-review",
		ShortUsage: "asc reviews response for-review [flags]",
		ShortHelp:  "Get the response for a specific review.",
		LongHelp: `Get the developer response for a specific customer review.

This command fetches the existing response (if any) for a given review ID.

Examples:
  asc reviews response for-review --review-id "REVIEW_ID"
  asc reviews response for-review --review-id "REVIEW_ID" --output table`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if strings.TrimSpace(*reviewID) == "" {
				fmt.Fprintln(os.Stderr, "Error: --review-id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("reviews response for-review: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetCustomerReviewResponseForReview(requestCtx, strings.TrimSpace(*reviewID))
			if err != nil {
				return fmt.Errorf("reviews response for-review: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}
