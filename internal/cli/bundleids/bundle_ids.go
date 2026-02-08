package bundleids

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

// BundleIDsCommand returns the bundle IDs command with subcommands.
func BundleIDsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("bundle-ids", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "bundle-ids",
		ShortUsage: "asc bundle-ids <subcommand> [flags]",
		ShortHelp:  "Manage bundle IDs and capabilities.",
		LongHelp: `Manage bundle IDs and capabilities.

Examples:
  asc bundle-ids list
  asc bundle-ids get --id "BUNDLE_ID"
  asc bundle-ids app get --id "BUNDLE_ID"
  asc bundle-ids profiles list --id "BUNDLE_ID"
  asc bundle-ids create --identifier "com.example.app" --name "Example" --platform IOS
  asc bundle-ids update --id "BUNDLE_ID" --name "New Name"
  asc bundle-ids delete --id "BUNDLE_ID" --confirm
  asc bundle-ids capabilities list --bundle "BUNDLE_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			BundleIDsListCommand(),
			BundleIDsGetCommand(),
			BundleIDsAppCommand(),
			BundleIDsProfilesCommand(),
			BundleIDsCreateCommand(),
			BundleIDsUpdateCommand(),
			BundleIDsDeleteCommand(),
			BundleIDsCapabilitiesCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// BundleIDsListCommand returns the bundle IDs list subcommand.
func BundleIDsListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("list", flag.ExitOnError)

	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc bundle-ids list [flags]",
		ShortHelp:  "List bundle IDs.",
		LongHelp: `List bundle IDs.

Examples:
  asc bundle-ids list
  asc bundle-ids list --limit 10
  asc bundle-ids list --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("bundle-ids list: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("bundle-ids list: %w", err)
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("bundle-ids list: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.BundleIDsOption{
				asc.WithBundleIDsLimit(*limit),
				asc.WithBundleIDsNextURL(*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithBundleIDsLimit(200))
				firstPage, err := client.GetBundleIDs(requestCtx, paginateOpts...)
				if err != nil {
					return fmt.Errorf("bundle-ids list: failed to fetch: %w", err)
				}

				paginated, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetBundleIDs(ctx, asc.WithBundleIDsNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("bundle-ids list: %w", err)
				}

				return shared.PrintOutput(paginated, *output, *pretty)
			}

			resp, err := client.GetBundleIDs(requestCtx, opts...)
			if err != nil {
				return fmt.Errorf("bundle-ids list: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}

// BundleIDsGetCommand returns the bundle IDs get subcommand.
func BundleIDsGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("get", flag.ExitOnError)

	id := fs.String("id", "", "Bundle ID")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc bundle-ids get --id \"BUNDLE_ID\"",
		ShortHelp:  "Get a bundle ID by ID.",
		LongHelp: `Get a bundle ID by ID.

Examples:
  asc bundle-ids get --id "BUNDLE_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if strings.TrimSpace(*id) == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("bundle-ids get: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetBundleID(requestCtx, strings.TrimSpace(*id))
			if err != nil {
				return fmt.Errorf("bundle-ids get: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}

// BundleIDsCreateCommand returns the bundle IDs create subcommand.
func BundleIDsCreateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("create", flag.ExitOnError)

	identifier := fs.String("identifier", "", "Bundle ID identifier (e.g., com.example.app)")
	name := fs.String("name", "", "Bundle ID name")
	platform := fs.String("platform", "IOS", "Platform: "+strings.Join(shared.PlatformList(), ", "))
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc bundle-ids create --identifier \"com.example.app\" --name \"Example\" [--platform IOS]",
		ShortHelp:  "Create a bundle ID.",
		LongHelp: `Create a bundle ID.

Examples:
  asc bundle-ids create --identifier "com.example.app" --name "Example" --platform IOS`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			identifierValue := strings.TrimSpace(*identifier)
			if identifierValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --identifier is required")
				return flag.ErrHelp
			}
			nameValue := strings.TrimSpace(*name)
			if nameValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --name is required")
				return flag.ErrHelp
			}
			platformValue, err := shared.NormalizePlatform(*platform)
			if err != nil {
				return fmt.Errorf("bundle-ids create: %w", err)
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("bundle-ids create: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			attrs := asc.BundleIDCreateAttributes{
				Name:       nameValue,
				Identifier: identifierValue,
				Platform:   platformValue,
			}
			resp, err := client.CreateBundleID(requestCtx, attrs)
			if err != nil {
				return fmt.Errorf("bundle-ids create: failed to create: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}

// BundleIDsUpdateCommand returns the bundle IDs update subcommand.
func BundleIDsUpdateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("update", flag.ExitOnError)

	id := fs.String("id", "", "Bundle ID")
	name := fs.String("name", "", "Bundle ID name")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "update",
		ShortUsage: "asc bundle-ids update --id \"BUNDLE_ID\" --name \"New Name\"",
		ShortHelp:  "Update a bundle ID.",
		LongHelp: `Update a bundle ID.

Examples:
  asc bundle-ids update --id "BUNDLE_ID" --name "New Name"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			idValue := strings.TrimSpace(*id)
			if idValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}
			nameValue := strings.TrimSpace(*name)
			if nameValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --name is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("bundle-ids update: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			attrs := asc.BundleIDUpdateAttributes{Name: nameValue}
			resp, err := client.UpdateBundleID(requestCtx, idValue, attrs)
			if err != nil {
				return fmt.Errorf("bundle-ids update: failed to update: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}

// BundleIDsDeleteCommand returns the bundle IDs delete subcommand.
func BundleIDsDeleteCommand() *ffcli.Command {
	fs := flag.NewFlagSet("delete", flag.ExitOnError)

	id := fs.String("id", "", "Bundle ID")
	confirm := fs.Bool("confirm", false, "Confirm deletion")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "delete",
		ShortUsage: "asc bundle-ids delete --id \"BUNDLE_ID\" --confirm",
		ShortHelp:  "Delete a bundle ID.",
		LongHelp: `Delete a bundle ID.

Examples:
  asc bundle-ids delete --id "BUNDLE_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			idValue := strings.TrimSpace(*id)
			if idValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}
			if !*confirm {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("bundle-ids delete: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			if err := client.DeleteBundleID(requestCtx, idValue); err != nil {
				return fmt.Errorf("bundle-ids delete: failed to delete: %w", err)
			}

			result := &asc.BundleIDDeleteResult{
				ID:      idValue,
				Deleted: true,
			}

			return shared.PrintOutput(result, *output, *pretty)
		},
	}
}
