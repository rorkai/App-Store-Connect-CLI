package appclips

import (
	"context"
	"flag"
	"fmt"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// newAppClipDomainStatusCommand returns the domain status command group.
func newAppClipDomainStatusCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("domain-status", flag.ExitOnError)

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
			newAppClipDomainStatusCacheCommand(d),
			newAppClipDomainStatusDebugCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newAppClipDomainStatusCacheCommand fetches domain cache status for a build bundle.
func newAppClipDomainStatusCacheCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("cache", flag.ExitOnError)

	buildBundleID := fs.String("build-bundle-id", "", "Build bundle ID")
	output := d.BindOutputFlags(fs)

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
				return d.RequiredFlag("--build-bundle-id")
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("app-clips domain-status cache: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			resp, err := client.GetBuildBundleAppClipDomainCacheStatus(requestCtx, buildBundleValue)
			if err != nil {
				if asc.IsNotFound(err) {
					result := asc.NewAppClipDomainStatusResult(buildBundleValue, nil)
					return d.PrintOutput(result, *output.Output, *output.Pretty)
				}
				return fmt.Errorf("app-clips domain-status cache: failed to fetch: %w", err)
			}

			result := asc.NewAppClipDomainStatusResult(buildBundleValue, resp)
			return d.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

// newAppClipDomainStatusDebugCommand fetches domain debug status for a build bundle.
func newAppClipDomainStatusDebugCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("debug", flag.ExitOnError)

	buildBundleID := fs.String("build-bundle-id", "", "Build bundle ID")
	output := d.BindOutputFlags(fs)

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
				return d.RequiredFlag("--build-bundle-id")
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("app-clips domain-status debug: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			resp, err := client.GetBuildBundleAppClipDomainDebugStatus(requestCtx, buildBundleValue)
			if err != nil {
				if asc.IsNotFound(err) {
					result := asc.NewAppClipDomainStatusResult(buildBundleValue, nil)
					return d.PrintOutput(result, *output.Output, *output.Pretty)
				}
				return fmt.Errorf("app-clips domain-status debug: failed to fetch: %w", err)
			}

			result := asc.NewAppClipDomainStatusResult(buildBundleValue, resp)
			return d.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}
