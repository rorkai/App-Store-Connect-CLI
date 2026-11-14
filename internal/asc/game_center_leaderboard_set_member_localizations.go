package asc

import (
	"net/url"
)

// GameCenterLeaderboardSetMemberLocalizationAttributes represents a leaderboard set member localization.
type GameCenterLeaderboardSetMemberLocalizationAttributes struct {
	Name   string `json:"name"`
	Locale string `json:"locale"`
}

// GCLeaderboardSetMemberLocalizationsOption is a functional option for GetGameCenterLeaderboardSetMemberLocalizations.
type GCLeaderboardSetMemberLocalizationsOption func(*gcLeaderboardSetMemberLocalizationsQuery)

type gcLeaderboardSetMemberLocalizationsQuery struct {
	listQuery
	leaderboardSetIDs []string
	leaderboardIDs    []string
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

// GameCenterLeaderboardSetMemberLocalizationDeleteResult represents CLI output for member localization deletions.
type GameCenterLeaderboardSetMemberLocalizationDeleteResult struct {
	ID      string `json:"id"`
	Deleted bool   `json:"deleted"`
}
