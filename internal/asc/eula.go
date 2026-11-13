package asc

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// EndUserLicenseAgreementAttributes describes an EULA resource.
type EndUserLicenseAgreementAttributes struct {
	AgreementText string `json:"agreementText,omitempty"`
}

// EndUserLicenseAgreementRelationships describes EULA relationships.
type EndUserLicenseAgreementRelationships struct {
	App         *Relationship     `json:"app,omitempty"`
	Territories *RelationshipList `json:"territories,omitempty"`
}

// EndUserLicenseAgreementResource represents an EULA resource.
type EndUserLicenseAgreementResource struct {
	Type          ResourceType                          `json:"type"`
	ID            string                                `json:"id"`
	Attributes    EndUserLicenseAgreementAttributes     `json:"attributes"`
	Relationships *EndUserLicenseAgreementRelationships `json:"relationships,omitempty"`
}

// EndUserLicenseAgreementResponse is the response from EULA endpoints.
type EndUserLicenseAgreementResponse struct {
	Data  EndUserLicenseAgreementResource `json:"data"`
	Links Links                           `json:"links"`
}

// EndUserLicenseAgreementCreateAttributes describes attributes for creating an EULA.
type EndUserLicenseAgreementCreateAttributes struct {
	AgreementText string `json:"agreementText"`
}

// EndUserLicenseAgreementCreateRelationships describes relationships for EULA creation.
type EndUserLicenseAgreementCreateRelationships struct {
	App         Relationship     `json:"app"`
	Territories RelationshipList `json:"territories"`
}

// EndUserLicenseAgreementCreateData is the data portion of an EULA create request.
type EndUserLicenseAgreementCreateData struct {
	Type          ResourceType                               `json:"type"`
	Attributes    EndUserLicenseAgreementCreateAttributes    `json:"attributes"`
	Relationships EndUserLicenseAgreementCreateRelationships `json:"relationships"`
}

// EndUserLicenseAgreementCreateRequest is a request to create an EULA.
type EndUserLicenseAgreementCreateRequest struct {
	Data EndUserLicenseAgreementCreateData `json:"data"`
}

// EndUserLicenseAgreementUpdateAttributes describes attributes for updating an EULA.
type EndUserLicenseAgreementUpdateAttributes struct {
	AgreementText *string `json:"agreementText,omitempty"`
}

// EndUserLicenseAgreementUpdateRelationships describes relationships for updating an EULA.
type EndUserLicenseAgreementUpdateRelationships struct {
	Territories *RelationshipList `json:"territories,omitempty"`
}

// EndUserLicenseAgreementUpdateData is the data portion of an EULA update request.
type EndUserLicenseAgreementUpdateData struct {
	Type          ResourceType                                `json:"type"`
	ID            string                                      `json:"id"`
	Attributes    *EndUserLicenseAgreementUpdateAttributes    `json:"attributes,omitempty"`
	Relationships *EndUserLicenseAgreementUpdateRelationships `json:"relationships,omitempty"`
}

// EndUserLicenseAgreementUpdateRequest is a request to update an EULA.
type EndUserLicenseAgreementUpdateRequest struct {
	Data EndUserLicenseAgreementUpdateData `json:"data"`
}

// EndUserLicenseAgreementDeleteResult represents CLI output for EULA deletions.
type EndUserLicenseAgreementDeleteResult struct {
	ID      string `json:"id"`
	Deleted bool   `json:"deleted"`
}

func buildTerritoryRelationship(ids []string) *RelationshipList {
	normalized := normalizeList(ids)
	if len(normalized) == 0 {
		return nil
	}
	data := make([]ResourceData, 0, len(normalized))
	for _, id := range normalized {
		data = append(data, ResourceData{
			Type: ResourceTypeTerritories,
			ID:   id,
		})
	}
	return &RelationshipList{Data: data}
}

// GetEndUserLicenseAgreement retrieves an EULA by ID.
func (c *Client) GetEndUserLicenseAgreement(ctx context.Context, id string) (*EndUserLicenseAgreementResponse, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, fmt.Errorf("id is required")
	}

	path := fmt.Sprintf("/v1/endUserLicenseAgreements/%s", id)
	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response EndUserLicenseAgreementResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetEndUserLicenseAgreementForApp retrieves an EULA for a specific app.
func (c *Client) GetEndUserLicenseAgreementForApp(ctx context.Context, appID string) (*EndUserLicenseAgreementResponse, error) {
	appID = strings.TrimSpace(appID)
	if appID == "" {
		return nil, fmt.Errorf("appID is required")
	}

	path := fmt.Sprintf("/v1/apps/%s/endUserLicenseAgreement", appID)
	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response EndUserLicenseAgreementResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// CreateEndUserLicenseAgreement creates an EULA for an app.
func (c *Client) CreateEndUserLicenseAgreement(ctx context.Context, appID, agreementText string, territoryIDs []string) (*EndUserLicenseAgreementResponse, error) {
	appID = strings.TrimSpace(appID)
	agreementText = strings.TrimSpace(agreementText)
	territoryIDs = normalizeList(territoryIDs)

	if appID == "" {
		return nil, fmt.Errorf("appID is required")
	}
	if agreementText == "" {
		return nil, fmt.Errorf("agreementText is required")
	}
	if len(territoryIDs) == 0 {
		return nil, fmt.Errorf("territoryIDs is required")
	}
	territories := buildTerritoryRelationship(territoryIDs)
	if territories == nil {
		return nil, fmt.Errorf("territoryIDs is required")
	}

	payload := EndUserLicenseAgreementCreateRequest{
		Data: EndUserLicenseAgreementCreateData{
			Type: ResourceTypeEndUserLicenseAgreements,
			Attributes: EndUserLicenseAgreementCreateAttributes{
				AgreementText: agreementText,
			},
			Relationships: EndUserLicenseAgreementCreateRelationships{
				App: Relationship{
					Data: ResourceData{
						Type: ResourceTypeApps,
						ID:   appID,
					},
				},
				Territories: *territories,
			},
		},
	}

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	data, err := c.do(ctx, "POST", "/v1/endUserLicenseAgreements", body)
	if err != nil {
		return nil, err
	}

	var response EndUserLicenseAgreementResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// UpdateEndUserLicenseAgreement updates an EULA by ID.
func (c *Client) UpdateEndUserLicenseAgreement(ctx context.Context, id string, agreementText *string, territoryIDs []string) (*EndUserLicenseAgreementResponse, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, fmt.Errorf("id is required")
	}

	var attrs *EndUserLicenseAgreementUpdateAttributes
	if agreementText != nil {
		trimmed := strings.TrimSpace(*agreementText)
		if trimmed == "" {
			return nil, fmt.Errorf("agreementText is required")
		}
		attrs = &EndUserLicenseAgreementUpdateAttributes{
			AgreementText: &trimmed,
		}
	}

	var relationships *EndUserLicenseAgreementUpdateRelationships
	if rel := buildTerritoryRelationship(territoryIDs); rel != nil {
		relationships = &EndUserLicenseAgreementUpdateRelationships{
			Territories: rel,
		}
	}

	if attrs == nil && relationships == nil {
		return nil, fmt.Errorf("at least one update field is required")
	}

	payload := EndUserLicenseAgreementUpdateRequest{
		Data: EndUserLicenseAgreementUpdateData{
			Type:          ResourceTypeEndUserLicenseAgreements,
			ID:            id,
			Attributes:    attrs,
			Relationships: relationships,
		},
	}

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/v1/endUserLicenseAgreements/%s", id)
	data, err := c.do(ctx, "PATCH", path, body)
	if err != nil {
		return nil, err
	}

	var response EndUserLicenseAgreementResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// DeleteEndUserLicenseAgreement deletes an EULA by ID.
func (c *Client) DeleteEndUserLicenseAgreement(ctx context.Context, id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("id is required")
	}

	path := fmt.Sprintf("/v1/endUserLicenseAgreements/%s", id)
	_, err := c.do(ctx, "DELETE", path, nil)
	return err
}

// GetEndUserLicenseAgreementTerritories retrieves territories for an EULA.
func (c *Client) GetEndUserLicenseAgreementTerritories(ctx context.Context, id string, opts ...EndUserLicenseAgreementTerritoriesOption) (*TerritoriesResponse, error) {
	id = strings.TrimSpace(id)
	query := &endUserLicenseAgreementTerritoriesQuery{}
	for _, opt := range opts {
		opt(query)
	}

	path := fmt.Sprintf("/v1/endUserLicenseAgreements/%s/territories", id)
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("endUserLicenseAgreementTerritories: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildEndUserLicenseAgreementTerritoriesQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response TerritoriesResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}
