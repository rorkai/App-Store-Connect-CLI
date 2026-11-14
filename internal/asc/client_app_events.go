package asc

import (
	"context"
	"fmt"
	"net/http"
	"strings"
)

// AppEventLocalizationRelationships describes relationships for app event localizations.
type AppEventLocalizationRelationships struct {
	AppEvent *Relationship `json:"appEvent"`
}

// AppEventScreenshotCreateAttributes describes attributes for creating screenshots.
type AppEventScreenshotCreateAttributes struct {
	FileSize          int64  `json:"fileSize"`
	FileName          string `json:"fileName"`
	AppEventAssetType string `json:"appEventAssetType"`
}

// AppEventVideoClipCreateAttributes describes attributes for creating video clips.
type AppEventVideoClipCreateAttributes struct {
	FileSize             int64  `json:"fileSize"`
	FileName             string `json:"fileName"`
	PreviewFrameTimeCode string `json:"previewFrameTimeCode,omitempty"`
	AppEventAssetType    string `json:"appEventAssetType"`
}

// GetAppEventLocalization retrieves an app event localization by ID.
func (c *Client) GetAppEventLocalization(ctx context.Context, localizationID string, opts ...LocalizationReadOption) (*AppEventLocalizationResponse, error) {
	localizationID = strings.TrimSpace(localizationID)
	if localizationID == "" {
		return nil, fmt.Errorf("localizationID is required")
	}
	path := localizationReadPath(fmt.Sprintf("/v1/appEventLocalizations/%s", localizationID), opts)
	return getJSON[AppEventLocalizationResponse](ctx, c, readRequest{Path: path})
}

// CreateAppEventScreenshot creates an app event screenshot reservation.
func (c *Client) CreateAppEventScreenshot(ctx context.Context, localizationID, fileName string, fileSize int64, assetType string) (*AppEventScreenshotResponse, error) {
	localizationID = strings.TrimSpace(localizationID)
	if localizationID == "" {
		return nil, fmt.Errorf("localizationID is required")
	}

	payload := AppEventScreenshotCreateRequest{
		Data: AppEventScreenshotCreateData{
			Type: ResourceTypeAppEventScreenshots,
			Attributes: AppEventScreenshotCreateAttributes{
				FileName:          strings.TrimSpace(fileName),
				FileSize:          fileSize,
				AppEventAssetType: assetType,
			},
			Relationships: &AppEventScreenshotRelationships{
				AppEventLocalization: &Relationship{
					Data: ResourceData{
						Type: ResourceTypeAppEventLocalizations,
						ID:   localizationID,
					},
				},
			},
		},
	}

	return sendJSON[AppEventScreenshotResponse](ctx, c, writeRequest{
		Method:  http.MethodPost,
		Path:    "/v1/appEventScreenshots",
		Payload: payload,
	})
}

// UpdateAppEventScreenshot updates an app event screenshot.
func (c *Client) UpdateAppEventScreenshot(ctx context.Context, screenshotID string, uploaded bool) (*AppEventScreenshotResponse, error) {
	screenshotID = strings.TrimSpace(screenshotID)
	if screenshotID == "" {
		return nil, fmt.Errorf("screenshotID is required")
	}

	payload := AppEventScreenshotUpdateRequest{
		Data: AppEventScreenshotUpdateData{
			Type: ResourceTypeAppEventScreenshots,
			ID:   screenshotID,
			Attributes: &AppEventScreenshotUpdateAttributes{
				Uploaded: &uploaded,
			},
		},
	}

	return sendJSON[AppEventScreenshotResponse](ctx, c, writeRequest{
		Method:  http.MethodPatch,
		Path:    fmt.Sprintf("/v1/appEventScreenshots/%s", screenshotID),
		Payload: payload,
	})
}

// CreateAppEventVideoClip creates an app event video clip reservation.
func (c *Client) CreateAppEventVideoClip(ctx context.Context, localizationID, fileName string, fileSize int64, assetType, previewFrameTimeCode string) (*AppEventVideoClipResponse, error) {
	localizationID = strings.TrimSpace(localizationID)
	if localizationID == "" {
		return nil, fmt.Errorf("localizationID is required")
	}

	attrs := AppEventVideoClipCreateAttributes{
		FileName:          strings.TrimSpace(fileName),
		FileSize:          fileSize,
		AppEventAssetType: assetType,
	}
	if strings.TrimSpace(previewFrameTimeCode) != "" {
		attrs.PreviewFrameTimeCode = strings.TrimSpace(previewFrameTimeCode)
	}

	payload := AppEventVideoClipCreateRequest{
		Data: AppEventVideoClipCreateData{
			Type:       ResourceTypeAppEventVideoClips,
			Attributes: attrs,
			Relationships: &AppEventVideoClipRelationships{
				AppEventLocalization: &Relationship{
					Data: ResourceData{
						Type: ResourceTypeAppEventLocalizations,
						ID:   localizationID,
					},
				},
			},
		},
	}

	return sendJSON[AppEventVideoClipResponse](ctx, c, writeRequest{
		Method:  http.MethodPost,
		Path:    "/v1/appEventVideoClips",
		Payload: payload,
	})
}
