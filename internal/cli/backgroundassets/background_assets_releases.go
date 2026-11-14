package backgroundassets

import (
	"context"
	"flag"

	"github.com/peterbourgon/ff/v3/ffcli"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
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
  asc background-assets app-store-releases view --id "RELEASE_ID"`,
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
	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		FlagSetName: "view",
		Name:        "view",
		ShortUsage:  "asc background-assets app-store-releases view --id \"RELEASE_ID\"",
		ShortHelp:   "View a background asset App Store release.",
		LongHelp: `View a background asset App Store release.

Examples:
  asc background-assets app-store-releases view --id "RELEASE_ID"`,
		IDUsage:            "Release ID",
		IDType:             "backgroundAssetVersionAppStoreReleases",
		RequiredDiagnostic: true,
		ErrorPrefix:        "background-assets app-store-releases view",
		FetchErrorPrefix:   "background-assets app-store-releases view: failed to fetch",
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.GetBackgroundAssetVersionAppStoreRelease(ctx, id)
		},
	})
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
  asc background-assets external-beta-releases view --id "RELEASE_ID"`,
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
	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		FlagSetName: "view",
		Name:        "view",
		ShortUsage:  "asc background-assets external-beta-releases view --id \"RELEASE_ID\"",
		ShortHelp:   "View a background asset external beta release.",
		LongHelp: `View a background asset external beta release.

Examples:
  asc background-assets external-beta-releases view --id "RELEASE_ID"`,
		IDUsage:            "Release ID",
		IDType:             "backgroundAssetVersionExternalBetaReleases",
		RequiredDiagnostic: true,
		ErrorPrefix:        "background-assets external-beta-releases view",
		FetchErrorPrefix:   "background-assets external-beta-releases view: failed to fetch",
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.GetBackgroundAssetVersionExternalBetaRelease(ctx, id)
		},
	})
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
  asc background-assets internal-beta-releases view --id "RELEASE_ID"`,
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
	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		FlagSetName: "view",
		Name:        "view",
		ShortUsage:  "asc background-assets internal-beta-releases view --id \"RELEASE_ID\"",
		ShortHelp:   "View a background asset internal beta release.",
		LongHelp: `View a background asset internal beta release.

Examples:
  asc background-assets internal-beta-releases view --id "RELEASE_ID"`,
		IDUsage:            "Release ID",
		IDType:             "backgroundAssetVersionInternalBetaReleases",
		RequiredDiagnostic: true,
		ErrorPrefix:        "background-assets internal-beta-releases view",
		FetchErrorPrefix:   "background-assets internal-beta-releases view: failed to fetch",
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.GetBackgroundAssetVersionInternalBetaRelease(ctx, id)
		},
	})
}
