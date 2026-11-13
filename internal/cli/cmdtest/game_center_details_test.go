package cmdtest

import (
	"testing"
)

func TestGameCenterDetailsValidationErrors(t *testing.T) {
	runGameCenterUsageCases(t, []gameCenterUsageCase{
		{[]string{"game-center", "details", "list"}, "--app is required (or set ASC_APP_ID)"},
		{[]string{"game-center", "details", "view"}, "--id is required"},
		{[]string{"game-center", "details", "app-versions", "list"}, "--id is required"},
		{[]string{"game-center", "details", "group", "view"}, "--id is required"},
		{[]string{"game-center", "details", "achievements-v2", "list"}, "--id is required"},
		{[]string{"game-center", "details", "leaderboards-v2", "list"}, "--id is required"},
		{[]string{"game-center", "details", "leaderboard-sets-v2", "list"}, "--id is required"},
		{[]string{"game-center", "details", "achievement-releases", "list"}, "--id is required"},
		{[]string{"game-center", "details", "leaderboard-releases", "list"}, "--id is required"},
		{[]string{"game-center", "details", "leaderboard-set-releases", "list"}, "--id is required"},
		{[]string{"game-center", "details", "achievements-v2", "list", "--id", "DETAIL_ID", "--limit", "300"}, "game-center details achievements-v2 list: --limit must be between 1 and 200"},
		{[]string{"game-center", "details", "metrics", "classic-matchmaking", "--granularity", "P1D"}, "--id is required"},
		{[]string{"game-center", "details", "metrics", "rule-based-matchmaking", "--granularity", "P1D"}, "--id is required"},
		{[]string{"game-center", "details", "metrics", "classic-matchmaking", "--id", "DETAIL_ID"}, "--granularity is required"},
		{[]string{"game-center", "details", "metrics", "rule-based-matchmaking", "--id", "DETAIL_ID"}, "--granularity is required"},
	})
}
