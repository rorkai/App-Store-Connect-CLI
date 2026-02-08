package productpages

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

// CustomPagesCommand returns the custom pages command group.
func CustomPagesCommand() *ffcli.Command {
	fs := flag.NewFlagSet("custom-pages", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "custom-pages",
		ShortUsage: "asc product-pages custom-pages <subcommand> [flags]",
		ShortHelp:  "Manage custom product pages.",
		LongHelp: `Manage custom product pages.

Examples:
  asc product-pages custom-pages list --app "APP_ID"
  asc product-pages custom-pages get --custom-page-id "PAGE_ID"
  asc product-pages custom-pages create --app "APP_ID" --name "Summer Campaign"
  asc product-pages custom-pages update --custom-page-id "PAGE_ID" --name "Updated"
  asc product-pages custom-pages delete --custom-page-id "PAGE_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			CustomPagesListCommand(),
			CustomPagesGetCommand(),
			CustomPagesCreateCommand(),
			CustomPagesUpdateCommand(),
			CustomPagesDeleteCommand(),
			CustomPageVersionsCommand(),
			CustomPageLocalizationsCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// CustomPagesListCommand returns the custom pages list subcommand.
func CustomPagesListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("custom-pages list", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID)")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc product-pages custom-pages list --app \"APP_ID\" [flags]",
		ShortHelp:  "List custom product pages.",
		LongHelp: `List custom product pages.

Examples:
  asc product-pages custom-pages list --app "APP_ID"
  asc product-pages custom-pages list --app "APP_ID" --limit 50
  asc product-pages custom-pages list --app "APP_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > productPagesMaxLimit) {
				return fmt.Errorf("custom-pages list: --limit must be between 1 and %d", productPagesMaxLimit)
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("custom-pages list: %w", err)
			}

			resolvedAppID := shared.ResolveAppID(*appID)
			if resolvedAppID == "" {
				fmt.Fprintln(os.Stderr, "Error: --app is required (or set ASC_APP_ID)")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("custom-pages list: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.AppCustomProductPagesOption{
				asc.WithAppCustomProductPagesLimit(*limit),
				asc.WithAppCustomProductPagesNextURL(*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithAppCustomProductPagesLimit(productPagesMaxLimit))
				firstPage, err := client.GetAppCustomProductPages(requestCtx, resolvedAppID, paginateOpts...)
				if err != nil {
					return fmt.Errorf("custom-pages list: failed to fetch: %w", err)
				}

				paginated, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetAppCustomProductPages(ctx, resolvedAppID, asc.WithAppCustomProductPagesNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("custom-pages list: %w", err)
				}

				return shared.PrintOutput(paginated, *output, *pretty)
			}

			resp, err := client.GetAppCustomProductPages(requestCtx, resolvedAppID, opts...)
			if err != nil {
				return fmt.Errorf("custom-pages list: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}

// CustomPagesGetCommand returns the custom pages get subcommand.
func CustomPagesGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("custom-pages get", flag.ExitOnError)

	customPageID := fs.String("custom-page-id", "", "Custom product page ID")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc product-pages custom-pages get --custom-page-id \"PAGE_ID\"",
		ShortHelp:  "Get a custom product page by ID.",
		LongHelp: `Get a custom product page by ID.

Examples:
  asc product-pages custom-pages get --custom-page-id "PAGE_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedID := strings.TrimSpace(*customPageID)
			if trimmedID == "" {
				fmt.Fprintln(os.Stderr, "Error: --custom-page-id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("custom-pages get: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetAppCustomProductPage(requestCtx, trimmedID)
			if err != nil {
				return fmt.Errorf("custom-pages get: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}

// CustomPagesCreateCommand returns the custom pages create subcommand.
func CustomPagesCreateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("custom-pages create", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID)")
	name := fs.String("name", "", "Custom product page name")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc product-pages custom-pages create --app \"APP_ID\" --name \"NAME\"",
		ShortHelp:  "Create a custom product page.",
		LongHelp: `Create a custom product page.

Examples:
  asc product-pages custom-pages create --app "APP_ID" --name "Summer Campaign"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			resolvedAppID := shared.ResolveAppID(*appID)
			if resolvedAppID == "" {
				fmt.Fprintln(os.Stderr, "Error: --app is required (or set ASC_APP_ID)")
				return flag.ErrHelp
			}

			nameValue := strings.TrimSpace(*name)
			if nameValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --name is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("custom-pages create: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.CreateAppCustomProductPage(requestCtx, resolvedAppID, nameValue)
			if err != nil {
				return fmt.Errorf("custom-pages create: failed to create: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}

// CustomPagesUpdateCommand returns the custom pages update subcommand.
func CustomPagesUpdateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("custom-pages update", flag.ExitOnError)

	customPageID := fs.String("custom-page-id", "", "Custom product page ID")
	name := fs.String("name", "", "Update page name")
	var visible shared.OptionalBool
	fs.Var(&visible, "visible", "Set visibility: true or false")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "update",
		ShortUsage: "asc product-pages custom-pages update --custom-page-id \"PAGE_ID\" [--name \"NAME\"] [--visible true|false]",
		ShortHelp:  "Update a custom product page.",
		LongHelp: `Update a custom product page.

Examples:
  asc product-pages custom-pages update --custom-page-id "PAGE_ID" --name "Updated"
  asc product-pages custom-pages update --custom-page-id "PAGE_ID" --visible true`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedID := strings.TrimSpace(*customPageID)
			if trimmedID == "" {
				fmt.Fprintln(os.Stderr, "Error: --custom-page-id is required")
				return flag.ErrHelp
			}

			attrs := asc.AppCustomProductPageUpdateAttributes{}
			if nameValue := strings.TrimSpace(*name); nameValue != "" {
				attrs.Name = &nameValue
			}
			if visible.IsSet() {
				value := visible.Value()
				attrs.Visible = &value
			}
			if attrs.Name == nil && attrs.Visible == nil {
				fmt.Fprintln(os.Stderr, "Error: --name or --visible is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("custom-pages update: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.UpdateAppCustomProductPage(requestCtx, trimmedID, attrs)
			if err != nil {
				return fmt.Errorf("custom-pages update: failed to update: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}

// CustomPagesDeleteCommand returns the custom pages delete subcommand.
func CustomPagesDeleteCommand() *ffcli.Command {
	fs := flag.NewFlagSet("custom-pages delete", flag.ExitOnError)

	customPageID := fs.String("custom-page-id", "", "Custom product page ID")
	confirm := fs.Bool("confirm", false, "Confirm deletion")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "delete",
		ShortUsage: "asc product-pages custom-pages delete --custom-page-id \"PAGE_ID\" --confirm",
		ShortHelp:  "Delete a custom product page.",
		LongHelp: `Delete a custom product page.

Examples:
  asc product-pages custom-pages delete --custom-page-id "PAGE_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedID := strings.TrimSpace(*customPageID)
			if trimmedID == "" {
				fmt.Fprintln(os.Stderr, "Error: --custom-page-id is required")
				return flag.ErrHelp
			}
			if !*confirm {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("custom-pages delete: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			if err := client.DeleteAppCustomProductPage(requestCtx, trimmedID); err != nil {
				return fmt.Errorf("custom-pages delete: failed to delete: %w", err)
			}

			result := &asc.AppCustomProductPageDeleteResult{ID: trimmedID, Deleted: true}
			return shared.PrintOutput(result, *output, *pretty)
		},
	}
}
