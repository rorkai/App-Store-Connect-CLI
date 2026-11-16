package cmdtest

import (
	"strings"
	"testing"
)

func TestWebDeveloperTeamFlagAcceptedOnPortalCommands(t *testing.T) {
	root := RootCommand("1.2.3")
	tests := []struct {
		path []string
	}{
		{path: []string{"web", "bundle-ids", "capabilities", "enable"}},
		{path: []string{"web", "app-groups", "list"}},
		{path: []string{"web", "app-groups", "create"}},
		{path: []string{"web", "app-groups", "assign"}},
		{path: []string{"web", "app-groups", "unassign"}},
		{path: []string{"web", "app-groups", "set"}},
		{path: []string{"web", "app-groups", "delete"}},
		{path: []string{"web", "agreements", "status"}},
		{path: []string{"web", "agreements", "download"}},
		{path: []string{"web", "agreements", "accept"}},
	}
	for _, test := range tests {
		t.Run(strings.Join(test.path, " "), func(t *testing.T) {
			sub := findSubcommand(root, test.path...)
			if sub == nil {
				t.Fatalf("expected %s to be registered", strings.Join(test.path, " "))
			}
			if sub.FlagSet.Lookup("developer-team") == nil {
				t.Fatalf("expected --developer-team on %s", strings.Join(test.path, " "))
			}
		})
	}
}

func TestWebPrivacyPullRejectsDeveloperTeamFlag(t *testing.T) {
	assertUsageExitWithDeps(t, nil, []string{"web", "privacy", "pull", "--app", "123", "--developer-team", "X"}, "unknown flag `--developer-team`")
}

func TestWebDeveloperTeamRejectsBlankSelectorBeforeSession(t *testing.T) {
	assertUsageExit(t, []string{"web", "app-groups", "list", "--developer-team", "", "--output", "json"}, "--developer-team must be a Developer Portal team ID or exact team name")
	assertUsageExit(t, []string{"web", "app-groups", "create", "--name", "X", "--identifier", "group.x", "--confirm", "--developer-team", "   "}, "--developer-team must be a Developer Portal team ID or exact team name")
}
