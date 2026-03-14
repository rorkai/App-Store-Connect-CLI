package profiles

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

// ProfilesRelationshipsCommand returns the profiles links command group.
func ProfilesRelationshipsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("links", flag.ExitOnError)

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
			ProfilesRelationshipsBundleIDCommand(),
			ProfilesRelationshipsCertificatesCommand(),
			ProfilesRelationshipsDevicesCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// ProfilesRelationshipsBundleIDCommand returns the bundle-id links command.
func ProfilesRelationshipsBundleIDCommand() *ffcli.Command {
	fs := flag.NewFlagSet("bundle-id", flag.ExitOnError)

	id := fs.String("id", "", "Profile ID")
	output := shared.BindOutputFlags(fs)

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
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("profiles links bundle-id: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetProfileBundleIDRelationship(requestCtx, idValue)
			if err != nil {
				return fmt.Errorf("profiles links bundle-id: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// ProfilesRelationshipsCertificatesCommand returns the certificates links command.
func ProfilesRelationshipsCertificatesCommand() *ffcli.Command {
	fs := flag.NewFlagSet("certificates", flag.ExitOnError)

	id := fs.String("id", "", "Profile ID")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "certificates",
		ShortUsage: "asc profiles links certificates --id \"PROFILE_ID\" [flags]",
		ShortHelp:  "Get certificate relationship linkages for a profile.",
		LongHelp: `Get certificate relationship linkages for a profile.

Examples:
  asc profiles links certificates --id "PROFILE_ID"
  asc profiles links certificates --id "PROFILE_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			idValue := strings.TrimSpace(*id)
			if idValue == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("profiles links certificates: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("profiles links certificates: %w", err)
			}
			if idValue == "" && strings.TrimSpace(*next) != "" {
				derivedID, err := extractProfileIDFromNextURL(*next, "certificates")
				if err != nil {
					return fmt.Errorf("profiles links certificates: %w", err)
				}
				idValue = derivedID
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("profiles links certificates: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.LinkagesOption{
				asc.WithLinkagesLimit(*limit),
				asc.WithLinkagesNextURL(*next),
			}

			if *paginate {
				if idValue == "" {
					fmt.Fprintln(os.Stderr, "Error: --id is required")
					return flag.ErrHelp
				}
				paginateOpts := append(opts, asc.WithLinkagesLimit(200))
				firstPage, err := client.GetProfileCertificatesRelationships(requestCtx, idValue, paginateOpts...)
				if err != nil {
					return fmt.Errorf("profiles links certificates: failed to fetch: %w", err)
				}

				paginated, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetProfileCertificatesRelationships(ctx, idValue, asc.WithLinkagesNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("profiles links certificates: %w", err)
				}

				return shared.PrintOutput(paginated, *output.Output, *output.Pretty)
			}

			resp, err := client.GetProfileCertificatesRelationships(requestCtx, idValue, opts...)
			if err != nil {
				return fmt.Errorf("profiles links certificates: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// ProfilesRelationshipsDevicesCommand returns the devices links command.
func ProfilesRelationshipsDevicesCommand() *ffcli.Command {
	fs := flag.NewFlagSet("devices", flag.ExitOnError)

	id := fs.String("id", "", "Profile ID")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "devices",
		ShortUsage: "asc profiles links devices --id \"PROFILE_ID\" [flags]",
		ShortHelp:  "Get device relationship linkages for a profile.",
		LongHelp: `Get device relationship linkages for a profile.

Examples:
  asc profiles links devices --id "PROFILE_ID"
  asc profiles links devices --id "PROFILE_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			idValue := strings.TrimSpace(*id)
			if idValue == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("profiles links devices: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("profiles links devices: %w", err)
			}
			if idValue == "" && strings.TrimSpace(*next) != "" {
				derivedID, err := extractProfileIDFromNextURL(*next, "devices")
				if err != nil {
					return fmt.Errorf("profiles links devices: %w", err)
				}
				idValue = derivedID
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("profiles links devices: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.LinkagesOption{
				asc.WithLinkagesLimit(*limit),
				asc.WithLinkagesNextURL(*next),
			}

			if *paginate {
				if idValue == "" {
					fmt.Fprintln(os.Stderr, "Error: --id is required")
					return flag.ErrHelp
				}
				paginateOpts := append(opts, asc.WithLinkagesLimit(200))
				firstPage, err := client.GetProfileDevicesRelationships(requestCtx, idValue, paginateOpts...)
				if err != nil {
					return fmt.Errorf("profiles links devices: failed to fetch: %w", err)
				}

				paginated, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetProfileDevicesRelationships(ctx, idValue, asc.WithLinkagesNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("profiles links devices: %w", err)
				}

				return shared.PrintOutput(paginated, *output.Output, *output.Pretty)
			}

			resp, err := client.GetProfileDevicesRelationships(requestCtx, idValue, opts...)
			if err != nil {
				return fmt.Errorf("profiles links devices: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
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

// DeprecatedProfilesRelationshipsAliasCommand preserves the legacy
// relationships surface as a hidden compatibility alias.
func DeprecatedProfilesRelationshipsAliasCommand() *ffcli.Command {
	fs := flag.NewFlagSet("relationships", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "relationships",
		ShortUsage: "asc profiles links <bundle-id|certificates|devices> [flags]",
		ShortHelp:  "DEPRECATED: use `asc profiles links ...`.",
		LongHelp:   "Deprecated compatibility alias for `asc profiles links ...`.",
		FlagSet:    fs,
		UsageFunc:  shared.DeprecatedUsageFunc,
		Subcommands: []*ffcli.Command{
			shared.DeprecatedAliasLeafCommand(
				ProfilesRelationshipsBundleIDCommand(),
				"bundle-id",
				"asc profiles links bundle-id --id \"PROFILE_ID\"",
				"asc profiles links bundle-id",
				"Warning: `asc profiles relationships bundle-id` is deprecated. Use `asc profiles links bundle-id`.",
			),
			shared.DeprecatedAliasLeafCommand(
				ProfilesRelationshipsCertificatesCommand(),
				"certificates",
				"asc profiles links certificates --id \"PROFILE_ID\" [flags]",
				"asc profiles links certificates",
				"Warning: `asc profiles relationships certificates` is deprecated. Use `asc profiles links certificates`.",
			),
			shared.DeprecatedAliasLeafCommand(
				ProfilesRelationshipsDevicesCommand(),
				"devices",
				"asc profiles links devices --id \"PROFILE_ID\" [flags]",
				"asc profiles links devices",
				"Warning: `asc profiles relationships devices` is deprecated. Use `asc profiles links devices`.",
			),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}
