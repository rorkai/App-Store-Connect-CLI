package asc

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// AppStoreVersionExperimentAttributes describes experiment attributes (v1).
type AppStoreVersionExperimentAttributes struct {
	Name              string `json:"name,omitempty"`
	TrafficProportion *int   `json:"trafficProportion,omitempty"`
	State             string `json:"state,omitempty"`
	ReviewRequired    *bool  `json:"reviewRequired,omitempty"`
	StartDate         string `json:"startDate,omitempty"`
	EndDate           string `json:"endDate,omitempty"`
}

// AppStoreVersionExperimentsResponse is the response from experiment list endpoints (v1).
type AppStoreVersionExperimentsResponse = Response[AppStoreVersionExperimentAttributes]

// AppStoreVersionExperimentResponse is the response from experiment endpoints (v1).
type AppStoreVersionExperimentResponse = SingleResponse[AppStoreVersionExperimentAttributes]

// AppStoreVersionExperimentCreateAttributes describes create payload attributes (v1).
type AppStoreVersionExperimentCreateAttributes struct {
	Name              string `json:"name"`
	TrafficProportion int    `json:"trafficProportion"`
}

// AppStoreVersionExperimentCreateRelationships describes create relationships (v1).
type AppStoreVersionExperimentCreateRelationships struct {
	AppStoreVersion *Relationship `json:"appStoreVersion"`
}

// AppStoreVersionExperimentCreateData is the data payload for create requests (v1).
type AppStoreVersionExperimentCreateData struct {
	Type          ResourceType                                  `json:"type"`
	Attributes    AppStoreVersionExperimentCreateAttributes     `json:"attributes"`
	Relationships *AppStoreVersionExperimentCreateRelationships `json:"relationships"`
}

// AppStoreVersionExperimentCreateRequest is a request to create an experiment (v1).
type AppStoreVersionExperimentCreateRequest struct {
	Data AppStoreVersionExperimentCreateData `json:"data"`
}

// AppStoreVersionExperimentUpdateAttributes describes update payload attributes (v1).
type AppStoreVersionExperimentUpdateAttributes struct {
	Name              *string `json:"name,omitempty"`
	TrafficProportion *int    `json:"trafficProportion,omitempty"`
	Started           *bool   `json:"started,omitempty"`
}

// AppStoreVersionExperimentUpdateData is the data payload for update requests (v1).
type AppStoreVersionExperimentUpdateData struct {
	Type       ResourceType                               `json:"type"`
	ID         string                                     `json:"id"`
	Attributes *AppStoreVersionExperimentUpdateAttributes `json:"attributes,omitempty"`
}

// AppStoreVersionExperimentUpdateRequest is a request to update an experiment (v1).
type AppStoreVersionExperimentUpdateRequest struct {
	Data AppStoreVersionExperimentUpdateData `json:"data"`
}

// AppStoreVersionExperimentDeleteResult represents CLI output for experiment deletions.
type AppStoreVersionExperimentDeleteResult struct {
	ID      string `json:"id"`
	Deleted bool   `json:"deleted"`
}

// AppStoreVersionExperimentV2Attributes describes experiment attributes (v2).
type AppStoreVersionExperimentV2Attributes struct {
	Name              string   `json:"name,omitempty"`
	Platform          Platform `json:"platform,omitempty"`
	TrafficProportion *int     `json:"trafficProportion,omitempty"`
	State             string   `json:"state,omitempty"`
	ReviewRequired    *bool    `json:"reviewRequired,omitempty"`
	StartDate         string   `json:"startDate,omitempty"`
	EndDate           string   `json:"endDate,omitempty"`
}

// AppStoreVersionExperimentsV2Response is the response from experiment list endpoints (v2).
type AppStoreVersionExperimentsV2Response = Response[AppStoreVersionExperimentV2Attributes]

// AppStoreVersionExperimentV2Response is the response from experiment endpoints (v2).
type AppStoreVersionExperimentV2Response = SingleResponse[AppStoreVersionExperimentV2Attributes]

// AppStoreVersionExperimentV2CreateAttributes describes create payload attributes (v2).
type AppStoreVersionExperimentV2CreateAttributes struct {
	Name              string   `json:"name"`
	Platform          Platform `json:"platform"`
	TrafficProportion int      `json:"trafficProportion"`
}

// AppStoreVersionExperimentV2CreateRelationships describes create relationships (v2).
type AppStoreVersionExperimentV2CreateRelationships struct {
	App *Relationship `json:"app"`
}

// AppStoreVersionExperimentV2CreateData is the data payload for create requests (v2).
type AppStoreVersionExperimentV2CreateData struct {
	Type          ResourceType                                    `json:"type"`
	Attributes    AppStoreVersionExperimentV2CreateAttributes     `json:"attributes"`
	Relationships *AppStoreVersionExperimentV2CreateRelationships `json:"relationships"`
}

// AppStoreVersionExperimentV2CreateRequest is a request to create an experiment (v2).
type AppStoreVersionExperimentV2CreateRequest struct {
	Data AppStoreVersionExperimentV2CreateData `json:"data"`
}

// AppStoreVersionExperimentV2UpdateAttributes describes update payload attributes (v2).
type AppStoreVersionExperimentV2UpdateAttributes struct {
	Name              *string `json:"name,omitempty"`
	TrafficProportion *int    `json:"trafficProportion,omitempty"`
	Started           *bool   `json:"started,omitempty"`
}

// AppStoreVersionExperimentV2UpdateData is the data payload for update requests (v2).
type AppStoreVersionExperimentV2UpdateData struct {
	Type       ResourceType                                 `json:"type"`
	ID         string                                       `json:"id"`
	Attributes *AppStoreVersionExperimentV2UpdateAttributes `json:"attributes,omitempty"`
}

// AppStoreVersionExperimentV2UpdateRequest is a request to update an experiment (v2).
type AppStoreVersionExperimentV2UpdateRequest struct {
	Data AppStoreVersionExperimentV2UpdateData `json:"data"`
}

// AppStoreVersionExperimentTreatmentAttributes describes treatment attributes.
type AppStoreVersionExperimentTreatmentAttributes struct {
	Name         string `json:"name,omitempty"`
	AppIconName  string `json:"appIconName,omitempty"`
	PromotedDate string `json:"promotedDate,omitempty"`
}

// AppStoreVersionExperimentTreatmentsResponse is the response from treatment list endpoints.
type AppStoreVersionExperimentTreatmentsResponse = Response[AppStoreVersionExperimentTreatmentAttributes]

// AppStoreVersionExperimentTreatmentResponse is the response from treatment endpoints.
type AppStoreVersionExperimentTreatmentResponse = SingleResponse[AppStoreVersionExperimentTreatmentAttributes]

// AppStoreVersionExperimentTreatmentCreateAttributes describes create payload attributes.
type AppStoreVersionExperimentTreatmentCreateAttributes struct {
	Name        string `json:"name"`
	AppIconName string `json:"appIconName,omitempty"`
}

// AppStoreVersionExperimentTreatmentCreateRelationships describes create relationships.
type AppStoreVersionExperimentTreatmentCreateRelationships struct {
	AppStoreVersionExperiment   *Relationship `json:"appStoreVersionExperiment,omitempty"`
	AppStoreVersionExperimentV2 *Relationship `json:"appStoreVersionExperimentV2,omitempty"`
}

// AppStoreVersionExperimentTreatmentCreateData is the data payload for create requests.
type AppStoreVersionExperimentTreatmentCreateData struct {
	Type          ResourceType                                           `json:"type"`
	Attributes    AppStoreVersionExperimentTreatmentCreateAttributes     `json:"attributes"`
	Relationships *AppStoreVersionExperimentTreatmentCreateRelationships `json:"relationships,omitempty"`
}

// AppStoreVersionExperimentTreatmentCreateRequest is a request to create a treatment.
type AppStoreVersionExperimentTreatmentCreateRequest struct {
	Data AppStoreVersionExperimentTreatmentCreateData `json:"data"`
}

// AppStoreVersionExperimentTreatmentUpdateAttributes describes update payload attributes.
type AppStoreVersionExperimentTreatmentUpdateAttributes struct {
	Name        *string `json:"name,omitempty"`
	AppIconName *string `json:"appIconName,omitempty"`
}

// AppStoreVersionExperimentTreatmentUpdateData is the data payload for update requests.
type AppStoreVersionExperimentTreatmentUpdateData struct {
	Type       ResourceType                                        `json:"type"`
	ID         string                                              `json:"id"`
	Attributes *AppStoreVersionExperimentTreatmentUpdateAttributes `json:"attributes,omitempty"`
}

// AppStoreVersionExperimentTreatmentUpdateRequest is a request to update a treatment.
type AppStoreVersionExperimentTreatmentUpdateRequest struct {
	Data AppStoreVersionExperimentTreatmentUpdateData `json:"data"`
}

// AppStoreVersionExperimentTreatmentDeleteResult represents CLI output for treatment deletions.
type AppStoreVersionExperimentTreatmentDeleteResult struct {
	ID      string `json:"id"`
	Deleted bool   `json:"deleted"`
}

// AppStoreVersionExperimentTreatmentLocalizationAttributes describes treatment localization attributes.
type AppStoreVersionExperimentTreatmentLocalizationAttributes struct {
	Locale string `json:"locale,omitempty"`
}

// AppStoreVersionExperimentTreatmentLocalizationsResponse is the response from treatment localization list endpoints.
type AppStoreVersionExperimentTreatmentLocalizationsResponse = Response[AppStoreVersionExperimentTreatmentLocalizationAttributes]

// AppStoreVersionExperimentTreatmentLocalizationResponse is the response from treatment localization endpoints.
type AppStoreVersionExperimentTreatmentLocalizationResponse = SingleResponse[AppStoreVersionExperimentTreatmentLocalizationAttributes]

// AppStoreVersionExperimentTreatmentLocalizationCreateAttributes describes create payload attributes.
type AppStoreVersionExperimentTreatmentLocalizationCreateAttributes struct {
	Locale string `json:"locale"`
}

// AppStoreVersionExperimentTreatmentLocalizationCreateRelationships describes create relationships.
type AppStoreVersionExperimentTreatmentLocalizationCreateRelationships struct {
	AppStoreVersionExperimentTreatment *Relationship `json:"appStoreVersionExperimentTreatment"`
}

// AppStoreVersionExperimentTreatmentLocalizationCreateData is the data payload for create requests.
type AppStoreVersionExperimentTreatmentLocalizationCreateData struct {
	Type          ResourceType                                                       `json:"type"`
	Attributes    AppStoreVersionExperimentTreatmentLocalizationCreateAttributes     `json:"attributes"`
	Relationships *AppStoreVersionExperimentTreatmentLocalizationCreateRelationships `json:"relationships"`
}

// AppStoreVersionExperimentTreatmentLocalizationCreateRequest is a request to create a treatment localization.
type AppStoreVersionExperimentTreatmentLocalizationCreateRequest struct {
	Data AppStoreVersionExperimentTreatmentLocalizationCreateData `json:"data"`
}

// AppStoreVersionExperimentTreatmentLocalizationDeleteResult represents CLI output for treatment localization deletions.
type AppStoreVersionExperimentTreatmentLocalizationDeleteResult struct {
	ID      string `json:"id"`
	Deleted bool   `json:"deleted"`
}

// GetAppStoreVersionExperiments retrieves experiments for an app store version (v1).
func (c *Client) GetAppStoreVersionExperiments(ctx context.Context, versionID string, opts ...AppStoreVersionExperimentsOption) (*AppStoreVersionExperimentsResponse, error) {
	query := &appStoreVersionExperimentsQuery{}
	for _, opt := range opts {
		opt(query)
	}

	versionID = strings.TrimSpace(versionID)
	if query.nextURL == "" && versionID == "" {
		return nil, fmt.Errorf("versionID is required")
	}
	path := fmt.Sprintf("/v1/appStoreVersions/%s/appStoreVersionExperiments", versionID)
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("appStoreVersionExperiments: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildAppStoreVersionExperimentsQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response AppStoreVersionExperimentsResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetAppStoreVersionExperimentsV2 retrieves experiments for an app (v2).
func (c *Client) GetAppStoreVersionExperimentsV2(ctx context.Context, appID string, opts ...AppStoreVersionExperimentsV2Option) (*AppStoreVersionExperimentsV2Response, error) {
	query := &appStoreVersionExperimentsV2Query{}
	for _, opt := range opts {
		opt(query)
	}

	appID = strings.TrimSpace(appID)
	if query.nextURL == "" && appID == "" {
		return nil, fmt.Errorf("appID is required")
	}
	path := fmt.Sprintf("/v1/apps/%s/appStoreVersionExperimentsV2", appID)
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("appStoreVersionExperimentsV2: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildAppStoreVersionExperimentsV2Query(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response AppStoreVersionExperimentsV2Response
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetAppStoreVersionExperiment retrieves an experiment by ID (v1).
func (c *Client) GetAppStoreVersionExperiment(ctx context.Context, experimentID string) (*AppStoreVersionExperimentResponse, error) {
	experimentID = strings.TrimSpace(experimentID)
	if experimentID == "" {
		return nil, fmt.Errorf("experimentID is required")
	}
	data, err := c.do(ctx, "GET", fmt.Sprintf("/v1/appStoreVersionExperiments/%s", experimentID), nil)
	if err != nil {
		return nil, err
	}

	var response AppStoreVersionExperimentResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetAppStoreVersionExperimentV2 retrieves an experiment by ID (v2).
func (c *Client) GetAppStoreVersionExperimentV2(ctx context.Context, experimentID string) (*AppStoreVersionExperimentV2Response, error) {
	experimentID = strings.TrimSpace(experimentID)
	if experimentID == "" {
		return nil, fmt.Errorf("experimentID is required")
	}
	data, err := c.do(ctx, "GET", fmt.Sprintf("/v2/appStoreVersionExperiments/%s", experimentID), nil)
	if err != nil {
		return nil, err
	}

	var response AppStoreVersionExperimentV2Response
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// CreateAppStoreVersionExperiment creates an experiment (v1).
func (c *Client) CreateAppStoreVersionExperiment(ctx context.Context, versionID, name string, trafficProportion int) (*AppStoreVersionExperimentResponse, error) {
	versionID = strings.TrimSpace(versionID)
	name = strings.TrimSpace(name)
	if versionID == "" {
		return nil, fmt.Errorf("versionID is required")
	}
	if name == "" {
		return nil, fmt.Errorf("name is required")
	}

	payload := AppStoreVersionExperimentCreateRequest{
		Data: AppStoreVersionExperimentCreateData{
			Type:       ResourceTypeAppStoreVersionExperiments,
			Attributes: AppStoreVersionExperimentCreateAttributes{Name: name, TrafficProportion: trafficProportion},
			Relationships: &AppStoreVersionExperimentCreateRelationships{
				AppStoreVersion: &Relationship{
					Data: ResourceData{
						Type: ResourceTypeAppStoreVersions,
						ID:   versionID,
					},
				},
			},
		},
	}

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	data, err := c.do(ctx, "POST", "/v1/appStoreVersionExperiments", body)
	if err != nil {
		return nil, err
	}

	var response AppStoreVersionExperimentResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// CreateAppStoreVersionExperimentV2 creates an experiment (v2).
func (c *Client) CreateAppStoreVersionExperimentV2(ctx context.Context, appID string, platform Platform, name string, trafficProportion int) (*AppStoreVersionExperimentV2Response, error) {
	appID = strings.TrimSpace(appID)
	name = strings.TrimSpace(name)
	if appID == "" {
		return nil, fmt.Errorf("appID is required")
	}
	if name == "" {
		return nil, fmt.Errorf("name is required")
	}

	payload := AppStoreVersionExperimentV2CreateRequest{
		Data: AppStoreVersionExperimentV2CreateData{
			Type:       ResourceTypeAppStoreVersionExperiments,
			Attributes: AppStoreVersionExperimentV2CreateAttributes{Name: name, Platform: platform, TrafficProportion: trafficProportion},
			Relationships: &AppStoreVersionExperimentV2CreateRelationships{
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

	data, err := c.do(ctx, "POST", "/v2/appStoreVersionExperiments", body)
	if err != nil {
		return nil, err
	}

	var response AppStoreVersionExperimentV2Response
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// UpdateAppStoreVersionExperiment updates an experiment (v1).
func (c *Client) UpdateAppStoreVersionExperiment(ctx context.Context, experimentID string, attrs AppStoreVersionExperimentUpdateAttributes) (*AppStoreVersionExperimentResponse, error) {
	experimentID = strings.TrimSpace(experimentID)
	if experimentID == "" {
		return nil, fmt.Errorf("experimentID is required")
	}

	payload := AppStoreVersionExperimentUpdateRequest{
		Data: AppStoreVersionExperimentUpdateData{
			Type: ResourceTypeAppStoreVersionExperiments,
			ID:   experimentID,
		},
	}
	if attrs.Name != nil || attrs.TrafficProportion != nil || attrs.Started != nil {
		payload.Data.Attributes = &attrs
	}

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	data, err := c.do(ctx, "PATCH", fmt.Sprintf("/v1/appStoreVersionExperiments/%s", experimentID), body)
	if err != nil {
		return nil, err
	}

	var response AppStoreVersionExperimentResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// UpdateAppStoreVersionExperimentV2 updates an experiment (v2).
func (c *Client) UpdateAppStoreVersionExperimentV2(ctx context.Context, experimentID string, attrs AppStoreVersionExperimentV2UpdateAttributes) (*AppStoreVersionExperimentV2Response, error) {
	experimentID = strings.TrimSpace(experimentID)
	if experimentID == "" {
		return nil, fmt.Errorf("experimentID is required")
	}

	payload := AppStoreVersionExperimentV2UpdateRequest{
		Data: AppStoreVersionExperimentV2UpdateData{
			Type: ResourceTypeAppStoreVersionExperiments,
			ID:   experimentID,
		},
	}
	if attrs.Name != nil || attrs.TrafficProportion != nil || attrs.Started != nil {
		payload.Data.Attributes = &attrs
	}

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	data, err := c.do(ctx, "PATCH", fmt.Sprintf("/v2/appStoreVersionExperiments/%s", experimentID), body)
	if err != nil {
		return nil, err
	}

	var response AppStoreVersionExperimentV2Response
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// DeleteAppStoreVersionExperiment deletes an experiment (v1).
func (c *Client) DeleteAppStoreVersionExperiment(ctx context.Context, experimentID string) error {
	experimentID = strings.TrimSpace(experimentID)
	if experimentID == "" {
		return fmt.Errorf("experimentID is required")
	}
	_, err := c.do(ctx, "DELETE", fmt.Sprintf("/v1/appStoreVersionExperiments/%s", experimentID), nil)
	return err
}

// DeleteAppStoreVersionExperimentV2 deletes an experiment (v2).
func (c *Client) DeleteAppStoreVersionExperimentV2(ctx context.Context, experimentID string) error {
	experimentID = strings.TrimSpace(experimentID)
	if experimentID == "" {
		return fmt.Errorf("experimentID is required")
	}
	_, err := c.do(ctx, "DELETE", fmt.Sprintf("/v2/appStoreVersionExperiments/%s", experimentID), nil)
	return err
}

// GetAppStoreVersionExperimentTreatments retrieves treatments for an experiment.
func (c *Client) GetAppStoreVersionExperimentTreatments(ctx context.Context, experimentID string, opts ...AppStoreVersionExperimentTreatmentsOption) (*AppStoreVersionExperimentTreatmentsResponse, error) {
	query := &appStoreVersionExperimentTreatmentsQuery{}
	for _, opt := range opts {
		opt(query)
	}

	experimentID = strings.TrimSpace(experimentID)
	if query.nextURL == "" && experimentID == "" {
		return nil, fmt.Errorf("experimentID is required")
	}
	path := fmt.Sprintf("/v1/appStoreVersionExperiments/%s/appStoreVersionExperimentTreatments", experimentID)
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("appStoreVersionExperimentTreatments: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildAppStoreVersionExperimentTreatmentsQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response AppStoreVersionExperimentTreatmentsResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetAppStoreVersionExperimentTreatmentsV2 retrieves treatments for a v2 experiment.
func (c *Client) GetAppStoreVersionExperimentTreatmentsV2(ctx context.Context, experimentID string, opts ...AppStoreVersionExperimentTreatmentsOption) (*AppStoreVersionExperimentTreatmentsResponse, error) {
	query := &appStoreVersionExperimentTreatmentsQuery{}
	for _, opt := range opts {
		opt(query)
	}

	experimentID = strings.TrimSpace(experimentID)
	if query.nextURL == "" && experimentID == "" {
		return nil, fmt.Errorf("experimentID is required")
	}
	path := fmt.Sprintf("/v2/appStoreVersionExperiments/%s/appStoreVersionExperimentTreatments", experimentID)
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("appStoreVersionExperimentTreatments: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildAppStoreVersionExperimentTreatmentsQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response AppStoreVersionExperimentTreatmentsResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetAppStoreVersionExperimentTreatmentsRelationships retrieves treatment linkages for an experiment (v1).
func (c *Client) GetAppStoreVersionExperimentTreatmentsRelationships(ctx context.Context, experimentID string, opts ...LinkagesOption) (*LinkagesResponse, error) {
	query := &linkagesQuery{}
	for _, opt := range opts {
		opt(query)
	}

	experimentID = strings.TrimSpace(experimentID)
	if query.nextURL == "" && experimentID == "" {
		return nil, fmt.Errorf("experimentID is required")
	}

	path := fmt.Sprintf("/v1/appStoreVersionExperiments/%s/relationships/appStoreVersionExperimentTreatments", experimentID)
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("appStoreVersionExperimentTreatmentsRelationships: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildLinkagesQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response LinkagesResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetAppStoreVersionExperimentTreatmentsV2Relationships retrieves treatment linkages for an experiment (v2).
func (c *Client) GetAppStoreVersionExperimentTreatmentsV2Relationships(ctx context.Context, experimentID string, opts ...LinkagesOption) (*LinkagesResponse, error) {
	query := &linkagesQuery{}
	for _, opt := range opts {
		opt(query)
	}

	experimentID = strings.TrimSpace(experimentID)
	if query.nextURL == "" && experimentID == "" {
		return nil, fmt.Errorf("experimentID is required")
	}

	path := fmt.Sprintf("/v2/appStoreVersionExperiments/%s/relationships/appStoreVersionExperimentTreatments", experimentID)
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("appStoreVersionExperimentTreatmentsV2Relationships: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildLinkagesQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response LinkagesResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetAppStoreVersionExperimentTreatment retrieves a treatment by ID.
func (c *Client) GetAppStoreVersionExperimentTreatment(ctx context.Context, treatmentID string) (*AppStoreVersionExperimentTreatmentResponse, error) {
	treatmentID = strings.TrimSpace(treatmentID)
	if treatmentID == "" {
		return nil, fmt.Errorf("treatmentID is required")
	}
	data, err := c.do(ctx, "GET", fmt.Sprintf("/v1/appStoreVersionExperimentTreatments/%s", treatmentID), nil)
	if err != nil {
		return nil, err
	}

	var response AppStoreVersionExperimentTreatmentResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// CreateAppStoreVersionExperimentTreatment creates a treatment.
func (c *Client) CreateAppStoreVersionExperimentTreatment(ctx context.Context, experimentID, name, appIconName string) (*AppStoreVersionExperimentTreatmentResponse, error) {
	return c.createAppStoreVersionExperimentTreatment(ctx, experimentID, name, appIconName, false)
}

// CreateAppStoreVersionExperimentTreatmentV2 creates a treatment for a v2 experiment.
func (c *Client) CreateAppStoreVersionExperimentTreatmentV2(ctx context.Context, experimentID, name, appIconName string) (*AppStoreVersionExperimentTreatmentResponse, error) {
	return c.createAppStoreVersionExperimentTreatment(ctx, experimentID, name, appIconName, true)
}

func (c *Client) createAppStoreVersionExperimentTreatment(ctx context.Context, experimentID, name, appIconName string, v2 bool) (*AppStoreVersionExperimentTreatmentResponse, error) {
	experimentID = strings.TrimSpace(experimentID)
	name = strings.TrimSpace(name)
	appIconName = strings.TrimSpace(appIconName)
	if experimentID == "" {
		return nil, fmt.Errorf("experimentID is required")
	}
	if name == "" {
		return nil, fmt.Errorf("name is required")
	}

	relationship := &Relationship{Data: ResourceData{
		Type: ResourceTypeAppStoreVersionExperiments,
		ID:   experimentID,
	}}
	relationships := &AppStoreVersionExperimentTreatmentCreateRelationships{}
	if v2 {
		relationships.AppStoreVersionExperimentV2 = relationship
	} else {
		relationships.AppStoreVersionExperiment = relationship
	}
	payload := AppStoreVersionExperimentTreatmentCreateRequest{
		Data: AppStoreVersionExperimentTreatmentCreateData{
			Type:          ResourceTypeAppStoreVersionExperimentTreatments,
			Attributes:    AppStoreVersionExperimentTreatmentCreateAttributes{Name: name, AppIconName: appIconName},
			Relationships: relationships,
		},
	}

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	data, err := c.do(ctx, "POST", "/v1/appStoreVersionExperimentTreatments", body)
	if err != nil {
		return nil, err
	}

	var response AppStoreVersionExperimentTreatmentResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// UpdateAppStoreVersionExperimentTreatment updates a treatment.
func (c *Client) UpdateAppStoreVersionExperimentTreatment(ctx context.Context, treatmentID string, attrs AppStoreVersionExperimentTreatmentUpdateAttributes) (*AppStoreVersionExperimentTreatmentResponse, error) {
	treatmentID = strings.TrimSpace(treatmentID)
	if treatmentID == "" {
		return nil, fmt.Errorf("treatmentID is required")
	}

	payload := AppStoreVersionExperimentTreatmentUpdateRequest{
		Data: AppStoreVersionExperimentTreatmentUpdateData{
			Type: ResourceTypeAppStoreVersionExperimentTreatments,
			ID:   treatmentID,
		},
	}
	if attrs.Name != nil || attrs.AppIconName != nil {
		payload.Data.Attributes = &attrs
	}

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	data, err := c.do(ctx, "PATCH", fmt.Sprintf("/v1/appStoreVersionExperimentTreatments/%s", treatmentID), body)
	if err != nil {
		return nil, err
	}

	var response AppStoreVersionExperimentTreatmentResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// DeleteAppStoreVersionExperimentTreatment deletes a treatment.
func (c *Client) DeleteAppStoreVersionExperimentTreatment(ctx context.Context, treatmentID string) error {
	treatmentID = strings.TrimSpace(treatmentID)
	if treatmentID == "" {
		return fmt.Errorf("treatmentID is required")
	}
	_, err := c.do(ctx, "DELETE", fmt.Sprintf("/v1/appStoreVersionExperimentTreatments/%s", treatmentID), nil)
	return err
}

// GetAppStoreVersionExperimentTreatmentLocalizations retrieves localizations for a treatment.
func (c *Client) GetAppStoreVersionExperimentTreatmentLocalizations(ctx context.Context, treatmentID string, opts ...AppStoreVersionExperimentTreatmentLocalizationsOption) (*AppStoreVersionExperimentTreatmentLocalizationsResponse, error) {
	query := &appStoreVersionExperimentTreatmentLocalizationsQuery{}
	for _, opt := range opts {
		opt(query)
	}

	treatmentID = strings.TrimSpace(treatmentID)
	if query.nextURL == "" && treatmentID == "" {
		return nil, fmt.Errorf("treatmentID is required")
	}
	path := fmt.Sprintf("/v1/appStoreVersionExperimentTreatments/%s/appStoreVersionExperimentTreatmentLocalizations", treatmentID)
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("appStoreVersionExperimentTreatmentLocalizations: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildAppStoreVersionExperimentTreatmentLocalizationsQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response AppStoreVersionExperimentTreatmentLocalizationsResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetAppStoreVersionExperimentTreatmentLocalizationsRelationships retrieves localization linkages for a treatment.
func (c *Client) GetAppStoreVersionExperimentTreatmentLocalizationsRelationships(ctx context.Context, treatmentID string, opts ...LinkagesOption) (*LinkagesResponse, error) {
	query := &linkagesQuery{}
	for _, opt := range opts {
		opt(query)
	}

	treatmentID = strings.TrimSpace(treatmentID)
	if query.nextURL == "" && treatmentID == "" {
		return nil, fmt.Errorf("treatmentID is required")
	}

	path := fmt.Sprintf("/v1/appStoreVersionExperimentTreatments/%s/relationships/appStoreVersionExperimentTreatmentLocalizations", treatmentID)
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("appStoreVersionExperimentTreatmentLocalizationsRelationships: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildLinkagesQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response LinkagesResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetAppStoreVersionExperimentTreatmentLocalization retrieves a treatment localization by ID.
func (c *Client) GetAppStoreVersionExperimentTreatmentLocalization(ctx context.Context, localizationID string, opts ...LocalizationReadOption) (*AppStoreVersionExperimentTreatmentLocalizationResponse, error) {
	localizationID = strings.TrimSpace(localizationID)
	if localizationID == "" {
		return nil, fmt.Errorf("localizationID is required")
	}
	data, err := c.do(ctx, "GET", localizationReadPath(fmt.Sprintf("/v1/appStoreVersionExperimentTreatmentLocalizations/%s", localizationID), opts), nil)
	if err != nil {
		return nil, err
	}

	var response AppStoreVersionExperimentTreatmentLocalizationResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// CreateAppStoreVersionExperimentTreatmentLocalization creates a treatment localization.
func (c *Client) CreateAppStoreVersionExperimentTreatmentLocalization(ctx context.Context, treatmentID, locale string) (*AppStoreVersionExperimentTreatmentLocalizationResponse, error) {
	treatmentID = strings.TrimSpace(treatmentID)
	locale = strings.TrimSpace(locale)
	if treatmentID == "" {
		return nil, fmt.Errorf("treatmentID is required")
	}
	if locale == "" {
		return nil, fmt.Errorf("locale is required")
	}

	payload := AppStoreVersionExperimentTreatmentLocalizationCreateRequest{
		Data: AppStoreVersionExperimentTreatmentLocalizationCreateData{
			Type:       ResourceTypeAppStoreVersionExperimentTreatmentLocalizations,
			Attributes: AppStoreVersionExperimentTreatmentLocalizationCreateAttributes{Locale: locale},
			Relationships: &AppStoreVersionExperimentTreatmentLocalizationCreateRelationships{
				AppStoreVersionExperimentTreatment: &Relationship{
					Data: ResourceData{
						Type: ResourceTypeAppStoreVersionExperimentTreatments,
						ID:   treatmentID,
					},
				},
			},
		},
	}

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	data, err := c.do(ctx, "POST", "/v1/appStoreVersionExperimentTreatmentLocalizations", body)
	if err != nil {
		return nil, err
	}

	var response AppStoreVersionExperimentTreatmentLocalizationResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// DeleteAppStoreVersionExperimentTreatmentLocalization deletes a treatment localization.
func (c *Client) DeleteAppStoreVersionExperimentTreatmentLocalization(ctx context.Context, localizationID string) error {
	localizationID = strings.TrimSpace(localizationID)
	if localizationID == "" {
		return fmt.Errorf("localizationID is required")
	}
	_, err := c.do(ctx, "DELETE", fmt.Sprintf("/v1/appStoreVersionExperimentTreatmentLocalizations/%s", localizationID), nil)
	return err
}

// GetAppStoreVersionExperimentTreatmentLocalizationPreviewSets retrieves preview sets for a treatment localization.
func (c *Client) GetAppStoreVersionExperimentTreatmentLocalizationPreviewSets(ctx context.Context, localizationID string, opts ...AppStoreVersionExperimentTreatmentLocalizationPreviewSetsOption) (*AppPreviewSetsResponse, error) {
	query := &appStoreVersionExperimentTreatmentLocalizationPreviewSetsQuery{}
	for _, opt := range opts {
		opt(query)
	}

	localizationID = strings.TrimSpace(localizationID)
	if query.nextURL == "" && localizationID == "" {
		return nil, fmt.Errorf("localizationID is required")
	}
	path := fmt.Sprintf("/v1/appStoreVersionExperimentTreatmentLocalizations/%s/appPreviewSets", localizationID)
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("appPreviewSets: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildAppStoreVersionExperimentTreatmentLocalizationPreviewSetsQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response AppPreviewSetsResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetAppStoreVersionExperimentTreatmentLocalizationPreviewSetsRelationships retrieves preview set linkages for a treatment localization.
func (c *Client) GetAppStoreVersionExperimentTreatmentLocalizationPreviewSetsRelationships(ctx context.Context, localizationID string, opts ...LinkagesOption) (*LinkagesResponse, error) {
	query := &linkagesQuery{}
	for _, opt := range opts {
		opt(query)
	}

	localizationID = strings.TrimSpace(localizationID)
	if query.nextURL == "" && localizationID == "" {
		return nil, fmt.Errorf("localizationID is required")
	}

	path := fmt.Sprintf("/v1/appStoreVersionExperimentTreatmentLocalizations/%s/relationships/appPreviewSets", localizationID)
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("appPreviewSetsRelationships: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildLinkagesQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response LinkagesResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetAppStoreVersionExperimentTreatmentLocalizationScreenshotSets retrieves screenshot sets for a treatment localization.
func (c *Client) GetAppStoreVersionExperimentTreatmentLocalizationScreenshotSets(ctx context.Context, localizationID string, opts ...AppStoreVersionExperimentTreatmentLocalizationScreenshotSetsOption) (*AppScreenshotSetsResponse, error) {
	query := &appStoreVersionExperimentTreatmentLocalizationScreenshotSetsQuery{}
	for _, opt := range opts {
		opt(query)
	}

	localizationID = strings.TrimSpace(localizationID)
	if query.nextURL == "" && localizationID == "" {
		return nil, fmt.Errorf("localizationID is required")
	}
	path := fmt.Sprintf("/v1/appStoreVersionExperimentTreatmentLocalizations/%s/appScreenshotSets", localizationID)
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("appScreenshotSets: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildAppStoreVersionExperimentTreatmentLocalizationScreenshotSetsQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response AppScreenshotSetsResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetAppStoreVersionExperimentTreatmentLocalizationScreenshotSetsRelationships retrieves screenshot set linkages for a treatment localization.
func (c *Client) GetAppStoreVersionExperimentTreatmentLocalizationScreenshotSetsRelationships(ctx context.Context, localizationID string, opts ...LinkagesOption) (*LinkagesResponse, error) {
	query := &linkagesQuery{}
	for _, opt := range opts {
		opt(query)
	}

	localizationID = strings.TrimSpace(localizationID)
	if query.nextURL == "" && localizationID == "" {
		return nil, fmt.Errorf("localizationID is required")
	}

	path := fmt.Sprintf("/v1/appStoreVersionExperimentTreatmentLocalizations/%s/relationships/appScreenshotSets", localizationID)
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("appScreenshotSetsRelationships: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildLinkagesQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response LinkagesResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}
