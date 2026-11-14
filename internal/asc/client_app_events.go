package asc

import (
	"context"
	"fmt"
	"net/http"
	"strings"
)

// AppEventCreateRelationships describes relationships for app event create requests.
type AppEventCreateRelationships struct {
	App *Relationship `json:"app"`
}

// AppEventCreateData is the data portion of an app event create request.
type AppEventCreateData struct {
	Type          ResourceType                 `json:"type"`
	Attributes    AppEventCreateAttributes     `json:"attributes"`
	Relationships *AppEventCreateRelationships `json:"relationships"`
}

// AppEventLocalizationRelationships describes relationships for app event localizations.
type AppEventLocalizationRelationships struct {
	AppEvent *Relationship `json:"appEvent"`
}

// AppEventLocalizationCreateData is the data portion of a localization create request.
type AppEventLocalizationCreateData struct {
	Type          ResourceType                         `json:"type"`
	Attributes    AppEventLocalizationCreateAttributes `json:"attributes"`
	Relationships *AppEventLocalizationRelationships   `json:"relationships"`
}

// AppEventScreenshotRelationships describes relationships for app event screenshots.
type AppEventScreenshotRelationships struct {
	AppEventLocalization *Relationship `json:"appEventLocalization"`
}

// AppEventScreenshotCreateAttributes describes attributes for creating screenshots.
type AppEventScreenshotCreateAttributes struct {
	FileSize          int64  `json:"fileSize"`
	FileName          string `json:"fileName"`
	AppEventAssetType string `json:"appEventAssetType"`
}

// AppEventScreenshotCreateData is the data portion of a screenshot create request.
type AppEventScreenshotCreateData struct {
	Type          ResourceType                       `json:"type"`
	Attributes    AppEventScreenshotCreateAttributes `json:"attributes"`
	Relationships *AppEventScreenshotRelationships   `json:"relationships"`
}

// AppEventVideoClipRelationships describes relationships for app event video clips.
type AppEventVideoClipRelationships struct {
	AppEventLocalization *Relationship `json:"appEventLocalization"`
}

// AppEventVideoClipCreateAttributes describes attributes for creating video clips.
type AppEventVideoClipCreateAttributes struct {
	FileSize             int64  `json:"fileSize"`
	FileName             string `json:"fileName"`
	PreviewFrameTimeCode string `json:"previewFrameTimeCode,omitempty"`
	AppEventAssetType    string `json:"appEventAssetType"`
}

// AppEventVideoClipCreateData is the data portion of a video clip create request.
type AppEventVideoClipCreateData struct {
	Type          ResourceType                      `json:"type"`
	Attributes    AppEventVideoClipCreateAttributes `json:"attributes"`
	Relationships *AppEventVideoClipRelationships   `json:"relationships"`
}

// CreateAppEvent creates an app event.
func (c *Client) CreateAppEvent(ctx context.Context, appID string, attrs AppEventCreateAttributes) (*AppEventResponse, error) {
	appID = strings.TrimSpace(appID)
	if appID == "" {
		return nil, fmt.Errorf("appID is required")
	}

	payload := AppEventCreateRequest{
		Data: AppEventCreateData{
			Type:       ResourceTypeAppEvents,
			Attributes: attrs,
			Relationships: &AppEventCreateRelationships{
				App: &Relationship{
					Data: ResourceData{
						Type: ResourceTypeApps,
						ID:   appID,
					},
				},
			},
		},
	}

	return sendJSON[AppEventResponse](ctx, c, writeRequest{
		Method:  http.MethodPost,
		Path:    "/v1/appEvents",
		Payload: payload,
	})
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

// CreateAppEventLocalization creates a localization for an app event.
func (c *Client) CreateAppEventLocalization(ctx context.Context, eventID string, attrs AppEventLocalizationCreateAttributes) (*AppEventLocalizationResponse, error) {
	eventID = strings.TrimSpace(eventID)
	if eventID == "" {
		return nil, fmt.Errorf("eventID is required")
	}

	payload := AppEventLocalizationCreateRequest{
		Data: AppEventLocalizationCreateData{
			Type:       ResourceTypeAppEventLocalizations,
			Attributes: attrs,
			Relationships: &AppEventLocalizationRelationships{
				AppEvent: &Relationship{
					Data: ResourceData{
						Type: ResourceTypeAppEvents,
						ID:   eventID,
					},
				},
			},
		},
	}

	return sendJSON[AppEventLocalizationResponse](ctx, c, writeRequest{
		Method:  http.MethodPost,
		Path:    "/v1/appEventLocalizations",
		Payload: payload,
	})
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
