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

// CustomPageVersionsCommand returns the custom page versions command group.
func CustomPageVersionsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("versions", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "versions",
		ShortUsage: "asc product-pages custom-pages versions <subcommand> [flags]",
		ShortHelp:  "Manage custom product page versions.",
		LongHelp: `Manage custom product page versions.

Examples:
  asc product-pages custom-pages versions list --custom-page-id "PAGE_ID"
  asc product-pages custom-pages versions create --custom-page-id "PAGE_ID" --deep-link "https://example.com"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			CustomPageVersionsListCommand(),
			CustomPageVersionsGetCommand(),
			CustomPageVersionsCreateCommand(),
			CustomPageVersionsUpdateCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// CustomPageVersionsListCommand returns the custom page versions list subcommand.
func CustomPageVersionsListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("custom-page-versions list", flag.ExitOnError)

	customPageID := fs.String("custom-page-id", "", "Custom product page ID")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc product-pages custom-pages versions list --custom-page-id \"PAGE_ID\" [flags]",
		ShortHelp:  "List custom product page versions.",
		LongHelp: `List custom product page versions.

Examples:
  asc product-pages custom-pages versions list --custom-page-id "PAGE_ID"
  asc product-pages custom-pages versions list --custom-page-id "PAGE_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > productPagesMaxLimit) {
				return fmt.Errorf("custom-pages versions list: --limit must be between 1 and %d", productPagesMaxLimit)
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("custom-pages versions list: %w", err)
			}

			trimmedID := strings.TrimSpace(*customPageID)
			if trimmedID == "" {
				fmt.Fprintln(os.Stderr, "Error: --custom-page-id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("custom-pages versions list: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.AppCustomProductPageVersionsOption{
				asc.WithAppCustomProductPageVersionsLimit(*limit),
				asc.WithAppCustomProductPageVersionsNextURL(*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithAppCustomProductPageVersionsLimit(productPagesMaxLimit))
				firstPage, err := client.GetAppCustomProductPageVersions(requestCtx, trimmedID, paginateOpts...)
				if err != nil {
					return fmt.Errorf("custom-pages versions list: failed to fetch: %w", err)
				}

				paginated, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetAppCustomProductPageVersions(ctx, trimmedID, asc.WithAppCustomProductPageVersionsNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("custom-pages versions list: %w", err)
				}

				return shared.PrintOutput(paginated, *output.Output, *output.Pretty)
			}

			resp, err := client.GetAppCustomProductPageVersions(requestCtx, trimmedID, opts...)
			if err != nil {
				return fmt.Errorf("custom-pages versions list: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// CustomPageVersionsGetCommand returns the custom page versions get subcommand.
func CustomPageVersionsGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("custom-page-versions get", flag.ExitOnError)

	versionID := fs.String("custom-page-version-id", "", "Custom product page version ID")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc product-pages custom-pages versions get --custom-page-version-id \"VERSION_ID\"",
		ShortHelp:  "Get a custom product page version by ID.",
		LongHelp: `Get a custom product page version by ID.

Examples:
  asc product-pages custom-pages versions get --custom-page-version-id "VERSION_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedID := strings.TrimSpace(*versionID)
			if trimmedID == "" {
				fmt.Fprintln(os.Stderr, "Error: --custom-page-version-id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("custom-pages versions get: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetAppCustomProductPageVersion(requestCtx, trimmedID)
			if err != nil {
				return fmt.Errorf("custom-pages versions get: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// CustomPageVersionsCreateCommand returns the custom page versions create subcommand.
func CustomPageVersionsCreateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("custom-page-versions create", flag.ExitOnError)

	customPageID := fs.String("custom-page-id", "", "Custom product page ID")
	deepLink := fs.String("deep-link", "", "Deep link URL")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc product-pages custom-pages versions create --custom-page-id \"PAGE_ID\" [--deep-link \"URL\"]",
		ShortHelp:  "Create a custom product page version.",
		LongHelp: `Create a custom product page version.

Examples:
  asc product-pages custom-pages versions create --custom-page-id "PAGE_ID"
  asc product-pages custom-pages versions create --custom-page-id "PAGE_ID" --deep-link "https://example.com"`,
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
				return fmt.Errorf("custom-pages versions create: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.CreateAppCustomProductPageVersion(requestCtx, trimmedID, strings.TrimSpace(*deepLink))
			if err != nil {
				return fmt.Errorf("custom-pages versions create: failed to create: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// CustomPageVersionsUpdateCommand returns the custom page versions update subcommand.
func CustomPageVersionsUpdateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("custom-page-versions update", flag.ExitOnError)

	versionID := fs.String("custom-page-version-id", "", "Custom product page version ID")
	deepLink := fs.String("deep-link", "", "Update deep link URL")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "update",
		ShortUsage: "asc product-pages custom-pages versions update --custom-page-version-id \"VERSION_ID\" --deep-link \"URL\"",
		ShortHelp:  "Update a custom product page version.",
		LongHelp: `Update a custom product page version.

Examples:
  asc product-pages custom-pages versions update --custom-page-version-id "VERSION_ID" --deep-link "https://example.com"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedID := strings.TrimSpace(*versionID)
			if trimmedID == "" {
				fmt.Fprintln(os.Stderr, "Error: --custom-page-version-id is required")
				return flag.ErrHelp
			}

			deepLinkValue := strings.TrimSpace(*deepLink)
			if deepLinkValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --deep-link is required")
				return flag.ErrHelp
			}

			attrs := asc.AppCustomProductPageVersionUpdateAttributes{
				DeepLink: &deepLinkValue,
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("custom-pages versions update: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.UpdateAppCustomProductPageVersion(requestCtx, trimmedID, attrs)
			if err != nil {
				return fmt.Errorf("custom-pages versions update: failed to update: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}
