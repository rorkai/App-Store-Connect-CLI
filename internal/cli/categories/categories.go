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

// NewCategoriesCommand returns the categories command with subcommands.
func NewCategoriesCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("categories", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "categories",
		ShortUsage: "asc categories <subcommand> [flags]",
		ShortHelp:  "Manage App Store categories.",
		LongHelp: `Manage App Store categories.

Examples:
  asc categories list
  asc categories view --category-id "GAMES"
  asc categories parent --category-id "GAMES"
  asc categories subcategories --category-id "GAMES"
  asc categories set --app APP_ID --primary GAMES`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newCategoriesListCommand(d),
			newCategoriesGetCommand(d),
			newCategoriesParentCommand(d),
			newCategoriesSubcategoriesCommand(d),
			newCategoriesSetCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newCategoriesListCommand returns the categories list subcommand.
func newCategoriesListCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("categories list", flag.ExitOnError)

	limit := fs.Int("limit", 200, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc categories list [flags]",
		ShortHelp:  "List available App Store categories.",
		LongHelp: `List available App Store categories.

Category IDs can be used when updating app information to set primary
and secondary categories.

Examples:
  asc categories list
  asc categories list --output table
  asc categories list --paginate
  asc categories list --next "<links.next>"
  asc categories list --paginate --next "<links.next>"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := shared.ValidateNextURL(*next); err != nil {
				return d.UsageErrorf("categories list: %v", err)
			}
			if err := rejectCategoriesListNextFlagConflicts(d, fs, *next); err != nil {
				return err
			}
			if *limit < 1 || *limit > 200 {
				return d.UsageErrorf("categories list: --limit must be between 1 and 200")
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("categories list: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			opts := []asc.AppCategoriesOption{
				asc.WithAppCategoriesLimit(*limit),
				asc.NextURL[asc.AppCategoriesOption](*next),
			}

			if *paginate {
				firstPage, err := client.GetAppCategories(requestCtx, opts...)
				if err != nil {
					return fmt.Errorf("categories list: %w", err)
				}
				categories, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetAppCategories(ctx, asc.NextURL[asc.AppCategoriesOption](nextURL))
				})
				if err != nil {
					return fmt.Errorf("categories list: %w", err)
				}
				return d.PrintOutput(categories, *output.Output, *output.Pretty)
			}

			categories, err := client.GetAppCategories(requestCtx, opts...)
			if err != nil {
				return fmt.Errorf("categories list: %w", err)
			}

			return d.PrintOutput(categories, *output.Output, *output.Pretty)
		},
	}
}

// rejectCategoriesListNextFlagConflicts rejects --limit when a --next cursor
// URL is supplied, because the cursor already encodes the page size and the
// CLI must never accept and silently ignore a flag.
func rejectCategoriesListNextFlagConflicts(d *shared.Deps, fs *flag.FlagSet, next string) error {
	if strings.TrimSpace(next) == "" {
		return nil
	}
	conflict := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "limit" {
			conflict = true
		}
	})
	if conflict {
		return d.UsageError("categories list: --next cannot be combined with --limit")
	}
	return nil
}

// newCategoriesSetCommand returns the categories set subcommand.
func newCategoriesSetCommand(d *shared.Deps) *ffcli.Command {
	return shared.NewCategoriesSetCommand(shared.CategoriesSetCommandConfig{
		Deps:        d,
		FlagSetName: "categories set",
		ShortUsage:  "asc categories set --app APP_ID --primary CATEGORY_ID [--secondary CATEGORY_ID] [flags]",
		ShortHelp:   "Set primary and secondary categories for an app.",
		LongHelp: `Set the primary and secondary categories for an app.

Use 'asc categories list' to find valid category IDs.
Use 'asc categories subcategories --category-id GAMES' to find valid subcategory IDs.

Note: The app must have an editable version in PREPARE_FOR_SUBMISSION state.

Examples:
  asc categories set --app 123456789 --primary GAMES
  asc categories set --app 123456789 --primary GAMES --secondary ENTERTAINMENT
  asc categories set --app 123456789 --primary GAMES --primary-subcategory-one GAMES_ACTION --primary-subcategory-two GAMES_SIMULATION
  asc categories set --app 123456789 --primary GAMES --primary-subcategory-one GAMES_ACTION --secondary ENTERTAINMENT
  asc categories set --app 123456789 --primary PHOTO_AND_VIDEO`,
		ErrorPrefix:    "categories set",
		IncludeAppInfo: true,
	})
}
