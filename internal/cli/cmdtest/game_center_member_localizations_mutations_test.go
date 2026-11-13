package cmdtest

import (
	"testing"
)

func TestGameCenterMemberLocalizationsMutationValidationErrors(t *testing.T) {
	runGameCenterUsageCases(t, []gameCenterUsageCase{
		{[]string{"game-center", "leaderboard-sets", "member-localizations", "create", "--leaderboard-id", "LB_ID", "--locale", "en-US", "--name", "Test"}, "--leaderboard-set-id is required"},
		{[]string{"game-center", "leaderboard-sets", "member-localizations", "create", "--leaderboard-set-id", "SET_ID", "--locale", "en-US", "--name", "Test"}, "--leaderboard-id is required"},
		{[]string{"game-center", "leaderboard-sets", "member-localizations", "create", "--leaderboard-set-id", "SET_ID", "--leaderboard-id", "LB_ID", "--name", "Test"}, "--locale is required"},
		{[]string{"game-center", "leaderboard-sets", "member-localizations", "create", "--leaderboard-set-id", "SET_ID", "--leaderboard-id", "LB_ID", "--locale", "en-US"}, "--name is required"},
		{[]string{"game-center", "leaderboard-sets", "member-localizations", "update", "--name", "New Name"}, "--id is required"},
		{[]string{"game-center", "leaderboard-sets", "member-localizations", "update", "--id", "LOCALIZATION_ID"}, "at least one update flag is required (--name)"},
		{[]string{"game-center", "leaderboard-sets", "member-localizations", "delete", "--confirm"}, "--id is required"},
		{[]string{"game-center", "leaderboard-sets", "member-localizations", "delete", "--id", "LOCALIZATION_ID"}, "--confirm is required"},
	})
}
