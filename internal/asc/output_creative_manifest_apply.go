package asc

import (
	"maps"
	"slices"
)

// CreativeManifestApplyReceipt is the durable local binding between uploaded
// source bytes and assigned placements. It contains no upload credentials.
type CreativeManifestApplyReceipt struct {
	SchemaVersion int                                       `json:"schemaVersion"`
	AppID         string                                    `json:"appId"`
	VersionID     string                                    `json:"versionId"`
	LibraryID     string                                    `json:"libraryId"`
	TransactionID string                                    `json:"transactionId"`
	Sources       map[string]*CreativeManifestAppliedSource `json:"sources"`
	Targets       map[string]*CreativeManifestAppliedTarget `json:"targets"`
}

type CreativeManifestAppliedSource struct {
	ReferenceName          string `json:"referenceName,omitempty"`
	File                   string `json:"file"`
	ContentSHA256          string `json:"contentSha256"`
	MediaType              string `json:"mediaType"`
	FileSize               int64  `json:"fileSize"`
	PosterFrameMode        string `json:"posterFrameMode"`
	PreviewFrameTimeCode   string `json:"previewFrameTimeCode,omitempty"`
	AssetID                string `json:"assetId,omitempty"`
	Phase                  string `json:"phase"`
	State                  string `json:"state,omitempty"`
	ObservedPosterTimeCode string `json:"observedPosterTimeCode,omitempty"`
}

type CreativeManifestAppliedTarget struct {
	Locale              string `json:"locale"`
	LocalizationID      string `json:"localizationId"`
	PlacementType       string `json:"placementType"`
	SourceKey           string `json:"sourceKey"`
	PreviousPlacementID string `json:"previousPlacementId,omitempty"`
	PreviousAssetID     string `json:"previousAssetId,omitempty"`
	PlacementID         string `json:"placementId,omitempty"`
	Phase               string `json:"phase"`
	State               string `json:"state,omitempty"`
}

func creativeManifestApplyRows(receipt *CreativeManifestApplyReceipt) ([]string, [][]string) {
	rows := [][]string{}
	for _, key := range slices.Sorted(maps.Keys(receipt.Sources)) {
		source := receipt.Sources[key]
		rows = append(rows, []string{"source", source.File, "", "", source.Phase, "", source.AssetID})
	}
	for _, key := range slices.Sorted(maps.Keys(receipt.Targets)) {
		target := receipt.Targets[key]
		assetID := ""
		if source := receipt.Sources[target.SourceKey]; source != nil {
			assetID = source.AssetID
		}
		rows = append(rows, []string{"placement", "", target.Locale, target.PlacementType, target.Phase, target.PlacementID, assetID})
	}
	return []string{"Kind", "File", "Locale", "Placement Type", "Phase", "Placement ID", "Asset ID"}, rows
}
