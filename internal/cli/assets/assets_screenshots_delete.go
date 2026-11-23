package assets

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/url"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// NewAssetsScreenshotsDeleteCommand returns the screenshot delete subcommand.
func NewAssetsScreenshotsDeleteCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("delete", flag.ExitOnError)
	id := shared.BindResourceIDFlag(fs, "id", "appScreenshots", "Screenshot ID")
	confirm := fs.Bool("confirm", false, "Confirm deletion")
	pruneLibrary := fs.Bool("prune-library", false, "Also delete the Asset Library image with the screenshot's ID when nothing else places it (requires --confirm)")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "delete",
		ShortUsage: "asc screenshots delete --id \"SCREENSHOT_ID\" --confirm [--prune-library]",
		ShortHelp:  "Delete a screenshot by ID.",
		LongHelp: `Delete a screenshot by ID.

Deleting a screenshot removes it from its screenshot set, but App Store Connect
keeps the Asset Library image it was uploaded as, under the same ID. Add
--prune-library to delete that image too. The image is deleted only when no
other placement uses it; otherwise it is kept and reported as "kept". A
screenshot with no matching library image is reported as "absent".

Examples:
  asc screenshots delete --id "SCREENSHOT_ID" --confirm
  asc screenshots delete --id "SCREENSHOT_ID" --confirm --prune-library`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			screenshotID := strings.TrimSpace(*id)
			if screenshotID == "" {
				fmt.Fprintln(d.IO.ErrOut, "Error: --id is required")
				return shared.MissingRequiredUsageError("--id")
			}
			if err := d.RequireConfirm(*confirm); err != nil {
				return err
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("screenshots delete: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()
			if err := client.DeleteAppScreenshot(requestCtx, screenshotID); err != nil {
				return fmt.Errorf("screenshots delete: %w", err)
			}

			if !*pruneLibrary {
				return d.PrintOutput(&asc.AssetDeleteResult{ID: screenshotID, Deleted: true}, *output.Output, *output.Pretty)
			}
			state, err := pruneScreenshotLibraryImage(requestCtx, client, screenshotID)
			if err != nil {
				return fmt.Errorf("screenshots delete: screenshot %s was deleted, but its Asset Library image was not: %w; to delete the image, run asc asset-library images delete --id %q --confirm", screenshotID, err, screenshotID)
			}
			result := &asc.ScreenshotDeleteResult{
				ID:           screenshotID,
				Deleted:      true,
				LibraryImage: &asc.ScreenshotLibraryImageResult{ID: screenshotID, State: state},
			}
			return d.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

// Screenshot library image outcomes of --prune-library.
const (
	screenshotLibraryImageDeleted = "deleted"
	screenshotLibraryImageKept    = "kept"
	screenshotLibraryImageAbsent  = "absent"
)

// pruneScreenshotLibraryImage deletes the Asset Library image that shares a
// deleted screenshot's ID, unless another placement still uses it.
func pruneScreenshotLibraryImage(ctx context.Context, client *asc.Client, imageID string) (string, error) {
	path := "/v1/appAssetLibraryImages/" + url.PathEscape(imageID)
	// Placements of an unknown image ID come back as an empty list, so the
	// image itself is read first to tell an absent image from an unplaced one.
	_, err := client.RawRequest(ctx, "GET", path, nil)
	if asc.IsNotFound(err) {
		return screenshotLibraryImageAbsent, nil
	}
	if err != nil {
		return "", fmt.Errorf("read it: %w", err)
	}
	raw, err := client.RawRequestWithQuery(ctx, "GET", path+"/placements", url.Values{"limit": {"1"}}, nil)
	if err != nil {
		return "", fmt.Errorf("list its placements: %w", err)
	}
	var placements struct {
		Data []json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(raw, &placements); err != nil {
		return "", fmt.Errorf("parse its placements: %w", err)
	}
	if len(placements.Data) > 0 {
		return screenshotLibraryImageKept, nil
	}
	if _, err := client.RawRequest(ctx, "DELETE", path, nil); err != nil {
		return "", err
	}
	return screenshotLibraryImageDeleted, nil
}
