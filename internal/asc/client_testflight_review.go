package asc

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// GetBetaAppReviewDetails retrieves beta app review details filtered by app.
func (c *Client) GetBetaAppReviewDetails(ctx context.Context, appID string, opts ...BetaAppReviewDetailsOption) (*BetaAppReviewDetailsResponse, error) {
	query := applyOptions(opts)
	return getJSON[BetaAppReviewDetailsResponse](ctx, c, readRequest{
		Path:        "/v1/betaAppReviewDetails",
		Query:       buildBetaAppReviewDetailsQuery(appID, query),
		Next:        query.nextURL,
		NextContext: "betaAppReviewDetails",
	})
}

// GetBetaAppReviewDetailApp retrieves the app for a beta app review detail.
func (c *Client) GetBetaAppReviewDetailApp(ctx context.Context, detailID string) (*AppResponse, error) {
	detailID = strings.TrimSpace(detailID)
	if detailID == "" {
		return nil, fmt.Errorf("detailID is required")
	}

	return getJSON[AppResponse](ctx, c, readRequest{Path: fmt.Sprintf("/v1/betaAppReviewDetails/%s/app", detailID)})
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
	query := applyOptions(opts)
	return getJSON[BetaAppReviewSubmissionsResponse](ctx, c, readRequest{
		Path:        "/v1/betaAppReviewSubmissions",
		Query:       buildBetaAppReviewSubmissionsQuery(query),
		Next:        query.nextURL,
		NextContext: "betaAppReviewSubmissions",
	})
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

	return sendJSON[BetaAppReviewSubmissionResponse](ctx, c, writeRequest{
		Method:  "POST",
		Path:    "/v1/betaAppReviewSubmissions",
		Payload: payload,
	})
}

// GetBetaAppReviewSubmission retrieves a beta app review submission by ID.
func (c *Client) GetBetaAppReviewSubmission(ctx context.Context, submissionID string) (*BetaAppReviewSubmissionResponse, error) {
	submissionID = strings.TrimSpace(submissionID)
	if submissionID == "" {
		return nil, fmt.Errorf("submissionID is required")
	}

	return getJSON[BetaAppReviewSubmissionResponse](ctx, c, readRequest{Path: fmt.Sprintf("/v1/betaAppReviewSubmissions/%s", submissionID)})
}

// GetBetaAppReviewSubmissionBuild retrieves the build for a beta app review submission.
func (c *Client) GetBetaAppReviewSubmissionBuild(ctx context.Context, submissionID string) (*BuildResponse, error) {
	submissionID = strings.TrimSpace(submissionID)
	if submissionID == "" {
		return nil, fmt.Errorf("submissionID is required")
	}

	return getJSON[BuildResponse](ctx, c, readRequest{Path: fmt.Sprintf("/v1/betaAppReviewSubmissions/%s/build", submissionID)})
}

// GetBuildBetaDetails retrieves build beta details.
func (c *Client) GetBuildBetaDetails(ctx context.Context, opts ...BuildBetaDetailsOption) (*BuildBetaDetailsResponse, error) {
	query := applyOptions(opts)
	return getJSON[BuildBetaDetailsResponse](ctx, c, readRequest{
		Path:        "/v1/buildBetaDetails",
		Query:       buildBuildBetaDetailsQuery(query),
		Next:        query.nextURL,
		NextContext: "buildBetaDetails",
	})
}

// GetBuildBetaDetailBuild retrieves the build for a build beta detail.
func (c *Client) GetBuildBetaDetailBuild(ctx context.Context, detailID string) (*BuildResponse, error) {
	detailID = strings.TrimSpace(detailID)
	if detailID == "" {
		return nil, fmt.Errorf("detailID is required")
	}

	return getJSON[BuildResponse](ctx, c, readRequest{Path: fmt.Sprintf("/v1/buildBetaDetails/%s/build", detailID)})
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

	return sendJSON[BuildBetaDetailResponse](ctx, c, writeRequest{
		Method:  "PATCH",
		Path:    fmt.Sprintf("/v1/buildBetaDetails/%s", detailID),
		Payload: payload,
	})
}

// GetBetaRecruitmentCriterionOptions retrieves beta recruitment criterion options.
func (c *Client) GetBetaRecruitmentCriterionOptions(ctx context.Context, opts ...BetaRecruitmentCriterionOptionsOption) (*BetaRecruitmentCriterionOptionsResponse, error) {
	query := applyOptions(opts)
	return getJSON[BetaRecruitmentCriterionOptionsResponse](ctx, c, readRequest{
		Path:        "/v1/betaRecruitmentCriterionOptions",
		Query:       buildBetaRecruitmentCriterionOptionsQuery(query),
		Next:        query.nextURL,
		NextContext: "betaRecruitmentCriterionOptions",
	})
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

	return sendJSON[BetaRecruitmentCriteriaResponse](ctx, c, writeRequest{
		Method:  "POST",
		Path:    "/v1/betaRecruitmentCriteria",
		Payload: payload,
	})
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

	return sendJSON[BetaRecruitmentCriteriaResponse](ctx, c, writeRequest{
		Method:  "PATCH",
		Path:    fmt.Sprintf("/v1/betaRecruitmentCriteria/%s", criteriaID),
		Payload: payload,
	})
}

// DeleteBetaRecruitmentCriteria deletes beta recruitment criteria by ID.
func (c *Client) DeleteBetaRecruitmentCriteria(ctx context.Context, criteriaID string) error {
	criteriaID = strings.TrimSpace(criteriaID)
	if criteriaID == "" {
		return fmt.Errorf("criteriaID is required")
	}
	return sendNoContent(ctx, c, "DELETE", fmt.Sprintf("/v1/betaRecruitmentCriteria/%s", criteriaID), nil)
}

// GetBetaGroupPublicLinkUsages retrieves public link usage metrics for a beta group.
func (c *Client) GetBetaGroupPublicLinkUsages(ctx context.Context, groupID string) (*BetaGroupPublicLinkUsagesResponse, error) {
	groupID = strings.TrimSpace(groupID)
	if groupID == "" {
		return nil, fmt.Errorf("groupID is required")
	}

	return getJSON[BetaGroupPublicLinkUsagesResponse](ctx, c, readRequest{Path: fmt.Sprintf("/v1/betaGroups/%s/metrics/publicLinkUsages", groupID)})
}

// GetBetaGroupTesterUsages retrieves beta tester usage metrics for a beta group.
func (c *Client) GetBetaGroupTesterUsages(ctx context.Context, groupID string) (*BetaGroupTesterUsagesResponse, error) {
	groupID = strings.TrimSpace(groupID)
	if groupID == "" {
		return nil, fmt.Errorf("groupID is required")
	}

	return getJSON[BetaGroupTesterUsagesResponse](ctx, c, readRequest{Path: fmt.Sprintf("/v1/betaGroups/%s/metrics/betaTesterUsages?groupBy=betaTesters", groupID)})
}
