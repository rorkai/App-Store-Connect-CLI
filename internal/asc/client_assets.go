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

// AppPreviewUpdateAttributes describes preview update attributes.
type AppPreviewUpdateAttributes struct {
	PreviewFrameTimeCode *string `json:"previewFrameTimeCode,omitempty"`
	SourceFileChecksum   *string `json:"sourceFileChecksum,omitempty"`
	Uploaded             *bool   `json:"uploaded,omitempty"`
}

// GetAppScreenshotSets retrieves screenshot sets for a localization.
func (c *Client) GetAppScreenshotSets(ctx context.Context, localizationID string, opts ...AppScreenshotSetsOption) (*AppScreenshotSetsResponse, error) {
	query := applyOptions(opts)
	if query.limitSet && (query.limit < 1 || query.limit > appScreenshotCollectionLimitMax) {
		return nil, fmt.Errorf("appScreenshotSets: limit must be between 1 and %d", appScreenshotCollectionLimitMax)
	}

	return getJSON[AppScreenshotSetsResponse](ctx, c, readRequest{
		Path:        fmt.Sprintf("/v1/appStoreVersionLocalizations/%s/appScreenshotSets", localizationID),
		Query:       buildAppScreenshotSetsQuery(query),
		Next:        query.nextURL,
		NextContext: "appScreenshotSets",
	})
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

// CreateAppScreenshotSetForExperimentTreatmentLocalization creates a screenshot set for a treatment localization.
func (c *Client) CreateAppScreenshotSetForExperimentTreatmentLocalization(ctx context.Context, localizationID string, displayType string) (*AppScreenshotSetResponse, error) {
	return c.createAppScreenshotSet(ctx, displayType, &AppScreenshotSetRelationships{
		AppStoreVersionExperimentTreatmentLocalization: &Relationship{
			Data: ResourceData{
				Type: ResourceTypeAppStoreVersionExperimentTreatmentLocalizations,
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

	return sendJSON[AppScreenshotSetResponse](ctx, c, writeRequest{Method: "POST", Path: "/v1/appScreenshotSets", Payload: payload})
}

// GetAppScreenshots retrieves screenshots for a set.
func (c *Client) GetAppScreenshots(ctx context.Context, setID string, opts ...AppScreenshotsOption) (*AppScreenshotsResponse, error) {
	query := applyOptions(opts)
	if query.limitSet && (query.limit < 1 || query.limit > appScreenshotCollectionLimitMax) {
		return nil, fmt.Errorf("appScreenshots: limit must be between 1 and %d", appScreenshotCollectionLimitMax)
	}

	return getJSON[AppScreenshotsResponse](ctx, c, readRequest{
		Path:        fmt.Sprintf("/v1/appScreenshotSets/%s/appScreenshots", setID),
		Query:       buildAppScreenshotsQuery(query),
		Next:        query.nextURL,
		NextContext: "appScreenshots",
	})
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

	return sendJSON[AppScreenshotResponse](ctx, c, writeRequest{Method: "POST", Path: "/v1/appScreenshots", Payload: payload})
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
		return nil, &screenshotCommitDecodeError{fmt.Errorf("failed to parse response: %w", err)}
	}

	return &response, nil
}

// GetAppPreviewSets retrieves preview sets for a localization.
func (c *Client) GetAppPreviewSets(ctx context.Context, localizationID string) (*AppPreviewSetsResponse, error) {
	return getJSON[AppPreviewSetsResponse](ctx, c, readRequest{Path: fmt.Sprintf("/v1/appStoreVersionLocalizations/%s/appPreviewSets", localizationID)})
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

// CreateAppPreviewSetForExperimentTreatmentLocalization creates a preview set for a treatment localization.
func (c *Client) CreateAppPreviewSetForExperimentTreatmentLocalization(ctx context.Context, localizationID string, previewType string) (*AppPreviewSetResponse, error) {
	return c.createAppPreviewSet(ctx, previewType, &AppPreviewSetRelationships{
		AppStoreVersionExperimentTreatmentLocalization: &Relationship{
			Data: ResourceData{
				Type: ResourceTypeAppStoreVersionExperimentTreatmentLocalizations,
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

	return sendJSON[AppPreviewSetResponse](ctx, c, writeRequest{Method: "POST", Path: "/v1/appPreviewSets", Payload: payload})
}

type appPreviewsQuery struct {
	nextURL string
}

// AppPreviewsOption configures app preview queries.
type AppPreviewsOption func(*appPreviewsQuery)

// WithAppPreviewsNextURL uses a next page URL directly.
func WithAppPreviewsNextURL(next string) AppPreviewsOption {
	return func(q *appPreviewsQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

// GetAppPreviews retrieves previews for a set.
func (c *Client) GetAppPreviews(ctx context.Context, setID string, opts ...AppPreviewsOption) (*AppPreviewsResponse, error) {
	query := applyOptions(opts)
	return getJSON[AppPreviewsResponse](ctx, c, readRequest{
		Path:        fmt.Sprintf("/v1/appPreviewSets/%s/appPreviews", setID),
		Next:        query.nextURL,
		NextContext: "appPreviews",
	})
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

	return sendJSON[AppPreviewResponse](ctx, c, writeRequest{Method: "POST", Path: "/v1/appPreviews", Payload: payload})
}
