package shared

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
)

var creativePlacementID = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

func creativePlacementCreateCommand(resource, prefix, label string) *ffcli.Command {
	return creativePlacementWriteCommand(resource, prefix, label, false)
}

func creativePlacementSwapCommand(resource, prefix, label string) *ffcli.Command {
	return creativePlacementWriteCommand(resource, prefix, label, true)
}

func creativePlacementWriteCommand(resource, prefix, label string, swap bool) *ffcli.Command {
	operation := "create"
	if swap {
		operation = "swap"
	}
	fs := flag.NewFlagSet("placements "+operation, flag.ExitOnError)
	loc := BindResourceIDFlag(fs, "localization-id", resource, label+" ID or API self-link")
	image := BindResourceIDFlag(fs, "image-id", "appAssetLibraryImages", "Uploaded asset library image ID or API self-link")
	video := BindResourceIDFlag(fs, "video-id", "appAssetLibraryVideos", "Uploaded asset library video ID or API self-link")
	kind := fs.String("placement-type", "", "Placement type: "+strings.Join(creativePlacementTypes(resource), ", "))
	var group, previous *string
	var confirm *bool
	if swap {
		previous = BindResourceIDFlag(fs, "placement-id", "appAssetLibraryPlacements", "Existing placement ID or API self-link to replace")
		confirm = fs.Bool("confirm", false, "Confirm replacement of the existing placement")
	} else {
		group = fs.String("placement-group", "", "Device placement group for screenshots/previews; creative assets use DEFAULT_PROFILE")
	}
	output := BindOutputFlags(fs)
	usage := "asc " + prefix + " placements " + operation + " --localization-id ID (--image-id ID | --video-id ID) --placement-type TYPE"
	shortHelp := "Assign an existing asset library image or video."
	longHelp := "Assign library media to " + label + ". Supply exactly one image or video ID. Screenshots require images and previews require videos, with an explicit device placement group such as IPHONE_DUO_PROFILE. Header, search, and event creative assets use DEFAULT_PROFILE. Apple validates media compatibility and slot availability. This does not submit for review or promise publication; no existing placement is removed automatically."
	exampleSuffix := ""
	if swap {
		usage += " --placement-id ID --confirm"
		shortHelp = "Replace an existing placement using one asset library swap request."
		longHelp = "Replace an existing placement for " + label + " by linking placementToSwapOut in one POST request. This relationship is a live-verified extension absent from the published OpenAPI 4.5.1 create schema. Supply exactly one image or video ID and the replacement placement type. The replacement must have the same media type as the existing placement. Screenshots require images; previews require videos. The replacement keeps the existing placement's group: the command reads it from the existing placement and sends it with the request, because Apple requires the two groups to match. Apple validates the existing placement, context, media compatibility, and editability. Replacement requires --confirm. This does not submit for review or promise publication. There is no separate delete or fallback removal if the request fails."
		exampleSuffix = " --placement-id OLD_PLACEMENT_ID --confirm"
	} else {
		usage += " [--placement-group GROUP]"
	}
	longHelp += "\n\nExamples:\n  asc " + prefix + " placements " + operation + " --localization-id ID --image-id IMAGE_ID --placement-type " + creativePlacementTypes(resource)[0] + exampleSuffix + "\n  asc " + prefix + " placements " + operation + " --localization-id ID --video-id VIDEO_ID --placement-type " + creativePlacementTypes(resource)[0] + exampleSuffix
	return &ffcli.Command{
		Name: operation, ShortHelp: shortHelp, ShortUsage: usage, FlagSet: fs, UsageFunc: DefaultUsageFunc, LongHelp: longHelp,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return UsageErrorf("placements "+operation+": unexpected argument %q", args[0])
			}
			if err := ValidateBoundOutputFlags(fs); err != nil {
				return UsageErrorf("placements "+operation+": %v", err)
			}
			localizationID, imageID, videoID := strings.TrimSpace(*loc), strings.TrimSpace(*image), strings.TrimSpace(*video)
			if !creativePlacementID.MatchString(localizationID) {
				return UsageError("placements " + operation + ": --localization-id requires a resource ID")
			}
			if (imageID == "") == (videoID == "") {
				return UsageError("placements " + operation + ": supply exactly one of --image-id or --video-id")
			}
			mediaID, mediaType, mediaRelationship := imageID, "appAssetLibraryImages", "image"
			if videoID != "" {
				mediaID, mediaType, mediaRelationship = videoID, "appAssetLibraryVideos", "video"
			}
			if !creativePlacementID.MatchString(mediaID) {
				return UsageError("placements " + operation + ": media ID must be a resource ID")
			}
			placementType, err := validatedPlacementCSV(*kind, creativePlacementTypes(resource))
			if err != nil || placementType == "" || strings.Contains(placementType, ",") {
				return UsageError("placements " + operation + ": --placement-type must be one of " + strings.Join(creativePlacementTypes(resource), ", "))
			}
			native := placementType == "APP_SCREENSHOT" || placementType == "IMESSAGE_APP_SCREENSHOT" || placementType == "APP_PREVIEW"
			if placementType == "APP_PREVIEW" && videoID == "" {
				return UsageError("placements " + operation + ": APP_PREVIEW requires --video-id")
			}
			if (placementType == "APP_SCREENSHOT" || placementType == "IMESSAGE_APP_SCREENSHOT") && imageID == "" {
				return UsageError("placements " + operation + ": screenshots require --image-id")
			}
			previousID, placementGroup := "", ""
			if swap {
				previousID = strings.TrimSpace(*previous)
				if !creativePlacementID.MatchString(previousID) {
					return UsageError("placements swap: --placement-id requires a resource ID")
				}
				if !*confirm {
					return UsageError("placements swap: --confirm is required")
				}
			} else {
				placementGroup = strings.TrimSpace(*group)
			}
			if !swap && native {
				if !creativePlacementID.MatchString(placementGroup) || placementGroup == "DEFAULT_PROFILE" {
					return UsageError("placements " + operation + ": screenshots/previews require a device --placement-group")
				}
			} else if !swap {
				if placementGroup != "" && placementGroup != "DEFAULT_PROFILE" {
					return UsageError("placements " + operation + ": creative assets require DEFAULT_PROFILE")
				}
				placementGroup = "DEFAULT_PROFILE"
			}
			relationship := func(resource, id string) map[string]any {
				return map[string]any{"data": map[string]string{"type": resource, "id": id}}
			}
			attributes := map[string]string{"placementType": placementType}
			relationships := map[string]any{mediaRelationship: relationship(mediaType, mediaID), creativePlacementRelationship(resource): relationship(resource, localizationID)}
			if swap {
				relationships["placementToSwapOut"] = relationship("appAssetLibraryPlacements", previousID)
			} else {
				attributes["placementGroup"] = placementGroup
			}
			client, err := GetASCClient()
			if err != nil {
				return fmt.Errorf("placements "+operation+": %w", err)
			}
			if swap {
				// Apple rejects a swap whose placementGroup differs from the
				// placement being replaced, and requires it for screenshots.
				placementGroup, err = existingPlacementGroup(ctx, client, previousID)
				if err != nil {
					return fmt.Errorf("placements swap: read placement %s: %w", previousID, err)
				}
				if placementGroup != "" {
					attributes["placementGroup"] = placementGroup
				}
			}
			payload, err := json.Marshal(map[string]any{"data": map[string]any{"type": "appAssetLibraryPlacements", "attributes": attributes, "relationships": relationships}})
			if err != nil {
				return err
			}
			placementID, state, err := postLibraryPlacement(ctx, client, payload)
			if err != nil {
				return fmt.Errorf("placements %s: %w", operation, err)
			}
			if swap {
				return PrintOutput(&asc.CreativePlacementSwapResult{ID: placementID, Swapped: true, PreviousPlacementID: previousID, LocalizationID: localizationID, ImageID: imageID, VideoID: videoID, PlacementType: placementType, PlacementGroup: placementGroup, State: state}, *output.Output, *output.Pretty)
			}
			return PrintOutput(&asc.CreativePlacementCreateResult{ID: placementID, Created: true, LocalizationID: localizationID, ImageID: imageID, VideoID: videoID, PlacementType: placementType, PlacementGroup: placementGroup, State: state}, *output.Output, *output.Pretty)
		},
	}
}

// CreateLibraryPlacement assigns uploaded library media to one localization.
// mediaRelationship is "image" or "video".
func CreateLibraryPlacement(ctx context.Context, client *asc.Client, resource, localizationID, mediaRelationship, mediaID, placementType, placementGroup string) (id, state string, err error) {
	mediaType := "appAssetLibraryImages"
	if mediaRelationship == "video" {
		mediaType = "appAssetLibraryVideos"
	}
	relationship := func(resource, id string) map[string]any {
		return map[string]any{"data": map[string]string{"type": resource, "id": id}}
	}
	payload, err := json.Marshal(map[string]any{"data": map[string]any{
		"type":          "appAssetLibraryPlacements",
		"attributes":    map[string]string{"placementType": placementType, "placementGroup": placementGroup},
		"relationships": map[string]any{mediaRelationship: relationship(mediaType, mediaID), creativePlacementRelationship(resource): relationship(resource, localizationID)},
	}})
	if err != nil {
		return "", "", err
	}
	return postLibraryPlacement(ctx, client, payload)
}

func postLibraryPlacement(ctx context.Context, client *asc.Client, payload []byte) (id, state string, err error) {
	requestCtx, cancel := ContextWithTimeout(ctx)
	defer cancel()
	response, err := client.RawRequest(requestCtx, "POST", "/v1/appAssetLibraryPlacements", payload)
	if err != nil {
		return "", "", err
	}
	var envelope struct {
		Data struct {
			ID         string `json:"id"`
			Type       string `json:"type"`
			Attributes struct {
				State string `json:"state"`
			} `json:"attributes"`
		} `json:"data"`
	}
	if err := json.Unmarshal(response, &envelope); err != nil {
		return "", "", fmt.Errorf("invalid response: %w", err)
	}
	if envelope.Data.ID == "" || envelope.Data.Type != "appAssetLibraryPlacements" {
		return "", "", fmt.Errorf("invalid placement response")
	}
	return envelope.Data.ID, envelope.Data.Attributes.State, nil
}

// existingPlacementGroup reads the placement group of an existing placement.
func existingPlacementGroup(ctx context.Context, client *asc.Client, placementID string) (string, error) {
	requestCtx, cancel := ContextWithTimeout(ctx)
	defer cancel()
	response, err := client.RawRequest(requestCtx, "GET", "/v1/appAssetLibraryPlacements/"+url.PathEscape(placementID), nil)
	if err != nil {
		return "", err
	}
	var envelope struct {
		Data struct {
			ID         string `json:"id"`
			Type       string `json:"type"`
			Attributes struct {
				PlacementGroup string `json:"placementGroup"`
			} `json:"attributes"`
		} `json:"data"`
	}
	if err := json.Unmarshal(response, &envelope); err != nil {
		return "", fmt.Errorf("invalid response: %w", err)
	}
	if envelope.Data.ID != placementID || envelope.Data.Type != "appAssetLibraryPlacements" {
		return "", fmt.Errorf("invalid placement response")
	}
	return strings.TrimSpace(envelope.Data.Attributes.PlacementGroup), nil
}

func creativePlacementDeleteCommand(prefix string) *ffcli.Command {
	fs := flag.NewFlagSet("placements delete", flag.ExitOnError)
	id := BindResourceIDFlag(fs, "id", "appAssetLibraryPlacements", "Asset library placement ID or API self-link")
	ids := fs.String("placement-ids", "", "Placement IDs to remove sequentially with DELETE, comma-separated")
	confirm := fs.Bool("confirm", false, "Confirm removal of these placements")
	output := BindOutputFlags(fs)
	return &ffcli.Command{
		Name: "delete", ShortHelp: "Remove an asset placement without deleting its library media.", ShortUsage: "asc " + prefix + " placements delete (--id ID | --placement-ids ID1,ID2) --confirm", FlagSet: fs, UsageFunc: DefaultUsageFunc,
		LongHelp: "Remove one placement with --id, or remove --placement-ids sequentially using the public DELETE endpoint. Supply exactly one of these flags and --confirm. Apple does not expose the website batch deletion route through the public API. Multiple deletes are not atomic: processing stops on the first error and prints a partial receipt of deleted IDs and the failed ID before returning failure. Library media remains available for reuse. IDs are authoritative: this command does not validate a placement's localization or type. Inspect placements before retrying an uncertain failure; earlier successful deletes are not rolled back.\n\nExamples:\n  asc " + prefix + " placements delete --id PLACEMENT_ID --confirm\n  asc " + prefix + " placements delete --placement-ids FIRST_ID,SECOND_ID --confirm",
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return UsageErrorf("placements delete: unexpected argument %q", args[0])
			}
			if err := ValidateBoundOutputFlags(fs); err != nil {
				return UsageErrorf("placements delete: %v", err)
			}
			placementID := strings.TrimSpace(*id)
			placementIDs, err := placementIDsCSV(*ids)
			if err != nil {
				return UsageErrorf("placements delete: --placement-ids %v", err)
			}
			if (placementID == "") == (len(placementIDs) == 0) {
				return UsageError("placements delete: supply exactly one of --id or --placement-ids")
			}
			if placementID != "" && !creativePlacementID.MatchString(placementID) {
				return UsageError("placements delete: --id requires a resource ID")
			}
			if !*confirm {
				return UsageError("placements delete: --confirm is required")
			}
			client, err := GetASCClient()
			if err != nil {
				return fmt.Errorf("placements delete: %w", err)
			}
			if len(placementIDs) > 0 {
				result := &asc.CreativePlacementBulkDeleteResult{PlacementIDs: placementIDs, DeletedPlacementIDs: make([]string, 0, len(placementIDs))}
				for _, id := range placementIDs {
					requestCtx, cancel := ContextWithTimeout(ctx)
					_, err := client.RawRequest(requestCtx, "DELETE", "/v1/appAssetLibraryPlacements/"+url.PathEscape(id), nil)
					cancel()
					if err != nil {
						result.FailedPlacementID = id
						if outputErr := PrintOutput(result, *output.Output, *output.Pretty); outputErr != nil {
							return fmt.Errorf("placements delete: failed on %s: %w (partial receipt output: %w)", id, err, outputErr)
						}
						return fmt.Errorf("placements delete: failed on %s after removing %d placement(s): %w", id, len(result.DeletedPlacementIDs), err)
					}
					result.DeletedPlacementIDs = append(result.DeletedPlacementIDs, id)
				}
				result.Deleted = true
				return PrintOutput(result, *output.Output, *output.Pretty)
			}
			requestCtx, cancel := ContextWithTimeout(ctx)
			defer cancel()
			if _, err := client.RawRequest(requestCtx, "DELETE", "/v1/appAssetLibraryPlacements/"+url.PathEscape(placementID), nil); err != nil {
				return fmt.Errorf("placements delete: %w", err)
			}
			return PrintOutput(&asc.CreativePlacementDeleteResult{ID: placementID, Deleted: true}, *output.Output, *output.Pretty)
		},
	}
}
