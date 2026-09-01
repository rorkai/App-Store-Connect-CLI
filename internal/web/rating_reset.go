package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

const ratingResetRequestResourceType = "resetRatingsRequests"

// RatingResetRequestAttributes describes an App Store overview-rating reset request.
type RatingResetRequestAttributes struct {
	ResetDate *string `json:"resetDate"`
}

// RatingResetRequest is the JSON:API resource returned for an overview-rating reset request.
type RatingResetRequest struct {
	Type       string                       `json:"type"`
	ID         string                       `json:"id"`
	Attributes RatingResetRequestAttributes `json:"attributes"`
}

// RatingResetRequestResponse is the JSON:API response for an overview-rating reset request.
type RatingResetRequestResponse struct {
	Data  RatingResetRequest `json:"data"`
	Links json.RawMessage    `json:"links,omitempty"`
	raw   json.RawMessage
}

// MarshalJSON preserves Apple's complete JSON:API response for JSON output,
// including fields that this experimental endpoint may add without notice.
func (response RatingResetRequestResponse) MarshalJSON() ([]byte, error) {
	if len(response.raw) > 0 {
		return append([]byte(nil), response.raw...), nil
	}
	type responseAlias RatingResetRequestResponse
	return json.Marshal(responseAlias(response))
}

func ratingResetRequestHeaders() http.Header {
	headers := make(http.Header)
	headers.Set("Accept", "application/json")
	headers.Set("Content-Type", "application/json")
	headers.Set("Origin", appStoreBaseURL)
	headers.Set("Referer", appStoreBaseURL+"/")
	headers.Set("X-Requested-With", "XMLHttpRequest")
	headers.Set("X-CSRF-ITC", "[asc-ui]")
	return headers
}

func (c *Client) doRatingResetRequest(ctx context.Context, method, path string, body any) ([]byte, error) {
	return c.doRequestBase(ctx, c.baseURL, method, path, body, ratingResetRequestHeaders())
}

func parseRatingResetResponse(data []byte) (*RatingResetRequestResponse, error) {
	var response RatingResetRequestResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse rating reset response: %w", err)
	}
	response.raw = append(json.RawMessage(nil), data...)
	return &response, nil
}

// GetAppStoreVersionRatingResetRequest returns the overview-rating reset scheduled for a version.
func (c *Client) GetAppStoreVersionRatingResetRequest(ctx context.Context, versionID string) (*RatingResetRequestResponse, error) {
	versionID = strings.TrimSpace(versionID)
	if versionID == "" {
		return nil, fmt.Errorf("version id is required")
	}

	path := "/appStoreVersions/" + url.PathEscape(versionID) + "/resetRatingsRequest"
	data, err := c.doRatingResetRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	return parseRatingResetResponse(data)
}

// CreateAppStoreVersionRatingResetRequest schedules an overview-rating reset when a version is released.
func (c *Client) CreateAppStoreVersionRatingResetRequest(ctx context.Context, versionID string) (*RatingResetRequestResponse, error) {
	versionID = strings.TrimSpace(versionID)
	if versionID == "" {
		return nil, fmt.Errorf("version id is required")
	}

	payload := map[string]any{
		"data": map[string]any{
			"type": ratingResetRequestResourceType,
			"relationships": map[string]any{
				"appStoreVersion": map[string]any{
					"data": map[string]string{
						"type": "appStoreVersions",
						"id":   versionID,
					},
				},
			},
		},
	}

	data, err := c.doRatingResetRequest(ctx, http.MethodPost, "/resetRatingsRequests", payload)
	if err != nil {
		return nil, err
	}
	return parseRatingResetResponse(data)
}

// DeleteAppStoreVersionRatingResetRequest cancels a scheduled overview-rating reset.
func (c *Client) DeleteAppStoreVersionRatingResetRequest(ctx context.Context, requestID string) error {
	requestID = strings.TrimSpace(requestID)
	if requestID == "" {
		return fmt.Errorf("rating reset request id is required")
	}

	path := "/resetRatingsRequests/" + url.PathEscape(requestID)
	_, err := c.doRatingResetRequest(ctx, http.MethodDelete, path, nil)
	return err
}
