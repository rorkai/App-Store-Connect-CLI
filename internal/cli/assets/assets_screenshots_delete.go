package assets

import (
	"context"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// NewAssetsScreenshotsDeleteCommand returns the screenshot delete subcommand.
func NewAssetsScreenshotsDeleteCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildConfirmDeleteCommand(shared.ConfirmDeleteCommandConfig{
		Deps:       d,
		Name:       "delete",
		ShortUsage: "asc screenshots delete --id \"SCREENSHOT_ID\" --confirm",
		ShortHelp:  "Delete a screenshot by ID.",
		LongHelp: `Delete a screenshot by ID.

Examples:
  asc screenshots delete --id "SCREENSHOT_ID" --confirm`,
		IDFlag:             "id",
		IDUsage:            "Screenshot ID",
		IDType:             "appScreenshots",
		RequiredDiagnostic: true,
		DeleteErrorPrefix:  "screenshots delete",
		Delete: func(ctx context.Context, client *asc.Client, id string) error {
			return client.DeleteAppScreenshot(ctx, id)
		},
		Result: func(id string) any {
			return &asc.AssetDeleteResult{ID: id, Deleted: true}
		},
	})
}
