package asc

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// ReviewSubmissionState represents the state of a review submission.
type ReviewSubmissionState string

const (
	ReviewSubmissionStateReadyForReview   ReviewSubmissionState = "READY_FOR_REVIEW"
	ReviewSubmissionStateWaitingForReview ReviewSubmissionState = "WAITING_FOR_REVIEW"
	ReviewSubmissionStateInReview         ReviewSubmissionState = "IN_REVIEW"
	ReviewSubmissionStateUnresolvedIssues ReviewSubmissionState = "UNRESOLVED_ISSUES"
	ReviewSubmissionStateCanceling        ReviewSubmissionState = "CANCELING"
	ReviewSubmissionStateComplete         ReviewSubmissionState = "COMPLETE"
)

// ReviewSubmissionAttributes describes review submission attributes.
type ReviewSubmissionAttributes struct {
	Platform        Platform              `json:"platform,omitempty"`
	SubmissionState ReviewSubmissionState `json:"state,omitempty"`
	SubmittedDate   string                `json:"submittedDate,omitempty"`
}

// ReviewSubmissionRelationships describes review submission relationships.
type ReviewSubmissionRelationships struct {
	App                *Relationship     `json:"app,omitempty"`
	Items              *RelationshipList `json:"items,omitempty"`
	SubmittedByActor   *Relationship     `json:"submittedByActor,omitempty"`
	LastUpdatedByActor *Relationship     `json:"lastUpdatedByActor,omitempty"`
}

// ReviewSubmissionResource represents a review submission resource.
type ReviewSubmissionResource struct {
	Type          ResourceType                   `json:"type"`
	ID            string                         `json:"id"`
	Attributes    ReviewSubmissionAttributes     `json:"attributes,omitempty"`
	Relationships *ReviewSubmissionRelationships `json:"relationships,omitempty"`
}

// ReviewSubmissionsResponse is the response from review submissions list endpoints.
type ReviewSubmissionsResponse struct {
	Data     []ReviewSubmissionResource `json:"data"`
	Links    Links                      `json:"links,omitempty"`
	Included json.RawMessage            `json:"included,omitempty"`
}

// GetLinks returns the links field for pagination.
func (r *ReviewSubmissionsResponse) GetLinks() *Links {
	return &r.Links
}

// GetData returns the data field for aggregation.
func (r *ReviewSubmissionsResponse) GetData() interface{} {
	return r.Data
}

// ReviewSubmissionResponse is the response from review submission detail endpoints.
type ReviewSubmissionResponse struct {
	Data     ReviewSubmissionResource `json:"data"`
	Links    Links                    `json:"links,omitempty"`
	Included json.RawMessage          `json:"included,omitempty"`
}

// ReviewSubmissionItemsLinkagesResponse is the response from review submission item linkage endpoints.
type ReviewSubmissionItemsLinkagesResponse = LinkagesResponse

// ReviewSubmissionCreateAttributes describes attributes for creating a review submission.
type ReviewSubmissionCreateAttributes struct {
	Platform Platform `json:"platform"`
}

// ReviewSubmissionCreateRelationships describes relationships for create requests.
type ReviewSubmissionCreateRelationships struct {
	App *Relationship `json:"app"`
}

// ReviewSubmissionCreateData is the data portion of a review submission create request.
type ReviewSubmissionCreateData struct {
	Type          ResourceType                         `json:"type"`
	Attributes    ReviewSubmissionCreateAttributes     `json:"attributes"`
	Relationships *ReviewSubmissionCreateRelationships `json:"relationships"`
}

// ReviewSubmissionCreateRequest is a request to create a review submission.
type ReviewSubmissionCreateRequest struct {
	Data ReviewSubmissionCreateData `json:"data"`
}

// ReviewSubmissionUpdateAttributes describes attributes for updating a review submission.
type ReviewSubmissionUpdateAttributes struct {
	Submitted *bool `json:"submitted,omitempty"`
	Canceled  *bool `json:"canceled,omitempty"`
}

// ReviewSubmissionUpdateData is the data portion of a review submission update request.
type ReviewSubmissionUpdateData struct {
	Type       ResourceType                     `json:"type"`
	ID         string                           `json:"id"`
	Attributes ReviewSubmissionUpdateAttributes `json:"attributes"`
}

// ReviewSubmissionUpdateRequest is a request to update a review submission.
type ReviewSubmissionUpdateRequest struct {
	Data ReviewSubmissionUpdateData `json:"data"`
}

// GetReviewSubmissions retrieves review submissions for an app.
func (c *Client) GetReviewSubmissions(ctx context.Context, appID string, opts ...ReviewSubmissionsOption) (*ReviewSubmissionsResponse, error) {
	query := &reviewSubmissionsQuery{}
	for _, opt := range opts {
		opt(query)
	}

	var path string
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("reviewSubmissions: %w", err)
		}
		path = query.nextURL
	} else {
		appID = strings.TrimSpace(appID)
		if appID == "" {
			return nil, fmt.Errorf("appID is required")
		}
		path = fmt.Sprintf("/v1/apps/%s/reviewSubmissions", appID)
		if queryString := buildReviewSubmissionsQuery(query); queryString != "" {
			path += "?" + queryString
		}
	}

	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response ReviewSubmissionsResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse review submissions response: %w", err)
	}

	return &response, nil
}

// GetReviewSubmission retrieves a review submission by ID.
func (c *Client) GetReviewSubmission(ctx context.Context, submissionID string) (*ReviewSubmissionResponse, error) {
	submissionID = strings.TrimSpace(submissionID)
	if submissionID == "" {
		return nil, fmt.Errorf("submissionID is required")
	}

	path := fmt.Sprintf("/v1/reviewSubmissions/%s", submissionID)
	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response ReviewSubmissionResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse review submission response: %w", err)
	}

	return &response, nil
}

// GetReviewSubmissionItemsRelationships retrieves item linkages for a review submission.
func (c *Client) GetReviewSubmissionItemsRelationships(ctx context.Context, submissionID string, opts ...LinkagesOption) (*ReviewSubmissionItemsLinkagesResponse, error) {
	query := &linkagesQuery{}
	for _, opt := range opts {
		opt(query)
	}

	var path string
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("reviewSubmissionItemsRelationships: %w", err)
		}
		path = query.nextURL
	} else {
		submissionID = strings.TrimSpace(submissionID)
		if submissionID == "" {
			return nil, fmt.Errorf("submissionID is required")
		}
		path = fmt.Sprintf("/v1/reviewSubmissions/%s/relationships/items", submissionID)
		if queryString := buildLinkagesQuery(query); queryString != "" {
			path += "?" + queryString
		}
	}

	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response ReviewSubmissionItemsLinkagesResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse review submission items relationships response: %w", err)
	}

	return &response, nil
}

// CreateReviewSubmission creates a new review submission.
func (c *Client) CreateReviewSubmission(ctx context.Context, appID string, platform Platform) (*ReviewSubmissionResponse, error) {
	appID = strings.TrimSpace(appID)
	if appID == "" {
		return nil, fmt.Errorf("appID is required")
	}
	if strings.TrimSpace(string(platform)) == "" {
		return nil, fmt.Errorf("platform is required")
	}

	payload := ReviewSubmissionCreateRequest{
		Data: ReviewSubmissionCreateData{
			Type:       ResourceTypeReviewSubmissions,
			Attributes: ReviewSubmissionCreateAttributes{Platform: platform},
			Relationships: &ReviewSubmissionCreateRelationships{
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

	data, err := c.do(ctx, "POST", "/v1/reviewSubmissions", body)
	if err != nil {
		return nil, err
	}

	var response ReviewSubmissionResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse review submission response: %w", err)
	}

	return &response, nil
}

// UpdateReviewSubmission updates a review submission by ID.
func (c *Client) UpdateReviewSubmission(ctx context.Context, submissionID string, attrs ReviewSubmissionUpdateAttributes) (*ReviewSubmissionResponse, error) {
	submissionID = strings.TrimSpace(submissionID)
	if submissionID == "" {
		return nil, fmt.Errorf("submissionID is required")
	}

	payload := ReviewSubmissionUpdateRequest{
		Data: ReviewSubmissionUpdateData{
			Type:       ResourceTypeReviewSubmissions,
			ID:         submissionID,
			Attributes: attrs,
		},
	}

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	data, err := c.do(ctx, "PATCH", fmt.Sprintf("/v1/reviewSubmissions/%s", submissionID), body)
	if err != nil {
		return nil, err
	}

	var response ReviewSubmissionResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse review submission response: %w", err)
	}

	return &response, nil
}

// SubmitReviewSubmission submits a review submission by ID.
func (c *Client) SubmitReviewSubmission(ctx context.Context, submissionID string) (*ReviewSubmissionResponse, error) {
	submitted := true
	return c.UpdateReviewSubmission(ctx, submissionID, ReviewSubmissionUpdateAttributes{Submitted: &submitted})
}

// CancelReviewSubmission cancels a review submission by ID.
func (c *Client) CancelReviewSubmission(ctx context.Context, submissionID string) (*ReviewSubmissionResponse, error) {
	canceled := true
	return c.UpdateReviewSubmission(ctx, submissionID, ReviewSubmissionUpdateAttributes{Canceled: &canceled})
}
