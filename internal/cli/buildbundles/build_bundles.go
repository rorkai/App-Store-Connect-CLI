package buildbundles

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

// BuildBundlesCommand returns the build-bundles command with subcommands.
func BuildBundlesCommand() *ffcli.Command {
	fs := flag.NewFlagSet("build-bundles", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "build-bundles",
		ShortUsage: "asc build-bundles <subcommand> [flags]",
		ShortHelp:  "Manage build bundles and App Clip data.",
		LongHelp: `Manage build bundles and App Clip data.

Examples:
  asc build-bundles list --build "BUILD_ID"
  asc build-bundles file-sizes list --id "BUILD_BUNDLE_ID"
  asc build-bundles app-clip cache-status get --id "BUILD_BUNDLE_ID"
  asc build-bundles app-clip debug-status get --id "BUILD_BUNDLE_ID"
  asc build-bundles app-clip invocations list --id "BUILD_BUNDLE_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			BuildBundlesListCommand(),
			BuildBundleFileSizesCommand(),
			BuildBundlesAppClipCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// BuildBundlesListCommand returns the build bundles list subcommand.
func BuildBundlesListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("list", flag.ExitOnError)

	buildID := fs.String("build", "", "Build ID")
	limit := fs.Int("limit", 0, "Maximum included build bundles (1-50)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc build-bundles list [flags]",
		ShortHelp:  "List build bundles for a build.",
		LongHelp: `List build bundles for a build.

Examples:
  asc build-bundles list --build "BUILD_ID"
  asc build-bundles list --build "BUILD_ID" --limit 10`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 50) {
				return fmt.Errorf("build-bundles list: --limit must be between 1 and 50")
			}

			buildValue := strings.TrimSpace(*buildID)
			if buildValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --build is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("build-bundles list: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.BuildBundlesOption{}
			if *limit > 0 {
				opts = append(opts, asc.WithBuildBundlesLimit(*limit))
			}

			resp, err := client.GetBuildBundlesForBuild(requestCtx, buildValue, opts...)
			if err != nil {
				return fmt.Errorf("build-bundles list: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// BuildBundleFileSizesCommand returns the build bundle file sizes command group.
func BuildBundleFileSizesCommand() *ffcli.Command {
	fs := flag.NewFlagSet("file-sizes", flag.ExitOnError)

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
			BuildBundleFileSizesListCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// BuildBundleFileSizesListCommand returns the list subcommand.
func BuildBundleFileSizesListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("list", flag.ExitOnError)

	buildBundleID := fs.String("id", "", "Build bundle ID")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := shared.BindOutputFlags(fs)

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
				return fmt.Errorf("build-bundles file-sizes list: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("build-bundles file-sizes list: %w", err)
			}

			buildBundleValue := strings.TrimSpace(*buildBundleID)
			if buildBundleValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("build-bundles file-sizes list: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.BuildBundleFileSizesOption{
				asc.WithBuildBundleFileSizesLimit(*limit),
				asc.WithBuildBundleFileSizesNextURL(*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithBuildBundleFileSizesLimit(200))
				firstPage, err := client.GetBuildBundleFileSizes(requestCtx, buildBundleValue, paginateOpts...)
				if err != nil {
					return fmt.Errorf("build-bundles file-sizes list: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetBuildBundleFileSizes(ctx, buildBundleValue, asc.WithBuildBundleFileSizesNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("build-bundles file-sizes list: %w", err)
				}

				return shared.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := client.GetBuildBundleFileSizes(requestCtx, buildBundleValue, opts...)
			if err != nil {
				return fmt.Errorf("build-bundles file-sizes list: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// BuildBundlesAppClipCommand returns the app-clip command group.
func BuildBundlesAppClipCommand() *ffcli.Command {
	fs := flag.NewFlagSet("app-clip", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "app-clip",
		ShortUsage: "asc build-bundles app-clip <subcommand> [flags]",
		ShortHelp:  "Manage App Clip data for build bundles.",
		LongHelp: `Manage App Clip data for build bundles.

Examples:
  asc build-bundles app-clip cache-status get --id "BUILD_BUNDLE_ID"
  asc build-bundles app-clip debug-status get --id "BUILD_BUNDLE_ID"
  asc build-bundles app-clip invocations list --id "BUILD_BUNDLE_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			BuildBundlesAppClipCacheStatusCommand(),
			BuildBundlesAppClipDebugStatusCommand(),
			BuildBundlesAppClipInvocationsCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// BuildBundlesAppClipCacheStatusCommand returns the cache-status command group.
func BuildBundlesAppClipCacheStatusCommand() *ffcli.Command {
	fs := flag.NewFlagSet("cache-status", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "cache-status",
		ShortUsage: "asc build-bundles app-clip cache-status <subcommand> [flags]",
		ShortHelp:  "Fetch App Clip domain cache status.",
		LongHelp: `Fetch App Clip domain cache status.

Examples:
  asc build-bundles app-clip cache-status get --id "BUILD_BUNDLE_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			BuildBundlesAppClipCacheStatusGetCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// BuildBundlesAppClipCacheStatusGetCommand returns the cache-status get subcommand.
func BuildBundlesAppClipCacheStatusGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("get", flag.ExitOnError)

	buildBundleID := fs.String("id", "", "Build bundle ID")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc build-bundles app-clip cache-status get --id \"BUILD_BUNDLE_ID\"",
		ShortHelp:  "Get App Clip domain cache status for a build bundle.",
		LongHelp: `Get App Clip domain cache status for a build bundle.

Examples:
  asc build-bundles app-clip cache-status get --id "BUILD_BUNDLE_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			buildBundleValue := strings.TrimSpace(*buildBundleID)
			if buildBundleValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("build-bundles app-clip cache-status get: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetBuildBundleAppClipDomainCacheStatus(requestCtx, buildBundleValue)
			if err != nil {
				if asc.IsNotFound(err) {
					result := asc.NewAppClipDomainStatusResult(buildBundleValue, nil)
					return shared.PrintOutput(result, *output.Output, *output.Pretty)
				}
				return fmt.Errorf("build-bundles app-clip cache-status get: failed to fetch: %w", err)
			}

			result := asc.NewAppClipDomainStatusResult(buildBundleValue, resp)
			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

// BuildBundlesAppClipDebugStatusCommand returns the debug-status command group.
func BuildBundlesAppClipDebugStatusCommand() *ffcli.Command {
	fs := flag.NewFlagSet("debug-status", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "debug-status",
		ShortUsage: "asc build-bundles app-clip debug-status <subcommand> [flags]",
		ShortHelp:  "Fetch App Clip domain debug status.",
		LongHelp: `Fetch App Clip domain debug status.

Examples:
  asc build-bundles app-clip debug-status get --id "BUILD_BUNDLE_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			BuildBundlesAppClipDebugStatusGetCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// BuildBundlesAppClipDebugStatusGetCommand returns the debug-status get subcommand.
func BuildBundlesAppClipDebugStatusGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("get", flag.ExitOnError)

	buildBundleID := fs.String("id", "", "Build bundle ID")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc build-bundles app-clip debug-status get --id \"BUILD_BUNDLE_ID\"",
		ShortHelp:  "Get App Clip domain debug status for a build bundle.",
		LongHelp: `Get App Clip domain debug status for a build bundle.

Examples:
  asc build-bundles app-clip debug-status get --id "BUILD_BUNDLE_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			buildBundleValue := strings.TrimSpace(*buildBundleID)
			if buildBundleValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("build-bundles app-clip debug-status get: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetBuildBundleAppClipDomainDebugStatus(requestCtx, buildBundleValue)
			if err != nil {
				if asc.IsNotFound(err) {
					result := asc.NewAppClipDomainStatusResult(buildBundleValue, nil)
					return shared.PrintOutput(result, *output.Output, *output.Pretty)
				}
				return fmt.Errorf("build-bundles app-clip debug-status get: failed to fetch: %w", err)
			}

			result := asc.NewAppClipDomainStatusResult(buildBundleValue, resp)
			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

// BuildBundlesAppClipInvocationsCommand returns the invocations command group.
func BuildBundlesAppClipInvocationsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("invocations", flag.ExitOnError)

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
			BuildBundlesAppClipInvocationsListCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// BuildBundlesAppClipInvocationsListCommand returns the invocations list subcommand.
func BuildBundlesAppClipInvocationsListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("list", flag.ExitOnError)

	buildBundleID := fs.String("id", "", "Build bundle ID")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := shared.BindOutputFlags(fs)

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
				return fmt.Errorf("build-bundles app-clip invocations list: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("build-bundles app-clip invocations list: %w", err)
			}

			buildBundleValue := strings.TrimSpace(*buildBundleID)
			if buildBundleValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("build-bundles app-clip invocations list: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.BetaAppClipInvocationsOption{
				asc.WithBetaAppClipInvocationsLimit(*limit),
				asc.WithBetaAppClipInvocationsNextURL(*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithBetaAppClipInvocationsLimit(200))
				firstPage, err := client.GetBuildBundleBetaAppClipInvocations(requestCtx, buildBundleValue, paginateOpts...)
				if err != nil {
					if asc.IsNotFound(err) {
						empty := &asc.BetaAppClipInvocationsResponse{Data: []asc.Resource[asc.BetaAppClipInvocationAttributes]{}}
						return shared.PrintOutput(empty, *output.Output, *output.Pretty)
					}
					return fmt.Errorf("build-bundles app-clip invocations list: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetBuildBundleBetaAppClipInvocations(ctx, buildBundleValue, asc.WithBetaAppClipInvocationsNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("build-bundles app-clip invocations list: %w", err)
				}

				return shared.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := client.GetBuildBundleBetaAppClipInvocations(requestCtx, buildBundleValue, opts...)
			if err != nil {
				if asc.IsNotFound(err) {
					empty := &asc.BetaAppClipInvocationsResponse{Data: []asc.Resource[asc.BetaAppClipInvocationAttributes]{}}
					return shared.PrintOutput(empty, *output.Output, *output.Pretty)
				}
				return fmt.Errorf("build-bundles app-clip invocations list: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}
