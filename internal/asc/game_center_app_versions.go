package asc

import (
	"net/url"
	"strconv"
)

// GameCenterAppVersionsResponse is the response from app version list endpoints.
type GameCenterAppVersionsResponse = Response[GameCenterAppVersionAttributes]

// GCAppVersionsOption is a functional option for Game Center app version queries.
type GCAppVersionsOption func(*gcAppVersionsQuery)

type gcAppVersionsQuery struct {
	listQuery
	enabled []string
}

// WithGCAppVersionsEnabled filters by enabled status.
func WithGCAppVersionsEnabled(enabled bool) GCAppVersionsOption {
	return func(q *gcAppVersionsQuery) {
		q.enabled = []string{strconv.FormatBool(enabled)}
	}
}

func buildGCAppVersionsQuery(query *gcAppVersionsQuery) string {
	values := url.Values{}
	addCSV(values, "filter[enabled]", query.enabled)
	addLimit(values, query.limit)
	return values.Encode()
}

// GameCenterAppVersionCreateAttributes describes attributes for creating a Game Center app version.
type GameCenterAppVersionCreateAttributes struct {
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
