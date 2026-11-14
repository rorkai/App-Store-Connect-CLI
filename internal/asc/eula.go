package asc

import (
	"context"
	"fmt"
	"strings"
)

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

// GetEndUserLicenseAgreementForApp retrieves an EULA for a specific app.
func (c *Client) GetEndUserLicenseAgreementForApp(ctx context.Context, appID string) (*EndUserLicenseAgreementResponse, error) {
	appID = strings.TrimSpace(appID)
	if appID == "" {
		return nil, fmt.Errorf("appID is required")
	}

	return getJSON[EndUserLicenseAgreementResponse](ctx, c, readRequest{Path: fmt.Sprintf("/v1/apps/%s/endUserLicenseAgreement", appID)})
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

	return sendJSON[EndUserLicenseAgreementResponse](ctx, c, writeRequest{Method: "POST", Path: "/v1/endUserLicenseAgreements", Payload: payload})
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

	return sendJSON[EndUserLicenseAgreementResponse](ctx, c, writeRequest{Method: "PATCH", Path: fmt.Sprintf("/v1/endUserLicenseAgreements/%s", id), Payload: payload})
}

// GetEndUserLicenseAgreementTerritories retrieves territories for an EULA.
func (c *Client) GetEndUserLicenseAgreementTerritories(ctx context.Context, id string, opts ...EndUserLicenseAgreementTerritoriesOption) (*TerritoriesResponse, error) {
	id = strings.TrimSpace(id)
	query := applyOptions(opts)
	return getJSON[TerritoriesResponse](ctx, c, readRequest{
		Path:        fmt.Sprintf("/v1/endUserLicenseAgreements/%s/territories", id),
		Query:       buildEndUserLicenseAgreementTerritoriesQuery(query),
		Next:        query.nextURL,
		NextContext: "endUserLicenseAgreementTerritories",
	})
}
