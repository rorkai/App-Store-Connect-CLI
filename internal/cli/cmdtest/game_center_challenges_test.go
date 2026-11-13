package cmdtest

import (
	"testing"
)

func TestGameCenterChallengesValidationErrors(t *testing.T) {
	runGameCenterUsageCases(t, []gameCenterUsageCase{
		{[]string{"game-center", "challenges", "list"}, "--app is required (or set ASC_APP_ID)"},
		{[]string{"game-center", "challenges", "create", "--reference-name", "Weekly", "--vendor-id", "com.example.weekly"}, "--app is required (or set ASC_APP_ID)"},
		{[]string{"game-center", "challenges", "create", "--app", "APP_ID", "--group-id", "GROUP_ID", "--reference-name", "Weekly", "--vendor-id", "com.example.weekly"}, "--app cannot be used with --group-id"},
		{[]string{"game-center", "challenges", "create", "--group-id", "GROUP_ID", "--reference-name", "Weekly", "--vendor-id", "com.example.weekly", "--leaderboard-id", "LEADERBOARD_ID"}, "--vendor-id must start with \"grp.\" when using --group-id"},
		{[]string{"game-center", "challenges", "create", "--app", "APP_ID", "--vendor-id", "com.example.weekly"}, "--reference-name is required"},
		{[]string{"game-center", "challenges", "create", "--app", "APP_ID", "--reference-name", "Weekly"}, "--vendor-id is required"},
		{[]string{"game-center", "challenges", "update"}, "--id is required"},
		{[]string{"game-center", "challenges", "update", "--id", "CHALLENGE_ID"}, "at least one update flag is required"},
		{[]string{"game-center", "challenges", "delete", "--confirm"}, "--id is required"},
		{[]string{"game-center", "challenges", "delete", "--id", "CHALLENGE_ID"}, "--confirm is required"},
		{[]string{"game-center", "challenges", "versions", "list"}, "--challenge-id is required"},
		{[]string{"game-center", "challenges", "versions", "create"}, "--challenge-id is required"},
		{[]string{"game-center", "challenges", "localizations", "list"}, "--version-id is required"},
		{[]string{"game-center", "challenges", "localizations", "create", "--locale", "en-US", "--name", "Weekly", "--description", "Desc"}, "--version-id is required"},
		{[]string{"game-center", "challenges", "localizations", "create", "--version-id", "VER_ID", "--name", "Weekly", "--description", "Desc"}, "--locale is required"},
		{[]string{"game-center", "challenges", "localizations", "create", "--version-id", "VER_ID", "--locale", "en-US", "--description", "Desc"}, "--name is required"},
		{[]string{"game-center", "challenges", "localizations", "create", "--version-id", "VER_ID", "--locale", "en-US", "--name", "Weekly"}, "--description is required"},
		{[]string{"game-center", "challenges", "localizations", "update"}, "--id is required"},
		{[]string{"game-center", "challenges", "localizations", "update", "--id", "LOC_ID"}, "at least one update flag is required"},
		{[]string{"game-center", "challenges", "localizations", "delete", "--confirm"}, "--id is required"},
		{[]string{"game-center", "challenges", "localizations", "delete", "--id", "LOC_ID"}, "--confirm is required"},
		{[]string{"game-center", "challenges", "images", "upload", "--localization-id", "LOC_ID"}, "--file is required"},
		{[]string{"game-center", "challenges", "images", "delete", "--confirm"}, "--id is required"},
		{[]string{"game-center", "challenges", "images", "delete", "--id", "IMAGE_ID"}, "--confirm is required"},
		{[]string{"game-center", "challenges", "releases", "list"}, "--app is required (or set ASC_APP_ID)"},
		{[]string{"game-center", "challenges", "releases", "create"}, "--version-id is required"},
		{[]string{"game-center", "challenges", "releases", "delete", "--confirm"}, "--id is required"},
		{[]string{"game-center", "challenges", "releases", "delete", "--id", "REL_ID"}, "--confirm is required"},
		{[]string{"game-center", "challenges", "list", "--app", "APP_ID", "--limit", "500"}, "game-center challenges list: --limit must be between 1 and 200"},
	})
}
