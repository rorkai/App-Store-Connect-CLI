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
