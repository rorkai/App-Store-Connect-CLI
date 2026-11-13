package cmdtest

import (
	"testing"
)

func TestGameCenterGroupsValidationErrors(t *testing.T) {
	runGameCenterUsageCases(t, []gameCenterUsageCase{
		{[]string{"game-center", "groups", "list"}, "--app is required (or set ASC_APP_ID)"},
		{[]string{"game-center", "groups", "view"}, "--id is required"},
		{[]string{"game-center", "groups", "update", "--reference-name", "Group 1"}, "--id is required"},
		{[]string{"game-center", "groups", "update", "--id", "GROUP_ID"}, "--reference-name is required"},
		{[]string{"game-center", "groups", "delete", "--confirm"}, "--id is required"},
		{[]string{"game-center", "groups", "delete", "--id", "GROUP_ID"}, "--confirm is required"},
		{[]string{"game-center", "groups", "achievements", "set", "--ids", "ACH_1"}, "--group-id is required"},
		{[]string{"game-center", "groups", "achievements", "set", "--group-id", "GROUP_ID"}, "--ids is required"},
		{[]string{"game-center", "groups", "leaderboards", "set", "--ids", "LB_1"}, "--group-id is required"},
		{[]string{"game-center", "groups", "leaderboards", "set", "--group-id", "GROUP_ID"}, "--ids is required"},
		{[]string{"game-center", "groups", "details", "list"}, "--group-id is required"},
		{[]string{"game-center", "groups", "details", "list", "--group-id", "GROUP_ID", "--limit", "300"}, "game-center groups details list: --limit must be between 1 and 200"},
		{[]string{"game-center", "groups", "achievements", "list"}, "--group-id is required"},
		{[]string{"game-center", "groups", "leaderboards", "list"}, "--group-id is required"},
		{[]string{"game-center", "groups", "leaderboard-sets", "list"}, "--group-id is required"},
		{[]string{"game-center", "groups", "activities", "list"}, "--group-id is required"},
		{[]string{"game-center", "groups", "challenges", "list"}, "--group-id is required"},
	})
}
