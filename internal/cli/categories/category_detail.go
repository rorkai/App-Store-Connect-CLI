package categories

import (
	"context"
	"flag"
	"fmt"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// newCategoriesGetCommand returns the category get subcommand.
func newCategoriesGetCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		FlagSetName: "categories view",
		Name:        "view",
		ShortUsage:  "asc categories view --category-id \"CATEGORY_ID\"",
		ShortHelp:   "View an App Store category by ID.",
		LongHelp: `View an App Store category by ID.

Examples:
  asc categories view --category-id "GAMES"`,
		IDFlag:             "category-id",
		IDUsage:            "App category ID",
		IDType:             "appCategories",
		RequiredDiagnostic: true,
		FetchErrorPrefix:   "categories view",
		Deps:               d,
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.GetAppCategory(ctx, id)
		},
	})
}

// newCategoriesParentCommand returns the category parent subcommand.
func newCategoriesParentCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("categories parent", flag.ExitOnError)

	categoryID := shared.BindResourceIDFlag(fs, "category-id", "appCategories", "App category ID")
	output := d.BindOutputFlags(fs)

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
				return d.RequiredFlag("--category-id")
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("categories parent: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			resp, err := client.GetAppCategoryParent(requestCtx, trimmedID)
			if err != nil {
				return fmt.Errorf("categories parent: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// newCategoriesSubcategoriesCommand returns the category subcategories subcommand.
func newCategoriesSubcategoriesCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("categories subcategories", flag.ExitOnError)

	categoryID := shared.BindResourceIDFlag(fs, "category-id", "appCategories", "App category ID")
	limit := fs.Int("limit", 0, shared.LimitFlagUsage(200))
	next := fs.String("next", "", shared.NextFlagUsage)
	paginate := fs.Bool("paginate", false, shared.PaginateFlagUsage)
	output := d.BindOutputFlags(fs)

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
			if err := d.ValidatePaginationFlags(ctx, fs, "categories subcategories", 200, "limit"); err != nil {
				return err
			}

			trimmedID := strings.TrimSpace(*categoryID)
			trimmedNext := strings.TrimSpace(*next)
			if trimmedID == "" && trimmedNext == "" {
				return d.RequiredFlag("--category-id")
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("categories subcategories: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			opts := []asc.AppCategoriesOption{
				asc.WithAppCategoriesLimit(*limit),
				asc.NextURL[asc.AppCategoriesOption](*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithAppCategoriesLimit(200))
				firstPage, err := client.GetAppCategorySubcategories(requestCtx, trimmedID, paginateOpts...)
				if err != nil {
					return fmt.Errorf("categories subcategories: failed to fetch: %w", err)
				}
				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetAppCategorySubcategories(ctx, trimmedID, asc.NextURL[asc.AppCategoriesOption](nextURL))
				})
				if err != nil {
					return fmt.Errorf("categories subcategories: %w", err)
				}
				return d.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := client.GetAppCategorySubcategories(requestCtx, trimmedID, opts...)
			if err != nil {
				return fmt.Errorf("categories subcategories: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}
