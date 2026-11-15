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

// AppStoreVersionExperimentTreatmentsResponse is the response from treatment list endpoints.
type AppStoreVersionExperimentTreatmentsResponse = Response[AppStoreVersionExperimentTreatmentAttributes]

// AppStoreVersionExperimentTreatmentCreateAttributes describes create payload attributes.
type AppStoreVersionExperimentTreatmentCreateAttributes struct {
	Name        string `json:"name"`
	AppIconName string `json:"appIconName,omitempty"`
}

// AppStoreVersionExperimentTreatmentLocalizationsResponse is the response from treatment localization list endpoints.
type AppStoreVersionExperimentTreatmentLocalizationsResponse = Response[AppStoreVersionExperimentTreatmentLocalizationAttributes]

// AppStoreVersionExperimentTreatmentLocalizationCreateRelationships describes create relationships.
type AppStoreVersionExperimentTreatmentLocalizationCreateRelationships struct {
	AppStoreVersionExperimentTreatment *Relationship `json:"appStoreVersionExperimentTreatment"`
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

// GetAppStoreVersionExperimentTreatmentLocalization retrieves a treatment localization by ID.
func (c *Client) GetAppStoreVersionExperimentTreatmentLocalization(ctx context.Context, localizationID string, opts ...LocalizationReadOption) (*AppStoreVersionExperimentTreatmentLocalizationResponse, error) {
	localizationID = strings.TrimSpace(localizationID)
	if localizationID == "" {
		return nil, fmt.Errorf("localizationID is required")
	}
	return getJSON[AppStoreVersionExperimentTreatmentLocalizationResponse](ctx, c, readRequest{Path: localizationReadPath(fmt.Sprintf("/v1/appStoreVersionExperimentTreatmentLocalizations/%s", localizationID), opts)})
}
