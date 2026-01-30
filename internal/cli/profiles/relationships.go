package profiles

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

// ProfilesRelationshipsCommand returns the profiles relationships command group.
func ProfilesRelationshipsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("relationships", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "relationships",
		ShortUsage: "asc profiles relationships <bundle-id|certificates|devices> [flags]",
		ShortHelp:  "View profile relationship linkages.",
		LongHelp: `View profile relationship linkages.

Examples:
  asc profiles relationships bundle-id --id "PROFILE_ID"
  asc profiles relationships certificates --id "PROFILE_ID"
  asc profiles relationships devices --id "PROFILE_ID"`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
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

// ProfilesRelationshipsBundleIDCommand returns the bundle-id relationships command.
func ProfilesRelationshipsBundleIDCommand() *ffcli.Command {
	fs := flag.NewFlagSet("bundle-id", flag.ExitOnError)

	id := fs.String("id", "", "Profile ID")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "bundle-id",
		ShortUsage: "asc profiles relationships bundle-id --id \"PROFILE_ID\"",
		ShortHelp:  "Get bundle ID relationship for a profile.",
		LongHelp: `Get bundle ID relationship for a profile.

Examples:
  asc profiles relationships bundle-id --id "PROFILE_ID"`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			idValue := strings.TrimSpace(*id)
			if idValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("profiles relationships bundle-id: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetProfileBundleIDRelationship(requestCtx, idValue)
			if err != nil {
				return fmt.Errorf("profiles relationships bundle-id: failed to fetch: %w", err)
			}

			return printOutput(resp, *output, *pretty)
		},
	}
}

// ProfilesRelationshipsCertificatesCommand returns the certificates relationships command.
func ProfilesRelationshipsCertificatesCommand() *ffcli.Command {
	fs := flag.NewFlagSet("certificates", flag.ExitOnError)

	id := fs.String("id", "", "Profile ID")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "certificates",
		ShortUsage: "asc profiles relationships certificates --id \"PROFILE_ID\" [flags]",
		ShortHelp:  "Get certificate relationship linkages for a profile.",
		LongHelp: `Get certificate relationship linkages for a profile.

Examples:
  asc profiles relationships certificates --id "PROFILE_ID"
  asc profiles relationships certificates --id "PROFILE_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			idValue := strings.TrimSpace(*id)
			if idValue == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("profiles relationships certificates: --limit must be between 1 and 200")
			}
			if err := validateNextURL(*next); err != nil {
				return fmt.Errorf("profiles relationships certificates: %w", err)
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("profiles relationships certificates: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			opts := []asc.LinkagesOption{
				asc.WithLinkagesLimit(*limit),
				asc.WithLinkagesNextURL(*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithLinkagesLimit(200))
				firstPage, err := client.GetProfileCertificatesRelationships(requestCtx, idValue, paginateOpts...)
				if err != nil {
					return fmt.Errorf("profiles relationships certificates: failed to fetch: %w", err)
				}

				paginated, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetProfileCertificatesRelationships(ctx, idValue, asc.WithLinkagesNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("profiles relationships certificates: %w", err)
				}

				return printOutput(paginated, *output, *pretty)
			}

			resp, err := client.GetProfileCertificatesRelationships(requestCtx, idValue, opts...)
			if err != nil {
				return fmt.Errorf("profiles relationships certificates: failed to fetch: %w", err)
			}

			return printOutput(resp, *output, *pretty)
		},
	}
}

// ProfilesRelationshipsDevicesCommand returns the devices relationships command.
func ProfilesRelationshipsDevicesCommand() *ffcli.Command {
	fs := flag.NewFlagSet("devices", flag.ExitOnError)

	id := fs.String("id", "", "Profile ID")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "devices",
		ShortUsage: "asc profiles relationships devices --id \"PROFILE_ID\" [flags]",
		ShortHelp:  "Get device relationship linkages for a profile.",
		LongHelp: `Get device relationship linkages for a profile.

Examples:
  asc profiles relationships devices --id "PROFILE_ID"
  asc profiles relationships devices --id "PROFILE_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			idValue := strings.TrimSpace(*id)
			if idValue == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("profiles relationships devices: --limit must be between 1 and 200")
			}
			if err := validateNextURL(*next); err != nil {
				return fmt.Errorf("profiles relationships devices: %w", err)
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("profiles relationships devices: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			opts := []asc.LinkagesOption{
				asc.WithLinkagesLimit(*limit),
				asc.WithLinkagesNextURL(*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithLinkagesLimit(200))
				firstPage, err := client.GetProfileDevicesRelationships(requestCtx, idValue, paginateOpts...)
				if err != nil {
					return fmt.Errorf("profiles relationships devices: failed to fetch: %w", err)
				}

				paginated, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetProfileDevicesRelationships(ctx, idValue, asc.WithLinkagesNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("profiles relationships devices: %w", err)
				}

				return printOutput(paginated, *output, *pretty)
			}

			resp, err := client.GetProfileDevicesRelationships(requestCtx, idValue, opts...)
			if err != nil {
				return fmt.Errorf("profiles relationships devices: failed to fetch: %w", err)
			}

			return printOutput(resp, *output, *pretty)
		},
	}
}
