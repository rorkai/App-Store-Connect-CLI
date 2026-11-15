package productpages

import (
	"context"
	"flag"

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
		UsageFunc: d.DefaultUsageFunc,
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
	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		FlagSetName: "custom-page-versions view",
		Name:        "view",
		ShortUsage:  "asc product-pages custom-pages versions view --custom-page-version-id \"VERSION_ID\"",
		ShortHelp:   "View a custom product page version by ID.",
		LongHelp: `View a custom product page version by ID.

Examples:
  asc product-pages custom-pages versions view --custom-page-version-id "VERSION_ID"`,
		IDFlag:             "custom-page-version-id",
		IDUsage:            "Custom product page version ID",
		IDType:             "appCustomProductPageVersions",
		RequiredDiagnostic: true,
		ErrorPrefix:        "custom-pages versions view",
		Deps:               d,
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.GetAppCustomProductPageVersion(ctx, id)
		},
	})
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
