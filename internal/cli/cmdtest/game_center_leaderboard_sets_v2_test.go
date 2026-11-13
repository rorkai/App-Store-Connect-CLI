package cmdtest

import (
	"testing"
)

func TestGameCenterLeaderboardSetsV2ValidationErrors(t *testing.T) {
	runGameCenterUsageCases(t, []gameCenterUsageCase{
		{[]string{"game-center", "leaderboard-sets", "v2", "list"}, "--app is required (or set ASC_APP_ID)"},
		{[]string{"game-center", "leaderboard-sets", "v2", "create", "--reference-name", "Season", "--vendor-id", "com.example.season"}, "--app is required (or set ASC_APP_ID)"},
		{[]string{"game-center", "leaderboard-sets", "v2", "create", "--app", "APP_ID", "--group-id", "GROUP_ID", "--reference-name", "Season", "--vendor-id", "com.example.season"}, "--app cannot be used with --group-id"},
		{[]string{"game-center", "leaderboard-sets", "v2", "create", "--group-id", "GROUP_ID", "--reference-name", "Season", "--vendor-id", "com.example.season"}, "--vendor-id must start with \"grp.\" when using --group-id"},
		{[]string{"game-center", "leaderboard-sets", "v2", "create", "--app", "APP_ID", "--vendor-id", "com.example.season"}, "--reference-name is required"},
		{[]string{"game-center", "leaderboard-sets", "v2", "create", "--app", "APP_ID", "--reference-name", "Season"}, "--vendor-id is required"},
		{[]string{"game-center", "leaderboard-sets", "v2", "create", "--app", "APP_ID", "--reference-name", "Season", "--vendor-id", "com.example.season", "--leaderboard-ids", " , "}, "--leaderboard-ids must include at least one ID"},
		{[]string{"game-center", "leaderboard-sets", "v2", "members", "list"}, "--set-id is required"},
		{[]string{"game-center", "leaderboard-sets", "v2", "versions", "list"}, "--set-id is required"},
		{[]string{"game-center", "leaderboard-sets", "v2", "localizations", "create", "--locale", "en-US", "--name", "Season"}, "--version-id is required"},
		{[]string{"game-center", "leaderboard-sets", "v2", "localizations", "create", "--version-id", "VER_ID", "--name", "Season"}, "--locale is required"},
		{[]string{"game-center", "leaderboard-sets", "v2", "localizations", "create", "--version-id", "VER_ID", "--locale", "en-US"}, "--name is required"},
		{[]string{"game-center", "leaderboard-sets", "v2", "images", "view"}, "--id or --localization-id is required"},
		{[]string{"game-center", "leaderboard-sets", "v2", "images", "delete", "--confirm"}, "--id is required"},
		{[]string{"game-center", "leaderboard-sets", "v2", "images", "delete", "--id", "IMAGE_ID"}, "--confirm is required"},
	})
}
