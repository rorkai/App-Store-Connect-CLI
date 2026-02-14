package categories

import (
	"context"
	"flag"
	"fmt"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// CategoriesCommand returns the categories command with subcommands.
func CategoriesCommand() *ffcli.Command {
	fs := flag.NewFlagSet("categories", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "categories",
		ShortUsage: "asc categories <subcommand> [flags]",
		ShortHelp:  "Manage App Store categories.",
		LongHelp: `Manage App Store categories.

Examples:
  asc categories list
  asc categories get --category-id "GAMES"
  asc categories parent --category-id "GAMES"
  asc categories subcategories --category-id "GAMES"
  asc categories set --app APP_ID --primary GAMES`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			CategoriesListCommand(),
			CategoriesGetCommand(),
			CategoriesParentCommand(),
			CategoriesSubcategoriesCommand(),
			CategoriesSetCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// CategoriesListCommand returns the categories list subcommand.
func CategoriesListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("categories list", flag.ExitOnError)

	limit := fs.Int("limit", 200, "Maximum results to fetch (1-200)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc categories list [flags]",
		ShortHelp:  "List available App Store categories.",
		LongHelp: `List available App Store categories.

Category IDs can be used when updating app information to set primary
and secondary categories.

Examples:
  asc categories list
  asc categories list --output table`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit < 1 || *limit > 200 {
				return fmt.Errorf("categories list: --limit must be between 1 and 200")
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("categories list: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			categories, err := client.GetAppCategories(requestCtx, asc.WithAppCategoriesLimit(*limit))
			if err != nil {
				return fmt.Errorf("categories list: %w", err)
			}

			return shared.PrintOutput(categories, *output.Output, *output.Pretty)
		},
	}
}

// CategoriesSetCommand returns the categories set subcommand.
func CategoriesSetCommand() *ffcli.Command {
	return shared.NewCategoriesSetCommand(shared.CategoriesSetCommandConfig{
		FlagSetName: "categories set",
		ShortUsage:  "asc categories set --app APP_ID --primary CATEGORY_ID [--secondary CATEGORY_ID] [--app-info APP_INFO_ID]",
		ShortHelp:   "Set primary and secondary categories for an app.",
		LongHelp: `Set the primary and secondary categories for an app.

Use 'asc categories list' to find valid category IDs.

Note: The app must have an editable version in PREPARE_FOR_SUBMISSION state.

Examples:
  asc categories set --app 123456789 --primary GAMES
  asc categories set --app 123456789 --primary GAMES --secondary ENTERTAINMENT
  asc categories set --app 123456789 --primary PHOTO_AND_VIDEO`,
		ErrorPrefix:    "categories set",
		IncludeAppInfo: true,
	})
}
