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

// RoutingAppCoverageResponse is the response for routing app coverage endpoints.
type RoutingAppCoverageResponse = SingleResponse[RoutingAppCoverageAttributes]

// RoutingAppCoverageCreateAttributes describes create attributes.
type RoutingAppCoverageCreateAttributes struct {
	FileSize int64  `json:"fileSize"`
	FileName string `json:"fileName"`
}

// RoutingAppCoverageRelationships describes routing app coverage relationships.
type RoutingAppCoverageRelationships struct {
	AppStoreVersion *Relationship `json:"appStoreVersion"`
}

// RoutingAppCoverageCreateData is the data portion of a create request.
type RoutingAppCoverageCreateData struct {
	Type          ResourceType                       `json:"type"`
	Attributes    RoutingAppCoverageCreateAttributes `json:"attributes"`
	Relationships *RoutingAppCoverageRelationships   `json:"relationships"`
}

// RoutingAppCoverageCreateRequest is a request to create routing app coverage.
type RoutingAppCoverageCreateRequest struct {
	Data RoutingAppCoverageCreateData `json:"data"`
}

// RoutingAppCoverageUpdateAttributes describes update attributes.
type RoutingAppCoverageUpdateAttributes struct {
	SourceFileChecksum *string `json:"sourceFileChecksum,omitempty"`
	Uploaded           *bool   `json:"uploaded,omitempty"`
}

// RoutingAppCoverageUpdateData is the data portion of an update request.
type RoutingAppCoverageUpdateData struct {
	Type       ResourceType                        `json:"type"`
	ID         string                              `json:"id"`
	Attributes *RoutingAppCoverageUpdateAttributes `json:"attributes,omitempty"`
}

// RoutingAppCoverageUpdateRequest is a request to update routing app coverage.
type RoutingAppCoverageUpdateRequest struct {
	Data RoutingAppCoverageUpdateData `json:"data"`
}

// RoutingAppCoverageDeleteResult represents CLI output for deletions.
type RoutingAppCoverageDeleteResult struct {
	ID      string `json:"id"`
	Deleted bool   `json:"deleted"`
}

// GetRoutingAppCoverage retrieves routing app coverage by ID.
func (c *Client) GetRoutingAppCoverage(ctx context.Context, coverageID string) (*RoutingAppCoverageResponse, error) {
	coverageID = strings.TrimSpace(coverageID)
	if coverageID == "" {
		return nil, fmt.Errorf("coverageID is required")
	}

	return getJSON[RoutingAppCoverageResponse](ctx, c, readRequest{Path: fmt.Sprintf("/v1/routingAppCoverages/%s", coverageID)})
}

// GetRoutingAppCoverageForVersion retrieves routing app coverage for an app store version.
func (c *Client) GetRoutingAppCoverageForVersion(ctx context.Context, versionID string) (*RoutingAppCoverageResponse, error) {
	versionID = strings.TrimSpace(versionID)
	if versionID == "" {
		return nil, fmt.Errorf("versionID is required")
	}

	return getJSON[RoutingAppCoverageResponse](ctx, c, readRequest{Path: fmt.Sprintf("/v1/appStoreVersions/%s/routingAppCoverage", versionID)})
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

// DeleteRoutingAppCoverage deletes routing app coverage by ID.
func (c *Client) DeleteRoutingAppCoverage(ctx context.Context, coverageID string) error {
	coverageID = strings.TrimSpace(coverageID)
	if coverageID == "" {
		return fmt.Errorf("coverageID is required")
	}

	return sendNoContent(ctx, c, http.MethodDelete, fmt.Sprintf("/v1/routingAppCoverages/%s", coverageID), nil)
}
