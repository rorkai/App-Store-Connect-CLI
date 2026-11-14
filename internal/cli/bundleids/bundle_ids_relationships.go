package bundleids

import (
	"context"
	"flag"
	"fmt"
	"net/url"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// newBundleIDsAppCommand returns the bundle ID app command group.
func newBundleIDsAppCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("app", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "app",
		ShortUsage: "asc bundle-ids app <subcommand> [flags]",
		ShortHelp:  "View the app linked to a bundle ID.",
		LongHelp: `View the app linked to a bundle ID.

Examples:
  asc bundle-ids app view --id "BUNDLE_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newBundleIDsAppGetCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newBundleIDsAppGetCommand returns the bundle ID app get subcommand.
func newBundleIDsAppGetCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		Deps:        d,
		FlagSetName: "view",
		Name:        "view",
		ShortUsage:  "asc bundle-ids app view --id \"BUNDLE_ID\"",
		ShortHelp:   "View the app linked to a bundle ID.",
		LongHelp: `View the app linked to a bundle ID.

Examples:
  asc bundle-ids app view --id "BUNDLE_ID"`,
		IDUsage:            "Bundle ID",
		IDType:             "bundleIds",
		RequiredDiagnostic: true,
		ErrorPrefix:        "bundle-ids app view",
		FetchErrorPrefix:   "bundle-ids app view: failed to fetch",
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.GetBundleIDApp(ctx, id)
		},
	})
}

// newBundleIDsProfilesCommand returns the bundle ID profiles command group.
func newBundleIDsProfilesCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("profiles", flag.ExitOnError)

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
			newBundleIDsProfilesListCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newBundleIDsProfilesListCommand returns the bundle ID profiles list subcommand.
func newBundleIDsProfilesListCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("list", flag.ExitOnError)

	id := shared.BindResourceIDFlag(fs, "id", "bundleIds", "Bundle ID")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := d.BindOutputFlags(fs)

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
				return d.RequiredFlag("--id")
			}
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return d.UsageError("bundle-ids profiles list: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return d.UsageErrorf("bundle-ids profiles list: %v", err)
			}
			if idValue == "" && strings.TrimSpace(*next) != "" {
				derivedID, err := extractBundleIDFromNextURL(*next)
				if err != nil {
					return d.UsageErrorf("bundle-ids profiles list: %v", err)
				}
				idValue = derivedID
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("bundle-ids profiles list: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			opts := []asc.BundleIDProfilesOption{
				asc.Limit[asc.BundleIDProfilesOption](*limit),
				asc.NextURL[asc.BundleIDProfilesOption](*next),
			}

			if *paginate {
				if idValue == "" {
					return d.RequiredFlag("--id")
				}
				paginateOpts := append(opts, asc.Limit[asc.BundleIDProfilesOption](200))
				firstPage, err := client.GetBundleIDProfiles(requestCtx, idValue, paginateOpts...)
				if err != nil {
					return fmt.Errorf("bundle-ids profiles list: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetBundleIDProfiles(ctx, idValue, asc.NextURL[asc.BundleIDProfilesOption](nextURL))
				})
				if err != nil {
					return fmt.Errorf("bundle-ids profiles list: %w", err)
				}

				return d.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := client.GetBundleIDProfiles(requestCtx, idValue, opts...)
			if err != nil {
				return fmt.Errorf("bundle-ids profiles list: failed to fetch: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
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
