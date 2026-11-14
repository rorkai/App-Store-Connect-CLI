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

// GameCenterLeaderboardSetMemberLocalizationCreateData is the data portion of a member localization create request.
type GameCenterLeaderboardSetMemberLocalizationCreateData struct {
	Type          ResourceType                                               `json:"type"`
	Attributes    GameCenterLeaderboardSetMemberLocalizationCreateAttributes `json:"attributes"`
	Relationships *GameCenterLeaderboardSetMemberLocalizationRelationships   `json:"relationships"`
}
