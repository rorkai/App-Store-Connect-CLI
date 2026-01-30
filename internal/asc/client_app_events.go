package asc

import (
	"context"
	"encoding/json"
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

// GetAppEvents retrieves app events for an app.
func (c *Client) GetAppEvents(ctx context.Context, appID string, opts ...AppEventsOption) (*AppEventsResponse, error) {
	query := &appEventsQuery{}
	for _, opt := range opts {
		opt(query)
	}

	appID = strings.TrimSpace(appID)
	if query.nextURL == "" && appID == "" {
		return nil, fmt.Errorf("appID is required")
	}

	path := fmt.Sprintf("/v1/apps/%s/appEvents", appID)
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("appEvents: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildAppEventsQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response AppEventsResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetAppEvent retrieves an app event by ID.
func (c *Client) GetAppEvent(ctx context.Context, eventID string) (*AppEventResponse, error) {
	eventID = strings.TrimSpace(eventID)
	if eventID == "" {
		return nil, fmt.Errorf("eventID is required")
	}
	path := fmt.Sprintf("/v1/appEvents/%s", eventID)
	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response AppEventResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
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

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	data, err := c.do(ctx, http.MethodPost, "/v1/appEvents", body)
	if err != nil {
		return nil, err
	}

	var response AppEventResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
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

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/v1/appEvents/%s", eventID)
	data, err := c.do(ctx, http.MethodPatch, path, body)
	if err != nil {
		return nil, err
	}

	var response AppEventResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// DeleteAppEvent deletes an app event.
func (c *Client) DeleteAppEvent(ctx context.Context, eventID string) error {
	eventID = strings.TrimSpace(eventID)
	if eventID == "" {
		return fmt.Errorf("eventID is required")
	}
	path := fmt.Sprintf("/v1/appEvents/%s", eventID)
	_, err := c.do(ctx, http.MethodDelete, path, nil)
	return err
}

// GetAppEventLocalizations retrieves localizations for an app event.
func (c *Client) GetAppEventLocalizations(ctx context.Context, eventID string, opts ...AppEventLocalizationsOption) (*AppEventLocalizationsResponse, error) {
	query := &appEventLocalizationsQuery{}
	for _, opt := range opts {
		opt(query)
	}

	eventID = strings.TrimSpace(eventID)
	if query.nextURL == "" && eventID == "" {
		return nil, fmt.Errorf("eventID is required")
	}

	path := fmt.Sprintf("/v1/appEvents/%s/localizations", eventID)
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("appEventLocalizations: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildAppEventLocalizationsQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response AppEventLocalizationsResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetAppEventLocalizationsRelationships retrieves localization linkages for an app event.
func (c *Client) GetAppEventLocalizationsRelationships(ctx context.Context, eventID string, opts ...LinkagesOption) (*AppEventLocalizationsLinkagesResponse, error) {
	query := &linkagesQuery{}
	for _, opt := range opts {
		opt(query)
	}

	eventID = strings.TrimSpace(eventID)
	if query.nextURL == "" && eventID == "" {
		return nil, fmt.Errorf("eventID is required")
	}

	path := fmt.Sprintf("/v1/appEvents/%s/relationships/localizations", eventID)
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("appEventLocalizationsRelationships: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildLinkagesQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response AppEventLocalizationsLinkagesResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetAppEventLocalization retrieves an app event localization by ID.
func (c *Client) GetAppEventLocalization(ctx context.Context, localizationID string) (*AppEventLocalizationResponse, error) {
	localizationID = strings.TrimSpace(localizationID)
	if localizationID == "" {
		return nil, fmt.Errorf("localizationID is required")
	}
	path := fmt.Sprintf("/v1/appEventLocalizations/%s", localizationID)
	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response AppEventLocalizationResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
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

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	data, err := c.do(ctx, http.MethodPost, "/v1/appEventLocalizations", body)
	if err != nil {
		return nil, err
	}

	var response AppEventLocalizationResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
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

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	data, err := c.do(ctx, http.MethodPatch, fmt.Sprintf("/v1/appEventLocalizations/%s", localizationID), body)
	if err != nil {
		return nil, err
	}

	var response AppEventLocalizationResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// DeleteAppEventLocalization deletes an app event localization.
func (c *Client) DeleteAppEventLocalization(ctx context.Context, localizationID string) error {
	localizationID = strings.TrimSpace(localizationID)
	if localizationID == "" {
		return fmt.Errorf("localizationID is required")
	}
	path := fmt.Sprintf("/v1/appEventLocalizations/%s", localizationID)
	_, err := c.do(ctx, http.MethodDelete, path, nil)
	return err
}

// GetAppEventScreenshots retrieves screenshots for an app event localization.
func (c *Client) GetAppEventScreenshots(ctx context.Context, localizationID string, opts ...AppEventScreenshotsOption) (*AppEventScreenshotsResponse, error) {
	query := &appEventScreenshotsQuery{}
	for _, opt := range opts {
		opt(query)
	}

	localizationID = strings.TrimSpace(localizationID)
	if query.nextURL == "" && localizationID == "" {
		return nil, fmt.Errorf("localizationID is required")
	}

	path := fmt.Sprintf("/v1/appEventLocalizations/%s/appEventScreenshots", localizationID)
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("appEventScreenshots: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildAppEventScreenshotsQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response AppEventScreenshotsResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetAppEventScreenshotsRelationships retrieves screenshot linkages for an app event localization.
func (c *Client) GetAppEventScreenshotsRelationships(ctx context.Context, localizationID string, opts ...LinkagesOption) (*AppEventLocalizationScreenshotsLinkagesResponse, error) {
	query := &linkagesQuery{}
	for _, opt := range opts {
		opt(query)
	}

	localizationID = strings.TrimSpace(localizationID)
	if query.nextURL == "" && localizationID == "" {
		return nil, fmt.Errorf("localizationID is required")
	}

	path := fmt.Sprintf("/v1/appEventLocalizations/%s/relationships/appEventScreenshots", localizationID)
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("appEventScreenshotsRelationships: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildLinkagesQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response AppEventLocalizationScreenshotsLinkagesResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetAppEventScreenshot retrieves an app event screenshot by ID.
func (c *Client) GetAppEventScreenshot(ctx context.Context, screenshotID string) (*AppEventScreenshotResponse, error) {
	screenshotID = strings.TrimSpace(screenshotID)
	if screenshotID == "" {
		return nil, fmt.Errorf("screenshotID is required")
	}
	path := fmt.Sprintf("/v1/appEventScreenshots/%s", screenshotID)
	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response AppEventScreenshotResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
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

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	data, err := c.do(ctx, http.MethodPost, "/v1/appEventScreenshots", body)
	if err != nil {
		return nil, err
	}

	var response AppEventScreenshotResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
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

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/v1/appEventScreenshots/%s", screenshotID)
	data, err := c.do(ctx, http.MethodPatch, path, body)
	if err != nil {
		return nil, err
	}

	var response AppEventScreenshotResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// DeleteAppEventScreenshot deletes an app event screenshot.
func (c *Client) DeleteAppEventScreenshot(ctx context.Context, screenshotID string) error {
	screenshotID = strings.TrimSpace(screenshotID)
	if screenshotID == "" {
		return fmt.Errorf("screenshotID is required")
	}
	path := fmt.Sprintf("/v1/appEventScreenshots/%s", screenshotID)
	_, err := c.do(ctx, http.MethodDelete, path, nil)
	return err
}

// GetAppEventVideoClips retrieves video clips for an app event localization.
func (c *Client) GetAppEventVideoClips(ctx context.Context, localizationID string, opts ...AppEventVideoClipsOption) (*AppEventVideoClipsResponse, error) {
	query := &appEventVideoClipsQuery{}
	for _, opt := range opts {
		opt(query)
	}

	localizationID = strings.TrimSpace(localizationID)
	if query.nextURL == "" && localizationID == "" {
		return nil, fmt.Errorf("localizationID is required")
	}

	path := fmt.Sprintf("/v1/appEventLocalizations/%s/appEventVideoClips", localizationID)
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("appEventVideoClips: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildAppEventVideoClipsQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response AppEventVideoClipsResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetAppEventVideoClipsRelationships retrieves video clip linkages for an app event localization.
func (c *Client) GetAppEventVideoClipsRelationships(ctx context.Context, localizationID string, opts ...LinkagesOption) (*AppEventLocalizationVideoClipsLinkagesResponse, error) {
	query := &linkagesQuery{}
	for _, opt := range opts {
		opt(query)
	}

	localizationID = strings.TrimSpace(localizationID)
	if query.nextURL == "" && localizationID == "" {
		return nil, fmt.Errorf("localizationID is required")
	}

	path := fmt.Sprintf("/v1/appEventLocalizations/%s/relationships/appEventVideoClips", localizationID)
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("appEventVideoClipsRelationships: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildLinkagesQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response AppEventLocalizationVideoClipsLinkagesResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetAppEventVideoClip retrieves an app event video clip by ID.
func (c *Client) GetAppEventVideoClip(ctx context.Context, clipID string) (*AppEventVideoClipResponse, error) {
	clipID = strings.TrimSpace(clipID)
	if clipID == "" {
		return nil, fmt.Errorf("clipID is required")
	}
	path := fmt.Sprintf("/v1/appEventVideoClips/%s", clipID)
	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response AppEventVideoClipResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
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

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	data, err := c.do(ctx, http.MethodPost, "/v1/appEventVideoClips", body)
	if err != nil {
		return nil, err
	}

	var response AppEventVideoClipResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
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

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/v1/appEventVideoClips/%s", clipID)
	data, err := c.do(ctx, http.MethodPatch, path, body)
	if err != nil {
		return nil, err
	}

	var response AppEventVideoClipResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// DeleteAppEventVideoClip deletes an app event video clip.
func (c *Client) DeleteAppEventVideoClip(ctx context.Context, clipID string) error {
	clipID = strings.TrimSpace(clipID)
	if clipID == "" {
		return fmt.Errorf("clipID is required")
	}
	path := fmt.Sprintf("/v1/appEventVideoClips/%s", clipID)
	_, err := c.do(ctx, http.MethodDelete, path, nil)
	return err
}
