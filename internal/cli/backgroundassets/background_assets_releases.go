package backgroundassets

import (
	"context"
	"flag"

	"github.com/peterbourgon/ff/v3/ffcli"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func newBackgroundAssetsAppStoreReleasesCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("app-store-releases", flag.ExitOnError)

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
			newBackgroundAssetsAppStoreReleasesGetCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

func newBackgroundAssetsAppStoreReleasesGetCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		Deps:       d,
		Name:       "view",
		ShortUsage: "asc background-assets app-store-releases view --id \"RELEASE_ID\"",
		ShortHelp:  "View a background asset App Store release.",
		LongHelp: `View a background asset App Store release.

Examples:
  asc background-assets app-store-releases view --id "RELEASE_ID"`,
		IDUsage:            "Release ID",
		IDType:             "backgroundAssetVersionAppStoreReleases",
		RequiredDiagnostic: true,
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.GetBackgroundAssetVersionAppStoreRelease(ctx, id)
		},
	})
}

func newBackgroundAssetsExternalBetaReleasesCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("external-beta-releases", flag.ExitOnError)

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
			newBackgroundAssetsExternalBetaReleasesGetCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

func newBackgroundAssetsExternalBetaReleasesGetCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		Deps:       d,
		Name:       "view",
		ShortUsage: "asc background-assets external-beta-releases view --id \"RELEASE_ID\"",
		ShortHelp:  "View a background asset external beta release.",
		LongHelp: `View a background asset external beta release.

Examples:
  asc background-assets external-beta-releases view --id "RELEASE_ID"`,
		IDUsage:            "Release ID",
		IDType:             "backgroundAssetVersionExternalBetaReleases",
		RequiredDiagnostic: true,
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.GetBackgroundAssetVersionExternalBetaRelease(ctx, id)
		},
	})
}

func newBackgroundAssetsInternalBetaReleasesCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("internal-beta-releases", flag.ExitOnError)

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
			newBackgroundAssetsInternalBetaReleasesGetCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

func newBackgroundAssetsInternalBetaReleasesGetCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		Deps:       d,
		Name:       "view",
		ShortUsage: "asc background-assets internal-beta-releases view --id \"RELEASE_ID\"",
		ShortHelp:  "View a background asset internal beta release.",
		LongHelp: `View a background asset internal beta release.

Examples:
  asc background-assets internal-beta-releases view --id "RELEASE_ID"`,
		IDUsage:            "Release ID",
		IDType:             "backgroundAssetVersionInternalBetaReleases",
		RequiredDiagnostic: true,
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.GetBackgroundAssetVersionInternalBetaRelease(ctx, id)
		},
	})
}
