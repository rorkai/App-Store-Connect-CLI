package asc

import (
	"net/url"
)

// GameCenterGroupDeleteResult represents CLI output for group deletions.
type GameCenterGroupDeleteResult struct {
	ID      string `json:"id"`
	Deleted bool   `json:"deleted"`
}

// GCGroupsOption is a functional option for GetGameCenterGroups.
type GCGroupsOption func(*gcGroupsQuery)

type gcGroupsQuery struct {
	listQuery
	gameCenterDetailIDs []string
}

// WithGCGroupsGameCenterDetailIDs filters groups by Game Center detail IDs.
func WithGCGroupsGameCenterDetailIDs(detailIDs []string) GCGroupsOption {
	return func(q *gcGroupsQuery) {
		q.gameCenterDetailIDs = normalizeList(detailIDs)
	}
}

func buildGCGroupsQuery(query *gcGroupsQuery) string {
	values := url.Values{}
	addCSV(values, "filter[gameCenterDetails]", query.gameCenterDetailIDs)
	addLimit(values, query.limit)
	return values.Encode()
}
