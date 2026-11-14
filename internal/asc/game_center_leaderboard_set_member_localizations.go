package asc

import (
	"net/url"
	"strings"
)

// GameCenterLeaderboardSetMemberLocalizationAttributes represents a leaderboard set member localization.
type GameCenterLeaderboardSetMemberLocalizationAttributes struct {
	Name   string `json:"name"`
	Locale string `json:"locale"`
}

// GameCenterLeaderboardSetMemberLocalizationsResponse is the response from member localization list endpoints.
type GameCenterLeaderboardSetMemberLocalizationsResponse = Response[GameCenterLeaderboardSetMemberLocalizationAttributes]

// GameCenterLeaderboardSetMemberLocalizationResponse is the response from member localization detail endpoints.
type GameCenterLeaderboardSetMemberLocalizationResponse = SingleResponse[GameCenterLeaderboardSetMemberLocalizationAttributes]

// GCLeaderboardSetMemberLocalizationsOption is a functional option for GetGameCenterLeaderboardSetMemberLocalizations.
type GCLeaderboardSetMemberLocalizationsOption func(*gcLeaderboardSetMemberLocalizationsQuery)

type gcLeaderboardSetMemberLocalizationsQuery struct {
	listQuery
	leaderboardSetIDs []string
	leaderboardIDs    []string
}

// WithGCLeaderboardSetMemberLocalizationsLimit sets the max number of localizations to return.
func WithGCLeaderboardSetMemberLocalizationsLimit(limit int) GCLeaderboardSetMemberLocalizationsOption {
	return func(q *gcLeaderboardSetMemberLocalizationsQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithGCLeaderboardSetMemberLocalizationsNextURL uses a next page URL directly.
func WithGCLeaderboardSetMemberLocalizationsNextURL(next string) GCLeaderboardSetMemberLocalizationsOption {
	return func(q *gcLeaderboardSetMemberLocalizationsQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

// WithGCLeaderboardSetMemberLocalizationsLeaderboardSetIDs filters by leaderboard set IDs.
func WithGCLeaderboardSetMemberLocalizationsLeaderboardSetIDs(ids []string) GCLeaderboardSetMemberLocalizationsOption {
	return func(q *gcLeaderboardSetMemberLocalizationsQuery) {
		q.leaderboardSetIDs = normalizeList(ids)
	}
}

// WithGCLeaderboardSetMemberLocalizationsLeaderboardIDs filters by leaderboard IDs.
func WithGCLeaderboardSetMemberLocalizationsLeaderboardIDs(ids []string) GCLeaderboardSetMemberLocalizationsOption {
	return func(q *gcLeaderboardSetMemberLocalizationsQuery) {
		q.leaderboardIDs = normalizeList(ids)
	}
}

func buildGCLeaderboardSetMemberLocalizationsQuery(query *gcLeaderboardSetMemberLocalizationsQuery) string {
	values := url.Values{}
	addCSV(values, "filter[gameCenterLeaderboardSet]", query.leaderboardSetIDs)
	addCSV(values, "filter[gameCenterLeaderboard]", query.leaderboardIDs)
	addLimit(values, query.limit)
	return values.Encode()
}

// GameCenterLeaderboardSetMemberLocalizationCreateAttributes describes attributes for creating a member localization.
type GameCenterLeaderboardSetMemberLocalizationCreateAttributes struct {
	Name   string `json:"name"`
	Locale string `json:"locale"`
}

// GameCenterLeaderboardSetMemberLocalizationUpdateAttributes describes attributes for updating a member localization.
type GameCenterLeaderboardSetMemberLocalizationUpdateAttributes struct {
	Name *string `json:"name,omitempty"`
}

// GameCenterLeaderboardSetMemberLocalizationRelationships describes relationships for member localizations.
type GameCenterLeaderboardSetMemberLocalizationRelationships struct {
	GameCenterLeaderboardSet *Relationship `json:"gameCenterLeaderboardSet"`
	GameCenterLeaderboard    *Relationship `json:"gameCenterLeaderboard"`
}

// GameCenterLeaderboardSetMemberLocalizationCreateData is the data portion of a member localization create request.
type GameCenterLeaderboardSetMemberLocalizationCreateData struct {
	Type          ResourceType                                               `json:"type"`
	Attributes    GameCenterLeaderboardSetMemberLocalizationCreateAttributes `json:"attributes"`
	Relationships *GameCenterLeaderboardSetMemberLocalizationRelationships   `json:"relationships"`
}

// GameCenterLeaderboardSetMemberLocalizationCreateRequest is a request to create a member localization.
type GameCenterLeaderboardSetMemberLocalizationCreateRequest struct {
	Data GameCenterLeaderboardSetMemberLocalizationCreateData `json:"data"`
}

// GameCenterLeaderboardSetMemberLocalizationUpdateData is the data portion of a member localization update request.
type GameCenterLeaderboardSetMemberLocalizationUpdateData struct {
	Type       ResourceType                                                `json:"type"`
	ID         string                                                      `json:"id"`
	Attributes *GameCenterLeaderboardSetMemberLocalizationUpdateAttributes `json:"attributes,omitempty"`
}

// GameCenterLeaderboardSetMemberLocalizationUpdateRequest is a request to update a member localization.
type GameCenterLeaderboardSetMemberLocalizationUpdateRequest struct {
	Data GameCenterLeaderboardSetMemberLocalizationUpdateData `json:"data"`
}

// GameCenterLeaderboardSetMemberLocalizationDeleteResult represents CLI output for member localization deletions.
type GameCenterLeaderboardSetMemberLocalizationDeleteResult struct {
	ID      string `json:"id"`
	Deleted bool   `json:"deleted"`
}
