package productpages

import (
	"context"
	"flag"
	"fmt"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// newCustomPageVersionsCommand returns the custom page versions command group.
func newCustomPageVersionsCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("versions", flag.ExitOnError)

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
			newCustomPageVersionsListCommand(d),
			newCustomPageVersionsGetCommand(d),
			newCustomPageVersionsCreateCommand(d),
			newCustomPageVersionsUpdateCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newCustomPageVersionsListCommand returns the custom page versions list subcommand.
func newCustomPageVersionsListCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("custom-page-versions list", flag.ExitOnError)

	customPageID := shared.BindResourceIDFlag(fs, "custom-page-id", "appCustomProductPages", "Custom product page ID")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := d.BindOutputFlags(fs)

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
			if err := d.ValidatePaginationFlags(ctx, fs, "product-pages custom-pages versions list", productPagesMaxLimit); err != nil {
				return err
			}

			trimmedID := strings.TrimSpace(*customPageID)
			if trimmedID == "" && strings.TrimSpace(*next) == "" {
				return d.RequiredFlag("--custom-page-id")
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("custom-pages versions list: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			opts := []asc.AppCustomProductPageVersionsOption{
				asc.Limit[asc.AppCustomProductPageVersionsOption](*limit),
				asc.NextURL[asc.AppCustomProductPageVersionsOption](*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.Limit[asc.AppCustomProductPageVersionsOption](productPagesMaxLimit))
				firstPage, err := client.GetAppCustomProductPageVersions(requestCtx, trimmedID, paginateOpts...)
				if err != nil {
					return fmt.Errorf("custom-pages versions list: failed to fetch: %w", err)
				}

				paginated, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetAppCustomProductPageVersions(ctx, trimmedID, asc.NextURL[asc.AppCustomProductPageVersionsOption](nextURL))
				})
				if err != nil {
					return fmt.Errorf("custom-pages versions list: %w", err)
				}

				return d.PrintOutput(paginated, *output.Output, *output.Pretty)
			}

			resp, err := client.GetAppCustomProductPageVersions(requestCtx, trimmedID, opts...)
			if err != nil {
				return fmt.Errorf("custom-pages versions list: failed to fetch: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// newCustomPageVersionsGetCommand returns the custom page versions get subcommand.
func newCustomPageVersionsGetCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("custom-page-versions view", flag.ExitOnError)

	versionID := shared.BindResourceIDFlag(fs, "custom-page-version-id", "appCustomProductPageVersions", "Custom product page version ID")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "view",
		ShortUsage: "asc product-pages custom-pages versions view --custom-page-version-id \"VERSION_ID\"",
		ShortHelp:  "View a custom product page version by ID.",
		LongHelp: `View a custom product page version by ID.

Examples:
  asc product-pages custom-pages versions view --custom-page-version-id "VERSION_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedID := strings.TrimSpace(*versionID)
			if trimmedID == "" {
				return d.RequiredFlag("--custom-page-version-id")
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("custom-pages versions view: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			resp, err := client.GetAppCustomProductPageVersion(requestCtx, trimmedID)
			if err != nil {
				return fmt.Errorf("custom-pages versions view: failed to fetch: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// newCustomPageVersionsCreateCommand returns the custom page versions create subcommand.
func newCustomPageVersionsCreateCommand(d *shared.Deps) *ffcli.Command {
	var deepLink string
	return shared.BuildCreateCommand(shared.WriteCommandConfig{
		FlagSetName: "custom-page-versions create",
		Name:        "create",
		ShortUsage:  "asc product-pages custom-pages versions create --custom-page-id \"PAGE_ID\" [--deep-link \"URL\"]",
		ShortHelp:   "Create a custom product page version.",
		LongHelp: `Create a custom product page version.

Examples:
  asc product-pages custom-pages versions create --custom-page-id "PAGE_ID"
  asc product-pages custom-pages versions create --custom-page-id "PAGE_ID" --deep-link "https://example.com"`,
		ErrorPrefix: "custom-pages versions create",
		IDFlag:      "custom-page-id",
		IDUsage:     "Custom product page ID",
		IDType:      "appCustomProductPages",
		Fields: []shared.AttributeFlag{
			shared.StringFlag(&deepLink, "deep-link", "Deep link URL"),
		},
		Deps: d,
		Send: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.CreateAppCustomProductPageVersion(ctx, id, deepLink)
		},
	})
}

// newCustomPageVersionsUpdateCommand returns the custom page versions update subcommand.
func newCustomPageVersionsUpdateCommand(d *shared.Deps) *ffcli.Command {
	var deepLink string
	return shared.BuildUpdateCommand(shared.WriteCommandConfig{
		FlagSetName: "custom-page-versions update",
		Name:        "update",
		ShortUsage:  "asc product-pages custom-pages versions update --custom-page-version-id \"VERSION_ID\" --deep-link \"URL\"",
		ShortHelp:   "Update a custom product page version.",
		LongHelp: `Update a custom product page version.

Examples:
  asc product-pages custom-pages versions update --custom-page-version-id "VERSION_ID" --deep-link "https://example.com"`,
		ErrorPrefix: "custom-pages versions update",
		IDFlag:      "custom-page-version-id",
		IDUsage:     "Custom product page version ID",
		IDType:      "appCustomProductPageVersions",
		Fields: []shared.AttributeFlag{
			shared.RequiredStringFlag(&deepLink, "deep-link", "Update deep link URL"),
		},
		Deps: d,
		Send: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.UpdateAppCustomProductPageVersion(ctx, id, asc.AppCustomProductPageVersionUpdateAttributes{DeepLink: &deepLink})
		},
	})
}
