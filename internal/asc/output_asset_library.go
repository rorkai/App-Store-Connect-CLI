package asc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

type assetLibraryResource struct {
	ID         string                     `json:"id"`
	Type       string                     `json:"type"`
	Attributes map[string]json.RawMessage `json:"attributes"`
}

// ValidateAssetLibraryResponse verifies the resource envelope without imposing
// a schema on undocumented attribute values.
func ValidateAssetLibraryResponse(payload []byte) error {
	_, err := assetLibraryResources(payload)
	return err
}

func assetLibraryResources(payload []byte) ([]assetLibraryResource, error) {
	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(payload, &envelope); err != nil {
		return nil, fmt.Errorf("asset-library: invalid response: %w", err)
	}
	data := bytes.TrimSpace(envelope.Data)
	if len(data) == 0 || bytes.Equal(data, []byte("null")) {
		return nil, fmt.Errorf("asset-library: response has no resource data")
	}
	var items []assetLibraryResource
	switch data[0] {
	case '[':
		if err := json.Unmarshal(data, &items); err != nil {
			return nil, fmt.Errorf("asset-library: invalid collection: %w", err)
		}
	case '{':
		var item assetLibraryResource
		if err := json.Unmarshal(data, &item); err != nil {
			return nil, fmt.Errorf("asset-library: invalid resource: %w", err)
		}
		items = []assetLibraryResource{item}
	default:
		return nil, fmt.Errorf("asset-library: resource data must be an object or array")
	}
	for _, item := range items {
		if item.ID == "" || item.Type == "" {
			return nil, fmt.Errorf("asset-library: resource missing id or type")
		}
	}
	return items, nil
}

// AssetLibraryRows validates a live Asset Library envelope and derives display
// rows without re-encoding or filtering the caller's JSON response.
func AssetLibraryRows(payload []byte) ([]string, [][]string, error) {
	items, err := assetLibraryResources(payload)
	if err != nil {
		return nil, nil, err
	}
	headers := []string{"ID", "Name", "Category", "State", "Placement Type"}
	rows := make([][]string, 0, len(items))
	for _, item := range items {
		if item.ID == "" || item.Type == "" {
			return nil, nil, fmt.Errorf("asset-library: resource missing id or type")
		}
		if item.Type == "appAssetLibraryRefData" {
			headers = []string{"Media", "Spec ID", "Dimensions", "Aspect Ratio", "Placements"}
			for _, media := range []string{"image", "video"} {
				var specs []struct {
					ID         string          `json:"specId"`
					Ratio      json.RawMessage `json:"aspectRatio"`
					Placements []string        `json:"compatiblePlacementTypes"`
					Dimensions struct {
						MinWidth  int `json:"minWidth"`
						MaxWidth  int `json:"maxWidth"`
						MinHeight int `json:"minHeight"`
						MaxHeight int `json:"maxHeight"`
					} `json:"dimensions"`
				}
				if raw, ok := item.Attributes[media+"Specs"]; ok {
					if err := json.Unmarshal(raw, &specs); err != nil {
						return nil, nil, fmt.Errorf("asset-library: invalid %s specs: %w", media, err)
					}
				}
				for _, spec := range specs {
					d := spec.Dimensions
					var ratio string
					if err := json.Unmarshal(spec.Ratio, &ratio); err != nil && len(spec.Ratio) > 0 {
						var compact bytes.Buffer
						if err := json.Compact(&compact, spec.Ratio); err != nil {
							return nil, nil, fmt.Errorf("asset-library: invalid aspect ratio: %w", err)
						}
						ratio = compact.String()
					}
					rows = append(rows, []string{media, spec.ID, fmt.Sprintf("%d-%d x %d-%d", d.MinWidth, d.MaxWidth, d.MinHeight, d.MaxHeight), ratio, strings.Join(spec.Placements, ", ")})
				}
			}
			continue
		}
		text := func(key string) string {
			var value string
			_ = json.Unmarshal(item.Attributes[key], &value)
			return value
		}
		name := text("referenceName")
		if name == "" {
			name = text("fileName")
		}
		rows = append(rows, []string{item.ID, name, text("category"), text("state"), text("placementType")})
	}
	return headers, rows, nil
}

// AssetLibraryImageUploadEvent contains only bounded, non-identifying diagnostics.
type AssetLibraryImageUploadEvent struct {
	Event      string `json:"event"`
	Stage      string `json:"stage"`
	Status     string `json:"status"`
	DurationMs int64  `json:"durationMs"`
	RetryCount int    `json:"retryCount"`
	HTTPStatus int    `json:"httpStatus,omitempty"`
	ErrorCode  string `json:"errorCode,omitempty"`
}

// AssetLibraryImageUploadResult reports upload and processing independently.
// ImageID is retained in a partial result after reservation.
type AssetLibraryImageUploadResult struct {
	Diagnostics   []AssetLibraryImageUploadEvent `json:"diagnostics,omitempty"`
	ImageID       string                         `json:"imageId"`
	Category      string                         `json:"category,omitempty"`
	LibraryID     string                         `json:"libraryId"`
	FileName      string                         `json:"fileName"`
	FileSize      int64                          `json:"fileSize"`
	ReferenceName string                         `json:"referenceName,omitempty"`
	Uploaded      bool                           `json:"uploaded"`
	Ready         bool                           `json:"ready"`
	State         string                         `json:"state,omitempty"`
	SpecID        string                         `json:"specId,omitempty"`
	Width         int                            `json:"width,omitempty"`
	Height        int                            `json:"height,omitempty"`
}

func assetLibraryImageUploadRows(result *AssetLibraryImageUploadResult) ([]string, [][]string) {
	return []string{"Image ID", "Library ID", "File", "Uploaded", "Ready", "State", "Dimensions"}, [][]string{{result.ImageID, result.LibraryID, result.FileName, fmt.Sprint(result.Uploaded), fmt.Sprint(result.Ready), result.State, fmt.Sprintf("%d x %d", result.Width, result.Height)}}
}

// AssetLibraryVideoUploadResult reports upload and processing independently.
// VideoID is retained in a partial result after reservation.
type AssetLibraryVideoUploadResult struct {
	VideoID       string `json:"videoId"`
	Category      string `json:"category,omitempty"`
	LibraryID     string `json:"libraryId"`
	FileName      string `json:"fileName"`
	FileSize      int64  `json:"fileSize"`
	ReferenceName string `json:"referenceName,omitempty"`
	Uploaded      bool   `json:"uploaded"`
	Ready         bool   `json:"ready"`
	State         string `json:"state,omitempty"`
	SpecID        string `json:"specId,omitempty"`
	Width         int    `json:"width,omitempty"`
	Height        int    `json:"height,omitempty"`
}

func assetLibraryVideoUploadRows(result *AssetLibraryVideoUploadResult) ([]string, [][]string) {
	return []string{"Video ID", "Library ID", "File", "Uploaded", "Ready", "State", "Dimensions"}, [][]string{{result.VideoID, result.LibraryID, result.FileName, fmt.Sprint(result.Uploaded), fmt.Sprint(result.Ready), result.State, fmt.Sprintf("%d x %d", result.Width, result.Height)}}
}

// AssetLibraryLocalizationUploadItem reports one file. State is would-upload,
// pending, upload-failed, uploaded, placement-failed or placed.
type AssetLibraryLocalizationUploadItem struct {
	FileName       string `json:"fileName"`
	FilePath       string `json:"filePath"`
	State          string `json:"state"`
	AssetID        string `json:"assetId,omitempty"`
	PlacementID    string `json:"placementId,omitempty"`
	PlacementState string `json:"placementState,omitempty"`
	Error          string `json:"error,omitempty"`
}

// AssetLibraryLocalizationUploadResult is the receipt for uploading media to the
// library and placing it on a version localization. Upload and placement are
// separate steps: Complete is true only when every file is placed and ordered.
type AssetLibraryLocalizationUploadResult struct {
	VersionLocalizationID string                               `json:"versionLocalizationId"`
	LibraryID             string                               `json:"libraryId"`
	PlacementType         string                               `json:"placementType"`
	PlacementGroup        string                               `json:"placementGroup"`
	MaxCount              int                                  `json:"maxCount"`
	ExistingPlacementIDs  []string                             `json:"existingPlacementIds"`
	DryRun                bool                                 `json:"dryRun"`
	Complete              bool                                 `json:"complete"`
	Ordered               bool                                 `json:"ordered"`
	OrderingRequestID     string                               `json:"orderingRequestId,omitempty"`
	Items                 []AssetLibraryLocalizationUploadItem `json:"items"`
}

func assetLibraryLocalizationUploadRows(result *AssetLibraryLocalizationUploadResult) ([]string, [][]string) {
	rows := make([][]string, len(result.Items))
	for i, item := range result.Items {
		rows[i] = []string{item.FileName, item.State, result.PlacementGroup, item.AssetID, item.PlacementID, item.PlacementState, item.Error}
	}
	return []string{"File", "State", "Group", "Asset ID", "Placement ID", "Placement State", "Error"}, rows
}
