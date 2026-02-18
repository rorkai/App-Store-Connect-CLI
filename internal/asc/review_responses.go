package asc

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// CustomerReviewResponseAttributes describes a customer review response.
type CustomerReviewResponseAttributes struct {
	ResponseBody string `json:"responseBody"`
	LastModified string `json:"lastModifiedDate,omitempty"`
	State        string `json:"state,omitempty"`
}

// CustomerReviewResponseResource is a customer review response resource.
type CustomerReviewResponseResource struct {
	Type       ResourceType                     `json:"type"`
	ID         string                           `json:"id"`
	Attributes CustomerReviewResponseAttributes `json:"attributes"`
}

// CustomerReviewResponsesResponse is the response from customer review responses endpoints (list).
type CustomerReviewResponsesResponse struct {
	Data  []CustomerReviewResponseResource `json:"data"`
	Links Links                            `json:"links"`
}

// CustomerReviewResponseResponse is the response from customer review response detail/create/update.
type CustomerReviewResponseResponse struct {
	Data  CustomerReviewResponseResource `json:"data"`
	Links Links                          `json:"links"`
}

// CustomerReviewResponseLinkageResponse is the response for customer review response relationship.
type CustomerReviewResponseLinkageResponse struct {
	Data  ResourceData `json:"data"`
	Links Links        `json:"links"`
}

// CustomerReviewResponseCreateAttributes describes attributes for creating a review response.
type CustomerReviewResponseCreateAttributes struct {
	ResponseBody string `json:"responseBody"`
}

// CustomerReviewResponseCreateData is the data portion of a review response create request.
type CustomerReviewResponseCreateData struct {
	Type          ResourceType                           `json:"type"`
	Attributes    CustomerReviewResponseCreateAttributes `json:"attributes"`
	Relationships *CustomerReviewResponseRelationships   `json:"relationships"`
}

// CustomerReviewResponseRelationships describes relationships for review response requests.
type CustomerReviewResponseRelationships struct {
	Review *Relationship `json:"review"`
}

// CustomerReviewResponseCreateRequest is a request to create a review response.
type CustomerReviewResponseCreateRequest struct {
	Data CustomerReviewResponseCreateData `json:"data"`
}

// CustomerReviewResponseDeleteResult represents CLI output for review response deletions.
type CustomerReviewResponseDeleteResult struct {
	ID      string `json:"id"`
	Deleted bool   `json:"deleted"`
}

// ResourceTypeCustomerReviewResponses is the resource type for customer review responses.
const ResourceTypeCustomerReviewResponses ResourceType = "customerReviewResponses"

// ResourceTypeCustomerReviews is the resource type for customer reviews.
const ResourceTypeCustomerReviews ResourceType = "customerReviews"

// CreateCustomerReviewResponse creates a response to a customer review.
func (c *Client) CreateCustomerReviewResponse(ctx context.Context, reviewID, responseBody string) (*CustomerReviewResponseResponse, error) {
	reviewID = strings.TrimSpace(reviewID)
	responseBody = strings.TrimSpace(responseBody)

	if reviewID == "" {
		return nil, fmt.Errorf("reviewID is required")
	}
	if responseBody == "" {
		return nil, fmt.Errorf("responseBody is required")
	}

	payload := CustomerReviewResponseCreateRequest{
		Data: CustomerReviewResponseCreateData{
			Type: ResourceTypeCustomerReviewResponses,
			Attributes: CustomerReviewResponseCreateAttributes{
				ResponseBody: responseBody,
			},
			Relationships: &CustomerReviewResponseRelationships{
				Review: &Relationship{
					Data: ResourceData{
						Type: ResourceTypeCustomerReviews,
						ID:   reviewID,
					},
				},
			},
		},
	}

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	data, err := c.do(ctx, "POST", "/v1/customerReviewResponses", body)
	if err != nil {
		return nil, err
	}

	var response CustomerReviewResponseResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetCustomerReviewResponse retrieves a customer review response by ID.
func (c *Client) GetCustomerReviewResponse(ctx context.Context, responseID string) (*CustomerReviewResponseResponse, error) {
	responseID = strings.TrimSpace(responseID)
	if responseID == "" {
		return nil, fmt.Errorf("responseID is required")
	}

	path := fmt.Sprintf("/v1/customerReviewResponses/%s", responseID)
	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response CustomerReviewResponseResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// DeleteCustomerReviewResponse deletes a customer review response by ID.
func (c *Client) DeleteCustomerReviewResponse(ctx context.Context, responseID string) error {
	responseID = strings.TrimSpace(responseID)
	if responseID == "" {
		return fmt.Errorf("responseID is required")
	}

	path := fmt.Sprintf("/v1/customerReviewResponses/%s", responseID)
	_, err := c.do(ctx, "DELETE", path, nil)
	return err
}

// GetCustomerReviewResponseForReview retrieves the response for a specific review.
func (c *Client) GetCustomerReviewResponseForReview(ctx context.Context, reviewID string) (*CustomerReviewResponseResponse, error) {
	reviewID = strings.TrimSpace(reviewID)
	if reviewID == "" {
		return nil, fmt.Errorf("reviewID is required")
	}

	path := fmt.Sprintf("/v1/customerReviews/%s/response", reviewID)
	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response CustomerReviewResponseResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetCustomerReviewResponseRelationshipForReview retrieves the response linkage for a specific review.
func (c *Client) GetCustomerReviewResponseRelationshipForReview(ctx context.Context, reviewID string) (*CustomerReviewResponseLinkageResponse, error) {
	reviewID = strings.TrimSpace(reviewID)
	if reviewID == "" {
		return nil, fmt.Errorf("reviewID is required")
	}

	path := fmt.Sprintf("/v1/customerReviews/%s/relationships/response", reviewID)
	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response CustomerReviewResponseLinkageResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}
