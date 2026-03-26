package asc

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// AppScreenshotSetRelationships describes relationships for screenshot sets.
type AppScreenshotSetRelationships struct {
	AppStoreVersionLocalization                    *Relationship `json:"appStoreVersionLocalization,omitempty"`
	AppCustomProductPageLocalization               *Relationship `json:"appCustomProductPageLocalization,omitempty"`
	AppStoreVersionExperimentTreatmentLocalization *Relationship `json:"appStoreVersionExperimentTreatmentLocalization,omitempty"`
}

// AppScreenshotSetCreateData is the data portion of a screenshot set create request.
type AppScreenshotSetCreateData struct {
	Type          ResourceType                   `json:"type"`
	Attributes    AppScreenshotSetAttributes     `json:"attributes"`
	Relationships *AppScreenshotSetRelationships `json:"relationships"`
}

// AppScreenshotSetCreateRequest is a request to create a screenshot set.
type AppScreenshotSetCreateRequest struct {
	Data AppScreenshotSetCreateData `json:"data"`
}

// AppScreenshotRelationships describes relationships for screenshots.
type AppScreenshotRelationships struct {
	AppScreenshotSet *Relationship `json:"appScreenshotSet"`
}

// AppScreenshotCreateData is the data portion of a screenshot create request.
type AppScreenshotCreateData struct {
	Type          ResourceType                `json:"type"`
	Attributes    AppScreenshotAttributes     `json:"attributes"`
	Relationships *AppScreenshotRelationships `json:"relationships"`
}

// AppScreenshotCreateRequest is a request to create a screenshot.
type AppScreenshotCreateRequest struct {
	Data AppScreenshotCreateData `json:"data"`
}

// AppScreenshotUpdateAttributes describes screenshot update attributes.
type AppScreenshotUpdateAttributes struct {
	SourceFileChecksum *string `json:"sourceFileChecksum,omitempty"`
	Uploaded           *bool   `json:"uploaded,omitempty"`
}

// AppScreenshotUpdateData is the data portion of a screenshot update request.
type AppScreenshotUpdateData struct {
	Type       ResourceType                   `json:"type"`
	ID         string                         `json:"id"`
	Attributes *AppScreenshotUpdateAttributes `json:"attributes,omitempty"`
}

// AppScreenshotUpdateRequest is a request to update a screenshot.
type AppScreenshotUpdateRequest struct {
	Data AppScreenshotUpdateData `json:"data"`
}

// AppPreviewSetRelationships describes relationships for preview sets.
type AppPreviewSetRelationships struct {
	AppStoreVersionLocalization                    *Relationship `json:"appStoreVersionLocalization,omitempty"`
	AppCustomProductPageLocalization               *Relationship `json:"appCustomProductPageLocalization,omitempty"`
	AppStoreVersionExperimentTreatmentLocalization *Relationship `json:"appStoreVersionExperimentTreatmentLocalization,omitempty"`
}

// AppPreviewSetCreateData is the data portion of a preview set create request.
type AppPreviewSetCreateData struct {
	Type          ResourceType                `json:"type"`
	Attributes    AppPreviewSetAttributes     `json:"attributes"`
	Relationships *AppPreviewSetRelationships `json:"relationships"`
}

// AppPreviewSetCreateRequest is a request to create a preview set.
type AppPreviewSetCreateRequest struct {
	Data AppPreviewSetCreateData `json:"data"`
}

// AppPreviewRelationships describes relationships for previews.
type AppPreviewRelationships struct {
	AppPreviewSet *Relationship `json:"appPreviewSet"`
}

// AppPreviewCreateData is the data portion of a preview create request.
type AppPreviewCreateData struct {
	Type          ResourceType             `json:"type"`
	Attributes    AppPreviewAttributes     `json:"attributes"`
	Relationships *AppPreviewRelationships `json:"relationships"`
}

// AppPreviewCreateRequest is a request to create a preview.
type AppPreviewCreateRequest struct {
	Data AppPreviewCreateData `json:"data"`
}

// AppPreviewUpdateAttributes describes preview update attributes.
type AppPreviewUpdateAttributes struct {
	PreviewFrameTimeCode *string `json:"previewFrameTimeCode,omitempty"`
	SourceFileChecksum   *string `json:"sourceFileChecksum,omitempty"`
	Uploaded             *bool   `json:"uploaded,omitempty"`
}

// AppPreviewUpdateData is the data portion of a preview update request.
type AppPreviewUpdateData struct {
	Type       ResourceType                `json:"type"`
	ID         string                      `json:"id"`
	Attributes *AppPreviewUpdateAttributes `json:"attributes,omitempty"`
}

// AppPreviewUpdateRequest is a request to update a preview.
type AppPreviewUpdateRequest struct {
	Data AppPreviewUpdateData `json:"data"`
}

// GetAppScreenshotSets retrieves screenshot sets for a localization.
func (c *Client) GetAppScreenshotSets(ctx context.Context, localizationID string) (*AppScreenshotSetsResponse, error) {
	path := fmt.Sprintf("/v1/appStoreVersionLocalizations/%s/appScreenshotSets", localizationID)
	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response AppScreenshotSetsResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetAppScreenshotSet retrieves a screenshot set by ID.
func (c *Client) GetAppScreenshotSet(ctx context.Context, setID string) (*AppScreenshotSetResponse, error) {
	path := fmt.Sprintf("/v1/appScreenshotSets/%s", setID)
	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response AppScreenshotSetResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// CreateAppScreenshotSet creates a screenshot set for a localization.
func (c *Client) CreateAppScreenshotSet(ctx context.Context, localizationID string, displayType string) (*AppScreenshotSetResponse, error) {
	return c.createAppScreenshotSet(ctx, displayType, &AppScreenshotSetRelationships{
		AppStoreVersionLocalization: &Relationship{
			Data: ResourceData{
				Type: ResourceTypeAppStoreVersionLocalizations,
				ID:   localizationID,
			},
		},
	})
}

// CreateAppScreenshotSetForCustomProductPageLocalization creates a screenshot set for a custom product page localization.
func (c *Client) CreateAppScreenshotSetForCustomProductPageLocalization(ctx context.Context, localizationID string, displayType string) (*AppScreenshotSetResponse, error) {
	return c.createAppScreenshotSet(ctx, displayType, &AppScreenshotSetRelationships{
		AppCustomProductPageLocalization: &Relationship{
			Data: ResourceData{
				Type: ResourceTypeAppCustomProductPageLocalizations,
				ID:   localizationID,
			},
		},
	})
}

func (c *Client) createAppScreenshotSet(ctx context.Context, displayType string, relationships *AppScreenshotSetRelationships) (*AppScreenshotSetResponse, error) {
	payload := AppScreenshotSetCreateRequest{
		Data: AppScreenshotSetCreateData{
			Type:          ResourceTypeAppScreenshotSets,
			Attributes:    AppScreenshotSetAttributes{ScreenshotDisplayType: displayType},
			Relationships: relationships,
		},
	}

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	data, err := c.do(ctx, "POST", "/v1/appScreenshotSets", body)
	if err != nil {
		return nil, err
	}

	var response AppScreenshotSetResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// DeleteAppScreenshotSet deletes a screenshot set by ID.
func (c *Client) DeleteAppScreenshotSet(ctx context.Context, setID string) error {
	path := fmt.Sprintf("/v1/appScreenshotSets/%s", setID)
	_, err := c.do(ctx, "DELETE", path, nil)
	return err
}

// GetAppScreenshots retrieves screenshots for a set.
func (c *Client) GetAppScreenshots(ctx context.Context, setID string) (*AppScreenshotsResponse, error) {
	path := fmt.Sprintf("/v1/appScreenshotSets/%s/appScreenshots", setID)
	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response AppScreenshotsResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetAppScreenshotSetAppScreenshotsRelationships retrieves screenshot linkages for a screenshot set.
func (c *Client) GetAppScreenshotSetAppScreenshotsRelationships(ctx context.Context, setID string, opts ...LinkagesOption) (*LinkagesResponse, error) {
	query := &linkagesQuery{}
	for _, opt := range opts {
		opt(query)
	}

	setID = strings.TrimSpace(setID)
	if query.nextURL == "" && setID == "" {
		return nil, fmt.Errorf("setID is required")
	}

	path := fmt.Sprintf("/v1/appScreenshotSets/%s/relationships/appScreenshots", setID)
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("appScreenshotsRelationships: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildLinkagesQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response LinkagesResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// UpdateAppScreenshotSetAppScreenshotsRelationship replaces the screenshots on a screenshot set.
func (c *Client) UpdateAppScreenshotSetAppScreenshotsRelationship(ctx context.Context, setID string, screenshotIDs []string) error {
	setID = strings.TrimSpace(setID)
	if setID == "" {
		return fmt.Errorf("setID is required")
	}

	data := buildRelationshipData(ResourceTypeAppScreenshots, screenshotIDs)
	if data == nil {
		data = []RelationshipData{}
	}
	payload := RelationshipRequest{Data: data}
	body, err := BuildRequestBody(payload)
	if err != nil {
		return err
	}

	path := fmt.Sprintf("/v1/appScreenshotSets/%s/relationships/appScreenshots", setID)
	_, err = c.do(ctx, "PATCH", path, body)
	return err
}

// GetAppScreenshot retrieves a screenshot by ID.
func (c *Client) GetAppScreenshot(ctx context.Context, screenshotID string) (*AppScreenshotResponse, error) {
	path := fmt.Sprintf("/v1/appScreenshots/%s", screenshotID)
	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response AppScreenshotResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// CreateAppScreenshot creates a screenshot reservation.
func (c *Client) CreateAppScreenshot(ctx context.Context, setID string, fileName string, fileSize int64) (*AppScreenshotResponse, error) {
	payload := AppScreenshotCreateRequest{
		Data: AppScreenshotCreateData{
			Type: ResourceTypeAppScreenshots,
			Attributes: AppScreenshotAttributes{
				FileName: fileName,
				FileSize: fileSize,
			},
			Relationships: &AppScreenshotRelationships{
				AppScreenshotSet: &Relationship{
					Data: ResourceData{
						Type: ResourceTypeAppScreenshotSets,
						ID:   setID,
					},
				},
			},
		},
	}

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	data, err := c.do(ctx, "POST", "/v1/appScreenshots", body)
	if err != nil {
		return nil, err
	}

	var response AppScreenshotResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// UpdateAppScreenshot updates a screenshot (used to commit upload).
func (c *Client) UpdateAppScreenshot(ctx context.Context, screenshotID string, uploaded bool, checksumHash string) (*AppScreenshotResponse, error) {
	payload := AppScreenshotUpdateRequest{
		Data: AppScreenshotUpdateData{
			Type: ResourceTypeAppScreenshots,
			ID:   screenshotID,
			Attributes: &AppScreenshotUpdateAttributes{
				Uploaded:           &uploaded,
				SourceFileChecksum: &checksumHash,
			},
		},
	}

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	data, err := c.do(ctx, "PATCH", fmt.Sprintf("/v1/appScreenshots/%s", screenshotID), body)
	if err != nil {
		return nil, err
	}

	var response AppScreenshotResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// DeleteAppScreenshot deletes a screenshot by ID.
func (c *Client) DeleteAppScreenshot(ctx context.Context, screenshotID string) error {
	path := fmt.Sprintf("/v1/appScreenshots/%s", screenshotID)
	_, err := c.do(ctx, "DELETE", path, nil)
	return err
}

// GetAppPreviewSets retrieves preview sets for a localization.
func (c *Client) GetAppPreviewSets(ctx context.Context, localizationID string) (*AppPreviewSetsResponse, error) {
	path := fmt.Sprintf("/v1/appStoreVersionLocalizations/%s/appPreviewSets", localizationID)
	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response AppPreviewSetsResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetAppPreviewSet retrieves a preview set by ID.
func (c *Client) GetAppPreviewSet(ctx context.Context, setID string) (*AppPreviewSetResponse, error) {
	path := fmt.Sprintf("/v1/appPreviewSets/%s", setID)
	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response AppPreviewSetResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// CreateAppPreviewSet creates a preview set for a localization.
func (c *Client) CreateAppPreviewSet(ctx context.Context, localizationID string, previewType string) (*AppPreviewSetResponse, error) {
	return c.createAppPreviewSet(ctx, previewType, &AppPreviewSetRelationships{
		AppStoreVersionLocalization: &Relationship{
			Data: ResourceData{
				Type: ResourceTypeAppStoreVersionLocalizations,
				ID:   localizationID,
			},
		},
	})
}

// CreateAppPreviewSetForCustomProductPageLocalization creates a preview set for a custom product page localization.
func (c *Client) CreateAppPreviewSetForCustomProductPageLocalization(ctx context.Context, localizationID string, previewType string) (*AppPreviewSetResponse, error) {
	return c.createAppPreviewSet(ctx, previewType, &AppPreviewSetRelationships{
		AppCustomProductPageLocalization: &Relationship{
			Data: ResourceData{
				Type: ResourceTypeAppCustomProductPageLocalizations,
				ID:   localizationID,
			},
		},
	})
}

func (c *Client) createAppPreviewSet(ctx context.Context, previewType string, relationships *AppPreviewSetRelationships) (*AppPreviewSetResponse, error) {
	payload := AppPreviewSetCreateRequest{
		Data: AppPreviewSetCreateData{
			Type:          ResourceTypeAppPreviewSets,
			Attributes:    AppPreviewSetAttributes{PreviewType: previewType},
			Relationships: relationships,
		},
	}

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	data, err := c.do(ctx, "POST", "/v1/appPreviewSets", body)
	if err != nil {
		return nil, err
	}

	var response AppPreviewSetResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// DeleteAppPreviewSet deletes a preview set by ID.
func (c *Client) DeleteAppPreviewSet(ctx context.Context, setID string) error {
	path := fmt.Sprintf("/v1/appPreviewSets/%s", setID)
	_, err := c.do(ctx, "DELETE", path, nil)
	return err
}

// GetAppPreviews retrieves previews for a set.
func (c *Client) GetAppPreviews(ctx context.Context, setID string) (*AppPreviewsResponse, error) {
	path := fmt.Sprintf("/v1/appPreviewSets/%s/appPreviews", setID)
	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response AppPreviewsResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetAppPreviewSetAppPreviewsRelationships retrieves preview linkages for a preview set.
func (c *Client) GetAppPreviewSetAppPreviewsRelationships(ctx context.Context, setID string, opts ...LinkagesOption) (*LinkagesResponse, error) {
	query := &linkagesQuery{}
	for _, opt := range opts {
		opt(query)
	}

	setID = strings.TrimSpace(setID)
	if query.nextURL == "" && setID == "" {
		return nil, fmt.Errorf("setID is required")
	}

	path := fmt.Sprintf("/v1/appPreviewSets/%s/relationships/appPreviews", setID)
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("appPreviewsRelationships: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildLinkagesQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response LinkagesResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// UpdateAppPreviewSetAppPreviewsRelationship replaces the previews on a preview set.
func (c *Client) UpdateAppPreviewSetAppPreviewsRelationship(ctx context.Context, setID string, previewIDs []string) error {
	setID = strings.TrimSpace(setID)
	if setID == "" {
		return fmt.Errorf("setID is required")
	}

	data := buildRelationshipData(ResourceTypeAppPreviews, previewIDs)
	if data == nil {
		data = []RelationshipData{}
	}
	payload := RelationshipRequest{Data: data}
	body, err := BuildRequestBody(payload)
	if err != nil {
		return err
	}

	path := fmt.Sprintf("/v1/appPreviewSets/%s/relationships/appPreviews", setID)
	_, err = c.do(ctx, "PATCH", path, body)
	return err
}

// GetAppPreview retrieves a preview by ID.
func (c *Client) GetAppPreview(ctx context.Context, previewID string) (*AppPreviewResponse, error) {
	path := fmt.Sprintf("/v1/appPreviews/%s", previewID)
	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response AppPreviewResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// CreateAppPreview creates a preview reservation.
func (c *Client) CreateAppPreview(ctx context.Context, setID string, fileName string, fileSize int64, mimeType string) (*AppPreviewResponse, error) {
	payload := AppPreviewCreateRequest{
		Data: AppPreviewCreateData{
			Type: ResourceTypeAppPreviews,
			Attributes: AppPreviewAttributes{
				FileName: fileName,
				FileSize: fileSize,
				MimeType: mimeType,
			},
			Relationships: &AppPreviewRelationships{
				AppPreviewSet: &Relationship{
					Data: ResourceData{
						Type: ResourceTypeAppPreviewSets,
						ID:   setID,
					},
				},
			},
		},
	}

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	data, err := c.do(ctx, "POST", "/v1/appPreviews", body)
	if err != nil {
		return nil, err
	}

	var response AppPreviewResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// UpdateAppPreview updates a preview (used to commit upload).
func (c *Client) UpdateAppPreview(ctx context.Context, previewID string, uploaded bool, checksumHash string) (*AppPreviewResponse, error) {
	payload := AppPreviewUpdateRequest{
		Data: AppPreviewUpdateData{
			Type: ResourceTypeAppPreviews,
			ID:   previewID,
			Attributes: &AppPreviewUpdateAttributes{
				Uploaded:           &uploaded,
				SourceFileChecksum: &checksumHash,
			},
		},
	}

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	data, err := c.do(ctx, "PATCH", fmt.Sprintf("/v1/appPreviews/%s", previewID), body)
	if err != nil {
		return nil, err
	}

	var response AppPreviewResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// SetAppPreviewFrameTimeCode sets the poster frame timecode for a preview.
func (c *Client) SetAppPreviewFrameTimeCode(ctx context.Context, previewID string, timeCode string) (*AppPreviewResponse, error) {
	payload := AppPreviewUpdateRequest{
		Data: AppPreviewUpdateData{
			Type: ResourceTypeAppPreviews,
			ID:   previewID,
			Attributes: &AppPreviewUpdateAttributes{
				PreviewFrameTimeCode: &timeCode,
			},
		},
	}

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	data, err := c.do(ctx, "PATCH", fmt.Sprintf("/v1/appPreviews/%s", previewID), body)
	if err != nil {
		return nil, err
	}

	var response AppPreviewResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// DeleteAppPreview deletes a preview by ID.
func (c *Client) DeleteAppPreview(ctx context.Context, previewID string) error {
	path := fmt.Sprintf("/v1/appPreviews/%s", previewID)
	_, err := c.do(ctx, "DELETE", path, nil)
	return err
}
