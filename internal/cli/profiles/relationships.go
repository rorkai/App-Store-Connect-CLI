package profiles

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

// newProfilesRelationshipsCommand returns the profiles links command group.
func newProfilesRelationshipsCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("links", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "links",
		ShortUsage: "asc profiles links <bundle-id|certificates|devices> [flags]",
		ShortHelp:  "View profile relationship linkages.",
		LongHelp: `View profile relationship linkages.

Examples:
  asc profiles links bundle-id --id "PROFILE_ID"
  asc profiles links certificates --id "PROFILE_ID"
  asc profiles links devices --id "PROFILE_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newProfilesRelationshipsBundleIDCommand(d),
			newProfilesRelationshipsCertificatesCommand(d),
			newProfilesRelationshipsDevicesCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newProfilesRelationshipsBundleIDCommand returns the bundle-id links command.
func newProfilesRelationshipsBundleIDCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("bundle-id", flag.ExitOnError)

	id := shared.BindResourceIDFlag(fs, "id", "profiles", "Profile ID")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "bundle-id",
		ShortUsage: "asc profiles links bundle-id --id \"PROFILE_ID\"",
		ShortHelp:  "Get bundle ID relationship for a profile.",
		LongHelp: `Get bundle ID relationship for a profile.

Examples:
  asc profiles links bundle-id --id "PROFILE_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			idValue := strings.TrimSpace(*id)
			if idValue == "" {
				return d.RequiredFlag("--id")
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("profiles links bundle-id: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			resp, err := client.GetProfileBundleIDRelationship(requestCtx, idValue)
			if err != nil {
				return fmt.Errorf("profiles links bundle-id: failed to fetch: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// newProfilesRelationshipsCertificatesCommand returns the certificates links command.
func newProfilesRelationshipsCertificatesCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildPaginatedListCommand(shared.PaginatedListCommandConfig{
		Name:       "certificates",
		ShortUsage: "asc profiles links certificates --id \"PROFILE_ID\" [flags]",
		ShortHelp:  "Get certificate relationship linkages for a profile.",
		LongHelp: `Get certificate relationship linkages for a profile.

Examples:
  asc profiles links certificates --id "PROFILE_ID"
  asc profiles links certificates --id "PROFILE_ID" --paginate`,
		ParentFlag:           "id",
		ParentUsage:          "Profile ID",
		ParentType:           "profiles",
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
				if _, err := extractProfileIDFromNextURL(next.String(), "certificates"); err != nil {
					return d.UsageErrorf("profiles links certificates: %v", err)
				}
				return nil
			}
		},
		FetchPage: func(ctx context.Context, client *asc.Client, id string, limit int, next string) (asc.PaginatedResponse, error) {
			return client.GetProfileCertificatesRelationships(ctx, id,
				asc.Limit[asc.LinkagesOption](limit),
				asc.NextURL[asc.LinkagesOption](next),
			)
		},
	})
}

// newProfilesRelationshipsDevicesCommand returns the devices links command.
func newProfilesRelationshipsDevicesCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildPaginatedListCommand(shared.PaginatedListCommandConfig{
		Name:       "devices",
		ShortUsage: "asc profiles links devices --id \"PROFILE_ID\" [flags]",
		ShortHelp:  "Get device relationship linkages for a profile.",
		LongHelp: `Get device relationship linkages for a profile.

Examples:
  asc profiles links devices --id "PROFILE_ID"
  asc profiles links devices --id "PROFILE_ID" --paginate`,
		ParentFlag:           "id",
		ParentUsage:          "Profile ID",
		ParentType:           "profiles",
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
				if _, err := extractProfileIDFromNextURL(next.String(), "devices"); err != nil {
					return d.UsageErrorf("profiles links devices: %v", err)
				}
				return nil
			}
		},
		FetchPage: func(ctx context.Context, client *asc.Client, id string, limit int, next string) (asc.PaginatedResponse, error) {
			return client.GetProfileDevicesRelationships(ctx, id,
				asc.Limit[asc.LinkagesOption](limit),
				asc.NextURL[asc.LinkagesOption](next),
			)
		},
	})
}

func extractProfileIDFromNextURL(nextURL string, relationship string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(nextURL))
	if err != nil {
		return "", fmt.Errorf("invalid --next URL")
	}
	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(parts) < 5 || parts[0] != "v1" || parts[1] != "profiles" || parts[3] != "relationships" || parts[4] != relationship {
		return "", fmt.Errorf("invalid --next URL")
	}
	if strings.TrimSpace(parts[2]) == "" {
		return "", fmt.Errorf("invalid --next URL")
	}
	return parts[2], nil
}
