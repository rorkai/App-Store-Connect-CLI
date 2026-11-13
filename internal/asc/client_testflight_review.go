package asc

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// GetBetaAppReviewDetails retrieves beta app review details filtered by app.
func (c *Client) GetBetaAppReviewDetails(ctx context.Context, appID string, opts ...BetaAppReviewDetailsOption) (*BetaAppReviewDetailsResponse, error) {
	query := &betaAppReviewDetailsQuery{}
	for _, opt := range opts {
		opt(query)
	}

	path := "/v1/betaAppReviewDetails"
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("betaAppReviewDetails: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildBetaAppReviewDetailsQuery(appID, query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response BetaAppReviewDetailsResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetBetaAppReviewDetail retrieves a beta app review detail by ID.
func (c *Client) GetBetaAppReviewDetail(ctx context.Context, detailID string) (*BetaAppReviewDetailResponse, error) {
	detailID = strings.TrimSpace(detailID)
	if detailID == "" {
		return nil, fmt.Errorf("detailID is required")
	}

	path := fmt.Sprintf("/v1/betaAppReviewDetails/%s", detailID)
	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response BetaAppReviewDetailResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetBetaAppReviewDetailApp retrieves the app for a beta app review detail.
func (c *Client) GetBetaAppReviewDetailApp(ctx context.Context, detailID string) (*AppResponse, error) {
	detailID = strings.TrimSpace(detailID)
	if detailID == "" {
		return nil, fmt.Errorf("detailID is required")
	}

	path := fmt.Sprintf("/v1/betaAppReviewDetails/%s/app", detailID)
	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response AppResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// UpdateBetaAppReviewDetail updates beta app review details by ID.
func (c *Client) UpdateBetaAppReviewDetail(ctx context.Context, detailID string, attrs BetaAppReviewDetailUpdateAttributes) (*BetaAppReviewDetailResponse, error) {
	detailID = strings.TrimSpace(detailID)
	if detailID == "" {
		return nil, fmt.Errorf("detailID is required")
	}
	if err := validateSecretMutationValue(attrs.DemoAccountPassword); err != nil {
		return nil, err
	}

	payload := BetaAppReviewDetailUpdateRequest{
		Data: BetaAppReviewDetailUpdateData{
			Type:       ResourceTypeBetaAppReviewDetails,
			ID:         detailID,
			Attributes: &attrs,
		},
	}

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	data, err := c.do(ctx, "PATCH", fmt.Sprintf("/v1/betaAppReviewDetails/%s", detailID), body)
	if err != nil {
		return nil, redactSubmittedSecretFromError(err, attrs.DemoAccountPassword)
	}

	var response BetaAppReviewDetailResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetBetaAppReviewSubmissions retrieves beta app review submissions.
func (c *Client) GetBetaAppReviewSubmissions(ctx context.Context, opts ...BetaAppReviewSubmissionsOption) (*BetaAppReviewSubmissionsResponse, error) {
	query := &betaAppReviewSubmissionsQuery{}
	for _, opt := range opts {
		opt(query)
	}

	path := "/v1/betaAppReviewSubmissions"
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("betaAppReviewSubmissions: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildBetaAppReviewSubmissionsQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response BetaAppReviewSubmissionsResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// CreateBetaAppReviewSubmission submits a build for beta app review.
func (c *Client) CreateBetaAppReviewSubmission(ctx context.Context, buildID string) (*BetaAppReviewSubmissionResponse, error) {
	buildID = strings.TrimSpace(buildID)
	if buildID == "" {
		return nil, fmt.Errorf("buildID is required")
	}

	payload := BetaAppReviewSubmissionCreateRequest{
		Data: BetaAppReviewSubmissionCreateData{
			Type: ResourceTypeBetaAppReviewSubmissions,
			Relationships: &BetaAppReviewSubmissionCreateRelationships{
				Build: &Relationship{
					Data: ResourceData{
						Type: ResourceTypeBuilds,
						ID:   buildID,
					},
				},
			},
		},
	}

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	data, err := c.do(ctx, "POST", "/v1/betaAppReviewSubmissions", body)
	if err != nil {
		return nil, err
	}

	var response BetaAppReviewSubmissionResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetBetaAppReviewSubmission retrieves a beta app review submission by ID.
func (c *Client) GetBetaAppReviewSubmission(ctx context.Context, submissionID string) (*BetaAppReviewSubmissionResponse, error) {
	submissionID = strings.TrimSpace(submissionID)
	if submissionID == "" {
		return nil, fmt.Errorf("submissionID is required")
	}

	path := fmt.Sprintf("/v1/betaAppReviewSubmissions/%s", submissionID)
	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response BetaAppReviewSubmissionResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetBetaAppReviewSubmissionBuild retrieves the build for a beta app review submission.
func (c *Client) GetBetaAppReviewSubmissionBuild(ctx context.Context, submissionID string) (*BuildResponse, error) {
	submissionID = strings.TrimSpace(submissionID)
	if submissionID == "" {
		return nil, fmt.Errorf("submissionID is required")
	}

	path := fmt.Sprintf("/v1/betaAppReviewSubmissions/%s/build", submissionID)
	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response BuildResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetBuildBetaDetails retrieves build beta details.
func (c *Client) GetBuildBetaDetails(ctx context.Context, opts ...BuildBetaDetailsOption) (*BuildBetaDetailsResponse, error) {
	query := &buildBetaDetailsQuery{}
	for _, opt := range opts {
		opt(query)
	}

	path := "/v1/buildBetaDetails"
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("buildBetaDetails: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildBuildBetaDetailsQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response BuildBetaDetailsResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetBuildBetaDetail retrieves a build beta detail by ID.
func (c *Client) GetBuildBetaDetail(ctx context.Context, detailID string) (*BuildBetaDetailResponse, error) {
	detailID = strings.TrimSpace(detailID)
	if detailID == "" {
		return nil, fmt.Errorf("detailID is required")
	}

	path := fmt.Sprintf("/v1/buildBetaDetails/%s", detailID)
	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response BuildBetaDetailResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetBuildBetaDetailBuild retrieves the build for a build beta detail.
func (c *Client) GetBuildBetaDetailBuild(ctx context.Context, detailID string) (*BuildResponse, error) {
	detailID = strings.TrimSpace(detailID)
	if detailID == "" {
		return nil, fmt.Errorf("detailID is required")
	}

	path := fmt.Sprintf("/v1/buildBetaDetails/%s/build", detailID)
	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response BuildResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// UpdateBuildBetaDetail updates build beta details by ID.
func (c *Client) UpdateBuildBetaDetail(ctx context.Context, detailID string, attrs BuildBetaDetailUpdateAttributes) (*BuildBetaDetailResponse, error) {
	detailID = strings.TrimSpace(detailID)
	if detailID == "" {
		return nil, fmt.Errorf("detailID is required")
	}

	payload := BuildBetaDetailUpdateRequest{
		Data: BuildBetaDetailUpdateData{
			Type:       ResourceTypeBuildBetaDetails,
			ID:         detailID,
			Attributes: &attrs,
		},
	}

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	data, err := c.do(ctx, "PATCH", fmt.Sprintf("/v1/buildBetaDetails/%s", detailID), body)
	if err != nil {
		return nil, err
	}

	var response BuildBetaDetailResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetBetaRecruitmentCriterionOptions retrieves beta recruitment criterion options.
func (c *Client) GetBetaRecruitmentCriterionOptions(ctx context.Context, opts ...BetaRecruitmentCriterionOptionsOption) (*BetaRecruitmentCriterionOptionsResponse, error) {
	query := &betaRecruitmentCriterionOptionsQuery{}
	for _, opt := range opts {
		opt(query)
	}

	path := "/v1/betaRecruitmentCriterionOptions"
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("betaRecruitmentCriterionOptions: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildBetaRecruitmentCriterionOptionsQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response BetaRecruitmentCriterionOptionsResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// CreateBetaRecruitmentCriteria creates beta recruitment criteria for a group.
func (c *Client) CreateBetaRecruitmentCriteria(ctx context.Context, groupID string, filters []DeviceFamilyOsVersionFilter) (*BetaRecruitmentCriteriaResponse, error) {
	groupID = strings.TrimSpace(groupID)
	if groupID == "" {
		return nil, fmt.Errorf("groupID is required")
	}
	if len(filters) == 0 {
		return nil, fmt.Errorf("filters are required")
	}

	payload := BetaRecruitmentCriteriaCreateRequest{
		Data: BetaRecruitmentCriteriaCreateData{
			Type: ResourceTypeBetaRecruitmentCriteria,
			Attributes: BetaRecruitmentCriteriaCreateAttributes{
				DeviceFamilyOsVersionFilters: filters,
			},
			Relationships: &BetaRecruitmentCriteriaRelationships{
				BetaGroup: &Relationship{
					Data: ResourceData{
						Type: ResourceTypeBetaGroups,
						ID:   groupID,
					},
				},
			},
		},
	}

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	data, err := c.do(ctx, "POST", "/v1/betaRecruitmentCriteria", body)
	if err != nil {
		return nil, err
	}

	var response BetaRecruitmentCriteriaResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// UpdateBetaRecruitmentCriteria updates beta recruitment criteria by ID.
func (c *Client) UpdateBetaRecruitmentCriteria(ctx context.Context, criteriaID string, filters []DeviceFamilyOsVersionFilter) (*BetaRecruitmentCriteriaResponse, error) {
	criteriaID = strings.TrimSpace(criteriaID)
	if criteriaID == "" {
		return nil, fmt.Errorf("criteriaID is required")
	}
	if len(filters) == 0 {
		return nil, fmt.Errorf("filters are required")
	}

	payload := BetaRecruitmentCriteriaUpdateRequest{
		Data: BetaRecruitmentCriteriaUpdateData{
			Type: ResourceTypeBetaRecruitmentCriteria,
			ID:   criteriaID,
			Attributes: &BetaRecruitmentCriteriaUpdateAttributes{
				DeviceFamilyOsVersionFilters: filters,
			},
		},
	}

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	data, err := c.do(ctx, "PATCH", fmt.Sprintf("/v1/betaRecruitmentCriteria/%s", criteriaID), body)
	if err != nil {
		return nil, err
	}

	var response BetaRecruitmentCriteriaResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// DeleteBetaRecruitmentCriteria deletes beta recruitment criteria by ID.
func (c *Client) DeleteBetaRecruitmentCriteria(ctx context.Context, criteriaID string) error {
	criteriaID = strings.TrimSpace(criteriaID)
	if criteriaID == "" {
		return fmt.Errorf("criteriaID is required")
	}
	path := fmt.Sprintf("/v1/betaRecruitmentCriteria/%s", criteriaID)
	_, err := c.do(ctx, "DELETE", path, nil)
	return err
}

// GetBetaGroupPublicLinkUsages retrieves public link usage metrics for a beta group.
func (c *Client) GetBetaGroupPublicLinkUsages(ctx context.Context, groupID string) (*BetaGroupPublicLinkUsagesResponse, error) {
	groupID = strings.TrimSpace(groupID)
	if groupID == "" {
		return nil, fmt.Errorf("groupID is required")
	}

	path := fmt.Sprintf("/v1/betaGroups/%s/metrics/publicLinkUsages", groupID)
	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response BetaGroupPublicLinkUsagesResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetBetaGroupTesterUsages retrieves beta tester usage metrics for a beta group.
func (c *Client) GetBetaGroupTesterUsages(ctx context.Context, groupID string) (*BetaGroupTesterUsagesResponse, error) {
	groupID = strings.TrimSpace(groupID)
	if groupID == "" {
		return nil, fmt.Errorf("groupID is required")
	}

	path := fmt.Sprintf("/v1/betaGroups/%s/metrics/betaTesterUsages?groupBy=betaTesters", groupID)
	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response BetaGroupTesterUsagesResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}
