package asc

import "strconv"

// CreativeManifestPlan records owned targets and local source identities.
type CreativeManifestPlan struct {
	AppID               string                      `json:"appId"`
	VersionID           string                      `json:"versionId"`
	VersionState        string                      `json:"versionState"`
	LibraryID           string                      `json:"libraryId"`
	ReadOnly            bool                        `json:"readOnly"`
	RemoteReuseVerified bool                        `json:"remoteReuseVerified"`
	UniqueSources       int                         `json:"uniqueSources"`
	Entries             []CreativeManifestPlanEntry `json:"entries"`
}

// CreativeManifestPlanEntry describes one requested creative placement.
type CreativeManifestPlanEntry struct {
	Locale               string                      `json:"locale"`
	LocalizationID       string                      `json:"localizationId"`
	PlacementType        string                      `json:"placementType"`
	PlacementGroup       string                      `json:"placementGroup"`
	File                 string                      `json:"file"`
	MediaType            string                      `json:"mediaType"`
	MediaFormat          string                      `json:"mediaFormat"`
	FileSize             int64                       `json:"fileSize"`
	ContentSHA256        string                      `json:"contentSha256"`
	SourceKey            string                      `json:"sourceKey"`
	PosterFrameMode      string                      `json:"posterFrameMode"`
	PreviewFrameTimeCode string                      `json:"previewFrameTimeCode,omitempty"`
	TargetStatus         string                      `json:"targetStatus"`
	ExistingPlacements   []CreativeManifestPlacement `json:"existingPlacements"`
}

// CreativeManifestPlacement is an observed placement, not a content match.
type CreativeManifestPlacement struct {
	ID        string `json:"id"`
	MediaType string `json:"mediaType"`
	AssetID   string `json:"assetId"`
	State     string `json:"state"`
}

func creativeManifestPlanRows(plan *CreativeManifestPlan) ([]string, [][]string) {
	rows := make([][]string, 0, len(plan.Entries))
	for _, item := range plan.Entries {
		key := item.SourceKey
		if len(key) > 19 {
			key = key[:19]
		}
		rows = append(rows, []string{item.Locale, item.File, item.PlacementType, item.TargetStatus, key, strconv.Itoa(len(item.ExistingPlacements))})
	}
	return []string{"Locale", "File", "Placement Type", "Target", "Source Key (short)", "Placements"}, rows
}
