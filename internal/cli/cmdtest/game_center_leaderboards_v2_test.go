package cmdtest

import (
	"testing"
)

func TestGameCenterLeaderboardsV2ValidationErrors(t *testing.T) {
	runGameCenterUsageCases(t, []gameCenterUsageCase{
		{[]string{"game-center", "leaderboards", "v2", "list"}, "--app is required (or set ASC_APP_ID)"},
		{[]string{"game-center", "leaderboards", "v2", "versions", "list"}, "--leaderboard-id is required"},
		{[]string{"game-center", "leaderboards", "v2", "localizations", "create", "--locale", "en-US", "--name", "Test"}, "--version-id is required"},
		{[]string{"game-center", "leaderboards", "v2", "localizations", "create", "--version-id", "VER_ID", "--name", "Test"}, "--locale is required"},
		{[]string{"game-center", "leaderboards", "v2", "localizations", "create", "--version-id", "VER_ID", "--locale", "en-US"}, "--name is required"},
		{[]string{"game-center", "leaderboards", "v2", "images", "view"}, "--id or --localization-id is required"},
		{[]string{"game-center", "leaderboards", "v2", "images", "delete", "--confirm"}, "--id is required"},
		{[]string{"game-center", "leaderboards", "v2", "images", "delete", "--id", "IMAGE_ID"}, "--confirm is required"},
	})
}
