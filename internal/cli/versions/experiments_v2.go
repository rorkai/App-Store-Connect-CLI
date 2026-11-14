package versions

import (
	"context"
	"flag"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// VersionsExperimentsV2Command returns the experiments v2 command group.
func VersionsExperimentsV2Command() *ffcli.Command {
	fs := flag.NewFlagSet("experiments-v2", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "experiments-v2",
		ShortUsage: "asc versions experiments-v2 <subcommand> [flags]",
		ShortHelp:  "Manage App Store version experiments (v2).",
		LongHelp: `Manage App Store version experiments (v2).

Examples:
  asc versions experiments-v2 list --version-id "VERSION_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			VersionsExperimentsV2ListCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// VersionsExperimentsV2ListCommand lists v2 experiments for a version.
func VersionsExperimentsV2ListCommand() *ffcli.Command {
	return shared.BuildPaginatedListCommand(shared.PaginatedListCommandConfig{
		FlagSetName: "experiments-v2 list",
		Name:        "list",
		ShortUsage:  "asc versions experiments-v2 list --version-id \"VERSION_ID\" [flags]",
		ShortHelp:   "List v2 experiments for an app store version.",
		LongHelp: `List v2 experiments for an app store version.

Examples:
  asc versions experiments-v2 list --version-id "VERSION_ID"
  asc versions experiments-v2 list --version-id "VERSION_ID" --paginate`,
		ParentFlag:           "version-id",
		ParentUsage:          "App Store version ID",
		ParentType:           "appStoreVersions",
		RequiredDiagnostic:   true,
		PaginateIgnoresLimit: true,
		NoSpinner:            true,
		ErrorPrefix:          "versions experiments-v2 list",
		FetchErrorPrefix:     "versions experiments-v2 list: failed to fetch",
		FetchPage: func(ctx context.Context, client *asc.Client, parentID string, limit int, next string) (asc.PaginatedResponse, error) {
			return client.GetAppStoreVersionExperimentsV2ForVersion(ctx, parentID,
				asc.Limit[asc.AppStoreVersionExperimentsV2Option](limit),
				asc.NextURL[asc.AppStoreVersionExperimentsV2Option](next),
			)
		},
	})
}
