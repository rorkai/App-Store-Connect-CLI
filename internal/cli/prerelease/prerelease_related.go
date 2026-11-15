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

// newPreReleaseVersionsAppCommand returns the app command group built on d.
func newPreReleaseVersionsAppCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("app", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "app",
		ShortUsage: "asc pre-release-versions app <subcommand> [flags]",
		ShortHelp:  "View the app for a pre-release version.",
		LongHelp: `View the app for a pre-release version.

Examples:
  asc pre-release-versions app view --id "PR_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newPreReleaseVersionsAppGetCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newPreReleaseVersionsAppGetCommand returns the app get subcommand built on d.
func newPreReleaseVersionsAppGetCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		Deps:        d,
		FlagSetName: "app view",
		Name:        "view",
		ShortUsage:  "asc pre-release-versions app view --id \"PR_ID\"",
		ShortHelp:   "View the app for a pre-release version.",
		LongHelp: `View the app for a pre-release version.

Examples:
  asc pre-release-versions app view --id "PR_ID"`,
		IDUsage:            "Pre-release version ID",
		IDType:             "preReleaseVersions",
		RequiredDiagnostic: true,
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.GetPreReleaseVersionApp(ctx, id)
		},
	})
}

// newPreReleaseVersionsBuildsCommand returns the builds command group built on d.
func newPreReleaseVersionsBuildsCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("builds", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "builds",
		ShortUsage: "asc pre-release-versions builds <subcommand> [flags]",
		ShortHelp:  "List builds for a pre-release version.",
		LongHelp: `List builds for a pre-release version.

Examples:
  asc pre-release-versions builds list --id "PR_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newPreReleaseVersionsBuildsListCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newPreReleaseVersionsBuildsListCommand returns the builds list subcommand built on d.
func newPreReleaseVersionsBuildsListCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("builds list", flag.ExitOnError)

	id := shared.BindResourceIDFlag(fs, "id", "preReleaseVersions", "Pre-release version ID")
	limit := fs.Int("limit", 0, shared.LimitFlagUsage(200))
	next := fs.String("next", "", shared.NextFlagUsage)
	paginate := fs.Bool("paginate", false, shared.PaginateFlagUsage)
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc pre-release-versions builds list [flags]",
		ShortHelp:  "List builds for a pre-release version.",
		LongHelp: `List builds for a pre-release version.

Examples:
  asc pre-release-versions builds list --id "PR_ID"
  asc pre-release-versions builds list --id "PR_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := d.ValidatePaginationFlags(ctx, fs, "pre-release-versions builds list", 200); err != nil {
				return err
			}

			idValue := strings.TrimSpace(*id)
			if idValue == "" && strings.TrimSpace(*next) == "" {
				return d.RequiredFlag("--id")
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("pre-release-versions builds list: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			opts := []asc.PreReleaseVersionBuildsOption{
				asc.Limit[asc.PreReleaseVersionBuildsOption](*limit),
				asc.NextURL[asc.PreReleaseVersionBuildsOption](*next),
			}

			if *paginate {
				if idValue == "" {
					return d.RequiredFlag("--id")
				}
				paginateOpts := append(opts, asc.Limit[asc.PreReleaseVersionBuildsOption](200))
				firstPage, err := client.GetPreReleaseVersionBuilds(requestCtx, idValue, paginateOpts...)
				if err != nil {
					return fmt.Errorf("pre-release-versions builds list: failed to fetch: %w", err)
				}
				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetPreReleaseVersionBuilds(ctx, idValue, asc.NextURL[asc.PreReleaseVersionBuildsOption](nextURL))
				})
				if err != nil {
					return fmt.Errorf("pre-release-versions builds list: %w", err)
				}
				return d.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := client.GetPreReleaseVersionBuilds(requestCtx, idValue, opts...)
			if err != nil {
				return fmt.Errorf("pre-release-versions builds list: failed to fetch: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}
