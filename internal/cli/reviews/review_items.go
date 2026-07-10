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

// ReviewItemsCommand returns the nested review items command group.
func ReviewItemsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("items", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "items",
		ShortUsage: "asc review items <subcommand> [flags]",
		ShortHelp:  "Manage review submission items.",
		LongHelp: `Manage review submission items.

Examples:
  asc review items view --id "ITEM_ID"
  asc review items list --submission "SUBMISSION_ID"
  asc review items add --submission "SUBMISSION_ID" --item-type appStoreVersions --item-id "VERSION_ID"
  asc review items update --id "ITEM_ID" --state READY_FOR_REVIEW
  asc review items remove --id "ITEM_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			reviewItemsGetCommand("view", "review items view", `asc review items view --id "ITEM_ID"`),
			reviewItemsListCommand("list", "review items list", `asc review items list [flags]`, `asc review items list --submission "SUBMISSION_ID"
  asc review items list --submission "SUBMISSION_ID" --paginate`),
			reviewItemsAddCommand("add", "review items add", `asc review items add [flags]`, `asc review items add --submission "SUBMISSION_ID" --item-type appStoreVersions --item-id "VERSION_ID"
  asc review items add --submission "SUBMISSION_ID" --item-type gameCenterChallengeVersions --item-id "VERSION_ID"`),
			reviewItemsUpdateCommand("update", "review items update", `asc review items update --id "ITEM_ID" --state READY_FOR_REVIEW [flags]`, `asc review items update --id "ITEM_ID" --state READY_FOR_REVIEW`),
			reviewItemsRemoveCommand("remove", "review items remove", `asc review items remove [flags]`, `asc review items remove --id "ITEM_ID" --confirm`),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// ReviewItemsGetCommand returns the review items view subcommand.
func ReviewItemsGetCommand() *ffcli.Command {
	return reviewItemsGetCommand("items-get", "review items-get", `asc review items-get --id "ITEM_ID"`)
}

func reviewItemsGetCommand(name, errorPrefix, example string) *ffcli.Command {
	fs := flag.NewFlagSet(name, flag.ExitOnError)
	helpVerb := "Get"
	if name == "view" {
		helpVerb = "View"
	}

	itemID := fs.String("id", "", "Review submission item ID (required)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       name,
		ShortUsage: example + " [flags]",
		ShortHelp:  helpVerb + " a review submission item by ID.",
		LongHelp: helpVerb + ` a review submission item by ID.

Examples:
  ` + example,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedID := strings.TrimSpace(*itemID)
			if trimmedID == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return shared.MissingRequiredUsageError()
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("%s: %w", errorPrefix, err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetReviewSubmissionItem(requestCtx, trimmedID)
			if err != nil {
				return fmt.Errorf("%s: %w", errorPrefix, err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// ReviewItemsListCommand returns the review items list subcommand.
func ReviewItemsListCommand() *ffcli.Command {
	return reviewItemsListCommand("items-list", "review items-list", `asc review items-list [flags]`, `asc review items-list --submission "SUBMISSION_ID"
  asc review items-list --submission "SUBMISSION_ID" --paginate`)
}

func reviewItemsListCommand(name, errorPrefix, shortUsage, examples string) *ffcli.Command {
	fs := flag.NewFlagSet(name, flag.ExitOnError)

	submissionID := fs.String("submission", "", "Review submission ID (required)")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Next page URL from a previous response")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       name,
		ShortUsage: shortUsage,
		ShortHelp:  "List items in a review submission.",
		LongHelp: `List items in a review submission.

Examples:
  ` + examples,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("%s: --limit must be between 1 and 200", errorPrefix)
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("%s: %w", errorPrefix, err)
			}
			if strings.TrimSpace(*submissionID) == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --submission is required")
				return shared.MissingRequiredUsageError()
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("%s: %w", errorPrefix, err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.ReviewSubmissionItemsOption{
				asc.WithReviewSubmissionItemsLimit(*limit),
				asc.WithReviewSubmissionItemsNextURL(*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithReviewSubmissionItemsLimit(200))
				resp, err := shared.PaginateWithSpinner(
					requestCtx,
					func(ctx context.Context) (asc.PaginatedResponse, error) {
						return client.GetReviewSubmissionItems(ctx, strings.TrimSpace(*submissionID), paginateOpts...)
					},
					func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
						return client.GetReviewSubmissionItems(ctx, strings.TrimSpace(*submissionID), asc.WithReviewSubmissionItemsNextURL(nextURL))
					},
				)
				if err != nil {
					return fmt.Errorf("%s: %w", errorPrefix, err)
				}

				return shared.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := client.GetReviewSubmissionItems(requestCtx, strings.TrimSpace(*submissionID), opts...)
			if err != nil {
				return fmt.Errorf("%s: %w", errorPrefix, err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// ReviewItemsAddCommand returns the review items add subcommand.
func ReviewItemsAddCommand() *ffcli.Command {
	return reviewItemsAddCommand("items-add", "review items-add", `asc review items-add [flags]`, `asc review items-add --submission "SUBMISSION_ID" --item-type appStoreVersions --item-id "VERSION_ID"
  asc review items-add --submission "SUBMISSION_ID" --item-type gameCenterChallengeVersions --item-id "VERSION_ID"`)
}

func reviewItemsAddCommand(name, errorPrefix, shortUsage, examples string) *ffcli.Command {
	fs := flag.NewFlagSet(name, flag.ExitOnError)

	submissionID := fs.String("submission", "", "Review submission ID (required)")
	itemTypeValues := strings.Join(reviewSubmissionItemTypeList(), ", ")
	itemType := fs.String("item-type", "", fmt.Sprintf("Item type: %s (required)", itemTypeValues))
	itemID := fs.String("item-id", "", "Item ID (required)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       name,
		ShortUsage: shortUsage,
		ShortHelp:  "Add an item to a review submission.",
		LongHelp: `Add an item to a review submission.

Examples:
  ` + examples,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if strings.TrimSpace(*submissionID) == "" {
				fmt.Fprintln(os.Stderr, "Error: --submission is required")
				return shared.MissingRequiredUsageError()
			}
			if strings.TrimSpace(*itemType) == "" {
				fmt.Fprintln(os.Stderr, "Error: --item-type is required")
				return shared.MissingRequiredUsageError()
			}
			if strings.TrimSpace(*itemID) == "" {
				fmt.Fprintln(os.Stderr, "Error: --item-id is required")
				return shared.MissingRequiredUsageError()
			}

			normalizedType, err := normalizeReviewSubmissionItemType(*itemType)
			if err != nil {
				return shared.UsageError(err.Error())
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("%s: %w", errorPrefix, err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.CreateReviewSubmissionItem(requestCtx, strings.TrimSpace(*submissionID), normalizedType, strings.TrimSpace(*itemID))
			if err != nil {
				return fmt.Errorf("%s: %w", errorPrefix, err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// ReviewItemsUpdateCommand returns the review items update subcommand.
func ReviewItemsUpdateCommand() *ffcli.Command {
	return reviewItemsUpdateCommand("items-update", "review items-update", `asc review items-update --id "ITEM_ID" --state READY_FOR_REVIEW [flags]`, `asc review items-update --id "ITEM_ID" --state READY_FOR_REVIEW`)
}

func reviewItemsUpdateCommand(name, errorPrefix, shortUsage, examples string) *ffcli.Command {
	fs := flag.NewFlagSet(name, flag.ExitOnError)

	itemID := fs.String("id", "", "Review submission item ID (required)")
	state := fs.String("state", "", "Item state: READY_FOR_REVIEW, ACCEPTED, APPROVED, REJECTED, REMOVED (required)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       name,
		ShortUsage: shortUsage,
		ShortHelp:  "Update a review submission item.",
		LongHelp: `Update a review submission item.

Examples:
  ` + examples,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedID := strings.TrimSpace(*itemID)
			if trimmedID == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return shared.MissingRequiredUsageError()
			}
			if strings.TrimSpace(*state) == "" {
				fmt.Fprintln(os.Stderr, "Error: --state is required")
				return shared.MissingRequiredUsageError()
			}

			normalizedState, err := normalizeReviewSubmissionItemState(*state)
			if err != nil {
				return fmt.Errorf("%s: %w", errorPrefix, err)
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("%s: %w", errorPrefix, err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			attrs := asc.ReviewSubmissionItemUpdateAttributes{
				State: &normalizedState,
			}
			resp, err := client.UpdateReviewSubmissionItem(requestCtx, trimmedID, attrs)
			if err != nil {
				return fmt.Errorf("%s: %w", errorPrefix, err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// ReviewItemsRemoveCommand returns the review items remove subcommand.
func ReviewItemsRemoveCommand() *ffcli.Command {
	return reviewItemsRemoveCommand("items-remove", "review items-remove", `asc review items-remove [flags]`, `asc review items-remove --id "ITEM_ID" --confirm`)
}

func reviewItemsRemoveCommand(name, errorPrefix, shortUsage, examples string) *ffcli.Command {
	fs := flag.NewFlagSet(name, flag.ExitOnError)

	itemID := fs.String("id", "", "Review submission item ID (required)")
	confirm := fs.Bool("confirm", false, "Confirm removal (required)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       name,
		ShortUsage: shortUsage,
		ShortHelp:  "Remove an item from a review submission.",
		LongHelp: `Remove an item from a review submission.

Examples:
  ` + examples,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if !*confirm {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required to remove")
				return shared.MissingRequiredUsageError()
			}
			if strings.TrimSpace(*itemID) == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return shared.MissingRequiredUsageError()
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("%s: %w", errorPrefix, err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			if err := client.DeleteReviewSubmissionItem(requestCtx, strings.TrimSpace(*itemID)); err != nil {
				return fmt.Errorf("%s: %w", errorPrefix, err)
			}

			result := &asc.ReviewSubmissionItemDeleteResult{
				ID:      strings.TrimSpace(*itemID),
				Deleted: true,
			}

			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

func normalizeReviewSubmissionItemType(value string) (asc.ReviewSubmissionItemType, error) {
	if strings.TrimSpace(value) == "" {
		return "", fmt.Errorf("--item-type is required")
	}
	if itemType, ok := asc.ParseReviewSubmissionItemType(value); ok {
		return itemType, nil
	}
	return "", fmt.Errorf("--item-type must be one of: %s", strings.Join(reviewSubmissionItemTypeList(), ", "))
}

func reviewSubmissionItemTypeList() []string {
	return asc.ReviewSubmissionItemTypeNames()
}

func normalizeReviewSubmissionItemState(value string) (string, error) {
	normalized := strings.ToUpper(strings.TrimSpace(value))
	if normalized == "" {
		return "", fmt.Errorf("--state is required")
	}
	if _, ok := reviewSubmissionItemStates[normalized]; ok {
		return normalized, nil
	}
	return "", fmt.Errorf("--state must be one of: %s", strings.Join(reviewSubmissionItemStateList(), ", "))
}

func reviewSubmissionItemStateList() []string {
	return []string{
		"READY_FOR_REVIEW",
		"ACCEPTED",
		"APPROVED",
		"REJECTED",
		"REMOVED",
	}
}

var reviewSubmissionItemStates = map[string]struct{}{
	"READY_FOR_REVIEW": {},
	"ACCEPTED":         {},
	"APPROVED":         {},
	"REJECTED":         {},
	"REMOVED":          {},
}
