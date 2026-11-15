package reviews

import (
	"context"
	"flag"
	"fmt"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func newReviewsRespondCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("respond", flag.ExitOnError)

	reviewID := shared.BindResourceIDFlag(fs, "review-id", "customerReviews", "Customer review ID (required)")
	response := fs.String("response", "", "Response body text (required)")
	output := d.BindOutputFlags(fs)

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
				return d.RequiredFlag("--review-id")
			}
			if strings.TrimSpace(*response) == "" {
				return d.RequiredFlag("--response")
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("reviews respond: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			resp, err := client.CreateCustomerReviewResponse(requestCtx, strings.TrimSpace(*reviewID), strings.TrimSpace(*response))
			if err != nil {
				return fmt.Errorf("reviews respond: failed to create response: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

func newReviewsResponseCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("response", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "response",
		ShortUsage: "asc reviews response <subcommand> [flags]",
		ShortHelp:  "Manage customer review responses.",
		LongHelp: `Manage customer review responses.

Examples:
  asc reviews response view --id "RESPONSE_ID"
  asc reviews response delete --id "RESPONSE_ID" --confirm
  asc reviews response for-review --review-id "REVIEW_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newReviewsResponseGetCommand(d),
			newReviewsResponseDeleteCommand(d),
			newReviewsResponseForReviewCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

func newReviewsResponseGetCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		Deps:       d,
		Name:       "view",
		ShortUsage: "asc reviews response view [flags]",
		ShortHelp:  "View a customer review response by ID.",
		LongHelp: `View a customer review response by ID.

Examples:
  asc reviews response view --id "RESPONSE_ID"
  asc reviews response view --id "RESPONSE_ID" --output table`,
		IDUsage:            "Customer review response ID (required)",
		IDType:             "customerReviewResponses",
		RequiredDiagnostic: true,
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.GetCustomerReviewResponse(ctx, id)
		},
	})
}

func newReviewsResponseDeleteCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildConfirmDeleteCommand(shared.ConfirmDeleteCommandConfig{
		Deps:       d,
		Name:       "delete",
		ShortUsage: "asc reviews response delete [flags]",
		ShortHelp:  "Delete a customer review response.",
		LongHelp: `Delete a customer review response.

This action removes your response from the review and cannot be undone.

Examples:
  asc reviews response delete --id "RESPONSE_ID" --confirm`,
		IDFlag:             "id",
		IDUsage:            "Customer review response ID (required)",
		IDType:             "customerReviewResponses",
		RequiredDiagnostic: true,
		Delete: func(ctx context.Context, client *asc.Client, id string) error {
			return client.DeleteCustomerReviewResponse(ctx, id)
		},
		Result: func(id string) any {
			return &asc.DeleteResult{ID: id, Deleted: true}
		},
	})
}

func newReviewsResponseForReviewCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("for-review", flag.ExitOnError)

	reviewID := shared.BindResourceIDFlag(fs, "review-id", "customerReviews", "Customer review ID (required)")
	output := d.BindOutputFlags(fs)

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
				return d.RequiredFlag("--review-id")
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("reviews response for-review: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			reviewIDValue := strings.TrimSpace(*reviewID)
			resp, err := client.GetCustomerReviewResponseForReview(requestCtx, reviewIDValue)
			if err != nil {
				if asc.IsNotFound(err) {
					if _, reviewErr := client.GetCustomerReview(requestCtx, reviewIDValue); reviewErr != nil {
						return fmt.Errorf("reviews response for-review: failed to fetch: %w", reviewErr)
					}
					message := reviewResponseNotConfiguredMessage(reviewIDValue)
					warnNotConfigured(d, message)
					result := reviewResponseNotConfiguredResult{
						ReviewID:   reviewIDValue,
						Configured: false,
						Message:    message,
					}
					return d.PrintOutputWithRenderers(
						result,
						*output.Output,
						*output.Pretty,
						func() error {
							renderNotConfiguredState(d, "Review Response", "reviewId", reviewIDValue, message, false)
							return nil
						},
						func() error {
							renderNotConfiguredState(d, "Review Response", "reviewId", reviewIDValue, message, true)
							return nil
						},
					)
				}
				return fmt.Errorf("reviews response for-review: failed to fetch: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}
