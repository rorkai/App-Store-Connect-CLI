package asc

import (
	"context"
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
	Name         string      `json:"name,omitempty"`
	AppIcon      *ImageAsset `json:"appIcon,omitempty"`
	AppIconName  string      `json:"appIconName,omitempty"`
	PromotedDate string      `json:"promotedDate,omitempty"`
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
	query := applyOptions(opts)
	versionID = strings.TrimSpace(versionID)
	if query.nextURL == "" && versionID == "" {
		return nil, fmt.Errorf("versionID is required")
	}
	return getJSON[AppStoreVersionExperimentsResponse](ctx, c, readRequest{
		Path:        fmt.Sprintf("/v1/appStoreVersions/%s/appStoreVersionExperiments", versionID),
		Query:       buildAppStoreVersionExperimentsQuery(query),
		Next:        query.nextURL,
		NextContext: "appStoreVersionExperiments",
	})
}

// GetAppStoreVersionExperimentsV2 retrieves experiments for an app (v2).
func (c *Client) GetAppStoreVersionExperimentsV2(ctx context.Context, appID string, opts ...AppStoreVersionExperimentsV2Option) (*AppStoreVersionExperimentsV2Response, error) {
	query := applyOptions(opts)
	appID = strings.TrimSpace(appID)
	if query.nextURL == "" && appID == "" {
		return nil, fmt.Errorf("appID is required")
	}
	return getJSON[AppStoreVersionExperimentsV2Response](ctx, c, readRequest{
		Path:        fmt.Sprintf("/v1/apps/%s/appStoreVersionExperimentsV2", appID),
		Query:       buildAppStoreVersionExperimentsV2Query(query),
		Next:        query.nextURL,
		NextContext: "appStoreVersionExperimentsV2",
	})
}

// GetAppStoreVersionExperiment retrieves an experiment by ID (v1).
func (c *Client) GetAppStoreVersionExperiment(ctx context.Context, experimentID string) (*AppStoreVersionExperimentResponse, error) {
	experimentID = strings.TrimSpace(experimentID)
	if experimentID == "" {
		return nil, fmt.Errorf("experimentID is required")
	}
	return getJSON[AppStoreVersionExperimentResponse](ctx, c, readRequest{Path: fmt.Sprintf("/v1/appStoreVersionExperiments/%s", experimentID)})
}

// GetAppStoreVersionExperimentV2 retrieves an experiment by ID (v2).
func (c *Client) GetAppStoreVersionExperimentV2(ctx context.Context, experimentID string) (*AppStoreVersionExperimentV2Response, error) {
	experimentID = strings.TrimSpace(experimentID)
	if experimentID == "" {
		return nil, fmt.Errorf("experimentID is required")
	}
	return getJSON[AppStoreVersionExperimentV2Response](ctx, c, readRequest{Path: fmt.Sprintf("/v2/appStoreVersionExperiments/%s", experimentID)})
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
	return sendJSON[AppStoreVersionExperimentResponse](ctx, c, writeRequest{Method: "POST", Path: "/v1/appStoreVersionExperiments", Payload: payload})
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
	return sendJSON[AppStoreVersionExperimentV2Response](ctx, c, writeRequest{Method: "POST", Path: "/v2/appStoreVersionExperiments", Payload: payload})
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
	return sendJSON[AppStoreVersionExperimentResponse](ctx, c, writeRequest{Method: "PATCH", Path: fmt.Sprintf("/v1/appStoreVersionExperiments/%s", experimentID), Payload: payload})
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
	return sendJSON[AppStoreVersionExperimentV2Response](ctx, c, writeRequest{Method: "PATCH", Path: fmt.Sprintf("/v2/appStoreVersionExperiments/%s", experimentID), Payload: payload})
}

// DeleteAppStoreVersionExperiment deletes an experiment (v1).
func (c *Client) DeleteAppStoreVersionExperiment(ctx context.Context, experimentID string) error {
	experimentID = strings.TrimSpace(experimentID)
	if experimentID == "" {
		return fmt.Errorf("experimentID is required")
	}
	return sendNoContent(ctx, c, "DELETE", fmt.Sprintf("/v1/appStoreVersionExperiments/%s", experimentID), nil)
}

// DeleteAppStoreVersionExperimentV2 deletes an experiment (v2).
func (c *Client) DeleteAppStoreVersionExperimentV2(ctx context.Context, experimentID string) error {
	experimentID = strings.TrimSpace(experimentID)
	if experimentID == "" {
		return fmt.Errorf("experimentID is required")
	}
	return sendNoContent(ctx, c, "DELETE", fmt.Sprintf("/v2/appStoreVersionExperiments/%s", experimentID), nil)
}

// GetAppStoreVersionExperimentTreatments retrieves treatments for an experiment.
func (c *Client) GetAppStoreVersionExperimentTreatments(ctx context.Context, experimentID string, opts ...AppStoreVersionExperimentTreatmentsOption) (*AppStoreVersionExperimentTreatmentsResponse, error) {
	query := applyOptions(opts)
	experimentID = strings.TrimSpace(experimentID)
	if query.nextURL == "" && experimentID == "" {
		return nil, fmt.Errorf("experimentID is required")
	}
	return getJSON[AppStoreVersionExperimentTreatmentsResponse](ctx, c, readRequest{
		Path:        fmt.Sprintf("/v1/appStoreVersionExperiments/%s/appStoreVersionExperimentTreatments", experimentID),
		Query:       buildAppStoreVersionExperimentTreatmentsQuery(query),
		Next:        query.nextURL,
		NextContext: "appStoreVersionExperimentTreatments",
	})
}

// GetAppStoreVersionExperimentTreatmentsV2 retrieves treatments for a v2 experiment.
func (c *Client) GetAppStoreVersionExperimentTreatmentsV2(ctx context.Context, experimentID string, opts ...AppStoreVersionExperimentTreatmentsOption) (*AppStoreVersionExperimentTreatmentsResponse, error) {
	query := applyOptions(opts)
	experimentID = strings.TrimSpace(experimentID)
	if query.nextURL == "" && experimentID == "" {
		return nil, fmt.Errorf("experimentID is required")
	}
	return getJSON[AppStoreVersionExperimentTreatmentsResponse](ctx, c, readRequest{
		Path:        fmt.Sprintf("/v2/appStoreVersionExperiments/%s/appStoreVersionExperimentTreatments", experimentID),
		Query:       buildAppStoreVersionExperimentTreatmentsQuery(query),
		Next:        query.nextURL,
		NextContext: "appStoreVersionExperimentTreatments",
	})
}

// GetAppStoreVersionExperimentTreatment retrieves a treatment by ID.
func (c *Client) GetAppStoreVersionExperimentTreatment(ctx context.Context, treatmentID string) (*AppStoreVersionExperimentTreatmentResponse, error) {
	treatmentID = strings.TrimSpace(treatmentID)
	if treatmentID == "" {
		return nil, fmt.Errorf("treatmentID is required")
	}
	return getJSON[AppStoreVersionExperimentTreatmentResponse](ctx, c, readRequest{Path: fmt.Sprintf("/v1/appStoreVersionExperimentTreatments/%s", treatmentID)})
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
	return sendJSON[AppStoreVersionExperimentTreatmentResponse](ctx, c, writeRequest{Method: "POST", Path: "/v1/appStoreVersionExperimentTreatments", Payload: payload})
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
	return sendJSON[AppStoreVersionExperimentTreatmentResponse](ctx, c, writeRequest{Method: "PATCH", Path: fmt.Sprintf("/v1/appStoreVersionExperimentTreatments/%s", treatmentID), Payload: payload})
}

// DeleteAppStoreVersionExperimentTreatment deletes a treatment.
func (c *Client) DeleteAppStoreVersionExperimentTreatment(ctx context.Context, treatmentID string) error {
	treatmentID = strings.TrimSpace(treatmentID)
	if treatmentID == "" {
		return fmt.Errorf("treatmentID is required")
	}
	return sendNoContent(ctx, c, "DELETE", fmt.Sprintf("/v1/appStoreVersionExperimentTreatments/%s", treatmentID), nil)
}

// GetAppStoreVersionExperimentTreatmentLocalizations retrieves localizations for a treatment.
func (c *Client) GetAppStoreVersionExperimentTreatmentLocalizations(ctx context.Context, treatmentID string, opts ...AppStoreVersionExperimentTreatmentLocalizationsOption) (*AppStoreVersionExperimentTreatmentLocalizationsResponse, error) {
	query := applyOptions(opts)
	treatmentID = strings.TrimSpace(treatmentID)
	if query.nextURL == "" && treatmentID == "" {
		return nil, fmt.Errorf("treatmentID is required")
	}
	return getJSON[AppStoreVersionExperimentTreatmentLocalizationsResponse](ctx, c, readRequest{
		Path:        fmt.Sprintf("/v1/appStoreVersionExperimentTreatments/%s/appStoreVersionExperimentTreatmentLocalizations", treatmentID),
		Query:       buildAppStoreVersionExperimentTreatmentLocalizationsQuery(query),
		Next:        query.nextURL,
		NextContext: "appStoreVersionExperimentTreatmentLocalizations",
	})
}

// GetAppStoreVersionExperimentTreatmentLocalization retrieves a treatment localization by ID.
func (c *Client) GetAppStoreVersionExperimentTreatmentLocalization(ctx context.Context, localizationID string, opts ...LocalizationReadOption) (*AppStoreVersionExperimentTreatmentLocalizationResponse, error) {
	localizationID = strings.TrimSpace(localizationID)
	if localizationID == "" {
		return nil, fmt.Errorf("localizationID is required")
	}
	return getJSON[AppStoreVersionExperimentTreatmentLocalizationResponse](ctx, c, readRequest{Path: localizationReadPath(fmt.Sprintf("/v1/appStoreVersionExperimentTreatmentLocalizations/%s", localizationID), opts)})
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
	return sendJSON[AppStoreVersionExperimentTreatmentLocalizationResponse](ctx, c, writeRequest{Method: "POST", Path: "/v1/appStoreVersionExperimentTreatmentLocalizations", Payload: payload})
}

// DeleteAppStoreVersionExperimentTreatmentLocalization deletes a treatment localization.
func (c *Client) DeleteAppStoreVersionExperimentTreatmentLocalization(ctx context.Context, localizationID string) error {
	localizationID = strings.TrimSpace(localizationID)
	if localizationID == "" {
		return fmt.Errorf("localizationID is required")
	}
	return sendNoContent(ctx, c, "DELETE", fmt.Sprintf("/v1/appStoreVersionExperimentTreatmentLocalizations/%s", localizationID), nil)
}

// GetAppStoreVersionExperimentTreatmentLocalizationPreviewSets retrieves preview sets for a treatment localization.
func (c *Client) GetAppStoreVersionExperimentTreatmentLocalizationPreviewSets(ctx context.Context, localizationID string, opts ...AppStoreVersionExperimentTreatmentLocalizationPreviewSetsOption) (*AppPreviewSetsResponse, error) {
	query := applyOptions(opts)
	localizationID = strings.TrimSpace(localizationID)
	if query.nextURL == "" && localizationID == "" {
		return nil, fmt.Errorf("localizationID is required")
	}
	return getJSON[AppPreviewSetsResponse](ctx, c, readRequest{
		Path:        fmt.Sprintf("/v1/appStoreVersionExperimentTreatmentLocalizations/%s/appPreviewSets", localizationID),
		Query:       buildAppStoreVersionExperimentTreatmentLocalizationPreviewSetsQuery(query),
		Next:        query.nextURL,
		NextContext: "appPreviewSets",
	})
}

// GetAppStoreVersionExperimentTreatmentLocalizationScreenshotSets retrieves screenshot sets for a treatment localization.
func (c *Client) GetAppStoreVersionExperimentTreatmentLocalizationScreenshotSets(ctx context.Context, localizationID string, opts ...AppStoreVersionExperimentTreatmentLocalizationScreenshotSetsOption) (*AppScreenshotSetsResponse, error) {
	query := applyOptions(opts)
	localizationID = strings.TrimSpace(localizationID)
	if query.nextURL == "" && localizationID == "" {
		return nil, fmt.Errorf("localizationID is required")
	}
	return getJSON[AppScreenshotSetsResponse](ctx, c, readRequest{
		Path:        fmt.Sprintf("/v1/appStoreVersionExperimentTreatmentLocalizations/%s/appScreenshotSets", localizationID),
		Query:       buildAppStoreVersionExperimentTreatmentLocalizationScreenshotSetsQuery(query),
		Next:        query.nextURL,
		NextContext: "appScreenshotSets",
	})
}
