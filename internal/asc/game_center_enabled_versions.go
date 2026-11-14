package asc

// GameCenterEnabledVersionAttributes represents a Game Center enabled version resource.
type GameCenterEnabledVersionAttributes struct {
	Platform      Platform    `json:"platform,omitempty"`
	VersionString string      `json:"versionString,omitempty"`
	IconAsset     *ImageAsset `json:"iconAsset,omitempty"`
}

// GameCenterEnabledVersionsResponse is the response from enabled versions list endpoints.
type GameCenterEnabledVersionsResponse = Response[GameCenterEnabledVersionAttributes]
