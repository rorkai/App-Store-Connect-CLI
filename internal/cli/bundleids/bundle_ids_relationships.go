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
		UsageFunc: d.DefaultUsageFunc,
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
		Deps:       d,
		Name:       "view",
		ShortUsage: "asc bundle-ids app view --id \"BUNDLE_ID\"",
		ShortHelp:  "View the app linked to a bundle ID.",
		LongHelp: `View the app linked to a bundle ID.

Examples:
  asc bundle-ids app view --id "BUNDLE_ID"`,
		IDUsage:            "Bundle ID",
		IDType:             "bundleIds",
		RequiredDiagnostic: true,
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
		UsageFunc: d.DefaultUsageFunc,
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
	return shared.BuildPaginatedListCommand(shared.PaginatedListCommandConfig{
		Name:       "list",
		ShortUsage: "asc bundle-ids profiles list --id \"BUNDLE_ID\" [flags]",
		ShortHelp:  "List profiles linked to a bundle ID.",
		LongHelp: `List profiles linked to a bundle ID.

Examples:
  asc bundle-ids profiles list --id "BUNDLE_ID"
  asc bundle-ids profiles list --id "BUNDLE_ID" --paginate`,
		ParentFlag:           "id",
		ParentUsage:          "Bundle ID",
		ParentType:           "bundleIds",
		RequiredDiagnostic:   true,
		PaginateIgnoresLimit: true,
		NoSpinner:            true,
		Deps:                 d,
		Flags: func(fs *flag.FlagSet) func() error {
			id, next := fs.Lookup("id").Value, fs.Lookup("next").Value
			return func() error {
				if strings.TrimSpace(id.String()) != "" || strings.TrimSpace(next.String()) == "" {
					return nil
				}
				if _, err := extractBundleIDFromNextURL(next.String()); err != nil {
					return d.UsageErrorf("bundle-ids profiles list: %v", err)
				}
				return nil
			}
		},
		FetchPage: func(ctx context.Context, client *asc.Client, id string, limit int, next string) (asc.PaginatedResponse, error) {
			return client.GetBundleIDProfiles(ctx, id,
				asc.Limit[asc.BundleIDProfilesOption](limit),
				asc.NextURL[asc.BundleIDProfilesOption](next),
			)
		},
	})
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
