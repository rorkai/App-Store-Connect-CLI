package prerelease

import (
	"context"
	"flag"
	"fmt"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// NewPreReleaseVersionsCommand returns the pre-release-versions command built
// on d.
func NewPreReleaseVersionsCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("pre-release-versions", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "pre-release-versions",
		ShortUsage: "asc pre-release-versions <subcommand> [flags]",
		ShortHelp:  "Manage TestFlight pre-release versions.",
		LongHelp: `Manage TestFlight pre-release versions.

Examples:
  asc pre-release-versions list --app "APP_ID"
  asc pre-release-versions relationships view --id "PR_ID" --type "app"`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newPreReleaseVersionsListCommand(d),
			newPreReleaseVersionsGetCommand(d),
			newPreReleaseVersionsAppCommand(d),
			newPreReleaseVersionsBuildsCommand(d),
			newPreReleaseVersionsRelationshipsCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newPreReleaseVersionsListCommand returns the pre-release versions list subcommand built on d.
func newPreReleaseVersionsListCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("pre-release-versions list", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID)")
	platform := fs.String("platform", "", "Filter by platform: IOS, MAC_OS, TV_OS, VISION_OS")
	version := fs.String("version", "", "Filter by version string")
	limit := fs.Int("limit", 0, shared.LimitFlagUsage(200))
	next := fs.String("next", "", shared.NextFlagUsage)
	paginate := fs.Bool("paginate", false, shared.PaginateFlagUsage)
	output := d.BindOutputFlags(fs)

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
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := d.ValidatePaginationFlags(ctx, fs, "pre-release-versions list", 200, "platform", "version"); err != nil {
				return err
			}

			resolvedAppID := strings.TrimSpace(d.ResolveAppID(strings.TrimSpace(*appID)))
			nextValue := strings.TrimSpace(*next)
			if resolvedAppID == "" && nextValue == "" {
				fmt.Fprintln(d.IO.ErrOut, "Error: --app is required (or set ASC_APP_ID)")
				return shared.MissingRequiredUsageError("--app")
			}

			platforms, err := shared.NormalizeAppStoreVersionPlatforms(shared.SplitCSVUpper(*platform))
			if err != nil {
				return d.UsageErrorfCtx(ctx, "pre-release-versions list: %v", err)
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("pre-release-versions list: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			opts := []asc.PreReleaseVersionsOption{
				asc.WithPreReleaseVersionsAppID(resolvedAppID),
				asc.Limit[asc.PreReleaseVersionsOption](*limit),
				asc.NextURL[asc.PreReleaseVersionsOption](nextValue),
			}

			if len(platforms) > 0 {
				opts = append(opts, asc.WithPreReleaseVersionsPlatform(strings.Join(platforms, ",")))
			}
			if versions := shared.SplitCSV(*version); len(versions) > 0 {
				opts = append(opts, asc.WithPreReleaseVersionsVersion(strings.Join(versions, ",")))
			}

			if *paginate {
				// Fetch first page with limit set for consistent pagination
				paginateOpts := append(opts, asc.Limit[asc.PreReleaseVersionsOption](200))
				firstPage, err := client.GetPreReleaseVersions(requestCtx, paginateOpts...)
				if err != nil {
					return fmt.Errorf("pre-release-versions list: failed to fetch: %w", err)
				}

				// Fetch all remaining pages
				versions, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetPreReleaseVersions(ctx, asc.WithPreReleaseVersionsAppID(resolvedAppID), asc.NextURL[asc.PreReleaseVersionsOption](nextURL))
				})
				if err != nil {
					return fmt.Errorf("pre-release-versions list: %w", err)
				}

				return d.PrintOutput(versions, *output.Output, *output.Pretty)
			}

			versions, err := client.GetPreReleaseVersions(requestCtx, opts...)
			if err != nil {
				return fmt.Errorf("pre-release-versions list: failed to fetch: %w", err)
			}

			return d.PrintOutput(versions, *output.Output, *output.Pretty)
		},
	}
}

// newPreReleaseVersionsGetCommand returns the pre-release versions get subcommand built on d.
func newPreReleaseVersionsGetCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		Deps:        d,
		FlagSetName: "pre-release-versions view",
		Name:        "view",
		ShortUsage:  "asc pre-release-versions view [flags]",
		ShortHelp:   "View a TestFlight pre-release version by ID.",
		LongHelp: `View a TestFlight pre-release version by ID.

Examples:
  asc pre-release-versions view --id "PR_ID"`,
		IDUsage:            "Pre-release version ID",
		IDType:             "preReleaseVersions",
		RequiredDiagnostic: true,
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.GetPreReleaseVersion(ctx, id)
		},
	})
}
