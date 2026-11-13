package assetlibrary

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

var libraryTimeCode = regexp.MustCompile(`^[0-9]{2}:[0-5][0-9]:[0-5][0-9](:[0-9]{2}|\.[0-9]{3})$`)

func lifecycleCommand(action, resourceType string) *ffcli.Command {
	fs := flag.NewFlagSet(action, flag.ExitOnError)
	id := shared.BindResourceIDFlag(fs, "id", resourceType, "Asset ID or API self-link")
	var name, timeCode *string
	if action == "rename" {
		name = fs.String("name", "", "Internal reference name (required)")
	}
	if action == "set-poster-frame" {
		timeCode = fs.String("time-code", "", "Poster frame timecode HH:MM:SS:FF or HH:MM:SS.mmm (required)")
	}
	var confirm *bool
	if action == "archive" || action == "delete" {
		confirm = fs.Bool("confirm", false, "Confirm this operation (required)")
	}
	output := shared.BindOutputFlags(fs)
	groupName := "images"
	if resourceType == "appAssetLibraryVideos" {
		groupName = "videos"
	}
	usage := fmt.Sprintf("asc asset-library %s %s --id ID", groupName, action)
	if name != nil {
		usage += " --name NAME"
	}
	if timeCode != nil {
		usage += " --time-code TIMECODE"
	}
	if confirm != nil {
		usage += " --confirm"
	}
	help := "Manage an Asset Library asset."
	switch action {
	case "archive":
		help = "Only approved assets can be archived. Archived assets cannot be used in new placements."
	case "unarchive":
		help = "Restore an archived asset so it can be used in new placements."
	case "delete":
		help = "Delete an eligible asset from Asset Library. To remove an asset from a placement while keeping it in the library, delete the placement."
	case "set-poster-frame":
		help = "Changing the poster frame affects every placement using this video. Poster frames cannot be edited after the asset is approved."
	}
	return &ffcli.Command{
		Name: action, ShortUsage: usage, ShortHelp: fmt.Sprintf("%s an Asset Library asset.", strings.ToUpper(action[:1])+action[1:]),
		LongHelp: help + " Apple validates asset eligibility. This command does not submit App Review.",
		FlagSet:  fs, UsageFunc: shared.DefaultUsageFunc, Exec: func(ctx context.Context, args []string) error {
			if err := shared.ValidateBoundOutputFlags(fs); err != nil {
				return shared.UsageErrorf("asset-library %s: %v", action, err)
			}
			assetID := strings.TrimSpace(*id)
			if assetID == "" {
				return shared.UsageError("asset-library: --id is required")
			}
			if !resourceIDPattern.MatchString(assetID) {
				return shared.UsageError("asset-library: --id must be a resource ID")
			}
			if confirm != nil && !*confirm {
				return shared.UsageError("asset-library: --confirm is required")
			}
			result := asc.AssetLibraryMutationResult{AssetID: assetID, AssetType: resourceType, Action: action}
			attrs := map[string]any{}
			switch action {
			case "rename":
				value := strings.TrimSpace(*name)
				if value == "" {
					return shared.UsageError("asset-library rename: --name is required")
				}
				attrs["referenceName"] = value
				result.ReferenceName = value
			case "archive", "unarchive":
				value := action == "archive"
				attrs["archived"] = value
				result.Archived = &value
			case "set-poster-frame":
				value := strings.TrimSpace(*timeCode)
				if !libraryTimeCode.MatchString(value) {
					return shared.UsageError("asset-library set-poster-frame: --time-code must be HH:MM:SS:FF or HH:MM:SS.mmm")
				}
				attrs["previewFrameTimeCode"] = value
				result.PreviewFrameTimeCode = value
			}
			var body []byte
			method := "DELETE"
			if action != "delete" {
				method = "PATCH"
				payload := struct {
					Data struct {
						Type       string         `json:"type"`
						ID         string         `json:"id"`
						Attributes map[string]any `json:"attributes"`
					} `json:"data"`
				}{}
				payload.Data.Type = resourceType
				payload.Data.ID = assetID
				payload.Data.Attributes = attrs
				var err error
				body, err = json.Marshal(payload)
				if err != nil {
					return err
				}
			}
			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("asset-library %s: %w", action, err)
			}
			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()
			raw, err := client.RawRequest(requestCtx, method, "/v1/"+resourceType+"/"+url.PathEscape(assetID), body)
			if err != nil {
				return fmt.Errorf("asset-library %s: %w", action, err)
			}
			if method == "PATCH" {
				var response struct{ Data struct{ Type, ID string } }
				if err := json.Unmarshal(raw, &response); err != nil {
					return fmt.Errorf("asset-library %s: parse response: %w", action, err)
				}
				if response.Data.Type != resourceType || response.Data.ID != assetID {
					return fmt.Errorf("asset-library %s: response does not identify updated asset", action)
				}
			}
			return shared.PrintOutput(&result, *output.Output, *output.Pretty)
		},
	}
}
