package asc

import (
	"context"
	"fmt"
	"strings"
)

// CreateBackgroundAsset creates a background asset.
func (c *Client) CreateBackgroundAsset(ctx context.Context, appID, assetPackIdentifier string) (*BackgroundAssetResponse, error) {
	appID = strings.TrimSpace(appID)
	assetPackIdentifier = strings.TrimSpace(assetPackIdentifier)
	if appID == "" {
		return nil, fmt.Errorf("app ID is required")
	}
	if assetPackIdentifier == "" {
		return nil, fmt.Errorf("asset pack identifier is required")
	}

	request := BackgroundAssetCreateRequest{
		Data: BackgroundAssetCreateData{
			Type:       ResourceTypeBackgroundAssets,
			Attributes: BackgroundAssetCreateAttributes{AssetPackIdentifier: assetPackIdentifier},
			Relationships: BackgroundAssetCreateRelationships{
				App: Relationship{
					Data: ResourceData{
						Type: ResourceTypeApps,
						ID:   appID,
					},
				},
			},
		},
	}

	return sendJSON[BackgroundAssetResponse](ctx, c, writeRequest{Method: "POST", Path: "/v1/backgroundAssets", Payload: request})
}

// UpdateBackgroundAsset updates a background asset by ID.
func (c *Client) UpdateBackgroundAsset(ctx context.Context, id string, attrs BackgroundAssetUpdateAttributes) (*BackgroundAssetResponse, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, fmt.Errorf("background asset ID is required")
	}

	request := BackgroundAssetUpdateRequest{
		Data: BackgroundAssetUpdateData{
			Type: ResourceTypeBackgroundAssets,
			ID:   id,
		},
	}
	if attrs.Archived != nil {
		request.Data.Attributes = &attrs
	}

	return sendJSON[BackgroundAssetResponse](ctx, c, writeRequest{Method: "PATCH", Path: fmt.Sprintf("/v1/backgroundAssets/%s", id), Payload: request})
}

// CreateBackgroundAssetVersion creates a new background asset version.
func (c *Client) CreateBackgroundAssetVersion(ctx context.Context, backgroundAssetID string) (*BackgroundAssetVersionResponse, error) {
	backgroundAssetID = strings.TrimSpace(backgroundAssetID)
	if backgroundAssetID == "" {
		return nil, fmt.Errorf("background asset ID is required")
	}

	request := BackgroundAssetVersionCreateRequest{
		Data: BackgroundAssetVersionCreateData{
			Type: ResourceTypeBackgroundAssetVersions,
			Relationships: BackgroundAssetVersionCreateRelationships{
				BackgroundAsset: Relationship{
					Data: ResourceData{
						Type: ResourceTypeBackgroundAssets,
						ID:   backgroundAssetID,
					},
				},
			},
		},
	}

	return sendJSON[BackgroundAssetVersionResponse](ctx, c, writeRequest{Method: "POST", Path: "/v1/backgroundAssetVersions", Payload: request})
}

// CreateBackgroundAssetUploadFile creates a background asset upload file.
func (c *Client) CreateBackgroundAssetUploadFile(ctx context.Context, versionID, fileName string, fileSize int64, assetType BackgroundAssetUploadFileAssetType) (*BackgroundAssetUploadFileResponse, error) {
	versionID = strings.TrimSpace(versionID)
	fileName = strings.TrimSpace(fileName)
	if versionID == "" {
		return nil, fmt.Errorf("background asset version ID is required")
	}
	if fileName == "" {
		return nil, fmt.Errorf("file name is required")
	}
	if fileSize <= 0 {
		return nil, fmt.Errorf("file size must be greater than 0")
	}

	request := BackgroundAssetUploadFileCreateRequest{
		Data: BackgroundAssetUploadFileCreateData{
			Type: ResourceTypeBackgroundAssetUploadFiles,
			Attributes: BackgroundAssetUploadFileCreateAttributes{
				AssetType: assetType,
				FileName:  fileName,
				FileSize:  fileSize,
			},
			Relationships: BackgroundAssetUploadFileCreateRelationships{
				BackgroundAssetVersion: Relationship{
					Data: ResourceData{
						Type: ResourceTypeBackgroundAssetVersions,
						ID:   versionID,
					},
				},
			},
		},
	}

	return sendJSON[BackgroundAssetUploadFileResponse](ctx, c, writeRequest{Method: "POST", Path: "/v1/backgroundAssetUploadFiles", Payload: request})
}

// UpdateBackgroundAssetUploadFile updates a background asset upload file by ID.
func (c *Client) UpdateBackgroundAssetUploadFile(ctx context.Context, uploadFileID string, attrs BackgroundAssetUploadFileUpdateAttributes) (*BackgroundAssetUploadFileResponse, error) {
	uploadFileID = strings.TrimSpace(uploadFileID)
	if uploadFileID == "" {
		return nil, fmt.Errorf("background asset upload file ID is required")
	}

	request := BackgroundAssetUploadFileUpdateRequest{
		Data: BackgroundAssetUploadFileUpdateData{
			Type: ResourceTypeBackgroundAssetUploadFiles,
			ID:   uploadFileID,
		},
	}
	if attrs.SourceFileChecksum != nil || attrs.SourceFileChecksums != nil || attrs.Uploaded != nil {
		request.Data.Attributes = &attrs
	}

	return sendJSON[BackgroundAssetUploadFileResponse](ctx, c, writeRequest{Method: "PATCH", Path: fmt.Sprintf("/v1/backgroundAssetUploadFiles/%s", uploadFileID), Payload: request})
}
