package asc

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// GetBackgroundAssets retrieves background assets for an app.
func (c *Client) GetBackgroundAssets(ctx context.Context, appID string, opts ...BackgroundAssetsOption) (*BackgroundAssetsResponse, error) {
	query := &backgroundAssetsQuery{}
	for _, opt := range opts {
		opt(query)
	}

	path := fmt.Sprintf("/v1/apps/%s/backgroundAssets", strings.TrimSpace(appID))
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("backgroundAssets: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildBackgroundAssetsQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response BackgroundAssetsResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetBackgroundAsset retrieves a background asset by ID.
func (c *Client) GetBackgroundAsset(ctx context.Context, id string) (*BackgroundAssetResponse, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, fmt.Errorf("background asset ID is required")
	}

	path := fmt.Sprintf("/v1/backgroundAssets/%s", id)
	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response BackgroundAssetResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
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

	body, err := BuildRequestBody(request)
	if err != nil {
		return nil, err
	}

	data, err := c.do(ctx, "POST", "/v1/backgroundAssets", body)
	if err != nil {
		return nil, err
	}

	var response BackgroundAssetResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
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

	body, err := BuildRequestBody(request)
	if err != nil {
		return nil, err
	}

	data, err := c.do(ctx, "PATCH", fmt.Sprintf("/v1/backgroundAssets/%s", id), body)
	if err != nil {
		return nil, err
	}

	var response BackgroundAssetResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetBackgroundAssetVersions retrieves versions for a background asset.
func (c *Client) GetBackgroundAssetVersions(ctx context.Context, backgroundAssetID string, opts ...BackgroundAssetVersionsOption) (*BackgroundAssetVersionsResponse, error) {
	query := &backgroundAssetVersionsQuery{}
	for _, opt := range opts {
		opt(query)
	}

	path := fmt.Sprintf("/v1/backgroundAssets/%s/versions", strings.TrimSpace(backgroundAssetID))
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("backgroundAssetVersions: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildBackgroundAssetVersionsQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response BackgroundAssetVersionsResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetBackgroundAssetVersion retrieves a background asset version by ID.
func (c *Client) GetBackgroundAssetVersion(ctx context.Context, versionID string) (*BackgroundAssetVersionResponse, error) {
	versionID = strings.TrimSpace(versionID)
	if versionID == "" {
		return nil, fmt.Errorf("background asset version ID is required")
	}

	path := fmt.Sprintf("/v1/backgroundAssetVersions/%s", versionID)
	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response BackgroundAssetVersionResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
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

	body, err := BuildRequestBody(request)
	if err != nil {
		return nil, err
	}

	data, err := c.do(ctx, "POST", "/v1/backgroundAssetVersions", body)
	if err != nil {
		return nil, err
	}

	var response BackgroundAssetVersionResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetBackgroundAssetUploadFiles retrieves upload files for a background asset version.
func (c *Client) GetBackgroundAssetUploadFiles(ctx context.Context, versionID string, opts ...BackgroundAssetUploadFilesOption) (*BackgroundAssetUploadFilesResponse, error) {
	query := &backgroundAssetUploadFilesQuery{}
	for _, opt := range opts {
		opt(query)
	}

	path := fmt.Sprintf("/v1/backgroundAssetVersions/%s/backgroundAssetUploadFiles", strings.TrimSpace(versionID))
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("backgroundAssetUploadFiles: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildBackgroundAssetUploadFilesQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response BackgroundAssetUploadFilesResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetBackgroundAssetUploadFile retrieves a background asset upload file by ID.
func (c *Client) GetBackgroundAssetUploadFile(ctx context.Context, uploadFileID string) (*BackgroundAssetUploadFileResponse, error) {
	uploadFileID = strings.TrimSpace(uploadFileID)
	if uploadFileID == "" {
		return nil, fmt.Errorf("background asset upload file ID is required")
	}

	path := fmt.Sprintf("/v1/backgroundAssetUploadFiles/%s", uploadFileID)
	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response BackgroundAssetUploadFileResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
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

	body, err := BuildRequestBody(request)
	if err != nil {
		return nil, err
	}

	data, err := c.do(ctx, "POST", "/v1/backgroundAssetUploadFiles", body)
	if err != nil {
		return nil, err
	}

	var response BackgroundAssetUploadFileResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
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

	body, err := BuildRequestBody(request)
	if err != nil {
		return nil, err
	}

	data, err := c.do(ctx, "PATCH", fmt.Sprintf("/v1/backgroundAssetUploadFiles/%s", uploadFileID), body)
	if err != nil {
		return nil, err
	}

	var response BackgroundAssetUploadFileResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetBackgroundAssetVersionAppStoreRelease retrieves an App Store release by ID.
func (c *Client) GetBackgroundAssetVersionAppStoreRelease(ctx context.Context, releaseID string) (*BackgroundAssetVersionAppStoreReleaseResponse, error) {
	releaseID = strings.TrimSpace(releaseID)
	if releaseID == "" {
		return nil, fmt.Errorf("background asset version App Store release ID is required")
	}

	path := fmt.Sprintf("/v1/backgroundAssetVersionAppStoreReleases/%s", releaseID)
	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response BackgroundAssetVersionAppStoreReleaseResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetBackgroundAssetVersionExternalBetaRelease retrieves an external beta release by ID.
func (c *Client) GetBackgroundAssetVersionExternalBetaRelease(ctx context.Context, releaseID string) (*BackgroundAssetVersionExternalBetaReleaseResponse, error) {
	releaseID = strings.TrimSpace(releaseID)
	if releaseID == "" {
		return nil, fmt.Errorf("background asset version external beta release ID is required")
	}

	path := fmt.Sprintf("/v1/backgroundAssetVersionExternalBetaReleases/%s", releaseID)
	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response BackgroundAssetVersionExternalBetaReleaseResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetBackgroundAssetVersionInternalBetaRelease retrieves an internal beta release by ID.
func (c *Client) GetBackgroundAssetVersionInternalBetaRelease(ctx context.Context, releaseID string) (*BackgroundAssetVersionInternalBetaReleaseResponse, error) {
	releaseID = strings.TrimSpace(releaseID)
	if releaseID == "" {
		return nil, fmt.Errorf("background asset version internal beta release ID is required")
	}

	path := fmt.Sprintf("/v1/backgroundAssetVersionInternalBetaReleases/%s", releaseID)
	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response BackgroundAssetVersionInternalBetaReleaseResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}
