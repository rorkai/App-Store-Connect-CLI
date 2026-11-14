package asc

import (
	"strconv"
	"strings"
)

// CreativePlacementCreateResult records a successful public placement creation.
type CreativePlacementCreateResult struct {
	ID             string `json:"id"`
	Created        bool   `json:"created"`
	LocalizationID string `json:"localizationId"`
	ImageID        string `json:"imageId"`
	VideoID        string `json:"videoId,omitempty"`
	PlacementType  string `json:"placementType"`
	PlacementGroup string `json:"placementGroup"`
	State          string `json:"state,omitempty"`
}

func creativePlacementCreateResultRows(r *CreativePlacementCreateResult) ([]string, [][]string) {
	return []string{"ID", "Created", "Localization ID", "Image ID", "Video ID", "Placement Type", "Group", "State"}, [][]string{{r.ID, strconv.FormatBool(r.Created), r.LocalizationID, r.ImageID, r.VideoID, r.PlacementType, r.PlacementGroup, r.State}}
}

// CreativePlacementReorderResult records an accepted placement ordering request.
type CreativePlacementReorderResult struct {
	ID             string   `json:"id"`
	Reordered      bool     `json:"reordered"`
	LocalizationID string   `json:"localizationId"`
	PlacementGroup string   `json:"placementGroup"`
	PlacementIDs   []string `json:"placementIds"`
}

func creativePlacementReorderResultRows(r *CreativePlacementReorderResult) ([]string, [][]string) {
	return []string{"ID", "Reordered", "Localization ID", "Group", "Placement IDs"}, [][]string{{r.ID, strconv.FormatBool(r.Reordered), r.LocalizationID, r.PlacementGroup, strings.Join(r.PlacementIDs, ",")}}
}

// CreativePlacementSwapResult records a successful placement replacement.
type CreativePlacementSwapResult struct {
	ID                  string `json:"id"`
	Swapped             bool   `json:"swapped"`
	PreviousPlacementID string `json:"previousPlacementId"`
	LocalizationID      string `json:"localizationId"`
	ImageID             string `json:"imageId,omitempty"`
	VideoID             string `json:"videoId,omitempty"`
	PlacementType       string `json:"placementType"`
	PlacementGroup      string `json:"placementGroup,omitempty"`
	State               string `json:"state,omitempty"`
}

func creativePlacementSwapResultRows(r *CreativePlacementSwapResult) ([]string, [][]string) {
	return []string{"ID", "Swapped", "Previous Placement ID", "Localization ID", "Image ID", "Video ID", "Placement Type", "Placement Group", "State"}, [][]string{{r.ID, strconv.FormatBool(r.Swapped), r.PreviousPlacementID, r.LocalizationID, r.ImageID, r.VideoID, r.PlacementType, r.PlacementGroup, r.State}}
}

// CreativePlacementBulkDeleteResult records progress of sequential placement removals.
type CreativePlacementBulkDeleteResult struct {
	Deleted             bool     `json:"deleted"`
	PlacementIDs        []string `json:"placementIds"`
	DeletedPlacementIDs []string `json:"deletedPlacementIds"`
	FailedPlacementID   string   `json:"failedPlacementId,omitempty"`
}

func creativePlacementBulkDeleteResultRows(r *CreativePlacementBulkDeleteResult) ([]string, [][]string) {
	return []string{"Deleted", "Requested Placement IDs", "Deleted Placement IDs", "Failed Placement ID"}, [][]string{{strconv.FormatBool(r.Deleted), strings.Join(r.PlacementIDs, ","), strings.Join(r.DeletedPlacementIDs, ","), r.FailedPlacementID}}
}
