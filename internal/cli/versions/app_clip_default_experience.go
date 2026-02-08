package versions

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"
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
  asc versions app-clip-default-experience get --version-id "VERSION_ID"`,
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
	fs := flag.NewFlagSet("app-clip-default-experience get", flag.ExitOnError)

	versionID := fs.String("version-id", "", "App Store version ID")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc versions app-clip-default-experience get --version-id \"VERSION_ID\"",
		ShortHelp:  "Get App Clip default experience for an app store version.",
		LongHelp: `Get App Clip default experience for an app store version.

Examples:
  asc versions app-clip-default-experience get --version-id "VERSION_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			versionValue := strings.TrimSpace(*versionID)
			if versionValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --version-id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("versions app-clip-default-experience get: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetAppStoreVersionAppClipDefaultExperience(requestCtx, versionValue)
			if err != nil {
				return fmt.Errorf("versions app-clip-default-experience get: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}
