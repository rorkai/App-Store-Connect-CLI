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
	return shared.BuildPaginatedListCommand(shared.PaginatedListCommandConfig{
		FlagSetName: "custom-page-versions list",
		Name:        "list",
		ShortUsage:  "asc product-pages custom-pages versions list --custom-page-id \"PAGE_ID\" [flags]",
		ShortHelp:   "List custom product page versions.",
		LongHelp: `List custom product page versions.

Examples:
  asc product-pages custom-pages versions list --custom-page-id "PAGE_ID"
  asc product-pages custom-pages versions list --custom-page-id "PAGE_ID" --paginate`,
		ParentFlag:           "custom-page-id",
		ParentUsage:          "Custom product page ID",
		ParentType:           "appCustomProductPages",
		RequiredDiagnostic:   true,
		LimitMax:             productPagesMaxLimit,
		PaginateIgnoresLimit: true,
		NoSpinner:            true,
		ErrorPrefix:          "custom-pages versions list",
		Deps:                 d,
		FetchPage: func(ctx context.Context, client *asc.Client, id string, limit int, next string) (asc.PaginatedResponse, error) {
			return client.GetAppCustomProductPageVersions(ctx, id,
				asc.Limit[asc.AppCustomProductPageVersionsOption](limit),
				asc.NextURL[asc.AppCustomProductPageVersionsOption](next),
			)
		},
	})
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
	fs := d.NewFlagSet("custom-page-versions create", flag.ExitOnError)

	customPageID := shared.BindResourceIDFlag(fs, "custom-page-id", "appCustomProductPages", "Custom product page ID")
	deepLink := fs.String("deep-link", "", "Deep link URL")
	output := d.BindOutputFlags(fs)

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
				return d.RequiredFlag("--custom-page-id")
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("custom-pages versions create: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			resp, err := client.CreateAppCustomProductPageVersion(requestCtx, trimmedID, strings.TrimSpace(*deepLink))
			if err != nil {
				return fmt.Errorf("custom-pages versions create: failed to create: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// newCustomPageVersionsUpdateCommand returns the custom page versions update subcommand.
func newCustomPageVersionsUpdateCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("custom-page-versions update", flag.ExitOnError)

	versionID := shared.BindResourceIDFlag(fs, "custom-page-version-id", "appCustomProductPageVersions", "Custom product page version ID")
	deepLink := fs.String("deep-link", "", "Update deep link URL")
	output := d.BindOutputFlags(fs)

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
				return d.RequiredFlag("--custom-page-version-id")
			}

			deepLinkValue := strings.TrimSpace(*deepLink)
			if deepLinkValue == "" {
				return d.RequiredFlag("--deep-link")
			}

			attrs := asc.AppCustomProductPageVersionUpdateAttributes{
				DeepLink: &deepLinkValue,
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("custom-pages versions update: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			resp, err := client.UpdateAppCustomProductPageVersion(requestCtx, trimmedID, attrs)
			if err != nil {
				return fmt.Errorf("custom-pages versions update: failed to update: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}
