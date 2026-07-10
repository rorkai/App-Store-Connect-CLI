package appclips

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"
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
	fs := flag.NewFlagSet("header-image view", flag.ExitOnError)

	localizationID := fs.String("localization-id", "", "Default experience localization ID")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "view",
		ShortUsage: "asc app-clips default-experiences header-image view --localization-id \"LOCALIZATION_ID\"",
		ShortHelp:  "View the header image for a localization.",
		LongHelp: `View the header image for a localization.

Examples:
  asc app-clips default-experiences header-image view --localization-id "LOCALIZATION_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			locValue := strings.TrimSpace(*localizationID)
			if locValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --localization-id is required")
				return shared.MissingRequiredUsageError()
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("app-clips default-experiences header-image view: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetAppClipDefaultExperienceLocalizationHeaderImage(requestCtx, locValue)
			if err != nil {
				return fmt.Errorf("app-clips default-experiences header-image view: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}
