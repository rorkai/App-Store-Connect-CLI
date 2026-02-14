package marketplace

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

// MarketplaceSearchDetailsCommand returns the marketplace search details command group.
func MarketplaceSearchDetailsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("search-details", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "search-details",
		ShortUsage: "asc marketplace search-details <subcommand> [flags]",
		ShortHelp:  "Manage marketplace search details.",
		LongHelp: `Manage marketplace search details.

Examples:
  asc marketplace search-details get --app "APP_ID"
  asc marketplace search-details create --app "APP_ID" --catalog-url "https://example.com"
  asc marketplace search-details update --search-detail-id "DETAIL_ID" --catalog-url "https://example.com"
  asc marketplace search-details delete --search-detail-id "DETAIL_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			MarketplaceSearchDetailsGetCommand(),
			MarketplaceSearchDetailsCreateCommand(),
			MarketplaceSearchDetailsUpdateCommand(),
			MarketplaceSearchDetailsDeleteCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// MarketplaceSearchDetailsGetCommand returns the search details get subcommand.
func MarketplaceSearchDetailsGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("get", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	fields := fs.String("fields", "", "Fields to include: "+strings.Join(marketplaceSearchDetailFieldsList(), ", "))
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc marketplace search-details get --app \"APP_ID\" [flags]",
		ShortHelp:  "Get marketplace search details for an app.",
		LongHelp: `Get marketplace search details for an app.

Examples:
  asc marketplace search-details get --app "APP_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			resolvedAppID := shared.ResolveAppID(*appID)
			if resolvedAppID == "" {
				fmt.Fprintln(os.Stderr, "Error: --app is required (or set ASC_APP_ID)")
				return flag.ErrHelp
			}

			fieldsValue, err := normalizeMarketplaceSearchDetailFields(*fields)
			if err != nil {
				return fmt.Errorf("marketplace search-details get: %w", err)
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("marketplace search-details get: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			detail, err := client.GetMarketplaceSearchDetailForApp(requestCtx, resolvedAppID, fieldsValue)
			if err != nil {
				return fmt.Errorf("marketplace search-details get: failed to fetch: %w", err)
			}

			return shared.PrintOutput(detail, *output.Output, *output.Pretty)
		},
	}
}

// MarketplaceSearchDetailsCreateCommand returns the search details create subcommand.
func MarketplaceSearchDetailsCreateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("create", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	catalogURL := fs.String("catalog-url", "", "Marketplace catalog URL")
	output := shared.BindOutputFlags(fs)

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
				fmt.Fprintln(os.Stderr, "Error: --app is required (or set ASC_APP_ID)")
				return flag.ErrHelp
			}

			catalogURLValue := strings.TrimSpace(*catalogURL)
			if catalogURLValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --catalog-url is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("marketplace search-details create: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			detail, err := client.CreateMarketplaceSearchDetail(requestCtx, resolvedAppID, catalogURLValue)
			if err != nil {
				return fmt.Errorf("marketplace search-details create: failed to create: %w", err)
			}

			return shared.PrintOutput(detail, *output.Output, *output.Pretty)
		},
	}
}

// MarketplaceSearchDetailsUpdateCommand returns the search details update subcommand.
func MarketplaceSearchDetailsUpdateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("update", flag.ExitOnError)

	detailID := fs.String("search-detail-id", "", "Marketplace search detail ID")
	catalogURL := fs.String("catalog-url", "", "Marketplace catalog URL")
	output := shared.BindOutputFlags(fs)

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
				fmt.Fprintln(os.Stderr, "Error: --search-detail-id is required")
				return flag.ErrHelp
			}

			visited := map[string]bool{}
			fs.Visit(func(f *flag.Flag) {
				visited[f.Name] = true
			})

			if !visited["catalog-url"] {
				fmt.Fprintln(os.Stderr, "Error: at least one update flag is required")
				return flag.ErrHelp
			}

			attrs := asc.MarketplaceSearchDetailUpdateAttributes{}
			if visited["catalog-url"] {
				value := strings.TrimSpace(*catalogURL)
				attrs.CatalogURL = &value
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("marketplace search-details update: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			detail, err := client.UpdateMarketplaceSearchDetail(requestCtx, trimmedID, attrs)
			if err != nil {
				return fmt.Errorf("marketplace search-details update: failed to update: %w", err)
			}

			return shared.PrintOutput(detail, *output.Output, *output.Pretty)
		},
	}
}

// MarketplaceSearchDetailsDeleteCommand returns the search details delete subcommand.
func MarketplaceSearchDetailsDeleteCommand() *ffcli.Command {
	fs := flag.NewFlagSet("delete", flag.ExitOnError)

	detailID := fs.String("search-detail-id", "", "Marketplace search detail ID")
	confirm := fs.Bool("confirm", false, "Confirm deletion")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "delete",
		ShortUsage: "asc marketplace search-details delete --search-detail-id \"DETAIL_ID\" --confirm",
		ShortHelp:  "Delete marketplace search details.",
		LongHelp: `Delete marketplace search details.

Examples:
  asc marketplace search-details delete --search-detail-id "DETAIL_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedID := strings.TrimSpace(*detailID)
			if trimmedID == "" {
				fmt.Fprintln(os.Stderr, "Error: --search-detail-id is required")
				return flag.ErrHelp
			}
			if !*confirm {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("marketplace search-details delete: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			if err := client.DeleteMarketplaceSearchDetail(requestCtx, trimmedID); err != nil {
				return fmt.Errorf("marketplace search-details delete: failed to delete: %w", err)
			}

			result := &asc.MarketplaceSearchDetailDeleteResult{
				ID:      trimmedID,
				Deleted: true,
			}

			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
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
