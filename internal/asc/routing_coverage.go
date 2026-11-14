package asc

import (
	"context"
	"fmt"
	"net/http"
	"strings"
)

// RoutingAppCoverageAttributes describes routing app coverage attributes.
type RoutingAppCoverageAttributes struct {
	FileSize           int64               `json:"fileSize,omitempty"`
	FileName           string              `json:"fileName,omitempty"`
	SourceFileChecksum string              `json:"sourceFileChecksum,omitempty"`
	UploadOperations   []UploadOperation   `json:"uploadOperations,omitempty"`
	AssetDeliveryState *AppMediaAssetState `json:"assetDeliveryState,omitempty"`
}

// RoutingAppCoverageCreateAttributes describes create attributes.
type RoutingAppCoverageCreateAttributes struct {
	FileSize int64  `json:"fileSize"`
	FileName string `json:"fileName"`
}

// RoutingAppCoverageDeleteResult represents CLI output for deletions.
type RoutingAppCoverageDeleteResult struct {
	ID      string `json:"id"`
	Deleted bool   `json:"deleted"`
}

// CreateRoutingAppCoverage creates a routing app coverage upload reservation.
func (c *Client) CreateRoutingAppCoverage(ctx context.Context, versionID, fileName string, fileSize int64) (*RoutingAppCoverageResponse, error) {
	versionID = strings.TrimSpace(versionID)
	fileName = strings.TrimSpace(fileName)
	if versionID == "" {
		return nil, fmt.Errorf("versionID is required")
	}
	if fileName == "" {
		return nil, fmt.Errorf("fileName is required")
	}
	if fileSize <= 0 {
		return nil, fmt.Errorf("fileSize is required")
	}

	payload := RoutingAppCoverageCreateRequest{
		Data: RoutingAppCoverageCreateData{
			Type: ResourceTypeRoutingAppCoverages,
			Attributes: RoutingAppCoverageCreateAttributes{
				FileName: fileName,
				FileSize: fileSize,
			},
			Relationships: &RoutingAppCoverageRelationships{
				AppStoreVersion: &Relationship{
					Data: ResourceData{
						Type: ResourceTypeAppStoreVersions,
						ID:   versionID,
					},
				},
			},
		},
	}

	return sendJSON[RoutingAppCoverageResponse](ctx, c, writeRequest{Method: http.MethodPost, Path: "/v1/routingAppCoverages", Payload: payload})
}

// UpdateRoutingAppCoverage updates routing app coverage by ID.
func (c *Client) UpdateRoutingAppCoverage(ctx context.Context, coverageID string, attrs RoutingAppCoverageUpdateAttributes) (*RoutingAppCoverageResponse, error) {
	coverageID = strings.TrimSpace(coverageID)
	if coverageID == "" {
		return nil, fmt.Errorf("coverageID is required")
	}

	payload := RoutingAppCoverageUpdateRequest{
		Data: RoutingAppCoverageUpdateData{
			Type: ResourceTypeRoutingAppCoverages,
			ID:   coverageID,
		},
	}
	if attrs.SourceFileChecksum != nil || attrs.Uploaded != nil {
		payload.Data.Attributes = &attrs
	}

	return sendJSON[RoutingAppCoverageResponse](ctx, c, writeRequest{Method: http.MethodPatch, Path: fmt.Sprintf("/v1/routingAppCoverages/%s", coverageID), Payload: payload})
}
