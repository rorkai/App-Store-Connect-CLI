package asc

// AppPriceScheduleAttributes describes an app price schedule resource.
type AppPriceScheduleAttributes struct {
	// Usually empty - data is in relationships.
}

// AppPriceAttributes describes a price schedule entry.
type AppPriceAttributes struct {
	StartDate string `json:"startDate,omitempty"`
	EndDate   string `json:"endDate,omitempty"`
	Manual    bool   `json:"manual,omitempty"`
}

// AppAvailabilityV2Attributes describes app availability metadata.
type AppAvailabilityV2Attributes struct {
	AvailableInNewTerritories bool `json:"availableInNewTerritories"`
}

// TerritoryAvailabilityAttributes describes availability for a territory.
type TerritoryAvailabilityAttributes struct {
	Available           bool     `json:"available"`
	ReleaseDate         string   `json:"releaseDate,omitempty"`
	PreOrderEnabled     bool     `json:"preOrderEnabled,omitempty"`
	PreOrderPublishDate string   `json:"preOrderPublishDate,omitempty"`
	ContentStatuses     []string `json:"contentStatuses,omitempty"`
}

// Response types
type (
	TerritoriesResponse             = Response[TerritoryAttributes]
	TerritoryResponse               = SingleResponse[TerritoryAttributes]
	AppPricePointsV3Response        = Response[AppPricePointV3Attributes]
	AppPriceScheduleResponse        = SingleResponse[AppPriceScheduleAttributes]
	AppPricesResponse               = Response[AppPriceAttributes]
	AppAvailabilityV2Response       = SingleResponse[AppAvailabilityV2Attributes]
	TerritoryAvailabilitiesResponse = Response[TerritoryAvailabilityAttributes]
	TerritoryAvailabilityResponse   = SingleResponse[TerritoryAvailabilityAttributes]
)

// AppPriceScheduleCreateAttributes defines inputs for creating a price schedule.
type AppPriceScheduleCreateAttributes struct {
	PricePointID    string `json:"-"`
	StartDate       string `json:"-"`
	BaseTerritoryID string `json:"-"`
	// Prices, when set, replaces PricePointID and StartDate with one manual
	// price per entry, so the schedule can price several territories.
	Prices []AppPriceSchedulePrice `json:"-"`
}

// AppPriceSchedulePrice is one manual price in a schedule create request.
type AppPriceSchedulePrice struct {
	PricePointID string
	StartDate    string
	EndDate      string
}

// AppPriceScheduleCreateRequest is a request to create a price schedule.
type AppPriceScheduleCreateRequest struct {
	Data     AppPriceScheduleCreateData `json:"data"`
	Included []AppPriceCreateResource   `json:"included,omitempty"`
}

// AppPriceCreateResource represents an app price resource for schedule creation.
type AppPriceCreateResource struct {
	Type          ResourceType          `json:"type"`
	ID            string                `json:"id,omitempty"`
	Attributes    AppPriceAttributes    `json:"attributes"`
	Relationships AppPriceRelationships `json:"relationships"`
}

// AppPriceRelationships describes relationships for app prices.
type AppPriceRelationships struct {
	AppPricePoint Relationship `json:"appPricePoint"`
}

// AppAvailabilityV2CreateAttributes defines inputs for POST /v2/appAvailabilities.
type AppAvailabilityV2CreateAttributes struct {
	AvailableInNewTerritories *bool                         `json:"availableInNewTerritories,omitempty"`
	TerritoryAvailabilityIDs  []string                      `json:"-"`
	TerritoryAvailabilities   []TerritoryAvailabilityCreate `json:"-"`
}

// TerritoryAvailabilityCreate defines a territory availability input.
type TerritoryAvailabilityCreate struct {
	TerritoryID     string
	Available       bool
	ReleaseDate     string
	PreOrderEnabled *bool
}

// AppAvailabilityV2CreateRequest is the payload for POST /v2/appAvailabilities.
type AppAvailabilityV2CreateRequest struct {
	Data     AppAvailabilityV2CreateData         `json:"data"`
	Included []AppAvailabilityV2IncludedResource `json:"included,omitempty"`
}

// AppAvailabilityV2CreateData is the data portion of availability create.
type AppAvailabilityV2CreateData struct {
	Type          ResourceType                         `json:"type"`
	Attributes    *AppAvailabilityV2CreateAttributes   `json:"attributes,omitempty"`
	Relationships AppAvailabilityV2CreateRelationships `json:"relationships"`
}

// AppAvailabilityV2CreateRelationships describes availability relationships.
type AppAvailabilityV2CreateRelationships struct {
	App                     Relationship      `json:"app"`
	TerritoryAvailabilities *RelationshipList `json:"territoryAvailabilities,omitempty"`
}

// TerritoryAvailabilityCreateAttributes describes attributes for create.
type TerritoryAvailabilityCreateAttributes struct {
	Available       bool   `json:"available"`
	ReleaseDate     string `json:"releaseDate,omitempty"`
	PreOrderEnabled *bool  `json:"preOrderEnabled,omitempty"`
}

// AppAvailabilityV2IncludedResource represents an included resource used by availability create.
// It supports both inline territory availability IDs and full inline territory availability objects.
type AppAvailabilityV2IncludedResource struct {
	Type          ResourceType                           `json:"type"`
	ID            string                                 `json:"id,omitempty"`
	Attributes    *TerritoryAvailabilityCreateAttributes `json:"attributes,omitempty"`
	Relationships *TerritoryAvailabilityRelationships    `json:"relationships,omitempty"`
}

// TerritoryAvailabilityRelationships describes relationships for availability.
type TerritoryAvailabilityRelationships struct {
	Territory Relationship `json:"territory"`
}

// TerritoryAvailabilityUpdateAttributes describes update inputs for a territory availability.
type TerritoryAvailabilityUpdateAttributes struct {
	Available        *bool   `json:"available,omitempty"`
	ReleaseDate      *string `json:"releaseDate,omitempty"`
	PreOrderEnabled  *bool   `json:"preOrderEnabled,omitempty"`
	ClearReleaseDate bool    `json:"-"`
}

// EndAppAvailabilityPreOrderAttributes describes the end pre-order response attributes.
type EndAppAvailabilityPreOrderAttributes struct{}
