package asc

import (
	"net/url"
)

// GameCenterDetailsResponse is the response from Game Center detail list endpoints.
type GameCenterDetailsResponse = Response[GameCenterDetailAttributes]

// GCDetailsOption is a functional option for Game Center detail queries.
type GCDetailsOption func(*gcDetailsQuery)

type gcDetailsQuery struct {
	listQuery
}

func buildGCDetailsQuery(query *gcDetailsQuery) string {
	values := url.Values{}
	addLimit(values, query.limit)
	return values.Encode()
}

// GameCenterDetailCreateRelationships describes relationships for creating a Game Center detail.
type GameCenterDetailCreateRelationships struct {
	App *Relationship `json:"app"`
}

// GameCenterDetailUpdateRelationships describes relationships for updating a Game Center detail.
type GameCenterDetailUpdateRelationships struct {
	GameCenterGroup           *Relationship `json:"gameCenterGroup,omitempty"`
	DefaultLeaderboard        *Relationship `json:"defaultLeaderboard,omitempty"`
	DefaultLeaderboardV2      *Relationship `json:"defaultLeaderboardV2,omitempty"`
	DefaultGroupLeaderboard   *Relationship `json:"defaultGroupLeaderboard,omitempty"`
	DefaultGroupLeaderboardV2 *Relationship `json:"defaultGroupLeaderboardV2,omitempty"`
}

// GameCenterDetailCreateData is the data portion of a detail create request.
type GameCenterDetailCreateData struct {
	Type          ResourceType                         `json:"type"`
	Attributes    *GameCenterDetailCreateAttributes    `json:"attributes,omitempty"`
	Relationships *GameCenterDetailCreateRelationships `json:"relationships"`
}

// GameCenterDetailUpdateData is the data portion of a detail update request.
type GameCenterDetailUpdateData struct {
	Type          ResourceType                         `json:"type"`
	ID            string                               `json:"id"`
	Attributes    *GameCenterDetailUpdateAttributes    `json:"attributes,omitempty"`
	Relationships *GameCenterDetailUpdateRelationships `json:"relationships,omitempty"`
}
