package asc

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// AppStoreVersionAttributes describes app store version metadata.
type AppStoreVersionAttributes struct {
	Platform            Platform `json:"platform,omitempty"`
	VersionString       string   `json:"versionString,omitempty"`
	AppStoreState       string   `json:"appStoreState,omitempty"`
	AppVersionState     string   `json:"appVersionState,omitempty"`
	Copyright           string   `json:"copyright,omitempty"`
	CreatedDate         string   `json:"createdDate,omitempty"`
	ReleaseType         string   `json:"releaseType,omitempty"`
	EarliestReleaseDate string   `json:"earliestReleaseDate,omitempty"`
}

// AppStoreVersionCreateAttributes describes app store version create payload attributes.
type AppStoreVersionCreateAttributes struct {
	Platform      Platform `json:"platform"`
	VersionString string   `json:"versionString"`
	Copyright     string   `json:"copyright,omitempty"`
	ReleaseType   string   `json:"releaseType,omitempty"`
}

// AppStoreVersionUpdateAttributes describes app store version update payload attributes.
type AppStoreVersionUpdateAttributes struct {
	Copyright           *string `json:"copyright,omitempty"`
	ReleaseType         *string `json:"releaseType,omitempty"`
	EarliestReleaseDate *string `json:"earliestReleaseDate,omitempty"`
	VersionString       *string `json:"versionString,omitempty"`
}

// AppStoreVersionUpdateData is the data portion of an app store version update request.
type AppStoreVersionUpdateData struct {
	Type       ResourceType                    `json:"type"`
	ID         string                          `json:"id"`
	Attributes AppStoreVersionUpdateAttributes `json:"attributes"`
}

// AppStoreVersionUpdateRequest is a request to update an app store version.
type AppStoreVersionUpdateRequest struct {
	Data AppStoreVersionUpdateData `json:"data"`
}

// AppStoreVersionCreateRelationships describes relationships for app store version create requests.
type AppStoreVersionCreateRelationships struct {
	App *Relationship `json:"app"`
}

// AppStoreVersionCreateData is the data portion of an app store version create request.
type AppStoreVersionCreateData struct {
	Type          ResourceType                        `json:"type"`
	Attributes    AppStoreVersionCreateAttributes     `json:"attributes"`
	Relationships *AppStoreVersionCreateRelationships `json:"relationships"`
}

// AppStoreVersionCreateRequest is a request to create an app store version.
type AppStoreVersionCreateRequest struct {
	Data AppStoreVersionCreateData `json:"data"`
}

// AppStoreVersionsResponse is the response from app store versions endpoints.
type AppStoreVersionsResponse = Response[AppStoreVersionAttributes]

// AppStoreVersionResponse is the response from app store version detail.
type AppStoreVersionResponse = SingleResponse[AppStoreVersionAttributes]

// PreReleaseVersionAttributes describes TestFlight pre-release version metadata.
type PreReleaseVersionAttributes struct {
	Version  string   `json:"version,omitempty"`
	Platform Platform `json:"platform,omitempty"`
}

// PreReleaseVersion represents a pre-release version resource.
type PreReleaseVersion struct {
	Type       ResourceType                `json:"type"`
	ID         string                      `json:"id"`
	Attributes PreReleaseVersionAttributes `json:"attributes"`
}

// PreReleaseVersionsResponse is the response from pre-release versions endpoints.
type PreReleaseVersionsResponse struct {
	Data  []PreReleaseVersion `json:"data"`
	Links Links               `json:"links"`
}

// PreReleaseVersionResponse is the response from pre-release version detail.
type PreReleaseVersionResponse struct {
	Data  PreReleaseVersion `json:"data"`
	Links Links             `json:"links"`
}

// AppStoreVersionSubmissionCreateData is the data portion of an app store version submission create request.
type AppStoreVersionSubmissionCreateData struct {
	Type          ResourceType                            `json:"type"`
	Relationships *AppStoreVersionSubmissionRelationships `json:"relationships"`
}

// AppStoreVersionSubmissionCreateRequest is a request to create an app store version submission.
type AppStoreVersionSubmissionCreateRequest struct {
	Data AppStoreVersionSubmissionCreateData `json:"data"`
}

// AppStoreVersionSubmissionRelationships describes the relationships for an app store version submission.
type AppStoreVersionSubmissionRelationships struct {
	AppStoreVersion *Relationship `json:"appStoreVersion"`
}

// AppStoreVersionSubmissionAttributes describes an app store version submission resource.
type AppStoreVersionSubmissionAttributes struct {
	CreatedDate *string `json:"createdDate,omitempty"`
}

// AppStoreVersionSubmissionResponse is the response from app store version submission endpoint.
type AppStoreVersionSubmissionResponse = SingleResourceResponse[AppStoreVersionSubmissionAttributes]

// AppStoreVersionSubmissionResource represents a submission with relationships.
type AppStoreVersionSubmissionResource struct {
	Type       ResourceType `json:"type"`
	ID         string       `json:"id"`
	Attributes struct {
		CreatedDate *string `json:"createdDate,omitempty"`
	} `json:"attributes"`
	Relationships struct {
		AppStoreVersion *Relationship `json:"appStoreVersion,omitempty"`
	} `json:"relationships"`
}

// AppStoreVersionSubmissionResourceResponse is a response containing a submission resource.
type AppStoreVersionSubmissionResourceResponse struct {
	Data AppStoreVersionSubmissionResource `json:"data"`
}

// AppStoreVersionBuildRelationshipUpdateRequest is a request to attach a build to a version.
type AppStoreVersionBuildRelationshipUpdateRequest struct {
	Data ResourceData `json:"data"`
}

// AppStoreVersionAppClipDefaultExperienceRelationshipUpdateRequest is a request to attach an App Clip default experience to a version.
type AppStoreVersionAppClipDefaultExperienceRelationshipUpdateRequest struct {
	Data ResourceData `json:"data"`
}

// GetAppStoreVersions retrieves app store versions for an app.
func (c *Client) GetAppStoreVersions(ctx context.Context, appID string, opts ...AppStoreVersionsOption) (*AppStoreVersionsResponse, error) {
	query := &appStoreVersionsQuery{}
	for _, opt := range opts {
		opt(query)
	}

	path := fmt.Sprintf("/v1/apps/%s/appStoreVersions", appID)
	if query.nextURL != "" {
		// Validate nextURL to prevent credential exfiltration
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("appStoreVersions: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildAppStoreVersionsQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response AppStoreVersionsResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetPreReleaseVersions retrieves TestFlight pre-release versions for an app.
func (c *Client) GetPreReleaseVersions(ctx context.Context, appID string, opts ...PreReleaseVersionsOption) (*PreReleaseVersionsResponse, error) {
	query := &preReleaseVersionsQuery{}
	for _, opt := range opts {
		opt(query)
	}

	appID = strings.TrimSpace(appID)
	path := "/v1/preReleaseVersions"
	if query.nextURL != "" {
		// Validate nextURL to prevent credential exfiltration
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("preReleaseVersions: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildPreReleaseVersionsQuery(appID, query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response PreReleaseVersionsResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetPreReleaseVersion retrieves a TestFlight pre-release version by ID.
func (c *Client) GetPreReleaseVersion(ctx context.Context, id string) (*PreReleaseVersionResponse, error) {
	id = strings.TrimSpace(id)
	path := fmt.Sprintf("/v1/preReleaseVersions/%s", id)
	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response PreReleaseVersionResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetAppStoreVersion retrieves an app store version by ID.
func (c *Client) GetAppStoreVersion(ctx context.Context, versionID string, opts ...AppStoreVersionOption) (*AppStoreVersionResponse, error) {
	query := &appStoreVersionQuery{}
	for _, opt := range opts {
		opt(query)
	}

	path := fmt.Sprintf("/v1/appStoreVersions/%s", versionID)
	if queryString := buildAppStoreVersionQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response AppStoreVersionResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// CreateAppStoreVersion creates a new app store version for an app.
func (c *Client) CreateAppStoreVersion(ctx context.Context, appID string, attrs AppStoreVersionCreateAttributes) (*AppStoreVersionResponse, error) {
	payload := AppStoreVersionCreateRequest{
		Data: AppStoreVersionCreateData{
			Type:       ResourceTypeAppStoreVersions,
			Attributes: attrs,
			Relationships: &AppStoreVersionCreateRelationships{
				App: &Relationship{
					Data: ResourceData{
						Type: ResourceTypeApps,
						ID:   strings.TrimSpace(appID),
					},
				},
			},
		},
	}

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	data, err := c.do(ctx, "POST", "/v1/appStoreVersions", body)
	if err != nil {
		return nil, err
	}

	var response AppStoreVersionResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// UpdateAppStoreVersion updates an existing app store version.
func (c *Client) UpdateAppStoreVersion(ctx context.Context, versionID string, attrs AppStoreVersionUpdateAttributes) (*AppStoreVersionResponse, error) {
	payload := AppStoreVersionUpdateRequest{
		Data: AppStoreVersionUpdateData{
			Type:       ResourceTypeAppStoreVersions,
			ID:         strings.TrimSpace(versionID),
			Attributes: attrs,
		},
	}

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/v1/appStoreVersions/%s", strings.TrimSpace(versionID))
	data, err := c.do(ctx, "PATCH", path, body)
	if err != nil {
		return nil, err
	}

	var response AppStoreVersionResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// DeleteAppStoreVersion deletes an app store version.
// Only versions in PREPARE_FOR_SUBMISSION state can be deleted.
func (c *Client) DeleteAppStoreVersion(ctx context.Context, versionID string) error {
	path := fmt.Sprintf("/v1/appStoreVersions/%s", strings.TrimSpace(versionID))
	_, err := c.do(ctx, "DELETE", path, nil)
	return err
}

// AttachBuildToVersion attaches a build to an app store version.
func (c *Client) AttachBuildToVersion(ctx context.Context, versionID, buildID string) error {
	request := AppStoreVersionBuildRelationshipUpdateRequest{
		Data: ResourceData{
			Type: ResourceTypeBuilds,
			ID:   buildID,
		},
	}
	body, err := BuildRequestBody(request)
	if err != nil {
		return err
	}

	path := fmt.Sprintf("/v1/appStoreVersions/%s/relationships/build", versionID)
	if _, err := c.do(ctx, "PATCH", path, body); err != nil {
		return err
	}
	return nil
}

// AttachAppClipDefaultExperienceToVersion attaches an App Clip default experience to an app store version.
func (c *Client) AttachAppClipDefaultExperienceToVersion(ctx context.Context, versionID, experienceID string) error {
	versionID = strings.TrimSpace(versionID)
	experienceID = strings.TrimSpace(experienceID)
	if versionID == "" {
		return fmt.Errorf("versionID is required")
	}
	if experienceID == "" {
		return fmt.Errorf("experienceID is required")
	}

	request := AppStoreVersionAppClipDefaultExperienceRelationshipUpdateRequest{
		Data: ResourceData{
			Type: ResourceTypeAppClipDefaultExperiences,
			ID:   experienceID,
		},
	}
	body, err := BuildRequestBody(request)
	if err != nil {
		return err
	}

	path := fmt.Sprintf("/v1/appStoreVersions/%s/relationships/appClipDefaultExperience", versionID)
	if _, err := c.do(ctx, "PATCH", path, body); err != nil {
		return err
	}
	return nil
}

// GetAppStoreVersionBuild retrieves the build attached to a version.
func (c *Client) GetAppStoreVersionBuild(ctx context.Context, versionID string) (*BuildResponse, error) {
	path := fmt.Sprintf("/v1/appStoreVersions/%s/build", versionID)
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

// GetAppStoreVersionSubmissionResource retrieves a submission by submission ID.
func (c *Client) GetAppStoreVersionSubmissionResource(ctx context.Context, submissionID string) (*AppStoreVersionSubmissionResourceResponse, error) {
	path := fmt.Sprintf("/v1/appStoreVersionSubmissions/%s", submissionID)
	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response AppStoreVersionSubmissionResourceResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetAppStoreVersionSubmissionForVersion retrieves a submission by version ID.
func (c *Client) GetAppStoreVersionSubmissionForVersion(ctx context.Context, versionID string) (*AppStoreVersionSubmissionResourceResponse, error) {
	path := fmt.Sprintf("/v1/appStoreVersions/%s/appStoreVersionSubmission", versionID)
	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response AppStoreVersionSubmissionResourceResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// CreateAppStoreVersionSubmission creates a new app store version submission.
func (c *Client) CreateAppStoreVersionSubmission(ctx context.Context, req AppStoreVersionSubmissionCreateRequest) (*AppStoreVersionSubmissionResponse, error) {
	body, err := BuildRequestBody(req)
	if err != nil {
		return nil, err
	}

	data, err := c.do(ctx, "POST", "/v1/appStoreVersionSubmissions", body)
	if err != nil {
		return nil, err
	}

	var response AppStoreVersionSubmissionResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetAppStoreVersionSubmission retrieves an app store version submission by ID.
func (c *Client) GetAppStoreVersionSubmission(ctx context.Context, id string) (*AppStoreVersionSubmissionResponse, error) {
	data, err := c.do(ctx, "GET", fmt.Sprintf("/v1/appStoreVersionSubmissions/%s", id), nil)
	if err != nil {
		return nil, err
	}

	var response AppStoreVersionSubmissionResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// DeleteAppStoreVersionSubmission deletes an app store version submission.
func (c *Client) DeleteAppStoreVersionSubmission(ctx context.Context, id string) error {
	_, err := c.do(ctx, "DELETE", fmt.Sprintf("/v1/appStoreVersionSubmissions/%s", id), nil)
	return err
}
