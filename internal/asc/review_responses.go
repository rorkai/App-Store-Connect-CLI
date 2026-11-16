package asc

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// CustomerReviewResponseV1Attributes describes a customer review response.
type CustomerReviewResponseV1Attributes struct {
	ResponseBody string `json:"responseBody"`
	LastModified string `json:"lastModifiedDate,omitempty"`
	State        string `json:"state,omitempty"`
}

// CustomerReviewResponseResource is a customer review response resource.
type CustomerReviewResponseResource struct {
	Type       ResourceType                       `json:"type"`
	ID         string                             `json:"id"`
	Attributes CustomerReviewResponseV1Attributes `json:"attributes"`
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

// CustomerReviewResponseV1CreateAttributes describes attributes for creating a review response.
type CustomerReviewResponseV1CreateAttributes struct {
	ResponseBody string `json:"responseBody"`
}

// CustomerReviewResponseV1CreateData is the data portion of a review response create request.
type CustomerReviewResponseV1CreateData struct {
	Type          ResourceType                             `json:"type"`
	Attributes    CustomerReviewResponseV1CreateAttributes `json:"attributes"`
	Relationships *CustomerReviewResponseRelationships     `json:"relationships"`
}

// CustomerReviewResponseRelationships describes relationships for review response requests.
type CustomerReviewResponseRelationships struct {
	Review *Relationship `json:"review"`
}

// CustomerReviewResponseV1CreateRequest is a request to create a review response.
type CustomerReviewResponseV1CreateRequest struct {
	Data CustomerReviewResponseV1CreateData `json:"data"`
}

// ResourceTypeCustomerReviewResponses is the resource type for customer review responses.
const ResourceTypeCustomerReviewResponses ResourceType = "customerReviewResponses"

// ResourceTypeCustomerReviews is the resource type for customer reviews.
const ResourceTypeCustomerReviews ResourceType = "customerReviews"

// CustomerReviewPublishedResponseID extracts the response relationship from a customer review resource.
func CustomerReviewPublishedResponseID(review Resource[CustomerReviewAttributes]) (string, bool) {
	if len(review.Relationships) == 0 {
		return "", false
	}

	var relationships struct {
		Response *Relationship `json:"response"`
	}
	if err := json.Unmarshal(review.Relationships, &relationships); err != nil {
		return "", false
	}
	if relationships.Response == nil {
		return "", false
	}

	id := strings.TrimSpace(relationships.Response.Data.ID)
	if id == "" {
		return "", false
	}
	return id, true
}

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

	payload := CustomerReviewResponseV1CreateRequest{
		Data: CustomerReviewResponseV1CreateData{
			Type: ResourceTypeCustomerReviewResponses,
			Attributes: CustomerReviewResponseV1CreateAttributes{
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

	return sendJSON[CustomerReviewResponseResponse](ctx, c, writeRequest{Method: "POST", Path: "/v1/customerReviewResponses", Payload: payload})
}
