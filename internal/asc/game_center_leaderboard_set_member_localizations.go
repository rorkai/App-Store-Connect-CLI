package asc

// GameCenterLeaderboardSetMemberLocalizationAttributes represents a leaderboard set member localization.
type GameCenterLeaderboardSetMemberLocalizationAttributes struct {
	Name   string `json:"name"`
	Locale string `json:"locale"`
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
