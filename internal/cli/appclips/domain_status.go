package appclips

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

// AppClipDomainStatusCommand returns the domain status command group.
func AppClipDomainStatusCommand() *ffcli.Command {
	fs := flag.NewFlagSet("domain-status", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "domain-status",
		ShortUsage: "asc app-clips domain-status <subcommand> [flags]",
		ShortHelp:  "Fetch App Clip domain status for a build bundle.",
		LongHelp: `Fetch App Clip domain status for a build bundle.

Examples:
  asc app-clips domain-status cache --build-bundle-id "BUILD_BUNDLE_ID"
  asc app-clips domain-status debug --build-bundle-id "BUILD_BUNDLE_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			AppClipDomainStatusCacheCommand(),
			AppClipDomainStatusDebugCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// AppClipDomainStatusCacheCommand fetches domain cache status for a build bundle.
func AppClipDomainStatusCacheCommand() *ffcli.Command {
	fs := flag.NewFlagSet("cache", flag.ExitOnError)

	buildBundleID := fs.String("build-bundle-id", "", "Build bundle ID")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "cache",
		ShortUsage: "asc app-clips domain-status cache --build-bundle-id \"BUILD_BUNDLE_ID\"",
		ShortHelp:  "Get App Clip domain cache status for a build bundle.",
		LongHelp: `Get App Clip domain cache status for a build bundle.

Examples:
  asc app-clips domain-status cache --build-bundle-id "BUILD_BUNDLE_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			buildBundleValue := strings.TrimSpace(*buildBundleID)
			if buildBundleValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --build-bundle-id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("app-clips domain-status cache: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetBuildBundleAppClipDomainCacheStatus(requestCtx, buildBundleValue)
			if err != nil {
				if asc.IsNotFound(err) {
					result := asc.NewAppClipDomainStatusResult(buildBundleValue, nil)
					return shared.PrintOutput(result, *output.Output, *output.Pretty)
				}
				return fmt.Errorf("app-clips domain-status cache: failed to fetch: %w", err)
			}

			result := asc.NewAppClipDomainStatusResult(buildBundleValue, resp)
			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

// AppClipDomainStatusDebugCommand fetches domain debug status for a build bundle.
func AppClipDomainStatusDebugCommand() *ffcli.Command {
	fs := flag.NewFlagSet("debug", flag.ExitOnError)

	buildBundleID := fs.String("build-bundle-id", "", "Build bundle ID")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "debug",
		ShortUsage: "asc app-clips domain-status debug --build-bundle-id \"BUILD_BUNDLE_ID\"",
		ShortHelp:  "Get App Clip domain debug status for a build bundle.",
		LongHelp: `Get App Clip domain debug status for a build bundle.

Examples:
  asc app-clips domain-status debug --build-bundle-id "BUILD_BUNDLE_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			buildBundleValue := strings.TrimSpace(*buildBundleID)
			if buildBundleValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --build-bundle-id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("app-clips domain-status debug: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetBuildBundleAppClipDomainDebugStatus(requestCtx, buildBundleValue)
			if err != nil {
				if asc.IsNotFound(err) {
					result := asc.NewAppClipDomainStatusResult(buildBundleValue, nil)
					return shared.PrintOutput(result, *output.Output, *output.Pretty)
				}
				return fmt.Errorf("app-clips domain-status debug: failed to fetch: %w", err)
			}

			result := asc.NewAppClipDomainStatusResult(buildBundleValue, resp)
			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}
