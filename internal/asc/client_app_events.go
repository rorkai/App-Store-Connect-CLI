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

// AppEventCreateRequest is a request to create an app event.
type AppEventCreateRequest struct {
	Data AppEventCreateData `json:"data"`
}

// AppEventUpdateData is the data portion of an app event update request.
type AppEventUpdateData struct {
	Type       ResourceType              `json:"type"`
	ID         string                    `json:"id"`
	Attributes *AppEventUpdateAttributes `json:"attributes,omitempty"`
}

// AppEventUpdateRequest is a request to update an app event.
type AppEventUpdateRequest struct {
	Data AppEventUpdateData `json:"data"`
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

// AppEventLocalizationCreateRequest is a request to create an app event localization.
type AppEventLocalizationCreateRequest struct {
	Data AppEventLocalizationCreateData `json:"data"`
}

// AppEventLocalizationUpdateData is the data portion of a localization update request.
type AppEventLocalizationUpdateData struct {
	Type       ResourceType                          `json:"type"`
	ID         string                                `json:"id"`
	Attributes *AppEventLocalizationUpdateAttributes `json:"attributes,omitempty"`
}

// AppEventLocalizationUpdateRequest is a request to update an app event localization.
type AppEventLocalizationUpdateRequest struct {
	Data AppEventLocalizationUpdateData `json:"data"`
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

// AppEventScreenshotCreateRequest is a request to create an app event screenshot.
type AppEventScreenshotCreateRequest struct {
	Data AppEventScreenshotCreateData `json:"data"`
}

// AppEventScreenshotUpdateAttributes describes screenshot update attributes.
type AppEventScreenshotUpdateAttributes struct {
	Uploaded *bool `json:"uploaded,omitempty"`
}

// AppEventScreenshotUpdateData is the data portion of a screenshot update request.
type AppEventScreenshotUpdateData struct {
	Type       ResourceType                        `json:"type"`
	ID         string                              `json:"id"`
	Attributes *AppEventScreenshotUpdateAttributes `json:"attributes,omitempty"`
}

// AppEventScreenshotUpdateRequest is a request to update an app event screenshot.
type AppEventScreenshotUpdateRequest struct {
	Data AppEventScreenshotUpdateData `json:"data"`
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

// AppEventVideoClipCreateRequest is a request to create an app event video clip.
type AppEventVideoClipCreateRequest struct {
	Data AppEventVideoClipCreateData `json:"data"`
}

// AppEventVideoClipUpdateAttributes describes video clip update attributes.
type AppEventVideoClipUpdateAttributes struct {
	PreviewFrameTimeCode *string `json:"previewFrameTimeCode,omitempty"`
	Uploaded             *bool   `json:"uploaded,omitempty"`
}

// AppEventVideoClipUpdateData is the data portion of a video clip update request.
type AppEventVideoClipUpdateData struct {
	Type       ResourceType                       `json:"type"`
	ID         string                             `json:"id"`
	Attributes *AppEventVideoClipUpdateAttributes `json:"attributes,omitempty"`
}

// AppEventVideoClipUpdateRequest is a request to update an app event video clip.
type AppEventVideoClipUpdateRequest struct {
	Data AppEventVideoClipUpdateData `json:"data"`
}

// GetAppEvent retrieves an app event by ID.
func (c *Client) GetAppEvent(ctx context.Context, eventID string) (*AppEventResponse, error) {
	eventID = strings.TrimSpace(eventID)
	if eventID == "" {
		return nil, fmt.Errorf("eventID is required")
	}
	return getJSON[AppEventResponse](ctx, c, readRequest{Path: fmt.Sprintf("/v1/appEvents/%s", eventID)})
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

// UpdateAppEvent updates an app event.
func (c *Client) UpdateAppEvent(ctx context.Context, eventID string, attrs AppEventUpdateAttributes) (*AppEventResponse, error) {
	eventID = strings.TrimSpace(eventID)
	if eventID == "" {
		return nil, fmt.Errorf("eventID is required")
	}

	payload := AppEventUpdateRequest{
		Data: AppEventUpdateData{
			Type:       ResourceTypeAppEvents,
			ID:         eventID,
			Attributes: &attrs,
		},
	}

	return sendJSON[AppEventResponse](ctx, c, writeRequest{
		Method:  http.MethodPatch,
		Path:    fmt.Sprintf("/v1/appEvents/%s", eventID),
		Payload: payload,
	})
}

// DeleteAppEvent deletes an app event.
func (c *Client) DeleteAppEvent(ctx context.Context, eventID string) error {
	eventID = strings.TrimSpace(eventID)
	if eventID == "" {
		return fmt.Errorf("eventID is required")
	}
	return sendNoContent(ctx, c, http.MethodDelete, fmt.Sprintf("/v1/appEvents/%s", eventID), nil)
}

// GetAppEventLocalizations retrieves localizations for an app event.
func (c *Client) GetAppEventLocalizations(ctx context.Context, eventID string, opts ...AppEventLocalizationsOption) (*AppEventLocalizationsResponse, error) {
	query := applyOptions(opts)

	eventID = strings.TrimSpace(eventID)
	if query.nextURL == "" && eventID == "" {
		return nil, fmt.Errorf("eventID is required")
	}

	return getJSON[AppEventLocalizationsResponse](ctx, c, readRequest{
		Path:        fmt.Sprintf("/v1/appEvents/%s/localizations", eventID),
		Query:       buildAppEventLocalizationsQuery(query),
		Next:        query.nextURL,
		NextContext: "appEventLocalizations",
	})
}

// GetAppEventLocalizationsRelationships retrieves localization linkages for an app event.
func (c *Client) GetAppEventLocalizationsRelationships(ctx context.Context, eventID string, opts ...LinkagesOption) (*AppEventLocalizationsLinkagesResponse, error) {
	query := applyOptions(opts)

	eventID = strings.TrimSpace(eventID)
	if query.nextURL == "" && eventID == "" {
		return nil, fmt.Errorf("eventID is required")
	}

	return getJSON[AppEventLocalizationsLinkagesResponse](ctx, c, readRequest{
		Path:        fmt.Sprintf("/v1/appEvents/%s/relationships/localizations", eventID),
		Query:       buildLinkagesQuery(query),
		Next:        query.nextURL,
		NextContext: "appEventLocalizationsRelationships",
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

// UpdateAppEventLocalization updates an app event localization.
func (c *Client) UpdateAppEventLocalization(ctx context.Context, localizationID string, attrs AppEventLocalizationUpdateAttributes) (*AppEventLocalizationResponse, error) {
	localizationID = strings.TrimSpace(localizationID)
	if localizationID == "" {
		return nil, fmt.Errorf("localizationID is required")
	}

	payload := AppEventLocalizationUpdateRequest{
		Data: AppEventLocalizationUpdateData{
			Type:       ResourceTypeAppEventLocalizations,
			ID:         localizationID,
			Attributes: &attrs,
		},
	}

	return sendJSON[AppEventLocalizationResponse](ctx, c, writeRequest{
		Method:  http.MethodPatch,
		Path:    fmt.Sprintf("/v1/appEventLocalizations/%s", localizationID),
		Payload: payload,
	})
}

// DeleteAppEventLocalization deletes an app event localization.
func (c *Client) DeleteAppEventLocalization(ctx context.Context, localizationID string) error {
	localizationID = strings.TrimSpace(localizationID)
	if localizationID == "" {
		return fmt.Errorf("localizationID is required")
	}
	return sendNoContent(ctx, c, http.MethodDelete, fmt.Sprintf("/v1/appEventLocalizations/%s", localizationID), nil)
}

// GetAppEventScreenshots retrieves screenshots for an app event localization.
func (c *Client) GetAppEventScreenshots(ctx context.Context, localizationID string, opts ...AppEventScreenshotsOption) (*AppEventScreenshotsResponse, error) {
	query := applyOptions(opts)

	localizationID = strings.TrimSpace(localizationID)
	if query.nextURL == "" && localizationID == "" {
		return nil, fmt.Errorf("localizationID is required")
	}

	return getJSON[AppEventScreenshotsResponse](ctx, c, readRequest{
		Path:        fmt.Sprintf("/v1/appEventLocalizations/%s/appEventScreenshots", localizationID),
		Query:       buildAppEventScreenshotsQuery(query),
		Next:        query.nextURL,
		NextContext: "appEventScreenshots",
	})
}

// GetAppEventScreenshotsRelationships retrieves screenshot linkages for an app event localization.
func (c *Client) GetAppEventScreenshotsRelationships(ctx context.Context, localizationID string, opts ...LinkagesOption) (*AppEventLocalizationScreenshotsLinkagesResponse, error) {
	query := applyOptions(opts)

	localizationID = strings.TrimSpace(localizationID)
	if query.nextURL == "" && localizationID == "" {
		return nil, fmt.Errorf("localizationID is required")
	}

	return getJSON[AppEventLocalizationScreenshotsLinkagesResponse](ctx, c, readRequest{
		Path:        fmt.Sprintf("/v1/appEventLocalizations/%s/relationships/appEventScreenshots", localizationID),
		Query:       buildLinkagesQuery(query),
		Next:        query.nextURL,
		NextContext: "appEventScreenshotsRelationships",
	})
}

// GetAppEventScreenshot retrieves an app event screenshot by ID.
func (c *Client) GetAppEventScreenshot(ctx context.Context, screenshotID string) (*AppEventScreenshotResponse, error) {
	screenshotID = strings.TrimSpace(screenshotID)
	if screenshotID == "" {
		return nil, fmt.Errorf("screenshotID is required")
	}
	return getJSON[AppEventScreenshotResponse](ctx, c, readRequest{Path: fmt.Sprintf("/v1/appEventScreenshots/%s", screenshotID)})
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

// DeleteAppEventScreenshot deletes an app event screenshot.
func (c *Client) DeleteAppEventScreenshot(ctx context.Context, screenshotID string) error {
	screenshotID = strings.TrimSpace(screenshotID)
	if screenshotID == "" {
		return fmt.Errorf("screenshotID is required")
	}
	return sendNoContent(ctx, c, http.MethodDelete, fmt.Sprintf("/v1/appEventScreenshots/%s", screenshotID), nil)
}

// GetAppEventVideoClips retrieves video clips for an app event localization.
func (c *Client) GetAppEventVideoClips(ctx context.Context, localizationID string, opts ...AppEventVideoClipsOption) (*AppEventVideoClipsResponse, error) {
	query := applyOptions(opts)

	localizationID = strings.TrimSpace(localizationID)
	if query.nextURL == "" && localizationID == "" {
		return nil, fmt.Errorf("localizationID is required")
	}

	return getJSON[AppEventVideoClipsResponse](ctx, c, readRequest{
		Path:        fmt.Sprintf("/v1/appEventLocalizations/%s/appEventVideoClips", localizationID),
		Query:       buildAppEventVideoClipsQuery(query),
		Next:        query.nextURL,
		NextContext: "appEventVideoClips",
	})
}

// GetAppEventVideoClipsRelationships retrieves video clip linkages for an app event localization.
func (c *Client) GetAppEventVideoClipsRelationships(ctx context.Context, localizationID string, opts ...LinkagesOption) (*AppEventLocalizationVideoClipsLinkagesResponse, error) {
	query := applyOptions(opts)

	localizationID = strings.TrimSpace(localizationID)
	if query.nextURL == "" && localizationID == "" {
		return nil, fmt.Errorf("localizationID is required")
	}

	return getJSON[AppEventLocalizationVideoClipsLinkagesResponse](ctx, c, readRequest{
		Path:        fmt.Sprintf("/v1/appEventLocalizations/%s/relationships/appEventVideoClips", localizationID),
		Query:       buildLinkagesQuery(query),
		Next:        query.nextURL,
		NextContext: "appEventVideoClipsRelationships",
	})
}

// GetAppEventVideoClip retrieves an app event video clip by ID.
func (c *Client) GetAppEventVideoClip(ctx context.Context, clipID string) (*AppEventVideoClipResponse, error) {
	clipID = strings.TrimSpace(clipID)
	if clipID == "" {
		return nil, fmt.Errorf("clipID is required")
	}
	return getJSON[AppEventVideoClipResponse](ctx, c, readRequest{Path: fmt.Sprintf("/v1/appEventVideoClips/%s", clipID)})
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

// UpdateAppEventVideoClip updates an app event video clip.
func (c *Client) UpdateAppEventVideoClip(ctx context.Context, clipID string, attrs AppEventVideoClipUpdateAttributes) (*AppEventVideoClipResponse, error) {
	clipID = strings.TrimSpace(clipID)
	if clipID == "" {
		return nil, fmt.Errorf("clipID is required")
	}

	payload := AppEventVideoClipUpdateRequest{
		Data: AppEventVideoClipUpdateData{
			Type:       ResourceTypeAppEventVideoClips,
			ID:         clipID,
			Attributes: &attrs,
		},
	}

	return sendJSON[AppEventVideoClipResponse](ctx, c, writeRequest{
		Method:  http.MethodPatch,
		Path:    fmt.Sprintf("/v1/appEventVideoClips/%s", clipID),
		Payload: payload,
	})
}

// DeleteAppEventVideoClip deletes an app event video clip.
func (c *Client) DeleteAppEventVideoClip(ctx context.Context, clipID string) error {
	clipID = strings.TrimSpace(clipID)
	if clipID == "" {
		return fmt.Errorf("clipID is required")
	}
	return sendNoContent(ctx, c, http.MethodDelete, fmt.Sprintf("/v1/appEventVideoClips/%s", clipID), nil)
}
