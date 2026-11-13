package cmdtest

import (
	"testing"
)

func TestGameCenterAchievementsV2ValidationErrors(t *testing.T) {
	runGameCenterUsageCases(t, []gameCenterUsageCase{
		{[]string{"game-center", "achievements", "v2", "list"}, "--app is required (or set ASC_APP_ID)"},
		{[]string{"game-center", "achievements", "v2", "versions", "list"}, "--achievement-id is required"},
		{[]string{"game-center", "achievements", "v2", "localizations", "create", "--locale", "en-US", "--name", "Test", "--before-earned-description", "Before", "--after-earned-description", "After"}, "--version-id is required"},
		{[]string{"game-center", "achievements", "v2", "localizations", "create", "--version-id", "VER_ID", "--name", "Test", "--before-earned-description", "Before", "--after-earned-description", "After"}, "--locale is required"},
		{[]string{"game-center", "achievements", "v2", "localizations", "create", "--version-id", "VER_ID", "--locale", "en-US", "--before-earned-description", "Before", "--after-earned-description", "After"}, "--name is required"},
		{[]string{"game-center", "achievements", "v2", "localizations", "create", "--version-id", "VER_ID", "--locale", "en-US", "--name", "Test", "--after-earned-description", "After"}, "--before-earned-description is required"},
		{[]string{"game-center", "achievements", "v2", "localizations", "create", "--version-id", "VER_ID", "--locale", "en-US", "--name", "Test", "--before-earned-description", "Before"}, "--after-earned-description is required"},
		{[]string{"game-center", "achievements", "v2", "images", "view"}, "--id or --localization-id is required"},
		{[]string{"game-center", "achievements", "v2", "images", "delete", "--confirm"}, "--id is required"},
		{[]string{"game-center", "achievements", "v2", "images", "delete", "--id", "IMAGE_ID"}, "--confirm is required"},
	})
}
