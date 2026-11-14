package asc

import (
	"net/url"
	"strconv"
	"strings"
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
	enabled []string
}

// WithGCAppVersionsLimit sets the max number of app versions to return.
func WithGCAppVersionsLimit(limit int) GCAppVersionsOption {
	return func(q *gcAppVersionsQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithGCAppVersionsNextURL uses a next page URL directly.
func WithGCAppVersionsNextURL(next string) GCAppVersionsOption {
	return func(q *gcAppVersionsQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
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
