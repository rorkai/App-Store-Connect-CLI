package asc

import (
	"net/url"
	"strings"
)

// GameCenterGroupAttributes represents a Game Center group resource.
type GameCenterGroupAttributes struct {
	ReferenceName string `json:"referenceName,omitempty"`
}

// GameCenterGroupCreateAttributes describes attributes for creating a group.
type GameCenterGroupCreateAttributes struct {
	ReferenceName *string `json:"referenceName,omitempty"`
}

// GameCenterGroupUpdateAttributes describes attributes for updating a group.
type GameCenterGroupUpdateAttributes struct {
	ReferenceName *string `json:"referenceName,omitempty"`
}

// GameCenterGroupCreateData is the data portion of a group create request.
type GameCenterGroupCreateData struct {
	Type       ResourceType                     `json:"type"`
	Attributes *GameCenterGroupCreateAttributes `json:"attributes,omitempty"`
}

// GameCenterGroupCreateRequest is a request to create a group.
type GameCenterGroupCreateRequest struct {
	Data GameCenterGroupCreateData `json:"data"`
}

// GameCenterGroupUpdateData is the data portion of a group update request.
type GameCenterGroupUpdateData struct {
	Type       ResourceType                     `json:"type"`
	ID         string                           `json:"id"`
	Attributes *GameCenterGroupUpdateAttributes `json:"attributes,omitempty"`
}

// GameCenterGroupUpdateRequest is a request to update a group.
type GameCenterGroupUpdateRequest struct {
	Data GameCenterGroupUpdateData `json:"data"`
}

// GameCenterGroupsResponse is the response from group list endpoints.
type GameCenterGroupsResponse = Response[GameCenterGroupAttributes]

// GameCenterGroupResponse is the response from group detail endpoints.
type GameCenterGroupResponse = SingleResponse[GameCenterGroupAttributes]

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

// WithGCGroupsLimit sets the max number of groups to return.
func WithGCGroupsLimit(limit int) GCGroupsOption {
	return func(q *gcGroupsQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithGCGroupsNextURL uses a next page URL directly.
func WithGCGroupsNextURL(next string) GCGroupsOption {
	return func(q *gcGroupsQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
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
