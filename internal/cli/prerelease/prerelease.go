package prerelease

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

// PreReleaseVersionsCommand returns the pre-release-versions command.
func PreReleaseVersionsCommand() *ffcli.Command {
	return &ffcli.Command{
		Name:       "pre-release-versions",
		ShortUsage: "asc pre-release-versions <subcommand> [flags]",
		ShortHelp:  "Manage TestFlight pre-release versions.",
		LongHelp: `Manage TestFlight pre-release versions.

Examples:
  asc pre-release-versions list --app "APP_ID"
  asc pre-release-versions relationships get --id "PR_ID" --type "app"`,
		UsageFunc: DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			PreReleaseVersionsListCommand(),
			PreReleaseVersionsGetCommand(),
			PreReleaseVersionsRelationshipsCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// PreReleaseVersionsListCommand returns the pre-release versions list subcommand.
func PreReleaseVersionsListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("pre-release-versions list", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID)")
	platform := fs.String("platform", "", "Filter by platform: IOS, MAC_OS, TV_OS, VISION_OS")
	version := fs.String("version", "", "Filter by version string")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Next page URL from a previous response")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc pre-release-versions list [flags]",
		ShortHelp:  "List TestFlight pre-release versions for an app.",
		LongHelp: `List TestFlight pre-release versions for an app.

Examples:
  asc pre-release-versions list --app "APP_ID"
  asc pre-release-versions list --app "APP_ID" --platform IOS
  asc pre-release-versions list --app "APP_ID" --version "1.0.0"
  asc pre-release-versions list --app "APP_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("pre-release-versions list: --limit must be between 1 and 200")
			}
			if err := validateNextURL(*next); err != nil {
				return fmt.Errorf("pre-release-versions list: %w", err)
			}

			platforms, err := shared.NormalizeAppStoreVersionPlatforms(splitCSVUpper(*platform))
			if err != nil {
				return fmt.Errorf("pre-release-versions list: %w", err)
			}

			resolvedAppID := strings.TrimSpace(resolveAppID(strings.TrimSpace(*appID)))
			nextValue := strings.TrimSpace(*next)
			if resolvedAppID == "" && nextValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --app is required (or set ASC_APP_ID)")
				return flag.ErrHelp
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("pre-release-versions list: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			opts := []asc.PreReleaseVersionsOption{
				asc.WithPreReleaseVersionsLimit(*limit),
				asc.WithPreReleaseVersionsNextURL(nextValue),
			}

			if len(platforms) > 0 {
				opts = append(opts, asc.WithPreReleaseVersionsPlatform(strings.Join(platforms, ",")))
			}
			if versions := splitCSV(*version); len(versions) > 0 {
				opts = append(opts, asc.WithPreReleaseVersionsVersion(strings.Join(versions, ",")))
			}

			if *paginate {
				// Fetch first page with limit set for consistent pagination
				paginateOpts := append(opts, asc.WithPreReleaseVersionsLimit(200))
				firstPage, err := client.GetPreReleaseVersions(requestCtx, resolvedAppID, paginateOpts...)
				if err != nil {
					return fmt.Errorf("pre-release-versions list: failed to fetch: %w", err)
				}

				// Fetch all remaining pages
				versions, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetPreReleaseVersions(ctx, resolvedAppID, asc.WithPreReleaseVersionsNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("pre-release-versions list: %w", err)
				}

				return printOutput(versions, *output, *pretty)
			}

			versions, err := client.GetPreReleaseVersions(requestCtx, resolvedAppID, opts...)
			if err != nil {
				return fmt.Errorf("pre-release-versions list: failed to fetch: %w", err)
			}

			return printOutput(versions, *output, *pretty)
		},
	}
}

// PreReleaseVersionsGetCommand returns the pre-release versions get subcommand.
func PreReleaseVersionsGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("pre-release-versions get", flag.ExitOnError)

	id := fs.String("id", "", "Pre-release version ID")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc pre-release-versions get [flags]",
		ShortHelp:  "Get a TestFlight pre-release version by ID.",
		LongHelp: `Get a TestFlight pre-release version by ID.

Examples:
  asc pre-release-versions get --id "PR_ID"`,
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
				return fmt.Errorf("pre-release-versions get: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			version, err := client.GetPreReleaseVersion(requestCtx, idValue)
			if err != nil {
				return fmt.Errorf("pre-release-versions get: failed to fetch: %w", err)
			}

			return printOutput(version, *output, *pretty)
		},
	}
}
