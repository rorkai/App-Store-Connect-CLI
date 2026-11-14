package asc

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// AppStoreReviewAttachmentAttributes describes attachment attributes.
type AppStoreReviewAttachmentAttributes struct {
	FileSize           int64               `json:"fileSize,omitempty"`
	FileName           string              `json:"fileName,omitempty"`
	SourceFileChecksum string              `json:"sourceFileChecksum,omitempty"`
	UploadOperations   []UploadOperation   `json:"uploadOperations,omitempty"`
	AssetDeliveryState *AppMediaAssetState `json:"assetDeliveryState,omitempty"`
}

// AppStoreReviewAttachmentsResponse is the response for attachment lists.
type AppStoreReviewAttachmentsResponse = Response[AppStoreReviewAttachmentAttributes]

// AppStoreReviewAttachmentCreateAttributes describes create attributes.
type AppStoreReviewAttachmentCreateAttributes struct {
	FileSize int64  `json:"fileSize"`
	FileName string `json:"fileName"`
}

// AppStoreReviewAttachmentDeleteResult represents CLI output for deletions.
type AppStoreReviewAttachmentDeleteResult struct {
	ID      string `json:"id"`
	Deleted bool   `json:"deleted"`
}

// GetAppStoreReviewAttachmentsForReviewDetail lists attachments for a review detail.
func (c *Client) GetAppStoreReviewAttachmentsForReviewDetail(ctx context.Context, reviewDetailID string, opts ...AppStoreReviewAttachmentsOption) (*AppStoreReviewAttachmentsResponse, error) {
	query := &appStoreReviewAttachmentsQuery{}
	for _, opt := range opts {
		opt(query)
	}

	reviewDetailID = strings.TrimSpace(reviewDetailID)
	path := fmt.Sprintf("/v1/appStoreReviewDetails/%s/appStoreReviewAttachments", reviewDetailID)
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("app-store-review-attachments: %w", err)
		}
		if reviewDetailID != "" || buildAppStoreReviewAttachmentsQuery(query) != "" {
			return nil, fmt.Errorf("app-store-review-attachments: next URL cannot be combined with reviewDetailID or query options")
		}
		path = query.nextURL
	} else {
		if reviewDetailID == "" {
			return nil, fmt.Errorf("reviewDetailID is required")
		}
		if queryString := buildAppStoreReviewAttachmentsQuery(query); queryString != "" {
			path += "?" + queryString
		}
	}

	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response AppStoreReviewAttachmentsResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetAppStoreReviewAttachment retrieves an attachment by ID.
func (c *Client) GetAppStoreReviewAttachment(ctx context.Context, attachmentID string, opts ...AppStoreReviewAttachmentsOption) (*AppStoreReviewAttachmentResponse, error) {
	attachmentID = strings.TrimSpace(attachmentID)
	if attachmentID == "" {
		return nil, fmt.Errorf("attachmentID is required")
	}

	query := applyOptions(opts)
	return getJSON[AppStoreReviewAttachmentResponse](ctx, c, readRequest{
		Path:  fmt.Sprintf("/v1/appStoreReviewAttachments/%s", attachmentID),
		Query: buildAppStoreReviewAttachmentsQuery(query),
	})
}

// CreateAppStoreReviewAttachment creates a new attachment.
func (c *Client) CreateAppStoreReviewAttachment(ctx context.Context, reviewDetailID, fileName string, fileSize int64) (*AppStoreReviewAttachmentResponse, error) {
	reviewDetailID = strings.TrimSpace(reviewDetailID)
	fileName = strings.TrimSpace(fileName)
	if reviewDetailID == "" {
		return nil, fmt.Errorf("reviewDetailID is required")
	}
	if fileName == "" {
		return nil, fmt.Errorf("fileName is required")
	}
	if fileSize <= 0 {
		return nil, fmt.Errorf("fileSize is required")
	}

	payload := AppStoreReviewAttachmentCreateRequest{
		Data: AppStoreReviewAttachmentCreateData{
			Type: ResourceTypeAppStoreReviewAttachments,
			Attributes: AppStoreReviewAttachmentCreateAttributes{
				FileName: fileName,
				FileSize: fileSize,
			},
			Relationships: &AppStoreReviewAttachmentRelationships{
				AppStoreReviewDetail: &Relationship{
					Data: ResourceData{
						Type: ResourceTypeAppStoreReviewDetails,
						ID:   reviewDetailID,
					},
				},
			},
		},
	}
	return sendJSON[AppStoreReviewAttachmentResponse](ctx, c, writeRequest{Method: http.MethodPost, Path: "/v1/appStoreReviewAttachments", Payload: payload})
}

// UpdateAppStoreReviewAttachment updates an attachment by ID.
func (c *Client) UpdateAppStoreReviewAttachment(ctx context.Context, attachmentID string, attrs AppStoreReviewAttachmentUpdateAttributes) (*AppStoreReviewAttachmentResponse, error) {
	attachmentID = strings.TrimSpace(attachmentID)
	if attachmentID == "" {
		return nil, fmt.Errorf("attachmentID is required")
	}

	payload := AppStoreReviewAttachmentUpdateRequest{
		Data: AppStoreReviewAttachmentUpdateData{
			Type: ResourceTypeAppStoreReviewAttachments,
			ID:   attachmentID,
		},
	}
	if attrs.SourceFileChecksum != nil || attrs.Uploaded != nil {
		payload.Data.Attributes = &attrs
	}
	return sendJSON[AppStoreReviewAttachmentResponse](ctx, c, writeRequest{Method: http.MethodPatch, Path: fmt.Sprintf("/v1/appStoreReviewAttachments/%s", attachmentID), Payload: payload})
}
