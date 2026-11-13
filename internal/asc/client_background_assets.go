package asc

import (
	"context"
	"fmt"
	"strings"
)

// GetBackgroundAssets retrieves background assets for an app.
func (c *Client) GetBackgroundAssets(ctx context.Context, appID string, opts ...BackgroundAssetsOption) (*BackgroundAssetsResponse, error) {
	query := applyOptions(opts)
	return getJSON[BackgroundAssetsResponse](ctx, c, readRequest{
		Path:        fmt.Sprintf("/v1/apps/%s/backgroundAssets", strings.TrimSpace(appID)),
		Query:       buildBackgroundAssetsQuery(query),
		Next:        query.nextURL,
		NextContext: "backgroundAssets",
	})
}

// GetBackgroundAsset retrieves a background asset by ID.
func (c *Client) GetBackgroundAsset(ctx context.Context, id string) (*BackgroundAssetResponse, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, fmt.Errorf("background asset ID is required")
	}

	return getJSON[BackgroundAssetResponse](ctx, c, readRequest{Path: fmt.Sprintf("/v1/backgroundAssets/%s", id)})
}

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

// GetBackgroundAssetVersions retrieves versions for a background asset.
func (c *Client) GetBackgroundAssetVersions(ctx context.Context, backgroundAssetID string, opts ...BackgroundAssetVersionsOption) (*BackgroundAssetVersionsResponse, error) {
	query := applyOptions(opts)
	return getJSON[BackgroundAssetVersionsResponse](ctx, c, readRequest{
		Path:        fmt.Sprintf("/v1/backgroundAssets/%s/versions", strings.TrimSpace(backgroundAssetID)),
		Query:       buildBackgroundAssetVersionsQuery(query),
		Next:        query.nextURL,
		NextContext: "backgroundAssetVersions",
	})
}

// GetBackgroundAssetVersion retrieves a background asset version by ID.
func (c *Client) GetBackgroundAssetVersion(ctx context.Context, versionID string) (*BackgroundAssetVersionResponse, error) {
	versionID = strings.TrimSpace(versionID)
	if versionID == "" {
		return nil, fmt.Errorf("background asset version ID is required")
	}

	return getJSON[BackgroundAssetVersionResponse](ctx, c, readRequest{Path: fmt.Sprintf("/v1/backgroundAssetVersions/%s", versionID)})
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

// GetBackgroundAssetUploadFiles retrieves upload files for a background asset version.
func (c *Client) GetBackgroundAssetUploadFiles(ctx context.Context, versionID string, opts ...BackgroundAssetUploadFilesOption) (*BackgroundAssetUploadFilesResponse, error) {
	query := applyOptions(opts)
	return getJSON[BackgroundAssetUploadFilesResponse](ctx, c, readRequest{
		Path:        fmt.Sprintf("/v1/backgroundAssetVersions/%s/backgroundAssetUploadFiles", strings.TrimSpace(versionID)),
		Query:       buildBackgroundAssetUploadFilesQuery(query),
		Next:        query.nextURL,
		NextContext: "backgroundAssetUploadFiles",
	})
}

// GetBackgroundAssetUploadFile retrieves a background asset upload file by ID.
func (c *Client) GetBackgroundAssetUploadFile(ctx context.Context, uploadFileID string) (*BackgroundAssetUploadFileResponse, error) {
	uploadFileID = strings.TrimSpace(uploadFileID)
	if uploadFileID == "" {
		return nil, fmt.Errorf("background asset upload file ID is required")
	}

	return getJSON[BackgroundAssetUploadFileResponse](ctx, c, readRequest{Path: fmt.Sprintf("/v1/backgroundAssetUploadFiles/%s", uploadFileID)})
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

// GetBackgroundAssetVersionAppStoreRelease retrieves an App Store release by ID.
func (c *Client) GetBackgroundAssetVersionAppStoreRelease(ctx context.Context, releaseID string) (*BackgroundAssetVersionAppStoreReleaseResponse, error) {
	releaseID = strings.TrimSpace(releaseID)
	if releaseID == "" {
		return nil, fmt.Errorf("background asset version App Store release ID is required")
	}

	return getJSON[BackgroundAssetVersionAppStoreReleaseResponse](ctx, c, readRequest{Path: fmt.Sprintf("/v1/backgroundAssetVersionAppStoreReleases/%s", releaseID)})
}

// GetBackgroundAssetVersionExternalBetaRelease retrieves an external beta release by ID.
func (c *Client) GetBackgroundAssetVersionExternalBetaRelease(ctx context.Context, releaseID string) (*BackgroundAssetVersionExternalBetaReleaseResponse, error) {
	releaseID = strings.TrimSpace(releaseID)
	if releaseID == "" {
		return nil, fmt.Errorf("background asset version external beta release ID is required")
	}

	return getJSON[BackgroundAssetVersionExternalBetaReleaseResponse](ctx, c, readRequest{Path: fmt.Sprintf("/v1/backgroundAssetVersionExternalBetaReleases/%s", releaseID)})
}

// GetBackgroundAssetVersionInternalBetaRelease retrieves an internal beta release by ID.
func (c *Client) GetBackgroundAssetVersionInternalBetaRelease(ctx context.Context, releaseID string) (*BackgroundAssetVersionInternalBetaReleaseResponse, error) {
	releaseID = strings.TrimSpace(releaseID)
	if releaseID == "" {
		return nil, fmt.Errorf("background asset version internal beta release ID is required")
	}

	return getJSON[BackgroundAssetVersionInternalBetaReleaseResponse](ctx, c, readRequest{Path: fmt.Sprintf("/v1/backgroundAssetVersionInternalBetaReleases/%s", releaseID)})
}
