package asc

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// ReviewSubmissionItemType describes supported review submission item types.
type ReviewSubmissionItemType string

const (
	ReviewSubmissionItemTypeAppStoreVersion                    ReviewSubmissionItemType = "appStoreVersions"
	ReviewSubmissionItemTypeAppCustomProductPage               ReviewSubmissionItemType = "appCustomProductPages"
	ReviewSubmissionItemTypeAppEvent                           ReviewSubmissionItemType = "appEvents"
	ReviewSubmissionItemTypeAppStoreVersionExperiment          ReviewSubmissionItemType = "appStoreVersionExperiments"
	ReviewSubmissionItemTypeAppStoreVersionExperimentTreatment ReviewSubmissionItemType = "appStoreVersionExperimentTreatments"
)

// ReviewSubmissionItemAttributes describes review submission item attributes.
type ReviewSubmissionItemAttributes struct {
	State string `json:"state,omitempty"`
}

// ReviewSubmissionItemRelationships describes review submission item relationships.
type ReviewSubmissionItemRelationships struct {
	ReviewSubmission                   *Relationship `json:"reviewSubmission,omitempty"`
	AppStoreVersion                    *Relationship `json:"appStoreVersion,omitempty"`
	AppCustomProductPage               *Relationship `json:"appCustomProductPage,omitempty"`
	AppEvent                           *Relationship `json:"appEvent,omitempty"`
	AppStoreVersionExperiment          *Relationship `json:"appStoreVersionExperiment,omitempty"`
	AppStoreVersionExperimentTreatment *Relationship `json:"appStoreVersionExperimentTreatment,omitempty"`
}

// ReviewSubmissionItemResource represents a review submission item resource.
type ReviewSubmissionItemResource struct {
	Type          ResourceType                       `json:"type"`
	ID            string                             `json:"id"`
	Attributes    ReviewSubmissionItemAttributes     `json:"attributes,omitempty"`
	Relationships *ReviewSubmissionItemRelationships `json:"relationships,omitempty"`
}

// ReviewSubmissionItemsResponse is the response from review submission items list endpoints.
type ReviewSubmissionItemsResponse struct {
	Data  []ReviewSubmissionItemResource `json:"data"`
	Links Links                          `json:"links,omitempty"`
}

// GetLinks returns the links field for pagination.
func (r *ReviewSubmissionItemsResponse) GetLinks() *Links {
	return &r.Links
}

// GetData returns the data field for aggregation.
func (r *ReviewSubmissionItemsResponse) GetData() interface{} {
	return r.Data
}

// ReviewSubmissionItemResponse is the response from review submission item detail endpoints.
type ReviewSubmissionItemResponse struct {
	Data  ReviewSubmissionItemResource `json:"data"`
	Links Links                        `json:"links,omitempty"`
}

// ReviewSubmissionItemCreateRelationships describes relationships for create requests.
type ReviewSubmissionItemCreateRelationships struct {
	ReviewSubmission                   *Relationship `json:"reviewSubmission"`
	AppStoreVersion                    *Relationship `json:"appStoreVersion,omitempty"`
	AppCustomProductPage               *Relationship `json:"appCustomProductPage,omitempty"`
	AppEvent                           *Relationship `json:"appEvent,omitempty"`
	AppStoreVersionExperiment          *Relationship `json:"appStoreVersionExperiment,omitempty"`
	AppStoreVersionExperimentTreatment *Relationship `json:"appStoreVersionExperimentTreatment,omitempty"`
}

// ReviewSubmissionItemCreateData is the data portion of a review submission item create request.
type ReviewSubmissionItemCreateData struct {
	Type          ResourceType                            `json:"type"`
	Relationships ReviewSubmissionItemCreateRelationships `json:"relationships"`
}

// ReviewSubmissionItemCreateRequest is a request to create a review submission item.
type ReviewSubmissionItemCreateRequest struct {
	Data ReviewSubmissionItemCreateData `json:"data"`
}

// ReviewSubmissionItemUpdateAttributes describes attributes for updating a review submission item.
type ReviewSubmissionItemUpdateAttributes struct {
	State    *string `json:"state,omitempty"`
	Resolved *bool   `json:"resolved,omitempty"`
	Removed  *bool   `json:"removed,omitempty"`
}

// ReviewSubmissionItemUpdateData is the data portion of a review submission item update request.
type ReviewSubmissionItemUpdateData struct {
	Type       ResourceType                        `json:"type"`
	ID         string                              `json:"id"`
	Attributes ReviewSubmissionItemUpdateAttributes `json:"attributes"`
}

// ReviewSubmissionItemUpdateRequest is a request to update a review submission item.
type ReviewSubmissionItemUpdateRequest struct {
	Data ReviewSubmissionItemUpdateData `json:"data"`
}

// ReviewSubmissionItemDeleteResult represents CLI output for review submission item deletions.
type ReviewSubmissionItemDeleteResult struct {
	ID      string `json:"id"`
	Deleted bool   `json:"deleted"`
}

// GetReviewSubmissionItems retrieves items for a review submission.
func (c *Client) GetReviewSubmissionItems(ctx context.Context, submissionID string, opts ...ReviewSubmissionItemsOption) (*ReviewSubmissionItemsResponse, error) {
	query := &reviewSubmissionItemsQuery{}
	for _, opt := range opts {
		opt(query)
	}

	var path string
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("reviewSubmissionItems: %w", err)
		}
		path = query.nextURL
	} else {
		submissionID = strings.TrimSpace(submissionID)
		if submissionID == "" {
			return nil, fmt.Errorf("submissionID is required")
		}
		path = fmt.Sprintf("/v1/reviewSubmissions/%s/items", submissionID)
		if queryString := buildReviewSubmissionItemsQuery(query); queryString != "" {
			path += "?" + queryString
		}
	}

	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response ReviewSubmissionItemsResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse review submission items response: %w", err)
	}

	return &response, nil
}

// GetReviewSubmissionItem retrieves a review submission item by ID.
func (c *Client) GetReviewSubmissionItem(ctx context.Context, itemID string) (*ReviewSubmissionItemResponse, error) {
	itemID = strings.TrimSpace(itemID)
	if itemID == "" {
		return nil, fmt.Errorf("itemID is required")
	}

	path := fmt.Sprintf("/v1/reviewSubmissionItems/%s", itemID)
	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response ReviewSubmissionItemResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse review submission item response: %w", err)
	}

	return &response, nil
}

// CreateReviewSubmissionItem creates a review submission item.
func (c *Client) CreateReviewSubmissionItem(ctx context.Context, submissionID string, itemType ReviewSubmissionItemType, itemID string) (*ReviewSubmissionItemResponse, error) {
	submissionID = strings.TrimSpace(submissionID)
	itemID = strings.TrimSpace(itemID)
	if submissionID == "" {
		return nil, fmt.Errorf("submissionID is required")
	}
	if strings.TrimSpace(string(itemType)) == "" {
		return nil, fmt.Errorf("itemType is required")
	}
	if itemID == "" {
		return nil, fmt.Errorf("itemID is required")
	}

	relationships := ReviewSubmissionItemCreateRelationships{
		ReviewSubmission: &Relationship{
			Data: ResourceData{
				Type: ResourceTypeReviewSubmissions,
				ID:   submissionID,
			},
		},
	}

	switch itemType {
	case ReviewSubmissionItemTypeAppStoreVersion:
		relationships.AppStoreVersion = &Relationship{
			Data: ResourceData{Type: ResourceTypeAppStoreVersions, ID: itemID},
		}
	case ReviewSubmissionItemTypeAppCustomProductPage:
		relationships.AppCustomProductPage = &Relationship{
			Data: ResourceData{Type: ResourceTypeAppCustomProductPages, ID: itemID},
		}
	case ReviewSubmissionItemTypeAppEvent:
		relationships.AppEvent = &Relationship{
			Data: ResourceData{Type: ResourceTypeAppEvents, ID: itemID},
		}
	case ReviewSubmissionItemTypeAppStoreVersionExperiment:
		relationships.AppStoreVersionExperiment = &Relationship{
			Data: ResourceData{Type: ResourceTypeAppStoreVersionExperiments, ID: itemID},
		}
	case ReviewSubmissionItemTypeAppStoreVersionExperimentTreatment:
		relationships.AppStoreVersionExperimentTreatment = &Relationship{
			Data: ResourceData{Type: ResourceTypeAppStoreVersionExperimentTreatments, ID: itemID},
		}
	default:
		return nil, fmt.Errorf("unsupported itemType: %s", itemType)
	}

	payload := ReviewSubmissionItemCreateRequest{
		Data: ReviewSubmissionItemCreateData{
			Type:          ResourceTypeReviewSubmissionItems,
			Relationships: relationships,
		},
	}

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	data, err := c.do(ctx, "POST", "/v1/reviewSubmissionItems", body)
	if err != nil {
		return nil, err
	}

	var response ReviewSubmissionItemResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse review submission item response: %w", err)
	}

	return &response, nil
}

// UpdateReviewSubmissionItem updates a review submission item by ID.
func (c *Client) UpdateReviewSubmissionItem(ctx context.Context, itemID string, attrs ReviewSubmissionItemUpdateAttributes) (*ReviewSubmissionItemResponse, error) {
	itemID = strings.TrimSpace(itemID)
	if itemID == "" {
		return nil, fmt.Errorf("itemID is required")
	}

	payload := ReviewSubmissionItemUpdateRequest{
		Data: ReviewSubmissionItemUpdateData{
			Type:       ResourceTypeReviewSubmissionItems,
			ID:         itemID,
			Attributes: attrs,
		},
	}

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	data, err := c.do(ctx, "PATCH", fmt.Sprintf("/v1/reviewSubmissionItems/%s", itemID), body)
	if err != nil {
		return nil, err
	}

	var response ReviewSubmissionItemResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse review submission item response: %w", err)
	}

	return &response, nil
}

// DeleteReviewSubmissionItem deletes a review submission item by ID.
func (c *Client) DeleteReviewSubmissionItem(ctx context.Context, itemID string) error {
	itemID = strings.TrimSpace(itemID)
	if itemID == "" {
		return fmt.Errorf("itemID is required")
	}

	path := fmt.Sprintf("/v1/reviewSubmissionItems/%s", itemID)
	_, err := c.do(ctx, "DELETE", path, nil)
	return err
}

// AddReviewSubmissionItem adds an app store version to a review submission.
// This is a convenience wrapper around CreateReviewSubmissionItem for adding app store versions.
func (c *Client) AddReviewSubmissionItem(ctx context.Context, submissionID, versionID string) (*ReviewSubmissionItemResponse, error) {
	return c.CreateReviewSubmissionItem(ctx, submissionID, ReviewSubmissionItemTypeAppStoreVersion, versionID)
}
