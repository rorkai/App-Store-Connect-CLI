package categories

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

// CategoriesGetCommand returns the category get subcommand.
func CategoriesGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("categories get", flag.ExitOnError)

	categoryID := fs.String("category-id", "", "App category ID")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc categories get --category-id \"CATEGORY_ID\"",
		ShortHelp:  "Get an App Store category by ID.",
		LongHelp: `Get an App Store category by ID.

Examples:
  asc categories get --category-id "GAMES"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedID := strings.TrimSpace(*categoryID)
			if trimmedID == "" {
				fmt.Fprintln(os.Stderr, "Error: --category-id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("categories get: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetAppCategory(requestCtx, trimmedID)
			if err != nil {
				return fmt.Errorf("categories get: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// CategoriesParentCommand returns the category parent subcommand.
func CategoriesParentCommand() *ffcli.Command {
	fs := flag.NewFlagSet("categories parent", flag.ExitOnError)

	categoryID := fs.String("category-id", "", "App category ID")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "parent",
		ShortUsage: "asc categories parent --category-id \"CATEGORY_ID\"",
		ShortHelp:  "Get the parent category for a category.",
		LongHelp: `Get the parent category for a category.

Examples:
  asc categories parent --category-id "GAMES"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedID := strings.TrimSpace(*categoryID)
			if trimmedID == "" {
				fmt.Fprintln(os.Stderr, "Error: --category-id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("categories parent: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetAppCategoryParent(requestCtx, trimmedID)
			if err != nil {
				return fmt.Errorf("categories parent: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// CategoriesSubcategoriesCommand returns the category subcategories subcommand.
func CategoriesSubcategoriesCommand() *ffcli.Command {
	fs := flag.NewFlagSet("categories subcategories", flag.ExitOnError)

	categoryID := fs.String("category-id", "", "App category ID")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "subcategories",
		ShortUsage: "asc categories subcategories --category-id \"CATEGORY_ID\"",
		ShortHelp:  "List subcategories for a category.",
		LongHelp: `List subcategories for a category.

Examples:
  asc categories subcategories --category-id "GAMES"
  asc categories subcategories --category-id "GAMES" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				fmt.Fprintln(os.Stderr, "Error: --limit must be between 1 and 200")
				return flag.ErrHelp
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("categories subcategories: %w", err)
			}

			trimmedID := strings.TrimSpace(*categoryID)
			trimmedNext := strings.TrimSpace(*next)
			if trimmedID == "" && trimmedNext == "" {
				fmt.Fprintln(os.Stderr, "Error: --category-id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("categories subcategories: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.AppCategoriesOption{
				asc.WithAppCategoriesLimit(*limit),
				asc.WithAppCategoriesNextURL(*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithAppCategoriesLimit(200))
				firstPage, err := client.GetAppCategorySubcategories(requestCtx, trimmedID, paginateOpts...)
				if err != nil {
					return fmt.Errorf("categories subcategories: failed to fetch: %w", err)
				}
				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetAppCategorySubcategories(ctx, trimmedID, asc.WithAppCategoriesNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("categories subcategories: %w", err)
				}
				return shared.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := client.GetAppCategorySubcategories(requestCtx, trimmedID, opts...)
			if err != nil {
				return fmt.Errorf("categories subcategories: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}
