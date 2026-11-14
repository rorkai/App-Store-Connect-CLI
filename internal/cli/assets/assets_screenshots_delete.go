package assets

import (
	"context"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// AssetsScreenshotsDeleteCommand returns the screenshot delete subcommand.
func AssetsScreenshotsDeleteCommand() *ffcli.Command {
	return shared.BuildConfirmDeleteCommand(shared.ConfirmDeleteCommandConfig{
		FlagSetName: "delete",
		Name:        "delete",
		ShortUsage:  "asc screenshots delete --id \"SCREENSHOT_ID\" --confirm",
		ShortHelp:   "Delete a screenshot by ID.",
		LongHelp: `Delete a screenshot by ID.

Examples:
  asc screenshots delete --id "SCREENSHOT_ID" --confirm`,
		IDFlag:                 "id",
		IDUsage:                "Screenshot ID",
		IDType:                 "appScreenshots",
		RequiredDiagnostic:     true,
		ConfirmRequiredMessage: "--confirm is required to delete",
		ErrorPrefix:            "screenshots delete",
		Delete: func(ctx context.Context, client *asc.Client, id string) error {
			return client.DeleteAppScreenshot(ctx, id)
		},
		Result: func(id string) any {
			return &asc.AssetDeleteResult{ID: id, Deleted: true}
		},
	})
}
