package appclips

import (
	"context"
	"flag"

	"github.com/peterbourgon/ff/v3/ffcli"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// AppClipDefaultExperienceHeaderImageCommand returns the default experience header image command group.
func AppClipDefaultExperienceHeaderImageCommand() *ffcli.Command {
	fs := flag.NewFlagSet("header-image", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "header-image",
		ShortUsage: "asc app-clips default-experiences header-image <subcommand> [flags]",
		ShortHelp:  "Manage default experience header images.",
		LongHelp: `Manage default experience header images.

Examples:
  asc app-clips default-experiences header-image view --localization-id "LOCALIZATION_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			AppClipDefaultExperienceHeaderImageGetCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// AppClipDefaultExperienceHeaderImageGetCommand retrieves the header image for a localization.
func AppClipDefaultExperienceHeaderImageGetCommand() *ffcli.Command {
	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		FlagSetName: "header-image view",
		Name:        "view",
		ShortUsage:  "asc app-clips default-experiences header-image view --localization-id \"LOCALIZATION_ID\"",
		ShortHelp:   "View the header image for a localization.",
		LongHelp: `View the header image for a localization.

Examples:
  asc app-clips default-experiences header-image view --localization-id "LOCALIZATION_ID"`,
		IDFlag:             "localization-id",
		IDUsage:            "Default experience localization ID",
		IDType:             "appClipDefaultExperienceLocalizations",
		RequiredDiagnostic: true,
		ErrorPrefix:        "app-clips default-experiences header-image view",
		FetchErrorPrefix:   "app-clips default-experiences header-image view: failed to fetch",
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.GetAppClipDefaultExperienceLocalizationHeaderImage(ctx, id)
		},
	})
}
