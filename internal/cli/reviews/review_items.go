package reviews

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

// ReviewItemsGetCommand returns the review items get subcommand.
func ReviewItemsGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("items-get", flag.ExitOnError)

	itemID := fs.String("id", "", "Review submission item ID (required)")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "items-get",
		ShortUsage: "asc review items-get --id \"ITEM_ID\" [flags]",
		ShortHelp:  "Get a review submission item by ID.",
		LongHelp: `Get a review submission item by ID.

Examples:
  asc review items-get --id "ITEM_ID"`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedID := strings.TrimSpace(*itemID)
			if trimmedID == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("review items-get: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetReviewSubmissionItem(requestCtx, trimmedID)
			if err != nil {
				return fmt.Errorf("review items-get: %w", err)
			}

			return printOutput(resp, *output, *pretty)
		},
	}
}

// ReviewItemsListCommand returns the review items list subcommand.
func ReviewItemsListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("items-list", flag.ExitOnError)

	submissionID := fs.String("submission", "", "Review submission ID (required)")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Next page URL from a previous response")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "items-list",
		ShortUsage: "asc review items-list [flags]",
		ShortHelp:  "List items in a review submission.",
		LongHelp: `List items in a review submission.

Examples:
  asc review items-list --submission "SUBMISSION_ID"
  asc review items-list --submission "SUBMISSION_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("review items-list: --limit must be between 1 and 200")
			}
			if err := validateNextURL(*next); err != nil {
				return fmt.Errorf("review items-list: %w", err)
			}
			if strings.TrimSpace(*submissionID) == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --submission is required")
				return flag.ErrHelp
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("review items-list: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			opts := []asc.ReviewSubmissionItemsOption{
				asc.WithReviewSubmissionItemsLimit(*limit),
				asc.WithReviewSubmissionItemsNextURL(*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithReviewSubmissionItemsLimit(200))
				firstPage, err := client.GetReviewSubmissionItems(requestCtx, strings.TrimSpace(*submissionID), paginateOpts...)
				if err != nil {
					return fmt.Errorf("review items-list: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetReviewSubmissionItems(ctx, strings.TrimSpace(*submissionID), asc.WithReviewSubmissionItemsNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("review items-list: %w", err)
				}

				return printOutput(resp, *output, *pretty)
			}

			resp, err := client.GetReviewSubmissionItems(requestCtx, strings.TrimSpace(*submissionID), opts...)
			if err != nil {
				return fmt.Errorf("review items-list: %w", err)
			}

			return printOutput(resp, *output, *pretty)
		},
	}
}

// ReviewItemsAddCommand returns the review items add subcommand.
func ReviewItemsAddCommand() *ffcli.Command {
	fs := flag.NewFlagSet("items-add", flag.ExitOnError)

	submissionID := fs.String("submission", "", "Review submission ID (required)")
	itemType := fs.String("item-type", "", "Item type: appStoreVersions, appCustomProductPages, appEvents, appStoreVersionExperiments, appStoreVersionExperimentTreatments (required)")
	itemID := fs.String("item-id", "", "Item ID (required)")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "items-add",
		ShortUsage: "asc review items-add [flags]",
		ShortHelp:  "Add an item to a review submission.",
		LongHelp: `Add an item to a review submission.

Examples:
  asc review items-add --submission "SUBMISSION_ID" --item-type appStoreVersions --item-id "VERSION_ID"`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if strings.TrimSpace(*submissionID) == "" {
				fmt.Fprintln(os.Stderr, "Error: --submission is required")
				return flag.ErrHelp
			}
			if strings.TrimSpace(*itemType) == "" {
				fmt.Fprintln(os.Stderr, "Error: --item-type is required")
				return flag.ErrHelp
			}
			if strings.TrimSpace(*itemID) == "" {
				fmt.Fprintln(os.Stderr, "Error: --item-id is required")
				return flag.ErrHelp
			}

			normalizedType, err := normalizeReviewSubmissionItemType(*itemType)
			if err != nil {
				return fmt.Errorf("review items-add: %w", err)
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("review items-add: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			resp, err := client.CreateReviewSubmissionItem(requestCtx, strings.TrimSpace(*submissionID), normalizedType, strings.TrimSpace(*itemID))
			if err != nil {
				return fmt.Errorf("review items-add: %w", err)
			}

			return printOutput(resp, *output, *pretty)
		},
	}
}

// ReviewItemsUpdateCommand returns the review items update subcommand.
func ReviewItemsUpdateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("items-update", flag.ExitOnError)

	itemID := fs.String("id", "", "Review submission item ID (required)")
	state := fs.String("state", "", "Item state: READY_FOR_REVIEW, ACCEPTED, APPROVED, REJECTED, REMOVED (required)")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "items-update",
		ShortUsage: "asc review items-update --id \"ITEM_ID\" --state READY_FOR_REVIEW [flags]",
		ShortHelp:  "Update a review submission item.",
		LongHelp: `Update a review submission item.

Examples:
  asc review items-update --id "ITEM_ID" --state READY_FOR_REVIEW`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedID := strings.TrimSpace(*itemID)
			if trimmedID == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}
			if strings.TrimSpace(*state) == "" {
				fmt.Fprintln(os.Stderr, "Error: --state is required")
				return flag.ErrHelp
			}

			normalizedState, err := normalizeReviewSubmissionItemState(*state)
			if err != nil {
				return fmt.Errorf("review items-update: %w", err)
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("review items-update: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			attrs := asc.ReviewSubmissionItemUpdateAttributes{
				State: &normalizedState,
			}
			resp, err := client.UpdateReviewSubmissionItem(requestCtx, trimmedID, attrs)
			if err != nil {
				return fmt.Errorf("review items-update: %w", err)
			}

			return printOutput(resp, *output, *pretty)
		},
	}
}

// ReviewItemsRemoveCommand returns the review items remove subcommand.
func ReviewItemsRemoveCommand() *ffcli.Command {
	fs := flag.NewFlagSet("items-remove", flag.ExitOnError)

	itemID := fs.String("id", "", "Review submission item ID (required)")
	confirm := fs.Bool("confirm", false, "Confirm removal (required)")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "items-remove",
		ShortUsage: "asc review items-remove [flags]",
		ShortHelp:  "Remove an item from a review submission.",
		LongHelp: `Remove an item from a review submission.

Examples:
  asc review items-remove --id "ITEM_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if !*confirm {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required to remove")
				return flag.ErrHelp
			}
			if strings.TrimSpace(*itemID) == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("review items-remove: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			if err := client.DeleteReviewSubmissionItem(requestCtx, strings.TrimSpace(*itemID)); err != nil {
				return fmt.Errorf("review items-remove: %w", err)
			}

			result := &asc.ReviewSubmissionItemDeleteResult{
				ID:      strings.TrimSpace(*itemID),
				Deleted: true,
			}

			return printOutput(result, *output, *pretty)
		},
	}
}

func normalizeReviewSubmissionItemType(value string) (asc.ReviewSubmissionItemType, error) {
	normalized := strings.TrimSpace(value)
	if normalized == "" {
		return "", fmt.Errorf("--item-type is required")
	}
	if itemType, ok := reviewSubmissionItemTypes[normalized]; ok {
		return itemType, nil
	}
	return "", fmt.Errorf("--item-type must be one of: %s", strings.Join(reviewSubmissionItemTypeList(), ", "))
}

func reviewSubmissionItemTypeList() []string {
	return []string{
		"appStoreVersions",
		"appCustomProductPages",
		"appEvents",
		"appStoreVersionExperiments",
		"appStoreVersionExperimentTreatments",
	}
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

var reviewSubmissionItemTypes = map[string]asc.ReviewSubmissionItemType{
	"appStoreVersions":                    asc.ReviewSubmissionItemTypeAppStoreVersion,
	"appCustomProductPages":               asc.ReviewSubmissionItemTypeAppCustomProductPage,
	"appEvents":                           asc.ReviewSubmissionItemTypeAppEvent,
	"appStoreVersionExperiments":          asc.ReviewSubmissionItemTypeAppStoreVersionExperiment,
	"appStoreVersionExperimentTreatments": asc.ReviewSubmissionItemTypeAppStoreVersionExperimentTreatment,
}

var reviewSubmissionItemStates = map[string]struct{}{
	"READY_FOR_REVIEW": {},
	"ACCEPTED":         {},
	"APPROVED":         {},
	"REJECTED":         {},
	"REMOVED":          {},
}
