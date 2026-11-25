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

func creativePlacementUsageError(d *Deps, message string, code DiagnosticCode, parameter string) error {
	return WithDiagnostic(d.UsageError(message), code, parameter)
}

func creativePlacementCreateCommand(d *Deps, resource, prefix, label string) *ffcli.Command {
	return creativePlacementWriteCommand(d, resource, prefix, label, false)
}

func creativePlacementSwapCommand(d *Deps, resource, prefix, label string) *ffcli.Command {
	return creativePlacementWriteCommand(d, resource, prefix, label, true)
}

func creativePlacementWriteCommand(d *Deps, resource, prefix, label string, swap bool) *ffcli.Command {
	operation := "create"
	if swap {
		operation = "swap"
	}
	fs := d.NewFlagSet("placements "+operation, flag.ExitOnError)
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
	output := d.BindOutputFlags(fs)
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
		Name: operation, ShortHelp: shortHelp, ShortUsage: usage, FlagSet: fs, UsageFunc: d.DefaultUsageFunc, LongHelp: longHelp,
		Exec: func(ctx context.Context, args []string) error {
			if err := ValidateBoundOutputFlags(fs); err != nil {
				return d.UsageErrorf("placements "+operation+": %v", err)
			}
			localizationID, imageID, videoID := strings.TrimSpace(*loc), strings.TrimSpace(*image), strings.TrimSpace(*video)
			if !creativePlacementID.MatchString(localizationID) {
				code := DiagnosticInvalidInput
				if localizationID == "" {
					code = DiagnosticRequiredInputMissing
				}
				return creativePlacementUsageError(d, "placements "+operation+": --localization-id requires a resource ID", code, "--localization-id")
			}
			if imageID == "" && videoID == "" {
				return creativePlacementUsageError(d, "placements "+operation+": supply exactly one of --image-id or --video-id", DiagnosticRequiredInputMissing, "")
			}
			if imageID != "" && videoID != "" {
				return creativePlacementUsageError(d, "placements "+operation+": supply exactly one of --image-id or --video-id", DiagnosticConflictingInput, "")
			}
			mediaID, mediaType, mediaRelationship := imageID, "appAssetLibraryImages", "image"
			if videoID != "" {
				mediaID, mediaType, mediaRelationship = videoID, "appAssetLibraryVideos", "video"
			}
			if !creativePlacementID.MatchString(mediaID) {
				parameter := "--image-id"
				if videoID != "" {
					parameter = "--video-id"
				}
				return creativePlacementUsageError(d, "placements "+operation+": media ID must be a resource ID", DiagnosticInvalidInput, parameter)
			}
			rawPlacementType := strings.TrimSpace(*kind)
			placementType, err := validatedPlacementCSV(rawPlacementType, creativePlacementTypes(resource))
			placementTypeMessage := "placements " + operation + ": --placement-type must be one of " + strings.Join(creativePlacementTypes(resource), ", ")
			if rawPlacementType == "" {
				return creativePlacementUsageError(d, placementTypeMessage, DiagnosticRequiredInputMissing, "--placement-type")
			}
			if err != nil || placementType == "" || strings.Contains(placementType, ",") {
				return creativePlacementUsageError(d, placementTypeMessage, DiagnosticInvalidInput, "--placement-type")
			}
			native := placementType == "APP_SCREENSHOT" || placementType == "IMESSAGE_APP_SCREENSHOT" || placementType == "APP_PREVIEW"
			if placementType == "APP_PREVIEW" && videoID == "" {
				return creativePlacementUsageError(d, "placements "+operation+": APP_PREVIEW requires --video-id", DiagnosticRequiredInputMissing, "--video-id")
			}
			if (placementType == "APP_SCREENSHOT" || placementType == "IMESSAGE_APP_SCREENSHOT") && imageID == "" {
				return creativePlacementUsageError(d, "placements "+operation+": screenshots require --image-id", DiagnosticRequiredInputMissing, "--image-id")
			}
			previousID, placementGroup := "", ""
			if swap {
				previousID = strings.TrimSpace(*previous)
				if !creativePlacementID.MatchString(previousID) {
					code := DiagnosticInvalidInput
					if previousID == "" {
						code = DiagnosticRequiredInputMissing
					}
					return creativePlacementUsageError(d, "placements swap: --placement-id requires a resource ID", code, "--placement-id")
				}
				if err := d.RequireConfirm(*confirm); err != nil {
					return err
				}
			} else {
				placementGroup = strings.TrimSpace(*group)
			}
			if !swap && native {
				groupMessage := "placements " + operation + ": screenshots/previews require a device --placement-group (not DEFAULT_PROFILE); inspect accepted groups with: asc asset-library specs --placement-type " + placementType
				if placementGroup == "" {
					return creativePlacementUsageError(d, groupMessage, DiagnosticRequiredInputMissing, "--placement-group")
				}
				if !creativePlacementID.MatchString(placementGroup) || placementGroup == "DEFAULT_PROFILE" {
					return creativePlacementUsageError(d, groupMessage, DiagnosticInvalidInput, "--placement-group")
				}
			} else if !swap {
				if placementGroup != "" && placementGroup != "DEFAULT_PROFILE" {
					return creativePlacementUsageError(d, "placements "+operation+": creative assets require DEFAULT_PROFILE", DiagnosticInvalidInput, "--placement-group")
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
			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("placements "+operation+": %w", err)
			}
			if swap {
				// Apple rejects a swap whose placementGroup differs from the
				// placement being replaced, and requires it for screenshots.
				placementGroup, err = existingPlacementGroup(ctx, d.Timeout, client, previousID)
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
			placementID, state, err := postLibraryPlacement(ctx, d.Timeout, client, payload)
			if err != nil {
				return fmt.Errorf("placements %s: %w", operation, err)
			}
			if swap {
				return d.PrintOutput(&asc.CreativePlacementSwapResult{ID: placementID, Swapped: true, PreviousPlacementID: previousID, LocalizationID: localizationID, ImageID: imageID, VideoID: videoID, PlacementType: placementType, PlacementGroup: placementGroup, State: state}, *output.Output, *output.Pretty)
			}
			return d.PrintOutput(&asc.CreativePlacementCreateResult{ID: placementID, Created: true, LocalizationID: localizationID, ImageID: imageID, VideoID: videoID, PlacementType: placementType, PlacementGroup: placementGroup, State: state}, *output.Output, *output.Pretty)
		},
	}
}

// CreateLibraryPlacement assigns uploaded library media to one localization,
// with the request deadline from d.Timeout. mediaRelationship is "image" or
// "video".
func (d *Deps) CreateLibraryPlacement(ctx context.Context, client *asc.Client, resource, localizationID, mediaRelationship, mediaID, placementType, placementGroup string) (id, state string, err error) {
	return createLibraryPlacement(ctx, d.Timeout, client, resource, localizationID, mediaRelationship, mediaID, placementType, placementGroup)
}

func createLibraryPlacement(ctx context.Context, timeout func(context.Context) (context.Context, context.CancelFunc), client *asc.Client, resource, localizationID, mediaRelationship, mediaID, placementType, placementGroup string) (id, state string, err error) {
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
	return postLibraryPlacement(ctx, timeout, client, payload)
}

func postLibraryPlacement(ctx context.Context, timeout func(context.Context) (context.Context, context.CancelFunc), client *asc.Client, payload []byte) (id, state string, err error) {
	requestCtx, cancel := timeout(ctx)
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
func existingPlacementGroup(ctx context.Context, timeout func(context.Context) (context.Context, context.CancelFunc), client *asc.Client, placementID string) (string, error) {
	requestCtx, cancel := timeout(ctx)
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

func creativePlacementDeleteCommand(d *Deps, prefix string) *ffcli.Command {
	fs := d.NewFlagSet("placements delete", flag.ExitOnError)
	id := BindResourceIDFlag(fs, "id", "appAssetLibraryPlacements", "Asset library placement ID or API self-link")
	ids := fs.String("placement-ids", "", "Placement IDs to remove sequentially with DELETE, comma-separated")
	confirm := fs.Bool("confirm", false, "Confirm removal of these placements")
	output := d.BindOutputFlags(fs)
	return &ffcli.Command{
		Name: "delete", ShortHelp: "Remove an asset placement without deleting its library media.", ShortUsage: "asc " + prefix + " placements delete (--id ID | --placement-ids ID1,ID2) --confirm", FlagSet: fs, UsageFunc: d.DefaultUsageFunc,
		LongHelp: "Remove one placement with --id, or remove --placement-ids sequentially using the public DELETE endpoint. Supply exactly one of these flags and --confirm. Apple does not expose the website batch deletion route through the public API. Multiple deletes are not atomic: processing stops on the first error and prints a partial receipt of deleted IDs and the failed ID before returning failure. Library media remains available for reuse. IDs are authoritative: this command does not validate a placement's localization or type. Inspect placements before retrying an uncertain failure; earlier successful deletes are not rolled back.\n\nExamples:\n  asc " + prefix + " placements delete --id PLACEMENT_ID --confirm\n  asc " + prefix + " placements delete --placement-ids FIRST_ID,SECOND_ID --confirm",
		Exec: func(ctx context.Context, args []string) error {
			if err := ValidateBoundOutputFlags(fs); err != nil {
				return d.UsageErrorf("placements delete: %v", err)
			}
			placementID := strings.TrimSpace(*id)
			placementIDs, err := placementIDsCSV(*ids)
			if err != nil {
				return d.UsageErrorf("placements delete: --placement-ids %v", err)
			}
			if (placementID == "") == (len(placementIDs) == 0) {
				return d.UsageError("placements delete: supply exactly one of --id or --placement-ids")
			}
			if placementID != "" && !creativePlacementID.MatchString(placementID) {
				return d.UsageError("placements delete: --id requires a resource ID")
			}
			if err := d.RequireConfirm(*confirm); err != nil {
				return err
			}
			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("placements delete: %w", err)
			}
			if len(placementIDs) > 0 {
				result := &asc.CreativePlacementBulkDeleteResult{PlacementIDs: placementIDs, DeletedPlacementIDs: make([]string, 0, len(placementIDs))}
				for _, id := range placementIDs {
					requestCtx, cancel := d.Timeout(ctx)
					_, err := client.RawRequest(requestCtx, "DELETE", "/v1/appAssetLibraryPlacements/"+url.PathEscape(id), nil)
					cancel()
					if err != nil {
						result.FailedPlacementID = id
						if outputErr := d.PrintOutput(result, *output.Output, *output.Pretty); outputErr != nil {
							return fmt.Errorf("placements delete: failed on %s: %w (partial receipt output: %w)", id, err, outputErr)
						}
						return fmt.Errorf("placements delete: failed on %s after removing %d placement(s): %w", id, len(result.DeletedPlacementIDs), err)
					}
					result.DeletedPlacementIDs = append(result.DeletedPlacementIDs, id)
				}
				result.Deleted = true
				return d.PrintOutput(result, *output.Output, *output.Pretty)
			}
			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()
			if _, err := client.RawRequest(requestCtx, "DELETE", "/v1/appAssetLibraryPlacements/"+url.PathEscape(placementID), nil); err != nil {
				return fmt.Errorf("placements delete: %w", err)
			}
			return d.PrintOutput(&asc.DeleteResult{ID: placementID, Deleted: true}, *output.Output, *output.Pretty)
		},
	}
}
