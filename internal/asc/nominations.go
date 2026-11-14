package asc

import (
	"context"
	"fmt"
	"strings"
)

// NominationType represents a featuring nomination type.
type NominationType string

const (
	NominationTypeAppLaunch       NominationType = "APP_LAUNCH"
	NominationTypeAppEnhancements NominationType = "APP_ENHANCEMENTS"
	NominationTypeNewContent      NominationType = "NEW_CONTENT"
)

// NominationState represents the state of a nomination.
type NominationState string

const (
	NominationStateDraft     NominationState = "DRAFT"
	NominationStateSubmitted NominationState = "SUBMITTED"
	NominationStateArchived  NominationState = "ARCHIVED"
)

// NominationAttributes describes a featuring nomination.
type NominationAttributes struct {
	Name                       string          `json:"name,omitempty"`
	Type                       NominationType  `json:"type,omitempty"`
	Description                string          `json:"description,omitempty"`
	CreatedDate                string          `json:"createdDate,omitempty"`
	LastModifiedDate           string          `json:"lastModifiedDate,omitempty"`
	SubmittedDate              string          `json:"submittedDate,omitempty"`
	State                      NominationState `json:"state,omitempty"`
	PublishStartDate           string          `json:"publishStartDate,omitempty"`
	PublishEndDate             string          `json:"publishEndDate,omitempty"`
	DeviceFamilies             []DeviceFamily  `json:"deviceFamilies,omitempty"`
	Locales                    []string        `json:"locales,omitempty"`
	SupplementalMaterialsURIs  []string        `json:"supplementalMaterialsUris,omitempty"`
	HasInAppEvents             bool            `json:"hasInAppEvents,omitempty"`
	LaunchInSelectMarketsFirst bool            `json:"launchInSelectMarketsFirst,omitempty"`
	Notes                      string          `json:"notes,omitempty"`
	PreOrderEnabled            bool            `json:"preOrderEnabled,omitempty"`
}

// NominationCreateAttributes describes attributes for creating a nomination.
type NominationCreateAttributes struct {
	Name                       string         `json:"name"`
	Type                       NominationType `json:"type"`
	Description                string         `json:"description"`
	Submitted                  bool           `json:"submitted"`
	PublishStartDate           string         `json:"publishStartDate"`
	PublishEndDate             *string        `json:"publishEndDate,omitempty"`
	DeviceFamilies             []DeviceFamily `json:"deviceFamilies,omitempty"`
	Locales                    []string       `json:"locales,omitempty"`
	SupplementalMaterialsURIs  []string       `json:"supplementalMaterialsUris,omitempty"`
	HasInAppEvents             *bool          `json:"hasInAppEvents,omitempty"`
	LaunchInSelectMarketsFirst *bool          `json:"launchInSelectMarketsFirst,omitempty"`
	Notes                      *string        `json:"notes,omitempty"`
	PreOrderEnabled            *bool          `json:"preOrderEnabled,omitempty"`
}

// NominationUpdateAttributes describes attributes for updating a nomination.
type NominationUpdateAttributes struct {
	Name                       *string         `json:"name,omitempty"`
	Type                       *NominationType `json:"type,omitempty"`
	Description                *string         `json:"description,omitempty"`
	Submitted                  *bool           `json:"submitted,omitempty"`
	Archived                   *bool           `json:"archived,omitempty"`
	PublishStartDate           *string         `json:"publishStartDate,omitempty"`
	PublishEndDate             *string         `json:"publishEndDate,omitempty"`
	DeviceFamilies             []DeviceFamily  `json:"deviceFamilies,omitempty"`
	Locales                    []string        `json:"locales,omitempty"`
	SupplementalMaterialsURIs  []string        `json:"supplementalMaterialsUris,omitempty"`
	HasInAppEvents             *bool           `json:"hasInAppEvents,omitempty"`
	LaunchInSelectMarketsFirst *bool           `json:"launchInSelectMarketsFirst,omitempty"`
	Notes                      *string         `json:"notes,omitempty"`
	PreOrderEnabled            *bool           `json:"preOrderEnabled,omitempty"`
}

// NominationRelationships describes relationships for nominations.
type NominationRelationships struct {
	RelatedApps          *RelationshipList `json:"relatedApps,omitempty"`
	InAppEvents          *RelationshipList `json:"inAppEvents,omitempty"`
	SupportedTerritories *RelationshipList `json:"supportedTerritories,omitempty"`
}

// NominationUpdateData is the data portion of a nomination update request.
type NominationUpdateData struct {
	Type          ResourceType                `json:"type"`
	ID            string                      `json:"id"`
	Attributes    *NominationUpdateAttributes `json:"attributes,omitempty"`
	Relationships *NominationRelationships    `json:"relationships,omitempty"`
}

// NominationDeleteResult represents CLI output for deletions.
type NominationDeleteResult struct {
	ID      string `json:"id"`
	Deleted bool   `json:"deleted"`
}

// GetNomination retrieves a nomination by ID.
func (c *Client) GetNomination(ctx context.Context, nominationID string, opts ...NominationsOption) (*NominationResponse, error) {
	nominationID = strings.TrimSpace(nominationID)
	if nominationID == "" {
		return nil, fmt.Errorf("nominationID is required")
	}

	query := applyOptions(opts)
	return getJSON[NominationResponse](ctx, c, readRequest{
		Path:  fmt.Sprintf("/v1/nominations/%s", nominationID),
		Query: buildNominationsDetailQuery(query),
	})
}

// CreateNomination creates a nomination.
func (c *Client) CreateNomination(ctx context.Context, attrs NominationCreateAttributes, relationships NominationRelationships) (*NominationResponse, error) {
	payload := NominationCreateRequest{
		Data: NominationCreateData{
			Type:          ResourceTypeNominations,
			Attributes:    attrs,
			Relationships: relationships,
		},
	}

	return sendJSON[NominationResponse](ctx, c, writeRequest{Method: "POST", Path: "/v1/nominations", Payload: payload})
}

// UpdateNomination updates a nomination by ID.
func (c *Client) UpdateNomination(ctx context.Context, nominationID string, attrs *NominationUpdateAttributes, relationships *NominationRelationships) (*NominationResponse, error) {
	nominationID = strings.TrimSpace(nominationID)
	if nominationID == "" {
		return nil, fmt.Errorf("nominationID is required")
	}

	payload := NominationUpdateRequest{
		Data: NominationUpdateData{
			Type: ResourceTypeNominations,
			ID:   nominationID,
		},
	}
	if attrs != nil {
		payload.Data.Attributes = attrs
	}
	if relationships != nil {
		payload.Data.Relationships = relationships
	}

	return sendJSON[NominationResponse](ctx, c, writeRequest{Method: "PATCH", Path: fmt.Sprintf("/v1/nominations/%s", nominationID), Payload: payload})
}
