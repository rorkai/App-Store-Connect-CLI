package cmdtest

import (
	"testing"
)

func TestGameCenterLeaderboardSetsValidationErrors(t *testing.T) {
	runGameCenterUsageCases(t, []gameCenterUsageCase{
		{[]string{"game-center", "leaderboard-sets", "list"}, "--app is required (or set ASC_APP_ID)"},
		{[]string{"game-center", "leaderboard-sets", "create", "--reference-name", "Test", "--vendor-id", "com.test"}, "--app is required (or set ASC_APP_ID)"},
		{[]string{"game-center", "leaderboard-sets", "create", "--app", "APP_ID", "--vendor-id", "com.test"}, "--reference-name is required"},
		{[]string{"game-center", "leaderboard-sets", "create", "--app", "APP_ID", "--reference-name", "Test"}, "--vendor-id is required"},
		{[]string{"game-center", "leaderboard-sets", "create", "--app", "APP_ID", "--reference-name", "Test", "--vendor-id", "com.test", "--leaderboard-ids", " , "}, "--leaderboard-ids must include at least one ID"},
		{[]string{"game-center", "leaderboard-sets", "members", "list"}, "--set-id is required"},
		{[]string{"game-center", "leaderboard-sets", "list", "--app", "APP_ID", "--limit", "500"}, "game-center leaderboard-sets list: --limit must be between 1 and 200"},
		{[]string{"game-center", "leaderboard-sets", "member-localizations", "list", "--leaderboard-id", "LB_ID"}, "--set-id is required"},
		{[]string{"game-center", "leaderboard-sets", "member-localizations", "list", "--set-id", "SET_ID"}, "--leaderboard-id is required"},
		{[]string{"game-center", "leaderboard-sets", "member-localizations", "list", "--set-id", "SET_ID", "--leaderboard-id", "LB_ID", "--limit", "300"}, "game-center leaderboard-sets member-localizations list: --limit must be between 1 and 200"},
	})
}
