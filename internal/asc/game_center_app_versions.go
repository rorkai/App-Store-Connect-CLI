package asc

import (
	"net/url"
)

// GameCenterAppVersionAttributes represents a Game Center app version resource.
type GameCenterAppVersionAttributes struct {
	Enabled bool `json:"enabled,omitempty"`
}

// GameCenterAppVersionsResponse is the response from app version list endpoints.
type GameCenterAppVersionsResponse = Response[GameCenterAppVersionAttributes]

// GameCenterAppVersionResponse is the response from app version detail endpoints.
type GameCenterAppVersionResponse = SingleResponse[GameCenterAppVersionAttributes]

// GCAppVersionsOption is a functional option for Game Center app version queries.
type GCAppVersionsOption func(*gcAppVersionsQuery)

type gcAppVersionsQuery struct {
	listQuery
}

func buildGCAppVersionsQuery(query *gcAppVersionsQuery) string {
	values := url.Values{}
	addLimit(values, query.limit)
	return values.Encode()
}

// GameCenterAppVersionCreateAttributes describes attributes for creating a Game Center app version.
type GameCenterAppVersionCreateAttributes struct {
	Enabled *bool `json:"enabled,omitempty"`
}

// GameCenterAppVersionUpdateAttributes describes attributes for updating a Game Center app version.
type GameCenterAppVersionUpdateAttributes struct {
	Enabled *bool `json:"enabled,omitempty"`
}

// GameCenterAppVersionRelationships describes relationships for Game Center app versions.
type GameCenterAppVersionRelationships struct {
	AppStoreVersion *Relationship `json:"appStoreVersion"`
}

// GameCenterAppVersionCreateData is the data portion of an app version create request.
type GameCenterAppVersionCreateData struct {
	Type          ResourceType                          `json:"type"`
	Attributes    *GameCenterAppVersionCreateAttributes `json:"attributes,omitempty"`
	Relationships *GameCenterAppVersionRelationships    `json:"relationships"`
}

// GameCenterAppVersionCreateRequest is a request to create a Game Center app version.
type GameCenterAppVersionCreateRequest struct {
	Data GameCenterAppVersionCreateData `json:"data"`
}

// GameCenterAppVersionUpdateData is the data portion of an app version update request.
type GameCenterAppVersionUpdateData struct {
	Type       ResourceType                          `json:"type"`
	ID         string                                `json:"id"`
	Attributes *GameCenterAppVersionUpdateAttributes `json:"attributes,omitempty"`
}

// GameCenterAppVersionUpdateRequest is a request to update a Game Center app version.
type GameCenterAppVersionUpdateRequest struct {
	Data GameCenterAppVersionUpdateData `json:"data"`
}
