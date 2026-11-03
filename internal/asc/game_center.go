package asc

// GameCenterDetailAttributes represents a Game Center detail resource.
type GameCenterDetailAttributes struct {
	ArcadeEnabled                      bool `json:"arcadeEnabled,omitempty"`
	ChallengeEnabled                   bool `json:"challengeEnabled,omitempty"`
	LeaderboardSetEnabled              bool `json:"leaderboardSetEnabled,omitempty"`
	LeaderboardEnabled                 bool `json:"leaderboardEnabled,omitempty"`
	AchievementEnabled                 bool `json:"achievementEnabled,omitempty"`
	MultiplayerSessionEnabled          bool `json:"multiplayerSessionEnabled,omitempty"`
	MultiplayerTurnBasedSessionEnabled bool `json:"multiplayerTurnBasedSessionEnabled,omitempty"`
}

// GameCenterDetailResponse is the response from Game Center detail endpoints.
type GameCenterDetailResponse = SingleResponse[GameCenterDetailAttributes]

// Valid leaderboard formatters.
var ValidLeaderboardFormatters = []string{
	"INTEGER",
	"DECIMAL_POINT_1_PLACE",
	"DECIMAL_POINT_2_PLACE",
	"DECIMAL_POINT_3_PLACE",
	"ELAPSED_TIME_CENTISECOND",
	"ELAPSED_TIME_MINUTE",
	"ELAPSED_TIME_SECOND",
	"MONEY_POUND_DECIMAL",
	"MONEY_POUND",
	"MONEY_DOLLAR_DECIMAL",
	"MONEY_DOLLAR",
	"MONEY_EURO_DECIMAL",
	"MONEY_EURO",
	"MONEY_FRANC_DECIMAL",
	"MONEY_FRANC",
	"MONEY_KRONER_DECIMAL",
	"MONEY_KRONER",
	"MONEY_YEN",
}

// Valid leaderboard score sort types.
var ValidScoreSortTypes = []string{
	"ASC",
	"DESC",
}

// Valid leaderboard submission types.
var ValidSubmissionTypes = []string{
	"BEST_SCORE",
	"MOST_RECENT_SCORE",
}

// Valid leaderboard visibilities.
var ValidLeaderboardVisibilities = []string{
	"SHOW_FOR_ALL",
	"HIDE_FOR_ALL",
}

// GameCenterVersionState represents the state of a Game Center version.
type GameCenterVersionState string

const (
	GameCenterVersionStatePrepareForSubmission GameCenterVersionState = "PREPARE_FOR_SUBMISSION"
	GameCenterVersionStateReadyForReview       GameCenterVersionState = "READY_FOR_REVIEW"
	GameCenterVersionStateWaitingForReview     GameCenterVersionState = "WAITING_FOR_REVIEW"
	GameCenterVersionStateInReview             GameCenterVersionState = "IN_REVIEW"
	GameCenterVersionStateDeveloperRejected    GameCenterVersionState = "DEVELOPER_REJECTED"
	GameCenterVersionStateRejected             GameCenterVersionState = "REJECTED"
	GameCenterVersionStateAccepted             GameCenterVersionState = "ACCEPTED"
	GameCenterVersionStatePendingRelease       GameCenterVersionState = "PENDING_RELEASE"
	GameCenterVersionStateLive                 GameCenterVersionState = "LIVE"
	GameCenterVersionStateReplacedWithNew      GameCenterVersionState = "REPLACED_WITH_NEW_VERSION"
)
