package marketplace

import (
	"context"
	"flag"
	"fmt"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// newMarketplaceSearchDetailsCommand returns the marketplace search details command group.
func newMarketplaceSearchDetailsCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("search-details", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "search-details",
		ShortUsage: "asc marketplace search-details <subcommand> [flags]",
		ShortHelp:  "Manage marketplace search details.",
		LongHelp: `Manage marketplace search details.

Examples:
  asc marketplace search-details view --app "APP_ID"
  asc marketplace search-details create --app "APP_ID" --catalog-url "https://example.com"
  asc marketplace search-details update --search-detail-id "DETAIL_ID" --catalog-url "https://example.com"
  asc marketplace search-details delete --search-detail-id "DETAIL_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newMarketplaceSearchDetailsGetCommand(d),
			newMarketplaceSearchDetailsCreateCommand(d),
			newMarketplaceSearchDetailsUpdateCommand(d),
			newMarketplaceSearchDetailsDeleteCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newMarketplaceSearchDetailsGetCommand returns the search details get subcommand.
func newMarketplaceSearchDetailsGetCommand(d *shared.Deps) *ffcli.Command {
	var fieldsValue []string

	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		FlagSetName: "view",
		Name:        "view",
		ShortUsage:  "asc marketplace search-details view --app \"APP_ID\" [flags]",
		ShortHelp:   "View marketplace search details for an app.",
		LongHelp: `View marketplace search details for an app.

Examples:
  asc marketplace search-details view --app "APP_ID"`,
		IDFlag:             "app",
		IDUsage:            "App Store Connect app ID (or ASC_APP_ID env)",
		ResolveApp:         true,
		RequiredDiagnostic: true,
		Flags: func(fs *flag.FlagSet) func() error {
			fields := fs.String("fields", "", "Fields to include: "+strings.Join(marketplaceSearchDetailFieldsList(), ", "))
			return func() (err error) {
				if fieldsValue, err = normalizeMarketplaceSearchDetailFields(*fields); err != nil {
					return d.UsageErrorf("marketplace search-details view: %v", err)
				}
				return nil
			}
		},
		ErrorPrefix:      "marketplace search-details view",
		FetchErrorPrefix: "marketplace search-details view: failed to fetch",
		Deps:             d,
		Fetch: func(ctx context.Context, client *asc.Client, appID string) (any, error) {
			return client.GetMarketplaceSearchDetailForApp(ctx, appID, fieldsValue)
		},
	})
}

// newMarketplaceSearchDetailsCreateCommand returns the search details create subcommand.
func newMarketplaceSearchDetailsCreateCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("create", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	catalogURL := fs.String("catalog-url", "", "Marketplace catalog URL")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc marketplace search-details create --app \"APP_ID\" --catalog-url \"URL\" [flags]",
		ShortHelp:  "Create marketplace search details for an app.",
		LongHelp: `Create marketplace search details for an app.

Examples:
  asc marketplace search-details create --app "APP_ID" --catalog-url "https://example.com"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			resolvedAppID := shared.ResolveAppID(*appID)
			if resolvedAppID == "" {
				fmt.Fprintln(d.IO.ErrOut, "Error: --app is required (or set ASC_APP_ID)")
				return shared.MissingRequiredUsageError("--app")
			}

			catalogURLValue := strings.TrimSpace(*catalogURL)
			if catalogURLValue == "" {
				return d.RequiredFlag("--catalog-url")
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("marketplace search-details create: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			detail, err := client.CreateMarketplaceSearchDetail(requestCtx, resolvedAppID, catalogURLValue)
			if err != nil {
				return fmt.Errorf("marketplace search-details create: failed to create: %w", err)
			}

			return d.PrintOutput(detail, *output.Output, *output.Pretty)
		},
	}
}

// newMarketplaceSearchDetailsUpdateCommand returns the search details update subcommand.
func newMarketplaceSearchDetailsUpdateCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("update", flag.ExitOnError)

	detailID := shared.BindResourceIDFlag(fs, "search-detail-id", "marketplaceSearchDetails", "Marketplace search detail ID")
	catalogURL := fs.String("catalog-url", "", "Marketplace catalog URL")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "update",
		ShortUsage: "asc marketplace search-details update --search-detail-id \"DETAIL_ID\" --catalog-url \"URL\" [flags]",
		ShortHelp:  "Update marketplace search details.",
		LongHelp: `Update marketplace search details.

Examples:
  asc marketplace search-details update --search-detail-id "DETAIL_ID" --catalog-url "https://example.com"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedID := strings.TrimSpace(*detailID)
			if trimmedID == "" {
				return d.RequiredFlag("--search-detail-id")
			}

			visited := map[string]bool{}
			fs.Visit(func(f *flag.Flag) {
				visited[f.Name] = true
			})

			if !visited["catalog-url"] {
				fmt.Fprintln(d.IO.ErrOut, "Error: at least one update flag is required")
				return shared.MissingRequiredUsageError("")
			}

			attrs := asc.MarketplaceSearchDetailUpdateAttributes{}
			if visited["catalog-url"] {
				value := strings.TrimSpace(*catalogURL)
				attrs.CatalogURL = &value
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("marketplace search-details update: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			detail, err := client.UpdateMarketplaceSearchDetail(requestCtx, trimmedID, attrs)
			if err != nil {
				return fmt.Errorf("marketplace search-details update: failed to update: %w", err)
			}

			return d.PrintOutput(detail, *output.Output, *output.Pretty)
		},
	}
}

// newMarketplaceSearchDetailsDeleteCommand returns the search details delete subcommand.
func newMarketplaceSearchDetailsDeleteCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildConfirmDeleteCommand(shared.ConfirmDeleteCommandConfig{
		FlagSetName: "delete",
		Name:        "delete",
		ShortUsage:  "asc marketplace search-details delete --search-detail-id \"DETAIL_ID\" --confirm",
		ShortHelp:   "Delete marketplace search details.",
		LongHelp: `Delete marketplace search details.

Examples:
  asc marketplace search-details delete --search-detail-id "DETAIL_ID" --confirm`,
		IDFlag:             "search-detail-id",
		IDUsage:            "Marketplace search detail ID",
		IDType:             "marketplaceSearchDetails",
		RequiredDiagnostic: true,
		ErrorPrefix:        "marketplace search-details delete",
		DeleteErrorPrefix:  "marketplace search-details delete: failed to delete",
		Deps:               d,
		Delete: func(ctx context.Context, client *asc.Client, id string) error {
			return client.DeleteMarketplaceSearchDetail(ctx, id)
		},
		Result: func(id string) any {
			return &asc.MarketplaceSearchDetailDeleteResult{ID: id, Deleted: true}
		},
	})
}

func normalizeMarketplaceSearchDetailFields(value string) ([]string, error) {
	fields := shared.SplitCSV(value)
	if len(fields) == 0 {
		return nil, nil
	}
	allowed := map[string]struct{}{}
	for _, field := range marketplaceSearchDetailFieldsList() {
		allowed[field] = struct{}{}
	}
	for _, field := range fields {
		if _, ok := allowed[field]; !ok {
			return nil, fmt.Errorf("--fields must be one of: %s", strings.Join(marketplaceSearchDetailFieldsList(), ", "))
		}
	}
	return fields, nil
}

func marketplaceSearchDetailFieldsList() []string {
	return []string{"catalogUrl"}
}
