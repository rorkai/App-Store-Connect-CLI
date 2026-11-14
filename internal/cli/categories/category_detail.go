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
	fs := d.NewFlagSet("categories view", flag.ExitOnError)

	categoryID := shared.BindResourceIDFlag(fs, "category-id", "appCategories", "App category ID")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "view",
		ShortUsage: "asc categories view --category-id \"CATEGORY_ID\"",
		ShortHelp:  "View an App Store category by ID.",
		LongHelp: `View an App Store category by ID.

Examples:
  asc categories view --category-id "GAMES"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedID := strings.TrimSpace(*categoryID)
			if trimmedID == "" {
				return d.RequiredFlag("--category-id")
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("categories view: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			resp, err := client.GetAppCategory(requestCtx, trimmedID)
			if err != nil {
				return fmt.Errorf("categories view: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
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
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
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
			if err := shared.ValidateNextURL(*next); err != nil {
				return d.UsageErrorf("categories subcategories: %v", err)
			}
			if err := rejectCategorySubcategoriesNextFlagConflicts(d, fs, *next, "category-id", "limit"); err != nil {
				return err
			}
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				fmt.Fprintln(d.IO.ErrOut, "Error: --limit must be between 1 and 200")
				return flag.ErrHelp
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

func rejectCategorySubcategoriesNextFlagConflicts(d *shared.Deps, fs *flag.FlagSet, next string, names ...string) error {
	if strings.TrimSpace(next) == "" {
		return nil
	}
	provided := make(map[string]struct{}, len(names))
	fs.Visit(func(f *flag.Flag) {
		provided[f.Name] = struct{}{}
	})
	for _, name := range names {
		if _, ok := provided[name]; ok {
			return d.UsageErrorf("categories subcategories: --next cannot be combined with --%s", name)
		}
	}
	return nil
}
