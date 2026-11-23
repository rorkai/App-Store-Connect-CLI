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

Deleting the last screenshot in a set leaves the empty set behind, which
asc validate and asc review doctor report as screenshots.required.set_nonempty.
To remove the set itself, for example so App Store Connect uses a larger
display size's screenshots instead, run:
  asc localizations screenshot-sets delete --id "SET_ID" --confirm

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
