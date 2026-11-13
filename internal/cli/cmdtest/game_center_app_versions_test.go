package cmdtest

import (
	"testing"
)

func TestGameCenterAppVersionsValidationErrors(t *testing.T) {
	runGameCenterUsageCases(t, []gameCenterUsageCase{
		{[]string{"game-center", "app-versions", "list"}, "--app is required (or set ASC_APP_ID)"},
		{[]string{"game-center", "app-versions", "view"}, "--id is required"},
		{[]string{"game-center", "app-versions", "compatibility", "list"}, "--id is required"},
		{[]string{"game-center", "app-versions", "app-store-version", "view"}, "--id is required"},
		{[]string{"game-center", "app-versions", "list", "--app", "APP_ID", "--limit", "300"}, "game-center app-versions list: --limit must be between 1 and 200"},
	})
}
