package asc

import (
	"context"
	"fmt"
	"net/http"
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

	payload := BackgroundAssetCreateRequest{
		Data: BackgroundAssetCreateData{
			Type: ResourceTypeBackgroundAssets,
			Attributes: BackgroundAssetCreateAttributes{
				AssetPackIdentifier: assetPackIdentifier,
			},
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

	return sendJSON[BackgroundAssetResponse](ctx, c, writeRequest{Method: http.MethodPost, Path: "/v1/backgroundAssets", Payload: payload})
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
