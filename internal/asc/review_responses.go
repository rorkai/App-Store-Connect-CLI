package asc

import (
	"encoding/json"
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
