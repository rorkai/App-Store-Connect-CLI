package versions

import (
	"context"
	"flag"

	"github.com/peterbourgon/ff/v3/ffcli"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// VersionsAppClipDefaultExperienceCommand returns the app clip default experience command group.
func VersionsAppClipDefaultExperienceCommand() *ffcli.Command {
	fs := flag.NewFlagSet("app-clip-default-experience", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "app-clip-default-experience",
		ShortUsage: "asc versions app-clip-default-experience <subcommand> [flags]",
		ShortHelp:  "Manage App Clip default experience for a version.",
		LongHelp: `Manage App Clip default experience for a version.

Examples:
  asc versions app-clip-default-experience view --version-id "VERSION_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			VersionsAppClipDefaultExperienceGetCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// VersionsAppClipDefaultExperienceGetCommand gets the App Clip default experience for a version.
func VersionsAppClipDefaultExperienceGetCommand() *ffcli.Command {
	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		FlagSetName: "app-clip-default-experience view",
		Name:        "view",
		ShortUsage:  "asc versions app-clip-default-experience view --version-id \"VERSION_ID\"",
		ShortHelp:   "View App Clip default experience for an app store version.",
		LongHelp: `View App Clip default experience for an app store version.

Examples:
  asc versions app-clip-default-experience view --version-id "VERSION_ID"`,
		IDFlag:             "version-id",
		IDUsage:            "App Store version ID",
		IDType:             "appStoreVersions",
		RequiredDiagnostic: true,
		ErrorPrefix:        "versions app-clip-default-experience view",
		FetchErrorPrefix:   "versions app-clip-default-experience view: failed to fetch",
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.GetAppStoreVersionAppClipDefaultExperience(ctx, id)
		},
	})
}
