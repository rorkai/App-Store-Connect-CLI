package asc

import (
	"net/url"
)

// GameCenterAppVersionsResponse is the response from app version list endpoints.
type GameCenterAppVersionsResponse = Response[GameCenterAppVersionAttributes]

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

// GameCenterAppVersionCreateData is the data portion of an app version create request.
type GameCenterAppVersionCreateData struct {
	Type          ResourceType                          `json:"type"`
	Attributes    *GameCenterAppVersionCreateAttributes `json:"attributes,omitempty"`
	Relationships *GameCenterAppVersionRelationships    `json:"relationships"`
}
