package bundleids

import (
	"context"
	"flag"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// BundleIDsAppCommand returns the bundle ID app command group.
func BundleIDsAppCommand() *ffcli.Command {
	fs := flag.NewFlagSet("app", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "app",
		ShortUsage: "asc bundle-ids app <subcommand> [flags]",
		ShortHelp:  "View the app linked to a bundle ID.",
		LongHelp: `View the app linked to a bundle ID.

Examples:
  asc bundle-ids app get --id "BUNDLE_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			BundleIDsAppGetCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// BundleIDsAppGetCommand returns the bundle ID app get subcommand.
func BundleIDsAppGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("get", flag.ExitOnError)

	id := fs.String("id", "", "Bundle ID")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc bundle-ids app get --id \"BUNDLE_ID\"",
		ShortHelp:  "Get the app linked to a bundle ID.",
		LongHelp: `Get the app linked to a bundle ID.

Examples:
  asc bundle-ids app get --id "BUNDLE_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			idValue := strings.TrimSpace(*id)
			if idValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("bundle-ids app get: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetBundleIDApp(requestCtx, idValue)
			if err != nil {
				return fmt.Errorf("bundle-ids app get: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// BundleIDsProfilesCommand returns the bundle ID profiles command group.
func BundleIDsProfilesCommand() *ffcli.Command {
	fs := flag.NewFlagSet("profiles", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "profiles",
		ShortUsage: "asc bundle-ids profiles <subcommand> [flags]",
		ShortHelp:  "List profiles linked to a bundle ID.",
		LongHelp: `List profiles linked to a bundle ID.

Examples:
  asc bundle-ids profiles list --id "BUNDLE_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			BundleIDsProfilesListCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// BundleIDsProfilesListCommand returns the bundle ID profiles list subcommand.
func BundleIDsProfilesListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("list", flag.ExitOnError)

	id := fs.String("id", "", "Bundle ID")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc bundle-ids profiles list --id \"BUNDLE_ID\" [flags]",
		ShortHelp:  "List profiles linked to a bundle ID.",
		LongHelp: `List profiles linked to a bundle ID.

Examples:
  asc bundle-ids profiles list --id "BUNDLE_ID"
  asc bundle-ids profiles list --id "BUNDLE_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			idValue := strings.TrimSpace(*id)
			if idValue == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("bundle-ids profiles list: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("bundle-ids profiles list: %w", err)
			}
			if idValue == "" && strings.TrimSpace(*next) != "" {
				derivedID, err := extractBundleIDFromNextURL(*next)
				if err != nil {
					return fmt.Errorf("bundle-ids profiles list: %w", err)
				}
				idValue = derivedID
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("bundle-ids profiles list: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.BundleIDProfilesOption{
				asc.WithBundleIDProfilesLimit(*limit),
				asc.WithBundleIDProfilesNextURL(*next),
			}

			if *paginate {
				if idValue == "" {
					fmt.Fprintln(os.Stderr, "Error: --id is required")
					return flag.ErrHelp
				}
				paginateOpts := append(opts, asc.WithBundleIDProfilesLimit(200))
				firstPage, err := client.GetBundleIDProfiles(requestCtx, idValue, paginateOpts...)
				if err != nil {
					return fmt.Errorf("bundle-ids profiles list: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetBundleIDProfiles(ctx, idValue, asc.WithBundleIDProfilesNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("bundle-ids profiles list: %w", err)
				}

				return shared.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := client.GetBundleIDProfiles(requestCtx, idValue, opts...)
			if err != nil {
				return fmt.Errorf("bundle-ids profiles list: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

func extractBundleIDFromNextURL(nextURL string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(nextURL))
	if err != nil {
		return "", fmt.Errorf("invalid --next URL")
	}
	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(parts) < 4 || parts[0] != "v1" || parts[1] != "bundleIds" {
		return "", fmt.Errorf("invalid --next URL")
	}
	if strings.TrimSpace(parts[2]) == "" {
		return "", fmt.Errorf("invalid --next URL")
	}
	if parts[3] == "profiles" {
		return parts[2], nil
	}
	if len(parts) >= 5 && parts[3] == "relationships" && parts[4] == "profiles" {
		return parts[2], nil
	}
	return "", fmt.Errorf("invalid --next URL")
}
