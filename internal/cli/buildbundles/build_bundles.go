package buildbundles

import (
	"context"
	"flag"
	"fmt"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// NewBuildBundlesCommand returns the build-bundles command with subcommands,
// built on d.
func NewBuildBundlesCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("build-bundles", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "build-bundles",
		ShortUsage: "asc build-bundles <subcommand> [flags]",
		ShortHelp:  "Manage build bundles and App Clip data.",
		LongHelp: `Manage build bundles and App Clip data.

Examples:
  asc build-bundles list --build-id "BUILD_ID"
  asc build-bundles file-sizes list --id "BUILD_BUNDLE_ID"
  asc build-bundles app-clip cache-status view --id "BUILD_BUNDLE_ID"
  asc build-bundles app-clip debug-status view --id "BUILD_BUNDLE_ID"
  asc build-bundles app-clip invocations list --id "BUILD_BUNDLE_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newBuildBundlesListCommand(d),
			newBuildBundleFileSizesCommand(d),
			newBuildBundlesAppClipCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newBuildBundlesListCommand returns the build bundles list subcommand built on d.
func newBuildBundlesListCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("list", flag.ExitOnError)

	buildID := shared.BindResourceIDFlag(fs, "build-id", "builds", "Build ID")
	limit := fs.Int("limit", 0, "Maximum included build bundles (1-50)")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc build-bundles list [flags]",
		ShortHelp:  "List build bundles for a build.",
		LongHelp: `List build bundles for a build.

Examples:
  asc build-bundles list --build-id "BUILD_ID"
  asc build-bundles list --build-id "BUILD_ID" --limit 10`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 50) {
				return d.UsageError("build-bundles list: --limit must be between 1 and 50")
			}

			buildValue := strings.TrimSpace(*buildID)
			if buildValue == "" {
				return d.RequiredFlag("--build-id")
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("build-bundles list: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			opts := []asc.BuildBundlesOption{}
			if *limit > 0 {
				opts = append(opts, asc.WithBuildBundlesLimit(*limit))
			}

			resp, err := client.GetBuildBundlesForBuild(requestCtx, buildValue, opts...)
			if err != nil {
				return fmt.Errorf("build-bundles list: failed to fetch: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// newBuildBundleFileSizesCommand returns the build bundle file sizes command group built on d.
func newBuildBundleFileSizesCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("file-sizes", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "file-sizes",
		ShortUsage: "asc build-bundles file-sizes <subcommand> [flags]",
		ShortHelp:  "Manage build bundle file size data.",
		LongHelp: `Manage build bundle file size data.

Examples:
  asc build-bundles file-sizes list --id "BUILD_BUNDLE_ID"
  asc build-bundles file-sizes list --id "BUILD_BUNDLE_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newBuildBundleFileSizesListCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newBuildBundleFileSizesListCommand returns the list subcommand built on d.
func newBuildBundleFileSizesListCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("list", flag.ExitOnError)

	buildBundleID := fs.String("id", "", "Build bundle ID")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc build-bundles file-sizes list [flags]",
		ShortHelp:  "List build bundle file sizes.",
		LongHelp: `List build bundle file sizes.

Examples:
  asc build-bundles file-sizes list --id "BUILD_BUNDLE_ID"
  asc build-bundles file-sizes list --id "BUILD_BUNDLE_ID" --limit 100
  asc build-bundles file-sizes list --id "BUILD_BUNDLE_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return d.UsageError("build-bundles file-sizes list: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return d.UsageErrorf("build-bundles file-sizes list: %v", err)
			}

			buildBundleValue := strings.TrimSpace(*buildBundleID)
			if buildBundleValue == "" {
				return d.RequiredFlag("--id")
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("build-bundles file-sizes list: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			opts := []asc.BuildBundleFileSizesOption{
				asc.Limit[asc.BuildBundleFileSizesOption](*limit),
				asc.NextURL[asc.BuildBundleFileSizesOption](*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.Limit[asc.BuildBundleFileSizesOption](200))
				firstPage, err := client.GetBuildBundleFileSizes(requestCtx, buildBundleValue, paginateOpts...)
				if err != nil {
					return fmt.Errorf("build-bundles file-sizes list: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetBuildBundleFileSizes(ctx, buildBundleValue, asc.NextURL[asc.BuildBundleFileSizesOption](nextURL))
				})
				if err != nil {
					return fmt.Errorf("build-bundles file-sizes list: %w", err)
				}

				return d.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := client.GetBuildBundleFileSizes(requestCtx, buildBundleValue, opts...)
			if err != nil {
				return fmt.Errorf("build-bundles file-sizes list: failed to fetch: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// newBuildBundlesAppClipCommand returns the app-clip command group built on d.
func newBuildBundlesAppClipCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("app-clip", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "app-clip",
		ShortUsage: "asc build-bundles app-clip <subcommand> [flags]",
		ShortHelp:  "Manage App Clip data for build bundles.",
		LongHelp: `Manage App Clip data for build bundles.

Examples:
  asc build-bundles app-clip cache-status view --id "BUILD_BUNDLE_ID"
  asc build-bundles app-clip debug-status view --id "BUILD_BUNDLE_ID"
  asc build-bundles app-clip invocations list --id "BUILD_BUNDLE_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newBuildBundlesAppClipCacheStatusCommand(d),
			newBuildBundlesAppClipDebugStatusCommand(d),
			newBuildBundlesAppClipInvocationsCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newBuildBundlesAppClipCacheStatusCommand returns the cache-status command group built on d.
func newBuildBundlesAppClipCacheStatusCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("cache-status", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "cache-status",
		ShortUsage: "asc build-bundles app-clip cache-status <subcommand> [flags]",
		ShortHelp:  "Fetch App Clip domain cache status.",
		LongHelp: `Fetch App Clip domain cache status.

Examples:
  asc build-bundles app-clip cache-status view --id "BUILD_BUNDLE_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newBuildBundlesAppClipCacheStatusGetCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newBuildBundlesAppClipCacheStatusGetCommand returns the cache-status get subcommand built on d.
func newBuildBundlesAppClipCacheStatusGetCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		Deps:        d,
		FlagSetName: "view",
		Name:        "view",
		ShortUsage:  "asc build-bundles app-clip cache-status view --id \"BUILD_BUNDLE_ID\"",
		ShortHelp:   "View App Clip domain cache status for a build bundle.",
		LongHelp: `View App Clip domain cache status for a build bundle.

Examples:
  asc build-bundles app-clip cache-status view --id "BUILD_BUNDLE_ID"`,
		IDUsage:            "Build bundle ID",
		RequiredDiagnostic: true,
		ErrorPrefix:        "build-bundles app-clip cache-status view",
		FetchErrorPrefix:   "build-bundles app-clip cache-status view: failed to fetch",
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			resp, err := client.GetBuildBundleAppClipDomainCacheStatus(ctx, id)
			if err != nil {
				if asc.IsNotFound(err) {
					return asc.NewAppClipDomainStatusResult(id, nil), nil
				}
				return nil, err
			}
			return asc.NewAppClipDomainStatusResult(id, resp), nil
		},
	})
}

// newBuildBundlesAppClipDebugStatusCommand returns the debug-status command group built on d.
func newBuildBundlesAppClipDebugStatusCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("debug-status", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "debug-status",
		ShortUsage: "asc build-bundles app-clip debug-status <subcommand> [flags]",
		ShortHelp:  "Fetch App Clip domain debug status.",
		LongHelp: `Fetch App Clip domain debug status.

Examples:
  asc build-bundles app-clip debug-status view --id "BUILD_BUNDLE_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newBuildBundlesAppClipDebugStatusGetCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newBuildBundlesAppClipDebugStatusGetCommand returns the debug-status get subcommand built on d.
func newBuildBundlesAppClipDebugStatusGetCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		Deps:        d,
		FlagSetName: "view",
		Name:        "view",
		ShortUsage:  "asc build-bundles app-clip debug-status view --id \"BUILD_BUNDLE_ID\"",
		ShortHelp:   "View App Clip domain debug status for a build bundle.",
		LongHelp: `View App Clip domain debug status for a build bundle.

Examples:
  asc build-bundles app-clip debug-status view --id "BUILD_BUNDLE_ID"`,
		IDUsage:            "Build bundle ID",
		RequiredDiagnostic: true,
		ErrorPrefix:        "build-bundles app-clip debug-status view",
		FetchErrorPrefix:   "build-bundles app-clip debug-status view: failed to fetch",
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			resp, err := client.GetBuildBundleAppClipDomainDebugStatus(ctx, id)
			if err != nil {
				if asc.IsNotFound(err) {
					return asc.NewAppClipDomainStatusResult(id, nil), nil
				}
				return nil, err
			}
			return asc.NewAppClipDomainStatusResult(id, resp), nil
		},
	})
}

// newBuildBundlesAppClipInvocationsCommand returns the invocations command group built on d.
func newBuildBundlesAppClipInvocationsCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("invocations", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "invocations",
		ShortUsage: "asc build-bundles app-clip invocations <subcommand> [flags]",
		ShortHelp:  "Manage App Clip invocations.",
		LongHelp: `Manage App Clip invocations.

Examples:
  asc build-bundles app-clip invocations list --id "BUILD_BUNDLE_ID"
  asc build-bundles app-clip invocations list --id "BUILD_BUNDLE_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newBuildBundlesAppClipInvocationsListCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newBuildBundlesAppClipInvocationsListCommand returns the invocations list subcommand built on d.
func newBuildBundlesAppClipInvocationsListCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("list", flag.ExitOnError)

	buildBundleID := fs.String("id", "", "Build bundle ID")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc build-bundles app-clip invocations list [flags]",
		ShortHelp:  "List App Clip invocations for a build bundle.",
		LongHelp: `List App Clip invocations for a build bundle.

Examples:
  asc build-bundles app-clip invocations list --id "BUILD_BUNDLE_ID"
  asc build-bundles app-clip invocations list --id "BUILD_BUNDLE_ID" --limit 50
  asc build-bundles app-clip invocations list --id "BUILD_BUNDLE_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return d.UsageError("build-bundles app-clip invocations list: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return d.UsageErrorf("build-bundles app-clip invocations list: %v", err)
			}

			buildBundleValue := strings.TrimSpace(*buildBundleID)
			if buildBundleValue == "" {
				return d.RequiredFlag("--id")
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("build-bundles app-clip invocations list: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			opts := []asc.BetaAppClipInvocationsOption{
				asc.Limit[asc.BetaAppClipInvocationsOption](*limit),
				asc.NextURL[asc.BetaAppClipInvocationsOption](*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.Limit[asc.BetaAppClipInvocationsOption](200))
				firstPage, err := client.GetBuildBundleBetaAppClipInvocations(requestCtx, buildBundleValue, paginateOpts...)
				if err != nil {
					if asc.IsNotFound(err) {
						empty := &asc.BetaAppClipInvocationsResponse{Data: []asc.Resource[asc.BetaAppClipInvocationAttributes]{}}
						return d.PrintOutput(empty, *output.Output, *output.Pretty)
					}
					return fmt.Errorf("build-bundles app-clip invocations list: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetBuildBundleBetaAppClipInvocations(ctx, buildBundleValue, asc.NextURL[asc.BetaAppClipInvocationsOption](nextURL))
				})
				if err != nil {
					return fmt.Errorf("build-bundles app-clip invocations list: %w", err)
				}

				return d.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := client.GetBuildBundleBetaAppClipInvocations(requestCtx, buildBundleValue, opts...)
			if err != nil {
				if asc.IsNotFound(err) {
					empty := &asc.BetaAppClipInvocationsResponse{Data: []asc.Resource[asc.BetaAppClipInvocationAttributes]{}}
					return d.PrintOutput(empty, *output.Output, *output.Pretty)
				}
				return fmt.Errorf("build-bundles app-clip invocations list: failed to fetch: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}
