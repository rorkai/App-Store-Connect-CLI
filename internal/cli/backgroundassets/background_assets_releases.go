package backgroundassets

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// BackgroundAssetsAppStoreReleasesCommand returns the App Store releases command group.
func BackgroundAssetsAppStoreReleasesCommand() *ffcli.Command {
	fs := flag.NewFlagSet("app-store-releases", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "app-store-releases",
		ShortUsage: "asc background-assets app-store-releases <subcommand> [flags]",
		ShortHelp:  "Get App Store releases for background assets.",
		LongHelp: `Get App Store releases for background assets.

Examples:
  asc background-assets app-store-releases get --id "RELEASE_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			BackgroundAssetsAppStoreReleasesGetCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// BackgroundAssetsAppStoreReleasesGetCommand returns the App Store releases get subcommand.
func BackgroundAssetsAppStoreReleasesGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("get", flag.ExitOnError)

	id := fs.String("id", "", "Release ID")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc background-assets app-store-releases get --id \"RELEASE_ID\"",
		ShortHelp:  "Get a background asset App Store release.",
		LongHelp: `Get a background asset App Store release.

Examples:
  asc background-assets app-store-releases get --id "RELEASE_ID"`,
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
				return fmt.Errorf("background-assets app-store-releases get: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetBackgroundAssetVersionAppStoreRelease(requestCtx, idValue)
			if err != nil {
				return fmt.Errorf("background-assets app-store-releases get: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// BackgroundAssetsExternalBetaReleasesCommand returns the external beta releases command group.
func BackgroundAssetsExternalBetaReleasesCommand() *ffcli.Command {
	fs := flag.NewFlagSet("external-beta-releases", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "external-beta-releases",
		ShortUsage: "asc background-assets external-beta-releases <subcommand> [flags]",
		ShortHelp:  "Get external beta releases for background assets.",
		LongHelp: `Get external beta releases for background assets.

Examples:
  asc background-assets external-beta-releases get --id "RELEASE_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			BackgroundAssetsExternalBetaReleasesGetCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// BackgroundAssetsExternalBetaReleasesGetCommand returns the external beta releases get subcommand.
func BackgroundAssetsExternalBetaReleasesGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("get", flag.ExitOnError)

	id := fs.String("id", "", "Release ID")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc background-assets external-beta-releases get --id \"RELEASE_ID\"",
		ShortHelp:  "Get a background asset external beta release.",
		LongHelp: `Get a background asset external beta release.

Examples:
  asc background-assets external-beta-releases get --id "RELEASE_ID"`,
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
				return fmt.Errorf("background-assets external-beta-releases get: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetBackgroundAssetVersionExternalBetaRelease(requestCtx, idValue)
			if err != nil {
				return fmt.Errorf("background-assets external-beta-releases get: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// BackgroundAssetsInternalBetaReleasesCommand returns the internal beta releases command group.
func BackgroundAssetsInternalBetaReleasesCommand() *ffcli.Command {
	fs := flag.NewFlagSet("internal-beta-releases", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "internal-beta-releases",
		ShortUsage: "asc background-assets internal-beta-releases <subcommand> [flags]",
		ShortHelp:  "Get internal beta releases for background assets.",
		LongHelp: `Get internal beta releases for background assets.

Examples:
  asc background-assets internal-beta-releases get --id "RELEASE_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			BackgroundAssetsInternalBetaReleasesGetCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// BackgroundAssetsInternalBetaReleasesGetCommand returns the internal beta releases get subcommand.
func BackgroundAssetsInternalBetaReleasesGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("get", flag.ExitOnError)

	id := fs.String("id", "", "Release ID")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc background-assets internal-beta-releases get --id \"RELEASE_ID\"",
		ShortHelp:  "Get a background asset internal beta release.",
		LongHelp: `Get a background asset internal beta release.

Examples:
  asc background-assets internal-beta-releases get --id "RELEASE_ID"`,
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
				return fmt.Errorf("background-assets internal-beta-releases get: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetBackgroundAssetVersionInternalBetaRelease(requestCtx, idValue)
			if err != nil {
				return fmt.Errorf("background-assets internal-beta-releases get: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}
