package shared

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

func creativePlacementTypes(resource string) []string {
	if resource == "appEventLocalizations" {
		return []string{"EVENT_CARD_ASSET", "EVENT_DETAILS_PAGE_ASSET"}
	}
	kinds := []string{"PRODUCT_PAGE_HEADER_ASSET", "APP_STORE_SEARCH_RESULTS_ASSET", "APP_SCREENSHOT", "APP_PREVIEW"}
	if resource == "appStoreVersionLocalizations" {
		kinds = append(kinds, "IMESSAGE_APP_SCREENSHOT")
	}
	return kinds
}

func creativePlacementRelationship(resource string) string {
	switch resource {
	case "appStoreVersionLocalizations":
		return "appStoreVersionLocalization"
	case "appCustomProductPageLocalizations":
		return "appCustomProductPageLocalization"
	case "appStoreVersionExperimentTreatmentLocalizations":
		return "appStoreVersionExperimentTreatmentLocalization"
	case "appEventLocalizations":
		return "appEventLocalization"
	default:
		return ""
	}
}

func placementIDsCSV(raw string) ([]string, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	values := strings.Split(raw, ",")
	seen := make(map[string]bool, len(values))
	for i, value := range values {
		value = strings.TrimSpace(value)
		if !creativePlacementID.MatchString(value) {
			return nil, fmt.Errorf("contains invalid resource ID %q", value)
		}
		if seen[value] {
			return nil, fmt.Errorf("contains duplicate resource ID %q", value)
		}
		seen[value] = true
		values[i] = value
	}
	return values, nil
}

func creativePlacementReorderCommand(resource, prefix, label string) *ffcli.Command {
	fs := flag.NewFlagSet("placements reorder", flag.ExitOnError)
	localization := BindResourceIDFlag(fs, "localization-id", resource, label+" ID or API self-link")
	group := fs.String("placement-group", "", "Placement group whose assets will be ordered")
	ids := fs.String("placement-ids", "", "Placement IDs in desired order, comma-separated")
	output := BindOutputFlags(fs)
	exampleGroup := "IPHONE_DUO_PROFILE"
	return &ffcli.Command{
		Name: "reorder", ShortHelp: "Order existing asset library placements within a localization and group.", ShortUsage: "asc " + prefix + " placements reorder --localization-id ID --placement-group GROUP --placement-ids ID1,ID2", FlagSet: fs, UsageFunc: DefaultUsageFunc,
		LongHelp: "Order existing placements for " + label + " using a placement ordering request. Supply unique placement IDs in the desired order and their placement group. Apple validates localization/group membership and whether the resource is editable. This command does not delete media, submit assets for review, or promise publication.\n\nExample:\n  asc " + prefix + " placements reorder --localization-id ID --placement-group " + exampleGroup + " --placement-ids FIRST_ID,SECOND_ID",
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return UsageErrorf("placements reorder: unexpected argument %q", args[0])
			}
			if err := ValidateBoundOutputFlags(fs); err != nil {
				return UsageErrorf("placements reorder: %v", err)
			}
			locID, placementGroup := strings.TrimSpace(*localization), strings.TrimSpace(*group)
			if !creativePlacementID.MatchString(locID) {
				return UsageError("placements reorder: --localization-id requires a resource ID")
			}
			if !creativePlacementID.MatchString(placementGroup) {
				return UsageError("placements reorder: --placement-group is required and must be a group ID")
			}
			placementIDs, err := placementIDsCSV(*ids)
			if err != nil {
				return UsageErrorf("placements reorder: --placement-ids %v", err)
			}
			if len(placementIDs) == 0 {
				return UsageError("placements reorder: --placement-ids is required")
			}
			client, err := GetASCClient()
			if err != nil {
				return fmt.Errorf("placements reorder: %w", err)
			}
			requestID, err := OrderLibraryPlacements(ctx, client, resource, locID, placementGroup, placementIDs)
			if err != nil {
				return fmt.Errorf("placements reorder: %w", err)
			}
			return PrintOutput(&asc.CreativePlacementReorderResult{ID: requestID, Reordered: true, LocalizationID: locID, PlacementGroup: placementGroup, PlacementIDs: placementIDs}, *output.Output, *output.Pretty)
		},
	}
}

// OrderLibraryPlacements submits one ordering request for a localization's
// placement group and returns the request ID.
func OrderLibraryPlacements(ctx context.Context, client *asc.Client, resource, localizationID, placementGroup string, placementIDs []string) (string, error) {
	ordered := make([]map[string]string, len(placementIDs))
	for i, id := range placementIDs {
		ordered[i] = map[string]string{"type": "appAssetLibraryPlacements", "id": id}
	}
	payload, err := json.Marshal(map[string]any{"data": map[string]any{"type": "appAssetLibraryPlacementOrderingRequests", "attributes": map[string]string{"placementGroup": placementGroup}, "relationships": map[string]any{"orderedPlacements": map[string]any{"data": ordered}, creativePlacementRelationship(resource): map[string]any{"data": map[string]string{"type": resource, "id": localizationID}}}}})
	if err != nil {
		return "", err
	}
	requestCtx, cancel := ContextWithTimeout(ctx)
	defer cancel()
	response, err := client.RawRequest(requestCtx, "POST", "/v1/appAssetLibraryPlacementOrderingRequests", payload)
	if err != nil {
		return "", err
	}
	var envelope struct {
		Data struct {
			ID   string `json:"id"`
			Type string `json:"type"`
		} `json:"data"`
	}
	if err := json.Unmarshal(response, &envelope); err != nil {
		return "", fmt.Errorf("invalid response: %w", err)
	}
	if envelope.Data.ID == "" || envelope.Data.Type != "appAssetLibraryPlacementOrderingRequests" {
		return "", fmt.Errorf("invalid ordering response")
	}
	return envelope.Data.ID, nil
}
