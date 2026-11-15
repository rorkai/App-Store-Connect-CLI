package versions

import (
	"context"
	"flag"

	"github.com/peterbourgon/ff/v3/ffcli"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// newVersionsAppClipDefaultExperienceCommand returns the app clip default experience command group.
func newVersionsAppClipDefaultExperienceCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("app-clip-default-experience", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "app-clip-default-experience",
		ShortUsage: "asc versions app-clip-default-experience <subcommand> [flags]",
		ShortHelp:  "Manage App Clip default experience for a version.",
		LongHelp: `Manage App Clip default experience for a version.

Examples:
  asc versions app-clip-default-experience view --version-id "VERSION_ID"`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newVersionsAppClipDefaultExperienceGetCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newVersionsAppClipDefaultExperienceGetCommand gets the App Clip default experience for a version.
func newVersionsAppClipDefaultExperienceGetCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		Deps:        d,
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
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.GetAppStoreVersionAppClipDefaultExperience(ctx, id)
		},
	})
}
