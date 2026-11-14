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
	platforms      []string
	versionStrings []string
	ids            []string
	sort           []string
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

// WithGCEnabledVersionsPlatforms filters enabled versions by platform.
func WithGCEnabledVersionsPlatforms(platforms []string) GCEnabledVersionsOption {
	return func(q *gcEnabledVersionsQuery) {
		q.platforms = normalizeList(platforms)
	}
}

// WithGCEnabledVersionsVersionStrings filters enabled versions by version string.
func WithGCEnabledVersionsVersionStrings(versions []string) GCEnabledVersionsOption {
	return func(q *gcEnabledVersionsQuery) {
		q.versionStrings = normalizeList(versions)
	}
}

// WithGCEnabledVersionsIDs filters enabled versions by ID.
func WithGCEnabledVersionsIDs(ids []string) GCEnabledVersionsOption {
	return func(q *gcEnabledVersionsQuery) {
		q.ids = normalizeList(ids)
	}
}

// WithGCEnabledVersionsSort sets the sort fields.
func WithGCEnabledVersionsSort(values []string) GCEnabledVersionsOption {
	return func(q *gcEnabledVersionsQuery) {
		q.sort = normalizeList(values)
	}
}

func buildGCEnabledVersionsQuery(query *gcEnabledVersionsQuery) string {
	values := url.Values{}
	addCSV(values, "filter[platform]", query.platforms)
	addCSV(values, "filter[versionString]", query.versionStrings)
	addCSV(values, "filter[id]", query.ids)
	addCSV(values, "sort", query.sort)
	addLimit(values, query.limit)
	return values.Encode()
}
