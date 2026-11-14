package asc

import (
	"net/url"
)

// GameCenterEnabledVersionAttributes represents a Game Center enabled version resource.
type GameCenterEnabledVersionAttributes struct {
	Platform      Platform    `json:"platform,omitempty"`
	VersionString string      `json:"versionString,omitempty"`
	IconAsset     *ImageAsset `json:"iconAsset,omitempty"`
}

// GameCenterEnabledVersionsResponse is the response from enabled versions list endpoints.
type GameCenterEnabledVersionsResponse = Response[GameCenterEnabledVersionAttributes]

// GCEnabledVersionsOption is a functional option for Game Center enabled version queries.
type GCEnabledVersionsOption func(*gcEnabledVersionsQuery)

type gcEnabledVersionsQuery struct {
	listQuery
}

func buildGCEnabledVersionsQuery(query *gcEnabledVersionsQuery) string {
	values := url.Values{}
	addLimit(values, query.limit)
	return values.Encode()
}
