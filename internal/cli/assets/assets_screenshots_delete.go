package assets

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// AssetsScreenshotsDeleteCommand returns the screenshot delete subcommand.
func AssetsScreenshotsDeleteCommand() *ffcli.Command {
	fs := flag.NewFlagSet("delete", flag.ExitOnError)

	id := fs.String("id", "", "Screenshot ID")
	confirm := fs.Bool("confirm", false, "Confirm deletion")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "delete",
		ShortUsage: "asc screenshots delete --id \"SCREENSHOT_ID\" --confirm",
		ShortHelp:  "Delete a screenshot by ID.",
		LongHelp: `Delete a screenshot by ID.

Examples:
  asc screenshots delete --id "SCREENSHOT_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			assetID := strings.TrimSpace(*id)
			if assetID == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return shared.MissingRequiredUsageError()
			}
			if !*confirm {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required to delete")
				return shared.MissingRequiredUsageError()
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("screenshots delete: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			if err := client.DeleteAppScreenshot(requestCtx, assetID); err != nil {
				return fmt.Errorf("screenshots delete: %w", err)
			}

			result := asc.AssetDeleteResult{
				ID:      assetID,
				Deleted: true,
			}

			return shared.PrintOutput(&result, *output.Output, *output.Pretty)
		},
	}
}
