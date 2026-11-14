package asc

import (
	"net/url"
	"strings"
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

// WithGCEnabledVersionsLimit sets the max number of enabled versions to return.
func WithGCEnabledVersionsLimit(limit int) GCEnabledVersionsOption {
	return func(q *gcEnabledVersionsQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithGCEnabledVersionsNextURL uses a next page URL directly.
func WithGCEnabledVersionsNextURL(next string) GCEnabledVersionsOption {
	return func(q *gcEnabledVersionsQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

func buildGCEnabledVersionsQuery(query *gcEnabledVersionsQuery) string {
	values := url.Values{}
	addLimit(values, query.limit)
	return values.Encode()
}
